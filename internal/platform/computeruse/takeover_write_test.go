package computeruse

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestTakeoverWriteConsumesConfirmationAndPersistsTwoPhaseResultEvidence(t *testing.T) {
	service, repo, run, lease := takeoverWriteFixture(t)
	issued, err := service.IssueFinalConfirmation(context.Background(), run.OrganizationID, run.ProjectID, run.ID, run.Version, run.Authority.ApprovalActionHash, "operator_1")
	if err != nil {
		t.Fatal(err)
	}
	authorized, err := service.AuthorizeTakeoverAction(context.Background(), AuthorizeTakeoverActionRequest{OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, RunID: run.ID, ExpectedVersion: run.Version, StepID: "step_submit", Sequence: 1, ConfirmationID: issued.Confirmation.ID, Token: issued.Token, LeaseID: lease.ID, FencingToken: lease.FencingToken, IdempotencyKey: "final-click-1", PageKind: "project_create", PlatformProjectID: "test-project-1", BeforePageFacts: map[string]string{"submit_enabled": "true"}, FieldReadback: map[string]string{"daily_budget": "300"}, DiffKeys: []string{}, PageReference: "https://ad.oceanengine.com/project/create", SelectorVersion: "live/v1", ActionVersion: "takeover-submit/v1", Actor: "operator_1"})
	if err != nil {
		t.Fatal(err)
	}
	if authorized.Run.State != RunSubmitting || !authorized.Run.Paused || !authorized.Run.TakeoverActive || authorized.Attempt.Status != "authorized" || authorized.Evidence.FieldReadback["daily_budget"] != "300" {
		t.Fatalf("authorization=%#v", authorized)
	}
	result, err := service.RecordTakeoverOutcome(context.Background(), RecordTakeoverOutcomeRequest{OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, RunID: run.ID, AttemptID: authorized.Attempt.ID, ExpectedVersion: authorized.Run.Version, LeaseID: lease.ID, FencingToken: lease.FencingToken, StepID: "step_result", Sequence: 2, Outcome: TakeoverResultObserved, PageKind: "project_result", PlatformProjectID: "test-project-1", FieldReadback: map[string]string{"platform_object_id": "platform-1", "platform_status": "pending_review"}, PageReference: "https://ad.oceanengine.com/project/result", SelectorVersion: "live/v1", ActionVersion: "takeover-result/v1", Actor: "operator_1"})
	if err != nil || result.Run.State != RunVerifying {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	mismatch := RecordTakeoverOutcomeRequest{OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, RunID: run.ID, AttemptID: authorized.Attempt.ID, ExpectedVersion: result.Run.Version, LeaseID: lease.ID, FencingToken: lease.FencingToken, StepID: "step_list_mismatch", Sequence: 3, Outcome: TakeoverListConfirmed, PageKind: "project_list", PlatformProjectID: "test-project-1", FieldReadback: map[string]string{"platform_object_id": "platform-2", "platform_status": "pending_review"}, PageReference: "https://ad.oceanengine.com/project/list", SelectorVersion: "live/v1", ActionVersion: "takeover-list/v1", Actor: "operator_1"}
	if _, err := service.RecordTakeoverOutcome(context.Background(), mismatch); err != ErrInvalidContract {
		t.Fatalf("mismatched list readback err=%v", err)
	}
	confirmed, err := service.RecordTakeoverOutcome(context.Background(), RecordTakeoverOutcomeRequest{OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, RunID: run.ID, AttemptID: authorized.Attempt.ID, ExpectedVersion: result.Run.Version, LeaseID: lease.ID, FencingToken: lease.FencingToken, StepID: "step_list", Sequence: 3, Outcome: TakeoverListConfirmed, PageKind: "project_list", PlatformProjectID: "test-project-1", FieldReadback: map[string]string{"platform_object_id": "platform-1", "platform_status": "pending_review"}, PageReference: "https://ad.oceanengine.com/project/list", SelectorVersion: "live/v1", ActionVersion: "takeover-list/v1", Actor: "operator_1"})
	if err != nil || confirmed.Run.State != RunSucceeded {
		t.Fatalf("confirmed=%#v err=%v", confirmed, err)
	}
	evidence, _ := repo.ListEvidence(context.Background(), run.OrganizationID, run.ProjectID, run.ID)
	if len(evidence) != 3 || evidence[0].ID == evidence[1].ID || evidence[1].ID == evidence[2].ID {
		t.Fatalf("evidence=%#v", evidence)
	}
}

func TestTakeoverWriteRejectsDriftAndUnknownResultCannotResubmit(t *testing.T) {
	service, _, run, lease := takeoverWriteFixture(t)
	issued, err := service.IssueFinalConfirmation(context.Background(), run.OrganizationID, run.ProjectID, run.ID, run.Version, run.Authority.ApprovalActionHash, "operator_1")
	if err != nil {
		t.Fatal(err)
	}
	request := AuthorizeTakeoverActionRequest{OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, RunID: run.ID, ExpectedVersion: run.Version, StepID: "step_submit", Sequence: 1, ConfirmationID: issued.Confirmation.ID, Token: issued.Token, LeaseID: lease.ID, FencingToken: lease.FencingToken, IdempotencyKey: "final-click-1", PageKind: "project_create", PlatformProjectID: "test-project-1", FieldReadback: map[string]string{"daily_budget": "300"}, DiffKeys: []string{"daily_budget"}, PageReference: "https://ad.oceanengine.com/project/create", SelectorVersion: "live/v1", ActionVersion: "takeover-submit/v1", Actor: "operator_1"}
	if _, err := service.AuthorizeTakeoverAction(context.Background(), request); err != ErrInvalidContract {
		t.Fatalf("drift err=%v", err)
	}
	request.DiffKeys = []string{}
	authorized, err := service.AuthorizeTakeoverAction(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	unknown, err := service.RecordTakeoverOutcome(context.Background(), RecordTakeoverOutcomeRequest{OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, RunID: run.ID, AttemptID: authorized.Attempt.ID, ExpectedVersion: authorized.Run.Version, LeaseID: lease.ID, FencingToken: lease.FencingToken, StepID: "step_unknown", Sequence: 2, Outcome: TakeoverResultUnknown, PageKind: "project_result", PlatformProjectID: "test-project-1", PageReference: "https://ad.oceanengine.com/project/result", SelectorVersion: "live/v1", ActionVersion: "takeover-result/v1", Actor: "operator_1"})
	if err != nil || unknown.Run.State != RunResultUnknown || unknown.Run.BlockingReason != BlockResultReconciliation {
		t.Fatalf("unknown=%#v err=%v", unknown, err)
	}
	request.ExpectedVersion = unknown.Run.Version
	request.IdempotencyKey = "final-click-2"
	if _, err := service.AuthorizeTakeoverAction(context.Background(), request); err != ErrConfirmationInvalid {
		t.Fatalf("resubmit err=%v", err)
	}
}

func takeoverWriteFixture(t *testing.T) (Service, *MemoryRepository, ComputerUseRun, SessionLease) {
	t.Helper()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	repo := NewMemoryRepository()
	run := validRun(now)
	run.State, run.Paused, run.TakeoverActive, run.LeaseID = RunAwaitingTakeover, true, true, "lease_1"
	run.PolicyID = "policy_1"
	if _, _, err := repo.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	lease := validLease(now)
	if _, err := repo.AcquireLease(context.Background(), lease); err != nil {
		t.Fatal(err)
	}
	repo.PutSitePolicy(SitePolicy{ID: run.PolicyID, OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, Platform: run.Platform, AccountID: run.AccountID, AllowedProtocols: []string{"https"}, AllowedHosts: []string{"ad.oceanengine.com"}, AllowedPageKinds: []string{"project_create", "project_result", "project_list"}, AllowedPlatformProjects: []string{"test-project-1"}, Version: 1})
	sequence := 0
	service := Service{Repository: repo, Now: func() time.Time { return now }, NewID: func(prefix string) (string, error) { sequence++; return fmt.Sprintf("%s_%d", prefix, sequence), nil }}
	return service, repo, run, lease
}
