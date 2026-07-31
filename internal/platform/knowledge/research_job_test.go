package knowledge

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
)

func TestJobRuntimeResearchSchedulerQueuesOpaqueRunReference(t *testing.T) {
	now := time.Date(2026, time.July, 29, 8, 0, 0, 0, time.UTC)
	store := &capturingResearchJobStore{}
	scheduler := JobRuntimeResearchScheduler{
		Store: store, NewID: func() (string, error) { return "job_research_1", nil },
		Now: func() time.Time { return now },
	}
	err := scheduler.Schedule(context.Background(), ResearchRun{
		ID: "researchrun_1", OrganizationID: "org_1", ProjectID: "project_1",
	})
	if err != nil {
		t.Fatalf("schedule research: %v", err)
	}
	if store.request.Job.Kind != ResearchJobKind || store.request.Job.ID != "job_research_1" {
		t.Fatalf("unexpected job: %#v", store.request.Job)
	}
	if string(store.request.Payload) != `{"research_run_id":"researchrun_1"}` {
		t.Fatalf("payload leaked more than the run reference: %s", store.request.Payload)
	}
}

type capturingResearchJobStore struct {
	request jobruntime.CreateRequest
}

func (s *capturingResearchJobStore) Enqueue(_ context.Context, request jobruntime.CreateRequest) (contract.Job, bool, error) {
	s.request = request
	return request.Job, false, nil
}
