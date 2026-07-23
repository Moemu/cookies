package creative

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
)

func TestCreativeImageRequestCreatesProviderJobFromApprovedPlan(t *testing.T) {
	t.Parallel()
	actor := actor()
	resolver, err := identity.NewStaticResolver(actor)
	if err != nil {
		t.Fatal(err)
	}
	jobs := &jobStub{}
	handler := NewHTTPHandler(HTTPDependencies{
		Service: Service{
			Store: &planStore{items: map[string]Plan{"plan_1": {
				ID: "plan_1", OrganizationID: "org_1", ProjectID: "project_1", MediaType: MediaImage,
				Prompt: "safe prompt", ModelAlias: "cookies.image.standard",
			}}},
			Jobs: jobs,
		},
		Resolver: resolver, Authorizer: identity.StaticProjectAuthorizer{ProjectID: "project_1"},
		Projects: creativeStaticProjects{project: project()},
	})
	request := httptest.NewRequest(http.MethodPost, "/api/creative/v1/projects/project_1/plans/plan_1/image-jobs", bytes.NewBufferString(`{"width":1024,"height":1024}`))
	request.Header.Set("Idempotency-Key", "creative-job-1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if jobs.request.SourceSystem != "creative" || jobs.request.SourceTaskID != "plan_1" {
		t.Fatalf("unexpected job request: %#v", jobs.request)
	}
	if strings.Contains(response.Body.String(), "safe prompt") || strings.Contains(response.Body.String(), "api_key") {
		t.Fatalf("creative API leaked protected data: %s", response.Body.String())
	}
}

type creativeStaticProjects struct{ project contract.ProjectContext }

func (s creativeStaticProjects) GetContext(context.Context, contract.ActorContext, contract.ProjectID) (contract.ProjectContext, error) {
	return s.project, nil
}
