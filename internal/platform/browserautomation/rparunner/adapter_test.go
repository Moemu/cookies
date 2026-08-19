package rparunner

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/browserautomation"
	"github.com/shikanon/cookies/internal/platform/contract"
)

type stubCompiler struct {
	mode string
}

func (s stubCompiler) CompilePrepare(run browserautomation.BrowserRpaRun, _ browserautomation.SitePolicy) (RpaPlan, error) {
	s.mode = "prepare"
	return minimalPlan("prepare", run), nil
}

func (s stubCompiler) CompileSubmit(run browserautomation.BrowserRpaRun, _ browserautomation.ControlledActionAttempt, _ browserautomation.SitePolicy) (RpaPlan, error) {
	s.mode = "submit"
	return minimalPlan("submit", run), nil
}

func minimalPlan(mode string, run browserautomation.BrowserRpaRun) RpaPlan {
	return RpaPlan{
		SchemaVersion: PlanSchemaV2,
		Browser:       "msedge",
		Mode:          mode,
		AccountID:     run.AccountID,
		Steps:         []RpaStep{{ID: "identify_account_and_object", Kind: "identify_page", PageKind: "promotion_list"}},
	}
}

func testAdapter(t *testing.T, mode string, env browserautomation.ExecutionEnvironment, policy browserautomation.SitePolicy) PlaywrightRPAAdapter {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("testdata", "fake-runner.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	repo := browserautomation.NewMemoryRepository()
	if _, err := repo.CreateEnvironment(context.Background(), env); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateSitePolicy(context.Background(), policy); err != nil {
		t.Fatal(err)
	}
	return PlaywrightRPAAdapter{
		Runner: Runner{
			Command:        []string{"node", abs},
			ScriptPath:     mode,
			PrepareTimeout: 30 * time.Second,
			SubmitTimeout:  30 * time.Second,
		},
		Compiler: stubCompiler{},
		Store:    repo,
	}
}

func adapterFixture() (browserautomation.BrowserRpaRun, browserautomation.ExecutionEnvironment, browserautomation.SitePolicy) {
	run := browserautomation.BrowserRpaRun{
		ID:             "run_test",
		OrganizationID: contract.OrganizationID("org_test"),
		ProjectID:      contract.ProjectID("project_test"),
		AccountID:      "account_test",
		EnvironmentID:  "env_test",
		PolicyID:       "policy_test",
		Authority: browserautomation.AuthorityBinding{
			SchemaVersion:          browserautomation.AuthoritySchemaV1,
			Action:                 "update_promotion_budget",
			AccountReferenceID:     "account_test",
			TargetPlatformObjectID: "promotion_test",
			PromotionMutation: &browserautomation.PromotionMutationBinding{
				CurrentDailyBudgetMinor: 30000,
				TargetDailyBudgetMinor:  50000,
				CurrentStateHash:        strings.Repeat("a", 64),
				TargetStateHash:         strings.Repeat("b", 64),
			},
		},
	}
	env := browserautomation.ExecutionEnvironment{
		ID: "env_test", OrganizationID: run.OrganizationID, ProjectID: run.ProjectID,
		Platform: browserautomation.PlatformOceanEngine, AccountID: "account_test",
		Mode: "local_visible", BrowserVersion: "edge-test", Region: "local",
		Healthy: true, CDPEndpoint: "success", Version: 1,
	}
	policy := browserautomation.SitePolicy{
		ID: "policy_test", OrganizationID: run.OrganizationID, ProjectID: run.ProjectID,
		Platform: browserautomation.PlatformOceanEngine, AccountID: "account_test",
		AllowedProtocols: []string{"https"}, AllowedHosts: []string{"ad.oceanengine.com"},
		AllowedPageKinds: []string{"promotion_list"}, AllowedPlatformProjects: []string{"platform_project_test"},
		Version: 1,
	}
	return run, env, policy
}

func TestAdapterPrepareInjectsServerOwnedReadback(t *testing.T) {
	run, env, policy := adapterFixture()
	adapter := testAdapter(t, "success", env, policy)
	page, err := adapter.Prepare(context.Background(), run)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if page.Readback["platform_object_id"] != "promotion_test" {
		t.Fatalf("runner object identity was not promoted: %+v", page.Readback)
	}
	if page.Readback["current_state_hash"] != strings.Repeat("a", 64) || page.Readback["target_state_hash"] != strings.Repeat("b", 64) {
		t.Fatalf("server-owned state hashes were not injected: %+v", page.Readback)
	}
	if page.SelectorVersion != SelectorVersion || page.ActionVersion != ActionVersion {
		t.Fatalf("evidence provenance must identify the playwright adapter: %+v", page)
	}
}

func TestAdapterPrepareRejectsAccountMismatchingEnvironment(t *testing.T) {
	run, env, policy := adapterFixture()
	env.AccountID = "another_account"
	adapter := testAdapter(t, "success", env, policy)
	_, err := adapter.Prepare(context.Background(), run)
	if !errors.Is(err, browserautomation.ErrAccountMismatch) {
		t.Fatalf("expected account mismatch, got %v", err)
	}
}

func TestAdapterPrepareRequiresACDPEndpoint(t *testing.T) {
	run, env, policy := adapterFixture()
	env.CDPEndpoint = ""
	adapter := testAdapter(t, "success", env, policy)
	_, err := adapter.Prepare(context.Background(), run)
	if !errors.Is(err, browserautomation.ErrEnvironmentUnavailable) {
		t.Fatalf("expected environment unavailable, got %v", err)
	}
}

func TestAdapterPrepareClassifiesRunnerPageDrift(t *testing.T) {
	run, env, policy := adapterFixture()
	adapter := testAdapter(t, "page-drift", env, policy)
	_, err := adapter.Prepare(context.Background(), run)
	if !errors.Is(err, browserautomation.ErrPageDrift) {
		t.Fatalf("expected page drift, got %v", err)
	}
}

func TestAdapterSubmitInfrastructureFailureIsResultUnknown(t *testing.T) {
	run, env, policy := adapterFixture()
	env.CDPEndpoint = "garbage"
	adapter := testAdapter(t, "garbage", env, policy)
	outcome, _, err := adapter.Submit(context.Background(), run, browserautomation.ControlledActionAttempt{})
	if err != nil {
		t.Fatalf("submit must not surface infrastructure errors as authorization failures: %v", err)
	}
	if outcome != browserautomation.WorkerResultUnknown {
		t.Fatalf("infrastructure failure after authorization must be result_unknown, got %s", outcome)
	}
}

func TestAdapterSubmitRejectsSuccessWithoutClick(t *testing.T) {
	run, env, policy := adapterFixture()
	adapter := testAdapter(t, "success", env, policy)
	outcome, _, err := adapter.Submit(context.Background(), run, browserautomation.ControlledActionAttempt{})
	if err == nil || outcome != browserautomation.WorkerFailed {
		t.Fatalf("success without the final click must fail, got outcome=%s err=%v", outcome, err)
	}
}
