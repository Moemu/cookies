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
