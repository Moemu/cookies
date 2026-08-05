package strategy

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/agent"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

const (
	ScopeRead        contract.Scope = "strategy.read"
	ScopeWrite       contract.Scope = "strategy.write"
	ScopeConfirm     contract.Scope = "strategy.confirm"
	ScopeReview      contract.Scope = "strategy.review"
	ScopeApprove     contract.Scope = "strategy.approve"
	ScopePackageRead contract.Scope = "strategy.package.read"
)

type Workspace struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	Name           string                  `json:"name"`
	IsPrimary      bool                    `json:"is_primary"`
	Status         string                  `json:"status"`
	Version        int64                   `json:"version"`
	CreatedBy      string                  `json:"created_by"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

type Conversation struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	WorkspaceID    string                  `json:"workspace_id"`
	Status         string                  `json:"status"`
	Version        int64                   `json:"version"`
	CreatedBy      string                  `json:"created_by"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

type Message struct {
	ID              string                  `json:"id"`
	OrganizationID  contract.OrganizationID `json:"organization_id"`
	ProjectID       contract.ProjectID      `json:"project_id"`
	ConversationID  string                  `json:"conversation_id"`
	Role            string                  `json:"role"`
	ContentType     string                  `json:"content_type"`
	Content         string                  `json:"content"`
	ContentBlocks   []MessageContentBlock   `json:"content_blocks,omitempty"`
	RequestedPolicy *MessageRequestedPolicy `json:"requested_policy,omitempty"`
	AIGenerated     bool                    `json:"ai_generated"`
	AgentTaskID     string                  `json:"agent_task_id,omitempty"`
	SkillRunIDs     []string                `json:"skill_run_ids,omitempty"`
	CreatedBy       string                  `json:"created_by"`
	CreatedAt       time.Time               `json:"created_at"`
}

type ConversationEvent struct {
	Sequence       int64           `json:"sequence"`
	ID             string          `json:"event_id"`
	ConversationID string          `json:"conversation_id"`
	Type           string          `json:"event_type"`
	Payload        json.RawMessage `json:"payload"`
	CreatedAt      time.Time       `json:"created_at"`
}

type Task struct {
	ID                 string                  `json:"id"`
	OrganizationID     contract.OrganizationID `json:"organization_id"`
	ProjectID          contract.ProjectID      `json:"project_id"`
	WorkspaceID        string                  `json:"workspace_id"`
	ConversationID     string                  `json:"conversation_id"`
	BriefID            string                  `json:"brief_id"`
	CurrentAgentTaskID string                  `json:"current_agent_task_id,omitempty"`
	CurrentStrategyID  string                  `json:"current_strategy_id,omitempty"`
	Status             string                  `json:"status"`
	DiscardedAt        *time.Time              `json:"discarded_at,omitempty"`
	DiscardedBy        string                  `json:"discarded_by,omitempty"`
	DiscardReason      string                  `json:"discard_reason,omitempty"`
	Version            int64                   `json:"version"`
	CreatedAt          time.Time               `json:"created_at"`
	UpdatedAt          time.Time               `json:"updated_at"`
}

type CreateTaskRequest struct {
	Name      string `json:"name"`
	Objective string `json:"objective"`
}

type LifecycleRequest struct {
	ExpectedVersion int64  `json:"expected_version"`
	Reason          string `json:"reason,omitempty"`
}

type TaskListItem struct {
	Task                  Task       `json:"task"`
	Name                  string     `json:"name"`
	Objective             string     `json:"objective"`
	BriefStatus           string     `json:"brief_status"`
	BriefReady            bool       `json:"brief_ready"`
	StrategyStatus        string     `json:"strategy_status,omitempty"`
	ReviewStatus          string     `json:"review_status,omitempty"`
	StrategyRevision      int64      `json:"strategy_revision"`
	StrategyVersion       int64      `json:"strategy_version"`
	StrategyArchivedAt    *time.Time `json:"strategy_archived_at,omitempty"`
	StrategyArchivedBy    string     `json:"strategy_archived_by,omitempty"`
	StrategyArchiveReason string     `json:"strategy_archive_reason,omitempty"`
}

