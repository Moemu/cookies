package assets

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const MaxImageBytes int64 = 20 * 1024 * 1024
const MaxImageDimension = 16384
const MaxImagePixels int64 = 100_000_000

type AssetStatus string

const (
	AssetProcessing  AssetStatus = "processing"
	AssetReady       AssetStatus = "ready"
	AssetQuarantined AssetStatus = "quarantined"
	AssetFailed      AssetStatus = "failed"
	AssetArchived    AssetStatus = "archived"
)

type Asset struct {
	ID             contract.AssetID        `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	Kind           contract.AssetKind      `json:"asset_kind"`
	Status         AssetStatus             `json:"status"`
	OwnerSystem    string                  `json:"owner_system"`
	LatestVersion  int64                   `json:"latest_version"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

func validSHA256(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

type AssetVersion struct {
	OrganizationID        contract.OrganizationID  `json:"organization_id"`
	AssetID               contract.AssetID         `json:"asset_id"`
	Version               int64                    `json:"version"`
	Status                AssetStatus              `json:"status"`
	SourceType            contract.AssetSourceType `json:"source_type"`
	MIMEType              string                   `json:"mime_type"`
	SizeBytes             int64                    `json:"size_bytes"`
	SHA256                string                   `json:"sha256"`
	WidthPixels           int                      `json:"width_pixels,omitempty"`
	HeightPixels          int                      `json:"height_pixels,omitempty"`
	ProviderJobID         string                   `json:"provider_job_id,omitempty"`
	ProviderOutputID      string                   `json:"provider_output_id,omitempty"`
	ProjectContextVersion int64                    `json:"project_context_version,omitempty"`
	CreatedAt             time.Time                `json:"created_at"`
	Blob                  ObjectLocation           `json:"-"`
}

func (v AssetVersion) Ref() contract.AssetVersionRef {
	return contract.AssetVersionRef{AssetID: v.AssetID, Version: v.Version}
}

type ProjectAsset struct {
	Ref       contract.ProjectAssetRef `json:"ref"`
	Asset     Asset                    `json:"asset"`
	Version   AssetVersion             `json:"version"`
	CreatedAt time.Time                `json:"created_at"`
}

type UploadStatus string

const (
	UploadCreated    UploadStatus = "created"
	UploadUploaded   UploadStatus = "uploaded"
	UploadProcessing UploadStatus = "processing"
	UploadSucceeded  UploadStatus = "succeeded"
	UploadFailed     UploadStatus = "failed"
	UploadExpired    UploadStatus = "expired"
)

type CreateUploadRequest struct {
	Filename          string  `json:"filename"`
	DeclaredMIMEType  string  `json:"declared_mime_type"`
	DeclaredSizeBytes int64   `json:"declared_size_bytes"`
	DeclaredSHA256    *string `json:"declared_sha256"`
}

func (r CreateUploadRequest) Validate() error {
	if strings.TrimSpace(r.Filename) == "" || len(r.Filename) > 512 {
		return fmt.Errorf("filename must be between 1 and 512 characters")
	}
	if !allowedDeclaredImageMIME(r.DeclaredMIMEType) {
		return fmt.Errorf("declared_mime_type must be image/jpeg or image/png")
	}
	if r.DeclaredSizeBytes < 1 || r.DeclaredSizeBytes > MaxImageBytes {
		return fmt.Errorf("declared_size_bytes must be between 1 and %d", MaxImageBytes)
	}
	if r.DeclaredSHA256 != nil && !validSHA256(*r.DeclaredSHA256) {
		return fmt.Errorf("declared_sha256 must be a lowercase hexadecimal SHA-256 digest")
	}
	return nil
}

type UploadSession struct {
	ID                    string                    `json:"id"`
	OrganizationID        contract.OrganizationID   `json:"organization_id"`
	ProjectID             contract.ProjectID        `json:"project_id"`
	Principal             contract.Principal        `json:"principal"`
	Status                UploadStatus              `json:"status"`
	Filename              string                    `json:"filename"`
	DeclaredMIMEType      string                    `json:"declared_mime_type"`
	DeclaredSizeBytes     int64                     `json:"declared_size_bytes"`
	DeclaredSHA256        *string                   `json:"declared_sha256"`
	Quarantine            ObjectLocation            `json:"-"`
	IdempotencyKey        contract.IdempotencyKey   `json:"-"`
	RequestHash           string                    `json:"-"`
	ProjectContextVersion int64                     `json:"project_context_version"`
	TargetAssetID         contract.AssetID          `json:"-"`
	TargetBlobID          string                    `json:"-"`
	RequestID             string                    `json:"-"`
	TraceID               string                    `json:"-"`
	ProjectAssetRef       *contract.ProjectAssetRef `json:"project_asset_ref"`
	ErrorCode             string                    `json:"error_code,omitempty"`
	ExpiresAt             time.Time                 `json:"expires_at"`
	CreatedAt             time.Time                 `json:"created_at"`
	UpdatedAt             time.Time                 `json:"updated_at"`
}

type CreateUploadResponse struct {
	Session UploadSession  `json:"session"`
	Upload  *SignedRequest `json:"upload"`
}

type AssetCommit struct {
	BlobID                string
	OrganizationID        contract.OrganizationID
	ProjectID             contract.ProjectID
	AssetID               contract.AssetID
	Version               int64
	Kind                  contract.AssetKind
	SourceType            contract.AssetSourceType
	OwnerSystem           string
	MIMEType              string
	SizeBytes             int64
	SHA256                string
	WidthPixels           int
	HeightPixels          int
	ProviderJobID         string
	ProviderOutputID      string
	ProjectContextVersion int64
	Location              ObjectLocation
	Event                 contract.EventEnvelope
}

func allowedDeclaredImageMIME(value string) bool {
	return value == "image/jpeg" || value == "image/png"
}
