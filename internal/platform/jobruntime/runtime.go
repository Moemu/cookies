// Package jobruntime owns durable, generic execution mechanics. A domain Job
// keeps its own input and result semantics behind this small interface.
package jobruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

var ErrIdempotencyConflict = errors.New("idempotency key was reused with a different request")
var ErrNoHandler = errors.New("no job handler is registered for this kind")

type CreateRequest struct {
	Job            contract.Job
	Payload        json.RawMessage
	IdempotencyKey contract.IdempotencyKey
	RequestHash    string
}

func (r CreateRequest) Validate() error {
	if err := r.Job.Validate(); err != nil {
		return err
	}
	if r.Job.Status != contract.JobQueued || r.Job.AttemptCount != 0 {
		return fmt.Errorf("new jobs must be queued with zero attempts")
	}
	if err := r.IdempotencyKey.Validate(); err != nil {
		return err
	}
	if len(r.RequestHash) != 64 {
		return fmt.Errorf("request hash must be a SHA-256 hex string")
	}
	if !json.Valid(r.Payload) {
		return fmt.Errorf("job payload must be valid JSON")
	}
	return nil
}

type Claim struct {
	Job       contract.Job
	Payload   json.RawMessage
	LockOwner string
}

type Result struct{ Ref *contract.ResourceRef }

// LeaseRecovery reports how many abandoned executions became available again
// and how many exhausted their attempt budget while abandoned.
type LeaseRecovery struct {
	Rescheduled int64
	Failed      int64
}

// LeaseRecoverer is deliberately separate from Store because recovery is a
// scheduler concern, not something every request-time store caller needs.
type LeaseRecoverer interface {
	ReclaimExpired(ctx context.Context, now time.Time, leaseDuration time.Duration) (LeaseRecovery, error)
}

type Store interface {
	Enqueue(ctx context.Context, request CreateRequest) (job contract.Job, duplicate bool, err error)
	Claim(ctx context.Context, workerID string, now time.Time) (Claim, bool, error)
	Succeed(ctx context.Context, claim Claim, result Result, now time.Time) error
	Fail(ctx context.Context, claim Claim, problem contract.JobError, now time.Time) error
	Reschedule(ctx context.Context, claim Claim, availableAt time.Time, now time.Time) error
}

type Handler func(ctx context.Context, claim Claim) (Result, error)

type ExecutionError struct{ JobError contract.JobError }

func (e ExecutionError) Error() string { return e.JobError.Code }

// DeferredError asks the runtime to release a claimed job and make it
// available again later. Domain handlers use it for polling and other
// recoverable waits without incorrectly recording a terminal failure.
type DeferredError struct{ AvailableAt time.Time }

func (e DeferredError) Error() string { return "job execution deferred" }

type Worker struct {
	Store    Store
	Handlers map[string]Handler
	Now      func() time.Time
}

func (w Worker) RunOnce(ctx context.Context, workerID string) (bool, error) {
	if w.Store == nil {
		return false, fmt.Errorf("job store is required")
	}
	if w.Now == nil {
		w.Now = time.Now
	}
	claim, found, err := w.Store.Claim(ctx, workerID, w.Now().UTC())
	if err != nil || !found {
		return found, err
	}
	handler := w.Handlers[claim.Job.Kind]
	if handler == nil {
		return true, w.Store.Fail(ctx, claim, contract.JobError{Code: "JOB_HANDLER_UNAVAILABLE", Message: "No handler is configured for this job kind", Retryable: false}, w.Now().UTC())
	}
	result, err := handler(ctx, claim)
	if err == nil {
		return true, w.Store.Succeed(ctx, claim, result, w.Now().UTC())
	}
	var deferred DeferredError
	if errors.As(err, &deferred) {
		availableAt := deferred.AvailableAt.UTC()
		if availableAt.IsZero() || !availableAt.After(w.Now().UTC()) {
			return true, w.Store.Fail(ctx, claim, contract.JobError{Code: "JOB_DEFER_INVALID", Message: "Job deferral time must be in the future", Retryable: false}, w.Now().UTC())
		}
		if claim.Job.AttemptCount >= claim.Job.MaxAttempts {
			return true, w.Store.Fail(ctx, claim, contract.JobError{Code: "JOB_ATTEMPT_LIMIT_EXCEEDED", Message: "Job reached its maximum execution attempts", Retryable: false}, w.Now().UTC())
		}
		return true, w.Store.Reschedule(ctx, claim, availableAt, w.Now().UTC())
	}
	var executionError ExecutionError
	if errors.As(err, &executionError) {
		return true, w.Store.Fail(ctx, claim, executionError.JobError, w.Now().UTC())
	}
	return true, w.Store.Fail(ctx, claim, contract.JobError{Code: "JOB_EXECUTION_FAILED", Message: "Job execution failed", Retryable: true}, w.Now().UTC())
}
