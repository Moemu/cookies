package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Cecillia803/cookies/internal/platform/contract"
	"github.com/Cecillia803/cookies/internal/platform/identity"
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
	resolver, err := identity.NewStaticResolver(contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}, ProjectID: "project_1"})
	if err != nil {
		t.Fatal(err)
	}
	server := NewWithDependencies(Dependencies{Resolver: resolver, ProjectAuthorizer: identity.StaticProjectAuthorizer{}})
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
		ProjectID:      "project_1",
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
