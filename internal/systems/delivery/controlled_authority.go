package delivery

import (
	"slices"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	ControlledChangeSetSchemaV1 = "delivery-controlled-change-set/v1"
	RemoteWriteApprovalSchemaV1 = "delivery-remote-write-approval/v1"
	PlatformEntityMappingV1     = "delivery-platform-entity-mapping/v1"
	RemoteWriteApprovalTTL      = 30 * time.Minute
)

type ControlledChangeSetStatus string

const (
	ControlledChangeSetReady       ControlledChangeSetStatus = "ready_for_approval"
	ControlledChangeSetApproved    ControlledChangeSetStatus = "approved"
	ControlledChangeSetRejected    ControlledChangeSetStatus = "rejected"
	ControlledChangeSetExecuting   ControlledChangeSetStatus = "executing"
	ControlledChangeSetExecuted    ControlledChangeSetStatus = "executed"
	ControlledChangeSetInvalidated ControlledChangeSetStatus = "invalidated"
)

type ControlledAction string

const (
	ControlledActionCreateProjectAndPromotions        ControlledAction = "create_project_and_promotions"
	ControlledActionCreatePromotionsInExistingProject ControlledAction = "create_promotions_in_existing_project"
	ControlledActionUpdatePromotionBudget             ControlledAction = "update_promotion_budget"
	ControlledActionUpdatePromotionSchedule           ControlledAction = "update_promotion_schedule"
	ControlledActionUpdatePromotionMaterials          ControlledAction = "update_promotion_materials"
	ControlledActionPausePromotion                    ControlledAction = "pause_promotion"
)

func (a ControlledAction) Valid() bool {
	return slices.Contains([]ControlledAction{
		ControlledActionCreateProjectAndPromotions,
		ControlledActionCreatePromotionsInExistingProject,
		ControlledActionUpdatePromotionBudget,
		ControlledActionUpdatePromotionSchedule,
		ControlledActionUpdatePromotionMaterials,
		ControlledActionPausePromotion,
	}, a)
}

func (a ControlledAction) ModifiesExistingPromotion() bool {
	return slices.Contains([]ControlledAction{
		ControlledActionUpdatePromotionBudget,
		ControlledActionUpdatePromotionSchedule,
		ControlledActionUpdatePromotionMaterials,
	}, a)
}

func (a ControlledAction) ChangesExistingPromotion() bool {
	return a.ModifiesExistingPromotion() || a == ControlledActionPausePromotion
}

type ControlledScheduleWindow struct {
	StartAt  time.Time `json:"start_at"`
	EndAt    time.Time `json:"end_at"`
	Timezone string    `json:"timezone"`
}

func (s ControlledScheduleWindow) Validate() error {
	if s.StartAt.IsZero() || !s.EndAt.After(s.StartAt) || strings.TrimSpace(s.Timezone) == "" {
		return ErrInvalidRequest
	}
	return nil
}

type ControlledMaterialReference struct {
	ReferenceID             string `json:"reference_id"`
	AuthorizationEvidenceID string `json:"authorization_evidence_id"`
}

type ControlledPromotionMutation struct {
	CurrentDailyBudgetMinor int64                         `json:"current_daily_budget_minor,omitempty"`
	TargetDailyBudgetMinor  int64                         `json:"target_daily_budget_minor,omitempty"`
	CurrentSchedule         *ControlledScheduleWindow     `json:"current_schedule,omitempty"`
	TargetSchedule          *ControlledScheduleWindow     `json:"target_schedule,omitempty"`
	CurrentMaterials        []ControlledMaterialReference `json:"current_materials,omitempty"`
	TargetMaterials         []ControlledMaterialReference `json:"target_materials,omitempty"`
	CurrentStateHash        string                        `json:"current_state_hash"`
	TargetStateHash         string                        `json:"target_state_hash"`
}