type TaskBundle struct {
	Workspace    Workspace    `json:"workspace"`
	Conversation Conversation `json:"conversation"`
	Task         Task         `json:"task"`
	BriefDraft   BriefDraft   `json:"brief_draft"`
}

type BriefDocument struct {
	ContractVersion string                     `json:"contract_version"`
	Core            BriefCoreV3                `json:"-"`
	Facts           []BriefFactV3              `json:"-"`
	Assumptions     []BriefAssumptionV3        `json:"-"`
	Unknowns        []BriefUnknownV3           `json:"-"`
	Conflicts       []BriefConflictV3          `json:"-"`
	AssetRefs       []contract.AssetVersionRef `json:"-"`
	Extensions      map[string]json.RawMessage `json:"-"`
	Brand           BriefBrand                 `json:"brand,omitempty"`
	Product         BriefProduct               `json:"product,omitempty"`
	Industry        string                     `json:"industry,omitempty"`
	Region          string                     `json:"region,omitempty"`
	Language        string                     `json:"language,omitempty"`
	Campaign        BriefCampaign              `json:"campaign"`
	Audience        BriefAudience              `json:"audience"`
	Proposition     string                     `json:"proposition"`
	Channels        []string                   `json:"channels"`
	Budget          BriefBudget                `json:"budget"`
	Schedule        BriefSchedule              `json:"schedule"`
	Constraints     []string                   `json:"constraints"`
	Measurement     BriefMeasurement           `json:"measurement"`
	PlatformBriefs  []BriefPlatform            `json:"platform_briefs,omitempty"`
	Creative        BriefCreative              `json:"creative,omitempty"`
	ReferenceIDs    []string                   `json:"reference_ids,omitempty"`
}

type BriefBrand struct {
	Name string `json:"name,omitempty"`
}
type BriefProduct struct {
	Name          string                     `json:"name,omitempty"`
	Category      string                     `json:"category,omitempty"`
	SellingPoints []string                   `json:"selling_points,omitempty"`
	Evidence      []string                   `json:"evidence,omitempty"`
	AssetRefs     []contract.AssetVersionRef `json:"asset_refs,omitempty"`
}
type BriefCampaign struct {
	Objective string `json:"objective"`
}
type BriefAudience struct {
	Primary    string   `json:"primary"`
	PainPoints []string `json:"pain_points,omitempty"`
	Scenarios  []string `json:"scenarios,omitempty"`
	Exclusions []string `json:"exclusions,omitempty"`
}
type BriefBudget struct {
	Total string `json:"total"`
}
type BriefSchedule struct {
	Window string `json:"window"`
}
type BriefMeasurement struct {
	PrimaryKPI string `json:"primary_kpi"`
}
type BriefPlatform struct {
	Platform       string   `json:"platform"`
	Role           string   `json:"role,omitempty"`
	ContentFormats []string `json:"content_formats,omitempty"`
	ConversionPath string   `json:"conversion_path,omitempty"`
	Budget         string   `json:"budget,omitempty"`
	PrimaryKPI     string   `json:"primary_kpi,omitempty"`
}
type BriefCreative struct {
	Tone              []string `json:"tone,omitempty"`
	MandatoryElements []string `json:"mandatory_elements,omitempty"`
	ProhibitedClaims  []string `json:"prohibited_claims,omitempty"`
}

// MarshalJSON keeps the frozen v1/v2 wire shapes stable while allowing the v3
// requirement aggregate to expose a smaller, evidence-aware contract.
func (d BriefDocument) MarshalJSON() ([]byte, error) {
	if d.ContractVersion == BriefContractVersionV3 {
		return marshalBriefDocumentV3(d)
	}
	if d.ContractVersion != "strategy-brief-version/v1" {
		type alias BriefDocument
		return json.Marshal(alias(d))
	}
	return json.Marshal(struct {
		ContractVersion string           `json:"contract_version"`
		Campaign        BriefCampaign    `json:"campaign"`
		Audience        BriefAudience    `json:"audience"`
		Proposition     string           `json:"proposition"`
		Channels        []string         `json:"channels"`
		Budget          BriefBudget      `json:"budget"`
		Schedule        BriefSchedule    `json:"schedule"`
		Constraints     []string         `json:"constraints"`
		Measurement     BriefMeasurement `json:"measurement"`
	}{
		ContractVersion: d.ContractVersion,
		Campaign:        d.Campaign,
		Audience:        d.Audience,
		Proposition:     d.Proposition,
		Channels:        d.Channels,
		Budget:          d.Budget,
		Schedule:        d.Schedule,
		Constraints:     d.Constraints,
		Measurement:     d.Measurement,
	})
}

