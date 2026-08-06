package creative

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const EditTaskContractVersion = "creative-edit-task/v1"

type EditTaskStatus string

const EditTaskDraft EditTaskStatus = "draft"

type EditTaskEntrySource string

const (
	EditTaskEntryManual          EditTaskEntrySource = "manual"
	EditTaskEntryShortDramaV2    EditTaskEntrySource = "short_drama_preroll_v2"
	EditTaskEntryCreativeVersion EditTaskEntrySource = "creative_version"
)

// EditTask is the durable workspace for a generic video edit. It belongs to a
// Project, not to any generator task. A generator can only provide an entry
// source and prefilled asset references.
type EditTask struct {
	ContractVersion      string                  `json:"contract_version"`
	ID                   string                  `json:"id"`
	OrganizationID       contract.OrganizationID `json:"organization_id"`
	ProjectID            contract.ProjectID      `json:"project_id"`
	DisplayName          string                  `json:"display_name"`
	Status               EditTaskStatus          `json:"status"`
	EntrySource          EditTaskEntrySource     `json:"entry_source"`
	SourceCreativeTaskID string                  `json:"source_creative_task_id,omitempty"`
	CurrentTimeline      TimelineVersion         `json:"current_timeline"`
	CreatedBy            string                  `json:"created_by"`
	CreatedAt            time.Time               `json:"created_at"`
	UpdatedAt            time.Time               `json:"updated_at"`
}

type TimelineVersion struct {
	Version     int64             `json:"version"`
	Timeline    EditingTimelineV1 `json:"timeline"`
	ContentHash string            `json:"content_hash"`
	CreatedBy   string            `json:"created_by"`
	CreatedAt   time.Time         `json:"created_at"`
}

type EditTaskRepository interface {
	CreateEditTask(context.Context, EditTask, TimelineVersion) (EditTask, error)
	GetEditTask(context.Context, contract.OrganizationID, contract.ProjectID, string) (EditTask, error)
	FindEditTaskBySource(context.Context, contract.OrganizationID, contract.ProjectID, EditTaskEntrySource, string) (EditTask, error)
	AppendEditTimeline(context.Context, EditTask, int64, TimelineVersion) (EditTask, error)
}

type CreateEditTaskRequest struct {
	DisplayName string            `json:"display_name"`
	Timeline    EditingTimelineV1 `json:"timeline"`
}

type CreateShortDramaV2EditTaskRequest struct {
	SourceCreativeTaskID string                   `json:"source_creative_task_id"`
	PrerollAsset         contract.AssetVersionRef `json:"preroll_asset"`
	SourceAsset          contract.AssetVersionRef `json:"source_asset"`
}

type CreateCreativeVersionEditTaskRequest struct {
	SourceCreativeTaskID string                   `json:"source_creative_task_id"`
	FinalVideo           contract.AssetVersionRef `json:"final_video"`
	DisplayName          string                   `json:"display_name"`
}

type SaveEditTimelineRequest struct {
	ExpectedVersion int64             `json:"expected_version"`
	Timeline        EditingTimelineV1 `json:"timeline"`
}

func (s Service) CreateEditTask(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreateEditTaskRequest) (EditTask, error) {
	return s.createEditTask(ctx, actor, projectID, strings.TrimSpace(request.DisplayName), EditTaskEntryManual, "", request.Timeline)
}

// CreateShortDramaV2EditTask is an idempotent convenience entry: the generated
// pre-roll is placed before the source video, while both remain ordinary
// primary-video clips that the user may subsequently reorder or remove.
func (s Service) CreateShortDramaV2EditTask(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreateShortDramaV2EditTaskRequest) (EditTask, error) {
	if strings.TrimSpace(request.SourceCreativeTaskID) == "" {
		return EditTask{}, fmt.Errorf("source_creative_task_id is required")
	}
	if existing, err := s.getEditTaskBySource(ctx, actor, projectID, EditTaskEntryShortDramaV2, request.SourceCreativeTaskID); err == nil {
		return existing, nil
	} else if err != ErrNotFound {
		return EditTask{}, err
	}
	preroll, source, err := s.readEditableVideos(ctx, actor, projectID, request.PrerollAsset, request.SourceAsset)
	if err != nil {
		return EditTask{}, err
	}
	timeline := EditingTimelineV1{
		SchemaVersion: EditingTimelineSchemaV1, OutputProfile: EditingMVPVerticalOutputProfile,
		DurationMS: int(preroll.DurationMS + source.DurationMS),
		Tracks: []EditingTimelineTrack{{ID: "video-primary", Role: EditingTrackPrimaryVideo, Clips: []EditingTimelineClip{
			{ID: "clip-preroll", AssetRef: &request.PrerollAsset, TimelineStartMS: 0, TimelineEndMS: int(preroll.DurationMS), SourceOutMS: int(preroll.DurationMS)},
			{ID: "clip-source", AssetRef: &request.SourceAsset, TimelineStartMS: int(preroll.DurationMS), TimelineEndMS: int(preroll.DurationMS + source.DurationMS), SourceOutMS: int(source.DurationMS)},
		}}},
	}
	return s.createEditTask(ctx, actor, projectID, "短剧前贴混剪", EditTaskEntryShortDramaV2, request.SourceCreativeTaskID, timeline)
}

