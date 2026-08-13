package computeruse

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestMySQLAuthorizeControlledActionIsAtomic(t *testing.T) {
	dsn := os.Getenv("COOKIES_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("COOKIES_TEST_MYSQL_DSN is not configured")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	suffix := fmt.Sprint(time.Now().UnixNano())
	org := "cu_org_" + suffix
	project := "cu_project_" + suffix
	env := "cu_env_" + suffix
	profile := "cu_profile_" + suffix
	policy := "cu_policy_" + suffix
	if _, err = db.ExecContext(ctx, `INSERT INTO organizations (id,name,status) VALUES (?,?,'active')`, org, "computer-use integration"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO projects (id,organization_id,name,status) VALUES (?,?,?,'draft')`, project, org, "computer-use integration"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, statement := range []string{`DELETE FROM computer_use_controlled_action_attempts WHERE organization_id=?`, `DELETE FROM computer_use_final_confirmations WHERE organization_id=?`, `DELETE FROM computer_use_evidence WHERE organization_id=?`, `DELETE FROM computer_use_events WHERE organization_id=?`, `DELETE FROM computer_use_run_steps WHERE organization_id=?`, `UPDATE computer_use_runs SET lease_id=NULL WHERE organization_id=?`, `DELETE FROM computer_use_session_leases WHERE organization_id=?`, `DELETE FROM computer_use_runs WHERE organization_id=?`, `DELETE FROM computer_use_site_policies WHERE organization_id=?`, `DELETE FROM computer_use_browser_profiles WHERE organization_id=?`, `DELETE FROM computer_use_environments WHERE organization_id=?`, `DELETE FROM projects WHERE organization_id=?`, `DELETE FROM organizations WHERE id=?`} {
			_, _ = db.ExecContext(context.Background(), statement, org)
		}
	}()
	_, err = db.ExecContext(ctx, `INSERT INTO computer_use_environments (id,organization_id,project_id,platform,account_id,mode,browser_version,region,healthy,version) VALUES (?,?,?,'ocean_engine','account_1','local_visible','test','local',TRUE,1)`, env, org, project)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO computer_use_browser_profiles (id,organization_id,project_id,environment_id,platform,account_id,state,version) VALUES (?,?,?,?,'ocean_engine','account_1','ready',1)`, profile, org, project, env)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO computer_use_site_policies (id,organization_id,project_id,platform,account_id,allowed_protocols,allowed_hosts,allowed_page_kinds,allowed_platform_project_ids,version) VALUES (?,?,?,'ocean_engine','account_1',JSON_ARRAY('https'),JSON_ARRAY('example.test'),JSON_ARRAY('review'),JSON_ARRAY('platform_project_1'),1)`, policy, org, project)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	idSequence := 0
	run := validRun(now)
	run.ID = "cu_run_" + suffix
	run.OrganizationID = contract.OrganizationID(org)
	run.ProjectID = contract.ProjectID(project)
	run.Authority.OrganizationID = run.OrganizationID
	run.Authority.ProjectID = run.ProjectID
	run.EnvironmentID = env
	run.ProfileID = profile
	run.PolicyID = policy
	run.IdempotencyKey = "run_" + suffix
	repo := MySQLRepository{DB: db}
	service := Service{Repository: repo, Now: func() time.Time { return now }, NewID: func(prefix string) (string, error) {
		idSequence++
		return fmt.Sprintf("%s_%s_%d", prefix, suffix, idSequence), nil
	}}
	if _, _, err = service.CreateRun(ctx, CreateRunRequest{Run: run}); err != nil {
		t.Fatal(err)
	}
	lease := validLease(now)
	lease.ID = "cu_lease_" + suffix
	lease.RunID = run.ID
	lease.OrganizationID = run.OrganizationID
	lease.ProjectID = run.ProjectID
	lease.EnvironmentID = env
	lease.ProfileID = profile
	if _, err = repo.AcquireLease(ctx, lease); err != nil {
		t.Fatal(err)
	}
	lease, err = service.HeartbeatLease(ctx, run.OrganizationID, run.ProjectID, lease.ID, lease.Version, lease.FencingToken)
	if err != nil || lease.Version != 2 {
		t.Fatalf("heartbeat lease=%#v err=%v", lease, err)
	}
	stepID := "cu_step_" + suffix
	if _, err = db.ExecContext(ctx, `INSERT INTO computer_use_run_steps (id,organization_id,project_id,run_id,sequence_number,workflow_step_id,action,status,attempt,version) VALUES (?,?,?,?,1,'submit','submit_platform_configuration','pending',1,1)`, stepID, org, project, run.ID); err != nil {
		t.Fatal(err)
	}
	run, err = service.TransitionRun(ctx, run.OrganizationID, run.ProjectID, run.ID, 1, RunEnvironmentCheck, "")
	if err != nil {
		t.Fatal(err)
	}
	run, err = service.TransitionRun(ctx, run.OrganizationID, run.ProjectID, run.ID, 2, RunPreparing, "")
	if err != nil {
		t.Fatal(err)
	}
	run, err = service.TransitionRun(ctx, run.OrganizationID, run.ProjectID, run.ID, 3, RunAwaitingConfirmation, BlockFinalConfirmationRequired)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := service.IssueFinalConfirmation(ctx, run.OrganizationID, run.ProjectID, run.ID, run.Version, run.Authority.ApprovalActionHash, "operator")
	if err != nil {
		t.Fatal(err)
	}
	request := AuthorizeActionRequest{OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, RunID: run.ID, StepID: stepID, ConfirmationID: issued.Confirmation.ID, Token: issued.Token, LeaseID: lease.ID, FencingToken: lease.FencingToken, IdempotencyKey: "attempt_" + suffix}
	if _, err = service.AuthorizeAction(ctx, request); err != nil {
		t.Fatal(err)
	}
	request.IdempotencyKey = "retry_" + suffix
	if _, err = service.AuthorizeAction(ctx, request); err != ErrConfirmationInvalid {
		t.Fatalf("second confirmation consume err=%v", err)
	}
	var attempts int
	var consumed sql.NullTime
	if err = db.QueryRowContext(ctx, `SELECT COUNT(*),MAX(consumed_at) FROM computer_use_controlled_action_attempts a JOIN computer_use_final_confirmations c ON c.id=a.confirmation_id WHERE a.organization_id=? AND a.run_id=?`, org, run.ID).Scan(&attempts, &consumed); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 || !consumed.Valid {
		t.Fatalf("attempts=%d consumed=%v", attempts, consumed.Valid)
	}
	lease, err = service.ReleaseLease(ctx, run.OrganizationID, run.ProjectID, lease.ID, lease.Version, lease.FencingToken)
	if err != nil || lease.ReleasedAt == nil {
		t.Fatalf("release lease=%#v err=%v", lease, err)
	}

	takeoverRun := validRun(now)
	takeoverRun.ID = "cu_takeover_run_" + suffix
	takeoverRun.OrganizationID = contract.OrganizationID(org)
	takeoverRun.ProjectID = contract.ProjectID(project)
	takeoverRun.Authority.OrganizationID = takeoverRun.OrganizationID
	takeoverRun.Authority.ProjectID = takeoverRun.ProjectID
	takeoverRun.EnvironmentID = env
	takeoverRun.ProfileID = profile
	takeoverRun.PolicyID = policy
	takeoverRun.IdempotencyKey = "takeover_run_" + suffix
	if _, _, err = service.CreateRun(ctx, CreateRunRequest{Run: takeoverRun}); err != nil {
		t.Fatal(err)
	}
	takeoverRun, err = service.TransitionRun(ctx, takeoverRun.OrganizationID, takeoverRun.ProjectID, takeoverRun.ID, 1, RunEnvironmentCheck, "")
	if err != nil {
		t.Fatal(err)
	}
	takeoverRun, err = service.ControlRun(ctx, takeoverRun.OrganizationID, takeoverRun.ProjectID, takeoverRun.ID, takeoverRun.Version, ControlTakeover)
	if err != nil {
		t.Fatal(err)
	}
	acquired, err := service.AcquireRunLease(ctx, takeoverRun.OrganizationID, takeoverRun.ProjectID, takeoverRun.ID, takeoverRun.Version, "operator")
	if err != nil {
		t.Fatal(err)
	}
	recorded, err := service.RecordTakeoverEvidence(ctx, RecordTakeoverEvidenceRequest{OrganizationID: takeoverRun.OrganizationID, ProjectID: takeoverRun.ProjectID, RunID: takeoverRun.ID, ExpectedVersion: acquired.Run.Version, LeaseID: acquired.Lease.ID, FencingToken: acquired.Lease.FencingToken, StepID: "cu_takeover_step_" + suffix, Sequence: 1, Action: TakeoverVerifyNoWrite, Status: StepSucceeded, PageKind: "review", PlatformProjectID: "platform_project_1", BeforePageFacts: map[string]string{"account_balance": "0.00"}, AfterPageFacts: map[string]string{"write_detected": "false"}, FieldReadback: map[string]string{"daily_budget": "300"}, PageReference: "https://example.test/review?account=secret", SelectorVersion: "live/v1", ActionVersion: "takeover/v1", Actor: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	if recorded.Run.Version != acquired.Run.Version+1 || recorded.Evidence.BeforePageFacts["account_balance"] != redactedValue || recorded.Evidence.PageReference != "https://example.test/review" {
		t.Fatalf("recorded=%#v", recorded)
	}
	issued, err = service.IssueFinalConfirmation(ctx, recorded.Run.OrganizationID, recorded.Run.ProjectID, recorded.Run.ID, recorded.Run.Version, recorded.Run.Authority.ApprovalActionHash, "operator")
	if err != nil {
		t.Fatal(err)
	}
	authorized, err := service.AuthorizeTakeoverAction(ctx, AuthorizeTakeoverActionRequest{OrganizationID: recorded.Run.OrganizationID, ProjectID: recorded.Run.ProjectID, RunID: recorded.Run.ID, ExpectedVersion: recorded.Run.Version, StepID: "cu_takeover_submit_" + suffix, Sequence: 2, ConfirmationID: issued.Confirmation.ID, Token: issued.Token, LeaseID: acquired.Lease.ID, FencingToken: acquired.Lease.FencingToken, IdempotencyKey: "takeover_attempt_" + suffix, PageKind: "review", PlatformProjectID: "platform_project_1", BeforePageFacts: map[string]string{"submit_enabled": "true"}, FieldReadback: map[string]string{"daily_budget": "300"}, DiffKeys: []string{}, PageReference: "https://example.test/review", SelectorVersion: "live/v1", ActionVersion: "takeover-submit/v1", Actor: "operator"})
	if err != nil || authorized.Run.State != RunSubmitting {
		t.Fatalf("authorized=%#v err=%v", authorized, err)
	}
	unknown, err := service.RecordTakeoverOutcome(ctx, RecordTakeoverOutcomeRequest{OrganizationID: recorded.Run.OrganizationID, ProjectID: recorded.Run.ProjectID, RunID: recorded.Run.ID, AttemptID: authorized.Attempt.ID, ExpectedVersion: authorized.Run.Version, LeaseID: acquired.Lease.ID, FencingToken: acquired.Lease.FencingToken, StepID: "cu_takeover_unknown_" + suffix, Sequence: 3, Outcome: TakeoverResultUnknown, PageKind: "review", PlatformProjectID: "platform_project_1", PageReference: "https://example.test/review", SelectorVersion: "live/v1", ActionVersion: "takeover-result/v1", Actor: "operator"})
	if err != nil || unknown.Run.State != RunResultUnknown || unknown.Run.BlockingReason != BlockResultReconciliation {
		t.Fatalf("unknown=%#v err=%v", unknown, err)
	}
	var takeoverEvidence, takeoverEvents, takeoverSteps int
	if err = db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM computer_use_evidence WHERE organization_id=? AND run_id=?),(SELECT COUNT(*) FROM computer_use_events WHERE organization_id=? AND run_id=?),(SELECT COUNT(*) FROM computer_use_run_steps WHERE organization_id=? AND run_id=?)`, org, takeoverRun.ID, org, takeoverRun.ID, org, takeoverRun.ID).Scan(&takeoverEvidence, &takeoverEvents, &takeoverSteps); err != nil {
		t.Fatal(err)
	}
	if takeoverEvidence != 3 || takeoverEvents != 6 || takeoverSteps != 3 {
		t.Fatalf("evidence=%d events=%d steps=%d", takeoverEvidence, takeoverEvents, takeoverSteps)
	}
}
