package strategy

import (
	"encoding/json"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/strategy/prompts"
)

type ProposalStatus string

const (
	ProposalDraft     ProposalStatus = "draft"
	ProposalGenerated ProposalStatus = "generated"
	ProposalApproved  ProposalStatus = "approved"
)

type Proposal struct {
	ID              string                  `json:"id"`
	OrganizationID  contract.OrganizationID `json:"organization_id"`
	ProjectID       contract.ProjectID      `json:"project_id"`
	SourceType      string                  `json:"source_type,omitempty"`
	SourceObjectURI string                  `json:"source_object_uri,omitempty"`
	Input           prompts.ProposalInput   `json:"input"`
	InputHash       string                  `json:"-"`
	TemplateVersion string                  `json:"template_version"`
	Status          ProposalStatus          `json:"status"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

type StrategyOutput struct {
	ID             string                  `json:"id"`
	ProposalID     string                  `json:"proposal_id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	Content        json.RawMessage         `json:"content"`
	ModelAlias     string                  `json:"model_alias"`
	ModelVersion   string                  `json:"model_version"`
	ProviderCode   string                  `json:"provider_code"`
	ApprovedAt     *time.Time              `json:"approved_at"`
	CreatedAt      time.Time               `json:"created_at"`
}

func (s StrategyOutput) Approved() bool { return s.ApprovedAt != nil }
