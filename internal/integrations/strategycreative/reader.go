// Package strategycreative contains the explicit composition adapter between
// two bounded contexts. It is intentionally not part of either domain package.
package strategycreative

import (
	"context"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/creative"
	"github.com/shikanon/cookies/internal/systems/strategy"
)

// Reader converts only the approved Strategy package resource into Creative's
// input vocabulary. Strategy remains the authorization owner of the package;
// Creative retains ownership of the resulting Intake and all later work.
type Reader struct {
	Service strategy.Service
}

func (r Reader) ListCreativeSources(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
) ([]creative.CreativeSourceOption, error) {
	packages, err := r.Service.ListPackages(ctx, actor, projectID)
	if err != nil {
		return nil, err
	}
	briefs, err := r.Service.ListProjectBriefVersions(ctx, actor, projectID)
	if err != nil {
		return nil, err
	}
	options := make([]creative.CreativeSourceOption, 0, len(packages)+len(briefs))
	for _, value := range packages {
		options = append(options, creative.CreativeSourceOption{
			SourceRef: creative.CreativeSourceReference{
				Kind: creative.CreativeSourceStrategy, ID: value.PackageID,
				Version: value.Version, ContentHash: string(value.ContentHash),
			},
			Status: "approved", Product: productFactsFromBrief(value.Snapshot.Brief, value.Snapshot.Strategy),
			ConfirmedAt: value.PublishedAt,
		})
	}
	for _, value := range briefs {
		options = append(options, creative.CreativeSourceOption{
			SourceRef: creative.CreativeSourceReference{
				Kind: creative.CreativeSourceConfirmedBrief, ID: value.BriefID,
				Version: value.Version, ContentHash: string(value.ContentHash),
			},
			Status: "confirmed", Product: productFactsFromBrief(value, strategy.StrategyDocument{}),
			ConfirmedAt: value.ConfirmedAt,
		})
	}
	if len(briefs) > 0 {
		// Briefs are returned newest-first. Creative defaults to the latest
		// human-confirmed Brief while keeping approved Strategy packages as
		// explicitly selectable historical sources.
		options[len(packages)].Preferred = true
	} else if len(options) > 0 {
		options[0].Preferred = true
	}
	return options, nil
}

func (r Reader) ReadCreativeSource(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	reference creative.CreativeSourceReference,
) (creative.CreativeSourceSnapshot, error) {
	if err := reference.Validate(); err != nil {
		return creative.CreativeSourceSnapshot{}, err
	}
	switch reference.Kind {
	case creative.CreativeSourceStrategy:
		value, err := r.Service.GetPackage(ctx, actor, projectID, reference.ID, reference.Version)
		if err != nil {
			return creative.CreativeSourceSnapshot{}, err
		}
		if !strings.EqualFold(string(value.ContentHash), strings.TrimSpace(reference.ContentHash)) {
			return creative.CreativeSourceSnapshot{}, fmt.Errorf("strategy package content hash no longer matches the selected version")
		}
		return creative.CreativeSourceSnapshot{
			SourceRef: reference,
			Product:   productFactsFromBrief(value.Snapshot.Brief, value.Snapshot.Strategy),
		}, nil
	case creative.CreativeSourceConfirmedBrief:
		value, err := r.Service.GetBriefVersion(ctx, actor, reference.ID, reference.Version)
		if err != nil {
			return creative.CreativeSourceSnapshot{}, err
		}
		if value.ProjectID != projectID ||
			!strings.EqualFold(string(value.ContentHash), strings.TrimSpace(reference.ContentHash)) {
			return creative.CreativeSourceSnapshot{}, fmt.Errorf("Brief content hash no longer matches the selected version")
		}
		return creative.CreativeSourceSnapshot{
			SourceRef: reference,
			Product:   productFactsFromBrief(value, strategy.StrategyDocument{}),
		}, nil
	default:
		return creative.CreativeSourceSnapshot{}, fmt.Errorf("unsupported creative source kind %q", reference.Kind)
	}
}

