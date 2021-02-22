package handler

import (
	"github.com/RedAFD/lim/pkg/connection"
)

// Handler handler interface
// you can implement your own handler
type Handler interface {
	Handle(connection.Conn)
}
