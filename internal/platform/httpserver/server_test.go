package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/agent"
	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/platform/knowledge"
	"github.com/shikanon/cookies/internal/platform/project"
	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/platform/remix"
)

func TestHealthDoesNotRequireIdentity(t *testing.T) {
	t.Parallel()
	server := New(identity.RejectingResolver{})
	response := httptest.NewRecorder()

	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if got, want := response.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected response request ID")
	}
}

func TestProjectWorkflowRoutesCoverDetailTasksOperationsChangeSetsAndAudit(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{
		OrganizationID: "org_1",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"},
		Scopes:         []contract.Scope{"project.read", "project.write"},
	}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	manager := &workflowProjectManager{projectValue: project.Project{
		ID:                    "project_1",
		OrganizationID:        "org_1",
		Name:                  "Investor Demo",
		Status:                project.StatusActive,
		ProjectContextVersion: 1,
		CreatedAt:             now,
		UpdatedAt:             now,
	}}
	server := NewWithDependencies(Dependencies{
		Resolver:          resolver,
		ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"},
		Projects:          manager,
	})

	createArtifact := httptest.NewRecorder()
	createArtifactRequest := httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/artifacts", strings.NewReader(`{"kind":"brief","status":"draft","content":"首版策略 Brief"}`))
	createArtifactRequest.Header.Set("Idempotency-Key", "artifact-create-1")
	server.ServeHTTP(createArtifact, createArtifactRequest)
	if createArtifact.Code != http.StatusCreated || createArtifact.Header().Get("Location") != "/platform/v1/projects/project_1/artifacts/artifact_1" {
		t.Fatalf("create artifact status=%d location=%q body=%s", createArtifact.Code, createArtifact.Header().Get("Location"), createArtifact.Body.String())
	}
	listArtifacts := httptest.NewRecorder()
	server.ServeHTTP(listArtifacts, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/artifacts", nil))
	if listArtifacts.Code != http.StatusOK || !strings.Contains(listArtifacts.Body.String(), `"id":"artifact_1"`) {
		t.Fatalf("list artifacts status=%d body=%s", listArtifacts.Code, listArtifacts.Body.String())
	}
	patchArtifact := httptest.NewRecorder()
	server.ServeHTTP(patchArtifact, httptest.NewRequest(http.MethodPatch, "/platform/v1/projects/project_1/artifacts/artifact_1", strings.NewReader(`{"content":"已确认策略 Brief","status":"ready","expected_version":1}`)))
	if patchArtifact.Code != http.StatusOK || !strings.Contains(patchArtifact.Body.String(), `"version":2`) {
		t.Fatalf("patch artifact status=%d body=%s", patchArtifact.Code, patchArtifact.Body.String())
	}

	taskBody := `{"type":"creative","name":"生成素材","objective":"生成首版广告素材","source_task_ids":["task_strategy"],"source_artifact_ids":["brief_v1"]}`
	createTask := httptest.NewRecorder()
	createTaskRequest := httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/tasks", strings.NewReader(taskBody))
	createTaskRequest.Header.Set("Idempotency-Key", "task-create-1")
	server.ServeHTTP(createTask, createTaskRequest)
	if createTask.Code != http.StatusCreated || createTask.Header().Get("Location") != "/platform/v1/projects/project_1/tasks/task_1" {
		t.Fatalf("create task status=%d location=%q body=%s", createTask.Code, createTask.Header().Get("Location"), createTask.Body.String())
	}

	patchTask := httptest.NewRecorder()
	server.ServeHTTP(patchTask, httptest.NewRequest(http.MethodPatch, "/platform/v1/projects/project_1/tasks/task_1", strings.NewReader(`{"status":"ready","output_artifact_ids":["creative_v1"],"expected_version":1}`)))
	if patchTask.Code != http.StatusOK || !strings.Contains(patchTask.Body.String(), `"status":"ready"`) {
		t.Fatalf("patch task status=%d body=%s", patchTask.Code, patchTask.Body.String())
	}

	operationBody := `{"kind":"metric","title":"投放健康度","status":"healthy","occurred_at":"2026-07-28T10:00:00Z","fields":{"owner":"ops","spend":120.5}}`
	createOperation := httptest.NewRecorder()
	createOperationRequest := httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/operations", strings.NewReader(operationBody))
	createOperationRequest.Header.Set("Idempotency-Key", "operation-create-1")
	server.ServeHTTP(createOperation, createOperationRequest)
	if createOperation.Code != http.StatusCreated || createOperation.Header().Get("Location") != "/platform/v1/projects/project_1/operations/operation_1" {
		t.Fatalf("create operation status=%d location=%q body=%s", createOperation.Code, createOperation.Header().Get("Location"), createOperation.Body.String())
	}
	upsertOperation := httptest.NewRecorder()
	server.ServeHTTP(upsertOperation, httptest.NewRequest(http.MethodPut, "/platform/v1/projects/project_1/operations/stable_op", strings.NewReader(operationBody)))
	if upsertOperation.Code != http.StatusOK || !strings.Contains(upsertOperation.Body.String(), `"id":"stable_op"`) {
		t.Fatalf("upsert operation status=%d body=%s", upsertOperation.Code, upsertOperation.Body.String())
	}

	changeSetBody := `{"name":"预算调优","artifact_refs":[{"project_id":"project_1","asset_version":{"asset_id":"asset_creative","version":1}}],"budget_limit":3000}`
	createChangeSet := httptest.NewRecorder()
	createChangeSetRequest := httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/change-sets", strings.NewReader(changeSetBody))
	createChangeSetRequest.Header.Set("Idempotency-Key", "changeset-create-1")
	server.ServeHTTP(createChangeSet, createChangeSetRequest)
	if createChangeSet.Code != http.StatusCreated || createChangeSet.Header().Get("Location") != "/platform/v1/projects/project_1/change-sets/changeset_1" {
		t.Fatalf("create change set status=%d location=%q body=%s", createChangeSet.Code, createChangeSet.Header().Get("Location"), createChangeSet.Body.String())
	}
	for _, step := range []struct {
		method string
		path   string
		body   string
		want   string
	}{
		{http.MethodPost, "/platform/v1/projects/project_1/change-sets/changeset_1/preflight", "", `"status":"preflight_passed"`},
		{http.MethodPost, "/platform/v1/projects/project_1/change-sets/changeset_1/approve", `{"actor":"demo-approver","role":"owner","note":"ok"}`, `"status":"approved"`},
		{http.MethodPost, "/platform/v1/projects/project_1/change-sets/changeset_1/execute", "", `"status":"executed"`},
		{http.MethodPost, "/platform/v1/projects/project_1/change-sets/changeset_1/rollback", `{"actor":"demo-approver","reason":"演示回滚"}`, `"status":"rolled_back"`},
	} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(step.method, step.path, strings.NewReader(step.body)))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), step.want) {
			t.Fatalf("%s status=%d want body containing %s body=%s", step.path, response.Code, step.want, response.Body.String())
		}
	}

	audit := httptest.NewRecorder()
	server.ServeHTTP(audit, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/audit-events?entity_type=change_set&entity_id=changeset_1", nil))
	if audit.Code != http.StatusOK || strings.Count(audit.Body.String(), `"entity_type":"change_set"`) != 5 {
		t.Fatalf("audit status=%d body=%s", audit.Code, audit.Body.String())
	}
	detail := httptest.NewRecorder()
	server.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1", nil))
	if detail.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", detail.Code, detail.Body.String())
	}
	for _, required := range []string{`"project":`, `"runtime":`, `"artifacts":[]`, `"assets":[]`, `"tasks":`, `"operations":`, `"change_sets":`} {
		if !strings.Contains(detail.Body.String(), required) {
			t.Fatalf("detail missing %s: %s", required, detail.Body.String())
		}
	}
	updated := httptest.NewRecorder()
	server.ServeHTTP(updated, httptest.NewRequest(http.MethodPatch, "/platform/v1/projects/project_1", strings.NewReader(`{"name":"Precision Evidence","brand":"White Precision","goal":"Create verified production leads","expected_context_version":1}`)))
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), `"name":"Precision Evidence"`) || !strings.Contains(updated.Body.String(), `"project_context_version":2`) {
		t.Fatalf("update project status=%d body=%s", updated.Code, updated.Body.String())
	}
	stale := httptest.NewRecorder()
	server.ServeHTTP(stale, httptest.NewRequest(http.MethodPatch, "/platform/v1/projects/project_1", strings.NewReader(`{"name":"stale","expected_context_version":1}`)))
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale update status=%d body=%s", stale.Code, stale.Body.String())
	}
	workbench := httptest.NewRecorder()
	server.ServeHTTP(workbench, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/workbench", nil))
	if workbench.Code != http.StatusOK || !strings.Contains(workbench.Body.String(), `"project_id":"project_1"`) {
		t.Fatalf("workbench status=%d body=%s", workbench.Code, workbench.Body.String())
	}
}

func TestGeneratedIntakeRouteRequiresScopeAndReturnsLocation(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}, Scopes: []contract.Scope{"assets.write"}}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	server := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, Intakes: fakeIntakeManager{}})
	now := time.Now().UTC()
	requestBody := assets.GeneratedAssetIntakeRequest{ProviderJobID: "job_1", Output: contract.ProviderOutputRef{ProviderCode: "fake", ProviderJobID: "job_1", OutputID: "out_1", RetrievalExpiresAt: now.Add(time.Hour), DeclaredMIMEType: "image/png", DeclaredSizeBytes: 100}, Provenance: assets.GenerationProvenance{Capability: "image.generate", ProviderCode: "fake", ModelAlias: "standard", ModelVersion: "v1", SourceAssetRefs: []contract.AssetVersionRef{}, ProjectContextVersion: 1, GeneratedAt: now}}
	body, _ := json.Marshal(requestBody)
	request := httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/assets/generated-intakes", bytes.NewReader(body))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if response.Header().Get("Location") != "/platform/v1/projects/project_1/assets/generated-intakes/intake_1" {
		t.Fatalf("location=%q", response.Header().Get("Location"))
	}
	responseBody := response.Body.String()
	for _, forbidden := range []string{"provider_code", "retrieval_expires_at", "declared_mime_type", "declared_size_bytes", "bucket", "object_key", "vendor"} {
		if strings.Contains(responseBody, forbidden) {
			t.Fatalf("generated intake response leaked %q: %s", forbidden, responseBody)
		}
	}

	actor.Scopes = []contract.Scope{}
	resolver, _ = identity.NewStaticResolver(actor)
	deniedServer := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, Intakes: fakeIntakeManager{}})
	denied := httptest.NewRecorder()
	deniedServer.ServeHTTP(denied, httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/assets/generated-intakes", bytes.NewReader(body)))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("scope denied status=%d", denied.Code)
	}
}

