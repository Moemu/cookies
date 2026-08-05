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

type GamePrerollPlanner interface {
	Plan(
		context.Context,
		contract.ActorContext,
		contract.ProjectContext,
		GamePrerollInputSnapshot,
		string,
		string,
		int64,
		GamePrerollGenerationConfig,
		time.Time,
	) (GameCandidateBatch, error)
}

type GamePrerollTextGenerator interface {
	GenerateText(context.Context, provider.TextGenerateRequest) (provider.SynchronousResponse, error)
}

type DeterministicGamePrerollPlanner struct{}

func (DeterministicGamePrerollPlanner) Plan(
	_ context.Context,
	_ contract.ActorContext,
	_ contract.ProjectContext,
	snapshot GamePrerollInputSnapshot,
	inputHash string,
	batchID string,
	revision int64,
	config GamePrerollGenerationConfig,
	now time.Time,
) (GameCandidateBatch, error) {
	return planGamePrerollCandidateBatch(snapshot, inputHash, batchID, revision, config, now)
}

type FallbackGamePrerollPlanner struct {
	Primary          GamePrerollPlanner
	Fallback         GamePrerollPlanner
	OnPrimaryFailure func(error)
}

func (p FallbackGamePrerollPlanner) Plan(
	ctx context.Context,
	actor contract.ActorContext,
	project contract.ProjectContext,
	snapshot GamePrerollInputSnapshot,
	inputHash string,
	batchID string,
	revision int64,
	config GamePrerollGenerationConfig,
	now time.Time,
) (GameCandidateBatch, error) {
	if p.Primary != nil {
		batch, err := p.Primary.Plan(ctx, actor, project, snapshot, inputHash, batchID, revision, config, now)
		if err == nil {
			return batch, nil
		}
		if p.OnPrimaryFailure != nil {
			p.OnPrimaryFailure(err)
		}
	}
	if p.Fallback == nil {
		return GameCandidateBatch{}, fmt.Errorf("game preroll planner and fallback are unavailable")
	}
	batch, err := p.Fallback.Plan(ctx, actor, project, snapshot, inputHash, batchID, revision, config, now)
	if err != nil {
		return GameCandidateBatch{}, err
	}
	batch.PlannerVersion = "fallback:" + batch.PlannerVersion
	return batch, nil
}

type ModelGamePrerollPlanner struct {
	Text       GamePrerollTextGenerator
	ModelAlias string
}

type modelGameCandidateOutline struct {
	HookMechanism       GameHookMechanism `json:"hook_mechanism"`
	ExecutionAngle      string            `json:"execution_angle"`
	PrimaryTestVariable string            `json:"primary_test_variable"`
	VariantHypothesis   string            `json:"variant_hypothesis"`
	HookLine            string            `json:"hook_line"`
	EvidenceMomentIDs   []string          `json:"evidence_moment_ids"`
}

type modelGamePlan struct {
	Candidates []modelGameCandidateOutline `json:"candidates"`
}

