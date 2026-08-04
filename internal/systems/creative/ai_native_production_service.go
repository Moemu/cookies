package creative

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
	"github.com/shikanon/cookies/internal/platform/media"
	"github.com/shikanon/cookies/internal/platform/provider"
)

const AINativeProductionJobKind = "creative.ai_native.production.generate"

type AINativeProductionOperation struct {
	ID                       string                `json:"id"`
	Version                  int64                 `json:"version"`
	WorkspaceID              string                `json:"workspace_id"`
	ProjectID                contract.ProjectID    `json:"project_id"`
	ExpectedWorkspaceVersion int64                 `json:"expected_workspace_version"`
	StoryboardRevision       int64                 `json:"storyboard_revision"`
	Actor                    contract.ActorContext `json:"actor"`
	CreatedAt                time.Time             `json:"created_at"`
}

type AINativeProductionJobPayload struct {
	Operation AINativeProductionOperation `json:"operation"`
}

type StartAINativeProductionRequest struct {
	ExpectedWorkspaceVersion int64 `json:"expected_workspace_version"`
}

type RetryAINativeProductionUnitRequest struct {
	ExpectedWorkspaceVersion int64  `json:"expected_workspace_version"`
	UnitID                   string `json:"unit_id"`
}

type AINativeProductionRepository interface {
	GetAINativeProductionWorkspace(context.Context, contract.OrganizationID, contract.ProjectID, string) (AINativeRequirementWorkspace, error)
	BeginAINativeProduction(context.Context, contract.OrganizationID, contract.ProjectID, string, AINativeProductionOperation, AINativeProductionPlan, time.Time) (AINativeRequirementWorkspace, error)
	SaveAINativeProductionPlan(context.Context, contract.OrganizationID, contract.ProjectID, string, AINativeProductionOperation, AINativeProductionPlan, time.Time) (AINativeRequirementWorkspace, error)
	CancelAINativeProduction(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, time.Time) (AINativeRequirementWorkspace, error)
}

type AINativeProductionScheduler interface {
	ScheduleAINativeProduction(context.Context, AINativeProductionOperation) error
}

type AINativeVideoJobManager interface {
	CreateVideoJob(context.Context, provider.CreateVideoJobRequest) (contract.ProviderJob, bool, error)
	GetJob(context.Context, contract.OrganizationID, contract.ProjectID, string) (contract.ProviderJob, error)
}

func (s Service) StartAINativeProduction(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, workspaceID string, request StartAINativeProductionRequest) (AINativeRequirementWorkspace, error) {
	if s.Projects == nil || s.AINativeProductions == nil || s.AINativeProductionScheduler == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("AI native production dependencies are incomplete")
	}
	if !actor.HasScope(ScopeWrite) || request.ExpectedWorkspaceVersion < 1 {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	project, err := s.Projects.RequireActiveContext(ctx, actor, projectID)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	workspace, err := s.AINativeProductions.GetAINativeProductionWorkspace(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID))
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if workspace.WorkspaceVersion != request.ExpectedWorkspaceVersion || workspace.StoryboardStatus != AINativeStoryboardConfirmedStatus || workspace.Storyboard == nil || workspace.ConfirmedStoryboardRevision == nil || workspace.ActiveOperationID != "" {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	plan, err := CompileAINativeProductionPlan(workspace.Requirement, *workspace.Storyboard, project.ProjectID, s.now())
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	for index := range plan.Units {
		attemptID, idErr := s.idGenerator()("ainativeattempt")
		if idErr != nil {
			return AINativeRequirementWorkspace{}, idErr
		}
		plan.Units[index].Attempts = []AINativeGenerationAttempt{{ID: attemptID, Ordinal: 1, Status: AINativeAttemptPlannedStatus, CreatedAt: s.now(), UpdatedAt: s.now()}}
	}
	for index := range plan.SpeechUnits {
		attemptID, idErr := s.idGenerator()("ainativespeechattempt")
		if idErr != nil {
			return AINativeRequirementWorkspace{}, idErr
		}
		plan.SpeechUnits[index].Attempts = []AINativeGenerationAttempt{{ID: attemptID, Ordinal: 1, Status: AINativeAttemptPlannedStatus, CreatedAt: s.now(), UpdatedAt: s.now()}}
	}
	operationID, err := s.idGenerator()("ainativeproductionoperation")
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	operation := AINativeProductionOperation{ID: operationID, Version: 1, WorkspaceID: workspace.WorkspaceID, ProjectID: projectID, ExpectedWorkspaceVersion: request.ExpectedWorkspaceVersion,
		StoryboardRevision: *workspace.ConfirmedStoryboardRevision, Actor: actor, CreatedAt: s.now()}
	updated, err := s.AINativeProductions.BeginAINativeProduction(ctx, actor.OrganizationID, projectID, workspace.WorkspaceID, operation, plan, s.now())
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if err := s.AINativeProductionScheduler.ScheduleAINativeProduction(ctx, operation); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return withAINativeProductionProgress(updated, s.now()), nil
}