func productFactsFromBrief(brief strategy.BriefVersion, document strategy.StrategyDocument) creative.CommerceProductFacts {
	snapshot := brief.Snapshot
	sellingPoints := append([]string{}, snapshot.Product.SellingPoints...)
	if len(sellingPoints) == 0 {
		sellingPoints = append(sellingPoints, snapshot.Product.Evidence...)
	}
	if len(sellingPoints) == 0 && strings.TrimSpace(document.Proposition) != "" {
		sellingPoints = append(sellingPoints, document.Proposition)
	}
	tone := append([]string{}, snapshot.Creative.Tone...)
	visualKeywords := append([]string{}, tone...)
	mandatory := append([]string{}, snapshot.Creative.MandatoryElements...)
	mandatory = append(mandatory, snapshot.Constraints...)
	prohibited := append([]string{}, snapshot.Creative.ProhibitedClaims...)
	return creative.CommerceProductFacts{
		BrandName: snapshot.Brand.Name, ProductName: snapshot.Product.Name,
		ProductCategory: snapshot.Product.Category, SellingPoints: sellingPoints,
		Tone: tone, VisualKeywords: visualKeywords,
		Mandatory: mandatory, Prohibited: prohibited,
		ProductAssets: append([]contract.AssetVersionRef{}, snapshot.Product.AssetRefs...),
	}
}

func (r Reader) ReadForCreative(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, reference creative.StrategyPackageReference) (creative.StrategyPackageSnapshot, error) {
	value, err := r.Service.GetPackage(ctx, actor, projectID, reference.PackageID, reference.PackageVersion)
	if err != nil {
		return creative.StrategyPackageSnapshot{}, err
	}
	if !strings.EqualFold(string(value.ContentHash), strings.TrimSpace(reference.ExpectedContentHash)) {
		return creative.StrategyPackageSnapshot{}, fmt.Errorf("strategy package content hash no longer matches the selected version")
	}
	document := value.Snapshot.Strategy
	concept := ""
	if len(document.CreativeRecommendations) > 0 {
		concept = document.CreativeRecommendations[0]
	}
	mandatory := append([]string{}, document.Constraints...)
	tone := []string{"清晰", "可信"}
	prohibited := []string{}
	if value.Snapshot.Brief.Snapshot.ContractVersion == "strategy-brief-version/v2" {
		if len(value.Snapshot.Brief.Snapshot.Creative.Tone) > 0 {
			tone = append([]string{}, value.Snapshot.Brief.Snapshot.Creative.Tone...)
		}
		mandatory = append(mandatory, value.Snapshot.Brief.Snapshot.Creative.MandatoryElements...)
		prohibited = append(prohibited, value.Snapshot.Brief.Snapshot.Creative.ProhibitedClaims...)
		for _, plan := range document.PlatformPlans {
			if plan.Platform != "xiaohongshu" {
				continue
			}
			if len(plan.CreativeIdeas) > 0 {
				concept = plan.CreativeIdeas[0]
			}
			mandatory = append(mandatory, plan.Constraints...)
			break
		}
	}
	routes := make([]creative.CreativeRouteSnapshot, 0, len(value.Snapshot.CreativeRoutes))
	for _, route := range value.Snapshot.CreativeRoutes {
		routes = append(routes, creative.CreativeRouteSnapshot{
			RouteType: route.RouteType, VideoPurpose: route.VideoPurpose,
			Channels: append([]string{}, route.Channels...), Reason: route.Reason,
			TargetDurationSeconds: route.TargetDurationSeconds, AspectRatio: route.AspectRatio,
			SourceAssetRefs:           append([]contract.AssetVersionRef{}, route.SourceAssetRefs...),
			EvidenceRefs:              append([]string{}, route.EvidenceRefs...),
			RequiresHumanConfirmation: route.RequiresHumanConfirmation,
		})
	}
	return creative.StrategyPackageSnapshot{
		PackageID: value.PackageID, PackageVersion: value.Version, ContentHash: string(value.ContentHash),
		CreativeReady: value.Snapshot.Readiness.CreativeReady,
		Objective:     document.Objective, Audience: document.Audience.Primary, CoreMessage: document.Proposition,
		Concept: concept, Tone: tone, VisualKeywords: []string{"品牌主视觉", "真实使用场景"},
		Mandatory: mandatory, Prohibited: prohibited,
		CreativeRoutes: routes,
	}, nil
}
