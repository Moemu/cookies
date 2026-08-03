package creative

import (
	"context"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/media"
	"github.com/shikanon/cookies/internal/platform/provider"
)

type SelectGamePrerollCandidateRequest struct {
	ExpectedRevision int64  `json:"expected_revision"`
	CandidateID      string `json:"candidate_id"`
}

type RegenerateGamePrerollCandidatesRequest struct {
	ExpectedRevision int64                       `json:"expected_revision"`
	GenerationConfig GamePrerollGenerationConfig `json:"generation_config"`
}

type PrepareGamePrerollEvidenceRequest struct {
	ExpectedRevision int64 `json:"expected_revision"`
}

func (s Service) PrepareGamePrerollEvidence(ctx context.Context, requestContext contract.RequestContext, projectID contract.ProjectID, taskID string, request PrepareGamePrerollEvidenceRequest) (TaskDetail, error) {
	if request.ExpectedRevision < 1 {
		return TaskDetail{}, fmt.Errorf("expected_revision must be positive")
	}
	if s.GameEvidenceFrames == nil || s.DerivedAssets == nil {
		return TaskDetail{}, fmt.Errorf("game evidence preparation capability is unavailable")
	}
	detail, err := s.requireGamePrerollWorkspace(ctx, requestContext.Actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	if request.ExpectedRevision != detail.VideoDraft.Revision {
		return TaskDetail{}, ErrVersionConflict
	}
	game := detail.VideoDraft.GamePreroll
	frames := make([]GameEvidenceFrameAsset, 0, len(game.InputSnapshot.EvidenceMoments))
	for _, moment := range game.InputSnapshot.EvidenceMoments {
		timestampMS := int64(moment.StartMilliseconds + (moment.EndMilliseconds-moment.StartMilliseconds)/2)
		extracted, err := s.GameEvidenceFrames.ExtractFrame(ctx, media.FrameExtractionRequest{
			OrganizationID: requestContext.Actor.OrganizationID, ProjectID: projectID,
			SourceVideo: game.InputSnapshot.SourceVideo, TimestampMS: timestampMS,
		})
		if err != nil {
			return TaskDetail{}, fmt.Errorf("extract evidence frame %s: %w", moment.ID, err)
		}
		derivationHash, err := contract.CanonicalJSONHash(struct {
			Source    contract.AssetVersionRef `json:"source"`
			Timestamp int64                    `json:"timestamp_ms"`
			Version   string                   `json:"extractor_version"`
		}{game.InputSnapshot.SourceVideo, timestampMS, extracted.Version})
		if err != nil {
			_ = extracted.Content.Close()
			return TaskDetail{}, err
		}
		ref, writeErr := s.DerivedAssets.IngestDerivedImage(ctx, requestContext, projectID, "game-frame-"+derivationHash, game.InputSnapshot.SourceVideo, extracted.Content, extracted.SizeBytes, extracted.MIMEType)
		closeErr := extracted.Content.Close()
		if writeErr != nil {
			return TaskDetail{}, fmt.Errorf("persist evidence frame %s: %w", moment.ID, writeErr)
		}
		if closeErr != nil {
			return TaskDetail{}, fmt.Errorf("close evidence frame %s: %w", moment.ID, closeErr)
		}
		frames = append(frames, GameEvidenceFrameAsset{
			EvidenceMomentID: moment.ID, SourceStartMilliseconds: moment.StartMilliseconds,
			SourceEndMilliseconds: moment.EndMilliseconds, RepresentativeFrameMS: int(timestampMS),
			FrameAsset: ref, ExtractionVersion: extracted.Version,
		})
	}
	contentHash, err := contract.CanonicalJSONHash(frames)
	if err != nil {
		return TaskDetail{}, err
	}
	now := s.now()
	next := *detail.VideoDraft
	next.Revision++
	next.CreatedAt = now
	updated := *game
	updated.ContractVersion = "creative-game-preroll-draft/v2"
	updated.Revision = next.Revision
	updated.EvidenceAssets = &GameEvidenceAssetSet{SourceVideo: game.InputSnapshot.SourceVideo, Status: "ready", Frames: frames, ContentHash: "sha256:" + contentHash}
	updated.Readiness.Blockers = removeStrings(updated.Readiness.Blockers, "evidence_assets")
	if updated.SelectedCandidateID != "" {
		for index := range updated.Candidates {
			if updated.Candidates[index].ID == updated.SelectedCandidateID {
				updated.GenerationSpec, err = compileGamePrerollGenerationSpec(updated, updated.Candidates[index], projectID, next.Revision)
				break
			}
		}
		if err != nil {
			return TaskDetail{}, err
		}
		updated.Readiness.GenerationReady = updated.GenerationSpec != nil
	}
	updated.UpdatedAt = now
	next.GamePreroll = &updated
	status := TaskInProgress
	if updated.Readiness.GenerationReady {
		status = TaskReady
	}
	if _, err := s.ViralRemakes.ReviseVideoDraft(ctx, requestContext.Actor.OrganizationID, projectID, taskID, detail.VideoDraft.Revision, next, status); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, requestContext.Actor.OrganizationID, projectID, taskID)
}

