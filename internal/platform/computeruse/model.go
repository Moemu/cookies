package computeruse

import (
	"errors"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	RunSchemaV1          = "computer-use-run/v1"
	AuthoritySchemaV1    = "computer-use-authority/v1"
	EvidenceSchemaV1     = "computer-use-evidence/v1"
	ConfirmationSchemaV1 = "computer-use-final-confirmation/v1"
)

var (
	ErrInvalidContract   = errors.New("invalid computer-use contract")
	ErrInvalidTransition = errors.New("invalid computer-use run transition")
)

type RunState string

const (
	RunQueued               RunState = "queued"
	RunEnvironmentCheck     RunState = "environment_check"
	RunAwaitingTakeover     RunState = "awaiting_takeover"
	RunPreparing            RunState = "preparing"
	RunAwaitingConfirmation RunState = "awaiting_confirmation"
	RunSubmitting           RunState = "submitting"
	RunVerifying            RunState = "verifying"
	RunSucceeded            RunState = "succeeded"
	RunFailed               RunState = "failed"
	RunPartial              RunState = "partial"
	RunResultUnknown        RunState = "result_unknown"
	RunCancelled            RunState = "cancelled"
)

type BlockingReason string

const (
	BlockFinalConfirmationRequired BlockingReason = "FINAL_CONFIRMATION_REQUIRED"
	BlockFinalConfirmationInvalid  BlockingReason = "FINAL_CONFIRMATION_INVALID"
	BlockApprovalInvalid           BlockingReason = "APPROVAL_INVALID"
	BlockLeaseInvalid              BlockingReason = "LEASE_INVALID"
	BlockKillSwitchActive          BlockingReason = "KILL_SWITCH_ACTIVE"
	BlockAccountMismatch           BlockingReason = "ACCOUNT_MISMATCH"
	BlockProjectNotAllowed         BlockingReason = "PROJECT_NOT_ALLOWED"
	BlockSiteNotAllowed            BlockingReason = "SITE_NOT_ALLOWED"
	BlockPageDrift                 BlockingReason = "PAGE_DRIFT"
	BlockWorkflowDrift             BlockingReason = "WORKFLOW_DRIFT"
	BlockSkillDrift                BlockingReason = "SKILL_DRIFT"
	BlockResultReconciliation      BlockingReason = "RESULT_RECONCILIATION_REQUIRED"
)

type Platform string

const PlatformOceanEngine Platform = "ocean_engine"

type AuthorityBinding struct {
	SchemaVersion              string                  `json:"schema_version"`
	OrganizationID             contract.OrganizationID `json:"organization_id"`
	ProjectID                  contract.ProjectID      `json:"project_id"`
	BusinessExecutionID        string                  `json:"business_execution_id"`
	ChangeSetID                string                  `json:"change_set_id"`
	ApprovalID                 string                  `json:"approval_id"`
	ApprovalActionHash         string                  `json:"approval_action_hash"`
	AccountReferenceID         string                  `json:"account_reference_id"`
	ParentPlatformProjectID    string                  `json:"parent_platform_project_id,omitempty"`
	ObjectFingerprint          string                  `json:"object_fingerprint"`
	Action                     string                  `json:"action"`
	ProjectBudgetMode          string                  `json:"project_budget_mode,omitempty"`
	ProjectBudgetLimitMinor    int64                   `json:"project_budget_limit_minor"`
	PromotionBudgetLimitMinor  int64                   `json:"promotion_budget_limit_minor"`
	BudgetLimitMinor           int64                   `json:"budget_limit_minor"`
	Currency                   string                  `json:"currency"`
	PlanCanonicalHash          string                  `json:"plan_canonical_hash"`
	IntentCanonicalHash        string                  `json:"intent_canonical_hash"`
	FeedbackCanonicalHash      string                  `json:"feedback_canonical_hash"`
	DecisionCanonicalHash      string                  `json:"decision_canonical_hash"`
	ConfigurationCanonicalHash string                  `json:"configuration_canonical_hash"`
	WorkflowID                 string                  `json:"workflow_id"`
	WorkflowCanonicalHash      string                  `json:"workflow_canonical_hash"`
	WorkflowStepID             string                  `json:"workflow_step_id"`
	SkillID                    string                  `json:"skill_id,omitempty"`
	SkillVersion               string                  `json:"skill_version,omitempty"`
}

