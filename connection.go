package lim

import (
	"net"
	"sync"
)

type connection struct {
	net.Conn
	status int64      // 0 means normal
	m      sync.Mutex // for label field
	label  map[string]struct{}
}

func newConnection(conn net.Conn) *connection {
	connection := &connection{conn, 0, sync.Mutex{}, make(map[string]struct{})}
	label(connection, "global")
	return connection
}
