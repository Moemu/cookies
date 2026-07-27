// Package insights owns evidence-backed reports and reusable experience.
package insights

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/ids"
)

const (
	ScopeRead    contract.Scope = "insights.read"
	ScopeWrite   contract.Scope = "insights.write"
	ScopeConfirm contract.Scope = "insights.confirm"
)

var (
	ErrNotFound        = errors.New("insights resource not found")
	ErrInvalidRequest  = errors.New("insights request is invalid")
	ErrInvalidState    = errors.New("insights resource is not in a state that allows this action")
	ErrVersionConflict = errors.New("insights resource version conflict")
)

type ReportStatus string

const (
	ReportDraft     ReportStatus = "draft"
	ReportConfirmed ReportStatus = "confirmed"
)

type ExperienceStatus string

const ExperienceConfirmed ExperienceStatus = "confirmed"

type DeliveryExecutionSnapshot struct {
	ID                string                  `json:"id"`
	ChangeSetID       string                  `json:"change_set_id"`
	PlanID            string                  `json:"plan_id"`
	CreativePackageID string                  `json:"creative_package_id"`
	Mode              string                  `json:"mode"`
	EvidenceID        string                  `json:"evidence_id"`
	EvidenceSummary   string                  `json:"evidence_summary"`
	MetricSnapshot    *DeliveryMetricSnapshot `json:"metric_snapshot,omitempty"`
}

type RawMetrics struct {
	Impressions int64 `json:"impressions"`
	Clicks      int64 `json:"clicks"`
	Conversions int64 `json:"conversions"`
	SpendCents  int64 `json:"spend_cents"`
}

type DeliveryMetricSnapshot struct {
	ID             string     `json:"id"`
	Source         string     `json:"source"`
	IsSimulated    bool       `json:"is_simulated"`
	DatasetVersion string     `json:"dataset_version"`
	Currency       string     `json:"currency"`
	RawMetrics     RawMetrics `json:"raw_metrics"`
}

type CreateReportRequest struct {
	ExecutionID string   `json:"execution_id"`
	Summary     string   `json:"summary"`
	Findings    []string `json:"findings"`
}

func (r CreateReportRequest) Validate() error {
	if strings.TrimSpace(r.ExecutionID) == "" {
		return ErrInvalidRequest
	}
	return nil
}

type InsightReport struct {
	ID                string                  `json:"id"`
	OrganizationID    contract.OrganizationID `json:"organization_id"`
	ProjectID         contract.ProjectID      `json:"project_id"`
	ExecutionID       string                  `json:"execution_id"`
	DeliveryMode      string                  `json:"delivery_mode"`
	EvidenceID        string                  `json:"evidence_id"`
	EvidenceSummary   string                  `json:"evidence_summary"`
	MetricSnapshotID  string                  `json:"metric_snapshot_id"`
	CreativePackageID string                  `json:"creative_package_id"`
	IsSimulated       bool                    `json:"is_simulated"`
	DatasetVersion    string                  `json:"dataset_version"`
	Status            ReportStatus            `json:"status"`
	Summary           string                  `json:"summary"`
	Findings          []string                `json:"findings"`
	Version           int64                   `json:"version"`
	CreatedBy         string                  `json:"created_by"`
	ConfirmedBy       string                  `json:"confirmed_by,omitempty"`
	ConfirmedAt       *time.Time              `json:"confirmed_at,omitempty"`
	CreatedAt         time.Time               `json:"created_at"`
	UpdatedAt         time.Time               `json:"updated_at"`
}

type CreateExperienceRequest struct {
	Conclusion      string   `json:"conclusion"`
	Conditions      []string `json:"conditions"`
	Counterexamples []string `json:"counterexamples"`
}

func (r CreateExperienceRequest) Validate() error {
	if strings.TrimSpace(r.Conclusion) == "" || len(r.Conclusion) > 2000 ||
		len(r.Conditions) > 20 || len(r.Counterexamples) > 20 {
		return ErrInvalidRequest
	}
	for _, values := range [][]string{r.Conditions, r.Counterexamples} {
		for _, value := range values {
			if strings.TrimSpace(value) == "" || len(value) > 500 {
				return ErrInvalidRequest
			}
		}
	}
	return nil
}

