package lim

import "net"

type handler interface {
	handle(net.Conn)
}

const (
	TypeHeartbeat = iota
	TypeBroadcast
)

type defaultHandler struct {
}

func (h *defaultHandler) handle(c net.Conn) {
	// TODO
}
