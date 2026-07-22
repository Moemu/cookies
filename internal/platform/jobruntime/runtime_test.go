package jobruntime

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

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

type memoryStore struct {
	claim       Claim
	claimed     bool
	succeeded   bool
	failed      bool
	rescheduled bool
	availableAt time.Time
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
func (s *memoryStore) Fail(context.Context, Claim, contract.JobError, time.Time) error {
	s.failed = true
	return nil
}
func (s *memoryStore) Reschedule(_ context.Context, _ Claim, availableAt time.Time, _ time.Time) error {
	s.rescheduled = true
	s.availableAt = availableAt
	return nil
}
