package creative

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type memoryEditTaskRepository struct {
	stored EditTask
}

type projectBoundEditingAssetReader struct {
	project  contract.ProjectID
	snapshot CreativeAssetSnapshot
}

func (r projectBoundEditingAssetReader) ReadForCreative(_ context.Context, _ contract.ActorContext, project contract.ProjectID, ref contract.AssetVersionRef) (CreativeAssetSnapshot, error) {
	if project != r.project || ref != r.snapshot.Ref {
		return CreativeAssetSnapshot{}, ErrNotFound
	}
	return r.snapshot, nil
}

func (r *memoryEditTaskRepository) CreateEditTask(_ context.Context, task EditTask, timeline TimelineVersion) (EditTask, error) {
	r.stored = task
	r.stored.CurrentTimeline = timeline
	return r.stored, nil
}

func (r *memoryEditTaskRepository) GetEditTask(_ context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (EditTask, error) {
	if r.stored.ID != id || r.stored.OrganizationID != org || r.stored.ProjectID != project {
		return EditTask{}, ErrNotFound
	}
	return r.stored, nil
}

func (r *memoryEditTaskRepository) FindEditTaskBySource(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _ EditTaskEntrySource, _ string) (EditTask, error) {
	return EditTask{}, ErrNotFound
}

func (r *memoryEditTaskRepository) AppendEditTimeline(_ context.Context, task EditTask, expectedVersion int64, version TimelineVersion) (EditTask, error) {
	if r.stored.ID != task.ID || r.stored.CurrentTimeline.Version != expectedVersion {
		return EditTask{}, ErrVersionConflict
	}
	r.stored.CurrentTimeline = version
	return r.stored, nil
}

func TestCreateShortDramaV2EditTaskPrefillsPrerollThenSourceVideo(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	repository := &memoryEditTaskRepository{}
	preroll := contract.AssetVersionRef{AssetID: "asset_preroll", Version: 1}
	source := contract.AssetVersionRef{AssetID: "asset_source", Version: 3}
	service := Service{
		Projects:  testProjects{},
		EditTasks: repository,
		Assets: testAssetReader{snapshots: map[contract.AssetID]CreativeAssetSnapshot{
			preroll.AssetID: {Ref: preroll, Kind: contract.AssetVideo, MIMEType: "video/mp4", Ready: true, DurationMS: 6000},
			source.AssetID:  {Ref: source, Kind: contract.AssetVideo, MIMEType: "video/mp4", Ready: true, DurationMS: 15000},
		}},
		NewID: func(prefix string) (string, error) { return prefix + "_1", nil },
		Now:   func() time.Time { return now },
	}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeWrite}}

	created, err := service.CreateShortDramaV2EditTask(context.Background(), actor, "project_1", CreateShortDramaV2EditTaskRequest{
		SourceCreativeTaskID: "creative_task_1", PrerollAsset: preroll, SourceAsset: source,
	})
	if err != nil {
		t.Fatalf("CreateShortDramaV2EditTask() error = %v", err)
	}
	if created.EntrySource != EditTaskEntryShortDramaV2 || created.SourceCreativeTaskID != "creative_task_1" || created.CurrentTimeline.Version != 1 {
		t.Fatalf("created edit task = %#v", created)
	}
	clips := created.CurrentTimeline.Timeline.Tracks[0].Clips
	if len(clips) != 2 || clips[0].AssetRef == nil || *clips[0].AssetRef != preroll || clips[0].TimelineStartMS != 0 || clips[0].TimelineEndMS != 6000 ||
		clips[1].AssetRef == nil || *clips[1].AssetRef != source || clips[1].TimelineStartMS != 6000 || clips[1].TimelineEndMS != 21000 {
		t.Fatalf("prefilled primary timeline clips = %#v", clips)
	}
}

func TestSaveEditTimelineAppendsAnImmutableVersion(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	repository := &memoryEditTaskRepository{}
	asset := contract.AssetVersionRef{AssetID: "asset_source", Version: 1}
	timeline := EditingTimelineV1{SchemaVersion: EditingTimelineSchemaV1, OutputProfile: EditingMVPVerticalOutputProfile, DurationMS: 6000,
		Tracks: []EditingTimelineTrack{{ID: "video-primary", Role: EditingTrackPrimaryVideo, Clips: []EditingTimelineClip{{
			ID: "clip-source", AssetRef: &asset, TimelineEndMS: 6000, SourceOutMS: 6000,
		}}}}}
	service := Service{Projects: testProjects{}, EditTasks: repository,
		Assets: testAssetReader{snapshots: map[contract.AssetID]CreativeAssetSnapshot{
			asset.AssetID: {Ref: asset, Kind: contract.AssetVideo, MIMEType: "video/mp4", Ready: true, DurationMS: 6000},
		}}, NewID: func(prefix string) (string, error) { return prefix + "_1", nil }, Now: func() time.Time { return now }}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeWrite}}
	created, err := service.CreateEditTask(context.Background(), actor, "project_1", CreateEditTaskRequest{DisplayName: "手动剪辑", Timeline: timeline})
	if err != nil {
		t.Fatalf("CreateEditTask() error = %v", err)
	}
	updatedTimeline := timeline
	updatedTimeline.Tracks = append(updatedTimeline.Tracks, EditingTimelineTrack{ID: "caption", Role: EditingTrackCaption, Clips: []EditingTimelineClip{{
		ID: "caption-1", TimelineEndMS: 6000, Text: "前贴结束，正片开始",
	}}})
	updated, err := service.SaveEditTimeline(context.Background(), actor, "project_1", created.ID, SaveEditTimelineRequest{ExpectedVersion: 1, Timeline: updatedTimeline})
	if err != nil {
		t.Fatalf("SaveEditTimeline() error = %v", err)
	}
	if updated.CurrentTimeline.Version != 2 || updated.CurrentTimeline.ContentHash == created.CurrentTimeline.ContentHash {
		t.Fatalf("updated edit task = %#v", updated)
	}
}