func TestRemoveProjectAssetRequiresWriteScope(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}, Scopes: []contract.Scope{"assets.write"}}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	uploads := &fakeUploadManager{}
	server := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, Uploads: uploads})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/platform/v1/projects/project_1/assets/asset_1/versions/3", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if uploads.removed.AssetID != "asset_1" || uploads.removed.Version != 3 {
		t.Fatalf("removed=%#v", uploads.removed)
	}

	actor.Scopes = nil
	resolver, _ = identity.NewStaticResolver(actor)
	deniedServer := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, Uploads: &fakeUploadManager{}})
	denied := httptest.NewRecorder()
	deniedServer.ServeHTTP(denied, httptest.NewRequest(http.MethodDelete, "/platform/v1/projects/project_1/assets/asset_1/versions/3", nil))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("scope denied status=%d", denied.Code)
	}
}

func TestRemixPlanRoutesCreateAndReadSavedPlan(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}, Scopes: []contract.Scope{"assets.write", "assets.read"}}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	plans := &fakeRemixPlanManager{}
	server := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, RemixPlans: plans})
	body, _ := json.Marshal(remix.CreatePlanRequest{
		SchemaVersion: SchemaVersionV2ForHTTPTest(),
		ClientPlanID:  "client_plan_1",
		TargetSeconds: 30,
		ActualSeconds: 9.6,
		Pace:          remix.PaceBalanced,
		Segments: []remix.SegmentPlan{
			httpRemixSegment(remix.SegmentOpening, "前段", "asset_opening"),
			httpRemixSegment(remix.SegmentMiddle, "中段", "asset_middle"),
			httpRemixSegment(remix.SegmentEnding, "后段", "asset_ending"),
		},
		Summary: remix.PlanSummary{SelectedAssets: 3, UsedAssets: 3, CoveragePercent: 32, Strategy: "balanced"},
	})

	create := httptest.NewRecorder()
	server.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/remix-plans", bytes.NewReader(body)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	if create.Header().Get("Location") != "/platform/v1/projects/project_1/remix-plans/remixplan_1" {
		t.Fatalf("location=%q", create.Header().Get("Location"))
	}
	var created remix.Plan
	if err := json.NewDecoder(create.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.SchemaVersion != remix.SchemaVersionV2 || len(created.Segments[0].Shots) != 1 {
		t.Fatalf("created plan did not preserve v2 shots: %#v", created)
	}

	read := httptest.NewRecorder()
	server.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/remix-plans/remixplan_1", nil))
	if read.Code != http.StatusOK {
		t.Fatalf("read status=%d body=%s", read.Code, read.Body.String())
	}
	list := httptest.NewRecorder()
	server.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/remix-plans?limit=5", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	var listBody struct {
		Items []remix.Plan `json:"items"`
	}
	if err := json.NewDecoder(list.Body).Decode(&listBody); err != nil {
		t.Fatal(err)
	}
	if len(listBody.Items) != 1 || listBody.Items[0].ID != "remixplan_1" {
		t.Fatalf("list body=%#v", listBody)
	}

	renderBody, _ := json.Marshal(remix.CreateRenderJobRequest{PlanID: "remixplan_1", TargetQuality: "draft"})
	render := httptest.NewRecorder()
	renderRequest := httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/remix-render-jobs", bytes.NewReader(renderBody))
	renderRequest.Header.Set("Idempotency-Key", "render-key-1")
	server.ServeHTTP(render, renderRequest)
	if render.Code != http.StatusAccepted {
		t.Fatalf("render status=%d body=%s", render.Code, render.Body.String())
	}
	if render.Header().Get("Location") != "/platform/v1/projects/project_1/remix-render-jobs/remixrender_1" {
		t.Fatalf("render location=%q", render.Header().Get("Location"))
	}
	renderRead := httptest.NewRecorder()
	server.ServeHTTP(renderRead, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/remix-render-jobs/remixrender_1", nil))
	if renderRead.Code != http.StatusOK {
		t.Fatalf("render read status=%d body=%s", renderRead.Code, renderRead.Body.String())
	}
	if plans.renderKey != "render-key-1" {
		t.Fatalf("render idempotency key = %q", plans.renderKey)
	}

	qualityBody, _ := json.Marshal(remix.CreateQualityReportRequest{RenderJobID: "remixrender_1"})
	quality := httptest.NewRecorder()
	server.ServeHTTP(quality, httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/remix-quality-reports", bytes.NewReader(qualityBody)))
	if quality.Code != http.StatusCreated {
		t.Fatalf("quality status=%d body=%s", quality.Code, quality.Body.String())
	}
	if quality.Header().Get("Location") != "/platform/v1/projects/project_1/remix-quality-reports/qualityreport_1" {
		t.Fatalf("quality location=%q", quality.Header().Get("Location"))
	}
	var report remix.QualityReport
	if err := json.NewDecoder(quality.Body).Decode(&report); err != nil {
		t.Fatal(err)
	}
	if report.RenderJobID != "remixrender_1" || report.Verdict != remix.QualityVerdictMajor || len(report.Issues) != 1 {
		t.Fatalf("quality report=%#v", report)
	}
	reportRead := httptest.NewRecorder()
	server.ServeHTTP(reportRead, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/remix-render-jobs/remixrender_1/quality-report", nil))
	if reportRead.Code != http.StatusOK {
		t.Fatalf("quality read status=%d body=%s", reportRead.Code, reportRead.Body.String())
	}
	var reportEnvelope struct {
		QualityReport *remix.QualityReport `json:"quality_report"`
	}
	if err := json.NewDecoder(reportRead.Body).Decode(&reportEnvelope); err != nil {
		t.Fatal(err)
	}
	if reportEnvelope.QualityReport == nil || reportEnvelope.QualityReport.ID != "qualityreport_1" {
		t.Fatalf("quality envelope=%#v", reportEnvelope)
	}

	duplicate := httptest.NewRecorder()
	duplicateRequest := httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/remix-render-jobs", bytes.NewReader(renderBody))
	duplicateRequest.Header.Set("Idempotency-Key", "render-key-1")
	server.ServeHTTP(duplicate, duplicateRequest)
	if duplicate.Code != http.StatusAccepted {
		t.Fatalf("duplicate render status=%d body=%s", duplicate.Code, duplicate.Body.String())
	}

	conflictBody, _ := json.Marshal(remix.CreateRenderJobRequest{PlanID: "remixplan_1", TargetQuality: "high"})
	conflict := httptest.NewRecorder()
	conflictRequest := httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/remix-render-jobs", bytes.NewReader(conflictBody))
	conflictRequest.Header.Set("Idempotency-Key", "render-key-1")
	server.ServeHTTP(conflict, conflictRequest)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("conflict render status=%d body=%s", conflict.Code, conflict.Body.String())
	}

	actor.Scopes = []contract.Scope{"assets.read"}
	resolver, _ = identity.NewStaticResolver(actor)
	deniedServer := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, RemixPlans: plans})
	denied := httptest.NewRecorder()
	deniedServer.ServeHTTP(denied, httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/remix-plans", bytes.NewReader(body)))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("scope denied status=%d", denied.Code)
	}
}

