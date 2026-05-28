package antivirus

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestParseClamdResponseClean(t *testing.T) {
	result, err := parseClamdResponse("stream: OK\x00")
	if err != nil {
		t.Fatalf("parseClamdResponse returned error: %v", err)
	}
	if result.Result != "clean" {
		t.Fatalf("result = %q", result.Result)
	}
}

func TestParseClamdResponseInfected(t *testing.T) {
	result, err := parseClamdResponse("stream: Eicar-Test-Signature FOUND\x00")
	if !errors.Is(err, ErrInfected) {
		t.Fatalf("expected ErrInfected, got %v", err)
	}
	if result.Signature != "Eicar-Test-Signature" {
		t.Fatalf("signature = %q", result.Signature)
	}
}

func TestParseClamdResponseError(t *testing.T) {
	result, err := parseClamdResponse("stream: INSTREAM size limit exceeded. ERROR\x00")
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Result != "failed" {
		t.Fatalf("result = %q", result.Result)
	}
}

func TestClamdScannerScanSendsInstreamChunks(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	done := make(chan []byte, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- nil
			return
		}
		defer conn.Close()

		command := make([]byte, len("zINSTREAM\x00"))
		if _, err := io.ReadFull(conn, command); err != nil {
			done <- nil
			return
		}
		var payload bytes.Buffer
		for {
			var size [4]byte
			if _, err := io.ReadFull(conn, size[:]); err != nil {
				done <- nil
				return
			}
			n := binary.BigEndian.Uint32(size[:])
			if n == 0 {
				break
			}
			chunk := make([]byte, n)
			if _, err := io.ReadFull(conn, chunk); err != nil {
				done <- nil
				return
			}
			payload.Write(chunk)
		}
		conn.Write([]byte("stream: OK\x00"))
		done <- append(command, payload.Bytes()...)
	}()

	scanner, err := NewClamdScanner(listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("scanner: %v", err)
	}
	result, err := scanner.Scan(context.Background(), bytes.NewReader([]byte("bee-bytes")))
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if result.Result != "clean" {
		t.Fatalf("result = %q", result.Result)
	}
	got := <-done
	if string(got) != "zINSTREAM\x00bee-bytes" {
		t.Fatalf("server saw %q", string(got))
	}
}
