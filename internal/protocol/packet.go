package protocol

import (
	"bytes"
	"errors"
	"fmt"
)

const (
	HandShakePacketId  = 0x00 // handshake state, C2S
	DisconnectPacketId = 0x00 // login state, S2C

	HandshakeNextStateStatus = 1
	HandshakeNextStateLogin  = 2
)

func ReadHandshakePacket(reader BufferReader) (*HandshakePacket, error) {
	packet, err := readPacket(
		reader,
		func(packetId int32) (Packet, error) {
			if packetId == HandShakePacketId {
				return &HandshakePacket{}, nil
			}
			return nil, fmt.Errorf("unexpected packet ID %d, should be handshake packet ID %d", packetId, HandShakePacketId)
		},
		func(packetLen int32) error {
			if packetLen == 254 {
				return errors.New("discard legacy server ping")
			}
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	return packet.(*HandshakePacket), nil
}

//goland:noinspection GoUnusedExportedFunction
func ReadPacket(reader BufferReader, packetFactory func(int32) (Packet, error)) (Packet, error) {
	return readPacket(reader, packetFactory, nil)
}

func readPacket(reader BufferReader, packetFactory func(int32) (Packet, error), packetLenChecker func(int32) error) (Packet, error) {
	packetLen, err := reader.ReadVarInt()
	if err != nil {
		return nil, fmt.Errorf("failed to read packet length: %v", err)
	}

	if packetLenChecker != nil {
		if err := packetLenChecker(packetLen); err != nil {
			return nil, err
		}
	}

	packetBody, err := reader.Read(int(packetLen))
	if err != nil {
		return nil, fmt.Errorf("failed to read packet body: %v", err)
	}
	bodyReader := NewPacketReadWriter(bytes.NewBuffer(packetBody[:]))

	packetId, err := bodyReader.ReadVarInt()
	if err != nil {
		return nil, fmt.Errorf("failed to read packet ID: %v", err)
	}

	packet, err := packetFactory(packetId)
	if err != nil {
		return nil, fmt.Errorf("failed to create packet for ID %d: %v", packetId, err)
	}

	if err := packet.ReadFrom(bodyReader); err != nil {
		return nil, fmt.Errorf("failed to deserialize packet fields: %v", err)
	}
	if bodyReader.GetReadLen() != int(packetLen) {
		return nil, fmt.Errorf("packet field read len mismatched: total len %d, read len %d", packetLen, bodyReader.GetReadLen())
	}

	return packet, nil
}

func WritePacket(writer BufferWriter, packet Packet) error {
	bodyWriter := NewPacketReadWriter(bytes.NewBuffer([]byte{}))
	if err := bodyWriter.WriteVarInt(packet.GetId()); err != nil {
		return fmt.Errorf("failed to serialize packet ID: %v", err)
	}
	if err := packet.WriteTo(bodyWriter); err != nil {
		return fmt.Errorf("failed to serialize packet fields: %v", err)
	}

	packetLen := bodyWriter.GetWriteLen()
	packetBody, err := bodyWriter.Read(packetLen)
	if err != nil {
		return fmt.Errorf("failed to extract packet body: %v", err)
	}

	if err := writer.WriteVarInt(int32(packetLen)); err != nil {
		return fmt.Errorf("failed to write packet length: %v", err)
	}
	if err := writer.Write(packetBody); err != nil {
		return fmt.Errorf("failed to write packet body: %v", err)
	}

	return nil
}

type Packet interface {
	ReadFrom(reader BufferReader) error
	WriteTo(writer BufferWriter) error
	GetId() int32
}

// HandshakePacket is in handshake state, C2S
type HandshakePacket struct {
	Protocol  int32
	Hostname  string
	Port      uint16
	NextState int32
}

var _ Packet = &HandshakePacket{}

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
	if p.Port, err = reader.ReadUnsignedShort(); err != nil {
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
	if err := writer.WriteUnsignedShort(p.Port); err != nil {
		return fmt.Errorf("failed to write handshake port: %v", err)
	}
	if err := writer.WriteVarInt(p.NextState); err != nil {
		return fmt.Errorf("failed to write handshake next state: %v", err)
	}
	return nil
}

// DisconnectPacket is in login state, S2C
type DisconnectPacket struct {
	Reason string
}

var _ Packet = &DisconnectPacket{}

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
