package strategy

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/systems/strategy/prompts"
)

func TestCreateProposalPersistsPolarisBriefAndAuditMetadata(t *testing.T) {
	t.Parallel()
	store := newMemoryStore()
	service := Service{Store: store, NewID: func(prefix string) (string, error) { return prefix + "_1", nil }}
	proposal, duplicate, err := service.CreateProposal(context.Background(), testActor(), testProject(), polarisInput())
	if err != nil || duplicate {
		t.Fatalf("CreateProposal() proposal=%#v duplicate=%v err=%v", proposal, duplicate, err)
	}
	if proposal.TemplateVersion != prompts.TemplateVersion || proposal.Input.Brand != "极地鲜生" || proposal.Status != ProposalDraft {
		t.Fatalf("unexpected proposal: %#v", proposal)
	}
	_, duplicate, err = service.CreateProposal(context.Background(), testActor(), testProject(), polarisInput())
	if err != nil || !duplicate {
		t.Fatalf("duplicate proposal was not idempotent: duplicate=%v err=%v", duplicate, err)
	}
}

func TestGenerateAndApproveStrategy(t *testing.T) {
	t.Parallel()
	store := newMemoryStore()
	service := Service{
		Store: store, Text: textStub{}, NewID: func(prefix string) (string, error) { return prefix + "_1", nil },
	}
	proposal, _, err := service.CreateProposal(context.Background(), testActor(), testProject(), polarisInput())
	if err != nil {
		t.Fatal(err)
	}
	output, err := service.GenerateStrategy(context.Background(), testActor(), testProject(), proposal.ID)
	if err != nil || !json.Valid(output.Content) {
		t.Fatalf("GenerateStrategy() output=%#v err=%v", output, err)
	}
	approved, err := service.ApproveStrategy(context.Background(), testActor(), testProject(), output.ID)
	if err != nil || !approved.Approved() {
		t.Fatalf("ApproveStrategy() output=%#v err=%v", approved, err)
	}
}

type textStub struct{}

func (textStub) GenerateText(context.Context, provider.TextGenerateRequest) (provider.SynchronousResponse, error) {
	return provider.SynchronousResponse{ProviderCode: "fake", ModelAlias: DefaultTextModelAlias, ModelVersion: "v1", StructuredOutput: json.RawMessage(`{"insight":"insight","proposition":"value","strategy":"strategy","channels":[],"creative_directions":[],"compliance_checklist":[]}`)}, nil
}

type memoryStore struct {
	proposals  map[string]Proposal
	strategies map[string]StrategyOutput
}

func newMemoryStore() *memoryStore {
	return &memoryStore{proposals: map[string]Proposal{}, strategies: map[string]StrategyOutput{}}
}
func (s *memoryStore) CreateProposal(_ context.Context, proposal Proposal) (Proposal, bool, error) {
	for _, item := range s.proposals {
		if item.InputHash == proposal.InputHash && item.ProjectID == proposal.ProjectID && item.OrganizationID == proposal.OrganizationID {
			return item, true, nil
		}
	}
	s.proposals[proposal.ID] = proposal
	return proposal, false, nil
}
func (s *memoryStore) GetProposal(_ context.Context, org contract.OrganizationID, projectID contract.ProjectID, id string) (Proposal, error) {
	item, ok := s.proposals[id]
	if !ok || item.OrganizationID != org || item.ProjectID != projectID {
		return Proposal{}, ErrProposalNotFound
	}
	return item, nil
}
func (s *memoryStore) CreateStrategy(_ context.Context, output StrategyOutput) (StrategyOutput, error) {
	s.strategies[output.ID] = output
	proposal := s.proposals[output.ProposalID]
	proposal.Status = ProposalGenerated
	s.proposals[proposal.ID] = proposal
	return output, nil
}
func (s *memoryStore) GetStrategy(_ context.Context, org contract.OrganizationID, projectID contract.ProjectID, id string) (StrategyOutput, error) {
	item, ok := s.strategies[id]
	if !ok || item.OrganizationID != org || item.ProjectID != projectID {
		return StrategyOutput{}, ErrStrategyNotFound
	}
	return item, nil
}
func (s *memoryStore) GetStrategyByProposal(_ context.Context, org contract.OrganizationID, projectID contract.ProjectID, proposalID string) (StrategyOutput, error) {
	for _, item := range s.strategies {
		if item.OrganizationID == org && item.ProjectID == projectID && item.ProposalID == proposalID {
			return item, nil
		}
	}
	return StrategyOutput{}, ErrStrategyNotFound
}
func (s *memoryStore) ApproveStrategy(_ context.Context, org contract.OrganizationID, projectID contract.ProjectID, id string) (StrategyOutput, error) {
	item, err := s.GetStrategy(context.Background(), org, projectID, id)
	if err != nil {
		return StrategyOutput{}, err
	}
	now := time.Now().UTC()
	item.ApprovedAt = &now
	s.strategies[id] = item
	return item, nil
}

func testActor() contract.ActorContext {
	return contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{provider.ScopeTextGenerate, provider.ScopeJobCreate}}
}
func testProject() contract.ProjectContext {
	brand := contract.BrandID("brand_1")
	return contract.ProjectContext{OrganizationID: "org_1", ProjectID: "project_1", BrandID: &brand, ProductIDs: []contract.ProductID{}, ProjectContextVersion: 1}
}
func polarisInput() prompts.ProposalInput {
	return prompts.ProposalInput{Brand: "极地鲜生", Product: "深海鳕鱼柳", Audience: "家庭消费者", Platform: "抖音", Budget: "20万", Timeline: "618", Compliance: []string{"禁用绝对化用语"}, Directions: []string{"冷链鲜度"}}
}
