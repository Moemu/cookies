package creative

import (
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	TaskStrategyContractVersion = "creative-task-strategy/v1"

	BusinessXiaohongshuImageText = "xiaohongshu_image_text"
	BusinessShortDramaPreroll    = "short_drama_preroll"
	BusinessCommercePreroll      = "commerce_preroll"
	BusinessViralRemake          = "viral_remake"
	BusinessGamePreroll          = "game_preroll"
	BusinessBrandVideo           = "brand_video"
	BusinessWechatArticle        = "wechat_official_article"
)

type TaskStrategyReference struct {
	PlanID              string `json:"plan_id"`
	StrategyVersion     int64  `json:"strategy_version"`
	ExpectedContentHash string `json:"expected_content_hash"`
}

func (r TaskStrategyReference) Validate() error {
	if strings.TrimSpace(r.PlanID) == "" || r.StrategyVersion < 1 ||
		strings.TrimSpace(r.ExpectedContentHash) == "" {
		return fmt.Errorf("task_strategy plan_id, strategy_version, and expected_content_hash are required")
	}
	return nil
}

type TaskStrategyAudience struct {
	Primary  string   `json:"primary"`
	Insights []string `json:"insights"`
}

type TaskStrategyMediaItem struct {
	AssetRef        contract.AssetVersionRef `json:"asset_ref"`
	Role            string                   `json:"role"`
	Kind            string                   `json:"kind,omitempty"`
	MIMEType        string                   `json:"mime_type,omitempty"`
	Status          string                   `json:"status"`
	Usefulness      string                   `json:"usefulness"`
	StrategyUses    []string                 `json:"strategy_uses"`
	Observations    []string                 `json:"observations"`
	Limitations     []string                 `json:"limitations"`
	WidthPixels     int                      `json:"width_pixels,omitempty"`
	HeightPixels    int                      `json:"height_pixels,omitempty"`
	DurationSeconds float64                  `json:"duration_seconds,omitempty"`
}

type TaskStrategyReferenceUse struct {
	Locator      string   `json:"locator,omitempty"`
	RightsStatus string   `json:"rights_status"`
	IntendedUse  string   `json:"intended_use"`
	Warnings     []string `json:"warnings"`
}

type TaskStrategyLineage struct {
	BriefID                string `json:"brief_id"`
	BriefVersion           int64  `json:"brief_version"`
	BriefContentHash       string `json:"brief_content_hash"`
	SourceStrategyID       string `json:"source_strategy_id,omitempty"`
	SourceStrategyRevision int64  `json:"source_strategy_revision,omitempty"`
	SourceStrategyHash     string `json:"source_strategy_content_hash,omitempty"`
	BusinessGeneration     int64  `json:"business_generation"`
	BusinessVersion        string `json:"business_version"`
	BusinessContentHash    string `json:"business_content_hash"`
	SkillName              string `json:"skill_name"`
	SkillVersion           string `json:"skill_version"`
	SkillContentHash       string `json:"skill_content_hash"`
	PromptVersion          string `json:"prompt_version"`
	ProjectContextVersion  int64  `json:"project_context_version"`
}

// TaskStrategyInput is Creative's frozen projection of a Strategy-owned
// version. It is produced by TaskStrategyReader and is never trusted from an
// HTTP caller.
type TaskStrategyInput struct {
	ContractVersion   string                   `json:"contract_version"`
	BusinessCode      string                   `json:"business_code"`
	BusinessStrategy  map[string]any           `json:"business_strategy"`
	MessageHierarchy  []string                 `json:"message_hierarchy"`
	Audience          TaskStrategyAudience     `json:"audience"`
	ClaimsAndEvidence []string                 `json:"claims_and_evidence"`
	Guardrails        []string                 `json:"guardrails"`
	Media             []TaskStrategyMediaItem  `json:"media"`
	ReferenceUse      TaskStrategyReferenceUse `json:"reference_use"`
	OpenQuestions     []string                 `json:"open_questions"`
	Lineage           TaskStrategyLineage      `json:"lineage"`
}

type TaskStrategySnapshot struct {
	PlanID            string
	StrategyVersion   int64
	ContentHash       string
	BusinessCode      string
	Objective         string
	Audience          TaskStrategyAudience
	CoreMessage       string
	CallToAction      string
	Concept           string
	Tone              []string
	VisualKeywords    []string
	Mandatory         []string
	Prohibited        []string
	BusinessStrategy  map[string]any
	MessageHierarchy  []string
	ClaimsAndEvidence []string
	Guardrails        []string
	Media             []TaskStrategyMediaItem
	ReferenceUse      TaskStrategyReferenceUse
	OpenQuestions     []string
	Lineage           TaskStrategyLineage
}

