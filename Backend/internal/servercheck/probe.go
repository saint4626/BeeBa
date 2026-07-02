package servercheck

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"time"
	"unicode/utf8"

	"beeba.org/internal/domain/servers"
)

const (
	ServerInfoQueryMagic      uint32 = 0xBA515101
	ServerInfoResponseMagic   uint32 = 0xBA515102
	ServerInfoProtocolVersion uint16 = 1
	ServerInfoMinRequestBytes        = 384

	liteNetLibPacketPropertyMask         byte = 0x1F
	liteNetLibUnconnectedMessageProperty byte = 8
	liteNetLibInvalidProtocolProperty    byte = 15
	liteNetLibHeaderSize                      = 1
)

type Info struct {
	OnlinePlayers   int
	MaxPlayers      int
	ProtocolVersion int
	ServerName      string
	Motd            string
	RoundTripMS     int
}

func Probe(ctx context.Context, host string, port uint16, timeout time.Duration) (Info, error) {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	if err := servers.ValidateEndpointForPublicCheck(servers.Endpoint{Host: host, Port: port}); err != nil {
		return Info{}, err
	}

	resolveCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupIPAddr(resolveCtx, host)
	if err != nil {
		return Info{}, fmt.Errorf("resolve server host: %w", err)
	}
	var selected net.IP
	for _, address := range addresses {
		if err := servers.ValidateEndpointForPublicCheck(servers.Endpoint{Host: address.IP.String(), Port: port}); err == nil {
			selected = address.IP
			break
		}
	}
	if selected == nil {
		return Info{}, servers.ErrUnsafeEndpoint
	}

	nonce, err := randomNonce()
	if err != nil {
		return Info{}, err
	}
	packet := BuildQueryPacket(nonce)
	remote := net.JoinHostPort(selected.String(), strconv.Itoa(int(port)))
	start := time.Now()
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "udp", remote)
	if err != nil {
		return Info{}, fmt.Errorf("dial server info UDP: %w", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(timeout)
	_ = conn.SetDeadline(deadline)
	if _, err := conn.Write(packet); err != nil {
		return Info{}, fmt.Errorf("send server info query: %w", err)
	}
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		return Info{}, fmt.Errorf("read server info response: %w", err)
	}
	return ParseResponse(buffer[:n], nonce, int(time.Since(start).Milliseconds()))
}

func BuildQueryPacket(nonce uint16) []byte {
	packet := make([]byte, 0, liteNetLibHeaderSize+ServerInfoMinRequestBytes)
	packet = append(packet, liteNetLibUnconnectedMessageProperty)
	packet = appendUint32LE(packet, ServerInfoQueryMagic)
	packet = appendUint16LE(packet, ServerInfoProtocolVersion)
	packet = appendUint16LE(packet, nonce)
	for len(packet)-liteNetLibHeaderSize < ServerInfoMinRequestBytes {
		packet = append(packet, 0)
	}
	return packet
}

func ParseResponse(packet []byte, expectedNonce uint16, roundTripMS int) (Info, error) {
	payload, err := unwrapLiteNetLibUnconnectedPacket(packet)
	if err != nil {
		return Info{}, err
	}
	reader := packetReader{data: payload}
	magic, err := reader.uint32()
	if err != nil {
		return Info{}, err
	}
	if magic != ServerInfoResponseMagic {
		return Info{}, fmt.Errorf("unexpected server info response magic")
	}
	protocol, err := reader.uint16()
	if err != nil {
		return Info{}, err
	}
	nonce, err := reader.uint16()
	if err != nil {
		return Info{}, err
	}
	if nonce != expectedNonce {
		return Info{}, fmt.Errorf("unexpected server info response nonce")
	}
	online, err := reader.uint16()
	if err != nil {
		return Info{}, err
	}
	maxPlayers, err := reader.uint16()
	if err != nil {
		return Info{}, err
	}
	name, err := reader.liteNetString()
	if err != nil {
		return Info{}, err
	}
	motd, err := reader.liteNetString()
	if err != nil {
		return Info{}, err
	}
	return Info{
		OnlinePlayers:   int(online),
		MaxPlayers:      int(maxPlayers),
		ProtocolVersion: int(protocol),
		ServerName:      name,
		Motd:            motd,
		RoundTripMS:     roundTripMS,
	}, nil
}

func unwrapLiteNetLibUnconnectedPacket(packet []byte) ([]byte, error) {
	if len(packet) < liteNetLibHeaderSize {
		return nil, fmt.Errorf("server info response is truncated")
	}
	property := packet[0] & liteNetLibPacketPropertyMask
	if property == liteNetLibInvalidProtocolProperty {
		return nil, fmt.Errorf("server rejected server info query as invalid LiteNetLib protocol")
	}
	if property != liteNetLibUnconnectedMessageProperty {
		return nil, fmt.Errorf("unexpected LiteNetLib packet property %d", property)
	}
	return packet[liteNetLibHeaderSize:], nil
}

func randomNonce() (uint16, error) {
	var raw [2]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return 0, fmt.Errorf("generate server check nonce: %w", err)
	}
	return binary.LittleEndian.Uint16(raw[:]), nil
}

type packetReader struct {
	data []byte
	pos  int
}

func (r *packetReader) uint16() (uint16, error) {
	if len(r.data)-r.pos < 2 {
		return 0, fmt.Errorf("server info response is truncated")
	}
	value := readUint16LE(r.data[r.pos : r.pos+2])
	r.pos += 2
	return value, nil
}

func (r *packetReader) uint32() (uint32, error) {
	if len(r.data)-r.pos < 4 {
		return 0, fmt.Errorf("server info response is truncated")
	}
	value := readUint32LE(r.data[r.pos : r.pos+4])
	r.pos += 4
	return value, nil
}

func (r *packetReader) liteNetString() (string, error) {
	size, err := r.uint16()
	if err != nil {
		return "", err
	}
	if size == 0 {
		return "", nil
	}
	actualSize := int(size) - 1
	if actualSize < 0 || len(r.data)-r.pos < actualSize {
		return "", fmt.Errorf("server info string is truncated")
	}
	raw := r.data[r.pos : r.pos+actualSize]
	r.pos += actualSize
	if !utf8.Valid(raw) {
		return "", fmt.Errorf("server info string is not valid UTF-8")
	}
	return string(raw), nil
}

func appendUint16LE(packet []byte, value uint16) []byte {
	return binary.LittleEndian.AppendUint16(packet, value)
}

func appendUint32LE(packet []byte, value uint32) []byte {
	return binary.LittleEndian.AppendUint32(packet, value)
}

func readUint16LE(packet []byte) uint16 {
	return binary.LittleEndian.Uint16(packet)
}

func readUint32LE(packet []byte) uint32 {
	return binary.LittleEndian.Uint32(packet)
}

func appendLiteNetString(packet []byte, value string) []byte {
	if value == "" {
		return appendUint16LE(packet, 0)
	}
	raw := []byte(value)
	packet = appendUint16LE(packet, uint16(len(raw)+1))
	return append(packet, raw...)
}
