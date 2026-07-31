package creative

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

type ShortDramaPrerollPlanner interface {
	Plan(
		context.Context,
		contract.ActorContext,
		contract.ProjectContext,
		ShortDramaPrerollInputSnapshot,
		string,
		string,
		int64,
		ShortDramaGenerationConfig,
		string,
		time.Time,
	) (ShortDramaCandidateBatch, error)
}

type ShortDramaPrerollTextGenerator interface {
	GenerateText(context.Context, provider.TextGenerateRequest) (provider.SynchronousResponse, error)
}

type DeterministicShortDramaPrerollPlanner struct{}

func (DeterministicShortDramaPrerollPlanner) Plan(
	_ context.Context,
	_ contract.ActorContext,
	_ contract.ProjectContext,
	snapshot ShortDramaPrerollInputSnapshot,
	inputHash string,
	batchID string,
	revision int64,
	config ShortDramaGenerationConfig,
	variationIntent string,
	now time.Time,
) (ShortDramaCandidateBatch, error) {
	return planShortDramaCandidateBatch(
		snapshot,
		inputHash,
		batchID,
		revision,
		config,
		variationIntent,
		now,
	)
}

type FallbackShortDramaPrerollPlanner struct {
	Primary          ShortDramaPrerollPlanner
	Fallback         ShortDramaPrerollPlanner
	OnPrimaryFailure func(error)
}

func (p FallbackShortDramaPrerollPlanner) Plan(
	ctx context.Context,
	actor contract.ActorContext,
	project contract.ProjectContext,
	snapshot ShortDramaPrerollInputSnapshot,
	inputHash string,
	batchID string,
	revision int64,
	config ShortDramaGenerationConfig,
	variationIntent string,
	now time.Time,
) (ShortDramaCandidateBatch, error) {
	if p.Primary != nil {
		batch, err := p.Primary.Plan(
			ctx,
			actor,
			project,
			snapshot,
			inputHash,
			batchID,
			revision,
			config,
			variationIntent,
			now,
		)
		if err == nil {
			return batch, nil
		}
		if p.OnPrimaryFailure != nil {
			p.OnPrimaryFailure(err)
		}
	}
	if p.Fallback == nil {
		return ShortDramaCandidateBatch{}, fmt.Errorf("short drama preroll planner and fallback are unavailable")
	}
	batch, err := p.Fallback.Plan(
		ctx,
		actor,
		project,
		snapshot,
		inputHash,
		batchID,
		revision,
		config,
		variationIntent,
		now,
	)
	if err != nil {
		return ShortDramaCandidateBatch{}, err
	}
	batch.PlannerVersion = "fallback:" + batch.PlannerVersion
	return batch, nil
}

type ModelShortDramaPrerollPlanner struct {
	Text       ShortDramaPrerollTextGenerator
	ModelAlias string
}

type modelShortDramaCandidateOutline struct {
	ExecutionAngle      string `json:"execution_angle"`
	PrimaryTestVariable string `json:"primary_test_variable"`
	VariantHypothesis   string `json:"variant_hypothesis"`
	GroundingQuote      string `json:"grounding_quote"`
	HookLine            string `json:"hook_line"`
	OpeningVisual       string `json:"opening_visual"`
	MiddleVisual        string `json:"middle_visual"`
	MiddleCopy          string `json:"middle_copy"`
	TransitionLine      string `json:"transition_line"`
}

type modelShortDramaPlan struct {
	Candidates []modelShortDramaCandidateOutline `json:"candidates"`
}

