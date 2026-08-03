package creative

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

const (
	brandBriefPromptVersion   = "brand-brief-analysis/v1"
	brandConceptPromptVersion = "brand-concept-set/v1"
	brandFilmPromptVersion    = "brand-film-plan/v1"
)

type BrandFilmTextGenerator interface {
	GenerateText(context.Context, provider.TextGenerateRequest) (provider.SynchronousResponse, error)
}

type BrandFilmPlanner interface {
	AnalyzeBrief(context.Context, contract.ActorContext, contract.ProjectContext, BrandFilmSourceSnapshot, int64, time.Time) (BrandBriefAnalysisVersion, error)
	GenerateConcepts(context.Context, contract.ActorContext, contract.ProjectContext, BrandFilmSourceSnapshot, BrandBriefAnalysisVersion, int64, time.Time) (BrandCreativeConceptSet, error)
	GenerateFilmPlan(context.Context, contract.ActorContext, contract.ProjectContext, BrandFilmSourceSnapshot, BrandBriefAnalysisVersion, BrandCreativeConcept, int64, time.Time) (BrandFilmPlanVersion, error)
}

type DeterministicBrandFilmPlanner struct{}

func (DeterministicBrandFilmPlanner) AnalyzeBrief(_ context.Context, _ contract.ActorContext, _ contract.ProjectContext, _ BrandFilmSourceSnapshot, revision int64, now time.Time) (BrandBriefAnalysisVersion, error) {
	return BrandBriefAnalysisVersion{
		Revision:    revision,
		Summary:     "娇兰 25X 蜂皇水面向关注补水修护与轻盈肤感的人群，以水感、蜂巢与暖金光影建立高端品牌记忆。",
		Audience:    "关注补水、修护、屏障护理与轻盈肤感的都市护肤人群。",
		CoreMessage: "轻盈如水的使用体验，承载娇兰黑蜂修护科技与高端品牌质感。",
		SellingPoints: []BrandBriefFact{
			{Text: "精华水质地轻盈不黏腻", Locator: "fixture://briefs/guerlain-25x-bee-water-v1#pages=8-12", Confidence: 0.96, Status: "brief_fact"},
			{Text: "强调补水、修护与屏障护理", Locator: "fixture://briefs/guerlain-25x-bee-water-v1#pages=8-12", Confidence: 0.94, Status: "needs_confirmation"},
			{Text: "含微囊蜂王浆与黑蜂修护科技", Locator: "fixture://briefs/guerlain-25x-bee-water-v1#pages=8-12", Confidence: 0.92, Status: "brief_fact"},
			{Text: "适合湿敷与日常全脸护理", Locator: "fixture://briefs/guerlain-25x-bee-water-v1#pages=8-12", Confidence: 0.9, Status: "brief_fact"},
		},
		Mandatory:         []string{"商品瓶型、颜色、标签与比例保持真实", "书面使用“25X蜂皇水”，口播使用“二十五倍蜂皇水”", "包含湿敷或全脸使用方式", "功效表达绑定 Brief 事实"},
		Prohibited:        []string{"不得编造医学或绝对化功效", "不得生成错误 Logo、包装文字、价格或促销信息", "不得混入其他娇兰产品卖点"},
		ImageRequirements: []string{"优先使用无水印商品正面图", "保留琥珀金瓶身、黑色瓶盖和原始标签比例", "Logo 与包装文字进入生产前必须人工确认"},
		VideoRequirements: []string{"抖音 9:16，15 秒", "使用水感微距、暖金蜂巢与克制运镜", "结尾至少保留 2 秒稳定商品定格"},
		VoiceDirection:    "温柔、克制的年轻成熟女声，中低语速，品牌名与“二十五倍蜂皇水”咬字清晰。",
		AssetCandidates: []BrandBriefAssetCandidate{
			{ID: "asset_product_front", Role: "product_front", Label: "25X 蜂皇水正面图", SourceLocator: "fixture://briefs/guerlain-25x-bee-water-v1#product-front", FixtureURI: "/assets/guerlain-25x-bee-water.png", RightsStatus: "needs_confirmation"},
			{ID: "asset_brand_logo", Role: "logo", Label: "娇兰 Logo", SourceLocator: "fixture://briefs/guerlain-25x-bee-water-v1#logo", RightsStatus: "needs_confirmation"},
		},
		Uncertainties: []string{"98% 天然来源成分的传播脚注与适用范围需要人工确认", "统一口播音色的 Voice ID 与授权尚未提供"},
		ModelAlias:    "fixture.deterministic", ModelVersion: "guerlain-brand-v1", PromptVersion: brandBriefPromptVersion, CreatedAt: now,
	}, nil
}

