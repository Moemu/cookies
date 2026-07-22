package assets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/ids"
)

type ActiveProjectResolver interface {
	RequireActiveContext(context.Context, contract.ActorContext, contract.ProjectID) (contract.ProjectContext, error)
}

type UploadService struct {
	Repository       Repository
	Projects         ActiveProjectResolver
	Blobs            BlobStore
	Scanner          ContentScanner
	QuarantineBucket string
	AssetsBucket     string
	UploadTTL        time.Duration
	PreviewTTL       time.Duration
	Now              func() time.Time
	NewID            ids.Generator
}

func (s UploadService) Create(ctx context.Context, requestContext contract.RequestContext, projectID contract.ProjectID, key contract.IdempotencyKey, request CreateUploadRequest) (CreateUploadResponse, error) {
	if err := s.validateDependencies(); err != nil {
		return CreateUploadResponse{}, err
	}
	if err := key.Validate(); err != nil {
		return CreateUploadResponse{}, err
	}
	if err := request.Validate(); err != nil {
		return CreateUploadResponse{}, err
	}
	projectContext, err := s.Projects.RequireActiveContext(ctx, requestContext.Actor, projectID)
	if err != nil {
		return CreateUploadResponse{}, err
	}
	request.Filename = safeFilename(request.Filename)
	requestHash, err := contract.CanonicalJSONHash(request)
	if err != nil {
		return CreateUploadResponse{}, err
	}
	newID := s.idGenerator()
	sessionID, err := newID("upload")
	if err != nil {
		return CreateUploadResponse{}, err
	}
	assetID, err := newID("asset")
	if err != nil {
		return CreateUploadResponse{}, err
	}
	blobID, err := newID("blob")
	if err != nil {
		return CreateUploadResponse{}, err
	}
	now := s.now()
	session := UploadSession{
		ID: sessionID, OrganizationID: requestContext.Actor.OrganizationID, ProjectID: projectID,
		Principal: requestContext.Actor.Principal, Status: UploadCreated, Filename: request.Filename,
		DeclaredMIMEType: request.DeclaredMIMEType, DeclaredSizeBytes: request.DeclaredSizeBytes,
		DeclaredSHA256: request.DeclaredSHA256, Quarantine: ObjectLocation{Provider: s.BlobProvider(), Bucket: s.QuarantineBucket, Key: quarantineKey(requestContext.Actor.OrganizationID, sessionID)},
		IdempotencyKey: key, RequestHash: requestHash, ProjectContextVersion: projectContext.ProjectContextVersion,
		TargetAssetID: contract.AssetID(assetID), TargetBlobID: blobID, RequestID: requestContext.RequestID,
		TraceID: requestContext.TraceID, ExpiresAt: now.Add(s.uploadTTL()), CreatedAt: now, UpdatedAt: now,
	}
	stored, duplicate, err := s.Repository.CreateUpload(ctx, session)
	if err != nil {
		return CreateUploadResponse{}, err
	}
	if duplicate && stored.RequestHash != requestHash {
		return CreateUploadResponse{}, ErrIdempotencyConflict
	}
	if stored.Status != UploadCreated && stored.Status != UploadUploaded {
		return CreateUploadResponse{Session: stored}, nil
	}
	ttl := stored.ExpiresAt.Sub(s.now())
	if ttl <= 0 {
		return CreateUploadResponse{}, ErrInvalidState
	}
	signed, err := s.Blobs.SignPut(ctx, stored.Quarantine.Bucket, stored.Quarantine.Key, stored.DeclaredMIMEType, stored.DeclaredSizeBytes, ttl)
	if err != nil {
		return CreateUploadResponse{}, err
	}
	return CreateUploadResponse{Session: stored, Upload: &signed}, nil
}

