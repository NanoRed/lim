package connection

import (
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	statusHealth = iota
	statusClose
)

// Connsrv net.Conn wrapper
type Connsrv struct {
	net.Conn
	status int64
	label  map[string]struct{}
	lm     sync.Mutex // for label field
	wm     sync.Mutex // for safe io writing
	once   sync.Once  // for Close method
}

// NewConnsrv create a connection
func NewConnsrv(conn net.Conn) (c *Connsrv) {
	c = &Connsrv{
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
func (c *Connsrv) Close() error {
	c.once.Do(func() {
		atomic.StoreInt64(&c.status, statusClose)
		c.Conn.Close()
		ClearLabel(c)
	})
	return nil
}

// Label label the connection
func (c *Connsrv) Label(label string) {
	c.lm.Lock()
	defer c.lm.Unlock()
	c.label[label] = struct{}{}
}

// Dislabel dislabel the connection
func (c *Connsrv) Dislabel(label string) {
	c.lm.Lock()
	defer c.lm.Unlock()
	delete(c.label, label)
}

// ListLabel get all the connection's labels
func (c *Connsrv) ListLabel() []string {
	list := make([]string, 0)
	c.lm.Lock()
	defer c.lm.Unlock()
	for label := range c.label {
		list = append(list, label)
	}
	return list
}

// Health connection health status
func (c *Connsrv) Health() bool {
	if atomic.LoadInt64(&c.status) == statusHealth {
		return true
	}
	return false
}

// SafetyWrite goroutine safety write
func (c *Connsrv) SafetyWrite(timeout time.Duration, b []byte) (n int, err error) {
	c.wm.Lock()
	defer c.wm.Unlock()
	err = c.SetWriteDeadline(time.Now().Add(timeout))
	if err != nil {
		return
	}
	return c.Conn.Write(b)
}
