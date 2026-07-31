package insights

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/systems/insights/skills"
)

// 特征提取运行器：AM-005 里「AI 提」的那一半。
//
// **模型产出只进 AI 推断层，永远不进人工结论层**（03 §14、09 §8）。
// 这不是一句原则，是代码路径上的事实：这里唯一的写入口是 ExtractFeatures，
// 它写的是 source='ai' 的行，而人工结论走 PatchFeatures，两条路不相交。
// 提取完成后素材进「待确认」，等人来看——模型说完话，事情没完。
//
// **只有人点按钮才会走到这里。** 素材登记时不自动排队：自动提取意味着
// 复核队列会被灌满没人要看的结果，而复核是这套东西唯一的质量闸门。

// TextGenerator 是这里对供应商的全部依赖。
// 定义成接口而不是直接用 *provider.Service，是为了让测试能在不连供应商的
// 情况下走完整条路——包括失败路径，那条路在真实供应商上很难稳定复现。
type TextGenerator interface {
	GenerateText(context.Context, provider.TextGenerateRequest) (provider.SynchronousResponse, error)
}

// AnalyzeAssetRequest 是人点下「提取特征」时带上的东西。
//
// Content 必填，而且必须由调用方给。**这是当前的真实边界，不是设计选择**：
// insight_assets 只存素材的身份和状态，不存正文——正文在创意系统或用户手里。
// 与其在这里假装能拿到，不如让调用方显式带上，出问题时错误指向清楚。
// 素材库补齐正文存储之后，这个字段应当变成可选。
type AnalyzeAssetRequest struct {
	ExpectedVersion int64 `json:"expected_version"`
	// Content 是要分析的素材内容：图文类填正文，视频类填脚本或画面描述。
	Content string `json:"content"`
	// Note 是人写的补充说明（例如「这条是竖版重剪，画面同 A 版」）。
	Note string `json:"note,omitempty"`
}

func (r AnalyzeAssetRequest) validate() error {
	content := strings.TrimSpace(r.Content)
	if content == "" {
		return fmt.Errorf("%w: 提取特征需要素材内容，请先补上正文或画面描述", ErrInvalidRequest)
	}
	// 上限不是性能考虑，是成本和可解释性：超长输入下模型的注意力会散开，
	// 提出来的特征更多是「文章里提过」而不是「这篇文章的特点」。
	if len([]rune(content)) > 20000 {
		return fmt.Errorf("%w: 素材内容超过 2 万字，请截取要分析的部分", ErrInvalidRequest)
	}
	if len([]rune(r.Note)) > 1000 {
		return fmt.Errorf("%w: 补充说明请控制在 1000 字以内", ErrInvalidRequest)
	}
	return nil
}

// AnalyzeAssetResult 是一次提取的完整结果。
//
// Dropped 一定要回给调用方。少几条特征在界面上看不出来——
// 用户只会觉得「模型没看出来」，而真实原因可能是格式没对上，
// 那是要修的 bug，不是模型能力问题。
type AnalyzeAssetResult struct {
	Run      AnalysisRun    `json:"run"`
	Features []AssetFeature `json:"features"`
	Dropped  []string       `json:"dropped_fields,omitempty"`
}

