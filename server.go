package lim

import (
	"context"
	"log"
	"net"
)

// Server lim server
type Server struct {
	addr string
}

// NewServer create a new server
func NewServer(addr string) *Server {
	return &Server{addr: addr}
}

// ListenAndServe create a listener and start to serve
func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

// Serve accept connection
func (s *Server) Serve(l net.Listener) error {
	defer l.Close()
	ctx := context.Background()
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		go newConnection(conn).serve(ctx)
	}
}
