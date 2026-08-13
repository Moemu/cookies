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

const aiNativeScriptPromptVersion = "ai-ad-script/douyin/v1" // legacy fixture compatibility

type AINativeScriptTextGenerator interface {
	GenerateText(context.Context, provider.TextGenerateRequest) (provider.SynchronousResponse, error)
}

type AINativeScriptPlanner interface {
	Plan(context.Context, contract.ActorContext, contract.ProjectContext, AINativeRequirementDraft, ChannelCreativeProfile, string) (AINativeScriptRevision, error)
}

type ModelAINativeScriptPlanner struct {
	Text       AINativeScriptTextGenerator
	ModelAlias string
	Now        func() time.Time
}

type modelAINativeScript struct {
	Title           string                       `json:"title"`
	CreativeSummary string                       `json:"creative_summary"`
	Segments        []modelAINativeScriptSegment `json:"segments"`
}

type modelAINativeScriptSegment struct {
	ID               string   `json:"id"`
	StartMS          int      `json:"start_ms"`
	EndMS            int      `json:"end_ms"`
	Purpose          string   `json:"purpose"`
	VisualIntent     string   `json:"visual_intent"`
	Voiceover        string   `json:"voiceover"`
	Subtitle         string   `json:"subtitle"`
	SellingPointIDs  []string `json:"selling_point_ids"`
	ConversionAction string   `json:"conversion_action"`
}

var aiNativeScriptSchema = json.RawMessage(`{
  "type":"object","additionalProperties":false,"required":["title","creative_summary","segments"],
  "properties":{
    "title":{"type":"string"},"creative_summary":{"type":"string"},
    "segments":{"type":"array","minItems":3,"maxItems":12,"items":{"type":"object","additionalProperties":false,
      "required":["id","start_ms","end_ms","purpose","visual_intent","voiceover","subtitle","selling_point_ids","conversion_action"],
      "properties":{"id":{"type":"string"},"start_ms":{"type":"integer","minimum":0},"end_ms":{"type":"integer","minimum":1},
        "purpose":{"type":"string","enum":["hook","pain","proof","benefit","cta"]},"visual_intent":{"type":"string"},
        "voiceover":{"type":"string"},"subtitle":{"type":"string"},"selling_point_ids":{"type":"array","items":{"type":"string"}},
        "conversion_action":{"type":"string"}}}}
  }
}`)

func (p ModelAINativeScriptPlanner) Plan(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, requirement AINativeRequirementDraft, profile ChannelCreativeProfile, regenerationNote string) (AINativeScriptRevision, error) {
	if p.Text == nil || strings.TrimSpace(p.ModelAlias) == "" {
		return AINativeScriptRevision{}, fmt.Errorf("AI native script text model is unavailable")
	}
	input, err := json.Marshal(map[string]any{"requirement": requirement, "channel_profile": profile, "regeneration_note": strings.TrimSpace(regenerationNote)})
	if err != nil {
		return AINativeScriptRevision{}, err
	}
	now := time.Now
	if p.Now != nil {
		now = p.Now
	}
	started := now()
	response, err := p.generate(ctx, actor, project, string(input), false)
	if err != nil {
		return AINativeScriptRevision{}, err
	}
	script, decodeErr := decodeModelAINativeScript(response, requirement, profile, regenerationNote, now().Sub(started))
	if decodeErr == nil {
		return script, nil
	}
	repairInput, _ := json.Marshal(map[string]any{"invalid_output": rawTextResponse(response), "validation_error": decodeErr.Error(), "required_duration_ms": requirement.DurationSeconds * 1000})
	repaired, repairErr := p.generate(ctx, actor, project, string(repairInput), true)
	if repairErr != nil {
		return AINativeScriptRevision{}, fmt.Errorf("repair AI native script: %w", repairErr)
	}
	script, repairErr = decodeModelAINativeScript(repaired, requirement, profile, regenerationNote, now().Sub(started))
	if repairErr != nil {
		return AINativeScriptRevision{}, fmt.Errorf("AI native script remained invalid after one repair: %w", repairErr)
	}
	return script, nil
}

