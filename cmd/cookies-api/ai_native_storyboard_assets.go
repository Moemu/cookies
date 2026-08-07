package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/systems/creative"
)

type creativeAINativeStoryboardAssetPreparer struct {
	provider *provider.Service
	now      func() time.Time
}

func buildAINativeStoryboardAssetPrompt(asset creative.AINativeStoryboardAsset) string {
	variations := []string{
		"使用与上一轮明显不同的构图和机位。",
		"保留场景要求，同时改变主体安排和视觉层级。",
		"保留场景要求，同时改变景别和光线处理。",
	}
	attempt := asset.GenerationAttempt
	if attempt < 1 {
		attempt = 1
	}
	parts := []string{strings.TrimSpace(asset.GenerationBrief)}
	if feedback := strings.TrimSpace(asset.RegenerationFeedback); feedback != "" {
		parts = append(parts, "用户重新生成反馈："+feedback)
	}
	parts = append(parts,
		variations[(attempt-1)%len(variations)],
		"生成适合作为图生视频输入的竖版广告分镜参考图。",
		"不出现可识别的真实人物或未成年人；需要人物时仅使用虚构成年人物。",
		"镜面中不出现清晰人物面部倒影。",
		"不包含受版权保护角色、品牌 Logo、包装文字、水印或虚构商品。",
	)
	return strings.Join(parts, "\n")
}

func (p creativeAINativeStoryboardAssetPreparer) PrepareAINativeStoryboardAsset(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, operation creative.AINativeStoryboardOperation, asset creative.AINativeStoryboardAsset) (*contract.AssetVersionRef, *time.Time, error) {
	if p.provider == nil || asset.Source != creative.AINativeStoryboardAssetSourceAIGenerated || strings.TrimSpace(asset.GenerationBrief) == "" {
		return nil, nil, fmt.Errorf("storyboard asset preparation input is invalid")
	}
	attempt := asset.GenerationAttempt
	if attempt < 1 {
		attempt = 1
	}
	prompt := buildAINativeStoryboardAssetPrompt(asset)
	requestHash, err := contract.CanonicalJSONHash(struct {
		WorkspaceID         string `json:"workspace_id"`
		RequirementRevision int64  `json:"requirement_revision"`
		ScriptRevision      int64  `json:"script_revision"`
		AssetID             string `json:"asset_id"`
		Prompt              string `json:"prompt"`
		Attempt             int    `json:"attempt"`
	}{WorkspaceID: operation.WorkspaceID, RequirementRevision: operation.RequirementRevision, ScriptRevision: operation.ScriptRevision,
		AssetID: asset.ID, Prompt: prompt, Attempt: attempt})
	if err != nil {
		return nil, nil, err
	}
	providerActor := actor
	providerActor.Scopes = append([]contract.Scope{}, actor.Scopes...)
	if !providerActor.HasScope(provider.ScopeJobCreate) {
		providerActor.Scopes = append(providerActor.Scopes, provider.ScopeJobCreate)
	}
	job, _, err := p.provider.CreateImageJob(ctx, provider.CreateImageJobRequest{
		Actor: providerActor, Project: project, IdempotencyKey: contract.IdempotencyKey("ai-native-storyboard-asset-" + requestHash),
		RequestHash: requestHash, ModelAlias: "cookies.image.standard", SourceSystem: "creative.ai_native.storyboard", SourceTaskID: "ainativeasset/" + requestHash[:16] + "/" + asset.ID,
		Input: provider.ImageGenerationInput{Prompt: prompt, Width: 1024, Height: 1536},
	})
	if err != nil {
		return nil, nil, err
	}
	latest, err := p.provider.GetJob(ctx, actor.OrganizationID, project.ProjectID, job.ID)
	if err != nil {
		return nil, nil, err
	}
	switch latest.ProviderStatus {
	case contract.ProviderJobSucceeded, contract.ProviderJobPartiallySucceeded:
		if len(latest.ProjectAssetRefs) == 0 {
			return nil, nil, fmt.Errorf("provider storyboard image job completed without a durable AssetVersionRef")
		}
		ref := latest.ProjectAssetRefs[0].AssetVersion
		return &ref, nil, nil
	case contract.ProviderJobFailed, contract.ProviderJobCancelled, contract.ProviderJobExpired:
		if latest.Error != nil {
			return nil, nil, fmt.Errorf("provider storyboard image failed: %s", latest.Error.Message)
		}
		return nil, nil, fmt.Errorf("provider storyboard image failed with status %s", latest.ProviderStatus)
	default:
		now := time.Now().UTC()
		if p.now != nil {
			now = p.now().UTC()
		}
		next := now.Add(5 * time.Second)
		return nil, &next, nil
	}
}
