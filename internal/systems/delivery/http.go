package delivery

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/platform/ids"
)

const (
	ScopePlanRead  contract.Scope = "delivery.plan.read"
	ScopePlanWrite contract.Scope = "delivery.plan.write"
)

type HTTPDependencies struct {
	Service    Service
	Resolver   identity.Resolver
	Authorizer identity.ProjectAuthorizer
}

func NewHTTPHandler(dependencies HTTPDependencies) http.Handler {
	handler := &httpHandler{dependencies: dependencies}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/delivery/v1/plans", handler.listPlans)
	mux.HandleFunc("POST /api/delivery/v1/plans", handler.createPlan)
	mux.HandleFunc("GET /api/delivery/v1/plans/{plan_id}", handler.getPlan)
	mux.HandleFunc("PATCH /api/delivery/v1/plans/{plan_id}", handler.updatePlan)
	mux.HandleFunc("GET /api/delivery/v1/plans/{plan_id}/versions", handler.listVersions)
	mux.HandleFunc("GET /api/delivery/v1/plans/{plan_id}/versions/{version}", handler.getVersion)
	mux.HandleFunc("POST /api/delivery/v1/plans/{plan_id}/preflight", handler.preflightPlan)
	return mux
}

type httpHandler struct{ dependencies HTTPDependencies }

func (h *httpHandler) listPlans(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.authenticate(w, r, ScopePlanRead)
	if !ok {
		return
	}
	projectID := contract.ProjectID(strings.TrimSpace(r.URL.Query().Get("project_id")))
	if projectID == "" {
		writeDeliveryError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "project_id is required", false)
		return
	}
	if !h.authorizeProject(w, r, actor, projectID) {
		return
	}
	plans, err := h.dependencies.Service.ListPlans(r.Context(), actor, projectID)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	writeDeliveryJSON(w, http.StatusOK, PlanList{Items: plans, Source: SourceMock, Scenario: ScenarioPlanList})
}

func (h *httpHandler) createPlan(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.authenticate(w, r, ScopePlanWrite)
	if !ok {
		return
	}
	var request CreatePlanRequest
	if !decodeDeliveryJSON(w, r, &request) {
		return
	}
	if err := request.Validate(); err != nil {
		writeDeliveryError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "The plan draft does not satisfy the API contract.", false)
		return
	}
	if !h.authorizeProject(w, r, actor, request.ProjectID) {
		return
	}
	plan, err := h.dependencies.Service.CreatePlan(r.Context(), actor, request)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/delivery/v1/plans/"+plan.ID)
	writeDeliveryJSON(w, http.StatusCreated, plan)
}

func (h *httpHandler) getPlan(w http.ResponseWriter, r *http.Request) {
	actor, plan, ok := h.scopedPlan(w, r, ScopePlanRead)
	if !ok {
		return
	}
	_ = actor
	writeDeliveryJSON(w, http.StatusOK, plan)
}

func (h *httpHandler) updatePlan(w http.ResponseWriter, r *http.Request) {
	actor, _, ok := h.scopedPlan(w, r, ScopePlanWrite)
	if !ok {
		return
	}
	var request UpdatePlanRequest
	if !decodeDeliveryJSON(w, r, &request) {
		return
	}
	if err := request.Validate(); err != nil {
		writeDeliveryError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "The plan update does not satisfy the API contract.", false)
		return
	}
	plan, err := h.dependencies.Service.UpdatePlan(r.Context(), actor, r.PathValue("plan_id"), request)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	writeDeliveryJSON(w, http.StatusOK, plan)
}

func (h *httpHandler) listVersions(w http.ResponseWriter, r *http.Request) {
	_, plan, ok := h.scopedPlan(w, r, ScopePlanRead)
	if !ok {
		return
	}
	writeDeliveryJSON(w, http.StatusOK, PlanVersionList{Items: plan.Versions, Source: SourceMock, Scenario: plan.Scenario})
}

func (h *httpHandler) getVersion(w http.ResponseWriter, r *http.Request) {
	_, plan, ok := h.scopedPlan(w, r, ScopePlanRead)
	if !ok {
		return
	}
	versionNumber, err := strconv.Atoi(r.PathValue("version"))
	if err != nil || versionNumber < 1 {
		writeDeliveryError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "The version path parameter must be a positive integer.", false)
		return
	}
	for _, version := range plan.Versions {
		if version.VersionNumber == versionNumber {
			writeDeliveryJSON(w, http.StatusOK, version)
			return
		}
	}
	writeDeliveryError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "The scoped plan version does not exist.", false)
}