// AnalyzeAsset 跑一次特征提取。
//
// 失败路径上仍然会留下一条 failed 的 AnalysisRun——**这是有意的**：
// 只记成功的话，运营面上的成功率永远是 100%，供应商挂掉的那天也是一片绿。
func (s Service) AnalyzeAsset(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, assetID string, request AnalyzeAssetRequest) (AnalyzeAssetResult, error) {
	if err := s.assetsReady(actor, projectID, ScopeWrite); err != nil {
		return AnalyzeAssetResult{}, err
	}
	if s.Runs == nil {
		return AnalyzeAssetResult{}, fmt.Errorf("分析任务留痕未配置，拒绝执行无法追溯的提取")
	}
	if err := request.validate(); err != nil {
		return AnalyzeAssetResult{}, err
	}
	projectContext, err := s.Projects.RequireActiveContext(ctx, actor, projectID)
	if err != nil {
		return AnalyzeAssetResult{}, err
	}
	asset, err := s.Assets.GetAsset(ctx, actor.OrganizationID, projectID, assetID)
	if err != nil {
		return AnalyzeAssetResult{}, err
	}
	if !asset.TypeIdentified() {
		return AnalyzeAssetResult{}, fmt.Errorf("%w: 素材类型待识别，无法确定要提取哪套特征", ErrInvalidState)
	}
	schema, ok := FeatureSchemaFor(asset.AssetType)
	if !ok {
		return AnalyzeAssetResult{}, fmt.Errorf("%w: 素材类型 %q 没有特征体系", ErrInvalidState, string(asset.AssetType))
	}
	registry, err := s.skillRegistry()
	if err != nil {
		return AnalyzeAssetResult{}, err
	}
	skill, ok := registry.For(string(asset.AssetType))
	if !ok {
		return AnalyzeAssetResult{}, fmt.Errorf("%w: %s还没有特征提取 Skill", ErrInvalidState, schema.Label)
	}
	outputSchema, err := featureOutputSchema(schema)
	if err != nil {
		return AnalyzeAssetResult{}, err
	}

	instruction := extractionInstruction(skill, schema)
	content := strings.TrimSpace(request.Content)
	payload := extractionPayload(asset, content, strings.TrimSpace(request.Note))

	// 先落一行 running，再调模型。反过来的话，调用期间进程挂掉就什么都不剩，
	// 而那正是最需要被看见的一类失败——它不会出现在任何统计里。
	started := s.now()
	runID, err := s.idGenerator()("insightrun")
	if err != nil {
		return AnalyzeAssetResult{}, err
	}
	inputSummary, _ := json.Marshal(map[string]any{
		"asset_revision": asset.Revision,
		"content_chars":  len([]rune(content)),
		"has_note":       request.Note != "",
		"field_count":    len(schema.Fields),
	})
	run := AnalysisRun{
		ID: runID, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		Kind: AnalysisRunFeatureExtraction, AssetID: asset.ID, AssetType: asset.AssetType,
		Status:  AnalysisRunRunning,
		SkillID: skill.Name, SkillVersion: skill.Version, SkillContentHash: skill.ContentHash,
		PromptVersion: extractionPromptVersion,
		// 输入指纹算的是真正发出去的那两段，不是原始正文——
		// 指令变了而正文没变，也应当算作不同的输入。
		InputHash: hashPayload([]byte(instruction + "\n" + payload)),
		// 全文不入表（09 §7），只留规模。
		InputSummary: inputSummary,
		ModelAlias:   s.textModelAlias(),
		StartedAt:    started, CreatedBy: actor.Principal.ID, CreatedAt: started, UpdatedAt: started,
	}
	if run, err = s.Runs.CreateAnalysisRun(ctx, run); err != nil {
		return AnalyzeAssetResult{}, err
	}

	inputs, dropped, trace, err := s.generateFeatures(ctx, actor, projectContext, schema, instruction, payload, outputSchema, asset.ID)
	if err != nil {
		s.failRun(ctx, run, err)
		return AnalyzeAssetResult{}, err
	}
	if len(inputs) == 0 {
		// 一条都没提出来当失败处理。当成功会让「模型什么也没看出来」被计入
		// 成功率，而这种运行对使用者来说和报错没有区别——都是白等一场。
		failure := fmt.Errorf("%w: 模型没有提出任何符合%s特征体系的字段", ErrInvalidState, schema.Label)
		s.failRun(ctx, run, failure)
		return AnalyzeAssetResult{}, failure
	}

	features, err := s.ExtractFeatures(ctx, actor, projectID, asset.ID, ExtractFeaturesRequest{
		ExpectedVersion: request.ExpectedVersion,
		SkillID:         skill.Name, SkillVersion: skill.Version, Features: inputs,
	})
	if err != nil {
		s.failRun(ctx, run, err)
		return AnalyzeAssetResult{}, err
	}

	finished := s.now()
	run.Status = AnalysisRunSucceeded
	run.ProviderCode = trace.providerCode
	run.ModelVersion = trace.modelVersion
	run.RouteRevisionID = trace.routeRevisionID
	run.ResponseMode = trace.responseMode
	run.GenerationMode = trace.mode
	run.ResultHash = hashFeatureInputs(inputs)
	run.FeatureCount = len(features)
	run.DroppedFields = dropped
	run.PromptTokens = trace.promptTokens
	run.CompletionTokens = trace.completionTokens
	run.LatencyMS = int(finished.Sub(started).Milliseconds())
	run.FinishedAt = &finished
	run.UpdatedAt = finished
	saved, err := s.Runs.FinishAnalysisRun(ctx, run)
	if err != nil {
		// 特征已经写进去了，留痕却没写上。**不回滚特征**：
		// 有结果没留痕，比没结果好办——留痕能补，结果重跑要再花一次钱。
		// 但必须报出来，否则这条特征会永远缺一段来历。
		return AnalyzeAssetResult{}, fmt.Errorf("特征已写入但分析留痕失败，这批特征暂时无法追溯：%w", err)
	}
	return AnalyzeAssetResult{Run: saved, Features: features, Dropped: dropped}, nil
}

