package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/systems/insights"
)

// registerConnectorRoutes exposes 数据接入 and the metric overview behind
// 投后分析 (doc10, 20 §6.1 第一批).
//
// The five 数据接入 views map to real queries rather than five labels over one
// list (22 §8.3): 数据源 and 字段映射 read the same rows but different fields,
// 导入任务 and 同步记录 are the same table filtered by kind, and 素材映射 is the
// asset-mapping endpoint that already exists.
func (s *Server) registerConnectorRoutes() {
	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/data-sources", s.listDataSources)
	s.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/data-sources", s.registerDataSource)
	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/data-sources/{data_source_id}", s.getDataSource)
	s.mux.HandleFunc("PATCH /api/insights/v1/projects/{project_id}/data-sources/{data_source_id}", s.updateDataSource)
	s.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/data-sources/{data_source_action}", s.dataSourceAction)

	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/import-batches", s.listImportBatches)
	s.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/import-batches", s.importMetrics)

	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/metric-overview", s.metricOverview)
	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/performance-analysis", s.performanceAnalysis)

	// doc10 §10 写的是 `GET /api/insights/v1/data-quality`，没带 project。
	// 这里挂在 project 下，理由是质量问题全部由某个 Project 的数据源和指标算出，
	// 不带 project 的话服务端得替调用方猜是哪个项目。文档冲突已登记在功能基线 §8。
	s.mux.HandleFunc("GET /api/insights/v1/projects/{project_id}/data-quality", s.dataQuality)
	s.mux.HandleFunc("POST /api/insights/v1/projects/{project_id}/data-quality/dispositions", s.resolveQualityIssue)
}

func (s *Server) registerDataSource(writer http.ResponseWriter, request *http.Request) {
	var body insights.RegisterDataSourceRequest
	if !decode(writer, request, &body) {
		return
	}
	value, err := s.app.RegisterDataSource(request.Context(), mustActor(request), projectID(request), body)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, value)
}