func (b AuthorityBinding) Validate() error {
	if b.SchemaVersion != AuthoritySchemaV1 || b.OrganizationID == "" || b.ProjectID == "" ||
		b.BusinessExecutionID == "" || b.ChangeSetID == "" || b.ApprovalID == "" || b.AccountReferenceID == "" ||
		b.ObjectFingerprint == "" || b.Action == "" || b.ProjectBudgetLimitMinor < 0 || b.PromotionBudgetLimitMinor < 0 || b.BudgetLimitMinor < 0 || b.Currency != "CNY" ||
		b.WorkflowID == "" || b.WorkflowStepID == "" || (b.SkillID == "") != (b.SkillVersion == "") {
		return ErrInvalidContract
	}
	if b.Action == "create_promotions_in_existing_project" && (strings.TrimSpace(b.ParentPlatformProjectID) == "" || b.PromotionBudgetLimitMinor < 1 || b.BudgetLimitMinor != b.PromotionBudgetLimitMinor) {
		return ErrInvalidContract
	}
	for _, hash := range []string{b.ApprovalActionHash, b.PlanCanonicalHash, b.IntentCanonicalHash, b.FeedbackCanonicalHash, b.DecisionCanonicalHash, b.ConfigurationCanonicalHash, b.WorkflowCanonicalHash} {
		if !isSHA256(hash) {
			return ErrInvalidContract
		}
	}
	return nil
}

type ExecutionEnvironment struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	Platform       Platform                `json:"platform"`
	AccountID      string                  `json:"account_id"`
	Mode           string                  `json:"mode"`
	BrowserVersion string                  `json:"browser_version"`
	Region         string                  `json:"region"`
	Healthy        bool                    `json:"healthy"`
	Version        int64                   `json:"version"`
}

func (e ExecutionEnvironment) Validate() error {
	if e.ID == "" || e.OrganizationID == "" || e.ProjectID == "" || e.Platform != PlatformOceanEngine || strings.TrimSpace(e.AccountID) == "" || e.Mode != "local_visible" || strings.TrimSpace(e.BrowserVersion) == "" || strings.TrimSpace(e.Region) == "" || e.Version < 1 {
		return ErrInvalidContract
	}
	return nil
}

type BrowserProfile struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	EnvironmentID  string                  `json:"environment_id"`
	Platform       Platform                `json:"platform"`
	AccountID      string                  `json:"account_id"`
	State          string                  `json:"state"`
	Version        int64                   `json:"version"`
}

func (p BrowserProfile) Validate() error {
	if p.ID == "" || p.OrganizationID == "" || p.ProjectID == "" || p.EnvironmentID == "" || p.Platform != PlatformOceanEngine || strings.TrimSpace(p.AccountID) == "" || p.Version < 1 {
		return ErrInvalidContract
	}
	if p.State != "ready" && p.State != "takeover_required" && p.State != "disabled" {
		return ErrInvalidContract
	}
	return nil
}

type SessionLease struct {
	ID                string                  `json:"id"`
	OrganizationID    contract.OrganizationID `json:"organization_id"`
	ProjectID         contract.ProjectID      `json:"project_id"`
	RunID             string                  `json:"run_id"`
	EnvironmentID     string                  `json:"environment_id"`
	ProfileID         string                  `json:"profile_id"`
	Platform          Platform                `json:"platform"`
	AccountID         string                  `json:"account_id"`
	Holder            string                  `json:"holder"`
	FencingToken      int64                   `json:"fencing_token"`
	Version           int64                   `json:"version"`
	ExpiresAt         time.Time               `json:"expires_at"`
	HeartbeatDeadline time.Time               `json:"heartbeat_deadline"`
	ReleasedAt        *time.Time              `json:"released_at,omitempty"`
}

