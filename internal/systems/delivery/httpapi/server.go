// Package httpapi exposes Delivery's authenticated v1 transport surface.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/delivery"
)

type Application interface {
	CreatePlan(context.Context, contract.ActorContext, contract.ProjectID, delivery.CreatePlanRequest) (delivery.DeliveryPlan, error)
	ListPlans(context.Context, contract.ActorContext, contract.ProjectID, int) ([]delivery.DeliveryPlan, error)
	GetPlanDetail(context.Context, contract.ActorContext, contract.ProjectID, string) (delivery.PlanDetail, error)
	CreateChangeSet(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (delivery.ChangeSet, error)
	Preflight(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (delivery.ChangeSet, error)
	Approve(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (delivery.ChangeSet, error)
	Execute(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (delivery.ExecutionResult, error)
	Rollback(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (delivery.ChangeSet, error)
	ListExecutions(context.Context, contract.ActorContext, contract.ProjectID, int) ([]delivery.ExecutionResult, error)
	CreateDemoMetricSnapshot(context.Context, contract.ActorContext, contract.ProjectID, string, delivery.CreateMetricSnapshotRequest) (delivery.DeliveryMetricSnapshot, error)
	ListMetricSnapshots(context.Context, contract.ActorContext, contract.ProjectID, string, int) ([]delivery.DeliveryMetricSnapshot, error)
}

type Server struct {
	app Application
	mux *http.ServeMux
}

func New(app Application) *Server {
	server := &Server{app: app, mux: http.NewServeMux()}
	server.mux.HandleFunc("GET /api/delivery/v1/projects/{project_id}/plans", server.listPlans)
	server.mux.HandleFunc("POST /api/delivery/v1/projects/{project_id}/plans", server.createPlan)
	server.mux.HandleFunc("GET /api/delivery/v1/projects/{project_id}/plans/{plan_id}", server.getPlan)
	server.mux.HandleFunc("POST /api/delivery/v1/projects/{project_id}/plans/{plan_action}", server.createChangeSet)
	server.mux.HandleFunc("POST /api/delivery/v1/projects/{project_id}/change-sets/{change_set_action}", server.changeSetAction)
	server.mux.HandleFunc("GET /api/delivery/v1/projects/{project_id}/executions", server.listExecutions)
	server.mux.HandleFunc("POST /api/delivery/v1/projects/{project_id}/executions/{execution_id}/metric-snapshots", server.createMetricSnapshot)
	server.mux.HandleFunc("GET /api/delivery/v1/projects/{project_id}/executions/{execution_id}/metric-snapshots", server.listMetricSnapshots)
	return server
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.mux.ServeHTTP(writer, request)
}

func (s *Server) createPlan(writer http.ResponseWriter, request *http.Request) {
	var body delivery.CreatePlanRequest
	if !decode(writer, request, &body) {
		return
	}
	value, err := s.app.CreatePlan(request.Context(), mustActor(request), projectID(request), body)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writer.Header().Set("Location", "/api/delivery/v1/projects/"+string(projectID(request))+"/plans/"+value.ID)
	writeJSON(writer, http.StatusCreated, value)
}

func (s *Server) listPlans(writer http.ResponseWriter, request *http.Request) {
	values, err := s.app.ListPlans(request.Context(), mustActor(request), projectID(request), queryLimit(request))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) getPlan(writer http.ResponseWriter, request *http.Request) {
	value, err := s.app.GetPlanDetail(request.Context(), mustActor(request), projectID(request), request.PathValue("plan_id"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) createChangeSet(writer http.ResponseWriter, request *http.Request) {
	action := request.PathValue("plan_action")
	if !strings.HasSuffix(action, ":create-change-set") {
		http.NotFound(writer, request)
		return
	}
	var body struct {
		ExpectedVersion int64 `json:"expected_version"`
	}
	if !decode(writer, request, &body) || body.ExpectedVersion < 1 {
		if body.ExpectedVersion < 1 {
			writeError(writer, request, delivery.ErrInvalidRequest)
		}
		return
	}
	value, err := s.app.CreateChangeSet(request.Context(), mustActor(request), projectID(request), strings.TrimSuffix(action, ":create-change-set"), body.ExpectedVersion)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, value)
}

func (s *Server) changeSetAction(writer http.ResponseWriter, request *http.Request) {
	action := request.PathValue("change_set_action")
	var body struct {
		ExpectedVersion int64 `json:"expected_version"`
	}
	if !decode(writer, request, &body) || body.ExpectedVersion < 1 {
		if body.ExpectedVersion < 1 {
			writeError(writer, request, delivery.ErrInvalidRequest)
		}
		return
	}
	var (
		value any
		err   error
	)
	switch {
	case strings.HasSuffix(action, ":preflight"):
		value, err = s.app.Preflight(request.Context(), mustActor(request), projectID(request), strings.TrimSuffix(action, ":preflight"), body.ExpectedVersion)
	case strings.HasSuffix(action, ":approve"):
		value, err = s.app.Approve(request.Context(), mustActor(request), projectID(request), strings.TrimSuffix(action, ":approve"), body.ExpectedVersion)
	case strings.HasSuffix(action, ":execute"):
		value, err = s.app.Execute(request.Context(), mustActor(request), projectID(request), strings.TrimSuffix(action, ":execute"), body.ExpectedVersion)
	case strings.HasSuffix(action, ":rollback"):
		value, err = s.app.Rollback(request.Context(), mustActor(request), projectID(request), strings.TrimSuffix(action, ":rollback"), body.ExpectedVersion)
	default:
		http.NotFound(writer, request)
		return
	}
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) listExecutions(writer http.ResponseWriter, request *http.Request) {
	values, err := s.app.ListExecutions(request.Context(), mustActor(request), projectID(request), queryLimit(request))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) createMetricSnapshot(writer http.ResponseWriter, request *http.Request) {
	var body delivery.CreateMetricSnapshotRequest
	if !decode(writer, request, &body) {
		return
	}
	value, err := s.app.CreateDemoMetricSnapshot(
		request.Context(), mustActor(request), projectID(request), request.PathValue("execution_id"), body,
	)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, value)
}

func (s *Server) listMetricSnapshots(writer http.ResponseWriter, request *http.Request) {
	values, err := s.app.ListMetricSnapshots(
		request.Context(), mustActor(request), projectID(request), request.PathValue("execution_id"), queryLimit(request),
	)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

func mustActor(request *http.Request) contract.ActorContext {
	value, _ := contract.RequestContextFrom(request.Context())
	return value.Actor
}

func projectID(request *http.Request) contract.ProjectID {
	return contract.ProjectID(request.PathValue("project_id"))
}

func queryLimit(request *http.Request) int {
	value, _ := strconv.Atoi(request.URL.Query().Get("limit"))
	if value < 1 || value > 100 {
		return 50
	}
	return value
}

func decode(writer http.ResponseWriter, request *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(writer, request, delivery.ErrInvalidRequest)
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(writer, request, delivery.ErrInvalidRequest)
		return false
	}
	return true
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, request *http.Request, err error) {
	status, code, message := http.StatusInternalServerError, "INTERNAL", "服务暂时不可用，请稍后重试"
	retryable := true
	switch {
	case errors.Is(err, delivery.ErrInvalidRequest):
		status, code, message, retryable = http.StatusBadRequest, "INVALID_REQUEST", "请求参数无效", false
	case errors.Is(err, delivery.ErrNotFound):
		status, code, message, retryable = http.StatusNotFound, "RESOURCE_NOT_FOUND", "投放资源不存在", false
	case errors.Is(err, delivery.ErrInvalidState):
		status, code, message, retryable = http.StatusConflict, "INVALID_STATE", "当前状态不允许该操作", false
	case errors.Is(err, delivery.ErrVersionConflict):
		status, code, message, retryable = http.StatusPreconditionFailed, "VERSION_CONFLICT", "资源已被更新，请刷新后重试", false
	case strings.Contains(err.Error(), "scope is required"):
		status, code, message, retryable = http.StatusForbidden, "SCOPE_REQUIRED", "缺少所需的投放权限", false
	}
	requestContext, _ := contract.RequestContextFrom(request.Context())
	writeJSON(writer, status, contract.Problem{Error: contract.Error{
		Code: code, Message: message, RequestID: requestContext.RequestID,
		Retryable: retryable, Details: []contract.FieldViolation{},
	}})
}
