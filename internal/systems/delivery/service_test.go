package delivery

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	frozen, err := service.GetChangeSet(context.Background(), actor, "project_a", changeSet.ID)
	if err != nil {
		t.Fatal(err)
	}
	if frozen.PlanName != plan.CurrentVersion.Name {
		t.Fatalf("ChangeSet plan name = %q, want immutable V1 name %q", frozen.PlanName, plan.CurrentVersion.Name)
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
	if changeSet.PlanName != plan.CurrentVersion.Name {
		t.Fatalf("ChangeSet plan name = %q, want %q", changeSet.PlanName, plan.CurrentVersion.Name)
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

func TestApprovalRemainsValidAfterExecutionAndRollbackLifecycleTransitions(t *testing.T) {
	service, actor, _ := newTestServiceClock()
	approved := approveGoldenChangeSet(t, &service, actor)
	approvalID := approved.Approval.ApprovalID
	approvalVersion := approved.Approval.ChangeSetVersion

	executed, err := service.Execute(context.Background(), actor, "project_a", approved.ID, approved.Version)
	if err != nil {
		t.Fatal(err)
	}
	if executed.ChangeSet.Status != ChangeSetExecuted || executed.ChangeSet.Version != approvalVersion+1 {
		t.Fatalf("unexpected executed lifecycle state: %#v", executed.ChangeSet)
	}
	refreshed, err := service.GetChangeSet(context.Background(), actor, "project_a", approved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Approval == nil || !refreshed.Approval.Valid ||
		refreshed.Approval.ApprovalID != approvalID ||
		refreshed.Approval.ChangeSetVersion != approvalVersion {
		t.Fatalf("execution invalidated the immutable approval: %#v", refreshed.Approval)
	}

	rolledBack, err := service.Rollback(context.Background(), actor, "project_a", approved.ID, refreshed.Version)
	if err != nil {
		t.Fatal(err)
	}
	if rolledBack.Status != ChangeSetRolledBack || rolledBack.Version != approvalVersion+2 {
		t.Fatalf("unexpected rolled-back lifecycle state: %#v", rolledBack)
	}
	if rolledBack.Approval == nil || !rolledBack.Approval.Valid ||
		rolledBack.Approval.ApprovalID != approvalID ||
		rolledBack.Approval.ChangeSetVersion != approvalVersion {
		t.Fatalf("rollback invalidated the immutable approval: %#v", rolledBack.Approval)
	}
}

func TestApprovalIsValidFor24HoursThenExpires(t *testing.T) {
	service, actor, setNow := newTestServiceClock()
	changeSet := approveGoldenChangeSet(t, &service, actor)
	if changeSet.Approval == nil || !changeSet.Approval.Valid {
		t.Fatalf("approval should initially be valid: %#v", changeSet.Approval)
	}
	if got := changeSet.Approval.ExpiresAt.Sub(changeSet.Approval.ApprovedAt); got != ApprovalTTL {
		t.Fatalf("approval TTL = %s, want %s", got, ApprovalTTL)
	}

	setNow(changeSet.Approval.ExpiresAt.Add(-time.Nanosecond))
	beforeExpiry, err := service.GetChangeSet(context.Background(), actor, "project_a", changeSet.ID)
	if err != nil {
		t.Fatal(err)
	}
	if beforeExpiry.Approval == nil || !beforeExpiry.Approval.Valid {
		t.Fatalf("approval should remain valid immediately before expiry: %#v", beforeExpiry.Approval)
	}

	setNow(changeSet.Approval.ExpiresAt)
	expired, err := service.GetChangeSet(context.Background(), actor, "project_a", changeSet.ID)
	if err != nil {
		t.Fatal(err)
	}
	if expired.Approval == nil || expired.Approval.Valid || expired.Approval.InvalidReason != ApprovalInvalidExpired {
		t.Fatalf("unexpected expired approval view: %#v", expired.Approval)
	}
	if _, err := service.Execute(context.Background(), actor, "project_a", changeSet.ID, changeSet.Version); !errors.Is(err, ErrApprovalExpired) {
		t.Fatalf("execute after expiry error = %v, want APPROVAL_EXPIRED", err)
	}
}

func TestPlanVersionChangePermanentlyInvalidatesApprovalEvenAfterContentReverts(t *testing.T) {
	service, actor, _ := newTestServiceClock()
	changeSet := approveGoldenChangeSet(t, &service, actor)
	plan, err := service.GetPlan(context.Background(), actor, "project_a", changeSet.PlanID)
	if err != nil {
		t.Fatal(err)
	}
	changed := goldenDraft()
	changed.Budget.TotalMinor++
	if _, err := service.UpdatePlan(context.Background(), actor, "project_a", plan.ID, UpdatePlanRequest{
		ExpectedVersion: 1, PlanDraft: changed,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdatePlan(context.Background(), actor, "project_a", plan.ID, UpdatePlanRequest{
		ExpectedVersion: 2, PlanDraft: goldenDraft(),
	}); err != nil {
		t.Fatal(err)
	}
	stale, err := service.GetChangeSet(context.Background(), actor, "project_a", changeSet.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stale.Approval == nil || stale.Approval.Valid || stale.Approval.InvalidReason != ApprovalInvalidStalePlan {
		t.Fatalf("reverted content reactivated an old approval: %#v", stale.Approval)
	}
	if _, err := service.Execute(context.Background(), actor, "project_a", changeSet.ID, changeSet.Version); !errors.Is(err, ErrStalePlanVersion) {
		t.Fatalf("execute stale approval error = %v", err)
	}
	repository := service.Repository.(*memoryRepository)
	if got := len(repository.approvals[repositoryKey(actor.OrganizationID, "project_a", changeSet.ID)]); got != 1 {
		t.Fatalf("old approval audit count = %d, want 1", got)
	}
}

func TestExecuteRejectsApprovalContentAndChangeSetVersionMismatch(t *testing.T) {
	t.Run("action hash", func(t *testing.T) {
		service, actor, _ := newTestServiceClock()
		changeSet := approveGoldenChangeSet(t, &service, actor)
		repository := service.Repository.(*memoryRepository)
		key := repositoryKey(actor.OrganizationID, "project_a", changeSet.ID)
		repository.approvals[key][0].ActionHash = "tampered"
		if _, err := service.Execute(context.Background(), actor, "project_a", changeSet.ID, changeSet.Version); !errors.Is(err, ErrApprovalContentMismatch) {
			t.Fatalf("execute tampered action hash error = %v", err)
		}
	})

	t.Run("change set version", func(t *testing.T) {
		service, actor, _ := newTestServiceClock()
		changeSet := approveGoldenChangeSet(t, &service, actor)
		repository := service.Repository.(*memoryRepository)
		key := repositoryKey(actor.OrganizationID, "project_a", changeSet.ID)
		stored := repository.changeSets[key]
		stored.Version++
		repository.changeSets[key] = stored
		if _, err := service.Execute(context.Background(), actor, "project_a", changeSet.ID, stored.Version); !errors.Is(err, ErrApprovalContentMismatch) {
			t.Fatalf("execute mismatched ChangeSetVersion error = %v", err)
		}
	})
}

func TestExecuteRejectsApprovalScopeAndBudgetExceeded(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*DeliveryApproval)
	}{
		{name: "scope", mutate: func(value *DeliveryApproval) { value.Scope = "execute_real" }},
		{name: "budget", mutate: func(value *DeliveryApproval) { value.BudgetLimitMinor-- }},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			service, actor, _ := newTestServiceClock()
			changeSet := approveGoldenChangeSet(t, &service, actor)
			repository := service.Repository.(*memoryRepository)
			key := repositoryKey(actor.OrganizationID, "project_a", changeSet.ID)
			approval := repository.approvals[key][0]
			testCase.mutate(&approval)
			var err error
			approval.ActionHash, err = ApprovalActionHash(approval)
			if err != nil {
				t.Fatal(err)
			}
			repository.approvals[key][0] = approval
			if _, err := service.Execute(context.Background(), actor, "project_a", changeSet.ID, changeSet.Version); !errors.Is(err, ErrApprovalScopeExceeded) {
				t.Fatalf("execute exceeded approval error = %v", err)
			}
		})
	}
}

func TestApprovalRequiresTrustedScopeAndProjectAndCannotBeOverwritten(t *testing.T) {
	service, actor, _ := newTestServiceClock()
	plan, err := service.CreatePlan(context.Background(), actor, "project_a", CreatePlanRequest{PlanDraft: goldenDraft()})
	if err != nil {
		t.Fatal(err)
	}
	changeSet, err := service.CreateChangeSet(context.Background(), actor, "project_a", plan.ID, plan.Version)
	if err != nil {
		t.Fatal(err)
	}
	changeSet, err = service.Preflight(context.Background(), actor, "project_a", changeSet.ID, changeSet.Version)
	if err != nil {
		t.Fatal(err)
	}

	withoutApprove := actor
	withoutApprove.Scopes = contract.ScopesFromStrings([]string{
		string(ScopeRead), string(ScopeWrite), string(ScopeExecute),
	})
	if _, err := service.Approve(context.Background(), withoutApprove, "project_a", changeSet.ID, changeSet.Version); err == nil || !strings.Contains(err.Error(), string(ScopeApprove)) {
		t.Fatalf("approve without trusted scope error = %v", err)
	}
	if _, err := service.Approve(context.Background(), actor, "project_b", changeSet.ID, changeSet.Version); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-project approve error = %v, want hidden not found", err)
	}
	otherOrganization := actor
	otherOrganization.OrganizationID = "org_b"
	if _, err := service.GetChangeSet(context.Background(), otherOrganization, "project_a", changeSet.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-organization read error = %v, want hidden not found", err)
	}
	if _, err := service.Approve(context.Background(), otherOrganization, "project_a", changeSet.ID, changeSet.Version); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-organization approve error = %v, want hidden not found", err)
	}
	if _, err := service.Approve(context.Background(), actor, "project_a", changeSet.ID, changeSet.Version-1); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale approve expected_version error = %v", err)
	}

	approved, err := service.Approve(context.Background(), actor, "project_a", changeSet.ID, changeSet.Version)
	if err != nil {
		t.Fatal(err)
	}
	if approved.Approval == nil ||
		approved.Approval.Source != SourceMock ||
		approved.Approval.Scenario != ScenarioGoldenPath ||
		approved.Approval.ApprovedBy != actor.Principal.ID ||
		approved.Approval.Scope != ApprovalScopeExecuteMock {
		t.Fatalf("unexpected approval projection: %#v", approved.Approval)
	}
	if _, err := service.Approve(context.Background(), actor, "project_a", changeSet.ID, approved.Version); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("second approval error = %v", err)
	}
	repository := service.Repository.(*memoryRepository)
	if got := len(repository.approvals[repositoryKey(actor.OrganizationID, "project_a", changeSet.ID)]); got != 1 {
		t.Fatalf("approval count = %d, want immutable singleton", got)
	}
}

