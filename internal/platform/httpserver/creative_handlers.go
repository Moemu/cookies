package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/systems/creative"
)

type creativeDirectionManager interface {
	GenerateDirectionCandidates(context.Context, contract.ActorContext, contract.ProjectID, string, creative.GenerateDirectionRequest) (creative.CreativeDirectionBatch, error)
	ConfirmDirection(context.Context, contract.ActorContext, contract.ProjectID, string) (creative.CreativeDirectionVersion, error)
}

type creativeImageTextManager interface {
	GetImageTextWorkspace(context.Context, contract.ActorContext, contract.ProjectID, string) (creative.ImageTextWorkspace, error)
	GenerateImageTextDraft(context.Context, contract.ActorContext, contract.ProjectID, string, creative.GenerateImageTextDraftRequest) (creative.ImageTextDraft, error)
	UpdateImageTextDraft(context.Context, contract.ActorContext, contract.ProjectID, string, creative.UpdateImageTextDraftRequest) (creative.ImageTextDraft, error)
	PrepareImageSlotGeneration(context.Context, contract.RequestContext, contract.ProjectID, string, int, creative.PrepareImageSlotRequest, contract.IdempotencyKey) (creative.ImagePromptPackage, creative.ImageGenerationAttempt, bool, error)
	AttachImageProviderJob(context.Context, contract.ActorContext, contract.ProjectID, string, string) (creative.ImageGenerationAttempt, error)
	AdoptImageGenerationAttempt(context.Context, contract.ActorContext, contract.ProjectID, string, int, string, creative.AdoptImageAttemptRequest) (creative.ImageSlotSelection, error)
}

func (s *Server) imageTextManager(w http.ResponseWriter, r *http.Request) (creativeImageTextManager, bool) {
	manager, ok := s.creative.(creativeImageTextManager)
	if !ok || manager == nil {
		s.notImplemented(w, r)
		return nil, false
	}
	return manager, true
}

