package creative

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	AINativeRequirementDraftStatus      = "draft"
	AINativeRequirementConfirmedStatus  = "confirmed"
	AINativeRequirementSupersededStatus = "superseded"
	AINativeStageRequirement            = "requirement"
	PerformanceModeAINativeAd           = "ai_ad_generation"
)

type AINativeRequirementWorkspace struct {
	WorkspaceID                 string                      `json:"workspace_id"`
	CreativeIntakeID            string                      `json:"creative_intake_id"`
	CreativeTaskID              string                      `json:"creative_task_id"`
	OrganizationID              contract.OrganizationID     `json:"organization_id"`
	ProjectID                   contract.ProjectID          `json:"project_id"`
	Status                      string                      `json:"status"`
	CurrentStage                string                      `json:"current_stage"`
	WorkspaceVersion            int64                       `json:"workspace_version"`
	ActiveOperationID           string                      `json:"active_operation_id,omitempty"`
	ActiveOperationVersion      *int64                      `json:"active_operation_version,omitempty"`
	CurrentRevision             int64                       `json:"current_revision"`
	ConfirmedRevision           *int64                      `json:"confirmed_revision,omitempty"`
	Requirement                 AINativeRequirementDraft    `json:"requirement"`
	ScriptStatus                string                      `json:"script_status,omitempty"`
	CurrentScriptRevision       *int64                      `json:"current_script_revision,omitempty"`
	ConfirmedScriptRevision     *int64                      `json:"confirmed_script_revision,omitempty"`
	Script                      *AINativeScriptRevision     `json:"script,omitempty"`
	StoryboardStatus            string                      `json:"storyboard_status,omitempty"`
	CurrentStoryboardRevision   *int64                      `json:"current_storyboard_revision,omitempty"`
	ConfirmedStoryboardRevision *int64                      `json:"confirmed_storyboard_revision,omitempty"`
	Storyboard                  *AINativeStoryboardRevision `json:"storyboard,omitempty"`
	StoryboardPlan              *AINativeStoryboardRevision `json:"storyboard_plan,omitempty"`
	ProductionStatus            string                      `json:"production_status,omitempty"`
	CurrentProductionRevision   *int64                      `json:"current_production_revision,omitempty"`
	ProductionPlan              *AINativeProductionPlan     `json:"production_plan,omitempty"`
	ProductionProgress          *AINativeProductionProgress `json:"production_progress,omitempty"`
	CreatedBy                   string                      `json:"created_by"`
	ConfirmedBy                 string                      `json:"confirmed_by,omitempty"`
	CreatedAt                   time.Time                   `json:"created_at"`
	UpdatedAt                   time.Time                   `json:"updated_at"`
}