func (p ModelShortDramaPrerollPlanner) Plan(
	ctx context.Context,
	actor contract.ActorContext,
	project contract.ProjectContext,
	snapshot ShortDramaPrerollInputSnapshot,
	inputHash string,
	batchID string,
	revision int64,
	config ShortDramaGenerationConfig,
	variationIntent string,
	now time.Time,
) (ShortDramaCandidateBatch, error) {
	if p.Text == nil || strings.TrimSpace(p.ModelAlias) == "" {
		return ShortDramaCandidateBatch{}, fmt.Errorf("short drama preroll model planner is not configured")
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return ShortDramaCandidateBatch{}, fmt.Errorf("encode short drama planner snapshot: %w", err)
	}
	plannerActor := contract.ActorContext{
		OrganizationID: actor.OrganizationID,
		Principal:      actor.Principal,
		Scopes:         []contract.Scope{provider.ScopeTextGenerate},
	}
	response, err := p.Text.GenerateText(ctx, provider.TextGenerateRequest{
		Actor:         plannerActor,
		Project:       project,
		ModelAlias:    p.ModelAlias,
		InvocationKey: contract.IdempotencyKey(fmt.Sprintf("%s_%d", batchID, revision)),
		Messages: []provider.TextMessage{
			{
				Role: provider.TextRoleSystem,
				Content: "你是效果广告短剧前贴编导。只输出 3 个彼此不同的结构化候选，不生成最终视频 Prompt。" +
					"每条候选必须直接由 INPUT_SNAPSHOT 的短剧标题、剧情梗概或已审核剧情卖点支撑。" +
					"grounding_quote 必须逐字复制输入中至少 2 个汉字的连续原文，并且必须真实出现在 hook_line、opening_visual、middle_visual 或 middle_copy 中。" +
					"不得编造人物、关系、证据、时间、地点或结局；不得把通用的豪门、身份反转、全场沉默等模板套到没有这些事实的故事。" +
					"三个 execution_angle 必须互不相同，只能从 dialogue_confrontation、action_reveal、reaction_escalation、result_first 中选择。" +
					"候选是独立 6 秒、9:16 的短剧导流广告，最后由服务端统一加入 CTA 并编译 PromptPackage。",
			},
			{
				Role: provider.TextRoleUser,
				Content: "请根据以下不可变输入生成 3 个剧情强相关候选。" +
					"\nVARIATION_INTENT=" + variationIntent +
					"\nINPUT_SNAPSHOT=" + string(snapshotJSON),
			},
		},
		OutputJSONSchema: shortDramaPlannerOutputJSONSchema,
	})
	if err != nil {
		return ShortDramaCandidateBatch{}, fmt.Errorf("short drama preroll model planning: %w", err)
	}
	raw := response.StructuredOutput
	if len(raw) == 0 {
		raw = json.RawMessage(response.Text)
	}
	var planned modelShortDramaPlan
	if err := json.Unmarshal(raw, &planned); err != nil {
		return ShortDramaCandidateBatch{}, fmt.Errorf("decode short drama planner output: %w", err)
	}
	variants, err := modelShortDramaExecutionVariants(snapshot, planned)
	if err != nil {
		return ShortDramaCandidateBatch{}, err
	}
	batch, err := compileShortDramaCandidateBatch(
		snapshot,
		inputHash,
		batchID,
		revision,
		config,
		variationIntent,
		variants,
		fmt.Sprintf("model:%s/%s", response.ProviderCode, response.ModelVersion),
		now,
	)
	if err != nil {
		return ShortDramaCandidateBatch{}, err
	}
	for _, candidate := range batch.Candidates {
		if !shortDramaCandidateGroundedInSnapshot(snapshot, candidate) {
			return ShortDramaCandidateBatch{}, fmt.Errorf("short drama model candidate %q failed story grounding", candidate.ID)
		}
	}
	return batch, nil
}

func modelShortDramaExecutionVariants(
	snapshot ShortDramaPrerollInputSnapshot,
	planned modelShortDramaPlan,
) ([]shortDramaExecutionVariant, error) {
	if len(planned.Candidates) != 3 {
		return nil, fmt.Errorf("short drama model planner must return exactly three candidates")
	}
	templateByID := make(map[string]shortDramaExecutionVariant, 4)
	for _, template := range shortDramaExecutionVariants(snapshot) {
		templateByID[template.ID] = template
	}
	seenAngles := make(map[string]struct{}, 3)
	seenHooks := make(map[string]struct{}, 3)
	variants := make([]shortDramaExecutionVariant, 0, 3)
	for _, candidate := range planned.Candidates {
		template, ok := templateByID[candidate.ExecutionAngle]
		if !ok {
			return nil, fmt.Errorf("short drama model planner used unsupported execution_angle %q", candidate.ExecutionAngle)
		}
		if _, duplicate := seenAngles[candidate.ExecutionAngle]; duplicate {
			return nil, fmt.Errorf("short drama model planner duplicated execution_angle %q", candidate.ExecutionAngle)
		}
		if strings.TrimSpace(candidate.PrimaryTestVariable) == "" ||
			strings.TrimSpace(candidate.VariantHypothesis) == "" ||
			strings.TrimSpace(candidate.HookLine) == "" ||
			strings.TrimSpace(candidate.OpeningVisual) == "" ||
			strings.TrimSpace(candidate.MiddleVisual) == "" ||
			strings.TrimSpace(candidate.MiddleCopy) == "" ||
			strings.TrimSpace(candidate.TransitionLine) == "" {
			return nil, fmt.Errorf("short drama model planner returned an incomplete candidate")
		}
		if err := validateShortDramaGroundingQuote(snapshot, candidate); err != nil {
			return nil, err
		}
		normalizedHook := strings.TrimSpace(candidate.HookLine)
		if _, duplicate := seenHooks[normalizedHook]; duplicate {
			return nil, fmt.Errorf("short drama model planner duplicated hook_line")
		}
		template.PrimaryTestVariable = candidate.PrimaryTestVariable
		template.VariantHypothesis = candidate.VariantHypothesis
		template.HookLine = candidate.HookLine
		template.OpeningVisual = candidate.OpeningVisual
		template.MiddleVisual = candidate.MiddleVisual
		template.MiddleCopy = candidate.MiddleCopy
		template.TransitionLine = candidate.TransitionLine
		template.GroundingQuote = strings.TrimSpace(candidate.GroundingQuote)
		variants = append(variants, template)
		seenAngles[candidate.ExecutionAngle] = struct{}{}
		seenHooks[normalizedHook] = struct{}{}
	}
	return variants, nil
}

