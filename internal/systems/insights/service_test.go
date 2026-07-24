package insights

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestConfirmedReportBecomesReusableExperienceAndPreLaunchReference(t *testing.T) {
	t.Parallel()
	service := testService()
	actor := testActor()
	report, err := service.CreateReport(context.Background(), actor, "project_1", CreateReportRequest{
		ExecutionID: "deliveryexecution_1",
		Summary:     "蓝色主视觉获得更高的模拟点击意向",
		Findings:    []string{"首图信息密度适中", "标题利益点明确"},
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err = service.ConfirmReport(context.Background(), actor, "project_1", report.ID, report.Version)
	if err != nil {
		t.Fatal(err)
	}
	experience, err := service.CreateExperience(context.Background(), actor, "project_1", report.ID, report.Version, CreateExperienceRequest{
		Conclusion:      "面对新品种草时，首图保持单一利益点。",
		Conditions:      []string{"小红书图文", "新品首发"},
		Counterexamples: []string{"复杂参数对比内容"},
	})
	if err != nil {
		t.Fatal(err)
	}
	preLaunch, err := service.GetPreLaunch(context.Background(), actor, "project_1")
	if err != nil {
		t.Fatal(err)
	}
	if experience.Status != ExperienceConfirmed || len(preLaunch.ExperienceReferences) != 1 ||
		preLaunch.ExperienceReferences[0].ID != experience.ID {
		t.Fatalf("experience=%#v prelaunch=%#v", experience, preLaunch)
	}
}

func TestInsightsRejectsExperienceFromUnconfirmedReport(t *testing.T) {
	t.Parallel()
	service := testService()
	actor := testActor()
	report, _ := service.CreateReport(context.Background(), actor, "project_1", CreateReportRequest{
		ExecutionID: "deliveryexecution_1", Summary: "摘要", Findings: []string{"发现"},
	})
	if _, err := service.CreateExperience(context.Background(), actor, "project_1", report.ID, report.Version, CreateExperienceRequest{
		Conclusion: "结论", Conditions: []string{}, Counterexamples: []string{},
	}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("error=%v", err)
	}
}

func testActor() contract.ActorContext {
	return contract.ActorContext{
		OrganizationID: "org_1",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
		Scopes:         []contract.Scope{ScopeRead, ScopeWrite, ScopeConfirm},
	}
}

func testService() Service {
	sequence := 0
	return Service{
		Repository: &memoryRepository{reports: map[string]InsightReport{}, experiences: map[string]Experience{}},
		Projects:   testProjects{},
		Delivery:   testDelivery{},
		Now:        func() time.Time { return time.Date(2026, 7, 24, 13, 0, 0, 0, time.UTC) },
		NewID: func(prefix string) (string, error) {
			sequence++
			return fmt.Sprintf("%s_%d", prefix, sequence), nil
		},
	}
}

type testProjects struct{}

func (testProjects) RequireActiveContext(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID) (contract.ProjectContext, error) {
	return contract.ProjectContext{OrganizationID: actor.OrganizationID, ProjectID: projectID, ProjectContextVersion: 1}, nil
}

type testDelivery struct{}

func (testDelivery) ReadExecution(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, id string) (DeliveryExecutionSnapshot, error) {
	return DeliveryExecutionSnapshot{
		ID: id, ChangeSetID: "deliverychangeset_1", PlanID: "deliveryplan_1",
		Mode: "local_simulation", EvidenceID: "deliveryevidence_1",
		EvidenceSummary: "本地模拟执行完成，无真实广告平台写入。",
	}, nil
}

func (testDelivery) ListExecutions(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, _ int) ([]DeliveryExecutionSnapshot, error) {
	value, _ := (testDelivery{}).ReadExecution(context.Background(), contract.ActorContext{}, "", "deliveryexecution_1")
	return []DeliveryExecutionSnapshot{value}, nil
}

type memoryRepository struct {
	reports     map[string]InsightReport
	experiences map[string]Experience
}

func (r *memoryRepository) CreateReport(_ context.Context, value InsightReport) (InsightReport, error) {
	r.reports[value.ID] = value
	return value, nil
}
func (r *memoryRepository) ListReports(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, _ int) ([]InsightReport, error) {
	values := make([]InsightReport, 0)
	for _, value := range r.reports {
		if value.OrganizationID == organizationID && value.ProjectID == projectID {
			values = append(values, value)
		}
	}
	return values, nil
}
func (r *memoryRepository) GetReport(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (InsightReport, error) {
	value, ok := r.reports[id]
	if !ok || value.OrganizationID != organizationID || value.ProjectID != projectID {
		return InsightReport{}, ErrNotFound
	}
	return value, nil
}
func (r *memoryRepository) ConfirmReport(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string, expectedVersion int64, actorID string, now time.Time) (InsightReport, error) {
	value, err := r.GetReport(context.Background(), organizationID, projectID, id)
	if err != nil {
		return InsightReport{}, err
	}
	if value.Version != expectedVersion {
		return InsightReport{}, ErrVersionConflict
	}
	value.Status = ReportConfirmed
	value.Version++
	value.ConfirmedBy = actorID
	value.ConfirmedAt = &now
	value.UpdatedAt = now
	r.reports[id] = value
	return value, nil
}
func (r *memoryRepository) CreateExperience(_ context.Context, value Experience) (Experience, error) {
	r.experiences[value.ID] = value
	return value, nil
}
func (r *memoryRepository) ListExperiences(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, _ int) ([]Experience, error) {
	values := make([]Experience, 0)
	for _, value := range r.experiences {
		if value.OrganizationID == organizationID && value.ProjectID == projectID {
			values = append(values, value)
		}
	}
	return values, nil
}
func (r *memoryRepository) GetExperience(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (Experience, error) {
	value, ok := r.experiences[id]
	if !ok || value.OrganizationID != organizationID || value.ProjectID != projectID {
		return Experience{}, ErrNotFound
	}
	return value, nil
}
