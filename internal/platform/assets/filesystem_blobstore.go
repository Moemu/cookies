package assets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FilesystemBlobStore is a persistent local-development store. Production is
// deliberately restricted to TOS by config validation.
type FilesystemBlobStore struct {
	root string
}

func NewFilesystemBlobStore(root string) (*FilesystemBlobStore, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("filesystem blob root is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve filesystem blob root: %w", err)
	}
	if err := os.MkdirAll(absolute, 0o700); err != nil {
		return nil, fmt.Errorf("create filesystem blob root: %w", err)
	}
	return &FilesystemBlobStore{root: absolute}, nil
}

func (s *FilesystemBlobStore) Put(ctx context.Context, bucket, key string, content io.Reader, size int64, mimeType string) (ObjectInfo, error) {
	if size < 0 {
		return ObjectInfo{}, fmt.Errorf("object size must not be negative")
	}
	target, err := s.objectPath(bucket, key)
	if err != nil {
		return ObjectInfo{}, err
	}
	if err := ctx.Err(); err != nil {
		return ObjectInfo{}, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return ObjectInfo{}, fmt.Errorf("create object directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), ".upload-*")
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("create temporary object: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)

	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(temporary, hash), io.LimitReader(content, size+1))
	closeErr := temporary.Close()
	if copyErr != nil {
		return ObjectInfo{}, fmt.Errorf("write object: %w", copyErr)
	}
	if closeErr != nil {
		return ObjectInfo{}, fmt.Errorf("close object: %w", closeErr)
	}
	if written != size {
		return ObjectInfo{}, fmt.Errorf("object size does not match content length")
	}
	if err := ctx.Err(); err != nil {
		return ObjectInfo{}, err
	}

	etag := fmt.Sprintf("%x", hash.Sum(nil))
	if err := os.Link(temporaryName, target); err != nil {
		existing, readErr := os.ReadFile(target)
		candidate, candidateErr := os.ReadFile(temporaryName)
		if readErr != nil || candidateErr != nil || int64(len(existing)) != size || !bytes.Equal(existing, candidate) {
			return ObjectInfo{}, fmt.Errorf("create immutable object: %w", err)
		}
		return ObjectInfo{ObjectLocation: ObjectLocation{Provider: "filesystem", Bucket: bucket, Key: key, ETag: etag}, SizeBytes: size, MIMEType: mimeType}, nil
	}
	return ObjectInfo{ObjectLocation: ObjectLocation{Provider: "filesystem", Bucket: bucket, Key: key, ETag: etag}, SizeBytes: size, MIMEType: mimeType}, nil
}

func (s *FilesystemBlobStore) Open(ctx context.Context, location ObjectLocation) (io.ReadCloser, ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, ObjectInfo{}, err
	}
	path, err := s.objectPath(location.Bucket, location.Key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, ObjectInfo{}, fmt.Errorf("open filesystem object: %w", err)
	}
	info, err := filesystemObjectInfo(file, location)
	if err != nil {
		file.Close()
		return nil, ObjectInfo{}, err
	}
	return file, info, nil
}

func (s *FilesystemBlobStore) Head(ctx context.Context, location ObjectLocation) (ObjectInfo, error) {
	reader, info, err := s.Open(ctx, location)
	if err != nil {
		return ObjectInfo{}, err
	}
	if err := reader.Close(); err != nil {
		return ObjectInfo{}, fmt.Errorf("close filesystem object: %w", err)
	}
	return info, nil
}

func (s *FilesystemBlobStore) Delete(ctx context.Context, location ObjectLocation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path, err := s.objectPath(location.Bucket, location.Key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete filesystem object: %w", err)
	}
	return nil
}

func (s *FilesystemBlobStore) SignPut(_ context.Context, bucket, key, _ string, size int64, ttl time.Duration) (SignedRequest, error) {
	if _, err := s.objectPath(bucket, key); err != nil {
		return SignedRequest{}, err
	}
	if ttl <= 0 || ttl > time.Hour || size < 1 {
		return SignedRequest{}, fmt.Errorf("signed upload lifetime or size is invalid")
	}
	return SignedRequest{Method: http.MethodPut, Headers: map[string]string{}, ExpiresAt: time.Now().UTC().Add(ttl)}, nil
}

func (s *FilesystemBlobStore) SignGet(ctx context.Context, location ObjectLocation, ttl time.Duration) (SignedRequest, error) {
	if ttl <= 0 || ttl > time.Hour {
		return SignedRequest{}, fmt.Errorf("signed download lifetime is invalid")
	}
	if _, err := s.Head(ctx, location); err != nil {
		return SignedRequest{}, err
	}
	return SignedRequest{Method: http.MethodGet, Headers: map[string]string{}, ExpiresAt: time.Now().UTC().Add(ttl)}, nil
}

func (s *FilesystemBlobStore) objectPath(bucket, key string) (string, error) {
	if s == nil || s.root == "" {
		return "", fmt.Errorf("filesystem blob root is required")
	}
	if err := validateObjectTarget(bucket, key); err != nil {
		return "", err
	}
	target := filepath.Join(s.root, bucket, filepath.FromSlash(key))
	relative, err := filepath.Rel(s.root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("object target escapes filesystem blob root")
	}
	return target, nil
}

func filesystemObjectInfo(file *os.File, location ObjectLocation) (ObjectInfo, error) {
	stat, err := file.Stat()
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("stat filesystem object: %w", err)
	}
	if !stat.Mode().IsRegular() {
		return ObjectInfo{}, fmt.Errorf("filesystem object is not a regular file")
	}
	prefix := make([]byte, 512)
	read, err := file.Read(prefix)
	if err != nil && err != io.EOF {
		return ObjectInfo{}, fmt.Errorf("read filesystem object metadata: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return ObjectInfo{}, fmt.Errorf("rewind filesystem object: %w", err)
	}
	location.Provider = "filesystem"
	return ObjectInfo{ObjectLocation: location, SizeBytes: stat.Size(), MIMEType: http.DetectContentType(prefix[:read])}, nil
}
