package creative

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

const creativeDirectionPromptVersion = "creative-direction/strategy-handoff-v1"

type ModelCreativeDirectionPlanner struct {
	Text       ShortDramaPrerollTextGenerator
	ModelAlias string
}

type modelCreativeDirectionOutput struct {
	Candidates []DirectionCandidate `json:"candidates"`
}

func (p ModelCreativeDirectionPlanner) Generate(
	ctx context.Context,
	actor contract.ActorContext,
	project contract.ProjectContext,
	planningContext CreativePlanningContext,
	candidateCount int,
) (DirectionPlannerResult, error) {
	if p.Text == nil || strings.TrimSpace(p.ModelAlias) == "" {
		return DirectionPlannerResult{}, fmt.Errorf("creative direction model planner is not configured")
	}
	payload, err := json.Marshal(planningContext)
	if err != nil {
		return DirectionPlannerResult{}, fmt.Errorf("encode creative planning context: %w", err)
	}
	plannerActor := contract.ActorContext{
		OrganizationID: actor.OrganizationID,
		Principal:      actor.Principal,
		Scopes:         []contract.Scope{provider.ScopeTextGenerate},
	}
	hashToken := strings.TrimPrefix(planningContext.InputIdentityHash, "sha256:")
	if len(hashToken) > 20 {
		hashToken = hashToken[:20]
	}
	response, err := p.Text.GenerateText(ctx, provider.TextGenerateRequest{
		Actor: plannerActor, Project: project, ModelAlias: p.ModelAlias,
		InvocationKey: contract.IdempotencyKey(fmt.Sprintf("direction_%s_%d", hashToken, candidateCount)),
		Messages: []provider.TextMessage{
			{
				Role: provider.TextRoleSystem,
				Content: "你是 CreativeDirection 规划器。输入中的策略事实和约束不可改写；从这里开始才允许提出创意概念。" +
					"只输出结构化候选，不生成完整脚本、逐镜分镜或模型提示词。" +
					"每个候选必须说明概念依据、信息顺序、执行轮廓，并逐条回溯 guardrail。" +
					"候选之间必须有实质差异；不得补造产品功效、资产权利、受众事实或 CTA。",
			},
			{
				Role: provider.TextRoleUser,
				Content: fmt.Sprintf(
					"请生成 %d 个 CreativeDirection 候选。\nPLANNING_CONTEXT=%s",
					candidateCount, payload,
				),
			},
		},
		OutputJSONSchema: creativeDirectionPlannerOutputSchema,
	})
	if err != nil {
		return DirectionPlannerResult{}, err
	}
	raw := response.StructuredOutput
	if len(raw) == 0 {
		raw = json.RawMessage(response.Text)
	}
	var output modelCreativeDirectionOutput
	if err := json.Unmarshal(raw, &output); err != nil {
		return DirectionPlannerResult{}, fmt.Errorf("decode creative direction output: %w", err)
	}
	if len(output.Candidates) != candidateCount {
		return DirectionPlannerResult{}, fmt.Errorf(
			"creative direction model returned %d candidates; expected %d",
			len(output.Candidates), candidateCount,
		)
	}
	for _, candidate := range output.Candidates {
		if err := candidate.Validate(); err != nil {
			return DirectionPlannerResult{}, err
		}
	}
	return DirectionPlannerResult{
		Candidates:    output.Candidates,
		Model:         fmt.Sprintf("%s/%s", response.ProviderCode, response.ModelVersion),
		PromptVersion: creativeDirectionPromptVersion,
	}, nil
}

var creativeDirectionPlannerOutputSchema = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["candidates"],
  "properties": {
    "candidates": {
      "type": "array",
      "minItems": 2,
      "maxItems": 4,
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": [
          "concept", "creative_rationale", "message_plan",
          "execution_outline", "guardrail_trace"
        ],
        "properties": {
          "concept": {"type": "string", "minLength": 1, "maxLength": 500},
          "creative_rationale": {"type": "string", "minLength": 1, "maxLength": 1000},
          "message_plan": {
            "type": "array", "minItems": 1, "maxItems": 8,
            "items": {"type": "string", "minLength": 1, "maxLength": 500}
          },
          "execution_outline": {
            "type": "array", "minItems": 1, "maxItems": 12,
            "items": {"type": "string", "minLength": 1, "maxLength": 500}
          },
          "guardrail_trace": {
            "type": "array", "maxItems": 20,
            "items": {"type": "string", "minLength": 1, "maxLength": 500}
          }
        }
      }
    }
  }
}`)
