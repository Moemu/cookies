package strategy

import (
	"context"
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/knowledge"
	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/systems/strategy/promptkit"
	strategyskills "github.com/shikanon/cookies/internal/systems/strategy/skills"
)

func TestEvidenceFromBriefPreservesConfirmationBoundary(t *testing.T) {
	t.Parallel()
	brief := BriefVersion{
		Snapshot: BriefDocument{
			ContractVersion: "strategy-brief-version/v1",
			Campaign:        BriefCampaign{Objective: "新品认知"},
			Audience:        BriefAudience{Primary: "研发负责人"},
			Proposition:     "缩短研发周期",
			Channels:        []string{"xiaohongshu"},
		},
		FieldStates: map[string]FieldState{
			"campaign.objective": {Confirmation: "confirmed", Confidence: "high"},
			"audience.primary":   {Confirmation: "confirmed", Confidence: "high"},
			"proposition":        {Confirmation: "unconfirmed", Confidence: "medium"},
			"channels":           {Confirmation: "confirmed", Confidence: "high"},
		},
	}
	values := evidenceFromBrief(brief)
	if len(values) != 4 {
		t.Fatalf("evidence count = %d, want 4", len(values))
	}
	if values[2].FieldPath != "proposition" || values[2].Confirmed {
		t.Fatalf("proposition evidence = %#v", values[2])
	}
}

func TestStrategyQualityRejectsSuperficiallyCompleteDocument(t *testing.T) {
	t.Parallel()
	document := StrategyDocument{
		ContractVersion: "strategy-draft/v1", Objective: "认知",
		Audience:        StrategyAudience{Primary: "研发负责人"},
		Proposition:     "提效",
		ChannelStrategy: []ChannelStrategy{{Platform: "xiaohongshu", Role: "种草", Formats: []string{"图文"}}},
		Lineage:         StrategyLineage{BriefID: "brief_1", BriefVersion: 1, ProjectContextVersion: 1},
	}
	report := evaluateStrategyQuality(document, GenerationContext{
		Project: contract.ProjectContext{ProjectContextVersion: 1},
	})
	if report.Passed || report.Score >= 100 {
		t.Fatalf("quality report = %#v", report)
	}
}

func TestStrategyQualityRejectsConfirmedBriefDrift(t *testing.T) {
	t.Parallel()
	generation := GenerationContext{Brief: BriefVersion{Snapshot: BriefDocument{
		Campaign:    BriefCampaign{Objective: "新品认知"},
		Audience:    BriefAudience{Primary: "制造企业研发负责人"},
		Proposition: "缩短研发决策周期",
		Channels:    []string{"xiaohongshu"},
		Measurement: BriefMeasurement{PrimaryKPI: "高意向咨询数"},
	}}}
	document := StrategyDocument{
		ContractVersion: "strategy-draft/v1",
		Objective:       "电商成交",
		Audience: StrategyAudience{
			Primary:  "泛消费人群",
			Insights: []string{"需要对比真实使用场景"},
		},
		Proposition: "低价促销",
		ChannelStrategy: []ChannelStrategy{{
			Platform: "xiaohongshu", Role: "决策辅助", Formats: []string{"图文"},
		}},
		CreativeRecommendations: []string{"场景问题开篇", "证据清单正文", "评论区承接咨询"},
		ExperimentMatrix:        []Experiment{{Hypothesis: "证据清单提高咨询", Variable: "正文结构", Metric: "高意向咨询数"}},
		Measurement:             []string{"高意向咨询数"},
		Lineage: StrategyLineage{
			BriefID: "brief_1", BriefVersion: 1, ProjectContextVersion: 1,
		},
	}
	report := evaluateStrategyQuality(document, generation)
	if report.Passed || len(report.Errors) < 3 {
		t.Fatalf("quality report = %#v", report)
	}
}

