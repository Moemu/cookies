package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/insights"
)

func TestInsightsHTTPExposesReportExperienceAndPreLaunchLoop(t *testing.T) {
	t.Parallel()
	app := &applicationStub{
		report:     insights.InsightReport{ID: "insightreport_1", Version: 1},
		experience: insights.Experience{ID: "experience_1"},
	}
	server := New(app)
	tests := []struct {
		method string
		path   string
		body   string
		status int
		want   string
	}{
		{http.MethodPost, "/api/insights/v1/projects/project_1/reports", `{"execution_id":"deliveryexecution_1","summary":"摘要","findings":["发现"]}`, 201, "insightreport_1"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/reports/insightreport_1:confirm", `{"expected_version":1}`, 200, "insightreport_1"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/reports/insightreport_1:create-experience", `{"expected_report_version":1,"conclusion":"结论","conditions":[],"counterexamples":[]}`, 201, "experience_1"},
		{http.MethodGet, "/api/insights/v1/projects/project_1/prelaunch", "", 200, "experience_1"},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, authenticatedRequest(test.method, test.path, test.body))
		if response.Code != test.status || !strings.Contains(response.Body.String(), test.want) {
			t.Fatalf("%s status=%d body=%s", test.path, response.Code, response.Body.String())
		}
	}
}

// 跨渠道比较默认关闭（03 §10.3②）。缺参数、写别的值都算关闭——
// 只有显式 true 才打开，因为打开意味着把不可直接比较的渠道并排放。
func TestPreLaunchCrossChannelIsOffUnlessExplicitlyTrue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		query string
		want  bool
	}{
		{"", false},
		{"?cross_channel=false", false},
		{"?cross_channel=1", false},
		{"?cross_channel=true", true},
	}
	for _, test := range tests {
		app := &applicationStub{experience: insights.Experience{ID: "experience_1"}}
		server := New(app)
		response := httptest.NewRecorder()
		server.ServeHTTP(response, authenticatedRequest(http.MethodGet,
			"/api/insights/v1/projects/project_1/prelaunch"+test.query, ""))
		if response.Code != http.StatusOK {
			t.Fatalf("%q status=%d", test.query, response.Code)
		}
		if app.preLaunchFilter.CrossChannel != test.want {
			t.Fatalf("%q cross_channel=%v，想要 %v", test.query, app.preLaunchFilter.CrossChannel, test.want)
		}
	}
}

func TestPreLaunchPassesScopeFiltersThrough(t *testing.T) {
	t.Parallel()
	app := &applicationStub{experience: insights.Experience{ID: "experience_1"}}
	server := New(app)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, authenticatedRequest(http.MethodGet,
		"/api/insights/v1/projects/project_1/prelaunch?channel=douyin&creative_type=short_video&objective=conversion&q=首图", ""))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	got := app.preLaunchFilter
	if got.Channel != "douyin" || got.CreativeType != "short_video" ||
		got.Objective != "conversion" || got.Query != "首图" {
		t.Fatalf("筛选条件 = %#v", got)
	}
}

func TestInsightsHTTPExposesExperienceLifecycleActions(t *testing.T) {
	t.Parallel()
	app := &applicationStub{experience: insights.Experience{ID: "experience_1"}}
	server := New(app)
	tests := []struct {
		method string
		path   string
		body   string
		status int
		want   string
	}{
		{http.MethodPost, "/api/insights/v1/projects/project_1/experiences/experience_1:confirm", `{"expected_version":1}`, 200, "experience_1"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/experiences/experience_1:reject", `{"expected_version":1,"reason":"证据不足"}`, 200, "experience_1"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/experiences/experience_1:request-review", `{"expected_version":2,"reason":"新数据冲突"}`, 200, "experience_1"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/experiences/experience_1:retire", `{"expected_version":3,"reason":"结论已过时"}`, 200, "experience_1"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/experiences/experience_1:revise", `{"expected_version":2,"conclusion":"新结论","conditions":[],"counterexamples":[],"reason":"补充样本"}`, 201, "experience_1"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/experiences/experience_1:record-reference", `{"consumer_kind":"strategy","consumer_id":"strategy_1","outcome":"adopted","note":""}`, 201, "experienceref_1"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/experiences/experience_1:unknown", `{}`, 404, ""},
		{http.MethodGet, "/api/insights/v1/projects/project_1/experiences/experience_1/references", "", 200, "experienceref_1"},
		{http.MethodGet, "/api/insights/v1/projects/project_1/experiences/experience_1/audits", "", 200, "experienceaudit_1"},
		{http.MethodGet, "/api/insights/v1/projects/project_1/experiences/experience_1/lineage", "", 200, "experience_1"},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, authenticatedRequest(test.method, test.path, test.body))
		if response.Code != test.status || !strings.Contains(response.Body.String(), test.want) {
			t.Fatalf("%s status=%d body=%s", test.path, response.Code, response.Body.String())
		}
	}
}