func (r RegenerateGamePrerollCandidatesRequest) Validate() error {
	if r.ExpectedRevision < 1 {
		return fmt.Errorf("expected_revision must be positive")
	}
	return r.GenerationConfig.Validate()
}

func (s Service) GetLatestGamePrerollWorkspace(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
) (TaskDetail, error) {
	tasks, err := s.ListTasks(ctx, actor, projectID, 100)
	if err != nil {
		return TaskDetail{}, err
	}
	var latest *CreativeTask
	for index := range tasks {
		task := &tasks[index]
		if task.Format != FormatVideo || task.PerformanceMode != PerformanceModeGamePreroll ||
			task.Status == TaskArchived {
			continue
		}
		if latest == nil || task.UpdatedAt.After(latest.UpdatedAt) {
			latest = task
		}
	}
	if latest == nil {
		return TaskDetail{}, ErrNotFound
	}
	return s.GetTaskDetail(ctx, actor, projectID, latest.ID)
}

func (s Service) SelectGamePrerollCandidate(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	taskID string,
	request SelectGamePrerollCandidateRequest,
) (TaskDetail, error) {
	detail, err := s.requireGamePrerollWorkspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	if request.ExpectedRevision != detail.VideoDraft.Revision {
		return TaskDetail{}, ErrVersionConflict
	}
	if strings.TrimSpace(request.CandidateID) == "" {
		return TaskDetail{}, fmt.Errorf("candidate_id is required")
	}
	game := detail.VideoDraft.GamePreroll
	selected := -1
	for index, candidate := range game.Candidates {
		if candidate.ID == request.CandidateID {
			selected = index
			break
		}
	}
	if selected < 0 {
		return TaskDetail{}, fmt.Errorf("game preroll candidate does not belong to this task")
	}
	now := s.now()
	next := *detail.VideoDraft
	next.Revision++
	next.CreatedAt = now
	updated := *game
	updated.Revision = next.Revision
	updated.SelectedCandidateID = request.CandidateID
	updated.Readiness.GenerationReady = false
	updated.Readiness.ProductionReady = false
	updated.Readiness.Blockers = removeStrings(updated.Readiness.Blockers, "selected_candidate")
	if updated.EvidenceAssets == nil || updated.EvidenceAssets.Status != "ready" {
		updated.Readiness.Blockers = appendUnique(updated.Readiness.Blockers, "evidence_assets")
	} else {
		updated.GenerationSpec, err = compileGamePrerollGenerationSpec(updated, updated.Candidates[selected], projectID, next.Revision)
		if err != nil {
			return TaskDetail{}, err
		}
		updated.Readiness.GenerationReady = true
		updated.Readiness.Blockers = removeStrings(updated.Readiness.Blockers, "evidence_assets")
	}
	updated.UpdatedAt = now
	next.GamePreroll = &updated
	next.Prompt = updated.Candidates[selected].PromptPackage.CompiledPrompt
	if _, err := s.ViralRemakes.ReviseVideoDraft(
		ctx,
		actor.OrganizationID,
		projectID,
		taskID,
		detail.VideoDraft.Revision,
		next,
		map[bool]TaskStatus{true: TaskReady, false: TaskInProgress}[updated.Readiness.GenerationReady],
	); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
}

