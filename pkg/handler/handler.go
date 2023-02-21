package handler

import (
	"bufio"
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NanoRed/lim/pkg/connection"
	"github.com/NanoRed/lim/pkg/logger"
)

// SrvHandler server handler interface
type SrvHandler interface {
	Handle(net.Conn)
}

// CliHandler client handler interface
type CliHandler interface {
	Bind(net.Conn) error
	Close()
}

// message type
const (
	// TypeLabel means puts a label on its connection
	TypeLabel = iota
	// TypeRemoveLabel remove the label from its connection
	TypeDislabel
	// TypeBroadcast means this is a broadcast job message
	TypeBroadcast
	// TypeResponse means this is a response message
	TypeResponse
)

// response result
const (
	// RespSuccess means this is a success response
	RespSuccess = iota
	// RespFailure means this is a failure response
	RespFailure
	// RespTimeout timeout and the result is unknowable
	RespTimeout
)

// client error code
const (
	_ = iota + 10000
	// ErrorUnknown unknown error
	ErrorUnknown
	// ErrorFailure failure
	ErrorFailure
	// ErrorInvalidMessageType invalid message type
	ErrorInvalidMessageType
	// ErrorInvalidLabel invalid label
	ErrorInvalidLabel
	// ErrorInvalidContent invalid content
	ErrorInvalidContent
)

// +------------------+
// +  server handler  +
// +------------------+

// DefaultSrvHandler default server handler
type DefaultSrvHandler struct {
	ioLimitedSec   time.Duration
	respLimitedSec time.Duration
}

// NewDefaultSrvHandler create a new server handler
// ioWaitingSec: limited seconds for waiting future read calls or write calls
// respWaitingSec: limited seconds for response message to client
func NewDefaultSrvHandler(ioLimitedSec, respLimitedSec time.Duration) *DefaultSrvHandler {
	return &DefaultSrvHandler{
		ioLimitedSec:   ioLimitedSec,
		respLimitedSec: respLimitedSec,
	}
}

// Handle handle connection
func (h *DefaultSrvHandler) Handle(c net.Conn) {
	defer connection.Close(c)
	reader := bufio.NewReader(c)
	for {
		err := c.SetReadDeadline(time.Now().Add(h.ioLimitedSec))
		if err != nil {
			logger.Error("set read deadline error: %v", err)
			return
		}
		sizestr, err := reader.ReadString(',')
		if err != nil {
			logger.Error("get message size error: %v", err)
			return
		}
		size, err := strconv.Atoi(strings.TrimRight(sizestr, ","))
		if err != nil {
			logger.Error("illegal size: %v", err)
			return
		} else if size == 0 {
			continue
		}
		// TODO(): size should have a limit value
		message := make([]byte, size)
		_, err = io.ReadFull(reader, message)
		if err != nil {
			logger.Error("unexpected size packet: %v", err)
			return
		}
		h.parse(c, message)
	}
}

func (h *DefaultSrvHandler) parse(c net.Conn, message []byte) {
	buffer := bytes.NewBuffer(message)
	seg1, err := buffer.ReadString(',')
	if err != nil {
		logger.Error("get message type error: %v", err)
		h.respFailure(c, ErrorInvalidMessageType)
		return
	}
	t, err := strconv.Atoi(strings.TrimRight(seg1, ","))
	if err != nil {
		logger.Error("illegal message type: %v", err)
		h.respFailure(c, ErrorInvalidMessageType)
		return
	}

	switch t {
	case TypeLabel:
		code, err := h.jobLabel(c, buffer)
		if err != nil {
			h.respFailure(c, code)
			return
		}
	case TypeDislabel:
		code, err := h.jobDislabel(c, buffer)
		if err != nil {
			h.respFailure(c, code)
			return
		}
	case TypeBroadcast:
		code, err := h.jobBroadcast(c, buffer)
		if err != nil {
			h.respFailure(c, code)
			return
		}
	}

	h.respSuccess(c)
}