func (p ModelGamePrerollPlanner) Plan(
	ctx context.Context,
	actor contract.ActorContext,
	project contract.ProjectContext,
	snapshot GamePrerollInputSnapshot,
	inputHash string,
	batchID string,
	revision int64,
	config GamePrerollGenerationConfig,
	now time.Time,
) (GameCandidateBatch, error) {
	if p.Text == nil || strings.TrimSpace(p.ModelAlias) == "" {
		return GameCandidateBatch{}, fmt.Errorf("game preroll model planner is not configured")
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return GameCandidateBatch{}, fmt.Errorf("encode game planner snapshot: %w", err)
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
				Content: "你是效果广告游戏前贴规划器。只输出 3 个彼此不同的结构化候选，不生成最终视频 Prompt。" +
					"只能使用 allowed_mechanisms 和 evidence_moments；不得推断素材未证明的胜负、奖励、升级、合成或数值结果。" +
					"hook_line 必须简短、可静音理解，并且不得改写游戏 UI 文案。" +
					"三个候选的顺序固定为：选择挑战、战术取舍、波次压力；机制与证据由服务端绑定，你只规划表达角度、测试变量、假设和钩子文案。",
			},
			{
				Role: provider.TextRoleUser,
				Content: "请为以下不可变事实快照规划 3 个完整 6 秒候选，并严格保持既定顺序。" +
					"\nINPUT_SNAPSHOT=" + string(snapshotJSON),
			},
		},
		OutputJSONSchema: gamePlannerOutputJSONSchema,
	})
	if err != nil {
		return GameCandidateBatch{}, fmt.Errorf("game preroll model planning: %w", err)
	}
	raw := response.StructuredOutput
	if len(raw) == 0 {
		raw = json.RawMessage(response.Text)
	}
	var planned modelGamePlan
	if err := json.Unmarshal(raw, &planned); err != nil {
		return GameCandidateBatch{}, fmt.Errorf("decode game planner output: %w", err)
	}
	outlines, err := modelGameOutlines(snapshot, planned)
	if err != nil {
		return GameCandidateBatch{}, err
	}
	return compileGamePrerollCandidateBatch(
		snapshot,
		inputHash,
		batchID,
		revision,
		config,
		outlines,
		fmt.Sprintf("model:%s/%s", response.ProviderCode, response.ModelVersion),
		now,
	)
}

func modelGameOutlines(snapshot GamePrerollInputSnapshot, planned modelGamePlan) ([]gameCandidateOutline, error) {
	if len(planned.Candidates) != 3 {
		return nil, fmt.Errorf("game model planner must return exactly three candidates")
	}
	templates := defaultGameCandidateOutlines(snapshot)
	seenHookLines := make(map[string]struct{}, 3)
	outlines := make([]gameCandidateOutline, 0, 3)
	for index, candidate := range planned.Candidates {
		template := templates[index]
		if candidate.HookMechanism != "" {
			for _, approved := range templates {
				if approved.Mechanism == candidate.HookMechanism {
					template = approved
					break
				}
			}
			if template.Mechanism != candidate.HookMechanism {
				return nil, fmt.Errorf("game model planner used unapproved hook mechanism %q", candidate.HookMechanism)
			}
		}
		if strings.TrimSpace(candidate.ExecutionAngle) == "" ||
			strings.TrimSpace(candidate.PrimaryTestVariable) == "" ||
			strings.TrimSpace(candidate.VariantHypothesis) == "" ||
			strings.TrimSpace(candidate.HookLine) == "" {
			return nil, fmt.Errorf("game model planner returned an incomplete candidate")
		}
		normalizedHookLine := strings.TrimSpace(candidate.HookLine)
		if _, duplicate := seenHookLines[normalizedHookLine]; duplicate {
			return nil, fmt.Errorf("game model planner duplicated hook line")
		}
		template.ExecutionAngle = candidate.ExecutionAngle
		template.PrimaryTestVariable = candidate.PrimaryTestVariable
		template.VariantHypothesis = candidate.VariantHypothesis
		template.HookLine = candidate.HookLine
		if len(candidate.EvidenceMomentIDs) > 0 {
			if !gameEvidenceIDsExist(snapshot.EvidenceMoments, candidate.EvidenceMomentIDs) {
				return nil, fmt.Errorf("game model planner returned ungrounded evidence")
			}
			template.EvidenceMomentIDs = append([]string{}, candidate.EvidenceMomentIDs...)
		}
		outlines = append(outlines, template)
		seenHookLines[normalizedHookLine] = struct{}{}
	}
	return outlines, nil
}

var gamePlannerOutputJSONSchema = json.RawMessage(`{
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
          "hook_line"
        ],
        "properties": {
          "execution_angle": {"type": "string", "minLength": 1, "maxLength": 80},
          "primary_test_variable": {"type": "string", "minLength": 1, "maxLength": 80},
          "variant_hypothesis": {"type": "string", "minLength": 1, "maxLength": 240},
          "hook_line": {"type": "string", "minLength": 1, "maxLength": 80}
        }
      }
    }
  }
}`)