func (d *BriefDocument) UnmarshalJSON(data []byte) error {
	var header struct {
		ContractVersion string `json:"contract_version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return err
	}
	if header.ContractVersion == BriefContractVersionV3 {
		return unmarshalBriefDocumentV3(data, d)
	}
	type alias BriefDocument
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*d = BriefDocument(value)
	return nil
}

func EmptyBriefDocument() BriefDocument {
	return BriefDocument{
		ContractVersion: "strategy-brief-version/v1",
		Channels:        []string{},
		Constraints:     []string{},
	}
}

func EmptyBriefDocumentV2() BriefDocument {
	return BriefDocument{
		ContractVersion: "strategy-brief-version/v2",
		Channels:        []string{},
		Constraints:     []string{},
		PlatformBriefs:  []BriefPlatform{},
		ReferenceIDs:    []string{},
	}
}

func EmptyBriefDocumentV3() BriefDocument {
	return BriefDocument{
		ContractVersion: BriefContractVersionV3,
		Facts:           []BriefFactV3{},
		Constraints:     []string{},
		Assumptions:     []BriefAssumptionV3{},
		Unknowns:        []BriefUnknownV3{},
		Conflicts:       []BriefConflictV3{},
		AssetRefs:       []contract.AssetVersionRef{},
		ReferenceIDs:    []string{},
		Extensions:      map[string]json.RawMessage{},
	}
}

type FieldSource struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Locator string `json:"locator,omitempty"`
}

type FieldState struct {
	FieldPath    string        `json:"field_path"`
	Source       FieldSource   `json:"source"`
	Confidence   string        `json:"confidence"`
	Confirmation string        `json:"confirmation"`
	UpdatedBy    string        `json:"updated_by"`
	UpdatedAt    time.Time     `json:"updated_at"`
	Conflicts    []FieldSource `json:"conflicts"`
}

type Completeness struct {
	Ready    bool              `json:"ready"`
	Blockers []ValidationError `json:"blockers"`
	Warnings []ValidationError `json:"warnings"`
}

type BriefDraft struct {
	ID               string                  `json:"id"`
	OrganizationID   contract.OrganizationID `json:"organization_id"`
	ProjectID        contract.ProjectID      `json:"project_id"`
	BriefID          string                  `json:"brief_id"`
	Status           string                  `json:"status"`
	Version          int64                   `json:"version"`
	BaseBriefVersion *int64                  `json:"base_brief_version,omitempty"`
	Document         BriefDocument           `json:"document"`
	FieldStates      map[string]FieldState   `json:"field_states"`
	Completeness     Completeness            `json:"completeness"`
	UpdatedBy        string                  `json:"updated_by"`
	CreatedAt        time.Time               `json:"created_at"`
	UpdatedAt        time.Time               `json:"updated_at"`
}

type BriefVersion struct {
	BriefID            string                  `json:"brief_id"`
	Version            int64                   `json:"version"`
	OrganizationID     contract.OrganizationID `json:"organization_id"`
	ProjectID          contract.ProjectID      `json:"project_id"`
	Snapshot           BriefDocument           `json:"snapshot"`
	FieldStates        map[string]FieldState   `json:"field_states"`
	ContentHash        contract.ContentHash    `json:"content_hash"`
	SourceDraftID      string                  `json:"source_draft_id"`
	SourceDraftVersion int64                   `json:"source_draft_version"`
	ConfirmedBy        string                  `json:"confirmed_by"`
	ConfirmedAt        time.Time               `json:"confirmed_at"`
}

type BriefPatchOperation struct {
	Op           string          `json:"op"`
	FieldPath    string          `json:"field_path"`
	Value        json.RawMessage `json:"value"`
	Source       FieldSource     `json:"source,omitempty"`
	Confidence   string          `json:"confidence,omitempty"`
	Confirmation string          `json:"confirmation,omitempty"`
}

type BriefPatch struct {
	ContractVersion string                `json:"contract_version,omitempty"`
	BaseVersion     int64                 `json:"base_version,omitempty"`
	ExpectedVersion int64                 `json:"expected_version,omitempty"`
	Operations      []BriefPatchOperation `json:"operations"`
	Questions       []string              `json:"questions,omitempty"`
	Warnings        []string              `json:"warnings,omitempty"`
}

type ConversationQuestion struct {
	FieldPath string `json:"field_path"`
	Text      string `json:"text"`
}

// ConversationTurnDecision is the model-facing result for one Strategy
// conversation turn. It is deliberately separate from the frozen Brief patch
// contract: conversational intent and copy may evolve without changing the
// artifact exchanged with downstream systems.
type ConversationTurnDecision struct {
	Intent            string                 `json:"intent"`
	AssistantReply    string                 `json:"assistant_reply"`
	Patch             BriefPatch             `json:"patch"`
	ConfirmFields     []string               `json:"confirm_fields"`
	FollowUpQuestions []ConversationQuestion `json:"follow_up_questions"`
	Warnings          []string               `json:"warnings,omitempty"`
}

type StrategyDocument struct {
	ContractVersion         string            `json:"contract_version"`
	Objective               string            `json:"objective"`
	Audience                StrategyAudience  `json:"audience"`
	Proposition             string            `json:"proposition"`
	ChannelStrategy         []ChannelStrategy `json:"channel_strategy"`
	CreativeRecommendations []string          `json:"creative_recommendations"`
	Constraints             []string          `json:"constraints"`
	BudgetAndCadence        BudgetAndCadence  `json:"budget_and_cadence"`
	ExperimentMatrix        []Experiment      `json:"experiment_matrix"`
	Measurement             []string          `json:"measurement"`
	AssumptionsAndGaps      []string          `json:"assumptions_and_gaps"`
	Lineage                 StrategyLineage   `json:"lineage"`
	ExecutiveSummary        string            `json:"executive_summary,omitempty"`
	CrossPlatformRole       string            `json:"cross_platform_role,omitempty"`
	PlatformPlans           []PlatformPlan    `json:"platform_plans,omitempty"`
	EvidenceRefs            []string          `json:"evidence_refs,omitempty"`
	Compliance              *ComplianceReport `json:"compliance,omitempty"`
}

type StrategyAudience struct {
	Primary  string   `json:"primary"`
	Insights []string `json:"insights"`
}
type ChannelStrategy struct {
	Platform string   `json:"platform"`
	Role     string   `json:"role"`
	Formats  []string `json:"formats"`
}
type PlatformPlan struct {
	Platform       string   `json:"platform"`
	Role           string   `json:"role"`
	AudienceAngle  string   `json:"audience_angle"`
	ContentPillars []string `json:"content_pillars"`
	Formats        []string `json:"formats"`
	ConversionPath string   `json:"conversion_path"`
	Cadence        string   `json:"cadence"`
	PrimaryKPI     string   `json:"primary_kpi"`
	CreativeIdeas  []string `json:"creative_ideas"`
	Constraints    []string `json:"constraints"`
}
type BudgetAndCadence struct {
	Budget  string `json:"budget"`
	Cadence string `json:"cadence"`
}
type Experiment struct {
	Hypothesis string `json:"hypothesis"`
	Variable   string `json:"variable"`
	Metric     string `json:"metric"`
}
type StrategyLineage struct {
	BriefID               string            `json:"brief_id"`
	BriefVersion          int64             `json:"brief_version"`
	ProjectContextVersion int64             `json:"project_context_version"`
	SkillVersions         map[string]string `json:"skill_versions"`
}

func (d StrategyDocument) Validate() error {
	if (d.ContractVersion != "strategy-draft/v1" && d.ContractVersion != "strategy-draft/v2") ||
		strings.TrimSpace(d.Objective) == "" ||
		strings.TrimSpace(d.Audience.Primary) == "" || strings.TrimSpace(d.Proposition) == "" ||
		len(d.ChannelStrategy) == 0 || d.Lineage.BriefID == "" || d.Lineage.BriefVersion < 1 ||
		d.Lineage.ProjectContextVersion < 1 {
		return fmt.Errorf("%w: strategy document is incomplete", ErrInvalidRequest)
	}
	for _, channel := range d.ChannelStrategy {
		if !supportedPlatform(channel.Platform) || len(channel.Formats) == 0 {
			return fmt.Errorf("%w: channel strategy is invalid", ErrInvalidRequest)
		}
	}
	if d.ContractVersion == "strategy-draft/v2" {
		if len(d.PlatformPlans) == 0 || strings.TrimSpace(d.ExecutiveSummary) == "" {
			return fmt.Errorf("%w: multi-platform strategy is incomplete", ErrInvalidRequest)
		}
		seen := map[string]bool{}
		for _, plan := range d.PlatformPlans {
			if !supportedPlatform(plan.Platform) || strings.TrimSpace(plan.Role) == "" ||
				strings.TrimSpace(plan.ConversionPath) == "" || len(plan.Formats) == 0 ||
				len(plan.CreativeIdeas) == 0 || seen[plan.Platform] {
				return fmt.Errorf("%w: platform plan is invalid", ErrInvalidRequest)
			}
			seen[plan.Platform] = true
		}
	}
	return nil
}

func supportedPlatform(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "xiaohongshu", "douyin", "taobao_tmall", "wechat_ecosystem":
		return true
	default:
		return false
	}
}

type ComplianceIssue struct {
	RuleID   string `json:"rule_id"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Evidence string `json:"evidence,omitempty"`
}

type ComplianceReport struct {
	ContractVersion string            `json:"contract_version"`
	ContentHash     string            `json:"content_hash,omitempty"`
	Passed          bool              `json:"passed"`
	Issues          []ComplianceIssue `json:"issues"`
	CheckedAt       time.Time         `json:"checked_at"`
}

type SkillRun struct {
	ID                    string                  `json:"id"`
	OrganizationID        contract.OrganizationID `json:"organization_id"`
	ProjectID             contract.ProjectID      `json:"project_id"`
	AgentTaskID           string                  `json:"agent_task_id"`
	SkillName             string                  `json:"skill_name"`
	SkillVersion          string                  `json:"skill_version"`
	Status                string                  `json:"status"`
	InputHash             string                  `json:"input_hash"`
	OutputHash            string                  `json:"output_hash,omitempty"`
	ProviderCode          string                  `json:"provider_code,omitempty"`
	ModelVersion          string                  `json:"model_version,omitempty"`
	GenerationMode        string                  `json:"generation_mode,omitempty"`
	ModelAlias            string                  `json:"model_alias,omitempty"`
	PromptVersion         string                  `json:"prompt_version,omitempty"`
	GenerationContextHash string                  `json:"generation_context_hash,omitempty"`
	LatencyMS             int64                   `json:"latency_ms"`
	ValidationAttempts    int                     `json:"validation_attempts"`
	QualityReport         *QualityReport          `json:"quality_report,omitempty"`
	Attempts              []SkillRunAttempt       `json:"attempts"`
	StartedAt             time.Time               `json:"started_at"`
	CompletedAt           time.Time               `json:"completed_at"`
}

type SkillRunAttempt struct {
	AttemptNo        int                       `json:"attempt_no"`
	Purpose          string                    `json:"purpose"`
	ProviderCode     string                    `json:"provider_code"`
	ModelAlias       string                    `json:"model_alias,omitempty"`
	ModelVersion     string                    `json:"model_version,omitempty"`
	RouteRevisionID  string                    `json:"route_revision_id,omitempty"`
	ResponseMode     provider.TextResponseMode `json:"response_mode,omitempty"`
	APIMode          provider.TextAPIMode      `json:"api_mode,omitempty"`
	Background       bool                      `json:"background,omitempty"`
	PromptVersion    string                    `json:"prompt_version,omitempty"`
	Usage            *provider.TokenUsage      `json:"usage,omitempty"`
	LatencyMS        int64                     `json:"latency_ms"`
	ValidationPassed bool                      `json:"validation_passed"`
	ValidationErrors []string                  `json:"validation_errors"`
	OutputHash       string                    `json:"output_hash,omitempty"`
	CreatedAt        time.Time                 `json:"created_at"`
}

type DraftRevision struct {
	StrategyID      string               `json:"strategy_id"`
	Revision        int64                `json:"revision"`
	BaseRevision    *int64               `json:"base_revision,omitempty"`
	Document        StrategyDocument     `json:"document"`
	ChangedSections []string             `json:"changed_sections"`
	ContentHash     contract.ContentHash `json:"content_hash"`
	CreatedBy       string               `json:"created_by"`
	CreatedAt       time.Time            `json:"created_at"`
}

type Draft struct {
	ID                    string                  `json:"id"`
	OrganizationID        contract.OrganizationID `json:"organization_id"`
	ProjectID             contract.ProjectID      `json:"project_id"`
	TaskID                string                  `json:"task_id"`
	BriefID               string                  `json:"brief_id"`
	BriefVersion          int64                   `json:"brief_version"`
	ProjectContextVersion int64                   `json:"project_context_version"`
	Status                string                  `json:"status"`
	ArchivedAt            *time.Time              `json:"archived_at,omitempty"`
	ArchivedBy            string                  `json:"archived_by,omitempty"`
	ArchiveReason         string                  `json:"archive_reason,omitempty"`
	CurrentRevision       int64                   `json:"current_revision"`
	CurrentReviewID       string                  `json:"current_review_id,omitempty"`
	Version               int64                   `json:"version"`
	SkillVersions         map[string]string       `json:"skill_versions"`
	Revision              *DraftRevision          `json:"revision,omitempty"`
	CreatedAt             time.Time               `json:"created_at"`
	UpdatedAt             time.Time               `json:"updated_at"`
}

type Review struct {
	ID                    string                  `json:"id"`
	OrganizationID        contract.OrganizationID `json:"organization_id"`
	ProjectID             contract.ProjectID      `json:"project_id"`
	StrategyID            string                  `json:"strategy_id"`
	CandidateRevision     int64                   `json:"candidate_revision"`
	CandidateContentHash  contract.ContentHash    `json:"candidate_content_hash"`
	BriefID               string                  `json:"brief_id"`
	BriefVersion          int64                   `json:"brief_version"`
	ProjectContextVersion int64                   `json:"project_context_version"`
	Status                string                  `json:"status"`
	DecisionReason        string                  `json:"decision_reason,omitempty"`
	DecidedBy             string                  `json:"decided_by,omitempty"`
	DecidedAt             *time.Time              `json:"decided_at,omitempty"`
	CreatedBy             string                  `json:"created_by"`
	CreatedAt             time.Time               `json:"created_at"`
	UpdatedAt             time.Time               `json:"updated_at"`
	ReviewMode            string                  `json:"review_mode,omitempty"`
	RequiredApprovals     int                     `json:"required_approvals,omitempty"`
	ApprovalCount         int                     `json:"approval_count,omitempty"`
	Assignments           []ReviewAssignment      `json:"assignments,omitempty"`
}

type ReviewAssignment struct {
	ID                string                  `json:"id"`
	OrganizationID    contract.OrganizationID `json:"organization_id"`
	ProjectID         contract.ProjectID      `json:"project_id"`
	ReviewID          string                  `json:"review_id"`
	ReviewerUserID    string                  `json:"reviewer_user_id"`
	ReviewMode        string                  `json:"review_mode"`
	Status            string                  `json:"status"`
	DecisionReason    string                  `json:"decision_reason,omitempty"`
	DecidedAt         *time.Time              `json:"decided_at,omitempty"`
	CreatedAt         time.Time               `json:"created_at"`
	UpdatedAt         time.Time               `json:"updated_at"`
	AllowSelfApproval bool                    `json:"-"`
}

type ReviewPolicy struct {
	OrganizationID    contract.OrganizationID `json:"organization_id"`
	ProjectID         contract.ProjectID      `json:"project_id"`
	Mode              string                  `json:"mode"`
	ApproverUserIDs   []string                `json:"approver_user_ids"`
	AllowSelfApproval bool                    `json:"allow_self_approval"`
	Version           int64                   `json:"version"`
	UpdatedBy         string                  `json:"updated_by,omitempty"`
	CreatedAt         time.Time               `json:"created_at,omitempty"`
	UpdatedAt         time.Time               `json:"updated_at,omitempty"`
}

type UpdateReviewPolicyRequest struct {
	Mode              string   `json:"mode"`
	ApproverUserIDs   []string `json:"approver_user_ids"`
	AllowSelfApproval bool     `json:"allow_self_approval"`
	ExpectedVersion   int64    `json:"expected_version"`
}

type ReviewComment struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	ReviewID       string                  `json:"review_id"`
	AuthorID       string                  `json:"author_id"`
	Body           string                  `json:"body"`
	CreatedAt      time.Time               `json:"created_at"`
}