func (s *Server) getCreativeImageTextWorkspace(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.imageTextManager(w, r)
	if !ok {
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.GetImageTextWorkspace(
		r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("task_id"),
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) generateCreativeImageTextDraft(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.imageTextManager(w, r)
	if !ok {
		return
	}
	var body creative.GenerateImageTextDraftRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.GenerateImageTextDraft(
		r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("task_id"), body,
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) updateCreativeImageTextDraft(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.imageTextManager(w, r)
	if !ok {
		return
	}
	var body creative.UpdateImageTextDraftRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.UpdateImageTextDraft(
		r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("task_id"), body,
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) generateCreativeImageTextSlot(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.imageTextManager(w, r)
	if !ok || s.providerJobs == nil || s.projects == nil {
		if ok {
			s.notImplemented(w, r)
		}
		return
	}
	slotAction := r.PathValue("slot_action")
	actionIndex := strings.LastIndex(slotAction, ":")
	if actionIndex < 1 || (slotAction[actionIndex:] != ":generate" && slotAction[actionIndex:] != ":retry") {
		s.notFound(w, r)
		return
	}
	order, err := strconv.Atoi(slotAction[:actionIndex])
	if err != nil || order < 1 || order > 3 {
		s.badRequest(w, r, fmt.Errorf("image slot order must be between 1 and 3"))
		return
	}
	var body creative.PrepareImageSlotRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	projectID := contract.ProjectID(r.PathValue("project_id"))
	taskID := r.PathValue("task_id")
	prompt, attempt, _, err := manager.PrepareImageSlotGeneration(
		r.Context(), rc, projectID, taskID, order, body, key,
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	project, err := s.projects.GetContext(r.Context(), rc.Actor, projectID)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	draftRevision := attempt.DraftRevision
	sourceAssets := make([]contract.ProjectAssetRef, 0, len(prompt.SourceAssetRefs))
	for _, ref := range prompt.SourceAssetRefs {
		sourceAssets = append(sourceAssets, contract.ProjectAssetRef{ProjectID: projectID, AssetVersion: ref})
	}
	job, _, err := s.providerJobs.CreateImageJob(r.Context(), provider.CreateImageJobRequest{
		Actor: rc.Actor, Project: project, IdempotencyKey: key, RequestHash: attempt.RequestHash,
		ModelAlias: attempt.GenerationSpec.ModelAlias, SourceSystem: "creative", SourceTaskID: taskID,
		Operation: "creative.image_text.generate",
		Input: provider.ImageGenerationInput{
			Prompt: prompt.CompiledPrompt, Width: attempt.GenerationSpec.Width, Height: attempt.GenerationSpec.Height,
			SourceAssets: sourceAssets,
			PromptRef: &contract.ResourceRef{
				Type: "creative_image_prompt_package", ID: prompt.ID,
			},
			SourceResourceRefs: []contract.ResourceRef{
				{Type: "creative_task", ID: taskID},
				{Type: "creative_image_text_draft", ID: taskID, Version: &draftRevision},
				{Type: "creative_direction", ID: prompt.DirectionID},
			},
		},
	})
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	attempt, err = manager.AttachImageProviderJob(r.Context(), rc.Actor, projectID, attempt.ID, job.ID)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"attempt": attempt, "provider_job": job})
}

func (s *Server) adoptCreativeImageTextSlot(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.imageTextManager(w, r)
	if !ok {
		return
	}
	order, err := strconv.Atoi(r.PathValue("order"))
	if err != nil || order < 1 || order > 3 {
		s.badRequest(w, r, fmt.Errorf("image slot order must be between 1 and 3"))
		return
	}
	var body creative.AdoptImageAttemptRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	attemptAction := r.PathValue("attempt_action")
	if !strings.HasSuffix(attemptAction, ":adopt") {
		s.notFound(w, r)
		return
	}
	attemptID := strings.TrimSuffix(attemptAction, ":adopt")
	if attemptID == "" {
		s.notFound(w, r)
		return
	}
	value, err := manager.AdoptImageGenerationAttempt(
		r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")),
		r.PathValue("task_id"), order, attemptID, body,
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) listCommercePrerollSources(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	values, err := s.creative.ListCommercePrerollSources(
		r.Context(),
		rc.Actor,
		contract.ProjectID(r.PathValue("project_id")),
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) prepareCommercePreroll(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	var body creative.PrepareCommercePrerollRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.PrepareCommercePreroll(
		r.Context(),
		rc.Actor,
		contract.ProjectID(r.PathValue("project_id")),
		body,
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) ensureCommerceFixtureWorkspace(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	var body creative.EnsureCommerceFixtureWorkspaceRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.EnsureCommerceFixtureWorkspace(
		r.Context(),
		rc,
		contract.ProjectID(r.PathValue("project_id")),
		key,
		body,
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) getLatestCommercePrerollWorkspace(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.GetLatestCommerceWorkspace(
		r.Context(),
		rc.Actor,
		contract.ProjectID(r.PathValue("project_id")),
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) getCommercePrerollWorkspace(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.GetCommerceWorkspace(
		r.Context(),
		rc.Actor,
		contract.ProjectID(r.PathValue("project_id")),
		r.PathValue("task_id"),
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) updateCommercePrerollDraft(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	if _, ok := idempotencyKey(w, r); !ok {
		return
	}
	var body creative.UpdateCommercePrerollDraftRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.UpdateCommercePrerollDraft(
		r.Context(),
		rc.Actor,
		contract.ProjectID(r.PathValue("project_id")),
		r.PathValue("task_id"),
		body,
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) confirmCommerceGeneration(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	if _, ok := idempotencyKey(w, r); !ok {
		return
	}
	var body creative.ConfirmCommerceGenerationRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.ConfirmCommerceGeneration(
		r.Context(),
		rc.Actor,
		contract.ProjectID(r.PathValue("project_id")),
		r.PathValue("task_id"),
		body,
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) createCreativeIntake(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	var body creative.CreateIntakeRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.CreateIntake(r.Context(), rc, contract.ProjectID(r.PathValue("project_id")), key, body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/api/creative/v1/projects/%s/creative-intakes/%s", r.PathValue("project_id"), value.ID))
	if view, viewErr := value.V3View(); viewErr == nil {
		writeJSON(w, http.StatusCreated, view)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

func (s *Server) listCreativeIntakes(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	values, err := s.creative.ListIntakes(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), queryLimit(r))
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) getCreativeIntake(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.GetIntake(
		r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")),
		r.PathValue("intake_id"),
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	if view, viewErr := value.V3View(); viewErr == nil {
		writeJSON(w, http.StatusOK, view)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) createCreativeDirectionBatch(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(creativeDirectionManager)
	if !ok {
		s.notImplemented(w, r)
		return
	}
	var body creative.GenerateDirectionRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.GenerateDirectionCandidates(
		r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")),
		r.PathValue("intake_id"), body,
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf(
		"/api/creative/v1/projects/%s/creative-direction-batches/%s",
		r.PathValue("project_id"), value.ID,
	))
	writeJSON(w, http.StatusCreated, value)
}

func (s *Server) confirmCreativeDirection(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.creative.(creativeDirectionManager)
	if !ok {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := manager.ConfirmDirection(
		r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")),
		r.PathValue("direction_id"),
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) listCreativeBusinessCapabilities(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	values, err := s.creative.ListBusinessCapabilities(
		r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")),
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) createCreativeTask(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	action := r.PathValue("intake_action")
	createVideo := strings.HasSuffix(action, ":create-video-task")
	if !createVideo && !strings.HasSuffix(action, ":create-task") {
		s.notFound(w, r)
		return
	}
	suffix := ":create-task"
	if createVideo {
		suffix = ":create-video-task"
	}
	intakeID := strings.TrimSuffix(action, suffix)
	if intakeID == "" {
		s.notFound(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	var value creative.CreativeTask
	var err error
	if createVideo {
		var body creative.CreateVideoTaskRequest
		if decodeErr := decodeJSON(w, r, &body); decodeErr != nil {
			s.badRequest(w, r, decodeErr)
			return
		}
		value, err = s.creative.CreateVideoTask(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), intakeID, body)
	} else {
		var body creative.CreateTaskRequest
		if decodeErr := decodeJSON(w, r, &body); decodeErr != nil {
			s.badRequest(w, r, decodeErr)
			return
		}
		value, err = s.creative.CreateTask(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), intakeID, body)
	}
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/api/creative/v1/projects/%s/creative-tasks/%s", r.PathValue("project_id"), value.ID))
	writeJSON(w, http.StatusCreated, value)
}

func (s *Server) listCreativeTasks(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	values, err := s.creative.ListTasks(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), queryLimit(r))
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) getCreativeTask(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.GetTaskDetail(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("task_id"))
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) getViralRemakeWorkspace(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.GetTaskDetail(
		r.Context(),
		rc.Actor,
		contract.ProjectID(r.PathValue("project_id")),
		r.PathValue("task_id"),
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	if value.Task.Format != creative.FormatVideo ||
		value.Task.PerformanceMode != creative.PerformanceModeViralRemake ||
		value.VideoDraft == nil ||
		value.VideoDraft.ViralRemake == nil {
		s.notFound(w, r)
		return
	}
	if s.providerJobs != nil && rc.Actor.HasScope(creative.ScopeWrite) {
		for _, candidate := range value.VideoDraft.ViralRemake.Candidates {
			if candidate.Status != creative.ViralCandidateQueued && candidate.Status != creative.ViralCandidateRunning {
				continue
			}
			job, jobErr := s.providerJobs.GetJob(r.Context(), rc.Actor.OrganizationID, value.Task.ProjectID, candidate.ProviderJobID)
			if jobErr != nil {
				continue
			}
			reconciled, reconcileErr := s.creative.ReconcileViralCandidate(r.Context(), rc.Actor, value.Task.ProjectID, value.Task.ID, job)
			if reconcileErr == nil {
				value = reconciled
			}
		}
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) getShortDramaPrerollWorkspace(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.GetTaskDetail(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("task_id"))
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	if value.Task.Format != creative.FormatVideo || value.Task.PerformanceMode != creative.PerformanceModeShortDramaPreroll ||
		value.VideoDraft == nil || value.VideoDraft.ShortDramaPreroll == nil {
		s.notFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) getLatestShortDramaPrerollWorkspace(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.GetLatestShortDramaWorkspace(
		r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")),
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) getLatestGamePrerollWorkspace(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.GetLatestGamePrerollWorkspace(
		r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")),
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) selectGamePrerollCandidate(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	if _, ok := idempotencyKey(w, r); !ok {
		return
	}
	var body creative.SelectGamePrerollCandidateRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.SelectGamePrerollCandidate(
		r.Context(),
		rc.Actor,
		contract.ProjectID(r.PathValue("project_id")),
		r.PathValue("task_id"),
		body,
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) regenerateGamePrerollCandidates(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	if _, ok := idempotencyKey(w, r); !ok {
		return
	}
	var body creative.RegenerateGamePrerollCandidatesRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.RegenerateGamePrerollCandidates(
		r.Context(),
		rc.Actor,
		contract.ProjectID(r.PathValue("project_id")),
		r.PathValue("task_id"),
		body,
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) selectShortDramaCandidate(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	if _, ok := idempotencyKey(w, r); !ok {
		return
	}
	var body creative.SelectShortDramaCandidateRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.SelectShortDramaCandidate(
		r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("task_id"), body,
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) regenerateShortDramaCandidates(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	if _, ok := idempotencyKey(w, r); !ok {
		return
	}
	var body creative.RegenerateShortDramaCandidatesRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.RegenerateShortDramaCandidates(
		r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("task_id"), body,
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) analyzeViralRemake(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	if _, ok := idempotencyKey(w, r); !ok {
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.AnalyzeViralRemake(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("task_id"))
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) updateViralPrompt(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	var body creative.UpdateViralPromptRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.UpdateViralPrompt(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("task_id"), body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) confirmViralGeneration(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	if _, ok := idempotencyKey(w, r); !ok {
		return
	}
	var body creative.ConfirmViralGenerationRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.ConfirmViralGeneration(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("task_id"), body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) transitionViralCandidate(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("candidate_action")
	if !strings.HasSuffix(action, ":submit-review") {
		s.notFound(w, r)
		return
	}
	candidateID := strings.TrimSuffix(action, ":submit-review")
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.SubmitViralCandidateReview(
		r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("task_id"), candidateID,
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) archiveCreativeTask(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	if err := s.creative.ArchiveTask(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("task_id")); err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) reviseCreativeDraft(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	action := r.PathValue("task_action")
	if !strings.HasSuffix(action, ":draft") {
		s.notFound(w, r)
		return
	}
	taskID := strings.TrimSuffix(action, ":draft")
	if taskID == "" {
		s.notFound(w, r)
		return
	}
	var body creative.ReviseDraftRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.ReviseDraft(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), taskID, body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) createCreativeCoverImageJob(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	action := r.PathValue("task_action")
	if strings.HasSuffix(action, ":freeze-version") {
		s.freezeCreativeVersion(w, r, strings.TrimSuffix(action, ":freeze-version"))
		return
	}
	if strings.HasSuffix(action, ":bind-image-asset") {
		s.bindCreativeImageAsset(w, r, strings.TrimSuffix(action, ":bind-image-asset"))
		return
	}
	if strings.HasSuffix(action, ":video-job") {
		s.createCreativeVideoJob(w, r, strings.TrimSuffix(action, ":video-job"))
		return
	}
	if strings.HasSuffix(action, ":render-preroll") {
		s.createCreativeRenderJob(w, r, strings.TrimSuffix(action, ":render-preroll"))
		return
	}
	if (!strings.HasSuffix(action, ":cover-image-job") && !strings.HasSuffix(action, ":image-job")) || s.providerJobs == nil || s.projects == nil {
		s.notFound(w, r)
		return
	}
	legacyCover := strings.HasSuffix(action, ":cover-image-job")
	taskID := strings.TrimSuffix(action, ":image-job")
	if legacyCover {
		taskID = strings.TrimSuffix(action, ":cover-image-job")
	}
	if taskID == "" {
		s.notFound(w, r)
		return
	}
	var body creative.CreateImageJobRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	modelAlias := strings.TrimSpace(body.ModelAlias)
	if modelAlias == "" {
		modelAlias = "cookies.image.standard"
	}
	if legacyCover {
		body.ImagePlanOrder = 1
	}
	if err := body.Validate(); err != nil {
		s.badRequest(w, r, err)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	projectID := contract.ProjectID(r.PathValue("project_id"))
	detail, err := s.creative.GetTaskDetail(r.Context(), rc.Actor, projectID, taskID)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	project, err := s.projects.GetContext(r.Context(), rc.Actor, projectID)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	if err := project.ValidateBrandBound(); err != nil || project.OrganizationID != rc.Actor.OrganizationID {
		s.writeServiceError(w, r, fmt.Errorf("creative cover requires active brand-bound project"))
		return
	}
	if body.ImagePlanOrder > len(detail.Draft.ImagePlan) {
		s.badRequest(w, r, fmt.Errorf("image_plan_order does not exist in the current draft"))
		return
	}
	prompt := imagePlanPrompt(detail, body.ImagePlanOrder)
	requestBody := struct {
		TaskID                string `json:"task_id"`
		ModelAlias            string `json:"model_alias"`
		Prompt                string `json:"prompt"`
		ProjectContextVersion int64  `json:"project_context_version"`
		ImagePlanOrder        int    `json:"image_plan_order"`
	}{TaskID: taskID, ModelAlias: modelAlias, Prompt: prompt, ProjectContextVersion: project.ProjectContextVersion, ImagePlanOrder: body.ImagePlanOrder}
	hash, err := contract.CanonicalJSONHash(requestBody)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	job, _, err := s.providerJobs.CreateImageJob(r.Context(), provider.CreateImageJobRequest{
		Actor: rc.Actor, Project: project, IdempotencyKey: key, RequestHash: hash, ModelAlias: modelAlias,
		SourceSystem: "creative", SourceTaskID: detail.Task.ID,
		Input: provider.ImageGenerationInput{Prompt: prompt, Width: 1024, Height: 1024},
	})
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	if err := s.creative.RegisterImagePlanJob(r.Context(), rc.Actor, projectID, taskID, body.ImagePlanOrder, job.ID); err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) createCreativeRenderJob(w http.ResponseWriter, r *http.Request, taskID string) {
	if taskID == "" || s.providerJobs == nil {
		s.notFound(w, r)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	projectID := contract.ProjectID(r.PathValue("project_id"))
	detail, err := s.creative.GetTaskDetail(r.Context(), rc.Actor, projectID, taskID)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	providerJobID := ""
	for _, job := range detail.ProductionJobs {
		if job.Kind == "video_generate" {
			providerJobID = job.ProviderJobID
		}
	}
	if providerJobID == "" {
		s.writeServiceError(w, r, creative.ErrInvalidState)
		return
	}
	providerJob, err := s.providerJobs.GetJob(r.Context(), rc.Actor.OrganizationID, projectID, providerJobID)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	if providerJob.ProviderStatus != contract.ProviderJobSucceeded || len(providerJob.ProjectAssetRefs) != 1 {
		s.writeServiceError(w, r, creative.ErrInvalidState)
		return
	}
	value, duplicate, err := s.creative.CreateRenderJob(r.Context(), rc, projectID, taskID, creative.CreateRenderJobRequest{
		PreRollVideo: providerJob.ProjectAssetRefs[0].AssetVersion,
	}, key)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	if duplicate {
		w.Header().Set("Idempotent-Replay", "true")
	}
	w.Header().Set("Location", fmt.Sprintf("/api/creative/v1/projects/%s/creative-render-jobs/%s", projectID, value.ID))
	writeJSON(w, http.StatusAccepted, value)
}

func (s *Server) getCreativeRenderJob(w http.ResponseWriter, r *http.Request) {
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.GetRenderJob(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), r.PathValue("render_job_id"))
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) createCreativeVideoJob(w http.ResponseWriter, r *http.Request, taskID string) {
	if taskID == "" || s.providerJobs == nil || s.projects == nil {
		s.notFound(w, r)
		return
	}
	var body creative.CreateVideoJobRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	if err := body.Validate(); err != nil {
		s.badRequest(w, r, err)
		return
	}
	modelAlias := strings.TrimSpace(body.ModelAlias)
	if modelAlias == "" {
		modelAlias = "cookies.video.standard"
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	projectID := contract.ProjectID(r.PathValue("project_id"))
	detail, err := s.creative.GetTaskDetail(r.Context(), rc.Actor, projectID, taskID)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	if detail.Task.Format != creative.FormatVideo || detail.VideoDraft == nil {
		s.writeServiceError(w, r, creative.ErrInvalidState)
		return
	}
	project, err := s.projects.GetContext(r.Context(), rc.Actor, projectID)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	videoInput := provider.VideoGenerationInput{
		Prompt: detail.VideoDraft.Prompt, DurationSeconds: detail.VideoDraft.DurationSeconds,
		AspectRatio: detail.VideoDraft.AspectRatio, Resolution: detail.VideoDraft.Resolution,
	}
	var requestBody any = struct {
		TaskID                string              `json:"task_id"`
		ModelAlias            string              `json:"model_alias"`
		Draft                 creative.VideoDraft `json:"draft"`
		ProjectContextVersion int64               `json:"project_context_version"`
	}{TaskID: taskID, ModelAlias: modelAlias, Draft: *detail.VideoDraft, ProjectContextVersion: project.ProjectContextVersion}
	isViral := detail.Task.PerformanceMode == creative.PerformanceModeViralRemake && detail.VideoDraft.ViralRemake != nil
	isShortDrama := detail.Task.PerformanceMode == creative.PerformanceModeShortDramaPreroll && detail.VideoDraft.ShortDramaPreroll != nil
	isGamePreroll := detail.Task.PerformanceMode == creative.PerformanceModeGamePreroll && detail.VideoDraft.GamePreroll != nil
	isCommercePreroll := detail.Task.PerformanceMode == creative.PerformanceModeCommercePreroll && detail.VideoDraft.CommercePreroll != nil
	if isViral {
		var promptHash string
		videoInput, promptHash, err = s.creative.ViralProviderInput(r.Context(), rc.Actor, projectID, taskID)
		if err != nil {
			s.writeServiceError(w, r, err)
			return
		}
		requestBody = struct {
			TaskID                string                        `json:"task_id"`
			ModelAlias            string                        `json:"model_alias"`
			PromptPackageHash     string                        `json:"prompt_package_hash"`
			Input                 provider.VideoGenerationInput `json:"input"`
			ProjectContextVersion int64                         `json:"project_context_version"`
		}{TaskID: taskID, ModelAlias: modelAlias, PromptPackageHash: promptHash, Input: videoInput, ProjectContextVersion: project.ProjectContextVersion}
	} else if isShortDrama {
		var promptHash string
		videoInput, promptHash, err = s.creative.ShortDramaProviderInput(r.Context(), rc.Actor, projectID, taskID)
		if err != nil {
			s.writeServiceError(w, r, err)
			return
		}
		requestBody = struct {
			TaskID                string                        `json:"task_id"`
			ModelAlias            string                        `json:"model_alias"`
			PromptPackageHash     string                        `json:"prompt_package_hash"`
			Input                 provider.VideoGenerationInput `json:"input"`
			ProjectContextVersion int64                         `json:"project_context_version"`
		}{TaskID: taskID, ModelAlias: modelAlias, PromptPackageHash: promptHash, Input: videoInput, ProjectContextVersion: project.ProjectContextVersion}
	} else if isGamePreroll {
		var promptHash string
		videoInput, promptHash, err = s.creative.GamePrerollProviderInput(r.Context(), rc.Actor, projectID, taskID)
		if err != nil {
			s.writeServiceError(w, r, err)
			return
		}
		requestBody = struct {
			TaskID                string                        `json:"task_id"`
			ModelAlias            string                        `json:"model_alias"`
			PromptPackageHash     string                        `json:"prompt_package_hash"`
			Input                 provider.VideoGenerationInput `json:"input"`
			ProjectContextVersion int64                         `json:"project_context_version"`
		}{TaskID: taskID, ModelAlias: modelAlias, PromptPackageHash: promptHash, Input: videoInput, ProjectContextVersion: project.ProjectContextVersion}
	} else if isCommercePreroll {
		var promptHash string
		videoInput, promptHash, err = s.creative.CommerceProviderInput(r.Context(), rc.Actor, projectID, taskID)
		if err != nil {
			s.writeServiceError(w, r, err)
			return
		}
		requestBody = struct {
			TaskID                string                        `json:"task_id"`
			ModelAlias            string                        `json:"model_alias"`
			PromptPackageHash     string                        `json:"prompt_package_hash"`
			Input                 provider.VideoGenerationInput `json:"input"`
			ProjectContextVersion int64                         `json:"project_context_version"`
		}{TaskID: taskID, ModelAlias: modelAlias, PromptPackageHash: promptHash, Input: videoInput, ProjectContextVersion: project.ProjectContextVersion}
	} else if body.GenerationSpec != nil {
		videoInput, err = body.ProviderInput(projectID, taskID)
		if err != nil {
			s.badRequest(w, r, err)
			return
		}
		requestBody = struct {
			TaskID                string                               `json:"task_id"`
			ModelAlias            string                               `json:"model_alias"`
			PromptHash            string                               `json:"prompt_hash"`
			GenerationSpec        creative.CreativeVideoGenerationSpec `json:"generation_spec"`
			Approval              creative.VideoGenerationApproval     `json:"approval"`
			ProjectContextVersion int64                                `json:"project_context_version"`
		}{
			TaskID: taskID, ModelAlias: modelAlias, PromptHash: body.Prompt.Hash,
			GenerationSpec: *body.GenerationSpec, Approval: *body.Approval,
			ProjectContextVersion: project.ProjectContextVersion,
		}
	}
	hash, err := contract.CanonicalJSONHash(requestBody)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	job, _, err := s.providerJobs.CreateVideoJob(r.Context(), provider.CreateVideoJobRequest{
		Actor: rc.Actor, Project: project, IdempotencyKey: key, RequestHash: hash,
		ModelAlias: modelAlias, SourceSystem: "creative", SourceTaskID: taskID,
		Input: videoInput,
	})
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	if isViral {
		if _, err := s.creative.RegisterViralCandidateJob(r.Context(), rc.Actor, projectID, taskID, job.ID); err != nil {
			s.writeServiceError(w, r, err)
			return
		}
	} else if isShortDrama {
		if _, err := s.creative.RegisterShortDramaGenerationAttempt(r.Context(), rc.Actor, projectID, taskID, job.ID); err != nil {
			s.writeServiceError(w, r, err)
			return
		}
	} else if isGamePreroll {
		if _, err := s.creative.RegisterGamePrerollGenerationAttempt(r.Context(), rc.Actor, projectID, taskID, job.ID); err != nil {
			s.writeServiceError(w, r, err)
			return
		}
	} else if isCommercePreroll {
		if _, err := s.creative.RegisterCommerceGenerationAttempt(r.Context(), rc.Actor, projectID, taskID, job.ID); err != nil {
			s.writeServiceError(w, r, err)
			return
		}
	} else if err := s.creative.RegisterVideoJob(r.Context(), rc.Actor, projectID, taskID, job.ID); err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) bindCreativeImageAsset(w http.ResponseWriter, r *http.Request, taskID string) {
	if taskID == "" || s.uploads == nil {
		s.notFound(w, r)
		return
	}
	var body creative.BindImageAssetRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	projectID := contract.ProjectID(r.PathValue("project_id"))
	// Preview reuses the Asset boundary's authorization and ready-state check;
	// the URL is discarded because Creative retains only an immutable ref.
	if _, err := s.uploads.Preview(r.Context(), rc.Actor, projectID, body.AssetRef); err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	value, err := s.creative.BindImageAsset(r.Context(), rc.Actor, projectID, taskID, body)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) freezeCreativeVersion(w http.ResponseWriter, r *http.Request, taskID string) {
	if taskID == "" {
		s.notFound(w, r)
		return
	}
	var body creative.FreezeVersionRequest
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, duplicate, err := s.creative.FreezeVersion(r.Context(), rc, contract.ProjectID(r.PathValue("project_id")), taskID, body, key)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	if duplicate {
		w.Header().Set("Idempotent-Replay", "true")
	}
	w.Header().Set("Location", fmt.Sprintf("/api/creative/v1/projects/%s/creative-versions/%s/versions/%d", r.PathValue("project_id"), value.ID, value.Version))
	writeJSON(w, http.StatusCreated, value)
}

func coverPrompt(detail creative.TaskDetail) string {
	brief := ""
	if len(detail.Draft.ImagePlan) > 0 {
		brief = detail.Draft.ImagePlan[0].VisualBrief
	}
	return strings.TrimSpace(fmt.Sprintf("%s。小红书封面，%s。封面文字区域：%s。", brief, strings.Join(detail.Task.Direction.Tone, "、"), detail.Draft.CoverCopy))
}

func imagePlanPrompt(detail creative.TaskDetail, imagePlanOrder int) string {
	brief := ""
	caption := detail.Draft.CoverCopy
	if imagePlanOrder >= 1 && imagePlanOrder <= len(detail.Draft.ImagePlan) {
		item := detail.Draft.ImagePlan[imagePlanOrder-1]
		brief = item.VisualBrief
		caption = item.Caption
	}
	return strings.TrimSpace(fmt.Sprintf("%s. Xiaohongshu image %d of %d. Tone: %s. Caption area: %s.", brief, imagePlanOrder, len(detail.Draft.ImagePlan), strings.Join(detail.Task.Direction.Tone, ", "), caption))
}

func (s *Server) transitionCreativeVersion(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	action := r.PathValue("version_action")
	projectID := contract.ProjectID(r.PathValue("project_id"))
	rc, _ := contract.RequestContextFrom(r.Context())
	var value any
	var err error
	switch {
	case strings.HasSuffix(action, ":check"):
		value, err = s.creative.CheckVersion(r.Context(), rc.Actor, projectID, strings.TrimSuffix(action, ":check"))
	case strings.HasSuffix(action, ":approve"):
		value, err = s.creative.ApproveVersion(r.Context(), rc.Actor, projectID, strings.TrimSuffix(action, ":approve"))
	case strings.HasSuffix(action, ":deliver"):
		value, err = s.creative.DeliverVersion(r.Context(), rc.Actor, projectID, strings.TrimSuffix(action, ":deliver"))
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

func (s *Server) listCreativeVersions(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	values, err := s.creative.ListVersions(
		r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")),
		strings.TrimSpace(r.URL.Query().Get("task_id")), queryLimit(r),
	)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) listCreativePackages(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	values, err := s.creative.ListPackages(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), queryLimit(r))
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": values})
}

func queryLimit(r *http.Request) int {
	value, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || value < 1 {
		return 50
	}
	if value > 100 {
		return 100
	}
	return value
}
