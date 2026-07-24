package delivery

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestDeliveryLifecyclePersistsApprovalExecutionEvidenceAndRollback(t *testing.T) {
	t.Parallel()
	service := testService()
	actor := testActor()

	plan, err := service.CreatePlan(context.Background(), actor, "project_1", CreatePlanRequest{
		CreativePackageID: "creativepackage_1",
		Name:              "小红书首轮投放",
		Objective:         "验证点击意向",
		BudgetCents:       100000,
		StartAt:           time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC),
		EndAt:             time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	changeSet, err := service.CreateChangeSet(context.Background(), actor, "project_1", plan.ID, plan.Version)
	if err != nil {
		t.Fatal(err)
	}
	changeSet, err = service.Preflight(context.Background(), actor, "project_1", changeSet.ID, changeSet.Version)
	if err != nil {
		t.Fatal(err)
	}
	if changeSet.Status != ChangeSetPreflightPassed {
		t.Fatalf("preflight status=%q", changeSet.Status)
	}
	changeSet, err = service.Approve(context.Background(), actor, "project_1", changeSet.ID, changeSet.Version)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Execute(context.Background(), actor, "project_1", changeSet.ID, changeSet.Version)
	if err != nil {
		t.Fatal(err)
	}
	if result.Execution.Mode != ExecutionModeLocalSimulation || result.ChangeSet.Status != ChangeSetExecuted {
		t.Fatalf("result=%#v", result)
	}
	if result.Evidence.Summary == "" || !result.Evidence.Reversible {
		t.Fatalf("evidence=%#v", result.Evidence)
	}
	rolledBack, err := service.Rollback(context.Background(), actor, "project_1", changeSet.ID, result.ChangeSet.Version)
	if err != nil {
		t.Fatal(err)
	}
	if rolledBack.Status != ChangeSetRolledBack {
		t.Fatalf("rollback status=%q", rolledBack.Status)
	}
}

func TestDeliveryRejectsStaleChangeSetVersion(t *testing.T) {
	t.Parallel()
	service := testService()
	actor := testActor()
	plan, _ := service.CreatePlan(context.Background(), actor, "project_1", validPlanRequest())
	changeSet, _ := service.CreateChangeSet(context.Background(), actor, "project_1", plan.ID, plan.Version)
	if _, err := service.Preflight(context.Background(), actor, "project_1", changeSet.ID, changeSet.Version+1); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("error=%v", err)
	}
}

func TestExecutionEvidenceProjectionDoesNotRequireDeliveryReadScope(t *testing.T) {
	t.Parallel()
	service := testService()
	actor := testActor()
	plan, _ := service.CreatePlan(context.Background(), actor, "project_1", validPlanRequest())
	changeSet, _ := service.CreateChangeSet(context.Background(), actor, "project_1", plan.ID, plan.Version)
	changeSet, _ = service.Preflight(context.Background(), actor, "project_1", changeSet.ID, changeSet.Version)
	changeSet, _ = service.Approve(context.Background(), actor, "project_1", changeSet.ID, changeSet.Version)
	if _, err := service.Execute(context.Background(), actor, "project_1", changeSet.ID, changeSet.Version); err != nil {
		t.Fatal(err)
	}

	insightsActor := actor
	insightsActor.Scopes = []contract.Scope{"insights.read"}
	if _, err := service.ListExecutions(context.Background(), insightsActor, "project_1", 10); err == nil {
		t.Fatal("public Delivery query must still require delivery.read")
	}
	values, err := service.ListExecutionEvidence(context.Background(), insightsActor, "project_1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].Evidence.ID == "" {
		t.Fatalf("evidence values=%#v", values)
	}
}

func validPlanRequest() CreatePlanRequest {
	return CreatePlanRequest{
		CreativePackageID: "creativepackage_1", Name: "投放计划", Objective: "验证素材",
		BudgetCents: 10000, StartAt: time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC),
		EndAt: time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC),
	}
}

func testActor() contract.ActorContext {
	return contract.ActorContext{
		OrganizationID: "org_1",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
		Scopes:         []contract.Scope{ScopeRead, ScopeWrite, ScopeApprove, ScopeExecute},
	}
}

