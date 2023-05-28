package internal

import (
	"bytes"
	"errors"
	"net"
	"time"

	"github.com/NanoRed/lim/internal/websocket"
	"github.com/NanoRed/lim/pkg/logger"
)

type Server struct {
	frameProcessor FrameProcessor
}

func NewServer(frameProcessor FrameProcessor) *Server {
	return &Server{frameProcessor}
}

func (s *Server) EnableWebsocket(addr string) {
	go func() {
		defer func() {
			logger.Warn("restart websocket server in 1 seconds...")
			time.Sleep(time.Second)
			s.EnableWebsocket(addr)
		}()
		if err := websocket.NewServer(func(c net.Conn) {
			s.handle(&conn{Conn: c})
		}).ListenAndServe(addr); err != nil {
			logger.Error("websocket server error: %v", err)
		}
	}()
}

func (s *Server) ListenAndServe(addr string) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Panic("failed to listen the address: %v", err)
	}
	defer ln.Close()
	for {
		c, err := ln.Accept()
		if err != nil {
			logger.Error("accept error: %v", err)
			continue
		}
		go s.handle(&conn{Conn: c})
	}
}

func (s *Server) handle(conn *conn) {
	defer _connlib.remove(conn)
	if err := s.handshake(conn); err != nil {
		logger.Error("verification failed: %v", err)
		return
	}
	for {
		err := conn.SetReadDeadline(time.Now().Add(ConnReadDuration))
		if err != nil {
			logger.Error("failed to set read deadline: %v", err)
			return
		}
		frame, err := s.frameProcessor.Next(conn)
		if err != nil {
			s.frameProcessor.Recycle(frame)
			logger.Error("failed to read next frame: %v", err)
			return
		}
		switch frame.Type() {
		case FTResponse:
			// heartbeat
			s.frameProcessor.Recycle(frame)
		case FTMulticast:
			s.multicast(conn, frame)
		case FTLabel:
			s.label(conn, frame)
		}
	}
}

func (s *Server) response(conn *conn, success bool, message ...string) {
	var payload []byte
	if success {
		payload = []byte{}
	} else if len(message) > 0 {
		payload = []byte(message[0])
	} else {
		payload = []byte{'i', 'n', 'v', 'a', 'l', 'i', 'd', ' ', 'r', 'e', 'q', 'u', 'e', 's', 't'}
	}
	respFrame := s.frameProcessor.Make(FTResponse, "", payload)
	defer s.frameProcessor.Recycle(respFrame)
	if _, err := conn.writex(respFrame.Encode()); err != nil {
		logger.Error("failed to write data: %s %v", payload, err)
		_connlib.remove(conn)
	}
}

func (s *Server) handshake(conn *conn) (err error) {
	err = conn.SetReadDeadline(time.Now().Add(ConnReadDuration))
	if err != nil {
		return
	}
	frame, err := s.frameProcessor.Next(conn)
	defer s.frameProcessor.Recycle(frame)
	if err != nil {
		return
	}
	if frame.Type() == FTHandshake && bytes.Equal(frame.Payload(), []byte("sample_secret")) { // TODO
		_connlib.register(conn)
		s.response(conn, true) // only response on a successful handshake
	} else {
		err = errors.New("illegal connection")
	}
	return
}

func (s *Server) multicast(conn *conn, frame Frame) {
	defer s.frameProcessor.Recycle(frame)
	label := frame.Label()
	pool, err := _connlib.pool(label)
	if err != nil {
		errMsg := "failed to get connection pool"
		logger.Error("%s: %s %v", errMsg, label, err)
		s.response(conn, false, errMsg)
		return
	}
	go s.sendToPool(pool, label, frame.Raw())
	s.response(conn, true)
}

func (s *Server) sendToPool(pool *pool, label string, data []byte) {
	for current := pool.Entry(); current != nil; current = current.Next() {
		go func(conn *conn, label string, data []byte) {
			if _, err := conn.writex(data); err != nil {
				logger.Error("failed to write data: %s %v", label, err)
				_connlib.remove(conn)
			}
		}(current.Load().(*conn), label, data)
	}
}

func (s *Server) label(conn *conn, frame Frame) {
	// TODO support set multiple labels in one time
	defer s.frameProcessor.Recycle(frame)
	label := frame.Label()
	if len(frame.Payload()) > 0 {
		if err := _connlib.dislabel(conn, label); err != nil {
			errMsg := "failed to dislabel connection"
			logger.Error("%s: %s %v", errMsg, label, err)
			s.response(conn, false, errMsg)
			return
		}
	} else if err := _connlib.label(conn, label); err != nil {
		errMsg := "failed to label connection"
		logger.Error("%s: %s %v", errMsg, label, err)
		s.response(conn, false, errMsg)
		return
	}
	s.response(conn, true)
}
