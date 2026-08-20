package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/connector"
	"github.com/shikanon/cookies/internal/platform/contract"
)

type readerStub struct{ query connector.Query }

func (r *readerStub) Snapshot(_ context.Context, q connector.Query) (connector.CanonicalSnapshot, error) {
	r.query = q
	return connector.CanonicalSnapshot{DatasetVersion: connector.DatasetVersion, PredictionCutoff: q.PredictionCutoff}, nil
}

type syncerStub struct{ request connector.SyncRequest }
type authorizerStub struct{ err error }

func (a authorizerStub) AuthorizeProject(context.Context, contract.ActorContext, contract.ProjectID) error {
	return a.err
}

func (s *syncerStub) Sync(_ context.Context, r connector.SyncRequest) (connector.SyncResult, error) {
	s.request = r
	return connector.SyncResult{RunID: "sync_opaque"}, nil
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
	if syncer.request.IdempotencyKey != "sync-request-1" || syncer.request.AccountRef != "account-1" || !syncer.request.WindowEnd.After(syncer.request.WindowStart) {
		t.Fatalf("request=%#v", syncer.request)
	}
	if strings.Contains(strings.ToLower(response.Body.String()), "token") || strings.Contains(strings.ToLower(response.Body.String()), "cookie") {
		t.Fatalf("sensitive response=%s", response.Body.String())
	}
}