func (DeterministicBrandFilmPlanner) GenerateConcepts(_ context.Context, _ contract.ActorContext, _ contract.ProjectContext, _ BrandFilmSourceSnapshot, analysis BrandBriefAnalysisVersion, revision int64, now time.Time) (BrandCreativeConceptSet, error) {
	return BrandCreativeConceptSet{
		Revision: revision, AnalysisRevision: analysis.Revision,
		Candidates: []BrandCreativeConcept{
			{ID: "concept_hive_awaken", Title: "蜂巢苏醒", OneLiner: "一滴水唤醒金色蜂巢，让修护能量汇入肌肤。", StoryMechanism: "从微观蜂巢能量进入产品，再落到真实湿敷动作。", BrandEntrance: "第 5 秒由蜂巢光影折射出产品正面。", VisualLanguage: []string{"暖金蜂巢", "水滴微距", "琥珀瓶身"}, SoundIdea: "细微水滴与低频弦乐渐起", BriefRationale: "对应黑蜂科技、水感质地与湿敷使用方式。", Risk: "蜂巢元素不可盖过商品识别。"},
			{ID: "concept_light_on_skin", Title: "晨光入肤", OneLiner: "清晨的一层轻盈水光，让肌肤与一天同时苏醒。", StoryMechanism: "以都市清晨的时间推进展示轻盈、湿敷与稳定光泽。", BrandEntrance: "产品从晨光中的梳妆台自然进入人物动作。", VisualLanguage: []string{"柔和晨光", "肌肤水光", "奢华留白"}, SoundIdea: "呼吸感钢琴与轻柔环境声", BriefRationale: "服务轻盈肤感和日常护理场景。", Risk: "人物肌肤效果不得被表达成即时医学功效。"},
			{ID: "concept_water_sculpture", Title: "水之雕刻", OneLiner: "流动的水被金色光线雕刻成娇兰瓶身。", StoryMechanism: "用抽象水体逐步显形为产品，最后回到真实使用。", BrandEntrance: "产品是视觉变化的结果而非后置贴片。", VisualLanguage: []string{"水体雕塑", "金色折射", "黑色高光"}, SoundIdea: "水流声与克制电子脉冲", BriefRationale: "把精华水的轻盈质感转译为高级视觉记忆。", Risk: "生成阶段需严控瓶身变形与标签错误。"},
		},
		ModelAlias: "fixture.deterministic", ModelVersion: "guerlain-brand-v1", PromptVersion: brandConceptPromptVersion, CreatedAt: now,
	}, nil
}

func (DeterministicBrandFilmPlanner) GenerateFilmPlan(_ context.Context, _ contract.ActorContext, _ contract.ProjectContext, _ BrandFilmSourceSnapshot, _ BrandBriefAnalysisVersion, concept BrandCreativeConcept, revision int64, now time.Time) (BrandFilmPlanVersion, error) {
	shots := []BrandFilmShot{
		{ID: "shot_01", Order: 1, StartSecond: 0, EndSecond: 3, Purpose: "建立品牌世界", Visual: "暗金背景中一滴清水落入蜂巢纹理，光线沿六边形缓慢苏醒。", Action: "水滴扩散为柔和金色波纹。", Camera: "微距缓慢推进", Lighting: "低照度暖金轮廓光", Voiceover: "当自然的修护能量，被轻盈唤醒。", ReferenceRole: "style", ContinuityNotes: "金色温度与后续产品高光一致。"},
		{ID: "shot_02", Order: 2, StartSecond: 3, EndSecond: 6, Purpose: "完成品牌进入", Visual: "蜂巢光影汇聚，娇兰 25X 蜂皇水瓶身清晰出现。", Action: "瓶身从水雾中稳定显形并保持标签正面。", Camera: "中近景轻微环绕", Lighting: "暖金侧光与黑色轮廓高光", Voiceover: "娇兰二十五倍蜂皇水。", OnScreenText: "娇兰 25X蜂皇水", ReferenceRole: "required_identity", ContinuityNotes: "瓶型、黑色瓶盖、标签比例必须保持真实。"},
		{ID: "shot_03", Order: 3, StartSecond: 6, EndSecond: 10, Purpose: "展示质地与使用", Visual: "轻盈水感掠过肌肤，人物将浸润化妆棉贴于面颊。", Action: "一次自然湿敷动作，不做前后对比。", Camera: "水感微距切至面部近景", Lighting: "柔和晨光与金色反射", Voiceover: "轻盈不黏腻，温柔补水修护。", ReferenceRole: "composition", ContinuityNotes: "人物、肤色与产品方向连续。"},
		{ID: "shot_04", Order: 4, StartSecond: 10, EndSecond: 13, Purpose: "沉淀感受", Visual: "人物取下化妆棉，水光与金色波纹在肌肤边缘自然呼应。", Action: "轻微转头，神态放松。", Camera: "固定近景", Lighting: "干净柔光", Voiceover: "让每一次护理，都回到柔润与从容。", ReferenceRole: "style", ContinuityNotes: "不表现夸张即时功效。"},
		{ID: "shot_05", Order: 5, StartSecond: 13, EndSecond: 15, Purpose: "品牌定格", Visual: "产品正面在暖金蜂巢背景前稳定定格，品牌留白清晰。", Action: "仅保留细微光线流动。", Camera: "固定商品英雄镜头", Lighting: "高端暖金主光", Voiceover: "法国娇兰。", OnScreenText: "法国娇兰 · 25X蜂皇水", ReferenceRole: "required_identity", ContinuityNotes: "不新增价格、促销或错误文字。"},
	}
	return BrandFilmPlanVersion{
		Revision: revision, ConceptID: concept.ID, Title: "《" + concept.Title + "》", StorySummary: concept.OneLiner,
		VoiceDirection: "温柔、克制的年轻成熟女声，中低语速。", MusicDirection: concept.SoundIdea, Shots: shots,
		ModelAlias: "fixture.deterministic", ModelVersion: "guerlain-brand-v1", PromptVersion: brandFilmPromptVersion, CreatedAt: now,
	}, nil
}

