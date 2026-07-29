// Package httpapi exposes Insights' authenticated v1 transport surface.
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
	"github.com/shikanon/cookies/internal/systems/insights"
)

type Application interface {
	CreateReport(context.Context, contract.ActorContext, contract.ProjectID, insights.CreateReportRequest) (insights.InsightReport, error)
	ListReports(context.Context, contract.ActorContext, contract.ProjectID, int) ([]insights.InsightReport, error)
	ConfirmReport(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (insights.InsightReport, error)
	CreateExperience(context.Context, contract.ActorContext, contract.ProjectID, string, int64, insights.CreateExperienceRequest) (insights.Experience, error)
	ListExperiences(context.Context, contract.ActorContext, contract.ProjectID, insights.ExperienceStatus, int) ([]insights.Experience, error)
	ConfirmExperience(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (insights.Experience, error)
	RejectExperience(context.Context, contract.ActorContext, contract.ProjectID, string, insights.ExperienceTransitionRequest) (insights.Experience, error)
	RequestExperienceReview(context.Context, contract.ActorContext, contract.ProjectID, string, insights.ExperienceTransitionRequest) (insights.Experience, error)
	RetireExperience(context.Context, contract.ActorContext, contract.ProjectID, string, insights.ExperienceTransitionRequest) (insights.Experience, error)
	ReviseExperience(context.Context, contract.ActorContext, contract.ProjectID, string, insights.ReviseExperienceRequest) (insights.Experience, error)
	RecordExperienceReference(context.Context, contract.ActorContext, contract.ProjectID, string, insights.RecordExperienceReferenceRequest) (insights.ExperienceReference, error)
	ListExperienceReferences(context.Context, contract.ActorContext, contract.ProjectID, string, int) ([]insights.ExperienceReference, error)
	ListProjectExperienceReferences(context.Context, contract.ActorContext, contract.ProjectID, int) ([]insights.ExperienceReference, error)
	ListExperienceAudits(context.Context, contract.ActorContext, contract.ProjectID, string, int) ([]insights.ExperienceAudit, error)
	ListExperienceLineage(context.Context, contract.ActorContext, contract.ProjectID, string) ([]insights.Experience, error)
	GetPreLaunch(context.Context, contract.ActorContext, contract.ProjectID, insights.PreLaunchFilter) (insights.PreLaunchInsight, error)
	GetPerformance(context.Context, contract.ActorContext, contract.ProjectID) (insights.PerformanceOverview, error)

	// 分析素材库与内容分析（03 §9 AM-001~006）。
	IndexAsset(context.Context, contract.ActorContext, contract.ProjectID, insights.IndexAssetRequest) (insights.Asset, error)
	ListAssets(context.Context, contract.ActorContext, contract.ProjectID, insights.AssetFilter) ([]insights.Asset, error)
	GetAsset(context.Context, contract.ActorContext, contract.ProjectID, string) (insights.Asset, error)
	ListAssetLineage(context.Context, contract.ActorContext, contract.ProjectID, string) ([]insights.Asset, error)
	IdentifyAssetType(context.Context, contract.ActorContext, contract.ProjectID, string, insights.IdentifyAssetTypeRequest) (insights.Asset, error)
	RegisterAssetMapping(context.Context, contract.ActorContext, contract.ProjectID, insights.RegisterAssetMappingRequest) (insights.AssetMapping, error)
	ListAssetMappings(context.Context, contract.ActorContext, contract.ProjectID, insights.AssetMappingFilter) ([]insights.AssetMapping, error)
	ResolveAssetMapping(context.Context, contract.ActorContext, contract.ProjectID, string, insights.ResolveAssetMappingRequest) (insights.AssetMapping, error)
	ExtractFeatures(context.Context, contract.ActorContext, contract.ProjectID, string, insights.ExtractFeaturesRequest) ([]insights.AssetFeature, error)
	PatchFeatures(context.Context, contract.ActorContext, contract.ProjectID, string, insights.PatchFeaturesRequest) ([]insights.AssetFeature, error)
	ListAssetFeatures(context.Context, contract.ActorContext, contract.ProjectID, string) ([]insights.AssetFeature, error)
	ConfirmAssetAnalysis(context.Context, contract.ActorContext, contract.ProjectID, string, insights.AssetTransitionRequest) (insights.Asset, error)
	RequestAssetReview(context.Context, contract.ActorContext, contract.ProjectID, string, insights.AssetTransitionRequest) (insights.Asset, error)
	RetireAsset(context.Context, contract.ActorContext, contract.ProjectID, string, insights.AssetTransitionRequest) (insights.Asset, error)
	GetFeatureMatrix(context.Context, contract.ActorContext, contract.ProjectID, []string) (insights.FeatureMatrix, error)

	// 数据接入与投后分析指标（doc10）。
	RegisterDataSource(context.Context, contract.ActorContext, contract.ProjectID, insights.RegisterDataSourceRequest) (insights.DataSource, error)
	ListDataSources(context.Context, contract.ActorContext, contract.ProjectID, insights.DataSourceFilter) ([]insights.DataSource, error)
	GetDataSource(context.Context, contract.ActorContext, contract.ProjectID, string) (insights.DataSource, error)
	UpdateDataSource(context.Context, contract.ActorContext, contract.ProjectID, string, insights.UpdateDataSourceRequest) (insights.DataSource, error)
	SetDataSourceQuality(context.Context, contract.ActorContext, contract.ProjectID, string, insights.SetDataSourceQualityRequest) (insights.DataSource, error)
	ImportMetrics(context.Context, contract.ActorContext, contract.ProjectID, insights.ImportMetricsRequest) (insights.ImportResult, error)
	ListImportBatches(context.Context, contract.ActorContext, contract.ProjectID, insights.ImportBatchFilter) ([]insights.ImportBatch, error)
	GetMetricOverview(context.Context, contract.ActorContext, contract.ProjectID, insights.MetricWindow) (insights.MetricOverview, error)
	GetPerformanceAnalysis(context.Context, contract.ActorContext, contract.ProjectID, insights.MetricWindow) (insights.PerformanceAnalysis, error)

	// 数据质量（doc10 §10）。
	GetDataQuality(context.Context, contract.ActorContext, contract.ProjectID, insights.MetricWindow) (insights.QualityReport, error)
	ResolveQualityIssue(context.Context, contract.ActorContext, contract.ProjectID, insights.ResolveQualityIssueRequest) (insights.QualityDisposition, error)

	// 能力运营（03 §一级导航；20 §4.1）。只读：这个模块治理的东西全部记在
	// features.go 和指标字典里，改它们要改代码并过评审，不走接口。
	GetCapabilityOperations(context.Context, contract.ActorContext, contract.ProjectID, insights.MetricWindow) (insights.CapabilityOperations, error)
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
	server.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/experiences/{experience_action}", server.experienceAction)
	server.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/experience-references", server.listProjectExperienceReferences)
	server.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/experiences/{experience_id}/references", server.listExperienceReferences)
	server.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/experiences/{experience_id}/audits", server.listExperienceAudits)
	server.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/experiences/{experience_id}/lineage", server.listExperienceLineage)
	server.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/prelaunch", server.preLaunch)
	server.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/performance", server.performance)
	server.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/capability-operations", server.capabilityOperations)
	server.registerAssetRoutes()
	server.registerConnectorRoutes()
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
		// 内嵌 CreateExperienceRequest 而不是逐字段抄：洞察卡九字段（03 §8.1）
		// 在这里沉淀。以前只转发结论/条件/反例，等于从复盘沉淀出来的经验永远
		// 停在「假设 / 方向性 / 没有依据」，而复盘恰恰是最有依据的那一次。
		var body struct {
			ExpectedReportVersion int64 `json:"expected_report_version"`
			insights.CreateExperienceRequest
		}
		if !decode(writer, request, &body) {
			return
		}
		value, err := s.app.CreateExperience(request.Context(), mustActor(request), projectID(request),
			strings.TrimSuffix(action, ":create-experience"), body.ExpectedReportVersion,
			body.CreateExperienceRequest)
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
	status := insights.ExperienceStatus(request.URL.Query().Get("status"))
	values, err := s.app.ListExperiences(request.Context(), mustActor(request), projectID(request), status, queryLimit(request))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

// experienceAction carries the lifecycle verbs of PRD §11.1. Each one is an
// explicit human decision, never an implicit side effect of another write.
func (s *Server) experienceAction(writer http.ResponseWriter, request *http.Request) {
	action := request.PathValue("experience_action")
	switch {
	case strings.HasSuffix(action, ":confirm"):
		var body struct {
			ExpectedVersion int64 `json:"expected_version"`
		}
		if !decode(writer, request, &body) {
			return
		}
		s.writeExperience(writer, request, http.StatusOK, func(id string) (insights.Experience, error) {
			return s.app.ConfirmExperience(request.Context(), mustActor(request), projectID(request), id, body.ExpectedVersion)
		}, strings.TrimSuffix(action, ":confirm"))
	case strings.HasSuffix(action, ":reject"):
		s.transitionAction(writer, request, strings.TrimSuffix(action, ":reject"), s.app.RejectExperience)
	case strings.HasSuffix(action, ":request-review"):
		s.transitionAction(writer, request, strings.TrimSuffix(action, ":request-review"), s.app.RequestExperienceReview)
	case strings.HasSuffix(action, ":retire"):
		s.transitionAction(writer, request, strings.TrimSuffix(action, ":retire"), s.app.RetireExperience)
	case strings.HasSuffix(action, ":revise"):
		var body insights.ReviseExperienceRequest
		if !decode(writer, request, &body) {
			return
		}
		s.writeExperience(writer, request, http.StatusCreated, func(id string) (insights.Experience, error) {
			return s.app.ReviseExperience(request.Context(), mustActor(request), projectID(request), id, body)
		}, strings.TrimSuffix(action, ":revise"))
	case strings.HasSuffix(action, ":record-reference"):
		var body insights.RecordExperienceReferenceRequest
		if !decode(writer, request, &body) {
			return
		}
		value, err := s.app.RecordExperienceReference(request.Context(), mustActor(request), projectID(request),
			strings.TrimSuffix(action, ":record-reference"), body)
		if err != nil {
			writeError(writer, request, err)
			return
		}
		writeJSON(writer, http.StatusCreated, value)
	default:
		http.NotFound(writer, request)
	}
}

type transitionFunc func(context.Context, contract.ActorContext, contract.ProjectID, string, insights.ExperienceTransitionRequest) (insights.Experience, error)

func (s *Server) transitionAction(writer http.ResponseWriter, request *http.Request, experienceID string, apply transitionFunc) {
	var body insights.ExperienceTransitionRequest
	if !decode(writer, request, &body) {
		return
	}
	s.writeExperience(writer, request, http.StatusOK, func(id string) (insights.Experience, error) {
		return apply(request.Context(), mustActor(request), projectID(request), id, body)
	}, experienceID)
}

func (s *Server) writeExperience(writer http.ResponseWriter, request *http.Request, status int, apply func(string) (insights.Experience, error), experienceID string) {
	value, err := apply(experienceID)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, status, value)
}

func (s *Server) listExperienceReferences(writer http.ResponseWriter, request *http.Request) {
	values, err := s.app.ListExperienceReferences(request.Context(), mustActor(request), projectID(request),
		request.PathValue("experience_id"), queryLimit(request))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

// listProjectExperienceReferences backs the 引用记录 view: one call answers
// "which experiences were used downstream" instead of one call per experience.
func (s *Server) listProjectExperienceReferences(writer http.ResponseWriter, request *http.Request) {
	values, err := s.app.ListProjectExperienceReferences(request.Context(), mustActor(request), projectID(request), queryLimit(request))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) listExperienceAudits(writer http.ResponseWriter, request *http.Request) {
	values, err := s.app.ListExperienceAudits(request.Context(), mustActor(request), projectID(request),
		request.PathValue("experience_id"), queryLimit(request))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) listExperienceLineage(writer http.ResponseWriter, request *http.Request) {
	values, err := s.app.ListExperienceLineage(request.Context(), mustActor(request), projectID(request),
		request.PathValue("experience_id"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) preLaunch(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	filter := insights.PreLaunchFilter{
		Channel: query.Get("channel"), CreativeType: query.Get("creative_type"),
		Objective: query.Get("objective"), Query: query.Get("q"),
		// 跨渠道比较必须显式打开（03 §10.3②）：默认关闭，缺参数就是关闭。
		CrossChannel: query.Get("cross_channel") == "true",
	}
	value, err := s.app.GetPreLaunch(request.Context(), mustActor(request), projectID(request), filter)
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

// capabilityOperations 一次返回五个 L2 视图的数据（特征体系/指标字典/分析 Skills/
// 评测集/质量看板）。不拆成五个端点：这五张表算的是同一批素材、特征、数据源和日指标，
// 拆开会让前端在五次请求之间拿到互相对不上的数字——治理面上「特征数」和「待办数」
// 对不上，比慢一点严重得多。
func (s *Server) capabilityOperations(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	window, ok := parseWindow(writer, request, query.Get("start"), query.Get("end"))
	if !ok {
		return
	}
	value, err := s.app.GetCapabilityOperations(request.Context(), mustActor(request), projectID(request), window)
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
	return decodeWithin(writer, request, target, 64<<10)
}

// decodeLarge is for the one write that carries a whole report file: an import
// batch may hold 5000 daily rows, which does not fit the ordinary body limit.
func decodeLarge(writer http.ResponseWriter, request *http.Request, target any) bool {
	return decodeWithin(writer, request, target, 8<<20)
}

func decodeWithin(writer http.ResponseWriter, request *http.Request, target any, limit int64) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, limit))
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
	if status == http.StatusInternalServerError {
		log.Printf("insights: internal error on %s %s: %v", request.Method, request.URL.Path, err)
	}
	writeJSON(writer, status, contract.Problem{Error: contract.Error{
		Code: code, Message: message, RequestID: requestContext.RequestID,
		Retryable: retryable, Details: []contract.FieldViolation{},
	}})
}
