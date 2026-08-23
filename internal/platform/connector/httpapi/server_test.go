package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/connector"
	"github.com/shikanon/cookies/internal/platform/contract"
)

type readerStub struct{ query connector.Query }

func (r *readerStub) Snapshot(_ context.Context, q connector.Query) (connector.CanonicalSnapshot, error) {
	r.query = q
	return connector.CanonicalSnapshot{DatasetVersion: connector.DatasetVersion, PredictionCutoff: q.PredictionCutoff}, nil
}

type syncerStub struct {
	mu      sync.Mutex
	request connector.SyncRequest
}
type authorizerStub struct{ err error }
type sessionManagerStub struct{ plaintext string }
type accountManagerStub struct{ verifyErr error }

func (s accountManagerStub) Register(context.Context, connector.RegisterAccountRequest) (connector.PlatformAccount, error) {
	return connector.PlatformAccount{}, nil
}
func (s accountManagerStub) List(context.Context, string, string) ([]connector.PlatformAccount, error) {
	return nil, nil
}
func (s accountManagerStub) Claim(context.Context, string, string, string) (connector.PlatformAccount, error) {
	return connector.PlatformAccount{}, nil
}
func (s accountManagerStub) Verify(context.Context, string, string, string) (connector.PlatformAccount, error) {
	return connector.PlatformAccount{}, s.verifyErr
}
func (s accountManagerStub) Revoke(context.Context, string, string, string) (connector.PlatformAccount, error) {
	return connector.PlatformAccount{}, nil
}

func (s *sessionManagerStub) Get(context.Context, string, string) (connector.OceanEngineAccountSession, error) {
	return connector.OceanEngineAccountSession{ID: "session_safe", OrganizationID: "org_1", AccountID: "oeacct_safe", Status: connector.AccountSessionUnverified, CredentialRefPresent: true, Version: 1}, nil
}
func (s *sessionManagerStub) Update(_ context.Context, _, _ string, plaintext []byte, _ int64) (connector.OceanEngineAccountSession, error) {
	s.plaintext = string(plaintext)
	return connector.OceanEngineAccountSession{ID: "session_safe", OrganizationID: "org_1", AccountID: "oeacct_safe", Status: connector.AccountSessionUnverified, CredentialRefPresent: true, Version: 1}, nil
}

func (a authorizerStub) AuthorizeProject(context.Context, contract.ActorContext, contract.ProjectID) error {
	return a.err
}

func (s *syncerStub) Sync(_ context.Context, r connector.SyncRequest) (connector.SyncResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.request = r
	return connector.SyncResult{RunID: "sync_opaque"}, nil
}

func (s *syncerStub) lastRequest() connector.SyncRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.request
}
func request(method, path, body, scope string) *http.Request {
	value := httptest.NewRequest(method, path, strings.NewReader(body))
	value = value.WithContext(contract.WithRequestContext(value.Context(), contract.RequestContext{RequestID: "request_1", TraceID: "trace_1", Actor: contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{contract.Scope(scope)}}}))
	return value
}

