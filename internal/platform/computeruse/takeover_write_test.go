package computeruse

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
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
	if attempt := repo.attempts[scopeKey(run.OrganizationID, run.ProjectID, "final-click-1")]; attempt.Status != ControlledActionVerified {
		t.Fatalf("attempt status=%q", attempt.Status)
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
	if attempt := service.Repository.(*MemoryRepository).attempts[scopeKey(run.OrganizationID, run.ProjectID, "final-click-1")]; attempt.Status != ControlledActionResultUnknown {
		t.Fatalf("attempt status=%q", attempt.Status)
	}
	request.ExpectedVersion = unknown.Run.Version
	request.IdempotencyKey = "final-click-2"
	if _, err := service.AuthorizeTakeoverAction(context.Background(), request); err != ErrConfirmationInvalid {
		t.Fatalf("resubmit err=%v", err)
	}
}

func TestTakeoverWriteOutcomeCanUseReacquiredRecoveryLease(t *testing.T) {
	now := time.Date(2026, 8, 14, 1, 0, 0, 0, time.UTC)
	repo := NewMemoryRepository()
	run := validRun(now)
	run.State, run.Paused, run.TakeoverActive, run.LeaseID = RunAwaitingTakeover, true, true, "lease_1"
	run.PolicyID = "policy_1"
	if _, _, err := repo.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	originalLease := validLease(now)
	if _, err := repo.AcquireLease(context.Background(), originalLease); err != nil {
		t.Fatal(err)
	}
	repo.PutSitePolicy(SitePolicy{ID: run.PolicyID, OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, Platform: run.Platform, AccountID: run.AccountID, AllowedProtocols: []string{"https"}, AllowedHosts: []string{"ad.oceanengine.com"}, AllowedPageKinds: []string{"project_create", "project_result"}, AllowedPlatformProjects: []string{"test-project-1"}, Version: 1})
	sequence := 0
	service := Service{Repository: repo, Now: func() time.Time { return now }, NewID: func(prefix string) (string, error) { sequence++; return fmt.Sprintf("%s_%d", prefix, sequence), nil }}

	issued, err := service.IssueFinalConfirmation(context.Background(), run.OrganizationID, run.ProjectID, run.ID, run.Version, run.Authority.ApprovalActionHash, "operator_1")
	if err != nil {
		t.Fatal(err)
	}
	authorized, err := service.AuthorizeTakeoverAction(context.Background(), AuthorizeTakeoverActionRequest{OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, RunID: run.ID, ExpectedVersion: run.Version, StepID: "step_submit", Sequence: 1, ConfirmationID: issued.Confirmation.ID, Token: issued.Token, LeaseID: originalLease.ID, FencingToken: originalLease.FencingToken, IdempotencyKey: "final-click-recovery", PageKind: "project_create", PlatformProjectID: "test-project-1", FieldReadback: map[string]string{"daily_budget": "300"}, DiffKeys: []string{}, PageReference: "https://ad.oceanengine.com/project/create", SelectorVersion: "live/v1", ActionVersion: "takeover-submit/v1", Actor: "operator_1"})
	if err != nil {
		t.Fatal(err)
	}

	now = now.Add(SessionHeartbeatTTL + time.Second)
	released, err := service.ReleaseRunLease(context.Background(), run.OrganizationID, run.ProjectID, run.ID, originalLease.ID, authorized.Run.Version, originalLease.Version, originalLease.FencingToken)
	if err != nil {
		t.Fatal(err)
	}
	reacquired, err := service.AcquireRunLease(context.Background(), run.OrganizationID, run.ProjectID, run.ID, released.Run.Version, "recovery_operator")
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.RecordTakeoverOutcome(context.Background(), RecordTakeoverOutcomeRequest{OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, RunID: run.ID, AttemptID: authorized.Attempt.ID, ExpectedVersion: reacquired.Run.Version, LeaseID: reacquired.Lease.ID, FencingToken: reacquired.Lease.FencingToken, StepID: "step_result", Sequence: 2, Outcome: TakeoverResultObserved, PageKind: "project_result", PlatformProjectID: "test-project-1", FieldReadback: map[string]string{"platform_object_id": "platform-1", "platform_status": "pending_review"}, PageReference: "https://ad.oceanengine.com/project/result", SelectorVersion: "live/v1", ActionVersion: "takeover-result/v1", Actor: "recovery_operator"})
	if err != nil || result.Run.State != RunVerifying {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	stored := repo.attempts[scopeKey(run.OrganizationID, run.ProjectID, "final-click-recovery")]
	if stored.LeaseID != originalLease.ID || stored.FencingToken != originalLease.FencingToken || stored.Status != ControlledActionVerified {
		t.Fatalf("attempt=%#v", stored)
	}
}

func TestPromotionMutationTakeoverRequiresExactObjectAndStateReadback(t *testing.T) {
	now := time.Date(2026, 8, 14, 4, 0, 0, 0, time.UTC)
	repo := NewMemoryRepository()
	run := validRun(now)
	currentHash, _ := contract.CanonicalJSONHash(struct {
		DailyBudgetMinor int64 `json:"daily_budget_minor"`
	}{30000})
	targetHash, _ := contract.CanonicalJSONHash(struct {
		DailyBudgetMinor int64 `json:"daily_budget_minor"`
	}{36000})
	run.Authority.Action = "update_promotion_budget"
	run.Authority.ParentPlatformProjectID = "test-project-1"
	run.Authority.TargetMappingID = "mapping_test"
	run.Authority.TargetMappingVersion = 2
	run.Authority.TargetPlatformObjectID = "promotion_test"
	run.Authority.TargetPlatformObjectKind = "promotion"
	run.Authority.PromotionBudgetLimitMinor = 36000
	run.Authority.BudgetLimitMinor = 36000
	run.Authority.PromotionMutation = &PromotionMutationBinding{CurrentDailyBudgetMinor: 30000, TargetDailyBudgetMinor: 36000, CurrentStateHash: currentHash, TargetStateHash: targetHash}
	run.State, run.Paused, run.TakeoverActive, run.LeaseID = RunAwaitingTakeover, true, true, "lease_1"
	run.PolicyID = "policy_1"
	if err := run.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	lease := validLease(now)
	if _, err := repo.AcquireLease(context.Background(), lease); err != nil {
		t.Fatal(err)
	}
	repo.PutSitePolicy(SitePolicy{ID: run.PolicyID, OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, Platform: run.Platform, AccountID: run.AccountID, AllowedProtocols: []string{"https"}, AllowedHosts: []string{"ad.oceanengine.com"}, AllowedPageKinds: []string{"promotion_edit", "promotion_result", "promotion_list"}, AllowedPlatformProjects: []string{"test-project-1"}, Version: 1})
	sequence := 0
	service := Service{Repository: repo, Now: func() time.Time { return now }, NewID: func(prefix string) (string, error) { sequence++; return fmt.Sprintf("%s_%d", prefix, sequence), nil }}
	issued, err := service.IssueFinalConfirmation(context.Background(), run.OrganizationID, run.ProjectID, run.ID, run.Version, run.Authority.ApprovalActionHash, "operator_1")
	if err != nil {
		t.Fatal(err)
	}
	request := AuthorizeTakeoverActionRequest{OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, RunID: run.ID, ExpectedVersion: run.Version, StepID: "step_mutation_submit", Sequence: 1, ConfirmationID: issued.Confirmation.ID, Token: issued.Token, LeaseID: lease.ID, FencingToken: lease.FencingToken, IdempotencyKey: "test", PageKind: "promotion_edit", PlatformProjectID: "test-project-1", FieldReadback: map[string]string{"platform_object_id": "promotion_test", "current_state_hash": currentHash, "target_state_hash": currentHash}, DiffKeys: []string{}, PageReference: "https://ad.oceanengine.com/promotion/edit", SelectorVersion: "live/v1", ActionVersion: "takeover-mutation/v1", Actor: "operator_1"}
	if _, err := service.AuthorizeTakeoverAction(context.Background(), request); err != ErrInvalidContract {
		t.Fatalf("target state drift err=%v", err)
	}
	request.FieldReadback["target_state_hash"] = targetHash
	authorized, err := service.AuthorizeTakeoverAction(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	resultRequest := RecordTakeoverOutcomeRequest{OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, RunID: run.ID, AttemptID: authorized.Attempt.ID, ExpectedVersion: authorized.Run.Version, LeaseID: lease.ID, FencingToken: lease.FencingToken, StepID: "step_mutation_result", Sequence: 2, Outcome: TakeoverResultObserved, PageKind: "promotion_result", PlatformProjectID: "test-project-1", FieldReadback: map[string]string{"platform_object_id": "promotion_other", "platform_status": "pending_review", "target_state_hash": targetHash}, PageReference: "https://ad.oceanengine.com/promotion/result", SelectorVersion: "live/v1", ActionVersion: "takeover-mutation-result/v1", Actor: "operator_1"}
	if _, err := service.RecordTakeoverOutcome(context.Background(), resultRequest); err != ErrInvalidContract {
		t.Fatalf("wrong promotion outcome err=%v", err)
	}
	resultRequest.FieldReadback["platform_object_id"] = "promotion_test"
	result, err := service.RecordTakeoverOutcome(context.Background(), resultRequest)
	if err != nil || result.Run.State != RunVerifying {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	listRequest := RecordTakeoverOutcomeRequest{OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, RunID: run.ID, AttemptID: authorized.Attempt.ID, ExpectedVersion: result.Run.Version, LeaseID: lease.ID, FencingToken: lease.FencingToken, StepID: "step_mutation_list", Sequence: 3, Outcome: TakeoverListConfirmed, PageKind: "promotion_list", PlatformProjectID: "test-project-1", FieldReadback: map[string]string{"platform_object_id": "promotion_test", "platform_status": "pending_review", "target_state_hash": currentHash}, PageReference: "https://ad.oceanengine.com/promotion/list", SelectorVersion: "live/v1", ActionVersion: "takeover-mutation-list/v1", Actor: "operator_1"}
	if _, err := service.RecordTakeoverOutcome(context.Background(), listRequest); err != ErrInvalidContract {
		t.Fatalf("list target drift err=%v", err)
	}
	listRequest.FieldReadback["target_state_hash"] = targetHash
	confirmed, err := service.RecordTakeoverOutcome(context.Background(), listRequest)
	if err != nil || confirmed.Run.State != RunSucceeded {
		t.Fatalf("confirmed=%#v err=%v", confirmed, err)
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
