// Package httpapi exposes the authenticated Strategy v1 HTTP surface. The
// platform server supplies trusted RequestContext; this package still enforces
// Strategy scopes and project authorization through the application service.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/agent"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
	"github.com/shikanon/cookies/internal/systems/strategy"
)

type Server struct {
	Service    strategy.Service
	Agents     agent.MySQLStore
	Jobs       jobruntime.MySQLStore
	mux        *http.ServeMux
	PollPeriod time.Duration
}

func New(service strategy.Service, agents agent.MySQLStore, jobs jobruntime.MySQLStore) *Server {
	server := &Server{Service: service, Agents: agents, Jobs: jobs, PollPeriod: time.Second}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/strategy/v1/workspaces", server.createWorkspace)
	mux.HandleFunc("GET /api/strategy/v1/projects/{project_id}/workspaces", server.listWorkspaces)
	mux.HandleFunc("GET /api/strategy/v1/workspaces/{workspace_id}", server.getWorkspace)
	mux.HandleFunc("POST /api/strategy/v1/conversations", server.createConversation)
	mux.HandleFunc("GET /api/strategy/v1/conversations/{conversation_id}", server.getConversation)
	mux.HandleFunc("GET /api/strategy/v1/conversations/{conversation_id}/memory", server.getConversationMemory)
	mux.HandleFunc("GET /api/strategy/v1/conversations/{conversation_id}/messages", server.listMessages)
	mux.HandleFunc("POST /api/strategy/v1/conversations/{conversation_id}/messages", server.sendMessage)
	mux.HandleFunc("GET /api/strategy/v1/conversations/{conversation_id}/events", server.streamEvents)
	mux.HandleFunc("GET /api/strategy/v1/agent-tasks/{agent_task_id}", server.getAgentTask)
	mux.HandleFunc("GET /api/strategy/v1/agent-tasks/{agent_task_id}/skill-runs", server.listSkillRuns)
	mux.HandleFunc("POST /api/strategy/v1/agent-tasks/{agent_task_action}", server.cancelAgentTask)
	mux.HandleFunc("GET /api/strategy/v1/tasks/{task_id}", server.getTask)
	mux.HandleFunc("GET /api/strategy/v1/tasks/{task_id}/brief-draft", server.getBriefDraft)
	mux.HandleFunc("PATCH /api/strategy/v1/tasks/{task_id}/brief-draft", server.patchBriefDraft)
	mux.HandleFunc("POST /api/strategy/v1/tasks/{task_id}/brief:confirm", server.confirmBrief)
	mux.HandleFunc("GET /api/strategy/v1/briefs/{brief_id}/versions", server.listBriefVersions)
	mux.HandleFunc("GET /api/strategy/v1/briefs/{brief_id}/versions/{version}", server.getBriefVersion)
	mux.HandleFunc("POST /api/strategy/v1/tasks/{task_id}/strategies", server.createStrategy)
	mux.HandleFunc("GET /api/strategy/v1/projects/{project_id}/generation-readiness", server.getGenerationReadiness)
	mux.HandleFunc("GET /api/strategy/v1/strategy-drafts/{strategy_id}", server.getStrategy)
	mux.HandleFunc("GET /api/strategy/v1/strategy-drafts/{strategy_id}/generation-metadata", server.getGenerationMetadata)
	mux.HandleFunc("GET /api/strategy/v1/strategy-drafts/{strategy_id}/revisions", server.listStrategyRevisions)
	mux.HandleFunc("GET /api/strategy/v1/strategy-drafts/{strategy_id}/revisions/{revision}", server.getStrategyRevision)
	mux.HandleFunc("PATCH /api/strategy/v1/strategy-drafts/{strategy_id}", server.patchStrategy)
	mux.HandleFunc("POST /api/strategy/v1/strategy-drafts/{strategy_action}", server.strategyAction)
	mux.HandleFunc("GET /api/strategy/v1/strategy-reviews/{review_id}", server.getReview)
	mux.HandleFunc("GET /api/strategy/v1/strategy-reviews/{review_id}/comments", server.listReviewComments)
	mux.HandleFunc("POST /api/strategy/v1/strategy-reviews/{review_id}/comments", server.addReviewComment)
	mux.HandleFunc("POST /api/strategy/v1/strategy-reviews/{review_action}", server.reviewAction)
	mux.HandleFunc("GET /api/strategy/v1/projects/{project_id}/strategy-packages", server.listPackages)
	mux.HandleFunc("GET /api/strategy/v1/projects/{project_id}/strategy-packages/{package_id}/versions/{version}", server.getPackage)
	mux.HandleFunc("GET /api/strategy/v1/projects/{project_id}/strategy-packages/{package_id}/versions/{version}/export.md", server.exportPackage)
	mux.HandleFunc("POST /api/strategy/v1/projects/{project_id}/feedback", server.createFeedback)
	mux.HandleFunc("GET /api/strategy/v1/projects/{project_id}/feedback", server.listFeedback)
	server.mux = mux
	return server
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.mux.ServeHTTP(writer, request)
}

