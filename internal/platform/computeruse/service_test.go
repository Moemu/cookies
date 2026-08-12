package computeruse

import (
	"context"
	"testing"
	"time"
)

func TestAuthorizeActionConsumesConfirmationExactlyOnce(t *testing.T) {
	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	repo := NewMemoryRepository()
	ids := 0
	service := Service{Repository: repo, Now: func() time.Time { return now }, NewID: func(prefix string) (string, error) { ids++; return prefix + "_id", nil }}
	run := validRun(now)
	if _, _, err := service.CreateRun(context.Background(), CreateRunRequest{Run: run}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AcquireLease(context.Background(), validLease(now)); err != nil {
		t.Fatal(err)
	}
	run, err := service.TransitionRun(context.Background(), "org_1", "project_1", run.ID, 1, RunEnvironmentCheck, "")
	if err != nil {
		t.Fatal(err)
	}
	run, err = service.TransitionRun(context.Background(), "org_1", "project_1", run.ID, 2, RunPreparing, "")
	if err != nil {
		t.Fatal(err)
	}
	run, err = service.TransitionRun(context.Background(), "org_1", "project_1", run.ID, 3, RunAwaitingConfirmation, BlockFinalConfirmationRequired)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := service.IssueFinalConfirmation(context.Background(), "org_1", "project_1", run.ID, run.Authority.ApprovalActionHash, "operator_1")
	if err != nil {
		t.Fatal(err)
	}
	req := AuthorizeActionRequest{OrganizationID: "org_1", ProjectID: "project_1", RunID: run.ID, StepID: "step_1", ConfirmationID: issued.Confirmation.ID, Token: issued.Token, LeaseID: "lease_1", FencingToken: 1, IdempotencyKey: "submit_1"}
	if _, err := service.AuthorizeAction(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	req.IdempotencyKey = "submit_2"
	if _, err := service.AuthorizeAction(context.Background(), req); err != ErrConfirmationInvalid {
		t.Fatalf("second consume err=%v", err)
	}
}

func TestAuthorizeActionFailsClosedForKillSwitchAndStaleFence(t *testing.T) {
	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	repo := NewMemoryRepository()
	service := Service{Repository: repo, Now: func() time.Time { return now }, NewID: func(prefix string) (string, error) { return prefix + "_id", nil }}
	run := validRun(now)
	run.State = RunAwaitingConfirmation
	repo.runs[scopeKey("org_1", "project_1", run.ID)] = run
	lease := validLease(now)
	repo.leases[scopeKey("org_1", "project_1", lease.ID)] = lease
	repo.killSwitches[killKey(KillSwitchGlobal, "*")] = KillSwitch{ID: "kill_1", Scope: KillSwitchGlobal, Active: true, Version: 1}
	_, err := service.AuthorizeAction(context.Background(), AuthorizeActionRequest{OrganizationID: "org_1", ProjectID: "project_1", RunID: run.ID, LeaseID: lease.ID, FencingToken: 1})
	if err != ErrKillSwitchActive {
		t.Fatalf("kill switch err=%v", err)
	}
	delete(repo.killSwitches, killKey(KillSwitchGlobal, "*"))
	_, err = service.AuthorizeAction(context.Background(), AuthorizeActionRequest{OrganizationID: "org_1", ProjectID: "project_1", RunID: run.ID, LeaseID: lease.ID, FencingToken: 2})
	if err != ErrLeaseUnavailable {
		t.Fatalf("fence err=%v", err)
	}
}

func validRun(now time.Time) ComputerUseRun {
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	a := AuthorityBinding{SchemaVersion: AuthoritySchemaV1, OrganizationID: "org_1", ProjectID: "project_1", BusinessExecutionID: "exec_1", ChangeSetID: "change_1", ApprovalID: "approval_1", ApprovalActionHash: hash, AccountReferenceID: "account_1", ObjectFingerprint: hash, Action: "create_project_and_promotions", BudgetLimitMinor: 100, Currency: "CNY", PlanCanonicalHash: hash, IntentCanonicalHash: hash, FeedbackCanonicalHash: hash, DecisionCanonicalHash: hash, ConfigurationCanonicalHash: hash, WorkflowID: "workflow_1", WorkflowCanonicalHash: hash, WorkflowStepID: "step_1", SkillID: "oceanengine-ecommerce-manual", SkillVersion: "v1"}
	return ComputerUseRun{SchemaVersion: RunSchemaV1, ID: "run_1", OrganizationID: "org_1", ProjectID: "project_1", Platform: PlatformOceanEngine, AccountID: "account_1", Authority: a, EnvironmentID: "env_1", ProfileID: "profile_1", PolicyID: "policy_1", State: RunQueued, Version: 1, IdempotencyKey: "run_key", RequestHash: hash, CreatedBy: "user_1", CreatedAt: now, UpdatedAt: now}
}

func validLease(now time.Time) SessionLease {
	return SessionLease{ID: "lease_1", OrganizationID: "org_1", ProjectID: "project_1", RunID: "run_1", EnvironmentID: "env_1", ProfileID: "profile_1", Platform: PlatformOceanEngine, AccountID: "account_1", Holder: "worker_1", FencingToken: 1, Version: 1, ExpiresAt: now.Add(time.Hour), HeartbeatDeadline: now.Add(time.Minute)}
}
