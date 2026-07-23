package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/platform/project"
	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/systems/creative"
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

func TestAuthenticatedDomainMountReceivesTrustedRequestContext(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{
		OrganizationID: "org_1",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
		Scopes:         []contract.Scope{"strategy.read"},
	}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	mount := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestContext, ok := contract.RequestContextFrom(request.Context())
		if !ok || requestContext.Actor.OrganizationID != actor.OrganizationID || requestContext.RequestID == "" {
			t.Fatal("domain mount did not receive trusted request context")
		}
		writer.WriteHeader(http.StatusNoContent)
	})
	server := NewWithDependencies(Dependencies{
		Resolver: resolver,
		AuthenticatedDomainMounts: []DomainMount{{
			Pattern: "/api/strategy/v1/",
			Handler: mount,
		}},
	})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/strategy/v1/workspaces/workspace_1", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}

	denied := NewWithDependencies(Dependencies{
		Resolver: identity.RejectingResolver{},
		AuthenticatedDomainMounts: []DomainMount{{
			Pattern: "/api/strategy/v1/",
			Handler: mount,
		}},
	})
	unauthenticated := httptest.NewRecorder()
	denied.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/api/strategy/v1/workspaces/workspace_1", nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d", unauthenticated.Code)
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

type fakeUploadManager struct {
	removed contract.AssetVersionRef
	content []byte
	mime    string
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
func (*fakeUploadManager) List(context.Context, contract.ActorContext, contract.ProjectID, int) ([]assets.ProjectAsset, error) {
	return nil, nil
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

type fakeIntakeManager struct{}

func (fakeIntakeManager) Create(_ context.Context, rc contract.RequestContext, projectID contract.ProjectID, key contract.IdempotencyKey, request assets.GeneratedAssetIntakeRequest) (assets.GeneratedIntake, error) {
	return assets.GeneratedIntake{ID: "intake_1", OrganizationID: rc.Actor.OrganizationID, ProjectID: projectID, ProviderJobID: request.ProviderJobID, OutputID: request.Output.OutputID, ProviderCode: request.Output.ProviderCode, Status: assets.GeneratedIntakeQueued, IdempotencyKey: key, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}
func (fakeIntakeManager) Get(context.Context, contract.ActorContext, contract.ProjectID, string) (assets.GeneratedIntake, error) {
	return assets.GeneratedIntake{}, assets.ErrNotFound
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

func TestCreativeCoverJobKeepsCreativeTaskLineage(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}, Scopes: []contract.Scope{creative.ScopeRead, creative.ScopeWrite, provider.ScopeJobCreate}}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	brandID := contract.BrandID("brand_1")
	jobs := &providerJobStub{job: providerJobForHTTPTest()}
	creativeManager := &creativeManagerStub{detail: creative.TaskDetail{
		Task:  creative.CreativeTask{ID: "creative_task_1", OrganizationID: "org_1", ProjectID: "project_1", Direction: creative.CreativeDirection{Tone: []string{"克制"}}},
		Draft: creative.ImageTextDraft{CoverCopy: "从容开始", ImagePlan: []creative.ImagePlanItem{{Order: 1, VisualBrief: "晨光中的咖啡桌"}}},
	}}
	server := NewWithDependencies(Dependencies{
		Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, Creative: creativeManager, ProviderJobs: jobs,
		Projects: staticProjectManager{context: contract.ProjectContext{OrganizationID: "org_1", ProjectID: "project_1", BrandID: &brandID, ProductIDs: []contract.ProductID{}, ProjectContextVersion: 7}},
	})
	request := httptest.NewRequest(http.MethodPost, "/api/creative/v1/projects/project_1/creative-tasks/creative_task_1:cover-image-job", bytes.NewBufferString(`{}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "creative-cover-1")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if jobs.request.SourceSystem != "creative" || jobs.request.SourceTaskID != "creative_task_1" {
		t.Fatalf("Provider source=%q task=%q", jobs.request.SourceSystem, jobs.request.SourceTaskID)
	}
	if creativeManager.registeredProviderJobID != "provider_job_1" {
		t.Fatalf("registered provider job=%q", creativeManager.registeredProviderJobID)
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

func (staticProjectManager) ListProjects(context.Context, contract.ActorContext) ([]project.Project, error) {
	return nil, nil
}

type providerJobStub struct {
	job         contract.ProviderJob
	request     provider.CreateImageJobRequest
	createCalls int
}

type creativeManagerStub struct {
	detail                  creative.TaskDetail
	registeredProviderJobID string
}

func (s *creativeManagerStub) CreateIntake(context.Context, contract.RequestContext, contract.ProjectID, contract.IdempotencyKey, creative.CreateIntakeRequest) (creative.CreativeIntake, error) {
	return creative.CreativeIntake{}, nil
}
func (s *creativeManagerStub) ListIntakes(context.Context, contract.ActorContext, contract.ProjectID, int) ([]creative.CreativeIntake, error) {
	return nil, nil
}
func (s *creativeManagerStub) CreateTask(context.Context, contract.ActorContext, contract.ProjectID, string) (creative.CreativeTask, error) {
	return creative.CreativeTask{}, nil
}
func (s *creativeManagerStub) ListTasks(context.Context, contract.ActorContext, contract.ProjectID, int) ([]creative.CreativeTask, error) {
	return nil, nil
}
func (s *creativeManagerStub) GetTaskDetail(context.Context, contract.ActorContext, contract.ProjectID, string) (creative.TaskDetail, error) {
	return s.detail, nil
}
func (s *creativeManagerStub) RegisterCoverImageJob(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, _ string, providerJobID string) error {
	s.registeredProviderJobID = providerJobID
	return nil
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