func actor(request *http.Request) (contract.ActorContext, bool) {
	requestContext, ok := contract.RequestContextFrom(request.Context())
	return requestContext.Actor, ok
}

func (s *Server) createWorkspace(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		ProjectID contract.ProjectID `json:"project_id"`
		Name      string             `json:"name"`
	}
	if !decode(writer, request, &body) {
		return
	}
	value, duplicate, err := s.Service.CreateWorkspace(request.Context(), mustActor(request), idempotencyKey(request), body.ProjectID, body.Name)
	if err != nil {
		writeError(writer, err)
		return
	}
	writer.Header().Set("Location", "/api/strategy/v1/workspaces/"+value.ID)
	if duplicate {
		writer.Header().Set("Idempotent-Replay", "true")
	}
	writeJSON(writer, http.StatusCreated, value)
}

func (s *Server) listWorkspaces(writer http.ResponseWriter, request *http.Request) {
	values, err := s.Service.ListWorkspaces(request.Context(), mustActor(request), contract.ProjectID(request.PathValue("project_id")))
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) getWorkspace(writer http.ResponseWriter, request *http.Request) {
	value, err := s.Service.GetWorkspaceDetail(request.Context(), mustActor(request), request.PathValue("workspace_id"))
	writeResult(writer, value, err)
}

func (s *Server) getGenerationReadiness(writer http.ResponseWriter, request *http.Request) {
	value, err := s.Service.GetGenerationReadiness(
		request.Context(), mustActor(request), contract.ProjectID(request.PathValue("project_id")),
	)
	writeResult(writer, value, err)
}

func (s *Server) getGenerationMetadata(writer http.ResponseWriter, request *http.Request) {
	value, err := s.Service.GetGenerationMetadata(
		request.Context(), mustActor(request), request.PathValue("strategy_id"),
	)
	writeResult(writer, value, err)
}

func (s *Server) createConversation(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		ProjectID   contract.ProjectID `json:"project_id"`
		WorkspaceID string             `json:"workspace_id"`
	}
	if !decode(writer, request, &body) {
		return
	}
	value, duplicate, err := s.Service.CreateConversation(request.Context(), mustActor(request), idempotencyKey(request), body.ProjectID, body.WorkspaceID)
	if err != nil {
		writeError(writer, err)
		return
	}
	if duplicate {
		writer.Header().Set("Idempotent-Replay", "true")
	}
	writer.Header().Set("Location", "/api/strategy/v1/conversations/"+value.Conversation.ID)
	writeJSON(writer, http.StatusCreated, value)
}

func (s *Server) getConversation(writer http.ResponseWriter, request *http.Request) {
	value, err := s.Service.GetConversation(request.Context(), mustActor(request), request.PathValue("conversation_id"))
	writeResult(writer, value, err)
}

func (s *Server) getConversationMemory(writer http.ResponseWriter, request *http.Request) {
	value, err := s.Service.GetConversationMemory(
		request.Context(), mustActor(request), request.PathValue("conversation_id"),
	)
	writeResult(writer, value, err)
}

