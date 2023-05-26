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

const (
	terminate int32 = iota
	preparing
	working
)

type Client struct {
	state          int32
	reqIn          chan<- any
	reqOut         <-chan any
	respSQs        *sync.Map
	mesgSQ         *container.SyncQueue
	dialer         func() (net.Conn, error)
	pauseValve     *sync.WaitGroup
	pauseTimes     uint32
	close          chan struct{}
	frameProcessor FrameProcessor
	labels         *sync.Map
	context        []any
}

func NewClient(dialer func() (net.Conn, error), frameProcessor FrameProcessor) *Client {
	client := &Client{
		state:          terminate,
		respSQs:        &sync.Map{},
		mesgSQ:         container.NewSyncQueue(),
		dialer:         dialer,
		pauseValve:     &sync.WaitGroup{},
		pauseTimes:     0,
		close:          make(chan struct{}, 1),
		frameProcessor: frameProcessor,
		labels:         &sync.Map{},
		context:        nil,
	}
	client.reqIn, client.reqOut = container.NewSyncQueue().Chan()
	return client
}

func (c *Client) Connect() error {
	if !atomic.CompareAndSwapInt32(&c.state, terminate, preparing) {
		return errors.New("The client has started connecting")
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
		if delay > time.Minute {
			delay = time.Minute
		}
	}
	go func() {
		defer func() {
			c.context = []any{times, block, delay}
			atomic.StoreInt32(&c.state, terminate)
			logger.Warn("reconnect in %d seconds...", delay)
			time.Sleep(delay)
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
		delay = 0
		if err = c.handshake(conn); err != nil {
			logger.Error("handshake failed: %v", err)
			return
		}
		if err = c.relabel(conn); err != nil {
			logger.Error("failed to relabel: %v", err)
			return
		}
		if newTimes := atomic.LoadUint32(&c.pauseTimes); times != newTimes {
			c.pauseValve.Done() // restart the queues
			times = newTimes
		}
		<-block
		c.respSQs.Store(conn, container.NewSyncQueue())
		defer c.respSQs.Delete(conn)
		go c.sendLoop(conn)
		atomic.StoreInt32(&c.state, working)
		c.recvLoop(conn)
	}()
	return nil
}

func (c *Client) recvLoop(conn *conn) {
	q, _ := c.respSQs.Load(conn)
	respSQ := q.(*container.SyncQueue)
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
			select {
			case respSQ.Pop().(chan any) <- frame:
			default:
				c.frameProcessor.Recycle(frame)
			}
		case FTMulticast:
			c.mesgSQ.Push(frame)
		}
	}
}

func (c *Client) sendLoop(conn *conn) {
	times := c.pauseTimes
	q, _ := c.respSQs.Load(conn)
	respSQ := q.(*container.SyncQueue)
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
			respSQ.Push(carrier)
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

func (c *Client) requestRoughly(conn *conn, frame Frame) (err error) {
	if _, err = conn.writex(frame.Encode()); err != nil {
		return
	}
	timer := time.NewTimer(ResponseTimeout)
	defer conn.SetReadDeadline(time.Time{})
	for {
		select {
		case <-timer.C:
			err = errors.New("request timed out")
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

func (c *Client) handshake(conn *conn) (err error) {
	frame := c.frameProcessor.Make(FTHandshake, "", []byte("sample_secret")) // TODO
	defer c.frameProcessor.Recycle(frame)
	return c.requestRoughly(conn, frame)
}

func (c *Client) relabel(conn *conn) (err error) {
	c.labels.Range(func(key, value any) bool {
		frame := c.frameProcessor.Make(FTLabel, key.(string), []byte{})
		defer c.frameProcessor.Recycle(frame)
		if err = c.requestRoughly(conn, frame); err != nil {
			return false
		}
		return true
	})
	return
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
	frame := c.frameProcessor.Make(FTLabel, label, []byte{})
	return c.request(frame)
}

func (c *Client) Dislabel(label string) (err error) {
	if len(label) == 0 {
		return errors.New("invalid label")
	}
	c.labels.Delete(label)
	frame := c.frameProcessor.Make(FTLabel, label, []byte{'-'})
	return c.request(frame)
}

func (c *Client) Multicast(label string, data []byte) (err error) {
	if len(label) == 0 {
		return errors.New("invalid label")
	} else if len(data) == 0 {
		return errors.New("invalid data")
	}
	frame := c.frameProcessor.Make(FTMulticast, label, data)
	return c.request(frame)
}

func (c *Client) Receive() (label string, data []byte) {
	frame := c.mesgSQ.Pop().(Frame)
	defer c.frameProcessor.Recycle(frame)
	return frame.Label(), frame.Payload()
}
