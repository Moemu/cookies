package delivery

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/shikanon/cookies/internal/platform/computeruse"
	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestMySQLComputerUseRunResolvesAndBindsDeliveryAuthority(t *testing.T) {
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
	org := contract.OrganizationID("cu_authority_org_" + suffix)
	project := contract.ProjectID("cu_authority_project_" + suffix)
	if _, err := db.ExecContext(ctx, `INSERT INTO organizations (id,name,status) VALUES (?,?,'active')`, org, "computer-use authority integration"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO projects (id,organization_id,name,status) VALUES (?,?,?,'draft')`, project, org, "computer-use authority integration"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, statement := range []string{`DELETE FROM computer_use_events WHERE organization_id=?`, `UPDATE computer_use_runs SET lease_id=NULL WHERE organization_id=?`, `DELETE FROM computer_use_session_leases WHERE organization_id=?`, `DELETE FROM computer_use_runs WHERE organization_id=?`, `DELETE FROM computer_use_site_policies WHERE organization_id=?`, `DELETE FROM computer_use_browser_profiles WHERE organization_id=?`, `DELETE FROM computer_use_environments WHERE organization_id=?`, `DELETE FROM delivery_controlled_executions WHERE organization_id=?`, `DELETE FROM delivery_remote_write_approvals WHERE organization_id=?`, `DELETE FROM delivery_controlled_change_sets WHERE organization_id=?`, `DELETE FROM projects WHERE organization_id=?`, `DELETE FROM organizations WHERE id=?`} {
			_, _ = db.ExecContext(context.Background(), statement, org)
		}
	})

	now := time.Now().UTC().Truncate(time.Microsecond)
	deliveryRepo := MySQLRepository{DB: db}
	binding := validControlledBinding()
	change := ControlledChangeSet{SchemaVersion: ControlledChangeSetSchemaV1, ID: "change_" + suffix, OrganizationID: org, ProjectID: project, Binding: binding, Action: ControlledActionCreateProjectAndPromotions, BudgetLimitMinor: 30000, Currency: "CNY", Status: ControlledChangeSetReady, Version: 1, CreatedBy: "operator", CreatedAt: now, UpdatedAt: now}
	change.CanonicalHash, _ = change.ComputeCanonicalHash()
	change, _, err = deliveryRepo.CreateControlledChangeSet(ctx, change)
	if err != nil {
		t.Fatal(err)
	}
	approval := RemoteWriteApproval{SchemaVersion: RemoteWriteApprovalSchemaV1, ID: "approval_" + suffix, OrganizationID: org, ProjectID: project, ControlledChangeSetID: change.ID, ControlledChangeSetHash: change.CanonicalHash, Binding: binding, Action: change.Action, Scope: "controlled_remote_write", BudgetLimitMinor: change.BudgetLimitMinor, Currency: change.Currency, ApprovedBy: "approver", ApprovedAt: now, ExpiresAt: now.Add(RemoteWriteApprovalTTL)}
	approval.ActionHash, _ = approval.ComputeActionHash()
	change, approval, err = deliveryRepo.ApproveControlledChangeSet(ctx, change, approval)
	if err != nil {
		t.Fatal(err)
	}
	execution := ControlledExecution{ID: "execution_" + suffix, OrganizationID: org, ProjectID: project, ControlledChangeSetID: change.ID, RemoteWriteApprovalID: approval.ID, Status: "pending", Version: 1, CreatedBy: "operator", CreatedAt: now, UpdatedAt: now}
	execution, err = deliveryRepo.CreateControlledExecution(ctx, execution)
	if err != nil {
		t.Fatal(err)
	}
	loadedChange, err := deliveryRepo.GetControlledChangeSet(ctx, org, project, change.ID)
	if err != nil || loadedChange.SchemaVersion != ControlledChangeSetSchemaV1 {
		t.Fatalf("loaded change schema=%q err=%v", loadedChange.SchemaVersion, err)
	}
	loadedApproval, err := deliveryRepo.GetRemoteWriteApproval(ctx, org, project, change.ID)
	if err != nil || loadedApproval.SchemaVersion != RemoteWriteApprovalSchemaV1 {
		t.Fatalf("loaded approval schema=%q err=%v", loadedApproval.SchemaVersion, err)
	}

	computerUseRepo := computeruse.MySQLRepository{DB: db}
	computerUseService := computeruse.Service{Repository: computerUseRepo, AuthorityProvider: ComputerUseAuthorityProvider{Repository: deliveryRepo}, Now: func() time.Time { return now }, NewID: func(prefix string) (string, error) { return prefix + "_" + suffix, nil }}
	environment, err := computerUseService.RegisterEnvironment(ctx, org, project, computeruse.ExecutionEnvironment{ID: "env_" + suffix, Platform: computeruse.PlatformOceanEngine, AccountID: binding.AccountReferenceID, Mode: "local_visible", BrowserVersion: "integration", Region: "local", Healthy: true})
	if err != nil {
		t.Fatal(err)
	}
	profile, err := computerUseService.RegisterBrowserProfile(ctx, org, project, computeruse.BrowserProfile{ID: "profile_" + suffix, EnvironmentID: environment.ID, Platform: computeruse.PlatformOceanEngine, AccountID: binding.AccountReferenceID, State: "ready"})
	if err != nil {
		t.Fatal(err)
	}
	policy, err := computerUseService.RegisterSitePolicy(ctx, org, project, computeruse.SitePolicy{ID: "policy_" + suffix, Platform: computeruse.PlatformOceanEngine, AccountID: binding.AccountReferenceID, AllowedProtocols: []string{"https"}, AllowedHosts: []string{"ad.oceanengine.com"}, AllowedPageKinds: []string{"project_create"}, AllowedPlatformProjects: []string{"new-project:" + binding.ObjectFingerprint}})
	if err != nil {
		t.Fatal(err)
	}
	request := computeruse.CreateBoundRunRequest{OrganizationID: org, ProjectID: project, Platform: computeruse.PlatformOceanEngine, AccountID: binding.AccountReferenceID, ExecutionID: execution.ID, EnvironmentID: environment.ID, ProfileID: profile.ID, PolicyID: policy.ID, IdempotencyKey: "run-key-" + suffix, CreatedBy: "operator"}
	run, replayed, err := computerUseService.CreateBoundRun(ctx, request)
	if err != nil || replayed {
		t.Fatalf("create run=%+v replayed=%t err=%v", run, replayed, err)
	}
	linked, err := deliveryRepo.GetControlledExecution(ctx, org, project, execution.ID)
	if err != nil || linked.ComputerUseRunID != run.ID || linked.Status != "running" {
		t.Fatalf("linked=%+v err=%v", linked, err)
	}
	replayedRun, replayed, err := computerUseService.CreateBoundRun(ctx, request)
	if err != nil || !replayed || replayedRun.ID != run.ID {
		t.Fatalf("replay run=%+v replayed=%t err=%v", replayedRun, replayed, err)
	}
}
