package main

import (
	"context"
	"fmt"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/systems/creative"
)

type creativeShortDramaV2ImageJobs struct{ provider *provider.Service }

func shortDramaV2FirstFrameInput(prompt string, width, height int) provider.ImageGenerationInput {
	return provider.ImageGenerationInput{Prompt: prompt, Width: width, Height: height}
}

func shortDramaV2FirstFrameSourceTaskID(request creative.ShortDramaV2FirstFrameJobRequest) string {
	// provider_jobs.source_task_id is VARCHAR(96). Candidate and batch lineage
	// already lives in the idempotency/request hashes, so the Provider source
	// link should remain the stable Creative task ID instead of concatenating
	// the much longer candidate ID.
	return request.TaskID
}

func (j creativeShortDramaV2ImageJobs) CreateFirstFrameJob(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, request creative.ShortDramaV2FirstFrameJobRequest) (contract.ProviderJob, error) {
	if j.provider == nil {
		return contract.ProviderJob{}, fmt.Errorf("short drama V2 image provider is unavailable")
	}
	providerActor := actor
	if !providerActor.HasScope(provider.ScopeJobCreate) {
		providerActor.Scopes = append(providerActor.Scopes, provider.ScopeJobCreate)
	}
	hash, err := contract.CanonicalJSONHash(struct {
		TaskID       string `json:"task_id"`
		BatchID      string `json:"batch_id"`
		CandidateID  string `json:"candidate_id"`
		VariantIndex int    `json:"variant_index"`
		Prompt       string `json:"prompt"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
	}{request.TaskID, request.BatchID, request.CandidateID, request.VariantIndex, request.Prompt, request.Width, request.Height})
	if err != nil {
		return contract.ProviderJob{}, err
	}
	job, _, err := j.provider.CreateImageJob(ctx, provider.CreateImageJobRequest{
		Actor: providerActor, Project: project,
		IdempotencyKey: contract.IdempotencyKey("short-drama-v2-frame-" + hash), RequestHash: hash,
		ModelAlias: "cookies.image.standard", SourceSystem: "creative.short_drama_preroll_v2", SourceTaskID: shortDramaV2FirstFrameSourceTaskID(request),
		Input: shortDramaV2FirstFrameInput(request.Prompt, request.Width, request.Height),
	})
	return job, err
}
