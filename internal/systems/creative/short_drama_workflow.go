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
