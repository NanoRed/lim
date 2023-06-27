package internal

import (
	"bytes"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NanoRed/lim/internal/protocol"
	"github.com/NanoRed/lim/pkg/container"
	"github.com/NanoRed/lim/pkg/logger"
)

const (
	terminate int32 = iota
	preparing
	working
)

type Client struct {
	state      int32
	reqIn      chan any
	reqOut     chan any
	arrive     *container.SyncQueue
	dialer     func() (net.Conn, error)
	pauseValve *sync.WaitGroup
	pauseTimes uint32
	close      chan struct{}
	labels     *sync.Map
	context    []any
	packer     *protocol.Packer
}

func NewClient(dialer func() (net.Conn, error)) *Client {
	sq := container.NewSyncQueue()
	sq2 := container.NewSyncQueue()
	client := &Client{
		state:      terminate,
		reqIn:      make(chan any),
		reqOut:     make(chan any),
		arrive:     sq,
		dialer:     dialer,
		pauseValve: &sync.WaitGroup{},
		pauseTimes: 0,
		close:      make(chan struct{}, 1),
		labels:     &sync.Map{},
		context:    nil,
		packer: protocol.NewPacker(func() *protocol.Frame {
			return sq.Pop().(*protocol.Frame)
		}),
	}
	sq2.Install(client.reqIn, client.reqOut)
	return client
}

func (c *Client) Connect() error {
	if !atomic.CompareAndSwapInt32(&c.state, terminate, preparing) {
		return errors.New("the client has started connecting")
	}
	var (
		times uint32
		block chan struct{}
		delay time.Duration
	)
	if c.context == nil {
		times = atomic.LoadUint32(&c.pauseTimes)
		block = make(chan struct{}, 1)
		block <- struct{}{}
		defer func() { block <- struct{}{}; close(block) }()
		delay = 0
	} else {
		times = c.context[0].(uint32)
		block = c.context[1].(chan struct{})
		delay = (c.context[2].(time.Duration) * 2) + 1
		if delay > 60 {
			delay = 60
		}
	}
	go func() {
		defer func() {
			c.context = []any{times, block, delay}
			atomic.StoreInt32(&c.state, terminate)
			logger.Warn("reconnect in %d seconds...", delay)
			time.Sleep(delay * time.Second)
			if err := c.Connect(); err != nil {
				logger.Error("reconnect failed: %v", err)
			}
		}()
		var err error
		conn := &conn{}
		conn.Conn, err = c.dialer()
		if err != nil {
			logger.Error("failed to dial to server: %v", err)
			return
		}
		defer conn.Close()
		processor := protocol.NewFrameProcessor(conn)
		if err = c.handshake(processor); err != nil {
			logger.Error("handshake failed: %v", err)
			return
		}
		delay = 0
		if err = c.relabel(processor); err != nil {
			logger.Error("failed to relabel: %v", err)
			return
		}
		if newTimes := atomic.LoadUint32(&c.pauseTimes); times != newTimes {
			c.pauseValve.Done() // restart the queues
			times = newTimes
		}
		<-block
		respSQ := container.NewSyncQueue()
		go c.sendLoop(processor.FrameEncoder, respSQ)
		atomic.StoreInt32(&c.state, working)
		c.recvLoop(processor.FrameDecoder, respSQ)
	}()
	return nil
}

func (c *Client) recvLoop(decoder *protocol.FrameDecoder, respSQ *container.SyncQueue) {
	for {
		frame := protocol.NewFrame()
		if _, err := decoder.Decode(frame); err != nil {
			logger.Error("failed to decode next frame: %v", err)
			return
		}
		switch frame.Act {
		case protocol.ActResponse:
			select {
			case respSQ.Pop().(chan any) <- frame:
			default:
			}
		case protocol.ActMulticast:
			c.arrive.Push(frame)
		}
	}
}

func (c *Client) sendLoop(encoder *protocol.FrameEncoder, respSQ *container.SyncQueue) {
	times := c.pauseTimes
	ticker := time.NewTicker(HeartbeatInterval)
	heartbeatFrame := protocol.NewFrame()
	heartbeatFrame.Act = protocol.ActResponse
	defer ticker.Stop()
	defer encoder.Close()
	for {
		select {
		case <-c.close:
			return
		case v := <-c.reqOut:
			switch val := v.(type) {
			case *protocol.Frame:
				if err := encoder.Encode(val); err != nil {
					c.pause(times)
					val.Recycle()
					logger.Error("failed to write data: %v", err)
					<-c.close
					return
				}
				val.Recycle()
			case chan any:
				reqFrame := (<-val).(*protocol.Frame)
				if err := encoder.Encode(reqFrame); err != nil {
					c.pause(times)
					select {
					case val <- err:
					default:
					}
					reqFrame.Recycle()
					logger.Error("failed to write data: %v", err)
					<-c.close
					return
				}
				respSQ.Push(val)
				reqFrame.Recycle()
			}
		case <-ticker.C:
			if err := encoder.Encode(heartbeatFrame); err != nil {
				c.pause(times)
				logger.Error("failed to write data: %v", err)
				<-c.close
				return
			}
		}
	}
}