func TestNormalizeGeneratedStrategyRepairsCommonModelDriftLocally(t *testing.T) {
	t.Parallel()
	brief := BriefVersion{
		BriefID: "brief_1",
		Version: 1,
		Snapshot: BriefDocument{
			Campaign:    BriefCampaign{Objective: "获取有效销售线索"},
			Audience:    BriefAudience{Primary: "制造企业质量负责人"},
			Proposition: "无需改造产线即可部署",
			Channels:    []string{"xiaohongshu"},
			Budget:      BriefBudget{Total: "30万元"},
			Measurement: BriefMeasurement{PrimaryKPI: "40条有效销售线索"},
		},
	}
	document := StrategyDocument{
		ContractVersion: "strategy-draft/v1",
		Objective:       "提升品牌影响力",
		Audience:        StrategyAudience{Primary: "泛制造业人群"},
		Proposition:     "智能检测",
		ChannelStrategy: []ChannelStrategy{{
			Platform: "小红书图文", Role: "决策辅助", Formats: []string{"图文"},
		}},
		CreativeRecommendations: []string{"展示真实产线场景"},
		ExperimentMatrix: []Experiment{{
			Hypothesis: "", Variable: "首图", Metric: "点击率",
		}},
		Lineage: StrategyLineage{
			BriefID: "brief_1", BriefVersion: 1, ProjectContextVersion: 1,
		},
	}

	normalizeGeneratedStrategy(&document, brief, Draft{ProjectContextVersion: 1})

	if document.Objective != brief.Snapshot.Campaign.Objective ||
		document.Audience.Primary != brief.Snapshot.Audience.Primary ||
		document.Proposition != brief.Snapshot.Proposition {
		t.Fatalf("confirmed fields were not anchored to brief: %#v", document)
	}
	if document.ChannelStrategy[0].Platform != "xiaohongshu" {
		t.Fatalf("channel = %q", document.ChannelStrategy[0].Platform)
	}
	if len(document.Audience.Insights) == 0 || len(document.CreativeRecommendations) < 3 ||
		len(document.ExperimentMatrix) == 0 || document.Measurement[0] != "40条有效销售线索" {
		t.Fatalf("local repair incomplete: %#v", document)
	}
}

func TestStrategyUserPromptOmitsDuplicatedConversationAndSkills(t *testing.T) {
	t.Parallel()
	prompt := strategyUserPrompt(GenerationContext{
		ContractVersion: "strategy-generation-context/v2",
		Project:         contract.ProjectContext{ProjectContextVersion: 3},
		Evidence:        []EvidenceItem{{FieldPath: "campaign.objective", Value: "获取线索", Confirmed: true}},
		Documents:       []KnowledgeExcerpt{{ID: "doc_1", Kind: "document", Title: "产品资料.md", Content: "产品事实证据"}},
		Conversation:    []ConversationExcerpt{{Role: "user", Content: "不应重复进入策略提示词"}},
		Skills:          []strategyskills.Snapshot{{Name: "不应重复的 Skill"}},
		PromptVersion:   "strategy.generate.v2",
	})
	if strings.Contains(prompt, "不应重复进入策略提示词") || strings.Contains(prompt, "不应重复的 Skill") {
		t.Fatalf("prompt still contains duplicated context: %s", prompt)
	}
	if !strings.Contains(prompt, "获取线索") || !strings.Contains(prompt, `"project_context_version":3`) {
		t.Fatalf("prompt omitted required context: %s", prompt)
	}
	if !strings.Contains(prompt, "产品事实证据") || !strings.Contains(prompt, `"id":"doc_1"`) {
		t.Fatalf("prompt omitted referenced document: %s", prompt)
	}
}

func TestStrategyUserPromptV3SeparatesConfirmedFactsAndAssumptions(t *testing.T) {
	t.Parallel()
	prompt := strategyUserPrompt(GenerationContext{
		ContractVersion: "strategy-generation-context/v2",
		Brief:           BriefVersion{Snapshot: BriefDocument{Constraints: []string{"不得承诺疗效"}}},
		Evidence: []EvidenceItem{
			{FieldPath: "campaign.objective", Value: "获取线索", Confirmed: true},
			{FieldPath: "budget.total", Value: "待确认预算", Confirmed: false},
		},
		PromptVersion: promptkit.GenerateV3,
	})
	for _, expected := range []string{`"confirmed_facts"`, `"open_assumptions"`, "不得承诺疗效", "获取线索", "待确认预算"} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt omitted %q: %s", expected, prompt)
		}
	}
}

