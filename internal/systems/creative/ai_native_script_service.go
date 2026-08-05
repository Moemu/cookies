package creative

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
)

const (
	AINativeScriptGeneratingStatus = "generating"
	AINativeScriptFailedStatus     = "failed"
	AINativeScriptJobKind          = "creative.ai_native.script.generate"
)

type AINativeScriptOperation struct {
	ID                       string                `json:"id"`
	Version                  int64                 `json:"version"`
	WorkspaceID              string                `json:"workspace_id"`
	ProjectID                contract.ProjectID    `json:"project_id"`
	ExpectedWorkspaceVersion int64                 `json:"expected_workspace_version"`
	RequirementRevision      int64                 `json:"requirement_revision"`
	RequirementHash          string                `json:"requirement_hash"`
	BasedOnScriptRevision    *int64                `json:"based_on_script_revision,omitempty"`
	RegenerationNote         string                `json:"regeneration_note,omitempty"`
	Actor                    contract.ActorContext `json:"actor"`
	ActorID                  string                `json:"actor_id"`
	CreatedAt                time.Time             `json:"created_at"`
}

type AINativeScriptJobPayload struct {
	Operation AINativeScriptOperation `json:"operation"`
}

type GenerateAINativeScriptRequest struct {
	ExpectedWorkspaceVersion int64  `json:"expected_workspace_version"`
	RegenerationNote         string `json:"regeneration_note,omitempty"`
}

type UpdateAINativeScriptRequest struct {
	ExpectedRevision int64                  `json:"expected_revision"`
	Script           AINativeScriptRevision `json:"script"`
}

type ConfirmAINativeScriptRequest struct {
	ExpectedRevision         int64 `json:"expected_revision"`
	ExpectedWorkspaceVersion int64 `json:"expected_workspace_version"`
}

type AINativeScriptRepository interface {
	GetAINativeScriptWorkspace(context.Context, contract.OrganizationID, contract.ProjectID, string) (AINativeRequirementWorkspace, error)
	BeginAINativeScriptGeneration(context.Context, contract.OrganizationID, contract.ProjectID, string, AINativeScriptOperation, time.Time) (AINativeRequirementWorkspace, error)
	CompleteAINativeScriptGeneration(context.Context, contract.OrganizationID, contract.ProjectID, string, AINativeScriptOperation, AINativeScriptRevision, string, time.Time) (AINativeRequirementWorkspace, error)
	FailAINativeScriptGeneration(context.Context, contract.OrganizationID, contract.ProjectID, string, string, int64, string, string, time.Time) error
	AppendAINativeScriptRevision(context.Context, AINativeRequirementWorkspace, int64, string, time.Time) (AINativeRequirementWorkspace, error)
	ConfirmAINativeScript(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, int64, string, time.Time) (AINativeRequirementWorkspace, error)
	GetAINativeScriptReopenImpact(context.Context, contract.OrganizationID, contract.ProjectID, string) (AINativeReopenImpact, error)
	ReopenAINativeScript(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, string, time.Time) (AINativeRequirementWorkspace, error)
}

type AINativeScriptScheduler interface {
	ScheduleAINativeScript(context.Context, AINativeScriptOperation) error
}

