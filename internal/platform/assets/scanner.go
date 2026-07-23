package assets

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

var ErrMalwareDetected = errors.New("asset content was rejected by malware scanner")

type ContentScanner interface {
	Scan(context.Context, io.Reader) error
}

type NoopScanner struct{}

func (NoopScanner) Scan(context.Context, io.Reader) error { return nil }

// ClamAVScanner implements ClamAV's INSTREAM protocol. The connection is
// bounded by the request context and a per-connection timeout.
type ClamAVScanner struct {
	Address     string
	DialTimeout time.Duration
	ChunkSize   int
}

func (s ClamAVScanner) Scan(ctx context.Context, content io.Reader) error {
	if strings.TrimSpace(s.Address) == "" {
		return fmt.Errorf("ClamAV address is required")
	}
	timeout := s.DialTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	connection, err := dialer.DialContext(ctx, "tcp", s.Address)
	if err != nil {
		return fmt.Errorf("connect to ClamAV: %w", err)
	}
	defer connection.Close()
	deadline := time.Now().Add(timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	_ = connection.SetDeadline(deadline)
	if _, err := connection.Write([]byte("zINSTREAM\x00")); err != nil {
		return fmt.Errorf("start ClamAV stream: %w", err)
	}
	chunkSize := s.ChunkSize
	if chunkSize <= 0 || chunkSize > 1024*1024 {
		chunkSize = 64 * 1024
	}
	buffer := make([]byte, chunkSize)
	for {
		count, readErr := content.Read(buffer)
		if count > 0 {
			var length [4]byte
			binary.BigEndian.PutUint32(length[:], uint32(count))
			if _, err := connection.Write(length[:]); err != nil {
				return fmt.Errorf("write ClamAV chunk length: %w", err)
			}
			if _, err := connection.Write(buffer[:count]); err != nil {
				return fmt.Errorf("write ClamAV chunk: %w", err)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read content for ClamAV: %w", readErr)
		}
	}
	if _, err := connection.Write([]byte{0, 0, 0, 0}); err != nil {
		return fmt.Errorf("finish ClamAV stream: %w", err)
	}
	response, err := bufio.NewReader(connection).ReadString(0)
	if err != nil && err != io.EOF {
		return fmt.Errorf("read ClamAV response: %w", err)
	}
	response = strings.TrimSpace(strings.TrimSuffix(response, "\x00"))
	if strings.HasSuffix(response, "OK") {
		return nil
	}
	if strings.Contains(response, "FOUND") {
		return ErrMalwareDetected
	}
	return fmt.Errorf("unexpected ClamAV response")
}
