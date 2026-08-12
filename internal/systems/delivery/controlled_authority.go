package delivery

import (
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

const ControlledActionCreateProjectAndPromotions ControlledAction = "create_project_and_promotions"

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
	ObjectFingerprint             string                         `json:"object_fingerprint"`
	SkillID                       string                         `json:"skill_id"`
	SkillVersion                  string                         `json:"skill_version"`
}

func (b ControlledAuthorityBinding) Validate() error {
	if b.SelectionID == "" || b.ObservatoryRunID == "" || b.OperatorFeedbackID == "" || b.PlanID == "" || b.PlanVersion < 1 || b.IntentID == "" || b.IntentVersion < 1 || b.DecisionID == "" || b.ConfigurationID == "" || b.ConfigurationVersion < 1 || b.WorkflowID == "" || b.AccountReferenceID == "" || b.ObjectFingerprint == "" || b.SkillID == "" || b.SkillVersion == "" {
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
	return nil
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
	if c.SchemaVersion != ControlledChangeSetSchemaV1 || c.ID == "" || c.OrganizationID == "" || c.ProjectID == "" || c.Action != ControlledActionCreateProjectAndPromotions || c.BudgetLimitMinor < 0 || c.Currency != "CNY" || c.Version < 1 || strings.TrimSpace(c.CreatedBy) == "" || c.CreatedAt.IsZero() || c.UpdatedAt.IsZero() {
		return ErrInvalidRequest
	}
	if err := c.Binding.Validate(); err != nil {
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
	if a.SchemaVersion != RemoteWriteApprovalSchemaV1 || a.ID == "" || a.OrganizationID == "" || a.ProjectID == "" || a.ControlledChangeSetID == "" || !isLowercaseSHA256(a.ControlledChangeSetHash) || a.Action != ControlledActionCreateProjectAndPromotions || a.Scope != "controlled_remote_write" || a.BudgetLimitMinor < 0 || a.Currency != "CNY" || a.ApprovedBy == "" || a.ApprovedAt.IsZero() || !a.ExpiresAt.After(a.ApprovedAt) {
		return ErrInvalidRequest
	}
	if err := a.Binding.Validate(); err != nil {
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
	ResultEvidenceID    string                      `json:"result_evidence_id"`
	ListEvidenceID      string                      `json:"list_evidence_id"`
	Status              PlatformEntityMappingStatus `json:"status"`
	Version             int64                       `json:"version"`
	CreatedAt           time.Time                   `json:"created_at"`
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
	if m.SchemaVersion != PlatformEntityMappingV1 || m.ID == "" || m.OrganizationID == "" || m.ProjectID == "" || m.AccountReferenceID == "" || m.PlanID == "" || m.ConfigurationID == "" || m.BusinessExecutionID == "" || m.ComputerUseRunID == "" || m.InternalObjectKind == "" || m.InternalObjectID == "" || m.PlatformObjectKind == "" || m.Version < 1 {
		return ErrInvalidRequest
	}
	switch m.Status {
	case PlatformEntityMappingPending:
	case PlatformEntityMappingConfirmed:
		if m.PlatformObjectID == "" || m.PlatformStatus == "" || m.ResultEvidenceID == "" || m.ListEvidenceID == "" {
			return ErrInvalidState
		}
	default:
		return ErrInvalidState
	}
	return nil
}
