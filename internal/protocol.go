package internal

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"time"
)

var (
	ConnReadDuration  time.Duration = time.Second * 10
	ConnWriteTimeout  time.Duration = time.Second * 3
	ResponseTimeout   time.Duration = time.Second * 3
	HeartbeatInterval time.Duration = time.Second * 3
)

type FrameType uint8

const (
	// warning: don't change the order
	FTResponse FrameType = iota
	FTHandshake
	FTLabel
	FTMulticast
)

type Frame interface {
	Raw() []byte
	Type() FrameType
	Label() string
	Payload() []byte
	Encode() []byte
}

type FrameProcessor interface {
	Make(ftype FrameType, label string, payload []byte) Frame
	Next(io.Reader) (Frame, error)
	Recycle(Frame)
}

type DefaultFrame struct {
	raw     *bytes.Buffer
	ftype   FrameType
	label   string
	payload *bytes.Buffer
}

func NewDefaultFrame() Frame {
	return &DefaultFrame{
		raw:     &bytes.Buffer{},
		payload: &bytes.Buffer{},
	}
}

func (f *DefaultFrame) Raw() []byte {
	rawBytes := f.raw.Bytes()
	rtBytes := make([]byte, len(rawBytes))
	copy(rtBytes, rawBytes)
	return rtBytes
}

func (f *DefaultFrame) Type() FrameType {
	return f.ftype
}

func (f *DefaultFrame) Label() string {
	return f.label
}

func (f *DefaultFrame) Encode() []byte {
	header := byte(f.ftype) << 6
	dataLen := f.payload.Len()
	if f.ftype&0xfe == 0x02 { // FTLabel + FTMulticast
		dataLen += len(f.label) + 1
	}
	f.raw.Reset()
	f.raw.WriteByte(0) // get room for header
	if dataLen > 0x1f {
		header |= 1 << 5
		for ; dataLen > 0; dataLen = dataLen << 8 {
			f.raw.WriteByte(byte(dataLen & 0xff))
			header++
		}
	} else {
		header += byte(dataLen)
	}
	f.raw.Bytes()[0] = header
	if f.ftype&0xfe == 0x02 { // FTLabel + FTMulticast
		f.raw.WriteString(f.label)
		f.raw.WriteByte(',')
	}
	if header&0x1f > 0 {
		f.raw.Write(f.payload.Bytes())
	}
	return f.Raw()
}

func (f *DefaultFrame) Payload() []byte {
	plBytes := f.payload.Bytes()
	rtBytes := make([]byte, len(plBytes))
	copy(rtBytes, plBytes)
	return rtBytes
}

type DefaultFrameProcessor struct {
	gcpool *sync.Pool
}

func NewDefaultFrameProcessor() FrameProcessor {
	return &DefaultFrameProcessor{
		&sync.Pool{New: func() any {
			return NewDefaultFrame()
		}},
	}
}

func (r *DefaultFrameProcessor) Make(ftype FrameType, label string, payload []byte) Frame {
	frame := r.gcpool.Get().(*DefaultFrame)
	frame.ftype = ftype
	frame.label = label
	frame.payload.Write(payload)
	return frame
}

func (r *DefaultFrameProcessor) Next(reader io.Reader) (Frame, error) {
	frame := r.gcpool.Get().(*DefaultFrame)
	if _, err := io.CopyN(frame.raw, reader, 1); err != nil {
		return frame, errors.Join(err, errors.New("failed to read header"))
	}
	header, _ := frame.raw.ReadByte()
	frame.raw.UnreadByte()
	frame.ftype = FrameType(header >> 6)
	var size int64
	if header&0x20 > 0 {
		if _, err := io.CopyN(frame.payload, reader, int64(header&0x1f)); err != nil {
			return frame, errors.Join(err, errors.New("failed to read size bytes length"))
		}
		max := frame.payload.Len()
		for i := 0; i < max; i++ {
			seg, _ := frame.payload.ReadByte()
			frame.raw.WriteByte(seg)
			size |= int64(seg) << (i * 8)
		}
	} else if size = int64(header & 0x1f); size == 0 {
		return frame, nil
	}
	if _, err := io.CopyN(frame.payload, reader, size); err != nil {
		return frame, errors.Join(err, errors.New("failed to read payload"))
	}
	frame.raw.Write(frame.payload.Bytes())
	if frame.ftype&0xfe == 0x02 { // FTLabel + FTMulticast
		if labelb, err := frame.payload.ReadBytes(','); err != nil {
			return frame, errors.Join(err, errors.New("failed to parse label"))
		} else {
			frame.label = string(labelb[:len(labelb)-1])
		}
	}
	return frame, nil
}

func (r *DefaultFrameProcessor) Recycle(frame Frame) {
	f := frame.(*DefaultFrame)
	f.raw.Reset()
	f.ftype = 0
	f.label = ""
	f.payload.Reset()
	r.gcpool.Put(f)
}
