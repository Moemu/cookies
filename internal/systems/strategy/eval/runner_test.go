package eval

import (
	"testing"

	"github.com/shikanon/cookies/internal/systems/strategy"
)

func TestLoadCasesAndEvaluateAlignedStrategy(t *testing.T) {
	t.Parallel()
	cases, err := LoadCases()
	if err != nil || len(cases) != 10 {
		t.Fatalf("cases=%d err=%v", len(cases), err)
	}
	testCase := cases[0]
	document := strategy.StrategyDocument{
		Objective:               testCase.Objective,
		Audience:                strategy.StrategyAudience{Primary: testCase.Audience, Insights: []string{"真实场景"}},
		Proposition:             testCase.Proposition,
		ChannelStrategy:         []strategy.ChannelStrategy{{Platform: "xiaohongshu", Role: "种草", Formats: []string{"图文"}}},
		CreativeRecommendations: []string{"开篇", "证据", "行动"},
		ExperimentMatrix:        []strategy.Experiment{{Hypothesis: "h", Variable: "v", Metric: "m"}},
		Measurement:             []string{"m"},
	}
	score := Evaluate(testCase, document)
	if score.Total != 10 || len(score.Failures) != 0 {
		t.Fatalf("score = %#v", score)
	}
}
