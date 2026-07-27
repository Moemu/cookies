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
	requestBody := struct {
		TaskID                string              `json:"task_id"`
		ModelAlias            string              `json:"model_alias"`
		Draft                 creative.VideoDraft `json:"draft"`
		ProjectContextVersion int64               `json:"project_context_version"`
	}{TaskID: taskID, ModelAlias: modelAlias, Draft: *detail.VideoDraft, ProjectContextVersion: project.ProjectContextVersion}
	hash, err := contract.CanonicalJSONHash(requestBody)
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	job, _, err := s.providerJobs.CreateVideoJob(r.Context(), provider.CreateVideoJobRequest{
		Actor: rc.Actor, Project: project, IdempotencyKey: key, RequestHash: hash,
		ModelAlias: modelAlias, SourceSystem: "creative", SourceTaskID: taskID,
		Input: provider.VideoGenerationInput{
			Prompt: detail.VideoDraft.Prompt, DurationSeconds: detail.VideoDraft.DurationSeconds,
			AspectRatio: detail.VideoDraft.AspectRatio, Resolution: detail.VideoDraft.Resolution,
		},
	})
	if err != nil {
		s.writeServiceError(w, r, err)
		return
	}
	if err := s.creative.RegisterVideoJob(r.Context(), rc.Actor, projectID, taskID, job.ID); err != nil {
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
