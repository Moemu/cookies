package jobruntime

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestWorkerDoesNotTransitionJobAfterLeaseRenewalFails(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 23, 3, 45, 0, 0, time.UTC)
	store := &memoryStore{claim: Claim{Job: contract.Job{ID: "job_lost", Kind: "slow", OrganizationID: "org_1", Status: contract.JobRunning, CreatedAt: now, UpdatedAt: now, Cancellable: true, AttemptCount: 1, MaxAttempts: 2, Version: 2}, LockOwner: "worker_1"}}
	worker := Worker{
		Store: store, LeaseRenewer: leaseRenewerFunc(func(context.Context, Claim, time.Time) error {
			return fmt.Errorf("database unavailable")
		}), HeartbeatInterval: time.Millisecond,
		Handlers: map[string]Handler{"slow": func(ctx context.Context, _ Claim) (Result, error) {
			<-ctx.Done()
			return Result{}, ctx.Err()
		}},
		Now: func() time.Time { return now },
	}
	processed, err := worker.RunOnce(context.Background(), "worker_1")
	if !processed || !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("RunOnce() processed=%v err=%v", processed, err)
	}
	if store.failed || store.succeeded || store.rescheduled {
		t.Fatalf("lost lease caused a state transition: %+v", store)
	}
}

func TestWorkerRenewsLeaseWhileHandlerRuns(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 23, 3, 30, 0, 0, time.UTC)
	store := &memoryStore{claim: Claim{Job: contract.Job{ID: "job_heartbeat", Kind: "slow", OrganizationID: "org_1", Status: contract.JobRunning, CreatedAt: now, UpdatedAt: now, Cancellable: true, AttemptCount: 1, MaxAttempts: 2, Version: 2}, LockOwner: "worker_1"}}
	var renewals atomic.Int32
	worker := Worker{
		Store: store, LeaseRenewer: leaseRenewerFunc(func(context.Context, Claim, time.Time) error {
			renewals.Add(1)
			return nil
		}), HeartbeatInterval: 5 * time.Millisecond,
		Handlers: map[string]Handler{"slow": func(context.Context, Claim) (Result, error) {
			time.Sleep(25 * time.Millisecond)
			return Result{}, nil
		}},
		Now: func() time.Time { return now },
	}
	processed, err := worker.RunOnce(context.Background(), "worker_1")
	if err != nil || !processed || !store.succeeded || renewals.Load() < 2 {
		t.Fatalf("RunOnce() processed=%v succeeded=%v renewals=%d err=%v", processed, store.succeeded, renewals.Load(), err)
	}
}

type leaseRenewerFunc func(context.Context, Claim, time.Time) error

func (f leaseRenewerFunc) RenewLease(ctx context.Context, claim Claim, now time.Time) error {
	return f(ctx, claim, now)
}

func TestWorkerClaimsAndCompletesAJob(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 21, 0, 0, 0, 0, time.UTC)
	store := &memoryStore{claim: Claim{Job: contract.Job{ID: "job_1", Kind: "provider.image.generate", OrganizationID: "org_1", Status: contract.JobRunning, CreatedAt: now, UpdatedAt: now, Cancellable: true, AttemptCount: 1, MaxAttempts: 1, Version: 2}, LockOwner: "worker_1"}}
	worker := Worker{
		Store: store,
		Handlers: map[string]Handler{
			"provider.image.generate": func(context.Context, Claim) (Result, error) { return Result{}, nil },
		},
		Now: func() time.Time { return now },
	}
	processed, err := worker.RunOnce(context.Background(), "worker_1")
	if err != nil || !processed || !store.succeeded {
		t.Fatalf("RunOnce() processed=%v succeeded=%v err=%v", processed, store.succeeded, err)
	}
}

func TestWorkerHonorsCancellationRequestedDuringHandler(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 23, 9, 0, 0, 0, time.UTC)
	store := &memoryStore{
		claim:           Claim{Job: contract.Job{ID: "job_cancel", Kind: "strategy.generate", OrganizationID: "org_1", Status: contract.JobRunning, CreatedAt: now, UpdatedAt: now, Cancellable: true, AttemptCount: 1, MaxAttempts: 2, Version: 2}, LockOwner: "worker_1"},
		cancelRequested: true,
	}
	worker := Worker{
		Store: store, Canceller: store,
		Handlers: map[string]Handler{
			"strategy.generate": func(context.Context, Claim) (Result, error) { return Result{}, nil },
		},
		Now: func() time.Time { return now },
	}
	processed, err := worker.RunOnce(context.Background(), "worker_1")
	if err != nil || !processed || !store.cancelled || store.succeeded {
		t.Fatalf("processed=%v cancelled=%v succeeded=%v err=%v", processed, store.cancelled, store.succeeded, err)
	}
}