func (s Service) GenerateAINativeScript(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, workspaceID string, request GenerateAINativeScriptRequest) (AINativeRequirementWorkspace, error) {
	if s.Projects == nil || s.AINativeScripts == nil || s.AINativeScriptScheduler == nil || s.AINativeScriptPlanner == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("AI native script dependencies are incomplete")
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
	workspace, err := s.AINativeScripts.GetAINativeScriptWorkspace(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID))
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if workspace.Status != AINativeRequirementConfirmedStatus || workspace.ConfirmedRevision == nil {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	if workspace.ScriptStatus == AINativeScriptGeneratingStatus || workspace.ScriptStatus == AINativeScriptConfirmedStatus {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	if workspace.WorkspaceVersion != request.ExpectedWorkspaceVersion {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	if _, err := s.AINativeScriptProfiles.Resolve(workspace.Requirement.Channel, "performance", "v1"); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	requirementHash, err := contract.CanonicalJSONHash(workspace.Requirement)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	operationID, err := s.idGenerator()("ainativescriptoperation")
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	operation := AINativeScriptOperation{ID: operationID, Version: 1, WorkspaceID: workspace.WorkspaceID, ProjectID: projectID,
		ExpectedWorkspaceVersion: request.ExpectedWorkspaceVersion, RequirementRevision: workspace.CurrentRevision,
		RequirementHash: requirementHash, BasedOnScriptRevision: workspace.CurrentScriptRevision,
		RegenerationNote: strings.TrimSpace(request.RegenerationNote), Actor: actor, ActorID: actor.Principal.ID, CreatedAt: s.now()}
	updated, err := s.AINativeScripts.BeginAINativeScriptGeneration(ctx, actor.OrganizationID, projectID, workspace.WorkspaceID, operation, s.now())
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if err := s.AINativeScriptScheduler.ScheduleAINativeScript(ctx, operation); err != nil {
		_ = s.AINativeScripts.FailAINativeScriptGeneration(ctx, actor.OrganizationID, projectID, workspace.WorkspaceID, operation.ID, operation.Version, "SCRIPT_JOB_SCHEDULE_FAILED", boundedError(err), s.now())
		return AINativeRequirementWorkspace{}, err
	}
	return updated, nil
}

func (s Service) UpdateAINativeScript(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, workspaceID string, request UpdateAINativeScriptRequest) (AINativeRequirementWorkspace, error) {
	if s.Projects == nil || s.AINativeScripts == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("AI native script persistence is unavailable")
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
	workspace, err := s.AINativeScripts.GetAINativeScriptWorkspace(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID))
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if workspace.ScriptStatus != AINativeScriptDraftStatus || workspace.Script == nil || workspace.CurrentScriptRevision == nil || *workspace.CurrentScriptRevision != request.ExpectedRevision {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	nextScript := *workspace.Script
	nextScript.Title = strings.TrimSpace(request.Script.Title)
	nextScript.CreativeSummary = strings.TrimSpace(request.Script.CreativeSummary)
	nextScript.Segments = append([]AINativeScriptSegment{}, request.Script.Segments...)
	nextScript.RegenerationNote = strings.TrimSpace(request.Script.RegenerationNote)
	nextScript.Revision = request.ExpectedRevision + 1
	nextScript.Status = AINativeScriptDraftStatus
	base := request.ExpectedRevision
	nextScript.BasedOnRevision = &base
	nextScript.ConfirmedBy, nextScript.ConfirmedAt = "", nil
	if err := nextScript.ValidateAgainst(workspace.Requirement); err != nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%w: %v", ErrInvalidAINativeRequirement, err)
	}
	next := workspace
	next.Script = &nextScript
	return s.AINativeScripts.AppendAINativeScriptRevision(ctx, next, request.ExpectedRevision, actor.Principal.ID, s.now())
}

func (s Service) ConfirmAINativeScript(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, workspaceID string, request ConfirmAINativeScriptRequest) (AINativeRequirementWorkspace, error) {
	if s.Projects == nil || s.AINativeScripts == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("AI native script persistence is unavailable")
	}
	if !actor.HasScope(ScopeWrite) {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if request.ExpectedRevision < 1 || request.ExpectedWorkspaceVersion < 1 {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%w: expected revisions must be positive", ErrInvalidAINativeRequirement)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return s.AINativeScripts.ConfirmAINativeScript(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID), request.ExpectedRevision, request.ExpectedWorkspaceVersion, actor.Principal.ID, s.now())
}

func (s Service) ReopenAINativeScript(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, workspaceID string, request ReopenAINativeRequirementRequest) (AINativeRequirementWorkspace, error) {
	if s.Projects == nil || s.AINativeScripts == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("AI native script persistence is unavailable")
	}
	if !actor.HasScope(ScopeWrite) {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if request.ExpectedWorkspaceVersion < 1 || !request.InvalidateDownstream {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%w: downstream invalidation must be explicitly confirmed", ErrInvalidAINativeRequirement)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return s.AINativeScripts.ReopenAINativeScript(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID), request.ExpectedWorkspaceVersion, actor.Principal.ID, s.now())
}

func (s Service) HandleAINativeScriptJob(ctx context.Context, claim jobruntime.Claim) (jobruntime.Result, error) {
	if claim.Job.Kind != AINativeScriptJobKind || s.Projects == nil || s.AINativeScripts == nil || s.AINativeScriptPlanner == nil {
		return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "AI_NATIVE_SCRIPT_HANDLER_UNAVAILABLE", Message: "AI native script handler is unavailable", Retryable: false}}
	}
	var payload AINativeScriptJobPayload
	if err := json.Unmarshal(claim.Payload, &payload); err != nil || strings.TrimSpace(payload.Operation.ID) == "" || payload.Operation.Version < 1 {
		return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "AI_NATIVE_SCRIPT_PAYLOAD_INVALID", Message: "AI native script job payload is invalid", Retryable: false}}
	}
	operation := payload.Operation
	workspace, err := s.AINativeScripts.GetAINativeScriptWorkspace(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, operation.WorkspaceID)
	if err != nil {
		return jobruntime.Result{}, err
	}
	if workspace.ActiveOperationID != operation.ID || workspace.ActiveOperationVersion == nil || *workspace.ActiveOperationVersion != operation.Version {
		return jobruntime.Result{}, nil
	}
	project, err := s.Projects.RequireActiveContext(ctx, operation.Actor, claim.Job.ProjectID)
	if err != nil {
		return s.failAINativeScriptJob(ctx, claim, operation, "AI_NATIVE_SCRIPT_PROJECT_UNAVAILABLE", err)
	}
	profile, err := s.AINativeScriptProfiles.Resolve(workspace.Requirement.Channel, "performance", "v1")
	if err != nil {
		return s.failAINativeScriptJob(ctx, claim, operation, "AI_NATIVE_SCRIPT_PROFILE_UNAVAILABLE", err)
	}
	script, err := s.AINativeScriptPlanner.Plan(ctx, operation.Actor, project, workspace.Requirement, profile, operation.RegenerationNote)
	if err != nil {
		return s.failAINativeScriptJob(ctx, claim, operation, "AI_NATIVE_SCRIPT_GENERATION_FAILED", err)
	}
	if script.ChannelProfileID != profile.ID || script.ChannelProfileHash != profile.ContentHash || script.Generation.ProfileHash != profile.ContentHash {
		return s.failAINativeScriptJob(ctx, claim, operation, "AI_NATIVE_SCRIPT_PROFILE_MISMATCH", fmt.Errorf("script output does not match the selected channel profile"))
	}
	script.BasedOnRequirementRevision = operation.RequirementRevision
	script.BasedOnRequirementHash = operation.RequirementHash
	script.BasedOnRevision = operation.BasedOnScriptRevision
	if err := script.ValidateAgainst(workspace.Requirement); err != nil {
		return s.failAINativeScriptJob(ctx, claim, operation, "AI_NATIVE_SCRIPT_VALIDATION_FAILED", err)
	}
	completed, err := s.AINativeScripts.CompleteAINativeScriptGeneration(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, operation.WorkspaceID, operation, script, operation.ActorID, s.now())
	if err != nil {
		return jobruntime.Result{}, err
	}
	ref := contract.ResourceRef{Type: "creative_ai_native_script", ID: completed.WorkspaceID, Version: &completed.Script.Revision}
	return jobruntime.Result{Ref: &ref}, nil
}

