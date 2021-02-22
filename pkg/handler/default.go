package handler

import (
	"bufio"
	"bytes"
	"container/list"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RedAFD/lim/pkg/connection"
	"github.com/RedAFD/lim/pkg/logger"
)

// message type
const (
	// TypeLabel means puts a label on its connection
	TypeLabel = iota
	// TypeRemoveLabel remove the label from its connection
	TypeRemoveLabel
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
	// ErrorInvalidMessageType invalid message type
	ErrorInvalidMessageType
	// ErrorInvalidLabel invalid label
	ErrorInvalidLabel
	// ErrorInvalidContent invalid content
	ErrorInvalidContent
)

// DefaultHandler default handler
type DefaultHandler struct {
	expireSecond    time.Duration
	responseTimeout time.Duration
}

// NewDefaultHandler create a new default handler
func NewDefaultHandler(expSec time.Duration, respTimeout time.Duration) *DefaultHandler {
	return &DefaultHandler{
		expireSecond:    expSec,
		responseTimeout: respTimeout,
	}
}

// Handle handle connection
func (h *DefaultHandler) Handle(c *connection.Connection) {
	defer c.Close()
	reader := bufio.NewReader(c)
	acquireSequence := h.lineup(c)
	for {
		c.SetReadDeadline(time.Now().Add(h.expireSecond))
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
		go h.parse(acquireSequence(), c, message)
	}
}

func (h *DefaultHandler) lineup(c *connection.Connection) func() <-chan struct{} {
	queue := &struct {
		l *list.List
		m sync.Mutex
		b chan struct{}
	}{
		l: list.New(),
		m: sync.Mutex{},
		b: make(chan struct{}, 1),
	}
	go func() {
		for {
			queue.m.Lock()
			if front := queue.l.Front(); front != nil {
				orderLock := queue.l.Remove(front).(chan struct{})
				queue.m.Unlock()
				select {
				case orderLock <- struct{}{}:
					close(orderLock)
				case <-time.After(h.responseTimeout):
					close(orderLock)
					h.respTimeout(c)
				}
			} else {
				queue.m.Unlock()
				<-queue.b
			}
		}
	}()
	return func() <-chan struct{} {
		orderLock := make(chan struct{})
		queue.m.Lock()
		queue.l.PushBack(c)
		queue.m.Unlock()
		select {
		case queue.b <- struct{}{}:
		default:
		}
		return orderLock
	}
}

func (h *DefaultHandler) parse(orderLock <-chan struct{}, c *connection.Connection, message []byte) {
	buffer := bytes.NewBuffer(message)
	seg1, err := buffer.ReadString(',')
	if err != nil {
		logger.Error("get message type error: %v", err)
		h.respFailure(orderLock, c, ErrorInvalidMessageType)
		return
	}
	t, err := strconv.Atoi(strings.TrimRight(seg1, ","))
	if err != nil {
		logger.Error("illegal message type: %v", err)
		h.respFailure(orderLock, c, ErrorInvalidMessageType)
		return
	}
	switch t {
	case TypeLabel:
		l, err := ioutil.ReadAll(buffer)
		if err != nil {
			logger.Error("get label error: %v", err)
			h.respFailure(orderLock, c, ErrorInvalidLabel)
			return
		} else if len(l) == 0 {
			logger.Error("empty label")
			h.respFailure(orderLock, c, ErrorInvalidLabel)
			return
		}
		connection.Label(c, string(l))
	case TypeRemoveLabel:
		l, err := ioutil.ReadAll(buffer)
		if err != nil {
			logger.Error("get label error: %v", err)
			h.respFailure(orderLock, c, ErrorInvalidLabel)
			return
		} else if len(l) == 0 {
			logger.Error("empty label")
			h.respFailure(orderLock, c, ErrorInvalidLabel)
			return
		}
		connection.RemoveLabel(c, string(l))
	case TypeBroadcast:
		seg2, err := buffer.ReadString(',')
		if err != nil {
			logger.Error("get label error: %v", err)
			h.respFailure(orderLock, c, ErrorInvalidLabel)
			return
		}
		label := strings.TrimRight(seg2, ",")
		if label == "" {
			logger.Error("empty label")
			h.respFailure(orderLock, c, ErrorInvalidLabel)
			return
		}
		content, err := ioutil.ReadAll(buffer)
		if err != nil {
			logger.Error("get content error: %v", err)
			h.respFailure(orderLock, c, ErrorInvalidContent)
			return
		} else if len(content) == 0 {
			logger.Error("empty content")
			h.respFailure(orderLock, c, ErrorInvalidContent)
			return
		}
		go h.broadcast(label, content)
	}
	h.respSuccess(orderLock, c)
}

func (h *DefaultHandler) broadcast(label string, message []byte) {
	connSet, err := connection.FindConnSet(label)
	if err == nil {
		b := &bytes.Buffer{}
		messageType := []byte(fmt.Sprintf("%d,", TypeBroadcast))
		b.WriteString(fmt.Sprintf("%d,", len(messageType)+len(message)))
		b.Write(messageType)
		b.Write(message)
		connSet.RangeDo(func(c *connection.Connection) {
			n, err := c.SafetyWrite(h.expireSecond, b.Bytes())
			if err != nil {
				logger.Error("broadcast io writing error: %v %v %v", label, n, err)
				c.Close()
			}
		})
	}
}

func (h *DefaultHandler) respFailure(orderLock <-chan struct{}, c *connection.Connection, errcode int) {
	b := &bytes.Buffer{}
	content := []byte(fmt.Sprintf("%d,%d,%d", TypeResponse, RespFailure, errcode))
	b.WriteString(fmt.Sprintf("%d,", len(content)))
	b.Write(content)
	if _, ok := <-orderLock; !ok {
		logger.Error("orderLock has been closed")
		return
	}
	n, err := c.SafetyWrite(h.expireSecond, b.Bytes())
	if err != nil {
		logger.Error("io writing error: %v %v", n, err)
		c.Close()
	}
}

func (h *DefaultHandler) respSuccess(orderLock <-chan struct{}, c *connection.Connection) {
	b := &bytes.Buffer{}
	content := []byte(fmt.Sprintf("%d,%d", TypeResponse, RespSuccess))
	b.WriteString(fmt.Sprintf("%d,", len(content)))
	b.Write(content)
	if _, ok := <-orderLock; !ok {
		logger.Error("orderLock has been closed")
		return
	}
	n, err := c.SafetyWrite(h.expireSecond, b.Bytes())
	if err != nil {
		logger.Error("io writing error: %v %v", n, err)
		c.Close()
	}
}

func (h *DefaultHandler) respTimeout(c *connection.Connection) {
	b := &bytes.Buffer{}
	content := []byte(fmt.Sprintf("%d,%d", TypeResponse, RespTimeout))
	b.WriteString(fmt.Sprintf("%d,", len(content)))
	b.Write(content)
	n, err := c.SafetyWrite(h.expireSecond, b.Bytes())
	if err != nil {
		logger.Error("io writing error: %v %v", n, err)
		c.Close()
	}
}
