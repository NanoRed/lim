package protocol

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

var _framePool = &sync.Pool{New: func() any {
	return &Frame{}
}}

type Frame struct {
	Act     Action
	Label   string
	Payload []byte
}

func NewFrame() *Frame {
	return _framePool.Get().(*Frame)
}

func (f *Frame) Recycle() {
	f.Act = 0
	f.Label = ""
	f.Payload = nil
	_framePool.Put(f)
}

type FrameProcessor struct {
	*FrameEncoder
	*FrameDecoder
}

func NewFrameProcessor(rw io.ReadWriter) *FrameProcessor {
	return &FrameProcessor{NewFrameEncoder(rw), NewFrameDecoder(rw)}
}

func (p *FrameProcessor) SetDecodeTimeout(t time.Duration) error {
	if conn, ok := p.w.(net.Conn); ok {
		if t > 0 {
			return conn.SetReadDeadline(time.Now().Add(t))
		} else {
			return conn.SetReadDeadline(time.Time{})
		}
	}
	return nil
}

type FrameEncoder struct {
	w io.Writer
}

func NewFrameEncoder(w io.Writer) *FrameEncoder {
	return &FrameEncoder{w}
}

func (e *FrameEncoder) Encode(frame *Frame) (err error) {
	data := []byte{0}
	if dlen := len(frame.Payload); dlen > 0 {
		if dlen > 4095 {
			return errors.New("payload is more than 4095 bytes")
		}
		data = make([]byte, dlen+2)
		binary.BigEndian.PutUint16(data, uint16(dlen))
		data[0] |= 0x20
		copy(data[2:], frame.Payload)
	}
	data[0] |= byte(frame.Act) << 6
	if llen := len(frame.Label); llen > 0 {
		if llen > 255 {
			return errors.New("label is more than 255 bytes")
		}
		data = append(data, byte(llen))
		data = append(data, frame.Label...)
		data[0] |= 0x10
	}
	_, err = e.w.Write(data)
	return
}

func (e *FrameEncoder) Close() (err error) {
	if closer, ok := e.w.(io.Closer); ok {
		err = closer.Close()
	}
	return
}

type FrameDecoder struct {
	r io.Reader
}

func NewFrameDecoder(r io.Reader) *FrameDecoder {
	return &FrameDecoder{r}
}

func (d *FrameDecoder) Decode(frame *Frame) (raw []byte, err error) {
	raw = make([]byte, 1, 2)
	if _, err = io.ReadFull(d.r, raw); err != nil {
		return
	}
	frame.Act = Action(raw[0] >> 6)
	if raw[0]&0x20 > 0 {
		raw = append(raw, 0)
		if _, err = io.ReadFull(d.r, raw[1:]); err != nil {
			return
		}
		size := uint16(raw[1]) | uint16(raw[0]&0x0f)<<8
		tmp := make([]byte, size+2)
		tmp[0], tmp[1] = raw[0], raw[1]
		if _, err = io.ReadFull(d.r, tmp[2:]); err != nil {
			return
		}
		raw = tmp
		frame.Payload = raw[2:]
	}
	if raw[0]&0x10 > 0 {
		p := len(raw)
		raw = append(raw, 0)
		if _, err = io.ReadFull(d.r, raw[p:]); err != nil {
			return
		}
		label := make([]byte, raw[p])
		if _, err = io.ReadFull(d.r, label); err != nil {
			return
		}
		raw = append(raw, label...)
		frame.Label = string(label)
	}
	return
}
