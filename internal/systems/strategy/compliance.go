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
	content := strings.ToLower(string(mustJSON(documentForClaimReview(document))))
	for _, phrase := range []string{
		"100%", "行业第一", "品类第一", "销量第一", "市场第一", "排名第一", "全网第一",
		"全国第一", "世界第一", "最好", "绝对", "保证", "永久", "国家级", "顶级", "零风险", "治愈", "药到病除",
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

// documentForClaimReview keeps the compliance scan focused on outward-facing
// claims. Guardrails describe what must not be said, so scanning them as claims
// creates false blockers such as "不得使用绝对健康". Generic uses of "第一"
// (for example "第一视角") are handled by using explicit ranking phrases above.
func documentForClaimReview(document StrategyDocument) StrategyDocument {
	document = documentWithoutCompliance(document)
	document.Constraints = nil
	document.CreativeRecommendations = outwardFacingRecommendations(document.CreativeRecommendations)
	document.PlatformPlans = append([]PlatformPlan(nil), document.PlatformPlans...)
	for index := range document.PlatformPlans {
		document.PlatformPlans[index].Constraints = nil
	}
	return document
}

func outwardFacingRecommendations(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if isGuardrailStatement(value) {
			continue
		}
		result = append(result, value)
	}
	return result
}

func isGuardrailStatement(value string) bool {
	for _, marker := range []string{"不得", "禁止", "严禁", "禁用", "避免", "不可", "不使用"} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}