func withAINativeProductionProgress(workspace AINativeRequirementWorkspace, now time.Time) AINativeRequirementWorkspace {
	if workspace.ProductionPlan != nil {
		progress := workspace.ProductionPlan.Progress(now)
		workspace.ProductionProgress = &progress
	}
	return workspace
}

func (s Service) HandleAINativeProductionJob(ctx context.Context, claim jobruntime.Claim) (jobruntime.Result, error) {
	if claim.Job.Kind != AINativeProductionJobKind || s.Projects == nil || s.AINativeProductions == nil || s.AINativeVideoJobs == nil || s.AINativeSpeech == nil || s.AudioAssets == nil {
		return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "AI_NATIVE_PRODUCTION_HANDLER_UNAVAILABLE", Message: "AI native production handler is unavailable", Retryable: false}}
	}
	var payload AINativeProductionJobPayload
	if err := json.Unmarshal(claim.Payload, &payload); err != nil || strings.TrimSpace(payload.Operation.ID) == "" {
		return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "AI_NATIVE_PRODUCTION_PAYLOAD_INVALID", Message: "AI native production payload is invalid", Retryable: false}}
	}
	op := payload.Operation
	workspace, err := s.AINativeProductions.GetAINativeProductionWorkspace(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, op.WorkspaceID)
	if err != nil {
		return jobruntime.Result{}, err
	}
	if workspace.ProductionStatus == AINativeProductionCancelledStatus {
		return jobruntime.Result{}, nil
	}
	if workspace.ProductionPlan == nil || workspace.ActiveOperationID != op.ID || workspace.ActiveOperationVersion == nil || *workspace.ActiveOperationVersion != op.Version {
		return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "AI_NATIVE_PRODUCTION_OPERATION_STALE", Message: "AI native production operation is stale", Retryable: false}}
	}
	project, err := s.Projects.RequireActiveContext(ctx, op.Actor, op.ProjectID)
	if err != nil {
		return jobruntime.Result{}, err
	}
	plan := *workspace.ProductionPlan
	changed := false
	active := 0
	for index := range plan.Units {
		unit := &plan.Units[index]
		if len(unit.Attempts) == 0 || unit.SelectedAttemptID != "" {
			continue
		}
		attempt := &unit.Attempts[len(unit.Attempts)-1]
		switch attempt.Status {
		case AINativeAttemptPlannedStatus:
			limit := s.AINativeMaxActiveUnits
			if limit <= 0 {
				limit = 2
			}
			if active >= limit {
				continue
			}
			input, inputErr := unit.ProviderInput(project.ProjectID)
			if inputErr != nil {
				return jobruntime.Result{}, inputErr
			}
			job, _, createErr := s.AINativeVideoJobs.CreateVideoJob(ctx, provider.CreateVideoJobRequest{Actor: op.Actor, Project: project,
				IdempotencyKey: contract.IdempotencyKey("ai-native-video-" + attempt.ID), RequestHash: unit.PromptHash, ModelAlias: plan.VideoModelAlias,
				SourceSystem: "creative.ai-native-ad", SourceTaskID: op.WorkspaceID + ":" + unit.ID + ":" + attempt.ID, Input: input})
			if createErr != nil {
				return jobruntime.Result{}, createErr
			}
			attempt.ProviderJobID, attempt.Status, attempt.UpdatedAt = job.ID, AINativeAttemptSubmittedStatus, s.now()
			active++
			changed = true
		case AINativeAttemptSubmittedStatus, AINativeAttemptRunningStatus, AINativeAttemptIngestingStatus:
			active++
			job, getErr := s.AINativeVideoJobs.GetJob(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, attempt.ProviderJobID)
			if getErr != nil {
				return jobruntime.Result{}, getErr
			}
			changed = reconcileAINativeVideoAttempt(unit, attempt, job, s.now()) || changed
			if unit.SelectedAttemptID != "" || attempt.Status == AINativeAttemptFailedStatus {
				active--
			}
		}
	}
	for index := range plan.SpeechUnits {
		unit := &plan.SpeechUnits[index]
		if len(unit.Attempts) == 0 || unit.SelectedAttemptID != "" {
			continue
		}
		attempt := &unit.Attempts[len(unit.Attempts)-1]
		if attempt.Status != AINativeAttemptPlannedStatus {
			continue
		}
		result, synthErr := s.AINativeSpeech.Synthesize(ctx, provider.SpeechSynthesisInput{OrganizationID: claim.Job.OrganizationID, ModelAlias: plan.SpeechModelAlias, RequestID: attempt.ID, Text: unit.Text, VoiceAlias: unit.VoiceAlias, Language: unit.Language, Format: "mp3", SampleRate: 24000, SpeakingRate: 1, NeedTimestamps: true})
		if synthErr != nil {
			attempt.Status, attempt.ErrorCode, attempt.ErrorMessage, attempt.UpdatedAt = AINativeAttemptFailedStatus, speechErrorCode(synthErr), boundedError(synthErr), s.now()
			changed = true
			continue
		}
		if result.DurationMS > 0 && result.DurationMS > (unit.EndMS-unit.StartMS)*108/100 {
			attempt.Status, attempt.ErrorCode, attempt.ErrorMessage, attempt.UpdatedAt = AINativeAttemptFailedStatus, "SPEECH_DURATION_EXCEEDED", "旁白时长超过分镜容量 8%", s.now()
			changed = true
			continue
		}
		sourceVersion := plan.BasedOnStoryboardRevision
		requestContext := contract.RequestContext{RequestID: op.ID + "-" + unit.ID, TraceID: op.ID, Actor: op.Actor}
		ref, ingestErr := s.AudioAssets.IngestDerivedAudio(ctx, requestContext, op.ProjectID, "ai-native-speech-"+attempt.ID, bytes.NewReader(result.Audio), int64(len(result.Audio)), speechMIMEType(result.Codec), []contract.ResourceRef{{Type: "creative_ai_native_storyboard", ID: op.WorkspaceID, Version: &sourceVersion}})
		if ingestErr != nil {
			return jobruntime.Result{}, ingestErr
		}
		assetRef := ref.AssetVersion
		attempt.Status, attempt.ProviderJobID, attempt.OutputAssetRef, attempt.UpdatedAt = AINativeAttemptSucceededStatus, result.ProviderRequestID, &assetRef, s.now()
		unit.SelectedAttemptID = attempt.ID
		unit.NormalizedText, unit.AudioCodec, unit.SampleRate, unit.DurationMS = result.NormalizedText, result.Codec, result.SampleRate, result.DurationMS
		unit.WordTimings, unit.ProviderSnapshot = append([]provider.SpeechWordTiming{}, result.WordTimings...), result.ModelAndVoiceSnapshot
		changed = true
	}
	if productionAssetsReady(plan) {
		if s.AINativeTimelineRenderer != nil && s.RenderedAssets != nil && workspace.Storyboard != nil {
			plan.Status = AINativeProductionRenderingStatus
			if plan.Render == nil {
				plan.Render = &AINativeRenderState{ID: op.ID + "-final", Status: AINativeProductionRenderingStatus, ProgressPercent: 0, ETASeconds: max(15, plan.TotalDurationMS/250), RendererVersion: media.TimelineRendererVersion, StartedAt: s.now(), UpdatedAt: s.now()}
			}
		} else {
			plan.Status = AINativeProductionReadyStatus
		}
	} else if productionHasFailedAttempt(plan) && !productionHasActiveWork(plan) {
		plan.Status = AINativeProductionFailedStatus
	} else {
		plan.Status = AINativeProductionRunningStatus
	}
	plan.UpdatedAt = s.now()
	if changed || plan.Status == AINativeProductionReadyStatus || plan.Status == AINativeProductionRenderingStatus {
		workspace, err = s.AINativeProductions.SaveAINativeProductionPlan(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, op.WorkspaceID, op, plan, s.now())
		if err != nil {
			return jobruntime.Result{}, err
		}
	}
	if plan.Status == AINativeProductionRenderingStatus {
		return s.renderAINativeFinal(ctx, claim, op, workspace, plan)
	}
	if plan.Status != AINativeProductionReadyStatus {
		if plan.Status == AINativeProductionFailedStatus {
			return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "GENERATION_ATTEMPTS_EXHAUSTED", Message: "one or more production units failed", Retryable: false}}
		}
		return jobruntime.Result{}, jobruntime.DeferredError{AvailableAt: s.now().Add(2 * time.Second)}
	}
	revision := plan.Revision
	return jobruntime.Result{Ref: &contract.ResourceRef{Type: "creative_ai_native_production_plan", ID: workspace.WorkspaceID, Version: &revision}}, nil
}