func (w AINativeRequirementWorkspace) Validate() error {
	if strings.TrimSpace(w.WorkspaceID) == "" || strings.TrimSpace(w.CreativeIntakeID) == "" || strings.TrimSpace(w.CreativeTaskID) == "" ||
		w.OrganizationID == "" || w.ProjectID == "" || (w.CurrentStage != AINativeStageRequirement && w.CurrentStage != AINativeStageScript && w.CurrentStage != AINativeStageStoryboard && w.CurrentStage != AINativeStageProduction) || w.WorkspaceVersion < 1 ||
		(w.Status != AINativeRequirementDraftStatus && w.Status != AINativeRequirementConfirmedStatus) ||
		w.CurrentRevision < 1 || w.Requirement.Revision != w.CurrentRevision || strings.TrimSpace(w.CreatedBy) == "" ||
		w.CreatedAt.IsZero() || w.UpdatedAt.IsZero() {
		return fmt.Errorf("AI native requirement workspace is invalid")
	}
	if w.Status == AINativeRequirementConfirmedStatus {
		if w.ConfirmedRevision == nil || *w.ConfirmedRevision != w.CurrentRevision || strings.TrimSpace(w.ConfirmedBy) == "" {
			return fmt.Errorf("confirmed AI native requirement workspace is invalid")
		}
	} else if w.ConfirmedRevision != nil || strings.TrimSpace(w.ConfirmedBy) != "" {
		return fmt.Errorf("draft AI native requirement workspace cannot be confirmed")
	}
	if (strings.TrimSpace(w.ActiveOperationID) == "") != (w.ActiveOperationVersion == nil) ||
		(w.ActiveOperationVersion != nil && *w.ActiveOperationVersion < 1) {
		return fmt.Errorf("AI native active operation reference is invalid")
	}
	if w.CurrentStage == AINativeStageScript {
		if w.Status != AINativeRequirementConfirmedStatus ||
			(w.ScriptStatus != AINativeScriptGeneratingStatus && w.ScriptStatus != AINativeScriptDraftStatus && w.ScriptStatus != AINativeScriptConfirmedStatus && w.ScriptStatus != AINativeScriptFailedStatus) {
			return fmt.Errorf("AI native script workspace is invalid")
		}
		if w.ScriptStatus == AINativeScriptGeneratingStatus && strings.TrimSpace(w.ActiveOperationID) == "" {
			return fmt.Errorf("AI native script generation operation is missing")
		}
		if w.Script != nil {
			if w.CurrentScriptRevision == nil || w.Script.Revision != *w.CurrentScriptRevision || w.Script.ValidateAgainst(w.Requirement) != nil {
				return fmt.Errorf("AI native current script is invalid")
			}
		}
	}
	if w.CurrentStage == AINativeStageStoryboard {
		if w.Status != AINativeRequirementConfirmedStatus || w.ScriptStatus != AINativeScriptConfirmedStatus ||
			(w.StoryboardStatus != AINativeStoryboardGeneratingStatus && w.StoryboardStatus != AINativeStoryboardDraftStatus && w.StoryboardStatus != AINativeStoryboardConfirmedStatus && w.StoryboardStatus != AINativeStoryboardFailedStatus) {
			return fmt.Errorf("AI native storyboard workspace is invalid")
		}
		if w.StoryboardStatus == AINativeStoryboardGeneratingStatus && strings.TrimSpace(w.ActiveOperationID) == "" {
			return fmt.Errorf("AI native storyboard generation operation is missing")
		}
		if w.Storyboard != nil {
			if w.CurrentStoryboardRevision == nil || w.Storyboard.Revision != *w.CurrentStoryboardRevision || w.Script == nil || w.Storyboard.ValidatePlanAgainst(w.Requirement, *w.Script) != nil {
				return fmt.Errorf("AI native current storyboard is invalid")
			}
		}
	}
	if w.CurrentStage == AINativeStageProduction {
		if w.StoryboardStatus != AINativeStoryboardConfirmedStatus || w.ConfirmedStoryboardRevision == nil || w.ProductionPlan == nil || w.CurrentProductionRevision == nil ||
			w.ProductionPlan.Revision != *w.CurrentProductionRevision || (w.ProductionStatus != AINativeProductionRunningStatus && w.ProductionStatus != AINativeProductionReadyStatus && w.ProductionStatus != AINativeProductionRenderingStatus && w.ProductionStatus != AINativeProductionCompletedStatus && w.ProductionStatus != AINativeProductionRenderFailedStatus && w.ProductionStatus != AINativeProductionFailedStatus && w.ProductionStatus != AINativeProductionCancelledStatus) {
			return fmt.Errorf("AI native production workspace is invalid")
		}
		if (w.ProductionStatus == AINativeProductionRunningStatus || w.ProductionStatus == AINativeProductionRenderingStatus) && strings.TrimSpace(w.ActiveOperationID) == "" {
			return fmt.Errorf("AI native production operation is missing")
		}
	}
	return w.Requirement.Validate()
}

type AINativeRequirementRepository interface {
	CreateAINativeRequirementWorkspace(context.Context, AINativeRequirementWorkspace) (AINativeRequirementWorkspace, error)
	GetAINativeRequirementWorkspace(context.Context, contract.OrganizationID, contract.ProjectID, string) (AINativeRequirementWorkspace, error)
	AppendAINativeRequirementRevision(context.Context, AINativeRequirementWorkspace, int64, string) (AINativeRequirementWorkspace, error)
	ConfirmAINativeRequirement(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, string, time.Time) (AINativeRequirementWorkspace, error)
	GetAINativeReopenImpact(context.Context, contract.OrganizationID, contract.ProjectID, string, string) (AINativeReopenImpact, error)
	ReopenAINativeRequirement(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, string, time.Time) (AINativeRequirementWorkspace, error)
}

