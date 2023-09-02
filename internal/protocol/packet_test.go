package protocol

import (
	"bytes"
	"testing"
)

func TestLegacy(t *testing.T) {
	reader := NewBufferReadWriter(bytes.NewBuffer(legacyServerPingHead[:]))

	// see https://wiki.vg/Server_List_Ping#1.6
	if value, err := reader.ReadUInt8(); value != 0xFE || err != nil {
		t.Fatalf("Read packet identifier for a server list ping failed %d %v", value, err)
	}
	if value, err := reader.ReadUInt8(); value != 0x01 || err != nil {
		t.Fatalf("Read server list ping's payload (always 1) failed %d %v", value, err)
	}
	if value, err := reader.ReadUInt8(); value != 0xFA || err != nil {
		t.Fatalf("Read packet identifier for a plugin message failed %d %v", value, err)
	}
	if value, err := reader.ReadUTF16BE(); value != "MC|PingHost" || err != nil {
		t.Fatalf("Read 'MC|PingHost' failed %s %v", value, err)
	}
}