// The experience library filters by lifecycle status, so the 待确认 queue and
// the quotable list are separate reads rather than one client-side split.
func TestListExperiencesForwardsStatusFilter(t *testing.T) {
	t.Parallel()
	app := &applicationStub{experience: insights.Experience{ID: "experience_1"}}
	server := New(app)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, authenticatedRequest(http.MethodGet, "/api/insights/v1/projects/project_1/experiences?status=pending", ""))
	if response.Code != http.StatusOK || app.listedStatus != insights.ExperiencePending {
		t.Fatalf("status=%d filter=%q", response.Code, app.listedStatus)
	}
}

func TestInsightsHTTPExposesAssetAnalysisSurface(t *testing.T) {
	t.Parallel()
	app := &applicationStub{
		asset:   insights.Asset{ID: "insightasset_1", Version: 1},
		mapping: insights.AssetMapping{ID: "insightassetmapping_1", Version: 1},
		feature: insights.AssetFeature{ID: "assetfeature_1", Key: "article_type"},
	}
	server := New(app)
	tests := []struct {
		method string
		path   string
		body   string
		status int
		want   string
	}{
		{http.MethodPost, "/api/insights/v1/projects/project_1/assets", `{"title":"素材","source_kind":"upload"}`, 201, "insightasset_1"},
		{http.MethodGet, "/api/insights/v1/projects/project_1/assets", "", 200, "insightasset_1"},
		{http.MethodGet, "/api/insights/v1/projects/project_1/assets/insightasset_1", "", 200, "insightasset_1"},
		{http.MethodGet, "/api/insights/v1/projects/project_1/assets/insightasset_1/lineage", "", 200, "insightasset_1"},
		{http.MethodGet, "/api/insights/v1/projects/project_1/assets/insightasset_1/features", "", 200, "article_type"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/assets/insightasset_1:identify-type",
			`{"expected_version":1,"asset_type":"wechat_article","source":"human","confidence":"","reason":""}`, 200, "insightasset_1"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/assets/insightasset_1:extract-features",
			`{"expected_version":1,"skill_id":"skill_1","skill_version":"v1","features":[{"key":"article_type","value":{"kind":"enum","terms":["知识"]},"confidence":"low"}]}`, 200, "article_type"},
		{http.MethodPatch, "/api/insights/v1/projects/project_1/assets/insightasset_1/features",
			`{"expected_version":2,"features":[{"key":"article_type","value":{"kind":"enum","terms":["案例"]}}],"reason":"人工修正"}`, 200, "article_type"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/assets/insightasset_1:confirm", `{"expected_version":3,"reason":""}`, 200, "insightasset_1"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/assets/insightasset_1:request-review", `{"expected_version":4,"reason":"新数据冲突"}`, 200, "insightasset_1"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/assets/insightasset_1:retire", `{"expected_version":5,"reason":"源文件下线"}`, 200, "insightasset_1"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/assets/insightasset_1:unknown", `{}`, 404, ""},
		{http.MethodPost, "/api/insights/v1/projects/project_1/asset-mappings",
			`{"platform":"demo_platform","platform_object_kind":"creative","platform_object_id":"cr-1","platform_object_name":"创意","asset_id":"","match_source":"","note":""}`, 201, "insightassetmapping_1"},
		{http.MethodGet, "/api/insights/v1/projects/project_1/asset-mappings", "", 200, "insightassetmapping_1"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/asset-mappings/insightassetmapping_1:resolve",
			`{"expected_version":1,"status":"matched","asset_id":"insightasset_1","note":""}`, 200, "insightassetmapping_1"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/asset-mappings/insightassetmapping_1:unknown", `{}`, 404, ""},
		{http.MethodGet, "/api/insights/v1/projects/project_1/feature-schemas", "", 200, "wechat_article"},
		{http.MethodGet, "/api/insights/v1/projects/project_1/feature-matrix?asset_ids=insightasset_1", "", 200, "共同特征"},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, authenticatedRequest(test.method, test.path, test.body))
		if response.Code != test.status || !strings.Contains(response.Body.String(), test.want) {
			t.Fatalf("%s %s status=%d body=%s", test.method, test.path, response.Code, response.Body.String())
		}
	}
}

