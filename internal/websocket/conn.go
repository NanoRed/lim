package websocket

import (
	"bytes"
	"time"

	"github.com/gorilla/websocket"
)

type conn struct {
	*websocket.Conn
	rb   *bytes.Buffer
	ordw chan struct{}
}

func newConn(wc *websocket.Conn) *conn {
	c := &conn{
		wc,
		&bytes.Buffer{},
		make(chan struct{}, 1),
	}
	c.ordw <- struct{}{}
	return c
}

func (c *conn) Read(b []byte) (n int, err error) {
	length := len(b)
	n, _ = c.rb.Read(b)
	for r := length - n; r > 0; r = length - n {
		if mt, p, err := c.ReadMessage(); err != nil {
			return n, err
		} else if mt == websocket.BinaryMessage {
			c.rb.Write(p)
			n2, _ := c.rb.Read(b[n:])
			n += n2
		}
	}
	return
}

func (c *conn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}

func (c *conn) Write(b []byte) (n int, err error) {
	n = len(b)
	<-c.ordw
	err = c.WriteMessage(websocket.BinaryMessage, b)
	c.ordw <- struct{}{}
	if err != nil {
		n = 0
	}
	return
}