func TestCreativeVersionCanEnterEditorButCannotCrossProject(t *testing.T) {
	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	ref := contract.AssetVersionRef{AssetID: "final_brand_video", Version: 2}
	repository := &memoryEditTaskRepository{}
	service := Service{Projects: testProjects{}, EditTasks: repository, Assets: projectBoundEditingAssetReader{project: "project_1", snapshot: CreativeAssetSnapshot{Ref: ref, Kind: contract.AssetVideo, Ready: true, DurationMS: 15000}}, NewID: func(prefix string) (string, error) { return prefix + "_1", nil }, Now: func() time.Time { return now }}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeWrite}}
	created, err := service.CreateCreativeVersionEditTask(context.Background(), actor, "project_1", CreateCreativeVersionEditTaskRequest{SourceCreativeTaskID: "brand_task_1", FinalVideo: ref})
	if err != nil {
		t.Fatal(err)
	}
	if created.EntrySource != EditTaskEntryCreativeVersion || created.CurrentTimeline.Timeline.Tracks[0].Clips[0].AssetRef == nil || *created.CurrentTimeline.Timeline.Tracks[0].Clips[0].AssetRef != ref {
		t.Fatalf("created=%#v", created)
	}
	_, err = service.CreateCreativeVersionEditTask(context.Background(), actor, "project_2", CreateCreativeVersionEditTaskRequest{SourceCreativeTaskID: "brand_task_2", FinalVideo: ref})
	if err == nil {
		t.Fatal("cross-project final video must be rejected")
	}
}

func TestEditTaskReloadsLatestConfirmedTimelineAndRejectsStaleSave(t *testing.T) {
	now := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	ref := contract.AssetVersionRef{AssetID: "source", Version: 1}
	repository := &memoryEditTaskRepository{}
	timeline := EditingTimelineV1{SchemaVersion: EditingTimelineSchemaV1, OutputProfile: EditingMVPVerticalOutputProfile, DurationMS: 6000, Tracks: []EditingTimelineTrack{{ID: "video", Role: EditingTrackPrimaryVideo, Clips: []EditingTimelineClip{{ID: "clip", AssetRef: &ref, TimelineEndMS: 6000, SourceOutMS: 6000}}}}}
	service := Service{Projects: testProjects{}, EditTasks: repository, Assets: testAssetReader{snapshots: map[contract.AssetID]CreativeAssetSnapshot{ref.AssetID: {Ref: ref, Kind: contract.AssetVideo, Ready: true, DurationMS: 6000}}}, NewID: func(prefix string) (string, error) { return prefix + "_1", nil }, Now: func() time.Time { return now }}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeRead, ScopeWrite}}
	created, err := service.CreateEditTask(context.Background(), actor, "project_1", CreateEditTaskRequest{DisplayName: "可恢复剪辑", Timeline: timeline})
	if err != nil {
		t.Fatal(err)
	}
	updated := timeline
	updated.Tracks = append(updated.Tracks, EditingTimelineTrack{ID: "caption", Role: EditingTrackCaption, Clips: []EditingTimelineClip{{ID: "caption-1", TimelineEndMS: 6000, Text: "已确认字幕"}}})
	if _, err = service.SaveEditTimeline(context.Background(), actor, "project_1", created.ID, SaveEditTimelineRequest{ExpectedVersion: 1, Timeline: updated}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.SaveEditTimeline(context.Background(), actor, "project_1", created.ID, SaveEditTimelineRequest{ExpectedVersion: 1, Timeline: timeline}); err != ErrVersionConflict {
		t.Fatalf("stale save error=%v", err)
	}
	reloaded, err := service.GetEditTask(context.Background(), actor, "project_1", created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.CurrentTimeline.Version != 2 || len(reloaded.CurrentTimeline.Timeline.Tracks) != 2 {
		t.Fatalf("reloaded=%#v", reloaded.CurrentTimeline)
	}
}
