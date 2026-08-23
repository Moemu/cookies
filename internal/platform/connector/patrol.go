package connector

import (
	"context"
	"fmt"
	"time"
)

type PatrolSyncer interface {
	Sync(context.Context, SyncRequest) (SyncResult, error)
}

type PatrolRunner struct {
	Sessions     ReadyAccountSessionSource
	Syncer       PatrolSyncer
	Now          func() time.Time
	LookbackDays int
	AccountLimit int
	Timeout      time.Duration
}

type PatrolResult struct {
	AccountCount int
	Completed    int
}

func (r PatrolRunner) RunOnce(ctx context.Context) (PatrolResult, error) {
	if r.Sessions == nil || r.Syncer == nil {
		return PatrolResult{}, ErrInvalidFact
	}
	now := time.Now().UTC()
	if r.Now != nil {
		now = r.Now().UTC()
	}
	lookbackDays := r.LookbackDays
	if lookbackDays < 2 || lookbackDays > 30 {
		lookbackDays = 14
	}
	limit := r.AccountLimit
	if limit < 1 {
		limit = 100
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	sessions, err := r.Sessions.ListReadyAccountSessions(ctx, limit)
	if err != nil {
		return PatrolResult{}, err
	}
	result := PatrolResult{AccountCount: len(sessions)}
	for _, session := range sessions {
		accountCtx, cancel := context.WithTimeout(ctx, timeout)
		_, syncErr := r.Syncer.Sync(accountCtx, SyncRequest{
			OrganizationID: session.OrganizationID,
			ProjectID:      session.ProjectID,
			AccountRef:     session.AccountID,
			IdempotencyKey: fmt.Sprintf("daily-metric-patrol-v1:%s", now.Format("2006-01-02")),
			WindowStart:    now.AddDate(0, 0, -lookbackDays),
			WindowEnd:      now,
			TimeZone:       "Asia/Shanghai",
			Currency:       "CNY",
			Mode:           SyncModeMetricsOnly,
		})
		cancel()
		if syncErr != nil {
			return result, fmt.Errorf("patrol account %s: %w", opaqueRef(session.AccountID), syncErr)
		}
		result.Completed++
	}
	return result, nil
}