type CreativeBusinessCapability struct {
	BusinessCode             string          `json:"business_code"`
	DisplayName              string          `json:"display_name"`
	Status                   string          `json:"status"`
	Format                   CreativeFormat  `json:"format,omitempty"`
	Channel                  CreativeChannel `json:"channel,omitempty"`
	PerformanceMode          string          `json:"performance_mode,omitempty"`
	DestinationArea          string          `json:"destination_area,omitempty"`
	DestinationView          string          `json:"destination_view,omitempty"`
	CanCreateTaskImmediately bool            `json:"can_create_task_immediately"`
	ProductionInputs         []string        `json:"production_inputs"`
	Limitation               string          `json:"limitation,omitempty"`
}

func CreativeBusinessCapabilities() []CreativeBusinessCapability {
	return []CreativeBusinessCapability{
		{
			BusinessCode: BusinessXiaohongshuImageText, DisplayName: "小红书图文",
			Status: "available", Format: FormatImageText, Channel: ChannelXiaohongshu,
			DestinationArea: "image-text", DestinationView: "小红书",
			CanCreateTaskImmediately: true,
			ProductionInputs:         []string{"确认内容焦点", "检查商业内容披露", "选择或生成配图"},
		},
		{
			BusinessCode: BusinessShortDramaPreroll, DisplayName: "短剧前贴",
			Status: "available", Format: FormatVideo, Channel: ChannelDouyin,
			PerformanceMode: PerformanceModeShortDramaPreroll,
			DestinationArea: "video", DestinationView: "效果广告",
			ProductionInputs: []string{"短剧剧情上下文", "角色参考素材", "人工选择候选方案", "生成前权利确认"},
		},
		{
			BusinessCode: BusinessCommercePreroll, DisplayName: "电商前贴",
			Status: "available", Format: FormatVideo, Channel: ChannelDouyin,
			PerformanceMode: "commerce_preroll",
			DestinationArea: "video", DestinationView: "效果广告",
			ProductionInputs: []string{"商品图片", "用于拼接的正片", "时长与画幅", "优惠事实有效性"},
		},
		{
			BusinessCode: BusinessViralRemake, DisplayName: "爆款复刻",
			Status: "available", Format: FormatVideo, Channel: ChannelDouyin,
			PerformanceMode: PerformanceModeViralRemake,
			DestinationArea: "video", DestinationView: "效果广告",
			ProductionInputs: []string{"参考视频 Asset", "具体生产用途", "下载或复用授权", "生成前原创性确认"},
		},
		{
			BusinessCode: BusinessGamePreroll, DisplayName: "游戏前贴",
			Status: "preview", Format: FormatVideo, Channel: ChannelDouyin,
			PerformanceMode:  "game_preroll",
			ProductionInputs: []string{"真实玩法素材", "角色与音乐权利", "渠道规格"},
			Limitation:       "Strategy 已可生成任务策略；Creative 当前只有演示工作区，尚未接入持久化生产链路。",
		},
		{
			BusinessCode: BusinessBrandVideo, DisplayName: "品牌广告",
			Status: "preview", Format: FormatVideo,
			ProductionInputs: []string{"品牌资产", "脚本与分镜", "人才和音乐权利", "审批人"},
			Limitation:       "Strategy 已可生成任务策略；Creative 当前只有方案演示，尚未接入稳定生成与交付链路。",
		},
		{
			BusinessCode: BusinessWechatArticle, DisplayName: "公众号文章",
			Status: "unsupported", Format: FormatImageText,
			ProductionInputs: []string{"来源引用", "头图与配图", "私域承接信息"},
			Limitation:       "Strategy 已可生成任务策略；Creative 尚无公众号文章的专属创作后端。",
		},
	}
}

func capabilityForBusiness(code string) (CreativeBusinessCapability, bool) {
	for _, capability := range CreativeBusinessCapabilities() {
		if capability.BusinessCode == strings.TrimSpace(code) {
			return capability, true
		}
	}
	return CreativeBusinessCapability{}, false
}

