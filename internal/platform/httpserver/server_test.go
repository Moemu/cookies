package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestCreativeDomainErrorsAreMappedToActionableHTTPProblems(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "invalid state", err: creative.ErrInvalidState, wantStatus: http.StatusConflict, wantCode: "INVALID_STATE"},
		{name: "stale version", err: creative.ErrVersionConflict, wantStatus: http.StatusPreconditionFailed, wantCode: "CREATIVE_VERSION_CONFLICT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/creative/v1/test", nil)

			(&Server{}).writeServiceError(response, request, tt.err)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
			var problem struct {
				Error contract.Error `json:"error"`
			}
			if err := json.NewDecoder(response.Body).Decode(&problem); err != nil {
				t.Fatalf("decode problem: %v", err)
			}
			if problem.Error.Code != tt.wantCode {
				t.Fatalf("code = %q, want %q", problem.Error.Code, tt.wantCode)
			}
			if problem.Error.Retryable {
				t.Fatal("creative domain conflict must not be retryable without refreshing state")
			}
		})
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

func TestCreateVideoJobUsesProviderVideoSeam(t *testing.T) {
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
	jobs.job.Kind = "provider.video.generate"
	server := NewWithDependencies(Dependencies{
		Resolver:          resolver,
		ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"},
		Projects: staticProjectManager{context: contract.ProjectContext{
			OrganizationID: "org_1", ProjectID: "project_1", BrandID: &brandID, ProductIDs: []contract.ProductID{}, ProjectContextVersion: 7,
		}},
		ProviderJobs: jobs,
	})
	request := httptest.NewRequest(http.MethodPost, "/platform/v1/projects/project_1/model/jobs", bytes.NewBufferString(`{
		"capability":"video.generate",
		"model_alias":"cookies.video.standard",
		"input":{"prompt":"five-second product pre-roll","duration_seconds":5,"aspect_ratio":"9:16","resolution":"720p"},
		"project_context_version":7,
		"source_system":"creative",
		"source_task_id":"creative_task_1"
	}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "create-video-1")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	if jobs.videoRequest.Input.DurationSeconds != 5 || jobs.videoRequest.Input.AspectRatio != "9:16" || jobs.videoRequest.SourceTaskID != "creative_task_1" {
		t.Fatalf("unexpected Provider video request: %+v", jobs.videoRequest)
	}
}

func TestGetViralRemakeWorkspaceRestoresPersistedDraft(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{
		OrganizationID: "org_1",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"},
		Scopes:         []contract.Scope{creative.ScopeRead},
	}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.July, 28, 10, 0, 0, 0, time.UTC)
	manager := &creativeManagerStub{detail: creative.TaskDetail{
		Task: creative.CreativeTask{
			ID: "creative_task_viral", OrganizationID: "org_1", ProjectID: "project_1",
			Format: creative.FormatVideo, PerformanceMode: creative.PerformanceModeViralRemake,
		},
		VideoDraft: &creative.VideoDraft{
			ContractVersion: "creative-video-draft/v1", TaskID: "creative_task_viral", Revision: 1,
			Concept: "viral", Prompt: "pending analysis", DurationSeconds: 15, AspectRatio: "9:16",
			Resolution: "720p", SourceVideo: contract.AssetVersionRef{AssetID: "asset_video", Version: 1},
			Mandatory: []string{}, Prohibited: []string{}, CreatedAt: now,
			ViralRemake: &creative.ViralRemakeDraft{
				ContractVersion: "creative-viral-remake-draft/v1", TaskID: "creative_task_viral", Revision: 1,
				Status: "waiting_for_analysis", SelectedRouteID: creative.ManualViralRemakeRouteID,
				InputSnapshot: creative.ViralRemakeInputSnapshot{
					Source: creative.IntakeSourceManual, SelectedRouteID: creative.ManualViralRemakeRouteID,
					ReferenceVideo: contract.AssetVersionRef{AssetID: "asset_video", Version: 1},
				},
				InputHash: "sha256:test", Readiness: creative.CreativeReadiness{
					PlanningReady: true, MissingFields: []string{}, Blockers: []string{"analysis_snapshot"},
				},
				CreatedAt: now, UpdatedAt: now,
			},
		},
	}}
	server := NewWithDependencies(Dependencies{
		Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"},
		Creative: manager,
	})
	request := httptest.NewRequest(http.MethodGet, "/api/creative/v1/projects/project_1/creative-tasks/creative_task_viral/viral-remake", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"selected_route_id":"route_manual_viral_remake_v1"`) {
		t.Fatalf("workspace body = %s", response.Body.String())
	}
}

func TestCreativeVideoJobRequiresAndMapsApprovedFirstLastFrameSpec(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{
		OrganizationID: "org_1",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"},
		Scopes:         []contract.Scope{creative.ScopeRead, creative.ScopeWrite, provider.ScopeJobCreate},
	}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := (creative.CommercePrerollPlanner{}).Plan(creative.CommercePrerollPlanningInput{
		TaskID: "creative_task_1", IntakeVersion: 1,
		TemplateID: creative.CommerceWindowRevealTemplateID, TemplateVersion: 1,
		BrandName: "Guerlain", ProductName: "Abeille Royale",
		ProductAsset:    contract.AssetVersionRef{AssetID: "asset_product", Version: 1},
		DurationSeconds: 6, AspectRatio: "9:16", Resolution: "720p",
		AudioPolicy: creative.VideoAudioSilent,
	})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	spec, err := plan.BindFrames(creative.ConditionedFrames{
		StartFrame: contract.AssetVersionRef{AssetID: "asset_first", Version: 1},
		TailFrame:  contract.AssetVersionRef{AssetID: "asset_last", Version: 1},
	})
	if err != nil {
		t.Fatalf("BindFrames() error = %v", err)
	}
	approval, err := creative.ApproveVideoGeneration(spec, actor.Principal.ID, time.Date(2026, time.July, 28, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ApproveVideoGeneration() error = %v", err)
	}
	body, err := json.Marshal(creative.CreateVideoJobRequest{
		ModelAlias:     "cookies.video.standard",
		Prompt:         &plan.Prompt,
		GenerationSpec: &spec,
		Approval:       &approval,
	})
	if err != nil {
		t.Fatal(err)
	}
	brandID := contract.BrandID("brand_1")
	jobs := &providerJobStub{job: providerJobForHTTPTest()}
	jobs.job.Kind = "provider.video.generate"
	creativeManager := &creativeManagerStub{detail: creative.TaskDetail{
		Task: creative.CreativeTask{ID: "creative_task_1", OrganizationID: "org_1", ProjectID: "project_1", Format: creative.FormatVideo},
		VideoDraft: &creative.VideoDraft{
			TaskID: "creative_task_1", Prompt: "legacy prompt", DurationSeconds: 5, AspectRatio: "9:16", Resolution: "720p",
		},
	}}
	server := NewWithDependencies(Dependencies{
		Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"},
		Creative: creativeManager, ProviderJobs: jobs,
		Projects: staticProjectManager{context: contract.ProjectContext{
			OrganizationID: "org_1", ProjectID: "project_1", BrandID: &brandID,
			ProductIDs: []contract.ProductID{}, ProjectContextVersion: 7,
		}},
	})
	request := httptest.NewRequest(http.MethodPost, "/api/creative/v1/projects/project_1/creative-tasks/creative_task_1:video-job", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "creative-video-approved-1")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	input := jobs.videoRequest.Input
	if input.Prompt != plan.Prompt.CompiledPrompt ||
		input.InputMode != provider.VideoInputFirstLastFrame ||
		input.AudioPolicy != provider.VideoAudioSilent ||
		len(input.ConditioningAssets) != 2 {
		t.Fatalf("approved provider video input = %+v", input)
	}
	if creativeManager.registeredProviderJobID != jobs.job.ID {
		t.Fatalf("registered provider job = %q", creativeManager.registeredProviderJobID)
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

func TestFreezeCreativeVersionUsesIdempotencyKey(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}, Scopes: []contract.Scope{creative.ScopeRead, creative.ScopeWrite}}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	creativeManager := &creativeManagerStub{frozenVersion: creative.CreativeVersion{ID: "creative_version_1", OrganizationID: "org_1", ProjectID: "project_1", TaskID: "creative_task_1", Version: 1, DraftVersion: 1, Status: creative.CreativeVersionCreated}}
	server := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, Creative: creativeManager})
	request := httptest.NewRequest(http.MethodPost, "/api/creative/v1/projects/project_1/creative-tasks/creative_task_1:freeze-version", bytes.NewBufferString(`{"draft_version":1}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "creative-freeze-http-1")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if creativeManager.freezeKey != "creative-freeze-http-1" || creativeManager.freezeTaskID != "creative_task_1" {
		t.Fatalf("freeze request was not forwarded: key=%q task=%q", creativeManager.freezeKey, creativeManager.freezeTaskID)
	}
}

func TestReviseCreativeDraftUsesTaskActionAndExpectedVersion(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}, Scopes: []contract.Scope{creative.ScopeRead, creative.ScopeWrite}}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	creativeManager := &creativeManagerStub{revisedDraft: creative.ImageTextDraft{TaskID: "creative_task_1", Version: 2}}
	server := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, Creative: creativeManager})
	request := httptest.NewRequest(http.MethodPatch, "/api/creative/v1/projects/project_1/creative-tasks/creative_task_1:draft", bytes.NewBufferString(`{"expected_version":1,"title_candidates":["标题一","标题二","标题三"],"body":"正文","topics":[],"cover_copy":"封面标题","image_plan":[{"order":1,"purpose":"封面","visual_brief":"干净的产品图","caption":"封面标题"}]}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if creativeManager.reviseTaskID != "creative_task_1" || creativeManager.reviseRequest.ExpectedVersion != 1 {
		t.Fatalf("revision was not forwarded: task=%q request=%+v", creativeManager.reviseTaskID, creativeManager.reviseRequest)
	}
}

func TestCreativeHistoryReadEndpointsSurviveRefresh(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}, Scopes: []contract.Scope{creative.ScopeRead}}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	manager := &creativeManagerStub{
		versions: []creative.CreativeVersion{{ID: "creativeversion_1", TaskID: "creativetask_1"}},
		packages: []creative.CreativePackage{{ID: "creativepackage_1", CreativeVersionID: "creativeversion_1"}},
	}
	server := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"}, Creative: manager})

	for _, target := range []struct {
		path string
		want string
	}{
		{"/api/creative/v1/projects/project_1/creative-versions?task_id=creativetask_1", "creativeversion_1"},
		{"/api/creative/v1/projects/project_1/creative-packages", "creativepackage_1"},
	} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target.path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), target.want) {
			t.Fatalf("%s status=%d body=%s", target.path, response.Code, response.Body.String())
		}
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