func (s *Server) listDataSources(writer http.ResponseWriter, request *http.Request) {
	filter := insights.DataSourceFilter{Limit: queryLimit(request)}
	for _, value := range queryList(request, "status") {
		filter.Statuses = append(filter.Statuses, insights.DataSourceStatus(value))
	}
	for _, value := range queryList(request, "platform") {
		filter.Platforms = append(filter.Platforms, insights.Platform(value))
	}
	values, err := s.app.ListDataSources(request.Context(), mustActor(request), projectID(request), filter)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

func (s *Server) getDataSource(writer http.ResponseWriter, request *http.Request) {
	value, err := s.app.GetDataSource(request.Context(), mustActor(request), projectID(request),
		request.PathValue("data_source_id"))
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

// updateDataSource is the 接入向导 write: 字段映射, 口径 and lifecycle. PATCH
// rather than PUT because the caller only ever sends the step it just finished.
func (s *Server) updateDataSource(writer http.ResponseWriter, request *http.Request) {
	var body insights.UpdateDataSourceRequest
	if !decode(writer, request, &body) {
		return
	}
	value, err := s.app.UpdateDataSource(request.Context(), mustActor(request), projectID(request),
		request.PathValue("data_source_id"), body)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

func (s *Server) dataSourceAction(writer http.ResponseWriter, request *http.Request) {
	action := request.PathValue("data_source_action")
	if !strings.HasSuffix(action, ":set-quality") {
		http.NotFound(writer, request)
		return
	}
	var body insights.SetDataSourceQualityRequest
	if !decode(writer, request, &body) {
		return
	}
	value, err := s.app.SetDataSourceQuality(request.Context(), mustActor(request), projectID(request),
		strings.TrimSuffix(action, ":set-quality"), body)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

// importMetrics accepts a bigger body than the other writes: one batch carries
// up to 5000 daily rows, which the 64 KiB default would cut off mid-file.
func (s *Server) importMetrics(writer http.ResponseWriter, request *http.Request) {
	var body insights.ImportMetricsRequest
	if !decodeLarge(writer, request, &body) {
		return
	}
	value, err := s.app.ImportMetrics(request.Context(), mustActor(request), projectID(request), body)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	// 部分成功也是 201：批次真的建出来了，前端要拿它去看被拒的行。
	writeJSON(writer, http.StatusCreated, value)
}

func (s *Server) listImportBatches(writer http.ResponseWriter, request *http.Request) {
	filter := insights.ImportBatchFilter{
		DataSourceID: request.URL.Query().Get("data_source_id"), Limit: queryLimit(request),
	}
	for _, value := range queryList(request, "status") {
		filter.Statuses = append(filter.Statuses, insights.ImportStatus(value))
	}
	values, err := s.app.ListImportBatches(request.Context(), mustActor(request), projectID(request), filter)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"items": values})
}

// metricOverview backs 投后分析 · 指标总览. The window is explicit in the URL
// because 20 §4.1 requires the 数据窗口 to be on screen — a hidden default would
// make two people read the same chart as two different periods.
func (s *Server) metricOverview(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	window, ok := parseWindow(writer, request, query.Get("start"), query.Get("end"))
	if !ok {
		return
	}
	value, err := s.app.GetMetricOverview(request.Context(), mustActor(request), projectID(request), window)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

// performanceAnalysis backs 投后分析的另外五个二级视图（素材对比、趋势、疲劳、
// 异常、驱动因素）。和 metric-overview 分成两个端点，是因为总览只读指标事实，
// 这一个还要读素材特征；但五个视图之间共用一次返回，否则「趋势里看到的」和
// 「疲劳里算的」会来自两次不同的读取，对不上时没人解释得清。
func (s *Server) performanceAnalysis(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	window, ok := parseWindow(writer, request, query.Get("start"), query.Get("end"))
	if !ok {
		return
	}
	value, err := s.app.GetPerformanceAnalysis(request.Context(), mustActor(request), projectID(request), window)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

// dataQuality backs 数据质量的六个二级视图。一次返回全部问题而不是按类型分六个
// 端点：六个视图共用同一份判断（严重度排序、修复队列、能不能给强结论），拆开会让
// 「队列里到底还有几条」在不同视图里算出不同的数。
func (s *Server) dataQuality(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	window, ok := parseWindow(writer, request, query.Get("start"), query.Get("end"))
	if !ok {
		return
	}
	value, err := s.app.GetDataQuality(request.Context(), mustActor(request), projectID(request), window)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, value)
}

// resolveQualityIssue 记一次处置。没有 DELETE：撤销处置也是一次处置（改成 acknowledged），
// 这样「谁在什么时候改了主意」不会从记录里消失。
func (s *Server) resolveQualityIssue(writer http.ResponseWriter, request *http.Request) {
	var body insights.ResolveQualityIssueRequest
	if !decode(writer, request, &body) {
		return
	}
	value, err := s.app.ResolveQualityIssue(request.Context(), mustActor(request), projectID(request), body)
	if err != nil {
		writeError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusCreated, value)
}

// parseWindow defaults to the last 30 days ending today so a bare page load
// still shows something; both ends are always echoed back in the payload.
func parseWindow(writer http.ResponseWriter, request *http.Request, start, end string) (insights.MetricWindow, bool) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	window := insights.MetricWindow{Start: today.AddDate(0, 0, -29), End: today}
	if trimmed := strings.TrimSpace(start); trimmed != "" {
		parsed, err := time.ParseInLocation("2006-01-02", trimmed, time.UTC)
		if err != nil {
			writeError(writer, request, insights.ErrInvalidRequest)
			return insights.MetricWindow{}, false
		}
		window.Start = parsed
	}
	if trimmed := strings.TrimSpace(end); trimmed != "" {
		parsed, err := time.ParseInLocation("2006-01-02", trimmed, time.UTC)
		if err != nil {
			writeError(writer, request, insights.ErrInvalidRequest)
			return insights.MetricWindow{}, false
		}
		window.End = parsed
	}
	return window, true
}
