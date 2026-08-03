// Package strategycreative contains the explicit composition adapter between
// two bounded contexts. It is intentionally not part of either domain package.
package strategycreative

import (
	"context"
	"encoding/json"
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
	handoff, err := r.Service.GetCreativeHandoff(
		ctx, actor, projectID, reference.PackageID, reference.PackageVersion,
	)
	if err != nil {
		return creative.StrategyPackageSnapshot{}, err
	}
	if !strings.EqualFold(string(handoff.PackageRef.PackageContentHash), strings.TrimSpace(reference.ExpectedContentHash)) {
		return creative.StrategyPackageSnapshot{}, fmt.Errorf("strategy package content hash no longer matches the selected version")
	}
	if expected := strings.TrimSpace(reference.HandoffContractVersion); expected != "" &&
		expected != handoff.ContractVersion {
		return creative.StrategyPackageSnapshot{}, fmt.Errorf("strategy handoff contract no longer matches the selected version")
	}
	if expected := strings.TrimSpace(reference.ExpectedHandoffHash); expected != "" &&
		!strings.EqualFold(string(handoff.HandoffContentHash), expected) {
		return creative.StrategyPackageSnapshot{}, fmt.Errorf("strategy handoff content hash no longer matches the selected version")
	}

	audience := ""
	bestPriority := int(^uint(0) >> 1)
	for _, segment := range handoff.CreativeView.AudienceSegments {
		if segment.Priority < bestPriority {
			audience, bestPriority = segment.Label, segment.Priority
		}
	}
	mandatory := make([]string, 0)
	prohibited := make([]string, 0)
	for _, guardrail := range handoff.CreativeView.Guardrails {
		switch guardrail.Kind {
		case "mandatory":
			mandatory = append(mandatory, guardrail.Text)
		case "prohibited":
			prohibited = append(prohibited, guardrail.Text)
		}
	}
	cta := ""
	if len(handoff.CreativeView.Communication.ApprovedCTAs) > 0 {
		cta = handoff.CreativeView.Communication.ApprovedCTAs[0]
	}
	routes := make([]creative.CreativeRouteSnapshot, 0, len(handoff.Routes))
	for _, route := range handoff.Routes {
		routes = append(routes, creativeRouteSnapshotFromHandoff(route))
	}
	handoffSnapshot, err := json.Marshal(handoff)
	if err != nil {
		return creative.StrategyPackageSnapshot{}, fmt.Errorf("encode Strategy handoff snapshot: %w", err)
	}
	return creative.StrategyPackageSnapshot{
		PackageID: handoff.PackageRef.PackageID, PackageVersion: handoff.PackageRef.PackageVersion,
		ContentHash:            string(handoff.PackageRef.PackageContentHash),
		HandoffContractVersion: handoff.ContractVersion, HandoffContentHash: string(handoff.HandoffContentHash),
		CreativeReady: handoff.UpstreamReadiness.Status == "ready",
		Objective:     handoff.CreativeView.Objective.Statement, Audience: audience,
		CoreMessage:  handoff.CreativeView.Communication.SingleMindedProposition,
		CallToAction: cta, Tone: append([]string{}, handoff.CreativeView.Communication.ToneConstraints...),
		Mandatory: appendUnique(mandatory), Prohibited: appendUnique(prohibited),
		CreativeRoutes: routes, HandoffSnapshot: handoffSnapshot,
	}, nil
}

func creativeRouteSnapshotFromHandoff(route strategy.CreativeHandoffRoute) creative.CreativeRouteSnapshot {
	routeType := route.PerformanceMode
	videoPurpose := ""
	if route.DeliverableType == "image_text" {
		routeType = "image_text"
	} else {
		videoPurpose = route.Purpose
		if routeType == "" {
			routeType = "pre_roll"
		}
	}
	return creative.CreativeRouteSnapshot{
		RouteID: route.RouteID, RouteType: routeType, VideoPurpose: videoPurpose,
		Channels: append([]string{}, route.Channels...), Reason: route.Reason,
		TargetDurationSeconds: route.Spec.TargetDurationSeconds, AspectRatio: route.Spec.AspectRatio,
		Resolution:                route.Spec.Resolution,
		EvidenceRefs:              append([]string{}, route.ClaimRefs...),
		RequiresHumanConfirmation: route.DeliverableType == "video",
		ReadinessStatus:           route.RouteReadiness.Status,
	}
}