func (h *DefaultSrvHandler) jobLabel(c net.Conn, buffer *bytes.Buffer) (code int, err error) {
	l, err := ioutil.ReadAll(buffer)
	if err != nil {
		code = ErrorInvalidLabel
		logger.Error("%v", err)
		return
	} else if len(l) == 0 {
		err = errors.New("empty label")
		code = ErrorInvalidLabel
		logger.Error("%v", err)
		return
	}
	if err = connection.Label(c, string(l)); err != nil {
		code = ErrorFailure
		logger.Error("%v", err)
		return
	}
	return
}

func (h *DefaultSrvHandler) jobDislabel(c net.Conn, buffer *bytes.Buffer) (code int, err error) {
	l, err := ioutil.ReadAll(buffer)
	if err != nil {
		code = ErrorInvalidLabel
		logger.Error("%v", err)
		return
	} else if len(l) == 0 {
		err = errors.New("empty label")
		code = ErrorInvalidLabel
		logger.Error("%v", err)
		return
	}
	if err = connection.Dislabel(c, string(l)); err != nil {
		code = ErrorFailure
		logger.Error("%v", err)
		return
	}
	return
}

func (h *DefaultSrvHandler) jobBroadcast(c net.Conn, buffer *bytes.Buffer) (code int, err error) {
	seg2, err := buffer.ReadString(',')
	if err != nil {
		code = ErrorInvalidLabel
		logger.Error("%v", err)
		return
	}
	label := strings.TrimRight(seg2, ",")
	if label == "" {
		err = errors.New("empty label")
		code = ErrorInvalidLabel
		logger.Error("%v", err)
		return
	}
	message, err := ioutil.ReadAll(buffer)
	if err != nil {
		code = ErrorInvalidContent
		logger.Error("%v", err)
		return
	} else if len(message) == 0 {
		err = errors.New("empty message")
		code = ErrorInvalidContent
		logger.Error("%v", err)
		return
	}
	go func() {
		if pool, err := connection.FindPool(label); err == nil {
			b := &bytes.Buffer{}
			messageType := []byte(fmt.Sprintf("%d,%s,", TypeBroadcast, label))
			b.WriteString(fmt.Sprintf("%d,", len(messageType)+len(message)))
			b.Write(messageType)
			b.Write(message)
			for current := pool.Entry(); current != nil; current = current.Next() {
				c := current.Load().(net.Conn)
				n, err := connection.Write(c, h.ioLimitedSec, b.Bytes())
				if err != nil {
					logger.Error("broadcast io writing error: %v %v %v", label, n, err)
					connection.Close(c)
				}
			}
		}
	}()
	return
}

func (h *DefaultSrvHandler) respFailure(c net.Conn, errcode int) {
	b := &bytes.Buffer{}
	content := []byte(fmt.Sprintf("%d,%d,%d", TypeResponse, RespFailure, errcode))
	b.WriteString(fmt.Sprintf("%d,", len(content)))
	b.Write(content)
	n, err := connection.Write(c, h.ioLimitedSec, b.Bytes())
	if err != nil {
		logger.Error("respFailure io writing error: %v %v", n, err)
		connection.Close(c)
	}
}

func (h *DefaultSrvHandler) respSuccess(c net.Conn) {
	b := &bytes.Buffer{}
	content := []byte(fmt.Sprintf("%d,%d", TypeResponse, RespSuccess))
	b.WriteString(fmt.Sprintf("%d,", len(content)))
	b.Write(content)
	n, err := connection.Write(c, h.ioLimitedSec, b.Bytes())
	if err != nil {
		logger.Error("respSuccess io writing error: %v %v", n, err)
		connection.Close(c)
	}
}

// +------------------+
// +  client handler  +
// +------------------+

// DefaultCliHandler default client handler
type DefaultCliHandler struct {
	heartbeatInt time.Duration
	ioLimitedSec time.Duration
	conn         net.Conn
	taskIn       chan<- []byte
	taskOut      <-chan []byte
	respIn       chan<- []byte
	respOut      <-chan []byte
	requests     chan chan []byte
}

