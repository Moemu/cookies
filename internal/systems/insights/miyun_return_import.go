package insights

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type MiyunHandoffReturnStatus string

const (
	MiyunHandoffReturnCreated  MiyunHandoffReturnStatus = "created"
	MiyunHandoffReturnUploaded MiyunHandoffReturnStatus = "uploaded"
	MiyunHandoffReturnFailed   MiyunHandoffReturnStatus = "failed"
	MiyunHandoffReturnReturned MiyunHandoffReturnStatus = "returned"
)

func (s MiyunHandoffReturnStatus) valid() bool {
	return s == MiyunHandoffReturnCreated || s == MiyunHandoffReturnUploaded || s == MiyunHandoffReturnFailed || s == MiyunHandoffReturnReturned
}

type CreateMiyunHandoffReturnRequest struct {
	ExpectedVersion int64 `json:"expected_version"`
}
type UploadMiyunHandoffReturnRequest struct {
	ExpectedVersion   int64
	Filename          string
	DeclaredMIMEType  string
	DeclaredSizeBytes int64
	DeclaredSHA256    *string
	Content           io.Reader
}

// MiyunHandoffReturn is append-only business lineage; it deliberately keeps
// the source handoff snapshots immutable instead of copying their mutable data.
type MiyunHandoffReturn struct {
	ID                   string                   `json:"id"`
	OrganizationID       contract.OrganizationID  `json:"organization_id"`
	ProjectID            contract.ProjectID       `json:"project_id"`
	HandoffID            string                   `json:"handoff_id"`
	HandoffVersion       int64                    `json:"handoff_version"`
	ManifestVersion      string                   `json:"manifest_version"`
	InputHash            string                   `json:"input_hash"`
	ParameterVersion     string                   `json:"parameter_version"`
	ProductProfileID     string                   `json:"product_profile_id"`
	Status               MiyunHandoffReturnStatus `json:"status"`
	IdempotencyKey       string                   `json:"-"`
	RequestHash          string                   `json:"-"`
	UploadIdempotencyKey string                   `json:"-"`
	UploadRequestHash    string                   `json:"-"`
	Filename             string                   `json:"filename,omitempty"`
	AssetVersion         contract.AssetVersionRef `json:"asset_version,omitempty"`
	MIMEType             string                   `json:"mime_type,omitempty"`
	SHA256               string                   `json:"sha256,omitempty"`
	SizeBytes            int64                    `json:"size_bytes,omitempty"`
	InsightAssetID       string                   `json:"insight_asset_id,omitempty"`
	UploadedBy           string                   `json:"uploaded_by"`
	UploadedAt           *time.Time               `json:"uploaded_at,omitempty"`
	FailureCode          string                   `json:"failure_code,omitempty"`
	MarkIdempotencyKey   string                   `json:"-"`
	MarkRequestHash      string                   `json:"-"`
	ReturnedBy           string                   `json:"returned_by,omitempty"`
	ReturnedAt           *time.Time               `json:"returned_at,omitempty"`
	Version              int64                    `json:"version"`
	CreatedAt            time.Time                `json:"created_at"`
	UpdatedAt            time.Time                `json:"updated_at"`
}

