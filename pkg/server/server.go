package server

import (
	"net"
	"time"

	"github.com/RedAFD/lim/pkg/connection"
	"github.com/RedAFD/lim/pkg/handler"
	"github.com/RedAFD/lim/pkg/logger"
)

// Server lim server
type Server struct {
	addr    string
	handler handler.Handler
}

// NewServer create a new server
func NewServer(addr string) *Server {
	return &Server{addr: addr, handler: handler.NewSrvHandler(time.Second*10, time.Second*3)}
}

// RegisterHandler register a handler to the server
func (s *Server) RegisterHandler(h handler.Handler) {
	s.handler = h
}

// ListenAndServe create a listener and start to serve
func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

// Serve listen and accept connection
func (s *Server) Serve(l net.Listener) error {
	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			logger.Error("accept error: %v", err)
			continue
		}
		go s.handler.Handle(connection.NewConnsrv(conn))
	}
}
