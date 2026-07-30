package creative

import (
	"context"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

type SelectShortDramaCandidateRequest struct {
	ExpectedRevision int64  `json:"expected_revision"`
	CandidateID      string `json:"candidate_id"`
}

type RegenerateShortDramaCandidatesRequest struct {
	ExpectedRevision int64                      `json:"expected_revision"`
	GenerationConfig ShortDramaGenerationConfig `json:"generation_config"`
	VariationIntent  string                     `json:"variation_intent"`
}

func (s Service) GetLatestShortDramaWorkspace(
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
		if task.Format != FormatVideo || task.PerformanceMode != PerformanceModeShortDramaPreroll || task.Status == TaskArchived {
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

func (r RegenerateShortDramaCandidatesRequest) Validate() error {
	if r.ExpectedRevision < 1 {
		return fmt.Errorf("expected_revision must be positive")
	}
	if err := r.GenerationConfig.Validate(); err != nil {
		return err
	}
	switch r.VariationIntent {
	case "", "balanced", "more_visual", "more_dialogue", "more_suspense":
		return nil
	default:
		return fmt.Errorf("unsupported variation_intent")
	}
}

func (s Service) RegenerateShortDramaCandidates(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	taskID string,
	request RegenerateShortDramaCandidatesRequest,
) (TaskDetail, error) {
	if err := request.Validate(); err != nil {
		return TaskDetail{}, err
	}
	detail, err := s.requireShortDramaWorkspace(ctx, actor, projectID, taskID, true)
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
	variationIntent := request.VariationIntent
	if variationIntent == "" {
		variationIntent = "balanced"
	}
	now := s.now()
	nextRevision := detail.VideoDraft.Revision + 1
	batchID := fmt.Sprintf("%s_batch_%d", taskID, nextRevision)
	planner := s.ShortDramaPrerollPlanner
	if planner == nil {
		planner = DeterministicShortDramaPrerollPlanner{}
	}
	batch, err := planner.Plan(
		ctx,
		actor,
		projectContext,
		detail.VideoDraft.ShortDramaPreroll.InputSnapshot,
		detail.VideoDraft.ShortDramaPreroll.InputHash,
		batchID,
		nextRevision,
		request.GenerationConfig,
		variationIntent,
		now,
	)
	if err != nil {
		return TaskDetail{}, err
	}
	next := *detail.VideoDraft
	next.Revision = nextRevision
	next.CreatedAt = now
	updated := *detail.VideoDraft.ShortDramaPreroll
	updated.Revision = nextRevision
	updated.ActiveCandidateBatch = &batch
	updated.Candidates = batch.Candidates
	updated.SelectedCandidateID = ""
	updated.Readiness.GenerationReady = false
	updated.Readiness.ProductionReady = false
	updated.Readiness.Blockers = appendUnique(updated.Readiness.Blockers, "selected_candidate")
	updated.UpdatedAt = now
	next.ShortDramaPreroll = &updated
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

func (s Service) SelectShortDramaCandidate(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request SelectShortDramaCandidateRequest) (TaskDetail, error) {
	detail, err := s.requireShortDramaWorkspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	short := detail.VideoDraft.ShortDramaPreroll
	if request.ExpectedRevision != detail.VideoDraft.Revision || strings.TrimSpace(request.CandidateID) == "" {
		return TaskDetail{}, ErrVersionConflict
	}
	selected := -1
	for index, candidate := range short.Candidates {
		if candidate.ID == request.CandidateID {
			selected = index
			break
		}
	}
	if selected < 0 {
		return TaskDetail{}, fmt.Errorf("short drama candidate does not belong to this task")
	}
	now := s.now()
	next := *detail.VideoDraft
	next.Revision++
	next.CreatedAt = now
	updated := *short
	updated.Revision = next.Revision
	updated.SelectedCandidateID = request.CandidateID
	updated.Readiness.GenerationReady = true
	updated.Readiness.ProductionReady = false
	updated.Readiness.Blockers = removeStrings(updated.Readiness.Blockers, "selected_candidate")
	updated.UpdatedAt = now
	next.ShortDramaPreroll = &updated
	next.Prompt = updated.Candidates[selected].PromptPackage.CompiledPrompt
	if _, err := s.ViralRemakes.ReviseVideoDraft(ctx, actor.OrganizationID, projectID, taskID, detail.VideoDraft.Revision, next, TaskReady); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
}

func (s Service) ShortDramaProviderInput(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string) (provider.VideoGenerationInput, string, error) {
	detail, err := s.requireShortDramaWorkspace(ctx, actor, projectID, taskID, false)
	if err != nil {
		return provider.VideoGenerationInput{}, "", err
	}
	short := detail.VideoDraft.ShortDramaPreroll
	if !short.Readiness.GenerationReady || strings.TrimSpace(short.SelectedCandidateID) == "" {
		return provider.VideoGenerationInput{}, "", ErrInvalidState
	}
	for _, candidate := range short.Candidates {
		if candidate.ID == short.SelectedCandidateID {
			return provider.VideoGenerationInput{
				Prompt: candidate.PromptPackage.CompiledPrompt, DurationSeconds: 6,
				AspectRatio: "9:16", Resolution: "720p", AudioPolicy: provider.VideoAudioGenerated,
				InputMode: provider.VideoInputTextOnly,
			}, candidate.PromptPackage.ContentHash, nil
		}
	}
	return provider.VideoGenerationInput{}, "", ErrInvalidState
}

func (s Service) RegisterShortDramaGenerationAttempt(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	taskID string,
	providerJobID string,
) (ShortDramaGenerationAttempt, error) {
	if strings.TrimSpace(providerJobID) == "" {
		return ShortDramaGenerationAttempt{}, fmt.Errorf("provider_job_id is required")
	}
	detail, err := s.requireShortDramaWorkspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return ShortDramaGenerationAttempt{}, err
	}
	short := detail.VideoDraft.ShortDramaPreroll
	if !short.Readiness.GenerationReady || strings.TrimSpace(short.SelectedCandidateID) == "" {
		return ShortDramaGenerationAttempt{}, ErrInvalidState
	}
	var selected *ShortDramaPrerollCandidate
	for index := range short.Candidates {
		if short.Candidates[index].ID == short.SelectedCandidateID {
			selected = &short.Candidates[index]
			break
		}
	}
	if selected == nil {
		return ShortDramaGenerationAttempt{}, ErrInvalidState
	}
	videoInput, promptHash, err := s.ShortDramaProviderInput(ctx, actor, projectID, taskID)
	if err != nil {
		return ShortDramaGenerationAttempt{}, err
	}
	specHash, err := contract.CanonicalJSONHash(struct {
		PromptPackageHash string                        `json:"prompt_package_hash"`
		Input             provider.VideoGenerationInput `json:"input"`
	}{
		PromptPackageHash: promptHash,
		Input:             videoInput,
	})
	if err != nil {
		return ShortDramaGenerationAttempt{}, err
	}
	id, err := s.idGenerator()("shortdramaattempt")
	if err != nil {
		return ShortDramaGenerationAttempt{}, err
	}
	attempt := ShortDramaGenerationAttempt{
		ID: id, TaskID: taskID, DraftRevision: detail.VideoDraft.Revision,
		CandidateBatchID: selected.PromptPackage.CandidateBatchID,
		CandidateID:      selected.ID, PromptPackageHash: promptHash,
		GenerationSpecHash: "sha256:" + specHash, ProviderJobID: providerJobID, CreatedAt: s.now(),
	}
	return s.Repository.CreateShortDramaGenerationAttempt(
		ctx, actor.OrganizationID, projectID, attempt,
	)
}

func (s Service) requireShortDramaWorkspace(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, write bool) (TaskDetail, error) {
	if s.Repository == nil || s.ViralRemakes == nil || s.Projects == nil {
		return TaskDetail{}, fmt.Errorf("short drama preroll dependencies are incomplete")
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
	if detail.Task.Format != FormatVideo || detail.Task.PerformanceMode != PerformanceModeShortDramaPreroll ||
		detail.VideoDraft == nil || detail.VideoDraft.ShortDramaPreroll == nil || detail.Task.Status == TaskArchived {
		return TaskDetail{}, ErrInvalidState
	}
	return detail, nil
}