func TestExecuteRequiresAuthoritativeApprovalRecord(t *testing.T) {
	service, actor, _ := newTestServiceClock()
	plan, err := service.CreatePlan(context.Background(), actor, "project_a", CreatePlanRequest{PlanDraft: goldenDraft()})
	if err != nil {
		t.Fatal(err)
	}
	changeSet, err := service.CreateChangeSet(context.Background(), actor, "project_a", plan.ID, plan.Version)
	if err != nil {
		t.Fatal(err)
	}
	changeSet, err = service.Preflight(context.Background(), actor, "project_a", changeSet.ID, changeSet.Version)
	if err != nil {
		t.Fatal(err)
	}
	repository := service.Repository.(*memoryRepository)
	key := repositoryKey(actor.OrganizationID, "project_a", changeSet.ID)
	changeSet.Status, changeSet.Version = ChangeSetApproved, changeSet.Version+1
	repository.changeSets[key] = changeSet
	if _, err := service.Execute(context.Background(), actor, "project_a", changeSet.ID, changeSet.Version); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("execute without authoritative approval error = %v", err)
	}
}

func approveGoldenChangeSet(t *testing.T, service *Service, actor contract.ActorContext) ChangeSet {
	t.Helper()
	plan, err := service.CreatePlan(context.Background(), actor, "project_a", CreatePlanRequest{PlanDraft: goldenDraft()})
	if err != nil {
		t.Fatal(err)
	}
	changeSet, err := service.CreateChangeSet(context.Background(), actor, "project_a", plan.ID, plan.Version)
	if err != nil {
		t.Fatal(err)
	}
	changeSet, err = service.Preflight(context.Background(), actor, "project_a", changeSet.ID, changeSet.Version)
	if err != nil {
		t.Fatal(err)
	}
	changeSet, err = service.Approve(context.Background(), actor, "project_a", changeSet.ID, changeSet.Version)
	if err != nil {
		t.Fatal(err)
	}
	return changeSet
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
	service, actor, _ := newTestServiceClock()
	return service, actor
}