func TestGenerationDocumentsResolvesDocumentsAndResearchArtifacts(t *testing.T) {
	t.Parallel()
	service := Service{Knowledge: stubKnowledgeReader{values: map[string]knowledge.Reference{
		"doc_1": {
			ID: "doc_1", Kind: "document", Title: "产品资料.md",
			Content: "产品能力说明", ContentHash: strings.Repeat("a", 64),
		},
		"artifact_1": {
			ID: "artifact_1", Kind: "research_artifact", Title: "行业案例",
			Content: "行业研究结论", ContentHash: strings.Repeat("b", 64),
			Citations: []string{"https://example.test/source"},
		},
	}}}
	values, err := service.generationDocuments(
		context.Background(),
		contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}},
		"project_1",
		[]string{"doc_1", "artifact_1"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || values[0].Kind != "document" || values[1].Kind != "research_artifact" ||
		len(values[1].Citations) != 1 {
		t.Fatalf("knowledge excerpts = %#v", values)
	}
}

func TestContextSelectorKeepsCoverageAndRanksBriefRelevantChunks(t *testing.T) {
	t.Parallel()
	brief := BriefVersion{Snapshot: BriefDocument{
		Brand:       BriefBrand{Name: "轻氧"},
		Product:     BriefProduct{Name: "汽泡水", SellingPoints: []string{"零糖"}},
		Campaign:    BriefCampaign{Objective: "新品认知"},
		Audience:    BriefAudience{Primary: "健身人群"},
		Proposition: "零糖清爽",
	}}
	filler := strings.Repeat("普通行业背景资料。", 170)
	references := []KnowledgeExcerpt{
		{ID: "doc_1", Title: "产品手册", Content: filler + "\n# 产品证据\n轻氧汽泡水面向健身人群，核心卖点是零糖清爽。" + filler},
		{ID: "doc_2", Title: "渠道规范", Content: "所有新品认知内容都必须标注信息来源。"},
	}
	values := (DeterministicContextSelector{}).Select(brief, references)
	if len(values) < 2 || len(values) > contextMaxChunks {
		t.Fatalf("selected chunks = %d", len(values))
	}
	covered := map[string]bool{}
	foundRelevant := false
	for _, value := range values {
		covered[value.ReferenceID] = true
		if strings.Contains(value.Content, "轻氧汽泡水面向健身人群") {
			foundRelevant = true
			if value.RelevanceScore == 0 || value.StartLine < 1 || value.EndLine < value.StartLine {
				t.Fatalf("relevant chunk metadata = %#v", value)
			}
		}
	}
	if !covered["doc_1"] || !covered["doc_2"] || !foundRelevant {
		t.Fatalf("selected chunks = %#v", values)
	}
}

func TestGenerationDocumentsForBriefUsesSelectorOnlyWhenEnabled(t *testing.T) {
	t.Parallel()
	reference := knowledge.Reference{
		ID: "doc_1", Kind: "document", Title: "产品资料.md",
		Content:     strings.Repeat("背景资料。", 250) + "轻氧零糖汽泡水",
		ContentHash: strings.Repeat("a", 64),
	}
	actor := contract.ActorContext{
		OrganizationID: "org_1",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
	}
	brief := BriefVersion{Snapshot: BriefDocument{
		Product:      BriefProduct{Name: "轻氧零糖汽泡水"},
		ReferenceIDs: []string{"doc_1"},
	}}
	legacy := Service{Knowledge: stubKnowledgeReader{values: map[string]knowledge.Reference{"doc_1": reference}}}
	legacyValues, err := legacy.generationDocumentsForBrief(context.Background(), actor, "project_1", brief)
	if err != nil {
		t.Fatal(err)
	}
	selected := legacy
	selected.ContextSelectionEnabled = true
	selectedValues, err := selected.generationDocumentsForBrief(context.Background(), actor, "project_1", brief)
	if err != nil {
		t.Fatal(err)
	}
	if len(legacyValues) != 1 || legacyValues[0].ReferenceID != "" {
		t.Fatalf("legacy documents = %#v", legacyValues)
	}
	if len(selectedValues) == 0 || selectedValues[0].ReferenceID != "doc_1" ||
		!strings.Contains(selectedValues[0].ID, "#chunk-") {
		t.Fatalf("selected documents = %#v", selectedValues)
	}
}

