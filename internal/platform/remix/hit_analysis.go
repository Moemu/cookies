package remix

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	HitRoleHook    HitSegmentRole = "hook"
	HitRoleProblem HitSegmentRole = "problem"
	HitRoleProof   HitSegmentRole = "proof"
	HitRoleOffer   HitSegmentRole = "offer"
	HitRoleCTA     HitSegmentRole = "cta"
)

type HitSegmentRole string

type CreateHitAnalysisRequest struct {
	SourceAsset     contract.AssetVersionRef `json:"source_asset"`
	Title           string                   `json:"title"`
	DurationSeconds float64                  `json:"duration_seconds"`
	Language        string                   `json:"language"`
	Notes           string                   `json:"notes"`
}

type HitAnalysis struct {
	ID                  string                   `json:"id"`
	OrganizationID      contract.OrganizationID  `json:"organization_id"`
	ProjectID           contract.ProjectID       `json:"project_id"`
	SourceAsset         contract.AssetVersionRef `json:"source_asset"`
	Title               string                   `json:"title"`
	VideoMeta           HitVideoMeta             `json:"video_meta"`
	Segments            []HitSegment             `json:"segments"`
	Scripts             []HitScriptLine          `json:"scripts"`
	VisualElements      []string                 `json:"visual_elements"`
	ConversionNodes     []HitConversionNode      `json:"conversion_nodes"`
	ReplicationInsights []string                 `json:"replication_insights"`
	CreatedBy           contract.Principal       `json:"created_by"`
	CreatedAt           time.Time                `json:"created_at"`
	UpdatedAt           time.Time                `json:"updated_at"`
}

type HitVideoMeta struct {
	DurationSeconds float64 `json:"duration_seconds"`
	Language        string  `json:"language"`
}

type HitSegment struct {
	ID              string         `json:"id"`
	StartSeconds    float64        `json:"start_seconds"`
	EndSeconds      float64        `json:"end_seconds"`
	Role            HitSegmentRole `json:"role"`
	Summary         string         `json:"summary"`
	Script          string         `json:"script"`
	VisualElement   string         `json:"visual_element"`
	ConversionCue   string         `json:"conversion_cue"`
	ReplicationHint string         `json:"replication_hint"`
}

type HitScriptLine struct {
	SegmentID string `json:"segment_id"`
	Text      string `json:"text"`
}

type HitConversionNode struct {
	SegmentID string `json:"segment_id"`
	Cue       string `json:"cue"`
}

type CreateProductMappingRequest struct {
	HitAnalysisID    string                     `json:"hit_analysis_id"`
	TargetProduct    ProductProfile             `json:"target_product"`
	RequiredAssets   []contract.AssetVersionRef `json:"required_assets"`
	ReplacementRules []ReplacementRule          `json:"replacement_rules"`
	Constraints      []string                   `json:"constraints"`
	TargetSeconds    int                        `json:"target_seconds"`
	Pace             Pace                       `json:"pace"`
}

type ProductMapping struct {
	ID               string                     `json:"id"`
	OrganizationID   contract.OrganizationID    `json:"organization_id"`
	ProjectID        contract.ProjectID         `json:"project_id"`
	HitAnalysisID    string                     `json:"hit_analysis_id"`
	TargetProduct    ProductProfile             `json:"target_product"`
	RequiredAssets   []contract.AssetVersionRef `json:"required_assets"`
	ReplacementRules []ReplacementRule          `json:"replacement_rules"`
	Constraints      []string                   `json:"constraints"`
	TargetSeconds    int                        `json:"target_seconds"`
	Pace             Pace                       `json:"pace"`
	CreatedBy        contract.Principal         `json:"created_by"`
	CreatedAt        time.Time                  `json:"created_at"`
	UpdatedAt        time.Time                  `json:"updated_at"`
}

type ProductProfile struct {
	Name          string   `json:"name"`
	SellingPoints []string `json:"selling_points"`
	CTA           string   `json:"cta"`
}

type ReplacementRule struct {
	Role        HitSegmentRole           `json:"role"`
	TargetAsset contract.AssetVersionRef `json:"target_asset"`
	Message     string                   `json:"message"`
}

