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