// 22 §8.3 要求每个可见 L2 标签都真实改变数据集，所以筛选条件必须原样传到服务层，
// 而不是前端拿到同一批数据自己分。
func TestAssetQueriesForwardEveryFilterToTheService(t *testing.T) {
	t.Parallel()
	app := &applicationStub{}
	server := New(app)

	server.ServeHTTP(httptest.NewRecorder(), authenticatedRequest(http.MethodGet,
		"/api/insights/v1/projects/project_1/assets?status=awaiting_match,analysable&asset_type=wechat_article&source_kind=upload&lineage_id=insightasset_1&limit=7", ""))
	filter := app.assetFilter
	if len(filter.Statuses) != 2 || filter.Statuses[0] != insights.AnalysisAwaitingMatch ||
		filter.Statuses[1] != insights.AnalysisAnalysable {
		t.Fatalf("statuses=%#v", filter.Statuses)
	}
	if len(filter.AssetTypes) != 1 || filter.AssetTypes[0] != insights.AssetTypeWechatArticle ||
		len(filter.SourceKinds) != 1 || filter.SourceKinds[0] != insights.AssetSourceUpload ||
		filter.LineageID != "insightasset_1" || filter.Limit != 7 {
		t.Fatalf("filter=%#v", filter)
	}

	server.ServeHTTP(httptest.NewRecorder(), authenticatedRequest(http.MethodGet,
		"/api/insights/v1/projects/project_1/asset-mappings?status=unmatched&platform=demo_platform", ""))
	if len(app.mappingFilter.Statuses) != 1 || app.mappingFilter.Statuses[0] != insights.MappingUnmatched ||
		app.mappingFilter.Platform != "demo_platform" {
		t.Fatalf("mappingFilter=%#v", app.mappingFilter)
	}

	server.ServeHTTP(httptest.NewRecorder(), authenticatedRequest(http.MethodGet,
		"/api/insights/v1/projects/project_1/feature-matrix?asset_ids=a&asset_ids=b,c", ""))
	if len(app.matrixAssetIDs) != 3 || app.matrixAssetIDs[2] != "c" {
		t.Fatalf("matrixAssetIDs=%#v", app.matrixAssetIDs)
	}
}

func authenticatedRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	ctx := contract.WithRequestContext(request.Context(), contract.RequestContext{
		RequestID: "req_1", TraceID: "trace_1",
		Actor: contract.ActorContext{
			OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
			Scopes: []contract.Scope{insights.ScopeRead, insights.ScopeWrite, insights.ScopeConfirm},
		},
	})
	return request.WithContext(ctx)
}

