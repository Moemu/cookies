package insights

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	if experience.Status != ExperiencePending || experience.LineageID != experience.ID || experience.Revision != 1 {
		t.Fatalf("a deposited conclusion must start as 待确认: %#v", experience)
	}
	pending, err := service.GetPreLaunch(context.Background(), actor, "project_1", PreLaunchFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pending.ExperienceReferences) != 0 {
		t.Fatalf("unconfirmed experience must not be quotable: %#v", pending)
	}
	experience, err = service.ConfirmExperience(context.Background(), actor, "project_1", experience.ID, experience.Version)
	if err != nil {
		t.Fatal(err)
	}
	preLaunch, err := service.GetPreLaunch(context.Background(), actor, "project_1", PreLaunchFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if experience.Status != ExperienceConfirmed || len(preLaunch.ExperienceReferences) != 1 ||
		preLaunch.ExperienceReferences[0].ID != experience.ID {
		t.Fatalf("experience=%#v prelaunch=%#v", experience, preLaunch)
	}
}

func TestRetiredExperienceStopsBeingQuotableButStaysAuditable(t *testing.T) {
	t.Parallel()
	service := testService()
	actor := testActor()
	experience := confirmedExperience(t, service, actor)
	retired, err := service.RetireExperience(context.Background(), actor, "project_1", experience.ID, ExperienceTransitionRequest{
		ExpectedVersion: experience.Version, Reason: "平台口径变更，结论不再成立。",
	})
	if err != nil {
		t.Fatal(err)
	}
	preLaunch, err := service.GetPreLaunch(context.Background(), actor, "project_1", PreLaunchFilter{})
	if err != nil {
		t.Fatal(err)
	}
	audits, err := service.ListExperienceAudits(context.Background(), actor, "project_1", experience.ID, 50)
	if err != nil {
		t.Fatal(err)
	}
	if retired.Status != ExperienceRetired || len(preLaunch.ExperienceReferences) != 0 {
		t.Fatalf("retired=%#v prelaunch=%#v", retired, preLaunch)
	}
	if len(audits) != 3 || audits[2].FromStatus != ExperienceConfirmed || audits[2].ToStatus != ExperienceRetired ||
		audits[2].Reason != "平台口径变更，结论不再成立。" {
		t.Fatalf("retirement must leave an attributable trail: %#v", audits)
	}
	stored, err := service.ListExperiences(context.Background(), actor, "project_1", ExperienceRetired, 50)
	if err != nil || len(stored) != 1 {
		t.Fatalf("logical delete must keep the row readable: values=%#v err=%v", stored, err)
	}
}

func TestRetiringExperienceRequiresReasonAndConfirmScope(t *testing.T) {
	t.Parallel()
	service := testService()
	actor := testActor()
	experience := confirmedExperience(t, service, actor)
	if _, err := service.RetireExperience(context.Background(), actor, "project_1", experience.ID, ExperienceTransitionRequest{
		ExpectedVersion: experience.Version,
	}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("error=%v", err)
	}
	writer := testActor()
	writer.Scopes = []contract.Scope{ScopeRead, ScopeWrite}
	_, err := service.RetireExperience(context.Background(), writer, "project_1", experience.ID, ExperienceTransitionRequest{
		ExpectedVersion: experience.Version, Reason: "无确认权限也应被拒绝。",
	})
	if err == nil || !strings.Contains(err.Error(), string(ScopeConfirm)) {
		t.Fatalf("error=%v", err)
	}
}