func (s *Server) listMessages(writer http.ResponseWriter, request *http.Request) {
	limit, _ := strconv.Atoi(request.URL.Query().Get("limit"))
	values, err := s.Service.ListMessages(request.Context(), mustActor(request), request.PathValue("conversation_id"), request.URL.Query().Get("cursor"), limit)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) sendMessage(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Content string `json:"content"`
	}
	if !decode(writer, request, &body) {
		return
	}
	value, duplicate, err := s.Service.SendMessage(request.Context(), mustActor(request), idempotencyKey(request), request.PathValue("conversation_id"), body.Content)
	if err != nil {
		writeError(writer, err)
		return
	}
	if duplicate {
		writer.Header().Set("Idempotent-Replay", "true")
	}
	writeJSON(writer, http.StatusAccepted, value)
}

func (s *Server) getTask(writer http.ResponseWriter, request *http.Request) {
	value, err := s.Service.GetTask(request.Context(), mustActor(request), request.PathValue("task_id"))
	writeResult(writer, value, err)
}

func (s *Server) listSkillRuns(writer http.ResponseWriter, request *http.Request) {
	values, err := s.Service.ListSkillRuns(
		request.Context(), mustActor(request), request.PathValue("agent_task_id"),
	)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) createFeedback(writer http.ResponseWriter, request *http.Request) {
	var body strategy.CreateFeedbackRequest
	if !decode(writer, request, &body) {
		return
	}
	value, duplicate, err := s.Service.CreateFeedback(
		request.Context(), mustActor(request), contract.ProjectID(request.PathValue("project_id")),
		idempotencyKey(request), body,
	)
	if err != nil {
		writeError(writer, err)
		return
	}
	if duplicate {
		writer.Header().Set("Idempotent-Replay", "true")
	}
	writeJSON(writer, http.StatusCreated, value)
}

func (s *Server) listFeedback(writer http.ResponseWriter, request *http.Request) {
	version, err := strconv.ParseInt(request.URL.Query().Get("target_version"), 10, 64)
	if err != nil || version < 1 {
		writeError(writer, strategy.ErrInvalidRequest)
		return
	}
	values, err := s.Service.ListFeedback(
		request.Context(), mustActor(request), contract.ProjectID(request.PathValue("project_id")),
		request.URL.Query().Get("target_type"), request.URL.Query().Get("target_id"), version,
	)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) getBriefDraft(writer http.ResponseWriter, request *http.Request) {
	value, err := s.Service.GetTaskBriefDraft(request.Context(), mustActor(request), request.PathValue("task_id"))
	if err == nil {
		writer.Header().Set("ETag", fmt.Sprintf(`"v%d"`, value.Version))
	}
	writeResult(writer, value, err)
}

func (s *Server) patchBriefDraft(writer http.ResponseWriter, request *http.Request) {
	var body strategy.BriefPatch
	if !decode(writer, request, &body) {
		return
	}
	if body.ExpectedVersion == 0 {
		body.ExpectedVersion = parseIfMatch(request.Header.Get("If-Match"))
	}
	value, duplicate, err := s.Service.PatchBriefDraft(request.Context(), mustActor(request), idempotencyKey(request), request.PathValue("task_id"), body)
	if err != nil {
		writeError(writer, err)
		return
	}
	if duplicate {
		writer.Header().Set("Idempotent-Replay", "true")
	}
	writer.Header().Set("ETag", fmt.Sprintf(`"v%d"`, value.Version))
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) confirmBrief(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		ExpectedVersion int64 `json:"expected_version"`
	}
	if !decode(writer, request, &body) {
		return
	}
	value, duplicate, err := s.Service.ConfirmBrief(request.Context(), mustActor(request), idempotencyKey(request), request.PathValue("task_id"), body.ExpectedVersion)
	if err != nil {
		writeError(writer, err)
		return
	}
	if duplicate {
		writer.Header().Set("Idempotent-Replay", "true")
	}
	writeJSON(writer, http.StatusCreated, value)
}

func (s *Server) listBriefVersions(writer http.ResponseWriter, request *http.Request) {
	values, err := s.Service.ListBriefVersions(request.Context(), mustActor(request), request.PathValue("brief_id"))
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) getBriefVersion(writer http.ResponseWriter, request *http.Request) {
	version, ok := positivePathInt(writer, request, "version")
	if !ok {
		return
	}
	value, err := s.Service.GetBriefVersion(request.Context(), mustActor(request), request.PathValue("brief_id"), version)
	if err == nil {
		writer.Header().Set("ETag", `"`+string(value.ContentHash)+`"`)
		writer.Header().Set("Cache-Control", "private, no-cache")
	}
	writeResult(writer, value, err)
}

