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

const aiNativeStoryboardPromptVersion = "ai-ad-storyboard/douyin/v1"

type AINativeStoryboardPlanner interface {
	Plan(context.Context, contract.ActorContext, contract.ProjectContext, AINativeRequirementDraft, AINativeScriptRevision, ChannelCreativeProfile) (AINativeStoryboardRevision, error)
}

type ModelAINativeStoryboardPlanner struct {
	Text       AINativeScriptTextGenerator
	ModelAlias string
	Now        func() time.Time
}

type modelAINativeStoryboard struct {
	Assets []modelAINativeStoryboardAsset `json:"assets"`
	Shots  []modelAINativeStoryboardShot  `json:"shots"`
}

type modelAINativeStoryboardAsset struct {
	ID              string `json:"id"`
	Role            string `json:"role"`
	Name            string `json:"name"`
	GenerationBrief string `json:"generation_brief"`
}

type modelAINativeStoryboardShot struct {
	ID                      string   `json:"id"`
	StartMS                 int      `json:"start_ms"`
	EndMS                   int      `json:"end_ms"`
	VisualContent           string   `json:"visual_content"`
	SubjectsProductsActions string   `json:"subjects_products_actions"`
	ShotSize                string   `json:"shot_size"`
	CameraMovement          string   `json:"camera_movement"`
	ReferenceAssetIDs       []string `json:"reference_asset_ids"`
	Voiceover               string   `json:"voiceover"`
	Subtitle                string   `json:"subtitle"`
	SoundEffect             string   `json:"sound_effect"`
	BGMDirection            string   `json:"bgm_direction"`
	Transition              string   `json:"transition"`
	ProductIdentityRequired bool     `json:"product_identity_required"`
}

type aiNativeStoryboardAssetReferenceRewrite struct {
	keepOriginal bool
	replacements []string
}

var aiNativeStoryboardSchema = json.RawMessage(`{
  "type":"object","additionalProperties":false,"required":["assets","shots"],
  "properties":{
    "assets":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["id","role","name","generation_brief"],
      "properties":{"id":{"type":"string"},"role":{"type":"string","enum":["person_identity","scene_reference","composition_reference"]},"name":{"type":"string"},"generation_brief":{"type":"string"}}}},
    "shots":{"type":"array","minItems":1,"maxItems":12,"items":{"type":"object","additionalProperties":false,
      "required":["id","start_ms","end_ms","visual_content","subjects_products_actions","shot_size","camera_movement","reference_asset_ids","voiceover","subtitle","sound_effect","bgm_direction","transition","product_identity_required"],
      "properties":{"id":{"type":"string"},"start_ms":{"type":"integer","minimum":0},"end_ms":{"type":"integer","minimum":1},"visual_content":{"type":"string"},"subjects_products_actions":{"type":"string"},"shot_size":{"type":"string"},"camera_movement":{"type":"string"},"reference_asset_ids":{"type":"array","items":{"type":"string"}},"voiceover":{"type":"string"},"subtitle":{"type":"string"},"sound_effect":{"type":"string"},"bgm_direction":{"type":"string"},"transition":{"type":"string"},"product_identity_required":{"type":"boolean"}}}}
  }
}`)

func (p ModelAINativeStoryboardPlanner) Plan(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, requirement AINativeRequirementDraft, script AINativeScriptRevision, profile ChannelCreativeProfile) (AINativeStoryboardRevision, error) {
	if p.Text == nil || strings.TrimSpace(p.ModelAlias) == "" {
		return AINativeStoryboardRevision{}, fmt.Errorf("AI native storyboard text model is unavailable")
	}
	productAssets := storyboardProductAssets(requirement)
	if len(productAssets) == 0 {
		return AINativeStoryboardRevision{}, fmt.Errorf("AI native storyboard requires at least one imported product AssetVersionRef")
	}
	input, err := json.Marshal(map[string]any{"requirement": requirement, "confirmed_script": script, "channel_profile": profile, "fixed_product_assets": productAssets})
	if err != nil {
		return AINativeStoryboardRevision{}, err
	}
	now := time.Now
	if p.Now != nil {
		now = p.Now
	}
	started := now()
	response, err := p.generate(ctx, actor, project, string(input), false)
	if err != nil {
		return AINativeStoryboardRevision{}, err
	}
	value, decodeErr := decodeModelAINativeStoryboard(response, requirement, script, profile, productAssets, now().Sub(started))
	if decodeErr == nil {
		return value, nil
	}
	repairInput, _ := json.Marshal(map[string]any{"invalid_output": rawTextResponse(response), "validation_error": decodeErr.Error(), "required_duration_ms": requirement.DurationSeconds * 1000, "fixed_product_assets": productAssets})
	repaired, repairErr := p.generate(ctx, actor, project, string(repairInput), true)
	if repairErr != nil {
		return AINativeStoryboardRevision{}, fmt.Errorf("repair AI native storyboard: %w", repairErr)
	}
	value, repairErr = decodeModelAINativeStoryboard(repaired, requirement, script, profile, productAssets, now().Sub(started))
	if repairErr != nil {
		return AINativeStoryboardRevision{}, fmt.Errorf("AI native storyboard remained invalid after one repair: %w", repairErr)
	}
	return value, nil
}

