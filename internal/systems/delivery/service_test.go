package delivery

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestPlanLifecyclePersistsVersionsAndRejectsStaleUpdate(t *testing.T) {
	service, actor := newTestService()
	draft := goldenDraft()

	created, err := service.CreatePlan(context.Background(), actor, "project_a", CreatePlanRequest{PlanDraft: draft})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if created.Source != SourceMock || created.Scenario != ScenarioGoldenPath || created.Version != 1 {
		t.Fatalf("unexpected created plan: %#v", created)
	}

	draft.Budget.TotalMinor = 880_000
	updated, err := service.UpdatePlan(context.Background(), actor, "project_a", created.ID, UpdatePlanRequest{
		ExpectedVersion: 1, PlanDraft: draft,
	})
	if err != nil {
		t.Fatalf("update plan: %v", err)
	}
	if updated.Version != 2 || len(updated.Versions) != 2 || updated.Versions[0].Budget.TotalMinor == updated.Versions[1].Budget.TotalMinor {
		t.Fatalf("immutable version history was not retained: %#v", updated)
	}
	_, err = service.UpdatePlan(context.Background(), actor, "project_a", created.ID, UpdatePlanRequest{
		ExpectedVersion: 1, PlanDraft: draft,
	})
	if !errors.Is(err, ErrPlanVersionConflict) {
		t.Fatalf("expected plan version conflict, got %v", err)
	}
}

func TestPlanIsolationAndAuthoritativePreflightScenarios(t *testing.T) {
	service, actor := newTestService()
	cases := []struct {
		name           string
		mutate         func(*PlanDraft)
		scenario       Scenario
		blocked        bool
		failedCode     string
		failedSeverity CheckSeverity
	}{
		{name: "golden", mutate: func(*PlanDraft) {}, scenario: ScenarioGoldenPath},
		{name: "budget zero", mutate: func(value *PlanDraft) { value.Budget.TotalMinor = 0 }, scenario: ScenarioBudgetZero, blocked: true, failedCode: "budget_positive", failedSeverity: CheckSeverityError},
		{name: "creative warning", mutate: func(value *PlanDraft) { value.CreativeReferences[0].Confirmed = false }, scenario: ScenarioCreativeUnconfirmed, failedCode: "creative_confirmed", failedSeverity: CheckSeverityWarning},
		{name: "tracking missing", mutate: func(value *PlanDraft) { value.Tracking.PixelID = "" }, scenario: ScenarioTrackingMissing, blocked: true, failedCode: "tracking_complete", failedSeverity: CheckSeverityError},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			draft := goldenDraft()
			testCase.mutate(&draft)
			plan, err := service.CreatePlan(context.Background(), actor, "project_a", CreatePlanRequest{PlanDraft: draft})
			if err != nil {
				t.Fatalf("create plan: %v", err)
			}
			result, err := service.RunPlanPreflight(context.Background(), actor, "project_a", plan.ID)
			if err != nil {
				t.Fatalf("run preflight: %v", err)
			}
			if result.Source != SourceMock || result.Scenario != testCase.scenario || result.Blocked != testCase.blocked {
				t.Fatalf("unexpected result: %#v", result)
			}
			if testCase.failedCode != "" {
				found := false
				for _, check := range result.Checks {
					if check.Code == testCase.failedCode && !check.Passed && check.Severity == testCase.failedSeverity && check.Repair != nil {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing failed check %s: %#v", testCase.failedCode, result.Checks)
				}
			}
		})
	}

	plan, err := service.CreatePlan(context.Background(), actor, "project_a", CreatePlanRequest{PlanDraft: goldenDraft()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetPlan(context.Background(), actor, "project_b", plan.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-project read should be hidden, got %v", err)
	}
}

func TestChangeSetFreezesVersionAndRejectsStalePlan(t *testing.T) {
	service, actor := newTestService()
	plan, err := service.CreatePlan(context.Background(), actor, "project_a", CreatePlanRequest{PlanDraft: goldenDraft()})
	if err != nil {
		t.Fatal(err)
	}
	changeSet, err := service.CreateChangeSet(context.Background(), actor, "project_a", plan.ID, plan.Version)
	if err != nil {
		t.Fatal(err)
	}
	draft := goldenDraft()
	draft.Budget.TotalMinor++
	if _, err := service.UpdatePlan(context.Background(), actor, "project_a", plan.ID, UpdatePlanRequest{ExpectedVersion: 1, PlanDraft: draft}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Preflight(context.Background(), actor, "project_a", changeSet.ID, changeSet.Version); !errors.Is(err, ErrStalePlanVersion) {
		t.Fatalf("expected stale frozen version rejection, got %v", err)
	}
}

func TestChangeSetGoldenFlowAndMetricProvenance(t *testing.T) {
	service, actor := newTestService()
	plan, err := service.CreatePlan(context.Background(), actor, "project_a", CreatePlanRequest{PlanDraft: goldenDraft()})
	if err != nil {
		t.Fatal(err)
	}
	changeSet, err := service.CreateChangeSet(context.Background(), actor, "project_a", plan.ID, plan.Version)
	if err != nil {
		t.Fatal(err)
	}
	changeSet, err = service.Preflight(context.Background(), actor, "project_a", changeSet.ID, changeSet.Version)
	if err != nil || changeSet.Status != ChangeSetPreflightPassed {
		t.Fatalf("preflight: %#v %v", changeSet, err)
	}
	changeSet, err = service.Approve(context.Background(), actor, "project_a", changeSet.ID, changeSet.Version)
	if err != nil {
		t.Fatal(err)
	}
	executed, err := service.Execute(context.Background(), actor, "project_a", changeSet.ID, changeSet.Version)
	if err != nil {
		t.Fatal(err)
	}
	if executed.Execution.Mode != ExecutionModeLocalSimulation {
		t.Fatalf("unexpected mode %q", executed.Execution.Mode)
	}
	metric, err := service.CreateDemoMetricSnapshot(context.Background(), actor, "project_a", executed.Execution.ID, CreateMetricSnapshotRequest{DatasetVersion: DemoMetricDatasetVersion})
	if err != nil {
		t.Fatal(err)
	}
	if metric.Source != MetricSourceDemoFixture || !metric.IsSimulated {
		t.Fatalf("unexpected metric provenance: %#v", metric)
	}
}

func goldenDraft() PlanDraft {
	return PlanDraft{
		Name: "Mock 投放计划", Objective: "获取销售线索",
		Advertiser: AdvertiserInput{ID: "mock-advertiser-001", Name: "Cookies Mock 广告主", Platform: "ocean_engine"},
		Budget:     Budget{TotalMinor: 300_000, Currency: "CNY"},
		Schedule: Schedule{
			StartAt: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			EndAt:   time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC), Timezone: "Asia/Shanghai",
		},
		Tracking:              Tracking{LandingPage: "https://demo.cookies.local", PixelID: "PX-001", ConversionEvent: "lead_submit"},
		CreativeReferences:    []CreativeReference{{AssetID: "asset_mock_001", Version: 1, Confirmed: true}},
		SourceStrategyVersion: "strategy-v1",
	}
}

func newTestService() (Service, contract.ActorContext) {
	repository := newMemoryRepository()
	actor := contract.ActorContext{
		OrganizationID: "org_a",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "user_a"},
		Scopes: contract.ScopesFromStrings([]string{
			string(ScopeRead), string(ScopeWrite), string(ScopeApprove), string(ScopeExecute),
		}),
	}
	counter := 0
	return Service{
		Repository: repository,
		Projects:   testProjects{},
		Packages:   testPackages{},
		NewID: func(prefix string) (string, error) {
			counter++
			return fmt.Sprintf("%s_%d", prefix, counter), nil
		},
		Now: func() time.Time { return time.Date(2026, 7, 30, 10, 0, counter, 0, time.UTC) },
	}, actor
}