func (s *Server) createStrategy(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		BriefID      string `json:"brief_id"`
		BriefVersion int64  `json:"brief_version"`
	}
	if !decode(writer, request, &body) {
		return
	}
	value, duplicate, err := s.Service.CreateStrategy(request.Context(), mustActor(request), idempotencyKey(request), request.PathValue("task_id"), body.BriefID, body.BriefVersion)
	if err != nil {
		writeError(writer, err)
		return
	}
	if duplicate {
		writer.Header().Set("Idempotent-Replay", "true")
	}
	writeJSON(writer, http.StatusAccepted, value)
}

func (s *Server) getStrategy(writer http.ResponseWriter, request *http.Request) {
	value, err := s.Service.GetDraft(request.Context(), mustActor(request), request.PathValue("strategy_id"))
	if err == nil {
		writer.Header().Set("ETag", fmt.Sprintf(`"v%d"`, value.Version))
	}
	writeResult(writer, value, err)
}

func (s *Server) listStrategyRevisions(writer http.ResponseWriter, request *http.Request) {
	values, err := s.Service.ListDraftRevisions(request.Context(), mustActor(request), request.PathValue("strategy_id"))
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) getStrategyRevision(writer http.ResponseWriter, request *http.Request) {
	revision, ok := positivePathInt(writer, request, "revision")
	if !ok {
		return
	}
	value, err := s.Service.GetDraftRevision(request.Context(), mustActor(request), request.PathValue("strategy_id"), revision)
	writeResult(writer, value, err)
}

func (s *Server) patchStrategy(writer http.ResponseWriter, request *http.Request) {
	var body strategy.StrategySectionPatch
	if !decode(writer, request, &body) {
		return
	}
	value, duplicate, err := s.Service.PatchStrategy(request.Context(), mustActor(request), idempotencyKey(request), request.PathValue("strategy_id"), body)
	if err != nil {
		writeError(writer, err)
		return
	}
	if duplicate {
		writer.Header().Set("Idempotent-Replay", "true")
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) reviseStrategy(writer http.ResponseWriter, request *http.Request) {
	var body strategy.ReviseRequest
	if !decode(writer, request, &body) {
		return
	}
	value, duplicate, err := s.Service.ReviseStrategy(request.Context(), mustActor(request), idempotencyKey(request), request.PathValue("strategy_id"), body)
	if err != nil {
		writeError(writer, err)
		return
	}
	if duplicate {
		writer.Header().Set("Idempotent-Replay", "true")
	}
	writeJSON(writer, http.StatusAccepted, value)
}

func (s *Server) strategyAction(writer http.ResponseWriter, request *http.Request) {
	value := request.PathValue("strategy_action")
	switch {
	case strings.HasSuffix(value, ":revise"):
		request.SetPathValue("strategy_id", strings.TrimSuffix(value, ":revise"))
		s.reviseStrategy(writer, request)
	case strings.HasSuffix(value, ":submit"):
		request.SetPathValue("strategy_id", strings.TrimSuffix(value, ":submit"))
		s.submitStrategy(writer, request)
	case strings.HasSuffix(value, ":approve"):
		request.SetPathValue("strategy_id", strings.TrimSuffix(value, ":approve"))
		s.approveStrategy(writer, request)
	default:
		writeError(writer, strategy.ErrNotFound)
	}
}

func (s *Server) submitStrategy(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		ExpectedVersion   int64 `json:"expected_version"`
		CandidateRevision int64 `json:"candidate_revision"`
	}
	if !decode(writer, request, &body) {
		return
	}
	value, duplicate, err := s.Service.SubmitStrategy(request.Context(), mustActor(request), idempotencyKey(request), request.PathValue("strategy_id"), body.ExpectedVersion, body.CandidateRevision)
	if err != nil {
		writeError(writer, err)
		return
	}
	if duplicate {
		writer.Header().Set("Idempotent-Replay", "true")
	}
	writeJSON(writer, http.StatusCreated, value)
}

func (s *Server) getReview(writer http.ResponseWriter, request *http.Request) {
	value, err := s.Service.GetReview(request.Context(), mustActor(request), request.PathValue("review_id"))
	writeResult(writer, value, err)
}

