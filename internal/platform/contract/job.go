package contract

import (
	"fmt"
	"strings"
	"time"
)

type JobStatus string

const (
	JobQueued             JobStatus = "queued"
	JobRunning            JobStatus = "running"
	JobSucceeded          JobStatus = "succeeded"
	JobPartiallySucceeded JobStatus = "partially_succeeded"
	JobFailed             JobStatus = "failed"
	JobCancelled          JobStatus = "cancelled"
	JobExpired            JobStatus = "expired"
)

type JobError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

// Job is the portable contract for long-running shared capabilities. Its Kind
// identifies a capability (for example provider.image.generate), not a
// vertical-system business state machine.
type Job struct {
	ID             string         `json:"id"`
	Kind           string         `json:"kind"`
	OrganizationID OrganizationID `json:"organization_id"`
	ProjectID      ProjectID      `json:"project_id,omitempty"`
	Status         JobStatus      `json:"status"`
	Progress       int            `json:"progress"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	// ResultRef is retained for compatibility with the bootstrap contract.
	// New multi-output implementations use ResultRefs.
	ResultRef    *ResourceRef  `json:"result_ref,omitempty"`
	ResultRefs   []ResourceRef `json:"result_refs,omitempty"`
	Error        *JobError     `json:"error,omitempty"`
	Cancellable  bool          `json:"cancellable"`
	AttemptCount int           `json:"attempt_count"`
	MaxAttempts  int           `json:"max_attempts"`
	Version      int64         `json:"version"`
}

func (j Job) Validate() error {
	if strings.TrimSpace(j.ID) == "" || strings.TrimSpace(j.Kind) == "" {
		return fmt.Errorf("job ID and kind are required")
	}
	if strings.TrimSpace(string(j.OrganizationID)) == "" {
		return fmt.Errorf("organization_id is required")
	}
	if j.Progress < 0 || j.Progress > 100 {
		return fmt.Errorf("job progress must be between 0 and 100")
	}
	if j.CreatedAt.IsZero() || j.UpdatedAt.IsZero() {
		return fmt.Errorf("job timestamps are required")
	}
	if j.UpdatedAt.Before(j.CreatedAt) {
		return fmt.Errorf("job updated_at cannot be before created_at")
	}
	if j.Version < 1 {
		return fmt.Errorf("job version must be positive")
	}
	if j.AttemptCount < 0 || j.MaxAttempts < 1 || j.AttemptCount > j.MaxAttempts {
		return fmt.Errorf("job attempts are invalid")
	}
	if !j.Status.valid() {
		return fmt.Errorf("job status is invalid")
	}
	if j.ResultRef != nil {
		if err := j.ResultRef.Validate(); err != nil {
			return fmt.Errorf("invalid job result reference: %w", err)
		}
	}
	for index, resultRef := range j.ResultRefs {
		if err := resultRef.Validate(); err != nil {
			return fmt.Errorf("invalid job result reference at index %d: %w", index, err)
		}
	}
	if j.Status == JobSucceeded && j.Error != nil {
		return fmt.Errorf("succeeded job cannot include an error")
	}
	if j.Status == JobFailed && (j.Error == nil || strings.TrimSpace(j.Error.Code) == "") {
		return fmt.Errorf("failed job requires a stable error code")
	}
	if j.Status == JobPartiallySucceeded {
		if j.ResultRef == nil && len(j.ResultRefs) == 0 {
			return fmt.Errorf("partially succeeded job requires at least one result reference")
		}
		if j.Error == nil || strings.TrimSpace(j.Error.Code) == "" {
			return fmt.Errorf("partially succeeded job requires a stable summary error code")
		}
	}
	return nil
}

func (j Job) CanTransitionTo(next JobStatus) bool {
	switch j.Status {
	case JobQueued:
		return next == JobRunning || next == JobCancelled || next == JobExpired
	case JobRunning:
		return next == JobSucceeded || next == JobPartiallySucceeded || next == JobFailed || next == JobCancelled || next == JobExpired
	default:
		return false
	}
}

func (s JobStatus) valid() bool {
	switch s {
	case JobQueued, JobRunning, JobSucceeded, JobPartiallySucceeded, JobFailed, JobCancelled, JobExpired:
		return true
	default:
		return false
	}
}