type testProjects struct{}

func (testProjects) RequireActiveContext(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID) (contract.ProjectContext, error) {
	if projectID != "project_a" && projectID != "project_b" {
		return contract.ProjectContext{}, ErrNotFound
	}
	return contract.ProjectContext{OrganizationID: actor.OrganizationID, ProjectID: projectID}, nil
}

type testPackages struct{}

func (testPackages) ReadCreativePackage(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, id string) (CreativePackageSnapshot, error) {
	return CreativePackageSnapshot{ID: id, CreativeVersionID: "creative_v1", ContentHash: "sha256:mock"}, nil
}

type memoryRepository struct {
	plans      map[string]DeliveryPlan
	changeSets map[string]ChangeSet
	executions []ExecutionResult
	metrics    []DeliveryMetricSnapshot
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{plans: map[string]DeliveryPlan{}, changeSets: map[string]ChangeSet{}}
}

func repositoryKey(organizationID contract.OrganizationID, projectID contract.ProjectID, id string) string {
	return string(organizationID) + "/" + string(projectID) + "/" + id
}

func (r *memoryRepository) CreatePlan(_ context.Context, plan DeliveryPlan, version DeliveryPlanVersion) (DeliveryPlan, error) {
	key := repositoryKey(plan.OrganizationID, plan.ProjectID, plan.ID)
	plan.CurrentVersion = cloneVersion(version)
	plan.Versions = []DeliveryPlanVersion{cloneVersion(version)}
	r.plans[key] = plan
	return plan, nil
}

