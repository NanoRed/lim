package internal

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NanoRed/lim/pkg/container"
	"github.com/NanoRed/lim/pkg/logger"
)

type Client struct {
	addr           string
	reqIn          chan<- any
	reqOut         <-chan any
	respSQ         *container.SyncQueue
	mesgSQ         *container.SyncQueue
	pauseValve     *sync.WaitGroup
	pauseTimes     int32
	close          chan struct{}
	frameProcessor FrameProcessor
}

func NewClient(addr string, frameProcessor FrameProcessor) *Client {
	client := &Client{
		addr:           addr,
		respSQ:         container.NewSyncQueue(),
		mesgSQ:         container.NewSyncQueue(),
		pauseValve:     &sync.WaitGroup{},
		pauseTimes:     0,
		close:          make(chan struct{}, 1),
		frameProcessor: frameProcessor,
	}
	client.reqIn, client.reqOut = container.NewSyncQueue().Chan()
	return client
}

func (c *Client) Connect(a ...any) {
	var (
		times int32
		block chan struct{}
		delay time.Duration
	)
	if len(a) > 0 {
		times = a[0].(int32)
		block = a[1].(chan struct{})
		delay = a[2].(time.Duration)
	} else {
		times = atomic.LoadInt32(&c.pauseTimes)
		block = make(chan struct{}, 1)
		block <- struct{}{}
		defer func() { block <- struct{}{}; close(block) }()
		delay = 0
	}
	go func() {
		defer func() {
			time.Sleep(delay)
			if delay = (delay * 2) + 1; delay > time.Minute {
				delay = time.Minute
			}
			c.Connect(times, block, delay)
		}()
		var err error
		conn := &conn{}
		conn.Conn, err = net.Dial("tcp", c.addr)
		if err != nil {
			logger.Error("failed to dial to server: %v", err)
			return
		}
		defer conn.Close()
		delay = 0
		if err = c.handshake(conn); err != nil {
			logger.Error("handshake failed: %v", err)
			return
		}
		if newTimes := atomic.LoadInt32(&c.pauseTimes); times != newTimes {
			c.pauseValve.Done() // restart the queues
			times = newTimes
		}
		<-block
		go c.sendLoop(conn)
		c.recvLoop(conn)
	}()
}

func (c *Client) recvLoop(conn *conn) {
	defer conn.Close()
	for {
		frame, err := c.frameProcessor.Next(conn)
		if err != nil {
			c.frameProcessor.Recycle(frame)
			logger.Error("failed to read next frame: %v", err)
			return
		}
		switch frame.Type() {
		case FTResponse:
			if ccarrier := c.respSQ.Pop().([2]any); conn == ccarrier[0] {
				select {
				case ccarrier[1].(chan any) <- frame:
				default:
					c.frameProcessor.Recycle(frame)
				}
			}
		case FTMulticast:
			c.mesgSQ.Push(frame)
		}
	}
}

func (c *Client) sendLoop(conn *conn) {
	times := c.pauseTimes
	heartbeatFrame := c.frameProcessor.Make(FTResponse, "", []byte{})
	heartbeatBytes := heartbeatFrame.Encode()
	c.frameProcessor.Recycle(heartbeatFrame)
	ticker := time.NewTicker(HeartbeatInterval)
	defer conn.Close()
	defer ticker.Stop()
	for {
		select {
		case <-c.close:
			return
		case v := <-c.reqOut:
			carrier := v.(chan any)
			reqFrame := (<-carrier).(Frame)
			if _, err := conn.writex(reqFrame.Encode()); err != nil {
				c.pause(times)
				select {
				case carrier <- err:
				default:
				}
				c.frameProcessor.Recycle(reqFrame)
				<-c.close
				return
			}
			ccarrier := [2]any{conn, carrier}
			c.respSQ.Push(ccarrier)
			c.frameProcessor.Recycle(reqFrame)
		case <-ticker.C:
			if _, err := conn.writex(heartbeatBytes); err != nil {
				c.pause(times)
				<-c.close
				return
			}
		}
	}
}

func (c *Client) request(frame Frame) (err error) {
	c.pauseValve.Wait()
	times := c.pauseTimes
	carrier := make(chan any)
	c.reqIn <- carrier
	carrier <- frame
	select {
	case v := <-carrier:
		if respFrame, ok := v.(Frame); ok {
			if message := respFrame.Payload(); len(message) > 0 {
				err = errors.New(string(message))
			}
			c.frameProcessor.Recycle(respFrame)
		} else {
			err = v.(error)
		}
		close(carrier)
	case <-time.After(ResponseTimeout):
		c.pause(times)
		err = errors.New("request timed out")
	}
	return
}

func (c *Client) pause(times int32) {
	if atomic.CompareAndSwapInt32(&c.pauseTimes, times, times+1) {
		c.pauseValve.Add(1)
		c.close <- struct{}{}
	}
}

func (c *Client) handshake(conn *conn) (err error) {
	reqframe := c.frameProcessor.Make(FTHandshake, "", []byte("sample_secret")) // TODO
	defer c.frameProcessor.Recycle(reqframe)
	if _, err = conn.writex(reqframe.Encode()); err != nil {
		return
	}
	timer := time.NewTimer(ResponseTimeout)
	defer conn.SetReadDeadline(time.Time{})
	for {
		select {
		case <-timer.C:
			err = errors.New("handshake timed out")
			return
		default:
			err = conn.SetReadDeadline(time.Now().Add(ResponseTimeout))
			if err != nil {
				return
			}
			frame, e := c.frameProcessor.Next(conn)
			if e != nil {
				c.frameProcessor.Recycle(frame)
				return e
			}
			if frame.Type() == FTResponse {
				if message := frame.Payload(); len(message) > 0 {
					err = errors.New(string(message))
				}
				c.frameProcessor.Recycle(frame)
				return
			}
			c.frameProcessor.Recycle(frame)
		}
	}
}

func (c *Client) Label(label string) (err error) {
	frame := c.frameProcessor.Make(FTLabel, label, []byte{})
	return c.request(frame)
}

func (c *Client) Dislabel(label string) (err error) {
	frame := c.frameProcessor.Make(FTLabel, label, []byte{'-'})
	return c.request(frame)
}

func (c *Client) Multicast(label string, data []byte) (err error) {
	frame := c.frameProcessor.Make(FTMulticast, label, data)
	return c.request(frame)
}

func (c *Client) Receive() (label string, data []byte) {
	frame := c.mesgSQ.Pop().(Frame)
	defer c.frameProcessor.Recycle(frame)
	return frame.Label(), frame.Payload()
}
