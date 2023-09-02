package protocol

import (
	"bytes"
	"fmt"
)

func ReadHandshakePacket(reader BufferReader) (IHandshakePacket, error) {
	head, err := reader.Peek(1)
	if err != nil {
		return nil, fmt.Errorf("failed to peak the first byte: %v", err)
	}
	if head[0] == legacyHandshakeMagic {
		return readLegacyServerListPing(reader)
	}

	packet, err := ReadModernPacket(
		reader,
		func(packetId int32) (ModernPacket, error) {
			if packetId == HandShakePacketId {
				return &HandshakePacket{}, nil
			}
			return nil, fmt.Errorf("unexpected packet ID %d, should be handshake packet ID %d", packetId, HandShakePacketId)
		},
	)
	if err != nil {
		return nil, err
	}
	return packet.(*HandshakePacket), nil
}

func ReadModernPacket(reader BufferReader, packetFactory func(int32) (ModernPacket, error)) (ModernPacket, error) {
	packetLen, err := reader.ReadVarInt()
	if err != nil {
		return nil, fmt.Errorf("failed to read packet length: %v", err)
	}

	packetBody, err := reader.Read(int(packetLen))
	if err != nil {
		return nil, fmt.Errorf("failed to read packet body: %v", err)
	}
	var bodyReader BufferReader = NewBufferReadWriter(bytes.NewBuffer(packetBody[:]))

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

func readLegacyServerListPing(reader BufferReader) (*LegacyServerListPingPacket, error) {
	packet := LegacyServerListPingPacket{}
	if err := packet.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("failed to read legacy server list ping packet: %v", err)
	}
	return &packet, nil
}

func WritePacket(writer BufferWriter, packet Packet) error {
	if mp, ok := packet.(ModernPacket); ok {
		bodyWriter := NewBufferReadWriter(&bytes.Buffer{})
		if err := bodyWriter.WriteVarInt(mp.GetId()); err != nil {
			return fmt.Errorf("failed to write packet id: %v", err)
		}
		if err := packet.WriteTo(bodyWriter); err != nil {
			return fmt.Errorf("failed to serialize packet fields: %v", err)
		}

		packetBody, err := bodyWriter.Read(bodyWriter.GetWriteLen())
		if err != nil {
			return fmt.Errorf("failed to extract buffer: %v", err)
		}

		if err := writer.WriteVarInt(int32(len(packetBody))); err != nil {
			return fmt.Errorf("failed to write packet length: %v", err)
		}
		if err := writer.Write(packetBody); err != nil {
			return fmt.Errorf("failed to write packet body: %v", err)
		}
		return nil

	} else if lp, ok := packet.(*LegacyServerListPingPacket); ok {
		return lp.WriteTo(writer)

	} else {
		return fmt.Errorf("unsupported packet %+v", packet)
	}
}
