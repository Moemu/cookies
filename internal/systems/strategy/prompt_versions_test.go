package strategy

import (
	"testing"

	"github.com/shikanon/cookies/internal/systems/strategy/promptkit"
)

func TestDefaultPromptVersionsUseLatestGovernedDefinitions(t *testing.T) {
	t.Parallel()
	service := Service{}
	if got := service.generatePromptVersion(); got != promptkit.GenerateV5 {
		t.Fatalf("generate prompt version = %q, want %q", got, promptkit.GenerateV5)
	}
	if got := service.reviewPromptVersion(); got != promptkit.ReviewV2 {
		t.Fatalf("review prompt version = %q, want %q", got, promptkit.ReviewV2)
	}
}