func (m ControlledPromotionMutation) statePayload(action ControlledAction, target bool) (any, error) {
	dailyBudgetMinor := m.CurrentDailyBudgetMinor
	if target {
		dailyBudgetMinor = m.TargetDailyBudgetMinor
	}
	if dailyBudgetMinor < 30000 {
		return nil, ErrApprovalScopeExceeded
	}
	switch action {
	case ControlledActionUpdatePromotionBudget:
		if m.CurrentSchedule != nil || m.TargetSchedule != nil || len(m.CurrentMaterials) != 0 || len(m.TargetMaterials) != 0 || m.CurrentDailyBudgetMinor == m.TargetDailyBudgetMinor {
			return nil, ErrInvalidRequest
		}
		return struct {
			DailyBudgetMinor int64 `json:"daily_budget_minor"`
		}{dailyBudgetMinor}, nil
	case ControlledActionUpdatePromotionSchedule:
		if m.CurrentDailyBudgetMinor != m.TargetDailyBudgetMinor || len(m.CurrentMaterials) != 0 || len(m.TargetMaterials) != 0 {
			return nil, ErrApprovalContentMismatch
		}
		value := m.CurrentSchedule
		if target {
			value = m.TargetSchedule
		}
		if value == nil || value.Validate() != nil {
			return nil, ErrInvalidRequest
		}
		return struct {
			DailyBudgetMinor int64                    `json:"daily_budget_minor"`
			Schedule         ControlledScheduleWindow `json:"schedule"`
		}{dailyBudgetMinor, *value}, nil
	case ControlledActionUpdatePromotionMaterials:
		if m.CurrentDailyBudgetMinor != m.TargetDailyBudgetMinor || m.CurrentSchedule != nil || m.TargetSchedule != nil {
			return nil, ErrApprovalContentMismatch
		}
		value := m.CurrentMaterials
		if target {
			value = m.TargetMaterials
		}
		if target && len(value) == 0 {
			return nil, ErrInvalidRequest
		}
		previous := ""
		for _, reference := range value {
			if strings.TrimSpace(reference.ReferenceID) == "" || strings.TrimSpace(reference.AuthorizationEvidenceID) == "" || reference.ReferenceID <= previous {
				return nil, ErrInvalidRequest
			}
			previous = reference.ReferenceID
		}
		return struct {
			DailyBudgetMinor int64                         `json:"daily_budget_minor"`
			Materials        []ControlledMaterialReference `json:"materials"`
		}{dailyBudgetMinor, value}, nil
	default:
		return nil, ErrInvalidRequest
	}
}

func (m ControlledPromotionMutation) Validate(action ControlledAction) error {
	if !action.ModifiesExistingPromotion() || !isLowercaseSHA256(m.CurrentStateHash) || !isLowercaseSHA256(m.TargetStateHash) || m.CurrentStateHash == m.TargetStateHash {
		return ErrInvalidRequest
	}
	current, err := m.statePayload(action, false)
	if err != nil {
		return err
	}
	target, err := m.statePayload(action, true)
	if err != nil {
		return err
	}
	currentHash, err := contract.CanonicalJSONHash(current)
	if err != nil || currentHash != m.CurrentStateHash {
		return ErrApprovalContentMismatch
	}
	targetHash, err := contract.CanonicalJSONHash(target)
	if err != nil || targetHash != m.TargetStateHash {
		return ErrApprovalContentMismatch
	}
	return nil
}

type ControlledPromotionControl struct {
	CurrentDailyBudgetMinor int64  `json:"current_daily_budget_minor"`
	CurrentPlatformStatus   string `json:"current_platform_status"`
	TargetPlatformStatus    string `json:"target_platform_status"`
	CurrentStateHash        string `json:"current_state_hash"`
	TargetStateHash         string `json:"target_state_hash"`
}

func (c ControlledPromotionControl) statePayload(target bool) (any, error) {
	status := c.CurrentPlatformStatus
	if target {
		status = c.TargetPlatformStatus
	}
	if c.CurrentDailyBudgetMinor < 30000 || c.CurrentPlatformStatus != "delivering" || c.TargetPlatformStatus != "paused" {
		return nil, ErrApprovalScopeExceeded
	}
	return struct {
		DailyBudgetMinor int64  `json:"daily_budget_minor"`
		PlatformStatus   string `json:"platform_status"`
	}{c.CurrentDailyBudgetMinor, status}, nil
}