func (r MiyunHandoffReturn) Validate() error {
	if strings.TrimSpace(r.ID) == "" || r.OrganizationID == "" || r.ProjectID == "" || strings.TrimSpace(r.HandoffID) == "" || r.HandoffVersion < 1 ||
		strings.TrimSpace(r.ManifestVersion) == "" || strings.TrimSpace(r.ParameterVersion) == "" || strings.TrimSpace(r.ProductProfileID) == "" ||
		!miyunReturnImportSHA256Valid(r.InputHash) || !r.Status.valid() || strings.TrimSpace(r.IdempotencyKey) == "" || !miyunReturnImportSHA256Valid(r.RequestHash) || r.Version < 1 || r.CreatedAt.IsZero() || r.UpdatedAt.Before(r.CreatedAt) {
		return fmt.Errorf("%w: return identity, frozen lineage, idempotency, and version are required", ErrInvalidRequest)
	}
	if r.Status == MiyunHandoffReturnCreated {
		return nil
	}
	if r.Status == MiyunHandoffReturnFailed {
		if strings.TrimSpace(r.FailureCode) == "" {
			return fmt.Errorf("%w: failed return requires a failure code", ErrInvalidRequest)
		}
		return nil
	}
	if err := (MiyunReturnImportInput{HandoffID: r.HandoffID, ManifestInputHash: r.InputHash, Filename: r.Filename, AssetVersion: r.AssetVersion, MIMEType: r.MIMEType, SHA256: r.SHA256, SizeBytes: r.SizeBytes, ScanPassed: true, ProbePassed: true}).Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.UploadIdempotencyKey) == "" || !miyunReturnImportSHA256Valid(r.UploadRequestHash) {
		return fmt.Errorf("%w: uploaded return requires upload idempotency audit", ErrInvalidRequest)
	}
	if r.Status == MiyunHandoffReturnReturned && (r.ReturnedAt == nil || strings.TrimSpace(r.ReturnedBy) == "" || strings.TrimSpace(r.MarkIdempotencyKey) == "" || !miyunReturnImportSHA256Valid(r.MarkRequestHash)) {
		return fmt.Errorf("%w: returned record requires mark audit", ErrInvalidRequest)
	}
	return nil
}

type MiyunReturnAssetImportRequest struct {
	ReturnID          string
	Filename          string
	DeclaredMIMEType  string
	DeclaredSizeBytes int64
	DeclaredSHA256    *string
	Content           io.Reader
	SourceResources   []contract.ResourceRef
}
type MiyunReturnAssetImportResult struct {
	AssetVersion     contract.AssetVersionRef
	MIMEType, SHA256 string
	SizeBytes        int64
}
type MiyunReturnAssetImporter interface {
	ImportMiyunReturnMP4(context.Context, contract.RequestContext, contract.ProjectID, contract.IdempotencyKey, MiyunReturnAssetImportRequest) (MiyunReturnAssetImportResult, error)
}

// MiyunReturnImportMIMEType is deliberately the only v1 return format. A
// handoff ZIP and its manifest are export artifacts, never return inputs.
const MiyunReturnImportMIMEType = "video/mp4"

// MiyunReturnImportInput is the already-uploaded Assets result which is safe
// to attach to a Miyun handoff. The application service must obtain it only
// after Assets has uploaded, scanned, and probed the object; this value never
// contains an object-store location or content.
type MiyunReturnImportInput struct {
	HandoffID         string                   `json:"handoff_id"`
	ManifestInputHash string                   `json:"manifest_input_hash"`
	Filename          string                   `json:"filename"`
	AssetVersion      contract.AssetVersionRef `json:"asset_version"`
	MIMEType          string                   `json:"mime_type"`
	SHA256            string                   `json:"sha256"`
	SizeBytes         int64                    `json:"size_bytes"`
	ScanPassed        bool                     `json:"scan_passed"`
	ProbePassed       bool                     `json:"probe_passed"`
}

// Validate accepts exactly one scanned and probed MP4 AssetVersion. In
// particular, it rejects ZIPs and manifests rather than attempting to unpack
// or interpret them.
func (i MiyunReturnImportInput) Validate() error {
	if strings.TrimSpace(i.HandoffID) == "" || !miyunReturnImportSHA256Valid(i.ManifestInputHash) {
		return fmt.Errorf("%w: handoff ID and manifest input SHA-256 are required", ErrInvalidRequest)
	}
	if err := i.AssetVersion.Validate(); err != nil {
		return fmt.Errorf("%w: invalid return AssetVersion: %v", ErrInvalidRequest, err)
	}
	if strings.TrimSpace(i.Filename) == "" || !strings.EqualFold(filepath.Ext(i.Filename), ".mp4") ||
		i.MIMEType != MiyunReturnImportMIMEType {
		return fmt.Errorf("%w: Miyun return v1 accepts one MP4 only", ErrInvalidRequest)
	}
	if !miyunReturnImportSHA256Valid(i.SHA256) || i.SizeBytes < 1 {
		return fmt.Errorf("%w: return MP4 requires a valid SHA-256 and positive size", ErrInvalidRequest)
	}
	if !i.ScanPassed || !i.ProbePassed {
		return fmt.Errorf("%w: return MP4 must complete Assets scanning and probing", ErrInvalidState)
	}
	return nil
}

