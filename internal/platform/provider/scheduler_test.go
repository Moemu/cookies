package provider

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
)

func TestJobRuntimeSchedulerEnqueuesOpaqueProviderJobReference(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 22, 7, 0, 0, 0, time.UTC)
	store := &schedulerStore{}
	scheduler := JobRuntimeScheduler{Store: store, NewID: func() (string, error) { return "execution_job_1", nil }, Now: func() time.Time { return now }}
	job := providerJobForScheduler(now)

	if err := scheduler.Schedule(context.Background(), job); err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}
	if store.request.Job.ID != "execution_job_1" || store.request.Job.Kind != imageExecutionJobKind || store.request.Job.ProjectID != job.ProjectID || store.request.Job.MaxAttempts != job.MaxAttempts {
		t.Fatalf("unexpected runtime job: %+v", store.request.Job)
	}
	if store.request.IdempotencyKey != "provider-execution-provider_job_1" {
		t.Fatalf("runtime idempotency key = %q", store.request.IdempotencyKey)
	}
	var payload map[string]any
	if err := json.Unmarshal(store.request.Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(payload) != 1 || payload["provider_job_id"] != "provider_job_1" {
		t.Fatalf("runtime payload leaks provider input or has wrong value: %s", store.request.Payload)
	}
}

func TestJobRuntimeSchedulerUsesVideoExecutionKind(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 27, 7, 0, 0, 0, time.UTC)
	store := &schedulerStore{}
	scheduler := JobRuntimeScheduler{Store: store, NewID: func() (string, error) { return "execution_video_1", nil }, Now: func() time.Time { return now }}
	job := providerJobForScheduler(now)
	job.ID = "provider_video_1"
	job.Kind = videoGenerateJobKind
	job.MaxAttempts = videoExecutionMaxAttempts

	if err := scheduler.Schedule(context.Background(), job); err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}
	if store.request.Job.Kind != videoExecutionJobKind || store.request.Job.MaxAttempts != videoExecutionMaxAttempts {
		t.Fatalf("unexpected video runtime job: %+v", store.request.Job)
	}
	worker := NewRuntimeWorker(store, Service{})
	if _, exists := worker.Handlers[videoExecutionJobKind]; !exists {
		t.Fatalf("runtime worker does not register %q", videoExecutionJobKind)
	}
}

func TestRuntimeHandlerMirrorsExecutionAttemptsToProviderJob(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 22, 7, 15, 0, 0, time.UTC)
	record := executableImageJobRecord(now)
	store := &processingStore{record: record}
	service := Service{Store: store, ImageAdapter: failingImageAdapter{}, Now: func() time.Time { return now }}
	payload, err := json.Marshal(struct {
		ProviderJobID string `json:"provider_job_id"`
	}{ProviderJobID: record.Job.ID})
	if err != nil {
		t.Fatal(err)
	}

	_, err = RuntimeHandler(service)(context.Background(), jobruntime.Claim{Job: contract.Job{
		ID: "execution_job_1", Kind: imageExecutionJobKind, OrganizationID: record.Job.OrganizationID, ProjectID: record.Job.ProjectID,
		Status: contract.JobRunning, Cancellable: false, AttemptCount: 2, MaxAttempts: 100, Version: 2, CreatedAt: now, UpdatedAt: now,
	}, Payload: payload, LockOwner: "worker_1"})
	var deferred jobruntime.DeferredError
	if !errors.As(err, &deferred) {
		t.Fatalf("handler error = %v, want deferred execution", err)
	}
	if store.record.Job.AttemptCount != 2 || store.record.Job.MaxAttempts != 100 {
		t.Fatalf("ProviderJob execution attempts were not mirrored: %+v", store.record.Job)
	}
}

func TestRuntimeHandlerDefersWhenExecutionAttemptCannotBeRecorded(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 27, 9, 30, 0, 0, time.UTC)
	record := executableImageJobRecord(now)
	store := &attemptRecordingFailureStore{
		processingStore: processingStore{record: record},
		err:             errors.New("temporary database interruption"),
	}
	service := Service{Store: store, ImageAdapter: failingImageAdapter{}, Now: func() time.Time { return now }}
	payload, err := json.Marshal(struct {
		ProviderJobID string `json:"provider_job_id"`
	}{ProviderJobID: record.Job.ID})
	if err != nil {
		t.Fatal(err)
	}

	_, err = RuntimeHandler(service)(context.Background(), jobruntime.Claim{Job: contract.Job{
		ID: "execution_job_1", Kind: imageExecutionJobKind, OrganizationID: record.Job.OrganizationID, ProjectID: record.Job.ProjectID,
		Status: contract.JobRunning, Cancellable: false, AttemptCount: 3, MaxAttempts: 100, Version: 3, CreatedAt: now, UpdatedAt: now,
	}, Payload: payload, LockOwner: "worker_1"})
	var deferred jobruntime.DeferredError
	if !errors.As(err, &deferred) {
		t.Fatalf("handler error = %v, want deferred execution after a temporary attempt-recording failure", err)
	}
	if !deferred.AvailableAt.After(time.Now().UTC()) {
		t.Fatalf("deferred time %s is not in the future", deferred.AvailableAt)
	}
}

