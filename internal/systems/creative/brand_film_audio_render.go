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
	"github.com/shikanon/cookies/internal/platform/media"
)

const (
	AudioMixRenderPreview = "preview"
	AudioMixRenderJobKind = "creative.brand-audio.mix-render"
)

type AudioMixRenderScheduler interface {
	ScheduleAudioMixRender(context.Context, contract.OrganizationID, contract.ProjectID, string, AudioMixRenderJob) error
}

func (s Service) RenderBrandFilmAudioPreview(ctx context.Context, requestContext contract.RequestContext, projectID contract.ProjectID, taskID string, request BrandFilmRevisionRequest) (TaskDetail, error) {
	if s.AudioMixRenderer == nil || s.AudioMixScheduler == nil || s.RenderedAssets == nil || s.Assets == nil {
		return TaskDetail{}, fmt.Errorf("brand audio mix rendering dependencies are incomplete")
	}
	detail, err := s.requireBrandFilmWorkspace(ctx, requestContext.Actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	if request.ExpectedRevision != detail.VideoDraft.Revision {
		return TaskDetail{}, ErrVersionConflict
	}
	if detail.VideoDraft.BrandFilm.Audio == nil {
		return TaskDetail{}, ErrInvalidState
	}
	audio := *detail.VideoDraft.BrandFilm.Audio
	mix := audio.CurrentMix()
	if mix == nil || !brandAudioMixAssetsReady(mix) {
		return TaskDetail{}, fmt.Errorf("audio assets must be ready before rendering")
	}
	compiled, err := CompileBrandAudioMix(requestContext.Actor.OrganizationID, projectID, *mix)
	if err != nil {
		return TaskDetail{}, err
	}
	for _, clip := range compiled.Clips {
		asset, readErr := s.Assets.ReadForCreative(ctx, requestContext.Actor, projectID, clip.Asset)
		if readErr != nil {
			return TaskDetail{}, readErr
		}
		if !asset.Ready || asset.Kind != contract.AssetAudio {
			return TaskDetail{}, fmt.Errorf("audio clip %s is not a ready project audio asset", clip.ID)
		}
	}
	for _, existing := range audio.RenderJobs {
		if existing.Kind == AudioMixRenderPreview && existing.MixContentHash == mix.ContentHash && (existing.Status == string(RenderQueued) || existing.Status == string(RenderRunning) || existing.Status == string(RenderSucceeded)) {
			if existing.Status == string(RenderQueued) {
				if err := s.AudioMixScheduler.ScheduleAudioMixRender(ctx, requestContext.Actor.OrganizationID, projectID, taskID, existing); err != nil {
					return TaskDetail{}, err
				}
			}
			return detail, nil
		}
	}
	id, err := s.idGenerator()("audiomixrender")
	if err != nil {
		return TaskDetail{}, err
	}
	now := s.now()
	job := AudioMixRenderJob{ID: id, TaskID: taskID, MixRevision: mix.Revision, MixContentHash: mix.ContentHash, Kind: AudioMixRenderPreview, Status: string(RenderQueued), RendererVersion: media.AudioMixRendererVersion, CreatedBy: requestContext.Actor.Principal, CreatedAt: now, UpdatedAt: now}
	audio.RenderJobs = append(audio.RenderJobs, job)
	audio.Status, audio.UpdatedAt = "preview_queued", now
	next := cloneBrandVideoDraft(*detail.VideoDraft)
	next.Revision++
	next.BrandFilm.Revision, next.BrandFilm.Audio, next.BrandFilm.UpdatedAt = next.Revision, &audio, now
	stored, err := s.persistBrandFilmDraft(ctx, requestContext.Actor, projectID, taskID, *detail.VideoDraft, next)
	if err != nil {
		return TaskDetail{}, err
	}
	if err := s.AudioMixScheduler.ScheduleAudioMixRender(ctx, requestContext.Actor.OrganizationID, projectID, taskID, job); err != nil {
		return TaskDetail{}, err
	}
	return stored, nil
}

func (s Service) ExecuteBrandFilmAudioRender(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, taskID, jobID string) error {
	if s.AudioMixRenderer == nil || s.RenderedAssets == nil {
		return fmt.Errorf("brand audio rendering execution dependencies are incomplete")
	}
	detail, job, mix, err := s.loadAudioRender(ctx, organizationID, projectID, taskID, jobID)
	if err != nil {
		return err
	}
	if job.Status == string(RenderSucceeded) {
		return nil
	}
	if job.Status == string(RenderFailed) {
		return ErrInvalidState
	}
	actor := contract.ActorContext{OrganizationID: organizationID, Principal: job.CreatedBy, Scopes: []contract.Scope{"project.read", "assets.read", "assets.write", ScopeRead, ScopeWrite}}
	if err := s.updateAudioRenderJob(ctx, actor, projectID, taskID, detail, jobID, string(RenderRunning), nil, "", ""); err != nil {
		return err
	}
	request, err := CompileBrandAudioMix(organizationID, projectID, mix)
	if err != nil {
		return s.failAudioRender(ctx, actor, projectID, taskID, jobID, "AUDIO_MIX_COMPILE_FAILED", err)
	}
	output, err := s.AudioMixRenderer.RenderAudioMix(ctx, request)
	if err != nil {
		return s.failAudioRender(ctx, actor, projectID, taskID, jobID, "AUDIO_MIX_RENDER_FAILED", err)
	}
	defer output.Content.Close()
	rc := contract.RequestContext{RequestID: "audio-mix-" + jobID, TraceID: "audio-mix-" + jobID, Actor: actor}
	ref, err := s.RenderedAssets.IngestRenderedVideo(ctx, rc, projectID, jobID, output.Content, output.SizeBytes)
	if err != nil {
		return s.failAudioRender(ctx, actor, projectID, taskID, jobID, "AUDIO_MIX_ASSET_INTAKE_FAILED", err)
	}
	assetRef := ref.AssetVersion
	latest, _, _, err := s.loadAudioRender(ctx, organizationID, projectID, taskID, jobID)
	if err != nil {
		return err
	}
	return s.updateAudioRenderJob(ctx, actor, projectID, taskID, latest, jobID, string(RenderSucceeded), &assetRef, "", "")
}

func (s Service) loadAudioRender(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, taskID, jobID string) (TaskDetail, AudioMixRenderJob, AudioMixVersion, error) {
	detail, err := s.Repository.GetTaskDetail(ctx, org, project, taskID)
	if err != nil {
		return TaskDetail{}, AudioMixRenderJob{}, AudioMixVersion{}, err
	}
	if detail.VideoDraft == nil || detail.VideoDraft.BrandFilm == nil || detail.VideoDraft.BrandFilm.Audio == nil {
		return TaskDetail{}, AudioMixRenderJob{}, AudioMixVersion{}, ErrInvalidState
	}
	audio := detail.VideoDraft.BrandFilm.Audio
	var job *AudioMixRenderJob
	for index := range audio.RenderJobs {
		if audio.RenderJobs[index].ID == jobID {
			job = &audio.RenderJobs[index]
			break
		}
	}
	if job == nil {
		return TaskDetail{}, AudioMixRenderJob{}, AudioMixVersion{}, fmt.Errorf("audio render job not found")
	}
	for _, variant := range audio.Variants {
		for _, mix := range variant.MixVersions {
			if mix.Revision == job.MixRevision && mix.ContentHash == job.MixContentHash {
				return detail, *job, mix, nil
			}
		}
	}
	return TaskDetail{}, AudioMixRenderJob{}, AudioMixVersion{}, fmt.Errorf("audio render mix revision is unavailable")
}

func (s Service) updateAudioRenderJob(ctx context.Context, actor contract.ActorContext, project contract.ProjectID, taskID string, detail TaskDetail, jobID, status string, output *contract.AssetVersionRef, code, message string) error {
	next := cloneBrandVideoDraft(*detail.VideoDraft)
	now := s.now()
	audio := next.BrandFilm.Audio
	found := false
	for index := range audio.RenderJobs {
		if audio.RenderJobs[index].ID == jobID {
			audio.RenderJobs[index].Status, audio.RenderJobs[index].UpdatedAt, audio.RenderJobs[index].OutputAssetRef, audio.RenderJobs[index].ErrorCode, audio.RenderJobs[index].ErrorMessage = status, now, output, code, message
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("audio render job not found")
	}
	switch status {
	case string(RenderRunning):
		audio.Status = "preview_rendering"
	case string(RenderSucceeded):
		audio.Status, audio.MixedPreview = "preview_ready", output
	case string(RenderFailed):
		audio.Status = "preview_failed"
	}
	audio.UpdatedAt = now
	next.Revision++
	next.BrandFilm.Revision, next.BrandFilm.UpdatedAt = next.Revision, now
	_, err := s.persistBrandFilmDraft(ctx, actor, project, taskID, *detail.VideoDraft, next)
	return err
}

func (s Service) failAudioRender(ctx context.Context, actor contract.ActorContext, project contract.ProjectID, taskID, jobID, code string, cause error) error {
	detail, _, _, loadErr := s.loadAudioRender(ctx, actor.OrganizationID, project, taskID, jobID)
	if loadErr == nil {
		_ = s.updateAudioRenderJob(ctx, actor, project, taskID, detail, jobID, string(RenderFailed), nil, code, boundedError(cause))
	}
	return cause
}

type JobRuntimeAudioMixRenderScheduler struct {
	Store jobruntime.Store
	NewID func() (string, error)
	Now   func() time.Time
}

func (s JobRuntimeAudioMixRenderScheduler) ScheduleAudioMixRender(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, taskID string, render AudioMixRenderJob) error {
	if s.Store == nil || s.NewID == nil {
		return fmt.Errorf("job runtime store and ID generator are required")
	}
	payload, err := json.Marshal(struct {
		TaskID      string `json:"task_id"`
		RenderJobID string `json:"render_job_id"`
	}{taskID, render.ID})
	if err != nil {
		return err
	}
	id, err := s.NewID()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	digest := sha256.Sum256([]byte(taskID + ":" + render.ID + ":" + render.MixContentHash))
	_, _, err = s.Store.Enqueue(ctx, jobruntime.CreateRequest{Job: contract.Job{ID: id, Kind: AudioMixRenderJobKind, OrganizationID: org, ProjectID: project, Status: contract.JobQueued, MaxAttempts: 3, Version: 1, CreatedAt: now, UpdatedAt: now}, Payload: payload, IdempotencyKey: contract.IdempotencyKey("audio-mix-render-" + render.ID), RequestHash: hex.EncodeToString(digest[:])})
	return err
}

func AudioMixRenderRuntimeHandler(service Service) jobruntime.Handler {
	return func(ctx context.Context, claim jobruntime.Claim) (jobruntime.Result, error) {
		var payload struct {
			TaskID      string `json:"task_id"`
			RenderJobID string `json:"render_job_id"`
		}
		if claim.Job.Kind != AudioMixRenderJobKind || json.Unmarshal(claim.Payload, &payload) != nil || strings.TrimSpace(payload.TaskID) == "" || strings.TrimSpace(payload.RenderJobID) == "" {
			return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "AUDIO_MIX_RENDER_PAYLOAD_INVALID", Message: "Audio mix render payload is invalid", Retryable: false}}
		}
		if err := service.ExecuteBrandFilmAudioRender(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, payload.TaskID, payload.RenderJobID); err != nil {
			return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "AUDIO_MIX_RENDER_FAILED", Message: "Audio mix rendering failed", Retryable: false}}
		}
		return jobruntime.Result{}, nil
	}
}
