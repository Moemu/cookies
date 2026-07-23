package creative

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/platform/provider"
)

const ScopePlanWrite contract.Scope = "creative.plan.write"

type ProjectContextReader interface {
	GetContext(context.Context, contract.ActorContext, contract.ProjectID) (contract.ProjectContext, error)
}

type HTTPDependencies struct {
	Service    Service
	Resolver   identity.Resolver
	Authorizer identity.ProjectAuthorizer
	Projects   ProjectContextReader
}

func NewHTTPHandler(dependencies HTTPDependencies) http.Handler {
	mux := http.NewServeMux()
	handler := &httpHandler{dependencies: dependencies}
	mux.HandleFunc("POST /api/creative/v1/projects/{project_id}/plans", handler.createPlan)
	mux.HandleFunc("GET /api/creative/v1/projects/{project_id}/plans/{plan_id}", handler.getPlan)
	mux.HandleFunc("POST /api/creative/v1/projects/{project_id}/plans/{plan_id}/image-jobs", handler.createImageJob)
	return mux
}

type httpHandler struct{ dependencies HTTPDependencies }

func (h *httpHandler) createPlan(w http.ResponseWriter, r *http.Request) {
	actor, project, ok := h.authorize(w, r, ScopePlanWrite)
	if !ok {
		return
	}
	var request CreatePlanRequest
	if !decode(w, r, &request) {
		return
	}
	value, err := h.dependencies.Service.CreatePlan(r.Context(), actor, project, request)
	if errors.Is(err, ErrStrategyNotApproved) {
		writeError(w, http.StatusConflict, "STRATEGY_NOT_APPROVED")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

func (h *httpHandler) getPlan(w http.ResponseWriter, r *http.Request) {
	actor, _, ok := h.authorize(w, r, "")
	if !ok {
		return
	}
	value, err := h.dependencies.Service.GetPlan(r.Context(), actor.OrganizationID, contract.ProjectID(r.PathValue("project_id")), r.PathValue("plan_id"))
	if errors.Is(err, ErrPlanNotFound) {
		writeError(w, http.StatusNotFound, "RESOURCE_NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (h *httpHandler) createImageJob(w http.ResponseWriter, r *http.Request) {
	actor, project, ok := h.authorize(w, r, provider.ScopeJobCreate)
	if !ok {
		return
	}
	var request struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	}
	if !decode(w, r, &request) {
		return
	}
	key := contract.IdempotencyKey(r.Header.Get("Idempotency-Key"))
	if err := key.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_INVALID")
		return
	}
	value, _, err := h.dependencies.Service.CreateImageJob(r.Context(), actor, project, r.PathValue("plan_id"), request.Width, request.Height, key)
	if errors.Is(err, ErrPlanNotFound) {
		writeError(w, http.StatusNotFound, "RESOURCE_NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	writeJSON(w, http.StatusAccepted, value)
}

func (h *httpHandler) authorize(w http.ResponseWriter, r *http.Request, scope contract.Scope) (contract.ActorContext, contract.ProjectContext, bool) {
	if h.dependencies.Resolver == nil || h.dependencies.Authorizer == nil || h.dependencies.Projects == nil {
		writeError(w, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
		return contract.ActorContext{}, contract.ProjectContext{}, false
	}
	actor, err := h.dependencies.Resolver.Authenticate(r.Context(), r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED")
		return contract.ActorContext{}, contract.ProjectContext{}, false
	}
	if scope != "" && !actor.HasScope(scope) {
		writeError(w, http.StatusForbidden, "SCOPE_REQUIRED")
		return contract.ActorContext{}, contract.ProjectContext{}, false
	}
	projectID := contract.ProjectID(r.PathValue("project_id"))
	if err := h.dependencies.Authorizer.AuthorizeProject(r.Context(), actor, projectID); err != nil {
		writeError(w, http.StatusForbidden, "PROJECT_ACCESS_DENIED")
		return contract.ActorContext{}, contract.ProjectContext{}, false
	}
	project, err := h.dependencies.Projects.GetContext(r.Context(), actor, projectID)
	if err != nil || project.ValidateBrandBound() != nil {
		writeError(w, http.StatusConflict, "PROJECT_NOT_ACTIVE")
		return contract.ActorContext{}, contract.ProjectContext{}, false
	}
	return actor, project, true
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST")
		return false
	}
	if err := decoder.Decode(&struct{}{}); err == nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code}})
}
