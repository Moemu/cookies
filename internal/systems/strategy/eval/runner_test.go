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

func TestEvaluateRejectsDuplicatedPlatformPlans(t *testing.T) {
	t.Parallel()
	testCase := Case{
		ID: "multi", Objective: "认知", Audience: "新客", Proposition: "方便",
		Channels: []string{"xiaohongshu", "douyin"},
	}
	plan := strategy.PlatformPlan{
		Role: "种草", AudienceAngle: "方便", ContentPillars: []string{"场景"},
		Formats: []string{"短视频"}, ConversionPath: "内容到咨询", CreativeIdeas: []string{"演示"},
	}
	first := plan
	first.Platform = "xiaohongshu"
	second := plan
	second.Platform = "douyin"
	document := strategy.StrategyDocument{
		Objective: "认知", Audience: strategy.StrategyAudience{Primary: "新客", Insights: []string{"重视效率"}},
		Proposition: "方便",
		ChannelStrategy: []strategy.ChannelStrategy{
			{Platform: "xiaohongshu", Formats: []string{"图文"}},
			{Platform: "douyin", Formats: []string{"短视频"}},
		},
		CreativeRecommendations: []string{"场景", "证据", "行动"},
		ExperimentMatrix:        []strategy.Experiment{{Hypothesis: "h", Variable: "v", Metric: "m"}},
		Measurement:             []string{"m"},
		PlatformPlans:           []strategy.PlatformPlan{first, second},
	}
	score := Evaluate(testCase, document)
	if score.PlatformAdaptation != 0 {
		t.Fatalf("score = %#v", score)
	}
}