// NewDefaultCliHandler create a new client handler
// ioLimitedSec: limited seconds for waiting server response or write calls
func NewDefaultCliHandler(heartbeatInt time.Duration, ioLimitedSec time.Duration) *DefaultCliHandler {
	return &DefaultCliHandler{
		heartbeatInt: heartbeatInt,
		ioLimitedSec: ioLimitedSec,
	}
}

// Bind bind a connection and do some initial stuff
func (h *DefaultCliHandler) Bind(c net.Conn) error {
	h.conn = c
	h.taskIn, h.taskOut = h.nonBlockingQueue()
	h.respIn, h.respOut = h.nonBlockingQueue()
	h.requests = make(chan chan []byte)

	go h.recvLoop()
	go h.sendLoop()

	return nil
}

// Close close the handler and binding connection
func (h *DefaultCliHandler) Close() {
	connection.Close(h.conn)
}

func (h *DefaultCliHandler) nonBlockingQueue() (input chan<- []byte, output <-chan []byte) {
	type queue struct {
		l *list.List
		m sync.Mutex
		b chan struct{}
	}
	q := &queue{
		l: list.New(),
		b: make(chan struct{}, 1),
	}
	in := make(chan []byte)
	out := make(chan []byte)
	go func() {
		for {
			message, ok := <-in
			if !ok {
				break
			}
			q.m.Lock()
			q.l.PushBack(message)
			q.m.Unlock()
			select {
			case q.b <- struct{}{}:
			default:
			}
		}
	}()
	go func() {
		for {
			q.m.Lock()
			if front := q.l.Front(); front != nil {
				message := q.l.Remove(front).([]byte)
				q.m.Unlock()
				out <- message
			} else {
				q.m.Unlock()
				<-q.b
			}
		}
	}()
	return in, out
}

// Handle handle a connection
func (h *DefaultCliHandler) sendLoop() {
	defer h.Close()
	for {
		select {
		case exchange := <-h.requests:
			if h.sendMessage(<-exchange) != nil {
				close(exchange)
				return
			}
			select {
			case resp := <-h.respOut:
				exchange <- resp
			case <-time.After(h.ioLimitedSec):
				close(exchange)
				return
			}
		case <-time.After(h.heartbeatInt):
			if h.sendHeartbeat() != nil {
				return
			}
		}
	}
}

func (h *DefaultCliHandler) sendMessage(message []byte) (err error) {
	b := &bytes.Buffer{}
	b.WriteString(fmt.Sprintf("%d,", len(message)))
	b.Write(message)
	n, err := connection.Write(h.conn, h.ioLimitedSec, b.Bytes())
	if err != nil {
		logger.Error("sendMessage io writing error: %v %v", n, err)
	}
	return
}

func (h *DefaultCliHandler) sendHeartbeat() (err error) {
	n, err := connection.Write(h.conn, h.ioLimitedSec, []byte("0,"))
	if err != nil {
		logger.Error("sendHeartbeat io writing error: %v %v", n, err)
	}
	return
}

func (h *DefaultCliHandler) recvLoop() {
	defer h.Close()
	reader := bufio.NewReader(h.conn)
	for {
		err := h.conn.SetReadDeadline(time.Time{})
		if err != nil {
			logger.Error("set read deadline error: %v", err)
			return
		}
		sizestr, err := reader.ReadString(',')
		if err != nil {
			logger.Error("get message size error: %v", err)
			return
		}
		size, err := strconv.Atoi(strings.TrimRight(sizestr, ","))
		if err != nil {
			logger.Error("illegal size: %v", err)
			return
		} else if size == 0 {
			continue
		}
		// TODO(): size should have a limit value
		message := make([]byte, size)
		_, err = io.ReadFull(reader, message)
		if err != nil {
			logger.Error("unexpected size packet: %v", err)
			return
		}
		h.parse(message)
	}
}

