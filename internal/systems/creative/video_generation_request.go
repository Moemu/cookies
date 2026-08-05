package creative

import (
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

// ProviderInput crosses the Creative/Provider boundary only after an exact
// prompt, exact conditioned frames, and their paid-generation approval agree.
func (r CreateVideoJobRequest) ProviderInput(projectID contract.ProjectID, taskID string) (provider.VideoGenerationInput, error) {
	if err := r.Validate(); err != nil {
		return provider.VideoGenerationInput{}, err
	}
	if r.Prompt == nil || r.GenerationSpec == nil || r.Approval == nil {
		return provider.VideoGenerationInput{}, fmt.Errorf("approved commerce preroll generation fields are required")
	}
	spec := *r.GenerationSpec
	if err := spec.ValidateHash(); err != nil {
		return provider.VideoGenerationInput{}, err
	}
	if err := r.Prompt.ValidateHash(); err != nil {
		return provider.VideoGenerationInput{}, err
	}
	if strings.TrimSpace(taskID) == "" || spec.TaskID != taskID ||
		r.Prompt.ContractVersion != "creative-video-prompt/v1" ||
		r.Prompt.TaskID != taskID ||
		r.Prompt.Hash != spec.PromptHash {
		return provider.VideoGenerationInput{}, fmt.Errorf("creative video prompt does not match the generation spec")
	}
	if err := r.Approval.Authorizes(spec); err != nil {
		return provider.VideoGenerationInput{}, err
	}
	conditioning := make([]provider.VideoConditioningAsset, 0, len(spec.ConditioningAssets))
	for _, asset := range spec.ConditioningAssets {
		role := provider.VideoConditioningRole(asset.Role)
		conditioning = append(conditioning, provider.VideoConditioningAsset{
			Role: role,
			Reference: contract.ProjectAssetRef{
				ProjectID:    projectID,
				AssetVersion: asset.AssetRef,
			},
		})
	}
	audioPolicy := provider.VideoAudioPolicy(spec.AudioPolicy)
	input := provider.VideoGenerationInput{
		Prompt:             specPrompt(r.Prompt),
		DurationSeconds:    spec.DurationSeconds,
		AspectRatio:        spec.AspectRatio,
		Resolution:         spec.Resolution,
		AudioPolicy:        audioPolicy,
		InputMode:          provider.VideoInputFirstLastFrame,
		ConditioningAssets: conditioning,
	}
	if err := input.Validate(); err != nil {
		return provider.VideoGenerationInput{}, err
	}
	return input, nil
}

func specPrompt(prompt *CreativeVideoPrompt) string {
	if prompt == nil {
		return ""
	}
	return strings.TrimSpace(prompt.CompiledPrompt)
}