func (c ControlledPromotionControl) Validate(action ControlledAction) error {
	if action != ControlledActionPausePromotion || !isLowercaseSHA256(c.CurrentStateHash) || !isLowercaseSHA256(c.TargetStateHash) || c.CurrentStateHash == c.TargetStateHash {
		return ErrInvalidRequest
	}
	current, err := c.statePayload(false)
	if err != nil {
		return err
	}
	target, err := c.statePayload(true)
	if err != nil {
		return err
	}
	currentHash, err := contract.CanonicalJSONHash(current)
	if err != nil || currentHash != c.CurrentStateHash {
		return ErrApprovalContentMismatch
	}
	targetHash, err := contract.CanonicalJSONHash(target)
	if err != nil || targetHash != c.TargetStateHash {
		return ErrApprovalContentMismatch
	}
	return nil
}

type ControlledAuthorityBinding struct {
	SelectionID                   string                         `json:"selection_id"`
	ObservatoryRunID              string                         `json:"observatory_run_id"`
	ObservatoryRunCanonicalHash   string                         `json:"observatory_run_canonical_hash"`
	OperatorFeedbackID            string                         `json:"operator_feedback_id"`
	OperatorFeedbackCanonicalHash string                         `json:"operator_feedback_canonical_hash"`
	OperatorFeedbackDisposition   ObservatoryFeedbackDisposition `json:"operator_feedback_disposition"`
	PlanID                        string                         `json:"plan_id"`
	PlanVersion                   int                            `json:"plan_version"`
	PlanCanonicalHash             string                         `json:"plan_canonical_hash"`
	IntentID                      string                         `json:"intent_id"`
	IntentVersion                 int                            `json:"intent_version"`
	IntentCanonicalHash           string                         `json:"intent_canonical_hash"`
	DecisionID                    string                         `json:"decision_id"`
	DecisionCanonicalHash         string                         `json:"decision_canonical_hash"`
	ConfigurationID               string                         `json:"configuration_id"`
	ConfigurationVersion          int                            `json:"configuration_version"`
	ConfigurationCanonicalHash    string                         `json:"configuration_canonical_hash"`
	WorkflowID                    string                         `json:"workflow_id"`
	WorkflowCanonicalHash         string                         `json:"workflow_canonical_hash"`
	AccountReferenceID            string                         `json:"account_reference_id"`
	OperatorPrincipalID           string                         `json:"operator_principal_id,omitempty"`
	ParentPlatformProjectID       string                         `json:"parent_platform_project_id,omitempty"`
	TargetMappingID               string                         `json:"target_mapping_id,omitempty"`
	TargetMappingVersion          int64                          `json:"target_mapping_version,omitempty"`
	TargetPlatformObjectID        string                         `json:"target_platform_object_id,omitempty"`
	TargetPlatformObjectKind      string                         `json:"target_platform_object_kind,omitempty"`
	ProjectBudgetMode             string                         `json:"project_budget_mode,omitempty"`
	ProjectBudgetLimitMinor       int64                          `json:"project_budget_limit_minor"`
	PromotionBudgetLimitMinor     int64                          `json:"promotion_budget_limit_minor"`
	ObjectFingerprint             string                         `json:"object_fingerprint"`
	SkillID                       string                         `json:"skill_id,omitempty"`
	SkillVersion                  string                         `json:"skill_version,omitempty"`
	PromotionMutation             *ControlledPromotionMutation   `json:"promotion_mutation,omitempty"`
	PromotionControl              *ControlledPromotionControl    `json:"promotion_control,omitempty"`
}

