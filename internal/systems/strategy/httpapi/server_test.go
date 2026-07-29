package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/agent"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
	"github.com/shikanon/cookies/internal/systems/strategy"
)

func TestActionRoutesUseFrozenColonSyntax(t *testing.T) {
	t.Parallel()
	server := New(strategy.Service{}, agent.MySQLStore{}, jobruntime.MySQLStore{})
	tests := []string{
		"/api/strategy/v1/strategy-drafts/strategy_1:revise",
		"/api/strategy/v1/strategy-drafts/strategy_1:submit",
		"/api/strategy/v1/strategy-drafts/strategy_1:approve",
		"/api/strategy/v1/strategy-reviews/review_1:return",
		"/api/strategy/v1/agent-tasks/agent_1:cancel",
	}
	for _, path := range tests {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, strings.NewReader("{")))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestUnknownActionDoesNotReachService(t *testing.T) {
	t.Parallel()
	server := New(strategy.Service{}, agent.MySQLStore{}, jobruntime.MySQLStore{})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/strategy/v1/strategy-drafts/strategy_1:delete", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestCreativeHandoffRouteRejectsInvalidVersionBeforeService(t *testing.T) {
	t.Parallel()
	server := New(strategy.Service{}, agent.MySQLStore{}, jobruntime.MySQLStore{})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(
		http.MethodGet,
		"/api/strategy/v1/projects/project_1/strategy-packages/package_1/versions/not-a-version/creative-handoff",
		nil,
	))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestTaskAndDeepReviewRoutesRejectInvalidBodies(t *testing.T) {
	t.Parallel()
	server := New(strategy.Service{}, agent.MySQLStore{}, jobruntime.MySQLStore{})
	for _, path := range []string{
		"/api/strategy/v1/projects/project_1/tasks",
		"/api/strategy/v1/strategy-reviews/review_1/deep-analysis",
	} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, strings.NewReader("{")))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestMatchesIfNoneMatch(t *testing.T) {
	t.Parallel()
	etag := `"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`
	for _, header := range []string{
		etag,
		`"other", ` + etag,
		`W/` + etag,
		"*",
	} {
		if !matchesIfNoneMatch(header, etag) {
			t.Fatalf("header %q did not match %q", header, etag)
		}
	}
	for _, header := range []string{"", `"other"`, `W/"other"`} {
		if matchesIfNoneMatch(header, etag) {
			t.Fatalf("header %q unexpectedly matched %q", header, etag)
		}
	}
}
