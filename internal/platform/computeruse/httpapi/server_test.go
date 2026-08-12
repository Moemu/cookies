package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/computeruse"
	"github.com/shikanon/cookies/internal/platform/contract"
)

type projectAuthorizer struct{}

func (projectAuthorizer) AuthorizeProject(_ context.Context, actor contract.ActorContext, project contract.ProjectID) error {
	if actor.OrganizationID != "org_1" || project != "project_1" {
		return computeruse.ErrNotFound
	}
	return nil
}
func (projectAuthorizer) AuthorizeProjectAction(ctx context.Context, actor contract.ActorContext, project contract.ProjectID, _ string) error {
	return projectAuthorizer{}.AuthorizeProject(ctx, actor, project)
}

func TestRunEndpointsRequireScopeAndProjectIsolation(t *testing.T) {
	repo := computeruse.NewMemoryRepository()
	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	run := validHTTPRun(now)
	_, _, _ = repo.CreateRun(context.Background(), run)
	service := computeruse.Service{Repository: repo, Now: func() time.Time { return now }}
	server := New(service, computeruse.Worker{Service: service, Adapter: computeruse.DeterministicFakeAdapter{}}, projectAuthorizer{})
	request := httptest.NewRequest(http.MethodGet, "/api/platform/v1/computer-use/projects/project_1/runs/run_1", nil)
	request = request.WithContext(contract.WithRequestContext(request.Context(), contract.RequestContext{RequestID: "req", TraceID: "trace", Actor: contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user"}, Scopes: []contract.Scope{"delivery.read"}}}))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/api/platform/v1/computer-use/projects/project_1/runs/run_1", nil)
	request = request.WithContext(contract.WithRequestContext(request.Context(), contract.RequestContext{RequestID: "req", TraceID: "trace", Actor: contract.ActorContext{OrganizationID: "org_2", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user"}, Scopes: []contract.Scope{"delivery.read"}}}))
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-org status=%d", response.Code)
	}
}

func TestTakeoverEvidenceEndpointRecordsOnlyFencedReadback(t *testing.T) {
	repo := computeruse.NewMemoryRepository()
	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	run := validHTTPRun(now)
	run.State, run.Paused, run.TakeoverActive = computeruse.RunAwaitingTakeover, true, true
	_, _, _ = repo.CreateRun(context.Background(), run)
	repo.PutSitePolicy(computeruse.SitePolicy{ID: run.PolicyID, OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, Platform: run.Platform, AccountID: run.AccountID, AllowedProtocols: []string{"https"}, AllowedHosts: []string{"ad.oceanengine.com"}, AllowedPageKinds: []string{"project_create"}, AllowedPlatformProjects: []string{"test_project"}, Version: 1})
	idSequence := 0
	service := computeruse.Service{Repository: repo, Now: func() time.Time { return now }, NewID: func(prefix string) (string, error) {
		idSequence++
		return fmt.Sprintf("%s_http_%d", prefix, idSequence), nil
	}}
	acquired, err := service.AcquireRunLease(context.Background(), run.OrganizationID, run.ProjectID, run.ID, run.Version, "agent")
	if err != nil {
		t.Fatal(err)
	}
	server := New(service, computeruse.Worker{Service: service, Adapter: computeruse.DeterministicFakeAdapter{}}, projectAuthorizer{})
	body := fmt.Sprintf(`{"expected_version":%d,"lease_id":%q,"fencing_token":%d,"step_id":"step_1","sequence":1,"action":"field_readback","status":"succeeded","page_kind":"project_create","platform_project_id":"test_project","before_page_facts":{"page_kind":"project_create"},"after_page_facts":{"page_kind":"project_create"},"field_readback":{"daily_budget":"300"},"diff_keys":[],"page_reference":"https://ad.oceanengine.com/superior/create-project?aadvid=secret","selector_version":"oceanengine-live-locators/v0.1","action_version":"takeover-readback/v1"}`, acquired.Run.Version, acquired.Lease.ID, acquired.Lease.FencingToken)
	request := httptest.NewRequest(http.MethodPost, "/api/platform/v1/computer-use/projects/project_1/runs/run_1/takeover-evidence", strings.NewReader(body))
	request = request.WithContext(contract.WithRequestContext(request.Context(), contract.RequestContext{RequestID: "req", TraceID: "trace", Actor: contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user"}, Scopes: []contract.Scope{"delivery.execute"}}}))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "aadvid") || !strings.Contains(response.Body.String(), `"daily_budget":"300"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRunLeaseHeartbeatAndReleaseAreScopedAndDetachTheRun(t *testing.T) {
	repo := computeruse.NewMemoryRepository()
	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	run := validHTTPRun(now)
	_, _, _ = repo.CreateRun(context.Background(), run)
	service := computeruse.Service{Repository: repo, Now: func() time.Time { return now }}
	acquired, err := service.AcquireRunLease(context.Background(), run.OrganizationID, run.ProjectID, run.ID, run.Version, "agent")
	if err != nil {
		t.Fatal(err)
	}
	server := New(service, computeruse.Worker{Service: service, Adapter: computeruse.DeterministicFakeAdapter{}}, projectAuthorizer{})
	call := func(action, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/platform/v1/computer-use/projects/project_1/runs/run_1/leases/%s:%s", acquired.Lease.ID, action), strings.NewReader(body))
		request = request.WithContext(contract.WithRequestContext(request.Context(), contract.RequestContext{RequestID: "req", TraceID: "trace", Actor: contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user"}, Scopes: []contract.Scope{"delivery.execute"}}}))
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		return response
	}
	heartbeat := call("heartbeat", fmt.Sprintf(`{"expected_version":%d,"fencing_token":%d}`, acquired.Lease.Version, acquired.Lease.FencingToken))
	if heartbeat.Code != http.StatusOK {
		t.Fatalf("heartbeat status=%d body=%s", heartbeat.Code, heartbeat.Body.String())
	}
	var renewed computeruse.SessionLease
	if err := json.Unmarshal(heartbeat.Body.Bytes(), &renewed); err != nil {
		t.Fatal(err)
	}
	release := call("release", fmt.Sprintf(`{"expected_run_version":%d,"expected_lease_version":%d,"fencing_token":%d}`, acquired.Run.Version, renewed.Version, renewed.FencingToken))
	if release.Code != http.StatusOK || !strings.Contains(release.Body.String(), `"lease_id":""`) || !strings.Contains(release.Body.String(), `"released_at"`) {
		t.Fatalf("release status=%d body=%s", release.Code, release.Body.String())
	}
	latest, err := repo.GetRun(context.Background(), run.OrganizationID, run.ProjectID, run.ID)
	if err != nil || latest.LeaseID != "" || latest.Version != acquired.Run.Version+1 {
		t.Fatalf("detached run=%+v err=%v", latest, err)
	}
	reacquired, err := service.AcquireRunLease(context.Background(), run.OrganizationID, run.ProjectID, run.ID, latest.Version, "agent_2")
	if err != nil || reacquired.Lease.FencingToken <= renewed.FencingToken {
		t.Fatalf("reacquired=%+v err=%v", reacquired, err)
	}
}

func TestCreateRunEndpointIsProjectScopedAndIdempotent(t *testing.T) {
	repo := computeruse.NewMemoryRepository()
	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	template := validHTTPRun(now)
	repo.PutEnvironment(computeruse.ExecutionEnvironment{ID: template.EnvironmentID, OrganizationID: template.OrganizationID, ProjectID: template.ProjectID, Platform: template.Platform, AccountID: template.AccountID, Mode: "local_visible", BrowserVersion: "test", Region: "local", Healthy: true, Version: 1})
	repo.PutBrowserProfile(computeruse.BrowserProfile{ID: template.ProfileID, OrganizationID: template.OrganizationID, ProjectID: template.ProjectID, EnvironmentID: template.EnvironmentID, Platform: template.Platform, AccountID: template.AccountID, State: "ready", Version: 1})
	repo.PutSitePolicy(computeruse.SitePolicy{ID: template.PolicyID, OrganizationID: template.OrganizationID, ProjectID: template.ProjectID, Platform: template.Platform, AccountID: template.AccountID, AllowedProtocols: []string{"https"}, AllowedHosts: []string{"ad.oceanengine.com"}, AllowedPageKinds: []string{"project_create"}, AllowedPlatformProjects: []string{"test_project"}, Version: 1})
	idSequence := 0
	service := computeruse.Service{Repository: repo, Now: func() time.Time { return now }, NewID: func(prefix string) (string, error) {
		idSequence++
		return fmt.Sprintf("%s_create_%d", prefix, idSequence), nil
	}}
	server := New(service, computeruse.Worker{Service: service, Adapter: computeruse.DeterministicFakeAdapter{}}, projectAuthorizer{})
	body := fmt.Sprintf(`{"project_id":"project_1","platform":"ocean_engine","account_id":"account","environment_id":"env","profile_id":"profile","policy_id":"policy","authority":%s}`, mustJSON(t, template.Authority))
	call := func(payload, key string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/api/platform/v1/computer-use/projects/project_1/runs", strings.NewReader(payload))
		request.Header.Set("Idempotency-Key", key)
		request = request.WithContext(contract.WithRequestContext(request.Context(), contract.RequestContext{RequestID: "req", TraceID: "trace", Actor: contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user"}, Scopes: []contract.Scope{"delivery.execute"}}}))
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		return response
	}
	created := call(body, "run-key")
	if created.Code != http.StatusCreated {
		t.Fatalf("created status=%d body=%s", created.Code, created.Body.String())
	}
	replayed := call(body, "run-key")
	if replayed.Code != http.StatusOK || replayed.Body.String() != created.Body.String() {
		t.Fatalf("replayed status=%d body=%s", replayed.Code, replayed.Body.String())
	}
	drifted := strings.Replace(body, `"workflow_step_id":"submit"`, `"workflow_step_id":"review"`, 1)
	conflict := call(drifted, "run-key")
	if conflict.Code != http.StatusConflict {
		t.Fatalf("conflict status=%d body=%s", conflict.Code, conflict.Body.String())
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}

func validHTTPRun(now time.Time) computeruse.ComputerUseRun {
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	authority := computeruse.AuthorityBinding{SchemaVersion: computeruse.AuthoritySchemaV1, OrganizationID: "org_1", ProjectID: "project_1", BusinessExecutionID: "exec", ChangeSetID: "change", ApprovalID: "approval", ApprovalActionHash: hash, AccountReferenceID: "account", ObjectFingerprint: hash, Action: "create_project_and_promotions", Currency: "CNY", PlanCanonicalHash: hash, IntentCanonicalHash: hash, FeedbackCanonicalHash: hash, DecisionCanonicalHash: hash, ConfigurationCanonicalHash: hash, WorkflowID: "workflow", WorkflowCanonicalHash: hash, WorkflowStepID: "submit", SkillID: "skill", SkillVersion: "v1"}
	return computeruse.ComputerUseRun{SchemaVersion: computeruse.RunSchemaV1, ID: "run_1", OrganizationID: "org_1", ProjectID: "project_1", Platform: computeruse.PlatformOceanEngine, AccountID: "account", Authority: authority, EnvironmentID: "env", ProfileID: "profile", PolicyID: "policy", State: computeruse.RunQueued, Version: 1, IdempotencyKey: "key", RequestHash: hash, CreatedBy: "user", CreatedAt: now, UpdatedAt: now}
}