// ListAnalysisRuns 供「数据与方法」视图读这条素材的分析历史（03 §82）。
func (s Service) ListAnalysisRuns(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, filter AnalysisRunFilter) ([]AnalysisRun, error) {
	if err := s.assetsReady(actor, projectID, ScopeRead); err != nil {
		return nil, err
	}
	if s.Runs == nil {
		return nil, fmt.Errorf("分析任务留痕未配置")
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	return s.Runs.ListAnalysisRuns(ctx, actor.OrganizationID, projectID, filter)
}

const extractionPromptVersion = "insight.extract.v1"

// generationTrace 是一次模型调用留下的可追溯信息。
type generationTrace struct {
	mode             GenerationMode
	providerCode     string
	modelVersion     string
	routeRevisionID  string
	responseMode     string
	promptTokens     int
	completionTokens int
}

func (s Service) generateFeatures(ctx context.Context, actor contract.ActorContext, projectContext contract.ProjectContext, schema FeatureSchema, instruction, payload string, outputSchema json.RawMessage, assetID string) ([]FeatureInput, []string, generationTrace, error) {
	if s.Text == nil {
		// 没配供应商时不假装提取。返回空+模板模式，让上层按「一条都没提出来」失败，
		// 而不是写进一批编造的特征——库里一条假特征的代价，远大于一次失败的提取。
		return nil, nil, generationTrace{mode: GenerationModeTemplate}, fmt.Errorf("%w: 还没有配置可用的文本模型，无法提取特征", ErrInvalidState)
	}
	textActor := actor
	textActor.Scopes = append(append([]contract.Scope(nil), actor.Scopes...), provider.ScopeTextGenerate)
	response, err := s.Text.GenerateText(ctx, provider.TextGenerateRequest{
		Actor: textActor, Project: projectContext, ModelAlias: s.textModelAlias(),
		// 同一条素材重复点按钮时命中同一个 InvocationKey，避免连点付两次钱。
		InvocationKey: contract.IdempotencyKey("insight-extract-" + assetID),
		Messages: []provider.TextMessage{
			{Role: provider.TextRoleSystem, Content: instruction},
			{Role: provider.TextRoleUser, Content: payload},
		},
		OutputJSONSchema: outputSchema,
	})
	if err != nil {
		return nil, nil, generationTrace{}, fmt.Errorf("调用文本模型失败：%w", err)
	}
	trace := generationTrace{
		mode: GenerationModeModel, providerCode: response.ProviderCode,
		modelVersion: response.ModelVersion, routeRevisionID: response.RouteRevisionID,
		responseMode: string(response.ResponseMode),
	}
	if response.Usage != nil {
		trace.promptTokens = int(response.Usage.InputTokens)
		trace.completionTokens = int(response.Usage.OutputTokens)
	}
	candidate := response.StructuredOutput
	if len(candidate) == 0 {
		candidate = trimJSONFence(response.Text)
	}
	if len(bytes.TrimSpace(candidate)) == 0 {
		return nil, nil, trace, fmt.Errorf("%w: 模型没有返回内容", ErrInvalidState)
	}
	var output extractionOutput
	if err := decodeExactlyOneJSON(candidate, &output); err != nil {
		// 不把模型的原始返回写进错误信息（09 §7：不记录内容全文）。
		return nil, nil, trace, fmt.Errorf("%w: 模型返回的不是约定的特征格式", ErrInvalidState)
	}
	inputs, dropped := featureInputsFromOutput(schema, output)
	return inputs, dropped, trace, nil
}

// failRun 把 running 那一行结掉。
//
// 结不掉也只记不管——**原始失败必须原样返回给调用方**，
// 用留痕失败盖掉它会让人去查错的方向。
func (s Service) failRun(ctx context.Context, run AnalysisRun, cause error) {
	finished := s.now()
	run.Status = AnalysisRunFailed
	run.FinishedAt = &finished
	run.UpdatedAt = finished
	run.LatencyMS = int(finished.Sub(run.StartedAt).Milliseconds())
	run.ErrorCode = extractionErrorCode(cause)
	run.ErrorMessage = truncateRunes(cause.Error(), 900)
	_, _ = s.Runs.FinishAnalysisRun(ctx, run)
}

func extractionErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		return "invalid_request"
	case errors.Is(err, ErrInvalidState):
		return "invalid_state"
	case errors.Is(err, ErrVersionConflict):
		return "conflict"
	case errors.Is(err, ErrNotFound):
		return "not_found"
	default:
		return "provider_error"
	}
}

