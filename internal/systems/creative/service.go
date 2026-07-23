// Package creative owns creative-plan business state. It only calls Provider's
// public application seam and never handles vendor URLs or asset storage.
package creative

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/ids"
	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/systems/strategy"
	"github.com/shikanon/cookies/internal/systems/strategy/prompts"
)

type MediaType string

const (
	MediaImage MediaType = "image"
	MediaVideo MediaType = "video"
)

var (
	ErrPlanNotFound        = errors.New("creative plan not found")
	ErrStrategyNotApproved = errors.New("creative plan requires an approved strategy")
)

type Plan struct {
	ID               string                  `json:"id"`
	OrganizationID   contract.OrganizationID `json:"organization_id"`
	ProjectID        contract.ProjectID      `json:"project_id"`
	StrategyOutputID string                  `json:"strategy_output_id"`
	MediaType        MediaType               `json:"media_type"`
	Variant          int                     `json:"variant"`
	Prompt           string                  `json:"prompt"`
	ModelAlias       string                  `json:"model_alias"`
	CreatedAt        time.Time               `json:"created_at"`
}

type Store interface {
	Create(context.Context, Plan) (Plan, error)
	Get(context.Context, contract.OrganizationID, contract.ProjectID, string) (Plan, error)
}

type StrategyReader interface {
	GetStrategy(context.Context, contract.OrganizationID, contract.ProjectID, string) (strategy.StrategyOutput, error)
	GetProposal(context.Context, contract.OrganizationID, contract.ProjectID, string) (strategy.Proposal, error)
}

type ProviderJobs interface {
	CreateImageJob(context.Context, provider.CreateImageJobRequest) (contract.ProviderJob, bool, error)
}

type CreatePlanRequest struct {
	StrategyOutputID string    `json:"strategy_output_id"`
	MediaType        MediaType `json:"media_type"`
	Variant          int       `json:"variant"`
	ModelAlias       string    `json:"model_alias"`
}

type Service struct {
	Store      Store
	Strategies StrategyReader
	Jobs       ProviderJobs
	NewID      ids.Generator
	Now        func() time.Time
}

func (s Service) CreatePlan(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, request CreatePlanRequest) (Plan, error) {
	if s.Store == nil || s.Strategies == nil {
		return Plan{}, fmt.Errorf("creative store and strategy reader are required")
	}
	if err := actor.Validate(); err != nil || project.ValidateBrandBound() != nil || actor.OrganizationID != project.OrganizationID {
		return Plan{}, fmt.Errorf("authorized brand-bound project is required")
	}
	if request.MediaType != MediaImage && request.MediaType != MediaVideo || request.Variant < 1 || strings.TrimSpace(request.ModelAlias) == "" {
		return Plan{}, fmt.Errorf("strategy output ID, valid media type, positive variant, and model alias are required")
	}
	strategyOutput, err := s.Strategies.GetStrategy(ctx, actor.OrganizationID, project.ProjectID, request.StrategyOutputID)
	if err != nil {
		return Plan{}, err
	}
	if !strategyOutput.Approved() {
		return Plan{}, ErrStrategyNotApproved
	}
	proposal, err := s.Strategies.GetProposal(ctx, actor.OrganizationID, project.ProjectID, strategyOutput.ProposalID)
	if err != nil {
		return Plan{}, err
	}
	prompt := prompts.BuildImagePrompt(proposal.Input, request.Variant)
	if request.MediaType == MediaVideo {
		prompt = prompts.BuildVideoPrompt(proposal.Input, request.Variant)
	}
	id, err := s.newID()("creativeplan")
	if err != nil {
		return Plan{}, err
	}
	return s.Store.Create(ctx, Plan{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: project.ProjectID, StrategyOutputID: request.StrategyOutputID,
		MediaType: request.MediaType, Variant: request.Variant, Prompt: prompt, ModelAlias: request.ModelAlias, CreatedAt: s.now(),
	})
}

func (s Service) CreateImageJob(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, planID string, width, height int, key contract.IdempotencyKey) (contract.ProviderJob, bool, error) {
	if s.Store == nil || s.Jobs == nil {
		return contract.ProviderJob{}, false, fmt.Errorf("creative store and provider jobs are required")
	}
	plan, err := s.Store.Get(ctx, actor.OrganizationID, project.ProjectID, planID)
	if err != nil {
		return contract.ProviderJob{}, false, err
	}
	if plan.MediaType != MediaImage {
		return contract.ProviderJob{}, false, fmt.Errorf("only image plans can create image jobs")
	}
	hash, err := contract.CanonicalJSONHash(struct {
		PlanID string `json:"plan_id"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}{planID, width, height})
	if err != nil {
		return contract.ProviderJob{}, false, err
	}
	return s.Jobs.CreateImageJob(ctx, provider.CreateImageJobRequest{
		Actor: actor, Project: project, IdempotencyKey: key, RequestHash: hash, ModelAlias: plan.ModelAlias,
		SourceSystem: "creative", SourceTaskID: plan.ID, Input: provider.ImageGenerationInput{Prompt: plan.Prompt, Width: width, Height: height},
	})
}

func (s Service) GetPlan(ctx context.Context, org contract.OrganizationID, projectID contract.ProjectID, planID string) (Plan, error) {
	if s.Store == nil {
		return Plan{}, fmt.Errorf("creative store is required")
	}
	return s.Store.Get(ctx, org, projectID, planID)
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
