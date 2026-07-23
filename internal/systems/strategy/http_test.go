package strategy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
)

func TestStrategyGenerateRejectsMissingTextScope(t *testing.T) {
	t.Parallel()
	actor := testActor()
	actor.Scopes = []contract.Scope{ScopeStrategyWrite}
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHTTPHandler(HTTPDependencies{
		Service:  Service{Store: newMemoryStore()},
		Resolver: resolver, Authorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"},
		Projects: staticProjects{project: testProject()},
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/strategy/v1/projects/project_1/proposals/proposal_1/generate", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

type staticProjects struct{ project contract.ProjectContext }

func (s staticProjects) GetContext(context.Context, contract.ActorContext, contract.ProjectID) (contract.ProjectContext, error) {
	return s.project, nil
}
