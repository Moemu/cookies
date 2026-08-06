package creative

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
)

type editingRenderMemoryRepository struct {
	job      EditingRenderJob
	progress []int
}

func (r *editingRenderMemoryRepository) CreateEditingRender(_ context.Context, job EditingRenderJob) (EditingRenderJob, error) {
	r.job = job
	return job, nil
}
func (r *editingRenderMemoryRepository) GetEditingRender(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, id string) (EditingRenderJob, error) {
	if r.job.ID != id {
		return EditingRenderJob{}, ErrNotFound
	}
	return r.job, nil
}
func (r *editingRenderMemoryRepository) FindReusableEditingRender(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, taskID string, version int64, hash string, kind EditingRenderKind) (EditingRenderJob, error) {
	if r.job.Status == EditingRenderSucceeded && r.job.EditTaskID == taskID && r.job.Timeline.Version == version && r.job.Timeline.ContentHash == hash && r.job.Kind == kind && r.job.OutputAsset != nil {
		return r.job, nil
	}
	return EditingRenderJob{}, ErrNotFound
}
func (r *editingRenderMemoryRepository) MarkEditingRenderRunning(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, id string, now time.Time) (EditingRenderJob, error) {
	if r.job.ID != id || r.job.Status != EditingRenderQueued {
		return EditingRenderJob{}, ErrInvalidState
	}
	r.job.Status = EditingRenderRunning
	r.job.UpdatedAt = now
	return r.job, nil
}
func (r *editingRenderMemoryRepository) UpdateEditingRenderProgress(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _ string, progress int, _ time.Time) error {
	r.progress = append(r.progress, progress)
	r.job.ProgressPercent = progress
	return nil
}
func (r *editingRenderMemoryRepository) CompleteEditingRender(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _ string, ref contract.ProjectAssetRef, now time.Time) error {
	r.job.Status = EditingRenderSucceeded
	r.job.ProgressPercent = 100
	r.job.OutputAsset = &ref
	r.job.UpdatedAt = now
	return nil
}
func (r *editingRenderMemoryRepository) FailEditingRender(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _, code, message string, now time.Time) error {
	r.job.Status = EditingRenderFailed
	r.job.ErrorCode = code
	r.job.ErrorMessage = message
	r.job.UpdatedAt = now
	return nil
}
func (r *editingRenderMemoryRepository) CancelEditingRender(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _ string, now time.Time) (EditingRenderJob, error) {
	r.job.Status = EditingRenderCancelled
	r.job.UpdatedAt = now
	return r.job, nil
}

type editingRenderSchedulerStub struct{ scheduled EditingRenderJob }

func (s *editingRenderSchedulerStub) ScheduleEditingRender(_ context.Context, job EditingRenderJob) error {
	s.scheduled = job
	return nil
}

func TestEditingRenderFreezesTimelineAndReturnsOutputAsset(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	asset := contract.AssetVersionRef{AssetID: "source", Version: 1}
	timeline := EditingTimelineV1{SchemaVersion: EditingTimelineSchemaV1, OutputProfile: EditingMVPVerticalOutputProfile, DurationMS: 6000, Tracks: []EditingTimelineTrack{{ID: "video", Role: EditingTrackPrimaryVideo, Clips: []EditingTimelineClip{{ID: "clip", AssetRef: &asset, TimelineEndMS: 6000, SourceOutMS: 6000}}}}}
	edits := &memoryEditTaskRepository{}
	renders := &editingRenderMemoryRepository{}
	scheduler := &editingRenderSchedulerStub{}
	renderer := &productionTimelineRendererStub{}
	writer := &productionRenderedWriterStub{}
	sequence := 0
	service := Service{Projects: testProjects{}, EditTasks: edits, EditingRenders: renders, EditingRenderScheduler: scheduler, Assets: testAssetReader{snapshots: map[contract.AssetID]CreativeAssetSnapshot{asset.AssetID: {Ref: asset, Kind: contract.AssetVideo, Ready: true, DurationMS: 6000}}}, AINativeTimelineRenderer: renderer, RenderedAssets: writer, NewID: func(prefix string) (string, error) { sequence++; return prefix + "_" + string(rune('0'+sequence)), nil }, Now: func() time.Time { return now }}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeRead, ScopeWrite}}
	task, err := service.CreateEditTask(context.Background(), actor, "project_1", CreateEditTaskRequest{DisplayName: "剪辑", Timeline: timeline})
	if err != nil {
		t.Fatal(err)
	}
	rc := contract.RequestContext{RequestID: "request_1", TraceID: "trace_1", Actor: actor}
	job, err := service.CreateEditingRender(context.Background(), rc, "project_1", task.ID, CreateEditingRenderRequest{Kind: EditingRenderPreview})
	if err != nil {
		t.Fatal(err)
	}
	if job.Timeline.Version != 1 || scheduler.scheduled.ID != job.ID {
		t.Fatalf("job was not frozen and queued: %#v %#v", job, scheduler.scheduled)
	}
	if err := service.ExecuteEditingRender(context.Background(), "org_1", "project_1", job.ID); err != nil {
		t.Fatal(err)
	}
	if renders.job.Status != EditingRenderSucceeded || renders.job.ProgressPercent != 100 || renders.job.OutputAsset == nil || renderer.calls != 1 || writer.calls != 1 {
		t.Fatalf("render result=%#v renderer=%d writer=%d", renders.job, renderer.calls, writer.calls)
	}
	reused, err := service.CreateEditingRender(context.Background(), rc, "project_1", task.ID, CreateEditingRenderRequest{Kind: EditingRenderPreview})
	if err != nil {
		t.Fatal(err)
	}
	if reused.ID != job.ID || reused.OutputAsset == nil || renderer.calls != 1 || writer.calls != 1 {
		t.Fatalf("proxy render was not reused: %#v", reused)
	}
}

