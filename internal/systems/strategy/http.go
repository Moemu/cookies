package strategy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/systems/strategy/prompts"
)

const (
	ScopeProposalWrite contract.Scope = "strategy.proposal.write"
	ScopeStrategyWrite contract.Scope = "strategy.strategy.write"
)

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
	mux.HandleFunc("POST /api/strategy/v1/projects/{project_id}/proposals", handler.createProposal)
	mux.HandleFunc("POST /api/strategy/v1/projects/{project_id}/proposals/{proposal_id}/generate", handler.generateStrategy)
	mux.HandleFunc("POST /api/strategy/v1/projects/{project_id}/strategies/{strategy_id}/approve", handler.approveStrategy)
	return mux
}

type httpHandler struct{ dependencies HTTPDependencies }

func (h *httpHandler) createProposal(w http.ResponseWriter, r *http.Request) {
	actor, project, ok := h.authorize(w, r, ScopeProposalWrite)
	if !ok {
		return
	}
	var input prompts.ProposalInput
	if !decode(w, r, &input) {
		return
	}
	value, _, err := h.dependencies.Service.CreateProposal(r.Context(), actor, project, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

func (h *httpHandler) generateStrategy(w http.ResponseWriter, r *http.Request) {
	actor, project, ok := h.authorize(w, r, ScopeStrategyWrite)
	if !ok {
		return
	}
	if !actor.HasScope(provider.ScopeTextGenerate) {
		writeError(w, http.StatusForbidden, "SCOPE_REQUIRED")
		return
	}
	value, err := h.dependencies.Service.GenerateStrategy(r.Context(), actor, project, r.PathValue("proposal_id"))
	if errors.Is(err, ErrProposalNotFound) {
		writeError(w, http.StatusNotFound, "RESOURCE_NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "STRATEGY_GENERATION_FAILED")
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

func (h *httpHandler) approveStrategy(w http.ResponseWriter, r *http.Request) {
	actor, project, ok := h.authorize(w, r, ScopeStrategyWrite)
	if !ok {
		return
	}
	value, err := h.dependencies.Service.ApproveStrategy(r.Context(), actor, project, r.PathValue("strategy_id"))
	if errors.Is(err, ErrStrategyNotFound) {
		writeError(w, http.StatusNotFound, "RESOURCE_NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusConflict, "INVALID_STATE")
		return
	}
	writeJSON(w, http.StatusOK, value)
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
	if !actor.HasScope(scope) {
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
	if err := decoder.Decode(target); err != nil || decoder.Decode(&struct{}{}) == nil {
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
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": strings.ReplaceAll(strings.ToLower(code), "_", " ")}})
}
