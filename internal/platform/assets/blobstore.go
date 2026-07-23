package assets

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

type ObjectLocation struct {
	Provider  string
	Bucket    string
	Key       string
	VersionID string
	ETag      string
}

type ObjectInfo struct {
	ObjectLocation
	SizeBytes int64
	MIMEType  string
}

type SignedRequest struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expires_at"`
}

type BlobStore interface {
	Put(context.Context, string, string, io.Reader, int64, string) (ObjectInfo, error)
	Open(context.Context, ObjectLocation) (io.ReadCloser, ObjectInfo, error)
	Head(context.Context, ObjectLocation) (ObjectInfo, error)
	Delete(context.Context, ObjectLocation) error
	SignPut(context.Context, string, string, string, int64, time.Duration) (SignedRequest, error)
	SignGet(context.Context, ObjectLocation, time.Duration) (SignedRequest, error)
}

func validateObjectTarget(bucket, key string) error {
	if strings.TrimSpace(bucket) == "" || strings.TrimSpace(key) == "" {
		return fmt.Errorf("bucket and object key are required")
	}
	if len(bucket) > 255 || bucket == "." || bucket == ".." || strings.ContainsAny(bucket, "/\\") ||
		len(key) > 1024 || strings.Contains(key, "..") || strings.HasPrefix(key, "/") || strings.Contains(key, "\\") {
		return fmt.Errorf("object target is invalid")
	}
	for _, character := range bucket {
		if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_' || character == '.') {
			return fmt.Errorf("bucket contains unsupported characters")
		}
	}
	for _, character := range key {
		if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '/' || character == '-' || character == '_' || character == '.') {
			return fmt.Errorf("object key contains unsupported characters")
		}
	}
	return nil
}
