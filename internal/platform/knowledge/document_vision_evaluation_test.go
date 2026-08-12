package knowledge

import (
	"fmt"
	"strings"
	"testing"
)

func TestEvaluateDocumentVisionKeepsAutomaticFallbackClosedWithoutCategoryCoverage(t *testing.T) {
	dataset := validDocumentVisionEvaluationDataset()
	dataset.Cases = dataset.Cases[:1]
	report, err := EvaluateDocumentVision(dataset)
	if err != nil {
		t.Fatalf("EvaluateDocumentVision() error = %v", err)
	}
	if report.AutoEnableAllowed {
		t.Fatal("one synthetic category must not enable automatic fallback")
	}
	assertVisionBlocker(t, report, "LABELLED_CATEGORY_COVERAGE_INSUFFICIENT")
}

func TestEvaluateDocumentVisionUsesBoundedLongDocumentSimilarity(t *testing.T) {
	dataset := validDocumentVisionEvaluationDataset()
	dataset.Cases = dataset.Cases[:1]
	gold := strings.Repeat("可追溯的文档结构 ", 700)
	dataset.Cases[0].GoldMarkdown = gold
	dataset.Cases[0].TextBaselineMarkdown = strings.Repeat("乱码 ", 700)
	dataset.Cases[0].HybridMarkdown = gold
	report, err := EvaluateDocumentVision(dataset)
	if err != nil {
		t.Fatalf("EvaluateDocumentVision() error = %v", err)
	}
	if report.MeanHybridQuality != 1 || report.MeanHybridQuality <= report.MeanBaselineQuality {
		t.Fatalf("long document quality report = %#v", report)
	}
}

func TestEvaluateDocumentVisionRequiresThreeCasesPerCategory(t *testing.T) {
	dataset := validDocumentVisionEvaluationDataset()
	thin := make([]DocumentVisionEvaluationCase, 0, len(documentVisionEvaluationCategories))
	seen := map[string]bool{}
	for _, item := range dataset.Cases {
		if !seen[item.Category] {
			thin = append(thin, item)
			seen[item.Category] = true
		}
	}
	dataset.Cases = thin
	report, err := EvaluateDocumentVision(dataset)
	if err != nil {
		t.Fatalf("EvaluateDocumentVision() error = %v", err)
	}
	if report.AutoEnableAllowed {
		t.Fatal("one case per category must not enable automatic fallback")
	}
	assertVisionBlocker(t, report, "LABELLED_CATEGORY_COVERAGE_INSUFFICIENT")
}

func TestEvaluateDocumentVisionRequiresCounterbalancedReviewOrderPerCategory(t *testing.T) {
	dataset := validDocumentVisionEvaluationDataset()
	for index := range dataset.Cases {
		for reviewIndex := range dataset.Cases[index].Reviews {
			dataset.Cases[index].Reviews[reviewIndex].ReviewOrder = "baseline_first"
		}
	}
	report, err := EvaluateDocumentVision(dataset)
	if err != nil {
		t.Fatalf("EvaluateDocumentVision() error = %v", err)
	}
	if report.AutoEnableAllowed {
		t.Fatal("unbalanced review order must not enable automatic fallback")
	}
	assertVisionBlocker(t, report, "BLINDED_REVIEW_ORDER_NOT_COUNTERBALANCED")
}

func TestEvaluateDocumentVisionRejectsMissingCorrectionDurationAndReviewEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*DocumentVisionEvaluationCase)
	}{
		{name: "zero correction duration", mutate: func(item *DocumentVisionEvaluationCase) { item.Reviews[0].HybridReviewAndCorrectionMS = 0 }},
		{name: "not blinded", mutate: func(item *DocumentVisionEvaluationCase) { item.OutputsBlinded = false }},
		{name: "one reviewer", mutate: func(item *DocumentVisionEvaluationCase) { item.Reviews = item.Reviews[:1] }},
		{name: "reused blind label", mutate: func(item *DocumentVisionEvaluationCase) { item.Reviews[1].BlindLabelID = item.Reviews[0].BlindLabelID }},
		{name: "self adjudication", mutate: func(item *DocumentVisionEvaluationCase) { item.AdjudicatorID = item.Reviews[0].ReviewerID }},
		{name: "not adjudicated", mutate: func(item *DocumentVisionEvaluationCase) { item.Adjudicated = false }},
		{name: "missing lineage", mutate: func(item *DocumentVisionEvaluationCase) { item.HybridRouteRevisionID = "" }},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			dataset := validDocumentVisionEvaluationDataset()
			testCase.mutate(&dataset.Cases[0])
			if _, err := EvaluateDocumentVision(dataset); err == nil {
				t.Fatal("incomplete evaluation evidence was accepted")
			}
		})
	}
}

func TestEvaluateDocumentVisionRejectsReusedSourcePages(t *testing.T) {
	dataset := validDocumentVisionEvaluationDataset()
	dataset.Cases[1].SourceSHA256 = dataset.Cases[0].SourceSHA256
	dataset.Cases[1].PageNumbers = dataset.Cases[0].PageNumbers
	if _, err := EvaluateDocumentVision(dataset); err == nil || !strings.Contains(err.Error(), "reuses an already counted source page") {
		t.Fatalf("EvaluateDocumentVision() error = %v", err)
	}
}