// MiyunReturnImportStatus records the result of a return attempt. A failed
// attempt never changes the handoff to returned; a later retry is a new
// attempt against the same exported/delivered handoff.
type MiyunReturnImportStatus string

const (
	MiyunReturnImportReturned MiyunReturnImportStatus = "returned"
	MiyunReturnImportFailed   MiyunReturnImportStatus = "failed"
)

func (s MiyunReturnImportStatus) valid() bool {
	return s == MiyunReturnImportReturned || s == MiyunReturnImportFailed
}

// MiyunReturnImportRecord is the persistence DTO for a return attempt. On a
// successful return it freezes the exact Assets version and byte identity,
// together with the originating handoff/manifest input and operator.
type MiyunReturnImportRecord struct {
	Status            MiyunReturnImportStatus  `json:"status"`
	HandoffID         string                   `json:"handoff_id"`
	ManifestInputHash string                   `json:"manifest_input_hash"`
	Filename          string                   `json:"filename,omitempty"`
	AssetVersion      contract.AssetVersionRef `json:"asset_version,omitempty"`
	MIMEType          string                   `json:"mime_type,omitempty"`
	SHA256            string                   `json:"sha256,omitempty"`
	SizeBytes         int64                    `json:"size_bytes,omitempty"`
	ReturnedBy        string                   `json:"returned_by"`
	ReturnedAt        *time.Time               `json:"returned_at,omitempty"`
	FailureCode       string                   `json:"failure_code,omitempty"`
}

func (r MiyunReturnImportRecord) Validate() error {
	if !r.Status.valid() || strings.TrimSpace(r.HandoffID) == "" ||
		!miyunReturnImportSHA256Valid(r.ManifestInputHash) || strings.TrimSpace(r.ReturnedBy) == "" {
		return fmt.Errorf("%w: return result, handoff lineage, manifest input hash, and operator are required", ErrInvalidRequest)
	}
	inputPresent := r.Filename != "" || r.AssetVersion.AssetID != "" || r.AssetVersion.Version != 0 ||
		r.MIMEType != "" || r.SHA256 != "" || r.SizeBytes != 0
	if r.Status == MiyunReturnImportReturned {
		if r.ReturnedAt == nil || strings.TrimSpace(r.FailureCode) != "" {
			return fmt.Errorf("%w: returned result requires a timestamp and no failure", ErrInvalidRequest)
		}
		return (MiyunReturnImportInput{HandoffID: r.HandoffID, ManifestInputHash: r.ManifestInputHash, Filename: r.Filename,
			AssetVersion: r.AssetVersion, MIMEType: r.MIMEType, SHA256: r.SHA256, SizeBytes: r.SizeBytes,
			ScanPassed: true, ProbePassed: true}).Validate()
	}
	if strings.TrimSpace(r.FailureCode) == "" || r.ReturnedAt != nil {
		return fmt.Errorf("%w: failed result requires a failure code and cannot be returned", ErrInvalidRequest)
	}
	if inputPresent {
		return (MiyunReturnImportInput{HandoffID: r.HandoffID, ManifestInputHash: r.ManifestInputHash, Filename: r.Filename,
			AssetVersion: r.AssetVersion, MIMEType: r.MIMEType, SHA256: r.SHA256, SizeBytes: r.SizeBytes,
			ScanPassed: true, ProbePassed: true}).Validate()
	}
	return nil
}

// MiyunHandoffStatusAfterReturn returns the only permitted handoff state
// update. Failed attempts intentionally preserve the current state, allowing
// recovery by retry without ever marking a failure as returned.
func MiyunHandoffStatusAfterReturn(current MiyunHandoffStatus, result MiyunReturnImportStatus) (MiyunHandoffStatus, error) {
	if !result.valid() {
		return "", fmt.Errorf("%w: unsupported Miyun return result %q", ErrInvalidRequest, result)
	}
	if result == MiyunReturnImportFailed {
		return current, nil
	}
	if current != MiyunHandoffExported && current != MiyunHandoffDelivered {
		return "", fmt.Errorf("%w: only exported or delivered handoffs may be returned", ErrInvalidState)
	}
	return MiyunHandoffReturned, nil
}

func miyunReturnImportSHA256Valid(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