func (b ControlledAuthorityBinding) Validate() error {
	if b.SelectionID == "" || b.ObservatoryRunID == "" || b.OperatorFeedbackID == "" || b.PlanID == "" || b.PlanVersion < 1 || b.IntentID == "" || b.IntentVersion < 1 || b.DecisionID == "" || b.ConfigurationID == "" || b.ConfigurationVersion < 1 || b.WorkflowID == "" || b.AccountReferenceID == "" || b.ObjectFingerprint == "" || b.ProjectBudgetLimitMinor < 0 || b.PromotionBudgetLimitMinor < 0 || (b.SkillID == "") != (b.SkillVersion == "") {
		return ErrInvalidRequest
	}
	if b.ProjectBudgetMode != "" && b.ProjectBudgetMode != OceanEngineBudgetModeDaily && b.ProjectBudgetMode != OceanEngineBudgetModeUnlimited {
		return ErrInvalidRequest
	}
	if b.OperatorFeedbackDisposition != ObservatoryFeedbackAccepted && b.OperatorFeedbackDisposition != ObservatoryFeedbackModified {
		return ErrInvalidState
	}
	for _, hash := range []string{b.ObservatoryRunCanonicalHash, b.OperatorFeedbackCanonicalHash, b.PlanCanonicalHash, b.IntentCanonicalHash, b.DecisionCanonicalHash, b.ConfigurationCanonicalHash, b.WorkflowCanonicalHash} {
		if !isLowercaseSHA256(hash) {
			return ErrApprovalContentMismatch
		}
	}
	if (b.PromotionMutation != nil || b.PromotionControl != nil) && !b.HasMutationTarget() {
		return ErrInvalidRequest
	}
	return nil
}

func (b ControlledAuthorityBinding) HasMutationTarget() bool {
	return b.TargetMappingID != "" && b.TargetMappingVersion > 0 && b.TargetPlatformObjectID != "" && b.TargetPlatformObjectKind == "promotion"
}

func (b ControlledAuthorityBinding) existingPromotionStateHashes(action ControlledAction) (string, string, error) {
	switch {
	case action.ModifiesExistingPromotion() && b.PromotionMutation != nil && b.PromotionControl == nil:
		return b.PromotionMutation.CurrentStateHash, b.PromotionMutation.TargetStateHash, nil
	case action == ControlledActionPausePromotion && b.PromotionMutation == nil && b.PromotionControl != nil:
		return b.PromotionControl.CurrentStateHash, b.PromotionControl.TargetStateHash, nil
	default:
		return "", "", ErrApprovalContentMismatch
	}
}

type ControlledChangeSet struct {
	SchemaVersion    string                     `json:"schema_version"`
	ID               string                     `json:"id"`
	OrganizationID   contract.OrganizationID    `json:"organization_id"`
	ProjectID        contract.ProjectID         `json:"project_id"`
	Binding          ControlledAuthorityBinding `json:"binding"`
	Action           ControlledAction           `json:"action"`
	BudgetLimitMinor int64                      `json:"budget_limit_minor"`
	Currency         string                     `json:"currency"`
	Status           ControlledChangeSetStatus  `json:"status"`
	CanonicalHash    string                     `json:"canonical_hash"`
	Version          int64                      `json:"version"`
	CreatedBy        string                     `json:"created_by"`
	CreatedAt        time.Time                  `json:"created_at"`
	UpdatedAt        time.Time                  `json:"updated_at"`
}

func (c ControlledChangeSet) canonicalPayload() any {
	return struct {
		SchemaVersion    string                     `json:"schema_version"`
		OrganizationID   contract.OrganizationID    `json:"organization_id"`
		ProjectID        contract.ProjectID         `json:"project_id"`
		Binding          ControlledAuthorityBinding `json:"binding"`
		Action           ControlledAction           `json:"action"`
		BudgetLimitMinor int64                      `json:"budget_limit_minor"`
		Currency         string                     `json:"currency"`
	}{c.SchemaVersion, c.OrganizationID, c.ProjectID, c.Binding, c.Action, c.BudgetLimitMinor, c.Currency}
}

func (c ControlledChangeSet) ComputeCanonicalHash() (string, error) {
	return contract.CanonicalJSONHash(c.canonicalPayload())
}