func (r CreateHitAnalysisRequest) Validate() error {
	if err := r.SourceAsset.Validate(); err != nil {
		return fmt.Errorf("source_asset: %w", err)
	}
	if strings.TrimSpace(r.Title) == "" || len(r.Title) > 160 {
		return fmt.Errorf("title must be between 1 and 160 characters")
	}
	if r.DurationSeconds <= 0 || r.DurationSeconds > 600 {
		return fmt.Errorf("duration_seconds must be between 0 and 600")
	}
	if len(r.Notes) > 2000 {
		return fmt.Errorf("notes is too long")
	}
	return nil
}

func (a HitAnalysis) Validate() error {
	if strings.TrimSpace(a.ID) == "" || strings.TrimSpace(string(a.OrganizationID)) == "" || strings.TrimSpace(string(a.ProjectID)) == "" {
		return fmt.Errorf("hit analysis identity is incomplete")
	}
	if err := a.SourceAsset.Validate(); err != nil {
		return fmt.Errorf("source_asset: %w", err)
	}
	if len(a.Segments) == 0 || len(a.Segments) > 24 {
		return fmt.Errorf("hit analysis requires 1 to 24 segments")
	}
	if err := validateContinuousSegments(a.Segments, a.VideoMeta.DurationSeconds); err != nil {
		return err
	}
	return nil
}

func (r CreateProductMappingRequest) Validate() error {
	if strings.TrimSpace(r.HitAnalysisID) == "" || len(r.HitAnalysisID) > 128 {
		return fmt.Errorf("hit_analysis_id must be between 1 and 128 characters")
	}
	if err := r.TargetProduct.Validate(); err != nil {
		return fmt.Errorf("target_product: %w", err)
	}
	if len(r.RequiredAssets) == 0 || len(r.RequiredAssets) > 80 {
		return fmt.Errorf("required_assets must contain 1 to 80 assets")
	}
	required := map[contract.AssetVersionRef]bool{}
	for index, asset := range r.RequiredAssets {
		if err := asset.Validate(); err != nil {
			return fmt.Errorf("required_assets %d: %w", index, err)
		}
		required[asset] = true
	}
	if len(r.ReplacementRules) == 0 || len(r.ReplacementRules) > 80 {
		return fmt.Errorf("replacement_rules must contain 1 to 80 rules")
	}
	coveredRoles := map[HitSegmentRole]bool{}
	for index, rule := range r.ReplacementRules {
		if err := rule.Validate(); err != nil {
			return fmt.Errorf("replacement_rules %d: %w", index, err)
		}
		if !required[rule.TargetAsset] {
			return fmt.Errorf("replacement_rules %d target_asset must be listed in required_assets", index)
		}
		coveredRoles[rule.Role] = true
	}
	if !coveredRoles[HitRoleHook] || !coveredRoles[HitRoleProof] || !coveredRoles[HitRoleCTA] {
		return fmt.Errorf("replacement_rules must cover hook, proof, and cta roles")
	}
	if r.TargetSeconds != 0 && (r.TargetSeconds < 9 || r.TargetSeconds > 180) {
		return fmt.Errorf("target_seconds must be between 9 and 180")
	}
	if r.Pace != "" && r.Pace != PaceFast && r.Pace != PaceBalanced && r.Pace != PaceStory {
		return fmt.Errorf("pace must be fast, balanced, or story")
	}
	if len(r.Constraints) > 20 {
		return fmt.Errorf("constraints exceed supported limits")
	}
	return nil
}

func (p ProductProfile) Validate() error {
	if strings.TrimSpace(p.Name) == "" || len(p.Name) > 120 {
		return fmt.Errorf("name must be between 1 and 120 characters")
	}
	if len(p.SellingPoints) == 0 || len(p.SellingPoints) > 12 {
		return fmt.Errorf("selling_points must contain 1 to 12 items")
	}
	for _, point := range p.SellingPoints {
		if strings.TrimSpace(point) == "" || len(point) > 160 {
			return fmt.Errorf("selling_points entries must be between 1 and 160 characters")
		}
	}
	if strings.TrimSpace(p.CTA) == "" || len(p.CTA) > 80 {
		return fmt.Errorf("cta must be between 1 and 80 characters")
	}
	return nil
}