func (l SessionLease) ValidAt(now time.Time) bool {
	return l.ID != "" && l.RunID != "" && l.FencingToken > 0 && l.Version > 0 && l.ReleasedAt == nil && now.Before(l.ExpiresAt) && now.Before(l.HeartbeatDeadline)
}

type SitePolicy struct {
	ID                      string                  `json:"id"`
	OrganizationID          contract.OrganizationID `json:"organization_id"`
	ProjectID               contract.ProjectID      `json:"project_id"`
	Platform                Platform                `json:"platform"`
	AccountID               string                  `json:"account_id"`
	AllowedProtocols        []string                `json:"allowed_protocols"`
	AllowedHosts            []string                `json:"allowed_hosts"`
	AllowedPageKinds        []string                `json:"allowed_page_kinds"`
	AllowedPlatformProjects []string                `json:"allowed_platform_project_ids"`
	Version                 int64                   `json:"version"`
}

func (p SitePolicy) Validate() error {
	if p.ID == "" || p.OrganizationID == "" || p.ProjectID == "" || p.Platform != PlatformOceanEngine || strings.TrimSpace(p.AccountID) == "" || p.Version < 1 || len(p.AllowedProtocols) == 0 || len(p.AllowedHosts) == 0 || len(p.AllowedPageKinds) == 0 || len(p.AllowedPlatformProjects) == 0 {
		return ErrInvalidContract
	}
	for _, protocol := range p.AllowedProtocols {
		if protocol != "https" {
			return ErrInvalidContract
		}
	}
	for _, host := range p.AllowedHosts {
		if host == "" || host != strings.ToLower(host) || strings.ContainsAny(host, "*/:@") {
			return ErrInvalidContract
		}
	}
	for _, values := range [][]string{p.AllowedPageKinds, p.AllowedPlatformProjects} {
		for _, value := range values {
			if strings.TrimSpace(value) == "" {
				return ErrInvalidContract
			}
		}
	}
	return nil
}

func (p SitePolicy) Allows(rawURL, pageKind, platformProjectID string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.User != nil || parsed.Hostname() == "" {
		return false
	}
	return slices.Contains(p.AllowedProtocols, parsed.Scheme) && slices.Contains(p.AllowedHosts, strings.ToLower(parsed.Hostname())) && slices.Contains(p.AllowedPageKinds, pageKind) && slices.Contains(p.AllowedPlatformProjects, platformProjectID)
}

type KillSwitchScope string

const (
	KillSwitchGlobal       KillSwitchScope = "global"
	KillSwitchPlatform     KillSwitchScope = "platform"
	KillSwitchOrganization KillSwitchScope = "organization"
)