func (s Service) CreateCreativeVersionEditTask(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreateCreativeVersionEditTaskRequest) (EditTask, error) {
	if strings.TrimSpace(request.SourceCreativeTaskID) == "" {
		return EditTask{}, fmt.Errorf("source_creative_task_id is required")
	}
	if existing, err := s.getEditTaskBySource(ctx, actor, projectID, EditTaskEntryCreativeVersion, request.SourceCreativeTaskID); err == nil {
		return existing, nil
	} else if err != ErrNotFound {
		return EditTask{}, err
	}
	asset, err := s.Assets.ReadForCreative(ctx, actor, projectID, request.FinalVideo)
	if err != nil {
		return EditTask{}, err
	}
	if asset.Ref != request.FinalVideo || asset.Kind != contract.AssetVideo || !asset.Ready || asset.DurationMS < 1000 {
		return EditTask{}, fmt.Errorf("final_video is unavailable")
	}
	duration := int(asset.DurationMS)
	timeline := EditingTimelineV1{SchemaVersion: EditingTimelineSchemaV1, OutputProfile: EditingMVPVerticalOutputProfile, DurationMS: duration, Tracks: []EditingTimelineTrack{{ID: "video-primary", Role: EditingTrackPrimaryVideo, Clips: []EditingTimelineClip{{ID: "clip-final", AssetRef: &request.FinalVideo, TimelineEndMS: duration, SourceOutMS: duration}}}}}
	name := strings.TrimSpace(request.DisplayName)
	if name == "" {
		name = "广告成片剪辑"
	}
	return s.createEditTask(ctx, actor, projectID, name, EditTaskEntryCreativeVersion, request.SourceCreativeTaskID, timeline)
}

func (s Service) GetEditTask(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string) (EditTask, error) {
	if s.EditTasks == nil || s.Projects == nil {
		return EditTask{}, fmt.Errorf("editing task dependencies are incomplete")
	}
	if !actor.HasScope(ScopeRead) {
		return EditTask{}, fmt.Errorf("%s scope is required", ScopeRead)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return EditTask{}, err
	}
	return s.EditTasks.GetEditTask(ctx, actor.OrganizationID, projectID, taskID)
}

func (s Service) SaveEditTimeline(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request SaveEditTimelineRequest) (EditTask, error) {
	if request.ExpectedVersion < 1 {
		return EditTask{}, ErrVersionConflict
	}
	if err := s.validateEditingDependencies(ctx, actor, projectID, true); err != nil {
		return EditTask{}, err
	}
	task, err := s.EditTasks.GetEditTask(ctx, actor.OrganizationID, projectID, taskID)
	if err != nil {
		return EditTask{}, err
	}
	if task.CurrentTimeline.Version != request.ExpectedVersion {
		return EditTask{}, ErrVersionConflict
	}
	if err := s.validateTimelineAssets(ctx, actor, projectID, request.Timeline); err != nil {
		return EditTask{}, err
	}
	version, err := newTimelineVersion(request.Timeline, request.ExpectedVersion+1, actor.Principal.ID, s.now())
	if err != nil {
		return EditTask{}, err
	}
	return s.EditTasks.AppendEditTimeline(ctx, task, request.ExpectedVersion, version)
}

func (s Service) createEditTask(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, displayName string, source EditTaskEntrySource, sourceCreativeTaskID string, timeline EditingTimelineV1) (EditTask, error) {
	if err := s.validateEditingDependencies(ctx, actor, projectID, true); err != nil {
		return EditTask{}, err
	}
	if len([]rune(displayName)) == 0 || len([]rune(displayName)) > 120 {
		return EditTask{}, fmt.Errorf("edit task display_name must contain 1 to 120 characters")
	}
	if source != EditTaskEntryManual && source != EditTaskEntryShortDramaV2 && source != EditTaskEntryCreativeVersion {
		return EditTask{}, fmt.Errorf("edit task entry source is unsupported")
	}
	if source == EditTaskEntryManual && sourceCreativeTaskID != "" {
		return EditTask{}, fmt.Errorf("manual edit task cannot have source_creative_task_id")
	}
	if err := s.validateTimelineAssets(ctx, actor, projectID, timeline); err != nil {
		return EditTask{}, err
	}
	now := s.now()
	id, err := s.idGenerator()("edit")
	if err != nil {
		return EditTask{}, err
	}
	version, err := newTimelineVersion(timeline, 1, actor.Principal.ID, now)
	if err != nil {
		return EditTask{}, err
	}
	task := EditTask{ContractVersion: EditTaskContractVersion, ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		DisplayName: displayName, Status: EditTaskDraft, EntrySource: source, SourceCreativeTaskID: sourceCreativeTaskID,
		CurrentTimeline: version, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now}
	if err := task.Validate(); err != nil {
		return EditTask{}, err
	}
	return s.EditTasks.CreateEditTask(ctx, task, version)
}