func TestEvaluateDocumentVisionAllowsOnlyMeasuredCrossCategoryImprovement(t *testing.T) {
	dataset := validDocumentVisionEvaluationDataset()
	report, err := EvaluateDocumentVision(dataset)
	if err != nil {
		t.Fatalf("EvaluateDocumentVision() error = %v", err)
	}
	if !report.AutoEnableAllowed || len(report.Blockers) != 0 {
		t.Fatalf("expected measured improvement to pass: %#v", report)
	}
	if report.CaseCount != 24 || report.TotalHybridCostMilliCNY != 120 || report.TotalHybridBillablePages != 24 {
		t.Fatalf("coverage/cost report = %#v", report)
	}
	if report.ReviewCount != 48 || report.CorrectionTimeReduction != .75 || report.TotalCorrectionTimeSavedMS != 4_320_000 {
		t.Fatalf("correction time report = %#v", report)
	}
	if report.DatasetSHA256 == "" || report.ContractVersion != DocumentVisionEvaluationReportContractVersion {
		t.Fatalf("report lineage = %#v", report)
	}
}

func TestEvaluateDocumentVisionBlocksFastButLowQualityHybrid(t *testing.T) {
	dataset := validDocumentVisionEvaluationDataset()
	for index := range dataset.Cases {
		dataset.Cases[index].HybridMarkdown = dataset.Cases[index].TextBaselineMarkdown
	}
	report, err := EvaluateDocumentVision(dataset)
	if err != nil {
		t.Fatalf("EvaluateDocumentVision() error = %v", err)
	}
	if report.AutoEnableAllowed {
		t.Fatal("correction-time improvement must not hide a quality failure")
	}
	assertVisionBlocker(t, report, "MEAN_QUALITY_GAIN_BELOW_THRESHOLD")
}

func validDocumentVisionEvaluationDataset() DocumentVisionEvaluationDataset {
	categories := []string{"text_pdf", "scanned_pdf", "two_column", "table_dense", "chinese_ppt", "broken_font_map", "header_footer_noise", "image_led"}
	dataset := DocumentVisionEvaluationDataset{
		ContractVersion:        DocumentVisionEvaluationDatasetContractVersion,
		DatasetID:              "synthetic-gate-test-v1",
		LabelPolicyVersion:     "label-policy-test-v1",
		RedactionPolicyVersion: "redaction-policy-test-v1",
		CostPolicyVersion:      "las-pricing-test-v1",
		CollectedAt:            "2026-08-11T10:00:00+08:00",
		Deidentified:           true,
		Cases:                  make([]DocumentVisionEvaluationCase, 0, 24),
	}
	for categoryIndex, category := range categories {
		for sample := 0; sample < minDocumentVisionCasesPerCategory; sample++ {
			caseID := fmt.Sprintf("synthetic-%s-%d", category, sample+1)
			dataset.Cases = append(dataset.Cases, DocumentVisionEvaluationCase{
				ID: caseID, Category: category,
				SourceSHA256:   strings.Repeat(fmt.Sprintf("%x", categoryIndex+1), 64),
				SourceMIMEType: "application/pdf", PageNumbers: []int{sample + 1},
				GoldMarkdown: "# 标题\n关键结论 2026", TextBaselineMarkdown: "乱码", HybridMarkdown: "# 标题\n关键结论 2026",
				BaselineParserCode: "tika", BaselineParserVersion: "3.2.3",
				HybridParserCode: "cookies-hybrid", HybridParserVersion: "v1",
				HybridModelAlias: "cookies.document.vision.standard", HybridRouteRevisionID: "route-test-v1", HybridPromptVersion: "vision-prompt-test-v1",
				OutputsBlinded: true,
				Reviews: []DocumentVisionReviewMeasurement{
					{BlindLabelID: caseID + "-a", ReviewerID: "reviewer-a", ReviewOrder: "baseline_first", BaselineCorrections: 12, HybridCorrections: 2, BaselineReviewAndCorrectionMS: 120_000, HybridReviewAndCorrectionMS: 30_000},
					{BlindLabelID: caseID + "-b", ReviewerID: "reviewer-b", ReviewOrder: "hybrid_first", BaselineCorrections: 12, HybridCorrections: 2, BaselineReviewAndCorrectionMS: 120_000, HybridReviewAndCorrectionMS: 30_000},
				},
				Adjudicated: true, AdjudicatorID: "adjudicator-c",
				BaselineLatencyMS: 100, HybridLatencyMS: 900, HybridBillablePages: 1, HybridCostMilliCNY: 5,
			})
		}
	}
	return dataset
}

func assertVisionBlocker(t *testing.T, report DocumentVisionEvaluationReport, expected string) {
	t.Helper()
	for _, blocker := range report.Blockers {
		if blocker == expected {
			return
		}
	}
	t.Fatalf("missing blocker %q in %#v", expected, report.Blockers)
}
