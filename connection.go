package lim

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// StatusHealth health status
	StatusHealth = iota
	// StatusClose connection has been closed
	StatusClose
)

// Connection net.Conn wrapper
type Connection struct {
	net.Conn
	status int64
	label  map[string]struct{}
	lm     sync.Mutex // for label field
	wm     sync.Mutex // for safe io writing
	once   sync.Once  // for Close method
}

func newConnection(conn net.Conn) (c *Connection) {
	c = &Connection{
		conn,
		0,
		make(map[string]struct{}),
		sync.Mutex{},
		sync.Mutex{},
		sync.Once{},
	}
	Label(c, "global")
	return
}

// Close close the connection
func (c *Connection) Close() {
	c.once.Do(func() {
		atomic.StoreInt64(&c.status, StatusClose)
		c.Conn.Close()
		ClearLabel(c)
	})
}

// SafetyWrite goroutine safety write
func (c *Connection) SafetyWrite(timeout time.Duration, b []byte) (n int, err error) {
	c.wm.Lock()
	defer c.wm.Unlock()
	err = c.SetWriteDeadline(time.Now().Add(timeout))
	if err != nil {
		return
	}
	return c.Conn.Write(b)
}

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

var connDict struct {
	sync.Map // kv container for label -> ConnSet
	sync.Mutex
}

// Label label a connection
func Label(c *Connection, label string) {
	if label == "" {
		return
	}
	defer func() {
		c.lm.Lock()
		defer c.lm.Unlock()
		c.label[label] = struct{}{}
	}()
	if val, ok := connDict.Load(label); ok {
		val.(*ConnSet).add(c)
		return
	}
	connDict.Lock()
	defer connDict.Unlock()
	if val, ok := connDict.Load(label); ok { // double check
		val.(*ConnSet).add(c)
		return
	}
	connSet := newConnSet()
	connSet.add(c)
	connDict.Store(label, connSet)
	return
}

// RemoveLabel remove a label from a connection
func RemoveLabel(c *Connection, label string) {
	defer func() {
		c.lm.Lock()
		defer c.lm.Unlock()
		delete(c.label, label)
	}()
	if val, ok := connDict.Load(label); ok {
		connSet := val.(*ConnSet)
		connSet.remove(c)
		if connSet.len() <= 0 {
			connDict.Delete(label)
		}
	}
}

// ClearLabel remove all the label from a connection
func ClearLabel(c *Connection) {
	c.lm.Lock()
	defer c.lm.Unlock()
	for label := range c.label {
		if val, ok := connDict.Load(label); ok {
			connSet := val.(*ConnSet)
			connSet.remove(c)
			if connSet.len() <= 0 {
				connDict.Delete(label)
			}
		}
		delete(c.label, label)
	}
}

// FindConnSet find a connections set by label
func FindConnSet(label string) (*ConnSet, error) {
	if val, ok := connDict.Load(label); ok {
		return val.(*ConnSet), nil
	}
	return nil, errors.New("can't find corresponding connections set")
}
