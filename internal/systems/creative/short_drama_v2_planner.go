package creative

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

var shortDramaV2DirectionSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"required":["directions"],"properties":{"directions":{"type":"array","minItems":4,"maxItems":4,"items":{"type":"object","additionalProperties":false,"required":["id","category","title","hook_copy","description","rationale","visual_intent","grounding_evidence_ids"],"properties":{"id":{"type":"string"},"category":{"type":"string","enum":["curiosity","summary"]},"title":{"type":"string"},"hook_copy":{"type":"string"},"description":{"type":"string"},"rationale":{"type":"string"},"visual_intent":{"type":"string"},"grounding_evidence_ids":{"type":"array","minItems":1,"items":{"type":"string"}}}}}}}`)

var shortDramaV2PromptSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"required":["image_prompt","video_description","video_prompt"],"properties":{"image_prompt":{"type":"string"},"video_description":{"type":"string"},"video_prompt":{"type":"string"}}}`)

type ModelShortDramaV2Planner struct {
	Text       ShortDramaPrerollTextGenerator
	ModelAlias string
}

type DeterministicShortDramaV2Planner struct{}

func (DeterministicShortDramaV2Planner) PlanDirections(_ context.Context, _ contract.ActorContext, _ contract.ProjectContext, analysis ShortDramaV2Analysis) ([]ShortDramaV2HookDirection, string, error) {
	content := analysis.Content
	evidenceIDs := make([]string, 0, len(content.Evidence))
	for _, evidence := range content.Evidence {
		if id := strings.TrimSpace(evidence.ID); id != "" {
			evidenceIDs = append(evidenceIDs, id)
		}
	}
	if len(evidenceIDs) == 0 {
		return nil, "", fmt.Errorf("short drama V2 deterministic planner requires grounded evidence")
	}
	grounding := func(index int) []string {
		return []string{evidenceIDs[index%len(evidenceIDs)]}
	}
	title := strings.TrimSpace(content.Title)
	return []ShortDramaV2HookDirection{
		{
			ID: "curiosity_fact_gap", Category: "curiosity", Title: "这一刻之后，真正的变局才刚刚开始",
			HookCopy:             fmt.Sprintf("%s，但最关键的问题仍未揭开：%s", content.OpeningBeat, content.UnresolvedHook),
			Description:          fmt.Sprintf("从开场事实切入，以“%s”制造信息缺口，引导点击观看正片。", content.UnresolvedHook),
			Rationale:            "使用视频理解结果中的开场事实与未解悬念，不补写新剧情。",
			VisualIntent:         fmt.Sprintf("用%s的真实场景建立悬念，随后快速压近关键人物或动作。", content.Tone),
			GroundingEvidenceIDs: grounding(0),
		},
		{
			ID: "curiosity_conflict", Category: "curiosity", Title: "她面对的真正阻力，究竟是什么？",
			HookCopy:             fmt.Sprintf("%s。局面会被谁改写？", content.CoreConflict),
			Description:          "把核心冲突前置，用结果未知的问题形成第二种猎奇机制。",
			Rationale:            "只放大已识别的核心冲突，不预告未出现的结局。",
			VisualIntent:         "以冲突双方或对立动作的快速交替建立压力，结尾保留未完成动作。",
			GroundingEvidenceIDs: grounding(1),
		},
		{
			ID: "summary_turning_point", Category: "summary", Title: fmt.Sprintf("看懂《%s》的关键转折", title),
			HookCopy:             fmt.Sprintf("从“%s”到“%s”，故事的转折就在这里。", content.OpeningBeat, content.CoreConflict),
			Description:          "按开场—冲突的顺序压缩剧情，让观众在短时间内建立观看上下文。",
			Rationale:            "使用视频梗概的因果顺序完成稳妥的剧情总结。",
			VisualIntent:         "按时间顺序呈现开场状态、冲突升级与悬念停顿，保持人物和场景连续。",
			GroundingEvidenceIDs: grounding(2),
		},
		{
			ID: "summary_core_story", Category: "summary", Title: fmt.Sprintf("6 秒抓住《%s》的核心矛盾", title),
			HookCopy:             fmt.Sprintf("%s 点击正片，看这场冲突如何继续。", content.CoreConflict),
			Description:          fmt.Sprintf("以核心冲突概括剧情，并用“%s”保留后续期待。", content.UnresolvedHook),
			Rationale:            "用核心矛盾而非额外设定完成另一种剧情总结。",
			VisualIntent:         "先给出冲突全景，再聚焦决定性人物表情或动作，最后自然停在信息缺口。",
			GroundingEvidenceIDs: grounding(3),
		},
	}, "deterministic/short-drama-v2-directions-v1", nil
}

