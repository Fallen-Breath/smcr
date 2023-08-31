package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

type BufferReader interface {
	GetReadLen() int
	Read(n int) ([]byte, error)
	ReadUnsignedShort() (uint16, error)
	ReadVarInt() (int32, error)
	ReadString() (string, error)
}

type BufferWriter interface {
	GetWriteLen() int
	Write(b []byte) error
	WriteVarInt(value int32) error
	WriteUnsignedShort(value uint16) error
	WriteString(s string) error
}

type BufferReadWriter interface {
	BufferReader
	BufferWriter
}

type bufferReadWriterImpl struct {
	buf      io.ReadWriter
	writeLen int
	readLen  int
}

var _ BufferReadWriter = &bufferReadWriterImpl{}

func NewPacketReadWriter(buf io.ReadWriter) BufferReadWriter {
	return &bufferReadWriterImpl{
		buf:      buf,
		readLen:  0,
		writeLen: 0,
	}
}

const (
	varIntSegmentBits = 0x7F
	varIntContinueBit = 0x80
)

func (p *bufferReadWriterImpl) GetReadLen() int {
	return p.readLen
}

func (p *bufferReadWriterImpl) GetWriteLen() int {
	return p.writeLen
}

func (p *bufferReadWriterImpl) Read(n int) ([]byte, error) {
	b := make([]byte, n)
	n, err := p.buf.Read(b)
	if err != nil {
		return nil, err
	}
	if n != len(b) {
		return nil, fmt.Errorf("read not enougth bytes, expected read %d, actual read %d", len(b), n)
	}
	p.readLen += n
	return b, nil
}

func (p *bufferReadWriterImpl) Write(b []byte) error {
	n, err := p.buf.Write(b)
	if err != nil {
		return err
	}
	if n != len(b) {
		return fmt.Errorf("write not enougth bytes, expected write %d, actual write %d", len(b), n)
	}
	p.writeLen += n
	return nil
}

func (p *bufferReadWriterImpl) ReadUnsignedShort() (uint16, error) {
	b, err := p.Read(2)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(b), nil
}

func (p *bufferReadWriterImpl) WriteUnsignedShort(value uint16) error {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, value)
	return p.Write(b)
}

func (p *bufferReadWriterImpl) ReadVarInt() (int32, error) {
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

func (p *bufferReadWriterImpl) WriteVarInt(value int32) error {
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

func (p *bufferReadWriterImpl) ReadString() (string, error) {
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

func (p *bufferReadWriterImpl) WriteString(s string) error {
	err := p.WriteVarInt(int32(len(s)))
	if err != nil {
		return err
	}
	return p.Write([]byte(s))
}
