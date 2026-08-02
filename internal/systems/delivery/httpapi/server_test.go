package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/systems/delivery"
)

func TestDeliveryHTTPExposesPlanAndControlledActions(t *testing.T) {
	t.Parallel()
	app := &applicationStub{
		plan:      delivery.DeliveryPlan{ID: "deliveryplan_1", Version: 1},
		changeSet: delivery.ChangeSet{ID: "deliverychangeset_1", PlanID: "deliveryplan_1", Version: 1},
	}
	server := New(app)

	response := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodPost, "/api/delivery/v1/projects/project_1/plans", `{
		"creative_package_id":"creativepackage_1","name":"首轮投放","objective":"验证点击",
		"budget_cents":10000,"start_at":"2026-07-25T00:00:00Z","end_at":"2026-07-26T00:00:00Z"
	}`)
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), "deliveryplan_1") {
		t.Fatalf("create status=%d body=%s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	request = authenticatedRequest(http.MethodPost, "/api/delivery/v1/projects/project_1/plans/deliveryplan_1:create-change-set", `{"expected_version":1}`)
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || app.createdPlanID != "deliveryplan_1" {
		t.Fatalf("change-set status=%d body=%s plan=%q", response.Code, response.Body.String(), app.createdPlanID)
	}
}

func TestDeliveryHTTPMapsProjectIsolationDenial(t *testing.T) {
	response := httptest.NewRecorder()
	request := authenticatedRequest(http.MethodGet, "/api/delivery/v1/projects/project_other/plans/plan_1", "")
	writeError(response, request, identity.ErrProjectAccessDenied)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "PROJECT_ACCESS_DENIED") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestApprovalRequestRejectsInjectedIdentityAndScopeFields(t *testing.T) {
	t.Parallel()
	server := New(&applicationStub{
		changeSet: delivery.ChangeSet{ID: "deliverychangeset_1", Version: 2},
	})
	fields := []string{"actor", "role", "approver", "scope"}
	for _, field := range fields {
		t.Run(field, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := authenticatedRequest(
				http.MethodPost,
				"/api/delivery/v1/projects/project_1/change-sets/deliverychangeset_1:approve",
				`{"expected_version":2,"`+field+`":"forged"}`,
			)
			server.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("injected %s status=%d body=%s", field, response.Code, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), `"source":"mock"`) ||
				!strings.Contains(response.Body.String(), `"scenario":"invalid_request"`) {
				t.Fatalf("injected %s response lacks mock provenance: %s", field, response.Body.String())
			}
		})
	}
}

func TestDeliveryHTTPMapsStableApprovalErrorsWithMockProvenance(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "required", err: delivery.ErrApprovalRequired, status: http.StatusConflict, code: "APPROVAL_REQUIRED"},
		{name: "expired", err: delivery.ErrApprovalExpired, status: http.StatusConflict, code: "APPROVAL_EXPIRED"},
		{name: "content mismatch", err: delivery.ErrApprovalContentMismatch, status: http.StatusConflict, code: "APPROVAL_CONTENT_MISMATCH"},
		{name: "scope exceeded", err: delivery.ErrApprovalScopeExceeded, status: http.StatusForbidden, code: "APPROVAL_SCOPE_EXCEEDED"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := authenticatedRequest(http.MethodPost, "/api/delivery/v1/projects/project_1/change-sets/change_1:execute", `{"expected_version":3}`)
			writeError(response, request, testCase.err)
			if response.Code != testCase.status ||
				!strings.Contains(response.Body.String(), `"`+testCase.code+`"`) ||
				!strings.Contains(response.Body.String(), `"source":"mock"`) ||
				!strings.Contains(response.Body.String(), `"scenario":"`+strings.ToLower(testCase.code)+`"`) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func authenticatedRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	ctx := contract.WithRequestContext(request.Context(), contract.RequestContext{
		RequestID: "req_1", TraceID: "trace_1",
		Actor: contract.ActorContext{
			OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
			Scopes: []contract.Scope{delivery.ScopeRead, delivery.ScopeWrite, delivery.ScopeApprove, delivery.ScopeExecute},
		},
	})
	return request.WithContext(ctx)
}