func (c ControlledChangeSet) Validate() error {
	if c.SchemaVersion != ControlledChangeSetSchemaV1 || c.ID == "" || c.OrganizationID == "" || c.ProjectID == "" || !c.Action.Valid() || c.BudgetLimitMinor < 0 || c.Currency != "CNY" || c.Version < 1 || strings.TrimSpace(c.CreatedBy) == "" || c.CreatedAt.IsZero() || c.UpdatedAt.IsZero() {
		return ErrInvalidRequest
	}
	if err := c.Binding.Validate(); err != nil {
		return err
	}
	if err := validateControlledActionBinding(c.Action, c.Binding, c.BudgetLimitMinor); err != nil {
		return err
	}
	switch c.Status {
	case ControlledChangeSetReady, ControlledChangeSetApproved, ControlledChangeSetRejected, ControlledChangeSetExecuting, ControlledChangeSetExecuted, ControlledChangeSetInvalidated:
	default:
		return ErrInvalidState
	}
	hash, err := c.ComputeCanonicalHash()
	if err != nil || hash != c.CanonicalHash {
		return ErrApprovalContentMismatch
	}
	return nil
}

type RemoteWriteApproval struct {
	SchemaVersion           string                     `json:"schema_version"`
	ID                      string                     `json:"id"`
	OrganizationID          contract.OrganizationID    `json:"organization_id"`
	ProjectID               contract.ProjectID         `json:"project_id"`
	ControlledChangeSetID   string                     `json:"controlled_change_set_id"`
	ControlledChangeSetHash string                     `json:"controlled_change_set_hash"`
	Binding                 ControlledAuthorityBinding `json:"binding"`
	Action                  ControlledAction           `json:"action"`
	Scope                   string                     `json:"scope"`
	BudgetLimitMinor        int64                      `json:"budget_limit_minor"`
	Currency                string                     `json:"currency"`
	ActionHash              string                     `json:"action_hash"`
	ApprovedBy              string                     `json:"approved_by"`
	ApprovedAt              time.Time                  `json:"approved_at"`
	ExpiresAt               time.Time                  `json:"expires_at"`
}

func (a RemoteWriteApproval) actionPayload() any {
	return struct {
		SchemaVersion           string                     `json:"schema_version"`
		OrganizationID          contract.OrganizationID    `json:"organization_id"`
		ProjectID               contract.ProjectID         `json:"project_id"`
		ControlledChangeSetID   string                     `json:"controlled_change_set_id"`
		ControlledChangeSetHash string                     `json:"controlled_change_set_hash"`
		Binding                 ControlledAuthorityBinding `json:"binding"`
		Action                  ControlledAction           `json:"action"`
		Scope                   string                     `json:"scope"`
		BudgetLimitMinor        int64                      `json:"budget_limit_minor"`
		Currency                string                     `json:"currency"`
	}{a.SchemaVersion, a.OrganizationID, a.ProjectID, a.ControlledChangeSetID, a.ControlledChangeSetHash, a.Binding, a.Action, a.Scope, a.BudgetLimitMinor, a.Currency}
}

func (a RemoteWriteApproval) ComputeActionHash() (string, error) {
	return contract.CanonicalJSONHash(a.actionPayload())
}

func (a RemoteWriteApproval) Validate(now time.Time) error {
	if a.SchemaVersion != RemoteWriteApprovalSchemaV1 || a.ID == "" || a.OrganizationID == "" || a.ProjectID == "" || a.ControlledChangeSetID == "" || !isLowercaseSHA256(a.ControlledChangeSetHash) || !a.Action.Valid() || a.Scope != "controlled_remote_write" || a.BudgetLimitMinor < 0 || a.Currency != "CNY" || a.ApprovedBy == "" || a.ApprovedAt.IsZero() || !a.ExpiresAt.After(a.ApprovedAt) {
		return ErrInvalidRequest
	}
	if err := a.Binding.Validate(); err != nil {
		return err
	}
	if err := validateControlledActionBinding(a.Action, a.Binding, a.BudgetLimitMinor); err != nil {
		return err
	}
	hash, err := a.ComputeActionHash()
	if err != nil || hash != a.ActionHash {
		return ErrApprovalContentMismatch
	}
	if !now.Before(a.ExpiresAt) {
		return ErrApprovalExpired
	}
	return nil
}

