package delivery

import (
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestControlledAuthorityRejectsHistoricalOrRejectedFeedback(t *testing.T) {
	binding := validControlledBinding()
	binding.OperatorFeedbackDisposition = ObservatoryFeedbackRejected
	if err := binding.Validate(); err != ErrInvalidState {
		t.Fatalf("expected rejected feedback to be ineligible, got %v", err)
	}
}

func TestRemoteWriteApprovalHashBindsEveryAuthorityIdentity(t *testing.T) {
	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	approval := RemoteWriteApproval{SchemaVersion: RemoteWriteApprovalSchemaV1, ID: "approval_1", OrganizationID: "org_1", ProjectID: "project_1", ControlledChangeSetID: "change_1", ControlledChangeSetHash: testHash("b"), Binding: validControlledBinding(), Action: ControlledActionCreateProjectAndPromotions, Scope: "controlled_remote_write", BudgetLimitMinor: 30000, Currency: "CNY", ApprovedBy: "user_1", ApprovedAt: now, ExpiresAt: now.Add(RemoteWriteApprovalTTL)}
	var err error
	approval.ActionHash, err = approval.ComputeActionHash()
	if err != nil {
		t.Fatal(err)
	}
	if err := approval.Validate(now); err != nil {
		t.Fatalf("valid approval failed: %v", err)
	}
	approval.Binding.WorkflowCanonicalHash = testHash("c")
	if err := approval.Validate(now); err != ErrApprovalContentMismatch {
		t.Fatalf("workflow drift should invalidate action hash, got %v", err)
	}
}

func TestConfirmedPlatformMappingRequiresTwoEvidenceReads(t *testing.T) {
	createdAt := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	mapping := PlatformEntityMapping{SchemaVersion: PlatformEntityMappingV1, ID: "mapping_1", OrganizationID: "org_1", ProjectID: "project_1", AccountReferenceID: "account_1", PlanID: "plan_1", ConfigurationID: "config_1", BusinessExecutionID: "execution_1", ComputerUseRunID: "run_1", InternalObjectKind: "project", InternalObjectID: "draft_1", PlatformObjectKind: "project", PlatformObjectID: "platform_1", PlatformStatus: "pending_review", ResultEvidenceID: "evidence_result", Status: PlatformEntityMappingConfirmed, Version: 1, CreatedAt: createdAt, UpdatedAt: createdAt}
	if err := mapping.Validate(); err != ErrInvalidState {
		t.Fatalf("confirmed mapping without list evidence was accepted: %v", err)
	}
	mapping.ListEvidenceID = "evidence_list"
	if err := mapping.Validate(); err != nil {
		t.Fatalf("two-read mapping should validate: %v", err)
	}
}

func validControlledBinding() ControlledAuthorityBinding {
	return ControlledAuthorityBinding{SelectionID: "selection_1", ObservatoryRunID: "run_1", ObservatoryRunCanonicalHash: testHash("a"), OperatorFeedbackID: "feedback_1", OperatorFeedbackCanonicalHash: testHash("b"), OperatorFeedbackDisposition: ObservatoryFeedbackAccepted, PlanID: "plan_1", PlanVersion: 2, PlanCanonicalHash: testHash("c"), IntentID: "intent_1", IntentVersion: 1, IntentCanonicalHash: testHash("d"), DecisionID: "decision_1", DecisionCanonicalHash: testHash("e"), ConfigurationID: "configuration_1", ConfigurationVersion: 3, ConfigurationCanonicalHash: testHash("f"), WorkflowID: "workflow_1", WorkflowCanonicalHash: testHash("1"), AccountReferenceID: "account_1", ObjectFingerprint: "fingerprint_1", SkillID: "oceanengine-ecommerce-manual", SkillVersion: "v0.1-calibration"}
}

func TestExistingProjectControlledActionRequiresBoundParentAndPromotionBudget(t *testing.T) {
	now := time.Date(2026, 8, 13, 9, 0, 0, 0, time.UTC)
	binding := validControlledBinding()
	binding.ProjectBudgetMode = OceanEngineBudgetModeUnlimited
	binding.ParentPlatformProjectID = "platform-project-1"
	binding.PromotionBudgetLimitMinor = 30000
	change := ControlledChangeSet{SchemaVersion: ControlledChangeSetSchemaV1, ID: "change-existing", OrganizationID: "org_1", ProjectID: "project_1", Binding: binding, Action: ControlledActionCreatePromotionsInExistingProject, BudgetLimitMinor: 30000, Currency: "CNY", Status: ControlledChangeSetReady, Version: 1, CreatedBy: "operator", CreatedAt: now, UpdatedAt: now}
	change.CanonicalHash, _ = change.ComputeCanonicalHash()
	if err := change.Validate(); err != nil {
		t.Fatal(err)
	}
	change.Binding.ParentPlatformProjectID = ""
	change.CanonicalHash, _ = change.ComputeCanonicalHash()
	if err := change.Validate(); err != ErrApprovalContentMismatch {
		t.Fatalf("missing parent project err=%v", err)
	}
	change.Binding.ParentPlatformProjectID = "platform-project-1"
	change.Binding.PromotionBudgetLimitMinor = 0
	change.CanonicalHash, _ = change.ComputeCanonicalHash()
	if err := change.Validate(); err != ErrApprovalContentMismatch {
		t.Fatalf("missing promotion budget err=%v", err)
	}
	change.Binding.PromotionBudgetLimitMinor = 30000
	change.Binding.OperatorPrincipalID = "operator"
	change.CanonicalHash, _ = change.ComputeCanonicalHash()
	if err := change.Validate(); err != ErrApprovalContentMismatch {
		t.Fatalf("create action accepted a post-launch operator binding: %v", err)
	}
}

func TestFinitePromotionMutationContractsBindScheduleAndAuthorizedMaterials(t *testing.T) {
	start := time.Date(2026, 8, 15, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	schedule, err := (CompileMappedControlledChangeSetRequest{
		Action:                  ControlledActionUpdatePromotionSchedule,
		CurrentDailyBudgetMinor: 30000,
		TargetDailyBudgetMinor:  30000,
		CurrentSchedule:         &ControlledScheduleWindow{StartAt: start, EndAt: start.Add(24 * time.Hour), Timezone: "Asia/Shanghai"},
		TargetSchedule:          &ControlledScheduleWindow{StartAt: start, EndAt: start.Add(48 * time.Hour), Timezone: "Asia/Shanghai"},
	}).mutation()
	if err != nil || schedule.CurrentStateHash == schedule.TargetStateHash {
		t.Fatalf("schedule mutation=%#v err=%v", schedule, err)
	}

	materials, err := (CompileMappedControlledChangeSetRequest{
		Action:                  ControlledActionUpdatePromotionMaterials,
		CurrentDailyBudgetMinor: 30000,
		TargetDailyBudgetMinor:  30000,
		CurrentMaterials:        []ControlledMaterialReference{{ReferenceID: "asset_a", AuthorizationEvidenceID: "evidence_a"}},
		TargetMaterials:         []ControlledMaterialReference{{ReferenceID: "asset_a", AuthorizationEvidenceID: "evidence_a"}, {ReferenceID: "asset_b", AuthorizationEvidenceID: "evidence_b"}},
	}).mutation()
	if err != nil || materials.CurrentStateHash == materials.TargetStateHash {
		t.Fatalf("materials mutation=%#v err=%v", materials, err)
	}

	_, err = (CompileMappedControlledChangeSetRequest{
		Action:                  ControlledActionUpdatePromotionMaterials,
		CurrentDailyBudgetMinor: 30000,
		TargetDailyBudgetMinor:  30000,
		CurrentMaterials:        []ControlledMaterialReference{{ReferenceID: "asset_a", AuthorizationEvidenceID: "evidence_a"}},
		TargetMaterials:         []ControlledMaterialReference{{ReferenceID: "asset_b"}},
	}).mutation()
	if err != ErrInvalidRequest {
		t.Fatalf("material without authorization evidence err=%v", err)
	}
}

func testHash(character string) string {
	value := ""
	for len(value) < 64 {
		value += character
	}
	return value
}

var _ contract.OrganizationID = "org_1"
