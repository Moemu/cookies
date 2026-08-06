package creative

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type failingShortDramaV2Planner struct{ err error }

func (p failingShortDramaV2Planner) PlanDirections(context.Context, contract.ActorContext, contract.ProjectContext, ShortDramaV2Analysis) ([]ShortDramaV2HookDirection, string, error) {
	return nil, "", p.err
}

func (p failingShortDramaV2Planner) CompilePrompts(context.Context, contract.ActorContext, contract.ProjectContext, ShortDramaV2Analysis, ShortDramaV2HookDirection, int) (ShortDramaV2PromptDraft, error) {
	return ShortDramaV2PromptDraft{}, p.err
}

func TestFallbackShortDramaV2PlannerRecoversFromTemporaryModelFailure(t *testing.T) {
	t.Parallel()

	analysisResult := testShortDramaV2AnalysisResult()
	analysis := ShortDramaV2Analysis{
		ShortDramaV2AsyncResource: ShortDramaV2AsyncResource{Status: ShortDramaV2ResourceReady},
		Revision:                  1, InputHash: analysisResult.InputHash, PromptVersion: analysisResult.PromptVersion,
		Content: analysisResult.Content,
	}
	providerErr := errors.New("Adapter gateway text request failed")
	failures := 0
	planner := FallbackShortDramaV2Planner{
		Primary: failingShortDramaV2Planner{err: providerErr}, Fallback: DeterministicShortDramaV2Planner{},
		OnPrimaryFailure: func(err error) {
			if !errors.Is(err, providerErr) {
				t.Fatalf("unexpected primary error: %v", err)
			}
			failures++
		},
	}

	directions, version, err := planner.PlanDirections(context.Background(), contract.ActorContext{}, contract.ProjectContext{}, analysis)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(version, "fallback:") || len(directions) != 4 {
		t.Fatalf("unexpected fallback directions: version=%q directions=%#v", version, directions)
	}
	if err := validateShortDramaV2Directions(analysis, directions); err != nil {
		t.Fatalf("fallback directions violate the workspace contract: %v", err)
	}

	prompt, err := planner.CompilePrompts(context.Background(), contract.ActorContext{}, contract.ProjectContext{}, analysis, directions[0], 6)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(prompt.CompilerVersion, "fallback:") || strings.TrimSpace(prompt.ImagePrompt) == "" ||
		strings.TrimSpace(prompt.VideoDescription) == "" || strings.TrimSpace(prompt.VideoPrompt) == "" {
		t.Fatalf("unexpected fallback prompt: %#v", prompt)
	}
	if failures != 2 {
		t.Fatalf("primary failure callback count = %d, want 2", failures)
	}
}