func TestCanonicalSnapshotRequiresCutoffAndScopesAccount(t *testing.T) {
	reader := &readerStub{}
	server := New(reader, nil, authorizerStub{}, nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request(http.MethodGet, "/api/connector/v1/projects/project_1/accounts/raw-account/canonical-snapshots?prediction_cutoff=2026-08-20T08:00:00Z", "", connector.ScopeRead))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if reader.query.ProjectID != "project_1" || reader.query.SourceRef == "raw-account" || reader.query.SourceRef == "" {
		t.Fatalf("query=%#v", reader.query)
	}
	if strings.Contains(response.Body.String(), "raw-account") || strings.Contains(strings.ToLower(response.Body.String()), "cookie") {
		t.Fatalf("sensitive response=%s", response.Body.String())
	}
}
func TestOrganizationSnapshotRequiresNoBusinessProject(t *testing.T) {
	reader := &readerStub{}
	server := New(reader, nil, authorizerStub{err: context.Canceled}, nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request(http.MethodGet, "/api/connector/v1/accounts/oeacct_safe/canonical-snapshots?prediction_cutoff=2026-08-20T08:00:00Z", "", connector.ScopeRead))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if reader.query.ProjectID != "" || reader.query.OrganizationID != "org_1" || reader.query.SourceRef == "" {
		t.Fatalf("query=%#v", reader.query)
	}
}
func TestCanonicalSnapshotRejectsMissingCutoffAndScope(t *testing.T) {
	server := New(&readerStub{}, nil, authorizerStub{}, nil)
	for _, test := range []struct {
		path, scope string
		status      int
	}{{"/api/connector/v1/projects/project_1/accounts/a/canonical-snapshots", connector.ScopeRead, http.StatusBadRequest}, {"/api/connector/v1/projects/project_1/accounts/a/canonical-snapshots?prediction_cutoff=2026-08-20T08:00:00Z", "insights.read", http.StatusForbidden}} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request(http.MethodGet, test.path, "", test.scope))
		if response.Code != test.status {
			t.Fatalf("path=%s status=%d body=%s", test.path, response.Code, response.Body.String())
		}
	}
}
func TestSyncRequiresIdempotencyAndPassesNoCredentialMaterial(t *testing.T) {
	syncer := &syncerStub{}
	server := New(nil, syncer, authorizerStub{}, nil)
	value := request(http.MethodPost, "/api/connector/v1/projects/project_1/accounts/account-1/syncs", `{"start":"2026-08-19T00:00:00Z","end":"2026-08-20T00:00:00Z","time_zone":"Asia/Shanghai","currency":"CNY"}`, connector.ScopeSync)
	value.Header.Set("Idempotency-Key", "sync-request-1")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, value)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	syncedRequest := syncer.lastRequest()
	if syncedRequest.IdempotencyKey != "sync-request-1" || syncedRequest.AccountRef != "account-1" || !syncedRequest.WindowEnd.After(syncedRequest.WindowStart) {
		t.Fatalf("request=%#v", syncedRequest)
	}
	if strings.Contains(strings.ToLower(response.Body.String()), "token") || strings.Contains(strings.ToLower(response.Body.String()), "cookie") {
		t.Fatalf("sensitive response=%s", response.Body.String())
	}
}

func TestOrganizationSyncAndSessionDoNotRequireProjectOrReturnCredential(t *testing.T) {
	syncer := &syncerStub{}
	sessions := &sessionManagerStub{}
	server := New(nil, syncer, authorizerStub{err: context.Canceled}, nil, sessions)
	sessionRequest := request(http.MethodPut, "/api/connector/v1/accounts/oeacct_safe/session", `{"session":"synthetic-cookie","expected_version":0}`, connector.ScopeSync)
	sessionResponse := httptest.NewRecorder()
	server.ServeHTTP(sessionResponse, sessionRequest)
	if sessionResponse.Code != http.StatusOK || sessions.plaintext != "synthetic-cookie" {
		t.Fatalf("status=%d plaintext=%q body=%s", sessionResponse.Code, sessions.plaintext, sessionResponse.Body.String())
	}
	if strings.Contains(sessionResponse.Body.String(), "synthetic-cookie") {
		t.Fatalf("credential returned in response: %s", sessionResponse.Body.String())
	}
	syncRequest := request(http.MethodPost, "/api/connector/v1/accounts/oeacct_safe/syncs", `{"start":"2026-08-19T00:00:00Z","end":"2026-08-20T00:00:00Z"}`, connector.ScopeSync)
	syncRequest.Header.Set("Idempotency-Key", "org-sync-1")
	syncResponse := httptest.NewRecorder()
	server.ServeHTTP(syncResponse, syncRequest)
	var syncedRequest connector.SyncRequest
	for attempt := 0; attempt < 100; attempt++ {
		syncedRequest = syncer.lastRequest()
		if syncedRequest.OrganizationID != "" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if syncResponse.Code != http.StatusAccepted || syncedRequest.ProjectID != "" || syncedRequest.OrganizationID != "org_1" {
		t.Fatalf("status=%d request=%#v body=%s", syncResponse.Code, syncedRequest, syncResponse.Body.String())
	}
}

func TestOrganizationVerifyDoesNotReportEveryPlatformFailureAsVersionConflict(t *testing.T) {
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{connector.ErrImmutableConflict, http.StatusConflict, "VERSION_CONFLICT"},
		{connector.ErrAccountSessionInvalid, http.StatusUnprocessableEntity, "CONNECTOR_ACCOUNT_SESSION_INVALID"},
		{connector.ErrAccountVerificationUnavailable, http.StatusBadGateway, "CONNECTOR_ACCOUNT_VERIFY_UNAVAILABLE"},
	}
	for _, test := range tests {
		server := New(nil, nil, authorizerStub{}, accountManagerStub{verifyErr: test.err})
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request(http.MethodPost, "/api/connector/v1/accounts/oeacct_safe/verify", "", connector.ScopeSync))
		if response.Code != test.status || !strings.Contains(response.Body.String(), test.code) {
			t.Fatalf("error=%v status=%d body=%s", test.err, response.Code, response.Body.String())
		}
	}
}