type AINativeInvalidatedResource struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Status  string `json:"status"`
	Version int64  `json:"version,omitempty"`
}

type AINativeOperationCanceller interface {
	CancelAINativeOperation(context.Context, contract.OrganizationID, contract.ProjectID, string, int64) error
}

type AINativeReopenImpact struct {
	WorkspaceID                    string                        `json:"workspace_id"`
	Stage                          string                        `json:"stage"`
	ExpectedWorkspaceVersion       int64                         `json:"expected_workspace_version"`
	SupersededRequirementRevisions []int64                       `json:"superseded_requirement_revisions"`
	SupersededScriptRevisions      []int64                       `json:"superseded_script_revisions,omitempty"`
	SupersededStoryboardRevisions  []int64                       `json:"superseded_storyboard_revisions,omitempty"`
	InvalidatedResources           []AINativeInvalidatedResource `json:"invalidated_resources"`
}

type ReopenAINativeRequirementRequest struct {
	ExpectedWorkspaceVersion int64 `json:"expected_workspace_version"`
	InvalidateDownstream     bool  `json:"invalidate_downstream"`
}

type UpdateAINativeRequirementRequest struct {
	ExpectedRevision        int64                      `json:"expected_revision"`
	ProductName             string                     `json:"product_name"`
	ProductDescription      string                     `json:"product_description"`
	TargetAudiences         []AINativeEditableText     `json:"target_audiences"`
	Media                   []AINativeRequirementMedia `json:"media"`
	CoreSellingPoints       []AINativeEditableText     `json:"core_selling_points"`
	SupplementalRequirement string                     `json:"supplemental_requirement"`
	AspectRatio             string                     `json:"aspect_ratio"`
	DurationSeconds         int                        `json:"duration_seconds"`
	Language                string                     `json:"language"`
}

type ConfirmAINativeRequirementRequest struct {
	ExpectedRevision int64 `json:"expected_revision"`
}

func (s Service) GetAINativeRequirementWorkspace(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, workspaceID string) (AINativeRequirementWorkspace, error) {
	if s.Projects == nil || s.AINativeRequirements == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("AI native requirement persistence is unavailable")
	}
	if !actor.HasScope(ScopeRead) {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%s scope is required", ScopeRead)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return s.AINativeRequirements.GetAINativeRequirementWorkspace(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID))
}