func (h *httpHandler) preflightPlan(w http.ResponseWriter, r *http.Request) {
	actor, _, ok := h.scopedPlan(w, r, ScopePlanWrite)
	if !ok {
		return
	}
	result, err := h.dependencies.Service.PreflightPlan(r.Context(), actor, r.PathValue("plan_id"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	writeDeliveryJSON(w, http.StatusOK, result)
}

func (h *httpHandler) scopedPlan(w http.ResponseWriter, r *http.Request, scope contract.Scope) (contract.ActorContext, DeliveryPlan, bool) {
	actor, ok := h.authenticate(w, r, scope)
	if !ok {
		return contract.ActorContext{}, DeliveryPlan{}, false
	}
	projectID := contract.ProjectID(strings.TrimSpace(r.URL.Query().Get("project_id")))
	if projectID == "" {
		writeDeliveryError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "project_id is required", false)
		return contract.ActorContext{}, DeliveryPlan{}, false
	}
	if !h.authorizeProject(w, r, actor, projectID) {
		return contract.ActorContext{}, DeliveryPlan{}, false
	}
	plan, err := h.dependencies.Service.GetPlan(r.Context(), actor, r.PathValue("plan_id"))
	if err != nil {
		h.writeServiceError(w, r, err)
		return contract.ActorContext{}, DeliveryPlan{}, false
	}
	if plan.ProjectID != projectID {
		writeDeliveryError(w, r, http.StatusForbidden, contract.ErrorProjectAccessDenied, "The plan does not belong to the requested Project.", false)
		return contract.ActorContext{}, DeliveryPlan{}, false
	}
	return actor, plan, true
}

func (h *httpHandler) authenticate(w http.ResponseWriter, r *http.Request, scope contract.Scope) (contract.ActorContext, bool) {
	if h.dependencies.Resolver == nil {
		writeDeliveryError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Identity resolution is unavailable.", true)
		return contract.ActorContext{}, false
	}
	actor, err := h.dependencies.Resolver.Authenticate(r.Context(), r)
	if err != nil {
		writeDeliveryError(w, r, http.StatusUnauthorized, contract.ErrorUnauthenticated, "Authentication is required.", false)
		return contract.ActorContext{}, false
	}
	if !actor.HasScope(scope) {
		writeDeliveryError(w, r, http.StatusForbidden, contract.ErrorScopeRequired, "The required delivery scope is missing.", false)
		return contract.ActorContext{}, false
	}
	return actor, true
}

func (h *httpHandler) authorizeProject(w http.ResponseWriter, r *http.Request, actor contract.ActorContext, projectID contract.ProjectID) bool {
	if h.dependencies.Authorizer == nil {
		writeDeliveryError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Project authorization is unavailable.", true)
		return false
	}
	if err := h.dependencies.Authorizer.AuthorizeProject(r.Context(), actor, projectID); err != nil {
		writeDeliveryError(w, r, http.StatusForbidden, contract.ErrorProjectAccessDenied, "Access to the plan's Project is denied.", false)
		return false
	}
	return true
}

func (h *httpHandler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeDeliveryError(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "The scoped delivery plan does not exist.", false)
	case errors.Is(err, ErrVersionConflict):
		writeDeliveryError(w, r, http.StatusConflict, "VERSION_CONFLICT", "The submitted expected version is stale.", false)
	default:
		writeDeliveryError(w, r, http.StatusInternalServerError, "INTERNAL", "The delivery service could not complete the request.", true)
	}
}

func decodeDeliveryJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeDeliveryError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "The request body is invalid.", false)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeDeliveryError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "The request body must contain exactly one JSON value.", false)
		return false
	}
	return true
}

func writeDeliveryJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeDeliveryError(w http.ResponseWriter, r *http.Request, status int, code, message string, retryable bool) {
	requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if requestID == "" {
		requestID, _ = ids.New("request")
	}
	writeDeliveryJSON(w, status, contract.Problem{Error: contract.Error{
		Code: code, Message: message, RequestID: requestID, Retryable: retryable,
		Details: []contract.FieldViolation{},
	}})
}
