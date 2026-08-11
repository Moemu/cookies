package creative

import (
	"context"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type RequirementSnapshotReference struct {
	BriefID      string `json:"brief_id"`
	BriefVersion int64  `json:"brief_version"`
	ContentHash  string `json:"content_hash"`
}

func (r RequirementSnapshotReference) Validate() error {
	if strings.TrimSpace(r.BriefID) == "" || r.BriefVersion < 1 {
		return fmt.Errorf("requirement snapshot brief_id and brief_version are required")
	}
	_, err := contract.ParseContentHash(r.ContentHash)
	return err
}

type BusinessCapabilityReference struct {
	BusinessCode string `json:"business_code"`
	Version      string `json:"version"`
	ContentHash  string `json:"content_hash"`
}

func (r BusinessCapabilityReference) Validate() error {
	if strings.TrimSpace(r.BusinessCode) == "" || strings.TrimSpace(r.Version) == "" {
		return fmt.Errorf("business capability code and version are required")
	}
	_, err := contract.ParseContentHash(r.ContentHash)
	return err
}

// RequirementSnapshot is returned by the Strategy-to-Creative adapter after
// authorization and immutable hash checks. Callers never submit this body.
type RequirementSnapshot struct {
	Objective         string
	DeliverableIntent string
	ProductOrSubject  string
	Audience          string
	CoreMessage       string
	SellingPoints     []string
	Constraints       []string
	ProhibitedClaims  []string
	AssetRefs         []contract.AssetVersionRef
	ReferenceIDs      []string
	Assumptions       []string
	BlockingQuestions []string
}

type RequirementSnapshotInput struct {
	Objective         string                     `json:"objective"`
	DeliverableIntent string                     `json:"deliverable_intent"`
	ProductOrSubject  string                     `json:"product_or_subject"`
	Audience          string                     `json:"audience"`
	CoreMessage       string                     `json:"core_message"`
	SellingPoints     []string                   `json:"selling_points"`
	Constraints       []string                   `json:"constraints"`
	ProhibitedClaims  []string                   `json:"prohibited_claims"`
	AssetRefs         []contract.AssetVersionRef `json:"asset_refs"`
	ReferenceIDs      []string                   `json:"reference_ids"`
	Assumptions       []string                   `json:"assumptions"`
	BlockingQuestions []string                   `json:"blocking_questions"`
}

func (r CreateIntakeRequest) validateRequirementSnapshotV4() error {
	if r.ContractVersion != CreativeIntakeCreateV4ContractVersion || r.RequirementSnapshotRef == nil || r.BusinessCapabilityRef == nil {
		return fmt.Errorf("requirement snapshot intake requires the v4 contract and immutable requirement/capability refs")
	}
	if r.BusinessCapabilityRef.BusinessCode != BusinessViralRemake {
		return ErrFullStrategyRequired
	}
	if r.SelectedRouteID != ManualViralRemakeRouteID {
		return fmt.Errorf("viral remake requirement intake requires selected_route_id %q", ManualViralRemakeRouteID)
	}
	if err := r.RequirementSnapshotRef.Validate(); err != nil {
		return err
	}
	if err := r.BusinessCapabilityRef.Validate(); err != nil {
		return err
	}
	if r.RequirementSnapshotInput != nil || r.ParentIntakeID != "" || r.StrategyPackage != nil || r.StrategyPackageRef != nil ||
		r.TaskStrategy != nil || r.TaskStrategyInput != nil || r.TaskOverlay != nil || r.TaskOverlayRef != nil || r.TaskOverlayInput != nil ||
		r.StrategyHandoffInput != nil || len(r.CreativeRoutes) != 0 || r.Format != "" || r.PerformanceMode != "" ||
		r.ManualViralRemake != nil || r.ManualShortDramaPreroll != nil || r.ManualGamePreroll != nil ||
		r.ManualCommercePreroll != nil || r.ManualBrandFilm != nil || r.Channel != "" || strings.TrimSpace(r.Objective) != "" ||
		strings.TrimSpace(r.Audience) != "" || strings.TrimSpace(r.CoreMessage) != "" || strings.TrimSpace(r.CallToAction) != "" ||
		strings.TrimSpace(r.Concept) != "" || len(r.Tone) != 0 || len(r.VisualKeywords) != 0 || len(r.Mandatory) != 0 || len(r.Prohibited) != 0 {
		return fmt.Errorf("requirement snapshot content is resolved server-side and must not be submitted by a caller")
	}
	return nil
}

func requirementSnapshotInput(snapshot RequirementSnapshot) *RequirementSnapshotInput {
	return &RequirementSnapshotInput{
		Objective: snapshot.Objective, DeliverableIntent: snapshot.DeliverableIntent,
		ProductOrSubject: snapshot.ProductOrSubject, Audience: snapshot.Audience,
		CoreMessage:       snapshot.CoreMessage,
		SellingPoints:     append([]string{}, snapshot.SellingPoints...),
		Constraints:       append([]string{}, snapshot.Constraints...),
		ProhibitedClaims:  append([]string{}, snapshot.ProhibitedClaims...),
		AssetRefs:         append([]contract.AssetVersionRef{}, snapshot.AssetRefs...),
		ReferenceIDs:      append([]string{}, snapshot.ReferenceIDs...),
		Assumptions:       append([]string{}, snapshot.Assumptions...),
		BlockingQuestions: append([]string{}, snapshot.BlockingQuestions...),
	}
}

func (s Service) resolveRequirementIntake(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	request CreateIntakeRequest,
) (CreateIntakeRequest, []string, []string, error) {
	if s.Requirements == nil {
		return CreateIntakeRequest{}, nil, nil, fmt.Errorf("requirement snapshot intake is unavailable")
	}
	snapshot, err := s.Requirements.ReadRequirementForCreative(
		ctx, actor, projectID, *request.RequirementSnapshotRef, *request.BusinessCapabilityRef,
	)
	if err != nil {
		return CreateIntakeRequest{}, nil, nil, err
	}
	if snapshot.DeliverableIntent != "" && snapshot.DeliverableIntent != PerformanceModeViralRemake {
		return CreateIntakeRequest{}, nil, nil, ErrFullStrategyRequired
	}

	coreMessage := strings.TrimSpace(snapshot.CoreMessage)
	if coreMessage == "" && len(snapshot.SellingPoints) > 0 {
		coreMessage = strings.TrimSpace(snapshot.SellingPoints[0])
	}
	if coreMessage == "" {
		coreMessage = strings.TrimSpace(snapshot.ProductOrSubject)
	}
	resolved := CreateIntakeRequest{
		ContractVersion:          request.ContractVersion,
		Source:                   IntakeSourceRequirement,
		RequirementSnapshotRef:   request.RequirementSnapshotRef,
		BusinessCapabilityRef:    request.BusinessCapabilityRef,
		RequirementSnapshotInput: requirementSnapshotInput(snapshot),
		SelectedRouteID:          request.SelectedRouteID,
		Format:                   FormatVideo,
		PerformanceMode:          PerformanceModeViralRemake,
		Channel:                  ChannelDouyin,
		Objective:                strings.TrimSpace(snapshot.Objective),
		Audience:                 strings.TrimSpace(snapshot.Audience),
		CoreMessage:              coreMessage,
		CallToAction:             "了解更多",
		Tone:                     []string{},
		VisualKeywords:           []string{},
		Mandatory:                append([]string{}, snapshot.Constraints...),
		Prohibited:               append([]string{}, snapshot.ProhibitedClaims...),
	}

	missing := make([]string, 0, 5+len(snapshot.BlockingQuestions))
	if resolved.Objective == "" {
		missing = append(missing, "requirement.core.objective")
	}
	if strings.TrimSpace(snapshot.ProductOrSubject) == "" {
		missing = append(missing, "requirement.core.product_or_subject")
	}
	if resolved.Audience == "" {
		missing = append(missing, "requirement.core.audience")
	}
	for _, question := range snapshot.BlockingQuestions {
		if question = strings.TrimSpace(question); question != "" {
			missing = append(missing, "requirement.unknown:"+question)
		}
	}

	warnings := make([]string, 0, len(snapshot.Assumptions)+2)
	for _, assumption := range snapshot.Assumptions {
		if assumption = strings.TrimSpace(assumption); assumption != "" {
			warnings = append(warnings, "未证实假设："+assumption)
		}
	}
	var referenceVideo contract.AssetVersionRef
	var referenceImage *contract.AssetVersionRef
	videoReady := false
	if len(snapshot.AssetRefs) > 0 && s.Assets == nil {
		return CreateIntakeRequest{}, nil, nil, fmt.Errorf("creative asset reader is required for requirement intake")
	}
	for _, ref := range snapshot.AssetRefs {
		asset, readErr := s.Assets.ReadForCreative(ctx, actor, projectID, ref)
		if readErr != nil || asset.Ref != ref {
			warnings = append(warnings, "素材不可用："+string(ref.AssetID)+"#"+formatRequirementVersion(ref.Version))
			continue
		}
		switch asset.Kind {
		case contract.AssetVideo:
			if referenceVideo.AssetID == "" {
				referenceVideo = ref
				videoReady = asset.Ready && asset.MIMEType == "video/mp4"
			}
		case contract.AssetImage:
			if referenceImage == nil && asset.Ready {
				value := ref
				referenceImage = &value
			}
		}
	}
	if referenceVideo.AssetID == "" {
		missing = append(missing, "requirement.reference_video")
	} else if !videoReady {
		missing = append(missing, "requirement.reference_video.ready")
	}

	route := CreativeRouteSnapshot{
		RouteID: request.SelectedRouteID, RouteType: PerformanceModeViralRemake,
		VideoPurpose: "performance", Channels: []string{string(ChannelDouyin)},
		Reason:                "从已确认 Requirement 直接进入爆款裂变，不创建形式化策略文档",
		TargetDurationSeconds: 15, AspectRatio: "9:16", RequiresHumanConfirmation: true,
		ReadinessStatus: map[bool]string{true: "ready", false: "needs_clarification"}[len(missing) == 0],
	}
	if referenceVideo.AssetID != "" {
		route.SourceAssetRefs = []contract.AssetVersionRef{referenceVideo}
	} else {
		route.SourceAssetRefs = []contract.AssetVersionRef{}
	}
	resolved.CreativeRoutes = []CreativeRouteSnapshot{route}
	if referenceVideo.AssetID != "" {
		resolved.ManualViralRemake = &ManualViralRemakeInput{
			ProductName:     strings.TrimSpace(snapshot.ProductOrSubject),
			SellingPoints:   append([]string{}, snapshot.SellingPoints...),
			UserInstruction: strings.TrimSpace(snapshot.Objective + "；" + coreMessage),
			ReferenceVideo:  referenceVideo, ReferenceImage: referenceImage,
			ReferenceVideoRights: RightsPending,
		}
		if referenceImage != nil {
			resolved.ManualViralRemake.ReferenceImageRights = RightsPending
		}
		warnings = append(warnings, "参考素材权利状态待生产前确认")
	}
	return resolved, missing, warnings, nil
}

func formatRequirementVersion(value int64) string {
	return fmt.Sprintf("%d", value)
}