func resolvedTaskStrategyRequest(reference *TaskStrategyReference, snapshot TaskStrategySnapshot) (CreateIntakeRequest, error) {
	capability, found := capabilityForBusiness(snapshot.BusinessCode)
	if !found || capability.Status != "available" {
		return CreateIntakeRequest{}, fmt.Errorf("Creative business %q is not available for handoff", snapshot.BusinessCode)
	}
	concept := strings.TrimSpace(snapshot.Concept)
	if concept == "" {
		concept = strings.TrimSpace(snapshot.CoreMessage)
	}
	input := &TaskStrategyInput{
		ContractVersion: TaskStrategyContractVersion, BusinessCode: snapshot.BusinessCode,
		BusinessStrategy: cloneMap(snapshot.BusinessStrategy),
		MessageHierarchy: append([]string{}, snapshot.MessageHierarchy...),
		Audience: TaskStrategyAudience{
			Primary: snapshot.Audience.Primary, Insights: append([]string{}, snapshot.Audience.Insights...),
		},
		ClaimsAndEvidence: append([]string{}, snapshot.ClaimsAndEvidence...),
		Guardrails:        append([]string{}, snapshot.Guardrails...),
		Media:             append([]TaskStrategyMediaItem{}, snapshot.Media...),
		ReferenceUse: TaskStrategyReferenceUse{
			Locator: snapshot.ReferenceUse.Locator, RightsStatus: snapshot.ReferenceUse.RightsStatus,
			IntendedUse: snapshot.ReferenceUse.IntendedUse,
			Warnings:    append([]string{}, snapshot.ReferenceUse.Warnings...),
		},
		OpenQuestions: append([]string{}, snapshot.OpenQuestions...),
		Lineage:       snapshot.Lineage,
	}
	request := CreateIntakeRequest{
		Source: IntakeSourceTaskStrategy, TaskStrategy: reference, TaskStrategyInput: input,
		Format: capability.Format, PerformanceMode: capability.PerformanceMode, Channel: capability.Channel,
		Objective: snapshot.Objective, Audience: snapshot.Audience.Primary,
		CoreMessage: snapshot.CoreMessage, CallToAction: snapshot.CallToAction, Concept: concept,
		Tone: append([]string{}, snapshot.Tone...), VisualKeywords: append([]string{}, snapshot.VisualKeywords...),
		Mandatory: append([]string{}, snapshot.Mandatory...), Prohibited: append([]string{}, snapshot.Prohibited...),
	}
	if capability.Format == FormatVideo {
		request.CreativeRoutes = []CreativeRouteSnapshot{taskStrategyVideoRoute(snapshot, capability)}
	}
	return request, request.validateResolvedTaskStrategy()
}

func taskStrategyVideoRoute(snapshot TaskStrategySnapshot, capability CreativeBusinessCapability) CreativeRouteSnapshot {
	duration := 5
	routeType := "pre_roll"
	switch snapshot.BusinessCode {
	case BusinessShortDramaPreroll:
		duration, routeType = 6, PerformanceModeShortDramaPreroll
	case BusinessViralRemake:
		duration, routeType = 15, PerformanceModeViralRemake
	}
	sourceRefs := make([]contract.AssetVersionRef, 0, len(snapshot.Media))
	for _, item := range snapshot.Media {
		if item.Status == "ready" && item.Kind == "video" && item.AssetRef.Validate() == nil {
			sourceRefs = append(sourceRefs, item.AssetRef)
		}
	}
	return CreativeRouteSnapshot{
		RouteID:   "route_task_strategy_" + snapshot.BusinessCode + "_v1",
		RouteType: routeType, VideoPurpose: "performance", Channels: []string{string(capability.Channel)},
		Reason:                "用户从冻结的创意任务策略显式进入 Creative 工作台",
		TargetDurationSeconds: duration, AspectRatio: "9:16", SourceAssetRefs: sourceRefs,
		EvidenceRefs:              append([]string{}, snapshot.ClaimsAndEvidence...),
		RequiresHumanConfirmation: true,
	}
}

func (r CreateIntakeRequest) validateResolvedTaskStrategy() error {
	if r.Source != IntakeSourceTaskStrategy || r.TaskStrategy == nil || r.TaskStrategyInput == nil {
		return fmt.Errorf("resolved task strategy intake is incomplete")
	}
	if r.TaskStrategyInput.ContractVersion != TaskStrategyContractVersion {
		return fmt.Errorf("unsupported task strategy contract %q", r.TaskStrategyInput.ContractVersion)
	}
	capability, found := capabilityForBusiness(r.TaskStrategyInput.BusinessCode)
	if !found || capability.Status != "available" ||
		r.Format != capability.Format || r.Channel != capability.Channel ||
		r.PerformanceMode != capability.PerformanceMode {
		return fmt.Errorf("resolved task strategy does not match the Creative capability")
	}
	if capability.Format == FormatImageText {
		return r.validateContent()
	}
	if err := r.validateVideoContent(); err != nil {
		return err
	}
	if len(r.CreativeRoutes) != 1 {
		return fmt.Errorf("resolved video task strategy requires one Creative route")
	}
	return r.CreativeRoutes[0].Validate()
}

func cloneMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}
