package protocol

import (
	"encoding/binary"
	"time"
	"unsafe"
)

type buffer struct {
	buf map[uint16][]byte
	len uint16
	max uint16
	ts  uint64
}

type stream struct {
	buf    map[uint16]*buffer
	len    uint16
	cursor *uint16
}

type Packer struct {
	blockBuf  map[[10]byte]*buffer
	streamBuf map[string]*stream
	readFrame func() *Frame
}

func NewPacker(readFrame func() *Frame) *Packer {
	particle := &Packer{
		blockBuf:  make(map[[10]byte]*buffer),
		streamBuf: make(map[string]*stream),
		readFrame: readFrame,
	}
	return particle
}

func (p *Packer) Assemble() (label string, data [][]byte) {
	// 0x06 is it a stream frame
	// 0x04 is it separated as pieces
	// 0x01 0 means it is the last piece, 1 means the other pieces
	for {
		frame := p.readFrame()
		if frame.Payload[0]&0x06 > 0 {
			s, ok := p.streamBuf[frame.Label]
			if !ok {
				s = &stream{buf: make(map[uint16]*buffer)}
				p.streamBuf[frame.Label] = s
			}
			i := binary.BigEndian.Uint16(frame.Payload[1:])
			if frame.Payload[0]&0x04 > 0 {
				j := binary.BigEndian.Uint16(frame.Payload[3:])
				t := binary.BigEndian.Uint64(frame.Payload[5:])
				b, ok := s.buf[i]
				if !ok {
					b = &buffer{
						buf: make(map[uint16][]byte),
						ts:  t,
					}
					s.buf[i] = b
				} else if t > b.ts {
					b.buf = make(map[uint16][]byte)
					b.len = 0
					b.max = 0
					b.ts = t
				}
				b.buf[j] = frame.Payload[13:]
				b.len++
				if frame.Payload[0]&0x01 == 0 {
					b.max = j + 1
				}
				if b.len == b.max {
					if s.cursor == nil {
						delete(s.buf, i)
						payload := make([]byte, 0, 1024)
						for k := uint16(0); k < b.max; k++ {
							payload = append(payload, b.buf[k]...)
						}
						label = frame.Label
						data = append(data, payload)
						i++
						s.cursor = &i
						frame.Recycle()
						return
					}
					s.len++
					goto NEXT
				}
			} else if s.cursor == nil {
				label = frame.Label
				data = append(data, frame.Payload[11:])
				i++
				s.cursor = &i
				frame.Recycle()
				return
			} else {
				s.buf[i] = &buffer{
					buf: map[uint16][]byte{0: frame.Payload[11:]},
					len: 1,
					max: 1,
					ts:  binary.BigEndian.Uint64(frame.Payload[3:]),
				}
				s.len++
				goto NEXT
			}
			goto CONTINUE
		NEXT:
			if b, ok := s.buf[*s.cursor]; ok && b.len == b.max {
				delete(s.buf, *s.cursor)
				payload := make([]byte, 0, 1024)
				for k := uint16(0); k < b.max; k++ {
					payload = append(payload, b.buf[k]...)
				}
				data = append(data, payload)
				s.len--
				*s.cursor++
				goto NEXT
			} else if data != nil {
				label = frame.Label
				frame.Recycle()
				return
			} else if s.len > 20 {
				var min *uint16
				for i, b := range s.buf {
					if b.len == b.max {
						if min == nil {
							tmp := i
							min = &tmp
						} else if b.ts < s.buf[*min].ts {
							*min = i
						}
					}
				}
				s.cursor = min
				goto NEXT
			}
		} else if frame.Payload[0]&0x04 > 0 {
			var key [10]byte
			copy(key[:], frame.Payload[1:])
			b, ok := p.blockBuf[key]
			if !ok {
				b = &buffer{
					buf: make(map[uint16][]byte),
					ts:  binary.BigEndian.Uint64(key[:8]),
				}
				p.blockBuf[key] = b
			}
			i := binary.BigEndian.Uint16(frame.Payload[11:])
			b.buf[i] = frame.Payload[13:]
			b.len++
			if frame.Payload[0]&0x01 == 0 {
				b.max = i + 1
			}
			if b.len == b.max {
				delete(p.blockBuf, key)
				payload := make([]byte, 0, 1024)
				for k := uint16(0); k < b.max; k++ {
					payload = append(payload, b.buf[k]...)
				}
				label = frame.Label
				data = append(data, payload)
				frame.Recycle()
				return
			}
			// TODO check ts and remove
		} else {
			label = frame.Label
			data = append(data, frame.Payload[1:])
			frame.Recycle()
			return
		}
	CONTINUE:
		frame.Recycle()
	}
}

