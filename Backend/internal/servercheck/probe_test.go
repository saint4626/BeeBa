package servercheck

import "testing"

func TestBuildQueryPacketMatchesBasisMinimumRequest(t *testing.T) {
	packet := BuildQueryPacket(42)
	if len(packet) != liteNetLibHeaderSize+ServerInfoMinRequestBytes {
		t.Fatalf("len(packet) = %d, want %d", len(packet), liteNetLibHeaderSize+ServerInfoMinRequestBytes)
	}
	if got := packet[0] & liteNetLibPacketPropertyMask; got != liteNetLibUnconnectedMessageProperty {
		t.Fatalf("packet property = %d, want %d", got, liteNetLibUnconnectedMessageProperty)
	}
	payload := packet[liteNetLibHeaderSize:]
	if got := readUint32LE(payload[:4]); got != ServerInfoQueryMagic {
		t.Fatalf("query magic = %#x, want %#x", got, ServerInfoQueryMagic)
	}
	if got := readUint16LE(payload[4:6]); got != ServerInfoProtocolVersion {
		t.Fatalf("protocol = %d, want %d", got, ServerInfoProtocolVersion)
	}
	if got := readUint16LE(payload[6:8]); got != 42 {
		t.Fatalf("nonce = %d, want 42", got)
	}
}

func TestParseResponseReadsLiteNetLibStrings(t *testing.T) {
	packet := []byte{liteNetLibUnconnectedMessageProperty}
	packet = appendUint32LE(packet, ServerInfoResponseMagic)
	packet = appendUint16LE(packet, ServerInfoProtocolVersion)
	packet = appendUint16LE(packet, 42)
	packet = appendUint16LE(packet, 3)
	packet = appendUint16LE(packet, 32)
	packet = appendLiteNetString(packet, "BeeBa Test")
	packet = appendLiteNetString(packet, "Welcome")

	info, err := ParseResponse(packet, 42, 17)
	if err != nil {
		t.Fatalf("ParseResponse returned error: %v", err)
	}
	if info.OnlinePlayers != 3 || info.MaxPlayers != 32 {
		t.Fatalf("players = %d/%d, want 3/32", info.OnlinePlayers, info.MaxPlayers)
	}
	if info.ServerName != "BeeBa Test" {
		t.Fatalf("name = %q, want BeeBa Test", info.ServerName)
	}
	if info.Motd != "Welcome" {
		t.Fatalf("motd = %q, want Welcome", info.Motd)
	}
	if info.RoundTripMS != 17 {
		t.Fatalf("rtt = %d, want 17", info.RoundTripMS)
	}
}

func TestParseResponseRejectsWrongNonce(t *testing.T) {
	packet := []byte{liteNetLibUnconnectedMessageProperty}
	packet = appendUint32LE(packet, ServerInfoResponseMagic)
	packet = appendUint16LE(packet, ServerInfoProtocolVersion)
	packet = appendUint16LE(packet, 99)

	if _, err := ParseResponse(packet, 42, 1); err == nil {
		t.Fatal("ParseResponse returned nil error for wrong nonce")
	}
}

func TestParseResponseRejectsInvalidLiteNetLibProtocol(t *testing.T) {
	packet := []byte{liteNetLibInvalidProtocolProperty}

	if _, err := ParseResponse(packet, 42, 1); err == nil {
		t.Fatal("ParseResponse returned nil error for invalid LiteNetLib protocol")
	}
}
