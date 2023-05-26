package websocket

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"syscall/js"
	"time"

	"github.com/NanoRed/lim/pkg/logger"
)

func NewDialer(ip, port string) func() (net.Conn, error) {
	return func() (net.Conn, error) {
		conn := &cliConn{
			ws:    js.Global().Get("Websocket").New(fmt.Sprintf("ws://%s:%s/", ip, port)),
			wsmu:  &sync.Mutex{},
			rb:    &bytes.Buffer{},
			errs:  make(chan error, 1),
			timer: time.NewTimer(0),
			block: make(chan struct{}, 1),
			close: make(chan struct{}, 1),
		}
		conn.timer.Stop()
		conn.ws.Call("addEventListener", "onmessage", js.FuncOf(func(this js.Value, args []js.Value) any {
			conn.rb.WriteString(args[0].String())
			select {
			case conn.block <- struct{}{}:
			default:
			}
			return nil
		}))
		conn.ws.Call("addEventListener", "onerror", js.FuncOf(func(this js.Value, args []js.Value) any {
			errMsg := args[0].Get("message").String()
			select {
			case conn.errs <- errors.New(errMsg):
			default:
			}
			logger.Error(errMsg)
			return nil
		}))
		conn.ws.Call("addEventListener", "onopen", js.FuncOf(func(this js.Value, args []js.Value) any {
			logger.Info("successfully connected to the websocket server")
			return nil
		}))
		conn.ws.Call("addEventListener", "onclose", js.FuncOf(func(this js.Value, args []js.Value) any {
			logger.Info("disconnected from websocket server")
			return nil
		}))
		js.Global().Set("onbeforeunload", js.FuncOf(func(this js.Value, args []js.Value) any {
			conn.Close()
			return nil
		}))
		return conn, nil
	}
}

type cliConn struct {
	ws    js.Value
	wsmu  *sync.Mutex
	rb    *bytes.Buffer
	errs  chan error
	timer *time.Timer
	block chan struct{}
	close chan struct{}
}

func (c *cliConn) Read(b []byte) (n int, err error) {
	length := len(b)
	for r := length - n; r > 0; r = length - n {
		if n2, _ := c.rb.Read(b[n:]); n2 == 0 {
			select {
			case <-c.block:
			case <-c.timer.C:
				err = errors.New("read timed out")
				return
			case <-c.close:
				err = io.EOF
				return
			}
		} else {
			n += n2
		}
	}
	return
}

func (c *cliConn) Write(b []byte) (n int, err error) {
	c.wsmu.Lock()
	defer c.wsmu.Unlock()
	c.ws.Call("send", string(b))
	for i := 0; i < 10; i++ {
		// loop 10 times to ensure the error has enqueued
		select {
		case err = <-c.errs:
		default:
		}
	}
	return
}

func (c *cliConn) Close() error {
	select {
	case c.close <- struct{}{}:
	default:
	}
	c.wsmu.Lock()
	defer c.wsmu.Unlock()
	c.ws.Call("close")
	return nil
}

func (c *cliConn) LocalAddr() net.Addr {
	return nil
}

func (c *cliConn) RemoteAddr() net.Addr {
	return nil
}

func (c *cliConn) SetDeadline(t time.Time) error {
	return c.SetReadDeadline(t)
}

func (c *cliConn) SetReadDeadline(t time.Time) error {
	if d := time.Until(t); d > 0 {
		c.timer.Reset(d)
	} else {
		c.timer.Stop()
	}
	return nil
}

func (c *cliConn) SetWriteDeadline(t time.Time) error {
	return nil
}
