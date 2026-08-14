package delivery

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/computeruse"
	"github.com/shikanon/cookies/internal/platform/contract"
)

type controlledMemoryRepository struct {
	*memoryRepository
	changes    map[string]ControlledChangeSet
	approvals  map[string]RemoteWriteApproval
	executions map[string]ControlledExecution
	mappings   map[string]PlatformEntityMapping
	evidence   map[string]platformMappingEvidence
}

func newControlledMemoryRepository() *controlledMemoryRepository {
	return &controlledMemoryRepository{memoryRepository: newMemoryRepository(), changes: map[string]ControlledChangeSet{}, approvals: map[string]RemoteWriteApproval{}, executions: map[string]ControlledExecution{}, mappings: map[string]PlatformEntityMapping{}, evidence: map[string]platformMappingEvidence{}}
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
func (r *controlledMemoryRepository) AttachComputerUseRun(_ context.Context, org contract.OrganizationID, project contract.ProjectID, id string, expectedVersion int64, runID string, now time.Time) (ControlledExecution, error) {
	key := repositoryKey(org, project, id)
	v, ok := r.executions[key]
	if !ok {
		return ControlledExecution{}, ErrNotFound
	}
	if v.Version != expectedVersion || v.Status != "pending" || v.ComputerUseRunID != "" {
		return ControlledExecution{}, ErrVersionConflict
	}
	v.ComputerUseRunID = runID
	v.Status = "running"
	v.Version++
	v.UpdatedAt = now
	r.executions[key] = v
	return v, nil
}
func (r *controlledMemoryRepository) CreatePlatformEntityMapping(_ context.Context, v PlatformEntityMapping) (PlatformEntityMapping, error) {
	r.mappings[repositoryKey(v.OrganizationID, v.ProjectID, v.ID)] = v
	return v, nil
}
func (r *controlledMemoryRepository) GetPlatformEntityMapping(_ context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (PlatformEntityMapping, error) {
	v, ok := r.mappings[repositoryKey(org, project, id)]
	if !ok {
		return PlatformEntityMapping{}, ErrNotFound
	}
	return v, nil
}
func (r *controlledMemoryRepository) ConfirmPlatformEntityMapping(_ context.Context, org contract.OrganizationID, project contract.ProjectID, id string, expectedVersion int64, resultEvidenceID, listEvidenceID string) (PlatformEntityMapping, error) {
	key := repositoryKey(org, project, id)
	current, ok := r.mappings[key]
	if !ok {
		return PlatformEntityMapping{}, ErrNotFound
	}
	if current.Version != expectedVersion || current.Status != PlatformEntityMappingPending {
		return PlatformEntityMapping{}, ErrVersionConflict
	}
	result, resultOK := r.evidence[resultEvidenceID]
	list, listOK := r.evidence[listEvidenceID]
	if !resultOK || !listOK {
		return PlatformEntityMapping{}, ErrNotFound
	}
	objectID, status, err := validatePlatformMappingEvidence(current, result, list)
	if err != nil {
		return PlatformEntityMapping{}, err
	}
	current.PlatformObjectID, current.PlatformStatus = objectID, status
	current.ResultEvidenceID, current.ListEvidenceID = resultEvidenceID, listEvidenceID
	current.Status, current.Version = PlatformEntityMappingConfirmed, current.Version+1
	r.mappings[key] = current
	return current, nil
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
	liveConfiguration := selection.Configuration
	liveConfiguration.Payload.OceanEngine.Project.ProjectDraftID = "platform-project-1"
	liveConfiguration.Payload.OceanEngine.Project.BudgetAndBidding.BudgetMode = OceanEngineBudgetModeUnlimited
	liveConfiguration.Payload.OceanEngine.Project.BudgetAndBidding.DailyBudgetMinor = 0
	promotionBid := int64(1)
	liveConfiguration.Payload.OceanEngine.Promotions[0].BudgetAndBidding = &OceanEngineBudgetAndBidding{BudgetMode: OceanEngineBudgetModeDaily, Currency: "CNY", DailyBudgetMinor: 30000, BiddingStrategy: "stable_cost", ChargingMode: "oCPM", BidMinor: &promotionBid}
	liveConfiguration, err = FinalizePlatformConfiguration(liveConfiguration)
	if err != nil {
		t.Fatal(err)
	}
	feedback := DeliveryObservatoryFeedback{SchemaVersion: ObservatoryFeedbackSchemaV1, ID: "feedback_1", OrganizationID: actor.OrganizationID, ProjectID: "project_a", RunID: run.ID, RunCanonicalHash: run.CanonicalHash, RunOutcome: run.Outcome, Disposition: ObservatoryFeedbackModified, Reason: "reviewed", DiffKeys: []string{"project.budget_and_bidding", "promotions.0.budget_and_bidding"}, FinalConfiguration: &liveConfiguration, FinalConfigurationCanonicalHash: liveConfiguration.CanonicalHash, CreatedBy: actor.Principal.ID, CreatedAt: now}
	feedback.CanonicalHash, _ = feedback.ComputeCanonicalHash()
	repo.observatoryFeedback[repositoryKey(actor.OrganizationID, "project_a", feedback.ID)] = feedback
	if _, _, err := service.CompileControlledChangeSet(context.Background(), actor, "project_a", CompileControlledChangeSetRequest{ObservatoryRunID: run.ID, Action: ControlledActionCreatePromotionsInExistingProject}); err != ErrInvalidRequest {
		t.Fatalf("existing-project action without parent id err=%v", err)
	}
	if _, _, err := service.CompileControlledChangeSet(context.Background(), actor, "project_a", CompileControlledChangeSetRequest{ObservatoryRunID: run.ID, Action: ControlledActionCreatePromotionsInExistingProject, ParentPlatformProjectID: "other-project"}); err != ErrApprovalContentMismatch {
		t.Fatalf("existing-project action with mismatched parent id err=%v", err)
	}
	change, replay, err := service.CompileControlledChangeSet(context.Background(), actor, "project_a", CompileControlledChangeSetRequest{ObservatoryRunID: run.ID, Action: ControlledActionCreatePromotionsInExistingProject, ParentPlatformProjectID: "platform-project-1"})
	if err != nil || replay {
		t.Fatalf("compile replay=%t err=%v", replay, err)
	}
	if change.Binding.OperatorFeedbackCanonicalHash != feedback.CanonicalHash || change.Binding.AccountReferenceID != selection.Workflow.AccountReference.ID {
		t.Fatalf("binding=%#v", change.Binding)
	}
	if change.Binding.SkillID != "oceanengine-ecommerce-manual" || change.Binding.SkillVersion != "v0.1-calibration" {
		t.Fatalf("stage B Platform Skill calibration was not bound: %#v", change.Binding)
	}
	if change.Action != ControlledActionCreatePromotionsInExistingProject || change.Binding.ParentPlatformProjectID != "platform-project-1" || change.Binding.ProjectBudgetMode != OceanEngineBudgetModeUnlimited || change.Binding.ProjectBudgetLimitMinor != 0 || change.Binding.PromotionBudgetLimitMinor != 30000 || change.BudgetLimitMinor != 30000 {
		t.Fatalf("existing-project authority boundary=%#v change=%#v", change.Binding, change)
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
	execution, err = service.AttachComputerUseRun(context.Background(), actor, "project_a", execution.ID, execution.Version, "run_1")
	if err != nil || execution.Status != "running" || execution.ComputerUseRunID != "run_1" || execution.Version != 2 {
		t.Fatalf("attached execution=%#v err=%v", execution, err)
	}
	mapping, err := service.CreatePendingPlatformEntityMapping(context.Background(), actor, PlatformEntityMapping{ID: "mapping_1", ProjectID: "project_a", AccountReferenceID: change.Binding.AccountReferenceID, PlanID: change.Binding.PlanID, ConfigurationID: change.Binding.ConfigurationID, BusinessExecutionID: execution.ID, ComputerUseRunID: execution.ComputerUseRunID, InternalObjectKind: "project", InternalObjectID: change.Binding.ObjectFingerprint, PlatformObjectKind: "project"})
	if err != nil || mapping.Status != PlatformEntityMappingPending || mapping.PlatformObjectID != "" || mapping.PlatformStatus != "" || mapping.ResultEvidenceID != "" || mapping.ListEvidenceID != "" {
		t.Fatalf("pending mapping=%#v err=%v", mapping, err)
	}
	repo.evidence["evidence_result"] = validMappingEvidence(mapping, "evidence_result", "step_result", 2, computeruse.TakeoverResultObserved, "platform_1", "pending_review")
	repo.evidence["evidence_list"] = validMappingEvidence(mapping, "evidence_list", "step_list", 3, computeruse.TakeoverListConfirmed, "platform_1", "pending_review")
	mapping, err = service.ConfirmPlatformEntityMapping(context.Background(), actor, "project_a", mapping.ID, ConfirmPlatformEntityMappingRequest{ExpectedVersion: mapping.Version, ResultEvidenceID: "evidence_result", ListEvidenceID: "evidence_list"})
	if err != nil || mapping.Status != PlatformEntityMappingConfirmed || mapping.Version != 2 {
		t.Fatalf("confirmed mapping=%#v err=%v", mapping, err)
	}
}

func validMappingEvidence(mapping PlatformEntityMapping, evidenceID, stepID string, sequence int, action computeruse.TakeoverWriteOutcome, objectID, status string) platformMappingEvidence {
	return platformMappingEvidence{
		Evidence: computeruse.Evidence{SchemaVersion: computeruse.EvidenceSchemaV1, ID: evidenceID, OrganizationID: mapping.OrganizationID, ProjectID: mapping.ProjectID, RunID: mapping.ComputerUseRunID, StepID: stepID, ObjectFingerprint: mapping.InternalObjectID, FieldReadback: map[string]string{"platform_object_id": objectID, "platform_status": status}},
		Step:     computeruse.RunStep{ID: stepID, RunID: mapping.ComputerUseRunID, Sequence: sequence, Action: string(action), Status: computeruse.StepSucceeded},
	}
}

func TestPlatformEntityMappingConfirmationRejectsUntrustedEvidence(t *testing.T) {
	now := time.Date(2026, 8, 13, 13, 0, 0, 0, time.UTC)
	actor := contract.ActorContext{OrganizationID: "org_a", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "operator"}, Scopes: contract.ScopesFromStrings([]string{string(ScopeExecute)})}
	base := PlatformEntityMapping{SchemaVersion: PlatformEntityMappingV1, ID: "mapping_1", OrganizationID: actor.OrganizationID, ProjectID: "project_a", AccountReferenceID: "account_1", PlanID: "plan_1", ConfigurationID: "configuration_1", BusinessExecutionID: "execution_1", ComputerUseRunID: "run_1", InternalObjectKind: "project", InternalObjectID: "fingerprint_1", PlatformObjectKind: "project", Status: PlatformEntityMappingPending, Version: 1, CreatedAt: now}
	tests := []struct {
		name   string
		mutate func(*controlledMemoryRepository)
		result string
		list   string
		want   error
	}{
		{name: "evidence does not exist", result: "forged_result", list: "forged_list", want: ErrNotFound},
		{name: "cross run evidence", result: "result", list: "list", want: ErrApprovalContentMismatch, mutate: func(repo *controlledMemoryRepository) {
			result := validMappingEvidence(base, "result", "step_result", 2, computeruse.TakeoverResultObserved, "platform_1", "pending_review")
			result.Evidence.RunID = "run_other"
			repo.evidence["result"] = result
			repo.evidence["list"] = validMappingEvidence(base, "list", "step_list", 3, computeruse.TakeoverListConfirmed, "platform_1", "pending_review")
		}},
		{name: "wrong step action", result: "result", list: "list", want: ErrApprovalContentMismatch, mutate: func(repo *controlledMemoryRepository) {
			repo.evidence["result"] = validMappingEvidence(base, "result", "step_result", 2, computeruse.TakeoverListConfirmed, "platform_1", "pending_review")
			repo.evidence["list"] = validMappingEvidence(base, "list", "step_list", 3, computeruse.TakeoverListConfirmed, "platform_1", "pending_review")
		}},
		{name: "forged object value", result: "result", list: "list", want: ErrApprovalContentMismatch, mutate: func(repo *controlledMemoryRepository) {
			repo.evidence["result"] = validMappingEvidence(base, "result", "step_result", 2, computeruse.TakeoverResultObserved, "platform_1", "pending_review")
			repo.evidence["list"] = validMappingEvidence(base, "list", "step_list", 3, computeruse.TakeoverListConfirmed, "platform_forged", "pending_review")
		}},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			repo := newControlledMemoryRepository()
			repo.mappings[repositoryKey(base.OrganizationID, base.ProjectID, base.ID)] = base
			if testCase.mutate != nil {
				testCase.mutate(repo)
			}
			service := Service{Repository: repo, Projects: testProjects{}, Now: func() time.Time { return now }}
			_, err := service.ConfirmPlatformEntityMapping(context.Background(), actor, base.ProjectID, base.ID, ConfirmPlatformEntityMappingRequest{ExpectedVersion: 1, ResultEvidenceID: testCase.result, ListEvidenceID: testCase.list})
			if err != testCase.want {
				t.Fatalf("err=%v want=%v", err, testCase.want)
			}
			if repo.mappings[repositoryKey(base.OrganizationID, base.ProjectID, base.ID)].Status != PlatformEntityMappingPending {
				t.Fatal("mapping was confirmed from untrusted evidence")
			}
		})
	}
}
