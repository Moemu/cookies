package strategy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/systems/strategy/promptkit"
)

func TestDeepReviewRouteReady(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		inspection provider.TextRouteInspection
		want       bool
	}{
		{
			name:       "synchronous Responses route",
			inspection: provider.TextRouteInspection{Ready: true, APIMode: provider.TextAPIResponses},
			want:       true,
		},
		{
			name:       "background Responses route",
			inspection: provider.TextRouteInspection{Ready: true, APIMode: provider.TextAPIResponses, Background: true},
			want:       true,
		},
		{
			name:       "chat completions route",
			inspection: provider.TextRouteInspection{Ready: true, APIMode: provider.TextAPIChatCompletions},
			want:       false,
		},
		{
			name:       "unready Responses route",
			inspection: provider.TextRouteInspection{APIMode: provider.TextAPIResponses},
			want:       false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := deepReviewRouteReady(test.inspection); got != test.want {
				t.Fatalf("deepReviewRouteReady(%#v) = %t, want %t", test.inspection, got, test.want)
			}
		})
	}
}

func TestDeepReviewV2SchemaAllowsNoFindings(t *testing.T) {
	t.Parallel()
	var schema map[string]any
	if err := json.Unmarshal(deepReviewOutputSchema(promptkit.ReviewV2), &schema); err != nil {
		t.Fatal(err)
	}
	properties := schema["properties"].(map[string]any)
	findings := properties["findings"].(map[string]any)
	if findings["minItems"].(float64) != 0 {
		t.Fatalf("findings minItems = %#v", findings["minItems"])
	}
}

func TestDeepReviewV2PromptIncludesBriefEvidenceAndQuality(t *testing.T) {
	t.Parallel()
	brief := BriefVersion{
		BriefID: "brief_1", Version: 2,
		Snapshot: BriefDocument{
			Campaign: BriefCampaign{Objective: "获取线索"},
		},
	}
	prompt := deepReviewUserPromptV2(
		brief,
		DraftRevision{
			Revision: 3, ContentHash: "hash_1",
			Document: StrategyDocument{Objective: "获取线索"},
		},
		[]KnowledgeExcerpt{{ID: "doc_1", Content: "产品证据"}},
		QualityReport{Passed: true, Score: 100},
	)
	for _, expected := range []string{"brief_1", "获取线索", "doc_1", "产品证据", `"score":100`} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt omitted %q: %s", expected, prompt)
		}
	}
}
