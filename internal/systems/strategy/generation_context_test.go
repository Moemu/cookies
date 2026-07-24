package strategy

import (
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestEvidenceFromBriefPreservesConfirmationBoundary(t *testing.T) {
	t.Parallel()
	brief := BriefVersion{
		Snapshot: BriefDocument{
			ContractVersion: "strategy-brief-version/v1",
			Campaign:        BriefCampaign{Objective: "新品认知"},
			Audience:        BriefAudience{Primary: "研发负责人"},
			Proposition:     "缩短研发周期",
			Channels:        []string{"xiaohongshu"},
		},
		FieldStates: map[string]FieldState{
			"campaign.objective": {Confirmation: "confirmed", Confidence: "high"},
			"audience.primary":   {Confirmation: "confirmed", Confidence: "high"},
			"proposition":        {Confirmation: "unconfirmed", Confidence: "medium"},
			"channels":           {Confirmation: "confirmed", Confidence: "high"},
		},
	}
	values := evidenceFromBrief(brief)
	if len(values) != 4 {
		t.Fatalf("evidence count = %d, want 4", len(values))
	}
	if values[2].FieldPath != "proposition" || values[2].Confirmed {
		t.Fatalf("proposition evidence = %#v", values[2])
	}
}

func TestStrategyQualityRejectsSuperficiallyCompleteDocument(t *testing.T) {
	t.Parallel()
	document := StrategyDocument{
		ContractVersion: "strategy-draft/v1", Objective: "认知",
		Audience:        StrategyAudience{Primary: "研发负责人"},
		Proposition:     "提效",
		ChannelStrategy: []ChannelStrategy{{Platform: "xiaohongshu", Role: "种草", Formats: []string{"图文"}}},
		Lineage:         StrategyLineage{BriefID: "brief_1", BriefVersion: 1, ProjectContextVersion: 1},
	}
	report := evaluateStrategyQuality(document, GenerationContext{
		Project: contract.ProjectContext{ProjectContextVersion: 1},
	})
	if report.Passed || report.Score >= 100 {
		t.Fatalf("quality report = %#v", report)
	}
}

func TestStrategyQualityRejectsConfirmedBriefDrift(t *testing.T) {
	t.Parallel()
	generation := GenerationContext{Brief: BriefVersion{Snapshot: BriefDocument{
		Campaign:    BriefCampaign{Objective: "新品认知"},
		Audience:    BriefAudience{Primary: "制造企业研发负责人"},
		Proposition: "缩短研发决策周期",
		Channels:    []string{"xiaohongshu"},
		Measurement: BriefMeasurement{PrimaryKPI: "高意向咨询数"},
	}}}
	document := StrategyDocument{
		ContractVersion: "strategy-draft/v1",
		Objective:       "电商成交",
		Audience: StrategyAudience{
			Primary:  "泛消费人群",
			Insights: []string{"需要对比真实使用场景"},
		},
		Proposition: "低价促销",
		ChannelStrategy: []ChannelStrategy{{
			Platform: "xiaohongshu", Role: "决策辅助", Formats: []string{"图文"},
		}},
		CreativeRecommendations: []string{"场景问题开篇", "证据清单正文", "评论区承接咨询"},
		ExperimentMatrix:        []Experiment{{Hypothesis: "证据清单提高咨询", Variable: "正文结构", Metric: "高意向咨询数"}},
		Measurement:             []string{"高意向咨询数"},
		Lineage: StrategyLineage{
			BriefID: "brief_1", BriefVersion: 1, ProjectContextVersion: 1,
		},
	}
	report := evaluateStrategyQuality(document, generation)
	if report.Passed || len(report.Errors) < 3 {
		t.Fatalf("quality report = %#v", report)
	}
}

func TestRetainAllowedRevisionSections(t *testing.T) {
	t.Parallel()
	before := StrategyDocument{
		Objective:               "认知",
		Audience:                StrategyAudience{Primary: "研发负责人"},
		Proposition:             "提效",
		CreativeRecommendations: []string{"原建议"},
		Measurement:             []string{"咨询数"},
	}
	after := before
	after.Objective = "成交"
	after.CreativeRecommendations = []string{"B2B 决策链内容"}
	after.Measurement = []string{"成交数"}
	allowed := allowedRevisionSections("把创意建议改得更适合 B2B 技术决策者，其他章节保持不变")
	retainAllowedRevisionSections(before, &after, allowed)
	if after.Objective != before.Objective || after.Measurement[0] != before.Measurement[0] ||
		after.CreativeRecommendations[0] == before.CreativeRecommendations[0] {
		t.Fatalf("retained revision = %#v; allowed=%v", after, allowed)
	}
}
