package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func validHTTPRun(now time.Time) computeruse.ComputerUseRun {
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	authority := computeruse.AuthorityBinding{SchemaVersion: computeruse.AuthoritySchemaV1, OrganizationID: "org_1", ProjectID: "project_1", BusinessExecutionID: "exec", ChangeSetID: "change", ApprovalID: "approval", ApprovalActionHash: hash, AccountReferenceID: "account", ObjectFingerprint: hash, Action: "create_project_and_promotions", Currency: "CNY", PlanCanonicalHash: hash, IntentCanonicalHash: hash, FeedbackCanonicalHash: hash, DecisionCanonicalHash: hash, ConfigurationCanonicalHash: hash, WorkflowID: "workflow", WorkflowCanonicalHash: hash, WorkflowStepID: "submit", SkillID: "skill", SkillVersion: "v1"}
	return computeruse.ComputerUseRun{SchemaVersion: computeruse.RunSchemaV1, ID: "run_1", OrganizationID: "org_1", ProjectID: "project_1", Platform: computeruse.PlatformOceanEngine, AccountID: "account", Authority: authority, EnvironmentID: "env", ProfileID: "profile", PolicyID: "policy", State: computeruse.RunQueued, Version: 1, IdempotencyKey: "key", RequestHash: hash, CreatedBy: "user", CreatedAt: now, UpdatedAt: now}
}