func TestWorkerCancellationWinsOverDeferredReschedule(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 23, 9, 5, 0, 0, time.UTC)
	store := &memoryStore{
		claim:           Claim{Job: contract.Job{ID: "job_cancel_deferred", Kind: "strategy.generate", OrganizationID: "org_1", Status: contract.JobRunning, CreatedAt: now, UpdatedAt: now, Cancellable: true, AttemptCount: 1, MaxAttempts: 2, Version: 2}, LockOwner: "worker_1"},
		cancelRequested: true,
	}
	worker := Worker{
		Store: store, Canceller: store,
		Handlers: map[string]Handler{
			"strategy.generate": func(context.Context, Claim) (Result, error) {
				return Result{}, DeferredError{AvailableAt: now.Add(time.Minute)}
			},
		},
		Now: func() time.Time { return now },
	}
	processed, err := worker.RunOnce(context.Background(), "worker_1")
	if err != nil || !processed || !store.cancelled || store.rescheduled || store.failed {
		t.Fatalf("processed=%v cancelled=%v rescheduled=%v failed=%v err=%v", processed, store.cancelled, store.rescheduled, store.failed, err)
	}
}

func TestWorkerReschedulesDeferredJob(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 22, 1, 0, 0, 0, time.UTC)
	availableAt := now.Add(30 * time.Second)
	store := &memoryStore{claim: Claim{Job: contract.Job{ID: "job_2", Kind: "provider.image.generate", OrganizationID: "org_1", Status: contract.JobRunning, CreatedAt: now, UpdatedAt: now, Cancellable: true, AttemptCount: 1, MaxAttempts: 3, Version: 2}, LockOwner: "worker_1"}}
	worker := Worker{
		Store: store,
		Handlers: map[string]Handler{
			"provider.image.generate": func(context.Context, Claim) (Result, error) {
				return Result{}, DeferredError{AvailableAt: availableAt}
			},
		},
		Now: func() time.Time { return now },
	}

	processed, err := worker.RunOnce(context.Background(), "worker_1")
	if err != nil || !processed {
		t.Fatalf("RunOnce() processed=%v err=%v", processed, err)
	}
	if !store.rescheduled || !store.availableAt.Equal(availableAt) || store.failed || store.succeeded {
		t.Fatalf("deferred job state = rescheduled:%v available_at:%v failed:%v succeeded:%v", store.rescheduled, store.availableAt, store.failed, store.succeeded)
	}
}

func TestWorkerFailsDeferredJobAtAttemptLimit(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 22, 1, 5, 0, 0, time.UTC)
	store := &memoryStore{claim: Claim{Job: contract.Job{ID: "job_3", Kind: "provider.image.execute", OrganizationID: "org_1", Status: contract.JobRunning, CreatedAt: now, UpdatedAt: now, Cancellable: false, AttemptCount: 3, MaxAttempts: 3, Version: 4}, LockOwner: "worker_1"}}
	worker := Worker{
		Store: store,
		Handlers: map[string]Handler{
			"provider.image.execute": func(context.Context, Claim) (Result, error) {
				return Result{}, DeferredError{AvailableAt: now.Add(30 * time.Second)}
			},
		},
		Now: func() time.Time { return now },
	}

	processed, err := worker.RunOnce(context.Background(), "worker_1")
	if err != nil || !processed || !store.failed || store.rescheduled {
		t.Fatalf("RunOnce() processed=%v failed=%v rescheduled=%v err=%v", processed, store.failed, store.rescheduled, err)
	}
	if store.problem.Code != "JOB_ATTEMPT_LIMIT_EXCEEDED" || store.problem.Retryable {
		t.Fatalf("failure problem = %+v", store.problem)
	}
}

type memoryStore struct {
	claim           Claim
	claimed         bool
	succeeded       bool
	failed          bool
	rescheduled     bool
	availableAt     time.Time
	problem         contract.JobError
	cancelRequested bool
	cancelled       bool
}

func (s *memoryStore) Enqueue(context.Context, CreateRequest) (contract.Job, bool, error) {
	return contract.Job{}, false, nil
}
func (s *memoryStore) Claim(context.Context, string, time.Time) (Claim, bool, error) {
	if s.claimed {
		return Claim{}, false, nil
	}
	s.claimed = true
	return s.claim, true, nil
}
func (s *memoryStore) Succeed(context.Context, Claim, Result, time.Time) error {
	s.succeeded = true
	return nil
}
func (s *memoryStore) Fail(_ context.Context, _ Claim, problem contract.JobError, _ time.Time) error {
	s.failed = true
	s.problem = problem
	return nil
}
func (s *memoryStore) Reschedule(_ context.Context, _ Claim, availableAt time.Time, _ time.Time) error {
	s.rescheduled = true
	s.availableAt = availableAt
	return nil
}
func (s *memoryStore) IsCancelRequested(context.Context, contract.OrganizationID, string) (bool, error) {
	return s.cancelRequested, nil
}
func (s *memoryStore) CancelClaim(context.Context, Claim, time.Time) error {
	s.cancelled = true
	return nil
}