func (staticProjectManager) GetDetail(context.Context, contract.ActorContext, contract.ProjectID) (project.ProjectDetail, error) {
	return project.ProjectDetail{}, nil
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
	job          contract.ProviderJob
	request      provider.CreateImageJobRequest
	videoRequest provider.CreateVideoJobRequest
	createCalls  int
}

type creativeManagerStub struct {
	detail                  creative.TaskDetail
	commerceSources         []creative.CreativeSourceOption
	preparedCommerce        creative.PreparedCommercePreroll
	registeredProviderJobID string
	frozenVersion           creative.CreativeVersion
	freezeKey               contract.IdempotencyKey
	freezeTaskID            string
	revisedDraft            creative.ImageTextDraft
	reviseTaskID            string
	reviseRequest           creative.ReviseDraftRequest
	versions                []creative.CreativeVersion
	packages                []creative.CreativePackage
}

func (s *creativeManagerStub) ListCommercePrerollSources(context.Context, contract.ActorContext, contract.ProjectID) ([]creative.CreativeSourceOption, error) {
	return s.commerceSources, nil
}
func (s *creativeManagerStub) PrepareCommercePreroll(context.Context, contract.ActorContext, contract.ProjectID, creative.PrepareCommercePrerollRequest) (creative.PreparedCommercePreroll, error) {
	return s.preparedCommerce, nil
}
func (s *creativeManagerStub) CreateIntake(context.Context, contract.RequestContext, contract.ProjectID, contract.IdempotencyKey, creative.CreateIntakeRequest) (creative.CreativeIntake, error) {
	return creative.CreativeIntake{}, nil
}
func (s *creativeManagerStub) ListIntakes(context.Context, contract.ActorContext, contract.ProjectID, int) ([]creative.CreativeIntake, error) {
	return nil, nil
}
func (s *creativeManagerStub) CreateTask(context.Context, contract.ActorContext, contract.ProjectID, string, creative.CreateTaskRequest) (creative.CreativeTask, error) {
	return creative.CreativeTask{}, nil
}
func (s *creativeManagerStub) CreateVideoTask(context.Context, contract.ActorContext, contract.ProjectID, string, creative.CreateVideoTaskRequest) (creative.CreativeTask, error) {
	return creative.CreativeTask{}, nil
}
func (s *creativeManagerStub) ListTasks(context.Context, contract.ActorContext, contract.ProjectID, int) ([]creative.CreativeTask, error) {
	return nil, nil
}
func (s *creativeManagerStub) GetTaskDetail(context.Context, contract.ActorContext, contract.ProjectID, string) (creative.TaskDetail, error) {
	return s.detail, nil
}
func (s *creativeManagerStub) AnalyzeViralRemake(context.Context, contract.ActorContext, contract.ProjectID, string) (creative.TaskDetail, error) {
	return s.detail, nil
}
func (s *creativeManagerStub) UpdateViralPrompt(context.Context, contract.ActorContext, contract.ProjectID, string, creative.UpdateViralPromptRequest) (creative.TaskDetail, error) {
	return s.detail, nil
}
func (s *creativeManagerStub) ConfirmViralGeneration(context.Context, contract.ActorContext, contract.ProjectID, string, creative.ConfirmViralGenerationRequest) (creative.TaskDetail, error) {
	return s.detail, nil
}
func (s *creativeManagerStub) ViralProviderInput(context.Context, contract.ActorContext, contract.ProjectID, string) (provider.VideoGenerationInput, string, error) {
	return provider.VideoGenerationInput{
		Prompt: "viral", DurationSeconds: 5, AspectRatio: "9:16", Resolution: "720p",
		InputMode: provider.VideoInputTextOnly, ConditioningAssets: []provider.VideoConditioningAsset{},
	}, "sha256:viral", nil
}
func (s *creativeManagerStub) RegisterViralCandidateJob(context.Context, contract.ActorContext, contract.ProjectID, string, string) (creative.TaskDetail, error) {
	return s.detail, nil
}
func (s *creativeManagerStub) ReconcileViralCandidate(context.Context, contract.ActorContext, contract.ProjectID, string, contract.ProviderJob) (creative.TaskDetail, error) {
	return s.detail, nil
}
func (s *creativeManagerStub) SubmitViralCandidateReview(context.Context, contract.ActorContext, contract.ProjectID, string, string) (creative.TaskDetail, error) {
	return s.detail, nil
}
func (s *creativeManagerStub) ArchiveTask(context.Context, contract.ActorContext, contract.ProjectID, string) error {
	return nil
}
func (s *creativeManagerStub) RegisterCoverImageJob(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, _ string, providerJobID string) error {
	s.registeredProviderJobID = providerJobID
	return nil
}
func (s *creativeManagerStub) RegisterImagePlanJob(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, _ string, _ int, providerJobID string) error {
	s.registeredProviderJobID = providerJobID
	return nil
}
func (s *creativeManagerStub) RegisterVideoJob(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, _ string, providerJobID string) error {
	s.registeredProviderJobID = providerJobID
	return nil
}
func (s *creativeManagerStub) CreateRenderJob(context.Context, contract.RequestContext, contract.ProjectID, string, creative.CreateRenderJobRequest, contract.IdempotencyKey) (creative.RenderJob, bool, error) {
	return creative.RenderJob{}, false, nil
}
func (s *creativeManagerStub) GetRenderJob(context.Context, contract.ActorContext, contract.ProjectID, string) (creative.RenderJob, error) {
	return creative.RenderJob{}, nil
}
func (s *creativeManagerStub) FreezeVersion(_ context.Context, _ contract.RequestContext, _ contract.ProjectID, taskID string, _ creative.FreezeVersionRequest, key contract.IdempotencyKey) (creative.CreativeVersion, bool, error) {
	s.freezeKey = key
	s.freezeTaskID = taskID
	return s.frozenVersion, false, nil
}
func (s *creativeManagerStub) ListVersions(context.Context, contract.ActorContext, contract.ProjectID, string, int) ([]creative.CreativeVersion, error) {
	return s.versions, nil
}
func (s *creativeManagerStub) ReviseDraft(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, taskID string, request creative.ReviseDraftRequest) (creative.ImageTextDraft, error) {
	s.reviseTaskID = taskID
	s.reviseRequest = request
	return s.revisedDraft, nil
}
func (s *creativeManagerStub) BindImageAsset(context.Context, contract.ActorContext, contract.ProjectID, string, creative.BindImageAssetRequest) (creative.ImageTextDraft, error) {
	return s.revisedDraft, nil
}
func (s *creativeManagerStub) CheckVersion(context.Context, contract.ActorContext, contract.ProjectID, string) (creative.CreativeVersion, error) {
	return s.frozenVersion, nil
}
func (s *creativeManagerStub) ApproveVersion(context.Context, contract.ActorContext, contract.ProjectID, string) (creative.CreativeVersion, error) {
	return s.frozenVersion, nil
}
func (s *creativeManagerStub) DeliverVersion(context.Context, contract.ActorContext, contract.ProjectID, string) (creative.CreativePackage, error) {
	return creative.CreativePackage{}, nil
}
func (s *creativeManagerStub) ListPackages(context.Context, contract.ActorContext, contract.ProjectID, int) ([]creative.CreativePackage, error) {
	return s.packages, nil
}

func (s *providerJobStub) CreateImageJob(_ context.Context, request provider.CreateImageJobRequest) (contract.ProviderJob, bool, error) {
	s.createCalls++
	s.request = request
	return s.job, false, nil
}

func (s *providerJobStub) CreateVideoJob(_ context.Context, request provider.CreateVideoJobRequest) (contract.ProviderJob, bool, error) {
	s.createCalls++
	s.videoRequest = request
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
