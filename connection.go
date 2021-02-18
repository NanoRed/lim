package lim

import (
	"context"
	"net"
	"sync"
)

type connection struct {
	conn   net.Conn
	status int64
	m      sync.Mutex // for label field
	label  map[string]struct{}
}

func newConnection(conn net.Conn) *connection {
	return &connection{
		conn:  conn,
		label: map[string]struct{}{"global": {}},
	}
}

func (c *connection) serve(ctx context.Context) {
	defer c.conn.Close()
	// TODO()
}