func (s Service) RegenerateGamePrerollCandidates(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	taskID string,
	request RegenerateGamePrerollCandidatesRequest,
) (TaskDetail, error) {
	if err := request.Validate(); err != nil {
		return TaskDetail{}, err
	}
	detail, err := s.requireGamePrerollWorkspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	if request.ExpectedRevision != detail.VideoDraft.Revision {
		return TaskDetail{}, ErrVersionConflict
	}
	projectContext, err := s.Projects.RequireActiveContext(ctx, actor, projectID)
	if err != nil {
		return TaskDetail{}, err
	}
	planner := s.GamePrerollPlanner
	if planner == nil {
		planner = DeterministicGamePrerollPlanner{}
	}
	now := s.now()
	nextRevision := detail.VideoDraft.Revision + 1
	batch, err := planner.Plan(
		ctx,
		actor,
		projectContext,
		detail.VideoDraft.GamePreroll.InputSnapshot,
		detail.VideoDraft.GamePreroll.InputHash,
		fmt.Sprintf("%s_batch_%d", taskID, nextRevision),
		nextRevision,
		request.GenerationConfig,
		now,
	)
	if err != nil {
		return TaskDetail{}, err
	}
	next := *detail.VideoDraft
	next.Revision = nextRevision
	next.CreatedAt = now
	updated := *detail.VideoDraft.GamePreroll
	updated.Revision = nextRevision
	updated.ActiveCandidateBatch = &batch
	updated.Candidates = batch.Candidates
	updated.SelectedCandidateID = ""
	updated.GenerationSpec = nil
	updated.Readiness.GenerationReady = false
	updated.Readiness.ProductionReady = false
	updated.Readiness.Blockers = appendUnique(updated.Readiness.Blockers, "selected_candidate")
	updated.UpdatedAt = now
	next.GamePreroll = &updated
	next.Prompt = batch.Candidates[0].PromptPackage.CompiledPrompt
	if _, err := s.ViralRemakes.ReviseVideoDraft(
		ctx,
		actor.OrganizationID,
		projectID,
		taskID,
		detail.VideoDraft.Revision,
		next,
		TaskInProgress,
	); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
}

func (s Service) GamePrerollProviderInput(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	taskID string,
) (provider.VideoGenerationInput, string, error) {
	detail, err := s.requireGamePrerollWorkspace(ctx, actor, projectID, taskID, false)
	if err != nil {
		return provider.VideoGenerationInput{}, "", err
	}
	game := detail.VideoDraft.GamePreroll
	if !game.Readiness.GenerationReady || strings.TrimSpace(game.SelectedCandidateID) == "" || game.GenerationSpec == nil {
		return provider.VideoGenerationInput{}, "", ErrInvalidState
	}
	for _, candidate := range game.Candidates {
		if candidate.ID != game.SelectedCandidateID {
			continue
		}
		conditioning := make([]provider.VideoConditioningAsset, 0, len(game.GenerationSpec.ConditioningAssets))
		for _, asset := range game.GenerationSpec.ConditioningAssets {
			conditioning = append(conditioning, provider.VideoConditioningAsset{Role: provider.VideoConditioningRole(asset.Role), Reference: asset.Reference})
		}
		input := provider.VideoGenerationInput{
			Prompt: candidate.PromptPackage.CompiledPrompt, DurationSeconds: game.GenerationSpec.DurationSeconds,
			AspectRatio: game.GenerationSpec.AspectRatio, Resolution: game.GenerationSpec.Resolution,
			AudioPolicy: provider.VideoAudioPolicy(game.GenerationSpec.AudioPolicy),
			InputMode:   provider.VideoInputMode(game.GenerationSpec.InputMode), ConditioningAssets: conditioning,
		}
		if err := input.Validate(); err != nil {
			return provider.VideoGenerationInput{}, "", err
		}
		return input, candidate.PromptPackage.ContentHash, nil
	}
	return provider.VideoGenerationInput{}, "", ErrInvalidState
}

func compileGamePrerollGenerationSpec(game GamePrerollDraft, candidate GamePrerollCandidate, projectID contract.ProjectID, revision int64) (*GamePrerollGenerationSpec, error) {
	if game.ActiveCandidateBatch == nil || game.EvidenceAssets == nil || game.EvidenceAssets.Status != "ready" || len(game.EvidenceAssets.Frames) < 3 {
		return nil, ErrInvalidState
	}
	if len(candidate.Storyboard) < 2 {
		return nil, ErrInvalidState
	}
	first, firstOK := gameEvidenceFrameByMoment(game.EvidenceAssets.Frames, candidate.Storyboard[0].EvidenceMomentID)
	last, lastOK := gameEvidenceFrameByMoment(game.EvidenceAssets.Frames, candidate.Storyboard[len(candidate.Storyboard)-1].EvidenceMomentID)
	if !firstOK || !lastOK || first.FrameAsset.AssetVersion == last.FrameAsset.AssetVersion {
		return nil, ErrInvalidState
	}
	spec := GamePrerollGenerationSpec{
		ContractVersion: "creative-game-preroll-generation-spec/v1", TaskID: game.TaskID, DraftRevision: revision,
		InputSnapshotHash: game.InputHash, CandidateBatchID: game.ActiveCandidateBatch.ID, CandidateID: candidate.ID,
		PromptPackageHash: candidate.PromptPackage.ContentHash, InputMode: string(provider.VideoInputFirstLastFrame),
		ConditioningAssets: []GameVideoConditioningAsset{
			{Role: string(provider.VideoConditioningFirstFrame), EvidenceMomentID: first.EvidenceMomentID, Reference: first.FrameAsset},
			{Role: string(provider.VideoConditioningLastFrame), EvidenceMomentID: last.EvidenceMomentID, Reference: last.FrameAsset},
		},
		DurationSeconds: 6, AspectRatio: "9:16", Resolution: "720p", AudioPolicy: string(provider.VideoAudioGenerated),
	}
	for _, asset := range spec.ConditioningAssets {
		if asset.Reference.ProjectID != projectID {
			return nil, ErrInvalidState
		}
	}
	hash, err := contract.CanonicalJSONHash(spec)
	if err != nil {
		return nil, err
	}
	spec.Hash = "sha256:" + hash
	return &spec, nil
}