func (s Service) renderAINativeFinal(ctx context.Context, claim jobruntime.Claim, op AINativeProductionOperation, workspace AINativeRequirementWorkspace, plan AINativeProductionPlan) (jobruntime.Result, error) {
	if workspace.Storyboard == nil || plan.Render == nil {
		return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "TIMELINE_INVALID", Message: "confirmed storyboard and render state are required", Retryable: false}}
	}
	timeline, err := CompileAINativeTimeline(plan, *workspace.Storyboard, claim.Job.OrganizationID, claim.Job.ProjectID)
	if err != nil {
		return s.failAINativeRender(ctx, claim, op, plan, "TIMELINE_INVALID", err)
	}
	lastSaved := -1
	output, err := s.AINativeTimelineRenderer.Render(ctx, timeline, func(progress media.TimelineProgress) error {
		if progress.Percent <= lastSaved {
			return nil
		}
		lastSaved = progress.Percent
		plan.Render.ProgressPercent = progress.Percent
		plan.Render.ETASeconds = max(1, (100-progress.Percent)*max(1, plan.TotalDurationMS/1000)/20)
		plan.Render.UpdatedAt = s.now()
		plan.UpdatedAt = s.now()
		_, saveErr := s.AINativeProductions.SaveAINativeProductionPlan(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, op.WorkspaceID, op, plan, s.now())
		return saveErr
	})
	if err != nil {
		return s.failAINativeRender(ctx, claim, op, plan, "TIMELINE_RENDER_FAILED", err)
	}
	defer output.Content.Close()
	requestContext := contract.RequestContext{RequestID: "ai-native-render-" + plan.Render.ID, TraceID: op.ID, Actor: op.Actor}
	ref, err := s.RenderedAssets.IngestRenderedVideo(ctx, requestContext, op.ProjectID, plan.Render.ID, output.Content, output.SizeBytes)
	if err != nil {
		return s.failAINativeRender(ctx, claim, op, plan, "FINAL_ASSET_INTAKE_FAILED", err)
	}
	assetRef := ref.AssetVersion
	plan.Render.Status, plan.Render.ProgressPercent, plan.Render.ETASeconds, plan.Render.OutputAssetRef, plan.Render.UpdatedAt = AINativeProductionCompletedStatus, 100, 0, &assetRef, s.now()
	plan.Status, plan.UpdatedAt = AINativeProductionCompletedStatus, s.now()
	workspace, err = s.AINativeProductions.SaveAINativeProductionPlan(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, op.WorkspaceID, op, plan, s.now())
	if err != nil {
		return jobruntime.Result{}, err
	}
	revision := plan.Revision
	return jobruntime.Result{Ref: &contract.ResourceRef{Type: "creative_ai_native_final_video", ID: workspace.WorkspaceID, Version: &revision}}, nil
}

