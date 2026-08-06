package creative

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
	"github.com/shikanon/cookies/internal/platform/media"
)

// EditingRenderJob freezes a TimelineVersion at submission time. A later edit
// can therefore never change the input of an already queued render.
type EditingRenderJob struct {
	ID              string                    `json:"id"`
	OrganizationID  contract.OrganizationID   `json:"organization_id"`
	ProjectID       contract.ProjectID        `json:"project_id"`
	EditTaskID      string                    `json:"edit_task_id"`
	Timeline        TimelineVersion           `json:"timeline"`
	Kind            EditingRenderKind         `json:"kind"`
	Status          EditingRenderStatus       `json:"status"`
	ProgressPercent int                       `json:"progress_percent"`
	OutputAsset     *contract.ProjectAssetRef `json:"output_asset,omitempty"`
	ErrorCode       string                    `json:"error_code,omitempty"`
	ErrorMessage    string                    `json:"error_message,omitempty"`
	RetryOf         string                    `json:"retry_of,omitempty"`
	CreatedBy       contract.Principal        `json:"created_by"`
	CreatedAt       time.Time                 `json:"created_at"`
	UpdatedAt       time.Time                 `json:"updated_at"`
}

type EditingRenderKind string

const (
	EditingRenderPreview EditingRenderKind = "preview"
	EditingRenderExport  EditingRenderKind = "export"
)

type EditingRenderStatus string

const (
	EditingRenderQueued    EditingRenderStatus = "queued"
	EditingRenderRunning   EditingRenderStatus = "running"
	EditingRenderSucceeded EditingRenderStatus = "succeeded"
	EditingRenderFailed    EditingRenderStatus = "failed"
	EditingRenderCancelled EditingRenderStatus = "cancelled"
)

type EditingRenderRepository interface {
	CreateEditingRender(context.Context, EditingRenderJob) (EditingRenderJob, error)
	GetEditingRender(context.Context, contract.OrganizationID, contract.ProjectID, string) (EditingRenderJob, error)
	FindReusableEditingRender(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, string, EditingRenderKind) (EditingRenderJob, error)
	MarkEditingRenderRunning(context.Context, contract.OrganizationID, contract.ProjectID, string, time.Time) (EditingRenderJob, error)
	UpdateEditingRenderProgress(context.Context, contract.OrganizationID, contract.ProjectID, string, int, time.Time) error
	CompleteEditingRender(context.Context, contract.OrganizationID, contract.ProjectID, string, contract.ProjectAssetRef, time.Time) error
	FailEditingRender(context.Context, contract.OrganizationID, contract.ProjectID, string, string, string, time.Time) error
	CancelEditingRender(context.Context, contract.OrganizationID, contract.ProjectID, string, time.Time) (EditingRenderJob, error)
}

type CreateEditingRenderRequest struct {
	Kind EditingRenderKind `json:"kind"`
}

const editingRenderExecutionKind = "creative.editing.render"

type EditingRenderScheduler interface {
	ScheduleEditingRender(context.Context, EditingRenderJob) error
}

