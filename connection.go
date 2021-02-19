package lim

import (
	"context"
	"net"
	"sync"
)

type connection struct {
	net.Conn
	status int64
	m      sync.Mutex // for label field
	label  map[string]struct{}
}

func newConnection(conn net.Conn) *connection {
	return &connection{conn, 0, sync.Mutex{}, map[string]struct{}{"global": {}}}
}

func (c *connection) serve(ctx context.Context) {
	defer c.Close()
	// TODO()
}