type applicationStub struct {
	report          insights.InsightReport
	experience      insights.Experience
	listedStatus    insights.ExperienceStatus
	preLaunchFilter insights.PreLaunchFilter

	asset          insights.Asset
	mapping        insights.AssetMapping
	feature        insights.AssetFeature
	assetFilter    insights.AssetFilter
	mappingFilter  insights.AssetMappingFilter
	matrixAssetIDs []string

	dataSource        insights.DataSource
	importBatch       insights.ImportBatch
	dataSourceFilter  insights.DataSourceFilter
	importBatchFilter insights.ImportBatchFilter
	importedRows      int
	window            insights.MetricWindow

	qualityReport  insights.QualityReport
	qualityRequest insights.ResolveQualityIssueRequest

	capabilityOperations insights.CapabilityOperations
}

func (s *applicationStub) CreateReport(context.Context, contract.ActorContext, contract.ProjectID, insights.CreateReportRequest) (insights.InsightReport, error) {
	return s.report, nil
}
func (s *applicationStub) ListReports(context.Context, contract.ActorContext, contract.ProjectID, int) ([]insights.InsightReport, error) {
	return []insights.InsightReport{s.report}, nil
}
func (s *applicationStub) ConfirmReport(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (insights.InsightReport, error) {
	return s.report, nil
}
func (s *applicationStub) CreateExperience(context.Context, contract.ActorContext, contract.ProjectID, string, int64, insights.CreateExperienceRequest) (insights.Experience, error) {
	return s.experience, nil
}
func (s *applicationStub) ListExperiences(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, status insights.ExperienceStatus, _ int) ([]insights.Experience, error) {
	s.listedStatus = status
	return []insights.Experience{s.experience}, nil
}
func (s *applicationStub) ConfirmExperience(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (insights.Experience, error) {
	return s.experience, nil
}
func (s *applicationStub) RejectExperience(context.Context, contract.ActorContext, contract.ProjectID, string, insights.ExperienceTransitionRequest) (insights.Experience, error) {
	return s.experience, nil
}
func (s *applicationStub) RequestExperienceReview(context.Context, contract.ActorContext, contract.ProjectID, string, insights.ExperienceTransitionRequest) (insights.Experience, error) {
	return s.experience, nil
}
func (s *applicationStub) RetireExperience(context.Context, contract.ActorContext, contract.ProjectID, string, insights.ExperienceTransitionRequest) (insights.Experience, error) {
	return s.experience, nil
}
func (s *applicationStub) ReviseExperience(context.Context, contract.ActorContext, contract.ProjectID, string, insights.ReviseExperienceRequest) (insights.Experience, error) {
	return s.experience, nil
}
func (s *applicationStub) RecordExperienceReference(context.Context, contract.ActorContext, contract.ProjectID, string, insights.RecordExperienceReferenceRequest) (insights.ExperienceReference, error) {
	return insights.ExperienceReference{ID: "experienceref_1", ExperienceID: s.experience.ID}, nil
}
func (s *applicationStub) ListExperienceReferences(context.Context, contract.ActorContext, contract.ProjectID, string, int) ([]insights.ExperienceReference, error) {
	return []insights.ExperienceReference{{ID: "experienceref_1", ExperienceID: s.experience.ID}}, nil
}
func (s *applicationStub) ListProjectExperienceReferences(context.Context, contract.ActorContext, contract.ProjectID, int) ([]insights.ExperienceReference, error) {
	return []insights.ExperienceReference{{ID: "experienceref_1", ExperienceID: s.experience.ID}}, nil
}
func (s *applicationStub) ListExperienceAudits(context.Context, contract.ActorContext, contract.ProjectID, string, int) ([]insights.ExperienceAudit, error) {
	return []insights.ExperienceAudit{{ID: "experienceaudit_1", ExperienceID: s.experience.ID}}, nil
}
func (s *applicationStub) ListExperienceLineage(context.Context, contract.ActorContext, contract.ProjectID, string) ([]insights.Experience, error) {
	return []insights.Experience{s.experience}, nil
}
func (s *applicationStub) GetPreLaunch(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, filter insights.PreLaunchFilter) (insights.PreLaunchInsight, error) {
	s.preLaunchFilter = filter
	return insights.PreLaunchInsight{ExperienceReferences: []insights.Experience{s.experience}}, nil
}
func (s *applicationStub) GetPerformance(context.Context, contract.ActorContext, contract.ProjectID) (insights.PerformanceOverview, error) {
	return insights.PerformanceOverview{}, nil
}

func (s *applicationStub) IndexAsset(context.Context, contract.ActorContext, contract.ProjectID, insights.IndexAssetRequest) (insights.Asset, error) {
	return s.asset, nil
}
func (s *applicationStub) ListAssets(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, filter insights.AssetFilter) ([]insights.Asset, error) {
	s.assetFilter = filter
	return []insights.Asset{s.asset}, nil
}
func (s *applicationStub) GetAsset(context.Context, contract.ActorContext, contract.ProjectID, string) (insights.Asset, error) {
	return s.asset, nil
}
func (s *applicationStub) ListAssetLineage(context.Context, contract.ActorContext, contract.ProjectID, string) ([]insights.Asset, error) {
	return []insights.Asset{s.asset}, nil
}
func (s *applicationStub) IdentifyAssetType(context.Context, contract.ActorContext, contract.ProjectID, string, insights.IdentifyAssetTypeRequest) (insights.Asset, error) {
	return s.asset, nil
}
func (s *applicationStub) RegisterAssetMapping(context.Context, contract.ActorContext, contract.ProjectID, insights.RegisterAssetMappingRequest) (insights.AssetMapping, error) {
	return s.mapping, nil
}
func (s *applicationStub) ListAssetMappings(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, filter insights.AssetMappingFilter) ([]insights.AssetMapping, error) {
	s.mappingFilter = filter
	return []insights.AssetMapping{s.mapping}, nil
}
func (s *applicationStub) ResolveAssetMapping(context.Context, contract.ActorContext, contract.ProjectID, string, insights.ResolveAssetMappingRequest) (insights.AssetMapping, error) {
	return s.mapping, nil
}
func (s *applicationStub) ExtractFeatures(context.Context, contract.ActorContext, contract.ProjectID, string, insights.ExtractFeaturesRequest) ([]insights.AssetFeature, error) {
	return []insights.AssetFeature{s.feature}, nil
}
func (s *applicationStub) PatchFeatures(context.Context, contract.ActorContext, contract.ProjectID, string, insights.PatchFeaturesRequest) ([]insights.AssetFeature, error) {
	return []insights.AssetFeature{s.feature}, nil
}
func (s *applicationStub) ListAssetFeatures(context.Context, contract.ActorContext, contract.ProjectID, string) ([]insights.AssetFeature, error) {
	return []insights.AssetFeature{s.feature}, nil
}
func (s *applicationStub) ConfirmAssetAnalysis(context.Context, contract.ActorContext, contract.ProjectID, string, insights.AssetTransitionRequest) (insights.Asset, error) {
	return s.asset, nil
}
func (s *applicationStub) RequestAssetReview(context.Context, contract.ActorContext, contract.ProjectID, string, insights.AssetTransitionRequest) (insights.Asset, error) {
	return s.asset, nil
}
func (s *applicationStub) RetireAsset(context.Context, contract.ActorContext, contract.ProjectID, string, insights.AssetTransitionRequest) (insights.Asset, error) {
	return s.asset, nil
}
func (s *applicationStub) GetFeatureMatrix(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, assetIDs []string) (insights.FeatureMatrix, error) {
	s.matrixAssetIDs = assetIDs
	assets := make([]insights.Asset, 0, len(assetIDs))
	for _, assetID := range assetIDs {
		assets = append(assets, insights.Asset{ID: assetID})
	}
	return insights.FeatureMatrix{Assets: assets, Disclosure: "仅比较各类型都有的共同特征。"}, nil
}
func (s *applicationStub) RegisterDataSource(context.Context, contract.ActorContext, contract.ProjectID, insights.RegisterDataSourceRequest) (insights.DataSource, error) {
	return s.dataSource, nil
}
func (s *applicationStub) ListDataSources(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, filter insights.DataSourceFilter) ([]insights.DataSource, error) {
	s.dataSourceFilter = filter
	return []insights.DataSource{s.dataSource}, nil
}
func (s *applicationStub) GetDataSource(context.Context, contract.ActorContext, contract.ProjectID, string) (insights.DataSource, error) {
	return s.dataSource, nil
}
func (s *applicationStub) UpdateDataSource(context.Context, contract.ActorContext, contract.ProjectID, string, insights.UpdateDataSourceRequest) (insights.DataSource, error) {
	return s.dataSource, nil
}
func (s *applicationStub) SetDataSourceQuality(context.Context, contract.ActorContext, contract.ProjectID, string, insights.SetDataSourceQualityRequest) (insights.DataSource, error) {
	return s.dataSource, nil
}
func (s *applicationStub) ImportMetrics(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, request insights.ImportMetricsRequest) (insights.ImportResult, error) {
	s.importedRows = len(request.Rows)
	return insights.ImportResult{Batch: s.importBatch}, nil
}
func (s *applicationStub) ListImportBatches(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, filter insights.ImportBatchFilter) ([]insights.ImportBatch, error) {
	s.importBatchFilter = filter
	return []insights.ImportBatch{s.importBatch}, nil
}
func (s *applicationStub) GetMetricOverview(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, window insights.MetricWindow) (insights.MetricOverview, error) {
	s.window = window
	return insights.MetricOverview{Window: window, Confidence: insights.ConfidenceLowSample,
		ConfidenceNote: "窗口内样本不足，只能当作观察。"}, nil
}

func (s *applicationStub) GetPerformanceAnalysis(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, window insights.MetricWindow) (insights.PerformanceAnalysis, error) {
	s.window = window
	return insights.PerformanceAnalysis{Window: window, Comparable: true}, nil
}
func (s *applicationStub) GetDataQuality(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, window insights.MetricWindow) (insights.QualityReport, error) {
	s.window = window
	report := s.qualityReport
	report.Window = window
	return report, nil
}
func (s *applicationStub) GetCapabilityOperations(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, window insights.MetricWindow) (insights.CapabilityOperations, error) {
	s.window = window
	report := s.capabilityOperations
	report.Window = window
	return report, nil
}
func (s *applicationStub) ResolveQualityIssue(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, request insights.ResolveQualityIssueRequest) (insights.QualityDisposition, error) {
	s.qualityRequest = request
	return insights.QualityDisposition{
		ID: "insightqualitydisposition_1", Fingerprint: request.Fingerprint,
		IssueKind: request.IssueKind, State: request.State, Note: request.Note, Version: 1,
	}, nil
}

func TestInsightsHTTPExposesDataIngestionSurface(t *testing.T) {
	t.Parallel()
	app := &applicationStub{
		dataSource:  insights.DataSource{ID: "insightsource_1", Version: 1},
		importBatch: insights.ImportBatch{ID: "insightbatch_1", Version: 1},
	}
	server := New(app)
	tests := []struct {
		method string
		path   string
		body   string
		status int
		want   string
	}{
		{http.MethodPost, "/api/insights/v1/projects/project_1/data-sources",
			`{"platform":"douyin","account_label":"主账户","account_ref":"adv_1","ingest_mode":"api","credential_ref":"vault://douyin","caliber":{"time_zone":"Asia/Shanghai","currency":"CNY","attribution_window":"click_7d","metric_schema_version":"v1"},"field_mapping":{"展示数":"impressions"}}`,
			201, "insightsource_1"},
		{http.MethodGet, "/api/insights/v1/projects/project_1/data-sources", "", 200, "insightsource_1"},
		{http.MethodGet, "/api/insights/v1/projects/project_1/data-sources/insightsource_1", "", 200, "insightsource_1"},
		{http.MethodPatch, "/api/insights/v1/projects/project_1/data-sources/insightsource_1",
			`{"expected_version":1,"status":"active","field_mapping":{"展示数":"impressions"}}`, 200, "insightsource_1"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/data-sources/insightsource_1:set-quality",
			`{"expected_version":2,"quality_status":"delayed","note":"平台回传延迟"}`, 200, "insightsource_1"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/data-sources/insightsource_1:unknown", `{}`, 404, ""},
		{http.MethodPost, "/api/insights/v1/projects/project_1/import-batches",
			`{"data_source_id":"insightsource_1","kind":"file","source_label":"7月.csv","content_hash":"hash_1","corrects_batch_id":"","register_objects":true,"rows":[{"platform_object_kind":"creative","platform_object_id":"c1","platform_object_name":"夏季前贴","stat_date":"2026-07-20","counts":{"impressions":1000,"clicks":20,"conversions":1,"video_views":0,"video_completions":0,"spend_cents":5000,"revenue_cents":0}}]}`,
			201, "insightbatch_1"},
		{http.MethodGet, "/api/insights/v1/projects/project_1/import-batches", "", 200, "insightbatch_1"},
		{http.MethodGet, "/api/insights/v1/projects/project_1/metric-overview?start=2026-07-01&end=2026-07-20", "", 200, "样本不足"},
		{http.MethodGet, "/api/insights/v1/projects/project_1/metric-overview?start=不是日期", "", 400, "INVALID_REQUEST"},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, authenticatedRequest(test.method, test.path, test.body))
		if response.Code != test.status || !strings.Contains(response.Body.String(), test.want) {
			t.Fatalf("%s %s status=%d body=%s", test.method, test.path, response.Code, response.Body.String())
		}
	}
	if app.importedRows != 1 {
		t.Fatalf("导入的行应当原样传到服务层：%d", app.importedRows)
	}
}

// 数据窗口必须来自 URL 并原样传下去（20 §4.1「必须显示数据窗口」），
// 缺省时给最近 30 天，而不是让服务层各自猜一个。
func TestConnectorQueriesForwardWindowAndFilters(t *testing.T) {
	t.Parallel()
	app := &applicationStub{}
	server := New(app)

	server.ServeHTTP(httptest.NewRecorder(), authenticatedRequest(http.MethodGet,
		"/api/insights/v1/projects/project_1/data-sources?status=active,draft&platform=douyin&limit=9", ""))
	if len(app.dataSourceFilter.Statuses) != 2 || app.dataSourceFilter.Statuses[0] != insights.DataSourceActive ||
		len(app.dataSourceFilter.Platforms) != 1 || app.dataSourceFilter.Platforms[0] != insights.PlatformDouyin ||
		app.dataSourceFilter.Limit != 9 {
		t.Fatalf("dataSourceFilter=%#v", app.dataSourceFilter)
	}

	server.ServeHTTP(httptest.NewRecorder(), authenticatedRequest(http.MethodGet,
		"/api/insights/v1/projects/project_1/import-batches?data_source_id=insightsource_1&status=failed,partial", ""))
	if app.importBatchFilter.DataSourceID != "insightsource_1" || len(app.importBatchFilter.Statuses) != 2 ||
		app.importBatchFilter.Statuses[0] != insights.ImportFailed {
		t.Fatalf("importBatchFilter=%#v", app.importBatchFilter)
	}

	server.ServeHTTP(httptest.NewRecorder(), authenticatedRequest(http.MethodGet,
		"/api/insights/v1/projects/project_1/metric-overview?start=2026-07-01&end=2026-07-20", ""))
	if app.window.Start.Format("2006-01-02") != "2026-07-01" || app.window.End.Format("2006-01-02") != "2026-07-20" {
		t.Fatalf("window=%#v", app.window)
	}

	server.ServeHTTP(httptest.NewRecorder(), authenticatedRequest(http.MethodGet,
		"/api/insights/v1/projects/project_1/metric-overview", ""))
	if app.window.Days() != 30 {
		t.Fatalf("缺省窗口应当是最近 30 天：%d 天", app.window.Days())
	}
}

func TestInsightsHTTPExposesDataQualitySurface(t *testing.T) {
	t.Parallel()
	app := &applicationStub{qualityReport: insights.QualityReport{
		Issues: []insights.QualityIssue{{
			Fingerprint: "freshness|lag|insightsource_1", Kind: insights.QualityFreshness,
			Severity: insights.SeverityBlocking, Title: "抖音 · 主账户 的数据滞后 6 天",
			State: insights.QualityOpen,
		}},
		ByKind: map[insights.QualityIssueKind]int{insights.QualityFreshness: 1},
		// 有阻断级问题时禁止强结论，前端靠这两个字段决定要不要收起结论区。
		StrongConclusionsAllowed: false, BlockedReason: "抖音 · 主账户 的数据滞后 6 天",
	}}
	server := New(app)
	tests := []struct {
		method string
		path   string
		body   string
		status int
		want   string
	}{
		{http.MethodGet, "/api/insights/v1/projects/project_1/data-quality?start=2026-07-01&end=2026-07-20",
			"", 200, "滞后 6 天"},
		{http.MethodGet, "/api/insights/v1/projects/project_1/data-quality?start=不是日期", "", 400, "INVALID_REQUEST"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/data-quality/dispositions",
			`{"fingerprint":"freshness|lag|insightsource_1","issue_kind":"freshness","state":"resolved","note":"已让平台侧重跑同步","observed_through":"2026-07-29T10:00:00Z"}`,
			201, "insightqualitydisposition_1"},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, authenticatedRequest(test.method, test.path, test.body))
		if response.Code != test.status || !strings.Contains(response.Body.String(), test.want) {
			t.Fatalf("%s %s status=%d body=%s", test.method, test.path, response.Code, response.Body.String())
		}
	}
	// observed_through 由前端回传它当时看到的那条问题的观测时间，服务端不能自己取 now，
	// 否则处置会连带盖掉处置之后才恶化的情况。
	if app.qualityRequest.ObservedThrough.Format(time.RFC3339) != "2026-07-29T10:00:00Z" {
		t.Fatalf("observed_through 没有原样传到服务层：%#v", app.qualityRequest)
	}
	if app.qualityRequest.State != insights.DispositionResolved {
		t.Fatalf("处置状态没有原样传到服务层：%#v", app.qualityRequest)
	}
}

func TestInsightsHTTPExposesCapabilityOperationsSurface(t *testing.T) {
	t.Parallel()
	app := &applicationStub{capabilityOperations: insights.CapabilityOperations{
		FeatureSystems: []insights.FeatureSystemHealth{{
			AssetType: insights.AssetTypePrerollAd, Label: "前贴片广告", AssetCount: 12,
			Fields: []insights.FeatureFieldUsage{{
				FeatureField: insights.FeatureField{Key: "opening_structure", Label: "开场结构"},
				AssetCount:   12, DistinctValues: 9,
				MergeCandidates: []string{"悬念式开场"},
			}},
		}},
		Evaluations: []insights.SkillEvaluation{{
			SkillID: "skill_preroll", SkillVersion: "v1", Reviewed: 4,
			Confidence: insights.ConfidenceLowSample,
			Note:       "样本不足 10 条，不给准确率",
		}},
	}}
	server := New(app)
	tests := []struct {
		path   string
		status int
		want   string
	}{
		{"/api/insights/v1/projects/project_1/capability-operations?start=2026-07-01&end=2026-07-20", 200, "悬念式开场"},
		// 样本不足这件事必须能从接口读出来，前端才知道该把准确率藏起来。
		{"/api/insights/v1/projects/project_1/capability-operations", 200, "low_sample"},
		{"/api/insights/v1/projects/project_1/capability-operations?start=不是日期", 400, "INVALID_REQUEST"},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, authenticatedRequest(http.MethodGet, test.path, ""))
		if response.Code != test.status || !strings.Contains(response.Body.String(), test.want) {
			t.Fatalf("GET %s status=%d body=%s", test.path, response.Code, response.Body.String())
		}
	}
	if app.window.Days() != 30 {
		t.Fatalf("缺省窗口应当是最近 30 天：%d 天", app.window.Days())
	}
}