func (p ModelAINativeScriptPlanner) generate(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, input string, repair bool) (provider.SynchronousResponse, error) {
	system := "你是效果广告脚本策划。严格遵循 requirement.delivery_treatment：旁白和字幕是独立字段，关闭旁白时 voiceover 必须为空，关闭字幕时 subtitle 必须为空，无旁白编辑型字幕仍需完整表达叙事。仅基于已确认商品事实和卖点 ID，一次只输出一份完整脚本。时间线必须连续并严格结束于目标时长，包含 hook、proof 和 cta。不得编造事实。只输出符合 JSON Schema 的 JSON。"
	if repair {
		system = "你是结构修复器。只修复给定脚本的 JSON 结构、时间线闭合和卖点 ID 引用，不增加新商品事实。只输出符合 JSON Schema 的 JSON。"
	}
	return p.Text.GenerateText(ctx, provider.TextGenerateRequest{Actor: actor, Project: project, ModelAlias: p.ModelAlias,
		Messages:         []provider.TextMessage{{Role: provider.TextRoleSystem, Content: system}, {Role: provider.TextRoleUser, Content: input}},
		OutputJSONSchema: aiNativeScriptSchema,
	})
}

func decodeModelAINativeScript(response provider.SynchronousResponse, requirement AINativeRequirementDraft, profile ChannelCreativeProfile, regenerationNote string, latency time.Duration) (AINativeScriptRevision, error) {
	raw := response.StructuredOutput
	if len(raw) == 0 {
		raw = json.RawMessage(strings.TrimSpace(response.Text))
	}
	var output modelAINativeScript
	if err := json.Unmarshal(raw, &output); err != nil {
		return AINativeScriptRevision{}, fmt.Errorf("decode AI native script output: %w", err)
	}
	segments := make([]AINativeScriptSegment, 0, len(output.Segments))
	for _, segment := range output.Segments {
		value := AINativeScriptSegment(segment)
		applyAINativeScriptDeliveryTreatment(&value, requirement.DeliveryTreatment)
		segments = append(segments, value)
	}
	generation := AINativeScriptGenerationMetadata{ModelAlias: response.ModelAlias, ModelVersion: response.ModelVersion,
		RouteRevisionID: response.RouteRevisionID, PromptVersion: profile.PromptVersion, ProfileHash: profile.ContentHash, LatencyMS: latency.Milliseconds()}
	if response.Usage != nil {
		generation.InputTokens, generation.OutputTokens, generation.TotalTokens = response.Usage.InputTokens, response.Usage.OutputTokens, response.Usage.TotalTokens
	}
	value := AINativeScriptRevision{ContractVersion: aiNativeScriptContract, Revision: 1, Status: AINativeScriptDraftStatus,
		Title: strings.TrimSpace(output.Title), CreativeSummary: strings.TrimSpace(output.CreativeSummary), ChannelProfileID: profile.ID,
		ChannelProfileHash: profile.ContentHash, DurationSeconds: requirement.DurationSeconds, Segments: segments,
		RegenerationNote: strings.TrimSpace(regenerationNote), BasedOnRequirementRevision: requirement.Revision, Generation: generation}
	requirementHash, hashErr := contract.CanonicalJSONHash(requirement)
	if hashErr != nil {
		return AINativeScriptRevision{}, hashErr
	}
	value.BasedOnRequirementHash = requirementHash
	if err := value.ValidateAgainst(requirement); err != nil {
		return AINativeScriptRevision{}, err
	}
	return value, nil
}

func applyAINativeScriptDeliveryTreatment(segment *AINativeScriptSegment, treatment AINativeDeliveryTreatment) {
	if treatment.VoiceoverMode == AINativeVoiceoverNone {
		segment.Voiceover = ""
	}
	if treatment.CaptionMode == AINativeCaptionNone {
		segment.Subtitle = ""
	} else if treatment.CaptionMode == AINativeCaptionEditorial && strings.TrimSpace(segment.Subtitle) == "" {
		segment.Subtitle = strings.TrimSpace(segment.Voiceover)
	}
}

func rawTextResponse(response provider.SynchronousResponse) string {
	if len(response.StructuredOutput) > 0 {
		return string(response.StructuredOutput)
	}
	return response.Text
}
