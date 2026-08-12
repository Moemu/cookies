package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/knowledge"
)

func TestRunEmitsVersionedReportFromStrictDataset(t *testing.T) {
	path := writeEvaluationDataset(t, validEvaluationDataset())
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-input", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("run() code = %d, stderr = %s", code, stderr.String())
	}
	var report knowledge.DocumentVisionEvaluationReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report.ContractVersion != knowledge.DocumentVisionEvaluationReportContractVersion || report.DatasetSHA256 == "" || report.ReviewCount != 2 {
		t.Fatalf("report = %#v", report)
	}
	if report.AutoEnableAllowed {
		t.Fatal("one case unexpectedly enabled automatic fallback")
	}
}

func TestRunRejectsUnknownAndTrailingJSON(t *testing.T) {
	dataset := validEvaluationDataset()
	encoded, err := json.Marshal(dataset)
	if err != nil {
		t.Fatal(err)
	}
	unknown := bytes.TrimSuffix(encoded, []byte("}"))
	unknown = append(unknown, []byte(`,"unreviewed":true}`)...)
	for name, content := range map[string][]byte{
		"unknown":  unknown,
		"trailing": append(encoded, []byte(` {}`)...),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "dataset.json")
			if err := os.WriteFile(path, content, 0o600); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			if code := run([]string{"-input", path}, &stdout, &stderr); code != 2 || stderr.Len() == 0 {
				t.Fatalf("run() code = %d, stdout = %s, stderr = %s", code, stdout.String(), stderr.String())
			}
		})
	}
}

func writeEvaluationDataset(t *testing.T, dataset knowledge.DocumentVisionEvaluationDataset) string {
	t.Helper()
	encoded, err := json.Marshal(dataset)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "dataset.json")
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func validEvaluationDataset() knowledge.DocumentVisionEvaluationDataset {
	return knowledge.DocumentVisionEvaluationDataset{
		ContractVersion:        knowledge.DocumentVisionEvaluationDatasetContractVersion,
		DatasetID:              "synthetic-cli-test-v1",
		LabelPolicyVersion:     "label-policy-test-v1",
		RedactionPolicyVersion: "redaction-policy-test-v1",
		CostPolicyVersion:      "las-pricing-test-v1",
		CollectedAt:            "2026-08-11T10:00:00+08:00",
		Deidentified:           true,
		Cases: []knowledge.DocumentVisionEvaluationCase{{
			ID: "synthetic-scan-1", Category: "scanned_pdf", SourceSHA256: strings.Repeat("a", 64),
			SourceMIMEType: "application/pdf", PageNumbers: []int{1},
			GoldMarkdown: "# 标题\n结论", TextBaselineMarkdown: "乱码", HybridMarkdown: "# 标题\n结论",
			BaselineParserCode: "tika", BaselineParserVersion: "3.2.3",
			HybridParserCode: "cookies-hybrid", HybridParserVersion: "v1",
			HybridModelAlias: "cookies.document.vision.standard", HybridRouteRevisionID: "route-test-v1", HybridPromptVersion: "vision-prompt-test-v1",
			OutputsBlinded: true,
			Reviews: []knowledge.DocumentVisionReviewMeasurement{
				{BlindLabelID: "blind-scan-1-a", ReviewerID: "reviewer-a", ReviewOrder: "baseline_first", BaselineCorrections: 8, HybridCorrections: 2, BaselineReviewAndCorrectionMS: 120_000, HybridReviewAndCorrectionMS: 30_000},
				{BlindLabelID: "blind-scan-1-b", ReviewerID: "reviewer-b", ReviewOrder: "hybrid_first", BaselineCorrections: 8, HybridCorrections: 2, BaselineReviewAndCorrectionMS: 120_000, HybridReviewAndCorrectionMS: 30_000},
			},
			Adjudicated: true, AdjudicatorID: "adjudicator-c",
			BaselineLatencyMS: 100, HybridLatencyMS: 900, HybridBillablePages: 1, HybridCostMilliCNY: 5,
		}},
	}
}
