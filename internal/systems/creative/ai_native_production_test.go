package creative

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
	"github.com/shikanon/cookies/internal/platform/media"
	"github.com/shikanon/cookies/internal/platform/provider"
)

type memoryAINativeProductionRepository struct {
	workspace AINativeRequirementWorkspace
	operation AINativeProductionOperation
}

func TestCompileAINativeTimelineUsesSelectedAssetsAndStoryboardCaptions(t *testing.T) {
	requirement, _ := validAINativeStoryboardInputs()
	storyboard := readyConfirmedAINativeStoryboard()
	plan, err := CompileAINativeProductionPlan(requirement, storyboard, "project_1", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	plan.Status = AINativeProductionReadyStatus
	for index := range plan.Units {
		ref := contract.AssetVersionRef{AssetID: contract.AssetID("video-" + plan.Units[index].ID), Version: 1}
		attempt := AINativeGenerationAttempt{ID: "video-attempt-" + plan.Units[index].ID, Status: AINativeAttemptSucceededStatus, OutputAssetRef: &ref}
		plan.Units[index].Attempts, plan.Units[index].SelectedAttemptID = []AINativeGenerationAttempt{attempt}, attempt.ID
	}
	for index := range plan.SpeechUnits {
		ref := contract.AssetVersionRef{AssetID: contract.AssetID("audio-" + plan.SpeechUnits[index].ID), Version: 1}
		attempt := AINativeGenerationAttempt{ID: "speech-attempt-" + plan.SpeechUnits[index].ID, Status: AINativeAttemptSucceededStatus, OutputAssetRef: &ref}
		plan.SpeechUnits[index].Attempts, plan.SpeechUnits[index].SelectedAttemptID = []AINativeGenerationAttempt{attempt}, attempt.ID
	}
	timeline, err := CompileAINativeTimeline(plan, storyboard, "org_1", "project_1")
	if err != nil {
		t.Fatal(err)
	}
	if timeline.DurationMS != 20000 || len(timeline.Video) != 4 || len(timeline.Audio) != len(plan.SpeechUnits) || len(timeline.Captions) != len(storyboard.Shots) {
		t.Fatalf("unexpected final timeline: %#v", timeline)
	}
	if timeline.Audio[0].Role != media.TimelineAudioVoiceover || timeline.Video[0].Asset.AssetID != "video-video-unit-01" || timeline.Captions[0].Text != storyboard.Shots[0].Subtitle {
		t.Fatalf("timeline lost frozen production lineage: %#v", timeline)
	}
}

func (r *memoryAINativeProductionRepository) GetAINativeProductionWorkspace(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _ string) (AINativeRequirementWorkspace, error) {
	return r.workspace, nil
}

func (r *memoryAINativeProductionRepository) BeginAINativeProduction(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _ string, operation AINativeProductionOperation, plan AINativeProductionPlan, now time.Time) (AINativeRequirementWorkspace, error) {
	if r.workspace.WorkspaceVersion != operation.ExpectedWorkspaceVersion {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	r.operation = operation
	status := AINativeProductionRunningStatus
	if plan.Status == AINativeProductionRenderingStatus {
		status = AINativeProductionRenderingStatus
	}
	r.workspace.CurrentStage, r.workspace.ProductionStatus = AINativeStageProduction, status
	r.workspace.ProductionPlan, r.workspace.ActiveOperationID, r.workspace.ActiveOperationVersion = &plan, operation.ID, &operation.Version
	r.workspace.WorkspaceVersion++
	r.workspace.UpdatedAt = now
	return r.workspace, nil
}

func (r *memoryAINativeProductionRepository) SaveAINativeProductionPlan(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _ string, _ AINativeProductionOperation, plan AINativeProductionPlan, _ time.Time) (AINativeRequirementWorkspace, error) {
	r.workspace.ProductionPlan, r.workspace.ProductionStatus = &plan, plan.Status
	if plan.Status == AINativeProductionReadyStatus || plan.Status == AINativeProductionCompletedStatus || plan.Status == AINativeProductionRenderFailedStatus || plan.Status == AINativeProductionFailedStatus {
		r.workspace.ActiveOperationID, r.workspace.ActiveOperationVersion = "", nil
	}
	return r.workspace, nil
}

func (r *memoryAINativeProductionRepository) CancelAINativeProduction(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _ string, _ int64, _ time.Time) (AINativeRequirementWorkspace, error) {
	r.workspace.ProductionStatus = AINativeProductionCancelledStatus
	r.workspace.ActiveOperationID, r.workspace.ActiveOperationVersion = "", nil
	return r.workspace, nil
}

type productionSchedulerStub struct{ operation AINativeProductionOperation }

func (s *productionSchedulerStub) ScheduleAINativeProduction(_ context.Context, operation AINativeProductionOperation) error {
	s.operation = operation
	return nil
}

type productionVideoJobsStub struct {
	created       int
	sourceTaskIDs []string
	createErr     error
}

func (s *productionVideoJobsStub) CreateVideoJob(_ context.Context, request provider.CreateVideoJobRequest) (contract.ProviderJob, bool, error) {
	s.created++
	s.sourceTaskIDs = append(s.sourceTaskIDs, request.SourceTaskID)
	if len(request.SourceTaskID) > 96 {
		return contract.ProviderJob{}, false, errors.New("source task id exceeds provider storage limit")
	}
	if s.createErr != nil {
		return contract.ProviderJob{}, false, s.createErr
	}
	return contract.ProviderJob{ID: "provider-job-" + request.SourceTaskID, OrganizationID: request.Actor.OrganizationID, ProjectID: request.Project.ProjectID, ProviderStatus: contract.ProviderJobSubmitted}, false, nil
}

func (s *productionVideoJobsStub) GetJob(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, jobID string) (contract.ProviderJob, error) {
	return contract.ProviderJob{ID: jobID, OrganizationID: organizationID, ProjectID: projectID, ProviderStatus: contract.ProviderJobSucceeded,
		ProjectAssetRefs: []contract.ProjectAssetRef{{ProjectID: projectID, AssetVersion: contract.AssetVersionRef{AssetID: contract.AssetID("video-" + jobID), Version: 1}}}}, nil
}

type productionSpeechStub struct{ calls int }

func (s *productionSpeechStub) Synthesize(_ context.Context, input provider.SpeechSynthesisInput) (provider.SpeechSynthesisResult, error) {
	s.calls++
	return provider.SpeechSynthesisResult{Audio: []byte("mp3-audio"), Codec: "mp3", SampleRate: 24000, DurationMS: 500, OriginalText: input.Text, NormalizedText: input.Text, ProviderRequestID: "speech-request"}, nil
}

type durationFittingSpeechStub struct {
	calls []provider.SpeechSynthesisInput
}

func (s *durationFittingSpeechStub) Synthesize(_ context.Context, input provider.SpeechSynthesisInput) (provider.SpeechSynthesisResult, error) {
	s.calls = append(s.calls, input)
	duration := 5000
	if input.SpeakingRate > 1 {
		duration = 4000
	}
	return provider.SpeechSynthesisResult{Audio: []byte("mp3-audio"), Codec: "mp3", SampleRate: 24000, DurationMS: duration,
		OriginalText: input.Text, NormalizedText: input.Text, ProviderRequestID: "speech-request"}, nil
}

type productionAudioWriterStub struct{ calls int }

func (s *productionAudioWriterStub) IngestDerivedAudio(_ context.Context, _ contract.RequestContext, projectID contract.ProjectID, derivationID string, content io.Reader, _ int64, _ string, _ []contract.ResourceRef) (contract.ProjectAssetRef, error) {
	s.calls++
	_, _ = io.ReadAll(content)
	return contract.ProjectAssetRef{ProjectID: projectID, AssetVersion: contract.AssetVersionRef{AssetID: contract.AssetID("audio-" + derivationID), Version: 1}}, nil
}

type productionTimelineRendererStub struct{ calls int }

func (s *productionTimelineRendererStub) Render(_ context.Context, _ media.TimelineRenderRequest, report media.TimelineProgressFunc) (media.CompositionOutput, error) {
	s.calls++
	if err := report(media.TimelineProgress{Percent: 50, OutTimeMS: 10000}); err != nil {
		return media.CompositionOutput{}, err
	}
	return media.CompositionOutput{Content: io.NopCloser(strings.NewReader("final-mp4")), SizeBytes: 9}, nil
}

type productionRenderedWriterStub struct{ calls int }

func (s *productionRenderedWriterStub) IngestRenderedVideo(_ context.Context, _ contract.RequestContext, projectID contract.ProjectID, _ string, content io.Reader, _ int64) (contract.ProjectAssetRef, error) {
	s.calls++
	_, _ = io.ReadAll(content)
	return contract.ProjectAssetRef{ProjectID: projectID, AssetVersion: contract.AssetVersionRef{AssetID: "final-video", Version: 1}}, nil
}

func TestCompileAINativeProductionPlanMapsConfirmedStoryboardToProviderUnits(t *testing.T) {
	requirement, _ := validAINativeStoryboardInputs()
	storyboard := readyConfirmedAINativeStoryboard()
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)

	plan, err := CompileAINativeProductionPlan(requirement, storyboard, contract.ProjectID("project-1"), now)
	if err != nil {
		t.Fatal(err)
	}

	wantDurations := []int{4, 6, 5, 5}
	if len(plan.Units) != len(wantDurations) {
		t.Fatalf("generation units = %d, want %d: %#v", len(plan.Units), len(wantDurations), plan.Units)
	}
	for index, unit := range plan.Units {
		if unit.DurationSeconds != wantDurations[index] {
			t.Fatalf("unit %d duration = %d, want %d", index, unit.DurationSeconds, wantDurations[index])
		}
		if unit.StartMS != []int{0, 4000, 10000, 15000}[index] || unit.EndMS != []int{4000, 10000, 15000, 20000}[index] {
			t.Fatalf("unit %d has unexpected timeline: %#v", index, unit)
		}
		input, err := unit.ProviderInput(contract.ProjectID("project-1"))
		if err != nil {
			t.Fatalf("unit %d provider input: %v", index, err)
		}
		if input.AudioPolicy != provider.VideoAudioSilent {
			t.Fatalf("unit %d must generate silent video", index)
		}
		if input.InputMode != provider.VideoInputReferenceImage || len(input.ConditioningAssets) != 1 || input.ConditioningAssets[0].Reference.AssetVersion.AssetID != "asset_product" {
			t.Fatalf("unit %d lost product identity conditioning: %#v", index, input)
		}
	}
	if len(plan.SpeechUnits) != 3 || plan.TotalDurationMS != 20000 {
		t.Fatalf("unexpected production plan totals: %#v", plan)
	}
	for _, speech := range plan.SpeechUnits {
		if speech.VoiceAlias != "cookies.voice.douyin.default" {
			t.Fatalf("speech unit %s exposed non-portable provider voice %q", speech.ID, speech.VoiceAlias)
		}
	}
}

func TestCompileAINativeProductionPlanDoesNotReusePersonIdentityAsVideoConditioning(t *testing.T) {
	requirement, _ := validAINativeStoryboardInputs()
	storyboard := readyConfirmedAINativeStoryboard()
	storyboard.Shots[0].ProductIdentityRequired = false
	storyboard.Shots[0].ReferenceAssetIDs = []string{"person_1", "scene_1"}
	storyboard.Shots[1].ProductIdentityRequired = false
	storyboard.Shots[1].ReferenceAssetIDs = []string{"person_1"}

	plan, err := CompileAINativeProductionPlan(requirement, storyboard, contract.ProjectID("project-1"), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range plan.Units {
		if unit.ReferenceRole == AINativeStoryboardAssetRolePersonIdentity {
			t.Fatalf("unit %s passed reusable person identity directly to the video provider: %#v", unit.ID, unit)
		}
	}
	if plan.Units[0].ReferenceRole != AINativeStoryboardAssetRoleSceneReference {
		t.Fatalf("first non-product shot did not prefer its scene reference: %#v", plan.Units[0])
	}
	for _, unit := range plan.Units {
		if unit.ShotIDs[0] == storyboard.Shots[1].ID && unit.ReferenceAsset != nil {
			t.Fatalf("person-only shot should safely fall back to text-only generation: %#v", unit)
		}
	}
}

func TestStartAINativeProductionFreezesPlanAndCreatesRecoverableAttempts(t *testing.T) {
	requirement, script := validAINativeStoryboardInputs()
	storyboard := readyConfirmedAINativeStoryboard()
	storyboardRevision := int64(1)
	workspace := AINativeRequirementWorkspace{WorkspaceID: "workspace-1", CreativeIntakeID: "intake-1", CreativeTaskID: "task-1", OrganizationID: "org-1", ProjectID: "project_1",
		Status: AINativeRequirementConfirmedStatus, CurrentStage: AINativeStageStoryboard, WorkspaceVersion: 7, CurrentRevision: 1, ConfirmedRevision: &storyboardRevision,
		Requirement: requirement, ScriptStatus: AINativeScriptConfirmedStatus, CurrentScriptRevision: &storyboardRevision, ConfirmedScriptRevision: &storyboardRevision, Script: &script,
		StoryboardStatus: AINativeStoryboardConfirmedStatus, CurrentStoryboardRevision: &storyboardRevision, ConfirmedStoryboardRevision: &storyboardRevision, Storyboard: &storyboard,
		CreatedBy: "user-1", ConfirmedBy: "user-1", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	repository := &memoryAINativeProductionRepository{workspace: workspace}
	scheduler := &productionSchedulerStub{}
	sequence := 0
	service := Service{Projects: testProjects{}, AINativeProductions: repository, AINativeProductionScheduler: scheduler,
		NewID: func(prefix string) (string, error) { sequence++; return prefix + "-" + string(rune('a'+sequence)), nil }, Now: func() time.Time { return time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC) }}
	actor := contract.ActorContext{OrganizationID: "org-1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user-1"}, Scopes: []contract.Scope{ScopeWrite}}

	updated, err := service.StartAINativeProduction(context.Background(), actor, "project_1", "workspace-1", StartAINativeProductionRequest{ExpectedWorkspaceVersion: 7})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ProductionPlan == nil || updated.ProductionStatus != AINativeProductionRunningStatus || scheduler.operation.ID == "" {
		t.Fatalf("production was not started: %#v", updated)
	}
	for _, unit := range updated.ProductionPlan.Units {
		if len(unit.Attempts) != 1 || unit.Attempts[0].Status != AINativeAttemptPlannedStatus {
			t.Fatalf("video unit has no recoverable initial attempt: %#v", unit)
		}
	}
	for _, unit := range updated.ProductionPlan.SpeechUnits {
		if len(unit.Attempts) != 1 || unit.Attempts[0].Status != AINativeAttemptPlannedStatus {
			t.Fatalf("speech unit has no recoverable initial attempt: %#v", unit)
		}
	}
}

func TestStartAINativeProductionReusesSuccessfulUnitsAfterOneStoryboardAssetChanges(t *testing.T) {
	requirement, script := validAINativeStoryboardInputs()
	storyboard := readyConfirmedAINativeStoryboard()
	storyboard.Shots[0].ProductIdentityRequired = false
	storyboard.Shots[0].ReferenceAssetIDs = []string{"scene_1"}
	previous, err := CompileAINativeProductionPlan(requirement, storyboard, "project_1", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	for index := range previous.Units {
		output := contract.AssetVersionRef{AssetID: contract.AssetID(fmt.Sprintf("old-video-%d", index)), Version: 1}
		attempt := AINativeGenerationAttempt{ID: fmt.Sprintf("old-video-attempt-%d", index), Status: AINativeAttemptSucceededStatus, OutputAssetRef: &output}
		previous.Units[index].Attempts, previous.Units[index].SelectedAttemptID = []AINativeGenerationAttempt{attempt}, attempt.ID
	}
	for index := range previous.SpeechUnits {
		output := contract.AssetVersionRef{AssetID: contract.AssetID(fmt.Sprintf("old-audio-%d", index)), Version: 1}
		attempt := AINativeGenerationAttempt{ID: fmt.Sprintf("old-speech-attempt-%d", index), Status: AINativeAttemptSucceededStatus, OutputAssetRef: &output}
		previous.SpeechUnits[index].Attempts, previous.SpeechUnits[index].SelectedAttemptID = []AINativeGenerationAttempt{attempt}, attempt.ID
	}
	changedReference := previous.Units[0].ReferenceAsset
	if changedReference == nil {
		t.Fatal("fixture first unit must use a storyboard reference")
	}
	for index := range storyboard.Assets {
		if storyboard.Assets[index].AssetRef != nil && storyboard.Assets[index].AssetRef.AssetID == changedReference.AssetID {
			replaced := *storyboard.Assets[index].AssetRef
			replaced.Version++
			storyboard.Assets[index].AssetRef = &replaced
		}
	}
	revision := int64(1)
	productionRevision := int64(1)
	workspace := AINativeRequirementWorkspace{WorkspaceID: "workspace-1", OrganizationID: "org-1", ProjectID: "project_1",
		Status: AINativeRequirementConfirmedStatus, CurrentStage: AINativeStageStoryboard, WorkspaceVersion: 7, CurrentRevision: 1, ConfirmedRevision: &revision,
		Requirement: requirement, ScriptStatus: AINativeScriptConfirmedStatus, CurrentScriptRevision: &revision, ConfirmedScriptRevision: &revision, Script: &script,
		StoryboardStatus: AINativeStoryboardConfirmedStatus, CurrentStoryboardRevision: &revision, ConfirmedStoryboardRevision: &revision, Storyboard: &storyboard,
		ProductionStatus: AINativeProductionCancelledStatus, CurrentProductionRevision: &productionRevision, ProductionPlan: &previous,
		CreatedBy: "user-1", ConfirmedBy: "user-1", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	repository := &memoryAINativeProductionRepository{workspace: workspace}
	sequence := 0
	service := Service{Projects: testProjects{}, AINativeProductions: repository, AINativeProductionScheduler: &productionSchedulerStub{},
		NewID: func(prefix string) (string, error) { sequence++; return fmt.Sprintf("%s-%d", prefix, sequence), nil }, Now: func() time.Time { return time.Date(2026, 8, 7, 15, 0, 0, 0, time.UTC) }}
	actor := contract.ActorContext{OrganizationID: "org-1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user-1"}, Scopes: []contract.Scope{ScopeWrite}}

	updated, err := service.StartAINativeProduction(context.Background(), actor, "project_1", "workspace-1", StartAINativeProductionRequest{ExpectedWorkspaceVersion: 7})
	if err != nil {
		t.Fatal(err)
	}
	reusedVideo := 0
	for _, unit := range updated.ProductionPlan.Units {
		usesChangedReference := unit.ReferenceAsset != nil && unit.ReferenceAsset.AssetID == changedReference.AssetID
		if usesChangedReference {
			if unit.SelectedAttemptID != "" || len(unit.Attempts) != 1 || unit.Attempts[0].Status != AINativeAttemptPlannedStatus {
				t.Fatalf("changed unit was not freshly planned: %#v", unit)
			}
			continue
		}
		if unit.SelectedAttemptID == "" {
			t.Fatalf("unchanged unit was not reused: %#v", unit)
		}
		reusedVideo++
	}
	if reusedVideo == 0 {
		t.Fatal("fixture must contain at least one video unit unaffected by the replaced asset")
	}
	for _, unit := range updated.ProductionPlan.SpeechUnits {
		if unit.SelectedAttemptID == "" {
			t.Fatalf("unchanged speech was not reused: %#v", unit)
		}
	}
}

func TestAINativeProductionJobResumesWithoutRepeatingSucceededSpeechOrVideoSubmissions(t *testing.T) {
	requirement, script := validAINativeStoryboardInputs()
	storyboard := readyConfirmedAINativeStoryboard()
	revision := int64(1)
	repository := &memoryAINativeProductionRepository{workspace: AINativeRequirementWorkspace{WorkspaceID: "workspace-1", CreativeIntakeID: "intake-1", CreativeTaskID: "task-1", OrganizationID: "org-1", ProjectID: "project_1",
		Status: AINativeRequirementConfirmedStatus, CurrentStage: AINativeStageStoryboard, WorkspaceVersion: 7, CurrentRevision: 1, ConfirmedRevision: &revision, Requirement: requirement,
		ScriptStatus: AINativeScriptConfirmedStatus, CurrentScriptRevision: &revision, ConfirmedScriptRevision: &revision, Script: &script,
		StoryboardStatus: AINativeStoryboardConfirmedStatus, CurrentStoryboardRevision: &revision, ConfirmedStoryboardRevision: &revision, Storyboard: &storyboard,
		CreatedBy: "user-1", ConfirmedBy: "user-1", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}}
	scheduler, videos, speech, audio := &productionSchedulerStub{}, &productionVideoJobsStub{}, &productionSpeechStub{}, &productionAudioWriterStub{}
	timeline, rendered := &productionTimelineRendererStub{}, &productionRenderedWriterStub{}
	sequence := 0
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	service := Service{Projects: testProjects{}, AINativeProductions: repository, AINativeProductionScheduler: scheduler, AINativeVideoJobs: videos, AINativeSpeech: speech, AudioAssets: audio,
		AINativeTimelineRenderer: timeline, RenderedAssets: rendered,
		NewID: func(prefix string) (string, error) { sequence++; return prefix + "-" + string(rune('a'+sequence)), nil }, Now: func() time.Time { return now }}
	actor := contract.ActorContext{OrganizationID: "org-1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user-1"}, Scopes: []contract.Scope{ScopeWrite, provider.ScopeJobCreate, "assets.write"}}
	if _, err := service.StartAINativeProduction(context.Background(), actor, "project_1", "workspace-1", StartAINativeProductionRequest{ExpectedWorkspaceVersion: 7}); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(AINativeProductionJobPayload{Operation: scheduler.operation})
	claim := jobruntime.Claim{Job: contract.Job{Kind: AINativeProductionJobKind, OrganizationID: "org-1", ProjectID: "project_1"}, Payload: payload}
	if _, err := service.HandleAINativeProductionJob(context.Background(), claim); err == nil {
		t.Fatal("first pass should defer while video provider jobs are running")
	} else {
		var deferred jobruntime.DeferredError
		if !errors.As(err, &deferred) {
			t.Fatalf("first pass error = %T %v", err, err)
		}
	}
	if speech.calls != 3 || audio.calls != 3 || videos.created != 2 {
		t.Fatalf("unexpected first pass work: speech=%d audio=%d video=%d", speech.calls, audio.calls, videos.created)
	}
	if _, err := service.HandleAINativeProductionJob(context.Background(), claim); err == nil {
		t.Fatal("second pass should submit remaining video units and defer")
	}
	if _, err := service.HandleAINativeProductionJob(context.Background(), claim); err != nil {
		t.Fatalf("third pass should reconcile all remaining units: %v", err)
	}
	if repository.workspace.ProductionStatus != AINativeProductionCompletedStatus || repository.workspace.ProductionPlan.Render == nil || repository.workspace.ProductionPlan.Render.OutputAssetRef == nil || speech.calls != 3 || audio.calls != 3 || videos.created != 4 || timeline.calls != 1 || rendered.calls != 1 {
		t.Fatalf("resume repeated successful work or did not finish: workspace=%#v speech=%d audio=%d video=%d", repository.workspace, speech.calls, audio.calls, videos.created)
	}
	repository.workspace.ProductionStatus = AINativeProductionRenderFailedStatus
	repository.workspace.ProductionPlan.Status = AINativeProductionRenderFailedStatus
	repository.workspace.ProductionPlan.Render.Status = AINativeProductionRenderFailedStatus
	repository.workspace.ProductionPlan.Render.OutputAssetRef = nil
	renderID := repository.workspace.ProductionPlan.Render.ID
	retried, err := service.RetryAINativeProductionUnit(context.Background(), actor, "project_1", "workspace-1", RetryAINativeProductionUnitRequest{ExpectedWorkspaceVersion: repository.workspace.WorkspaceVersion, UnitID: renderID})
	if err != nil {
		t.Fatal(err)
	}
	if retried.ProductionStatus != AINativeProductionRenderingStatus || retried.ProductionPlan.Render.ProgressPercent != 0 {
		t.Fatalf("final render retry did not preserve successful units: %#v", retried)
	}
}

func TestAINativeProductionJobAutomaticallyFitsSlightlyLongSpeechToItsShot(t *testing.T) {
	requirement, script := validAINativeStoryboardInputs()
	storyboard := readyConfirmedAINativeStoryboard()
	revision := int64(1)
	repository := &memoryAINativeProductionRepository{workspace: AINativeRequirementWorkspace{WorkspaceID: "workspace-1", CreativeIntakeID: "intake-1", CreativeTaskID: "task-1", OrganizationID: "org-1", ProjectID: "project_1",
		Status: AINativeRequirementConfirmedStatus, CurrentStage: AINativeStageStoryboard, WorkspaceVersion: 7, CurrentRevision: 1, ConfirmedRevision: &revision, Requirement: requirement,
		ScriptStatus: AINativeScriptConfirmedStatus, CurrentScriptRevision: &revision, ConfirmedScriptRevision: &revision, Script: &script,
		StoryboardStatus: AINativeStoryboardConfirmedStatus, CurrentStoryboardRevision: &revision, ConfirmedStoryboardRevision: &revision, Storyboard: &storyboard,
		CreatedBy: "user-1", ConfirmedBy: "user-1", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}}
	scheduler, videos := &productionSchedulerStub{}, &productionVideoJobsStub{}
	speech, audio := &durationFittingSpeechStub{}, &productionAudioWriterStub{}
	sequence := 0
	service := Service{Projects: testProjects{}, AINativeProductions: repository, AINativeProductionScheduler: scheduler, AINativeVideoJobs: videos,
		AINativeSpeech: speech, AudioAssets: audio,
		NewID: func(prefix string) (string, error) { sequence++; return prefix + "-" + string(rune('a'+sequence)), nil }, Now: func() time.Time { return time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC) }}
	actor := contract.ActorContext{OrganizationID: "org-1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user-1"}, Scopes: []contract.Scope{ScopeWrite, provider.ScopeJobCreate, "assets.write"}}
	if _, err := service.StartAINativeProduction(context.Background(), actor, "project_1", "workspace-1", StartAINativeProductionRequest{ExpectedWorkspaceVersion: 7}); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(AINativeProductionJobPayload{Operation: scheduler.operation})
	claim := jobruntime.Claim{Job: contract.Job{Kind: AINativeProductionJobKind, OrganizationID: "org-1", ProjectID: "project_1"}, Payload: payload}
	_, err := service.HandleAINativeProductionJob(context.Background(), claim)
	var deferred jobruntime.DeferredError
	if !errors.As(err, &deferred) {
		t.Fatalf("first pass error = %T %v, want DeferredError while video jobs run", err, err)
	}
	if len(speech.calls) != 4 || speech.calls[0].SpeakingRate != 1 || speech.calls[1].SpeakingRate <= 1 {
		t.Fatalf("speech fitting calls = %#v, want one normal-rate call followed by one faster retry", speech.calls)
	}
	first := repository.workspace.ProductionPlan.SpeechUnits[0]
	if first.SelectedAttemptID == "" || first.Attempts[0].Status != AINativeAttemptSucceededStatus || first.DurationMS != 4000 {
		t.Fatalf("slightly long speech was not fitted and retained as a successful unit: %#v", first)
	}
}

