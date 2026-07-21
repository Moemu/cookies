package contract

import (
	"testing"
	"time"
)

func TestJobLifecycleDoesNotReopenTerminalJobs(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	job := Job{
		ID:             "job_1",
		Kind:           "provider.image.generate",
		OrganizationID: "org_1",
		ProjectID:      "project_1",
		Status:         JobQueued,
		CreatedAt:      now,
		UpdatedAt:      now,
		Cancellable:    true,
		MaxAttempts:    1,
		Version:        1,
	}
	if err := job.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !job.CanTransitionTo(JobRunning) || job.CanTransitionTo(JobSucceeded) {
		t.Fatal("unexpected queued job transitions")
	}

	job.Status = JobSucceeded
	if job.CanTransitionTo(JobRunning) {
		t.Fatal("terminal job must not reopen")
	}
}

func TestJobLifecycleSupportsPartialSuccessAndExpiry(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	version := int64(1)
	job := Job{
		ID:             "job_1",
		Kind:           "provider.image.generate",
		OrganizationID: "org_1",
		ProjectID:      "project_1",
		Status:         JobRunning,
		CreatedAt:      now,
		UpdatedAt:      now,
		Cancellable:    true,
		MaxAttempts:    1,
		Version:        2,
	}

	if !job.CanTransitionTo(JobPartiallySucceeded) || !job.CanTransitionTo(JobExpired) {
		t.Fatal("running job must support partial success and expiry")
	}

	job.Status = JobPartiallySucceeded
	job.ResultRefs = []ResourceRef{{Type: "provider_output", ID: "output_1", Version: &version}}
	job.Error = &JobError{Code: "MODEL_OUTPUT_PARTIAL", Message: "one output failed", Retryable: true}
	if err := job.Validate(); err != nil {
		t.Fatalf("Validate() partial job error = %v", err)
	}
	if job.CanTransitionTo(JobRunning) {
		t.Fatal("partially succeeded job must be terminal")
	}
}

func TestPartiallySucceededJobRequiresResultAndSummaryError(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	job := Job{
		ID:             "job_1",
		Kind:           "provider.image.generate",
		OrganizationID: "org_1",
		ProjectID:      "project_1",
		Status:         JobPartiallySucceeded,
		CreatedAt:      now,
		UpdatedAt:      now,
		MaxAttempts:    1,
		Version:        1,
	}
	if err := job.Validate(); err == nil {
		t.Fatal("expected partial job without result and error to be rejected")
	}
}

func TestJobAcceptsBootstrapSingleResultReference(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	version := int64(1)
	job := Job{
		ID:             "job_1",
		Kind:           "provider.image.generate",
		OrganizationID: "org_1",
		ProjectID:      "project_1",
		Status:         JobSucceeded,
		CreatedAt:      now,
		UpdatedAt:      now,
		ResultRef:      &ResourceRef{Type: "provider_output", ID: "output_1", Version: &version},
		MaxAttempts:    1,
		Version:        1,
	}
	if err := job.Validate(); err != nil {
		t.Fatalf("Validate() compatibility error = %v", err)
	}
}
