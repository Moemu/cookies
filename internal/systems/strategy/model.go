package strategy

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
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
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	ConversationID string                  `json:"conversation_id"`
	Role           string                  `json:"role"`
	ContentType    string                  `json:"content_type"`
	Content        string                  `json:"content"`
	AIGenerated    bool                    `json:"ai_generated"`
	AgentTaskID    string                  `json:"agent_task_id,omitempty"`
	SkillRunIDs    []string                `json:"skill_run_ids,omitempty"`
	CreatedBy      string                  `json:"created_by"`
	CreatedAt      time.Time               `json:"created_at"`
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
	Version            int64                   `json:"version"`
	CreatedAt          time.Time               `json:"created_at"`
	UpdatedAt          time.Time               `json:"updated_at"`
}

type BriefDocument struct {
	ContractVersion string           `json:"contract_version"`
	Campaign        BriefCampaign    `json:"campaign"`
	Audience        BriefAudience    `json:"audience"`
	Proposition     string           `json:"proposition"`
	Channels        []string         `json:"channels"`
	Budget          BriefBudget      `json:"budget"`
	Schedule        BriefSchedule    `json:"schedule"`
	Constraints     []string         `json:"constraints"`
	Measurement     BriefMeasurement `json:"measurement"`
}

type BriefCampaign struct {
	Objective string `json:"objective"`
}
type BriefAudience struct {
	Primary string `json:"primary"`
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

func EmptyBriefDocument() BriefDocument {
	return BriefDocument{
		ContractVersion: "strategy-brief-version/v1",
		Channels:        []string{},
		Constraints:     []string{},
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
	if d.ContractVersion != "strategy-draft/v1" || strings.TrimSpace(d.Objective) == "" ||
		strings.TrimSpace(d.Audience.Primary) == "" || strings.TrimSpace(d.Proposition) == "" ||
		len(d.ChannelStrategy) == 0 || d.Lineage.BriefID == "" || d.Lineage.BriefVersion < 1 ||
		d.Lineage.ProjectContextVersion < 1 {
		return fmt.Errorf("%w: strategy document is incomplete", ErrInvalidRequest)
	}
	for _, channel := range d.ChannelStrategy {
		if strings.TrimSpace(channel.Platform) == "" || len(channel.Formats) == 0 {
			return fmt.Errorf("%w: channel strategy is invalid", ErrInvalidRequest)
		}
	}
	return nil
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
