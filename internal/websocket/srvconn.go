package websocket

import (
	"bytes"
	"time"

	"github.com/gorilla/websocket"
)

type srvConn struct {
	*websocket.Conn
	rb *bytes.Buffer
}

func newSrvConn(wc *websocket.Conn) *srvConn {
	return &srvConn{wc, &bytes.Buffer{}}
}

func (c *srvConn) Read(b []byte) (n int, err error) {
	length := len(b)
	n, _ = c.rb.Read(b)
	for r := length - n; r > 0; r = length - n {
		if mt, p, err := c.ReadMessage(); err != nil {
			return n, err
		} else if mt == websocket.TextMessage {
			c.rb.Write(p)
			n2, _ := c.rb.Read(b[n:])
			n += n2
		}
	}
	return
}

func (c *srvConn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}

func (c *srvConn) Write(b []byte) (n int, err error) {
	n = len(b)
	err = c.WriteMessage(websocket.TextMessage, b)
	if err != nil {
		n = 0
	}
	return
}
