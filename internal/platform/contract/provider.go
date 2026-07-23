package contract

import (
	"fmt"
	"strings"
	"time"
)

type ProviderJobStatus string

const (
	ProviderJobSubmitted          ProviderJobStatus = "submitted"
	ProviderJobRunning            ProviderJobStatus = "running"
	ProviderJobOutputsReady       ProviderJobStatus = "outputs_ready"
	ProviderJobIngesting          ProviderJobStatus = "ingesting"
	ProviderJobSucceeded          ProviderJobStatus = "succeeded"
	ProviderJobPartiallySucceeded ProviderJobStatus = "partially_succeeded"
	ProviderJobFailed             ProviderJobStatus = "failed"
	ProviderJobCancelled          ProviderJobStatus = "cancelled"
	ProviderJobExpired            ProviderJobStatus = "expired"
)

// ProviderJob keeps provider-domain progress separate from the generic
// execution state used by the shared worker runtime.
type ProviderJob struct {
	ID               string            `json:"id"`
	Kind             string            `json:"kind"`
	OrganizationID   OrganizationID    `json:"organization_id"`
	ProjectID        ProjectID         `json:"project_id"`
	ExecutionStatus  JobStatus         `json:"execution_status"`
	ProviderStatus   ProviderJobStatus `json:"provider_status"`
	Progress         int               `json:"progress"`
	ProjectAssetRefs []ProjectAssetRef `json:"project_asset_refs"`
	Error            *JobError         `json:"error"`
	AttemptCount     int               `json:"attempt_count"`
	MaxAttempts      int               `json:"max_attempts"`
	Version          int64             `json:"version"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

func (j ProviderJob) Validate() error {
	if strings.TrimSpace(j.ID) == "" || strings.TrimSpace(j.Kind) == "" {
		return fmt.Errorf("provider job ID and kind are required")
	}
	if strings.TrimSpace(string(j.OrganizationID)) == "" || strings.TrimSpace(string(j.ProjectID)) == "" {
		return fmt.Errorf("organization_id and project_id are required")
	}
	if !j.ExecutionStatus.valid() || !j.ProviderStatus.valid() {
		return fmt.Errorf("provider job status is invalid")
	}
	if j.Progress < 0 || j.Progress > 100 {
		return fmt.Errorf("provider job progress must be between 0 and 100")
	}
	if j.ProjectAssetRefs == nil {
		return fmt.Errorf("project_asset_refs must be an array")
	}
	if j.AttemptCount < 0 || j.MaxAttempts < 1 || j.AttemptCount > j.MaxAttempts {
		return fmt.Errorf("provider job attempts are invalid")
	}
	if j.Version < 1 || j.CreatedAt.IsZero() || j.UpdatedAt.IsZero() || j.UpdatedAt.Before(j.CreatedAt) {
		return fmt.Errorf("provider job version or timestamps are invalid")
	}
	for index, ref := range j.ProjectAssetRefs {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("invalid project_asset_ref at index %d: %w", index, err)
		}
		if ref.ProjectID != j.ProjectID {
			return fmt.Errorf("project_asset_ref at index %d belongs to another project", index)
		}
	}
	return nil
}

func (s ProviderJobStatus) valid() bool {
	switch s {
	case ProviderJobSubmitted, ProviderJobRunning, ProviderJobOutputsReady, ProviderJobIngesting,
		ProviderJobSucceeded, ProviderJobPartiallySucceeded, ProviderJobFailed,
		ProviderJobCancelled, ProviderJobExpired:
		return true
	default:
		return false
	}
}

// ProviderOutputRef is an opaque, authorized handle. It never exposes a
// vendor URL or storage key to Assets.
type ProviderOutputRef struct {
	ProviderCode       string    `json:"provider_code"`
	ProviderJobID      string    `json:"provider_job_id"`
	OutputID           string    `json:"output_id"`
	RetrievalExpiresAt time.Time `json:"retrieval_expires_at"`
	DeclaredMIMEType   string    `json:"declared_mime_type"`
	DeclaredSizeBytes  int64     `json:"declared_size_bytes"`
	DeclaredSHA256     *string   `json:"declared_sha256"`
}

func (r ProviderOutputRef) Validate() error {
	if strings.TrimSpace(r.ProviderCode) == "" || strings.TrimSpace(r.ProviderJobID) == "" || strings.TrimSpace(r.OutputID) == "" {
		return fmt.Errorf("provider_code, provider_job_id, and output_id are required")
	}
	if r.RetrievalExpiresAt.IsZero() {
		return fmt.Errorf("retrieval_expires_at is required")
	}
	if strings.TrimSpace(r.DeclaredMIMEType) == "" || r.DeclaredSizeBytes < 1 {
		return fmt.Errorf("declared MIME type and positive size are required")
	}
	if r.DeclaredSHA256 != nil && !validSHA256(*r.DeclaredSHA256) {
		return fmt.Errorf("declared_sha256 must be a lowercase hexadecimal SHA-256 digest")
	}
	return nil
}

type OutputMetadata struct {
	MIMEType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

func (m OutputMetadata) Validate() error {
	if strings.TrimSpace(m.MIMEType) == "" || m.SizeBytes < 1 || !validSHA256(m.SHA256) {
		return fmt.Errorf("output metadata requires MIME type, positive size, and SHA-256")
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}