func (r ReplacementRule) Validate() error {
	if !validHitRole(r.Role) {
		return fmt.Errorf("role must be hook, problem, proof, offer, or cta")
	}
	if err := r.TargetAsset.Validate(); err != nil {
		return fmt.Errorf("target_asset: %w", err)
	}
	if strings.TrimSpace(r.Message) == "" || len(r.Message) > 240 {
		return fmt.Errorf("message must be between 1 and 240 characters")
	}
	return nil
}

func (s *Service) CreateHitAnalysis(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreateHitAnalysisRequest) (HitAnalysis, error) {
	if err := ctx.Err(); err != nil {
		return HitAnalysis{}, err
	}
	if err := request.Validate(); err != nil {
		return HitAnalysis{}, err
	}
	id, err := s.newID()
	if err != nil {
		return HitAnalysis{}, err
	}
	now := s.nowUTC()
	analysis := fakeAnalyzeHitVideo(id, actor, projectID, request, now)
	if err := analysis.Validate(); err != nil {
		return HitAnalysis{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hitAnalyses[id] = analysis
	return cloneHitAnalysis(analysis), nil
}

func (s *Service) GetHitAnalysis(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (HitAnalysis, error) {
	if err := ctx.Err(); err != nil {
		return HitAnalysis{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	analysis, ok := s.hitAnalyses[id]
	if !ok || analysis.OrganizationID != actor.OrganizationID || analysis.ProjectID != projectID {
		return HitAnalysis{}, ErrNotFound
	}
	return cloneHitAnalysis(analysis), nil
}

func (s *Service) CreateProductMapping(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreateProductMappingRequest) (ProductMapping, error) {
	if err := ctx.Err(); err != nil {
		return ProductMapping{}, err
	}
	request = normalizeProductMappingRequest(request)
	if err := request.Validate(); err != nil {
		return ProductMapping{}, err
	}
	id, err := s.newID()
	if err != nil {
		return ProductMapping{}, err
	}
	now := s.nowUTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	analysis, ok := s.hitAnalyses[request.HitAnalysisID]
	if !ok || analysis.OrganizationID != actor.OrganizationID || analysis.ProjectID != projectID {
		return ProductMapping{}, ErrNotFound
	}
	if mappingUsesSourceAsset(request, analysis.SourceAsset) {
		return ProductMapping{}, fmt.Errorf("%w: replacement rules must not reuse the source video asset", ErrInvalidMapping)
	}
	mapping := ProductMapping{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID, HitAnalysisID: request.HitAnalysisID,
		TargetProduct: request.TargetProduct, RequiredAssets: append([]contract.AssetVersionRef(nil), request.RequiredAssets...),
		ReplacementRules: append([]ReplacementRule(nil), request.ReplacementRules...), Constraints: append([]string(nil), request.Constraints...),
		TargetSeconds: request.TargetSeconds, Pace: request.Pace, CreatedBy: actor.Principal, CreatedAt: now, UpdatedAt: now,
	}
	s.productMappings[id] = mapping
	return cloneProductMapping(mapping), nil
}

func (s *Service) GetProductMapping(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (ProductMapping, error) {
	if err := ctx.Err(); err != nil {
		return ProductMapping{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	mapping, ok := s.productMappings[id]
	if !ok || mapping.OrganizationID != actor.OrganizationID || mapping.ProjectID != projectID {
		return ProductMapping{}, ErrNotFound
	}
	return cloneProductMapping(mapping), nil
}

func (s *Service) GeneratePlanFromProductMapping(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, mappingID string) (Plan, error) {
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	s.mu.RLock()
	mapping, ok := s.productMappings[mappingID]
	analysis := s.hitAnalyses[mapping.HitAnalysisID]
	s.mu.RUnlock()
	if !ok || mapping.OrganizationID != actor.OrganizationID || mapping.ProjectID != projectID || analysis.ID == "" {
		return Plan{}, ErrNotFound
	}
	request := planRequestFromMapping(analysis, mapping)
	return s.Create(ctx, actor, projectID, request)
}

func fakeAnalyzeHitVideo(id string, actor contract.ActorContext, projectID contract.ProjectID, request CreateHitAnalysisRequest, now time.Time) HitAnalysis {
	duration := roundTimeline(request.DurationSeconds)
	cuts := []float64{0, roundTimeline(duration * 0.2), roundTimeline(duration * 0.72), duration}
	segments := []HitSegment{
		{ID: id + "_seg_01", StartSeconds: cuts[0], EndSeconds: cuts[1], Role: HitRoleHook, Summary: "3 秒内制造注意力缺口", Script: "先抛出结果反差，迫使用户继续观看。", VisualElement: "强对比开场画面", ConversionCue: "注意力停留", ReplicationHint: "用目标商品的高差异卖点替换原始冲突。"},
		{ID: id + "_seg_02", StartSeconds: cuts[1], EndSeconds: cuts[2], Role: HitRoleProof, Summary: "展示核心证据与使用场景", Script: "用连续镜头证明卖点可信。", VisualElement: "产品细节与场景演示", ConversionCue: "信任建立", ReplicationHint: "只使用已授权项目素材重建证明段。"},
		{ID: id + "_seg_03", StartSeconds: cuts[2], EndSeconds: cuts[3], Role: HitRoleCTA, Summary: "收束利益点并触发行动", Script: "给出清晰行动提示。", VisualElement: "商品定格与品牌收口", ConversionCue: "行动引导", ReplicationHint: "使用目标商品 CTA 和品牌边界改写结尾。"},
	}
	scripts := make([]HitScriptLine, 0, len(segments))
	conversions := make([]HitConversionNode, 0, len(segments))
	visuals := make([]string, 0, len(segments))
	insights := make([]string, 0, len(segments))
	for _, segment := range segments {
		scripts = append(scripts, HitScriptLine{SegmentID: segment.ID, Text: segment.Script})
		conversions = append(conversions, HitConversionNode{SegmentID: segment.ID, Cue: segment.ConversionCue})
		visuals = append(visuals, segment.VisualElement)
		insights = append(insights, segment.ReplicationHint)
	}
	language := strings.TrimSpace(request.Language)
	if language == "" {
		language = "zh-CN"
	}
	return HitAnalysis{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID, SourceAsset: request.SourceAsset, Title: strings.TrimSpace(request.Title),
		VideoMeta: HitVideoMeta{DurationSeconds: duration, Language: language}, Segments: segments, Scripts: scripts, VisualElements: visuals,
		ConversionNodes: conversions, ReplicationInsights: insights, CreatedBy: actor.Principal, CreatedAt: now, UpdatedAt: now,
	}
}

func normalizeProductMappingRequest(request CreateProductMappingRequest) CreateProductMappingRequest {
	if request.TargetSeconds == 0 {
		request.TargetSeconds = 30
	}
	if request.Pace == "" {
		request.Pace = PaceBalanced
	}
	return request
}

func planRequestFromMapping(analysis HitAnalysis, mapping ProductMapping) CreatePlanRequest {
	rulesByRole := make(map[HitSegmentRole][]ReplacementRule)
	for _, rule := range mapping.ReplacementRules {
		rulesByRole[rule.Role] = append(rulesByRole[rule.Role], rule)
	}
	segmentsByPlan := map[Segment][]Shot{
		SegmentOpening: {},
		SegmentMiddle:  {},
		SegmentEnding:  {},
	}
	targetSeconds := mapping.TargetSeconds
	if targetSeconds == 0 {
		targetSeconds = 30
	}
	scale := float64(targetSeconds) / analysis.VideoMeta.DurationSeconds
	cursorBySegment := map[Segment]float64{}
	for index, hitSegment := range analysis.Segments {
		planSegment := planSegmentForHitRole(hitSegment.Role)
		rule := firstRuleForRole(rulesByRole, hitSegment.Role)
		duration := roundTimeline((hitSegment.EndSeconds - hitSegment.StartSeconds) * scale)
		start := roundTimeline(cursorBySegment[planSegment])
		shot := Shot{
			ID:           fmt.Sprintf("%s_%02d", string(planSegment), index+1),
			Segment:      planSegment,
			Source:       ShotSourceExistingAsset,
			AssetVersion: rule.TargetAsset,
			Timeline:     ShotTimeline{StartSeconds: start, DurationSeconds: duration, InPointSeconds: 0, OutPointSeconds: duration},
			Creative: ShotCreative{
				Scene: hitSegment.Summary, ShotType: string(hitSegment.Role), DialogueOrNarration: rule.Message,
				Subtitle: rule.Message, Transition: "cut", CTAElement: ctaForRole(hitSegment.Role, mapping.TargetProduct.CTA),
			},
			Planning: ShotPlanning{
				Score: 0.86, ReasonCodes: []string{"hit_analysis", string(hitSegment.Role), "product_mapping"},
				Reason:   fmt.Sprintf("复刻 %s 结构并映射到%s", hitSegment.Role, mapping.TargetProduct.Name),
				Evidence: []string{hitSegment.ReplicationHint},
			},
			Risks: append([]string(nil), mapping.Constraints...),
		}
		segmentsByPlan[planSegment] = append(segmentsByPlan[planSegment], shot)
		cursorBySegment[planSegment] = roundTimeline(cursorBySegment[planSegment] + duration)
	}
	segments := make([]SegmentPlan, 0, 3)
	actualSeconds := 0.0
	for _, segment := range []Segment{SegmentOpening, SegmentMiddle, SegmentEnding} {
		shots := segmentsByPlan[segment]
		if len(shots) == 0 {
			shots = []Shot{fallbackShot(segment, mapping)}
		}
		seconds := sumShotSeconds(shots)
		actualSeconds += seconds
		segments = append(segments, SegmentPlan{
			Segment: segment, Label: segmentLabelsForPlan(segment), TargetSeconds: segmentTargetSeconds(segment, targetSeconds),
			ActualSeconds: seconds, Shots: shots,
		})
	}
	return CreatePlanRequest{
		SchemaVersion: SchemaVersionV2, ClientPlanID: "mapping_" + mapping.ID, TargetSeconds: targetSeconds,
		ActualSeconds: roundTimeline(actualSeconds), Pace: mapping.Pace, Segments: segments,
		Warnings: []string{"计划由爆款结构映射生成，未默认复用原视频二进制。"},
		Summary:  PlanSummary{SelectedAssets: len(mapping.RequiredAssets), UsedAssets: countUsedAssets(segments), CoveragePercent: coveragePercent(actualSeconds, targetSeconds), Strategy: "hit-analysis product mapping"},
	}
}

func validateContinuousSegments(segments []HitSegment, duration float64) error {
	if duration <= 0 {
		return fmt.Errorf("video_meta duration_seconds must be positive")
	}
	sorted := append([]HitSegment(nil), segments...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].StartSeconds < sorted[j].StartSeconds })
	cursor := 0.0
	for index, segment := range sorted {
		if strings.TrimSpace(segment.ID) == "" || !validHitRole(segment.Role) {
			return fmt.Errorf("segment %d identity or role is invalid", index)
		}
		if math.Abs(segment.StartSeconds-cursor) > 0.05 || segment.EndSeconds <= segment.StartSeconds {
			return fmt.Errorf("segments must be continuous and non-overlapping")
		}
		cursor = segment.EndSeconds
	}
	if math.Abs(cursor-duration) > 0.05 {
		return fmt.Errorf("segments must cover the full video duration")
	}
	return nil
}

func validHitRole(role HitSegmentRole) bool {
	return role == HitRoleHook || role == HitRoleProblem || role == HitRoleProof || role == HitRoleOffer || role == HitRoleCTA
}

func mappingUsesSourceAsset(request CreateProductMappingRequest, source contract.AssetVersionRef) bool {
	for _, rule := range request.ReplacementRules {
		if rule.TargetAsset == source {
			return true
		}
	}
	return false
}

func firstRuleForRole(rules map[HitSegmentRole][]ReplacementRule, role HitSegmentRole) ReplacementRule {
	if matches := rules[role]; len(matches) > 0 {
		return matches[0]
	}
	if role == HitRoleProblem || role == HitRoleOffer {
		if matches := rules[HitRoleProof]; len(matches) > 0 {
			return matches[0]
		}
	}
	for _, fallbackRole := range []HitSegmentRole{HitRoleHook, HitRoleProof, HitRoleCTA} {
		if matches := rules[fallbackRole]; len(matches) > 0 {
			return matches[0]
		}
	}
	return ReplacementRule{}
}

func planSegmentForHitRole(role HitSegmentRole) Segment {
	if role == HitRoleHook {
		return SegmentOpening
	}
	if role == HitRoleCTA || role == HitRoleOffer {
		return SegmentEnding
	}
	return SegmentMiddle
}

func fallbackShot(segment Segment, mapping ProductMapping) Shot {
	asset := mapping.RequiredAssets[0]
	if segment == SegmentEnding && len(mapping.RequiredAssets) > 1 {
		asset = mapping.RequiredAssets[len(mapping.RequiredAssets)-1]
	}
	duration := 3.0
	return Shot{
		ID: string(segment) + "_fallback_1", Segment: segment, Source: ShotSourceExistingAsset, AssetVersion: asset,
		Timeline: ShotTimeline{StartSeconds: 0, DurationSeconds: duration, InPointSeconds: 0, OutPointSeconds: duration},
		Creative: ShotCreative{Scene: string(segment), ShotType: "bridge", Transition: "cut", CTAElement: ctaForRole(HitRoleCTA, mapping.TargetProduct.CTA)},
		Planning: ShotPlanning{Score: 0.72, ReasonCodes: []string{"mapping_fallback"}, Reason: "补齐三段式计划结构", Evidence: []string{"required_assets"}},
		Risks:    append([]string(nil), mapping.Constraints...),
	}
}

func ctaForRole(role HitSegmentRole, cta string) string {
	if role == HitRoleCTA || role == HitRoleOffer {
		return cta
	}
	return ""
}

func segmentLabelsForPlan(segment Segment) string {
	if segment == SegmentOpening {
		return "前段"
	}
	if segment == SegmentMiddle {
		return "中段"
	}
	return "后段"
}

func segmentTargetSeconds(segment Segment, target int) int {
	if segment == SegmentMiddle {
		return target / 2
	}
	return target / 4
}

func sumShotSeconds(shots []Shot) float64 {
	total := 0.0
	for _, shot := range shots {
		total += shot.Timeline.DurationSeconds
	}
	return roundTimeline(total)
}

func countUsedAssets(segments []SegmentPlan) int {
	used := map[contract.AssetVersionRef]bool{}
	for _, segment := range segments {
		for _, shot := range segment.Shots {
			used[shot.AssetVersion] = true
		}
	}
	return len(used)
}

func coveragePercent(actual float64, target int) int {
	if target <= 0 {
		return 0
	}
	percent := int(math.Round(actual / float64(target) * 100))
	if percent > 100 {
		return 100
	}
	if percent < 0 {
		return 0
	}
	return percent
}

func roundTimeline(value float64) float64 {
	return math.Round(value*10) / 10
}

func cloneHitAnalysis(analysis HitAnalysis) HitAnalysis {
	analysis.Segments = append([]HitSegment(nil), analysis.Segments...)
	analysis.Scripts = append([]HitScriptLine(nil), analysis.Scripts...)
	analysis.VisualElements = append([]string(nil), analysis.VisualElements...)
	analysis.ConversionNodes = append([]HitConversionNode(nil), analysis.ConversionNodes...)
	analysis.ReplicationInsights = append([]string(nil), analysis.ReplicationInsights...)
	return analysis
}

func cloneProductMapping(mapping ProductMapping) ProductMapping {
	mapping.TargetProduct.SellingPoints = append([]string(nil), mapping.TargetProduct.SellingPoints...)
	mapping.RequiredAssets = append([]contract.AssetVersionRef(nil), mapping.RequiredAssets...)
	mapping.ReplacementRules = append([]ReplacementRule(nil), mapping.ReplacementRules...)
	mapping.Constraints = append([]string(nil), mapping.Constraints...)
	return mapping
}
