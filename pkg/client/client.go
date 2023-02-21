package client

import (
	"net"
	"reflect"
	"time"

	"github.com/NanoRed/lim/pkg/connection"
	"github.com/NanoRed/lim/pkg/handler"
	"github.com/NanoRed/lim/pkg/logger"
)

// Client lim client
type Client struct {
	addr string
}

// NewClient create a new lim client
func NewClient(addr string) *Client {
	return &Client{addr: addr}
}

// DialForHandler dial for a handler
func (c *Client) DialForHandler(raw handler.CliHandler) (activated handler.CliHandler, err error) {
	conn, err := net.Dial("tcp", c.addr)
	if err != nil {
		logger.Error("dial error: %v", err)
		return
	}
	// yes. you can label connection in the client too if you need
	// but only for golang client application
	connection.Register(conn)
	if raw == nil {
		activated = handler.NewDefaultCliHandler(time.Second*5, time.Second*3)
	} else if reflect.TypeOf(raw).Kind() != reflect.Ptr {
		activated = reflect.ValueOf(raw).Addr().Interface().(handler.CliHandler)
	} else {
		activated = raw
	}
	err = activated.Bind(conn)
	if err != nil {
		logger.Error("bind error: %v", err)
		return
	}
	return
}