func (r *memoryRepository) UpdatePlan(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string, expectedVersion int, version DeliveryPlanVersion) (DeliveryPlan, error) {
	key := repositoryKey(organizationID, projectID, id)
	plan, ok := r.plans[key]
	if !ok {
		return DeliveryPlan{}, ErrNotFound
	}
	if plan.CurrentVersionNumber != expectedVersion {
		return DeliveryPlan{}, ErrPlanVersionConflict
	}
	plan.Version = int64(version.VersionNumber)
	plan.CurrentVersionNumber = version.VersionNumber
	plan.CurrentVersion = cloneVersion(version)
	plan.Versions = append(plan.Versions, cloneVersion(version))
	plan.Name, plan.Objective = version.Name, version.Objective
	plan.BudgetCents, plan.StartAt, plan.EndAt = version.Budget.TotalMinor, version.Schedule.StartAt, version.Schedule.EndAt
	plan.Scenario, plan.UpdatedAt = version.Scenario, version.CreatedAt
	r.plans[key] = plan
	return plan, nil
}

func (r *memoryRepository) ListPlans(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, _ int) ([]DeliveryPlan, error) {
	values := make([]DeliveryPlan, 0)
	for _, plan := range r.plans {
		if plan.OrganizationID == organizationID && plan.ProjectID == projectID {
			values = append(values, plan)
		}
	}
	return values, nil
}

func (r *memoryRepository) GetPlan(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (DeliveryPlan, error) {
	value, ok := r.plans[repositoryKey(organizationID, projectID, id)]
	if !ok {
		return DeliveryPlan{}, ErrNotFound
	}
	return value, nil
}

func (r *memoryRepository) ListPlanVersions(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) ([]DeliveryPlanVersion, error) {
	plan, err := r.GetPlan(ctx, organizationID, projectID, id)
	return plan.Versions, err
}

func (r *memoryRepository) GetPlanVersion(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string, version int) (DeliveryPlanVersion, error) {
	values, err := r.ListPlanVersions(ctx, organizationID, projectID, id)
	if err != nil {
		return DeliveryPlanVersion{}, err
	}
	for _, value := range values {
		if value.VersionNumber == version {
			return value, nil
		}
	}
	return DeliveryPlanVersion{}, ErrNotFound
}

func (r *memoryRepository) CreateChangeSet(_ context.Context, value ChangeSet) (ChangeSet, error) {
	r.changeSets[repositoryKey(value.OrganizationID, value.ProjectID, value.ID)] = value
	return value, nil
}

func (r *memoryRepository) ListChangeSets(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, _ int) ([]ChangeSet, error) {
	values := make([]ChangeSet, 0)
	for _, value := range r.changeSets {
		if value.OrganizationID == organizationID && value.ProjectID == projectID {
			values = append(values, value)
		}
	}
	return values, nil
}

func (r *memoryRepository) GetChangeSet(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (ChangeSet, error) {
	value, ok := r.changeSets[repositoryKey(organizationID, projectID, id)]
	if !ok {
		return ChangeSet{}, ErrNotFound
	}
	return value, nil
}

func (r *memoryRepository) TransitionChangeSet(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string, expectedVersion int64, next ChangeSetStatus, actorID string, now time.Time) (ChangeSet, error) {
	value, err := r.GetChangeSet(ctx, organizationID, projectID, id)
	if err != nil {
		return ChangeSet{}, err
	}
	if value.Version != expectedVersion {
		return ChangeSet{}, ErrVersionConflict
	}
	value.Status, value.Version, value.UpdatedAt = next, value.Version+1, now
	if next == ChangeSetApproved {
		value.ApprovedBy, value.ApprovedAt = actorID, &now
	}
	r.changeSets[repositoryKey(organizationID, projectID, id)] = value
	return value, nil
}

func (r *memoryRepository) RecordExecution(_ context.Context, changeSet ChangeSet, execution Execution, evidence Evidence) (ExecutionResult, error) {
	changeSet.Status, changeSet.Version = ChangeSetExecuted, changeSet.Version+1
	r.changeSets[repositoryKey(changeSet.OrganizationID, changeSet.ProjectID, changeSet.ID)] = changeSet
	value := ExecutionResult{ChangeSet: changeSet, Execution: execution, Evidence: evidence}
	r.executions = append(r.executions, value)
	return value, nil
}

func (r *memoryRepository) ListExecutions(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, _ int) ([]ExecutionResult, error) {
	values := make([]ExecutionResult, 0)
	for _, value := range r.executions {
		if value.Execution.OrganizationID == organizationID && value.Execution.ProjectID == projectID {
			values = append(values, value)
		}
	}
	return values, nil
}

func (r *memoryRepository) CreateMetricSnapshot(_ context.Context, value DeliveryMetricSnapshot) (DeliveryMetricSnapshot, bool, error) {
	r.metrics = append(r.metrics, value)
	return value, true, nil
}

func (r *memoryRepository) ListMetricSnapshots(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, executionID string, _ int) ([]DeliveryMetricSnapshot, error) {
	values := make([]DeliveryMetricSnapshot, 0)
	for _, value := range r.metrics {
		if value.OrganizationID == organizationID && value.ProjectID == projectID && value.ExecutionID == executionID {
			values = append(values, value)
		}
	}
	return values, nil
}

var _ Repository = (*memoryRepository)(nil)
