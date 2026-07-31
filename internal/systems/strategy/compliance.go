package strategy

import (
	"strings"
	"time"
)

func evaluateCompliance(document StrategyDocument, brief BriefVersion, checkedAt time.Time) ComplianceReport {
	report := ComplianceReport{
		ContractVersion: "strategy-compliance-report/v1",
		Passed:          true,
		Issues:          []ComplianceIssue{},
		CheckedAt:       checkedAt.UTC(),
	}
	content := strings.ToLower(string(mustJSON(documentWithoutCompliance(document))))
	for _, phrase := range []string{
		"100%", "第一", "最好", "绝对", "保证", "永久", "国家级", "顶级", "零风险", "治愈", "药到病除",
	} {
		if strings.Contains(content, strings.ToLower(phrase)) {
			report.Issues = append(report.Issues, ComplianceIssue{
				RuleID: "advertising.absolute_claim", Severity: "blocker",
				Message: "策略包含需要事实与法务复核的绝对化或高风险表述", Evidence: phrase,
			})
		}
	}
	for _, phrase := range brief.Snapshot.Creative.ProhibitedClaims {
		phrase = strings.TrimSpace(phrase)
		if phrase != "" && strings.Contains(content, strings.ToLower(phrase)) {
			report.Issues = append(report.Issues, ComplianceIssue{
				RuleID: "brief.prohibited_claim", Severity: "blocker",
				Message: "策略使用了 Brief 明确禁止的表述", Evidence: phrase,
			})
		}
	}
	if len(document.EvidenceRefs) == 0 {
		report.Issues = append(report.Issues, ComplianceIssue{
			RuleID: "evidence.references_missing", Severity: "warning",
			Message: "策略未显式引用研究或文档证据；涉及事实性主张时应补充来源",
		})
	}
	for _, issue := range report.Issues {
		if issue.Severity == "blocker" {
			report.Passed = false
			break
		}
	}
	return report
}

func documentWithoutCompliance(document StrategyDocument) StrategyDocument {
	document.Compliance = nil
	return document
}