type KillSwitch struct {
	ID             string                  `json:"id"`
	Scope          KillSwitchScope         `json:"scope"`
	OrganizationID contract.OrganizationID `json:"organization_id,omitempty"`
	Platform       Platform                `json:"platform,omitempty"`
	Active         bool                    `json:"active"`
	Reason         string                  `json:"reason"`
	Version        int64                   `json:"version"`
	UpdatedBy      string                  `json:"updated_by"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

type ComputerUseRun struct {
	SchemaVersion  string                  `json:"schema_version"`
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	Platform       Platform                `json:"platform"`
	AccountID      string                  `json:"account_id"`
	Authority      AuthorityBinding        `json:"authority"`
	EnvironmentID  string                  `json:"environment_id"`
	ProfileID      string                  `json:"profile_id"`
	LeaseID        string                  `json:"lease_id"`
	PolicyID       string                  `json:"policy_id"`
	State          RunState                `json:"state"`
	BlockingReason BlockingReason          `json:"blocking_reason,omitempty"`
	Paused         bool                    `json:"paused"`
	TakeoverActive bool                    `json:"takeover_active"`
	Version        int64                   `json:"version"`
	IdempotencyKey string                  `json:"idempotency_key"`
	RequestHash    string                  `json:"request_hash"`
	CreatedBy      string                  `json:"created_by"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

func (r ComputerUseRun) authorizesPlatformProject(platformProjectID string) bool {
	return r.Authority.Action != "create_promotions_in_existing_project" || (r.Authority.ParentPlatformProjectID != "" && platformProjectID == r.Authority.ParentPlatformProjectID)
}

func (r ComputerUseRun) Validate() error {
	if r.SchemaVersion != RunSchemaV1 || r.ID == "" || r.OrganizationID == "" || r.ProjectID == "" || r.Platform != PlatformOceanEngine || r.AccountID == "" || r.Version < 1 || r.IdempotencyKey == "" || !isSHA256(r.RequestHash) {
		return ErrInvalidContract
	}
	if err := r.Authority.Validate(); err != nil || r.Authority.OrganizationID != r.OrganizationID || r.Authority.ProjectID != r.ProjectID || r.Authority.AccountReferenceID != r.AccountID {
		return ErrInvalidContract
	}
	if _, ok := runTransitions[r.State]; !ok {
		return ErrInvalidContract
	}
	return nil
}

var runTransitions = map[RunState][]RunState{
	RunQueued:               {RunEnvironmentCheck, RunCancelled},
	RunEnvironmentCheck:     {RunAwaitingTakeover, RunPreparing, RunFailed, RunCancelled},
	RunAwaitingTakeover:     {RunEnvironmentCheck, RunPreparing, RunCancelled},
	RunPreparing:            {RunAwaitingTakeover, RunAwaitingConfirmation, RunFailed, RunPartial, RunResultUnknown, RunCancelled},
	RunAwaitingConfirmation: {RunAwaitingTakeover, RunPreparing, RunSubmitting, RunFailed, RunCancelled},
	RunSubmitting:           {RunVerifying, RunFailed, RunPartial, RunResultUnknown},
	RunVerifying:            {RunSucceeded, RunFailed, RunPartial, RunResultUnknown},
	RunSucceeded:            {}, RunFailed: {}, RunPartial: {}, RunResultUnknown: {}, RunCancelled: {},
}

func CanTransition(from, to RunState) bool { return slices.Contains(runTransitions[from], to) }

type StepStatus string

const (
	StepPending       StepStatus = "pending"
	StepRunning       StepStatus = "running"
	StepSucceeded     StepStatus = "succeeded"
	StepFailed        StepStatus = "failed"
	StepResultUnknown StepStatus = "result_unknown"
	StepSkipped       StepStatus = "skipped"
)

type RunStep struct {
	ID             string         `json:"id"`
	RunID          string         `json:"run_id"`
	Sequence       int            `json:"sequence"`
	WorkflowStepID string         `json:"workflow_step_id"`
	Action         string         `json:"action"`
	Status         StepStatus     `json:"status"`
	BlockingReason BlockingReason `json:"blocking_reason,omitempty"`
	Attempt        int            `json:"attempt"`
	Version        int64          `json:"version"`
}

type RunEvent struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	RunID          string                  `json:"run_id"`
	Sequence       int64                   `json:"sequence"`
	Kind           string                  `json:"kind"`
	Summary        string                  `json:"summary"`
	Actor          string                  `json:"actor"`
	CreatedAt      time.Time               `json:"created_at"`
}