func TestAINativeProductionJobUsesProviderSafeSourceTaskIDsForLongWorkspaces(t *testing.T) {
	requirement, script := validAINativeStoryboardInputs()
	storyboard := readyConfirmedAINativeStoryboard()
	revision := int64(1)
	workspaceID := "ainativeworkspace_784e703c2a0d8b02c88bb684062917f4"
	repository := &memoryAINativeProductionRepository{workspace: AINativeRequirementWorkspace{WorkspaceID: workspaceID, CreativeIntakeID: "intake-1", CreativeTaskID: "task-1", OrganizationID: "org-1", ProjectID: "project_1",
		Status: AINativeRequirementConfirmedStatus, CurrentStage: AINativeStageStoryboard, WorkspaceVersion: 7, CurrentRevision: 1, ConfirmedRevision: &revision, Requirement: requirement,
		ScriptStatus: AINativeScriptConfirmedStatus, CurrentScriptRevision: &revision, ConfirmedScriptRevision: &revision, Script: &script,
		StoryboardStatus: AINativeStoryboardConfirmedStatus, CurrentStoryboardRevision: &revision, ConfirmedStoryboardRevision: &revision, Storyboard: &storyboard,
		CreatedBy: "user-1", ConfirmedBy: "user-1", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}}
	scheduler, videos := &productionSchedulerStub{}, &productionVideoJobsStub{}
	sequence := 0
	now := time.Date(2026, 8, 5, 8, 0, 0, 0, time.UTC)
	service := Service{Projects: testProjects{}, AINativeProductions: repository, AINativeProductionScheduler: scheduler, AINativeVideoJobs: videos,
		AINativeSpeech: &productionSpeechStub{}, AudioAssets: &productionAudioWriterStub{}, AINativeMaxActiveUnits: 1,
		NewID: func(prefix string) (string, error) { sequence++; return prefix + "_" + strings.Repeat("a", 32), nil }, Now: func() time.Time { return now }}
	actor := contract.ActorContext{OrganizationID: "org-1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user-1"}, Scopes: []contract.Scope{ScopeWrite, provider.ScopeJobCreate, "assets.write"}}
	if _, err := service.StartAINativeProduction(context.Background(), actor, "project_1", workspaceID, StartAINativeProductionRequest{ExpectedWorkspaceVersion: 7}); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(AINativeProductionJobPayload{Operation: scheduler.operation})
	claim := jobruntime.Claim{Job: contract.Job{Kind: AINativeProductionJobKind, OrganizationID: "org-1", ProjectID: "project_1"}, Payload: payload}
	if _, err := service.HandleAINativeProductionJob(context.Background(), claim); err == nil {
		t.Fatal("first pass should defer while the submitted video is running")
	} else {
		var deferred jobruntime.DeferredError
		if !errors.As(err, &deferred) {
			t.Fatalf("first pass error = %T %v, want DeferredError", err, err)
		}
	}
	if len(videos.sourceTaskIDs) != 1 || len(videos.sourceTaskIDs[0]) > 96 {
		t.Fatalf("provider source task ids = %#v, want one id of at most 96 characters", videos.sourceTaskIDs)
	}
}

