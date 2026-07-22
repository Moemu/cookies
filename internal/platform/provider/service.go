// Package provider owns capability-based model execution. It deliberately
// knows nothing about Assets persistence or vendor SDK request types.
package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

const ScopeJobCreate contract.Scope = "provider.job.create"

const imageJobKind = "provider.image.generate"
const imageOperation = "image.generate"

var ErrJobNotFound = errors.New("provider job not found")
var ErrVersionConflict = errors.New("provider job version conflict")

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
	ProviderCode          string
	ModelVersion          string
	ExternalTaskID        string
	Outputs               []OutputRecord
}

type OutputStatus string

const (
	OutputReady     OutputStatus = "ready"
	OutputIngesting OutputStatus = "ingesting"
	OutputSucceeded OutputStatus = "succeeded"
	OutputFailed    OutputStatus = "failed"
)

type OutputRecord struct {
	Ref             contract.ProviderOutputRef
	Status          OutputStatus
	IntakeID        string
	ProjectAssetRef *contract.ProjectAssetRef
	Error           *contract.JobError
}

// JobStore owns ProviderJob durability and Provider-specific idempotency. It
// intentionally does not reuse platform_jobs' narrower idempotency scope.
type JobStore interface {
	Create(ctx context.Context, record JobRecord) (stored JobRecord, duplicate bool, err error)
	Get(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, jobID string) (JobRecord, error)
	Update(ctx context.Context, record JobRecord) (JobRecord, error)
}

// GeneratedIntakeClient is Provider's seam to the Assets-owned intake
// capability. Its types remain owned by Assets; Provider never receives a
// database handle or object-storage URL from this interface.
type GeneratedIntakeClient interface {
	Create(ctx context.Context, projectID contract.ProjectID, request assets.GeneratedAssetIntakeRequest, key contract.IdempotencyKey) (assets.GeneratedAssetIntakeResponse, error)
	Get(ctx context.Context, projectID contract.ProjectID, intakeID string) (assets.GeneratedAssetIntakeResponse, error)
}

