package lim

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"
)

type handler interface {
	handle(*connection)
}

const (
	// LABEL means puts a label on its connection
	LABEL = iota
	// RMLABEL remove the label from its connection
	RMLABEL
	// BROADCAST means this is a broadcast job message
	BROADCAST
)

type defaultHandler struct {
	expireSecond time.Duration
}

func newDefaultHandler(expSec time.Duration) *defaultHandler {
	return &defaultHandler{expireSecond: expSec}
}

func (h *defaultHandler) handle(c *connection) {
	reader := bufio.NewReader(c)
	activate, stop := h.watch(c)
	defer stop()
	for {
		sizestr, err := reader.ReadString(',')
		if err != nil {
			log.Printf("[Error]get message size error: %v\n", err)
			return
		}
		activate()
		size, err := strconv.Atoi(strings.TrimRight(sizestr, ","))
		if err != nil {
			log.Printf("[Error]illegal size: %v\n", err)
			return
		} else if size == 0 {
			continue
		}
		// TODO(): size should have a limit value
		message := make([]byte, size)
		_, err = io.ReadFull(reader, message)
		if err != nil {
			log.Printf("[Error]unexpected size packet: %v\n", err)
			return
		}
		go h.parse(c, message)
	}
}

func (h *defaultHandler) watch(c *connection) (activate func(), stop func()) {
	timer := time.NewTimer(h.expireSecond)
	go func() {
		<-timer.C
		c.Close()
	}()
	activate = func() {
		if timer.Stop() {
			timer.Reset(h.expireSecond)
		}
	}
	stop = func() {
		if timer.Stop() {
			timer.Reset(0)
		}
	}
	return
}

func (h *defaultHandler) parse(c *connection, message []byte) {
	buffer := bytes.NewBuffer(message)
	seg1, err := buffer.ReadString(',')
	if err != nil {
		log.Printf("[Error]get message type error: %v\n", err)
		return
	}
	t, err := strconv.Atoi(strings.TrimRight(seg1, ","))
	if err != nil {
		log.Printf("[Error]illegal message type: %v\n", err)
		return
	}
	switch t {
	case LABEL:
		l, err := ioutil.ReadAll(buffer)
		if err != nil {
			log.Printf("[Error]LABEL, get label error: %v\n", err)
			return
		} else if len(l) == 0 {
			log.Println("[Error]LABEL, empty label")
			return
		}
		label(c, string(l))
	case RMLABEL:
		l, err := ioutil.ReadAll(buffer)
		if err != nil {
			log.Printf("[Error]RMLABEL, get label error: %v\n", err)
			return
		} else if len(l) == 0 {
			log.Println("[Error]RMLABEL, empty label")
			return
		}
		rmlabel(c, string(l))
	case BROADCAST:
		seg2, err := buffer.ReadString(',')
		if err != nil {
			log.Printf("[Error]BROADCAST, get label type: %v\n", err)
			return
		}
		label := strings.TrimRight(seg2, ",")
		if label == "" {
			log.Println("[Error]BROADCAST, empty label")
			return
		}
		content, err := ioutil.ReadAll(buffer)
		if err != nil {
			log.Printf("[Error]BROADCAST, get content error: %v\n", err)
			return
		} else if len(content) == 0 {
			log.Println("[Error]BROADCAST, empty content")
			return
		}
		broadcast(label, content)
	}
}