func newTestServiceClock() (Service, contract.ActorContext, func(time.Time)) {
	repository := newMemoryRepository()
	actor := contract.ActorContext{
		OrganizationID: "org_a",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "user_a"},
		Scopes: contract.ScopesFromStrings([]string{
			string(ScopeRead), string(ScopeWrite), string(ScopeApprove), string(ScopeExecute),
		}),
	}
	counter := 0
	now := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	service := Service{
		Repository: repository,
		Projects:   testProjects{},
		Packages:   testPackages{},
		NewID: func(prefix string) (string, error) {
			counter++
			return fmt.Sprintf("%s_%d", prefix, counter), nil
		},
		Now: func() time.Time { return now },
	}
	return service, actor, func(value time.Time) { now = value }
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
	approvals  map[string][]DeliveryApproval
	executions []ExecutionResult
	metrics    []DeliveryMetricSnapshot
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		plans: map[string]DeliveryPlan{}, changeSets: map[string]ChangeSet{},
		approvals: map[string][]DeliveryApproval{},
	}
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

func (r *memoryRepository) ApproveChangeSet(ctx context.Context, changeSet ChangeSet, approval DeliveryApproval) (ChangeSet, error) {
	plan, err := r.GetPlan(ctx, changeSet.OrganizationID, changeSet.ProjectID, changeSet.PlanID)
	if err != nil {
		return ChangeSet{}, err
	}
	if plan.Version != changeSet.PlanVersion {
		return ChangeSet{}, ErrStalePlanVersion
	}
	stored, err := r.GetChangeSet(ctx, changeSet.OrganizationID, changeSet.ProjectID, changeSet.ID)
	if err != nil {
		return ChangeSet{}, err
	}
	if stored.Status != ChangeSetPreflightPassed {
		return ChangeSet{}, ErrInvalidState
	}
	if stored.Version != changeSet.Version || approval.ChangeSetVersion != stored.Version+1 {
		return ChangeSet{}, ErrVersionConflict
	}
	key := repositoryKey(changeSet.OrganizationID, changeSet.ProjectID, changeSet.ID)
	if len(r.approvals[key]) != 0 {
		return ChangeSet{}, ErrInvalidState
	}
	r.approvals[key] = append(r.approvals[key], approval)
	stored.Status, stored.Version, stored.UpdatedAt = ChangeSetApproved, approval.ChangeSetVersion, approval.ApprovedAt
	stored.ApprovedBy = approval.ApprovedBy
	approvedAt := approval.ApprovedAt
	stored.ApprovedAt = &approvedAt
	r.changeSets[key] = stored
	return stored, nil
}