func TestDeterministicStrategyBuildsDistinctPlansForEveryV2Platform(t *testing.T) {
	t.Parallel()
	brief := BriefVersion{
		BriefID: "brief_1", Version: 1,
		Snapshot: BriefDocument{
			ContractVersion: "strategy-brief-version/v2",
			Campaign:        BriefCampaign{Objective: "新品成交"},
			Audience:        BriefAudience{Primary: "品质消费人群"},
			Proposition:     "可验证的产品效果",
			Channels:        []string{"xiaohongshu", "douyin", "taobao_tmall", "wechat_ecosystem"},
			Measurement:     BriefMeasurement{PrimaryKPI: "有效成交数"},
		},
	}
	document := deterministicStrategy(brief, Draft{ProjectContextVersion: 3, SkillVersions: map[string]string{"strategy.strategy.generate": "v2.0.0"}})
	if err := document.Validate(); err != nil {
		t.Fatal(err)
	}
	if document.ContractVersion != "strategy-draft/v2" || len(document.PlatformPlans) != 4 {
		t.Fatalf("strategy document = %#v", document)
	}
	roles := map[string]string{}
	for _, plan := range document.PlatformPlans {
		roles[plan.Platform] = plan.Role
	}
	if len(roles) != 4 || roles["xiaohongshu"] == roles["douyin"] ||
		roles["douyin"] == roles["taobao_tmall"] || roles["taobao_tmall"] == roles["wechat_ecosystem"] {
		t.Fatalf("platform roles are not distinct: %#v", roles)
	}
}

func TestQualityRejectsDuplicatedPlatformPlansAndUnknownEvidence(t *testing.T) {
	t.Parallel()
	brief := BriefVersion{
		Snapshot: BriefDocument{
			ContractVersion: "strategy-brief-version/v2",
			Campaign:        BriefCampaign{Objective: "获取线索"},
			Audience:        BriefAudience{Primary: "研发负责人"},
			Proposition:     "缩短研发周期",
			Channels:        []string{"xiaohongshu", "douyin"},
			ReferenceIDs:    []string{"doc_1"},
			Measurement:     BriefMeasurement{PrimaryKPI: "有效线索数"},
		},
	}
	plan := PlatformPlan{
		Role: "种草", AudienceAngle: "研发提效", ContentPillars: []string{"问题", "方案"},
		Formats: []string{"案例"}, ConversionPath: "内容到咨询", Cadence: "每周",
		PrimaryKPI: "有效线索数", CreativeIdeas: []string{"案例拆解"}, Constraints: []string{"不夸大"},
	}
	xiaohongshu := plan
	xiaohongshu.Platform = "xiaohongshu"
	douyin := plan
	douyin.Platform = "douyin"
	document := StrategyDocument{
		ContractVersion: "strategy-draft/v2",
		Objective:       "获取线索",
		Audience:        StrategyAudience{Primary: "研发负责人", Insights: []string{"决策谨慎"}},
		Proposition:     "缩短研发周期",
		ChannelStrategy: []ChannelStrategy{
			{Platform: "xiaohongshu", Role: "种草", Formats: []string{"案例"}},
			{Platform: "douyin", Role: "种草", Formats: []string{"案例"}},
		},
		CreativeRecommendations: []string{"案例开场呈现问题", "展示解决路径和证据", "明确咨询行动入口"},
		ExperimentMatrix:        []Experiment{{Hypothesis: "案例更有效", Variable: "开场", Metric: "有效线索数"}},
		Measurement:             []string{"有效线索数"},
		PlatformPlans:           []PlatformPlan{xiaohongshu, douyin},
		EvidenceRefs:            []string{"doc_unknown"},
		Lineage:                 StrategyLineage{BriefID: "brief_1", BriefVersion: 1, ProjectContextVersion: 1},
	}
	report := evaluateStrategyQuality(document, GenerationContext{Brief: brief})
	joined := strings.Join(report.Errors, "\n")
	if !strings.Contains(joined, "not meaningfully distinct") ||
		!strings.Contains(joined, "unknown references") {
		t.Fatalf("report = %#v", report)
	}
}

