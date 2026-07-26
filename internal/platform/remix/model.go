package remix

import (
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	ScopePlanRead  contract.Scope = "assets.read"
	ScopePlanWrite contract.Scope = "assets.write"

	SchemaVersionV1 = "remix_plan_v1"
	SchemaVersionV2 = "remix_plan_v2"

	SegmentOpening Segment = "opening"
	SegmentMiddle  Segment = "middle"
	SegmentEnding  Segment = "ending"

	ShotSourceExistingAsset = "existing_asset"

	PaceFast     Pace = "fast"
	PaceBalanced Pace = "balanced"
	PaceStory    Pace = "story"

	RenderQueued         RenderStatus = "queued"
	RenderRunning        RenderStatus = "running"
	RenderRequiresReview RenderStatus = "requires_review"
	RenderSucceeded      RenderStatus = "succeeded"
	RenderFailed         RenderStatus = "failed"

	QualityVerdictPass     QualityVerdict = "pass"
	QualityVerdictMajor    QualityVerdict = "major"
	QualityVerdictCritical QualityVerdict = "critical"
)

type Segment string
type Pace string
type RenderStatus string
type QualityVerdict string

type CreatePlanRequest struct {
	SchemaVersion string        `json:"schema_version"`
	ClientPlanID  string        `json:"client_plan_id"`
	TargetSeconds int           `json:"target_seconds"`
	ActualSeconds float64       `json:"actual_seconds"`
	Pace          Pace          `json:"pace"`
	Segments      []SegmentPlan `json:"segments"`
	Warnings      []string      `json:"warnings"`
	Summary       PlanSummary   `json:"summary"`
}

