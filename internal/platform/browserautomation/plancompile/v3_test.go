package plancompile

import (
	"context"
	"encoding/json"
	"slices"
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
	account := delivery.StableReference{ID: "1855554434276391", State: delivery.ReferenceResolved}
	configuration := &delivery.PlatformConfiguration{
		CanonicalHash: configurationHash,
		Payload: delivery.PlatformConfigurationPayload{OceanEngine: &delivery.OceanEngineConfiguration{Project: &delivery.OceanEngineProjectDraft{
			AccountReference: account,
			Carrier:          "owned_landing_page", OptimizationTargetReference: &optimization, DeepOptimizationMode: "conversion_roi",
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

func TestV3CompilerRunsOnePromotionCreateFromBoundConfiguration(t *testing.T) {
	now := time.Date(2026, 8, 25, 6, 0, 0, 0, time.UTC)
	configuration, intent := executableConfigurationFixture(now)
	planHash := strings.Repeat("a", 64)
	compiler := V3Compiler{Source: v3SourceStub{version: delivery.DeliveryPlanVersion{CanonicalHash: planHash, DeliveryIntent: &intent, PlatformConfiguration: &configuration}}, Now: func() time.Time { return now }}
	run := browserautomation.BrowserRpaRun{OrganizationID: "org_1", ProjectID: "project_1", AccountID: "1855554434276391", Authority: browserautomation.AuthorityBinding{Action: "create_promotions_in_existing_project", PlanID: "plan_1", PlanVersion: 1, PlanCanonicalHash: planHash, ConfigurationCanonicalHash: configuration.CanonicalHash, ParentPlatformProjectID: "7677595885572784182", BudgetLimitMinor: 30000}}
	policy := browserautomation.SitePolicy{AllowedProtocols: []string{"https"}, AllowedHosts: []string{"ad.oceanengine.com"}, AllowedPageKinds: []string{"promotion_create"}, AllowedPlatformProjects: []string{"7677595885572784182"}}
	raw, err := compiler.CompilePrepareV3(context.Background(), run, policy)
	if err != nil {
		t.Fatal(err)
	}
	var plan map[string]any
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	if plan["plan_kind"] != "promotion_create" || plan["parent_project_reference"] != "7677595885572784182" || plan["status"] != "ready" {
		t.Fatalf("plan = %#v", plan)
	}
	attempt := browserautomation.ControlledActionAttempt{ID: "attempt_1", Status: browserautomation.ControlledActionAuthorized, CreatedAt: now}
	submit, err := compiler.CompileSubmitV3(context.Background(), run, attempt, policy, "token")
	if err != nil {
		t.Fatal(err)
	}
	var submitted map[string]any
	if err := json.Unmarshal(submit, &submitted); err != nil {
		t.Fatal(err)
	}
	authority := submitted["execution_authority"].(map[string]any)
	if authority["schedule_date"] != configuration.Payload.OceanEngine.Project.Schedule.StartAt.Format(time.DateOnly) {
		t.Fatalf("schedule authority = %#v", authority)
	}
}

func TestV3CompilerRejectsCompoundFormsBeforeBrowserWrite(t *testing.T) {
	now := time.Date(2026, 8, 25, 6, 0, 0, 0, time.UTC)
	configuration, intent := executableConfigurationFixture(now)
	planHash := strings.Repeat("a", 64)
	compiler := V3Compiler{Source: v3SourceStub{version: delivery.DeliveryPlanVersion{CanonicalHash: planHash, DeliveryIntent: &intent, PlatformConfiguration: &configuration}}, Now: func() time.Time { return now }}
	run := browserautomation.BrowserRpaRun{OrganizationID: "org_1", ProjectID: "project_1", AccountID: "1855554434276391", Authority: browserautomation.AuthorityBinding{Action: "create_project_and_promotions", PlanID: "plan_1", PlanVersion: 1, PlanCanonicalHash: planHash, ConfigurationCanonicalHash: configuration.CanonicalHash}}
	_, err := compiler.CompilePrepareV3(context.Background(), run, browserautomation.SitePolicy{AllowedProtocols: []string{"https"}, AllowedHosts: []string{"ad.oceanengine.com"}, AllowedPageKinds: []string{"project_create"}})
	if err == nil || !strings.Contains(err.Error(), "staged controlled runs") {
		t.Fatalf("compound error = %v", err)
	}
}

func TestV3CompilerReportsUnavailableObjectsBeforeProjectCreate(t *testing.T) {
	now := time.Date(2026, 8, 25, 6, 0, 0, 0, time.UTC)
	configuration, intent := executableConfigurationFixture(now)
	product := configuration.Payload.OceanEngine.Project.MarketingProductReference
	product.Namespace = "cookies"
	product.ID = "product_internal_1"
	product.AuditAttributes = nil
	intent.Payload.ProductReferences[0] = *product
	planHash := strings.Repeat("a", 64)
	compiler := V3Compiler{Source: v3SourceStub{version: delivery.DeliveryPlanVersion{CanonicalHash: planHash, DeliveryIntent: &intent, PlatformConfiguration: &configuration}}, Now: func() time.Time { return now }}
	run := browserautomation.BrowserRpaRun{OrganizationID: "org_1", ProjectID: "project_1", AccountID: "1855554434276391", Authority: browserautomation.AuthorityBinding{Action: "create_project_and_promotions", PlanID: "plan_1", PlanVersion: 1, PlanCanonicalHash: planHash, ConfigurationCanonicalHash: configuration.CanonicalHash}}
	policy := browserautomation.SitePolicy{AllowedProtocols: []string{"https"}, AllowedHosts: []string{"ad.oceanengine.com"}, AllowedPageKinds: []string{"project_create"}}

	raw, err := compiler.CompilePrepareV3(context.Background(), run, policy)
	if err != nil {
		t.Fatal(err)
	}
	var plan v3Plan
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Status != "blocked" || !slices.Contains(plan.BlockedReasons, unavailablePlatformObjectsReason) {
		t.Fatalf("blocked plan = %#v", plan)
	}
	missing := slices.IndexFunc(plan.ObjectAvailability, func(value V3ObjectAvailability) bool {
		return value.FieldKey == "project.marketing_product_reference" && !value.Available
	})
	if missing < 0 || plan.ObjectAvailability[missing].InternalObjectID != "product_internal_1" {
		t.Fatalf("object availability = %#v", plan.ObjectAvailability)
	}
	attempt := browserautomation.ControlledActionAttempt{ID: "attempt_1", Status: browserautomation.ControlledActionAuthorized, CreatedAt: now}
	if _, err := compiler.CompileSubmitV3(context.Background(), run, attempt, policy, "token"); err == nil || !strings.Contains(err.Error(), unavailablePlatformObjectsReason) {
		t.Fatalf("submit error = %v", err)
	}
}
