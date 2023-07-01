package websocket

import (
	"net"
	"net/http"

	"github.com/NanoRed/lim/pkg/logger"
	"github.com/gorilla/websocket"
)

type Server struct {
	handle func(conn net.Conn)
}

func NewServer(handle func(conn net.Conn)) *Server {
	return &Server{handle}
}

func (s *Server) ListenAndServeTLS(addr string, certFile, keyFile string) (err error) {
	return http.ListenAndServeTLS(addr, certFile, keyFile, http.HandlerFunc(func(wt http.ResponseWriter, r *http.Request) {
		var upgrader = websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		}
		c, err := upgrader.Upgrade(wt, r, nil)
		if err != nil {
			logger.Error("websocket upgrade error: %v", err)
			return
		}
		s.handle(newConn(c))
	}))
}