type ModelBrandFilmPlanner struct {
	Text       BrandFilmTextGenerator
	ModelAlias string
}

type FallbackBrandFilmPlanner struct {
	Primary          BrandFilmPlanner
	Fallback         BrandFilmPlanner
	OnPrimaryFailure func(error)
}

func (p FallbackBrandFilmPlanner) AnalyzeBrief(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, source BrandFilmSourceSnapshot, revision int64, now time.Time) (BrandBriefAnalysisVersion, error) {
	value, err := p.Primary.AnalyzeBrief(ctx, actor, project, source, revision, now)
	if err == nil {
		return value, nil
	}
	if p.OnPrimaryFailure != nil {
		p.OnPrimaryFailure(err)
	}
	return p.Fallback.AnalyzeBrief(ctx, actor, project, source, revision, now)
}

func (p FallbackBrandFilmPlanner) GenerateConcepts(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, source BrandFilmSourceSnapshot, analysis BrandBriefAnalysisVersion, revision int64, now time.Time) (BrandCreativeConceptSet, error) {
	value, err := p.Primary.GenerateConcepts(ctx, actor, project, source, analysis, revision, now)
	if err == nil {
		return value, nil
	}
	if p.OnPrimaryFailure != nil {
		p.OnPrimaryFailure(err)
	}
	return p.Fallback.GenerateConcepts(ctx, actor, project, source, analysis, revision, now)
}

func (p FallbackBrandFilmPlanner) GenerateFilmPlan(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, source BrandFilmSourceSnapshot, analysis BrandBriefAnalysisVersion, concept BrandCreativeConcept, revision int64, now time.Time) (BrandFilmPlanVersion, error) {
	value, err := p.Primary.GenerateFilmPlan(ctx, actor, project, source, analysis, concept, revision, now)
	if err == nil {
		return value, nil
	}
	if p.OnPrimaryFailure != nil {
		p.OnPrimaryFailure(err)
	}
	return p.Fallback.GenerateFilmPlan(ctx, actor, project, source, analysis, concept, revision, now)
}

func brandPlannerActor(actor contract.ActorContext) contract.ActorContext {
	return contract.ActorContext{OrganizationID: actor.OrganizationID, Principal: actor.Principal, Scopes: []contract.Scope{provider.ScopeTextGenerate}}
}