type DeepReviewFinding struct {
	Severity       string   `json:"severity"`
	Section        string   `json:"section"`
	CheckType      string   `json:"check_type,omitempty"`
	StrategyPath   string   `json:"strategy_path,omitempty"`
	Title          string   `json:"title"`
	Detail         string   `json:"detail"`
	Recommendation string   `json:"recommendation"`
	BriefRefs      []string `json:"brief_refs,omitempty"`
	EvidenceRefs   []string `json:"evidence_refs,omitempty"`
}

type DeepReviewAnalysis struct {
	ID                   string                    `json:"id"`
	OrganizationID       contract.OrganizationID   `json:"organization_id"`
	ProjectID            contract.ProjectID        `json:"project_id"`
	ReviewID             string                    `json:"review_id"`
	StrategyID           string                    `json:"strategy_id"`
	CandidateRevision    int64                     `json:"candidate_revision"`
	CandidateContentHash contract.ContentHash      `json:"candidate_content_hash"`
	AgentTaskID          string                    `json:"agent_task_id"`
	Status               string                    `json:"status"`
	Summary              string                    `json:"summary,omitempty"`
	Findings             []DeepReviewFinding       `json:"findings"`
	ModelAlias           string                    `json:"model_alias,omitempty"`
	ModelVersion         string                    `json:"model_version,omitempty"`
	RouteRevisionID      string                    `json:"route_revision_id,omitempty"`
	ResponseMode         provider.TextResponseMode `json:"response_mode,omitempty"`
	APIMode              provider.TextAPIMode      `json:"api_mode,omitempty"`
	Background           bool                      `json:"background,omitempty"`
	Usage                *provider.TokenUsage      `json:"usage,omitempty"`
	LatencyMS            int64                     `json:"latency_ms,omitempty"`
	CreatedBy            string                    `json:"created_by"`
	CreatedAt            time.Time                 `json:"created_at"`
	UpdatedAt            time.Time                 `json:"updated_at"`
}