func (s Service) CreateEditingRender(ctx context.Context, rc contract.RequestContext, projectID contract.ProjectID, editTaskID string, request CreateEditingRenderRequest) (EditingRenderJob, error) {
	if s.EditingRenders == nil || s.EditingRenderScheduler == nil || s.AINativeTimelineRenderer == nil || s.RenderedAssets == nil {
		return EditingRenderJob{}, fmt.Errorf("editing render dependencies are incomplete")
	}
	if err := rc.Validate(); err != nil {
		return EditingRenderJob{}, err
	}
	if !rc.Actor.HasScope(ScopeWrite) {
		return EditingRenderJob{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if request.Kind != EditingRenderPreview && request.Kind != EditingRenderExport {
		return EditingRenderJob{}, fmt.Errorf("editing render kind is unsupported")
	}
	task, err := s.GetEditTask(ctx, rc.Actor, projectID, editTaskID)
	if err != nil {
		return EditingRenderJob{}, err
	}
	if cached, cacheErr := s.EditingRenders.FindReusableEditingRender(ctx, rc.Actor.OrganizationID, projectID, task.ID, task.CurrentTimeline.Version, task.CurrentTimeline.ContentHash, request.Kind); cacheErr == nil {
		return cached, nil
	} else if cacheErr != ErrNotFound {
		return EditingRenderJob{}, cacheErr
	}
	id, err := s.idGenerator()("editingrender")
	if err != nil {
		return EditingRenderJob{}, err
	}
	now := s.now()
	job := EditingRenderJob{ID: id, OrganizationID: rc.Actor.OrganizationID, ProjectID: projectID, EditTaskID: task.ID, Timeline: task.CurrentTimeline, Kind: request.Kind, Status: EditingRenderQueued, CreatedBy: rc.Actor.Principal, CreatedAt: now, UpdatedAt: now}
	created, err := s.EditingRenders.CreateEditingRender(ctx, job)
	if err != nil {
		return EditingRenderJob{}, err
	}
	if err := s.EditingRenderScheduler.ScheduleEditingRender(ctx, created); err != nil {
		return EditingRenderJob{}, err
	}
	return created, nil
}

type JobRuntimeEditingRenderScheduler struct {
	Store jobruntime.Store
	NewID func() (string, error)
	Now   func() time.Time
}

func (s JobRuntimeEditingRenderScheduler) ScheduleEditingRender(ctx context.Context, render EditingRenderJob) error {
	if s.Store == nil || s.NewID == nil {
		return fmt.Errorf("job runtime store and ID generator are required")
	}
	payload, err := json.Marshal(struct {
		RenderJobID string `json:"render_job_id"`
	}{render.ID})
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
	sum := sha256.Sum256([]byte(render.ID))
	_, _, err = s.Store.Enqueue(ctx, jobruntime.CreateRequest{Job: contract.Job{ID: id, Kind: editingRenderExecutionKind, OrganizationID: render.OrganizationID, ProjectID: render.ProjectID, Status: contract.JobQueued, MaxAttempts: 3, Version: 1, CreatedAt: now, UpdatedAt: now}, Payload: payload, IdempotencyKey: contract.IdempotencyKey("editing-render-" + render.ID), RequestHash: hex.EncodeToString(sum[:])})
	return err
}
func EditingRenderRuntimeHandler(service Service) jobruntime.Handler {
	return func(ctx context.Context, claim jobruntime.Claim) (jobruntime.Result, error) {
		var payload struct {
			RenderJobID string `json:"render_job_id"`
		}
		if claim.Job.Kind != editingRenderExecutionKind || json.Unmarshal(claim.Payload, &payload) != nil || strings.TrimSpace(payload.RenderJobID) == "" {
			return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "EDITING_RENDER_PAYLOAD_INVALID", Message: "editing render payload is invalid", Retryable: false}}
		}
		if err := service.ExecuteEditingRender(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, payload.RenderJobID); err != nil {
			// Keep the public RenderJob terminal even when restoration fails before
			// ExecuteEditingRender can mark it running. Otherwise the UI polls a
			// permanently queued job after the runtime job has already failed.
			if service.EditingRenders != nil {
				_ = service.EditingRenders.FailEditingRender(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, payload.RenderJobID, "EDITING_RENDER_FAILED", boundedError(err), service.now())
			}
			return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "EDITING_RENDER_FAILED", Message: "editing render failed", Retryable: false}}
		}
		return jobruntime.Result{}, nil
	}
}

func (s Service) ExecuteEditingRender(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, jobID string) error {
	if s.EditingRenders == nil || s.AINativeTimelineRenderer == nil || s.RenderedAssets == nil {
		return fmt.Errorf("editing render dependencies are incomplete")
	}
	job, err := s.EditingRenders.GetEditingRender(ctx, organizationID, projectID, jobID)
	if err != nil {
		return err
	}
	if job.Status == EditingRenderSucceeded {
		return nil
	}
	if job.Status == EditingRenderCancelled {
		return nil
	}
	if job.Status != EditingRenderQueued {
		return ErrInvalidState
	}
	job, err = s.EditingRenders.MarkEditingRenderRunning(ctx, organizationID, projectID, jobID, s.now())
	if err != nil {
		return err
	}
	request, err := CompileEditingTimelineV1(job.Timeline.Timeline, organizationID, projectID)
	if err != nil {
		return s.failEditingRender(ctx, job, "TIMELINE_INVALID", err)
	}
	last := -1
	output, err := s.AINativeTimelineRenderer.Render(ctx, request, func(progress media.TimelineProgress) error {
		latest, getErr := s.EditingRenders.GetEditingRender(ctx, organizationID, projectID, jobID)
		if getErr != nil {
			return getErr
		}
		if latest.Status == EditingRenderCancelled {
			return context.Canceled
		}
		if progress.Percent <= last {
			return nil
		}
		last = progress.Percent
		return s.EditingRenders.UpdateEditingRenderProgress(ctx, organizationID, projectID, jobID, progress.Percent, s.now())
	})
	if err != nil {
		return s.failEditingRender(ctx, job, "TIMELINE_RENDER_FAILED", err)
	}
	defer output.Content.Close()
	actor := contract.ActorContext{OrganizationID: organizationID, Principal: job.CreatedBy, Scopes: []contract.Scope{"project.read", "assets.read", "assets.write", ScopeRead, ScopeWrite}}
	ref, err := s.RenderedAssets.IngestRenderedVideo(ctx, contract.RequestContext{RequestID: "editing-render-" + jobID, TraceID: "editing-render-" + jobID, Actor: actor}, projectID, jobID, output.Content, output.SizeBytes)
	if err != nil {
		return s.failEditingRender(ctx, job, "RENDERED_ASSET_INTAKE_FAILED", err)
	}
	return s.EditingRenders.CompleteEditingRender(ctx, organizationID, projectID, jobID, ref, s.now())
}