func (p ModelAINativeStoryboardPlanner) generate(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, input string, repair bool) (provider.SynchronousResponse, error) {
	system := "你是抖音效果广告故事板导演。只使用已确认需求、脚本和固定商品素材 ID。输出完整闭合时间线；每个镜头必须填写画面、人物商品动作、景别、运镜、参考素材、旁白、字幕、音效、BGM 和转场。商品出现时必须引用 fixed_product_assets 中的真实素材，绝不计划或生成替代商品图。assets 只能规划人物、场景和构图素材，ID 必须分别使用 person_、scene_、composition_ 前缀；严禁输出 product_ 前缀或复用 fixed_product_assets 的任何 ID。只输出符合 JSON Schema 的 JSON。"
	if repair {
		system = "你是故事板结构修复器。只修复字段完整性、闭合时间线和素材引用。不得新增商品事实，不得用 AI 素材替代 fixed_product_assets。assets 只能使用 person_、scene_、composition_ 前缀，严禁复用 fixed_product_assets 的任何 ID。只输出符合 JSON Schema 的 JSON。"
	}
	return p.Text.GenerateText(ctx, provider.TextGenerateRequest{Actor: actor, Project: project, ModelAlias: p.ModelAlias,
		Messages: []provider.TextMessage{{Role: provider.TextRoleSystem, Content: system}, {Role: provider.TextRoleUser, Content: input}}, OutputJSONSchema: aiNativeStoryboardSchema})
}

func storyboardProductAssets(requirement AINativeRequirementDraft) []AINativeStoryboardAsset {
	result := make([]AINativeStoryboardAsset, 0, len(requirement.Media))
	for _, media := range requirement.Media {
		if media.AssetRef == nil || media.AssetRef.Validate() != nil {
			continue
		}
		ref := *media.AssetRef
		result = append(result, AINativeStoryboardAsset{ID: fmt.Sprintf("product_%d", len(result)+1), Role: AINativeStoryboardAssetRoleProductIdentity,
			Name: fmt.Sprintf("商品素材 %d", len(result)+1), Source: AINativeStoryboardAssetSourceProductImport, AssetRef: &ref, Status: AINativeStoryboardAssetReady})
	}
	return result
}

