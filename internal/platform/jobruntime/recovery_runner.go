package jobruntime

import (
	"context"
	"fmt"
	"time"
)

// RecoveryRunner keeps durable worker execution recoverable after a process
// crash. It periodically returns expired running leases to the queue before
// claiming the next available job; the domain worker remains responsible for
// normal execution and retry decisions.
//
// A Runner instance is deliberately stateful and must be owned by one process
// loop. Multiple processes may use independent instances against the same
// Store: ReclaimExpired is safe to call concurrently.
type RecoveryRunner struct {
	Worker           Worker
	Recoverer        LeaseRecoverer
	WorkerID         string
	LeaseDuration    time.Duration
	RecoveryInterval time.Duration
	Now              func() time.Time

	lastRecovery time.Time
}

// RunOnce performs scheduled lease recovery, then processes at most one job.
// It returns the Worker result so callers can retain their own polling and
// backoff policy.
func (r *RecoveryRunner) RunOnce(ctx context.Context) (bool, error) {
	if r.Recoverer == nil {
		return false, fmt.Errorf("lease recoverer is required")
	}
	if r.Worker.Store == nil {
		return false, fmt.Errorf("job worker store is required")
	}
	if r.WorkerID == "" {
		return false, fmt.Errorf("worker ID is required")
	}
	if r.LeaseDuration <= 0 || r.RecoveryInterval <= 0 {
		return false, fmt.Errorf("lease duration and recovery interval must be positive")
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	current := now().UTC()
	if r.lastRecovery.IsZero() || !current.Before(r.lastRecovery.Add(r.RecoveryInterval)) {
		if _, err := r.Recoverer.ReclaimExpired(ctx, current, r.LeaseDuration); err != nil {
			return false, fmt.Errorf("reclaim expired job leases: %w", err)
		}
		r.lastRecovery = current
	}
	return r.Worker.RunOnce(ctx, r.WorkerID)
}
