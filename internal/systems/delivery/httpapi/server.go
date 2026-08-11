// Package httpapi exposes Delivery's authenticated v1 transport surface.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/systems/delivery"
)

type Application interface {
	CreatePlan(context.Context, contract.ActorContext, contract.ProjectID, delivery.CreatePlanRequest) (delivery.DeliveryPlan, error)
	UpdatePlan(context.Context, contract.ActorContext, contract.ProjectID, string, delivery.UpdatePlanRequest) (delivery.DeliveryPlan, error)
	ListPlans(context.Context, contract.ActorContext, contract.ProjectID, int) ([]delivery.DeliveryPlan, error)
	GetPlan(context.Context, contract.ActorContext, contract.ProjectID, string) (delivery.DeliveryPlan, error)
	ListPlanVersions(context.Context, contract.ActorContext, contract.ProjectID, string) ([]delivery.DeliveryPlanVersion, error)
	GetPlanVersion(context.Context, contract.ActorContext, contract.ProjectID, string, int) (delivery.DeliveryPlanVersion, error)
	RunPlanPreflight(context.Context, contract.ActorContext, contract.ProjectID, string) (delivery.PreflightResult, error)
	GetPlanDetail(context.Context, contract.ActorContext, contract.ProjectID, string) (delivery.PlanDetail, error)
	ListChangeSets(context.Context, contract.ActorContext, contract.ProjectID, int) ([]delivery.ChangeSet, error)
	GetChangeSet(context.Context, contract.ActorContext, contract.ProjectID, string) (delivery.ChangeSet, error)
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
	server.mux.HandleFunc("PATCH /api/delivery/v1/projects/{project_id}/plans/{plan_id}", server.updatePlan)
	server.mux.HandleFunc("GET /api/delivery/v1/projects/{project_id}/plans/{plan_id}/versions", server.listPlanVersions)
	server.mux.HandleFunc("GET /api/delivery/v1/projects/{project_id}/plans/{plan_id}/versions/{version}", server.getPlanVersion)
	server.mux.HandleFunc("POST /api/delivery/v1/projects/{project_id}/plans/{plan_id}/preflight", server.planPreflight)
	server.mux.HandleFunc("GET /api/delivery/v1/projects/{project_id}/plans/{plan_id}/detail", server.getPlanDetail)
	server.mux.HandleFunc("POST /api/delivery/v1/projects/{project_id}/plans/{plan_action}", server.createChangeSet)
	server.mux.HandleFunc("GET /api/delivery/v1/projects/{project_id}/change-sets", server.listChangeSets)
	server.mux.HandleFunc("GET /api/delivery/v1/projects/{project_id}/change-sets/{change_set_id}", server.getChangeSet)
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
	writeJSON(writer, http.StatusOK, delivery.PlanList{
		Items: values, Source: delivery.SourceMock, Scenario: delivery.ScenarioPlanList,
	})
}

