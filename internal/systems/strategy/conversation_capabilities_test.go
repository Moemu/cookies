package strategy

import (
	"context"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/knowledge"
	"github.com/shikanon/cookies/internal/platform/provider"
)

type conversationCapabilityAdapter struct{}

type availableConversationResearch struct{}

func (availableConversationResearch) RunConversationWebSearch(
	context.Context, contract.ActorContext, contract.ProjectID, string, string,
) (knowledge.ResearchRun, error) {
	return knowledge.ResearchRun{}, nil
}

func (conversationCapabilityAdapter) InspectTextRoute(
	_ context.Context,
	_ contract.OrganizationID,
	modelAlias string,
) (provider.TextRouteInspection, error) {
	return provider.TextRouteInspection{ModelAlias: modelAlias, Ready: modelAlias == "cookies.text.deep"}, nil
}

func (conversationCapabilityAdapter) GenerateText(
	context.Context,
	provider.TextAdapterRequest,
) (provider.SynchronousResult, error) {
	return provider.SynchronousResult{}, nil
}

func TestConversationCapabilitiesExposeOnlyEffectiveServerFeatures(t *testing.T) {
	t.Parallel()
	service := Service{
		Text:                         &provider.Service{TextAdapter: conversationCapabilityAdapter{}},
		TextModelAlias:               "cookies.text.standard",
		DeepReviewModelAlias:         "cookies.text.deep",
		ConversationWebSearchEnabled: true,
		ConversationResearch:         availableConversationResearch{},
		V2Enabled:                    true,
		QuickViralRemakeEnabled:      true,
	}
	actor := contract.ActorContext{OrganizationID: "org_1", Scopes: []contract.Scope{ScopeRead}}
	capabilities, err := service.GetConversationCapabilities(context.Background(), actor)
	if err != nil {
		t.Fatalf("get capabilities: %v", err)
	}
	if capabilities.ContractVersion != ConversationCapabilitiesContractV1 ||
		!capabilities.MultimodalInput.Available || !capabilities.DeepReasoning.Available ||
		!capabilities.WebSearch.Available || !capabilities.QuickViralRemake.Available ||
		capabilities.WebSearch.Disclosure != "query_only_grounded_answer" || capabilities.WebSearch.EstimatedWaitSeconds != 180 {
		t.Fatalf("capabilities=%#v", capabilities)
	}
	if err := service.ensureConversationPolicyReady(context.Background(), actor.OrganizationID, &MessageRequestedPolicy{ReasoningMode: "deep", WebSearch: "allowed"}); err != nil {
		t.Fatalf("deep/search policy should be ready: %v", err)
	}
	if alias := service.conversationModelAlias(&MessageRequestedPolicy{ReasoningMode: "deep"}); alias != "cookies.text.deep" {
		t.Fatalf("deep conversation resolved alias %q", alias)
	}
}

func TestConversationCapabilitiesFailClosed(t *testing.T) {
	t.Parallel()
	service := Service{TextModelAlias: "cookies.text.standard", DeepReviewModelAlias: "cookies.text.standard"}
	actor := contract.ActorContext{OrganizationID: "org_1", Scopes: []contract.Scope{ScopeRead}}
	capabilities, err := service.GetConversationCapabilities(context.Background(), actor)
	if err != nil {
		t.Fatalf("get capabilities: %v", err)
	}
	if capabilities.DeepReasoning.Available || capabilities.WebSearch.Available {
		t.Fatalf("unsupported capabilities leaked: %#v", capabilities)
	}
	if err := service.ensureConversationPolicyReady(context.Background(), actor.OrganizationID, &MessageRequestedPolicy{ReasoningMode: "deep"}); err == nil {
		t.Fatal("deep policy should fail when it is not backed by a distinct route")
	}
	if err := service.ensureConversationPolicyReady(context.Background(), actor.OrganizationID, &MessageRequestedPolicy{WebSearch: "allowed"}); err == nil {
		t.Fatal("search policy should fail when research is disabled")
	}
}

func TestMessageV2RolloutFlagStopsNewWrites(t *testing.T) {
	t.Parallel()
	service := Service{V2Enabled: false}
	_, _, err := service.SendMessageV2(
		context.Background(),
		contract.ActorContext{},
		"request-1",
		"conversation_1",
		SendMessageV2Request{
			ContractVersion: MessageCreateContractV2,
			Content:         []MessageContentBlock{{Type: "text", Text: "hello"}},
		},
	)
	if err != ErrFeatureDisabled {
		t.Fatalf("flag-off Message v2 write err=%v", err)
	}
}
