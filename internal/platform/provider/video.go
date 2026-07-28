package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	videoGenerateJobKind      = "provider.video.generate"
	videoGenerateOperation    = "video.generate"
	videoExecutionMaxAttempts = 360
)

// VideoGenerationInput is the stable Provider-owned input for asynchronous
// video creation. Vendor request shapes remain private to video adapters.
type VideoGenerationInput struct {
	Prompt          string `json:"prompt"`
	DurationSeconds int    `json:"duration_seconds"`
	AspectRatio     string `json:"aspect_ratio"`
	Resolution      string `json:"resolution"`
}

func (i VideoGenerationInput) Validate() error {
	if strings.TrimSpace(i.Prompt) == "" {
		return fmt.Errorf("video prompt is required")
	}
	if i.DurationSeconds < 1 || i.DurationSeconds > 30 {
		return fmt.Errorf("video duration must be between 1 and 30 seconds")
	}
	switch i.AspectRatio {
	case "9:16", "16:9", "1:1":
	default:
		return fmt.Errorf("video aspect ratio is not supported")
	}
	switch i.Resolution {
	case "480p", "720p", "1080p":
	default:
		return fmt.Errorf("video resolution is not supported")
	}
	return nil
}

// CreateVideoJobRequest is the Provider application seam for video jobs.
// Actor and Project must already have been resolved from trusted context.
type CreateVideoJobRequest struct {
	Actor          contract.ActorContext
	Project        contract.ProjectContext
	IdempotencyKey contract.IdempotencyKey
	RequestHash    string
	ModelAlias     string
	SourceSystem   string
	SourceTaskID   string
	Input          VideoGenerationInput
}

func (r CreateVideoJobRequest) Validate() error {
	if err := r.Actor.Validate(); err != nil {
		return fmt.Errorf("invalid actor: %w", err)
	}
	if !r.Actor.HasScope(ScopeJobCreate) {
		return fmt.Errorf("%s scope is required", ScopeJobCreate)
	}
	if err := r.Project.ValidateBrandBound(); err != nil {
		return fmt.Errorf("invalid project for video generation: %w", err)
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

func (s Service) CreateVideoJob(ctx context.Context, request CreateVideoJobRequest) (contract.ProviderJob, bool, error) {
	if s.Store == nil {
		return contract.ProviderJob{}, false, fmt.Errorf("provider job store is required")
	}
	if s.NewID == nil {
		return contract.ProviderJob{}, false, fmt.Errorf("provider job ID generator is required")
	}
	if s.Scheduler == nil {
		return contract.ProviderJob{}, false, fmt.Errorf("provider execution scheduler is required")
	}
	if err := request.Validate(); err != nil {
		return contract.ProviderJob{}, false, err
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	providerJobID, err := s.NewID()
	if err != nil {
		return contract.ProviderJob{}, false, fmt.Errorf("generate provider job ID: %w", err)
	}
	createdAt := now().UTC()
	var route *VideoRouteSnapshot
	if s.VideoRoutes != nil {
		resolved, resolveErr := s.VideoRoutes.ResolveVideoRoute(ctx, request.Actor.OrganizationID, request.ModelAlias)
		if resolveErr != nil {
			return contract.ProviderJob{}, false, fmt.Errorf("resolve provider video route: %w", resolveErr)
		}
		route = &resolved
	}
	job := contract.ProviderJob{
		ID:               providerJobID,
		Kind:             videoGenerateJobKind,
		OrganizationID:   request.Actor.OrganizationID,
		ProjectID:        request.Project.ProjectID,
		ExecutionStatus:  contract.JobQueued,
		ProviderStatus:   contract.ProviderJobSubmitted,
		Progress:         0,
		ProjectAssetRefs: []contract.ProjectAssetRef{},
		AttemptCount:     0,
		MaxAttempts:      videoExecutionMaxAttempts,
		Version:          1,
		CreatedAt:        createdAt,
		UpdatedAt:        createdAt,
	}
	if err := job.Validate(); err != nil {
		return contract.ProviderJob{}, false, fmt.Errorf("create provider video job: %w", err)
	}
	stored, duplicate, err := s.Store.Create(ctx, JobRecord{
		Job:                   job,
		Principal:             request.Actor.Principal,
		Operation:             videoGenerateOperation,
		IdempotencyKey:        request.IdempotencyKey,
		RequestHash:           request.RequestHash,
		ProjectContextVersion: request.Project.ProjectContextVersion,
		ModelAlias:            request.ModelAlias,
		SourceSystem:          request.SourceSystem,
		SourceTaskID:          request.SourceTaskID,
		VideoInput:            request.Input,
		Route:                 route,
		SubmissionState:       SubmissionNotStarted,
	})
	if err != nil {
		return contract.ProviderJob{}, false, err
	}
	if err := s.Scheduler.Schedule(ctx, stored.Job); err != nil {
		return stored.Job, duplicate, fmt.Errorf("schedule provider video job execution: %w", err)
	}
	return stored.Job, duplicate, nil
}
