package strategy

import (
	"context"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

const ModelCapabilitiesContractV1 = "strategy-model-capabilities/v1"

const (
	ModelCapabilityAssistantSimple = "assistant.simple"
	ModelCapabilityAssistantText   = "assistant.standard"
	ModelCapabilityStrategyDraft   = "strategy.generate"
	ModelCapabilityDeepReview      = "strategy.deep_review"
	ModelCapabilityResearchWeb     = "research.web"
	ModelCapabilityDocumentVision  = "document.vision"
)

type ModelCapabilityStatus struct {
	Capability      string `json:"capability"`
	ModelAlias      string `json:"model_alias"`
	Available       bool   `json:"available"`
	ReasonCode      string `json:"reason_code,omitempty"`
	UpstreamModel   string `json:"upstream_model,omitempty"`
	RouteRevisionID string `json:"route_revision_id,omitempty"`
}

type ModelCapabilities struct {
	ContractVersion string                  `json:"contract_version"`
	Items           []ModelCapabilityStatus `json:"items"`
}

func (s Service) GetModelCapabilities(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
) (ModelCapabilities, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return ModelCapabilities{}, err
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return ModelCapabilities{}, err
	}

	standard := s.inspectTextCapability(ctx, actor.OrganizationID, strings.TrimSpace(s.TextModelAlias))
	lite := s.inspectTextCapability(ctx, actor.OrganizationID, strings.TrimSpace(s.LiteTextModelAlias))
	deep := s.inspectTextCapability(ctx, actor.OrganizationID, strings.TrimSpace(s.DeepReviewModelAlias))
	research := s.inspectResearchCapability(ctx, actor.OrganizationID, strings.TrimSpace(s.ResearchModelAlias))
	vision := s.inspectVisionCapability(ctx, actor.OrganizationID, strings.TrimSpace(s.DocumentVisionModelAlias))

	standard.Capability = ModelCapabilityAssistantText
	strategyDraft := standard
	strategyDraft.Capability = ModelCapabilityStrategyDraft
	lite.Capability = ModelCapabilityAssistantSimple
	deep.Capability = ModelCapabilityDeepReview
	research.Capability = ModelCapabilityResearchWeb
	vision.Capability = ModelCapabilityDocumentVision

	return ModelCapabilities{
		ContractVersion: ModelCapabilitiesContractV1,
		Items: []ModelCapabilityStatus{
			lite, standard, strategyDraft, deep, research, vision,
		},
	}, nil
}

func (s Service) inspectTextCapability(ctx context.Context, organizationID contract.OrganizationID, alias string) ModelCapabilityStatus {
	result := ModelCapabilityStatus{ModelAlias: alias}
	if alias == "" {
		result.ReasonCode = "MODEL_ALIAS_NOT_CONFIGURED"
		return result
	}
	if s.Text == nil || s.Text.TextAdapter == nil {
		result.ReasonCode = "PROVIDER_DISABLED"
		return result
	}
	inspection, err := s.Text.InspectTextRoute(ctx, organizationID, alias)
	if err != nil {
		result.ReasonCode = generationReadinessReason(err)
		return result
	}
	result.Available = inspection.Ready
	result.UpstreamModel = inspection.UpstreamModel
	result.RouteRevisionID = inspection.RouteRevisionID
	if !result.Available {
		result.ReasonCode = "MODEL_ROUTE_NOT_READY"
	}
	return result
}

func (s Service) inspectResearchCapability(ctx context.Context, organizationID contract.OrganizationID, alias string) ModelCapabilityStatus {
	result := ModelCapabilityStatus{ModelAlias: alias}
	if alias == "" {
		result.ReasonCode = "MODEL_ALIAS_NOT_CONFIGURED"
		return result
	}
	if s.ResearchRoutes == nil {
		result.ReasonCode = "RESEARCH_PROVIDER_DISABLED"
		return result
	}
	inspection, err := s.ResearchRoutes.InspectResearchRoute(ctx, organizationID, alias)
	return applyCapabilityInspection(result, inspection, err)
}

func (s Service) inspectVisionCapability(ctx context.Context, organizationID contract.OrganizationID, alias string) ModelCapabilityStatus {
	result := ModelCapabilityStatus{ModelAlias: alias}
	if alias == "" {
		result.ReasonCode = "MODEL_ALIAS_NOT_CONFIGURED"
		return result
	}
	if s.Text == nil || s.Text.VisionAdapter == nil {
		result.ReasonCode = "VISION_PROVIDER_DISABLED"
		return result
	}
	inspection, err := s.Text.InspectVisionRoute(ctx, organizationID, alias)
	return applyCapabilityInspection(result, inspection, err)
}

func applyCapabilityInspection(result ModelCapabilityStatus, inspection provider.CapabilityRouteInspection, err error) ModelCapabilityStatus {
	if err != nil {
		result.ReasonCode = generationReadinessReason(err)
		return result
	}
	result.Available = inspection.Ready
	result.UpstreamModel = inspection.UpstreamModel
	result.RouteRevisionID = inspection.RouteRevisionID
	if !result.Available {
		result.ReasonCode = "MODEL_ROUTE_NOT_READY"
	}
	return result
}
