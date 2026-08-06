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

func TestEvaluateDecisionQualityRewardsDistinctEvidenceBackedDirections(t *testing.T) {
	t.Parallel()
	testCase := Case{
		ID: "precision-leads", Objective: "获取销售线索", Audience: "制造企业研发负责人",
		Proposition: "缩短研发周期", Channels: []string{"xiaohongshu"},
		ExpectedSignals: []string{"资格筛选", "低摩擦 CTA", "有效线索率"},
	}
	document := strategy.StrategyDocument{
		Objective: "获取销售线索",
		Audience: strategy.StrategyAudience{
			Primary:  "制造企业研发负责人",
			Insights: []string{"既担心非标件精度无法复现，又不愿在首次接触时提交完整图纸"},
		},
		Proposition:      "缩短研发周期",
		ExecutiveSummary: "围绕研发负责人验证供应商精度与响应速度的关键决策，用可核验样件证据降低首次咨询风险，并通过低摩擦 CTA 完成资格筛选。",
		ChannelStrategy:  []strategy.ChannelStrategy{{Platform: "xiaohongshu", Role: "搜索承接", Formats: []string{"图文"}}},
		CreativeRecommendations: []string{
			"误差账本｜评审前夜｜用三次检测结果拆解稳定性｜检测报告 doc_1｜领取资格筛选清单",
			"图纸保密局｜首次询价｜用脱敏局部图演示报价路径｜保密流程 doc_2｜发起低摩擦 CTA",
			"交期倒推盘｜项目延期时｜用工序时间轴定位风险｜排产记录 doc_3｜预约工艺评审",
		},
		ExperimentMatrix: []strategy.Experiment{{Hypothesis: "证据型封面提高有效咨询", Variable: "封面证据类型", Metric: "有效线索率"}},
		Measurement:      []string{"有效线索率"},
		EvidenceRefs:     []string{"doc_1", "doc_2", "doc_3"},
		PlatformPlans: []strategy.PlatformPlan{{
			Platform: "xiaohongshu", Role: "搜索承接", AudienceAngle: "供应商验证",
			ContentPillars: []string{"精度证据"}, Formats: []string{"图文"}, ConversionPath: "收藏后私信", CreativeIdeas: []string{"误差账本"},
		}},
	}

	score := Evaluate(testCase, document)
	if !score.QualityGatePassed || score.QualityScore != 100 || len(score.QualityFailures) != 0 {
		t.Fatalf("score = %#v", score)
	}
}

func TestEvaluateDecisionQualityRejectsGenericDuplicatedDirections(t *testing.T) {
	t.Parallel()
	testCase := Case{
		ID: "generic", Objective: "认知", Audience: "新客", Proposition: "更方便",
		Channels: []string{"xiaohongshu"}, ExpectedSignals: []string{"真实场景"},
	}
	document := strategy.StrategyDocument{
		Objective: "认知", Audience: strategy.StrategyAudience{Primary: "新客", Insights: []string{"方便"}},
		Proposition: "更方便", ChannelStrategy: []strategy.ChannelStrategy{{Platform: "xiaohongshu"}},
		CreativeRecommendations: []string{"产品介绍", "产品介绍", "产品介绍"},
		ExperimentMatrix:        []strategy.Experiment{{Hypothesis: "测试", Metric: "点击率"}},
	}

	score := Evaluate(testCase, document)
	if score.QualityGatePassed || score.QualityScore >= 80 || len(score.QualityFailures) < 4 {
		t.Fatalf("score = %#v", score)
	}
}
