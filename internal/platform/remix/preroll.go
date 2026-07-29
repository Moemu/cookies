package remix

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	HookTypeConflict           HookType = "conflict"
	HookTypeReversal           HookType = "reversal"
	HookTypeSuspense           HookType = "suspense"
	HookTypeSellingPointBridge HookType = "selling_point_bridge"
	HookTypeProductDemo        HookType = "product_demo"
	HookTypeOffer              HookType = "offer"

	PrerollModePromptOnly    PrerollMode = "prompt_only"
	PrerollModeGenerateVideo PrerollMode = "generate_video"

	PrerollDraft   PrerollStatus = "draft"
	PrerollReady   PrerollStatus = "ready"
	PrerollFailed  PrerollStatus = "failed"
	PrerollApplied PrerollStatus = "applied"
)

type HookType string
type PrerollMode string
type PrerollStatus string

type CreatePrerollRequest struct {
	PlanID           string                   `json:"plan_id"`
	HookType         HookType                 `json:"hook_type"`
	ReferenceAsset   contract.AssetVersionRef `json:"reference_asset"`
	StyleConstraints []string                 `json:"style_constraints"`
	DurationSeconds  float64                  `json:"duration_seconds"`
	Mode             PrerollMode              `json:"mode"`
}

type Preroll struct {
	ID               string                    `json:"id"`
	OrganizationID   contract.OrganizationID   `json:"organization_id"`
	ProjectID        contract.ProjectID        `json:"project_id"`
	PlanID           string                    `json:"plan_id"`
	HookType         HookType                  `json:"hook_type"`
	ReferenceAsset   contract.AssetVersionRef  `json:"reference_asset"`
	StyleConstraints []string                  `json:"style_constraints"`
	DurationSeconds  float64                   `json:"duration_seconds"`
	Mode             PrerollMode               `json:"mode"`
	PromptDraft      string                    `json:"prompt_draft"`
	OutputAsset      *contract.ProjectAssetRef `json:"output_asset,omitempty"`
	QualityVerdict   QualityVerdict            `json:"quality_verdict"`
	Status           PrerollStatus             `json:"status"`
	ErrorCode        string                    `json:"error_code,omitempty"`
	ErrorMessage     string                    `json:"error_message,omitempty"`
	AppliedPlanID    string                    `json:"applied_plan_id,omitempty"`
	CreatedBy        contract.Principal        `json:"created_by"`
	CreatedAt        time.Time                 `json:"created_at"`
	UpdatedAt        time.Time                 `json:"updated_at"`
}

type PrerollGenerator interface {
	GeneratePreroll(context.Context, PrerollGenerationInput) (PrerollGenerationResult, error)
}

type PrerollGenerationInput struct {
	PrerollID       string
	Plan            Plan
	HookType        HookType
	ReferenceAsset  contract.AssetVersionRef
	DurationSeconds float64
	Mode            PrerollMode
	Constraints     []string
}

type PrerollGenerationResult struct {
	PromptDraft    string
	OutputAsset    *contract.ProjectAssetRef
	QualityVerdict QualityVerdict
	ErrorCode      string
	ErrorMessage   string
}

type FakePrerollGenerator struct{}

func (FakePrerollGenerator) GeneratePreroll(_ context.Context, input PrerollGenerationInput) (PrerollGenerationResult, error) {
	prompt := fmt.Sprintf("为 RemixPlan %s 生成 %.1f 秒 %s 前贴 Hook，参考素材 %s@v%d，9:16 竖版，3 秒内给出强钩子并保留自然拼接点。",
		input.Plan.ID, input.DurationSeconds, input.HookType, input.ReferenceAsset.AssetID, input.ReferenceAsset.Version)
	if len(input.Constraints) > 0 {
		prompt += " 约束：" + strings.Join(input.Constraints, "；") + "。"
	}
	if containsExact(input.Constraints, "quality:critical") {
		return PrerollGenerationResult{PromptDraft: prompt, QualityVerdict: QualityVerdictCritical, ErrorCode: "QUALITY_CRITICAL", ErrorMessage: "前贴质检存在 critical 问题，已阻断插入"}, nil
	}
	if containsExact(input.Constraints, "quality:major") {
		return PrerollGenerationResult{PromptDraft: prompt, QualityVerdict: QualityVerdictMajor, ErrorCode: "QUALITY_REVIEW_REQUIRED", ErrorMessage: "前贴质检存在 major 问题，需要重新生成"}, nil
	}
	result := PrerollGenerationResult{PromptDraft: prompt, QualityVerdict: QualityVerdictPass}
	if input.Mode == PrerollModeGenerateVideo {
		result.OutputAsset = &contract.ProjectAssetRef{
			ProjectID: input.Plan.ProjectID,
			AssetVersion: contract.AssetVersionRef{
				AssetID: contract.AssetID(input.PrerollID + "_asset"),
				Version: 1,
			},
		}
	}
	return result, nil
}