func (p ModelBrandFilmPlanner) AnalyzeBrief(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, source BrandFilmSourceSnapshot, revision int64, now time.Time) (BrandBriefAnalysisVersion, error) {
	if p.Text == nil || strings.TrimSpace(p.ModelAlias) == "" {
		return BrandBriefAnalysisVersion{}, fmt.Errorf("brand film model planner is not configured")
	}
	raw, err := json.Marshal(source)
	if err != nil {
		return BrandBriefAnalysisVersion{}, err
	}
	response, err := p.Text.GenerateText(ctx, provider.TextGenerateRequest{
		Actor: brandPlannerActor(actor), Project: project, ModelAlias: p.ModelAlias,
		InvocationKey: contract.IdempotencyKey(fmt.Sprintf("brand-brief-%s-%d", source.FixtureHash[7:19], revision)),
		Messages: []provider.TextMessage{
			{Role: provider.TextRoleSystem, Content: "你是品牌广告 Brief 分析师。只分析输入中的娇兰 25X 蜂皇水，不混入其他产品。只输出 JSON。事实必须保留 locator；不确定功效标记 needs_confirmation。"},
			{Role: provider.TextRoleUser, Content: "请提炼摘要、受众、核心信息、卖点、必须项、禁用项、图片/视频要求、统一口播方向和不确定项。INPUT=" + string(raw)},
		}, OutputJSONSchema: brandBriefAnalysisSchema,
	})
	if err != nil {
		return BrandBriefAnalysisVersion{}, err
	}
	var value BrandBriefAnalysisVersion
	if err := decodeBrandStructured(response, &value); err != nil {
		return BrandBriefAnalysisVersion{}, err
	}
	value.Revision, value.Confirmed, value.ConfirmedAt = revision, false, nil
	value.ModelAlias, value.ModelVersion, value.RouteRevisionID = p.ModelAlias, response.ModelVersion, response.RouteRevisionID
	value.PromptVersion, value.CreatedAt = brandBriefPromptVersion, now
	if len(value.AssetCandidates) == 0 {
		fallback, _ := (DeterministicBrandFilmPlanner{}).AnalyzeBrief(ctx, actor, project, source, revision, now)
		value.AssetCandidates = fallback.AssetCandidates
	}
	return value, value.Validate()
}

func (p ModelBrandFilmPlanner) GenerateConcepts(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, source BrandFilmSourceSnapshot, analysis BrandBriefAnalysisVersion, revision int64, now time.Time) (BrandCreativeConceptSet, error) {
	input, _ := json.Marshal(struct {
		Source   BrandFilmSourceSnapshot   `json:"source"`
		Analysis BrandBriefAnalysisVersion `json:"analysis"`
	}{source, analysis})
	response, err := p.Text.GenerateText(ctx, provider.TextGenerateRequest{
		Actor: brandPlannerActor(actor), Project: project, ModelAlias: p.ModelAlias,
		InvocationKey: contract.IdempotencyKey(fmt.Sprintf("brand-concepts-%s-%d", source.FixtureHash[7:19], revision)),
		Messages: []provider.TextMessage{
			{Role: provider.TextRoleSystem, Content: "你是高端美妆品牌广告创意总监。输出 3 个叙事机制明显不同的 15 秒竖屏方向，不生成镜头表，不编造 Brief 事实，只输出 JSON。"},
			{Role: provider.TextRoleUser, Content: "根据已确认 Brief 生成创意方向。INPUT=" + string(input)},
		}, OutputJSONSchema: brandConceptSetSchema,
	})
	if err != nil {
		return BrandCreativeConceptSet{}, err
	}
	var value BrandCreativeConceptSet
	if err := decodeBrandStructured(response, &value); err != nil {
		return BrandCreativeConceptSet{}, err
	}
	value.Revision, value.AnalysisRevision = revision, analysis.Revision
	for index := range value.Candidates {
		value.Candidates[index].ID = fmt.Sprintf("concept_%02d", index+1)
		value.Candidates[index].Selected, value.Candidates[index].Confirmed = false, false
	}
	value.ModelAlias, value.ModelVersion, value.RouteRevisionID = p.ModelAlias, response.ModelVersion, response.RouteRevisionID
	value.PromptVersion, value.CreatedAt = brandConceptPromptVersion, now
	return value, value.Validate()
}

