package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/ids"
	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/systems/strategy/prompts"
)

const DefaultTextModelAlias = "cookies.text.strategy"

type Service struct {
	Store Store
	Text  interface {
		GenerateText(context.Context, provider.TextGenerateRequest) (provider.SynchronousResponse, error)
	}
	NewID ids.Generator
	Now   func() time.Time
}

func (s Service) CreateProposal(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, input prompts.ProposalInput) (Proposal, bool, error) {
	if s.Store == nil {
		return Proposal{}, false, fmt.Errorf("strategy store is required")
	}
	if err := actor.Validate(); err != nil || project.ValidateBrandBound() != nil || actor.OrganizationID != project.OrganizationID {
		return Proposal{}, false, fmt.Errorf("authorized brand-bound project is required")
	}
	if err := input.Validate(); err != nil {
		return Proposal{}, false, err
	}
	hash, err := contract.CanonicalJSONHash(input)
	if err != nil {
		return Proposal{}, false, err
	}
	id, err := s.newID()("proposal")
	if err != nil {
		return Proposal{}, false, err
	}
	now := s.now()
	return s.Store.CreateProposal(ctx, Proposal{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: project.ProjectID, Input: input, InputHash: hash,
		TemplateVersion: prompts.TemplateVersion, Status: ProposalDraft, CreatedAt: now, UpdatedAt: now,
	})
}

func (s Service) GenerateStrategy(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, proposalID string) (StrategyOutput, error) {
	if s.Store == nil || s.Text == nil {
		return StrategyOutput{}, fmt.Errorf("strategy store and text provider are required")
	}
	proposal, err := s.Store.GetProposal(ctx, actor.OrganizationID, project.ProjectID, proposalID)
	if err != nil {
		return StrategyOutput{}, err
	}
	result, err := s.Text.GenerateText(ctx, provider.TextGenerateRequest{
		Actor: actor, Project: project, ModelAlias: DefaultTextModelAlias,
		Messages: prompts.BuildProposalStrategyMessages(proposal.Input), OutputJSONSchema: prompts.StrategyJSONSchema(),
	})
	if err != nil {
		return StrategyOutput{}, err
	}
	content := result.StructuredOutput
	if !json.Valid(content) {
		return StrategyOutput{}, fmt.Errorf("strategy provider returned invalid structured output")
	}
	id, err := s.newID()("strategy")
	if err != nil {
		return StrategyOutput{}, err
	}
	return s.Store.CreateStrategy(ctx, StrategyOutput{
		ID: id, ProposalID: proposal.ID, OrganizationID: actor.OrganizationID, ProjectID: project.ProjectID,
		Content: content, ModelAlias: result.ModelAlias, ModelVersion: result.ModelVersion, ProviderCode: result.ProviderCode, CreatedAt: s.now(),
	})
}

func (s Service) ApproveStrategy(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, strategyID string) (StrategyOutput, error) {
	if err := actor.Validate(); err != nil || project.ValidateBrandBound() != nil || actor.OrganizationID != project.OrganizationID {
		return StrategyOutput{}, fmt.Errorf("authorized brand-bound project is required")
	}
	return s.Store.ApproveStrategy(ctx, actor.OrganizationID, project.ProjectID, strategyID)
}

func (s Service) GetStrategy(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, strategyID string) (StrategyOutput, error) {
	if s.Store == nil {
		return StrategyOutput{}, fmt.Errorf("strategy store is required")
	}
	return s.Store.GetStrategy(ctx, organizationID, projectID, strategyID)
}

func (s Service) GetProposal(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, proposalID string) (Proposal, error) {
	if s.Store == nil {
		return Proposal{}, fmt.Errorf("strategy store is required")
	}
	return s.Store.GetProposal(ctx, organizationID, projectID, proposalID)
}

func (s Service) newID() ids.Generator {
	if s.NewID != nil {
		return s.NewID
	}
	return ids.New
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