func (s Service) failAINativeRender(ctx context.Context, claim jobruntime.Claim, op AINativeProductionOperation, plan AINativeProductionPlan, code string, cause error) (jobruntime.Result, error) {
	plan.Status = AINativeProductionRenderFailedStatus
	if plan.Render != nil {
		plan.Render.Status, plan.Render.ErrorCode, plan.Render.ErrorMessage, plan.Render.ETASeconds, plan.Render.UpdatedAt = AINativeProductionRenderFailedStatus, code, boundedError(cause), 0, s.now()
	}
	plan.UpdatedAt = s.now()
	_, _ = s.AINativeProductions.SaveAINativeProductionPlan(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, op.WorkspaceID, op, plan, s.now())
	return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: code, Message: boundedError(cause), Retryable: false}}
}

func productionHasFailedAttempt(plan AINativeProductionPlan) bool {
	for _, unit := range plan.Units {
		if len(unit.Attempts) > 0 && unit.Attempts[len(unit.Attempts)-1].Status == AINativeAttemptFailedStatus {
			return true
		}
	}
	for _, unit := range plan.SpeechUnits {
		if len(unit.Attempts) > 0 && unit.Attempts[len(unit.Attempts)-1].Status == AINativeAttemptFailedStatus {
			return true
		}
	}
	return false
}

