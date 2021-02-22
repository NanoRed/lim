package connection

import (
	"sync"
	"sync/atomic"
)

// ConnSet a set of Connection
type ConnSet struct {
	sync.Map // set container for Connection
	length   int64
}

func newConnSet() *ConnSet {
	return &ConnSet{}
}

func (cs *ConnSet) len() int64 {
	return atomic.LoadInt64(&cs.length)
}

func (cs *ConnSet) add(c *Connection) {
	if _, ok := cs.Load(c); !ok {
		cs.Store(c, struct{}{})
		atomic.AddInt64(&cs.length, 1)
	}
}

func (cs *ConnSet) remove(c *Connection) {
	if _, ok := cs.Load(c); ok {
		cs.Delete(c)
		atomic.AddInt64(&cs.length, ^int64(0))
	}
}

// RangeDo range the grouped connections and do custom task
func (cs *ConnSet) RangeDo(f func(c *Connection)) {
	var wg sync.WaitGroup
	cs.Range(func(key, value interface{}) bool {
		conn := key.(*Connection)
		if atomic.LoadInt64(&conn.status) == StatusHealth {
			wg.Add(1)
			go func() {
				defer wg.Done()
				f(conn)
			}()
		}
		return true
	})
	wg.Wait()
}
