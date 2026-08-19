package plancompile

import (
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/browserautomation"
	"github.com/shikanon/cookies/internal/platform/browserautomation/rparunner"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/delivery/calibrationmanifest"
)

func loadManifest(t *testing.T) calibrationmanifest.Manifest {
	t.Helper()
	manifest, err := calibrationmanifest.Current()
	if err != nil {
		t.Fatalf("load frozen calibration manifest: %v", err)
	}
	return manifest
}

func budgetRun() browserautomation.BrowserRpaRun {
	return browserautomation.BrowserRpaRun{
		ID:             "run_test",
		OrganizationID: contract.OrganizationID("org_test"),
		ProjectID:      contract.ProjectID("project_test"),
		AccountID:      "account_test",
		EnvironmentID:  "env_test",
		PolicyID:       "policy_test",
		Authority: browserautomation.AuthorityBinding{
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
}

func budgetPolicy() browserautomation.SitePolicy {
	return browserautomation.SitePolicy{
		AllowedProtocols:        []string{"https"},
		AllowedHosts:            []string{"ad.oceanengine.com"},
		AllowedPageKinds:        []string{"promotion_list"},
		AllowedPlatformProjects: []string{"project_platform_test"},
	}
}

func TestCompilePrepareEmitsFrozenCalibrationLocatorsWithoutWriteAuthority(t *testing.T) {
	compiler := Compiler{Manifest: loadManifest(t)}
	plan, err := compiler.CompilePrepare(budgetRun(), budgetPolicy())
	if err != nil {
		t.Fatalf("compile prepare: %v", err)
	}
	if plan.SchemaVersion != rparunner.PlanSchemaV2 || plan.AllowRemoteWrite || plan.Mode != "prepare" {
		t.Fatalf("prepare plan must be read-only v2: %+v", plan)
	}
	if plan.AccountID != "account_test" || plan.ExpectedObjectID != "promotion_test" {
		t.Fatalf("plan identity not bound to the run authority: %+v", plan)
	}
	if len(plan.Steps) != 2 || plan.Steps[0].Kind != "identify_page" || plan.Steps[1].Kind != "readback" {
		t.Fatalf("unexpected plan steps: %+v", plan.Steps)
	}
	readback := plan.Steps[1]
	if len(readback.ScopeChecks) == 0 || readback.ScopeChecks[0].Value != "单元预算" {
		t.Fatalf("scope check must come from the frozen manifest: %+v", readback.ScopeChecks)
	}
	if len(readback.Fields) != 1 || readback.Fields[0].Key != "daily_budget_yuan" || readback.Fields[0].Locator.Value != "spinbutton:单元日预算" {
		t.Fatalf("readback target must come from the frozen manifest: %+v", readback.Fields)
	}
}

func TestCompilePrepareRejectsUncalibratedActionsAndPolicies(t *testing.T) {
	compiler := Compiler{Manifest: loadManifest(t)}
	run := budgetRun()
	run.Authority.Action = "create_project_and_promotions"
	if _, err := compiler.CompilePrepare(run, budgetPolicy()); err == nil {
		t.Fatal("uncalibrated action compiled a prepare plan")
	}

	policy := budgetPolicy()
	policy.AllowedPageKinds = []string{"project_create"}
	if _, err := compiler.CompilePrepare(budgetRun(), policy); err == nil {
		t.Fatal("policy without the promotion_list page kind compiled a prepare plan")
	}
}

func TestCompileSubmitConvertsBudgetUnitsAndBindsTheWriteBoundary(t *testing.T) {
	compiler := Compiler{Manifest: loadManifest(t)}
	plan, err := compiler.CompileSubmit(budgetRun(), browserautomation.ControlledActionAttempt{}, budgetPolicy())
	if err != nil {
		t.Fatalf("compile submit: %v", err)
	}
	if !plan.AllowRemoteWrite || plan.Mode != "submit" {
		t.Fatalf("submit plan must carry write authority: %+v", plan)
	}
	var fill *rparunner.RpaStep
	var click *rparunner.RpaStep
	for i := range plan.Steps {
		switch plan.Steps[i].Kind {
		case "fill_money":
			fill = &plan.Steps[i]
		case "final_click":
			click = &plan.Steps[i]
		}
	}
	if fill == nil || click == nil {
		t.Fatalf("submit plan is missing fill or final click steps: %+v", plan.Steps)
	}
	if value, ok := fill.Fields[0].Value.(int64); !ok || value != 500 {
		t.Fatalf("target budget must convert 50000 fen to 500 yuan: %+v", fill.Fields[0].Value)
	}
	if click.Locator == nil || click.Locator.Value != "button:确定修改" || !click.RemoteWrite {
		t.Fatalf("final click must target the frozen write boundary exactly once: %+v", click)
	}
}

func TestCompileSubmitRejectsBudgetsNotRepresentableInInputUnits(t *testing.T) {
	compiler := Compiler{Manifest: loadManifest(t)}
	run := budgetRun()
	run.Authority.PromotionMutation.TargetDailyBudgetMinor = 30050
	if _, err := compiler.CompileSubmit(run, browserautomation.ControlledActionAttempt{}, budgetPolicy()); err == nil {
		t.Fatal("fractional-yuan budget compiled a submit plan")
	}
}