type Experience struct {
	ID                     string                  `json:"id"`
	OrganizationID         contract.OrganizationID `json:"organization_id"`
	ProjectID              contract.ProjectID      `json:"project_id"`
	ReportID               string                  `json:"report_id"`
	SourceExecutionID      string                  `json:"source_execution_id"`
	SourceEvidenceID       string                  `json:"source_evidence_id"`
	SourceMetricSnapshotID string                  `json:"source_metric_snapshot_id"`
	Conclusion             string                  `json:"conclusion"`
	Conditions             []string                `json:"conditions"`
	Counterexamples        []string                `json:"counterexamples"`
	Status                 ExperienceStatus        `json:"status"`
	Version                int64                   `json:"version"`
	CreatedBy              string                  `json:"created_by"`
	CreatedAt              time.Time               `json:"created_at"`
	UpdatedAt              time.Time               `json:"updated_at"`
}

type PreLaunchInsight struct {
	ProjectID            contract.ProjectID `json:"project_id"`
	ExperienceReferences []Experience       `json:"experience_references"`
	Disclosure           string             `json:"disclosure"`
}

type PerformanceOverview struct {
	ProjectID  contract.ProjectID          `json:"project_id"`
	Executions []DeliveryExecutionSnapshot `json:"executions"`
	Disclosure string                      `json:"disclosure"`
}

type ActiveProjectResolver interface {
	RequireActiveContext(context.Context, contract.ActorContext, contract.ProjectID) (contract.ProjectContext, error)
}

// DeliveryReader is Insights' only dependency on Delivery.
type DeliveryReader interface {
	ReadExecution(context.Context, contract.ActorContext, contract.ProjectID, string) (DeliveryExecutionSnapshot, error)
	ListExecutions(context.Context, contract.ActorContext, contract.ProjectID, int) ([]DeliveryExecutionSnapshot, error)
}

type Repository interface {
	CreateReport(context.Context, InsightReport) (InsightReport, error)
	ListReports(context.Context, contract.OrganizationID, contract.ProjectID, int) ([]InsightReport, error)
	GetReport(context.Context, contract.OrganizationID, contract.ProjectID, string) (InsightReport, error)
	ConfirmReport(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, string, time.Time) (InsightReport, error)
	CreateExperience(context.Context, Experience) (Experience, error)
	ListExperiences(context.Context, contract.OrganizationID, contract.ProjectID, int) ([]Experience, error)
	GetExperience(context.Context, contract.OrganizationID, contract.ProjectID, string) (Experience, error)
}

type Service struct {
	Repository Repository
	Projects   ActiveProjectResolver
	Delivery   DeliveryReader
	NewID      ids.Generator
	Now        func() time.Time
}

func (s Service) CreateReport(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreateReportRequest) (InsightReport, error) {
	if err := s.ready(actor, projectID, ScopeWrite); err != nil {
		return InsightReport{}, err
	}
	if err := request.Validate(); err != nil {
		return InsightReport{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return InsightReport{}, err
	}
	execution, err := s.Delivery.ReadExecution(ctx, actor, projectID, request.ExecutionID)
	if err != nil {
		return InsightReport{}, err
	}
	if execution.MetricSnapshot == nil || !execution.MetricSnapshot.IsSimulated ||
		execution.MetricSnapshot.Source != "demo_fixture" {
		return InsightReport{}, ErrInvalidState
	}
	id, err := s.idGenerator()("insightreport")
	if err != nil {
		return InsightReport{}, err
	}
	now := s.now()
	summary, findings := summarizeSimulatedMetrics(*execution.MetricSnapshot)
	return s.Repository.CreateReport(ctx, InsightReport{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		ExecutionID: execution.ID, DeliveryMode: execution.Mode, EvidenceID: execution.EvidenceID,
		EvidenceSummary:  execution.EvidenceSummary,
		MetricSnapshotID: execution.MetricSnapshot.ID, CreativePackageID: execution.CreativePackageID,
		IsSimulated: true, DatasetVersion: execution.MetricSnapshot.DatasetVersion, Status: ReportDraft,
		Summary: summary, Findings: findings,
		Version: 1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now,
	})
}

func (s Service) ListReports(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]InsightReport, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return nil, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.Repository.ListReports(ctx, actor.OrganizationID, projectID, normalizeLimit(limit))
}

func (s Service) ConfirmReport(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, reportID string, expectedVersion int64) (InsightReport, error) {
	if err := s.ready(actor, projectID, ScopeConfirm); err != nil {
		return InsightReport{}, err
	}
	value, err := s.Repository.GetReport(ctx, actor.OrganizationID, projectID, reportID)
	if err != nil {
		return InsightReport{}, err
	}
	if value.Status != ReportDraft {
		return InsightReport{}, ErrInvalidState
	}
	return s.Repository.ConfirmReport(ctx, actor.OrganizationID, projectID, reportID, expectedVersion, actor.Principal.ID, s.now())
}

