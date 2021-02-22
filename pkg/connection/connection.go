package connection

import (
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

// NewConnection create a connection
func NewConnection(conn net.Conn) (c *Connection) {
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
