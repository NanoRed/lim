package internal

import (
	"net"
	"time"
)

type conn struct {
	net.Conn
}

func (c *conn) Write(b []byte) (n int, err error) {
	err = c.Conn.SetWriteDeadline(time.Now().Add(ConnWriteTimeout))
	if err != nil {
		return
	}
	n, err = c.Conn.Write(b)
	return
}
