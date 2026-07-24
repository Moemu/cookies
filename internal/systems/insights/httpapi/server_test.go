package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/insights"
)

func TestInsightsHTTPExposesReportExperienceAndPreLaunchLoop(t *testing.T) {
	t.Parallel()
	app := &applicationStub{
		report:     insights.InsightReport{ID: "insightreport_1", Version: 1},
		experience: insights.Experience{ID: "experience_1"},
	}
	server := New(app)
	tests := []struct {
		method string
		path   string
		body   string
		status int
		want   string
	}{
		{http.MethodPost, "/api/insights/v1/projects/project_1/reports", `{"execution_id":"deliveryexecution_1","summary":"摘要","findings":["发现"]}`, 201, "insightreport_1"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/reports/insightreport_1:confirm", `{"expected_version":1}`, 200, "insightreport_1"},
		{http.MethodPost, "/api/insights/v1/projects/project_1/reports/insightreport_1:create-experience", `{"expected_report_version":1,"conclusion":"结论","conditions":[],"counterexamples":[]}`, 201, "experience_1"},
		{http.MethodGet, "/api/insights/v1/projects/project_1/prelaunch", "", 200, "experience_1"},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, authenticatedRequest(test.method, test.path, test.body))
		if response.Code != test.status || !strings.Contains(response.Body.String(), test.want) {
			t.Fatalf("%s status=%d body=%s", test.path, response.Code, response.Body.String())
		}
	}
}

func authenticatedRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	ctx := contract.WithRequestContext(request.Context(), contract.RequestContext{
		RequestID: "req_1", TraceID: "trace_1",
		Actor: contract.ActorContext{
			OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
			Scopes: []contract.Scope{insights.ScopeRead, insights.ScopeWrite, insights.ScopeConfirm},
		},
	})
	return request.WithContext(ctx)
}

type applicationStub struct {
	report     insights.InsightReport
	experience insights.Experience
}

func (s *applicationStub) CreateReport(context.Context, contract.ActorContext, contract.ProjectID, insights.CreateReportRequest) (insights.InsightReport, error) {
	return s.report, nil
}
func (s *applicationStub) ListReports(context.Context, contract.ActorContext, contract.ProjectID, int) ([]insights.InsightReport, error) {
	return []insights.InsightReport{s.report}, nil
}
func (s *applicationStub) ConfirmReport(context.Context, contract.ActorContext, contract.ProjectID, string, int64) (insights.InsightReport, error) {
	return s.report, nil
}
func (s *applicationStub) CreateExperience(context.Context, contract.ActorContext, contract.ProjectID, string, int64, insights.CreateExperienceRequest) (insights.Experience, error) {
	return s.experience, nil
}
func (s *applicationStub) ListExperiences(context.Context, contract.ActorContext, contract.ProjectID, int) ([]insights.Experience, error) {
	return []insights.Experience{s.experience}, nil
}
func (s *applicationStub) GetPreLaunch(context.Context, contract.ActorContext, contract.ProjectID) (insights.PreLaunchInsight, error) {
	return insights.PreLaunchInsight{ExperienceReferences: []insights.Experience{s.experience}}, nil
}
func (s *applicationStub) GetPerformance(context.Context, contract.ActorContext, contract.ProjectID) (insights.PerformanceOverview, error) {
	return insights.PerformanceOverview{}, nil
}