func TestRemixHitAnalysisMappingRoutesGeneratePlan(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}, Scopes: []contract.Scope{"assets.write", "assets.read"}}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{"hitanalysis_1", "productmapping_1", "remixplan_1"}
	service := remix.NewMemoryService(func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	})
	server := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, RemixPlans: service})

	analysisBody, _ := json.Marshal(remix.CreateHitAnalysisRequest{
		SourceAsset:     contract.AssetVersionRef{AssetID: "source_video", Version: 1},
		Title:           "爆款拆解样本",
		DurationSeconds: 30,
	})
	analysisResponse := httptest.NewRecorder()
	server.ServeHTTP(analysisResponse, httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/remix-hit-analyses", bytes.NewReader(analysisBody)))
	if analysisResponse.Code != http.StatusCreated {
		t.Fatalf("analysis status=%d body=%s", analysisResponse.Code, analysisResponse.Body.String())
	}
	if analysisResponse.Header().Get("Location") != "/platform/v1/projects/project_1/remix-hit-analyses/hitanalysis_1" {
		t.Fatalf("analysis location=%q", analysisResponse.Header().Get("Location"))
	}
	var analysis remix.HitAnalysis
	if err := json.NewDecoder(analysisResponse.Body).Decode(&analysis); err != nil {
		t.Fatal(err)
	}
	if len(analysis.Segments) != 3 || analysis.Segments[0].StartSeconds != 0 {
		t.Fatalf("analysis = %#v", analysis)
	}

	mappingBody, _ := json.Marshal(httpProductMappingRequest(analysis.ID))
	mappingResponse := httptest.NewRecorder()
	server.ServeHTTP(mappingResponse, httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/remix-product-mappings", bytes.NewReader(mappingBody)))
	if mappingResponse.Code != http.StatusCreated {
		t.Fatalf("mapping status=%d body=%s", mappingResponse.Code, mappingResponse.Body.String())
	}
	if mappingResponse.Header().Get("Location") != "/platform/v1/projects/project_1/remix-product-mappings/productmapping_1" {
		t.Fatalf("mapping location=%q", mappingResponse.Header().Get("Location"))
	}
	var mapping remix.ProductMapping
	if err := json.NewDecoder(mappingResponse.Body).Decode(&mapping); err != nil {
		t.Fatal(err)
	}
	if mapping.HitAnalysisID != analysis.ID || len(mapping.RequiredAssets) != 3 {
		t.Fatalf("mapping = %#v", mapping)
	}

	readMapping := httptest.NewRecorder()
	server.ServeHTTP(readMapping, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/remix-product-mappings/productmapping_1", nil))
	if readMapping.Code != http.StatusOK {
		t.Fatalf("read mapping status=%d body=%s", readMapping.Code, readMapping.Body.String())
	}

	planResponse := httptest.NewRecorder()
	server.ServeHTTP(planResponse, httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/remix-product-mappings/productmapping_1/plans", nil))
	if planResponse.Code != http.StatusCreated {
		t.Fatalf("plan status=%d body=%s", planResponse.Code, planResponse.Body.String())
	}
	if planResponse.Header().Get("Location") != "/platform/v1/projects/project_1/remix-plans/remixplan_1" {
		t.Fatalf("plan location=%q", planResponse.Header().Get("Location"))
	}
	var plan remix.Plan
	if err := json.NewDecoder(planResponse.Body).Decode(&plan); err != nil {
		t.Fatal(err)
	}
	if plan.SchemaVersion != remix.SchemaVersionV2 || len(plan.Segments) != 3 {
		t.Fatalf("plan = %#v", plan)
	}
	for _, segment := range plan.Segments {
		for _, shot := range segment.Shots {
			if shot.AssetVersion == analysis.SourceAsset {
				t.Fatalf("plan reused source video asset: %#v", shot)
			}
		}
	}
}

func TestRemixPrerollRoutesCreateReadAndApply(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}, Scopes: []contract.Scope{"assets.write", "assets.read"}}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	plans := &fakeRemixPlanManager{plan: remix.Plan{
		ID: "remixplan_1", OrganizationID: "org_1", ProjectID: "project_1", SchemaVersion: remix.SchemaVersionV2,
		ClientPlanID: "client_plan_1", TargetSeconds: 30, ActualSeconds: 14.4, Pace: remix.PaceBalanced,
		Segments: []remix.SegmentPlan{
			httpRemixSegment(remix.SegmentOpening, "前段", "asset_opening"),
			httpRemixSegment(remix.SegmentMiddle, "中段", "asset_middle"),
			httpRemixSegment(remix.SegmentEnding, "后段", "asset_ending"),
		},
		Summary: remix.PlanSummary{SelectedAssets: 3, UsedAssets: 3, CoveragePercent: 48, Strategy: "balanced"},
	}}
	server := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, RemixPlans: plans})
	body, _ := json.Marshal(remix.CreatePrerollRequest{
		PlanID: "remixplan_1", HookType: remix.HookTypeConflict,
		ReferenceAsset:  contract.AssetVersionRef{AssetID: "asset_opening", Version: 1},
		DurationSeconds: 4, Mode: remix.PrerollModeGenerateVideo,
		StyleConstraints: []string{"9:16 竖版"},
	})

	create := httptest.NewRecorder()
	server.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/remix-prerolls", bytes.NewReader(body)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	if create.Header().Get("Location") != "/platform/v1/projects/project_1/remix-prerolls/preroll_1" {
		t.Fatalf("location=%q", create.Header().Get("Location"))
	}
	var preroll remix.Preroll
	if err := json.NewDecoder(create.Body).Decode(&preroll); err != nil {
		t.Fatal(err)
	}
	if preroll.Status != remix.PrerollReady || preroll.OutputAsset == nil || preroll.HookType != remix.HookTypeConflict {
		t.Fatalf("created preroll=%#v", preroll)
	}

	read := httptest.NewRecorder()
	server.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/remix-prerolls/preroll_1", nil))
	if read.Code != http.StatusOK {
		t.Fatalf("read status=%d body=%s", read.Code, read.Body.String())
	}
	apply := httptest.NewRecorder()
	server.ServeHTTP(apply, httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/remix-prerolls/preroll_1/apply", nil))
	if apply.Code != http.StatusOK {
		t.Fatalf("apply status=%d body=%s", apply.Code, apply.Body.String())
	}
	var plan remix.Plan
	if err := json.NewDecoder(apply.Body).Decode(&plan); err != nil {
		t.Fatal(err)
	}
	if plan.ActualSeconds != 18.4 || !strings.Contains(strings.Join(plan.Warnings, ","), "ai_preroll_applied") {
		t.Fatalf("applied plan=%#v", plan)
	}

	plans.preroll.Status = remix.PrerollFailed
	blocked := httptest.NewRecorder()
	server.ServeHTTP(blocked, httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/remix-prerolls/preroll_1/apply", nil))
	if blocked.Code != http.StatusConflict {
		t.Fatalf("blocked status=%d body=%s", blocked.Code, blocked.Body.String())
	}
}

func TestRemixFeedbackRoutesAppendAggregateAndCreateWeights(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}, Scopes: []contract.Scope{"assets.write", "assets.read"}}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{"feedback_1", "feedback_2", "weights_1"}
	service := remix.NewMemoryService(func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	})
	server := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, RemixPlans: service})
	asset := contract.AssetVersionRef{AssetID: "asset_output_1", Version: 1}
	ratingBody, _ := json.Marshal(remix.CreateFeedbackEventRequest{EventType: remix.FeedbackEventRating, TargetType: remix.FeedbackTargetAsset, TargetID: "asset_output_1", AssetVersion: &asset, Rating: 4, Comment: "商品卖点清楚"})

	create := httptest.NewRecorder()
	server.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/remix-feedback-events", bytes.NewReader(ratingBody)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	if create.Header().Get("Location") != "/platform/v1/projects/project_1/remix-feedback-events/feedback_1" {
		t.Fatalf("location=%q", create.Header().Get("Location"))
	}
	selectedBody, _ := json.Marshal(remix.CreateFeedbackEventRequest{EventType: remix.FeedbackEventAssetSelected, TargetType: remix.FeedbackTargetRemixPlan, TargetID: "remixplan_1", AssetVersion: &asset})
	selected := httptest.NewRecorder()
	server.ServeHTTP(selected, httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/remix-feedback-events", bytes.NewReader(selectedBody)))
	if selected.Code != http.StatusCreated {
		t.Fatalf("selected status=%d body=%s", selected.Code, selected.Body.String())
	}

	list := httptest.NewRecorder()
	server.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/remix-feedback-events?target_type=asset&target_id=asset_output_1&limit=10", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	var listed struct {
		Items []remix.FeedbackEvent `json:"items"`
	}
	if err := json.NewDecoder(list.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 || listed.Items[0].Rating != 4 {
		t.Fatalf("listed=%#v", listed)
	}

	performance := httptest.NewRecorder()
	server.ServeHTTP(performance, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/remix-asset-performance", nil))
	if performance.Code != http.StatusOK {
		t.Fatalf("performance status=%d body=%s", performance.Code, performance.Body.String())
	}
	var perfBody struct {
		Items []remix.AssetPerformance `json:"items"`
	}
	if err := json.NewDecoder(performance.Body).Decode(&perfBody); err != nil {
		t.Fatal(err)
	}
	if len(perfBody.Items) != 1 || perfBody.Items[0].SelectedCount != 1 || perfBody.Items[0].AverageRating != 4 {
		t.Fatalf("performance=%#v", perfBody)
	}

	weights := httptest.NewRecorder()
	server.ServeHTTP(weights, httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/remix-planner-weight-snapshots", nil))
	if weights.Code != http.StatusCreated {
		t.Fatalf("weights status=%d body=%s", weights.Code, weights.Body.String())
	}
	if weights.Header().Get("Location") != "/platform/v1/projects/project_1/remix-planner-weight-snapshots/weights_1" {
		t.Fatalf("weights location=%q", weights.Header().Get("Location"))
	}
	var snapshot remix.PlannerWeightSnapshot
	if err := json.NewDecoder(weights.Body).Decode(&snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.AssetWeights) != 1 || snapshot.AssetWeights[0].AssetVersion.AssetID != "asset_output_1" {
		t.Fatalf("snapshot=%#v", snapshot)
	}

	actor.Scopes = []contract.Scope{"assets.read"}
	resolver, _ = identity.NewStaticResolver(actor)
	deniedServer := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, RemixPlans: service})
	denied := httptest.NewRecorder()
	deniedServer.ServeHTTP(denied, httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/remix-feedback-events", bytes.NewReader(ratingBody)))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("scope denied status=%d", denied.Code)
	}
}

func TestKnowledgeRoutesImportSearchAndReturnCitations(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}, Scopes: []contract.Scope{"assets.write", "assets.read"}}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	service := knowledge.NewMemoryService(func(prefix string) (string, error) {
		switch prefix {
		case "knowledgedoc":
			return "knowledgedoc_1", nil
		case "knowledgechunk":
			return "knowledgechunk_1", nil
		default:
			return prefix + "_1", nil
		}
	})
	server := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, Knowledge: service})
	body, _ := json.Marshal(knowledge.ImportDocumentRequest{
		Title:      "项目策略库",
		SourceURI:  "docs/策略/06-电商广告前贴与钩子视频生成策略.md",
		SourceType: "strategy",
		Text:       "电商前贴需要商品露出、强钩子，并在输出中保留 citation。",
	})

	create := httptest.NewRecorder()
	server.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/knowledge/documents", bytes.NewReader(body)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	if create.Header().Get("Location") != "/platform/v1/projects/project_1/knowledge/documents/knowledgedoc_1" {
		t.Fatalf("location=%q", create.Header().Get("Location"))
	}

	list := httptest.NewRecorder()
	server.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/knowledge/documents?limit=5", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	var listed struct {
		Items []knowledge.Document `json:"items"`
	}
	if err := json.NewDecoder(list.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 || listed.Items[0].ID != "knowledgedoc_1" {
		t.Fatalf("listed = %#v", listed)
	}

	search := httptest.NewRecorder()
	server.ServeHTTP(search, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/knowledge/search?q="+url.QueryEscape("商品 citation")+"&limit=3", nil))
	if search.Code != http.StatusOK {
		t.Fatalf("search status=%d body=%s", search.Code, search.Body.String())
	}
	var searched struct {
		Query string                   `json:"query"`
		Items []knowledge.SearchResult `json:"items"`
	}
	if err := json.NewDecoder(search.Body).Decode(&searched); err != nil {
		t.Fatal(err)
	}
	if searched.Query != "商品 citation" || len(searched.Items) != 1 || len(searched.Items[0].Citations) != 1 {
		t.Fatalf("searched = %#v", searched)
	}
	if searched.Items[0].Citations[0].SourceURI != "docs/策略/06-电商广告前贴与钩子视频生成策略.md" {
		t.Fatalf("citation = %#v", searched.Items[0].Citations[0])
	}

	actor.Scopes = []contract.Scope{"assets.read"}
	resolver, _ = identity.NewStaticResolver(actor)
	deniedServer := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, Knowledge: service})
	denied := httptest.NewRecorder()
	deniedServer.ServeHTTP(denied, httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/knowledge/documents", bytes.NewReader(body)))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("scope denied status=%d", denied.Code)
	}
}

