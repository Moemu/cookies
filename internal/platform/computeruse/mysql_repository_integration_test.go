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
		for _, statement := range []string{`DELETE FROM computer_use_controlled_action_attempts WHERE organization_id=?`, `DELETE FROM computer_use_final_confirmations WHERE organization_id=?`, `DELETE FROM computer_use_run_steps WHERE organization_id=?`, `UPDATE computer_use_runs SET lease_id=NULL WHERE organization_id=?`, `DELETE FROM computer_use_session_leases WHERE organization_id=?`, `DELETE FROM computer_use_runs WHERE organization_id=?`, `DELETE FROM computer_use_site_policies WHERE organization_id=?`, `DELETE FROM computer_use_browser_profiles WHERE organization_id=?`, `DELETE FROM computer_use_environments WHERE organization_id=?`, `DELETE FROM projects WHERE organization_id=?`, `DELETE FROM organizations WHERE id=?`} {
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
	service := Service{Repository: repo, Now: func() time.Time { return now }, NewID: func(prefix string) (string, error) { return prefix + "_" + suffix, nil }}
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
	issued, err := service.IssueFinalConfirmation(ctx, run.OrganizationID, run.ProjectID, run.ID, run.Authority.ApprovalActionHash, "operator")
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
}