// PutContent is a development and fallback proxy. Production clients should
// use the signed TOS PUT returned by Create.
func (s UploadService) PutContent(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, sessionID string, content io.Reader, size int64) error {
	if err := s.validateDependencies(); err != nil {
		return err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return err
	}
	session, err := s.Repository.GetUpload(ctx, actor.OrganizationID, projectID, sessionID)
	if err != nil {
		return err
	}
	if session.Principal != actor.Principal {
		return ErrNotFound
	}
	if session.Status != UploadCreated || s.now().After(session.ExpiresAt) {
		return ErrInvalidState
	}
	if size != session.DeclaredSizeBytes {
		return ErrOutputMetadataMismatch
	}
	if _, err := s.Blobs.Put(ctx, session.Quarantine.Bucket, session.Quarantine.Key, content, size, session.DeclaredMIMEType); err != nil {
		return err
	}
	return s.Repository.MarkUploadUploaded(ctx, actor.OrganizationID, projectID, sessionID, s.now())
}

func (s UploadService) Finalize(ctx context.Context, requestContext contract.RequestContext, projectID contract.ProjectID, sessionID string) (UploadSession, error) {
	if err := s.validateDependencies(); err != nil {
		return UploadSession{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, requestContext.Actor, projectID); err != nil {
		return UploadSession{}, err
	}
	session, err := s.Repository.GetUpload(ctx, requestContext.Actor.OrganizationID, projectID, sessionID)
	if err != nil {
		return UploadSession{}, err
	}
	if session.Principal != requestContext.Actor.Principal {
		return UploadSession{}, ErrNotFound
	}
	if session.Status == UploadSucceeded {
		return session, nil
	}
	if s.now().After(session.ExpiresAt) {
		_ = s.Repository.FailUpload(ctx, session.OrganizationID, session.ProjectID, session.ID, "UPLOAD_EXPIRED", s.now())
		return UploadSession{}, ErrInvalidState
	}
	if session.Status != UploadCreated && session.Status != UploadUploaded && session.Status != UploadProcessing {
		return UploadSession{}, ErrInvalidState
	}
	if session.Status != UploadProcessing {
		if err := s.Repository.MarkUploadProcessing(ctx, session.OrganizationID, session.ProjectID, session.ID, s.now()); err != nil {
			return UploadSession{}, err
		}
	}
	commit, err := s.ingestStoredObject(ctx, session.OrganizationID, session.ProjectID, session.TargetAssetID,
		session.TargetBlobID, session.ProjectContextVersion, contract.AssetSourceUpload, session.Quarantine,
		"", "", session.TraceID)
	if err != nil {
		code := "ASSET_INTAKE_FAILED"
		if errors.Is(err, ErrMalwareDetected) {
			code = contract.ErrorAssetQuarantined
		}
		if errors.Is(err, ErrMalwareDetected) || errors.Is(err, ErrInvalidAssetContent) {
			_ = s.Repository.FailUpload(ctx, session.OrganizationID, session.ProjectID, session.ID, code, s.now())
		}
		return UploadSession{}, err
	}
	if commit.MIMEType != session.DeclaredMIMEType || commit.SizeBytes != session.DeclaredSizeBytes {
		_ = s.Blobs.Delete(ctx, commit.Location)
		_ = s.Repository.FailUpload(ctx, session.OrganizationID, session.ProjectID, session.ID, "OUTPUT_METADATA_MISMATCH", s.now())
		return UploadSession{}, ErrOutputMetadataMismatch
	}
	if session.DeclaredSHA256 != nil && commit.SHA256 != *session.DeclaredSHA256 {
		_ = s.Blobs.Delete(ctx, commit.Location)
		_ = s.Repository.FailUpload(ctx, session.OrganizationID, session.ProjectID, session.ID, "ASSET_CHECKSUM_MISMATCH", s.now())
		return UploadSession{}, ErrAssetChecksumMismatch
	}
	ref, err := s.Repository.CompleteUpload(ctx, session.ID, commit, s.now())
	if err != nil {
		_ = s.Blobs.Delete(ctx, commit.Location)
		return UploadSession{}, err
	}
	_ = s.Blobs.Delete(ctx, session.Quarantine)
	session.Status = UploadSucceeded
	session.ProjectAssetRef = &ref
	session.UpdatedAt = s.now()
	return session, nil
}

func (s UploadService) List(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]ProjectAsset, error) {
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.Repository.ListProjectAssets(ctx, actor.OrganizationID, projectID, limit)
}