func TestChallengedExperienceGoesToReviewInsteadOfSilentOverwrite(t *testing.T) {
	t.Parallel()
	service := testService()
	actor := testActor()
	experience := confirmedExperience(t, service, actor)
	review, err := service.RequestExperienceReview(context.Background(), actor, "project_1", experience.ID, ExperienceTransitionRequest{
		ExpectedVersion: experience.Version, Reason: "新一轮数据与该结论冲突。",
	})
	if err != nil {
		t.Fatal(err)
	}
	preLaunch, err := service.GetPreLaunch(context.Background(), actor, "project_1", PreLaunchFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if review.Status != ExperienceNeedsReview || len(preLaunch.ExperienceReferences) != 0 {
		t.Fatalf("review=%#v prelaunch=%#v", review, preLaunch)
	}
	back, err := service.ConfirmExperience(context.Background(), actor, "project_1", review.ID, review.Version)
	if err != nil || back.Status != ExperienceConfirmed {
		t.Fatalf("review must be resolvable back to confirmed: %#v err=%v", back, err)
	}
}

func TestRevisionSupersedesPredecessorOnlyAfterItIsConfirmed(t *testing.T) {
	t.Parallel()
	service := testService()
	actor := testActor()
	original := confirmedExperience(t, service, actor)
	revision, err := service.ReviseExperience(context.Background(), actor, "project_1", original.ID, ReviseExperienceRequest{
		ExpectedVersion: original.Version,
		Conclusion:      "面对新品种草时，首图保持单一利益点，并在标题重复该利益点。",
		Conditions:      []string{"小红书图文", "新品首发"},
		Counterexamples: []string{"复杂参数对比内容"},
		Reason:          "补充标题层面的适用条件。",
	})
	if err != nil {
		t.Fatal(err)
	}
	if revision.Revision != 2 || revision.LineageID != original.LineageID || revision.SupersedesID != original.ID ||
		revision.Status != ExperiencePending {
		t.Fatalf("revision=%#v original=%#v", revision, original)
	}
	stillOriginal, err := service.GetPreLaunch(context.Background(), actor, "project_1", PreLaunchFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(stillOriginal.ExperienceReferences) != 1 || stillOriginal.ExperienceReferences[0].ID != original.ID {
		t.Fatalf("an unconfirmed revision must not replace the live conclusion: %#v", stillOriginal)
	}
	confirmed, err := service.ConfirmExperience(context.Background(), actor, "project_1", revision.ID, revision.Version)
	if err != nil {
		t.Fatal(err)
	}
	quotable, err := service.GetPreLaunch(context.Background(), actor, "project_1", PreLaunchFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(quotable.ExperienceReferences) != 1 || quotable.ExperienceReferences[0].ID != confirmed.ID {
		t.Fatalf("confirming a revision must leave exactly one quotable conclusion: %#v", quotable)
	}
	lineage, err := service.ListExperienceLineage(context.Background(), actor, "project_1", confirmed.ID)
	if err != nil {
		t.Fatal(err)
	}
	superseded, err := service.ListExperiences(context.Background(), actor, "project_1", ExperienceRetired, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(lineage) != 2 || len(superseded) != 1 || superseded[0].ID != original.ID ||
		superseded[0].SupersededByID != confirmed.ID {
		t.Fatalf("lineage=%#v superseded=%#v", lineage, superseded)
	}
}

func TestReferenceFeedbackOnlyAttachesToQuotableExperience(t *testing.T) {
	t.Parallel()
	service := testService()
	actor := testActor()
	experience := confirmedExperience(t, service, actor)
	reference, err := service.RecordExperienceReference(context.Background(), actor, "project_1", experience.ID, RecordExperienceReferenceRequest{
		ConsumerKind: "creative_task", ConsumerID: "creativetask_1", Outcome: ReferenceAdopted,
		Note: "首图按该结论收敛为单一利益点。",
	})
	if err != nil {
		t.Fatal(err)
	}
	values, err := service.ListExperienceReferences(context.Background(), actor, "project_1", experience.ID, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].ID != reference.ID || values[0].Outcome != ReferenceAdopted {
		t.Fatalf("references=%#v", values)
	}
	if _, err := service.RetireExperience(context.Background(), actor, "project_1", experience.ID, ExperienceTransitionRequest{
		ExpectedVersion: experience.Version, Reason: "结论失效。",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordExperienceReference(context.Background(), actor, "project_1", experience.ID, RecordExperienceReferenceRequest{
		ConsumerKind: "creative_task", ConsumerID: "creativetask_2", Outcome: ReferenceReferenced,
	}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("error=%v", err)
	}
	kept, err := service.ListExperienceReferences(context.Background(), actor, "project_1", experience.ID, 50)
	if err != nil || len(kept) != 1 {
		t.Fatalf("existing reference history must survive retirement: %#v err=%v", kept, err)
	}
}

func TestProjectReferenceListSpansAllExperiences(t *testing.T) {
	t.Parallel()
	service := testService()
	actor := testActor()
	first := confirmedExperience(t, service, actor)
	second := confirmedExperience(t, service, actor)
	for _, item := range []struct {
		experienceID string
		consumerID   string
	}{{first.ID, "creativetask_1"}, {second.ID, "creativetask_2"}} {
		if _, err := service.RecordExperienceReference(context.Background(), actor, "project_1", item.experienceID, RecordExperienceReferenceRequest{
			ConsumerKind: "creative_task", ConsumerID: item.consumerID, Outcome: ReferenceAdopted,
		}); err != nil {
			t.Fatal(err)
		}
	}
	values, err := service.ListProjectExperienceReferences(context.Background(), actor, "project_1", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 {
		t.Fatalf("引用记录视图需要一次拿到全项目的引用: %#v", values)
	}
}

func TestExperienceTransitionRejectsStaleVersion(t *testing.T) {
	t.Parallel()
	service := testService()
	actor := testActor()
	experience := confirmedExperience(t, service, actor)
	if _, err := service.RetireExperience(context.Background(), actor, "project_1", experience.ID, ExperienceTransitionRequest{
		ExpectedVersion: experience.Version - 1, Reason: "版本已过期。",
	}); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("error=%v", err)
	}
}

func confirmedExperience(t *testing.T, service Service, actor contract.ActorContext) Experience {
	t.Helper()
	report, err := service.CreateReport(context.Background(), actor, "project_1", CreateReportRequest{ExecutionID: "deliveryexecution_1"})
	if err != nil {
		t.Fatal(err)
	}
	report, err = service.ConfirmReport(context.Background(), actor, "project_1", report.ID, report.Version)
	if err != nil {
		t.Fatal(err)
	}
	experience, err := service.CreateExperience(context.Background(), actor, "project_1", report.ID, report.Version, CreateExperienceRequest{
		Conclusion:      "面对新品种草时，首图保持单一利益点。",
		Conditions:      []string{"小红书图文"},
		Counterexamples: []string{"复杂参数对比内容"},
	})
	if err != nil {
		t.Fatal(err)
	}
	experience, err = service.ConfirmExperience(context.Background(), actor, "project_1", experience.ID, experience.Version)
	if err != nil {
		t.Fatal(err)
	}
	return experience
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

func TestCreateReportDerivesFindingsFromSimulatedMetricSnapshot(t *testing.T) {
	t.Parallel()
	service := testService()
	report, err := service.CreateReport(context.Background(), testActor(), "project_1", CreateReportRequest{
		ExecutionID: "deliveryexecution_1",
		Summary:     "client supplied summary must be ignored",
		Findings:    []string{"client supplied finding must be ignored"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.IsSimulated || report.MetricSnapshotID != "deliverymetric_1" ||
		report.CreativePackageID != "creativepackage_1" {
		t.Fatalf("report lineage=%#v", report)
	}
	if strings.Contains(report.Summary, "client supplied") || strings.Contains(strings.Join(report.Findings, " "), "client supplied") {
		t.Fatalf("report must be server-derived: %#v", report)
	}
	if len(report.Digest) != 0 || report.WindowStart != "" || report.WindowEnd != "" {
		t.Fatalf("不带窗口的报告不该有汇总：%#v", report)
	}
}

// 带窗口的报告必须真的把三处结论取回来。取不到时要报错，不能悄悄建成一份空报告——
// 人在投后分析页点的是「定格这一屏」，拿到一份没有内容的报告比拿到错误更难发现。
func TestCreateReportWithWindowFailsRatherThanFreezingNothing(t *testing.T) {
	t.Parallel()
	service := testService() // 没有 Connectors / Assets / Experiments
	_, err := service.CreateReport(context.Background(), testActor(), "project_1", CreateReportRequest{
		ExecutionID: "deliveryexecution_1",
		Window: MetricWindow{
			Start: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC),
		},
	})
	if err == nil {
		t.Fatal("窗口分析取不到时，报告不该创建成功")
	}
}

func TestDropReportFindingMarksInsteadOfRemoving(t *testing.T) {
	t.Parallel()
	repository := &memoryRepository{reports: map[string]InsightReport{}, experiences: map[string]Experience{}}
	service := testService()
	service.Repository = repository
	actor := testActor()

	report, err := service.CreateReport(context.Background(), actor, "project_1", CreateReportRequest{
		ExecutionID: "deliveryexecution_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	// 直接塞进仓储：这条用例要验的是删减本身，不是汇总怎么算出来的。
	seeded := repository.reports[report.ID]
	seeded.Digest = []ReportFinding{
		{Kind: SectionAssetPerformance, Text: "首图单一利益点的组明显更好。"},
		{Kind: SectionExperiment, Text: "实验 A 支持这一点。"},
	}
	repository.reports[report.ID] = seeded

	dropped, err := service.DropReportFinding(context.Background(), actor, "project_1", report.ID, seeded.Version, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(dropped.Digest) != 2 || !dropped.Digest[1].Dropped || dropped.Digest[0].Dropped {
		t.Fatalf("删减应只打标记不删条目：%#v", dropped.Digest)
	}
	if dropped.Version != seeded.Version+1 {
		t.Fatalf("version=%d", dropped.Version)
	}

	restored, err := service.DropReportFinding(context.Background(), actor, "project_1", report.ID, dropped.Version, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Digest[1].Dropped {
		t.Fatalf("放回去应清掉标记：%#v", restored.Digest)
	}

	if _, err := service.DropReportFinding(context.Background(), actor, "project_1", report.ID, restored.Version, 9, true); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("越界下标 error=%v", err)
	}
	if _, err := service.DropReportFinding(context.Background(), actor, "project_1", report.ID, restored.Version-1, 0, true); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("版本冲突 error=%v", err)
	}

	confirmed, err := service.ConfirmReport(context.Background(), actor, "project_1", report.ID, restored.Version)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.DropReportFinding(context.Background(), actor, "project_1", report.ID, confirmed.Version, 0, true); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("已确认的报告不该还能删减 error=%v", err)
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
		CreativePackageID: "creativepackage_1",
		MetricSnapshot: &DeliveryMetricSnapshot{
			ID: "deliverymetric_1", DatasetVersion: "preroll-demo/v1", Source: "demo_fixture",
			IsSimulated: true, Currency: "CNY",
			RawMetrics: RawMetrics{Impressions: 10000, Clicks: 420, Conversions: 31, SpendCents: 50000},
		},
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
	order       []string
	references  []ExperienceReference
	audits      []ExperienceAudit
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
func (r *memoryRepository) UpdateReportDigest(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string, expectedVersion int64, digest []ReportFinding, now time.Time) (InsightReport, error) {
	value, err := r.GetReport(context.Background(), organizationID, projectID, id)
	if err != nil {
		return InsightReport{}, err
	}
	if value.Version != expectedVersion {
		return InsightReport{}, ErrVersionConflict
	}
	if value.Status != ReportDraft {
		return InsightReport{}, ErrInvalidState
	}
	value.Digest = digest
	value.Version++
	value.UpdatedAt = now
	r.reports[id] = value
	return value, nil
}
func (r *memoryRepository) CreateExperience(_ context.Context, value Experience, audit ExperienceAudit) (Experience, error) {
	r.experiences[value.ID] = value
	r.order = append(r.order, value.ID)
	r.audits = append(r.audits, audit)
	return value, nil
}
func (r *memoryRepository) ListExperiences(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, status ExperienceStatus, _ int) ([]Experience, error) {
	values := make([]Experience, 0)
	for _, id := range r.order {
		value := r.experiences[id]
		if value.OrganizationID != organizationID || value.ProjectID != projectID {
			continue
		}
		if status != "" && value.Status != status {
			continue
		}
		values = append(values, value)
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
func (r *memoryRepository) ListExperienceLineage(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, lineageID string) ([]Experience, error) {
	values := make([]Experience, 0)
	for _, id := range r.order {
		value := r.experiences[id]
		if value.OrganizationID == organizationID && value.ProjectID == projectID && value.LineageID == lineageID {
			values = append(values, value)
		}
	}
	return values, nil
}
func (r *memoryRepository) TransitionExperience(ctx context.Context, input TransitionExperienceInput) (Experience, error) {
	value, err := r.GetExperience(ctx, input.OrganizationID, input.ProjectID, input.ID)
	if err != nil {
		return Experience{}, err
	}
	if value.Version != input.ExpectedVersion {
		return Experience{}, ErrVersionConflict
	}
	if !containsStatus(input.From, value.Status) {
		return Experience{}, ErrInvalidState
	}
	r.audits = append(r.audits, ExperienceAudit{
		ID: input.AuditID, OrganizationID: input.OrganizationID, ProjectID: input.ProjectID,
		ExperienceID: value.ID, FromStatus: value.Status, ToStatus: input.To,
		Reason: input.Reason, ActorID: input.ActorID, CreatedAt: input.Now,
	})
	value.Status = input.To
	value.StatusReason = input.Reason
	value.StatusChangedBy = input.ActorID
	value.StatusChangedAt = &input.Now
	value.Version++
	value.UpdatedAt = input.Now
	r.experiences[value.ID] = value
	return value, nil
}
func (r *memoryRepository) ConfirmExperience(ctx context.Context, input ConfirmExperienceInput) (Experience, error) {
	value, err := r.TransitionExperience(ctx, TransitionExperienceInput{
		OrganizationID: input.OrganizationID, ProjectID: input.ProjectID, ID: input.ID,
		ExpectedVersion: input.ExpectedVersion,
		From:            []ExperienceStatus{ExperiencePending, ExperienceNeedsReview},
		To:              ExperienceConfirmed, ActorID: input.ActorID, Now: input.Now, AuditID: input.AuditID,
	})
	if err != nil {
		return Experience{}, err
	}
	value.ConfirmedBy = input.ActorID
	value.ConfirmedAt = &input.Now
	r.experiences[value.ID] = value
	if value.SupersedesID == "" {
		return value, nil
	}
	previous, err := r.GetExperience(ctx, input.OrganizationID, input.ProjectID, value.SupersedesID)
	if err != nil || previous.Status == ExperienceRetired {
		return value, nil
	}
	superseded, err := r.TransitionExperience(ctx, TransitionExperienceInput{
		OrganizationID: input.OrganizationID, ProjectID: input.ProjectID, ID: previous.ID,
		ExpectedVersion: previous.Version,
		From:            []ExperienceStatus{ExperiencePending, ExperienceConfirmed, ExperienceNeedsReview},
		To:              ExperienceRetired, Reason: "已被第 " + strconv.Itoa(value.Revision) + " 版取代。",
		ActorID: input.ActorID, Now: input.Now, AuditID: input.SupersedeAuditID,
	})
	if err != nil {
		return Experience{}, err
	}
	superseded.SupersededByID = value.ID
	r.experiences[superseded.ID] = superseded
	return value, nil
}
func (r *memoryRepository) CreateExperienceReference(_ context.Context, value ExperienceReference) (ExperienceReference, error) {
	r.references = append(r.references, value)
	return value, nil
}
func (r *memoryRepository) ListExperienceReferences(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, experienceID string, _ int) ([]ExperienceReference, error) {
	values := make([]ExperienceReference, 0)
	for _, value := range r.references {
		if value.OrganizationID != organizationID || value.ProjectID != projectID {
			continue
		}
		// 空 experienceID 表示"整个项目的引用记录"，与 MySQL 实现保持一致。
		if experienceID != "" && value.ExperienceID != experienceID {
			continue
		}
		values = append(values, value)
	}
	return values, nil
}
func (r *memoryRepository) ListExperienceAudits(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, experienceID string, _ int) ([]ExperienceAudit, error) {
	values := make([]ExperienceAudit, 0)
	for _, value := range r.audits {
		if value.OrganizationID == organizationID && value.ProjectID == projectID && value.ExperienceID == experienceID {
			values = append(values, value)
		}
	}
	return values, nil
}

func containsStatus(values []ExperienceStatus, value ExperienceStatus) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