type StartDeepReviewRequest struct {
	ExpectedReviewStatus string `json:"expected_review_status"`
}

type DeepReviewStartResult struct {
	Analysis  DeepReviewAnalysis `json:"analysis"`
	AgentTask agent.Task         `json:"agent_task"`
}

type Readiness struct {
	PublishBlockers []ValidationError `json:"publish_blockers"`
	CreativeReady   bool              `json:"creative_ready"`
	DeliveryReady   bool              `json:"delivery_ready"`
	InsightsReady   bool              `json:"insights_ready"`
}

type PackageApproval struct {
	ReviewID    string               `json:"review_id"`
	ApprovedBy  string               `json:"approved_by"`
	ApprovedAt  time.Time            `json:"approved_at"`
	ContentHash contract.ContentHash `json:"content_hash,omitempty"`
}

type PackageSnapshot struct {
	ContractVersion  string                  `json:"contract_version"`
	PackageID        string                  `json:"package_id"`
	PackageVersion   int64                   `json:"package_version"`
	OrganizationID   contract.OrganizationID `json:"organization_id"`
	ProjectID        contract.ProjectID      `json:"project_id"`
	StrategyID       string                  `json:"strategy_id"`
	StrategyRevision int64                   `json:"strategy_revision"`
	Brief            BriefVersion            `json:"brief"`
	Strategy         StrategyDocument        `json:"strategy"`
	CreativeRoutes   []CreativeRoute         `json:"creative_routes,omitempty"`
	Readiness        Readiness               `json:"readiness"`
	Approval         PackageApproval         `json:"approval"`
}

type PackageVersion struct {
	PackageID      string                  `json:"package_id"`
	Version        int64                   `json:"version"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	Snapshot       PackageSnapshot         `json:"snapshot"`
	ContentHash    contract.ContentHash    `json:"content_hash"`
	Status         string                  `json:"status"`
	PublishedBy    string                  `json:"published_by"`
	PublishedAt    time.Time               `json:"published_at"`
}