func decodeModelAINativeStoryboard(response provider.SynchronousResponse, requirement AINativeRequirementDraft, script AINativeScriptRevision, profile ChannelCreativeProfile, productAssets []AINativeStoryboardAsset, latency time.Duration) (AINativeStoryboardRevision, error) {
	raw := response.StructuredOutput
	if len(raw) == 0 {
		raw = json.RawMessage(strings.TrimSpace(response.Text))
	}
	var output modelAINativeStoryboard
	if err := json.Unmarshal(raw, &output); err != nil {
		return AINativeStoryboardRevision{}, fmt.Errorf("decode AI native storyboard output: %w", err)
	}
	assets := append([]AINativeStoryboardAsset{}, productAssets...)
	usedAssetIDs := make(map[string]bool, len(productAssets)+len(output.Assets))
	for _, asset := range productAssets {
		usedAssetIDs[asset.ID] = true
	}
	rewrites := make(map[string]aiNativeStoryboardAssetReferenceRewrite)
	for _, planned := range output.Assets {
		rawID := strings.TrimSpace(planned.ID)
		role := strings.TrimSpace(planned.Role)
		prefix := storyboardGeneratedAssetIDPrefix(role)
		normalizedID := rawID
		collides := usedAssetIDs[rawID]
		if rawID == "" || collides || !strings.HasPrefix(rawID, prefix+"_") {
			normalizedID = nextStoryboardGeneratedAssetID(prefix, usedAssetIDs)
			rewrite := rewrites[rawID]
			rewrite.keepOriginal = rewrite.keepOriginal || collides
			rewrite.replacements = append(rewrite.replacements, normalizedID)
			rewrites[rawID] = rewrite
		}
		usedAssetIDs[normalizedID] = true
		assets = append(assets, AINativeStoryboardAsset{ID: normalizedID, Role: role, Name: strings.TrimSpace(planned.Name),
			Source: AINativeStoryboardAssetSourceAIGenerated, GenerationBrief: strings.TrimSpace(planned.GenerationBrief), Status: AINativeStoryboardAssetPlanned})
	}
	shots := make([]AINativeStoryboardShot, 0, len(output.Shots))
	for _, shot := range output.Shots {
		referenceAssetIDs := rewriteStoryboardAssetReferences(shot.ReferenceAssetIDs, rewrites)
		shots = append(shots, AINativeStoryboardShot{ID: strings.TrimSpace(shot.ID), StartMS: shot.StartMS, EndMS: shot.EndMS, DurationMS: shot.EndMS - shot.StartMS,
			VisualContent: strings.TrimSpace(shot.VisualContent), SubjectsProductsActions: strings.TrimSpace(shot.SubjectsProductsActions), ShotSize: strings.TrimSpace(shot.ShotSize), CameraMovement: strings.TrimSpace(shot.CameraMovement),
			ReferenceAssetIDs: referenceAssetIDs, Voiceover: strings.TrimSpace(shot.Voiceover), Subtitle: strings.TrimSpace(shot.Subtitle), SoundEffect: strings.TrimSpace(shot.SoundEffect),
			BGMDirection: strings.TrimSpace(shot.BGMDirection), Transition: strings.TrimSpace(shot.Transition), ProductIdentityRequired: shot.ProductIdentityRequired})
	}
	requirementHash, err := contract.CanonicalJSONHash(requirement)
	if err != nil {
		return AINativeStoryboardRevision{}, err
	}
	scriptHash, err := contract.CanonicalJSONHash(script)
	if err != nil {
		return AINativeStoryboardRevision{}, err
	}
	generation := AINativeStoryboardGenerationMetadata{ModelAlias: response.ModelAlias, ModelVersion: response.ModelVersion, RouteRevisionID: response.RouteRevisionID,
		PromptVersion: aiNativeStoryboardPromptVersion, ProfileHash: profile.ContentHash, LatencyMS: latency.Milliseconds()}
	if response.Usage != nil {
		generation.InputTokens, generation.OutputTokens, generation.TotalTokens = response.Usage.InputTokens, response.Usage.OutputTokens, response.Usage.TotalTokens
	}
	value := AINativeStoryboardRevision{ContractVersion: aiNativeStoryboardContract, Revision: 1, Status: AINativeStoryboardDraftStatus, DurationSeconds: requirement.DurationSeconds,
		Assets: assets, Shots: shots, ChannelProfileID: profile.ID, ChannelProfileHash: profile.ContentHash, BasedOnRequirementRevision: requirement.Revision,
		BasedOnRequirementHash: requirementHash, BasedOnScriptRevision: script.Revision, BasedOnScriptHash: scriptHash, Generation: generation}
	if err := value.ValidatePlanAgainst(requirement, script); err != nil {
		return AINativeStoryboardRevision{}, err
	}
	return value, nil
}

func storyboardGeneratedAssetIDPrefix(role string) string {
	switch role {
	case AINativeStoryboardAssetRolePersonIdentity:
		return "person"
	case AINativeStoryboardAssetRoleSceneReference:
		return "scene"
	case AINativeStoryboardAssetRoleCompositionReference:
		return "composition"
	default:
		return "generated"
	}
}

func nextStoryboardGeneratedAssetID(prefix string, used map[string]bool) string {
	for ordinal := 1; ; ordinal++ {
		candidate := fmt.Sprintf("%s_%d", prefix, ordinal)
		if !used[candidate] {
			return candidate
		}
	}
}

func rewriteStoryboardAssetReferences(referenceIDs []string, rewrites map[string]aiNativeStoryboardAssetReferenceRewrite) []string {
	result := make([]string, 0, len(referenceIDs))
	seen := make(map[string]bool, len(referenceIDs))
	appendUnique := func(value string) {
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		result = append(result, value)
	}
	for _, rawID := range referenceIDs {
		assetID := strings.TrimSpace(rawID)
		rewrite, ok := rewrites[assetID]
		if !ok {
			appendUnique(assetID)
			continue
		}
		if rewrite.keepOriginal {
			appendUnique(assetID)
		}
		for _, replacement := range rewrite.replacements {
			appendUnique(replacement)
		}
	}
	return result
}
