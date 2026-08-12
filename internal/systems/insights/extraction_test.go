package insights

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

// 这批测试守的是提取运行器的三条底线：
// 模型说的话只能进 AI 层、失败也要留下痕迹、留痕里不能有正文。

// AM-006 / 03 §14 / 09 §8：模型产出只写 AI 推断层，且素材停在待确认。
// 模型说完话，事情没完——这是整条链路上唯一的人工闸门。
func TestExtractionWritesOnlyTheAILayerAndStopsForReview(t *testing.T) {
	t.Parallel()
	service, generator, runs := testExtractionService(t, extractionAnswer(map[string]any{
		"article_type":    map[string]any{"terms": []string{"知识"}, "confidence": "high", "evidence": "全文按「是什么/为什么/怎么做」三段展开"},
		"section_count":   map[string]any{"number": 3, "confidence": "medium", "evidence": "三个加粗小标题"},
		"conversion_path": map[string]any{"terms": []string{"私域"}, "confidence": "low", "evidence": "文末引导加企微"},
	}))
	actor := testActor()
	article := indexedAsset(t, service, actor, AssetTypeWechatArticle)

	result, err := service.AnalyzeAsset(context.Background(), actor, "project_1", article.ID, AnalyzeAssetRequest{
		ExpectedVersion: article.Version, Content: "这是一篇讲选品逻辑的公众号文章正文。",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Features) != 3 {
		t.Fatalf("三个字段应各写一行：%#v", result.Features)
	}
	for _, feature := range result.Features {
		if feature.Source != SourceAI || feature.ReviewState != ReviewPending {
			t.Fatalf("模型产出必须落在待确认的 AI 层：%#v", feature)
		}
	}
	if asset := mustGetAsset(t, service, actor, article.ID); asset.AnalysisStatus != AnalysisPendingConfirmation {
		t.Fatalf("提取完成后素材应进入待确认：%s", asset.AnalysisStatus)
	}
	if len(generator.requests) != 1 {
		t.Fatalf("一次提取只应调一次模型：%d", len(generator.requests))
	}
	if got := runs.finished(); len(got) != 1 || got[0].Status != AnalysisRunSucceeded || got[0].FeatureCount != 3 {
		t.Fatalf("应留下一条成功的分析任务：%#v", got)
	}
}

// 03 §113/§344 验收 12：结论要能回溯到素材、方法、Skill 版本和结果。
func TestSucceededRunCarriesTheWholeProvenanceChain(t *testing.T) {
	t.Parallel()
	service, _, runs := testExtractionService(t, extractionAnswer(map[string]any{
		"article_type": map[string]any{"terms": []string{"案例"}, "confidence": "high", "evidence": "通篇是一个品牌的复盘"},
	}))
	actor := testActor()
	article := indexedAsset(t, service, actor, AssetTypeWechatArticle)
	result, err := service.AnalyzeAsset(context.Background(), actor, "project_1", article.ID, AnalyzeAssetRequest{
		ExpectedVersion: article.Version, Content: "某美妆品牌的公众号复盘全文。",
	})
	if err != nil {
		t.Fatal(err)
	}
	run := result.Run
	missing := map[string]string{
		"素材":       run.AssetID,
		"Skill":    run.SkillID,
		"Skill 版本": run.SkillVersion,
		"Skill 指纹": run.SkillContentHash,
		"提示词版本":    run.PromptVersion,
		"供应商":      run.ProviderCode,
		"模型版本":     run.ModelVersion,
		"模型别名":     run.ModelAlias,
		"输入指纹":     run.InputHash,
		"结果指纹":     run.ResultHash,
	}
	for label, value := range missing {
		if value == "" {
			t.Fatalf("成功的分析任务缺少「%s」，这条特征就无法回溯", label)
		}
	}
	if run.SkillID != "insight.extract.wechat_article" {
		t.Fatalf("应按素材类型选 Skill：%s", run.SkillID)
	}
	if run.GenerationMode != GenerationModeModel {
		t.Fatalf("走了模型就要记成 model：%s", run.GenerationMode)
	}
	if run.FinishedAt == nil || run.Duration() <= 0 {
		t.Fatalf("成功的任务应当有结束时间和耗时：%#v", run)
	}
	if stored := runs.finished(); len(stored) != 1 || stored[0].ResultHash != run.ResultHash {
		t.Fatalf("回给调用方的和落库的应当是同一条：%#v", stored)
	}
}

// 09 §7：日志不记录内容全文。留痕里只能有规模，不能有正文。
func TestRunKeepsTheShapeOfTheInputNotTheInputItself(t *testing.T) {
	t.Parallel()
	const secretish = "这段正文里有一句只有这份素材才有的话"
	service, _, _ := testExtractionService(t, extractionAnswer(map[string]any{
		"article_type": map[string]any{"terms": []string{"知识"}, "confidence": "medium", "evidence": "科普结构"},
	}))
	actor := testActor()
	article := indexedAsset(t, service, actor, AssetTypeWechatArticle)
	result, err := service.AnalyzeAsset(context.Background(), actor, "project_1", article.ID, AnalyzeAssetRequest{
		ExpectedVersion: article.Version, Content: secretish, Note: "这条是重发版",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(result.Run.InputSummary), secretish) {
		t.Fatalf("分析留痕里出现了素材正文：%s", result.Run.InputSummary)
	}
	var summary struct {
		ContentChars int  `json:"content_chars"`
		HasNote      bool `json:"has_note"`
		FieldCount   int  `json:"field_count"`
	}
	if err := json.Unmarshal(result.Run.InputSummary, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.ContentChars != len([]rune(secretish)) || !summary.HasNote || summary.FieldCount == 0 {
		t.Fatalf("留痕应当记下输入的规模：%#v", summary)
	}
}

// 只记成功的话，运营面上的成功率永远是 100%，供应商挂掉那天也是一片绿。
func TestModelFailureStillLeavesAFailedRun(t *testing.T) {
	t.Parallel()
	service, _, runs := testExtractionService(t, extractionResponse{err: errors.New("上游 503")})
	actor := testActor()
	article := indexedAsset(t, service, actor, AssetTypeWechatArticle)
	_, err := service.AnalyzeAsset(context.Background(), actor, "project_1", article.ID, AnalyzeAssetRequest{
		ExpectedVersion: article.Version, Content: "正文。",
	})
	if err == nil {
		t.Fatal("模型失败时提取不应当成功")
	}
	stored := runs.finished()
	if len(stored) != 1 || stored[0].Status != AnalysisRunFailed || stored[0].ErrorCode != "provider_error" {
		t.Fatalf("供应商失败应记成 provider_error：%#v", stored)
	}
	if stored[0].ErrorMessage == "" || stored[0].FinishedAt == nil {
		t.Fatalf("失败的任务要写清原因和结束时间：%#v", stored[0])
	}
	if features := listFeatures(t, service, actor, article.ID); len(features) != 0 {
		t.Fatalf("失败的提取不应写进任何特征：%#v", features)
	}
	if asset := mustGetAsset(t, service, actor, article.ID); asset.AnalysisStatus == AnalysisPendingConfirmation {
		t.Fatal("失败的提取不应把素材推进待确认")
	}
}

// 一条都没提出来当失败处理：对使用者来说这和报错没有区别，都是白等一场。
func TestEmptyResultIsRecordedAsAFailureNotAnEmptySuccess(t *testing.T) {
	t.Parallel()
	for name, answer := range map[string]extractionResponse{
		"模型返回空对象":   extractionAnswer(map[string]any{}),
		"字段全部不属于本类": extractionAnswer(map[string]any{"hook_type": map[string]any{"terms": []string{"利益"}, "confidence": "high", "evidence": "开场"}}),
		"字段全部标了没有":  extractionAnswer(map[string]any{"article_type": map[string]any{"absent": true, "confidence": "high", "evidence": "全文没有明确类型"}}),
	} {
		t.Run(name, func(t *testing.T) {
			service, _, runs := testExtractionService(t, answer)
			actor := testActor()
			article := indexedAsset(t, service, actor, AssetTypeWechatArticle)
			_, err := service.AnalyzeAsset(context.Background(), actor, "project_1", article.ID, AnalyzeAssetRequest{
				ExpectedVersion: article.Version, Content: "正文。",
			})
			if !errors.Is(err, ErrInvalidState) {
				t.Fatalf("error=%v", err)
			}
			if stored := runs.finished(); len(stored) != 1 || stored[0].Status != AnalysisRunFailed {
				t.Fatalf("空结果应记成失败：%#v", stored)
			}
		})
	}
}

// 少几条特征在界面上看不出来——用户只会觉得「模型没看出来」，
// 而真实原因可能是格式没对上，那是要修的 bug。
func TestDroppedFieldsAreReportedNotSwallowed(t *testing.T) {
	t.Parallel()
	service, _, runs := testExtractionService(t, extractionAnswer(map[string]any{
		"article_type":    map[string]any{"terms": []string{"知识"}, "confidence": "high", "evidence": "科普结构"},
		"conversion_path": map[string]any{"terms": []string{"直播间"}, "confidence": "high", "evidence": "文末挂了直播预告"},
		"section_count":   map[string]any{"text": "三节", "confidence": "high", "evidence": "三个小标题"},
	}))
	actor := testActor()
	article := indexedAsset(t, service, actor, AssetTypeWechatArticle)
	result, err := service.AnalyzeAsset(context.Background(), actor, "project_1", article.ID, AnalyzeAssetRequest{
		ExpectedVersion: article.Version, Content: "正文。",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Features) != 1 {
		t.Fatalf("只有 article_type 是合格的：%#v", result.Features)
	}
	if len(result.Dropped) != 2 {
		t.Fatalf("词表外取值和类型不符都应被报出来：%#v", result.Dropped)
	}
	if stored := runs.finished(); len(stored) != 1 || len(stored[0].DroppedFields) != 2 {
		t.Fatalf("丢弃的字段也要落在留痕里：%#v", stored)
	}
}

// 03 §15② MVP 验收②：不把视频钩子字段套到公众号文章。
// 这里守的是「模型压根看不到别人的字段」，比事后拦截更早一步。
func TestModelOnlyEverSeesItsOwnAssetTypesSchema(t *testing.T) {
	t.Parallel()
	service, generator, _ := testExtractionService(t, extractionAnswer(map[string]any{
		"article_type": map[string]any{"terms": []string{"知识"}, "confidence": "high", "evidence": "科普结构"},
	}))
	actor := testActor()
	article := indexedAsset(t, service, actor, AssetTypeWechatArticle)
	if _, err := service.AnalyzeAsset(context.Background(), actor, "project_1", article.ID, AnalyzeAssetRequest{
		ExpectedVersion: article.Version, Content: "正文。",
	}); err != nil {
		t.Fatal(err)
	}
	request := generator.requests[0]
	sent := string(request.OutputJSONSchema)
	if !strings.Contains(sent, "article_type") {
		t.Fatalf("输出格式里应当有本类型的字段：%s", sent)
	}
	if strings.Contains(sent, "hook_type") || strings.Contains(sent, "digital_human") {
		t.Fatalf("输出格式里混进了别的素材类型的字段：%s", sent)
	}
	if len(request.Messages) != 2 || request.Messages[0].Role != provider.TextRoleSystem {
		t.Fatalf("应当是一条系统指令加一条素材内容：%#v", request.Messages)
	}
	if !strings.Contains(request.Messages[0].Content, "公众号") {
		t.Fatalf("系统指令应当来自公众号图文的 Skill：%s", request.Messages[0].Content)
	}
}

// 连点两下不应当付两次钱：同一条素材命中同一个幂等键。
func TestRepeatedExtractionReusesTheSameInvocationKey(t *testing.T) {
	t.Parallel()
	service, generator, _ := testExtractionService(t, extractionAnswer(map[string]any{
		"article_type": map[string]any{"terms": []string{"知识"}, "confidence": "high", "evidence": "科普结构"},
	}))
	actor := testActor()
	article := indexedAsset(t, service, actor, AssetTypeWechatArticle)
	for range 2 {
		current := mustGetAsset(t, service, actor, article.ID)
		if _, err := service.AnalyzeAsset(context.Background(), actor, "project_1", article.ID, AnalyzeAssetRequest{
			ExpectedVersion: current.Version, Content: "正文。",
		}); err != nil {
			t.Fatal(err)
		}
	}
	if len(generator.requests) != 2 {
		t.Fatalf("requests=%d", len(generator.requests))
	}
	first, second := generator.requests[0].InvocationKey, generator.requests[1].InvocationKey
	if first == "" || first != second {
		t.Fatalf("同一条素材的重复提取应当共用幂等键：%q vs %q", first, second)
	}
}

// 调用人本来就有发文本请求的权限时，不能再给他加一遍——重复的 scope 会被
// ActorContext.Validate 拒掉，请求根本发不出去。线上管理员正是这种账号，
// 而 testActor 不带这个 scope，所以别的用例都绕开了这条路。
func TestExtractionDoesNotDuplicateAScopeTheCallerAlreadyHas(t *testing.T) {
	t.Parallel()
	service, generator, _ := testExtractionService(t, extractionAnswer(map[string]any{
		"article_type": map[string]any{"terms": []string{"知识"}, "confidence": "high", "evidence": "科普结构"},
	}))
	actor := testActor()
	actor.Scopes = append(actor.Scopes, provider.ScopeTextGenerate)
	article := indexedAsset(t, service, actor, AssetTypeWechatArticle)
	if _, err := service.AnalyzeAsset(context.Background(), actor, "project_1", article.ID, AnalyzeAssetRequest{
		ExpectedVersion: article.Version, Content: "正文。",
	}); err != nil {
		t.Fatalf("已经带着 provider.text.generate 的人也应当能提取：%v", err)
	}
	if len(generator.requests) != 1 {
		t.Fatalf("requests=%d", len(generator.requests))
	}
	if err := generator.requests[0].Actor.Validate(); err != nil {
		t.Fatalf("发给模型的身份必须是合法的：%v", err)
	}
}

// 没配模型时不退化成模板产出——库里一条编造的特征，
// 代价远大于一次失败的提取。
func TestExtractionFailsInsteadOfFabricatingWhenNoModelIsConfigured(t *testing.T) {
	t.Parallel()
	service, _, runs := testExtractionService(t, extractionAnswer(map[string]any{}))
	service.Text = nil
	actor := testActor()
	article := indexedAsset(t, service, actor, AssetTypeWechatArticle)
	_, err := service.AnalyzeAsset(context.Background(), actor, "project_1", article.ID, AnalyzeAssetRequest{
		ExpectedVersion: article.Version, Content: "正文。",
	})
	if !errors.Is(err, ErrInvalidState) {
		t.Fatalf("error=%v", err)
	}
	if features := listFeatures(t, service, actor, article.ID); len(features) != 0 {
		t.Fatalf("没有模型时不应凭空写出特征：%#v", features)
	}
	if stored := runs.finished(); len(stored) != 1 || stored[0].Status != AnalysisRunFailed {
		t.Fatalf("配置缺失也要留下失败记录：%#v", stored)
	}
}

// 留痕仓储没接上时宁可不跑：一次无法追溯的提取，
// 产出的特征后面没人能说清是怎么来的。
func TestExtractionRefusesToRunWithoutARunLedger(t *testing.T) {
	t.Parallel()
	service, generator, _ := testExtractionService(t, extractionAnswer(map[string]any{}))
	service.Runs = nil
	actor := testActor()
	article := indexedAsset(t, service, actor, AssetTypeWechatArticle)
	if _, err := service.AnalyzeAsset(context.Background(), actor, "project_1", article.ID, AnalyzeAssetRequest{
		ExpectedVersion: article.Version, Content: "正文。",
	}); err == nil {
		t.Fatal("没有留痕仓储时提取应当被拒绝")
	}
	if len(generator.requests) != 0 {
		t.Fatal("被拒绝的提取不应当已经调过模型")
	}
}

func TestExtractionRejectsUnusableRequests(t *testing.T) {
	t.Parallel()
	service, _, _ := testExtractionService(t, extractionAnswer(map[string]any{}))
	actor := testActor()
	article := indexedAsset(t, service, actor, AssetTypeWechatArticle)
	cases := map[string]AnalyzeAssetRequest{
		"没有素材内容":    {ExpectedVersion: article.Version},
		"正文超过 2 万字": {ExpectedVersion: article.Version, Content: strings.Repeat("字", 20001)},
		"补充说明过长":    {ExpectedVersion: article.Version, Content: "正文。", Note: strings.Repeat("说", 1001)},
	}
	for name, request := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := service.AnalyzeAsset(context.Background(), actor, "project_1", article.ID, request); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	// 类型没识别就不知道该提哪套特征，这一步要挡在调模型之前。
	unknown := indexedAsset(t, service, actor, AssetTypeUnknown)
	if _, err := service.AnalyzeAsset(context.Background(), actor, "project_1", unknown.ID, AnalyzeAssetRequest{
		ExpectedVersion: unknown.Version, Content: "正文。",
	}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("error=%v", err)
	}
}

// 模型爱在 JSON 外面包一层 ```json 围栏，也爱在后面补一句解释。
// 前者要能吃下去，后者说明它没按格式来，前半段也不该信。
func TestFencedJSONIsAcceptedButTrailingContentIsNot(t *testing.T) {
	t.Parallel()
	body := `{"article_type":{"terms":["知识"],"confidence":"high","evidence":"科普结构"}}`
	t.Run("带围栏的纯 JSON 可以接受", func(t *testing.T) {
		service, _, _ := testExtractionService(t, extractionResponse{text: "```json\n" + body + "\n```"})
		actor := testActor()
		article := indexedAsset(t, service, actor, AssetTypeWechatArticle)
		result, err := service.AnalyzeAsset(context.Background(), actor, "project_1", article.ID, AnalyzeAssetRequest{
			ExpectedVersion: article.Version, Content: "正文。",
		})
		if err != nil || len(result.Features) != 1 {
			t.Fatalf("result=%#v err=%v", result, err)
		}
	})
	t.Run("JSON 后面还跟着话就整条不信", func(t *testing.T) {
		service, _, runs := testExtractionService(t, extractionResponse{text: body + "\n以上是我的分析。"})
		actor := testActor()
		article := indexedAsset(t, service, actor, AssetTypeWechatArticle)
		_, err := service.AnalyzeAsset(context.Background(), actor, "project_1", article.ID, AnalyzeAssetRequest{
			ExpectedVersion: article.Version, Content: "正文。",
		})
		if !errors.Is(err, ErrInvalidState) {
			t.Fatalf("error=%v", err)
		}
		// 09 §7：错误信息里不能带模型返回的原文。
		if stored := runs.finished(); len(stored) != 1 || strings.Contains(stored[0].ErrorMessage, "以上是我的分析") {
			t.Fatalf("错误信息里不应出现模型返回的内容：%#v", stored)
		}
	})
}

// 六类素材都要有自己的 Skill，缺一类就有一类素材点了按钮没反应。
func TestEverySixAssetTypesHaveAnExtractionSkill(t *testing.T) {
	t.Parallel()
	service, generator, _ := testExtractionService(t, extractionResponse{err: errors.New("到这一步就够了")})
	actor := testActor()
	for _, assetType := range AllAssetTypes() {
		asset := indexedAsset(t, service, actor, assetType)
		_, err := service.AnalyzeAsset(context.Background(), actor, "project_1", asset.ID, AnalyzeAssetRequest{
			ExpectedVersion: asset.Version, Content: "内容。",
		})
		// 走到调模型说明 Skill 和输出格式都拿到了；这里只关心没在那之前被挡下。
		if err == nil || strings.Contains(err.Error(), "Skill") {
			t.Fatalf("%s 缺少特征提取 Skill：%v", assetType, err)
		}
	}
	if len(generator.requests) != len(AllAssetTypes()) {
		t.Fatalf("六类素材都应当走到模型：%d", len(generator.requests))
	}
	// 六份系统指令互不相同——同一份提示词套六类素材等于没分类型。
	seen := map[string]bool{}
	for _, request := range generator.requests {
		if seen[request.Messages[0].Content] {
			t.Fatal("两类素材用了同一份系统指令")
		}
		seen[request.Messages[0].Content] = true
	}
}

func TestListAnalysisRunsIsScopedToTheAsset(t *testing.T) {
	t.Parallel()
	service, _, _ := testExtractionService(t, extractionAnswer(map[string]any{
		"article_type": map[string]any{"terms": []string{"知识"}, "confidence": "high", "evidence": "科普结构"},
	}))
	actor := testActor()
	article := indexedAsset(t, service, actor, AssetTypeWechatArticle)
	other := indexedAsset(t, service, actor, AssetTypeWechatArticle)
	for _, asset := range []Asset{article, other} {
		if _, err := service.AnalyzeAsset(context.Background(), actor, "project_1", asset.ID, AnalyzeAssetRequest{
			ExpectedVersion: asset.Version, Content: "正文。",
		}); err != nil {
			t.Fatal(err)
		}
	}
	runs, err := service.ListAnalysisRuns(context.Background(), actor, "project_1", AnalysisRunFilter{AssetID: article.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].AssetID != article.ID {
		t.Fatalf("按素材过滤应当只剩这条素材的记录：%#v", runs)
	}
}

// ---- 测试替身 ----

func testExtractionService(t *testing.T, response extractionResponse) (Service, *fakeTextGenerator, *memoryRunRepository) {
	t.Helper()
	service := testAssetService()
	generator := &fakeTextGenerator{response: response}
	runs := &memoryRunRepository{}
	service.Text = generator
	service.Runs = runs
	// 时钟必须往前走：耗时是运营面上判断供应商是否变慢的唯一依据，
	// 固定时钟会让每条任务的耗时都是 0，测不出这件事有没有被记下来。
	clock := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	service.Now = func() time.Time {
		clock = clock.Add(250 * time.Millisecond)
		return clock
	}
	return service, generator, runs
}

func listFeatures(t *testing.T, service Service, actor contract.ActorContext, assetID string) []AssetFeature {
	t.Helper()
	features, err := service.ListAssetFeatures(context.Background(), actor, "project_1", assetID)
	if err != nil {
		t.Fatal(err)
	}
	return features
}

// extractionAnswer 把一组字段答案包成模型的结构化返回。
func extractionAnswer(fields map[string]any) extractionResponse {
	encoded, err := json.Marshal(fields)
	if err != nil {
		panic(err)
	}
	return extractionResponse{structured: encoded}
}

type extractionResponse struct {
	structured json.RawMessage
	text       string
	err        error
}

type fakeTextGenerator struct {
	response extractionResponse
	requests []provider.TextGenerateRequest
}

// 先校验再作答，跟真实的 provider.Service.GenerateText 一样。假生成器什么都收的话，
// 一个组不出合法请求的 bug 在这里全绿，到线上才被供应商层拒掉。
func (g *fakeTextGenerator) GenerateText(_ context.Context, request provider.TextGenerateRequest) (provider.SynchronousResponse, error) {
	g.requests = append(g.requests, request)
	if err := request.Validate(); err != nil {
		return provider.SynchronousResponse{}, err
	}
	if g.response.err != nil {
		return provider.SynchronousResponse{}, g.response.err
	}
	return provider.SynchronousResponse{
		ProviderCode: "fake", ModelAlias: request.ModelAlias, ModelVersion: "fake-v1",
		RouteRevisionID: "route_1", ResponseMode: provider.TextResponseJSONSchema,
		StructuredOutput: g.response.structured, Text: g.response.text,
		Usage: &provider.TokenUsage{InputTokens: 1200, OutputTokens: 300, TotalTokens: 1500},
	}, nil
}

// memoryRunRepository 复刻 MySQL 那边靠约束拿到的两条保证：
// running 行先落地，且一条任务只能被结束一次。
type memoryRunRepository struct {
	rows []AnalysisRun
}

func (r *memoryRunRepository) CreateAnalysisRun(_ context.Context, value AnalysisRun) (AnalysisRun, error) {
	if err := value.validate(); err != nil {
		return AnalysisRun{}, err
	}
	if value.Status != AnalysisRunRunning {
		return AnalysisRun{}, fmt.Errorf("分析任务只能以 running 落地：%s", value.Status)
	}
	r.rows = append(r.rows, value)
	return value, nil
}

func (r *memoryRunRepository) FinishAnalysisRun(_ context.Context, value AnalysisRun) (AnalysisRun, error) {
	if err := value.validate(); err != nil {
		return AnalysisRun{}, err
	}
	for index, row := range r.rows {
		if row.ID != value.ID {
			continue
		}
		if row.Status != AnalysisRunRunning {
			return AnalysisRun{}, ErrNotFound
		}
		r.rows[index] = value
		return value, nil
	}
	return AnalysisRun{}, ErrNotFound
}

func (r *memoryRunRepository) ListAnalysisRuns(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, filter AnalysisRunFilter) ([]AnalysisRun, error) {
	var matched []AnalysisRun
	for index := len(r.rows) - 1; index >= 0; index-- {
		row := r.rows[index]
		if row.OrganizationID != organizationID || row.ProjectID != projectID {
			continue
		}
		if filter.AssetID != "" && row.AssetID != filter.AssetID {
			continue
		}
		if filter.Kind != "" && row.Kind != filter.Kind {
			continue
		}
		if len(filter.Statuses) > 0 && !containsRunStatus(filter.Statuses, row.Status) {
			continue
		}
		matched = append(matched, row)
		if filter.Limit > 0 && len(matched) >= filter.Limit {
			break
		}
	}
	return matched, nil
}

func containsRunStatus(values []AnalysisRunStatus, value AnalysisRunStatus) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

// finished 返回已经结掉的任务；还挂在 running 的不算数。
func (r *memoryRunRepository) finished() []AnalysisRun {
	var done []AnalysisRun
	for _, row := range r.rows {
		if row.Status != AnalysisRunRunning {
			done = append(done, row)
		}
	}
	return done
}
