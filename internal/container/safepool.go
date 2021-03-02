package container

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

// SafePool goroutine-safe pool
type SafePool struct {
	head   *unsafe.Pointer
	blanks sync.Pool // for reducing the pressure of the GC
}

// Object pool object
type Object struct {
	mark uint32 // 1 for end, 2 for deletion
	next *unsafe.Pointer
	v    interface{}
}

// Load load the object data
func (o *Object) Load() interface{} {
	return o.v
}

// Next traverse the next object from the pool
// nil represents that the pool has been finished traversing
func (o *Object) Next() *Object {
	obj := (*Object)(atomic.LoadPointer(o.next))
	for {
		switch atomic.LoadUint32(&obj.mark) {
		case 0:
			return obj
		case 1:
			return nil
		case 2:
			if !atomic.CompareAndSwapPointer(o.next, unsafe.Pointer(obj), atomic.LoadPointer(obj.next)) {
				o = obj
			}
			obj = (*Object)(atomic.LoadPointer(obj.next))
		default:
			return nil
		}
	}
}

// NewSafePool create a new safe pool
func NewSafePool() *SafePool {
	safePool := &SafePool{blanks: sync.Pool{New: func() interface{} {
		return unsafe.Pointer(&Object{mark: 1})
	}}}
	rootNext := safePool.blanks.Get().(unsafe.Pointer)
	safePool.head = &rootNext
	return safePool
}

// Entry get the entry of the pool
// nil represents that the pool is empty
func (l *SafePool) Entry() *Object {
	if obj := (*Object)(atomic.LoadPointer(l.head)); obj.v != nil {
		return obj
	}
	return nil
}

// Add Add a new object in the pool
func (l *SafePool) Add(value interface{}) *Object {
	blank := l.blanks.Get().(unsafe.Pointer)
	new := &Object{next: &blank, v: value}
	for {
		head := atomic.LoadPointer(l.head)
		l.blanks.Put(atomic.LoadPointer(new.next))
		atomic.StorePointer(new.next, head)
		if atomic.CompareAndSwapPointer(l.head, head, unsafe.Pointer(new)) {
			return new
		}
	}
}

// Remove remove a object from pool
func (l *SafePool) Remove(old *Object) {
	atomic.StoreUint32(&old.mark, 2)
}
