package handler

import (
	"time"

	"github.com/RedAFD/lim/pkg/connection"
)

// Handler handler interface
// you can implement your own handler
type Handler interface {
	Handle(*connection.Connection)
}

var handler Handler

func init() {
	handler = NewDefaultHandler(time.Second*10, time.Second*3)
}

// RegisterHandler register a handler
func RegisterHandler(h Handler) {
	handler = h
}

// Handle handle connection
func Handle(c *connection.Connection) {
	handler.Handle(c)
}