func TestAINativeProductionJobPersistsVideoSubmissionFailureForRetry(t *testing.T) {
	requirement, script := validAINativeStoryboardInputs()
	storyboard := readyConfirmedAINativeStoryboard()
	revision := int64(1)
	repository := &memoryAINativeProductionRepository{workspace: AINativeRequirementWorkspace{WorkspaceID: "workspace-1", CreativeIntakeID: "intake-1", CreativeTaskID: "task-1", OrganizationID: "org-1", ProjectID: "project_1",
		Status: AINativeRequirementConfirmedStatus, CurrentStage: AINativeStageStoryboard, WorkspaceVersion: 7, CurrentRevision: 1, ConfirmedRevision: &revision, Requirement: requirement,
		ScriptStatus: AINativeScriptConfirmedStatus, CurrentScriptRevision: &revision, ConfirmedScriptRevision: &revision, Script: &script,
		StoryboardStatus: AINativeStoryboardConfirmedStatus, CurrentStoryboardRevision: &revision, ConfirmedStoryboardRevision: &revision, Storyboard: &storyboard,
		CreatedBy: "user-1", ConfirmedBy: "user-1", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}}
	scheduler := &productionSchedulerStub{}
	videos := &productionVideoJobsStub{createErr: errors.New("provider rejected video request")}
	sequence := 0
	now := time.Date(2026, 8, 5, 8, 10, 0, 0, time.UTC)
	service := Service{Projects: testProjects{}, AINativeProductions: repository, AINativeProductionScheduler: scheduler, AINativeVideoJobs: videos,
		AINativeSpeech: &productionSpeechStub{}, AudioAssets: &productionAudioWriterStub{}, AINativeMaxActiveUnits: 1,
		NewID: func(prefix string) (string, error) { sequence++; return prefix + "-" + string(rune('a'+sequence)), nil }, Now: func() time.Time { return now }}
	actor := contract.ActorContext{OrganizationID: "org-1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user-1"}, Scopes: []contract.Scope{ScopeWrite, provider.ScopeJobCreate, "assets.write"}}
	if _, err := service.StartAINativeProduction(context.Background(), actor, "project_1", "workspace-1", StartAINativeProductionRequest{ExpectedWorkspaceVersion: 7}); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(AINativeProductionJobPayload{Operation: scheduler.operation})
	claim := jobruntime.Claim{Job: contract.Job{Kind: AINativeProductionJobKind, OrganizationID: "org-1", ProjectID: "project_1"}, Payload: payload}
	_, err := service.HandleAINativeProductionJob(context.Background(), claim)
	var execution jobruntime.ExecutionError
	if !errors.As(err, &execution) || execution.JobError.Code != "AI_NATIVE_VIDEO_SUBMISSION_FAILED" {
		t.Fatalf("job error = %T %v, want AI_NATIVE_VIDEO_SUBMISSION_FAILED", err, err)
	}
	failed := repository.workspace
	if failed.ProductionStatus != AINativeProductionFailedStatus || failed.ActiveOperationID != "" {
		t.Fatalf("workspace remained active after terminal submission failure: %#v", failed)
	}
	attempt := failed.ProductionPlan.Units[0].Attempts[len(failed.ProductionPlan.Units[0].Attempts)-1]
	if attempt.Status != AINativeAttemptFailedStatus || attempt.ErrorCode != "AI_NATIVE_VIDEO_SUBMISSION_FAILED" || !strings.Contains(attempt.ErrorMessage, "provider rejected") {
		t.Fatalf("failed attempt did not retain actionable error: %#v", attempt)
	}
}