func productionHasActiveWork(plan AINativeProductionPlan) bool {
	active := func(attempts []AINativeGenerationAttempt, selected string) bool {
		if selected != "" || len(attempts) == 0 {
			return false
		}
		switch attempts[len(attempts)-1].Status {
		case AINativeAttemptPlannedStatus, AINativeAttemptSubmittedStatus, AINativeAttemptRunningStatus, AINativeAttemptIngestingStatus:
			return true
		default:
			return false
		}
	}
	for _, unit := range plan.Units {
		if active(unit.Attempts, unit.SelectedAttemptID) {
			return true
		}
	}
	for _, unit := range plan.SpeechUnits {
		if active(unit.Attempts, unit.SelectedAttemptID) {
			return true
		}
	}
	return false
}

func (s Service) RetryAINativeProductionUnit(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, workspaceID string, request RetryAINativeProductionUnitRequest) (AINativeRequirementWorkspace, error) {
	if s.Projects == nil || s.AINativeProductions == nil || s.AINativeProductionScheduler == nil || !actor.HasScope(ScopeWrite) || request.ExpectedWorkspaceVersion < 1 {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	workspace, err := s.AINativeProductions.GetAINativeProductionWorkspace(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID))
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if workspace.WorkspaceVersion != request.ExpectedWorkspaceVersion || (workspace.ProductionStatus != AINativeProductionFailedStatus && workspace.ProductionStatus != AINativeProductionRenderFailedStatus) || workspace.ProductionPlan == nil || workspace.ConfirmedStoryboardRevision == nil || workspace.ActiveOperationID != "" {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	attemptID, err := s.idGenerator()("ainativeattempt")
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	plan := *workspace.ProductionPlan
	if workspace.ProductionStatus == AINativeProductionRenderFailedStatus {
		if plan.Render == nil || (request.UnitID != plan.Render.ID && request.UnitID != "final-render") {
			return AINativeRequirementWorkspace{}, ErrNotFound
		}
		plan.Status = AINativeProductionRenderingStatus
		plan.Render.Status, plan.Render.ProgressPercent, plan.Render.ETASeconds = AINativeProductionRenderingStatus, 0, max(15, plan.TotalDurationMS/250)
		plan.Render.ErrorCode, plan.Render.ErrorMessage, plan.Render.UpdatedAt, plan.UpdatedAt = "", "", s.now(), s.now()
	} else {
		plan, err = workspace.ProductionPlan.RetryUnit(strings.TrimSpace(request.UnitID), attemptID, s.now())
		if err != nil {
			return AINativeRequirementWorkspace{}, err
		}
	}
	operationID, err := s.idGenerator()("ainativeproductionoperation")
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	op := AINativeProductionOperation{ID: operationID, Version: 1, WorkspaceID: workspace.WorkspaceID, ProjectID: projectID, ExpectedWorkspaceVersion: request.ExpectedWorkspaceVersion,
		StoryboardRevision: *workspace.ConfirmedStoryboardRevision, Actor: actor, CreatedAt: s.now()}
	updated, err := s.AINativeProductions.BeginAINativeProduction(ctx, actor.OrganizationID, projectID, workspace.WorkspaceID, op, plan, s.now())
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if err := s.AINativeProductionScheduler.ScheduleAINativeProduction(ctx, op); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	return withAINativeProductionProgress(updated, s.now()), nil
}

func (s Service) CancelAINativeProduction(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, workspaceID string, expectedWorkspaceVersion int64) (AINativeRequirementWorkspace, error) {
	if s.Projects == nil || s.AINativeProductions == nil || !actor.HasScope(ScopeWrite) || expectedWorkspaceVersion < 1 {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	workspace, err := s.AINativeProductions.GetAINativeProductionWorkspace(ctx, actor.OrganizationID, projectID, strings.TrimSpace(workspaceID))
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if workspace.WorkspaceVersion != expectedWorkspaceVersion || (workspace.ProductionStatus != AINativeProductionRunningStatus && workspace.ProductionStatus != AINativeProductionRenderingStatus) || workspace.ActiveOperationVersion == nil {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	if s.AINativeOperationCanceller != nil {
		_ = s.AINativeOperationCanceller.CancelAINativeOperation(ctx, actor.OrganizationID, projectID, workspace.ActiveOperationID, *workspace.ActiveOperationVersion)
	}
	updated, err := s.AINativeProductions.CancelAINativeProduction(ctx, actor.OrganizationID, projectID, workspace.WorkspaceID, expectedWorkspaceVersion, s.now())
	return withAINativeProductionProgress(updated, s.now()), err
}

func reconcileAINativeVideoAttempt(unit *AINativeGenerationUnit, attempt *AINativeGenerationAttempt, job contract.ProviderJob, now time.Time) bool {
	previous := attempt.Status
	switch job.ProviderStatus {
	case contract.ProviderJobSubmitted:
		attempt.Status = AINativeAttemptSubmittedStatus
	case contract.ProviderJobRunning:
		attempt.Status = AINativeAttemptRunningStatus
	case contract.ProviderJobOutputsReady, contract.ProviderJobIngesting:
		attempt.Status = AINativeAttemptIngestingStatus
	case contract.ProviderJobSucceeded:
		if len(job.ProjectAssetRefs) == 1 && job.ProjectAssetRefs[0].ProjectID == job.ProjectID {
			ref := job.ProjectAssetRefs[0].AssetVersion
			attempt.Status, attempt.OutputAssetRef, unit.SelectedAttemptID = AINativeAttemptSucceededStatus, &ref, attempt.ID
		} else {
			attempt.Status, attempt.ErrorCode, attempt.ErrorMessage = AINativeAttemptFailedStatus, "PROVIDER_OUTPUT_INVALID", "successful video job did not return one project-scoped Asset"
		}
	case contract.ProviderJobFailed, contract.ProviderJobCancelled, contract.ProviderJobExpired:
		attempt.Status = AINativeAttemptFailedStatus
		if job.Error != nil {
			attempt.ErrorCode, attempt.ErrorMessage = job.Error.Code, job.Error.Message
		}
	}
	if attempt.Status != previous {
		attempt.UpdatedAt = now
		return true
	}
	return false
}

func productionAssetsReady(plan AINativeProductionPlan) bool {
	for _, unit := range plan.Units {
		if !selectedAttemptSucceeded(unit.Attempts, unit.SelectedAttemptID) {
			return false
		}
	}
	for _, unit := range plan.SpeechUnits {
		if !selectedAttemptSucceeded(unit.Attempts, unit.SelectedAttemptID) {
			return false
		}
	}
	return len(plan.Units) > 0
}

func speechErrorCode(err error) string {
	if value, ok := err.(provider.SpeechProviderError); ok {
		return strings.ToUpper(value.Code)
	}
	return "SPEECH_SYNTHESIS_FAILED"
}

func speechMIMEType(codec string) string {
	switch codec {
	case "wav":
		return "audio/wav"
	case "ogg_opus":
		return "audio/ogg"
	case "pcm":
		return "audio/L16"
	default:
		return "audio/mpeg"
	}
}

type JobRuntimeAINativeProductionScheduler struct {
	Store jobruntime.Store
	Now   func() time.Time
}

func (s JobRuntimeAINativeProductionScheduler) ScheduleAINativeProduction(ctx context.Context, operation AINativeProductionOperation) error {
	if s.Store == nil {
		return fmt.Errorf("AI native production job store is required")
	}
	payload, err := json.Marshal(AINativeProductionJobPayload{Operation: operation})
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	hash := sha256.Sum256(payload)
	_, _, err = s.Store.Enqueue(ctx, jobruntime.CreateRequest{Job: contract.Job{ID: operation.ID, Kind: AINativeProductionJobKind, OrganizationID: operation.Actor.OrganizationID,
		ProjectID: operation.ProjectID, Status: contract.JobQueued, Cancellable: true, MaxAttempts: 720, Version: 1, CreatedAt: now, UpdatedAt: now}, Payload: payload,
		IdempotencyKey: contract.IdempotencyKey("ai-native-production-" + operation.ID), RequestHash: hex.EncodeToString(hash[:])})
	return err
}