type applicationStub struct {
	plan          delivery.DeliveryPlan
	changeSet     delivery.ChangeSet
	createdPlanID string
}

func (s *applicationStub) CreatePlan(context.Context, contract.ActorContext, contract.ProjectID, delivery.CreatePlanRequest) (delivery.DeliveryPlan, error) {
	return s.plan, nil
}
func (s *applicationStub) UpdatePlan(context.Context, contract.ActorContext, contract.ProjectID, string, delivery.UpdatePlanRequest) (delivery.DeliveryPlan, error) {
	return s.plan, nil
}
func (s *applicationStub) ListPlans(context.Context, contract.ActorContext, contract.ProjectID, int) ([]delivery.DeliveryPlan, error) {
	return []delivery.DeliveryPlan{s.plan}, nil
}
func (s *applicationStub) GetPlan(context.Context, contract.ActorContext, contract.ProjectID, string) (delivery.DeliveryPlan, error) {
	return s.plan, nil
}
func (s *applicationStub) ListPlanVersions(context.Context, contract.ActorContext, contract.ProjectID, string) ([]delivery.DeliveryPlanVersion, error) {
	return s.plan.Versions, nil
}
func (s *applicationStub) GetPlanVersion(context.Context, contract.ActorContext, contract.ProjectID, string, int) (delivery.DeliveryPlanVersion, error) {
	return s.plan.CurrentVersion, nil
}
func (s *applicationStub) RunPlanPreflight(context.Context, contract.ActorContext, contract.ProjectID, string) (delivery.PreflightResult, error) {
	return delivery.PreflightResult{PlanID: s.plan.ID, Source: delivery.SourceMock}, nil
}
func (s *applicationStub) GetPlanDetail(context.Context, contract.ActorContext, contract.ProjectID, string) (delivery.PlanDetail, error) {
	return delivery.PlanDetail{Plan: s.plan}, nil
}
func (s *applicationStub) ListChangeSets(context.Context, contract.ActorContext, contract.ProjectID, int) ([]delivery.ChangeSet, error) {
	return []delivery.ChangeSet{s.changeSet}, nil
}
func (s *applicationStub) GetChangeSet(context.Context, contract.ActorContext, contract.ProjectID, string) (delivery.ChangeSet, error) {
	return s.changeSet, nil
}
func (s *applicationStub) CreateChangeSet(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, planID string, _ int64) (delivery.ChangeSet, error) {
	s.createdPlanID = planID
	return s.changeSet, nil
}
func (s *applicationStub) Preflight(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (delivery.ChangeSet, error) {
	return s.changeSet, nil
}
func (s *applicationStub) Approve(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (delivery.ChangeSet, error) {
	return s.changeSet, nil
}
func (s *applicationStub) Execute(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (delivery.ExecutionResult, error) {
	return delivery.ExecutionResult{ChangeSet: s.changeSet, Execution: delivery.Execution{CompletedAt: time.Now()}}, nil
}
func (s *applicationStub) Rollback(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (delivery.ChangeSet, error) {
	return s.changeSet, nil
}
func (s *applicationStub) ListExecutions(context.Context, contract.ActorContext, contract.ProjectID, int) ([]delivery.ExecutionResult, error) {
	return nil, nil
}
func (s *applicationStub) CreateDemoMetricSnapshot(context.Context, contract.ActorContext, contract.ProjectID, string, delivery.CreateMetricSnapshotRequest) (delivery.DeliveryMetricSnapshot, error) {
	return delivery.DeliveryMetricSnapshot{ID: "deliverymetric_1", IsSimulated: true}, nil
}
func (s *applicationStub) ListMetricSnapshots(context.Context, contract.ActorContext, contract.ProjectID, string, int) ([]delivery.DeliveryMetricSnapshot, error) {
	return []delivery.DeliveryMetricSnapshot{{ID: "deliverymetric_1", IsSimulated: true}}, nil
}
