package strategy

import (
	"context"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const ConversationCapabilitiesContractV1 = "strategy-conversation-capabilities/v1"

type ConversationCapability struct {
	Available            bool   `json:"available"`
	EstimatedWaitSeconds int    `json:"estimated_wait_seconds,omitempty"`
	Disclosure           string `json:"disclosure,omitempty"`
}

type ConversationCapabilities struct {
	ContractVersion  string                 `json:"contract_version"`
	MultimodalInput  ConversationCapability `json:"multimodal_input"`
	DeepReasoning    ConversationCapability `json:"deep_reasoning"`
	WebSearch        ConversationCapability `json:"web_search"`
	QuickViralRemake ConversationCapability `json:"quick_viral_remake"`
}

func (s Service) GetConversationCapabilities(ctx context.Context, actor contract.ActorContext) (ConversationCapabilities, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return ConversationCapabilities{}, err
	}
	capabilities := ConversationCapabilities{
		ContractVersion: ConversationCapabilitiesContractV1,
		MultimodalInput: ConversationCapability{Available: s.V2Enabled},
		WebSearch: ConversationCapability{
			Available:            s.V2Enabled && s.ConversationWebSearchEnabled,
			EstimatedWaitSeconds: 180,
			Disclosure:           "query_only_background",
		},
		QuickViralRemake: ConversationCapability{Available: s.V2Enabled && s.QuickViralRemakeEnabled},
	}
	if !s.V2Enabled {
		return capabilities, nil
	}
	deepAlias := strings.TrimSpace(s.DeepReviewModelAlias)
	standardAlias := s.conversationModelAlias(nil)
	if s.Text == nil || deepAlias == "" || deepAlias == standardAlias {
		return capabilities, nil
	}
	inspection, err := s.Text.InspectTextRoute(ctx, actor.OrganizationID, deepAlias)
	if err == nil && inspection.Ready {
		capabilities.DeepReasoning = ConversationCapability{
			Available:            true,
			EstimatedWaitSeconds: 30,
		}
	}
	return capabilities, nil
}

func (s Service) conversationModelAlias(policy *MessageRequestedPolicy) string {
	if policy != nil && policy.ReasoningMode == "deep" {
		return strings.TrimSpace(s.DeepReviewModelAlias)
	}
	alias := strings.TrimSpace(s.TextModelAlias)
	if alias == "" {
		return "cookies.text.standard"
	}
	return alias
}

func (s Service) ensureConversationPolicyReady(
	ctx context.Context,
	organizationID contract.OrganizationID,
	policy *MessageRequestedPolicy,
) error {
	if policy != nil && policy.WebSearch == "allowed" && !s.ConversationWebSearchEnabled {
		return ErrFeatureDisabled
	}
	if policy != nil && policy.ReasoningMode == "deep" {
		alias := s.conversationModelAlias(policy)
		if s.Text == nil || alias == "" || alias == s.conversationModelAlias(nil) {
			return ErrGenerationUnavailable
		}
	}
	if s.Text == nil {
		return nil
	}
	inspection, err := s.Text.InspectTextRoute(ctx, organizationID, s.conversationModelAlias(policy))
	if err != nil || !inspection.Ready {
		return ErrGenerationUnavailable
	}
	return nil
}