func testService() Service {
	sequence := 0
	return Service{
		Repository: &memoryRepository{
			plans:      map[string]DeliveryPlan{},
			changeSets: map[string]ChangeSet{},
			executions: map[string]Execution{},
			evidence:   map[string]Evidence{},
		},
		Projects: testProjects{},
		Packages: testPackages{},
		Now: func() time.Time {
			return time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
		},
		NewID: func(prefix string) (string, error) {
			sequence++
			return fmt.Sprintf("%s_%d", prefix, sequence), nil
		},
	}
}

type testProjects struct{}

func (testProjects) RequireActiveContext(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID) (contract.ProjectContext, error) {
	return contract.ProjectContext{OrganizationID: actor.OrganizationID, ProjectID: projectID, ProjectContextVersion: 1}, nil
}

type testPackages struct{}

func (testPackages) ReadCreativePackage(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, id string) (CreativePackageSnapshot, error) {
	return CreativePackageSnapshot{ID: id, ContentHash: "sha256:creative", CreativeVersionID: "creativeversion_1"}, nil
}

type memoryRepository struct {
	plans      map[string]DeliveryPlan
	changeSets map[string]ChangeSet
	executions map[string]Execution
	evidence   map[string]Evidence
}

func (r *memoryRepository) CreatePlan(_ context.Context, value DeliveryPlan) (DeliveryPlan, error) {
	r.plans[value.ID] = value
	return value, nil
}

func (r *memoryRepository) ListPlans(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, _ int) ([]DeliveryPlan, error) {
	values := make([]DeliveryPlan, 0)
	for _, value := range r.plans {
		if value.OrganizationID == organizationID && value.ProjectID == projectID {
			values = append(values, value)
		}
	}
	return values, nil
}

func (r *memoryRepository) GetPlan(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (DeliveryPlan, error) {
	value, ok := r.plans[id]
	if !ok || value.OrganizationID != organizationID || value.ProjectID != projectID {
		return DeliveryPlan{}, ErrNotFound
	}
	return value, nil
}

func (r *memoryRepository) CreateChangeSet(_ context.Context, value ChangeSet) (ChangeSet, error) {
	r.changeSets[value.ID] = value
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
	value, ok := r.changeSets[id]
	if !ok || value.OrganizationID != organizationID || value.ProjectID != projectID {
		return ChangeSet{}, ErrNotFound
	}
	return value, nil
}

func (r *memoryRepository) TransitionChangeSet(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string, expectedVersion int64, next ChangeSetStatus, actorID string, now time.Time) (ChangeSet, error) {
	value, err := r.GetChangeSet(context.Background(), organizationID, projectID, id)
	if err != nil {
		return ChangeSet{}, err
	}
	if value.Version != expectedVersion {
		return ChangeSet{}, ErrVersionConflict
	}
	value.Status = next
	value.Version++
	value.UpdatedAt = now
	if next == ChangeSetApproved {
		value.ApprovedBy = actorID
		value.ApprovedAt = &now
	}
	r.changeSets[id] = value
	return value, nil
}

func (r *memoryRepository) RecordExecution(_ context.Context, changeSet ChangeSet, execution Execution, evidence Evidence) (ExecutionResult, error) {
	current := r.changeSets[changeSet.ID]
	if current.Version != changeSet.Version {
		return ExecutionResult{}, ErrVersionConflict
	}
	changeSet.Status = ChangeSetExecuted
	changeSet.Version++
	changeSet.UpdatedAt = execution.CompletedAt
	r.changeSets[changeSet.ID] = changeSet
	r.executions[execution.ID] = execution
	r.evidence[evidence.ID] = evidence
	return ExecutionResult{ChangeSet: changeSet, Execution: execution, Evidence: evidence}, nil
}

func (r *memoryRepository) ListExecutions(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, _ int) ([]ExecutionResult, error) {
	values := make([]ExecutionResult, 0)
	for _, execution := range r.executions {
		if execution.OrganizationID != organizationID || execution.ProjectID != projectID {
			continue
		}
		for _, evidence := range r.evidence {
			if evidence.ExecutionID == execution.ID {
				values = append(values, ExecutionResult{ChangeSet: r.changeSets[execution.ChangeSetID], Execution: execution, Evidence: evidence})
			}
		}
	}
	return values, nil
}