func (r CreatePrerollRequest) Validate() error {
	if strings.TrimSpace(r.PlanID) == "" || len(r.PlanID) > 128 {
		return fmt.Errorf("plan_id must be between 1 and 128 characters")
	}
	if !validHookType(r.HookType) {
		return fmt.Errorf("hook_type is invalid")
	}
	if err := r.ReferenceAsset.Validate(); err != nil {
		return err
	}
	duration := r.DurationSeconds
	if duration == 0 {
		duration = 6
	}
	if duration < 3 || duration > 10 {
		return fmt.Errorf("duration_seconds must be between 3 and 10")
	}
	if r.Mode != "" && r.Mode != PrerollModePromptOnly && r.Mode != PrerollModeGenerateVideo {
		return fmt.Errorf("mode must be prompt_only or generate_video")
	}
	if len(r.StyleConstraints) > 20 {
		return fmt.Errorf("style_constraints cannot contain more than 20 items")
	}
	return nil
}

func (p Preroll) Validate() error {
	if strings.TrimSpace(p.ID) == "" || strings.TrimSpace(p.PlanID) == "" {
		return fmt.Errorf("preroll identity is incomplete")
	}
	if strings.TrimSpace(string(p.OrganizationID)) == "" || strings.TrimSpace(string(p.ProjectID)) == "" {
		return fmt.Errorf("preroll scope is incomplete")
	}
	if !validHookType(p.HookType) {
		return fmt.Errorf("hook_type is invalid")
	}
	if err := p.ReferenceAsset.Validate(); err != nil {
		return err
	}
	if p.DurationSeconds < 3 || p.DurationSeconds > 10 {
		return fmt.Errorf("duration_seconds must be between 3 and 10")
	}
	if p.Mode != PrerollModePromptOnly && p.Mode != PrerollModeGenerateVideo {
		return fmt.Errorf("preroll mode is invalid")
	}
	if p.Status != PrerollDraft && p.Status != PrerollReady && p.Status != PrerollFailed && p.Status != PrerollApplied {
		return fmt.Errorf("preroll status is invalid")
	}
	if p.QualityVerdict != QualityVerdictPass && p.QualityVerdict != QualityVerdictMajor && p.QualityVerdict != QualityVerdictCritical {
		return fmt.Errorf("quality_verdict is invalid")
	}
	if p.OutputAsset != nil {
		if err := p.OutputAsset.Validate(); err != nil {
			return err
		}
	}
	if strings.TrimSpace(p.PromptDraft) == "" {
		return fmt.Errorf("prompt_draft is required")
	}
	return nil
}

