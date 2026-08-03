package creative

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

const creativeDirectionPromptVersion = "creative-direction/strategy-handoff-v2"

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
	baseMessages := []provider.TextMessage{
		{
			Role: provider.TextRoleSystem,
			Content: "你是 CreativeDirection 规划器。输入中的策略事实和约束不可改写；从这里开始才允许提出创意概念。" +
				"只输出结构化候选，不生成完整脚本、逐镜分镜或模型提示词。" +
				"每个候选必须说明概念依据、信息顺序、执行轮廓，并逐条回溯 guardrail。" +
				"候选之间必须有实质差异；不得补造产品功效、资产权利、受众事实或 CTA。" +
				"所有概念、理由、信息计划和执行文案都按对外广告主张审核，禁止排名第一、最好、最优、首选、必买、必囤、神器、神仙、不踩雷、保证、绝对、永久、零风险、治愈等绝对化或无法证实的表达；guardrail_trace 的自我声明不能替代正文合规。",
		},
		{
			Role: provider.TextRoleUser,
			Content: fmt.Sprintf(
				"请生成 %d 个 CreativeDirection 候选。\nPLANNING_CONTEXT=%s",
				candidateCount, payload,
			),
		},
	}
	var lastValidationErr error
	for attempt := 0; attempt < 2; attempt++ {
		messages := append([]provider.TextMessage{}, baseMessages...)
		if attempt > 0 {
			messages = append(messages, provider.TextMessage{
				Role:    provider.TextRoleSystem,
				Content: "上一版候选未通过广告主张校验。请重新生成全新的候选，删除所有绝对化、永久性、群体普适性或无法证实的主张；不要只在 guardrail_trace 中声明规避。",
			})
		}
		response, err := p.Text.GenerateText(ctx, provider.TextGenerateRequest{
			Actor: plannerActor, Project: project, ModelAlias: p.ModelAlias,
			InvocationKey:    contract.IdempotencyKey(fmt.Sprintf("direction_%s_%d_v2_%d", hashToken, candidateCount, attempt)),
			Messages:         messages,
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
			lastValidationErr = fmt.Errorf("decode creative direction output: %w", err)
			continue
		}
		if len(output.Candidates) != candidateCount {
			lastValidationErr = fmt.Errorf(
				"creative direction model returned %d candidates; expected %d",
				len(output.Candidates), candidateCount,
			)
			continue
		}
		lastValidationErr = nil
		for _, candidate := range output.Candidates {
			if err := candidate.Validate(); err != nil {
				lastValidationErr = err
				break
			}
			if err := validateDirectionCandidateClaims(candidate); err != nil {
				lastValidationErr = err
				break
			}
		}
		if lastValidationErr == nil {
			return DirectionPlannerResult{
				Candidates:    output.Candidates,
				Model:         fmt.Sprintf("%s/%s", response.ProviderCode, response.ModelVersion),
				PromptVersion: creativeDirectionPromptVersion,
			}, nil
		}
	}
	return DirectionPlannerResult{}, fmt.Errorf("creative direction model output failed validation after repair: %w", lastValidationErr)
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