func (c *Client) requestUnfriendly(processor *protocol.FrameProcessor, frame *protocol.Frame) (err error) {
	if err = processor.Encode(frame); err != nil {
		return
	}
	timer := time.NewTimer(ResponseTimeout)
	defer processor.SetDecodeTimeout(0)
	for {
		select {
		case <-timer.C:
			err = errors.New("request timed out")
			return
		default:
			if err = processor.SetDecodeTimeout(ResponseTimeout); err != nil {
				return
			}
			frame.Payload = nil
			if _, err = processor.Decode(frame); err != nil {
				return
			}
			if frame.Act == protocol.ActResponse {
				if len(frame.Payload) > 0 {
					err = errors.New(string(frame.Payload))
				}
				return
			}
		}
	}
}

func (c *Client) handshake(processor *protocol.FrameProcessor) (err error) {
	frame := protocol.NewFrame()
	frame.Act = protocol.ActHandshake
	frame.Payload = []byte{'s', 'a', 'm', 'p', 'l', 'e', '_', 's', 'e', 'c', 'r', 'e', 't'}
	err = c.requestUnfriendly(processor, frame)
	frame.Recycle()
	return
}

func (c *Client) relabel(processor *protocol.FrameProcessor) (err error) {
	buf := &bytes.Buffer{}
	labels := make([]string, 0, 1)
	c.labels.Range(func(key, value any) bool {
		label := key.(string)
		if bl := buf.Len(); bl+len(label) > 255 {
			labels = append(labels, buf.String()[:bl-1])
			buf.Reset()
		}
		buf.WriteString(label)
		buf.WriteByte('|')
		return true
	})
	if bl := buf.Len(); bl > 0 {
		labels = append(labels, buf.String()[:bl-1])
		frame := protocol.NewFrame()
		frame.Act = protocol.ActLabel
		frame.Payload = []byte{'*'}
		for _, frame.Label = range labels {
			err = c.requestUnfriendly(processor, frame)
			if err != nil {
				break
			}
		}
		frame.Recycle()
	}
	return
}

func (c *Client) request(frame *protocol.Frame, waitResp bool) (err error) {
	c.pauseValve.Wait()
	if waitResp {
		times := c.pauseTimes
		carrier := make(chan any)
		c.reqIn <- carrier
		carrier <- frame
		select {
		case v := <-carrier:
			switch val := v.(type) {
			case *protocol.Frame:
				if len(val.Payload) > 0 {
					err = errors.New(string(val.Payload))
				}
				val.Recycle()
			case error:
				err = val
			}
			close(carrier)
		case <-time.After(ResponseTimeout):
			c.pause(times)
			err = errors.New("request timed out")
		}
	} else {
		c.reqIn <- frame
	}
	return
}

func (c *Client) pause(times uint32) {
	if atomic.CompareAndSwapUint32(&c.pauseTimes, times, times+1) {
		c.pauseValve.Add(1)
		c.close <- struct{}{}
	}
}

func (c *Client) Label(label string) (err error) {
	if len(label) == 0 {
		return errors.New("invalid label")
	}
	c.labels.Store(label, nil)
	frame := protocol.NewFrame()
	frame.Act = protocol.ActLabel
	frame.Label = label
	frame.Payload = []byte{'+'}
	err = c.request(frame, true)
	return
}

func (c *Client) Dislabel(label string) (err error) {
	if len(label) == 0 {
		return errors.New("invalid label")
	}
	c.labels.Delete(label)
	frame := protocol.NewFrame()
	frame.Act = protocol.ActLabel
	frame.Label = label
	frame.Payload = []byte{'-'}
	err = c.request(frame, true)
	return
}

func (c *Client) Multicast(label string, data []byte) (err error) {
	if len(data) == 0 {
		return errors.New("invalid data")
	} else if len(label) == 0 {
		return errors.New("invalid label")
	}
	for _, frame := range c.packer.Pack(label, data) {
		c.request(frame, false)
	}
	return
}

func (c *Client) Receive() (label string, data [][]byte) {
	return c.packer.Assemble()
}