func (s Service) CreateExperience(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, reportID string, expectedReportVersion int64, request CreateExperienceRequest) (Experience, error) {
	if err := s.ready(actor, projectID, ScopeWrite); err != nil {
		return Experience{}, err
	}
	if err := request.Validate(); err != nil {
		return Experience{}, err
	}
	report, err := s.Repository.GetReport(ctx, actor.OrganizationID, projectID, reportID)
	if err != nil {
		return Experience{}, err
	}
	if report.Version != expectedReportVersion {
		return Experience{}, ErrVersionConflict
	}
	if report.Status != ReportConfirmed {
		return Experience{}, ErrInvalidState
	}
	id, err := s.idGenerator()("experience")
	if err != nil {
		return Experience{}, err
	}
	now := s.now()
	return s.Repository.CreateExperience(ctx, Experience{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID, ReportID: report.ID,
		SourceExecutionID: report.ExecutionID, SourceEvidenceID: report.EvidenceID,
		SourceMetricSnapshotID: report.MetricSnapshotID,
		Conclusion:             strings.TrimSpace(request.Conclusion), Conditions: append([]string{}, request.Conditions...),
		Counterexamples: append([]string{}, request.Counterexamples...), Status: ExperienceConfirmed,
		Version: 1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now,
	})
}

func summarizeSimulatedMetrics(snapshot DeliveryMetricSnapshot) (string, []string) {
	metrics := snapshot.RawMetrics
	ctr := ratioPercent(metrics.Clicks, metrics.Impressions)
	cvr := ratioPercent(metrics.Conversions, metrics.Clicks)
	summary := fmt.Sprintf(
		"基于本地模拟投放数据集 %s 生成复盘：曝光 %d、点击 %d、转化 %d、模拟花费 %.2f %s。该结果仅用于验证产品闭环，不代表真实广告平台效果。",
		snapshot.DatasetVersion, metrics.Impressions, metrics.Clicks, metrics.Conversions,
		float64(metrics.SpendCents)/100, snapshot.Currency,
	)
	return summary, []string{
		fmt.Sprintf("模拟点击率 CTR 为 %.2f%%（%d/%d）。", ctr, metrics.Clicks, metrics.Impressions),
		fmt.Sprintf("模拟点击转化率 CVR 为 %.2f%%（%d/%d）。", cvr, metrics.Conversions, metrics.Clicks),
		"所有指标来源均为 demo_fixture 模拟数据，不得用于判断真实广告效果或对外宣称。",
	}
}

func ratioPercent(numerator, denominator int64) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator) * 100
}

func (s Service) ListExperiences(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]Experience, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return nil, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.Repository.ListExperiences(ctx, actor.OrganizationID, projectID, normalizeLimit(limit))
}

func (s Service) GetPreLaunch(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) (PreLaunchInsight, error) {
	values, err := s.ListExperiences(ctx, actor, projectID, 100)
	if err != nil {
		return PreLaunchInsight{}, err
	}
	return PreLaunchInsight{
		ProjectID: projectID, ExperienceReferences: values,
		Disclosure: "仅引用已确认经验；是否适用于本次投放仍需人工判断。",
	}, nil
}

func (s Service) GetPerformance(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) (PerformanceOverview, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return PerformanceOverview{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return PerformanceOverview{}, err
	}
	values, err := s.Delivery.ListExecutions(ctx, actor, projectID, 100)
	if err != nil {
		return PerformanceOverview{}, err
	}
	return PerformanceOverview{
		ProjectID: projectID, Executions: values,
		Disclosure: "当前为本地模拟执行证据，不代表真实广告平台效果数据。",
	}, nil
}

func (s Service) ready(actor contract.ActorContext, projectID contract.ProjectID, scope contract.Scope) error {
	if s.Repository == nil || s.Projects == nil || s.Delivery == nil {
		return fmt.Errorf("insights dependencies are incomplete")
	}
	if actor.OrganizationID == "" || projectID == "" || !actor.HasScope(scope) {
		return fmt.Errorf("%s scope is required", scope)
	}
	return nil
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s Service) idGenerator() ids.Generator {
	if s.NewID != nil {
		return s.NewID
	}
	return ids.New
}

func normalizeLimit(value int) int {
	if value < 1 || value > 100 {
		return 50
	}
	return value
}
