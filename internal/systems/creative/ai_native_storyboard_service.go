package creative

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
)

const AINativeStoryboardJobKind = "creative.ai_native.storyboard.generate"

type AINativeStoryboardOperation struct {
	ID                       string                `json:"id"`
	Version                  int64                 `json:"version"`
	WorkspaceID              string                `json:"workspace_id"`
	ProjectID                contract.ProjectID    `json:"project_id"`
	ExpectedWorkspaceVersion int64                 `json:"expected_workspace_version"`
	RequirementRevision      int64                 `json:"requirement_revision"`
	RequirementHash          string                `json:"requirement_hash"`
	ScriptRevision           int64                 `json:"script_revision"`
	ScriptHash               string                `json:"script_hash"`
	Actor                    contract.ActorContext `json:"actor"`
	ActorID                  string                `json:"actor_id"`
	CreatedAt                time.Time             `json:"created_at"`
}

type AINativeStoryboardJobPayload struct {
	Operation AINativeStoryboardOperation `json:"operation"`
}

type AINativeStoryboardRepository interface {
	GetAINativeStoryboardWorkspace(context.Context, contract.OrganizationID, contract.ProjectID, string) (AINativeRequirementWorkspace, error)
	BeginAINativeStoryboardGeneration(context.Context, contract.OrganizationID, contract.ProjectID, string, AINativeStoryboardOperation, time.Time) (AINativeRequirementWorkspace, error)
	SaveAINativeStoryboardPlan(context.Context, contract.OrganizationID, contract.ProjectID, string, AINativeStoryboardOperation, AINativeStoryboardRevision, time.Time) (AINativeRequirementWorkspace, error)
	CompleteAINativeStoryboardGeneration(context.Context, contract.OrganizationID, contract.ProjectID, string, AINativeStoryboardOperation, AINativeStoryboardRevision, string, time.Time) (AINativeRequirementWorkspace, error)
	FailAINativeStoryboardGeneration(context.Context, contract.OrganizationID, contract.ProjectID, string, string, int64, string, string, time.Time) error
	AppendAINativeStoryboardRevision(context.Context, AINativeRequirementWorkspace, int64, string, time.Time) (AINativeRequirementWorkspace, error)
	ConfirmAINativeStoryboard(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, int64, string, time.Time) (AINativeRequirementWorkspace, error)
	GetAINativeStoryboardReopenImpact(context.Context, contract.OrganizationID, contract.ProjectID, string) (AINativeReopenImpact, error)
	ReopenAINativeStoryboard(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, string, time.Time) (AINativeRequirementWorkspace, error)
}

type AINativeStoryboardScheduler interface {
	ScheduleAINativeStoryboard(context.Context, AINativeStoryboardOperation) error
}

type AINativeStoryboardAssetPreparer interface {
	PrepareAINativeStoryboardAsset(context.Context, contract.ActorContext, contract.ProjectContext, AINativeStoryboardOperation, AINativeStoryboardAsset) (*contract.AssetVersionRef, *time.Time, error)
}

