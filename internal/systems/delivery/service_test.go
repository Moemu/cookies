package delivery

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestPlanLifecyclePersistsImmutableVersionsAndRejectsStaleUpdates(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	service := Service{
		Store: NewMemoryStore(),
		NewID: func(string) (string, error) { return "deliveryplan_1", nil },
		Now:   func() time.Time { return now },
	}
	actor := deliveryActor("org_1")
	created, err := service.CreatePlan(context.Background(), actor, CreatePlanRequest{
		ProjectID: "project_1",
		PlanDraft: goldenDraft(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Source != SourceMock || created.Scenario != ScenarioGoldenPath || created.CurrentVersionNumber != 1 {
		t.Fatalf("created plan = %+v", created)
	}

	updatedDraft := goldenDraft()
	updatedDraft.Budget.TotalMinor = 350_000
	now = now.Add(time.Minute)
	updated, err := service.UpdatePlan(context.Background(), actor, created.ID, UpdatePlanRequest{
		ExpectedVersion: 1,
		PlanDraft:       updatedDraft,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.CurrentVersionNumber != 2 || len(updated.Versions) != 2 {
		t.Fatalf("updated versions = %d / %+v", updated.CurrentVersionNumber, updated.Versions)
	}
	if got := updated.Versions[0].Budget.TotalMinor; got != 300_000 {
		t.Fatalf("V1 budget = %d, want 300000", got)
	}
	if got := updated.Versions[1].Budget.TotalMinor; got != 350_000 {
		t.Fatalf("V2 budget = %d, want 350000", got)
	}

	_, err = service.UpdatePlan(context.Background(), actor, created.ID, UpdatePlanRequest{
		ExpectedVersion: 1,
		PlanDraft:       updatedDraft,
	})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale update error = %v, want ErrVersionConflict", err)
	}

	reloaded, err := service.GetPlan(context.Background(), actor, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.CurrentVersionNumber != 2 || len(reloaded.Versions) != 2 {
		t.Fatalf("reloaded plan = %+v", reloaded)
	}
}

func TestPlanStoreIsolatesOrganizationsAndProjects(t *testing.T) {
	t.Parallel()
	store := NewMemoryStore()
	service := Service{
		Store: store,
		NewID: func(string) (string, error) { return "deliveryplan_isolated", nil },
		Now:   func() time.Time { return time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC) },
	}
	created, err := service.CreatePlan(context.Background(), deliveryActor("org_1"), CreatePlanRequest{
		ProjectID: "project_1",
		PlanDraft: goldenDraft(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetPlan(context.Background(), deliveryActor("org_2"), created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-organization read error = %v, want ErrNotFound", err)
	}
	otherProject, err := service.ListPlans(context.Background(), deliveryActor("org_1"), "project_2")
	if err != nil {
		t.Fatal(err)
	}
	if len(otherProject) != 0 {
		t.Fatalf("cross-project list returned %+v", otherProject)
	}
}

func TestServerPreflightIsAuthoritativeAndSeverityGraded(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		mutate       func(*PlanDraft)
		wantScenario Scenario
		wantBlocked  bool
		wantCode     string
		wantSeverity CheckSeverity
	}{
		{name: "golden", mutate: func(*PlanDraft) {}, wantScenario: ScenarioGoldenPath},
		{name: "budget zero", mutate: func(draft *PlanDraft) { draft.Budget.TotalMinor = 0 }, wantScenario: ScenarioBudgetZero, wantBlocked: true, wantCode: "budget_positive", wantSeverity: CheckSeverityError},
		{name: "creative unconfirmed", mutate: func(draft *PlanDraft) { draft.CreativeReferences[0].Confirmed = false }, wantScenario: ScenarioCreativeUnconfirmed, wantCode: "creative_confirmed", wantSeverity: CheckSeverityWarning},
		{name: "tracking missing", mutate: func(draft *PlanDraft) { draft.Tracking.PixelID = "" }, wantScenario: ScenarioTrackingMissing, wantBlocked: true, wantCode: "tracking_complete", wantSeverity: CheckSeverityError},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			draft := goldenDraft()
			test.mutate(&draft)
			service := Service{
				Store: NewMemoryStore(),
				NewID: func(string) (string, error) { return "deliveryplan_" + stringsForID(test.name), nil },
				Now:   func() time.Time { return time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC) },
			}
			actor := deliveryActor("org_1")
			plan, err := service.CreatePlan(context.Background(), actor, CreatePlanRequest{ProjectID: "project_1", PlanDraft: draft})
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.PreflightPlan(context.Background(), actor, plan.ID)
			if err != nil {
				t.Fatal(err)
			}
			if result.Source != SourceMock || result.Scenario != test.wantScenario || result.Blocked != test.wantBlocked || result.Passed == test.wantBlocked {
				t.Fatalf("preflight = %+v", result)
			}
			if test.wantCode == "" {
				for _, item := range result.Checks {
					if !item.Passed {
						t.Fatalf("golden check failed: %+v", item)
					}
				}
				return
			}
			var found *PreflightCheck
			for index := range result.Checks {
				if result.Checks[index].Code == test.wantCode {
					found = &result.Checks[index]
					break
				}
			}
			if found == nil || found.Passed || found.Severity != test.wantSeverity || found.Repair == nil || found.Repair.Field == "" {
				t.Fatalf("target check = %+v", found)
			}
		})
	}
}

func goldenDraft() PlanDraft {
	return PlanDraft{
		Name:      "销售线索增长计划",
		Objective: "获取高质量销售线索",
		Advertiser: AdvertiserInput{
			ID: "mock-advertiser-001", Name: "Cookies Mock 广告主", Platform: "ocean_engine",
		},
		Budget: Budget{TotalMinor: 300_000, Currency: "CNY"},
		Schedule: Schedule{
			StartAt:  time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			EndAt:    time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
			Timezone: "Asia/Shanghai",
		},
		Tracking: Tracking{
			LandingPage: "https://demo.cookies.local/lead", PixelID: "PX-MOCK-LEAD", ConversionEvent: "lead_submit",
		},
		CreativeReferences:    []CreativeReference{{AssetID: "asset_mock_1", Version: 1, Confirmed: true}},
		SourceStrategyVersion: "strategy-v1",
	}
}

func deliveryActor(organizationID contract.OrganizationID) contract.ActorContext {
	return contract.ActorContext{
		OrganizationID: organizationID,
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
		Scopes:         []contract.Scope{ScopePlanRead, ScopePlanWrite},
	}
}

func stringsForID(value string) string {
	result := ""
	for _, character := range value {
		if character >= 'a' && character <= 'z' {
			result += string(character)
		}
	}
	return result
}
