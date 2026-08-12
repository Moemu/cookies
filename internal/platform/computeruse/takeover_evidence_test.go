package computeruse

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestRecordTakeoverEvidenceRequiresFencedLeasePolicyAndRedacts(t *testing.T) {
	now := time.Date(2026, 8, 12, 17, 30, 0, 0, time.UTC)
	repo := NewMemoryRepository()
	run := validRun(now)
	run.State = RunAwaitingTakeover
	run.Paused = true
	run.TakeoverActive = true
	run.PolicyID = "policy_live_1"
	_, _, err := repo.CreateRun(context.Background(), run)
	if err != nil {
		t.Fatal(err)
	}
	repo.PutSitePolicy(SitePolicy{ID: run.PolicyID, OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, Platform: run.Platform, AccountID: run.AccountID, AllowedProtocols: []string{"https"}, AllowedHosts: []string{"ad.oceanengine.com"}, AllowedPageKinds: []string{"project_create"}, AllowedPlatformProjects: []string{"test-project-1"}, Version: 1})
	idSequence := 0
	service := Service{Repository: repo, Now: func() time.Time { return now }, NewID: func(prefix string) (string, error) {
		idSequence++
		return fmt.Sprintf("%s_live_%d", prefix, idSequence), nil
	}}
	acquired, err := service.AcquireRunLease(context.Background(), run.OrganizationID, run.ProjectID, run.ID, run.Version, "agent")
	if err != nil {
		t.Fatal(err)
	}
	lease := acquired.Lease
	result, err := service.RecordTakeoverEvidence(context.Background(), RecordTakeoverEvidenceRequest{OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, RunID: run.ID, ExpectedVersion: acquired.Run.Version, LeaseID: lease.ID, FencingToken: lease.FencingToken, StepID: "step_observe_1", Sequence: 1, Action: TakeoverFieldReadback, Status: StepSucceeded, PageKind: "project_create", PlatformProjectID: "test-project-1", BeforePageFacts: map[string]string{"account_balance": "0.00", "page_kind": "project_create"}, AfterPageFacts: map[string]string{"page_kind": "project_create"}, FieldReadback: map[string]string{"daily_budget": "300", "customer_name": "sensitive"}, PageReference: "https://ad.oceanengine.com/superior/create-project?aadvid=sensitive", SelectorVersion: "oceanengine-live-locators/v0.1", ActionVersion: "takeover-readback/v1", Actor: "operator_1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Run.Version != 3 || result.Step.Action != string(TakeoverFieldReadback) || result.Evidence.RedactionVersion != "computer-use-redaction/v1" {
		t.Fatalf("result=%#v", result)
	}
	if result.Evidence.BeforePageFacts["account_balance"] != redactedValue || result.Evidence.FieldReadback["customer_name"] != redactedValue || result.Evidence.PageReference != "https://ad.oceanengine.com/superior/create-project" {
		t.Fatalf("evidence was not redacted: %#v", result.Evidence)
	}
	if values, _ := repo.ListEvidence(context.Background(), run.OrganizationID, run.ProjectID, run.ID); len(values) != 1 {
		t.Fatalf("evidence=%#v", values)
	}
	if events, _ := repo.ListEvents(context.Background(), run.OrganizationID, run.ProjectID, run.ID); len(events) != 2 || events[1].Kind != "takeover_evidence" {
		t.Fatalf("events=%#v", events)
	}
}

func TestRecordTakeoverEvidenceRejectsWriteActionAndPolicyDrift(t *testing.T) {
	now := time.Date(2026, 8, 12, 17, 30, 0, 0, time.UTC)
	repo := NewMemoryRepository()
	run := validRun(now)
	run.State, run.Paused, run.TakeoverActive, run.LeaseID, run.PolicyID = RunAwaitingTakeover, true, true, "lease_1", "policy_1"
	_, _, _ = repo.CreateRun(context.Background(), run)
	repo.PutSitePolicy(SitePolicy{ID: run.PolicyID, OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, Platform: run.Platform, AccountID: run.AccountID, AllowedProtocols: []string{"https"}, AllowedHosts: []string{"ad.oceanengine.com"}, AllowedPageKinds: []string{"project_create"}, AllowedPlatformProjects: []string{"test-project-1"}, Version: 1})
	_, _ = repo.AcquireLease(context.Background(), SessionLease{ID: run.LeaseID, OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, RunID: run.ID, EnvironmentID: run.EnvironmentID, ProfileID: run.ProfileID, Platform: run.Platform, AccountID: run.AccountID, Holder: "agent", FencingToken: 1, Version: 1, ExpiresAt: now.Add(time.Hour), HeartbeatDeadline: now.Add(time.Minute)})
	service := Service{Repository: repo, Now: func() time.Time { return now }}
	base := RecordTakeoverEvidenceRequest{OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, RunID: run.ID, ExpectedVersion: 1, LeaseID: run.LeaseID, FencingToken: 1, StepID: "step", Sequence: 1, Action: TakeoverObservePage, Status: StepSucceeded, PageKind: "project_create", PlatformProjectID: "test-project-1", PageReference: "https://evil.example/project", SelectorVersion: "v1", ActionVersion: "v1", Actor: "operator"}
	if _, err := service.RecordTakeoverEvidence(context.Background(), base); err != ErrInvalidContract {
		t.Fatalf("policy drift err=%v", err)
	}
	base.PageReference = "https://ad.oceanengine.com/project"
	base.Action = TakeoverEvidenceAction("submit")
	if _, err := service.RecordTakeoverEvidence(context.Background(), base); err != ErrInvalidContract {
		t.Fatalf("write action err=%v", err)
	}
}