func TestRuntimeHandlerFinalizesProviderJobWhenRecoveryIsExhausted(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 22, 7, 30, 0, 0, time.UTC)
	record := executableImageJobRecord(now)
	store := &processingStore{record: record}
	service := Service{Store: store, ImageAdapter: failingImageAdapter{}, Now: func() time.Time { return now }}
	handler := RuntimeHandler(service)
	payload, err := json.Marshal(struct {
		ProviderJobID string `json:"provider_job_id"`
	}{ProviderJobID: record.Job.ID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = handler(context.Background(), jobruntime.Claim{Job: contract.Job{
		ID: "execution_job_1", Kind: imageExecutionJobKind, OrganizationID: record.Job.OrganizationID, ProjectID: record.Job.ProjectID,
		Status: contract.JobRunning, Cancellable: false, AttemptCount: 100, MaxAttempts: 100, Version: 2, CreatedAt: now, UpdatedAt: now,
	}, Payload: payload, LockOwner: "worker_1"})
	var executionError jobruntime.ExecutionError
	if !errors.As(err, &executionError) || executionError.JobError.Code != "PROVIDER_EXECUTION_EXHAUSTED" {
		t.Fatalf("handler error = %v, want exhausted execution error", err)
	}
	if store.record.Job.ExecutionStatus != contract.JobFailed || store.record.Job.ProviderStatus != contract.ProviderJobFailed || store.record.Job.Error == nil || store.record.Job.Error.Code != "PROVIDER_EXECUTION_EXHAUSTED" {
		t.Fatalf("ProviderJob was not finalized: %+v", store.record.Job)
	}
}

func TestRuntimeHandlerDoesNotRetryUnknownSubmissionOutcome(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 22, 7, 45, 0, 0, time.UTC)
	record := executableImageJobRecord(now)
	store := &processingStore{record: record}
	service := Service{Store: store, ImageAdapter: unknownSubmissionAdapter{}, Now: func() time.Time { return now }}
	payload, err := json.Marshal(struct {
		ProviderJobID string `json:"provider_job_id"`
	}{ProviderJobID: record.Job.ID})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RuntimeHandler(service)(context.Background(), jobruntime.Claim{Job: contract.Job{ID: "execution_job_1", Kind: imageExecutionJobKind, OrganizationID: record.Job.OrganizationID, ProjectID: record.Job.ProjectID, Status: contract.JobRunning, Cancellable: false, AttemptCount: 1, MaxAttempts: 100, Version: 1, CreatedAt: now, UpdatedAt: now}, Payload: payload, LockOwner: "worker_1"})
	var executionError jobruntime.ExecutionError
	if !errors.As(err, &executionError) || executionError.JobError.Code != "MODEL_SUBMISSION_UNKNOWN" {
		t.Fatalf("handler error = %v, want terminal unknown submission error", err)
	}
	if store.record.Job.ProviderStatus != contract.ProviderJobFailed || store.record.Job.Error == nil || store.record.Job.Error.Code != "MODEL_SUBMISSION_UNKNOWN" {
		t.Fatalf("ProviderJob was not terminally failed: %+v", store.record.Job)
	}
}

func providerJobForScheduler(now time.Time) contract.ProviderJob {
	return contract.ProviderJob{
		ID: "provider_job_1", Kind: imageJobKind, OrganizationID: "org_1", ProjectID: "project_1",
		ExecutionStatus: contract.JobQueued, ProviderStatus: contract.ProviderJobSubmitted, ProjectAssetRefs: []contract.ProjectAssetRef{},
		MaxAttempts: 3, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
}

type schedulerStore struct{ request jobruntime.CreateRequest }

type failingImageAdapter struct{}

type unknownSubmissionAdapter struct{}

type attemptRecordingFailureStore struct {
	processingStore
	err error
}

func (s *attemptRecordingFailureStore) Update(context.Context, JobRecord) (JobRecord, error) {
	return JobRecord{}, s.err
}

func (unknownSubmissionAdapter) Submit(context.Context, ImageGenerationRequest) (ImageSubmission, error) {
	return ImageSubmission{}, ExecutionError{JobError: contract.JobError{Code: "MODEL_SUBMISSION_UNKNOWN", Message: "unknown", Retryable: false}}
}

func (unknownSubmissionAdapter) Poll(context.Context, ImageTaskReference) (ImageTaskResult, error) {
	return ImageTaskResult{}, nil
}

func (failingImageAdapter) Submit(context.Context, ImageGenerationRequest) (ImageSubmission, error) {
	return ImageSubmission{}, errors.New("temporary provider outage")
}

func (failingImageAdapter) Poll(context.Context, ImageTaskReference) (ImageTaskResult, error) {
	return ImageTaskResult{}, errors.New("temporary provider outage")
}

func (s *schedulerStore) Enqueue(_ context.Context, request jobruntime.CreateRequest) (contract.Job, bool, error) {
	s.request = request
	return request.Job, false, nil
}

func (*schedulerStore) Claim(context.Context, string, time.Time) (jobruntime.Claim, bool, error) {
	return jobruntime.Claim{}, false, nil
}

func (*schedulerStore) Succeed(context.Context, jobruntime.Claim, jobruntime.Result, time.Time) error {
	return nil
}

func (*schedulerStore) Fail(context.Context, jobruntime.Claim, contract.JobError, time.Time) error {
	return nil
}

func (*schedulerStore) Reschedule(context.Context, jobruntime.Claim, time.Time, time.Time) error {
	return nil
}