func (s *Service) CreatePreroll(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreatePrerollRequest) (Preroll, error) {
	if err := ctx.Err(); err != nil {
		return Preroll{}, err
	}
	request = normalizeCreatePrerollRequest(request)
	if err := request.Validate(); err != nil {
		return Preroll{}, err
	}
	s.mu.RLock()
	plan, ok := s.plans[request.PlanID]
	s.mu.RUnlock()
	if !ok || plan.OrganizationID != actor.OrganizationID || plan.ProjectID != projectID {
		return Preroll{}, ErrNotFound
	}
	id, err := s.newID()
	if err != nil {
		return Preroll{}, err
	}
	result, err := s.prerollGenerator.GeneratePreroll(ctx, PrerollGenerationInput{
		PrerollID: id, Plan: clonePlan(plan), HookType: request.HookType, ReferenceAsset: request.ReferenceAsset,
		DurationSeconds: request.DurationSeconds, Mode: request.Mode, Constraints: append([]string(nil), request.StyleConstraints...),
	})
	if err != nil {
		return Preroll{}, err
	}
	status := PrerollDraft
	if request.Mode == PrerollModeGenerateVideo && result.OutputAsset != nil && result.QualityVerdict == QualityVerdictPass {
		status = PrerollReady
	}
	if result.QualityVerdict == QualityVerdictMajor || result.QualityVerdict == QualityVerdictCritical {
		status = PrerollFailed
	}
	now := s.nowUTC()
	preroll := Preroll{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID, PlanID: plan.ID, HookType: request.HookType,
		ReferenceAsset: request.ReferenceAsset, StyleConstraints: append([]string(nil), request.StyleConstraints...),
		DurationSeconds: request.DurationSeconds, Mode: request.Mode, PromptDraft: result.PromptDraft, OutputAsset: result.OutputAsset,
		QualityVerdict: result.QualityVerdict, Status: status, ErrorCode: result.ErrorCode, ErrorMessage: result.ErrorMessage,
		CreatedBy: actor.Principal, CreatedAt: now, UpdatedAt: now,
	}
	if err := preroll.Validate(); err != nil {
		return Preroll{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prerolls[id] = clonePreroll(preroll)
	return clonePreroll(preroll), nil
}

func (s *Service) GetPreroll(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (Preroll, error) {
	if err := ctx.Err(); err != nil {
		return Preroll{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	preroll, ok := s.prerolls[id]
	if !ok || preroll.OrganizationID != actor.OrganizationID || preroll.ProjectID != projectID {
		return Preroll{}, ErrNotFound
	}
	return clonePreroll(preroll), nil
}

func (s *Service) ApplyPreroll(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (Plan, error) {
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	preroll, ok := s.prerolls[id]
	if !ok || preroll.OrganizationID != actor.OrganizationID || preroll.ProjectID != projectID {
		return Plan{}, ErrNotFound
	}
	if preroll.Status != PrerollReady || preroll.QualityVerdict != QualityVerdictPass || preroll.OutputAsset == nil {
		return Plan{}, ErrPrerollNotReady
	}
	plan, ok := s.plans[preroll.PlanID]
	if !ok || plan.OrganizationID != actor.OrganizationID || plan.ProjectID != projectID {
		return Plan{}, ErrNotFound
	}
	plan = applyPrerollToPlan(plan, preroll, s.nowUTC())
	preroll.Status = PrerollApplied
	preroll.AppliedPlanID = plan.ID
	preroll.UpdatedAt = plan.UpdatedAt
	if err := plan.Validate(); err != nil {
		return Plan{}, err
	}
	if err := preroll.Validate(); err != nil {
		return Plan{}, err
	}
	s.plans[plan.ID] = clonePlan(plan)
	s.prerolls[preroll.ID] = clonePreroll(preroll)
	return clonePlan(plan), nil
}

func normalizeCreatePrerollRequest(request CreatePrerollRequest) CreatePrerollRequest {
	if request.DurationSeconds == 0 {
		request.DurationSeconds = 6
	}
	if request.Mode == "" {
		request.Mode = PrerollModePromptOnly
	}
	request.StyleConstraints = append([]string(nil), request.StyleConstraints...)
	return request
}

func validHookType(value HookType) bool {
	switch value {
	case HookTypeConflict, HookTypeReversal, HookTypeSuspense, HookTypeSellingPointBridge, HookTypeProductDemo, HookTypeOffer:
		return true
	default:
		return false
	}
}

func applyPrerollToPlan(plan Plan, preroll Preroll, now time.Time) Plan {
	plan = clonePlan(plan)
	prerollShot := Shot{
		ID:           preroll.ID + "_shot",
		Segment:      SegmentOpening,
		Source:       ShotSourceExistingAsset,
		AssetVersion: preroll.OutputAsset.AssetVersion,
		Timeline:     ShotTimeline{StartSeconds: 0, DurationSeconds: preroll.DurationSeconds, InPointSeconds: 0, OutPointSeconds: preroll.DurationSeconds},
		Creative: ShotCreative{
			Scene:               "AI 前贴 Hook",
			ShotType:            string(preroll.HookType),
			DialogueOrNarration: preroll.PromptDraft,
			Subtitle:            "3 秒内制造停留理由",
			Transition:          "cut",
		},
		Planning: ShotPlanning{
			Score:       0.92,
			ReasonCodes: []string{"ai_preroll", string(preroll.HookType), "quality_pass"},
			Reason:      "AI 前贴通过质量门禁后插入 opening 段",
			Evidence:    []string{preroll.PromptDraft},
		},
		Risks: []string{},
	}
	actualSeconds := 0.0
	for index := range plan.Segments {
		segment := &plan.Segments[index]
		if segment.Segment == SegmentOpening {
			shots := cloneShots(segment.Shots)
			for shotIndex := range shots {
				shots[shotIndex].Timeline.StartSeconds = roundTimeline(shots[shotIndex].Timeline.StartSeconds + preroll.DurationSeconds)
			}
			segment.Shots = append([]Shot{prerollShot}, shots...)
			segment.Clips = clipsFromShots(segment.Shots)
			segment.ActualSeconds = sumShotSeconds(segment.Shots)
		}
		actualSeconds += segment.ActualSeconds
	}
	plan.ActualSeconds = roundTimeline(actualSeconds)
	plan.Summary.UsedAssets = countUsedAssets(plan.Segments)
	plan.Summary.SelectedAssets = plan.Summary.UsedAssets
	plan.Summary.CoveragePercent = coveragePercent(plan.ActualSeconds, plan.TargetSeconds)
	if !containsExact(plan.Warnings, "ai_preroll_applied") {
		plan.Warnings = append(plan.Warnings, "ai_preroll_applied")
	}
	plan.UpdatedAt = now
	return plan
}

func clonePreroll(preroll Preroll) Preroll {
	preroll.StyleConstraints = append([]string(nil), preroll.StyleConstraints...)
	if preroll.OutputAsset != nil {
		output := *preroll.OutputAsset
		preroll.OutputAsset = &output
	}
	return preroll
}

func containsExact(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
