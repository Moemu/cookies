package httpserver

import (
	"context"
	"net/http"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/systems/creative"
)

type gamePrerollV2Commands interface {
	CreateGamePrerollV2Workspace(context.Context, contract.RequestContext, contract.ProjectID, contract.IdempotencyKey, creative.CreateGamePrerollV2WorkspaceRequest) (creative.TaskDetail, error)
	GetGamePrerollV2Workspace(context.Context, contract.ActorContext, contract.ProjectID, string) (creative.TaskDetail, error)
	AnalyzeGamePrerollV2Source(context.Context, contract.ActorContext, contract.ProjectID, string, creative.AnalyzeGamePrerollV2SourceRequest) (creative.TaskDetail, error)
	ConfirmGamePrerollV2Brief(context.Context, contract.ActorContext, contract.ProjectID, string, creative.ConfirmGamePrerollV2BriefRequest) (creative.TaskDetail, error)
	PlanGamePrerollV2Candidates(context.Context, contract.ActorContext, contract.ProjectID, string, creative.PlanGamePrerollV2CandidatesRequest) (creative.TaskDetail, error)
	UpdateGamePrerollV2GenerationConfig(context.Context, contract.ActorContext, contract.ProjectID, string, creative.UpdateGamePrerollV2GenerationConfigRequest) (creative.TaskDetail, error)
	SelectGamePrerollCandidate(context.Context, contract.ActorContext, contract.ProjectID, string, creative.SelectGamePrerollCandidateRequest) (creative.TaskDetail, error)
	PrepareGamePrerollEvidence(context.Context, contract.RequestContext, contract.ProjectID, string, creative.PrepareGamePrerollEvidenceRequest) (creative.TaskDetail, error)
	GamePrerollProviderInput(context.Context, contract.ActorContext, contract.ProjectID, string) (provider.VideoGenerationInput, string, error)
	RegisterGamePrerollV2VideoJob(context.Context, contract.ActorContext, contract.ProjectID, string, int64, string) (creative.TaskDetail, error)
	ReconcileGamePrerollV2Video(context.Context, contract.ActorContext, contract.ProjectID, string, creative.ReconcileGamePrerollV2VideoRequest) (creative.TaskDetail, error)
}