func TestAINativeProductionProgressUsesCompletedDurationAndPreservesSuccessfulUnits(t *testing.T) {
	requirement, _ := validAINativeStoryboardInputs()
	plan, err := CompileAINativeProductionPlan(requirement, readyConfirmedAINativeStoryboard(), contract.ProjectID("project-1"), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	plan.Status = AINativeProductionRunningStatus
	output := contract.AssetVersionRef{AssetID: "video-asset-1", Version: 1}
	plan.Units[0].Attempts = []AINativeGenerationAttempt{{ID: "attempt-1", Ordinal: 1, Status: AINativeAttemptSucceededStatus, OutputAssetRef: &output}}
	plan.Units[0].SelectedAttemptID = "attempt-1"
	plan.Units[1].Attempts = []AINativeGenerationAttempt{{ID: "attempt-2", Ordinal: 1, Status: AINativeAttemptFailedStatus}}

	progress := plan.Progress(time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC))
	if progress.CompletedVideoDurationMS != 4000 || progress.CompletedVideoUnits != 1 || progress.ProgressPercent != 16 {
		t.Fatalf("unexpected weighted progress: %#v", progress)
	}

	retried, err := plan.RetryUnit("video-unit-02", "attempt-3", time.Date(2026, 8, 4, 10, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(retried.Units[0].Attempts) != 1 || retried.Units[0].SelectedAttemptID != "attempt-1" {
		t.Fatalf("retry changed successful unit: %#v", retried.Units[0])
	}
	if len(retried.Units[1].Attempts) != 2 || retried.Units[1].Attempts[1].RetryOf != "attempt-2" {
		t.Fatalf("retry did not append one local attempt: %#v", retried.Units[1])
	}
}

func TestAINativeProductionRetryDropsPrivacyRejectedPersonReference(t *testing.T) {
	now := time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)
	personRef := contract.AssetVersionRef{AssetID: "person-asset", Version: 1}
	plan := AINativeProductionPlan{Status: AINativeProductionFailedStatus, Units: []AINativeGenerationUnit{{
		ID: "video-unit-01", ReferenceAsset: &personRef, ReferenceRole: AINativeStoryboardAssetRolePersonIdentity, ProductIdentityRequired: false,
		Attempts: []AINativeGenerationAttempt{{ID: "attempt-1", Ordinal: 1, Status: AINativeAttemptFailedStatus,
			ErrorCode: "InputImageSensitiveContentDetected.PrivacyInformation", ErrorMessage: "input image may contain real person"}},
	}}}

	retried, err := plan.RetryUnit("video-unit-01", "attempt-2", now)
	if err != nil {
		t.Fatal(err)
	}
	unit := retried.Units[0]
	if unit.ReferenceAsset != nil || unit.ReferenceRole != "" {
		t.Fatalf("privacy-rejected person reference was reused: %#v", unit)
	}
	if len(unit.Attempts) != 2 || unit.Attempts[1].Status != AINativeAttemptPlannedStatus || unit.Attempts[1].RetryOf != "attempt-1" {
		t.Fatalf("retry attempt was not appended correctly: %#v", unit.Attempts)
	}
}

func TestReuseCompatibleProductionOutputsKeepsOnlySemanticallyUnchangedSuccesses(t *testing.T) {
	now := time.Date(2026, 8, 7, 15, 0, 0, 0, time.UTC)
	oldScene := contract.AssetVersionRef{AssetID: "scene-asset", Version: 1}
	newScene := contract.AssetVersionRef{AssetID: "scene-asset", Version: 2}
	product := contract.AssetVersionRef{AssetID: "product-asset", Version: 1}
	videoA := contract.AssetVersionRef{AssetID: "video-a", Version: 1}
	videoB := contract.AssetVersionRef{AssetID: "video-b", Version: 1}
	audio := contract.AssetVersionRef{AssetID: "audio-a", Version: 1}
	previous := AINativeProductionPlan{
		Status: AINativeProductionCompletedStatus,
		Units: []AINativeGenerationUnit{
			{ID: "video-unit-01", ShotIDs: []string{"shot-1"}, StartMS: 0, EndMS: 4000, DurationSeconds: 4, PromptHash: strings.Repeat("a", 64), AspectRatio: "9:16", Resolution: "720p", ReferenceAsset: &oldScene, ReferenceRole: AINativeStoryboardAssetRoleSceneReference, Attempts: []AINativeGenerationAttempt{{ID: "attempt-a", Status: AINativeAttemptSucceededStatus, OutputAssetRef: &videoA}}, SelectedAttemptID: "attempt-a"},
			{ID: "video-unit-02", ShotIDs: []string{"shot-2"}, StartMS: 4000, EndMS: 8000, DurationSeconds: 4, PromptHash: strings.Repeat("b", 64), AspectRatio: "9:16", Resolution: "720p", ReferenceAsset: &product, ReferenceRole: AINativeStoryboardAssetRoleProductIdentity, Attempts: []AINativeGenerationAttempt{{ID: "attempt-b", Status: AINativeAttemptSucceededStatus, OutputAssetRef: &videoB}}, SelectedAttemptID: "attempt-b"},
		},
		SpeechUnits: []AINativeSpeechUnit{{ID: "speech-unit-01", ShotID: "shot-1", StartMS: 0, EndMS: 4000, Text: "旁白", Language: "zh-CN", VoiceAlias: "voice", Attempts: []AINativeGenerationAttempt{{ID: "speech-attempt", Status: AINativeAttemptSucceededStatus, OutputAssetRef: &audio}}, SelectedAttemptID: "speech-attempt"}},
		Render:      &AINativeRenderState{ID: "render-1", Status: AINativeProductionCompletedStatus, OutputAssetRef: &videoA, UpdatedAt: now},
	}
	next := AINativeProductionPlan{
		Status: AINativeProductionPreparedStatus,
		Units: []AINativeGenerationUnit{
			{ID: "video-unit-01", ShotIDs: []string{"shot-1"}, StartMS: 0, EndMS: 4000, DurationSeconds: 4, PromptHash: strings.Repeat("a", 64), AspectRatio: "9:16", Resolution: "720p", ReferenceAsset: &newScene, ReferenceRole: AINativeStoryboardAssetRoleSceneReference},
			{ID: "video-unit-02", ShotIDs: []string{"shot-2"}, StartMS: 4000, EndMS: 8000, DurationSeconds: 4, PromptHash: strings.Repeat("b", 64), AspectRatio: "9:16", Resolution: "720p", ReferenceAsset: &product, ReferenceRole: AINativeStoryboardAssetRoleProductIdentity},
		},
		SpeechUnits: []AINativeSpeechUnit{{ID: "speech-unit-01", ShotID: "shot-1", StartMS: 0, EndMS: 4000, Text: "旁白", Language: "zh-CN", VoiceAlias: "voice"}},
	}

	reused := ReuseCompatibleProductionOutputs(previous, next)
	if reused.Units[0].SelectedAttemptID != "" || len(reused.Units[0].Attempts) != 0 {
		t.Fatalf("unit with a replaced reference was reused: %#v", reused.Units[0])
	}
	if reused.Units[1].SelectedAttemptID != "attempt-b" || len(reused.Units[1].Attempts) != 1 {
		t.Fatalf("unchanged video unit was not reused: %#v", reused.Units[1])
	}
	if reused.SpeechUnits[0].SelectedAttemptID != "speech-attempt" || len(reused.SpeechUnits[0].Attempts) != 1 {
		t.Fatalf("unchanged speech unit was not reused: %#v", reused.SpeechUnits[0])
	}
	if reused.Render != nil {
		t.Fatalf("final render must never be reused: %#v", reused.Render)
	}
}

func readyConfirmedAINativeStoryboard() AINativeStoryboardRevision {
	storyboard := validAINativeStoryboard()
	storyboard.Status = AINativeStoryboardConfirmedStatus
	storyboard.ConfirmedBy = "user-1"
	now := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	storyboard.ConfirmedAt = &now
	for index := range storyboard.Assets {
		storyboard.Assets[index].Status = AINativeStoryboardAssetReady
		if storyboard.Assets[index].AssetRef == nil {
			storyboard.Assets[index].AssetRef = &contract.AssetVersionRef{AssetID: contract.AssetID("asset-ref-" + storyboard.Assets[index].ID), Version: 1}
		}
	}
	return storyboard
}