// extractionInstruction 拼 system 消息：角色 + 规则 + 这套特征体系的说明。
//
// 特征清单也放进去，尽管输出格式里已经有了一份。**重复是有用的**：
// 有些供应商把 JSON Schema 当成事后校验而不是生成约束，模型看不到字段说明；
// 写在正文里能让「时长填秒数」这类要求真正到达模型。
func extractionInstruction(skill skills.Snapshot, schema FeatureSchema) string {
	var builder strings.Builder
	builder.WriteString(skill.Persona)
	builder.WriteString("\n\n提取规则：\n")
	for index, rule := range skill.Instructions {
		fmt.Fprintf(&builder, "%d. %s\n", index+1, rule)
	}
	builder.WriteString("\n要提取的特征（按分组）：\n")
	for _, group := range schema.Groups() {
		fmt.Fprintf(&builder, "【%s】\n", group)
		for _, field := range schema.Fields {
			if field.Group != group {
				continue
			}
			fmt.Fprintf(&builder, "- %s（%s）", field.Key, field.Label)
			if field.Unit != "" {
				fmt.Fprintf(&builder, "，单位 %s", field.Unit)
			}
			if len(field.Vocabulary) > 0 {
				fmt.Fprintf(&builder, "，只能选：%s", strings.Join(field.Vocabulary, "、"))
			}
			if field.Note != "" {
				fmt.Fprintf(&builder, "。%s", field.Note)
			}
			builder.WriteString("\n")
		}
	}
	builder.WriteString("\n只返回一个 JSON 对象，不要任何解释文字。")
	return builder.String()
}

// extractionPayload 拼 user 消息。素材标题也给出去——
// 标题本身就是要提取的对象之一（标题角度、标题与正文一致性）。
func extractionPayload(asset Asset, content, note string) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "素材标题：%s\n", asset.Title)
	if note != "" {
		fmt.Fprintf(&builder, "补充说明：%s\n", note)
	}
	builder.WriteString("\n素材内容：\n")
	builder.WriteString(content)
	return builder.String()
}

func (s Service) skillRegistry() (skills.Registry, error) {
	if s.Skills != nil {
		return *s.Skills, nil
	}
	registry, err := skills.DefaultRegistry()
	if err != nil {
		return skills.Registry{}, fmt.Errorf("加载特征提取 Skill 失败：%w", err)
	}
	return registry, nil
}

func (s Service) textModelAlias() string {
	if alias := strings.TrimSpace(s.TextModelAlias); alias != "" {
		return alias
	}
	return "cookies.text.standard"
}

// trimJSONFence 去掉模型习惯性包上的 ```json 围栏。
func trimJSONFence(value string) json.RawMessage {
	content := strings.TrimSpace(value)
	if strings.HasPrefix(content, "```") {
		if newline := strings.IndexByte(content, '\n'); newline >= 0 {
			content = content[newline+1:]
		}
		content = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(content), "```"))
	}
	return json.RawMessage(content)
}

// decodeExactlyOneJSON 只接受恰好一个 JSON 值。
// 后面还跟着东西说明模型没按格式来，此时前半段也不可信。
func decodeExactlyOneJSON(value json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(value))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("模型返回了不止一个 JSON 值")
	}
	return nil
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