func (h *DefaultCliHandler) parse(message []byte) {
	buffer := bytes.NewBuffer(message)
	seg1, err := buffer.ReadString(',')
	if err != nil {
		logger.Error("get message type error: %v", err)
		return
	}
	t, err := strconv.Atoi(strings.TrimRight(seg1, ","))
	if err != nil {
		logger.Error("illegal message type: %v", err)
		return
	}

	switch t {
	case TypeResponse:
		err := h.respEnqueue(buffer)
		if err != nil {
			return
		}
	case TypeBroadcast:
		err := h.taskEnqueue(buffer)
		if err != nil {
			return
		}
	}
}

func (h *DefaultCliHandler) respEnqueue(buffer *bytes.Buffer) (err error) {
	l, err := ioutil.ReadAll(buffer)
	if err != nil {
		logger.Error("%v", err)
		return
	}
	h.respIn <- l
	return
}

func (h *DefaultCliHandler) taskEnqueue(buffer *bytes.Buffer) (err error) {
	l, err := ioutil.ReadAll(buffer)
	if err != nil {
		logger.Error("%v", err)
		return
	}
	h.taskIn <- l
	return
}

// ConsumeTasks task consumption queue
func (h *DefaultCliHandler) ConsumeTasks() (label string, message []byte, err error) {
	task, ok := <-h.taskOut
	if !ok {
		logger.Error("task out has been closed")
		return
	}
	buffer := bytes.NewBuffer(task)
	seg1, err := buffer.ReadString(',')
	if err != nil {
		logger.Error("get label error: %v", err)
		return
	}
	message, err = ioutil.ReadAll(buffer)
	if err != nil {
		logger.Error("get message error: %v", err)
		return
	}
	label = strings.TrimRight(seg1, ",")
	return
}

// Label label connection on the service side
func (h *DefaultCliHandler) Label(label string) error {
	content := fmt.Sprintf("%d,%s", TypeLabel, label)

	exchange := make(chan []byte)
	select {
	case h.requests <- exchange:
	case <-time.After(h.ioLimitedSec):
		return fmt.Errorf("%d", RespTimeout)
	}
	exchange <- []byte(content)
	response, ok := <-exchange
	if !ok {
		return fmt.Errorf("%d", RespTimeout)
	}
	close(exchange)

	return h.parseResponse(response)
}

// Dislabel dislabel connection on the service side
func (h *DefaultCliHandler) Dislabel(label string) error {
	content := fmt.Sprintf("%d,%s", TypeDislabel, label)

	exchange := make(chan []byte)
	select {
	case h.requests <- exchange:
	case <-time.After(h.ioLimitedSec):
		return fmt.Errorf("%d", RespTimeout)
	}
	exchange <- []byte(content)
	response, ok := <-exchange
	if !ok {
		return fmt.Errorf("%d", RespTimeout)
	}
	close(exchange)

	return h.parseResponse(response)
}

// Broadcast broadcast a message to the connections
// that on the server side of the corresponding label
func (h *DefaultCliHandler) Broadcast(label string, message []byte) error {
	content := fmt.Sprintf("%d,%s,%s", TypeBroadcast, label, message)

	exchange := make(chan []byte)
	select {
	case h.requests <- exchange:
	case <-time.After(h.ioLimitedSec):
		return fmt.Errorf("%d", RespTimeout)
	}
	exchange <- []byte(content)
	response, ok := <-exchange
	if !ok {
		return fmt.Errorf("%d", RespTimeout)
	}
	close(exchange)

	return h.parseResponse(response)
}

func (h *DefaultCliHandler) parseResponse(message []byte) (err error) {
	s := strings.SplitN(string(message), ",", 2)
	t, err := strconv.Atoi(s[0])
	if err != nil {
		logger.Error("illegal result: %v", err)
		return
	}
	switch t {
	case RespSuccess:
		return
	case RespFailure:
		if len(s) < 2 {
			err = fmt.Errorf("%d", ErrorUnknown)
			return
		}
		err = errors.New(s[1])
		return
	}
	return errors.New("unknown result")
}