func (s *Server) getPlan(writer http.ResponseWriter, request *http.Request) {
	value, err := s.app.GetPlan(request.Context(), mustActor(request), projectID(request), request.PathValue("plan_id"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) getPlanDetail(writer http.ResponseWriter, request *http.Request) {
	value, err := s.app.GetPlanDetail(request.Context(), mustActor(request), projectID(request), request.PathValue("plan_id"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) updatePlan(writer http.ResponseWriter, request *http.Request) {
	var body delivery.UpdatePlanRequest
	if !decode(writer, request, &body) {
		return
	}
	value, err := s.app.UpdatePlan(
		request.Context(), mustActor(request), projectID(request), request.PathValue("plan_id"), body,
	)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) listPlanVersions(writer http.ResponseWriter, request *http.Request) {
	values, err := s.app.ListPlanVersions(
		request.Context(), mustActor(request), projectID(request), request.PathValue("plan_id"),
	)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, delivery.PlanVersionList{
		Items: values, Source: delivery.SourceMock, Scenario: delivery.ScenarioPlanList,
	})
}

func (s *Server) getPlanVersion(writer http.ResponseWriter, request *http.Request) {
	version, err := strconv.Atoi(request.PathValue("version"))
	if err != nil || version < 1 {
		writeError(writer, request, delivery.ErrInvalidRequest)
		return
	}
	value, err := s.app.GetPlanVersion(
		request.Context(), mustActor(request), projectID(request), request.PathValue("plan_id"), version,
	)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) planPreflight(writer http.ResponseWriter, request *http.Request) {
	value, err := s.app.RunPlanPreflight(
		request.Context(), mustActor(request), projectID(request), request.PathValue("plan_id"),
	)
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

func (s *Server) listChangeSets(writer http.ResponseWriter, request *http.Request) {
	values, err := s.app.ListChangeSets(request.Context(), mustActor(request), projectID(request), queryLimit(request))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"items": values, "source": delivery.SourceMock, "scenario": delivery.ScenarioApprovalQueue,
	})
}

func (s *Server) getChangeSet(writer http.ResponseWriter, request *http.Request) {
	value, err := s.app.GetChangeSet(
		request.Context(), mustActor(request), projectID(request), request.PathValue("change_set_id"),
	)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
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
	case errors.Is(err, identity.ErrProjectAccessDenied):
		status, code, message, retryable = http.StatusForbidden, "PROJECT_ACCESS_DENIED", "当前身份无权访问该项目", false
	case errors.Is(err, delivery.ErrInvalidState):
		status, code, message, retryable = http.StatusConflict, "INVALID_STATE", "当前状态不允许该操作", false
	case errors.Is(err, delivery.ErrPlanVersionConflict):
		status, code, message, retryable = http.StatusConflict, "VERSION_CONFLICT", "计划已被更新，请刷新后重试", false
	case errors.Is(err, delivery.ErrStalePlanVersion):
		status, code, message, retryable = http.StatusConflict, "STALE_PLAN_VERSION", "变更集引用的计划版本已过期，请重新创建", false
	case errors.Is(err, delivery.ErrApprovalRequired):
		status, code, message, retryable = http.StatusConflict, "APPROVAL_REQUIRED", "执行前需要有效审批", false
	case errors.Is(err, delivery.ErrApprovalExpired):
		status, code, message, retryable = http.StatusConflict, "APPROVAL_EXPIRED", "审批已过期，请重新预检并审批", false
	case errors.Is(err, delivery.ErrApprovalContentMismatch):
		status, code, message, retryable = http.StatusConflict, "APPROVAL_CONTENT_MISMATCH", "审批绑定的内容已变化，请重新预检并审批", false
	case errors.Is(err, delivery.ErrApprovalScopeExceeded):
		status, code, message, retryable = http.StatusForbidden, "APPROVAL_SCOPE_EXCEEDED", "执行范围或预算超出批准快照", false
	case errors.Is(err, delivery.ErrVersionConflict):
		status, code, message, retryable = http.StatusPreconditionFailed, "VERSION_CONFLICT", "资源已被更新，请刷新后重试", false
	case strings.Contains(err.Error(), "scope is required"):
		status, code, message, retryable = http.StatusForbidden, "SCOPE_REQUIRED", "缺少所需的投放权限", false
	}
	requestContext, _ := contract.RequestContextFrom(request.Context())
	if status == http.StatusInternalServerError {
		log.Printf("delivery request failed request_id=%s method=%s path=%s: %v", requestContext.RequestID, request.Method, request.URL.Path, err)
	}
	writeJSON(writer, status, struct {
		Error    contract.Error    `json:"error"`
		Source   delivery.Source   `json:"source"`
		Scenario delivery.Scenario `json:"scenario"`
	}{
		Error: contract.Error{
			Code: code, Message: message, RequestID: requestContext.RequestID,
			Retryable: retryable, Details: []contract.FieldViolation{},
		},
		Source: delivery.SourceMock, Scenario: errorScenario(code),
	})
}

func errorScenario(code string) delivery.Scenario {
	return delivery.Scenario(strings.ToLower(code))
}