func (s *Server) addReviewComment(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Body string `json:"body"`
	}
	if !decode(writer, request, &body) {
		return
	}
	value, err := s.Service.AddReviewComment(request.Context(), mustActor(request), request.PathValue("review_id"), body.Body)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusCreated, value)
}

func (s *Server) listReviewComments(writer http.ResponseWriter, request *http.Request) {
	values, err := s.Service.ListReviewComments(
		request.Context(), mustActor(request), request.PathValue("review_id"),
	)
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) returnReview(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		Reason string `json:"reason"`
	}
	if !decode(writer, request, &body) {
		return
	}
	value, err := s.Service.ReturnReview(request.Context(), mustActor(request), request.PathValue("review_id"), body.Reason)
	writeResult(writer, value, err)
}

func (s *Server) reviewAction(writer http.ResponseWriter, request *http.Request) {
	value := request.PathValue("review_action")
	if !strings.HasSuffix(value, ":return") {
		writeError(writer, strategy.ErrNotFound)
		return
	}
	request.SetPathValue("review_id", strings.TrimSuffix(value, ":return"))
	s.returnReview(writer, request)
}

func (s *Server) approveStrategy(writer http.ResponseWriter, request *http.Request) {
	var body strategy.ApproveRequest
	if !decode(writer, request, &body) {
		return
	}
	value, duplicate, err := s.Service.ApproveStrategy(request.Context(), mustActor(request), idempotencyKey(request), request.PathValue("strategy_id"), body)
	if err != nil {
		writeError(writer, err)
		return
	}
	if duplicate {
		writer.Header().Set("Idempotent-Replay", "true")
	}
	writeJSON(writer, http.StatusCreated, value)
}

