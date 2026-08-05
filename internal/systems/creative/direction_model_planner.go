package creative

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

const creativeDirectionPromptVersion = "creative-direction/strategy-handoff-v3"

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
	generationToken := strings.TrimSpace(planningContext.GenerationID)
	if len(generationToken) > 20 {
		generationToken = generationToken[len(generationToken)-20:]
	}
	if generationToken == "" {
		generationToken = "initial"
	}
	systemPrompt := "你是 CreativeDirection 规划器。输入中的策略事实和约束不可改写；从这里开始才允许提出创意概念。" +
		"只输出结构化候选，不生成完整脚本、逐镜分镜或模型提示词。" +
		"每个候选必须说明概念依据、信息顺序、执行轮廓，并逐条回溯 guardrail。" +
		"候选之间必须在核心张力、叙事机制和视觉语言上有实质差异；不得只替换标题，不得补造产品功效、资产权利、受众事实或 CTA。" +
		"所有概念、理由、信息计划和执行文案都按对外广告主张审核，禁止排名第一、最好、最优、首选、必买、必囤、神器、神仙、不踩雷、保证、绝对、永久、零风险、治愈等绝对化或无法证实的表达；guardrail_trace 的自我声明不能替代正文合规。"
	if planningContext.SelectedRoute.RouteType == CreativeRouteBrandVideo {
		systemPrompt += "当前路线是品牌视频。目标不是教程或转化清单，而是建立可复述的品牌认知与感受。" +
			"每个候选必须填写 direction_mode、emotional_arc、visual_grammar、brand_memory_device、human_moment；direction_mode 只能是 emotional、cinematic、utility。" +
			"至少两个候选必须是 emotional 或 cinematic，最多一个候选可为 utility。禁止让“指南、清单、三步、避坑、工具、教程、科普、方法论”成为两个或更多候选的核心概念。" +
			"三个候选应分别探索不同品牌领地，例如人物情绪弧、工程世界的视觉隐喻、品牌仪式或记忆符号；开场可以渠道原生，但不能以促销 CTA 或知识教学主导。品牌片结尾只允许回到品牌主张或品牌身份，不得使用点击了解、点击购买、评论区领取、私信、扣1、立即咨询等面向观众的效果 CTA；人物在工作场景中点击确认按钮属于叙事动作，不是 CTA。" +
			"不得擅自增加 4K、8K 等输入未确认的制作规格。emotional 或 cinematic 候选不得以指南、清单、三步、避坑、教程、科普、方法论或判断/核验工具为内容机制；可以客观说明产品本身属于效率工具。" +
			"emotional_arc 要写清从何种感受到何种感受；visual_grammar 要写镜头、光线、节奏或声音的统一法则；brand_memory_device 要落实为可重复的颜色、声音、动作、构图或文案装置；human_moment 要包含一个具体的人与工程判断瞬间。"
	}
	baseMessages := []provider.TextMessage{
		{
			Role:    provider.TextRoleSystem,
			Content: systemPrompt,
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
				Content: "上一版候选未通过质量校验：" + lastValidationErr.Error() + "。请重新生成全新的候选，修复该问题，并删除所有绝对化、永久性、群体普适性或无法证实的主张；不要只在 guardrail_trace 中声明规避。",
			})
		}
		response, err := p.Text.GenerateText(ctx, provider.TextGenerateRequest{
			Actor: plannerActor, Project: project, ModelAlias: p.ModelAlias,
			InvocationKey:    contract.IdempotencyKey(fmt.Sprintf("direction_%s_%d_%s_v3_%d", hashToken, candidateCount, generationToken, attempt)),
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
			lastValidationErr = validateDirectionBatchQuality(planningContext, output.Candidates)
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
          },
          "direction_mode": {"type": "string", "enum": ["emotional", "cinematic", "utility"]},
          "emotional_arc": {"type": "string", "minLength": 1, "maxLength": 500},
          "visual_grammar": {"type": "string", "minLength": 1, "maxLength": 500},
          "brand_memory_device": {"type": "string", "minLength": 1, "maxLength": 500},
          "human_moment": {"type": "string", "minLength": 1, "maxLength": 500}
        }
      }
    }
  }
}`)