func (DeterministicShortDramaV2Planner) CompilePrompts(_ context.Context, _ contract.ActorContext, _ contract.ProjectContext, analysis ShortDramaV2Analysis, direction ShortDramaV2HookDirection, duration int) (ShortDramaV2PromptDraft, error) {
	keywords := strings.Join(analysis.Content.VisualKeywords, "、")
	if keywords == "" {
		keywords = "与原短剧一致的人物、时代和场景"
	}
	return ShortDramaV2PromptDraft{
		ImagePrompt:      fmt.Sprintf("短剧前贴视觉设定，%s。画面主题：%s。视觉元素：%s。保持原创人物比例、电影感光影、明确主体、无品牌标识、无额外文字。", analysis.Content.Tone, direction.VisualIntent, keywords),
		VideoDescription: fmt.Sprintf("%d 秒独立短剧前贴。%s 结尾字幕：点击观看正片。", duration, direction.HookCopy),
		VideoPrompt:      fmt.Sprintf("生成 %d 秒的独立短剧前贴视频。开场立即建立视觉锚点：%s；中段围绕%s推进；最后 1 秒保留悬念并尝试生成简短字幕“点击观看正片”。人物、时代、场景保持连续，不虚构输入剧情之外的事实。", duration, direction.VisualIntent, direction.HookCopy),
		CompilerVersion:  "deterministic/short-drama-v2-prompt-v1",
	}, nil
}

type FallbackShortDramaV2Planner struct {
	Primary          ShortDramaV2Planner
	Fallback         ShortDramaV2Planner
	OnPrimaryFailure func(error)
}

func (p FallbackShortDramaV2Planner) PlanDirections(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, analysis ShortDramaV2Analysis) ([]ShortDramaV2HookDirection, string, error) {
	if p.Primary != nil {
		directions, version, err := p.Primary.PlanDirections(ctx, actor, project, analysis)
		if err == nil {
			return directions, version, nil
		}
		if p.OnPrimaryFailure != nil {
			p.OnPrimaryFailure(err)
		}
	}
	if p.Fallback == nil {
		return nil, "", fmt.Errorf("short drama V2 direction planner and fallback are unavailable")
	}
	directions, version, err := p.Fallback.PlanDirections(ctx, actor, project, analysis)
	if err != nil {
		return nil, "", err
	}
	return directions, "fallback:" + version, nil
}

func (p FallbackShortDramaV2Planner) CompilePrompts(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, analysis ShortDramaV2Analysis, direction ShortDramaV2HookDirection, duration int) (ShortDramaV2PromptDraft, error) {
	if p.Primary != nil {
		prompt, err := p.Primary.CompilePrompts(ctx, actor, project, analysis, direction, duration)
		if err == nil {
			return prompt, nil
		}
		if p.OnPrimaryFailure != nil {
			p.OnPrimaryFailure(err)
		}
	}
	if p.Fallback == nil {
		return ShortDramaV2PromptDraft{}, fmt.Errorf("short drama V2 prompt compiler and fallback are unavailable")
	}
	prompt, err := p.Fallback.CompilePrompts(ctx, actor, project, analysis, direction, duration)
	if err != nil {
		return ShortDramaV2PromptDraft{}, err
	}
	prompt.CompilerVersion = "fallback:" + prompt.CompilerVersion
	return prompt, nil
}

