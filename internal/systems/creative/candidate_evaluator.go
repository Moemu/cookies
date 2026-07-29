package creative

import (
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type CandidateStatus string

const (
	CandidateAccepted CandidateStatus = "accepted"
	CandidateRejected CandidateStatus = "rejected"
)

type QualityDimension string

const (
	QualityBottleShape  QualityDimension = "bottle_shape"
	QualityLabelText    QualityDimension = "label_text"
	QualityProductColor QualityDimension = "product_color"
	QualityWipeMotion   QualityDimension = "wipe_motion"
	QualityFinalHold    QualityDimension = "final_hold"
)

var requiredQualityDimensions = []QualityDimension{
	QualityBottleShape,
	QualityLabelText,
	QualityProductColor,
	QualityWipeMotion,
	QualityFinalHold,
}

type ManualQualityCheck struct {
	Dimension QualityDimension `json:"dimension"`
	Passed    bool             `json:"passed"`
	Note      string           `json:"note,omitempty"`
}

type CandidateEvaluationRequest struct {
	TaskID              string
	GenerationSpec      CreativeVideoGenerationSpec
	Approval            VideoGenerationApproval
	GeneratedAsset      contract.AssetVersionRef
	DurationSeconds     int
	AspectRatio         string
	Resolution          string
	HasAudio            bool
	ReviewedBy          string
	ReviewedAt          time.Time
	ManualQualityChecks []ManualQualityCheck
}

type CandidateEvaluationReport struct {
	ContractVersion    string                   `json:"contract_version"`
	TaskID             string                   `json:"task_id"`
	GenerationSpecHash string                   `json:"generation_spec_hash"`
	GeneratedAsset     contract.AssetVersionRef `json:"generated_asset_ref"`
	Status             CandidateStatus          `json:"status"`
	Issues             []string                 `json:"issues"`
	ManualChecks       []ManualQualityCheck     `json:"manual_checks"`
	ReviewedBy         string                   `json:"reviewed_by"`
	ReviewedAt         time.Time                `json:"reviewed_at"`
}

// CandidateEvaluator keeps automated media checks and human visual judgment
// explicit. A generated video is never promoted merely because the provider
// job succeeded.
type CandidateEvaluator struct{}

func (CandidateEvaluator) Evaluate(request CandidateEvaluationRequest) (CandidateEvaluationReport, error) {
	if strings.TrimSpace(request.TaskID) == "" || request.TaskID != request.GenerationSpec.TaskID {
		return CandidateEvaluationReport{}, fmt.Errorf("candidate task must match the generation spec")
	}
	if err := request.Approval.Authorizes(request.GenerationSpec); err != nil {
		return CandidateEvaluationReport{}, fmt.Errorf("candidate approval: %w", err)
	}
	if err := request.GeneratedAsset.Validate(); err != nil {
		return CandidateEvaluationReport{}, fmt.Errorf("generated candidate asset: %w", err)
	}
	if strings.TrimSpace(request.ReviewedBy) == "" || request.ReviewedAt.IsZero() {
		return CandidateEvaluationReport{}, fmt.Errorf("candidate reviewer and review time are required")
	}
	checks, err := normalizeManualQualityChecks(request.ManualQualityChecks)
	if err != nil {
		return CandidateEvaluationReport{}, err
	}
	issues := make([]string, 0)
	if request.DurationSeconds != request.GenerationSpec.DurationSeconds {
		issues = append(issues, "duration_seconds does not match the approved generation spec")
	}
	if request.AspectRatio != request.GenerationSpec.AspectRatio {
		issues = append(issues, "aspect_ratio does not match the approved generation spec")
	}
	if request.Resolution != request.GenerationSpec.Resolution {
		issues = append(issues, "resolution does not match the approved generation spec")
	}
	if request.GenerationSpec.AudioPolicy == VideoAudioSilent && request.HasAudio {
		issues = append(issues, "audio is present but the approved generation spec is silent")
	}
	for _, check := range checks {
		if !check.Passed {
			issue := string(check.Dimension) + " failed"
			if note := strings.TrimSpace(check.Note); note != "" {
				issue += ": " + note
			}
			issues = append(issues, issue)
		}
	}
	status := CandidateAccepted
	if len(issues) > 0 {
		status = CandidateRejected
	}
	return CandidateEvaluationReport{
		ContractVersion: "creative-video-candidate-evaluation/v1",
		TaskID:          request.TaskID, GenerationSpecHash: request.GenerationSpec.Hash,
		GeneratedAsset: request.GeneratedAsset, Status: status, Issues: issues,
		ManualChecks: checks, ReviewedBy: strings.TrimSpace(request.ReviewedBy),
		ReviewedAt: request.ReviewedAt.UTC(),
	}, nil
}

func normalizeManualQualityChecks(values []ManualQualityCheck) ([]ManualQualityCheck, error) {
	if len(values) != len(requiredQualityDimensions) {
		return nil, fmt.Errorf("candidate evaluation requires all five manual quality checks")
	}
	byDimension := make(map[QualityDimension]ManualQualityCheck, len(values))
	for _, value := range values {
		switch value.Dimension {
		case QualityBottleShape, QualityLabelText, QualityProductColor, QualityWipeMotion, QualityFinalHold:
		default:
			return nil, fmt.Errorf("candidate quality dimension %q is invalid", value.Dimension)
		}
		if _, exists := byDimension[value.Dimension]; exists {
			return nil, fmt.Errorf("candidate quality dimension %q is duplicated", value.Dimension)
		}
		byDimension[value.Dimension] = value
	}
	normalized := make([]ManualQualityCheck, 0, len(requiredQualityDimensions))
	for _, dimension := range requiredQualityDimensions {
		value, exists := byDimension[dimension]
		if !exists {
			return nil, fmt.Errorf("candidate quality dimension %q is required", dimension)
		}
		value.Note = strings.TrimSpace(value.Note)
		normalized = append(normalized, value)
	}
	return normalized, nil
}
