package protocol

import (
	"bytes"
	"fmt"
	"math"
)

const (
	HandShakePacketId  = 0x00 // handshake state, C2S
	DisconnectPacketId = 0x00 // login state, S2C

	HandshakeNextStateStatus = 1
	HandshakeNextStateLogin  = 2

	legacyHandshakeMagic = 0xFE
)

type Packet interface {
	ReadFrom(reader BufferReader) error
	WriteTo(writer BufferWriter) error
}

type ModernPacket interface {
	Packet
	GetId() int32
}

type IHandshakePacket interface {
	Packet
	IsLegacy() bool
	GetHostname() *string
	GetPort() *uint16
}

// HandshakePacket is in handshake state, C2S
type HandshakePacket struct {
	Protocol  int32
	Hostname  string
	Port      uint16
	NextState int32
}

var _ ModernPacket = &HandshakePacket{}
var _ IHandshakePacket = &HandshakePacket{}

func (p *HandshakePacket) IsLegacy() bool {
	return false
}
func (p *HandshakePacket) GetHostname() *string {
	return &p.Hostname
}
func (p *HandshakePacket) GetPort() *uint16 {
	return &p.Port
}

func (p *HandshakePacket) GetId() int32 {
	return HandShakePacketId
}

func (p *HandshakePacket) ReadFrom(reader BufferReader) error {
	var err error
	if p.Protocol, err = reader.ReadVarInt(); err != nil {
		return fmt.Errorf("failed to read handshake protocol: %v", err)
	}
	if p.Hostname, err = reader.ReadString(); err != nil {
		return fmt.Errorf("failed to read handshake address: %v", err)
	}
	if p.Port, err = reader.ReadUInt16(); err != nil {
		return fmt.Errorf("failed to read handshake port: %v", err)
	}
	if p.NextState, err = reader.ReadVarInt(); err != nil {
		return fmt.Errorf("failed to read handshake next state: %v", err)
	}
	return nil
}

func (p *HandshakePacket) WriteTo(writer BufferWriter) error {
	if err := writer.WriteVarInt(p.Protocol); err != nil {
		return fmt.Errorf("failed to write handshake protocol: %v", err)
	}
	if err := writer.WriteString(p.Hostname); err != nil {
		return fmt.Errorf("failed to write handshake address: %v", err)
	}
	if err := writer.WriteUInt16(p.Port); err != nil {
		return fmt.Errorf("failed to write handshake port: %v", err)
	}
	if err := writer.WriteVarInt(p.NextState); err != nil {
		return fmt.Errorf("failed to write handshake next state: %v", err)
	}
	return nil
}

// LegacyServerListPingPacket is in handshake state, C2S
// see https://wiki.vg/Server_List_Ping#1.6
type LegacyServerListPingPacket struct {
	Header   []byte // the first 27 bytes of whatever things. see legacyServerPingHead
	Protocol uint8
	Hostname string
	Port     uint16
}

var legacyServerPingHead = []byte{
	0xFE,
	0x01,
	0xFA,
	0x00, 0x0B,
	0x00, 0x4D, 0x00, 0x43, 0x00, 0x7C, 0x00, 0x50, 0x00, 0x69, 0x00, 0x6E, 0x00, 0x67, 0x00, 0x48, 0x00, 0x6F, 0x00, 0x73, 0x00, 0x74,
}

var _ IHandshakePacket = &LegacyServerListPingPacket{}

func (p *LegacyServerListPingPacket) IsLegacy() bool {
	return true
}
func (p *LegacyServerListPingPacket) GetHostname() *string {
	return &p.Hostname
}
func (p *LegacyServerListPingPacket) GetPort() *uint16 {
	return &p.Port
}

func (p *LegacyServerListPingPacket) ReadFrom(reader BufferReader) error {
	var err error
	if p.Header, err = reader.Read(len(legacyServerPingHead)); err != nil {
		return fmt.Errorf("failed to read header: %v", err)
	}
	if !bytes.Equal(legacyServerPingHead, p.Header) {
		return fmt.Errorf("invalid header, expected %v, found %v", legacyServerPingHead, p.Header)
	}

	if _, err := reader.ReadInt16(); err != nil {
		return fmt.Errorf("failed to read remaining len: %v", err)
	}
	if p.Protocol, err = reader.ReadUInt8(); err != nil {
		return fmt.Errorf("failed to read protocol: %v", err)
	}
	if p.Hostname, err = reader.ReadUTF16BE(); err != nil {
		return fmt.Errorf("failed to read hostname: %v", err)
	}
	port, err := reader.ReadInt32()
	if err != nil {
		return fmt.Errorf("failed to read port: %v", err)
	}
	if port < 0 || port > math.MaxUint16 {
		return fmt.Errorf("port value %d out of range", port)
	}
	p.Port = uint16(port)

	return nil
}

func (p *LegacyServerListPingPacket) WriteTo(writer BufferWriter) error {
	if err := writer.Write(p.Header); err != nil {
		return fmt.Errorf("failed to write header: %v", err)
	}

	w := NewBufferReadWriter(&bytes.Buffer{})
	if err := w.WriteUInt8(p.Protocol); err != nil {
		return fmt.Errorf("failed to write protocol: %v", err)
	}
	if err := w.WriteUTF16BE(p.Hostname); err != nil {
		return fmt.Errorf("failed to write hostname: %v", err)
	}
	if err := w.WriteInt32(int32(p.Port)); err != nil {
		return fmt.Errorf("failed to write port: %v", err)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush buffer: %v", err)
	}
	b, err := w.Read(w.GetWriteLen())
	if err != nil {
		return fmt.Errorf("failed to extract buffer: %v", err)
	}

	if err := writer.WriteInt16(int16(len(b))); err != nil {
		return fmt.Errorf("failed to write reset length: %v", err)
	}
	if err := writer.Write(b); err != nil {
		return fmt.Errorf("failed to write reset buf: %v", err)
	}
	return nil
}

// DisconnectPacket is in login state, S2C
type DisconnectPacket struct {
	Reason string
}

var _ ModernPacket = &DisconnectPacket{}

func (p *DisconnectPacket) GetId() int32 {
	return DisconnectPacketId
}

func (p *DisconnectPacket) ReadFrom(reader BufferReader) error {
	var err error
	if p.Reason, err = reader.ReadString(); err != nil {
		return fmt.Errorf("failed to read DisconnectPacket reason: %v", err)
	}
	return nil
}

func (p *DisconnectPacket) WriteTo(writer BufferWriter) error {
	if err := writer.WriteString(p.Reason); err != nil {
		return fmt.Errorf("failed to write DisconnectPacket reason: %v", err)
	}
	return nil
}
