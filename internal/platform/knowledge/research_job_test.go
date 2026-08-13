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
	request := store.requests[0]
	if request.Job.Kind != ResearchJobKind || request.Job.ID != "job_research_1" {
		t.Fatalf("unexpected job: %#v", request.Job)
	}
	if string(request.Payload) != `{"research_run_id":"researchrun_1"}` {
		t.Fatalf("payload leaked more than the run reference: %s", request.Payload)
	}
}

func TestKnowledgeRetrySchedulersCreateFreshAttempts(t *testing.T) {
	now := time.Date(2026, time.August, 10, 9, 0, 0, 0, time.UTC)
	store := &capturingResearchJobStore{}
	ids := []string{"researchjob_retry_1", "documentjob_retry_1"}
	nextID := func() (string, error) {
		value := ids[0]
		ids = ids[1:]
		return value, nil
	}
	researchScheduler := JobRuntimeResearchScheduler{Store: store, NewID: nextID, Now: func() time.Time { return now }}
	documentScheduler := JobRuntimeDocumentParseScheduler{Store: store, NewID: nextID, Now: func() time.Time { return now }}
	if err := researchScheduler.ScheduleResearchRetry(context.Background(), ResearchRun{
		ID: "researchrun_1", OrganizationID: "org_1", ProjectID: "project_1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := documentScheduler.ScheduleDocumentParseRetry(context.Background(), Document{
		ID: "document_1", OrganizationID: "org_1", ProjectID: "project_1",
	}); err != nil {
		t.Fatal(err)
	}
	if got := string(store.requests[0].IdempotencyKey); got != "knowledge_research_researchrun_1_retry_researchjob_retry_1" {
		t.Fatalf("research retry idempotency key=%q", got)
	}
	if got := string(store.requests[1].IdempotencyKey); got != "knowledge_parse_document_1_retry_documentjob_retry_1" {
		t.Fatalf("document retry idempotency key=%q", got)
	}
	if string(store.requests[0].Payload) != `{"research_run_id":"researchrun_1"}` ||
		string(store.requests[1].Payload) != `{"document_id":"document_1"}` {
		t.Fatalf("retry payloads must remain opaque references: %#v", store.requests)
	}
}

type capturingResearchJobStore struct {
	requests []jobruntime.CreateRequest
}

func (s *capturingResearchJobStore) Enqueue(_ context.Context, request jobruntime.CreateRequest) (contract.Job, bool, error) {
	s.requests = append(s.requests, request)
	return request.Job, false, nil
}