func (s Service) CancelEditingRender(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, jobID string) (EditingRenderJob, error) {
	if s.EditingRenders == nil || s.Projects == nil {
		return EditingRenderJob{}, fmt.Errorf("editing render dependencies are incomplete")
	}
	if !actor.HasScope(ScopeWrite) {
		return EditingRenderJob{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return EditingRenderJob{}, err
	}
	return s.EditingRenders.CancelEditingRender(ctx, actor.OrganizationID, projectID, jobID, s.now())
}

func (s Service) RetryEditingRender(ctx context.Context, rc contract.RequestContext, projectID contract.ProjectID, jobID string) (EditingRenderJob, error) {
	if s.EditingRenders == nil || s.EditingRenderScheduler == nil {
		return EditingRenderJob{}, fmt.Errorf("editing render dependencies are incomplete")
	}
	if err := rc.Validate(); err != nil {
		return EditingRenderJob{}, err
	}
	if !rc.Actor.HasScope(ScopeWrite) {
		return EditingRenderJob{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	previous, err := s.GetEditingRender(ctx, rc.Actor, projectID, jobID)
	if err != nil {
		return EditingRenderJob{}, err
	}
	if previous.Status != EditingRenderFailed {
		return EditingRenderJob{}, ErrInvalidState
	}
	id, err := s.idGenerator()("editingrender")
	if err != nil {
		return EditingRenderJob{}, err
	}
	now := s.now()
	next := EditingRenderJob{ID: id, OrganizationID: rc.Actor.OrganizationID, ProjectID: projectID, EditTaskID: previous.EditTaskID, Timeline: previous.Timeline, Kind: previous.Kind, Status: EditingRenderQueued, RetryOf: previous.ID, CreatedBy: rc.Actor.Principal, CreatedAt: now, UpdatedAt: now}
	created, err := s.EditingRenders.CreateEditingRender(ctx, next)
	if err != nil {
		return EditingRenderJob{}, err
	}
	if err = s.EditingRenderScheduler.ScheduleEditingRender(ctx, created); err != nil {
		return EditingRenderJob{}, err
	}
	return created, nil
}

func (s Service) GetEditingRender(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, jobID string) (EditingRenderJob, error) {
	if s.EditingRenders == nil || s.Projects == nil {
		return EditingRenderJob{}, fmt.Errorf("editing render dependencies are incomplete")
	}
	if !actor.HasScope(ScopeRead) {
		return EditingRenderJob{}, fmt.Errorf("%s scope is required", ScopeRead)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return EditingRenderJob{}, err
	}
	return s.EditingRenders.GetEditingRender(ctx, actor.OrganizationID, projectID, jobID)
}

func (s Service) failEditingRender(ctx context.Context, job EditingRenderJob, code string, cause error) error {
	_ = s.EditingRenders.FailEditingRender(ctx, job.OrganizationID, job.ProjectID, job.ID, code, boundedError(cause), s.now())
	return cause
}

type editingRenderedAssetWriter interface {
	IngestRenderedVideo(context.Context, contract.RequestContext, contract.ProjectID, string, io.Reader, int64) (contract.ProjectAssetRef, error)
}

func (j EditingRenderJob) Validate() error {
	if strings.TrimSpace(j.ID) == "" || j.OrganizationID == "" || j.ProjectID == "" || strings.TrimSpace(j.EditTaskID) == "" || j.Timeline.Validate() != nil || (j.Kind != EditingRenderPreview && j.Kind != EditingRenderExport) || (j.Status != EditingRenderQueued && j.Status != EditingRenderRunning && j.Status != EditingRenderSucceeded && j.Status != EditingRenderFailed && j.Status != EditingRenderCancelled) || j.ProgressPercent < 0 || j.ProgressPercent > 100 || j.CreatedAt.IsZero() || j.UpdatedAt.IsZero() {
		return fmt.Errorf("editing render job is incomplete")
	}
	return nil
}