func TestRemixEvalRoutesRunDeterministicEvaluation(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}, Scopes: []contract.Scope{"assets.write", "assets.read"}}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	evals := remix.NewMemoryService(func() (string, error) { return "unused", nil })
	server := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, Evals: evals})

	cases := httptest.NewRecorder()
	server.ServeHTTP(cases, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/remix-eval-cases", nil))
	if cases.Code != http.StatusOK {
		t.Fatalf("cases status=%d body=%s", cases.Code, cases.Body.String())
	}
	var casesBody struct {
		Items []remix.EvalCase `json:"items"`
	}
	if err := json.NewDecoder(cases.Body).Decode(&casesBody); err != nil {
		t.Fatal(err)
	}
	if len(casesBody.Items) != 2 {
		t.Fatalf("cases body=%#v", casesBody)
	}

	runBody, _ := json.Marshal(remix.CreateEvalRunRequest{
		PlannerVersion: "planner.v1",
		PromptVersion:  "prompt.v1",
		Submissions: []remix.EvalSubmission{
			{CaseID: "remix_mmlu_hook_mcq_v1", ChoiceID: "a"},
			{CaseID: "remix_mmlu_rubric_v1", AnswerText: "authorized timeline risk"},
		},
	})
	run := httptest.NewRecorder()
	server.ServeHTTP(run, httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/remix-eval-runs", bytes.NewReader(runBody)))
	if run.Code != http.StatusAccepted {
		t.Fatalf("run status=%d body=%s", run.Code, run.Body.String())
	}
	if run.Header().Get("Location") != "/platform/v1/projects/project_1/remix-eval-runs/remixevalrun_1" {
		t.Fatalf("run location=%q", run.Header().Get("Location"))
	}
	var runResult remix.EvalRun
	if err := json.NewDecoder(run.Body).Decode(&runResult); err != nil {
		t.Fatal(err)
	}
	if runResult.PassedCases != 1 || len(runResult.FailedCases) != 1 || runResult.FailedCases[0] != "remix_mmlu_hook_mcq_v1" {
		t.Fatalf("run result=%#v", runResult)
	}

	read := httptest.NewRecorder()
	server.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/remix-eval-runs/remixevalrun_1", nil))
	if read.Code != http.StatusOK {
		t.Fatalf("read status=%d body=%s", read.Code, read.Body.String())
	}

	actor.Scopes = []contract.Scope{"assets.read"}
	resolver, _ = identity.NewStaticResolver(actor)
	deniedServer := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, Evals: evals})
	denied := httptest.NewRecorder()
	deniedServer.ServeHTTP(denied, httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/remix-eval-runs", bytes.NewReader(runBody)))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("scope denied status=%d", denied.Code)
	}
}

func TestAgentRunRoutesCreateReadListAndRequireScopes(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}, Scopes: []contract.Scope{agent.ScopeRunWrite, agent.ScopeRunRead, remix.ScopePlanRead}}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	renders := &fakeRemixPlanManager{render: remix.RenderJob{
		ID: "remixrender_1", OrganizationID: "org_1", ProjectID: "project_1", PlanID: "remixplan_1",
		Status: remix.RenderFailed, TargetFormat: "mp4", TargetQuality: "draft", ErrorCode: "ENCODER_FAILED", ErrorMessage: "encoder failed",
	}}
	nextAgentID := 0
	agentRuns := agent.NewMemoryService(renders, func(prefix string) (string, error) {
		nextAgentID++
		switch prefix {
		case "agentrun":
			return "agentrun_1", nil
		case "toolcall":
			return "toolcall_1", nil
		default:
			return prefix + "_" + string(rune('0'+nextAgentID)), nil
		}
	})
	server := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, AgentRuns: agentRuns})
	body, _ := json.Marshal(agent.CreateRunRequest{Workflow: agent.WorkflowRenderDiagnosis, Target: agent.DiagnosisTarget{RenderJobID: "remixrender_1"}})

	create := httptest.NewRecorder()
	server.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/agent-runs", bytes.NewReader(body)))
	if create.Code != http.StatusAccepted {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	if create.Header().Get("Location") != "/platform/v1/projects/project_1/agent-runs/agentrun_1" {
		t.Fatalf("location=%q", create.Header().Get("Location"))
	}
	var created agent.AgentRun
	if err := json.NewDecoder(create.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Status != agent.RunSucceeded || len(created.ToolCalls) != 1 || len(created.TraceSpans) != 3 {
		t.Fatalf("created agent run = %#v", created)
	}

	read := httptest.NewRecorder()
	server.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/agent-runs/agentrun_1", nil))
	if read.Code != http.StatusOK {
		t.Fatalf("read status=%d body=%s", read.Code, read.Body.String())
	}
	list := httptest.NewRecorder()
	server.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/agent-runs?limit=5", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}

	actor.Scopes = []contract.Scope{agent.ScopeRunRead, remix.ScopePlanRead}
	resolver, _ = identity.NewStaticResolver(actor)
	deniedServer := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, AgentRuns: agentRuns})
	denied := httptest.NewRecorder()
	deniedServer.ServeHTTP(denied, httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/agent-runs", bytes.NewReader(body)))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("scope denied status=%d", denied.Code)
	}
}

func TestLocalAssetPreviewReturnsProtectedContentURL(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}, Scopes: []contract.Scope{"assets.read"}}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	uploads := &fakeUploadManager{content: []byte("png-bytes"), mime: "image/png"}
	server := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, Uploads: uploads})

	preview := httptest.NewRecorder()
	server.ServeHTTP(preview, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/assets/asset_1/versions/2/preview", nil))
	if preview.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", preview.Code, preview.Body.String())
	}
	var signed assets.SignedRequest
	if err := json.NewDecoder(preview.Body).Decode(&signed); err != nil {
		t.Fatal(err)
	}
	wantURL := "/platform/v1/projects/project_1/assets/asset_1/versions/2/content"
	if signed.URL != wantURL || signed.Method != http.MethodGet {
		t.Fatalf("signed request=%#v, want URL %q", signed, wantURL)
	}

	content := httptest.NewRecorder()
	server.ServeHTTP(content, httptest.NewRequest(http.MethodGet, signed.URL, nil))
	if content.Code != http.StatusOK || content.Body.String() != "png-bytes" {
		t.Fatalf("content status=%d body=%q", content.Code, content.Body.String())
	}
	if content.Header().Get("Content-Type") != "image/png" || content.Header().Get("Cache-Control") != "private, no-store" || content.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("unexpected content headers: %#v", content.Header())
	}

	actor.Scopes = nil
	resolver, _ = identity.NewStaticResolver(actor)
	deniedServer := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, Uploads: uploads})
	denied := httptest.NewRecorder()
	deniedServer.ServeHTTP(denied, httptest.NewRequest(http.MethodGet, wantURL, nil))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("scope denied status=%d", denied.Code)
	}
}

func TestListAssetsReturnsMediaMetadata(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}, Scopes: []contract.Scope{"assets.read"}}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	uploads := &fakeUploadManager{items: []assets.ProjectAsset{{
		Ref: contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "asset_1", Version: 1}},
		Asset: assets.Asset{
			ID: "asset_1", OrganizationID: "org_1", Kind: contract.AssetVideo, Status: assets.AssetReady,
			OwnerSystem: "assets", LatestVersion: 1, CreatedAt: now, UpdatedAt: now,
		},
		Version: assets.AssetVersion{
			OrganizationID: "org_1", AssetID: "asset_1", Version: 1, Status: assets.AssetReady,
			SourceType: contract.AssetSourceUpload, MIMEType: "video/mp4", SizeBytes: 1024, SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			Media:     assets.MediaMetadata{DurationSeconds: 9.6, FPS: 30, Codec: "h264", ProbeStatus: assets.MediaProbeSucceeded},
			CreatedAt: now,
		},
		CreatedAt: now,
	}}}
	server := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, Uploads: uploads})

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/assets", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Items []assets.ProjectAsset `json:"items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 || body.Items[0].Version.Media.DurationSeconds != 9.6 || body.Items[0].Version.Media.ProbeStatus != assets.MediaProbeSucceeded {
		t.Fatalf("media metadata missing from API response: %#v", body.Items)
	}
}

func TestAssetFeatureRoutesReadWriteAndDegradeMissingFeature(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}, Scopes: []contract.Scope{"assets.read", "assets.write"}}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	uploads := &fakeUploadManager{}
	server := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, Uploads: uploads})
	payload := `{"schema_version":"asset_feature_v1","hook_strength":0.86,"product_visibility":0.74,"scene_tags":["factory"],"product_tags":["cnc"],"person_tags":["engineer"],"action_tags":["cutting"],"emotion_tags":["trust"],"selling_points":["0.01mm precision"],"cta_presence":true,"similarity_group":"precision-demo-a","similarity_risk":"medium","evidence":["00:00-00:03 strong hook"]}`

	put := httptest.NewRecorder()
	server.ServeHTTP(put, httptest.NewRequest(http.MethodPut, "/platform/v1/projects/project_1/assets/asset_1/versions/2/features/vlm-2026-07-26", bytes.NewBufferString(payload)))
	if put.Code != http.StatusOK {
		t.Fatalf("put status=%d body=%s", put.Code, put.Body.String())
	}
	if uploads.feature.AssetID != "asset_1" || uploads.feature.AssetVersion != 2 || uploads.feature.ProjectID != "project_1" || uploads.feature.FeatureVersion != "vlm-2026-07-26" {
		t.Fatalf("feature scope not set from URL: %#v", uploads.feature)
	}

	get := httptest.NewRecorder()
	server.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/assets/asset_1/versions/2/features/vlm-2026-07-26", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", get.Code, get.Body.String())
	}
	var getBody struct {
		Feature *assets.AssetFeature `json:"feature"`
	}
	if err := json.NewDecoder(get.Body).Decode(&getBody); err != nil {
		t.Fatal(err)
	}
	if getBody.Feature == nil || getBody.Feature.HookStrength != 0.86 || getBody.Feature.SimilarityRisk != assets.AssetFeatureRiskMedium {
		t.Fatalf("unexpected feature body: %#v", getBody.Feature)
	}

	list := httptest.NewRecorder()
	server.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/assets/features?limit=10", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	var listBody struct {
		Items []assets.AssetFeature `json:"items"`
	}
	if err := json.NewDecoder(list.Body).Decode(&listBody); err != nil {
		t.Fatal(err)
	}
	if len(listBody.Items) != 1 || listBody.Items[0].SellingPoints[0] != "0.01mm precision" {
		t.Fatalf("unexpected list body: %#v", listBody)
	}

	missing := httptest.NewRecorder()
	server.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/assets/asset_2/versions/1/features/missing", nil))
	if missing.Code != http.StatusOK || !strings.Contains(missing.Body.String(), `"feature":null`) {
		t.Fatalf("missing status=%d body=%s", missing.Code, missing.Body.String())
	}

	actor.Scopes = []contract.Scope{"assets.read"}
	resolver, _ = identity.NewStaticResolver(actor)
	deniedServer := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, Uploads: &fakeUploadManager{}})
	denied := httptest.NewRecorder()
	deniedServer.ServeHTTP(denied, httptest.NewRequest(http.MethodPut, "/platform/v1/projects/project_1/assets/asset_1/versions/2/features/vlm-2026-07-26", bytes.NewBufferString(payload)))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("scope denied status=%d", denied.Code)
	}
}

