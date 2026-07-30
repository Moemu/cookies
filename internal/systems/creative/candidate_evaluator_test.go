package creative

import (
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestCandidateEvaluatorRequiresTechnicalAndManualAcceptance(t *testing.T) {
	spec := acceptedGenerationSpec(t)
	approval, err := ApproveVideoGeneration(spec, "user_1", time.Date(2026, time.July, 28, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ApproveVideoGeneration() error = %v", err)
	}
	request := CandidateEvaluationRequest{
		TaskID:          spec.TaskID,
		GenerationSpec:  spec,
		Approval:        approval,
		GeneratedAsset:  contract.AssetVersionRef{AssetID: "asset_video_candidate", Version: 1},
		DurationSeconds: 6,
		AspectRatio:     "9:16",
		Resolution:      "720p",
		HasAudio:        false,
		ReviewedBy:      "reviewer_1",
		ReviewedAt:      time.Date(2026, time.July, 28, 9, 15, 0, 0, time.UTC),
		ManualQualityChecks: []ManualQualityCheck{
			{Dimension: QualityBottleShape, Passed: true},
			{Dimension: QualityLabelText, Passed: true},
			{Dimension: QualityProductColor, Passed: true},
			{Dimension: QualityWipeMotion, Passed: true},
			{Dimension: QualityFinalHold, Passed: true},
		},
	}

	report, err := (CandidateEvaluator{}).Evaluate(request)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if report.Status != CandidateAccepted || len(report.Issues) != 0 {
		t.Fatalf("report = %+v, want accepted without issues", report)
	}

	failed := request
	failed.ManualQualityChecks = append([]ManualQualityCheck(nil), request.ManualQualityChecks...)
	failed.ManualQualityChecks[1].Passed = false
	failed.ManualQualityChecks[1].Note = "label glyph changed"
	report, err = (CandidateEvaluator{}).Evaluate(failed)
	if err != nil {
		t.Fatalf("Evaluate(failed) error = %v", err)
	}
	if report.Status != CandidateRejected || len(report.Issues) != 1 || !strings.Contains(report.Issues[0], "label_text") {
		t.Fatalf("failed report = %+v", report)
	}
}

func acceptedGenerationSpec(t *testing.T) CreativeVideoGenerationSpec {
	t.Helper()
	spec := CreativeVideoGenerationSpec{
		ContractVersion: "creative-video-generation-spec/v1",
		TaskID:          "creative_task_guerlain_1",
		PromptHash:      "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ConditioningAssets: []VideoConditioningAsset{
			{Role: VideoConditioningFirstFrame, AssetRef: contract.AssetVersionRef{AssetID: "asset_first", Version: 1}},
			{Role: VideoConditioningLastFrame, AssetRef: contract.AssetVersionRef{AssetID: "asset_last", Version: 1}},
		},
		DurationSeconds: 6,
		AspectRatio:     "9:16",
		Resolution:      "720p",
		AudioPolicy:     VideoAudioSilent,
		CandidateCount:  1,
		GenerationReady: true,
		ProductionReady: false,
	}
	if err := spec.Seal(); err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	return spec
}