func (s *Server) listPackages(writer http.ResponseWriter, request *http.Request) {
	values, err := s.Service.ListPackages(request.Context(), mustActor(request), contract.ProjectID(request.PathValue("project_id")))
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) getPackage(writer http.ResponseWriter, request *http.Request) {
	value, ok := s.packageFromRequest(writer, request)
	if !ok {
		return
	}
	if request.Header.Get("If-None-Match") == `"`+string(value.ContentHash)+`"` {
		writer.WriteHeader(http.StatusNotModified)
		return
	}
	writer.Header().Set("ETag", `"`+string(value.ContentHash)+`"`)
	writer.Header().Set("Cache-Control", "private, no-cache")
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) exportPackage(writer http.ResponseWriter, request *http.Request) {
	value, ok := s.packageFromRequest(writer, request)
	if !ok {
		return
	}
	writer.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="strategy-%s-v%d.md"`, value.PackageID, value.Version))
	writer.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(writer, strategy.ExportPackageMarkdown(value))
}

func (s *Server) packageFromRequest(writer http.ResponseWriter, request *http.Request) (strategy.PackageVersion, bool) {
	version, ok := positivePathInt(writer, request, "version")
	if !ok {
		return strategy.PackageVersion{}, false
	}
	value, err := s.Service.GetPackage(request.Context(), mustActor(request), contract.ProjectID(request.PathValue("project_id")), request.PathValue("package_id"), version)
	if err != nil {
		writeError(writer, err)
		return strategy.PackageVersion{}, false
	}
	return value, true
}

func (s *Server) getAgentTask(writer http.ResponseWriter, request *http.Request) {
	actor := mustActor(request)
	// Resource-only routes derive project scope without accepting it from the
	// caller; the first query is organization-scoped and project authorization
	// runs before the complete task is returned.
	task, err := s.getAgentByOrganization(request.Context(), actor, request.PathValue("agent_task_id"), strategy.ScopeRead)
	if err != nil {
		writeError(writer, mapAgentError(err))
		return
	}
	if _, err := s.Service.GetTask(request.Context(), actor, task.SourceID); err != nil && task.SourceType == "strategy_task" {
		writeError(writer, err)
		return
	}
	var job any
	if task.JobID != "" {
		job, _ = s.Jobs.Get(request.Context(), actor.OrganizationID, task.ProjectID, task.JobID)
	}
	writeJSON(writer, http.StatusOK, map[string]any{"task": task, "job": job})
}

func (s *Server) getAgentByOrganization(ctx context.Context, actor contract.ActorContext, id string, scope contract.Scope) (agent.Task, error) {
	var projectID contract.ProjectID
	if err := s.Service.DB.QueryRowContext(ctx, `SELECT project_id FROM platform_agent_tasks WHERE organization_id = ? AND id = ?`, actor.OrganizationID, id).Scan(&projectID); err != nil {
		return agent.Task{}, agent.ErrNotFound
	}
	if err := s.Service.AuthorizeProject(ctx, actor, projectID, scope); err != nil {
		return agent.Task{}, err
	}
	return s.Agents.Get(ctx, actor.OrganizationID, projectID, id)
}

func (s *Server) cancelAgentTask(writer http.ResponseWriter, request *http.Request) {
	var body struct {
		ExpectedVersion int64 `json:"expected_version"`
	}
	if !decode(writer, request, &body) {
		return
	}
	actor := mustActor(request)
	action := request.PathValue("agent_task_action")
	if !strings.HasSuffix(action, ":cancel") {
		writeError(writer, strategy.ErrNotFound)
		return
	}
	agentTaskID := strings.TrimSuffix(action, ":cancel")
	task, err := s.getAgentByOrganization(request.Context(), actor, agentTaskID, strategy.ScopeWrite)
	if err != nil {
		writeError(writer, mapAgentError(err))
		return
	}
	if task.Version != body.ExpectedVersion {
		writeError(writer, strategy.ErrVersionConflict)
		return
	}
	if task.JobID != "" {
		job, err := s.Jobs.Get(request.Context(), actor.OrganizationID, task.ProjectID, task.JobID)
		if err == nil {
			_, err = s.Jobs.RequestCancel(request.Context(), actor.OrganizationID, task.ProjectID, task.JobID, job.Version, time.Now().UTC())
		}
		if err != nil && !errors.Is(err, jobruntime.ErrJobNotCancellable) {
			writeError(writer, err)
			return
		}
	}
	value, err := s.Agents.RequestCancel(request.Context(), actor.OrganizationID, task.ProjectID, task.ID, task.Version, time.Now().UTC())
	if err != nil && task.Status == agent.TaskRunning {
		value, err = task, nil
	}
	if err != nil {
		writeError(writer, mapAgentError(err))
		return
	}
	writeJSON(writer, http.StatusAccepted, value)
}

func (s *Server) streamEvents(writer http.ResponseWriter, request *http.Request) {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		writeError(writer, fmt.Errorf("streaming unsupported"))
		return
	}
	lastID, _ := strconv.ParseInt(request.Header.Get("Last-Event-ID"), 10, 64)
	if err := s.Service.ValidateConversationCursor(request.Context(), mustActor(request), request.PathValue("conversation_id"), lastID); err != nil {
		writeError(writer, err)
		return
	}
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("X-Accel-Buffering", "no")
	writer.WriteHeader(http.StatusOK)
	controller := http.NewResponseController(writer)
	poll := s.PollPeriod
	if poll <= 0 {
		poll = time.Second
	}
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	send := func() error {
		events, err := s.Service.ListConversationEvents(request.Context(), mustActor(request), request.PathValue("conversation_id"), lastID, 100)
		if err != nil {
			return err
		}
		for _, event := range events {
			_ = controller.SetWriteDeadline(time.Now().Add(20 * time.Second))
			if _, err := fmt.Fprintf(writer, "id: %d\nevent: %s\ndata: %s\n\n", event.Sequence, event.Type, event.Payload); err != nil {
				return err
			}
			lastID = event.Sequence
		}
		flusher.Flush()
		return nil
	}
	if err := send(); err != nil {
		return
	}
	for {
		select {
		case <-request.Context().Done():
			return
		case <-ticker.C:
			if err := send(); err != nil {
				return
			}
		case <-heartbeat.C:
			_ = controller.SetWriteDeadline(time.Now().Add(20 * time.Second))
			if _, err := io.WriteString(writer, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func decode(writer http.ResponseWriter, request *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(writer, strategy.ErrInvalidRequest)
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(writer, strategy.ErrInvalidRequest)
		return false
	}
	return true
}

func mustActor(request *http.Request) contract.ActorContext {
	value, _ := actor(request)
	return value
}

func idempotencyKey(request *http.Request) contract.IdempotencyKey {
	return contract.IdempotencyKey(strings.TrimSpace(request.Header.Get("Idempotency-Key")))
}

func parseIfMatch(value string) int64 {
	value = strings.Trim(value, `" `)
	value = strings.TrimPrefix(value, "v")
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

func positivePathInt(writer http.ResponseWriter, request *http.Request, name string) (int64, bool) {
	value, err := strconv.ParseInt(request.PathValue(name), 10, 64)
	if err != nil || value < 1 {
		writeError(writer, strategy.ErrInvalidRequest)
		return 0, false
	}
	return value, true
}

func writeResult(writer http.ResponseWriter, value any, err error) {
	if err != nil {
		writeError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "INTERNAL"
	message := "服务暂时不可用，请稍后重试"
	retryable := true
	var details []strategy.ValidationError
	switch {
	case errors.Is(err, strategy.ErrInvalidRequest):
		status, code, message, retryable = 400, "INVALID_REQUEST", "请求参数无效", false
	case errors.Is(err, strategy.ErrScopeRequired):
		status, code, message, retryable = 403, "SCOPE_REQUIRED", "缺少所需的 Strategy 权限", false
	case errors.Is(err, strategy.ErrFeatureDisabled):
		status, code, message, retryable = 403, "FEATURE_DISABLED", "Strategy feature is disabled", false
	case errors.Is(err, strategy.ErrGenerationUnavailable):
		status, code, message, retryable = 503, "GENERATION_PROVIDER_UNAVAILABLE", "真实策略生成服务尚未就绪", true
	case errors.Is(err, strategy.ErrProjectAccessDenied):
		status, code, message, retryable = 403, "PROJECT_ACCESS_DENIED", "当前身份无权访问该项目", false
	case errors.Is(err, strategy.ErrNotFound):
		status, code, message, retryable = 404, "RESOURCE_NOT_FOUND", "资源不存在", false
	case errors.Is(err, strategy.ErrIdempotencyConflict):
		status, code, message, retryable = 409, "IDEMPOTENCY_CONFLICT", "幂等键已用于不同请求", false
	case errors.Is(err, strategy.ErrInvalidState):
		status, code, message, retryable = 409, "INVALID_STATE", "资源当前状态不允许该操作", false
	case errors.Is(err, strategy.ErrBriefBlocked):
		status, code, message, retryable = 409, "BRIEF_BLOCKED", "Brief 还有必须解决的信息", false
		var blocked strategy.BlockedError
		if errors.As(err, &blocked) {
			details = blocked.Problems
		}
	case errors.Is(err, strategy.ErrReviewStale):
		status, code, message, retryable = 409, "REVIEW_STALE", "评审候选版本已经失效", false
	case errors.Is(err, strategy.ErrVersionConflict):
		status, code, message, retryable = 412, "VERSION_CONFLICT", "资源已被其他操作更新", false
	case errors.Is(err, strategy.ErrConcurrencyLimit):
		status, code, message, retryable = 429, "TASK_CONCURRENCY_LIMIT", "项目正在运行的策略任务过多", true
	case errors.Is(err, strategy.ErrEventCursorExpired):
		status, code, message, retryable = 410, "EVENT_CURSOR_EXPIRED", "增量事件已过期，请重新加载完整状态", false
	case errors.Is(err, jobruntime.ErrJobNotFound):
		status, code, message, retryable = 404, "RESOURCE_NOT_FOUND", "任务不存在", false
	case errors.Is(err, jobruntime.ErrJobVersionConflict):
		status, code, message, retryable = 412, "VERSION_CONFLICT", "任务已被其他操作更新", false
	}
	requestID := writer.Header().Get("X-Request-ID")
	writeJSON(writer, status, map[string]any{"error": map[string]any{
		"code": code, "message": message, "request_id": requestID,
		"retryable": retryable, "details": details,
	}})
}

func mapAgentError(err error) error {
	if errors.Is(err, agent.ErrNotFound) {
		return strategy.ErrNotFound
	}
	if errors.Is(err, agent.ErrVersionConflict) {
		return strategy.ErrVersionConflict
	}
	return err
}
