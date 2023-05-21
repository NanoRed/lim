package container

import (
	"sync/atomic"
	"unsafe"
)

type SyncPoolNode struct {
	status int32 // -1:deleted  0:end  1:valid
	next   unsafe.Pointer
	value  any // this field do not involve concurrency
}

func (n *SyncPoolNode) Load() any {
	return n.value
}

func (n *SyncPoolNode) Next() *SyncPoolNode {
	for {
		nextNode := (*SyncPoolNode)(atomic.LoadPointer(&n.next))
		if s := atomic.LoadInt32(&nextNode.status); s > 0 {
			return nextNode
		} else if s < 0 {
			// try to remove this node
			atomic.CompareAndSwapPointer(&n.next, unsafe.Pointer(nextNode), nextNode.next)
			n = nextNode
		} else {
			return nil // nil represents the iterating is over
		}
	}
}

type SyncPool struct {
	head unsafe.Pointer
}

func NewSyncPool() (syncPool *SyncPool) {
	sentinel := &SyncPoolNode{}
	syncPool = &SyncPool{unsafe.Pointer(sentinel)}
	sentinel.next = syncPool.head
	return
}

func (p *SyncPool) Entry() *SyncPoolNode {
	curNode := (*SyncPoolNode)(atomic.LoadPointer(&p.head))
	if s := atomic.LoadInt32(&curNode.status); s > 0 {
		return curNode
	} else if s < 0 {
		nextNode := curNode.Next()
		// try to remove this node
		if nextNode == nil {
			atomic.CompareAndSwapPointer(&p.head, unsafe.Pointer(curNode), unsafe.Pointer(&SyncPoolNode{}))
		} else {
			atomic.CompareAndSwapPointer(&p.head, unsafe.Pointer(curNode), unsafe.Pointer(nextNode))
		}
		return nextNode
	}
	return nil
}

func (p *SyncPool) Add(value any) *SyncPoolNode {
	newNode := &SyncPoolNode{status: 1, value: value}
	for {
		newNode.next = atomic.LoadPointer(&p.head)
		if atomic.CompareAndSwapPointer(&p.head, newNode.next, unsafe.Pointer(newNode)) {
			return newNode
		}
	}
}

func (p *SyncPool) Remove(old *SyncPoolNode) {
	atomic.CompareAndSwapInt32(&old.status, 1, -1)
}