type Evidence struct {
	SchemaVersion       string                  `json:"schema_version"`
	ID                  string                  `json:"id"`
	OrganizationID      contract.OrganizationID `json:"organization_id"`
	ProjectID           contract.ProjectID      `json:"project_id"`
	RunID               string                  `json:"run_id"`
	StepID              string                  `json:"step_id"`
	BeforePageFacts     map[string]string       `json:"before_page_facts"`
	AfterPageFacts      map[string]string       `json:"after_page_facts"`
	FieldReadback       map[string]string       `json:"field_readback"`
	DiffKeys            []string                `json:"diff_keys"`
	PageReference       string                  `json:"page_reference"`
	ScreenshotReference string                  `json:"screenshot_reference,omitempty"`
	ObjectFingerprint   string                  `json:"object_fingerprint"`
	SkillVersion        string                  `json:"skill_version,omitempty"`
	SelectorVersion     string                  `json:"selector_version"`
	ActionVersion       string                  `json:"action_version"`
	RedactionVersion    string                  `json:"redaction_version"`
	CreatedAt           time.Time               `json:"created_at"`
}

type TakeoverEvidenceAction string

const (
	TakeoverObservePage   TakeoverEvidenceAction = "observe_page"
	TakeoverBeginFormFill TakeoverEvidenceAction = "begin_form_fill"
	TakeoverFieldReadback TakeoverEvidenceAction = "field_readback"
	TakeoverDiscardDraft  TakeoverEvidenceAction = "discard_draft"
	TakeoverVerifyNoWrite TakeoverEvidenceAction = "verify_no_write"
)

func (a TakeoverEvidenceAction) Valid() bool {
	return slices.Contains([]TakeoverEvidenceAction{TakeoverObservePage, TakeoverBeginFormFill, TakeoverFieldReadback, TakeoverDiscardDraft, TakeoverVerifyNoWrite}, a)
}

type TakeoverWriteOutcome string

const (
	TakeoverResultObserved TakeoverWriteOutcome = "result_observed"
	TakeoverListConfirmed  TakeoverWriteOutcome = "list_confirmed"
	TakeoverWriteRejected  TakeoverWriteOutcome = "rejected_or_error"
	TakeoverResultUnknown  TakeoverWriteOutcome = "result_unknown"
)

func (o TakeoverWriteOutcome) Valid() bool {
	return slices.Contains([]TakeoverWriteOutcome{TakeoverResultObserved, TakeoverListConfirmed, TakeoverWriteRejected, TakeoverResultUnknown}, o)
}

type FinalConfirmation struct {
	SchemaVersion  string                  `json:"schema_version"`
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	RunID          string                  `json:"run_id"`
	BindingHash    string                  `json:"binding_hash"`
	TokenDigest    string                  `json:"-"`
	IssuedBy       string                  `json:"issued_by"`
	IssuedAt       time.Time               `json:"issued_at"`
	ExpiresAt      time.Time               `json:"expires_at"`
	ConsumedAt     *time.Time              `json:"consumed_at,omitempty"`
	RejectedAt     *time.Time              `json:"rejected_at,omitempty"`
	InvalidatedAt  *time.Time              `json:"invalidated_at,omitempty"`
	Version        int64                   `json:"version"`
}

func (c FinalConfirmation) UsableAt(now time.Time) bool {
	return c.SchemaVersion == ConfirmationSchemaV1 && c.ID != "" && c.RunID != "" && isSHA256(c.BindingHash) && isSHA256(c.TokenDigest) && c.Version > 0 && c.ConsumedAt == nil && c.RejectedAt == nil && c.InvalidatedAt == nil && now.Before(c.ExpiresAt)
}

type ControlledActionAttempt struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	RunID          string                  `json:"run_id"`
	StepID         string                  `json:"step_id"`
	ConfirmationID string                  `json:"confirmation_id"`
	ApprovalID     string                  `json:"approval_id"`
	LeaseID        string                  `json:"lease_id"`
	FencingToken   int64                   `json:"fencing_token"`
	ActionHash     string                  `json:"action_hash"`
	IdempotencyKey string                  `json:"idempotency_key"`
	Status         string                  `json:"status"`
	CreatedAt      time.Time               `json:"created_at"`
}

const (
	ControlledActionAuthorized    = "authorized"
	ControlledActionVerified      = "verified"
	ControlledActionFailed        = "failed"
	ControlledActionResultUnknown = "result_unknown"
)

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}
