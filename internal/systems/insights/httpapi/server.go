// Package httpapi exposes Insights' authenticated v1 transport surface.
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
	"github.com/shikanon/cookies/internal/systems/insights"
)

type Application interface {
	CreateReport(context.Context, contract.ActorContext, contract.ProjectID, insights.CreateReportRequest) (insights.InsightReport, error)
	ListReports(context.Context, contract.ActorContext, contract.ProjectID, int) ([]insights.InsightReport, error)
	ConfirmReport(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (insights.InsightReport, error)
	CreateExperience(context.Context, contract.ActorContext, contract.ProjectID, string, int64, insights.CreateExperienceRequest) (insights.Experience, error)
	ListExperiences(context.Context, contract.ActorContext, contract.ProjectID, int) ([]insights.Experience, error)
	GetPreLaunch(context.Context, contract.ActorContext, contract.ProjectID) (insights.PreLaunchInsight, error)
	GetPerformance(context.Context, contract.ActorContext, contract.ProjectID) (insights.PerformanceOverview, error)
}

type Server struct {
	app Application
	mux *http.ServeMux
}

func New(app Application) *Server {
	server := &Server{app: app, mux: http.NewServeMux()}
	server.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/reports", server.listReports)
	server.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/reports", server.createReport)
	server.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/reports/{report_action}", server.reportAction)
	server.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/experiences", server.listExperiences)
	server.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/prelaunch", server.preLaunch)
	server.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/performance", server.performance)
	return server
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.mux.ServeHTTP(writer, request)
}

func (s *Server) createReport(writer http.ResponseWriter, request *http.Request) {
	var body insights.CreateReportRequest
	if !decode(writer, request, &body) {
		return
	}
	value, err := s.app.CreateReport(request.Context(), mustActor(request), projectID(request), body)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, value)
}

func (s *Server) listReports(writer http.ResponseWriter, request *http.Request) {
	values, err := s.app.ListReports(request.Context(), mustActor(request), projectID(request), queryLimit(request))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) reportAction(writer http.ResponseWriter, request *http.Request) {
	action := request.PathValue("report_action")
	switch {
	case strings.HasSuffix(action, ":confirm"):
		var body struct {
			ExpectedVersion int64 `json:"expected_version"`
		}
		if !decode(writer, request, &body) {
			return
		}
		value, err := s.app.ConfirmReport(request.Context(), mustActor(request), projectID(request), strings.TrimSuffix(action, ":confirm"), body.ExpectedVersion)
		if err != nil {
			writeError(writer, request, err)
			return
		}
		writeJSON(writer, http.StatusOK, value)
	case strings.HasSuffix(action, ":create-experience"):
		var body struct {
			ExpectedReportVersion int64    `json:"expected_report_version"`
			Conclusion            string   `json:"conclusion"`
			Conditions            []string `json:"conditions"`
			Counterexamples       []string `json:"counterexamples"`
		}
		if !decode(writer, request, &body) {
			return
		}
		value, err := s.app.CreateExperience(request.Context(), mustActor(request), projectID(request),
			strings.TrimSuffix(action, ":create-experience"), body.ExpectedReportVersion,
			insights.CreateExperienceRequest{Conclusion: body.Conclusion, Conditions: body.Conditions, Counterexamples: body.Counterexamples})
		if err != nil {
			writeError(writer, request, err)
			return
		}
		writeJSON(writer, http.StatusCreated, value)
	default:
		http.NotFound(writer, request)
	}
}

func (s *Server) listExperiences(writer http.ResponseWriter, request *http.Request) {
	values, err := s.app.ListExperiences(request.Context(), mustActor(request), projectID(request), queryLimit(request))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) preLaunch(writer http.ResponseWriter, request *http.Request) {
	value, err := s.app.GetPreLaunch(request.Context(), mustActor(request), projectID(request))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) performance(writer http.ResponseWriter, request *http.Request) {
	value, err := s.app.GetPerformance(request.Context(), mustActor(request), projectID(request))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
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
		writeError(writer, request, insights.ErrInvalidRequest)
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(writer, request, insights.ErrInvalidRequest)
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
	case errors.Is(err, insights.ErrInvalidRequest):
		status, code, message, retryable = http.StatusBadRequest, "INVALID_REQUEST", "请求参数无效", false
	case errors.Is(err, insights.ErrNotFound):
		status, code, message, retryable = http.StatusNotFound, "RESOURCE_NOT_FOUND", "洞察资源不存在", false
	case errors.Is(err, insights.ErrInvalidState):
		status, code, message, retryable = http.StatusConflict, "INVALID_STATE", "当前状态不允许该操作", false
	case errors.Is(err, insights.ErrVersionConflict):
		status, code, message, retryable = http.StatusPreconditionFailed, "VERSION_CONFLICT", "资源已被更新，请刷新后重试", false
	case strings.Contains(err.Error(), "scope is required"):
		status, code, message, retryable = http.StatusForbidden, "SCOPE_REQUIRED", "缺少所需的洞察权限", false
	}
	requestContext, _ := contract.RequestContextFrom(request.Context())
	writeJSON(writer, status, contract.Problem{Error: contract.Error{
		Code: code, Message: message, RequestID: requestContext.RequestID,
		Retryable: retryable, Details: []contract.FieldViolation{},
	}})
}
