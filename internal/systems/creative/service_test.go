package creative

import (
	"context"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/systems/strategy"
	"github.com/shikanon/cookies/internal/systems/strategy/prompts"
)

func TestCreativePlanRequiresApprovedStrategy(t *testing.T) {
	t.Parallel()
	service := Service{
		Store:      &planStore{items: map[string]Plan{}},
		Strategies: strategyReader{output: strategy.StrategyOutput{ID: "strategy_1", ProposalID: "proposal_1", OrganizationID: "org_1", ProjectID: "project_1"}},
		NewID:      func(string) (string, error) { return "plan_1", nil },
	}
	_, err := service.CreatePlan(context.Background(), actor(), project(), CreatePlanRequest{StrategyOutputID: "strategy_1", MediaType: MediaImage, Variant: 1, ModelAlias: "cookies.image.standard"})
	if err != ErrStrategyNotApproved {
		t.Fatalf("CreatePlan() error=%v, want %v", err, ErrStrategyNotApproved)
	}
}

func TestCreateImageJobUsesCreativePlanSource(t *testing.T) {
	t.Parallel()
	plans := &planStore{items: map[string]Plan{"plan_1": {ID: "plan_1", OrganizationID: "org_1", ProjectID: "project_1", MediaType: MediaImage, Prompt: "safe prompt", ModelAlias: "cookies.image.standard"}}}
	jobs := &jobStub{}
	service := Service{Store: plans, Jobs: jobs}
	_, _, err := service.CreateImageJob(context.Background(), actor(), project(), "plan_1", 1024, 1024, "creative-image-1")
	if err != nil {
		t.Fatal(err)
	}
	if jobs.request.SourceSystem != "creative" || jobs.request.SourceTaskID != "plan_1" || jobs.request.Input.Prompt != "safe prompt" {
		t.Fatalf("unexpected provider request: %#v", jobs.request)
	}
}

type planStore struct{ items map[string]Plan }

func (s *planStore) Create(_ context.Context, plan Plan) (Plan, error) {
	s.items[plan.ID] = plan
	return plan, nil
}
func (s *planStore) Get(_ context.Context, org contract.OrganizationID, projectID contract.ProjectID, id string) (Plan, error) {
	plan, ok := s.items[id]
	if !ok || plan.OrganizationID != org || plan.ProjectID != projectID {
		return Plan{}, ErrPlanNotFound
	}
	return plan, nil
}

type strategyReader struct{ output strategy.StrategyOutput }

func (s strategyReader) GetStrategy(context.Context, contract.OrganizationID, contract.ProjectID, string) (strategy.StrategyOutput, error) {
	return s.output, nil
}
func (s strategyReader) GetProposal(context.Context, contract.OrganizationID, contract.ProjectID, string) (strategy.Proposal, error) {
	return strategy.Proposal{ID: "proposal_1", Input: prompts.ProposalInput{Brand: "极地鲜生", Product: "深海鳕鱼柳", Compliance: []string{}, Directions: []string{"冷链鲜度"}}}, nil
}

type jobStub struct {
	request provider.CreateImageJobRequest
}

func (s *jobStub) CreateImageJob(_ context.Context, request provider.CreateImageJobRequest) (contract.ProviderJob, bool, error) {
	s.request = request
	return contract.ProviderJob{}, false, nil
}
func actor() contract.ActorContext {
	return contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{provider.ScopeJobCreate}}
}
func project() contract.ProjectContext {
	brand := contract.BrandID("brand_1")
	return contract.ProjectContext{OrganizationID: "org_1", ProjectID: "project_1", BrandID: &brand, ProductIDs: []contract.ProductID{}, ProjectContextVersion: 1}
}