func (p ModelShortDramaV2Planner) PlanDirections(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, analysis ShortDramaV2Analysis) ([]ShortDramaV2HookDirection, string, error) {
	if p.Text == nil || strings.TrimSpace(p.ModelAlias) == "" {
		return nil, "", fmt.Errorf("short drama V2 model planner is not configured")
	}
	input, err := json.Marshal(analysis.Content)
	if err != nil {
		return nil, "", err
	}
	response, err := p.Text.GenerateText(ctx, provider.TextGenerateRequest{
		Actor: actor, Project: project, ModelAlias: p.ModelAlias,
		InvocationKey: contract.IdempotencyKey("short-drama-v2-directions-" + strings.TrimPrefix(analysis.InputHash, "sha256:")),
		Messages: []provider.TextMessage{
			{Role: provider.TextRoleSystem, Content: "你是短剧前贴编导。严格基于输入事实生成恰好4个方向：前2个category=curiosity，后2个category=summary。每个方向必须引用输入中的evidence id，不得虚构人物、关系、事件或结局。方向之间必须是机制差异，不是换词。只返回JSON。"},
			{Role: provider.TextRoleUser, Content: string(input)},
		},
		OutputJSONSchema: shortDramaV2DirectionSchema,
	})
	if err != nil {
		return nil, "", err
	}
	var decoded struct {
		Directions []ShortDramaV2HookDirection `json:"directions"`
	}
	if err := json.Unmarshal(response.StructuredOutput, &decoded); err != nil {
		return nil, "", fmt.Errorf("decode short drama V2 directions: %w", err)
	}
	return decoded.Directions, "model:" + response.ProviderCode + "/" + response.ModelVersion, nil
}

func (p ModelShortDramaV2Planner) CompilePrompts(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, analysis ShortDramaV2Analysis, direction ShortDramaV2HookDirection, duration int) (ShortDramaV2PromptDraft, error) {
	input, err := json.Marshal(struct {
		Analysis  ShortDramaV2AnalysisContent `json:"analysis"`
		Direction ShortDramaV2HookDirection   `json:"direction"`
		Duration  int                         `json:"duration_seconds"`
	}{analysis.Content, direction, duration})
	if err != nil {
		return ShortDramaV2PromptDraft{}, err
	}
	response, err := p.Text.GenerateText(ctx, provider.TextGenerateRequest{
		Actor: actor, Project: project, ModelAlias: p.ModelAlias,
		InvocationKey: contract.IdempotencyKey(fmt.Sprintf("short-drama-v2-prompts-%s-%d", direction.ID, duration)),
		Messages: []provider.TextMessage{
			{Role: provider.TextRoleSystem, Content: "你是短剧前贴生成提示词编译器。输出视觉参考图片提示词、业务可读视频描述、可执行视频提示词。保留用户创作意图，强化故事、情绪、氛围和视觉锚点，弱化机械分镜语言。视频为独立前贴，不要声称已经拼接。字幕由视频模型尝试生成，文字短而少。不得虚构输入事实。只返回JSON。"},
			{Role: provider.TextRoleUser, Content: string(input)},
		},
		OutputJSONSchema: shortDramaV2PromptSchema,
	})
	if err != nil {
		return ShortDramaV2PromptDraft{}, err
	}
	var prompt ShortDramaV2PromptDraft
	if err := json.Unmarshal(response.StructuredOutput, &prompt); err != nil {
		return ShortDramaV2PromptDraft{}, fmt.Errorf("decode short drama V2 prompts: %w", err)
	}
	prompt.CompilerVersion = "short-drama-prompt/model-v1"
	return prompt, nil
}
