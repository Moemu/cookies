package assets

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

type memoryObject struct {
	data []byte
	info ObjectInfo
}

type MemoryBlobStore struct {
	mu      sync.RWMutex
	objects map[string]memoryObject
}

func NewMemoryBlobStore() *MemoryBlobStore {
	return &MemoryBlobStore{objects: make(map[string]memoryObject)}
}

func (s *MemoryBlobStore) Put(_ context.Context, bucket, key string, content io.Reader, size int64, mimeType string) (ObjectInfo, error) {
	if err := validateObjectTarget(bucket, key); err != nil {
		return ObjectInfo{}, err
	}
	if size < 0 {
		return ObjectInfo{}, fmt.Errorf("object size must not be negative")
	}
	data, err := io.ReadAll(io.LimitReader(content, size+1))
	if err != nil {
		return ObjectInfo{}, err
	}
	if int64(len(data)) != size {
		return ObjectInfo{}, fmt.Errorf("object size does not match content length")
	}
	info := ObjectInfo{ObjectLocation: ObjectLocation{Provider: "memory", Bucket: bucket, Key: key, ETag: fmt.Sprintf("memory-%x", len(data))}, SizeBytes: size, MIMEType: mimeType}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.objects[bucket+"\x00"+key]; ok {
		if bytes.Equal(existing.data, data) && existing.info.MIMEType == mimeType {
			return existing.info, nil
		}
		return ObjectInfo{}, fmt.Errorf("object already exists with different content")
	}
	s.objects[bucket+"\x00"+key] = memoryObject{data: append([]byte(nil), data...), info: info}
	return info, nil
}

func (s *MemoryBlobStore) Open(_ context.Context, location ObjectLocation) (io.ReadCloser, ObjectInfo, error) {
	object, err := s.object(location)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	return io.NopCloser(bytes.NewReader(object.data)), object.info, nil
}

func (s *MemoryBlobStore) Head(_ context.Context, location ObjectLocation) (ObjectInfo, error) {
	object, err := s.object(location)
	return object.info, err
}

func (s *MemoryBlobStore) Delete(_ context.Context, location ObjectLocation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, location.Bucket+"\x00"+location.Key)
	return nil
}

func (s *MemoryBlobStore) SignPut(_ context.Context, bucket, key, _ string, _ int64, ttl time.Duration) (SignedRequest, error) {
	if err := validateObjectTarget(bucket, key); err != nil {
		return SignedRequest{}, err
	}
	if ttl <= 0 || ttl > time.Hour {
		return SignedRequest{}, fmt.Errorf("signed upload lifetime is invalid")
	}
	return SignedRequest{Method: "PUT", Headers: map[string]string{}, ExpiresAt: time.Now().UTC().Add(ttl)}, nil
}

func (s *MemoryBlobStore) SignGet(_ context.Context, location ObjectLocation, ttl time.Duration) (SignedRequest, error) {
	if _, err := s.object(location); err != nil {
		return SignedRequest{}, err
	}
	if ttl <= 0 || ttl > time.Hour {
		return SignedRequest{}, fmt.Errorf("signed download lifetime is invalid")
	}
	return SignedRequest{Method: "GET", Headers: map[string]string{}, ExpiresAt: time.Now().UTC().Add(ttl)}, nil
}

func (s *MemoryBlobStore) object(location ObjectLocation) (memoryObject, error) {
	if err := validateObjectTarget(location.Bucket, location.Key); err != nil {
		return memoryObject{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	object, ok := s.objects[location.Bucket+"\x00"+location.Key]
	if !ok {
		return memoryObject{}, fmt.Errorf("object not found")
	}
	object.data = append([]byte(nil), object.data...)
	return object, nil
}