func (p *Packer) Pack(label string, data any) <-chan *Frame {
	switch data := data.(type) {
	case []byte:
		if dlen := len(data); dlen > 0 {
			if dlen < 4095 {
				frame := NewFrame()
				frame.Act = ActMulticast
				frame.Label = label
				frame.Payload = make([]byte, 1+dlen)
				copy(frame.Payload[1:], data)
				frames := make(chan *Frame, 1)
				frames <- frame
				close(frames)
				return frames
			} else {
				var i uint16
				var s uint64
				now := uint64(time.Now().UnixNano())
				rand := uint16(uintptr(unsafe.Pointer(&data)))
				frames := make(chan *Frame, dlen/4082+1)
				for ; dlen > 4082; dlen = dlen - 4082 {
					e := s + 4082
					frame := NewFrame()
					frame.Act = ActMulticast
					frame.Label = label
					frame.Payload = make([]byte, 4095)
					frame.Payload[0] = 0x03
					binary.BigEndian.PutUint64(frame.Payload[1:], now)
					binary.BigEndian.PutUint16(frame.Payload[9:], rand)
					binary.BigEndian.PutUint16(frame.Payload[11:], i)
					copy(frame.Payload[13:], data[s:e])
					frames <- frame
					s = e
					i++
				}
				frame := NewFrame()
				frame.Act = ActMulticast
				frame.Label = label
				frame.Payload = make([]byte, 13+dlen)
				frame.Payload[0] = 0x02
				binary.BigEndian.PutUint64(frame.Payload[1:], now)
				binary.BigEndian.PutUint16(frame.Payload[9:], rand)
				binary.BigEndian.PutUint16(frame.Payload[11:], i)
				copy(frame.Payload[13:], data[s:])
				frames <- frame
				close(frames)
				return frames
			}
		}
	case chan []byte:
		frames := make(chan *Frame)
		go func() {
			var i uint16
			for b := range data {
				dlen := len(b)
				if dlen == 0 {
					continue
				}
				now := uint64(time.Now().UnixNano())
				if dlen <= 4084 {
					frame := NewFrame()
					frame.Act = ActMulticast
					frame.Label = label
					frame.Payload = make([]byte, 11+dlen)
					frame.Payload[0] = 0x04
					binary.BigEndian.PutUint16(frame.Payload[1:], i)
					binary.BigEndian.PutUint64(frame.Payload[3:], now)
					copy(frame.Payload[11:], b)
					frames <- frame
				} else {
					var j uint16
					var s uint64
					for ; dlen > 4082; dlen = dlen - 4082 {
						e := s + 4082
						frame := NewFrame()
						frame.Act = ActMulticast
						frame.Label = label
						frame.Payload = make([]byte, 4095)
						frame.Payload[0] = 0x07
						binary.BigEndian.PutUint16(frame.Payload[1:], i)
						binary.BigEndian.PutUint16(frame.Payload[3:], j)
						binary.BigEndian.PutUint64(frame.Payload[5:], now)
						copy(frame.Payload[13:], b[s:e])
						frames <- frame
						s = e
						j++
					}
					frame := NewFrame()
					frame.Act = ActMulticast
					frame.Label = label
					frame.Payload = make([]byte, 13+dlen)
					frame.Payload[0] = 0x06
					binary.BigEndian.PutUint16(frame.Payload[1:], i)
					binary.BigEndian.PutUint16(frame.Payload[3:], j)
					binary.BigEndian.PutUint64(frame.Payload[5:], now)
					copy(frame.Payload[13:], b[s:])
					frames <- frame
				}
				i++
			}
			close(frames)
		}()
		return frames
	}
	frames := make(chan *Frame)
	close(frames)
	return frames
}
