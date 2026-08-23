package connector

import (
	"context"
	"testing"
	"time"
)

type patrolSessionSource struct{ values []OceanEngineAccountSession }

func (s patrolSessionSource) ListReadyAccountSessions(context.Context, int) ([]OceanEngineAccountSession, error) {
	return s.values, nil
}

type patrolSyncRecorder struct{ requests []SyncRequest }

func (s *patrolSyncRecorder) Sync(_ context.Context, request SyncRequest) (SyncResult, error) {
	s.requests = append(s.requests, request)
	return SyncResult{}, nil
}

func TestPatrolRunnerUsesMetricsOnlyLookback(t *testing.T) {
	now := time.Date(2026, 8, 21, 6, 0, 0, 0, time.UTC)
	syncer := &patrolSyncRecorder{}
	runner := PatrolRunner{Sessions: patrolSessionSource{values: []OceanEngineAccountSession{{OrganizationID: "org_1", ProjectID: "project_1", AccountID: "oeacct_local"}}}, Syncer: syncer, Now: func() time.Time { return now }, LookbackDays: 14}
	result, err := runner.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Completed != 1 || len(syncer.requests) != 1 {
		t.Fatalf("result=%#v requests=%d", result, len(syncer.requests))
	}
	request := syncer.requests[0]
	if request.Mode != SyncModeMetricsOnly || request.ProjectID != "project_1" || !request.WindowEnd.Equal(now) || !request.WindowStart.Equal(now.AddDate(0, 0, -14)) {
		t.Fatalf("request=%#v", request)
	}
}