type fakeUploadManager struct {
	removed contract.AssetVersionRef
	content []byte
	items   []assets.ProjectAsset
	mime    string
	feature assets.AssetFeature
}

func (*fakeUploadManager) Create(context.Context, contract.RequestContext, contract.ProjectID, contract.IdempotencyKey, assets.CreateUploadRequest) (assets.CreateUploadResponse, error) {
	return assets.CreateUploadResponse{}, nil
}
func (*fakeUploadManager) PutContent(context.Context, contract.ActorContext, contract.ProjectID, string, io.Reader, int64) error {
	return nil
}
func (*fakeUploadManager) Finalize(context.Context, contract.RequestContext, contract.ProjectID, string) (assets.UploadSession, error) {
	return assets.UploadSession{}, nil
}
func (f *fakeUploadManager) List(context.Context, contract.ActorContext, contract.ProjectID, int) ([]assets.ProjectAsset, error) {
	return f.items, nil
}
func (*fakeUploadManager) Preview(context.Context, contract.ActorContext, contract.ProjectID, contract.AssetVersionRef) (assets.SignedRequest, error) {
	return assets.SignedRequest{Method: http.MethodGet}, nil
}
func (f *fakeUploadManager) OpenPreview(context.Context, contract.ActorContext, contract.ProjectID, contract.AssetVersionRef) (io.ReadCloser, assets.ObjectInfo, error) {
	return io.NopCloser(bytes.NewReader(f.content)), assets.ObjectInfo{SizeBytes: int64(len(f.content)), MIMEType: f.mime}, nil
}
func (f *fakeUploadManager) Remove(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, ref contract.AssetVersionRef) error {
	f.removed = ref
	return nil
}
func (f *fakeUploadManager) UpsertFeature(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID, feature assets.AssetFeature) (assets.AssetFeature, error) {
	feature.OrganizationID = actor.OrganizationID
	feature.ProjectID = projectID
	feature.CreatedAt = time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	feature.UpdatedAt = feature.CreatedAt
	f.feature = feature
	return feature, nil
}
func (f *fakeUploadManager) GetFeature(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, ref contract.AssetVersionRef, featureVersion string) (assets.AssetFeature, error) {
	if f.feature.AssetID != ref.AssetID || f.feature.AssetVersion != ref.Version || f.feature.FeatureVersion != featureVersion {
		return assets.AssetFeature{}, assets.ErrNotFound
	}
	return f.feature, nil
}
func (f *fakeUploadManager) ListFeatures(context.Context, contract.ActorContext, contract.ProjectID, int) ([]assets.AssetFeature, error) {
	if f.feature.AssetID == "" {
		return nil, nil
	}
	return []assets.AssetFeature{f.feature}, nil
}

type fakeIntakeManager struct{}

