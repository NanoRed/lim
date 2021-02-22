package connection

import (
	"sync"
	"sync/atomic"
)

// ConnSet a set of Conn
type ConnSet struct {
	sync.Map // set container for Conn
	length   int64
}

func newConnSet() *ConnSet {
	return &ConnSet{}
}

func (cs *ConnSet) len() int64 {
	return atomic.LoadInt64(&cs.length)
}

func (cs *ConnSet) add(c Conn) {
	if _, ok := cs.Load(c); !ok {
		cs.Store(c, struct{}{})
		atomic.AddInt64(&cs.length, 1)
	}
}

func (cs *ConnSet) remove(c Conn) {
	if _, ok := cs.Load(c); ok {
		cs.Delete(c)
		atomic.AddInt64(&cs.length, ^int64(0))
	}
}

// RangeDo range the grouped connections and do custom task
func (cs *ConnSet) RangeDo(f func(Conn)) {
	var wg sync.WaitGroup
	cs.Range(func(key, value interface{}) bool {
		c := key.(Conn)
		if c.Health() {
			wg.Add(1)
			go func() {
				defer wg.Done()
				f(c)
			}()
		}
		return true
	})
	wg.Wait()
}
