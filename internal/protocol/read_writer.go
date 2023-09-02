package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"unicode/utf16"
)

type PeekableReader interface {
	// PeekByte for detecting the legacy ping packet
	PeekByte() (byte, error)
}

type BufReader interface {
	PeekableReader

	GetReadLen() int
	Read(n int) ([]byte, error)

	ReadUInt8() (uint8, error)   // Unsigned byte
	ReadUInt16() (uint16, error) // Unsigned Short
	ReadInt16() (int16, error)   // Short
	ReadUInt32() (uint32, error) // Unsigned Int
	ReadInt32() (int32, error)   // Int

	ReadVarInt() (int32, error)
	ReadString() (string, error)
	ReadUTF16BE() (string, error)
}

type BufWriter interface {
	GetWriteLen() int
	Write(b []byte) error

	WriteUInt8(value uint8) error   // Unsigned byte
	WriteUInt16(value uint16) error // Unsigned Short
	WriteInt16(value int16) error   // Short
	WriteUInt32(value uint32) error // Unsigned Int
	WriteInt32(value int32) error   // Int

	WriteVarInt(value int32) error
	WriteString(s string) error
	WriteUTF16BE(s string) error
}

type BufReadWriter interface {
	BufReader
	PeekableReader
	BufWriter
}

type bufReadWriterImpl struct {
	rw       io.ReadWriter
	peekByte *byte
	writeLen int
	readLen  int
}

var _ BufReadWriter = &bufReadWriterImpl{}

func NewBufferReadWriter(buf io.ReadWriter) BufReadWriter {
	return &bufReadWriterImpl{
		rw:       buf,
		readLen:  0,
		writeLen: 0,
	}
}

const (
	varIntSegmentBits = 0x7F
	varIntContinueBit = 0x80
)

func (p *bufReadWriterImpl) GetReadLen() int {
	return p.readLen
}

func (p *bufReadWriterImpl) GetWriteLen() int {
	return p.writeLen
}

func (p *bufReadWriterImpl) PeekByte() (byte, error) {
	if p.peekByte != nil {
		return *p.peekByte, nil
	}
	b, err := p.read(1)
	if err != nil {
		return 0, err
	}
	p.peekByte = &b[0]
	return b[0], nil
}

func (p *bufReadWriterImpl) Read(n int) ([]byte, error) {
	peek := p.peekByte
	if p.peekByte != nil {
		n -= 1
		p.peekByte = nil
	}
	b, err := p.read(n)
	if err != nil {
		return nil, err
	}
	if peek != nil {
		p.readLen += n + 1
		b = append([]byte{*peek}, b...)
	} else {
		p.readLen += n
	}
	return b, nil
}

func (p *bufReadWriterImpl) read(n int) ([]byte, error) {
	b := make([]byte, n)
	n, err := io.ReadFull(p.rw, b)
	if err != nil {
		return nil, err
	}
	if n != len(b) {
		return nil, fmt.Errorf("read not enougth bytes, expected read %d, actual read %d", len(b), n)
	}
	return b, nil
}

func (p *bufReadWriterImpl) Write(b []byte) error {
	n, err := p.rw.Write(b)
	if err != nil {
		return err
	}
	if n != len(b) {
		return fmt.Errorf("write not enougth bytes, expected write %d, actual write %d", len(b), n)
	}
	p.writeLen += n
	return nil
}

func (p *bufReadWriterImpl) ReadUInt8() (uint8, error) {
	b, err := p.Read(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (p *bufReadWriterImpl) WriteUInt8(value uint8) error {
	return p.Write([]byte{value})
}

func (p *bufReadWriterImpl) ReadUInt16() (uint16, error) {
	b, err := p.Read(2)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(b), nil
}

func (p *bufReadWriterImpl) WriteUInt16(value uint16) error {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, value)
	return p.Write(b)
}

func (p *bufReadWriterImpl) ReadInt16() (int16, error) {
	value, err := p.ReadUInt16()
	if err != nil {
		return 0, err
	}
	return int16(value), err
}

func (p *bufReadWriterImpl) WriteInt16(value int16) error {
	return p.WriteUInt16(uint16(value))
}

func (p *bufReadWriterImpl) ReadUInt32() (uint32, error) {
	b, err := p.Read(4)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(b), nil
}

func (p *bufReadWriterImpl) WriteUInt32(value uint32) error {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, value)
	return p.Write(b)
}

func (p *bufReadWriterImpl) ReadInt32() (int32, error) {
	value, err := p.ReadUInt32()
	if err != nil {
		return 0, err
	}
	return int32(value), err
}

func (p *bufReadWriterImpl) WriteInt32(value int32) error {
	return p.WriteUInt32(uint32(value))
}

func (p *bufReadWriterImpl) ReadVarInt() (int32, error) {
	var value int32 = 0
	position := 0
	for {
		b, err := p.Read(1)
		if err != nil {
			return 0, err
		}

		value |= int32(b[0]&varIntSegmentBits) << position
		if (b[0] & varIntContinueBit) == 0 {
			break
		}

		position += 7
		if position >= 32 {
			return 0, fmt.Errorf("VarInt is too big, %d", position)
		}
	}
	return value, nil
}

func (p *bufReadWriterImpl) WriteVarInt(value int32) error {
	for {
		if (value & ^varIntSegmentBits) == 0 {
			return p.Write([]byte{uint8(value)})
		}

		x := (value & varIntSegmentBits) | varIntContinueBit
		if err := p.Write([]byte{uint8(x)}); err != nil {
			return err
		}

		value = int32(uint32(value) >> 7) // shift the sign bit
	}
}

func (p *bufReadWriterImpl) ReadString() (string, error) {
	length, err := p.ReadVarInt()
	if err != nil {
		return "", err
	}

	b, err := p.Read(int(length))
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func (p *bufReadWriterImpl) WriteString(s string) error {
	err := p.WriteVarInt(int32(len(s)))
	if err != nil {
		return err
	}
	return p.Write([]byte(s))
}

func (p *bufReadWriterImpl) ReadUTF16BE() (string, error) {
	strLen, err := p.ReadInt16()
	if err != nil {
		return "", err
	}

	buf, err := p.Read(int(strLen * 2))
	if err != nil {
		return "", err
	}

	var u16s []uint16
	r := bytes.NewReader(buf)
	for r.Len() > 0 {
		var u16 uint16
		if err := binary.Read(r, binary.BigEndian, &u16); err != nil {
			return "", err
		}
		u16s = append(u16s, u16)
	}

	return string(utf16.Decode(u16s)), nil
}

func (p *bufReadWriterImpl) WriteUTF16BE(s string) error {
	u16s := utf16.Encode([]rune(s))

	var buf bytes.Buffer
	for _, u16 := range u16s {
		if err := binary.Write(&buf, binary.BigEndian, u16); err != nil {
			return err
		}
	}

	b := buf.Bytes()
	if err := p.WriteInt16(int16(len(b))); err != nil {
		return err
	}
	if err := p.Write(b); err != nil {
		return err
	}
	return nil
}
