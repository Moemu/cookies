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