func validateControlledActionBinding(action ControlledAction, binding ControlledAuthorityBinding, budgetLimitMinor int64) error {
	switch action {
	case ControlledActionCreateProjectAndPromotions:
		if binding.ParentPlatformProjectID != "" || binding.OperatorPrincipalID != "" || hasMutationTargetFields(binding) || binding.PromotionMutation != nil || binding.PromotionControl != nil || (binding.ProjectBudgetMode != "" && budgetLimitMinor != binding.ProjectBudgetLimitMinor) {
			return ErrApprovalContentMismatch
		}
	case ControlledActionCreatePromotionsInExistingProject:
		if strings.TrimSpace(binding.ParentPlatformProjectID) == "" || binding.OperatorPrincipalID != "" || hasMutationTargetFields(binding) || binding.PromotionMutation != nil || binding.PromotionControl != nil || binding.PromotionBudgetLimitMinor < 1 || budgetLimitMinor != binding.PromotionBudgetLimitMinor {
			return ErrApprovalContentMismatch
		}
	case ControlledActionUpdatePromotionBudget, ControlledActionUpdatePromotionSchedule, ControlledActionUpdatePromotionMaterials:
		if strings.TrimSpace(binding.ParentPlatformProjectID) == "" || strings.TrimSpace(binding.OperatorPrincipalID) == "" || !binding.HasMutationTarget() || binding.PromotionMutation == nil || binding.PromotionControl != nil {
			return ErrApprovalContentMismatch
		}
		if err := binding.PromotionMutation.Validate(action); err != nil {
			return err
		}
		if budgetLimitMinor != binding.PromotionMutation.TargetDailyBudgetMinor || binding.PromotionBudgetLimitMinor != binding.PromotionMutation.TargetDailyBudgetMinor {
			return ErrApprovalScopeExceeded
		}
	case ControlledActionPausePromotion:
		if strings.TrimSpace(binding.ParentPlatformProjectID) == "" || strings.TrimSpace(binding.OperatorPrincipalID) == "" || !binding.HasMutationTarget() || binding.PromotionMutation != nil || binding.PromotionControl == nil {
			return ErrApprovalContentMismatch
		}
		if err := binding.PromotionControl.Validate(action); err != nil {
			return err
		}
		if budgetLimitMinor != binding.PromotionControl.CurrentDailyBudgetMinor || binding.PromotionBudgetLimitMinor != binding.PromotionControl.CurrentDailyBudgetMinor {
			return ErrApprovalScopeExceeded
		}
	default:
		return ErrInvalidRequest
	}
	return nil
}

func hasMutationTargetFields(binding ControlledAuthorityBinding) bool {
	return binding.TargetMappingID != "" || binding.TargetMappingVersion != 0 || binding.TargetPlatformObjectID != "" || binding.TargetPlatformObjectKind != ""
}

type PlatformEntityMappingStatus string

const (
	PlatformEntityMappingPending   PlatformEntityMappingStatus = "pending_verification"
	PlatformEntityMappingConfirmed PlatformEntityMappingStatus = "confirmed"
)

type PlatformEntityMapping struct {
	SchemaVersion       string                      `json:"schema_version"`
	ID                  string                      `json:"id"`
	OrganizationID      contract.OrganizationID     `json:"organization_id"`
	ProjectID           contract.ProjectID          `json:"project_id"`
	AccountReferenceID  string                      `json:"account_reference_id"`
	PlanID              string                      `json:"plan_id"`
	ConfigurationID     string                      `json:"configuration_id"`
	BusinessExecutionID string                      `json:"business_execution_id"`
	ComputerUseRunID    string                      `json:"computer_use_run_id"`
	InternalObjectKind  string                      `json:"internal_object_kind"`
	InternalObjectID    string                      `json:"internal_object_id"`
	PlatformObjectKind  string                      `json:"platform_object_kind"`
	PlatformObjectID    string                      `json:"platform_object_id"`
	PlatformStatus      string                      `json:"platform_status"`
	CurrentStateAction  ControlledAction            `json:"current_state_action,omitempty"`
	CurrentStateHash    string                      `json:"current_state_hash,omitempty"`
	ResultEvidenceID    string                      `json:"result_evidence_id"`
	ListEvidenceID      string                      `json:"list_evidence_id"`
	Status              PlatformEntityMappingStatus `json:"status"`
	Version             int64                       `json:"version"`
	CreatedAt           time.Time                   `json:"created_at"`
	UpdatedAt           time.Time                   `json:"updated_at"`
}

