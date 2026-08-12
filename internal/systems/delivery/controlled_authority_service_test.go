package delivery

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type controlledMemoryRepository struct {
	*memoryRepository
	changes    map[string]ControlledChangeSet
	approvals  map[string]RemoteWriteApproval
	executions map[string]ControlledExecution
}

func newControlledMemoryRepository() *controlledMemoryRepository {
	return &controlledMemoryRepository{memoryRepository: newMemoryRepository(), changes: map[string]ControlledChangeSet{}, approvals: map[string]RemoteWriteApproval{}, executions: map[string]ControlledExecution{}}
}
func (r *controlledMemoryRepository) CreateControlledChangeSet(_ context.Context, v ControlledChangeSet) (ControlledChangeSet, bool, error) {
	for _, existing := range r.changes {
		if existing.CanonicalHash == v.CanonicalHash {
			return existing, true, nil
		}
	}
	r.changes[repositoryKey(v.OrganizationID, v.ProjectID, v.ID)] = v
	return v, false, nil
}
func (r *controlledMemoryRepository) GetControlledChangeSet(_ context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (ControlledChangeSet, error) {
	v, ok := r.changes[repositoryKey(org, project, id)]
	if !ok {
		return ControlledChangeSet{}, ErrNotFound
	}
	return v, nil
}
func (r *controlledMemoryRepository) ApproveControlledChangeSet(_ context.Context, c ControlledChangeSet, a RemoteWriteApproval) (ControlledChangeSet, RemoteWriteApproval, error) {
	c.Status = ControlledChangeSetApproved
	c.Version++
	c.UpdatedAt = a.ApprovedAt
	r.changes[repositoryKey(c.OrganizationID, c.ProjectID, c.ID)] = c
	r.approvals[repositoryKey(c.OrganizationID, c.ProjectID, c.ID)] = a
	return c, a, nil
}
func (r *controlledMemoryRepository) GetRemoteWriteApproval(_ context.Context, org contract.OrganizationID, project contract.ProjectID, changeID string) (RemoteWriteApproval, error) {
	v, ok := r.approvals[repositoryKey(org, project, changeID)]
	if !ok {
		return RemoteWriteApproval{}, ErrNotFound
	}
	return v, nil
}
func (r *controlledMemoryRepository) CreateControlledExecution(_ context.Context, v ControlledExecution) (ControlledExecution, error) {
	r.executions[repositoryKey(v.OrganizationID, v.ProjectID, v.ID)] = v
	return v, nil
}
func (r *controlledMemoryRepository) GetControlledExecution(_ context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (ControlledExecution, error) {
	v, ok := r.executions[repositoryKey(org, project, id)]
	if !ok {
		return ControlledExecution{}, ErrNotFound
	}
	return v, nil
}

func TestControlledAuthorityCompilesLatestReviewedStateAndApprovesExactHash(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	repo := newControlledMemoryRepository()
	counter := 0
	service := Service{Repository: repo, Projects: testProjects{}, Now: func() time.Time { return now }, NewID: func(prefix string) (string, error) { counter++; return prefix + "_test", nil }}
	actor := contract.ActorContext{OrganizationID: "org_a", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "operator_1"}, Scopes: contract.ScopesFromStrings([]string{string(ScopeWrite), string(ScopeApprove), string(ScopeExecute)})}
	selection := validObservatorySelection(t)
	selection.OrganizationID, selection.ProjectID = actor.OrganizationID, "project_a"
	selection.Workflow.OrganizationID, selection.Workflow.ProjectID = actor.OrganizationID, "project_a"
	repo.selections[repositoryKey(actor.OrganizationID, "project_a", selection.ID)] = selection
	decision := DeliveryDecision{ID: selection.DecisionID, OrganizationID: actor.OrganizationID, ProjectID: "project_a", CanonicalHash: selection.DecisionCanonicalHash, Inputs: DecisionInputBindings{PlanID: "plan_1", PlanVersion: 2, PlanCanonicalHash: selection.FinalApprovalBinding.PlanCanonicalHash, IntentID: "intent_1", IntentVersion: 1, IntentCanonicalHash: selection.FinalApprovalBinding.IntentCanonicalHash}}
	repo.decisions[repositoryKey(actor.OrganizationID, "project_a", decision.ID)] = decision
	run, err := BuildObservatoryRun(selection, validObservatoryRequest(selection, ObservatoryModePrepareNew), actor.Principal.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	repo.observatoryRuns[repositoryKey(actor.OrganizationID, "project_a", run.ID)] = run
	feedback := DeliveryObservatoryFeedback{SchemaVersion: ObservatoryFeedbackSchemaV1, ID: "feedback_1", OrganizationID: actor.OrganizationID, ProjectID: "project_a", RunID: run.ID, RunCanonicalHash: run.CanonicalHash, RunOutcome: run.Outcome, Disposition: ObservatoryFeedbackAccepted, Reason: "reviewed", DiffKeys: []string{}, CreatedBy: actor.Principal.ID, CreatedAt: now}
	feedback.CanonicalHash, _ = feedback.ComputeCanonicalHash()
	repo.observatoryFeedback[repositoryKey(actor.OrganizationID, "project_a", feedback.ID)] = feedback
	change, replay, err := service.CompileControlledChangeSet(context.Background(), actor, "project_a", CompileControlledChangeSetRequest{ObservatoryRunID: run.ID, SkillVersion: "2026-08-12"})
	if err != nil || replay {
		t.Fatalf("compile replay=%t err=%v", replay, err)
	}
	if change.Binding.OperatorFeedbackCanonicalHash != feedback.CanonicalHash || change.Binding.AccountReferenceID != selection.Workflow.AccountReference.ID {
		t.Fatalf("binding=%#v", change.Binding)
	}
	approved, approval, err := service.ApproveControlledChangeSet(context.Background(), actor, "project_a", change.ID, ApproveControlledChangeSetRequest{ExpectedVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	if approved.Status != ControlledChangeSetApproved || approval.ControlledChangeSetHash != change.CanonicalHash {
		t.Fatalf("approval=%#v", approval)
	}
	execution, err := service.CreateControlledExecution(context.Background(), actor, "project_a", change.ID)
	if err != nil {
		t.Fatal(err)
	}
	if execution.RemoteWriteApprovalID != approval.ID || execution.Status != "pending" {
		t.Fatalf("execution=%#v", execution)
	}
}