func validateShortDramaGroundingQuote(
	snapshot ShortDramaPrerollInputSnapshot,
	candidate modelShortDramaCandidateOutline,
) error {
	quote := strings.TrimSpace(candidate.GroundingQuote)
	if utf8.RuneCountInString(quote) < 2 || !shortDramaGroundingQuoteExists(snapshot, quote) {
		return fmt.Errorf("short drama model grounding_quote %q is not present in the immutable story input", quote)
	}
	visible := strings.Join([]string{
		candidate.HookLine,
		candidate.OpeningVisual,
		candidate.MiddleVisual,
		candidate.MiddleCopy,
	}, " ")
	if !strings.Contains(visible, quote) {
		return fmt.Errorf("short drama model grounding_quote %q is not used in visible candidate content", quote)
	}
	return nil
}

func shortDramaGroundingQuoteExists(snapshot ShortDramaPrerollInputSnapshot, quote string) bool {
	if strings.Contains(snapshot.StoryTitle, quote) || strings.Contains(snapshot.Synopsis, quote) {
		return true
	}
	for _, point := range snapshot.ReviewedSellingPoints {
		if strings.Contains(point, quote) {
			return true
		}
	}
	return false
}

func shortDramaCandidateGroundedInSnapshot(
	snapshot ShortDramaPrerollInputSnapshot,
	candidate ShortDramaPrerollCandidate,
) bool {
	visible := strings.Join([]string{
		candidate.HookLine,
		candidate.Voiceover,
		candidate.VisualIntent,
		candidate.TransitionLine,
	}, " ")
	for _, beat := range candidate.Storyboard {
		visible += " " + beat.Visual + " " + beat.Copy
	}
	for _, evidence := range candidate.Evidence {
		value := strings.TrimSpace(evidence)
		if utf8.RuneCountInString(value) >= 2 &&
			shortDramaGroundingQuoteExists(snapshot, value) &&
			strings.Contains(visible, value) {
			return true
		}
	}
	if strings.TrimSpace(snapshot.StoryTitle) != "" && strings.Contains(visible, snapshot.StoryTitle) {
		return true
	}
	for _, point := range snapshot.ReviewedSellingPoints {
		if value := strings.TrimSpace(point); utf8.RuneCountInString(value) >= 2 && strings.Contains(visible, value) {
			return true
		}
	}
	for _, moment := range shortDramaSynopsisGroundingMoments(snapshot.Synopsis) {
		if strings.Contains(visible, moment) {
			return true
		}
	}
	return false
}

func shortDramaSynopsisGroundingMoments(synopsis string) []string {
	normalized := strings.NewReplacer(
		"。", "，",
		"！", "，",
		"？", "，",
		"；", "，",
	).Replace(strings.TrimSpace(synopsis))
	parts := strings.Split(normalized, "，")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if utf8.RuneCountInString(value) < 2 {
			continue
		}
		result = append(result, value)
	}
	return result
}

var shortDramaPlannerOutputJSONSchema = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["candidates"],
  "properties": {
    "candidates": {
      "type": "array",
      "minItems": 3,
      "maxItems": 3,
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": [
          "execution_angle",
          "primary_test_variable",
          "variant_hypothesis",
          "grounding_quote",
          "hook_line",
          "opening_visual",
          "middle_visual",
          "middle_copy",
          "transition_line"
        ],
        "properties": {
          "execution_angle": {
            "type": "string",
            "enum": ["dialogue_confrontation", "action_reveal", "reaction_escalation", "result_first"]
          },
          "primary_test_variable": {"type": "string", "minLength": 1, "maxLength": 80},
          "variant_hypothesis": {"type": "string", "minLength": 1, "maxLength": 240},
          "grounding_quote": {"type": "string", "minLength": 2, "maxLength": 80},
          "hook_line": {"type": "string", "minLength": 1, "maxLength": 80},
          "opening_visual": {"type": "string", "minLength": 1, "maxLength": 240},
          "middle_visual": {"type": "string", "minLength": 1, "maxLength": 240},
          "middle_copy": {"type": "string", "minLength": 1, "maxLength": 120},
          "transition_line": {"type": "string", "minLength": 1, "maxLength": 120}
        }
      }
    }
  }
}`)
