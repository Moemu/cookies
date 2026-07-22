package jobruntime

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestRecoveryRunnerReclaimsAtConfiguredIntervalBeforeProcessing(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 22, 10, 0, 0, 0, time.UTC)
	store := &recoveryRunnerStore{}
	runner := RecoveryRunner{
		Worker: Worker{Store: store}, Recoverer: store, WorkerID: "worker_1",
		LeaseDuration: time.Minute, RecoveryInterval: 30 * time.Second,
		Now: func() time.Time { return now },
	}

	processed, err := runner.RunOnce(context.Background())
	if err != nil || processed || store.reclaimCalls != 1 || store.claimCalls != 1 {
		t.Fatalf("first RunOnce() = (processed=%v, err=%v), reclaim=%d claim=%d", processed, err, store.reclaimCalls, store.claimCalls)
	}

	processed, err = runner.RunOnce(context.Background())
	if err != nil || processed || store.reclaimCalls != 1 || store.claimCalls != 2 {
		t.Fatalf("second RunOnce() = (processed=%v, err=%v), reclaim=%d claim=%d", processed, err, store.reclaimCalls, store.claimCalls)
	}

	now = now.Add(30 * time.Second)
	processed, err = runner.RunOnce(context.Background())
	if err != nil || processed || store.reclaimCalls != 2 || store.claimCalls != 3 {
		t.Fatalf("interval RunOnce() = (processed=%v, err=%v), reclaim=%d claim=%d", processed, err, store.reclaimCalls, store.claimCalls)
	}
}

func TestRecoveryRunnerStopsWhenLeaseRecoveryFails(t *testing.T) {
	t.Parallel()
	store := &recoveryRunnerStore{reclaimErr: errRecoveryUnavailable{}}
	runner := RecoveryRunner{
		Worker: Worker{Store: store}, Recoverer: store, WorkerID: "worker_1",
		LeaseDuration: time.Minute, RecoveryInterval: time.Minute,
	}

	processed, err := runner.RunOnce(context.Background())
	if err == nil || processed || store.claimCalls != 0 {
		t.Fatalf("RunOnce() = (processed=%v, err=%v), claim=%d", processed, err, store.claimCalls)
	}
}

type recoveryRunnerStore struct {
	reclaimCalls int
	claimCalls   int
	reclaimErr   error
}

func (s *recoveryRunnerStore) ReclaimExpired(context.Context, time.Time, time.Duration) (LeaseRecovery, error) {
	s.reclaimCalls++
	return LeaseRecovery{}, s.reclaimErr
}

func (s *recoveryRunnerStore) Enqueue(context.Context, CreateRequest) (contract.Job, bool, error) {
	return contract.Job{}, false, nil
}

func (s *recoveryRunnerStore) Claim(context.Context, string, time.Time) (Claim, bool, error) {
	s.claimCalls++
	return Claim{}, false, nil
}

func (*recoveryRunnerStore) Succeed(context.Context, Claim, Result, time.Time) error { return nil }
func (*recoveryRunnerStore) Fail(context.Context, Claim, contract.JobError, time.Time) error {
	return nil
}
func (*recoveryRunnerStore) Reschedule(context.Context, Claim, time.Time, time.Time) error {
	return nil
}

type errRecoveryUnavailable struct{}

func (errRecoveryUnavailable) Error() string { return "lease recovery unavailable" }
