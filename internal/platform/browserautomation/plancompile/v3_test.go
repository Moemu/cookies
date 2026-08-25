package plancompile

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/browserautomation"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/delivery"
)

type v3SourceStub struct {
	version delivery.DeliveryPlanVersion
}

func (s v3SourceStub) GetPlanVersion(context.Context, contract.OrganizationID, contract.ProjectID, string, int) (delivery.DeliveryPlanVersion, error) {
	return s.version, nil
}

func TestV3CompilerConvertsBoundBudgetRunAndIssuesOneTimeAuthority(t *testing.T) {
	now := time.Date(2026, 8, 25, 6, 0, 0, 0, time.UTC)
	configurationHash := strings.Repeat("b", 64)
	planHash := strings.Repeat("a", 64)
	optimization := delivery.StableReference{ID: "in_app_order"}
	configuration := &delivery.PlatformConfiguration{
		CanonicalHash: configurationHash,
		Payload: delivery.PlatformConfigurationPayload{OceanEngine: &delivery.OceanEngineConfiguration{Project: &delivery.OceanEngineProjectDraft{
			Carrier: "owned_landing_page", OptimizationTargetReference: &optimization, DeepOptimizationMode: "conversion_roi",
			DeliveryMode: "manual", PlacementStrategy: "automatic",
		}}},
	}
	compiler := V3Compiler{Source: v3SourceStub{version: delivery.DeliveryPlanVersion{CanonicalHash: planHash, PlatformConfiguration: configuration}}, Now: func() time.Time { return now }}
	run := browserautomation.BrowserRpaRun{
		OrganizationID: "org_1", ProjectID: "project_1", AccountID: "1855554434276391",
		Authority: browserautomation.AuthorityBinding{
			Action: "update_promotion_budget", PlanID: "plan_1", PlanVersion: 2, PlanCanonicalHash: planHash, ConfigurationCanonicalHash: configurationHash,
			ParentPlatformProjectID: "7677595885572784182", TargetPlatformObjectID: "7683558668450021382", BudgetLimitMinor: 30000,
			PromotionMutation: &browserautomation.PromotionMutationBinding{CurrentDailyBudgetMinor: 40000, TargetDailyBudgetMinor: 30000},
		},
	}
	policy := browserautomation.SitePolicy{
		AllowedProtocols: []string{"https"}, AllowedHosts: []string{"ad.oceanengine.com"},
		AllowedPageKinds: []string{"promotion_edit"}, AllowedPlatformProjects: []string{"7677595885572784182"},
	}
	prepare, err := compiler.CompilePrepareV3(context.Background(), run, policy)
	if err != nil {
		t.Fatalf("compile prepare: %v", err)
	}
	var preparePlan map[string]any
	if err := json.Unmarshal(prepare, &preparePlan); err != nil {
		t.Fatal(err)
	}
	if preparePlan["schema_version"] != "oceanengine-playwright-rpa-plan/v3" || preparePlan["plan_kind"] != "promotion_edit" || preparePlan["allow_remote_write"] != false {
		t.Fatalf("prepare plan = %#v", preparePlan)
	}

	attempt := browserautomation.ControlledActionAttempt{ID: "attempt_1", Status: browserautomation.ControlledActionAuthorized, CreatedAt: now}
	submit, err := compiler.CompileSubmitV3(context.Background(), run, attempt, policy, "single-use-token")
	if err != nil {
		t.Fatalf("compile submit: %v", err)
	}
	var submitPlan map[string]any
	if err := json.Unmarshal(submit, &submitPlan); err != nil {
		t.Fatal(err)
	}
	authority, ok := submitPlan["execution_authority"].(map[string]any)
	if !ok || authority["authority_id"] != "attempt_1" || authority["confirm_token_sha256"] == "single-use-token" {
		t.Fatalf("execution authority = %#v", authority)
	}
	delete(submitPlan, "execution_authority")
	expectedHash, err := contract.CanonicalJSONHash(submitPlan)
	if err != nil {
		t.Fatal(err)
	}
	if authority["plan_sha256"] != expectedHash || submitPlan["allow_remote_write"] != true || submitPlan["maximum_final_clicks"] != float64(1) {
		t.Fatalf("submit authority/plan mismatch: authority=%#v plan=%#v", authority, submitPlan)
	}
}

func TestV3CompilerFailsClosedForUnsupportedControlActions(t *testing.T) {
	compiler := V3Compiler{Source: v3SourceStub{}}
	run := browserautomation.BrowserRpaRun{Authority: browserautomation.AuthorityBinding{Action: "pause_promotion", PlanID: "plan_1", PlanVersion: 1}}
	_, err := compiler.CompilePrepareV3(context.Background(), run, browserautomation.SitePolicy{})
	if err == nil || !strings.Contains(err.Error(), "no Runner v3 one-form path") {
		t.Fatalf("unsupported action error = %v", err)
	}
}