type PlatformEntityMappingRevision struct {
	MappingID           string                  `json:"mapping_id"`
	OrganizationID      contract.OrganizationID `json:"organization_id"`
	ProjectID           contract.ProjectID      `json:"project_id"`
	Version             int64                   `json:"version"`
	Action              ControlledAction        `json:"action"`
	BusinessExecutionID string                  `json:"business_execution_id"`
	ComputerUseRunID    string                  `json:"computer_use_run_id"`
	PlatformObjectID    string                  `json:"platform_object_id"`
	PlatformStatus      string                  `json:"platform_status"`
	PreviousStateAction ControlledAction        `json:"previous_state_action,omitempty"`
	PreviousStateHash   string                  `json:"previous_state_hash,omitempty"`
	CurrentStateAction  ControlledAction        `json:"current_state_action,omitempty"`
	CurrentStateHash    string                  `json:"current_state_hash,omitempty"`
	ResultEvidenceID    string                  `json:"result_evidence_id"`
	ListEvidenceID      string                  `json:"list_evidence_id"`
	CreatedAt           time.Time               `json:"created_at"`
}

type ControlledExecution struct {
	ID                    string                  `json:"id"`
	OrganizationID        contract.OrganizationID `json:"organization_id"`
	ProjectID             contract.ProjectID      `json:"project_id"`
	ControlledChangeSetID string                  `json:"controlled_change_set_id"`
	RemoteWriteApprovalID string                  `json:"remote_write_approval_id"`
	ComputerUseRunID      string                  `json:"computer_use_run_id,omitempty"`
	Status                string                  `json:"status"`
	Version               int64                   `json:"version"`
	CreatedBy             string                  `json:"created_by"`
	CreatedAt             time.Time               `json:"created_at"`
	UpdatedAt             time.Time               `json:"updated_at"`
}

func (e ControlledExecution) Validate() error {
	if e.ID == "" || e.OrganizationID == "" || e.ProjectID == "" || e.ControlledChangeSetID == "" || e.RemoteWriteApprovalID == "" || e.Version < 1 || e.CreatedBy == "" || e.CreatedAt.IsZero() || e.UpdatedAt.IsZero() {
		return ErrInvalidRequest
	}
	switch e.Status {
	case "pending", "running", "succeeded", "failed", "partial", "result_unknown", "cancelled":
		return nil
	default:
		return ErrInvalidState
	}
}

func (m PlatformEntityMapping) Validate() error {
	if m.SchemaVersion != PlatformEntityMappingV1 || m.ID == "" || m.OrganizationID == "" || m.ProjectID == "" || m.AccountReferenceID == "" || m.PlanID == "" || m.ConfigurationID == "" || m.BusinessExecutionID == "" || m.ComputerUseRunID == "" || m.InternalObjectKind == "" || m.InternalObjectID == "" || m.PlatformObjectKind == "" || m.Version < 1 || m.CreatedAt.IsZero() || m.UpdatedAt.IsZero() || (!m.CreatedAt.Equal(m.UpdatedAt) && m.UpdatedAt.Before(m.CreatedAt)) || (m.CurrentStateHash == "") != (m.CurrentStateAction == "") || (m.CurrentStateHash != "" && (!isLowercaseSHA256(m.CurrentStateHash) || !m.CurrentStateAction.ChangesExistingPromotion())) {
		return ErrInvalidRequest
	}
	switch m.Status {
	case PlatformEntityMappingPending:
	case PlatformEntityMappingConfirmed:
		if m.PlatformObjectID == "" || m.PlatformStatus == "" || m.ResultEvidenceID == "" || m.ListEvidenceID == "" || m.ResultEvidenceID == m.ListEvidenceID {
			return ErrInvalidState
		}
	default:
		return ErrInvalidState
	}
	return nil
}
