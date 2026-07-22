package assets

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/volcengine/ve-tos-golang-sdk/v2/tos"
	"github.com/volcengine/ve-tos-golang-sdk/v2/tos/enum"
)

type TOSConfig struct {
	Endpoint      string
	Region        string
	AccessKey     string
	SecretKey     string
	SecurityToken string
}

type TOSBlobStore struct{ client *tos.ClientV2 }

func NewTOSBlobStore(config TOSConfig) (*TOSBlobStore, error) {
	if config.Endpoint == "" || config.Region == "" || config.AccessKey == "" || config.SecretKey == "" {
		return nil, fmt.Errorf("TOS endpoint, region, access key, and secret key are required")
	}
	credentials := tos.NewStaticCredentials(config.AccessKey, config.SecretKey)
	if config.SecurityToken != "" {
		credentials.WithSecurityToken(config.SecurityToken)
	}
	client, err := tos.NewClientV2(config.Endpoint, tos.WithRegion(config.Region), tos.WithCredentials(credentials), tos.WithMaxRetryCount(3))
	if err != nil {
		return nil, fmt.Errorf("create TOS client: %w", err)
	}
	return &TOSBlobStore{client: client}, nil
}

func (s *TOSBlobStore) Put(ctx context.Context, bucket, key string, content io.Reader, size int64, mimeType string) (ObjectInfo, error) {
	if s == nil || s.client == nil {
		return ObjectInfo{}, fmt.Errorf("TOS client is required")
	}
	if err := validateObjectTarget(bucket, key); err != nil {
		return ObjectInfo{}, err
	}
	output, err := s.client.PutObjectV2(ctx, &tos.PutObjectV2Input{PutObjectBasicInput: tos.PutObjectBasicInput{
		Bucket: bucket, Key: key, ContentLength: size, ContentType: mimeType, ForbidOverwrite: true,
	}, Content: content})
	if err != nil {
		// A lost response can make PUT outcome ambiguous. Stable, unguessable
		// keys make a matching existing object a safe idempotent success; the
		// intake path still verifies the bytes and SHA-256 before visibility.
		existing, headErr := s.Head(ctx, ObjectLocation{Provider: "tos", Bucket: bucket, Key: key})
		if headErr == nil && existing.SizeBytes == size && existing.MIMEType == mimeType {
			return existing, nil
		}
		return ObjectInfo{}, fmt.Errorf("put TOS object: %w", err)
	}
	return ObjectInfo{ObjectLocation: ObjectLocation{Provider: "tos", Bucket: bucket, Key: key, VersionID: output.VersionID, ETag: output.ETag}, SizeBytes: size, MIMEType: mimeType}, nil
}

func (s *TOSBlobStore) Open(ctx context.Context, location ObjectLocation) (io.ReadCloser, ObjectInfo, error) {
	if s == nil || s.client == nil {
		return nil, ObjectInfo{}, fmt.Errorf("TOS client is required")
	}
	if err := validateObjectTarget(location.Bucket, location.Key); err != nil {
		return nil, ObjectInfo{}, err
	}
	output, err := s.client.GetObjectV2(ctx, &tos.GetObjectV2Input{Bucket: location.Bucket, Key: location.Key, VersionID: location.VersionID})
	if err != nil {
		return nil, ObjectInfo{}, fmt.Errorf("get TOS object: %w", err)
	}
	info := ObjectInfo{ObjectLocation: ObjectLocation{Provider: "tos", Bucket: location.Bucket, Key: location.Key, VersionID: output.VersionID, ETag: output.ETag}, SizeBytes: output.ContentLength, MIMEType: output.ContentType}
	return output.Content, info, nil
}

func (s *TOSBlobStore) Head(ctx context.Context, location ObjectLocation) (ObjectInfo, error) {
	if s == nil || s.client == nil {
		return ObjectInfo{}, fmt.Errorf("TOS client is required")
	}
	if err := validateObjectTarget(location.Bucket, location.Key); err != nil {
		return ObjectInfo{}, err
	}
	output, err := s.client.HeadObjectV2(ctx, &tos.HeadObjectV2Input{Bucket: location.Bucket, Key: location.Key, VersionID: location.VersionID})
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("head TOS object: %w", err)
	}
	return ObjectInfo{ObjectLocation: ObjectLocation{Provider: "tos", Bucket: location.Bucket, Key: location.Key, VersionID: output.VersionID, ETag: output.ETag}, SizeBytes: output.ContentLength, MIMEType: output.ContentType}, nil
}

func (s *TOSBlobStore) Delete(ctx context.Context, location ObjectLocation) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("TOS client is required")
	}
	if err := validateObjectTarget(location.Bucket, location.Key); err != nil {
		return err
	}
	_, err := s.client.DeleteObjectV2(ctx, &tos.DeleteObjectV2Input{Bucket: location.Bucket, Key: location.Key, VersionID: location.VersionID})
	if err != nil {
		return fmt.Errorf("delete TOS object: %w", err)
	}
	return nil
}

func (s *TOSBlobStore) SignPut(_ context.Context, bucket, key, mimeType string, size int64, ttl time.Duration) (SignedRequest, error) {
	if s == nil || s.client == nil {
		return SignedRequest{}, fmt.Errorf("TOS client is required")
	}
	if err := validateObjectTarget(bucket, key); err != nil {
		return SignedRequest{}, err
	}
	if ttl <= 0 || ttl > time.Hour || size < 1 {
		return SignedRequest{}, fmt.Errorf("signed upload lifetime or size is invalid")
	}
	headers := map[string]string{"Content-Type": mimeType, "Content-Length": strconv.FormatInt(size, 10), "x-tos-forbid-overwrite": "true"}
	output, err := s.client.PreSignedURL(&tos.PreSignedURLInput{HTTPMethod: enum.HttpMethodPut, Bucket: bucket, Key: key, Expires: int64(ttl.Seconds()), Header: headers, IsSignedAllHeaders: true})
	if err != nil {
		return SignedRequest{}, fmt.Errorf("sign TOS upload: %w", err)
	}
	signedHeaders := output.SignedHeader
	if signedHeaders == nil {
		signedHeaders = map[string]string{}
	}
	return SignedRequest{URL: output.SignedUrl, Method: "PUT", Headers: signedHeaders, ExpiresAt: time.Now().UTC().Add(ttl)}, nil
}

func (s *TOSBlobStore) SignGet(_ context.Context, location ObjectLocation, ttl time.Duration) (SignedRequest, error) {
	if s == nil || s.client == nil {
		return SignedRequest{}, fmt.Errorf("TOS client is required")
	}
	if err := validateObjectTarget(location.Bucket, location.Key); err != nil {
		return SignedRequest{}, err
	}
	if ttl <= 0 || ttl > time.Hour {
		return SignedRequest{}, fmt.Errorf("signed download lifetime is invalid")
	}
	query := map[string]string{}
	if location.VersionID != "" {
		query["versionId"] = location.VersionID
	}
	output, err := s.client.PreSignedURL(&tos.PreSignedURLInput{HTTPMethod: enum.HttpMethodGet, Bucket: location.Bucket, Key: location.Key, Expires: int64(ttl.Seconds()), Query: query})
	if err != nil {
		return SignedRequest{}, fmt.Errorf("sign TOS download: %w", err)
	}
	headers := output.SignedHeader
	if headers == nil {
		headers = map[string]string{}
	}
	return SignedRequest{URL: output.SignedUrl, Method: "GET", Headers: headers, ExpiresAt: time.Now().UTC().Add(ttl)}, nil
}
