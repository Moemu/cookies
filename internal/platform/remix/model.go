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

	SegmentOpening Segment = "opening"
	SegmentMiddle  Segment = "middle"
	SegmentEnding  Segment = "ending"

	PaceFast     Pace = "fast"
	PaceBalanced Pace = "balanced"
	PaceStory    Pace = "story"

	RenderQueued    RenderStatus = "queued"
	RenderRunning   RenderStatus = "running"
	RenderSucceeded RenderStatus = "succeeded"
	RenderFailed    RenderStatus = "failed"
)

type Segment string
type Pace string
type RenderStatus string

type CreatePlanRequest struct {
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

type RenderJob struct {
	ID             string                    `json:"id"`
	OrganizationID contract.OrganizationID   `json:"organization_id"`
	ProjectID      contract.ProjectID        `json:"project_id"`
	PlanID         string                    `json:"plan_id"`
	Status         RenderStatus              `json:"status"`
	TargetFormat   string                    `json:"target_format"`
	TargetQuality  string                    `json:"target_quality"`
	OutputAsset    *contract.ProjectAssetRef `json:"output_asset,omitempty"`
	ErrorCode      string                    `json:"error_code,omitempty"`
	ErrorMessage   string                    `json:"error_message,omitempty"`
	CreatedBy      contract.Principal        `json:"created_by"`
	CreatedAt      time.Time                 `json:"created_at"`
	UpdatedAt      time.Time                 `json:"updated_at"`
}

func (r CreatePlanRequest) Validate() error {
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
	totalClips := 0
	for index, segment := range r.Segments {
		if err := segment.Validate(); err != nil {
			return fmt.Errorf("segment %d: %w", index, err)
		}
		if seen[segment.Segment] {
			return fmt.Errorf("segment %q is duplicated", segment.Segment)
		}
		seen[segment.Segment] = true
		totalClips += len(segment.Clips)
	}
	if !seen[SegmentOpening] || !seen[SegmentMiddle] || !seen[SegmentEnding] {
		return fmt.Errorf("opening, middle, and ending segments are required")
	}
	if totalClips > 240 {
		return fmt.Errorf("plan cannot contain more than 240 clips")
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
	for index, clip := range s.Clips {
		if err := clip.Validate(s.Segment); err != nil {
			return fmt.Errorf("clip %d: %w", index, err)
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

func (j RenderJob) Validate() error {
	if strings.TrimSpace(j.ID) == "" || strings.TrimSpace(j.PlanID) == "" {
		return fmt.Errorf("render job identity is incomplete")
	}
	if strings.TrimSpace(string(j.OrganizationID)) == "" || strings.TrimSpace(string(j.ProjectID)) == "" {
		return fmt.Errorf("render job scope is incomplete")
	}
	if j.Status != RenderQueued && j.Status != RenderRunning && j.Status != RenderSucceeded && j.Status != RenderFailed {
		return fmt.Errorf("render job status is invalid")
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
	return nil
}
