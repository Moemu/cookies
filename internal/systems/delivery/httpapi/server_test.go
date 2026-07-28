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
func (s *applicationStub) ListPlans(context.Context, contract.ActorContext, contract.ProjectID, int) ([]delivery.DeliveryPlan, error) {
	return []delivery.DeliveryPlan{s.plan}, nil
}
func (s *applicationStub) GetPlanDetail(context.Context, contract.ActorContext, contract.ProjectID, string) (delivery.PlanDetail, error) {
	return delivery.PlanDetail{Plan: s.plan}, nil
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
