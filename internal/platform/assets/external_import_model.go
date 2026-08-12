package assets

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// ExternalImportStatus is the durable ledger state for authorized third-party
// media imports. It is intentionally separate from generated model intakes.
type ExternalImportStatus string

const (
	ExternalImportQueued    ExternalImportStatus = "queued"
	ExternalImportRunning   ExternalImportStatus = "running"
	ExternalImportSucceeded ExternalImportStatus = "succeeded"
	ExternalImportFailed    ExternalImportStatus = "failed"
)

func (s ExternalImportStatus) valid() bool {
	switch s {
	case ExternalImportQueued, ExternalImportRunning, ExternalImportSucceeded, ExternalImportFailed:
		return true
	}
	return false
}

// ExternalImport records idempotency and result reconciliation only. The
// actual import service is introduced in a later slice; no provider-generation
// job or output identifiers are reused here.
type ExternalImport struct {
	ID                    string                  `json:"id"`
	OrganizationID        contract.OrganizationID `json:"organization_id"`
	ProjectID             contract.ProjectID      `json:"project_id"`
	SourceProvider        string                  `json:"source_provider"`
	SourceObjectID        string                  `json:"source_object_id"`
	SourceLocator         string                  `json:"source_locator,omitempty"`
	IdempotencyKey        string                  `json:"idempotency_key"`
	RequestSnapshot       json.RawMessage         `json:"request_snapshot"`
	RequestHash           string                  `json:"request_hash"`
	Status                ExternalImportStatus    `json:"status"`
	AttemptCount          int                     `json:"attempt_count"`
	ResultUnknownAt       *time.Time              `json:"result_unknown_at,omitempty"`
	ResultUnknownReason   string                  `json:"result_unknown_reason,omitempty"`
	RecoveryAttemptCount  int                     `json:"recovery_attempt_count"`
	LastErrorCode         string                  `json:"last_error_code,omitempty"`
	LastErrorMessage      string                  `json:"last_error_message,omitempty"`
	CommittedAssetID      contract.AssetID        `json:"committed_asset_id,omitempty"`
	CommittedAssetVersion int64                   `json:"committed_asset_version,omitempty"`
	Version               int64                   `json:"version"`
	CreatedAt             time.Time               `json:"created_at"`
	UpdatedAt             time.Time               `json:"updated_at"`
}

func (i ExternalImport) Validate() error {
	if strings.TrimSpace(i.ID) == "" || strings.TrimSpace(string(i.OrganizationID)) == "" ||
		strings.TrimSpace(string(i.ProjectID)) == "" || strings.TrimSpace(i.SourceProvider) == "" ||
		strings.TrimSpace(i.SourceObjectID) == "" || strings.TrimSpace(i.IdempotencyKey) == "" {
		return fmt.Errorf("external import identity, scope, source, and idempotency key are required")
	}
	if !i.Status.valid() {
		return fmt.Errorf("unsupported external import status %q", i.Status)
	}
	if len(i.RequestHash) != 64 {
		return fmt.Errorf("external import request hash must be a SHA-256 hex string")
	}
	if !json.Valid(i.RequestSnapshot) {
		return fmt.Errorf("external import request snapshot must be valid JSON")
	}
	if i.Version < 1 || i.AttemptCount < 0 || i.RecoveryAttemptCount < 0 || i.RecoveryAttemptCount > i.AttemptCount {
		return fmt.Errorf("external import version and attempt policy are invalid")
	}
	if i.CreatedAt.IsZero() || i.UpdatedAt.IsZero() || i.UpdatedAt.Before(i.CreatedAt) {
		return fmt.Errorf("external import timestamps are invalid")
	}
	if (i.ResultUnknownAt == nil) != (strings.TrimSpace(i.ResultUnknownReason) == "") {
		return fmt.Errorf("external import result-unknown time and reason must be supplied together")
	}
	if (i.CommittedAssetID == "") != (i.CommittedAssetVersion == 0) {
		return fmt.Errorf("external import asset ID and version must be supplied together")
	}
	if i.CommittedAssetVersion < 0 {
		return fmt.Errorf("external import asset version must not be negative")
	}
	if i.Status == ExternalImportSucceeded && i.CommittedAssetID == "" {
		return fmt.Errorf("successful external import requires an asset result")
	}
	if i.Status == ExternalImportFailed && strings.TrimSpace(i.LastErrorCode) == "" {
		return fmt.Errorf("failed external import requires an error code")
	}
	return nil
}