func (fakeIntakeManager) Create(_ context.Context, rc contract.RequestContext, projectID contract.ProjectID, key contract.IdempotencyKey, request assets.GeneratedAssetIntakeRequest) (assets.GeneratedIntake, error) {
	return assets.GeneratedIntake{ID: "intake_1", OrganizationID: rc.Actor.OrganizationID, ProjectID: projectID, ProviderJobID: request.ProviderJobID, OutputID: request.Output.OutputID, ProviderCode: request.Output.ProviderCode, Status: assets.GeneratedIntakeQueued, IdempotencyKey: key, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}
func (fakeIntakeManager) Get(context.Context, contract.ActorContext, contract.ProjectID, string) (assets.GeneratedIntake, error) {
	return assets.GeneratedIntake{}, assets.ErrNotFound
}

type fakeRemixPlanManager struct {
	plan       remix.Plan
	render     remix.RenderJob
	quality    remix.QualityReport
	analysis   remix.HitAnalysis
	mapping    remix.ProductMapping
	preroll    remix.Preroll
	renderKey  contract.IdempotencyKey
	renderHash string
}

func (f *fakeRemixPlanManager) Create(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID, request remix.CreatePlanRequest) (remix.Plan, error) {
	f.plan = remix.Plan{
		ID:             "remixplan_1",
		OrganizationID: actor.OrganizationID,
		ProjectID:      projectID,
		CreatedBy:      actor.Principal,
		SchemaVersion:  request.SchemaVersion,
		ClientPlanID:   request.ClientPlanID,
		TargetSeconds:  request.TargetSeconds,
		ActualSeconds:  request.ActualSeconds,
		Pace:           request.Pace,
		Segments:       request.Segments,
		Warnings:       request.Warnings,
		Summary:        request.Summary,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	return f.plan, nil
}

func (f *fakeRemixPlanManager) Get(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, id string) (remix.Plan, error) {
	if id != f.plan.ID {
		return remix.Plan{}, remix.ErrNotFound
	}
	return f.plan, nil
}

func (f *fakeRemixPlanManager) List(context.Context, contract.ActorContext, contract.ProjectID, int) ([]remix.Plan, error) {
	if f.plan.ID == "" {
		return nil, nil
	}
	return []remix.Plan{f.plan}, nil
}

func (f *fakeRemixPlanManager) CreateRenderJob(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID, key contract.IdempotencyKey, request remix.CreateRenderJobRequest) (remix.RenderJob, error) {
	if request.PlanID != f.plan.ID {
		return remix.RenderJob{}, remix.ErrNotFound
	}
	hash, _ := contract.CanonicalJSONHash(request)
	if f.render.ID != "" {
		if f.renderKey == key && f.renderHash == hash {
			return f.render, nil
		}
		if f.renderKey == key {
			return remix.RenderJob{}, remix.ErrIdempotencyConflict
		}
	}
	f.renderKey = key
	f.renderHash = hash
	f.render = remix.RenderJob{
		ID:             "remixrender_1",
		OrganizationID: actor.OrganizationID,
		ProjectID:      projectID,
		PlanID:         request.PlanID,
		Status:         remix.RenderQueued,
		Progress:       0,
		TargetFormat:   "mp4",
		TargetQuality:  request.TargetQuality,
		IdempotencyKey: key,
		RequestHash:    hash,
		InputSnapshot:  remix.RenderInputSnapshot{Plan: f.plan, Request: request},
		CreatedBy:      actor.Principal,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	return f.render, nil
}

func (f *fakeRemixPlanManager) GetRenderJob(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, id string) (remix.RenderJob, error) {
	if id != f.render.ID {
		return remix.RenderJob{}, remix.ErrNotFound
	}
	return f.render, nil
}

func (f *fakeRemixPlanManager) CreateQualityReport(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID, request remix.CreateQualityReportRequest) (remix.QualityReport, error) {
	if request.RenderJobID != f.render.ID {
		return remix.QualityReport{}, remix.ErrNotFound
	}
	now := time.Now().UTC()
	f.quality = remix.QualityReport{
		ID:             "qualityreport_1",
		OrganizationID: actor.OrganizationID,
		ProjectID:      projectID,
		RenderJobID:    request.RenderJobID,
		OutputAsset:    request.OutputAsset,
		Verdict:        remix.QualityVerdictMajor,
		Score:          0.64,
		Dimensions: []remix.QualityDimension{
			{Name: "aesthetics", Score: 0.58, Verdict: string(remix.QualityVerdictMajor), Summary: "字幕遮挡主体"},
		},
		Issues: []remix.QualityIssue{{
			Code:             "LOW_READABILITY",
			Severity:         remix.QualityVerdictMajor,
			Dimension:        "aesthetics",
			StartSeconds:     5,
			EndSeconds:       6.5,
			Description:      "字幕和商品主体重叠，major 质检要求人工复核",
			RepairSuggestion: "调整字幕安全区",
		}},
		Evidence:          []remix.QualityEvidence{{Kind: "vlm_frame", TimestampSec: 5.8, Summary: "fake VLM 检出字幕遮挡主体"}},
		RepairSuggestions: []string{"调整字幕安全区"},
		CreatedBy:         actor.Principal,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	f.render.Status = remix.RenderRequiresReview
	f.render.RequiresReview = true
	f.render.QualityReportID = f.quality.ID
	return f.quality, nil
}

func (f *fakeRemixPlanManager) GetQualityReport(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, id string) (remix.QualityReport, error) {
	if id != f.quality.ID {
		return remix.QualityReport{}, remix.ErrNotFound
	}
	return f.quality, nil
}

func (f *fakeRemixPlanManager) GetQualityReportForRenderJob(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, id string) (remix.QualityReport, error) {
	if id != f.quality.RenderJobID {
		return remix.QualityReport{}, remix.ErrNotFound
	}
	return f.quality, nil
}

func (f *fakeRemixPlanManager) CreateHitAnalysis(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID, request remix.CreateHitAnalysisRequest) (remix.HitAnalysis, error) {
	f.analysis = remix.HitAnalysis{
		ID:             "hitanalysis_1",
		OrganizationID: actor.OrganizationID,
		ProjectID:      projectID,
		SourceAsset:    request.SourceAsset,
		Title:          request.Title,
		VideoMeta:      remix.HitVideoMeta{DurationSeconds: request.DurationSeconds, Language: request.Language},
		Segments:       []remix.HitSegment{{ID: "seg_1", StartSeconds: 0, EndSeconds: request.DurationSeconds, Role: remix.HitRoleHook}},
		CreatedBy:      actor.Principal,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	return f.analysis, nil
}

func (f *fakeRemixPlanManager) GetHitAnalysis(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, id string) (remix.HitAnalysis, error) {
	if id != f.analysis.ID {
		return remix.HitAnalysis{}, remix.ErrNotFound
	}
	return f.analysis, nil
}

func (f *fakeRemixPlanManager) CreateProductMapping(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID, request remix.CreateProductMappingRequest) (remix.ProductMapping, error) {
	f.mapping = remix.ProductMapping{
		ID:               "productmapping_1",
		OrganizationID:   actor.OrganizationID,
		ProjectID:        projectID,
		HitAnalysisID:    request.HitAnalysisID,
		TargetProduct:    request.TargetProduct,
		RequiredAssets:   request.RequiredAssets,
		ReplacementRules: request.ReplacementRules,
		Constraints:      request.Constraints,
		TargetSeconds:    request.TargetSeconds,
		Pace:             request.Pace,
		CreatedBy:        actor.Principal,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	return f.mapping, nil
}

func (f *fakeRemixPlanManager) GetProductMapping(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, id string) (remix.ProductMapping, error) {
	if id != f.mapping.ID {
		return remix.ProductMapping{}, remix.ErrNotFound
	}
	return f.mapping, nil
}

func (f *fakeRemixPlanManager) GeneratePlanFromProductMapping(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, id string) (remix.Plan, error) {
	if id != f.mapping.ID {
		return remix.Plan{}, remix.ErrNotFound
	}
	return f.plan, nil
}

func (f *fakeRemixPlanManager) CreatePreroll(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID, request remix.CreatePrerollRequest) (remix.Preroll, error) {
	if request.PlanID != f.plan.ID {
		return remix.Preroll{}, remix.ErrNotFound
	}
	now := time.Now().UTC()
	f.preroll = remix.Preroll{
		ID:               "preroll_1",
		OrganizationID:   actor.OrganizationID,
		ProjectID:        projectID,
		PlanID:           request.PlanID,
		HookType:         request.HookType,
		ReferenceAsset:   request.ReferenceAsset,
		StyleConstraints: request.StyleConstraints,
		DurationSeconds:  request.DurationSeconds,
		Mode:             request.Mode,
		PromptDraft:      "为 opening 段生成冲突钩子",
		QualityVerdict:   remix.QualityVerdictPass,
		Status:           remix.PrerollReady,
		OutputAsset:      &contract.ProjectAssetRef{ProjectID: projectID, AssetVersion: contract.AssetVersionRef{AssetID: "preroll_asset", Version: 1}},
		CreatedBy:        actor.Principal,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	return f.preroll, nil
}

func (f *fakeRemixPlanManager) GetPreroll(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, id string) (remix.Preroll, error) {
	if id != f.preroll.ID {
		return remix.Preroll{}, remix.ErrNotFound
	}
	return f.preroll, nil
}

func (f *fakeRemixPlanManager) ApplyPreroll(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, id string) (remix.Plan, error) {
	if id != f.preroll.ID {
		return remix.Plan{}, remix.ErrNotFound
	}
	if f.preroll.Status != remix.PrerollReady {
		return remix.Plan{}, remix.ErrPrerollNotReady
	}
	f.plan.Warnings = append(f.plan.Warnings, "ai_preroll_applied")
	f.plan.ActualSeconds += f.preroll.DurationSeconds
	f.preroll.Status = remix.PrerollApplied
	return f.plan, nil
}

func (f *fakeRemixPlanManager) CreateFeedbackEvent(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID, request remix.CreateFeedbackEventRequest) (remix.FeedbackEvent, error) {
	return remix.FeedbackEvent{ID: "feedback_1", OrganizationID: actor.OrganizationID, ProjectID: projectID, EventType: request.EventType, TargetType: request.TargetType, TargetID: request.TargetID, AssetVersion: request.AssetVersion, Rating: request.Rating, Comment: request.Comment, CreatedBy: actor.Principal, CreatedAt: time.Now().UTC()}, nil
}

func (f *fakeRemixPlanManager) ListFeedbackEvents(context.Context, contract.ActorContext, contract.ProjectID, remix.FeedbackEventFilter) ([]remix.FeedbackEvent, error) {
	return nil, nil
}

func (f *fakeRemixPlanManager) GetAssetPerformanceSnapshot(context.Context, contract.ActorContext, contract.ProjectID) ([]remix.AssetPerformance, error) {
	return nil, nil
}

func (f *fakeRemixPlanManager) CreatePlannerWeightSnapshot(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID) (remix.PlannerWeightSnapshot, error) {
	return remix.PlannerWeightSnapshot{ID: "weights_1", OrganizationID: actor.OrganizationID, ProjectID: projectID, CreatedBy: actor.Principal, CreatedAt: time.Now().UTC()}, nil
}

func httpRemixSegment(segment remix.Segment, label string, assetID contract.AssetID) remix.SegmentPlan {
	return remix.SegmentPlan{
		Segment:       segment,
		Label:         label,
		TargetSeconds: 10,
		ActualSeconds: 3.2,
		Shots: []remix.Shot{{
			ID:           string(segment) + "_shot_1",
			Segment:      segment,
			Source:       remix.ShotSourceExistingAsset,
			AssetVersion: contract.AssetVersionRef{AssetID: assetID, Version: 1},
			Timeline:     remix.ShotTimeline{StartSeconds: 0, DurationSeconds: 3.2, InPointSeconds: 0, OutPointSeconds: 3.2},
			Creative:     remix.ShotCreative{ShotType: "close_up", Transition: "cut"},
			Planning:     remix.ShotPlanning{Score: 0.8, ReasonCodes: []string{"test"}, Reason: "test", Evidence: []string{"fixture"}},
			Risks:        []string{},
		}},
	}
}

func httpProductMappingRequest(analysisID string) remix.CreateProductMappingRequest {
	return remix.CreateProductMappingRequest{
		HitAnalysisID: analysisID,
		TargetProduct: remix.ProductProfile{
			Name:          "白域精工新品",
			SellingPoints: []string{"±0.01mm 精度", "98% 准时交付"},
			CTA:           "预约获取打样方案",
		},
		RequiredAssets: []contract.AssetVersionRef{
			{AssetID: "target_hook", Version: 1},
			{AssetID: "target_proof", Version: 1},
			{AssetID: "target_cta", Version: 1},
		},
		ReplacementRules: []remix.ReplacementRule{
			{Role: remix.HitRoleHook, TargetAsset: contract.AssetVersionRef{AssetID: "target_hook", Version: 1}, Message: "先展示交期风险反差"},
			{Role: remix.HitRoleProof, TargetAsset: contract.AssetVersionRef{AssetID: "target_proof", Version: 1}, Message: "用精度和产线证据替换原证明段"},
			{Role: remix.HitRoleCTA, TargetAsset: contract.AssetVersionRef{AssetID: "target_cta", Version: 1}, Message: "引导预约打样方案"},
		},
		Constraints:   []string{"不得复用原视频二进制"},
		TargetSeconds: 30,
		Pace:          remix.PaceBalanced,
	}
}

func SchemaVersionV2ForHTTPTest() string {
	return remix.SchemaVersionV2
}

type workflowProjectManager struct {
	staticProjectManager
	projectValue project.Project
	runtime      project.ProjectRuntime
	artifact     project.ProjectArtifact
	task         project.BusinessTask
	operations   []project.OperationalRecord
	changeSet    project.ChangeSet
	auditEvents  []project.AuditEvent
}

func (m *workflowProjectManager) GetDetail(context.Context, contract.ActorContext, contract.ProjectID) (project.ProjectDetail, error) {
	runtime := m.runtime
	if runtime.Code == "" {
		runtime = project.ProjectRuntime{
			Code:      string(m.projectValue.ID),
			Stage:     string(m.projectValue.Status),
			Progress:  60,
			Status:    "active",
			Owner:     "user:usr_1",
			Budget:    0,
			Currency:  "CNY",
			Timezone:  "Asia/Shanghai",
			UpdatedAt: m.projectValue.UpdatedAt,
		}
	}
	return project.ProjectDetail{
		Project:    m.projectValue,
		Runtime:    runtime,
		Artifacts:  []project.ProjectArtifactSummary{},
		Tasks:      m.tasks(),
		Operations: m.operations,
		ChangeSets: m.changeSets(),
	}, nil
}

func (m *workflowProjectManager) UpdateProject(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, request project.UpdateProjectRequest) (project.Project, error) {
	if request.ExpectedContextVersion != nil && *request.ExpectedContextVersion != m.projectValue.ProjectContextVersion {
		return project.Project{}, project.ErrVersionConflict
	}
	if request.Name != nil {
		m.projectValue.Name = *request.Name
	}
	if request.Industry != nil {
		m.projectValue.Industry = *request.Industry
	}
	if request.Brand != nil {
		m.runtime.Brand = *request.Brand
	}
	if request.Goal != nil {
		m.runtime.Goal = *request.Goal
	}
	m.projectValue.ProjectContextVersion++
	return m.projectValue, nil
}

func (m *workflowProjectManager) CreateProjectArtifact(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID, request project.CreateProjectArtifactRequest) (project.ProjectArtifact, error) {
	now := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	m.artifact = project.ProjectArtifact{
		ID: "artifact_1", OrganizationID: actor.OrganizationID, ProjectID: projectID,
		Kind: request.Kind, Status: request.Status, Content: request.Content, SourceJobID: request.SourceJobID,
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	return m.artifact, nil
}

func (m *workflowProjectManager) ListProjectArtifacts(context.Context, contract.ActorContext, contract.ProjectID) ([]project.ProjectArtifact, error) {
	if m.artifact.ID == "" {
		return []project.ProjectArtifact{}, nil
	}
	return []project.ProjectArtifact{m.artifact}, nil
}

func (m *workflowProjectManager) GetProjectArtifact(context.Context, contract.ActorContext, contract.ProjectID, string) (project.ProjectArtifact, error) {
	if m.artifact.ID == "" {
		return project.ProjectArtifact{}, project.ErrNotFound
	}
	return m.artifact, nil
}

func (m *workflowProjectManager) UpdateProjectArtifact(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, _ string, request project.UpdateProjectArtifactRequest) (project.ProjectArtifact, error) {
	if m.artifact.ID == "" {
		return project.ProjectArtifact{}, project.ErrNotFound
	}
	if request.ExpectedVersion != nil && *request.ExpectedVersion != m.artifact.Version {
		return project.ProjectArtifact{}, project.ErrVersionConflict
	}
	if request.Content != nil {
		m.artifact.Content = *request.Content
	}
	if request.Status != nil {
		m.artifact.Status = *request.Status
	}
	if request.SourceJobID != nil {
		m.artifact.SourceJobID = *request.SourceJobID
	}
	m.artifact.Version++
	return m.artifact, nil
}

func (m *workflowProjectManager) GetWorkbench(_ context.Context, _ contract.ActorContext, projectID contract.ProjectID) (project.Workbench, error) {
	return project.Workbench{Project: project.WorkbenchProject{ProjectID: string(projectID)}}, nil
}

func (m *workflowProjectManager) CreateBusinessTask(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID, request project.CreateBusinessTaskRequest) (project.BusinessTask, error) {
	now := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	m.task = project.BusinessTask{
		ID:                "task_1",
		OrganizationID:    actor.OrganizationID,
		ProjectID:         projectID,
		Type:              request.Type,
		Name:              request.Name,
		Objective:         request.Objective,
		Status:            project.BusinessTaskDraft,
		SourceTaskIDs:     request.SourceTaskIDs,
		SourceArtifactIDs: request.SourceArtifactIDs,
		OutputArtifactIDs: []string{},
		Version:           1,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	return m.task, nil
}

func (m *workflowProjectManager) ListBusinessTasks(context.Context, contract.ActorContext, contract.ProjectID) ([]project.BusinessTask, error) {
	return m.tasks(), nil
}

func (m *workflowProjectManager) GetBusinessTask(context.Context, contract.ActorContext, contract.ProjectID, string) (project.BusinessTask, error) {
	if m.task.ID == "" {
		return project.BusinessTask{}, project.ErrNotFound
	}
	return m.task, nil
}

func (m *workflowProjectManager) UpdateBusinessTask(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, _ string, request project.UpdateBusinessTaskRequest) (project.BusinessTask, error) {
	if request.Status != nil {
		m.task.Status = *request.Status
	}
	if request.OutputArtifactIDs != nil {
		m.task.OutputArtifactIDs = request.OutputArtifactIDs
	}
	m.task.Version = 2
	return m.task, nil
}

func (m *workflowProjectManager) CreateOperationalRecord(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID, request project.UpsertOperationalRecordRequest) (project.OperationalRecord, error) {
	record := workflowOperation(actor.OrganizationID, projectID, "operation_1", request)
	m.operations = append(m.operations, record)
	return record, nil
}

func (m *workflowProjectManager) ListOperationalRecords(context.Context, contract.ActorContext, contract.ProjectID) ([]project.OperationalRecord, error) {
	return m.operations, nil
}

func (m *workflowProjectManager) GetOperationalRecord(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, id string) (project.OperationalRecord, error) {
	for _, record := range m.operations {
		if record.ID == id {
			return record, nil
		}
	}
	return project.OperationalRecord{}, project.ErrNotFound
}

func (m *workflowProjectManager) UpsertOperationalRecord(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string, request project.UpsertOperationalRecordRequest) (project.OperationalRecord, error) {
	record := workflowOperation(actor.OrganizationID, projectID, id, request)
	m.operations = append(m.operations, record)
	return record, nil
}

func (m *workflowProjectManager) CreateChangeSet(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID, request project.CreateChangeSetRequest) (project.ChangeSet, error) {
	now := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	m.changeSet = project.ChangeSet{
		ID:             "changeset_1",
		OrganizationID: actor.OrganizationID,
		ProjectID:      projectID,
		Name:           request.Name,
		Status:         project.ChangeSetDraft,
		ArtifactRefs:   request.ArtifactRefs,
		BudgetLimit:    request.BudgetLimit,
		AuditEvents:    []project.AuditEvent{},
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	m.appendAudit("change_set.created")
	m.changeSet.AuditEvents = m.auditEvents
	return m.changeSet, nil
}

func (m *workflowProjectManager) ListChangeSets(context.Context, contract.ActorContext, contract.ProjectID) ([]project.ChangeSet, error) {
	return m.changeSets(), nil
}

func (m *workflowProjectManager) GetChangeSet(context.Context, contract.ActorContext, contract.ProjectID, string) (project.ChangeSet, error) {
	if m.changeSet.ID == "" {
		return project.ChangeSet{}, project.ErrNotFound
	}
	m.changeSet.AuditEvents = m.auditEvents
	return m.changeSet, nil
}

func (m *workflowProjectManager) PreflightChangeSet(context.Context, contract.ActorContext, contract.ProjectID, string) (project.ChangeSet, error) {
	m.changeSet.Status = project.ChangeSetPreflightPassed
	m.changeSet.Preflight = &project.ChangeSetPreflight{Passed: true, Checks: []project.PreflightCheck{{Code: "ready_creative", Passed: true, Message: "ready", Repair: ""}}, CheckedAt: time.Date(2026, 7, 28, 10, 1, 0, 0, time.UTC)}
	m.appendAudit("change_set.preflight")
	m.changeSet.AuditEvents = m.auditEvents
	return m.changeSet, nil
}

func (m *workflowProjectManager) ApproveChangeSet(context.Context, contract.ActorContext, contract.ProjectID, string, project.ChangeSetApprovalRequest) (project.ChangeSet, error) {
	m.changeSet.Status = project.ChangeSetApproved
	m.appendAudit("change_set.approved")
	m.changeSet.AuditEvents = m.auditEvents
	return m.changeSet, nil
}

func (m *workflowProjectManager) ExecuteChangeSet(context.Context, contract.ActorContext, contract.ProjectID, string) (project.ChangeSet, error) {
	now := time.Date(2026, 7, 28, 10, 2, 0, 0, time.UTC)
	m.changeSet.Status = project.ChangeSetExecuted
	m.changeSet.Execution = &project.ChangeSetExecution{Simulated: true, Evidence: []project.ChangeSetEvidence{{Step: "simulate", Status: "ok", Message: "done", RecordedAt: now}}, ExecutedAt: now}
	m.appendAudit("change_set.executed")
	m.changeSet.AuditEvents = m.auditEvents
	return m.changeSet, nil
}

func (m *workflowProjectManager) RollbackChangeSet(context.Context, contract.ActorContext, contract.ProjectID, string, project.RollbackChangeSetRequest) (project.ChangeSet, error) {
	m.changeSet.Status = project.ChangeSetRolledBack
	m.changeSet.Rollback = &project.ChangeSetRollback{Simulated: true, Reason: "演示回滚", RolledBackAt: time.Date(2026, 7, 28, 10, 3, 0, 0, time.UTC)}
	m.appendAudit("change_set.rolled_back")
	m.changeSet.AuditEvents = m.auditEvents
	return m.changeSet, nil
}

func (m *workflowProjectManager) ListAuditEvents(context.Context, contract.ActorContext, contract.ProjectID) ([]project.AuditEvent, error) {
	return m.auditEvents, nil
}

func (m *workflowProjectManager) tasks() []project.BusinessTask {
	if m.task.ID == "" {
		return []project.BusinessTask{}
	}
	return []project.BusinessTask{m.task}
}

func (m *workflowProjectManager) changeSets() []project.ChangeSet {
	if m.changeSet.ID == "" {
		return []project.ChangeSet{}
	}
	m.changeSet.AuditEvents = m.auditEvents
	return []project.ChangeSet{m.changeSet}
}

func (m *workflowProjectManager) appendAudit(action string) {
	m.auditEvents = append(m.auditEvents, project.AuditEvent{
		ID:             fmt.Sprintf("audit_%d", len(m.auditEvents)+1),
		OrganizationID: "org_1",
		ProjectID:      "project_1",
		Actor:          "user:usr_1",
		Action:         action,
		EntityType:     project.AuditEntityChangeSet,
		EntityID:       "changeset_1",
		Metadata:       map[string]any{"source": "handler-test"},
		CreatedAt:      time.Date(2026, 7, 28, 10, len(m.auditEvents), 0, 0, time.UTC),
	})
}

func workflowOperation(organizationID contract.OrganizationID, projectID contract.ProjectID, id string, request project.UpsertOperationalRecordRequest) project.OperationalRecord {
	now := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	return project.OperationalRecord{
		ID:             id,
		OrganizationID: organizationID,
		ProjectID:      projectID,
		Kind:           request.Kind,
		Title:          request.Title,
		Status:         request.Status,
		OccurredAt:     request.OccurredAt,
		Fields:         request.Fields,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func TestContextFailsClosedWithoutTrustedIdentity(t *testing.T) {
	t.Parallel()
	server := New(identity.RejectingResolver{})
	response := httptest.NewRecorder()

	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/platform/v1/context", nil))

	if got, want := response.Code, http.StatusUnauthorized; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	var body contract.Problem
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if body.Error.Code != "UNAUTHENTICATED" || body.Error.RequestID == "" {
		t.Fatalf("unexpected problem: %#v", body)
	}
	if body.Error.Details == nil {
		t.Fatal("problem details must serialize as an empty array")
	}
}

func TestProjectProbeUsesSharedAuthenticationAndAuthorization(t *testing.T) {
	t.Parallel()
	resolver, err := identity.NewStaticResolver(contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}})
	if err != nil {
		t.Fatal(err)
	}
	server := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_1/context", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}

	denied := httptest.NewRecorder()
	server.ServeHTTP(denied, httptest.NewRequest(http.MethodGet, "/platform/v1/projects/project_2/context", nil))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("denied status = %d", denied.Code)
	}
}

func TestContextReturnsTrustedTenantAndTrace(t *testing.T) {
	t.Parallel()
	resolver, err := identity.NewStaticResolver(contract.ActorContext{
		OrganizationID: "org_1",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"},
		Scopes:         []contract.Scope{"strategy.brief.read"},
	})
	if err != nil {
		t.Fatalf("NewStaticResolver() error = %v", err)
	}
	server := New(resolver)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/platform/v1/context", nil)
	request.Header.Set("X-Request-ID", "req_from_client")
	request.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00")

	server.ServeHTTP(response, request)

	if got, want := response.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	var body contract.RequestContext
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode context: %v", err)
	}
	if body.RequestID != "req_from_client" || body.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" || body.Actor.OrganizationID != "org_1" {
		t.Fatalf("unexpected context: %#v", body)
	}
}

func TestInvalidClientRequestIDIsNotReflected(t *testing.T) {
	t.Parallel()
	server := New(identity.RejectingResolver{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("X-Request-ID", "bad\r\nvalue")

	server.ServeHTTP(response, request)

	if got := response.Header().Get("X-Request-ID"); got == "bad\r\nvalue" || got == "" {
		t.Fatalf("unexpected request ID response header: %q", got)
	}
}

func TestCreateImageJobUsesTrustedActorAndResolvedProjectContext(t *testing.T) {
	t.Parallel()
	resolver, err := identity.NewStaticResolver(contract.ActorContext{
		OrganizationID: "org_1",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"},
		Scopes:         []contract.Scope{provider.ScopeJobCreate},
	})
	if err != nil {
		t.Fatal(err)
	}
	brandID := contract.BrandID("brand_1")
	jobs := &providerJobStub{job: providerJobForHTTPTest()}
	server := NewWithDependencies(Dependencies{
		Resolver:          resolver,
		ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"},
		Projects: staticProjectManager{context: contract.ProjectContext{
			OrganizationID: "org_1", ProjectID: "project_1", BrandID: &brandID, ProductIDs: []contract.ProductID{}, ProjectContextVersion: 7,
		}},
		ProviderJobs: jobs,
	})
	request := httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/model/jobs", bytes.NewBufferString(`{
		"capability":"image.generate",
		"model_alias":"cookies.image.standard",
		"input":{"prompt":"launch poster","width":1024,"height":1024},
		"project_context_version":7
	}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "create-image-1")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	if jobs.request.Actor.OrganizationID != "org_1" || jobs.request.Project.ProjectContextVersion != 7 || jobs.request.Input.Prompt != "launch poster" {
		t.Fatalf("unexpected Provider request: %+v", jobs.request)
	}
	if jobs.request.RequestHash == "" || jobs.request.IdempotencyKey != "create-image-1" {
		t.Fatalf("request hash or idempotency key missing: %+v", jobs.request)
	}
}

func TestCreateImageJobRejectsStaleProjectContext(t *testing.T) {
	t.Parallel()
	resolver, err := identity.NewStaticResolver(contract.ActorContext{
		OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}, Scopes: []contract.Scope{provider.ScopeJobCreate},
	})
	if err != nil {
		t.Fatal(err)
	}
	brandID := contract.BrandID("brand_1")
	jobs := &providerJobStub{job: providerJobForHTTPTest()}
	server := NewWithDependencies(Dependencies{
		Resolver:          resolver,
		ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"},
		Projects: staticProjectManager{context: contract.ProjectContext{
			OrganizationID: "org_1", ProjectID: "project_1", BrandID: &brandID, ProductIDs: []contract.ProductID{}, ProjectContextVersion: 7,
		}},
		ProviderJobs: jobs,
	})
	request := httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/model/jobs", bytes.NewBufferString(`{"capability":"image.generate","model_alias":"cookies.image.standard","input":{"prompt":"launch poster","width":1024,"height":1024},"project_context_version":6}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "create-image-1")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusConflict || jobs.createCalls != 0 {
		t.Fatalf("status = %d create_calls=%d body=%s", response.Code, jobs.createCalls, response.Body.String())
	}
}

type staticProjectManager struct{ context contract.ProjectContext }

func (s staticProjectManager) GetContext(context.Context, contract.ActorContext, contract.ProjectID) (contract.ProjectContext, error) {
	return s.context, nil
}

func (staticProjectManager) CreateBrand(context.Context, contract.ActorContext, string) (project.Brand, error) {
	return project.Brand{}, nil
}

func (staticProjectManager) CreateProject(context.Context, contract.ActorContext, project.CreateProjectRequest) (project.Project, error) {
	return project.Project{}, nil
}
func (staticProjectManager) UpdateProject(context.Context, contract.ActorContext, contract.ProjectID, project.UpdateProjectRequest) (project.Project, error) {
	return project.Project{}, nil
}

func (staticProjectManager) ListProjects(context.Context, contract.ActorContext) ([]project.Project, error) {
	return nil, nil
}

func (staticProjectManager) GetDetail(context.Context, contract.ActorContext, contract.ProjectID) (project.ProjectDetail, error) {
	return project.ProjectDetail{}, nil
}
func (staticProjectManager) CreateProjectArtifact(context.Context, contract.ActorContext, contract.ProjectID, project.CreateProjectArtifactRequest) (project.ProjectArtifact, error) {
	return project.ProjectArtifact{}, nil
}
func (staticProjectManager) ListProjectArtifacts(context.Context, contract.ActorContext, contract.ProjectID) ([]project.ProjectArtifact, error) {
	return nil, nil
}
func (staticProjectManager) GetProjectArtifact(context.Context, contract.ActorContext, contract.ProjectID, string) (project.ProjectArtifact, error) {
	return project.ProjectArtifact{}, nil
}
func (staticProjectManager) UpdateProjectArtifact(context.Context, contract.ActorContext, contract.ProjectID, string, project.UpdateProjectArtifactRequest) (project.ProjectArtifact, error) {
	return project.ProjectArtifact{}, nil
}
func (staticProjectManager) GetWorkbench(context.Context, contract.ActorContext, contract.ProjectID) (project.Workbench, error) {
	return project.Workbench{}, nil
}

func (staticProjectManager) CreateBusinessTask(context.Context, contract.ActorContext, contract.ProjectID, project.CreateBusinessTaskRequest) (project.BusinessTask, error) {
	return project.BusinessTask{}, nil
}

func (staticProjectManager) ListBusinessTasks(context.Context, contract.ActorContext, contract.ProjectID) ([]project.BusinessTask, error) {
	return nil, nil
}

func (staticProjectManager) GetBusinessTask(context.Context, contract.ActorContext, contract.ProjectID, string) (project.BusinessTask, error) {
	return project.BusinessTask{}, nil
}

func (staticProjectManager) UpdateBusinessTask(context.Context, contract.ActorContext, contract.ProjectID, string, project.UpdateBusinessTaskRequest) (project.BusinessTask, error) {
	return project.BusinessTask{}, nil
}

func (staticProjectManager) CreateOperationalRecord(context.Context, contract.ActorContext, contract.ProjectID, project.UpsertOperationalRecordRequest) (project.OperationalRecord, error) {
	return project.OperationalRecord{}, nil
}

func (staticProjectManager) ListOperationalRecords(context.Context, contract.ActorContext, contract.ProjectID) ([]project.OperationalRecord, error) {
	return nil, nil
}

func (staticProjectManager) GetOperationalRecord(context.Context, contract.ActorContext, contract.ProjectID, string) (project.OperationalRecord, error) {
	return project.OperationalRecord{}, nil
}

func (staticProjectManager) UpsertOperationalRecord(context.Context, contract.ActorContext, contract.ProjectID, string, project.UpsertOperationalRecordRequest) (project.OperationalRecord, error) {
	return project.OperationalRecord{}, nil
}

func (staticProjectManager) CreateChangeSet(context.Context, contract.ActorContext, contract.ProjectID, project.CreateChangeSetRequest) (project.ChangeSet, error) {
	return project.ChangeSet{}, nil
}

func (staticProjectManager) ListChangeSets(context.Context, contract.ActorContext, contract.ProjectID) ([]project.ChangeSet, error) {
	return nil, nil
}

func (staticProjectManager) GetChangeSet(context.Context, contract.ActorContext, contract.ProjectID, string) (project.ChangeSet, error) {
	return project.ChangeSet{}, nil
}

func (staticProjectManager) PreflightChangeSet(context.Context, contract.ActorContext, contract.ProjectID, string) (project.ChangeSet, error) {
	return project.ChangeSet{}, nil
}

func (staticProjectManager) ApproveChangeSet(context.Context, contract.ActorContext, contract.ProjectID, string, project.ChangeSetApprovalRequest) (project.ChangeSet, error) {
	return project.ChangeSet{}, nil
}

func (staticProjectManager) ExecuteChangeSet(context.Context, contract.ActorContext, contract.ProjectID, string) (project.ChangeSet, error) {
	return project.ChangeSet{}, nil
}

func (staticProjectManager) RollbackChangeSet(context.Context, contract.ActorContext, contract.ProjectID, string, project.RollbackChangeSetRequest) (project.ChangeSet, error) {
	return project.ChangeSet{}, nil
}

func (staticProjectManager) ListAuditEvents(context.Context, contract.ActorContext, contract.ProjectID) ([]project.AuditEvent, error) {
	return nil, nil
}

type providerJobStub struct {
	job         contract.ProviderJob
	request     provider.CreateImageJobRequest
	createCalls int
}

func (s *providerJobStub) CreateImageJob(_ context.Context, request provider.CreateImageJobRequest) (contract.ProviderJob, bool, error) {
	s.createCalls++
	s.request = request
	return s.job, false, nil
}

func (s *providerJobStub) GetJob(context.Context, contract.OrganizationID, contract.ProjectID, string) (contract.ProviderJob, error) {
	return s.job, nil
}

func providerJobForHTTPTest() contract.ProviderJob {
	now := time.Date(2026, time.July, 22, 5, 0, 0, 0, time.UTC)
	return contract.ProviderJob{
		ID: "provider_job_1", Kind: "provider.image.generate", OrganizationID: "org_1", ProjectID: "project_1",
		ExecutionStatus: contract.JobQueued, ProviderStatus: contract.ProviderJobSubmitted, ProjectAssetRefs: []contract.ProjectAssetRef{},
		MaxAttempts: 3, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
}
