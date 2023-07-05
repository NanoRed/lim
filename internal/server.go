package internal

import (
	"bytes"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/NanoRed/lim/internal/protocol"
	"github.com/NanoRed/lim/internal/websocket"
	"github.com/NanoRed/lim/pkg/logger"
	"github.com/NanoRed/lim/website"
)

type Server struct {
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) EnableWSS(addr string, certFile, keyFile string) {
	go func() {
		defer func() {
			logger.Warn("restart websocket server in 1 seconds...")
			time.Sleep(time.Second)
			s.EnableWSS(addr, certFile, keyFile)
		}()
		if err := websocket.NewServer(func(c net.Conn) {
			s.handle(&conn{Conn: c})
		}).ListenAndServeTLS(addr, certFile, keyFile); err != nil {
			logger.Error("websocket server error: %v", err)
		}
	}()
}

func (s *Server) EnableWebsite(addr string, certFile, keyFile string) {
	go func() {
		defer func() {
			logger.Warn("restart website server in 1 seconds...")
			time.Sleep(time.Second)
			s.EnableWebsite(addr, certFile, keyFile)
		}()
		http.Handle("/", http.FileServer(http.FS(website.ChatRoomFS)))
		if err := http.ListenAndServeTLS(addr, certFile, keyFile, nil); err != nil {
			logger.Error("website server error: %v", err)
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

	frame := &protocol.Frame{}
	processor := protocol.NewFrameProcessor(conn)

	// handshake
	if err := s.handshake(processor, frame); err != nil {
		logger.Error("verification failed: %v", err)
		return
	} else { // only response on a successful handshake
		_connlib.register(conn)
		if err := s.response(processor, frame, ""); err != nil {
			logger.Error("failed to response: %v", err)
			return
		}
	}

	for {
		err := processor.SetDecodeTimeout(ConnReadDuration)
		if err != nil {
			logger.Error("failed to set decode deadline: %v", err)
			return
		}
		raw, err := processor.Decode(frame)
		if err != nil {
			logger.Error("failed to read next frame: %v", err)
			return
		}
		switch frame.Act {
		case protocol.ActResponse:
			// heartbeat
		case protocol.ActMulticast:
			s.multicast(frame.Label, raw)
		case protocol.ActLabel:
			if err := s.label(conn, frame.Label, frame.Payload); err != nil {
				errMsg := "failed to (dis)label connection"
				logger.Error("%s: %v %s %v", errMsg, err, frame.Label, frame.Payload)
				if err := s.response(processor, frame, errMsg); err != nil {
					logger.Error("failed to response: %v", err)
					return
				}
			} else if err := s.response(processor, frame, ""); err != nil {
				logger.Error("failed to response: %v", err)
				return
			}
		}
	}
}

func (s *Server) response(processor *protocol.FrameProcessor, frame *protocol.Frame, errMsg string) (err error) {
	frame.Act = protocol.ActResponse
	frame.Label = ""
	frame.Payload = []byte(errMsg)
	return processor.Encode(frame)
}

func (s *Server) handshake(processor *protocol.FrameProcessor, frame *protocol.Frame) (err error) {
	err = processor.SetDecodeTimeout(ConnReadDuration)
	if err != nil {
		return
	}
	_, err = processor.Decode(frame)
	if err != nil {
		return
	}
	if frame.Act != protocol.ActHandshake || !bytes.Equal(frame.Payload, []byte("sample_secret")) { // TODO
		err = errors.New("illegal connection")
	}
	return
}

func (s *Server) multicast(label string, data []byte) (err error) {
	pool, err := _connlib.pool(label)
	if err != nil {
		return
	}
	go func() {
		for current := pool.Entry(); current != nil; current = current.Next() {
			current.Load().(*conn).Write(data)
		}
	}()
	return
}

func (s *Server) label(conn *conn, label string, payload []byte) (err error) {
	if len(payload) > 0 {
		switch payload[0] {
		case '+':
			err = _connlib.label(conn, label)
		case '*':
			for _, l := range strings.Split(label, "|") {
				if err = _connlib.label(conn, l); err != nil {
					break
				}
			}
		case '-':
			err = _connlib.dislabel(conn, label)
		case '/':
			for _, l := range strings.Split(label, "|") {
				if err = _connlib.dislabel(conn, l); err != nil {
					break
				}
			}
		default:
			err = errors.New("unknown operator")
		}
	} else {
		err = errors.New("operator missing")
	}
	return
}