func (s Service) failAINativeScriptJob(ctx context.Context, claim jobruntime.Claim, operation AINativeScriptOperation, code string, cause error) (jobruntime.Result, error) {
	_ = s.AINativeScripts.FailAINativeScriptGeneration(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, operation.WorkspaceID, operation.ID, operation.Version, code, boundedError(cause), s.now())
	return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: code, Message: "AI native script generation failed", Retryable: false}}
}

type JobRuntimeAINativeScriptScheduler struct {
	Store jobruntime.Store
	Now   func() time.Time
}

func (s JobRuntimeAINativeScriptScheduler) ScheduleAINativeScript(ctx context.Context, operation AINativeScriptOperation) error {
	if s.Store == nil {
		return fmt.Errorf("AI native script job store is required")
	}
	payload, err := json.Marshal(AINativeScriptJobPayload{Operation: operation})
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	hash := sha256.Sum256(payload)
	_, _, err = s.Store.Enqueue(ctx, jobruntime.CreateRequest{Job: contract.Job{ID: operation.ID, Kind: AINativeScriptJobKind,
		OrganizationID: operation.Actor.OrganizationID, ProjectID: operation.ProjectID, Status: contract.JobQueued,
		Cancellable: true, MaxAttempts: 1, Version: 1, CreatedAt: now, UpdatedAt: now}, Payload: payload,
		IdempotencyKey: contract.IdempotencyKey("ai-native-script-" + operation.ID), RequestHash: hex.EncodeToString(hash[:])})
	return err
}
