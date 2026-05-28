package antivirus

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

const maxClamdChunk = 1024 * 1024

var ErrInfected = errors.New("antivirus detected infected content")

type Result struct {
	Scanner   string `json:"scanner"`
	Result    string `json:"result"`
	Signature string `json:"signature,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type ClamdScanner struct {
	addr    string
	timeout time.Duration
	dialer  net.Dialer
}

func NewClamdScanner(addr string, timeout time.Duration) (*ClamdScanner, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, fmt.Errorf("clamav address is required")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("clamav timeout must be greater than 0")
	}
	return &ClamdScanner{
		addr:    addr,
		timeout: timeout,
		dialer:  net.Dialer{Timeout: timeout},
	}, nil
}

func (s *ClamdScanner) Scan(ctx context.Context, reader io.Reader) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	conn, err := s.dialer.DialContext(ctx, "tcp", s.addr)
	if err != nil {
		return Result{}, fmt.Errorf("connect clamd: %w", err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if _, err := conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return Result{}, fmt.Errorf("send clamd instream command: %w", err)
	}

	buffer := make([]byte, maxClamdChunk)
	for {
		n, readErr := reader.Read(buffer)
		if n > 0 {
			var size [4]byte
			binary.BigEndian.PutUint32(size[:], uint32(n))
			if _, err := conn.Write(size[:]); err != nil {
				return Result{}, fmt.Errorf("send clamd chunk size: %w", err)
			}
			if _, err := conn.Write(buffer[:n]); err != nil {
				return Result{}, fmt.Errorf("send clamd chunk: %w", err)
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return Result{}, fmt.Errorf("read antivirus stream: %w", readErr)
		}
	}
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return Result{}, fmt.Errorf("finish clamd stream: %w", err)
	}

	response, err := bufio.NewReader(conn).ReadString('\x00')
	if err != nil && !errors.Is(err, io.EOF) {
		return Result{}, fmt.Errorf("read clamd response: %w", err)
	}
	result, err := parseClamdResponse(response)
	if err != nil {
		return result, err
	}
	return result, nil
}

func parseClamdResponse(response string) (Result, error) {
	trimmed := strings.TrimSpace(strings.TrimRight(response, "\x00"))
	if strings.HasSuffix(trimmed, ": OK") || trimmed == "stream: OK" {
		return Result{Scanner: "clamav", Result: "clean"}, nil
	}
	if strings.HasSuffix(trimmed, " FOUND") {
		signature := strings.TrimSuffix(trimmed, " FOUND")
		if index := strings.LastIndex(signature, ": "); index >= 0 {
			signature = signature[index+2:]
		}
		return Result{Scanner: "clamav", Result: "infected", Signature: signature}, ErrInfected
	}
	if strings.Contains(trimmed, " ERROR") {
		return Result{Scanner: "clamav", Result: "failed", Reason: trimmed}, fmt.Errorf("clamd scan failed: %s", trimmed)
	}
	if trimmed == "" {
		return Result{Scanner: "clamav", Result: "failed", Reason: "empty response"}, fmt.Errorf("clamd scan failed: empty response")
	}
	return Result{Scanner: "clamav", Result: "failed", Reason: trimmed}, fmt.Errorf("clamd scan returned unexpected response: %s", trimmed)
}
