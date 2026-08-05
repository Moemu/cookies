package strategy

import (
	"testing"
	"time"
)

func TestEvaluateComplianceBlocksAbsoluteAndBriefProhibitedClaims(t *testing.T) {
	t.Parallel()
	brief := BriefVersion{Snapshot: EmptyBriefDocumentV2()}
	brief.Snapshot.Creative.ProhibitedClaims = []string{"行业唯一"}
	document := StrategyDocument{
		ContractVersion: "strategy-draft/v2",
		Objective:       "保证 100% 提升转化",
		Proposition:     "行业唯一",
	}
	report := evaluateCompliance(document, brief, time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC))
	if report.Passed || len(report.Issues) < 2 {
		t.Fatalf("high-risk claims were not blocked: %#v", report)
	}
	for _, issue := range report.Issues {
		if issue.Evidence == "100%" || issue.Evidence == "行业唯一" {
			if issue.Severity != "blocker" {
				t.Fatalf("claim issue is not blocking: %#v", issue)
			}
		}
	}
}

func TestEvaluateComplianceWarnsWhenEvidenceReferencesAreMissing(t *testing.T) {
	t.Parallel()
	report := evaluateCompliance(
		StrategyDocument{ContractVersion: "strategy-draft/v2", Objective: "提升认知"},
		BriefVersion{Snapshot: EmptyBriefDocumentV2()},
		time.Now(),
	)
	if !report.Passed || len(report.Issues) != 1 || report.Issues[0].Severity != "warning" {
		t.Fatalf("missing evidence should be warning-only: %#v", report)
	}
}

func TestEvaluateComplianceDoesNotTreatGuardrailsOrFirstPersonPerspectiveAsClaims(t *testing.T) {
	t.Parallel()
	brief := BriefVersion{Snapshot: EmptyBriefDocumentV2()}
	brief.Snapshot.Creative.ProhibitedClaims = []string{"绝对健康"}
	document := StrategyDocument{
		ContractVersion: "strategy-draft/v2",
		Objective:       "提升品牌认知",
		CreativeRecommendations: []string{
			"使用第一视角记录通勤体验",
			"所有内容禁用减肥、治疗、绝对健康等违规表述",
		},
		Constraints: []string{"不得使用绝对健康等表述"},
		PlatformPlans: []PlatformPlan{{
			Platform: "xiaohongshu", Constraints: []string{"禁止绝对化承诺"},
		}},
	}

	report := evaluateCompliance(document, brief, time.Now())
	if !report.Passed {
		t.Fatalf("guardrails and first-person perspective must not be treated as claims: %#v", report)
	}
}

func TestEvaluateComplianceBlocksExplicitRankingClaim(t *testing.T) {
	t.Parallel()
	report := evaluateCompliance(
		StrategyDocument{ContractVersion: "strategy-draft/v2", Proposition: "销量第一"},
		BriefVersion{Snapshot: EmptyBriefDocumentV2()},
		time.Now(),
	)
	if report.Passed {
		t.Fatalf("explicit ranking claim must be blocked: %#v", report)
	}
}