func (s *Server) gamePrerollV2(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(gamePrerollV2Commands)
	if !ok {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	projectID := contract.ProjectID(r.PathValue("project_id"))
	taskID := r.PathValue("task_id")
	if r.Method == http.MethodGet {
		value, err := manager.GetGamePrerollV2Workspace(r.Context(), rc.Actor, projectID, taskID)
		if err != nil {
			s.writeServiceError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, value)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	if taskID == "" {
		var body creative.CreateGamePrerollV2WorkspaceRequest
		if err := decodeJSON(w, r, &body); err != nil {
			s.badRequest(w, r, err)
			return
		}
		value, err := manager.CreateGamePrerollV2Workspace(r.Context(), rc, projectID, key, body)
		if err != nil {
			s.writeServiceError(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, value)
		return
	}
	var value creative.TaskDetail
	var err error
	path := r.URL.Path
	switch {
	case strings.HasSuffix(path, "/actions/analyze-source"):
		var body creative.AnalyzeGamePrerollV2SourceRequest
		if decodeErr := decodeJSON(w, r, &body); decodeErr != nil {
			s.badRequest(w, r, decodeErr)
			return
		}
		value, err = manager.AnalyzeGamePrerollV2Source(r.Context(), rc.Actor, projectID, taskID, body)
	case strings.HasSuffix(path, "/actions/confirm-brief"):
		var body creative.ConfirmGamePrerollV2BriefRequest
		if decodeErr := decodeJSON(w, r, &body); decodeErr != nil {
			s.badRequest(w, r, decodeErr)
			return
		}
		value, err = manager.ConfirmGamePrerollV2Brief(r.Context(), rc.Actor, projectID, taskID, body)
	case strings.HasSuffix(path, "/actions/plan-candidates"):
		var body creative.PlanGamePrerollV2CandidatesRequest
		if decodeErr := decodeJSON(w, r, &body); decodeErr != nil {
			s.badRequest(w, r, decodeErr)
			return
		}
		value, err = manager.PlanGamePrerollV2Candidates(r.Context(), rc.Actor, projectID, taskID, body)
	case strings.HasSuffix(path, "/actions/update-generation-config"):
		var body creative.UpdateGamePrerollV2GenerationConfigRequest
		if decodeErr := decodeJSON(w, r, &body); decodeErr != nil {
			s.badRequest(w, r, decodeErr)
			return
		}
		value, err = manager.UpdateGamePrerollV2GenerationConfig(r.Context(), rc.Actor, projectID, taskID, body)
	case strings.HasSuffix(path, "/actions/select-candidate"):
		var body creative.SelectGamePrerollCandidateRequest
		if decodeErr := decodeJSON(w, r, &body); decodeErr != nil {
			s.badRequest(w, r, decodeErr)
			return
		}
		value, err = manager.SelectGamePrerollCandidate(r.Context(), rc.Actor, projectID, taskID, body)
	case strings.HasSuffix(path, "/actions/prepare-evidence"):
		var body creative.PrepareGamePrerollEvidenceRequest
		if decodeErr := decodeJSON(w, r, &body); decodeErr != nil {
			s.badRequest(w, r, decodeErr)
			return
		}
		value, err = manager.PrepareGamePrerollEvidence(r.Context(), rc, projectID, taskID, body)
	case strings.HasSuffix(path, "/actions/generate-video"):
		value, err = s.generateGamePrerollV2Video(w, r, rc, projectID, taskID, key, manager)
	case strings.HasSuffix(path, "/actions/reconcile-video"):
		if s.providerJobs == nil {
			err = creative.ErrInvalidState
			break
		}
		var body struct {
			ExpectedRevision int64  `json:"expected_revision"`
			ProviderJobID    string `json:"provider_job_id"`
		}
		if decodeErr := decodeJSON(w, r, &body); decodeErr != nil {
			s.badRequest(w, r, decodeErr)
			return
		}
		job, getErr := s.providerJobs.GetJob(r.Context(), rc.Actor.OrganizationID, projectID, body.ProviderJobID)
		if getErr != nil {
			err = getErr
		} else {
			value, err = manager.ReconcileGamePrerollV2Video(r.Context(), rc.Actor, projectID, taskID, creative.ReconcileGamePrerollV2VideoRequest{ExpectedRevision: body.ExpectedRevision, Job: job})
		}
	default:
		s.notFound(w, r)
		return
	}
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) generateGamePrerollV2Video(w http.ResponseWriter, r *http.Request, rc contract.RequestContext, projectID contract.ProjectID, taskID string, key contract.IdempotencyKey, manager gamePrerollV2Commands) (creative.TaskDetail, error) {
	if s.providerJobs == nil || s.projects == nil {
		return creative.TaskDetail{}, creative.ErrInvalidState
	}
	var body struct {
		ExpectedRevision int64 `json:"expected_revision"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		return creative.TaskDetail{}, err
	}
	detail, err := manager.GetGamePrerollV2Workspace(r.Context(), rc.Actor, projectID, taskID)
	if err != nil {
		return creative.TaskDetail{}, err
	}
	if detail.VideoDraft.Revision != body.ExpectedRevision {
		return creative.TaskDetail{}, creative.ErrVersionConflict
	}
	input, promptHash, err := manager.GamePrerollProviderInput(r.Context(), rc.Actor, projectID, taskID)
	if err != nil {
		return creative.TaskDetail{}, err
	}
	project, err := s.projects.GetContext(r.Context(), rc.Actor, projectID)
	if err != nil {
		return creative.TaskDetail{}, err
	}
	hash, err := contract.CanonicalJSONHash(struct {
		TaskID     string                        `json:"task_id"`
		Revision   int64                         `json:"revision"`
		PromptHash string                        `json:"prompt_hash"`
		Input      provider.VideoGenerationInput `json:"input"`
	}{taskID, body.ExpectedRevision, promptHash, input})
	if err != nil {
		return creative.TaskDetail{}, err
	}
	job, _, err := s.providerJobs.CreateVideoJob(r.Context(), provider.CreateVideoJobRequest{Actor: rc.Actor, Project: project, IdempotencyKey: key, RequestHash: hash, ModelAlias: "cookies.video.standard", SourceSystem: "creative.game_preroll_v2", SourceTaskID: taskID, Input: input})
	if err != nil {
		return creative.TaskDetail{}, err
	}
	return manager.RegisterGamePrerollV2VideoJob(r.Context(), rc.Actor, projectID, taskID, body.ExpectedRevision, job.ID)
}
