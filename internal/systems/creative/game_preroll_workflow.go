package creative

import (
	"context"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
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
	updated.Readiness.GenerationReady = true
	updated.Readiness.ProductionReady = false
	updated.Readiness.Blockers = removeStrings(updated.Readiness.Blockers, "selected_candidate")
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
		TaskReady,
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
	if !game.Readiness.GenerationReady || strings.TrimSpace(game.SelectedCandidateID) == "" {
		return provider.VideoGenerationInput{}, "", ErrInvalidState
	}
	for _, candidate := range game.Candidates {
		if candidate.ID != game.SelectedCandidateID {
			continue
		}
		return provider.VideoGenerationInput{
			Prompt: candidate.PromptPackage.CompiledPrompt, DurationSeconds: 6,
			AspectRatio: "9:16", Resolution: "720p",
			AudioPolicy: provider.VideoAudioGenerated,
			InputMode:   provider.VideoInputTextOnly,
		}, candidate.PromptPackage.ContentHash, nil
	}
	return provider.VideoGenerationInput{}, "", ErrInvalidState
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