func (s UploadService) Preview(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, ref contract.AssetVersionRef) (SignedRequest, error) {
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return SignedRequest{}, err
	}
	if err := ref.Validate(); err != nil {
		return SignedRequest{}, err
	}
	asset, err := s.Repository.GetProjectAsset(ctx, actor.OrganizationID, projectID, ref)
	if err != nil {
		return SignedRequest{}, err
	}
	if asset.Version.Status != AssetReady {
		return SignedRequest{}, ErrAssetNotReady
	}
	return s.Blobs.SignGet(ctx, asset.Version.Blob, s.previewTTL())
}

// OpenPreview re-authorizes the project relationship before streaming local
// content. It is used only when the configured blob store cannot issue an
// externally reachable signed URL.
func (s UploadService) OpenPreview(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, ref contract.AssetVersionRef) (io.ReadCloser, ObjectInfo, error) {
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return nil, ObjectInfo{}, err
	}
	if err := ref.Validate(); err != nil {
		return nil, ObjectInfo{}, err
	}
	asset, err := s.Repository.GetProjectAsset(ctx, actor.OrganizationID, projectID, ref)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	if asset.Version.Status != AssetReady {
		return nil, ObjectInfo{}, ErrAssetNotReady
	}
	reader, info, err := s.Blobs.Open(ctx, asset.Version.Blob)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	if info.SizeBytes != asset.Version.SizeBytes {
		reader.Close()
		return nil, ObjectInfo{}, ErrOutputMetadataMismatch
	}
	info.MIMEType = asset.Version.MIMEType
	return reader, info, nil
}

func (s UploadService) Remove(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, ref contract.AssetVersionRef) error {
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return err
	}
	if err := ref.Validate(); err != nil {
		return err
	}
	return s.Repository.RemoveProjectAsset(ctx, actor.OrganizationID, projectID, ref)
}

