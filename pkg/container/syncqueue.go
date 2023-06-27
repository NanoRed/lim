package container

import (
	"sync/atomic"
	"unsafe"
)

type syncQueueNode struct {
	next  unsafe.Pointer
	value any // this field do not involve concurrency
}

type SyncQueue struct {
	head  unsafe.Pointer
	tail  unsafe.Pointer
	block chan struct{}
}

func NewSyncQueue() (syncQueue *SyncQueue) {
	sentinel := &syncQueueNode{}
	syncQueue = &SyncQueue{
		block: make(chan struct{}, 1),
	}
	sentinel.next = unsafe.Pointer(syncQueue)
	syncQueue.head = unsafe.Pointer(sentinel)
	syncQueue.tail = syncQueue.head
	return
}

func (s *SyncQueue) Push(value any) {
	newNode := &syncQueueNode{value: value}
	for {
		headNode := atomic.LoadPointer(&s.head)
		if (*syncQueueNode)(headNode).next != nil &&
			atomic.CompareAndSwapPointer(&s.head, headNode, unsafe.Pointer(newNode)) {
			atomic.StorePointer(&(*syncQueueNode)(headNode).next, unsafe.Pointer(newNode))
			atomic.StorePointer(&newNode.next, unsafe.Pointer(s))
			select {
			case s.block <- struct{}{}:
			default:
			}
			return
		}
	}
}

func (s *SyncQueue) Pop() (value any) {
	tailNode := (*syncQueueNode)(s.tail)
	for {
		tailNext := atomic.LoadPointer(&tailNode.next)
		if tailNext == unsafe.Pointer(s) {
			<-s.block
		} else if (*syncQueueNode)(tailNext).next == nil ||
			(*syncQueueNode)(tailNext).next == unsafe.Pointer(s) {
			if atomic.CompareAndSwapPointer(&s.head, tailNext, s.tail) {
				atomic.CompareAndSwapPointer(&tailNode.next, tailNext, unsafe.Pointer(s))
				value = (*syncQueueNode)(tailNext).value
				return
			}
		} else if atomic.CompareAndSwapPointer(&tailNode.next, tailNext, (*syncQueueNode)(tailNext).next) {
			value = (*syncQueueNode)(tailNext).value
			return
		}
	}
}

func (s *SyncQueue) Install(in, out chan any) {
	go func(in chan any) {
		for element := range in {
			s.Push(element)
		}
	}(in)
	go func(out chan any) {
		defer func() { recover() }()
		for {
			out <- s.Pop()
		}
	}(out)
}
