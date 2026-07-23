package httpserver

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/systems/creative"
)

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
	if err := body.Validate(); err != nil {
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

func (s *Server) createCreativeTask(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil {
		s.notImplemented(w, r)
		return
	}
	action := r.PathValue("intake_action")
	if !strings.HasSuffix(action, ":create-task") {
		s.notFound(w, r)
		return
	}
	intakeID := strings.TrimSuffix(action, ":create-task")
	if intakeID == "" {
		s.notFound(w, r)
		return
	}
	rc, _ := contract.RequestContextFrom(r.Context())
	value, err := s.creative.CreateTask(r.Context(), rc.Actor, contract.ProjectID(r.PathValue("project_id")), intakeID)
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

func (s *Server) createCreativeCoverImageJob(w http.ResponseWriter, r *http.Request) {
	if s.creative == nil || s.providerJobs == nil || s.projects == nil {
		s.notImplemented(w, r)
		return
	}
	action := r.PathValue("task_action")
	if !strings.HasSuffix(action, ":cover-image-job") {
		s.notFound(w, r)
		return
	}
	taskID := strings.TrimSuffix(action, ":cover-image-job")
	if taskID == "" {
		s.notFound(w, r)
		return
	}
	var body struct {
		ModelAlias string `json:"model_alias"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		s.badRequest(w, r, err)
		return
	}
	modelAlias := strings.TrimSpace(body.ModelAlias)
	if modelAlias == "" {
		modelAlias = "cookies.image.standard"
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
	prompt := coverPrompt(detail)
	requestBody := struct {
		TaskID                string `json:"task_id"`
		ModelAlias            string `json:"model_alias"`
		Prompt                string `json:"prompt"`
		ProjectContextVersion int64  `json:"project_context_version"`
	}{TaskID: taskID, ModelAlias: modelAlias, Prompt: prompt, ProjectContextVersion: project.ProjectContextVersion}
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
	if err := s.creative.RegisterCoverImageJob(r.Context(), rc.Actor, projectID, taskID, job.ID); err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func coverPrompt(detail creative.TaskDetail) string {
	brief := ""
	if len(detail.Draft.ImagePlan) > 0 {
		brief = detail.Draft.ImagePlan[0].VisualBrief
	}
	return strings.TrimSpace(fmt.Sprintf("%s。小红书封面，%s。封面文字区域：%s。", brief, strings.Join(detail.Task.Direction.Tone, "、"), detail.Draft.CoverCopy))
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