// Service is the small application seam used by transport and workers.
type Service struct {
	Store         JobStore
	Scheduler     ExecutionScheduler
	ImageAdapter  ImageProviderAdapter
	TextAdapter   TextProviderAdapter
	VisionAdapter VisionProviderAdapter
	VisionSources VisionSourceResolver
	Intake        GeneratedIntakeClient
	NewID         func() string
	Now           func() time.Time
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
	if s.Scheduler == nil {
		return contract.ProviderJob{}, false, fmt.Errorf("provider execution scheduler is required")
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
	if err := s.Scheduler.Schedule(ctx, stored.Job); err != nil {
		return stored.Job, duplicate, fmt.Errorf("schedule provider job execution: %w", err)
	}
	return stored.Job, duplicate, nil
}

// GetJob returns the public state of a Provider-owned job after its
// organization and project scope have already been authorized by transport.
func (s Service) GetJob(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, jobID string) (contract.ProviderJob, error) {
	if s.Store == nil {
		return contract.ProviderJob{}, fmt.Errorf("provider job store is required")
	}
	record, err := s.Store.Get(ctx, organizationID, projectID, jobID)
	if err != nil {
		return contract.ProviderJob{}, err
	}
	return record.Job, nil
}

// ProcessImageJob advances only the Assets handoff portion of a ProviderJob.
// Adapter submission and polling are separate seams; this method starts after
// a verified ProviderOutputRef has been persisted at outputs_ready.
func (s Service) ProcessImageJob(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, jobID string) (contract.ProviderJob, *time.Time, error) {
	if s.Store == nil || s.Intake == nil {
		return contract.ProviderJob{}, nil, fmt.Errorf("provider job store and generated intake client are required")
	}
	record, err := s.Store.Get(ctx, organizationID, projectID, jobID)
	if err != nil {
		return contract.ProviderJob{}, nil, err
	}
	if isProviderTerminal(record.Job.ProviderStatus) {
		return record.Job, nil, nil
	}
	if record.Job.ProviderStatus != contract.ProviderJobOutputsReady && record.Job.ProviderStatus != contract.ProviderJobIngesting {
		return contract.ProviderJob{}, nil, fmt.Errorf("provider job %s is not ready for intake", jobID)
	}
	if strings.TrimSpace(record.ProviderCode) == "" || strings.TrimSpace(record.ModelVersion) == "" {
		return contract.ProviderJob{}, nil, fmt.Errorf("provider job %s is missing resolved provider model metadata", jobID)
	}
	now := s.nowUTC()
	pending := false
	for index := range record.Outputs {
		output := &record.Outputs[index]
		switch output.Status {
		case OutputReady:
			response, createErr := s.Intake.Create(ctx, projectID, assets.GeneratedAssetIntakeRequest{
				ProviderJobID: record.Job.ID,
				Output:        output.Ref,
				Provenance: assets.GenerationProvenance{
					Capability:            imageOperation,
					ProviderCode:          record.ProviderCode,
					ModelAlias:            record.ModelAlias,
					ModelVersion:          record.ModelVersion,
					PromptRef:             nil,
					SourceAssetRefs:       []contract.AssetVersionRef{},
					ProjectContextVersion: record.ProjectContextVersion,
					GeneratedAt:           now,
				},
			}, contract.IdempotencyKey(fmt.Sprintf("provider-job-%s-output-%s", record.Job.ID, output.Ref.OutputID)))
			if createErr != nil {
				return record.Job, nil, createErr
			}
			if responseErr := validateIntakeResponse(response, record.Job.ID, output.Ref.OutputID, projectID); responseErr != nil {
				return record.Job, nil, responseErr
			}
			output.IntakeID = response.ID
			applyIntakeResponse(output, response)
		case OutputIngesting:
			response, getErr := s.Intake.Get(ctx, projectID, output.IntakeID)
			if getErr != nil {
				return record.Job, nil, getErr
			}
			if responseErr := validateIntakeResponse(response, record.Job.ID, output.Ref.OutputID, projectID); responseErr != nil {
				return record.Job, nil, responseErr
			}
			applyIntakeResponse(output, response)
		}
		if output.Status == OutputReady || output.Status == OutputIngesting {
			pending = true
		}
	}
	if pending {
		record.Job.ProviderStatus = contract.ProviderJobIngesting
		record.Job.ExecutionStatus = contract.JobRunning
		record.Job.Progress = 80
		record.Job.UpdatedAt = now
		updated, updateErr := s.Store.Update(ctx, record)
		if updateErr != nil {
			return contract.ProviderJob{}, nil, updateErr
		}
		deferUntil := now.Add(5 * time.Second)
		return updated.Job, &deferUntil, nil
	}
	finalizeImageJob(&record.Job, record.Outputs, now)
	updated, updateErr := s.Store.Update(ctx, record)
	if updateErr != nil {
		return contract.ProviderJob{}, nil, updateErr
	}
	return updated.Job, nil, nil
}

func validateIntakeResponse(response assets.GeneratedAssetIntakeResponse, providerJobID, outputID string, projectID contract.ProjectID) error {
	if err := response.Validate(); err != nil {
		return fmt.Errorf("invalid generated intake response: %w", err)
	}
	if response.ProviderJobID != providerJobID || response.OutputID != outputID {
		return fmt.Errorf("generated intake response belongs to another provider output")
	}
	if response.ProjectAssetRef != nil && response.ProjectAssetRef.ProjectID != projectID {
		return fmt.Errorf("generated intake response belongs to another project")
	}
	return nil
}

func applyIntakeResponse(output *OutputRecord, response assets.GeneratedAssetIntakeResponse) {
	switch response.Status {
	case assets.GeneratedIntakeQueued, assets.GeneratedIntakeRunning:
		output.Status = OutputIngesting
	case assets.GeneratedIntakeSucceeded:
		output.Status = OutputSucceeded
		output.ProjectAssetRef = response.ProjectAssetRef
		output.Error = nil
	case assets.GeneratedIntakeFailed:
		output.Status = OutputFailed
		output.Error = response.Error
	}
}

func finalizeImageJob(job *contract.ProviderJob, outputs []OutputRecord, now time.Time) {
	refs := make([]contract.ProjectAssetRef, 0, len(outputs))
	var firstError *contract.JobError
	for _, output := range outputs {
		if output.Status == OutputSucceeded && output.ProjectAssetRef != nil {
			refs = append(refs, *output.ProjectAssetRef)
		}
		if firstError == nil && output.Error != nil {
			problem := *output.Error
			firstError = &problem
		}
	}
	job.ProjectAssetRefs = refs
	job.Progress = 100
	job.UpdatedAt = now
	switch {
	case len(refs) == len(outputs) && len(outputs) > 0:
		job.ExecutionStatus = contract.JobSucceeded
		job.ProviderStatus = contract.ProviderJobSucceeded
		job.Error = nil
	case len(refs) > 0:
		job.ExecutionStatus = contract.JobSucceeded
		job.ProviderStatus = contract.ProviderJobPartiallySucceeded
		job.Error = firstError
	default:
		job.ExecutionStatus = contract.JobFailed
		job.ProviderStatus = contract.ProviderJobFailed
		if firstError == nil {
			firstError = &contract.JobError{Code: contract.ErrorAssetIntakeFailed, Message: "No generated output became a durable project asset", Retryable: false}
		}
		job.Error = firstError
	}
}

func isProviderTerminal(status contract.ProviderJobStatus) bool {
	switch status {
	case contract.ProviderJobSucceeded, contract.ProviderJobPartiallySucceeded, contract.ProviderJobFailed, contract.ProviderJobCancelled, contract.ProviderJobExpired:
		return true
	default:
		return false
	}
}

func (s Service) nowUTC() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
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
