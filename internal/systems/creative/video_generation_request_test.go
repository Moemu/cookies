package creative

import (
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

func TestCreateVideoJobRequestBuildsApprovedProviderInput(t *testing.T) {
	spec := acceptedGenerationSpec(t)
	prompt := CreativeVideoPrompt{
		ContractVersion: "creative-video-prompt/v1",
		TaskID:          spec.TaskID,
		IntakeVersion:   1,
		Template:        TemplateReference{ID: CommerceWindowRevealTemplateID, Version: 1},
		ProductAsset:    contract.AssetVersionRef{AssetID: "asset_product", Version: 1},
		Version:         1,
		Fidelity:        "preserve the exact product",
		Camera:          "fixed 9:16 composition",
		Environment:     "clean studio",
		Timeline: []PromptTimelineSegment{
			{StartSeconds: 0, EndSeconds: 1.5, Purpose: TimelineInformationGap, Instruction: "frosted start"},
			{StartSeconds: 1.5, EndSeconds: 4, Purpose: TimelineSingleTransformation, Instruction: "one wipe"},
			{StartSeconds: 4, EndSeconds: 6, Purpose: TimelineProductHold, Instruction: "exact product hold"},
		},
		Guardrails:     []string{"no product distortion"},
		CompiledPrompt: "0-1.5s frosted glass; 1.5-4s one wipe; 4-6s exact product hold",
	}
	if err := prompt.Seal(); err != nil {
		t.Fatalf("prompt Seal() error = %v", err)
	}
	spec.PromptHash = prompt.Hash
	spec.Hash = ""
	if err := spec.Seal(); err != nil {
		t.Fatalf("spec Seal() error = %v", err)
	}
	approval, err := ApproveVideoGeneration(spec, "user_1", time.Date(2026, time.July, 28, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ApproveVideoGeneration() error = %v", err)
	}
	request := CreateVideoJobRequest{
		ModelAlias:     "cookies.video.standard",
		Prompt:         &prompt,
		GenerationSpec: &spec,
		Approval:       &approval,
	}

	input, err := request.ProviderInput(contract.ProjectID("project_1"), spec.TaskID)
	if err != nil {
		t.Fatalf("ProviderInput() error = %v", err)
	}
	if input.InputMode != provider.VideoInputFirstLastFrame || input.AudioPolicy != provider.VideoAudioSilent {
		t.Fatalf("provider input mode/audio = %q/%q", input.InputMode, input.AudioPolicy)
	}
	if len(input.ConditioningAssets) != 2 ||
		input.ConditioningAssets[0].Reference.ProjectID != "project_1" ||
		input.ConditioningAssets[1].Role != provider.VideoConditioningLastFrame {
		t.Fatalf("provider conditioning assets = %+v", input.ConditioningAssets)
	}

	changed := request
	changedPrompt := *request.Prompt
	changedPrompt.CompiledPrompt = "a different paid prompt"
	changed.Prompt = &changedPrompt
	if _, err := changed.ProviderInput("project_1", spec.TaskID); err == nil {
		t.Fatal("ProviderInput() accepts a prompt body changed after approval")
	}
	changed = request
	changedSpec := *request.GenerationSpec
	changedSpec.Hash = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	changed.GenerationSpec = &changedSpec
	if _, err := changed.ProviderInput("project_1", spec.TaskID); err == nil {
		t.Fatal("ProviderInput() accepts an incorrect generation spec hash")
	}
}