func (p ModelBrandFilmPlanner) GenerateFilmPlan(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, source BrandFilmSourceSnapshot, analysis BrandBriefAnalysisVersion, concept BrandCreativeConcept, revision int64, now time.Time) (BrandFilmPlanVersion, error) {
	input, _ := json.Marshal(struct {
		Source   BrandFilmSourceSnapshot   `json:"source"`
		Analysis BrandBriefAnalysisVersion `json:"analysis"`
		Concept  BrandCreativeConcept      `json:"concept"`
	}{source, analysis, concept})
	response, err := p.Text.GenerateText(ctx, provider.TextGenerateRequest{
		Actor: brandPlannerActor(actor), Project: project, ModelAlias: p.ModelAlias,
		InvocationKey: contract.IdempotencyKey(fmt.Sprintf("brand-film-plan-%s-%d", source.FixtureHash[7:19], revision)),
		Messages: []provider.TextMessage{
			{Role: provider.TextRoleSystem, Content: "你是品牌广告导演。输出可编辑的 15 秒、9:16 剧本分镜。镜头必须从 0 秒连续覆盖到 15 秒，至少 3 个；用户编辑镜头表而不是模型 Prompt。只输出 JSON。"},
			{Role: provider.TextRoleUser, Content: "基于已确认 Brief 与创意方向生成剧本、旁白和镜头表。INPUT=" + string(input)},
		}, OutputJSONSchema: brandFilmPlanSchema,
	})
	if err != nil {
		return BrandFilmPlanVersion{}, err
	}
	var value BrandFilmPlanVersion
	if err := decodeBrandStructured(response, &value); err != nil {
		return BrandFilmPlanVersion{}, err
	}
	value.Revision, value.ConceptID, value.Confirmed, value.ConfirmedAt = revision, concept.ID, false, nil
	for index := range value.Shots {
		value.Shots[index].ID, value.Shots[index].Order = fmt.Sprintf("shot_%02d", index+1), index+1
	}
	value.ModelAlias, value.ModelVersion, value.RouteRevisionID = p.ModelAlias, response.ModelVersion, response.RouteRevisionID
	value.PromptVersion, value.CreatedAt = brandFilmPromptVersion, now
	return value, value.Validate()
}

func decodeBrandStructured(response provider.SynchronousResponse, target any) error {
	raw := response.StructuredOutput
	if len(raw) == 0 {
		raw = json.RawMessage(response.Text)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode brand film planner output: %w", err)
	}
	return nil
}

var brandBriefAnalysisSchema = json.RawMessage(`{
  "type":"object","additionalProperties":false,
  "required":["summary","audience","core_message","selling_points","mandatory_elements","prohibited_claims","image_requirements","video_requirements","voice_direction","uncertainties"],
  "properties":{
    "summary":{"type":"string"},"audience":{"type":"string"},"core_message":{"type":"string"},
    "selling_points":{"type":"array","minItems":3,"items":{"type":"object","additionalProperties":false,"required":["text","locator","confidence","status"],"properties":{"text":{"type":"string"},"locator":{"type":"string"},"confidence":{"type":"number","minimum":0,"maximum":1},"status":{"enum":["brief_fact","needs_confirmation"]}}}},
    "mandatory_elements":{"type":"array","minItems":1,"items":{"type":"string"}},"prohibited_claims":{"type":"array","minItems":1,"items":{"type":"string"}},
    "image_requirements":{"type":"array","items":{"type":"string"}},"video_requirements":{"type":"array","items":{"type":"string"}},"voice_direction":{"type":"string"},"uncertainties":{"type":"array","items":{"type":"string"}}
  }
}`)

var brandConceptSetSchema = json.RawMessage(`{
  "type":"object","additionalProperties":false,"required":["candidates"],
  "properties":{"candidates":{"type":"array","minItems":3,"maxItems":3,"items":{"type":"object","additionalProperties":false,"required":["title","one_liner","story_mechanism","brand_entrance","visual_language","sound_idea","brief_rationale","risk"],"properties":{"title":{"type":"string"},"one_liner":{"type":"string"},"story_mechanism":{"type":"string"},"brand_entrance":{"type":"string"},"visual_language":{"type":"array","items":{"type":"string"}},"sound_idea":{"type":"string"},"brief_rationale":{"type":"string"},"risk":{"type":"string"}}}}}
}`)

var brandFilmPlanSchema = json.RawMessage(`{
  "type":"object","additionalProperties":false,"required":["title","story_summary","voice_direction","music_direction","shots"],
  "properties":{"title":{"type":"string"},"story_summary":{"type":"string"},"voice_direction":{"type":"string"},"music_direction":{"type":"string"},"shots":{"type":"array","minItems":3,"items":{"type":"object","additionalProperties":false,"required":["start_second","end_second","purpose","visual","action","camera","lighting","voiceover","on_screen_text","reference_role","continuity_notes"],"properties":{"start_second":{"type":"integer","minimum":0},"end_second":{"type":"integer","minimum":1,"maximum":15},"purpose":{"type":"string"},"visual":{"type":"string"},"action":{"type":"string"},"camera":{"type":"string"},"lighting":{"type":"string"},"voiceover":{"type":"string"},"on_screen_text":{"type":"string"},"reference_role":{"type":"string"},"continuity_notes":{"type":"string"}}}}}
}`)