func (r *memoryRepository) GetApproval(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, changeSetID string) (DeliveryApproval, error) {
	values := r.approvals[repositoryKey(organizationID, projectID, changeSetID)]
	if len(values) == 0 {
		return DeliveryApproval{}, ErrNotFound
	}
	return values[len(values)-1], nil
}

func (r *memoryRepository) RecordExecution(ctx context.Context, changeSet ChangeSet, approval DeliveryApproval, execution Execution, evidence Evidence) (ExecutionResult, error) {
	plan, err := r.GetPlan(ctx, changeSet.OrganizationID, changeSet.ProjectID, changeSet.PlanID)
	if err != nil {
		return ExecutionResult{}, err
	}
	if plan.Version != changeSet.PlanVersion {
		return ExecutionResult{}, ErrStalePlanVersion
	}
	storedApproval, err := r.GetApproval(ctx, changeSet.OrganizationID, changeSet.ProjectID, changeSet.ID)
	if errors.Is(err, ErrNotFound) {
		return ExecutionResult{}, ErrApprovalRequired
	}
	if err != nil {
		return ExecutionResult{}, err
	}
	if !execution.StartedAt.Before(storedApproval.ExpiresAt) {
		return ExecutionResult{}, ErrApprovalExpired
	}
	if !sameApproval(storedApproval, approval) {
		return ExecutionResult{}, ErrApprovalContentMismatch
	}
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
