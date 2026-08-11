package strategy

import (
	"context"
	"errors"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/knowledge"
	"github.com/shikanon/cookies/internal/platform/provider"
)

type modelCapabilityProjectReader struct{}

func (modelCapabilityProjectReader) GetContext(
	_ context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
) (contract.ProjectContext, error) {
	brandID := contract.BrandID("brand_1")
	return contract.ProjectContext{
		OrganizationID: actor.OrganizationID, ProjectID: projectID,
		BrandID: &brandID, ProductIDs: []contract.ProductID{}, ProjectContextVersion: 1,
	}, nil
}

type modelCapabilityDocumentVisionInspector struct {
	err error
}

func (i modelCapabilityDocumentVisionInspector) Inspect(
	_ context.Context,
	_ contract.OrganizationID,
	alias string,
) (knowledge.DocumentVisionCapability, error) {
	if i.err != nil {
		return knowledge.DocumentVisionCapability{ModelAlias: alias, ReasonCode: "DOCUMENT_VISION_ROUTE_UNAVAILABLE"}, i.err
	}
	return knowledge.DocumentVisionCapability{
		Available: true, ModelAlias: alias, UpstreamModel: "las-pdf-parser-v1", RouteRevisionID: "route_las_1",
	}, nil
}

type modelCapabilityResearchInspector struct {
	err error
}

func (i modelCapabilityResearchInspector) InspectResearchRoute(
	_ context.Context,
	_ contract.OrganizationID,
	alias string,
) (provider.CapabilityRouteInspection, error) {
	if i.err != nil {
		return provider.CapabilityRouteInspection{}, i.err
	}
	return provider.CapabilityRouteInspection{
		ModelAlias: alias, UpstreamModel: "seed-research-v1", RouteRevisionID: "route_research_1", Ready: true,
	}, nil
}

func TestModelCapabilitiesUseFixedAliasesAndInspectEveryRoute(t *testing.T) {
	t.Parallel()
	service := Service{
		Projects:                 modelCapabilityProjectReader{},
		Text:                     &provider.Service{TextAdapter: provider.FakeSyncAdapter{}},
		ResearchRoutes:           modelCapabilityResearchInspector{},
		DocumentVisionRoutes:     modelCapabilityDocumentVisionInspector{},
		TextModelAlias:           "cookies.text.standard",
		LiteTextModelAlias:       "cookies.text.lite",
		DeepReviewModelAlias:     "cookies.text.deep_review",
		ResearchModelAlias:       "cookies.research.web.standard",
		DocumentVisionModelAlias: "cookies.document.vision.standard",
	}
	actor := contract.ActorContext{
		OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
		Scopes: []contract.Scope{ScopeRead},
	}
	manifest, err := service.GetModelCapabilities(context.Background(), actor, "project_1")
	if err != nil {
		t.Fatalf("get model capabilities: %v", err)
	}
	if manifest.ContractVersion != ModelCapabilitiesContractV1 || len(manifest.Items) != 6 {
		t.Fatalf("manifest=%#v", manifest)
	}
	wantAliases := map[string]string{
		ModelCapabilityAssistantSimple: "cookies.text.lite",
		ModelCapabilityAssistantText:   "cookies.text.standard",
		ModelCapabilityStrategyDraft:   "cookies.text.standard",
		ModelCapabilityDeepReview:      "cookies.text.deep_review",
		ModelCapabilityResearchWeb:     "cookies.research.web.standard",
		ModelCapabilityDocumentVision:  "cookies.document.vision.standard",
	}
	for _, item := range manifest.Items {
		if !item.Available || item.ModelAlias != wantAliases[item.Capability] || item.RouteRevisionID == "" {
			t.Fatalf("capability did not use its fixed ready route: %#v", item)
		}
	}
}

func TestModelCapabilitiesUseDedicatedDocumentVisionRoute(t *testing.T) {
	t.Parallel()
	service := Service{
		Projects:                 modelCapabilityProjectReader{},
		Text:                     &provider.Service{TextAdapter: provider.FakeSyncAdapter{}},
		ResearchRoutes:           modelCapabilityResearchInspector{},
		DocumentVisionRoutes:     modelCapabilityDocumentVisionInspector{},
		TextModelAlias:           "cookies.text.standard",
		LiteTextModelAlias:       "cookies.text.lite",
		DeepReviewModelAlias:     "cookies.text.deep_review",
		ResearchModelAlias:       "cookies.research.web.standard",
		DocumentVisionModelAlias: "cookies.document.vision.standard",
	}
	actor := contract.ActorContext{OrganizationID: "org_1", Scopes: []contract.Scope{ScopeRead}}
	manifest, err := service.GetModelCapabilities(context.Background(), actor, "project_1")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range manifest.Items {
		if item.Capability == ModelCapabilityDocumentVision {
			if !item.Available || item.UpstreamModel != "las-pdf-parser-v1" || item.RouteRevisionID != "route_las_1" {
				t.Fatalf("document vision capability=%#v", item)
			}
			return
		}
	}
	t.Fatal("document vision capability is missing")
}

func TestModelCapabilitiesFailClosedWithoutFallback(t *testing.T) {
	t.Parallel()
	service := Service{
		Projects:       modelCapabilityProjectReader{},
		ResearchRoutes: modelCapabilityResearchInspector{err: errors.New("route missing")},
		TextModelAlias: "cookies.text.standard", LiteTextModelAlias: "cookies.text.lite",
		DeepReviewModelAlias: "cookies.text.deep_review", ResearchModelAlias: "cookies.research.web.standard",
		DocumentVisionModelAlias: "cookies.document.vision.standard",
	}
	actor := contract.ActorContext{OrganizationID: "org_1", Scopes: []contract.Scope{ScopeRead}}
	manifest, err := service.GetModelCapabilities(context.Background(), actor, "project_1")
	if err != nil {
		t.Fatalf("get model capabilities: %v", err)
	}
	for _, item := range manifest.Items {
		if item.Available {
			t.Fatalf("unconfigured capability was exposed: %#v", item)
		}
		if item.ModelAlias == "" || item.ReasonCode == "" {
			t.Fatalf("failure was not explicit: %#v", item)
		}
	}
}
