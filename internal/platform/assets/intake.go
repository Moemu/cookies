package assets

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// GeneratedOutputFetcher is owned by Assets and implemented by an injected
// Provider adapter. Implementations must authorize organization, project, job,
// and output ownership before returning bytes.
type GeneratedOutputFetcher interface {
	Open(context.Context, contract.ProviderOutputRef) (io.ReadCloser, contract.OutputMetadata, error)
}

// GeneratedAssetIntakeRequest admits exactly one successful provider output.
// Organization and project scope are taken from the trusted request context
// and project path, never from this payload.
type GeneratedAssetIntakeRequest struct {
	ProviderJobID string                     `json:"provider_job_id"`
	Output        contract.ProviderOutputRef `json:"output"`
	Provenance    GenerationProvenance       `json:"provenance"`
}

type GenerationProvenance struct {
	Capability            string                     `json:"capability"`
	ProviderCode          string                     `json:"provider_code"`
	ModelAlias            string                     `json:"model_alias"`
	ModelVersion          string                     `json:"model_version"`
	PromptRef             *contract.ResourceRef      `json:"prompt_ref"`
	SourceAssetRefs       []contract.AssetVersionRef `json:"source_asset_refs"`
	ProjectContextVersion int64                      `json:"project_context_version"`
	GeneratedAt           time.Time                  `json:"generated_at"`
}

type GeneratedIntakeStatus string

const (
	GeneratedIntakeQueued    GeneratedIntakeStatus = "queued"
	GeneratedIntakeRunning   GeneratedIntakeStatus = "running"
	GeneratedIntakeSucceeded GeneratedIntakeStatus = "succeeded"
	GeneratedIntakeFailed    GeneratedIntakeStatus = "failed"
)

type GeneratedIntake struct {
	ID              string                      `json:"id"`
	OrganizationID  contract.OrganizationID     `json:"organization_id"`
	ProjectID       contract.ProjectID          `json:"project_id"`
	ProviderJobID   string                      `json:"provider_job_id"`
	OutputID        string                      `json:"output_id"`
	ProviderCode    string                      `json:"provider_code"`
	Status          GeneratedIntakeStatus       `json:"status"`
	Request         GeneratedAssetIntakeRequest `json:"-"`
	IdempotencyKey  contract.IdempotencyKey     `json:"-"`
	RequestHash     string                      `json:"-"`
	TargetAssetID   contract.AssetID            `json:"-"`
	TargetBlobID    string                      `json:"-"`
	ProjectAssetRef *contract.ProjectAssetRef   `json:"project_asset_ref"`
	Error           *contract.JobError          `json:"error"`
	AttemptCount    int                         `json:"attempt_count"`
	MaxAttempts     int                         `json:"max_attempts"`
	AvailableAt     time.Time                   `json:"-"`
	LockOwner       string                      `json:"-"`
	RequestID       string                      `json:"-"`
	TraceID         string                      `json:"-"`
	CreatedAt       time.Time                   `json:"created_at"`
	UpdatedAt       time.Time                   `json:"updated_at"`
}

func (i GeneratedIntake) Response() GeneratedAssetIntakeResponse {
	return GeneratedAssetIntakeResponse{
		ID: i.ID, ProviderJobID: i.ProviderJobID, OutputID: i.OutputID, Status: i.Status,
		ProjectAssetRef: i.ProjectAssetRef, Error: i.Error,
	}
}

// GeneratedAssetIntakeResponse is returned by both the asynchronous create
// and query endpoints. ProjectAssetRef remains nil until all visibility gates
// for TOS, AssetVersion, and ProjectAssetRef have passed.
type GeneratedAssetIntakeResponse struct {
	ID              string                    `json:"id"`
	ProviderJobID   string                    `json:"provider_job_id"`
	OutputID        string                    `json:"output_id"`
	Status          GeneratedIntakeStatus     `json:"status"`
	ProjectAssetRef *contract.ProjectAssetRef `json:"project_asset_ref"`
	Error           *contract.JobError        `json:"error"`
}

func (r GeneratedAssetIntakeRequest) Validate() error {
	if strings.TrimSpace(r.ProviderJobID) == "" {
		return fmt.Errorf("provider_job_id is required")
	}
	if err := r.Output.Validate(); err != nil {
		return fmt.Errorf("invalid output: %w", err)
	}
	if r.Output.ProviderJobID != r.ProviderJobID {
		return fmt.Errorf("output provider_job_id must match request provider_job_id")
	}
	if err := r.Provenance.Validate(); err != nil {
		return fmt.Errorf("invalid provenance: %w", err)
	}
	if r.Provenance.ProviderCode != r.Output.ProviderCode {
		return fmt.Errorf("provenance provider_code must match output provider_code")
	}
	return nil
}

func (p GenerationProvenance) Validate() error {
	if strings.TrimSpace(p.Capability) == "" || strings.TrimSpace(p.ProviderCode) == "" {
		return fmt.Errorf("capability and provider_code are required")
	}
	if strings.TrimSpace(p.ModelAlias) == "" || strings.TrimSpace(p.ModelVersion) == "" {
		return fmt.Errorf("model_alias and model_version are required")
	}
	if p.ProjectContextVersion < 1 {
		return fmt.Errorf("project_context_version must be positive")
	}
	if p.SourceAssetRefs == nil {
		return fmt.Errorf("source_asset_refs must be an array")
	}
	if p.GeneratedAt.IsZero() {
		return fmt.Errorf("generated_at is required")
	}
	if p.PromptRef != nil {
		if err := p.PromptRef.Validate(); err != nil {
			return fmt.Errorf("invalid prompt_ref: %w", err)
		}
	}
	for index, assetRef := range p.SourceAssetRefs {
		if err := assetRef.Validate(); err != nil {
			return fmt.Errorf("invalid source_asset_ref at index %d: %w", index, err)
		}
	}
	return nil
}

func (r GeneratedAssetIntakeResponse) Validate() error {
	if strings.TrimSpace(r.ID) == "" || strings.TrimSpace(r.ProviderJobID) == "" || strings.TrimSpace(r.OutputID) == "" {
		return fmt.Errorf("intake ID, provider_job_id, and output_id are required")
	}
	switch r.Status {
	case GeneratedIntakeQueued, GeneratedIntakeRunning:
		if r.ProjectAssetRef != nil || r.Error != nil {
			return fmt.Errorf("pending intake cannot include a result or error")
		}
	case GeneratedIntakeSucceeded:
		if r.ProjectAssetRef == nil || r.Error != nil {
			return fmt.Errorf("succeeded intake requires one project asset and no error")
		}
		if err := r.ProjectAssetRef.Validate(); err != nil {
			return fmt.Errorf("invalid project_asset_ref: %w", err)
		}
	case GeneratedIntakeFailed:
		if r.ProjectAssetRef != nil || r.Error == nil || strings.TrimSpace(r.Error.Code) == "" {
			return fmt.Errorf("failed intake requires one stable error and no project asset")
		}
	default:
		return fmt.Errorf("generated intake status is invalid")
	}
	return nil
}