func (s Service) getEditTaskBySource(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, source EditTaskEntrySource, sourceTaskID string) (EditTask, error) {
	if err := s.validateEditingDependencies(ctx, actor, projectID, true); err != nil {
		return EditTask{}, err
	}
	return s.EditTasks.FindEditTaskBySource(ctx, actor.OrganizationID, projectID, source, sourceTaskID)
}

func (s Service) validateEditingDependencies(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, write bool) error {
	if s.EditTasks == nil || s.Projects == nil || s.Assets == nil {
		return fmt.Errorf("editing task dependencies are incomplete")
	}
	if write && !actor.HasScope(ScopeWrite) {
		return fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if !write && !actor.HasScope(ScopeRead) {
		return fmt.Errorf("%s scope is required", ScopeRead)
	}
	_, err := s.Projects.RequireActiveContext(ctx, actor, projectID)
	return err
}

func (s Service) readEditableVideos(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, refs ...contract.AssetVersionRef) (CreativeAssetSnapshot, CreativeAssetSnapshot, error) {
	if len(refs) != 2 {
		return CreativeAssetSnapshot{}, CreativeAssetSnapshot{}, fmt.Errorf("exactly two editable videos are required")
	}
	values := [2]CreativeAssetSnapshot{}
	for index, ref := range refs {
		value, err := s.Assets.ReadForCreative(ctx, actor, projectID, ref)
		if err != nil {
			return CreativeAssetSnapshot{}, CreativeAssetSnapshot{}, err
		}
		if value.Ref != ref || value.Kind != contract.AssetVideo || !value.Ready || value.DurationMS < 1000 {
			return CreativeAssetSnapshot{}, CreativeAssetSnapshot{}, fmt.Errorf("editable video %d is unavailable", index+1)
		}
		values[index] = value
	}
	return values[0], values[1], nil
}

func (s Service) validateTimelineAssets(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, timeline EditingTimelineV1) error {
	if err := timeline.Validate(); err != nil {
		return err
	}
	for _, track := range timeline.Tracks {
		if track.Role == EditingTrackCaption {
			continue
		}
		for _, clip := range track.Clips {
			asset, err := s.Assets.ReadForCreative(ctx, actor, projectID, *clip.AssetRef)
			if err != nil {
				return err
			}
			if asset.Ref != *clip.AssetRef || !asset.Ready || (track.Role == EditingTrackPrimaryVideo && asset.Kind != contract.AssetVideo) {
				return fmt.Errorf("editing timeline clip %s references an unavailable asset", clip.ID)
			}
		}
	}
	return nil
}

func newTimelineVersion(timeline EditingTimelineV1, version int64, actorID string, now time.Time) (TimelineVersion, error) {
	if err := timeline.Validate(); err != nil {
		return TimelineVersion{}, err
	}
	hash, err := contract.CanonicalJSONHash(timeline)
	if err != nil {
		return TimelineVersion{}, err
	}
	value := TimelineVersion{Version: version, Timeline: timeline, ContentHash: "sha256:" + hash, CreatedBy: actorID, CreatedAt: now}
	if err := value.Validate(); err != nil {
		return TimelineVersion{}, err
	}
	return value, nil
}

func (t EditTask) Validate() error {
	if t.ContractVersion != EditTaskContractVersion || strings.TrimSpace(t.ID) == "" || t.OrganizationID == "" || t.ProjectID == "" ||
		len([]rune(strings.TrimSpace(t.DisplayName))) == 0 || len([]rune(t.DisplayName)) > 120 || t.Status != EditTaskDraft ||
		(t.EntrySource != EditTaskEntryManual && t.EntrySource != EditTaskEntryShortDramaV2 && t.EntrySource != EditTaskEntryCreativeVersion) || strings.TrimSpace(t.CreatedBy) == "" ||
		t.CreatedAt.IsZero() || t.UpdatedAt.IsZero() {
		return fmt.Errorf("edit task is incomplete")
	}
	if (t.EntrySource == EditTaskEntryManual) != (t.SourceCreativeTaskID == "") {
		return fmt.Errorf("edit task source linkage is invalid")
	}
	return t.CurrentTimeline.Validate()
}

func (v TimelineVersion) Validate() error {
	if v.Version < 1 || strings.TrimSpace(v.ContentHash) == "" || strings.TrimSpace(v.CreatedBy) == "" || v.CreatedAt.IsZero() {
		return fmt.Errorf("timeline version is incomplete")
	}
	return v.Timeline.Validate()
}
