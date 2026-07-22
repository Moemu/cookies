package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
)

const imageExecutionJobKind = "provider.image.execute"

// NewRuntimeWorker returns the shared worker configuration required for
// Provider image jobs. The composition root owns its lifecycle and worker ID.
func NewRuntimeWorker(store jobruntime.Store, service Service) jobruntime.Worker {
	return jobruntime.Worker{
		Store: store,
		Handlers: map[string]jobruntime.Handler{
			imageExecutionJobKind: RuntimeHandler(service),
		},
	}
}

// ExecutionScheduler is the only Provider seam to the shared durable worker
// runtime. It schedules a ProviderJob by opaque ID; it never exposes input
// prompts to the generic job payload.
type ExecutionScheduler interface {
	Schedule(context.Context, contract.ProviderJob) error
}

// JobRuntimeScheduler adapts Provider execution into platform_jobs. The
// deterministic key makes a retry after the provider row was committed but
// before enqueue completed recoverable without duplicating work.
type JobRuntimeScheduler struct {
	Store jobruntime.Store
	NewID func() (string, error)
	Now   func() time.Time
}

func (s JobRuntimeScheduler) Schedule(ctx context.Context, providerJob contract.ProviderJob) error {
	if s.Store == nil || s.NewID == nil {
		return fmt.Errorf("job runtime store and ID generator are required")
	}
	if err := providerJob.Validate(); err != nil {
		return fmt.Errorf("invalid provider job: %w", err)
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	createdAt := now().UTC()
	payload, err := json.Marshal(struct {
		ProviderJobID string `json:"provider_job_id"`
	}{ProviderJobID: providerJob.ID})
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(providerJob.ID))
	executionJobID, err := s.NewID()
	if err != nil {
		return fmt.Errorf("generate provider execution job ID: %w", err)
	}
	_, _, err = s.Store.Enqueue(ctx, jobruntime.CreateRequest{
		Job: contract.Job{
			ID:             executionJobID,
			Kind:           imageExecutionJobKind,
			OrganizationID: providerJob.OrganizationID,
			ProjectID:      providerJob.ProjectID,
			Status:         contract.JobQueued,
			Progress:       0,
			Cancellable:    false,
			MaxAttempts:    100,
			Version:        1,
			CreatedAt:      createdAt,
			UpdatedAt:      createdAt,
		},
		Payload:        payload,
		IdempotencyKey: contract.IdempotencyKey("provider-execution-" + providerJob.ID),
		RequestHash:    hex.EncodeToString(digest[:]),
	})
	return err
}

// RuntimeHandler is registered once by the composition root. A successful
// generic run means the Provider state machine advanced; the public outcome
// remains on ProviderJob, which can itself be a domain-level failure.
func RuntimeHandler(service Service) jobruntime.Handler {
	return func(ctx context.Context, claim jobruntime.Claim) (jobruntime.Result, error) {
		if claim.Job.Kind != imageExecutionJobKind {
			return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "PROVIDER_JOB_KIND_INVALID", Message: "Provider handler received an unsupported job kind", Retryable: false}}
		}
		var payload struct {
			ProviderJobID string `json:"provider_job_id"`
		}
		if err := json.Unmarshal(claim.Payload, &payload); err != nil || strings.TrimSpace(payload.ProviderJobID) == "" {
			return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "PROVIDER_JOB_PAYLOAD_INVALID", Message: "Provider execution payload is invalid", Retryable: false}}
		}
		_, deferredUntil, err := service.ExecuteImageJob(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, payload.ProviderJobID)
		if err != nil {
			if claim.Job.AttemptCount >= claim.Job.MaxAttempts {
				return exhaustedProviderExecution(service, ctx, claim, payload.ProviderJobID)
			}
			return jobruntime.Result{}, jobruntime.DeferredError{AvailableAt: time.Now().UTC().Add(providerPollDelay)}
		}
		if deferredUntil != nil {
			if claim.Job.AttemptCount >= claim.Job.MaxAttempts {
				return exhaustedProviderExecution(service, ctx, claim, payload.ProviderJobID)
			}
			return jobruntime.Result{}, jobruntime.DeferredError{AvailableAt: *deferredUntil}
		}
		return jobruntime.Result{}, nil
	}
}

func exhaustedProviderExecution(service Service, ctx context.Context, claim jobruntime.Claim, providerJobID string) (jobruntime.Result, error) {
	if _, err := service.FailImageJobAfterExecutionExhausted(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, providerJobID); err != nil {
		return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "PROVIDER_STATE_UPDATE_FAILED", Message: "Provider job could not be finalized after execution exhaustion", Retryable: true}}
	}
	return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "PROVIDER_EXECUTION_EXHAUSTED", Message: "Provider job exceeded its recovery attempt limit", Retryable: false}}
}