func gameEvidenceFrameByMoment(frames []GameEvidenceFrameAsset, momentID string) (GameEvidenceFrameAsset, bool) {
	for _, frame := range frames {
		if frame.EvidenceMomentID == momentID {
			return frame, true
		}
	}
	return GameEvidenceFrameAsset{}, false
}

func (s Service) RegisterGamePrerollGenerationAttempt(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	taskID string,
	providerJobID string,
) (GamePrerollGenerationAttempt, error) {
	if strings.TrimSpace(providerJobID) == "" {
		return GamePrerollGenerationAttempt{}, fmt.Errorf("provider_job_id is required")
	}
	detail, err := s.requireGamePrerollWorkspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return GamePrerollGenerationAttempt{}, err
	}
	game := detail.VideoDraft.GamePreroll
	if !game.Readiness.GenerationReady || game.ActiveCandidateBatch == nil ||
		strings.TrimSpace(game.SelectedCandidateID) == "" {
		return GamePrerollGenerationAttempt{}, ErrInvalidState
	}
	var selected *GamePrerollCandidate
	for index := range game.Candidates {
		if game.Candidates[index].ID == game.SelectedCandidateID {
			selected = &game.Candidates[index]
			break
		}
	}
	if selected == nil {
		return GamePrerollGenerationAttempt{}, ErrInvalidState
	}
	videoInput, promptHash, err := s.GamePrerollProviderInput(ctx, actor, projectID, taskID)
	if err != nil {
		return GamePrerollGenerationAttempt{}, err
	}
	specHash, err := contract.CanonicalJSONHash(struct {
		PromptPackageHash string                        `json:"prompt_package_hash"`
		Input             provider.VideoGenerationInput `json:"input"`
	}{
		PromptPackageHash: promptHash,
		Input:             videoInput,
	})
	if err != nil {
		return GamePrerollGenerationAttempt{}, err
	}
	id, err := s.idGenerator()("gameprerollattempt")
	if err != nil {
		return GamePrerollGenerationAttempt{}, err
	}
	attempt := GamePrerollGenerationAttempt{
		ID: id, TaskID: taskID, DraftRevision: detail.VideoDraft.Revision,
		CandidateBatchID:  game.ActiveCandidateBatch.ID,
		CandidateID:       selected.ID,
		PromptPackageHash: promptHash, GenerationSpecHash: "sha256:" + specHash,
		ProviderJobID: providerJobID, CreatedAt: s.now(),
	}
	return s.Repository.CreateGamePrerollGenerationAttempt(
		ctx,
		actor.OrganizationID,
		projectID,
		attempt,
	)
}

func (s Service) requireGamePrerollWorkspace(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	taskID string,
	write bool,
) (TaskDetail, error) {
	if s.Repository == nil || s.ViralRemakes == nil || s.Projects == nil {
		return TaskDetail{}, fmt.Errorf("game preroll dependencies are incomplete")
	}
	if write && !actor.HasScope(ScopeWrite) {
		return TaskDetail{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return TaskDetail{}, err
	}
	detail, err := s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
	if err != nil {
		return TaskDetail{}, err
	}
	if detail.Task.Format != FormatVideo || detail.Task.PerformanceMode != PerformanceModeGamePreroll ||
		detail.VideoDraft == nil || detail.VideoDraft.GamePreroll == nil ||
		detail.Task.Status == TaskArchived {
		return TaskDetail{}, ErrInvalidState
	}
	return detail, nil
}