func (s Service) UpdateAINativeRequirement(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, workspaceID string, request UpdateAINativeRequirementRequest) (AINativeRequirementWorkspace, error) {
	if s.Projects == nil || s.AINativeRequirements == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("AI native requirement persistence is unavailable")
	}
	if !actor.HasScope(ScopeWrite) {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if request.ExpectedRevision < 1 {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%w: expected_revision must be positive", ErrInvalidAINativeRequirement)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	current, err := s.AINativeRequirements.GetAINativeRequirementWorkspace(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID))
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if current.Status != AINativeRequirementDraftStatus {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	for _, media := range request.Media {
		if media.AssetRef == nil || media.AssetRef.Validate() != nil {
			return AINativeRequirementWorkspace{}, fmt.Errorf("%w: requirement media must reference a Project Asset", ErrInvalidAINativeRequirement)
		}
	}
	next := current
	next.CurrentRevision = request.ExpectedRevision + 1
	next.WorkspaceVersion = current.WorkspaceVersion + 1
	next.Requirement.Revision = next.CurrentRevision
	next.Requirement.ProductName = strings.TrimSpace(request.ProductName)
	next.Requirement.ProductDescription = strings.TrimSpace(request.ProductDescription)
	next.Requirement.TargetAudiences = request.TargetAudiences
	next.Requirement.Media = request.Media
	next.Requirement.CoreSellingPoints = request.CoreSellingPoints
	next.Requirement.SupplementalRequirement = strings.TrimSpace(request.SupplementalRequirement)
	next.Requirement.AspectRatio = strings.TrimSpace(request.AspectRatio)
	next.Requirement.DurationSeconds = request.DurationSeconds
	next.Requirement.Language = strings.TrimSpace(request.Language)
	next.UpdatedAt = s.now()
	if err := next.Validate(); err != nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%w: %v", ErrInvalidAINativeRequirement, err)
	}
	return s.AINativeRequirements.AppendAINativeRequirementRevision(ctx, next, request.ExpectedRevision, actor.Principal.ID)
}

func (s Service) ConfirmAINativeRequirement(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, workspaceID string, request ConfirmAINativeRequirementRequest) (AINativeRequirementWorkspace, error) {
	if s.Projects == nil || s.AINativeRequirements == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("AI native requirement persistence is unavailable")
	}
	if !actor.HasScope(ScopeWrite) {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if request.ExpectedRevision < 1 {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%w: expected_revision must be positive", ErrInvalidAINativeRequirement)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return s.AINativeRequirements.ConfirmAINativeRequirement(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID), request.ExpectedRevision, actor.Principal.ID, s.now())
}

func (s Service) GetAINativeReopenImpact(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, workspaceID, stage string) (AINativeReopenImpact, error) {
	if s.Projects == nil {
		return AINativeReopenImpact{}, fmt.Errorf("AI native workspace persistence is unavailable")
	}
	if !actor.HasScope(ScopeRead) {
		return AINativeReopenImpact{}, fmt.Errorf("%s scope is required", ScopeRead)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return AINativeReopenImpact{}, err
	}
	if strings.TrimSpace(stage) == AINativeStageScript {
		if s.AINativeScripts == nil {
			return AINativeReopenImpact{}, fmt.Errorf("AI native script persistence is unavailable")
		}
		return s.AINativeScripts.GetAINativeScriptReopenImpact(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID))
	}
	if strings.TrimSpace(stage) == AINativeStageStoryboard {
		if s.AINativeStoryboards == nil {
			return AINativeReopenImpact{}, fmt.Errorf("AI native storyboard persistence is unavailable")
		}
		return s.AINativeStoryboards.GetAINativeStoryboardReopenImpact(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID))
	}
	if strings.TrimSpace(stage) != AINativeStageRequirement {
		return AINativeReopenImpact{}, fmt.Errorf("%w: stage cannot be reopened", ErrInvalidAINativeRequirement)
	}
	if s.AINativeRequirements == nil {
		return AINativeReopenImpact{}, fmt.Errorf("AI native requirement persistence is unavailable")
	}
	return s.AINativeRequirements.GetAINativeReopenImpact(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID), stage)
}

func (s Service) ReopenAINativeRequirement(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, workspaceID string, request ReopenAINativeRequirementRequest) (AINativeRequirementWorkspace, error) {
	if s.Projects == nil || s.AINativeRequirements == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("AI native requirement persistence is unavailable")
	}
	if !actor.HasScope(ScopeWrite) {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if request.ExpectedWorkspaceVersion < 1 {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%w: expected_workspace_version must be positive", ErrInvalidAINativeRequirement)
	}
	if !request.InvalidateDownstream {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%w: invalidate_downstream must be explicitly confirmed", ErrInvalidAINativeRequirement)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	impact, err := s.AINativeRequirements.GetAINativeReopenImpact(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID), AINativeStageRequirement)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	workspace, err := s.AINativeRequirements.ReopenAINativeRequirement(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID), request.ExpectedWorkspaceVersion, actor.Principal.ID, s.now())
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if s.AINativeOperationCanceller != nil {
		for _, resource := range impact.InvalidatedResources {
			if resource.Type == "operation" && resource.Version > 0 {
				_ = s.AINativeOperationCanceller.CancelAINativeOperation(ctx, actor.OrganizationID, projectID, resource.ID, resource.Version)
			}
		}
	}
	return workspace, nil
}