type stubKnowledgeReader struct {
	values map[string]knowledge.Reference
}

func (s stubKnowledgeReader) GetReference(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, id string) (knowledge.Reference, error) {
	return s.values[id], nil
}

func TestRetainAllowedRevisionSections(t *testing.T) {
	t.Parallel()
	before := StrategyDocument{
		Objective:               "认知",
		Audience:                StrategyAudience{Primary: "研发负责人"},
		Proposition:             "提效",
		CreativeRecommendations: []string{"原建议"},
		Measurement:             []string{"咨询数"},
	}
	after := before
	after.Objective = "成交"
	after.CreativeRecommendations = []string{"B2B 决策链内容"}
	after.Measurement = []string{"成交数"}
	allowed := allowedRevisionSections("把创意建议改得更适合 B2B 技术决策者，其他章节保持不变")
	retainAllowedRevisionSections(before, &after, allowed)
	if after.Objective != before.Objective || after.Measurement[0] != before.Measurement[0] ||
		after.CreativeRecommendations[0] == before.CreativeRecommendations[0] {
		t.Fatalf("retained revision = %#v; allowed=%v", after, allowed)
	}
}

func TestRevisionScopeFailsClosedWhenInstructionIsAmbiguous(t *testing.T) {
	t.Parallel()
	scope := resolveRevisionScope("写得更年轻一点")
	if scope.Resolved || scope.ExplicitAll || len(scope.Sections) != 0 {
		t.Fatalf("scope = %#v", scope)
	}
}

func TestRevisionScopeAllowsExplicitWholeStrategyRewrite(t *testing.T) {
	t.Parallel()
	scope := resolveRevisionScope("请整体重写整份策略")
	if !scope.Resolved || !scope.ExplicitAll || len(scope.Sections) < 10 {
		t.Fatalf("scope = %#v", scope)
	}
}

func TestRevisionScopeRoutesNamedPlatformChangeToPlatformPlans(t *testing.T) {
	t.Parallel()
	scope := resolveRevisionScope("把小红书的内容节奏调整为首周种草、第二周测评扩散")
	if !scope.Resolved || strings.Join(scope.Sections, ",") != "platform_plans" {
		t.Fatalf("scope = %#v", scope)
	}
}

func TestRepairSectionsAreLimitedToFailingAreas(t *testing.T) {
	t.Parallel()
	sections := repairSectionsForErrors([]string{
		"platform plans do not cover the confirmed Brief",
		"measurement does not include the confirmed primary KPI",
	})
	if strings.Join(sections, ",") != "platform_plans,measurement" {
		t.Fatalf("sections = %#v", sections)
	}
}

func TestModelCallAttemptCapturesPerCallValidationAndOutputHash(t *testing.T) {
	t.Parallel()
	attempt := modelCallAttempt(
		"repair",
		promptkit.RepairV2,
		provider.SynchronousResponse{
			ProviderCode: "ark", ModelAlias: "cookies.text.standard",
			ModelVersion: "model-v1", RouteRevisionID: "route_1",
			ResponseMode:     provider.TextResponseJSONSchema,
			APIMode:          provider.TextAPIResponses,
			StructuredOutput: []byte(`{"ok":true}`),
			Usage: &provider.TokenUsage{
				InputTokens: 10, OutputTokens: 3, TotalTokens: 13,
			},
		},
		42,
		[]string{"measurement is incomplete"},
	)
	if attempt.Purpose != "repair" || attempt.PromptVersion != promptkit.RepairV2 ||
		attempt.ValidationPassed || len(attempt.ValidationErrors) != 1 ||
		attempt.LatencyMS != 42 || attempt.Usage == nil || len(attempt.OutputHash) != 64 {
		t.Fatalf("attempt = %#v", attempt)
	}
}
