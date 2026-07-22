// Package provider owns capability-based model execution. It deliberately
// knows nothing about Assets persistence or vendor SDK request types.
package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const ScopeJobCreate contract.Scope = "provider.job.create"

const imageJobKind = "provider.image.generate"
const imageOperation = "image.generate"

// ImageGenerationInput is the stable Provider-owned input for image creation.
// Prompt contents may be persisted in protected Provider storage, but callers
// must never place them in events or ordinary logs.
type ImageGenerationInput struct {
	Prompt string
	Width  int
	Height int
}

func (i ImageGenerationInput) Validate() error {
	if strings.TrimSpace(i.Prompt) == "" {
		return fmt.Errorf("image prompt is required")
	}
	if i.Width < 1 || i.Height < 1 {
		return fmt.Errorf("image dimensions must be positive")
	}
	return nil
}

// CreateImageJobRequest is the Provider application's input seam. The
// handler obtains Actor from trusted identity and Project from an authorized
// Project module projection before calling this method.
type CreateImageJobRequest struct {
	Actor          contract.ActorContext
	Project        contract.ProjectContext
	IdempotencyKey contract.IdempotencyKey
	RequestHash    string
	ModelAlias     string
	SourceSystem   string
	SourceTaskID   string
	Input          ImageGenerationInput
}

func (r CreateImageJobRequest) Validate() error {
	if err := r.Actor.Validate(); err != nil {
		return fmt.Errorf("invalid actor: %w", err)
	}
	if !r.Actor.HasScope(ScopeJobCreate) {
		return fmt.Errorf("%s scope is required", ScopeJobCreate)
	}
	if err := r.Project.ValidateBrandBound(); err != nil {
		return fmt.Errorf("invalid project for image generation: %w", err)
	}
	if r.Project.OrganizationID != r.Actor.OrganizationID {
		return fmt.Errorf("project organization does not match actor organization")
	}
	if err := r.IdempotencyKey.Validate(); err != nil {
		return err
	}
	if !validSHA256(r.RequestHash) {
		return fmt.Errorf("request hash must be a lowercase hexadecimal SHA-256 digest")
	}
	if strings.TrimSpace(r.ModelAlias) == "" {
		return fmt.Errorf("model alias is required")
	}
	if err := r.Input.Validate(); err != nil {
		return err
	}
	return nil
}

// JobRecord is Provider's private durable state. ProjectContextVersion,
// principal and prompt data are retained here, not in the public ProviderJob
// response or cross-module events.
type JobRecord struct {
	Job                   contract.ProviderJob
	Principal             contract.Principal
	Operation             string
	IdempotencyKey        contract.IdempotencyKey
	RequestHash           string
	ProjectContextVersion int64
	ModelAlias            string
	SourceSystem          string
	SourceTaskID          string
	Input                 ImageGenerationInput
}

// JobStore owns ProviderJob durability and Provider-specific idempotency. It
// intentionally does not reuse platform_jobs' narrower idempotency scope.
type JobStore interface {
	Create(ctx context.Context, record JobRecord) (stored JobRecord, duplicate bool, err error)
}

// Service is the small application seam used by transport and workers.
type Service struct {
	Store JobStore
	NewID func() string
	Now   func() time.Time
}

func (s Service) CreateImageJob(ctx context.Context, request CreateImageJobRequest) (contract.ProviderJob, bool, error) {
	if s.Store == nil {
		return contract.ProviderJob{}, false, fmt.Errorf("provider job store is required")
	}
	if s.NewID == nil {
		return contract.ProviderJob{}, false, fmt.Errorf("provider job ID generator is required")
	}
	if err := request.Validate(); err != nil {
		return contract.ProviderJob{}, false, err
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	createdAt := now().UTC()
	job := contract.ProviderJob{
		ID:               s.NewID(),
		Kind:             imageJobKind,
		OrganizationID:   request.Actor.OrganizationID,
		ProjectID:        request.Project.ProjectID,
		ExecutionStatus:  contract.JobQueued,
		ProviderStatus:   contract.ProviderJobSubmitted,
		Progress:         0,
		ProjectAssetRefs: []contract.ProjectAssetRef{},
		AttemptCount:     0,
		MaxAttempts:      3,
		Version:          1,
		CreatedAt:        createdAt,
		UpdatedAt:        createdAt,
	}
	if err := job.Validate(); err != nil {
		return contract.ProviderJob{}, false, fmt.Errorf("create provider job: %w", err)
	}
	stored, duplicate, err := s.Store.Create(ctx, JobRecord{
		Job:                   job,
		Principal:             request.Actor.Principal,
		Operation:             imageOperation,
		IdempotencyKey:        request.IdempotencyKey,
		RequestHash:           request.RequestHash,
		ProjectContextVersion: request.Project.ProjectContextVersion,
		ModelAlias:            request.ModelAlias,
		SourceSystem:          request.SourceSystem,
		SourceTaskID:          request.SourceTaskID,
		Input:                 request.Input,
	})
	if err != nil {
		return contract.ProviderJob{}, false, err
	}
	return stored.Job, duplicate, nil
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