func TestEditingRenderCancelsQueuedWorkAndRetriesFailureFromFrozenTimeline(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	asset := contract.AssetVersionRef{AssetID: "source", Version: 1}
	timeline := EditingTimelineV1{SchemaVersion: EditingTimelineSchemaV1, OutputProfile: EditingMVPVerticalOutputProfile, DurationMS: 6000, Tracks: []EditingTimelineTrack{{ID: "video", Role: EditingTrackPrimaryVideo, Clips: []EditingTimelineClip{{ID: "clip", AssetRef: &asset, TimelineEndMS: 6000, SourceOutMS: 6000}}}}}
	edits, renders, scheduler := &memoryEditTaskRepository{}, &editingRenderMemoryRepository{}, &editingRenderSchedulerStub{}
	sequence := 0
	service := Service{Projects: testProjects{}, EditTasks: edits, EditingRenders: renders, EditingRenderScheduler: scheduler, Assets: testAssetReader{snapshots: map[contract.AssetID]CreativeAssetSnapshot{asset.AssetID: {Ref: asset, Kind: contract.AssetVideo, Ready: true, DurationMS: 6000}}}, AINativeTimelineRenderer: &productionTimelineRendererStub{}, RenderedAssets: &productionRenderedWriterStub{}, NewID: func(prefix string) (string, error) { sequence++; return prefix + string(rune('0'+sequence)), nil }, Now: func() time.Time { return now }}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeRead, ScopeWrite}}
	task, err := service.CreateEditTask(context.Background(), actor, "project_1", CreateEditTaskRequest{DisplayName: "剪辑", Timeline: timeline})
	if err != nil {
		t.Fatal(err)
	}
	rc := contract.RequestContext{RequestID: "request_1", TraceID: "trace_1", Actor: actor}
	job, err := service.CreateEditingRender(context.Background(), rc, "project_1", task.ID, CreateEditingRenderRequest{Kind: EditingRenderExport})
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := service.CancelEditingRender(context.Background(), actor, "project_1", job.ID)
	if err != nil || cancelled.Status != EditingRenderCancelled {
		t.Fatalf("cancelled=%#v err=%v", cancelled, err)
	}
	renders.job.Status, renders.job.ErrorCode = EditingRenderFailed, "TIMELINE_RENDER_FAILED"
	retry, err := service.RetryEditingRender(context.Background(), rc, "project_1", job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if retry.RetryOf != job.ID || retry.Timeline.ContentHash != job.Timeline.ContentHash || retry.Status != EditingRenderQueued || scheduler.scheduled.ID != retry.ID {
		t.Fatalf("retry=%#v scheduled=%#v", retry, scheduler.scheduled)
	}
}

func TestEditingRenderRuntimeFailureMakesPublicJobTerminal(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	renders := &editingRenderMemoryRepository{job: EditingRenderJob{
		ID: "render_1", OrganizationID: "org_1", ProjectID: "project_1",
		Status: EditingRenderQueued, CreatedAt: now, UpdatedAt: now,
	}}
	handler := EditingRenderRuntimeHandler(Service{EditingRenders: renders, Now: func() time.Time { return now }})
	_, err := handler(context.Background(), jobruntime.Claim{
		Job:     contract.Job{Kind: editingRenderExecutionKind, OrganizationID: "org_1", ProjectID: "project_1"},
		Payload: []byte(`{"render_job_id":"render_1"}`),
	})
	if err == nil {
		t.Fatal("expected runtime failure")
	}
	if renders.job.Status != EditingRenderFailed || renders.job.ErrorCode != "EDITING_RENDER_FAILED" {
		t.Fatalf("public render job remained non-terminal: %#v", renders.job)
	}
}