// readPackageProjectionLegacy is retained temporarily for migration tests and
// old snapshots. New handoffs must use ReadForCreative above.
func (r Reader) readPackageProjectionLegacy(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, reference creative.StrategyPackageReference) (creative.StrategyPackageSnapshot, error) {
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

// ReadTaskStrategyForCreative is the versioned, authorization-checked handoff
// from Strategy to Creative. It deliberately performs a deterministic field
// projection; no model call occurs at this integration boundary.
func (r Reader) ReadTaskStrategyForCreative(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	reference creative.TaskStrategyReference,
) (creative.TaskStrategySnapshot, error) {
	value, err := r.Service.GetCreativeTaskStrategyVersion(
		ctx, actor, reference.PlanID, reference.StrategyVersion,
	)
	if err != nil {
		return creative.TaskStrategySnapshot{}, err
	}
	if value.ProjectID != projectID {
		return creative.TaskStrategySnapshot{}, fmt.Errorf("task strategy does not belong to the selected project")
	}
	if !strings.EqualFold(strings.TrimSpace(value.ContentHash), strings.TrimSpace(reference.ExpectedContentHash)) {
		return creative.TaskStrategySnapshot{}, fmt.Errorf("task strategy content hash no longer matches the selected version")
	}
	if value.ContractVersion != creative.TaskStrategyContractVersion ||
		value.Document.ContractVersion != creative.TaskStrategyContractVersion {
		return creative.TaskStrategySnapshot{}, fmt.Errorf("unsupported task strategy contract %q", value.ContractVersion)
	}
	plan, err := r.Service.GetCreativeTaskPlan(ctx, actor, reference.PlanID)
	if err != nil {
		return creative.TaskStrategySnapshot{}, err
	}
	if plan.ProjectID != projectID || plan.BusinessCode != value.Document.BusinessRef.BusinessCode {
		return creative.TaskStrategySnapshot{}, fmt.Errorf("task strategy lineage does not match its plan")
	}
	brief, err := r.Service.GetBriefVersion(
		ctx, actor, value.Document.Lineage.BriefID, value.Document.Lineage.BriefVersion,
	)
	if err != nil {
		return creative.TaskStrategySnapshot{}, err
	}
	if brief.ProjectID != projectID ||
		!strings.EqualFold(string(brief.ContentHash), value.Document.Lineage.BriefContentHash) {
		return creative.TaskStrategySnapshot{}, fmt.Errorf("task strategy Brief lineage no longer matches")
	}
	document := value.Document
	mandatory := appendUnique(append([]string{}, brief.Snapshot.Creative.MandatoryElements...))
	prohibited := appendUnique(append([]string{}, brief.Snapshot.Creative.ProhibitedClaims...))
	tone := appendUnique(append([]string{}, brief.Snapshot.Creative.Tone...))
	visualKeywords := append([]string{}, tone...)
	if len(visualKeywords) == 0 {
		visualKeywords = append(visualKeywords, document.Audience.Insights...)
	}
	if len(visualKeywords) > 16 {
		visualKeywords = visualKeywords[:16]
	}
	media := make([]creative.TaskStrategyMediaItem, 0, len(document.Media.Items))
	for _, item := range document.Media.Items {
		media = append(media, creative.TaskStrategyMediaItem{
			AssetRef: item.AssetRef, Role: item.Role, Kind: item.Kind, MIMEType: item.MIMEType,
			Status: item.Status, Usefulness: item.Usefulness,
			StrategyUses: append([]string{}, item.StrategyUses...),
			Observations: append([]string{}, item.Observations...),
			Limitations:  append([]string{}, item.Limitations...),
			WidthPixels:  item.WidthPixels, HeightPixels: item.HeightPixels,
			DurationSeconds: item.DurationSeconds,
		})
	}
	return creative.TaskStrategySnapshot{
		PlanID: reference.PlanID, StrategyVersion: value.Version, ContentHash: value.ContentHash,
		BusinessCode: document.BusinessRef.BusinessCode,
		Objective:    document.Objective,
		Audience: creative.TaskStrategyAudience{
			Primary: document.Audience.Primary, Insights: append([]string{}, document.Audience.Insights...),
		},
		CoreMessage:  document.CoreMessage,
		CallToAction: taskStrategyCTA(plan.Answers, brief.Snapshot),
		Concept:      taskStrategyConcept(document.BusinessRef.BusinessCode, document.BusinessStrategy, document.CoreMessage),
		Tone:         tone, VisualKeywords: visualKeywords, Mandatory: mandatory, Prohibited: prohibited,
		BusinessStrategy:  cloneAnyMap(document.BusinessStrategy),
		MessageHierarchy:  append([]string{}, document.MessageHierarchy...),
		ClaimsAndEvidence: append([]string{}, document.ClaimsAndEvidence...),
		Guardrails:        appendUnique(append([]string{}, document.Guardrails...)),
		Media:             media,
		ReferenceUse: creative.TaskStrategyReferenceUse{
			Locator: document.ReferenceUse.Locator, RightsStatus: document.ReferenceUse.RightsStatus,
			IntendedUse: document.ReferenceUse.IntendedUse,
			Warnings:    append([]string{}, document.ReferenceUse.Warnings...),
		},
		OpenQuestions: append([]string{}, document.OpenQuestions...),
		Lineage: creative.TaskStrategyLineage{
			BriefID: document.Lineage.BriefID, BriefVersion: document.Lineage.BriefVersion,
			BriefContentHash:       document.Lineage.BriefContentHash,
			SourceStrategyID:       document.Lineage.SourceStrategyID,
			SourceStrategyRevision: document.Lineage.SourceStrategyRevision,
			SourceStrategyHash:     document.Lineage.SourceStrategyHash,
			BusinessGeneration:     document.BusinessRef.Generation,
			BusinessVersion:        document.BusinessRef.Version,
			BusinessContentHash:    document.BusinessRef.ContentHash,
			SkillName:              document.Lineage.SkillName, SkillVersion: document.Lineage.SkillVersion,
			SkillContentHash:      document.Lineage.SkillContentHash,
			PromptVersion:         document.Lineage.PromptVersion,
			ProjectContextVersion: document.Lineage.ProjectContextVersion,
		},
	}, nil
}

func (r Reader) ReadTaskOverlayForCreative(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	reference creative.TaskOverlayReference,
) (creative.TaskOverlaySnapshot, error) {
	value, err := r.Service.GetCreativeTaskOverlay(ctx, actor, projectID, reference.OverlayID)
	if err != nil {
		return creative.TaskOverlaySnapshot{}, err
	}
	if !strings.EqualFold(value.ContentHash, strings.TrimSpace(reference.ExpectedContentHash)) {
		return creative.TaskOverlaySnapshot{}, fmt.Errorf("task overlay content hash no longer matches the selected version")
	}
	rawSnapshot, err := json.Marshal(value)
	if err != nil {
		return creative.TaskOverlaySnapshot{}, fmt.Errorf("encode task overlay snapshot: %w", err)
	}
	return creative.TaskOverlaySnapshot{
		ContractVersion: value.ContractVersion, OverlayID: value.OverlayID,
		ContentHash: value.ContentHash,
		PackageRef: creative.StrategyPackageReference{
			PackageID: value.PackageRef.PackageID, PackageVersion: value.PackageRef.PackageVersion,
			ExpectedContentHash:    value.PackageRef.PackageContentHash,
			HandoffContractVersion: value.PackageRef.HandoffContractVersion,
			ExpectedHandoffHash:    value.PackageRef.HandoffContentHash,
		},
		SelectedRouteID:         value.SelectedRouteID,
		TaskStrategyPlanID:      value.TaskStrategyRef.PlanID,
		TaskStrategyVersion:     value.TaskStrategyRef.Version,
		TaskStrategyContentHash: value.TaskStrategyRef.ContentHash,
		ObjectiveRefinement:     value.ObjectiveRefinement,
		AudienceRefinement:      value.AudienceRefinement,
		MessagePriorities:       append([]string{}, value.MessagePriorities...),
		StrategyDimensions:      cloneAnyMap(value.StrategyDimensions),
		Hypotheses:              append([]string{}, value.Hypotheses...),
		Guardrails:              append([]string{}, value.Guardrails...),
		OpenQuestions:           append([]string{}, value.OpenQuestions...),
		RawSnapshot:             rawSnapshot,
	}, nil
}

func taskStrategyConcept(code string, values map[string]any, fallback string) string {
	keys := map[string][]string{
		creative.BusinessXiaohongshuImageText: {"content_angle"},
		creative.BusinessCommercePreroll:      {"conversion_message", "opening_mechanisms"},
		creative.BusinessShortDramaPreroll:    {"audience_bridge", "hook_mechanisms"},
		creative.BusinessViralRemake:          {"product_mapping", "transferable_mechanisms"},
	}
	for _, key := range keys[code] {
		if value := firstStrategyValue(values[key]); value != "" {
			return value
		}
	}
	return strings.TrimSpace(fallback)
}

func firstStrategyValue(value any) string {
	switch item := value.(type) {
	case string:
		return strings.TrimSpace(item)
	case []string:
		if len(item) > 0 {
			return strings.TrimSpace(item[0])
		}
	case []any:
		if len(item) > 0 {
			return strings.TrimSpace(fmt.Sprint(item[0]))
		}
	}
	return ""
}

func taskStrategyCTA(answers map[string]json.RawMessage, brief strategy.BriefDocument) string {
	for _, key := range []string{"conversion_action", "interaction_goal"} {
		var value string
		if raw, found := answers[key]; found && json.Unmarshal(raw, &value) == nil {
			labels := map[string]string{
				"purchase": "立即购买", "visit": "了解更多", "coupon": "领取优惠",
				"live": "进入直播间", "save": "收藏内容", "comment": "参与评论",
				"search": "搜索品牌", "install": "立即安装", "register": "立即注册",
				"reserve": "立即预约", "reactivate": "返回体验",
			}
			if label := labels[value]; label != "" {
				return label
			}
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	for _, platform := range brief.PlatformBriefs {
		if strings.TrimSpace(platform.ConversionPath) != "" {
			return strings.TrimSpace(platform.ConversionPath)
		}
	}
	return ""
}

func appendUnique(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func cloneAnyMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}
