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

func (p creativeAINativeStoryboardAssetPreparer) PrepareAINativeStoryboardAsset(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, operation creative.AINativeStoryboardOperation, asset creative.AINativeStoryboardAsset) (*contract.AssetVersionRef, *time.Time, error) {
	if p.provider == nil || asset.Source != creative.AINativeStoryboardAssetSourceAIGenerated || strings.TrimSpace(asset.GenerationBrief) == "" {
		return nil, nil, fmt.Errorf("storyboard asset preparation input is invalid")
	}
	requestHash, err := contract.CanonicalJSONHash(struct {
		OperationID string `json:"operation_id"`
		AssetID     string `json:"asset_id"`
		Brief       string `json:"brief"`
	}{OperationID: operation.ID, AssetID: asset.ID, Brief: asset.GenerationBrief})
	if err != nil {
		return nil, nil, err
	}
	providerActor := actor
	providerActor.Scopes = append([]contract.Scope{}, actor.Scopes...)
	if !providerActor.HasScope(provider.ScopeJobCreate) {
		providerActor.Scopes = append(providerActor.Scopes, provider.ScopeJobCreate)
	}
	job, _, err := p.provider.CreateImageJob(ctx, provider.CreateImageJobRequest{
		Actor: providerActor, Project: project, IdempotencyKey: contract.IdempotencyKey("ai-native-storyboard-" + operation.ID + "-" + asset.ID),
		RequestHash: requestHash, ModelAlias: "cookies.image.standard", SourceSystem: "creative.ai_native.storyboard", SourceTaskID: operation.ID + "/" + asset.ID,
		Input: provider.ImageGenerationInput{Prompt: asset.GenerationBrief + "。竖版广告分镜参考图，不包含品牌 Logo、包装文字或虚构商品。", Width: 1024, Height: 1536},
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
