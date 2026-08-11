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
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/insights"
)

type Application interface {
	GetMiyunConnection(context.Context, contract.ActorContext, contract.ProjectID) (insights.MiyunConnection, error)
	UpdateMiyunConnection(context.Context, contract.ActorContext, contract.ProjectID, insights.UpdateMiyunConnectionRequest) (insights.MiyunConnection, error)
	VerifyMiyunConnection(context.Context, contract.ActorContext, contract.ProjectID, insights.VerifyMiyunConnectionRequest) (insights.MiyunConnection, error)
	GetMiyunProductSource(context.Context, contract.ActorContext, contract.ProjectID) (insights.MiyunProductSource, error)
	AnalyzeMiyunProductProfile(context.Context, contract.ActorContext, contract.ProjectID, insights.AnalyzeMiyunProductProfileRequest) (insights.MiyunProductProfile, error)
	ConfirmMiyunProductProfile(context.Context, contract.ActorContext, contract.ProjectID, string, insights.ConfirmMiyunProductProfileRequest) (insights.MiyunProductProfile, error)
	ListMiyunProductProfiles(context.Context, contract.ActorContext, contract.ProjectID, int) ([]insights.MiyunProductProfile, error)
	GetMiyunProductProfile(context.Context, contract.ActorContext, contract.ProjectID, string) (insights.MiyunProductProfile, error)
	ManualImportMiyunMaterial(context.Context, contract.ActorContext, contract.ProjectID, contract.IdempotencyKey, insights.ManualMiyunMaterialRequest) (insights.MiyunManualImportResult, error)
	CreateMiyunCrawlJob(context.Context, contract.ActorContext, contract.ProjectID, contract.IdempotencyKey, insights.CreateMiyunCrawlJobRequest) (insights.MiyunCrawlJob, error)
	ListMiyunCrawlJobs(context.Context, contract.ActorContext, contract.ProjectID, int) ([]insights.MiyunCrawlJob, error)
	GetMiyunCrawlJob(context.Context, contract.ActorContext, contract.ProjectID, string) (insights.MiyunCrawlJob, error)
	CancelMiyunCrawlJob(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (insights.MiyunCrawlJob, error)
	RetryMiyunCrawlJob(context.Context, contract.ActorContext, contract.ProjectID, string, contract.IdempotencyKey) (insights.MiyunCrawlJob, error)
	ListMiyunMaterials(context.Context, contract.ActorContext, contract.ProjectID, int) ([]insights.MiyunMaterial, error)
	GetMiyunMaterialDetail(context.Context, contract.ActorContext, contract.ProjectID, string) (insights.MiyunMaterialDetail, error)
	OpenMiyunMaterialPreview(context.Context, contract.ActorContext, contract.ProjectID, string) (insights.MiyunMaterialPreview, error)
	DecideMiyunMaterial(context.Context, contract.ActorContext, contract.ProjectID, string, bool, insights.MiyunMaterialDecisionRequest) (insights.MiyunMaterial, error)
	RetryMiyunMaterialImport(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (insights.MiyunMaterial, error)
	CreateMiyunHandoff(context.Context, contract.ActorContext, contract.ProjectID, insights.CreateMiyunHandoffRequest) (insights.MiyunHandoff, error)
	ListMiyunHandoffs(context.Context, contract.ActorContext, contract.ProjectID, int) ([]insights.MiyunHandoff, error)
	GetMiyunHandoff(context.Context, contract.ActorContext, contract.ProjectID, string) (insights.MiyunHandoff, error)
	MarkMiyunHandoffDelivered(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (insights.MiyunHandoff, error)
	ExportMiyunHandoff(context.Context, contract.ActorContext, contract.ProjectID, string, io.Writer) error

	CreateReport(context.Context, contract.ActorContext, contract.ProjectID, insights.CreateReportRequest) (insights.InsightReport, error)
	ListReports(context.Context, contract.ActorContext, contract.ProjectID, int) ([]insights.InsightReport, error)
	ConfirmReport(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (insights.InsightReport, error)
	DropReportFinding(context.Context, contract.ActorContext, contract.ProjectID, string, int64, int, bool) (insights.InsightReport, error)
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

	// AI 提特征（03 §9 AM-005）。只有人点按钮才会走到这里：
	// 登记素材时自动排队会把复核队列灌满没人要看的结果，而复核是唯一的质量闸门。
	AnalyzeAsset(context.Context, contract.ActorContext, contract.ProjectID, string, insights.AnalyzeAssetRequest) (insights.AnalyzeAssetResult, error)
	ListAnalysisRuns(context.Context, contract.ActorContext, contract.ProjectID, insights.AnalysisRunFilter) ([]insights.AnalysisRun, error)

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

	// 实验中心（03 §7.3 / AM-009）。判定不在入参里：判定要是能传，
	// 事先定的样本门槛就形同虚设。人只写解读，系统给判定。
	CreateExperiment(context.Context, contract.ActorContext, contract.ProjectID, insights.CreateExperimentRequest) (insights.Experiment, error)
	ListExperiments(context.Context, contract.ActorContext, contract.ProjectID, insights.ExperimentFilter) ([]insights.Experiment, error)
	GetExperiment(context.Context, contract.ActorContext, contract.ProjectID, string) (insights.ExperimentDetail, error)
	AttachExperimentAsset(context.Context, contract.ActorContext, contract.ProjectID, string, string, insights.AttachExperimentAssetRequest) (insights.AttachExperimentAssetResult, error)
	DetachExperimentAsset(context.Context, contract.ActorContext, contract.ProjectID, string, string, string) (insights.ExperimentVariant, error)
	StartExperiment(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (insights.Experiment, error)
	ConcludeExperiment(context.Context, contract.ActorContext, contract.ProjectID, string, insights.ConcludeExperimentRequest) (insights.Experiment, error)

	// 系统设置（03 §78；20 §121）。只读且不读库：返回的每个值都是代码常量本身，
	// 不是抄过来的副本——抄一份迟早和代码对不上，那时候这一页就从说明变成误导。
	GetInsightSettings(context.Context, contract.ActorContext, contract.ProjectID) (insights.InsightSettings, error)
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
	server.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/settings", server.settings)
	server.registerAssetRoutes()
	server.registerConnectorRoutes()
	server.registerExperimentRoutes()
	server.registerMiyunRoutes()
	return server
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.mux.ServeHTTP(writer, request)
}

func (s *Server) createReport(writer http.ResponseWriter, request *http.Request) {
	// 窗口在线上传的是「2026-07-01」这样的日子，不是时间戳：投后分析页上人看到的
	// 就是两个日期，报告要定格的也是这两个日期。中间过一道时区换算，定格下来的
	// 窗口就可能和人当时看到的差一天，而报告上还是写着同一个区间。
	// 这里的 Window 遮住内嵌结构里的同名字段（Go 取层级更浅的那个）。
	var body struct {
		insights.CreateReportRequest
		Window struct {
			Start string `json:"start"`
			End   string `json:"end"`
		} `json:"window"`
	}
	if !decode(writer, request, &body) {
		return
	}
	payload := body.CreateReportRequest
	start, end := strings.TrimSpace(body.Window.Start), strings.TrimSpace(body.Window.End)
	switch {
	case start == "" && end == "":
		// 不传窗口就是老调用方，报告照旧生成，只是没有四块汇总。
	case start == "" || end == "":
		// 只传一头就退回去。补一个默认值等于替人挑了半个窗口，而报告上会写得
		// 像是他自己选的。
		writeError(writer, request, insights.ErrInvalidRequest)
		return
	default:
		parsedStart, err := time.ParseInLocation("2006-01-02", start, time.UTC)
		if err != nil {
			writeError(writer, request, insights.ErrInvalidRequest)
			return
		}
		parsedEnd, err := time.ParseInLocation("2006-01-02", end, time.UTC)
		if err != nil {
			writeError(writer, request, insights.ErrInvalidRequest)
			return
		}
		payload.Window = insights.MetricWindow{Start: parsedStart, End: parsedEnd}
	}
	value, err := s.app.CreateReport(request.Context(), mustActor(request), projectID(request), payload)
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
	case strings.HasSuffix(action, ":drop-finding"):
		// 人工删减。dropped 可以来回切，因为「删错了」比「删漏了」常见得多，
		// 而报告在确认之前本来就是可改的。
		var body struct {
			ExpectedVersion int64 `json:"expected_version"`
			Index           int   `json:"index"`
			Dropped         bool  `json:"dropped"`
		}
		if !decode(writer, request, &body) {
			return
		}
		value, err := s.app.DropReportFinding(request.Context(), mustActor(request), projectID(request),
			strings.TrimSuffix(action, ":drop-finding"), body.ExpectedVersion, body.Index, body.Dropped)
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

// settings 返回五组当前生效的阈值与规则。没有 PUT/PATCH 配对：整页只读，
// 改阈值要走代码评审（03 §17.3 把「最低样本由谁配置」列为待确认，在那条定下来
// 之前开放写接口，等于先替它选了答案）。
//
// 路径上带 project_id 只为沿用同一套鉴权，值本身对整个部署生效，不分 Project。
func (s *Server) settings(writer http.ResponseWriter, request *http.Request) {
	value, err := s.app.GetInsightSettings(request.Context(), mustActor(request), projectID(request))
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

// detailOr 取业务层写在哨兵错误后面的那句中文。
//
// 服务层几乎每处校验都写了一句人能直接读的话（「账户标识只能用英文字母…」），可这里
// 以前一律换成「请求参数无效」——不管填错的是账户标识、口径还是窗口，人看到的都是
// 同一句，只能靠猜。业务层那些话写了也白写。
//
// 只对这四个哨兵透传：INTERNAL 那一支带的是数据库和驱动的原文，那种东西不能出现在
// 页面上。哨兵的英文前缀对不上（例如包成了「read report: ...」）就退回原来的默认句，
// 宁可笼统，也不把内部措辞漏出去。
func detailOr(err error, sentinel error, fallback string) string {
	detail := strings.TrimSpace(strings.TrimPrefix(err.Error(), sentinel.Error()+": "))
	if detail == "" || detail == err.Error() {
		return fallback
	}
	return detail
}

func writeError(writer http.ResponseWriter, request *http.Request, err error) {
	status, code, message := http.StatusInternalServerError, "INTERNAL", "服务暂时不可用，请稍后重试"
	retryable := true
	switch {
	case errors.Is(err, insights.ErrInvalidRequest):
		status, code, retryable = http.StatusBadRequest, "INVALID_REQUEST", false
		message = detailOr(err, insights.ErrInvalidRequest, "请求参数无效")
	case errors.Is(err, insights.ErrNotFound):
		status, code, retryable = http.StatusNotFound, "RESOURCE_NOT_FOUND", false
		message = detailOr(err, insights.ErrNotFound, "洞察资源不存在")
	case errors.Is(err, insights.ErrInvalidState):
		status, code, retryable = http.StatusConflict, "INVALID_STATE", false
		message = detailOr(err, insights.ErrInvalidState, "当前状态不允许该操作")
	case errors.Is(err, insights.ErrVersionConflict):
		status, code, retryable = http.StatusPreconditionFailed, "VERSION_CONFLICT", false
		message = detailOr(err, insights.ErrVersionConflict, "资源已被更新，请刷新后重试")
	case errors.Is(err, insights.ErrIdempotencyConflict):
		status, code, retryable = http.StatusConflict, contract.ErrorIdempotencyConflict, false
		message = "The idempotency key conflicts with an earlier Miyun request."
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