func (s UploadService) ingestStoredObject(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, assetID contract.AssetID, blobID string, projectContextVersion int64, source contract.AssetSourceType, sourceLocation ObjectLocation, providerJobID, providerOutputID, traceID string) (AssetCommit, error) {
	reader, info, err := s.Blobs.Open(ctx, sourceLocation)
	if err != nil {
		return AssetCommit{}, err
	}
	defer reader.Close()
	if info.SizeBytes < 1 || info.SizeBytes > MaxImageBytes {
		return AssetCommit{}, fmt.Errorf("%w: size is outside the supported image range", ErrInvalidAssetContent)
	}
	data, err := io.ReadAll(io.LimitReader(reader, MaxImageBytes+1))
	if err != nil {
		return AssetCommit{}, err
	}
	if int64(len(data)) != info.SizeBytes || int64(len(data)) > MaxImageBytes {
		return AssetCommit{}, fmt.Errorf("%w: size changed while reading", ErrInvalidAssetContent)
	}
	mimeType := http.DetectContentType(data[:min(len(data), 512)])
	if !allowedDeclaredImageMIME(mimeType) {
		return AssetCommit{}, fmt.Errorf("%w: detected content is not a supported image", ErrInvalidAssetContent)
	}
	imageConfig, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return AssetCommit{}, fmt.Errorf("%w: decode image metadata: %v", ErrInvalidAssetContent, err)
	}
	if imageConfig.Width < 1 || imageConfig.Height < 1 {
		return AssetCommit{}, fmt.Errorf("%w: decoded image dimensions are invalid", ErrInvalidAssetContent)
	}
	if imageConfig.Width > MaxImageDimension || imageConfig.Height > MaxImageDimension || int64(imageConfig.Width)*int64(imageConfig.Height) > MaxImagePixels {
		return AssetCommit{}, fmt.Errorf("%w: decoded image dimensions exceed safety limits", ErrInvalidAssetContent)
	}
	if err := s.Scanner.Scan(ctx, bytes.NewReader(data)); err != nil {
		if errors.Is(err, ErrMalwareDetected) {
			return AssetCommit{}, err
		}
		return AssetCommit{}, fmt.Errorf("scan asset content: %w", err)
	}
	digest := sha256.Sum256(data)
	sha256Value := hex.EncodeToString(digest[:])
	durableKey := fmt.Sprintf("assets/%s/%s/versions/1/original", organizationID, assetID)
	durable, err := s.Blobs.Put(ctx, s.AssetsBucket, durableKey, bytes.NewReader(data), int64(len(data)), mimeType)
	if err != nil {
		return AssetCommit{}, err
	}
	eventID, err := s.idGenerator()("event")
	if err != nil {
		_ = s.Blobs.Delete(ctx, durable.ObjectLocation)
		return AssetCommit{}, err
	}
	version := int64(1)
	eventData, err := json.Marshal(contract.AssetReadyData{
		AssetKind: contract.AssetImage, MIMEType: mimeType, SizeBytes: int64(len(data)), SourceType: source,
		ProviderJobID: providerJobID, OutputID: providerOutputID,
	})
	if err != nil {
		_ = s.Blobs.Delete(ctx, durable.ObjectLocation)
		return AssetCommit{}, err
	}
	event := contract.EventEnvelope{
		EventID: eventID, EventType: "asset.ready.v1", OccurredAt: s.now(), Producer: "assets",
		OrganizationID: organizationID, ProjectID: projectID,
		Subject: contract.Subject{Type: "asset_version", ID: string(assetID), Version: &version},
		Data:    eventData, TraceID: traceID,
	}
	if err := event.Validate(); err != nil {
		_ = s.Blobs.Delete(ctx, durable.ObjectLocation)
		return AssetCommit{}, err
	}
	return AssetCommit{
		BlobID: blobID, OrganizationID: organizationID, ProjectID: projectID, AssetID: assetID, Version: 1,
		Kind: contract.AssetImage, SourceType: source, OwnerSystem: "assets", MIMEType: mimeType,
		SizeBytes: int64(len(data)), SHA256: sha256Value, WidthPixels: imageConfig.Width, HeightPixels: imageConfig.Height,
		ProviderJobID: providerJobID, ProviderOutputID: providerOutputID, ProjectContextVersion: projectContextVersion,
		Location: durable.ObjectLocation, Event: event,
	}, nil
}

func (s UploadService) validateDependencies() error {
	if s.Repository == nil || s.Projects == nil || s.Blobs == nil || s.Scanner == nil || s.QuarantineBucket == "" || s.AssetsBucket == "" {
		return fmt.Errorf("upload service dependencies are incomplete")
	}
	return nil
}

func (s UploadService) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s UploadService) idGenerator() ids.Generator {
	if s.NewID != nil {
		return s.NewID
	}
	return ids.New
}

func (s UploadService) uploadTTL() time.Duration {
	if s.UploadTTL <= 0 {
		return 15 * time.Minute
	}
	return s.UploadTTL
}

func (s UploadService) previewTTL() time.Duration {
	if s.PreviewTTL <= 0 {
		return 5 * time.Minute
	}
	return s.PreviewTTL
}

func (s UploadService) BlobProvider() string {
	switch s.Blobs.(type) {
	case *TOSBlobStore:
		return "tos"
	default:
		return "memory"
	}
}

func quarantineKey(organizationID contract.OrganizationID, sessionID string) string {
	return fmt.Sprintf("quarantine/%s/%s", organizationID, sessionID)
}

func safeFilename(value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	value = path.Base(value)
	value = strings.Map(func(character rune) rune {
		if character < 32 || character == 127 {
			return -1
		}
		return character
	}, value)
	if value == "." || value == "" {
		return "upload"
	}
	return value
}
