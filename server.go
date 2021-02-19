package lim

import (
	"log"
	"net"
)

// Server lim server
type Server struct {
	addr    string
	handler handler
}

// NewServer create a new server
func NewServer(addr string) *Server {
	return &Server{addr: addr, handler: &defaultHandler{}}
}

// RegisterHandler register a handler
func (s *Server) RegisterHandler(h handler) {
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
			log.Printf("accept error: %v", err)
			continue
		}
		go s.handler.handle(newConnection(conn))
	}
}