type Plan struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	CreatedBy      contract.Principal      `json:"created_by"`
	SchemaVersion  string                  `json:"schema_version"`
	ClientPlanID   string                  `json:"client_plan_id"`
	TargetSeconds  int                     `json:"target_seconds"`
	ActualSeconds  float64                 `json:"actual_seconds"`
	Pace           Pace                    `json:"pace"`
	Segments       []SegmentPlan           `json:"segments"`
	Warnings       []string                `json:"warnings"`
	Summary        PlanSummary             `json:"summary"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

type SegmentPlan struct {
	Segment       Segment `json:"segment"`
	Label         string  `json:"label"`
	TargetSeconds int     `json:"target_seconds"`
	ActualSeconds float64 `json:"actual_seconds"`
	Clips         []Clip  `json:"clips"`
	Shots         []Shot  `json:"shots"`
}

type Clip struct {
	ID              string                   `json:"id"`
	Segment         Segment                  `json:"segment"`
	AssetVersion    contract.AssetVersionRef `json:"asset_version"`
	Label           string                   `json:"label"`
	SourceType      string                   `json:"source_type"`
	MimeType        string                   `json:"mime_type"`
	Aspect          string                   `json:"aspect"`
	StartSeconds    float64                  `json:"start_seconds"`
	DurationSeconds float64                  `json:"duration_seconds"`
	InPointSeconds  float64                  `json:"in_point_seconds"`
	OutPointSeconds float64                  `json:"out_point_seconds"`
	Score           float64                  `json:"score"`
	Reason          string                   `json:"reason"`
}

type Shot struct {
	ID           string                   `json:"id"`
	Segment      Segment                  `json:"segment"`
	Source       string                   `json:"source"`
	AssetVersion contract.AssetVersionRef `json:"asset_version"`
	Timeline     ShotTimeline             `json:"timeline"`
	Creative     ShotCreative             `json:"creative"`
	Planning     ShotPlanning             `json:"planning"`
	Risks        []string                 `json:"risks"`
}

type ShotTimeline struct {
	StartSeconds    float64 `json:"start_seconds"`
	DurationSeconds float64 `json:"duration_seconds"`
	InPointSeconds  float64 `json:"in_point_seconds"`
	OutPointSeconds float64 `json:"out_point_seconds"`
}

type ShotCreative struct {
	Scene               string `json:"scene"`
	ShotType            string `json:"shot_type"`
	CameraAngle         string `json:"camera_angle"`
	DialogueOrNarration string `json:"dialogue_or_narration"`
	Subtitle            string `json:"subtitle"`
	Transition          string `json:"transition"`
	CTAElement          string `json:"cta_element"`
}

type ShotPlanning struct {
	Score       float64  `json:"score"`
	ReasonCodes []string `json:"reason_codes"`
	Reason      string   `json:"reason"`
	Evidence    []string `json:"evidence"`
}

type PlanSummary struct {
	SelectedAssets  int    `json:"selected_assets"`
	UsedAssets      int    `json:"used_assets"`
	CoveragePercent int    `json:"coverage_percent"`
	Strategy        string `json:"strategy"`
}

type CreateRenderJobRequest struct {
	PlanID        string `json:"plan_id"`
	TargetFormat  string `json:"target_format"`
	TargetQuality string `json:"target_quality"`
}

type CompleteRenderJobOutputRequest struct {
	Output                contract.ProviderOutputRef `json:"output"`
	ModelAlias            string                     `json:"model_alias"`
	ModelVersion          string                     `json:"model_version"`
	ProjectContextVersion int64                      `json:"project_context_version"`
}

type RenderJob struct {
	ID              string                    `json:"id"`
	OrganizationID  contract.OrganizationID   `json:"organization_id"`
	ProjectID       contract.ProjectID        `json:"project_id"`
	PlanID          string                    `json:"plan_id"`
	Status          RenderStatus              `json:"status"`
	Progress        int                       `json:"progress"`
	TargetFormat    string                    `json:"target_format"`
	TargetQuality   string                    `json:"target_quality"`
	IdempotencyKey  contract.IdempotencyKey   `json:"idempotency_key,omitempty"`
	RequestHash     string                    `json:"request_hash,omitempty"`
	InputSnapshot   RenderInputSnapshot       `json:"input_snapshot"`
	RequiresReview  bool                      `json:"requires_review"`
	QualityReportID string                    `json:"quality_report_id,omitempty"`
	OutputAsset     *contract.ProjectAssetRef `json:"output_asset,omitempty"`
	OutputPreview   *RenderOutputPreview      `json:"output_preview,omitempty"`
	Provenance      *RenderProvenanceSummary  `json:"provenance,omitempty"`
	ErrorCode       string                    `json:"error_code,omitempty"`
	ErrorMessage    string                    `json:"error_message,omitempty"`
	CreatedBy       contract.Principal        `json:"created_by"`
	CreatedAt       time.Time                 `json:"created_at"`
	UpdatedAt       time.Time                 `json:"updated_at"`
}

type RenderOutputPreview struct {
	URL string `json:"url"`
}

type RenderProvenanceSummary struct {
	PlanID      string                     `json:"plan_id"`
	RenderJobID string                     `json:"render_job_id"`
	InputAssets []contract.AssetVersionRef `json:"input_assets"`
}

type RenderInputSnapshot struct {
	Plan    Plan                   `json:"plan"`
	Request CreateRenderJobRequest `json:"request"`
}

type CreateQualityReportRequest struct {
	RenderJobID string                    `json:"render_job_id"`
	OutputAsset *contract.ProjectAssetRef `json:"output_asset,omitempty"`
	Policy      string                    `json:"policy,omitempty"`
}

type QualityReport struct {
	ID                string                    `json:"id"`
	OrganizationID    contract.OrganizationID   `json:"organization_id"`
	ProjectID         contract.ProjectID        `json:"project_id"`
	RenderJobID       string                    `json:"render_job_id"`
	OutputAsset       *contract.ProjectAssetRef `json:"output_asset,omitempty"`
	Verdict           QualityVerdict            `json:"verdict"`
	Score             float64                   `json:"score"`
	Dimensions        []QualityDimension        `json:"dimensions"`
	Issues            []QualityIssue            `json:"issues"`
	Evidence          []QualityEvidence         `json:"evidence"`
	RepairSuggestions []string                  `json:"repair_suggestions"`
	CreatedBy         contract.Principal        `json:"created_by"`
	CreatedAt         time.Time                 `json:"created_at"`
	UpdatedAt         time.Time                 `json:"updated_at"`
}

type QualityDimension struct {
	Name    string  `json:"name"`
	Score   float64 `json:"score"`
	Verdict string  `json:"verdict"`
	Summary string  `json:"summary"`
}

type QualityIssue struct {
	Code             string         `json:"code"`
	Severity         QualityVerdict `json:"severity"`
	Dimension        string         `json:"dimension"`
	StartSeconds     float64        `json:"start_seconds"`
	EndSeconds       float64        `json:"end_seconds"`
	Description      string         `json:"description"`
	RepairSuggestion string         `json:"repair_suggestion"`
}

type QualityEvidence struct {
	Kind         string  `json:"kind"`
	TimestampSec float64 `json:"timestamp_sec"`
	Summary      string  `json:"summary"`
}

func (r CreatePlanRequest) Validate() error {
	if r.SchemaVersion != "" && r.SchemaVersion != SchemaVersionV1 && r.SchemaVersion != SchemaVersionV2 {
		return fmt.Errorf("schema_version must be remix_plan_v1 or remix_plan_v2")
	}
	if strings.TrimSpace(r.ClientPlanID) == "" || len(r.ClientPlanID) > 128 {
		return fmt.Errorf("client_plan_id must be between 1 and 128 characters")
	}
	if r.TargetSeconds < 9 || r.TargetSeconds > 180 {
		return fmt.Errorf("target_seconds must be between 9 and 180")
	}
	if r.ActualSeconds < 0 || r.ActualSeconds > 360 {
		return fmt.Errorf("actual_seconds must be between 0 and 360")
	}
	if r.Pace != PaceFast && r.Pace != PaceBalanced && r.Pace != PaceStory {
		return fmt.Errorf("pace must be fast, balanced, or story")
	}
	if len(r.Segments) != 3 {
		return fmt.Errorf("exactly three segments are required")
	}
	seen := map[Segment]bool{}
	totalTimelineEntries := 0
	for index, segment := range r.Segments {
		if err := segment.Validate(); err != nil {
			return fmt.Errorf("segment %d: %w", index, err)
		}
		if seen[segment.Segment] {
			return fmt.Errorf("segment %q is duplicated", segment.Segment)
		}
		seen[segment.Segment] = true
		totalTimelineEntries += len(segment.Clips)
		if len(segment.Shots) > 0 {
			totalTimelineEntries += len(segment.Shots)
		}
	}
	if !seen[SegmentOpening] || !seen[SegmentMiddle] || !seen[SegmentEnding] {
		return fmt.Errorf("opening, middle, and ending segments are required")
	}
	if totalTimelineEntries > 240 {
		return fmt.Errorf("plan cannot contain more than 240 timeline entries")
	}
	if len(r.Warnings) > 20 {
		return fmt.Errorf("plan cannot contain more than 20 warnings")
	}
	if err := r.Summary.Validate(); err != nil {
		return fmt.Errorf("summary: %w", err)
	}
	return nil
}

func (p Plan) Validate() error {
	if strings.TrimSpace(p.ID) == "" || strings.TrimSpace(string(p.OrganizationID)) == "" || strings.TrimSpace(string(p.ProjectID)) == "" {
		return fmt.Errorf("plan identity is incomplete")
	}
	return CreatePlanRequest{
		SchemaVersion: p.SchemaVersion,
		ClientPlanID:  p.ClientPlanID,
		TargetSeconds: p.TargetSeconds,
		ActualSeconds: p.ActualSeconds,
		Pace:          p.Pace,
		Segments:      p.Segments,
		Warnings:      p.Warnings,
		Summary:       p.Summary,
	}.Validate()
}

func (s SegmentPlan) Validate() error {
	if s.Segment != SegmentOpening && s.Segment != SegmentMiddle && s.Segment != SegmentEnding {
		return fmt.Errorf("segment must be opening, middle, or ending")
	}
	if strings.TrimSpace(s.Label) == "" || len(s.Label) > 64 {
		return fmt.Errorf("label must be between 1 and 64 characters")
	}
	if s.TargetSeconds < 0 || s.TargetSeconds > 180 || s.ActualSeconds < 0 || s.ActualSeconds > 180 {
		return fmt.Errorf("segment seconds are outside supported range")
	}
	if len(s.Clips) > 80 {
		return fmt.Errorf("segment cannot contain more than 80 clips")
	}
	if len(s.Shots) > 80 {
		return fmt.Errorf("segment cannot contain more than 80 shots")
	}
	for index, clip := range s.Clips {
		if err := clip.Validate(s.Segment); err != nil {
			return fmt.Errorf("clip %d: %w", index, err)
		}
	}
	for index, shot := range s.Shots {
		if err := shot.Validate(s.Segment); err != nil {
			return fmt.Errorf("shot %d: %w", index, err)
		}
	}
	return nil
}

func (c Clip) Validate(segment Segment) error {
	if strings.TrimSpace(c.ID) == "" || len(c.ID) > 160 {
		return fmt.Errorf("id must be between 1 and 160 characters")
	}
	if c.Segment != segment {
		return fmt.Errorf("clip segment must match parent segment")
	}
	if err := c.AssetVersion.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(c.MimeType) == "" || !strings.HasPrefix(c.MimeType, "video/") {
		return fmt.Errorf("clip mime_type must be a video MIME type")
	}
	if c.DurationSeconds <= 0 || c.DurationSeconds > 60 || c.OutPointSeconds < c.InPointSeconds {
		return fmt.Errorf("clip timing is invalid")
	}
	if c.Score < 0 || c.Score > 1 {
		return fmt.Errorf("clip score must be between 0 and 1")
	}
	return nil
}

func (s Shot) Validate(segment Segment) error {
	if strings.TrimSpace(s.ID) == "" || len(s.ID) > 160 {
		return fmt.Errorf("id must be between 1 and 160 characters")
	}
	if s.Segment != segment {
		return fmt.Errorf("shot segment must match parent segment")
	}
	if s.Source != ShotSourceExistingAsset {
		return fmt.Errorf("shot source must be existing_asset")
	}
	if err := s.AssetVersion.Validate(); err != nil {
		return err
	}
	if err := s.Timeline.Validate(); err != nil {
		return err
	}
	if s.Planning.Score < 0 || s.Planning.Score > 1 {
		return fmt.Errorf("shot planning score must be between 0 and 1")
	}
	if len(s.Planning.ReasonCodes) > 20 || len(s.Planning.Evidence) > 20 || len(s.Risks) > 20 {
		return fmt.Errorf("shot annotations exceed supported limits")
	}
	return nil
}

func (t ShotTimeline) Validate() error {
	if t.StartSeconds < 0 || t.DurationSeconds <= 0 || t.DurationSeconds > 60 {
		return fmt.Errorf("shot timing is invalid")
	}
	if t.InPointSeconds < 0 || t.OutPointSeconds < t.InPointSeconds {
		return fmt.Errorf("shot timing is invalid")
	}
	return nil
}

func (s PlanSummary) Validate() error {
	if s.SelectedAssets < 0 || s.UsedAssets < 0 || s.CoveragePercent < 0 || s.CoveragePercent > 100 {
		return fmt.Errorf("summary counters are outside supported range")
	}
	if len(s.Strategy) > 280 {
		return fmt.Errorf("strategy is too long")
	}
	return nil
}

func (r CreateRenderJobRequest) Validate() error {
	if strings.TrimSpace(r.PlanID) == "" || len(r.PlanID) > 128 {
		return fmt.Errorf("plan_id must be between 1 and 128 characters")
	}
	if r.TargetFormat != "" && r.TargetFormat != "mp4" {
		return fmt.Errorf("target_format must be mp4")
	}
	if r.TargetQuality != "" && r.TargetQuality != "draft" && r.TargetQuality != "standard" && r.TargetQuality != "high" {
		return fmt.Errorf("target_quality must be draft, standard, or high")
	}
	return nil
}

func (r CompleteRenderJobOutputRequest) Validate() error {
	if err := r.Output.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.ModelAlias) == "" || len(r.ModelAlias) > 128 {
		return fmt.Errorf("model_alias is required")
	}
	if strings.TrimSpace(r.ModelVersion) == "" || len(r.ModelVersion) > 128 {
		return fmt.Errorf("model_version is required")
	}
	if r.ProjectContextVersion < 1 {
		return fmt.Errorf("project_context_version must be positive")
	}
	return nil
}

func (j RenderJob) Validate() error {
	if strings.TrimSpace(j.ID) == "" || strings.TrimSpace(j.PlanID) == "" {
		return fmt.Errorf("render job identity is incomplete")
	}
	if strings.TrimSpace(string(j.OrganizationID)) == "" || strings.TrimSpace(string(j.ProjectID)) == "" {
		return fmt.Errorf("render job scope is incomplete")
	}
	if j.Status != RenderQueued && j.Status != RenderRunning && j.Status != RenderRequiresReview && j.Status != RenderSucceeded && j.Status != RenderFailed {
		return fmt.Errorf("render job status is invalid")
	}
	if j.Progress < 0 || j.Progress > 100 {
		return fmt.Errorf("render job progress must be between 0 and 100")
	}
	if j.TargetFormat != "mp4" {
		return fmt.Errorf("render job target_format must be mp4")
	}
	if j.TargetQuality != "draft" && j.TargetQuality != "standard" && j.TargetQuality != "high" {
		return fmt.Errorf("render job target_quality is invalid")
	}
	if j.OutputAsset != nil {
		if err := j.OutputAsset.Validate(); err != nil {
			return err
		}
	}
	if j.Status == RenderRequiresReview && !j.RequiresReview {
		return fmt.Errorf("render job requires_review must be true for requires_review status")
	}
	return nil
}

func (r CreateQualityReportRequest) Validate() error {
	if strings.TrimSpace(r.RenderJobID) == "" || len(r.RenderJobID) > 128 {
		return fmt.Errorf("render_job_id must be between 1 and 128 characters")
	}
	if r.OutputAsset != nil {
		if err := r.OutputAsset.Validate(); err != nil {
			return err
		}
	}
	if r.Policy != "" && r.Policy != "fail_critical" && r.Policy != "review_all_issues" {
		return fmt.Errorf("policy must be fail_critical or review_all_issues")
	}
	return nil
}

func (r QualityReport) Validate() error {
	if strings.TrimSpace(r.ID) == "" || strings.TrimSpace(r.RenderJobID) == "" {
		return fmt.Errorf("quality report identity is incomplete")
	}
	if strings.TrimSpace(string(r.OrganizationID)) == "" || strings.TrimSpace(string(r.ProjectID)) == "" {
		return fmt.Errorf("quality report scope is incomplete")
	}
	if r.OutputAsset != nil {
		if err := r.OutputAsset.Validate(); err != nil {
			return err
		}
	}
	if r.Verdict != QualityVerdictPass && r.Verdict != QualityVerdictMajor && r.Verdict != QualityVerdictCritical {
		return fmt.Errorf("quality report verdict is invalid")
	}
	if r.Score < 0 || r.Score > 1 {
		return fmt.Errorf("quality report score must be between 0 and 1")
	}
	if len(r.Dimensions) == 0 || len(r.Dimensions) > 20 {
		return fmt.Errorf("quality report dimensions must be between 1 and 20")
	}
	for index, dimension := range r.Dimensions {
		if err := dimension.Validate(); err != nil {
			return fmt.Errorf("dimension %d: %w", index, err)
		}
	}
	if len(r.Issues) > 50 || len(r.Evidence) > 50 || len(r.RepairSuggestions) > 50 {
		return fmt.Errorf("quality report annotations exceed supported limits")
	}
	for index, issue := range r.Issues {
		if err := issue.Validate(); err != nil {
			return fmt.Errorf("issue %d: %w", index, err)
		}
	}
	for index, evidence := range r.Evidence {
		if err := evidence.Validate(); err != nil {
			return fmt.Errorf("evidence %d: %w", index, err)
		}
	}
	return nil
}

func (d QualityDimension) Validate() error {
	if strings.TrimSpace(d.Name) == "" || len(d.Name) > 64 {
		return fmt.Errorf("name must be between 1 and 64 characters")
	}
	if d.Score < 0 || d.Score > 1 {
		return fmt.Errorf("score must be between 0 and 1")
	}
	if d.Verdict != string(QualityVerdictPass) && d.Verdict != string(QualityVerdictMajor) && d.Verdict != string(QualityVerdictCritical) {
		return fmt.Errorf("verdict is invalid")
	}
	if len(d.Summary) > 280 {
		return fmt.Errorf("summary is too long")
	}
	return nil
}

func (i QualityIssue) Validate() error {
	if strings.TrimSpace(i.Code) == "" || len(i.Code) > 96 {
		return fmt.Errorf("code must be between 1 and 96 characters")
	}
	if i.Severity != QualityVerdictMajor && i.Severity != QualityVerdictCritical {
		return fmt.Errorf("severity must be major or critical")
	}
	if strings.TrimSpace(i.Dimension) == "" || len(i.Dimension) > 64 {
		return fmt.Errorf("dimension must be between 1 and 64 characters")
	}
	if i.StartSeconds < 0 || i.EndSeconds < i.StartSeconds {
		return fmt.Errorf("issue timing is invalid")
	}
	if strings.TrimSpace(i.Description) == "" || len(i.Description) > 500 {
		return fmt.Errorf("description must be between 1 and 500 characters")
	}
	if len(i.RepairSuggestion) > 500 {
		return fmt.Errorf("repair_suggestion is too long")
	}
	return nil
}

func (e QualityEvidence) Validate() error {
	if strings.TrimSpace(e.Kind) == "" || len(e.Kind) > 64 {
		return fmt.Errorf("kind must be between 1 and 64 characters")
	}
	if e.TimestampSec < 0 {
		return fmt.Errorf("timestamp_sec must be non-negative")
	}
	if strings.TrimSpace(e.Summary) == "" || len(e.Summary) > 500 {
		return fmt.Errorf("summary must be between 1 and 500 characters")
	}
	return nil
}