func (s Service) GenerateAINativeStoryboard(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, workspaceID string, request GenerateAINativeStoryboardRequest) (AINativeRequirementWorkspace, error) {
	if s.Projects == nil || s.AINativeStoryboards == nil || s.AINativeStoryboardPlanner == nil || s.AINativeStoryboardScheduler == nil || s.AINativeStoryboardAssetPreparer == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("AI native storyboard dependencies are incomplete")
	}
	if !actor.HasScope(ScopeWrite) {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if request.ExpectedWorkspaceVersion < 1 {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%w: expected_workspace_version must be positive", ErrInvalidAINativeRequirement)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	workspace, err := s.AINativeStoryboards.GetAINativeStoryboardWorkspace(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID))
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if workspace.ScriptStatus != AINativeScriptConfirmedStatus || workspace.Script == nil || workspace.ConfirmedScriptRevision == nil || workspace.WorkspaceVersion != request.ExpectedWorkspaceVersion ||
		workspace.StoryboardStatus == AINativeStoryboardGeneratingStatus || workspace.StoryboardStatus == AINativeStoryboardConfirmedStatus {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	requirementHash, err := contract.CanonicalJSONHash(workspace.Requirement)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	scriptHash, err := contract.CanonicalJSONHash(workspace.Script)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	id, err := s.idGenerator()("ainativestoryboardop")
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	operation := AINativeStoryboardOperation{ID: id, Version: 1, WorkspaceID: workspace.WorkspaceID, ProjectID: projectID, ExpectedWorkspaceVersion: request.ExpectedWorkspaceVersion,
		RequirementRevision: workspace.CurrentRevision, RequirementHash: requirementHash, ScriptRevision: *workspace.ConfirmedScriptRevision, ScriptHash: scriptHash,
		Actor: actor, ActorID: actor.Principal.ID, CreatedAt: s.now()}
	updated, err := s.AINativeStoryboards.BeginAINativeStoryboardGeneration(ctx, actor.OrganizationID, projectID, workspace.WorkspaceID, operation, s.now())
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if err := s.AINativeStoryboardScheduler.ScheduleAINativeStoryboard(ctx, operation); err != nil {
		_ = s.AINativeStoryboards.FailAINativeStoryboardGeneration(ctx, actor.OrganizationID, projectID, workspace.WorkspaceID, operation.ID, operation.Version, "STORYBOARD_JOB_SCHEDULE_FAILED", boundedError(err), s.now())
		return AINativeRequirementWorkspace{}, err
	}
	return updated, nil
}

func (s Service) RegenerateAINativeStoryboardAsset(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, workspaceID, assetID string, request RegenerateAINativeStoryboardAssetRequest) (AINativeRequirementWorkspace, error) {
	if s.Projects == nil || s.AINativeStoryboards == nil || s.AINativeStoryboardScheduler == nil || s.AINativeStoryboardAssetPreparer == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("AI native storyboard dependencies are incomplete")
	}
	if !actor.HasScope(ScopeWrite) {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	feedback := strings.TrimSpace(request.Feedback)
	if request.ExpectedWorkspaceVersion < 1 || strings.TrimSpace(assetID) == "" || utf8.RuneCountInString(feedback) > 500 {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%w: asset regeneration request is invalid", ErrInvalidAINativeRequirement)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	workspace, err := s.AINativeStoryboards.GetAINativeStoryboardWorkspace(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID))
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if workspace.WorkspaceVersion != request.ExpectedWorkspaceVersion || workspace.StoryboardStatus != AINativeStoryboardDraftStatus || workspace.Storyboard == nil ||
		workspace.ScriptStatus != AINativeScriptConfirmedStatus || workspace.Script == nil || workspace.ConfirmedScriptRevision == nil || strings.TrimSpace(workspace.ActiveOperationID) != "" {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	plan := *workspace.Storyboard
	plan.Assets = append([]AINativeStoryboardAsset{}, workspace.Storyboard.Assets...)
	found := false
	for index := range plan.Assets {
		asset := &plan.Assets[index]
		if asset.ID != strings.TrimSpace(assetID) {
			continue
		}
		if asset.Source != AINativeStoryboardAssetSourceAIGenerated || strings.TrimSpace(asset.GenerationBrief) == "" {
			return AINativeRequirementWorkspace{}, fmt.Errorf("%w: only AI-generated storyboard assets can be regenerated", ErrInvalidAINativeRequirement)
		}
		asset.AssetRef = nil
		asset.RegenerationFeedback = feedback
		asset.Status = AINativeStoryboardAssetPlanned
		asset.GenerationAttempt++
		if asset.GenerationAttempt < 1 {
			asset.GenerationAttempt = 1
		}
		asset.ErrorCode, asset.ErrorMessage = "", ""
		found = true
		break
	}
	if !found {
		return AINativeRequirementWorkspace{}, ErrNotFound
	}
	id, err := s.idGenerator()("ainativestoryboardop")
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	operation := AINativeStoryboardOperation{ID: id, Version: 1, WorkspaceID: workspace.WorkspaceID, ProjectID: projectID, ExpectedWorkspaceVersion: request.ExpectedWorkspaceVersion,
		RequirementRevision: workspace.CurrentRevision, RequirementHash: plan.BasedOnRequirementHash, ScriptRevision: *workspace.ConfirmedScriptRevision, ScriptHash: plan.BasedOnScriptHash,
		Actor: actor, ActorID: actor.Principal.ID, CreatedAt: s.now()}
	if _, err = s.AINativeStoryboards.BeginAINativeStoryboardGeneration(ctx, actor.OrganizationID, projectID, workspace.WorkspaceID, operation, s.now()); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	updated, err := s.AINativeStoryboards.SaveAINativeStoryboardPlan(ctx, actor.OrganizationID, projectID, workspace.WorkspaceID, operation, plan, s.now())
	if err != nil {
		_ = s.AINativeStoryboards.FailAINativeStoryboardGeneration(context.WithoutCancel(ctx), actor.OrganizationID, projectID, workspace.WorkspaceID, operation.ID, operation.Version, "STORYBOARD_ASSET_REGENERATION_START_FAILED", boundedError(err), s.now())
		return AINativeRequirementWorkspace{}, err
	}
	if err := s.AINativeStoryboardScheduler.ScheduleAINativeStoryboard(ctx, operation); err != nil {
		_ = s.AINativeStoryboards.FailAINativeStoryboardGeneration(context.WithoutCancel(ctx), actor.OrganizationID, projectID, workspace.WorkspaceID, operation.ID, operation.Version, "STORYBOARD_ASSET_REGENERATION_SCHEDULE_FAILED", boundedError(err), s.now())
		return AINativeRequirementWorkspace{}, err
	}
	return updated, nil
}

func (s Service) UpdateAINativeStoryboard(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, workspaceID string, request UpdateAINativeStoryboardRequest) (AINativeRequirementWorkspace, error) {
	if s.Projects == nil || s.AINativeStoryboards == nil || request.ExpectedRevision < 1 {
		return AINativeRequirementWorkspace{}, fmt.Errorf("AI native storyboard persistence is unavailable")
	}
	if !actor.HasScope(ScopeWrite) {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	workspace, err := s.AINativeStoryboards.GetAINativeStoryboardWorkspace(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID))
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if workspace.StoryboardStatus != AINativeStoryboardDraftStatus || workspace.Storyboard == nil || workspace.Script == nil || workspace.CurrentStoryboardRevision == nil || *workspace.CurrentStoryboardRevision != request.ExpectedRevision {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	next := request.Storyboard
	next.Revision = request.ExpectedRevision + 1
	next.Status = AINativeStoryboardDraftStatus
	next.ContractVersion = aiNativeStoryboardContract
	next.BasedOnRequirementRevision, next.BasedOnRequirementHash = workspace.Storyboard.BasedOnRequirementRevision, workspace.Storyboard.BasedOnRequirementHash
	next.BasedOnScriptRevision, next.BasedOnScriptHash = workspace.Storyboard.BasedOnScriptRevision, workspace.Storyboard.BasedOnScriptHash
	next.ChannelProfileID, next.ChannelProfileHash, next.Generation = workspace.Storyboard.ChannelProfileID, workspace.Storyboard.ChannelProfileHash, workspace.Storyboard.Generation
	next.DurationSeconds = workspace.Requirement.DurationSeconds
	if err := next.ValidateReadyAgainst(workspace.Requirement, *workspace.Script); err != nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%w: %v", ErrInvalidAINativeRequirement, err)
	}
	workspace.Storyboard = &next
	return s.AINativeStoryboards.AppendAINativeStoryboardRevision(ctx, workspace, request.ExpectedRevision, actor.Principal.ID, s.now())
}

func (s Service) ConfirmAINativeStoryboard(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, workspaceID string, request ConfirmAINativeStoryboardRequest) (AINativeRequirementWorkspace, error) {
	if s.Projects == nil || s.AINativeStoryboards == nil || request.ExpectedRevision < 1 || request.ExpectedWorkspaceVersion < 1 {
		return AINativeRequirementWorkspace{}, fmt.Errorf("AI native storyboard persistence is unavailable")
	}
	if !actor.HasScope(ScopeWrite) {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	workspace, err := s.AINativeStoryboards.GetAINativeStoryboardWorkspace(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID))
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if workspace.Storyboard == nil || workspace.Script == nil || workspace.Storyboard.ValidateReadyAgainst(workspace.Requirement, *workspace.Script) != nil {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	return s.AINativeStoryboards.ConfirmAINativeStoryboard(ctx, actor.OrganizationID, projectID, workspace.WorkspaceID, request.ExpectedRevision, request.ExpectedWorkspaceVersion, actor.Principal.ID, s.now())
}

func (s Service) ReopenAINativeStoryboard(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, workspaceID string, request ReopenAINativeRequirementRequest) (AINativeRequirementWorkspace, error) {
	if s.Projects == nil || s.AINativeStoryboards == nil || request.ExpectedWorkspaceVersion < 1 || !request.InvalidateDownstream {
		return AINativeRequirementWorkspace{}, fmt.Errorf("AI native storyboard persistence is unavailable")
	}
	if !actor.HasScope(ScopeWrite) {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	impact, err := s.AINativeStoryboards.GetAINativeStoryboardReopenImpact(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID))
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	workspace, err := s.AINativeStoryboards.ReopenAINativeStoryboard(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID), request.ExpectedWorkspaceVersion, actor.Principal.ID, s.now())
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

func (s Service) HandleAINativeStoryboardJob(ctx context.Context, claim jobruntime.Claim) (jobruntime.Result, error) {
	if claim.Job.Kind != AINativeStoryboardJobKind || s.Projects == nil || s.AINativeStoryboards == nil || s.AINativeStoryboardPlanner == nil || s.AINativeStoryboardAssetPreparer == nil {
		return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "AI_NATIVE_STORYBOARD_HANDLER_UNAVAILABLE", Message: "AI native storyboard handler is unavailable", Retryable: false}}
	}
	var payload AINativeStoryboardJobPayload
	if err := json.Unmarshal(claim.Payload, &payload); err != nil || strings.TrimSpace(payload.Operation.ID) == "" {
		return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "AI_NATIVE_STORYBOARD_PAYLOAD_INVALID", Message: "AI native storyboard payload is invalid", Retryable: false}}
	}
	op := payload.Operation
	workspace, err := s.AINativeStoryboards.GetAINativeStoryboardWorkspace(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, op.WorkspaceID)
	if err != nil {
		return jobruntime.Result{}, err
	}
	if workspace.ActiveOperationID != op.ID || workspace.ActiveOperationVersion == nil || *workspace.ActiveOperationVersion != op.Version {
		return jobruntime.Result{}, nil
	}
	project, err := s.Projects.RequireActiveContext(ctx, op.Actor, claim.Job.ProjectID)
	if err != nil {
		return s.failAINativeStoryboardJob(ctx, claim, op, "AI_NATIVE_STORYBOARD_PROJECT_UNAVAILABLE", err)
	}
	if workspace.Script == nil || workspace.Script.Revision != op.ScriptRevision {
		return s.failAINativeStoryboardJob(ctx, claim, op, "AI_NATIVE_STORYBOARD_SCRIPT_MISMATCH", ErrInvalidState)
	}
	plan := workspace.StoryboardPlan
	if plan == nil {
		profile, resolveErr := s.resolveFrozenChannelProfile(workspace.Requirement)
		if resolveErr != nil {
			return s.failAINativeStoryboardJob(ctx, claim, op, "AI_NATIVE_STORYBOARD_PROFILE_UNAVAILABLE", resolveErr)
		}
		generated, planErr := s.AINativeStoryboardPlanner.Plan(ctx, op.Actor, project, workspace.Requirement, *workspace.Script, profile)
		if planErr != nil {
			return s.failAINativeStoryboardJob(ctx, claim, op, "AI_NATIVE_STORYBOARD_GENERATION_FAILED", planErr)
		}
		generated.BasedOnRequirementRevision, generated.BasedOnRequirementHash = op.RequirementRevision, op.RequirementHash
		generated.BasedOnScriptRevision, generated.BasedOnScriptHash = op.ScriptRevision, op.ScriptHash
		if generated.ValidatePlanAgainst(workspace.Requirement, *workspace.Script) != nil {
			return s.failAINativeStoryboardJob(ctx, claim, op, "AI_NATIVE_STORYBOARD_VALIDATION_FAILED", generated.ValidatePlanAgainst(workspace.Requirement, *workspace.Script))
		}
		if generated.Generation.PromptVersion != profile.StoryboardPromptVersion() || generated.Generation.ProfileHash != profile.ContentHash {
			return s.failAINativeStoryboardJob(ctx, claim, op, "AI_NATIVE_STORYBOARD_PROFILE_MISMATCH", fmt.Errorf("storyboard output does not match the frozen channel profile"))
		}
		persisted, saveErr := s.AINativeStoryboards.SaveAINativeStoryboardPlan(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, op.WorkspaceID, op, generated, s.now())
		if saveErr != nil {
			return s.failAINativeStoryboardJob(ctx, claim, op, "AI_NATIVE_STORYBOARD_PERSISTENCE_FAILED", saveErr)
		}
		plan = persisted.StoryboardPlan
	}
	ready := *plan
	ready.Assets = append([]AINativeStoryboardAsset{}, plan.Assets...)
	var deferUntil *time.Time
	var assetFailure error
	for index := range ready.Assets {
		asset := ready.Assets[index]
		if asset.Status == AINativeStoryboardAssetReady {
			continue
		}
		if asset.Status == AINativeStoryboardAssetFailed {
			ready.Assets[index].GenerationAttempt++
		} else if ready.Assets[index].GenerationAttempt < 1 {
			ready.Assets[index].GenerationAttempt = 1
		}
		if asset.Status != AINativeStoryboardAssetGenerating {
			ready.Assets[index].Status = AINativeStoryboardAssetGenerating
			ready.Assets[index].ErrorCode = ""
			ready.Assets[index].ErrorMessage = ""
			if _, saveErr := s.AINativeStoryboards.SaveAINativeStoryboardPlan(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, op.WorkspaceID, op, ready, s.now()); saveErr != nil {
				return s.failAINativeStoryboardJob(ctx, claim, op, "AI_NATIVE_STORYBOARD_PERSISTENCE_FAILED", saveErr)
			}
		}
		asset = ready.Assets[index]
		ref, pending, prepareErr := s.AINativeStoryboardAssetPreparer.PrepareAINativeStoryboardAsset(ctx, op.Actor, project, op, asset)
		if prepareErr != nil {
			ready.Assets[index].Status = AINativeStoryboardAssetFailed
			ready.Assets[index].ErrorCode = "AI_NATIVE_STORYBOARD_ASSET_FAILED"
			ready.Assets[index].ErrorMessage = boundedError(prepareErr)
			if _, saveErr := s.AINativeStoryboards.SaveAINativeStoryboardPlan(context.WithoutCancel(ctx), claim.Job.OrganizationID, claim.Job.ProjectID, op.WorkspaceID, op, ready, s.now()); saveErr != nil {
				return s.failAINativeStoryboardJob(ctx, claim, op, "AI_NATIVE_STORYBOARD_PERSISTENCE_FAILED", saveErr)
			}
			if assetFailure == nil {
				assetFailure = prepareErr
			}
			continue
		}
		if ref == nil {
			if pending != nil && (deferUntil == nil || pending.Before(*deferUntil)) {
				deferUntil = pending
			}
			continue
		}
		ready.Assets[index].AssetRef = ref
		ready.Assets[index].Status = AINativeStoryboardAssetReady
		ready.Assets[index].ErrorCode = ""
		ready.Assets[index].ErrorMessage = ""
		if _, saveErr := s.AINativeStoryboards.SaveAINativeStoryboardPlan(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, op.WorkspaceID, op, ready, s.now()); saveErr != nil {
			return s.failAINativeStoryboardJob(ctx, claim, op, "AI_NATIVE_STORYBOARD_PERSISTENCE_FAILED", saveErr)
		}
	}
	if assetFailure != nil {
		return s.failAINativeStoryboardJob(ctx, claim, op, "AI_NATIVE_STORYBOARD_ASSET_FAILED", assetFailure)
	}
	if deferUntil != nil {
		return jobruntime.Result{}, jobruntime.DeferredError{AvailableAt: *deferUntil}
	}
	if err := ready.ValidateReadyAgainst(workspace.Requirement, *workspace.Script); err != nil {
		return s.failAINativeStoryboardJob(ctx, claim, op, "AI_NATIVE_STORYBOARD_ASSET_VALIDATION_FAILED", err)
	}
	completed, err := s.AINativeStoryboards.CompleteAINativeStoryboardGeneration(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, op.WorkspaceID, op, ready, op.ActorID, s.now())
	if err != nil {
		return s.failAINativeStoryboardJob(ctx, claim, op, "AI_NATIVE_STORYBOARD_PERSISTENCE_FAILED", err)
	}
	ref := contract.ResourceRef{Type: "creative_ai_native_storyboard", ID: completed.WorkspaceID, Version: completed.CurrentStoryboardRevision}
	return jobruntime.Result{Ref: &ref}, nil
}

func (s Service) failAINativeStoryboardJob(ctx context.Context, claim jobruntime.Claim, op AINativeStoryboardOperation, code string, cause error) (jobruntime.Result, error) {
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = s.AINativeStoryboards.FailAINativeStoryboardGeneration(persistCtx, claim.Job.OrganizationID, claim.Job.ProjectID, op.WorkspaceID, op.ID, op.Version, code, boundedError(cause), s.now())
	return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: code, Message: "AI native storyboard generation failed", Retryable: false}}
}

type JobRuntimeAINativeStoryboardScheduler struct {
	Store jobruntime.Store
	Now   func() time.Time
}

func (s JobRuntimeAINativeStoryboardScheduler) ScheduleAINativeStoryboard(ctx context.Context, operation AINativeStoryboardOperation) error {
	if s.Store == nil {
		return fmt.Errorf("AI native storyboard job store is required")
	}
	payload, err := json.Marshal(AINativeStoryboardJobPayload{Operation: operation})
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	hash := sha256.Sum256(payload)
	_, _, err = s.Store.Enqueue(ctx, jobruntime.CreateRequest{Job: contract.Job{ID: operation.ID, Kind: AINativeStoryboardJobKind, OrganizationID: operation.Actor.OrganizationID,
		ProjectID: operation.ProjectID, Status: contract.JobQueued, Cancellable: true, MaxAttempts: 120, Version: 1, CreatedAt: now, UpdatedAt: now}, Payload: payload,
		IdempotencyKey: contract.IdempotencyKey("ai-native-storyboard-" + operation.ID), RequestHash: hex.EncodeToString(hash[:])})
	return err
}
