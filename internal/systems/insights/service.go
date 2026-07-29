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

// ExperienceStatus follows the lifecycle in the asset management PRD §11.1:
// 待确认 -> 已确认 -> 待复审 -> 已失效. Retirement is logical, so a referenced
// conclusion stays auditable after it stops being reusable.
type ExperienceStatus string

const (
	ExperiencePending     ExperienceStatus = "pending"
	ExperienceConfirmed   ExperienceStatus = "confirmed"
	ExperienceNeedsReview ExperienceStatus = "needs_review"
	ExperienceRetired     ExperienceStatus = "retired"
)

func (s ExperienceStatus) valid() bool {
	switch s {
	case ExperiencePending, ExperienceConfirmed, ExperienceNeedsReview, ExperienceRetired:
		return true
	}
	return false
}

// ExperienceReferenceOutcome records what a downstream consumer did with a
// confirmed experience (AM-014).
type ExperienceReferenceOutcome string

const (
	ReferenceReferenced ExperienceReferenceOutcome = "referenced"
	ReferenceAdopted    ExperienceReferenceOutcome = "adopted"
	ReferenceModified   ExperienceReferenceOutcome = "modified"
	ReferenceRejected   ExperienceReferenceOutcome = "rejected"
)

func (o ExperienceReferenceOutcome) valid() bool {
	switch o {
	case ReferenceReferenced, ReferenceAdopted, ReferenceModified, ReferenceRejected:
		return true
	}
	return false
}

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

	// 洞察卡剩下的五个字段（03 §8.1）。类型和置信有默认值，是为了让老调用方
	// 不填也能过——但默认落在最保守的一格（假设 / 方向性），不替录入的人做判断。
	CardType          InsightCardType `json:"card_type"`
	Confidence        ConfidenceLevel `json:"confidence"`
	RecommendedAction string          `json:"recommended_action"`
	Applicability     Applicability   `json:"applicability"`
	DataBasis         DataBasis       `json:"data_basis"`
	ContentBasis      ContentBasis    `json:"content_basis"`
}

// withCardDefaults 补上没填的类型和置信。放在这里而不是各调用点，
// 是为了让「不填等于假设 + 方向性」只有一处定义。
func (r CreateExperienceRequest) withCardDefaults() CreateExperienceRequest {
	if strings.TrimSpace(string(r.CardType)) == "" {
		r.CardType = CardHypothesis
	}
	if strings.TrimSpace(string(r.Confidence)) == "" {
		r.Confidence = ConfidenceDirectional
	}
	return r
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
	filled := r.withCardDefaults()
	if err := validateApplicability(filled.Applicability); err != nil {
		return err
	}
	if err := validateContentBasis(filled.ContentBasis); err != nil {
		return err
	}
	return validateCard(filled.CardType, filled.Confidence, filled.RecommendedAction, filled.DataBasis)
}

type Experience struct {
	ID                     string                  `json:"id"`
	OrganizationID         contract.OrganizationID `json:"organization_id"`
	ProjectID              contract.ProjectID      `json:"project_id"`
	LineageID              string                  `json:"lineage_id"`
	Revision               int                     `json:"revision"`
	SupersedesID           string                  `json:"supersedes_id,omitempty"`
	SupersededByID         string                  `json:"superseded_by_id,omitempty"`
	ReportID               string                  `json:"report_id"`
	SourceExecutionID      string                  `json:"source_execution_id"`
	SourceEvidenceID       string                  `json:"source_evidence_id"`
	SourceMetricSnapshotID string                  `json:"source_metric_snapshot_id"`
	Conclusion             string                  `json:"conclusion"`
	Conditions             []string                `json:"conditions"`
	Counterexamples        []string                `json:"counterexamples"`
	CardType               InsightCardType         `json:"card_type"`
	Confidence             ConfidenceLevel         `json:"confidence"`
	RecommendedAction      string                  `json:"recommended_action"`
	Applicability          Applicability           `json:"applicability"`
	DataBasis              DataBasis               `json:"data_basis"`
	ContentBasis           ContentBasis            `json:"content_basis"`
	Status                 ExperienceStatus        `json:"status"`
	StatusReason           string                  `json:"status_reason"`
	StatusChangedBy        string                  `json:"status_changed_by"`
	StatusChangedAt        *time.Time              `json:"status_changed_at,omitempty"`
	ConfirmedBy            string                  `json:"confirmed_by,omitempty"`
	ConfirmedAt            *time.Time              `json:"confirmed_at,omitempty"`
	Version                int64                   `json:"version"`
	CreatedBy              string                  `json:"created_by"`
	CreatedAt              time.Time               `json:"created_at"`
	UpdatedAt              time.Time               `json:"updated_at"`
}

// Reusable reports whether downstream Skills may quote this experience by
// default (MVP acceptance §15.10): confirmed and not retired.
func (e Experience) Reusable() bool { return e.Status == ExperienceConfirmed }

// ExperienceAudit is the append-only trail behind PRD §11.2. Nothing is
// physically deleted, so every status change stays attributable.
type ExperienceAudit struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	ExperienceID   string                  `json:"experience_id"`
	FromStatus     ExperienceStatus        `json:"from_status"`
	ToStatus       ExperienceStatus        `json:"to_status"`
	Reason         string                  `json:"reason"`
	ActorID        string                  `json:"actor_id"`
	CreatedAt      time.Time               `json:"created_at"`
}

type ExperienceReference struct {
	ID             string                     `json:"id"`
	OrganizationID contract.OrganizationID    `json:"organization_id"`
	ProjectID      contract.ProjectID         `json:"project_id"`
	ExperienceID   string                     `json:"experience_id"`
	ConsumerKind   string                     `json:"consumer_kind"`
	ConsumerID     string                     `json:"consumer_id"`
	Outcome        ExperienceReferenceOutcome `json:"outcome"`
	Note           string                     `json:"note"`
	Version        int64                      `json:"version"`
	CreatedBy      string                     `json:"created_by"`
	CreatedAt      time.Time                  `json:"created_at"`
	UpdatedAt      time.Time                  `json:"updated_at"`
}

// ExperienceTransitionRequest carries the human reason a conclusion moved
// state. Rejection, review and retirement all require one.
type ExperienceTransitionRequest struct {
	ExpectedVersion int64  `json:"expected_version"`
	Reason          string `json:"reason"`
}

func (r ExperienceTransitionRequest) validate(reasonRequired bool) error {
	reason := strings.TrimSpace(r.Reason)
	if len(reason) > 1000 || (reasonRequired && reason == "") {
		return ErrInvalidRequest
	}
	return nil
}

// ReviseExperienceRequest supersedes a conclusion instead of overwriting it
// (PRD §7.6). The revision starts as 待确认 and only replaces its predecessor
// once someone confirms it.
type ReviseExperienceRequest struct {
	ExpectedVersion int64    `json:"expected_version"`
	Conclusion      string   `json:"conclusion"`
	Conditions      []string `json:"conditions"`
	Counterexamples []string `json:"counterexamples"`
	Reason          string   `json:"reason"`

	CardType          InsightCardType `json:"card_type"`
	Confidence        ConfidenceLevel `json:"confidence"`
	RecommendedAction string          `json:"recommended_action"`
	Applicability     Applicability   `json:"applicability"`
	DataBasis         DataBasis       `json:"data_basis"`
	ContentBasis      ContentBasis    `json:"content_basis"`
}

// card 把修订请求折回创建请求，让两条路径共用同一套校验和默认值。
func (r ReviseExperienceRequest) card() CreateExperienceRequest {
	return CreateExperienceRequest{
		Conclusion: r.Conclusion, Conditions: r.Conditions, Counterexamples: r.Counterexamples,
		CardType: r.CardType, Confidence: r.Confidence, RecommendedAction: r.RecommendedAction,
		Applicability: r.Applicability, DataBasis: r.DataBasis, ContentBasis: r.ContentBasis,
	}.withCardDefaults()
}

func (r ReviseExperienceRequest) validate() error {
	if err := r.card().Validate(); err != nil {
		return err
	}
	if len(strings.TrimSpace(r.Reason)) > 1000 {
		return ErrInvalidRequest
	}
	return nil
}

type RecordExperienceReferenceRequest struct {
	ConsumerKind string                     `json:"consumer_kind"`
	ConsumerID   string                     `json:"consumer_id"`
	Outcome      ExperienceReferenceOutcome `json:"outcome"`
	Note         string                     `json:"note"`
}

func (r RecordExperienceReferenceRequest) validate() error {
	kind, id := strings.TrimSpace(r.ConsumerKind), strings.TrimSpace(r.ConsumerID)
	if kind == "" || len(kind) > 32 || id == "" || len(id) > 96 ||
		!r.Outcome.valid() || len(strings.TrimSpace(r.Note)) > 1000 {
		return ErrInvalidRequest
	}
	return nil
}

// PreLaunchInsight 与投前洞察的投影逻辑一起放在 prelaunch.go。

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

// TransitionExperienceInput moves one experience between lifecycle states and
// appends the matching audit row in the same transaction.
type TransitionExperienceInput struct {
	OrganizationID  contract.OrganizationID
	ProjectID       contract.ProjectID
	ID              string
	ExpectedVersion int64
	From            []ExperienceStatus
	To              ExperienceStatus
	Reason          string
	ActorID         string
	Now             time.Time
	AuditID         string
}

// ConfirmExperienceInput confirms a revision and, when it supersedes an older
// one, retires the predecessor atomically so two revisions are never reusable
// at the same time.
type ConfirmExperienceInput struct {
	OrganizationID   contract.OrganizationID
	ProjectID        contract.ProjectID
	ID               string
	ExpectedVersion  int64
	ActorID          string
	Now              time.Time
	AuditID          string
	SupersedeAuditID string
}

type Repository interface {
	CreateReport(context.Context, InsightReport) (InsightReport, error)
	ListReports(context.Context, contract.OrganizationID, contract.ProjectID, int) ([]InsightReport, error)
	GetReport(context.Context, contract.OrganizationID, contract.ProjectID, string) (InsightReport, error)
	ConfirmReport(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, string, time.Time) (InsightReport, error)
	CreateExperience(context.Context, Experience, ExperienceAudit) (Experience, error)
	ListExperiences(context.Context, contract.OrganizationID, contract.ProjectID, ExperienceStatus, int) ([]Experience, error)
	GetExperience(context.Context, contract.OrganizationID, contract.ProjectID, string) (Experience, error)
	ListExperienceLineage(context.Context, contract.OrganizationID, contract.ProjectID, string) ([]Experience, error)
	TransitionExperience(context.Context, TransitionExperienceInput) (Experience, error)
	ConfirmExperience(context.Context, ConfirmExperienceInput) (Experience, error)
	CreateExperienceReference(context.Context, ExperienceReference) (ExperienceReference, error)
	ListExperienceReferences(context.Context, contract.OrganizationID, contract.ProjectID, string, int) ([]ExperienceReference, error)
	ListExperienceAudits(context.Context, contract.OrganizationID, contract.ProjectID, string, int) ([]ExperienceAudit, error)
}

type Service struct {
	Repository Repository
	// Assets backs 分析素材库 and 内容分析. It is a separate interface from
	// Repository because the two lifecycles share nothing but the module.
	Assets AssetRepository
	// Connectors backs 数据接入 and the Canonical daily metrics 投后分析 reads.
	Connectors ConnectorRepository
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
	auditID, err := s.idGenerator()("experienceaudit")
	if err != nil {
		return Experience{}, err
	}
	now := s.now()
	card := request.withCardDefaults()
	// A newly deposited conclusion starts as 待确认. Only an explicit confirm
	// makes it quotable downstream.
	value := Experience{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		LineageID: id, Revision: 1, ReportID: report.ID,
		SourceExecutionID: report.ExecutionID, SourceEvidenceID: report.EvidenceID,
		SourceMetricSnapshotID: report.MetricSnapshotID,
		Conclusion:             strings.TrimSpace(request.Conclusion), Conditions: append([]string{}, request.Conditions...),
		Counterexamples: append([]string{}, request.Counterexamples...), Status: ExperiencePending,
		CardType: card.CardType, Confidence: card.Confidence,
		RecommendedAction: strings.TrimSpace(card.RecommendedAction),
		Applicability:     card.Applicability, DataBasis: card.DataBasis, ContentBasis: card.ContentBasis,
		StatusChangedBy: actor.Principal.ID, StatusChangedAt: &now,
		Version: 1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now,
	}
	return s.Repository.CreateExperience(ctx, value, ExperienceAudit{
		ID: auditID, OrganizationID: actor.OrganizationID, ProjectID: projectID, ExperienceID: id,
		FromStatus: "", ToStatus: ExperiencePending, Reason: "从已确认复盘报告沉淀，等待人工确认。",
		ActorID: actor.Principal.ID, CreatedAt: now,
	})
}

// ConfirmExperience promotes 待确认 or 待复审 to 已确认. Confirming a revision
// retires the predecessor it supersedes in the same transaction.
func (s Service) ConfirmExperience(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, experienceID string, expectedVersion int64) (Experience, error) {
	if err := s.ready(actor, projectID, ScopeConfirm); err != nil {
		return Experience{}, err
	}
	current, err := s.Repository.GetExperience(ctx, actor.OrganizationID, projectID, experienceID)
	if err != nil {
		return Experience{}, err
	}
	if current.Status != ExperiencePending && current.Status != ExperienceNeedsReview {
		return Experience{}, ErrInvalidState
	}
	auditID, err := s.idGenerator()("experienceaudit")
	if err != nil {
		return Experience{}, err
	}
	supersedeAuditID, err := s.idGenerator()("experienceaudit")
	if err != nil {
		return Experience{}, err
	}
	return s.Repository.ConfirmExperience(ctx, ConfirmExperienceInput{
		OrganizationID: actor.OrganizationID, ProjectID: projectID, ID: experienceID,
		ExpectedVersion: expectedVersion, ActorID: actor.Principal.ID, Now: s.now(),
		AuditID: auditID, SupersedeAuditID: supersedeAuditID,
	})
}

// RejectExperience discards a 待确认 conclusion without deleting the row.
func (s Service) RejectExperience(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, experienceID string, request ExperienceTransitionRequest) (Experience, error) {
	return s.transition(ctx, actor, projectID, experienceID, ScopeConfirm,
		[]ExperienceStatus{ExperiencePending}, ExperienceRetired, request, true)
}

// RequestExperienceReview flags a confirmed conclusion as 待复审 when new data
// challenges it, instead of silently overwriting it.
func (s Service) RequestExperienceReview(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, experienceID string, request ExperienceTransitionRequest) (Experience, error) {
	return s.transition(ctx, actor, projectID, experienceID, ScopeWrite,
		[]ExperienceStatus{ExperienceConfirmed}, ExperienceNeedsReview, request, true)
}

// RetireExperience is the logical delete in PRD §11.2: the row stays readable
// and its reference history stays auditable.
func (s Service) RetireExperience(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, experienceID string, request ExperienceTransitionRequest) (Experience, error) {
	return s.transition(ctx, actor, projectID, experienceID, ScopeConfirm,
		[]ExperienceStatus{ExperienceConfirmed, ExperienceNeedsReview}, ExperienceRetired, request, true)
}

func (s Service) transition(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, experienceID string, scope contract.Scope, from []ExperienceStatus, to ExperienceStatus, request ExperienceTransitionRequest, reasonRequired bool) (Experience, error) {
	if err := s.ready(actor, projectID, scope); err != nil {
		return Experience{}, err
	}
	if err := request.validate(reasonRequired); err != nil {
		return Experience{}, err
	}
	auditID, err := s.idGenerator()("experienceaudit")
	if err != nil {
		return Experience{}, err
	}
	return s.Repository.TransitionExperience(ctx, TransitionExperienceInput{
		OrganizationID: actor.OrganizationID, ProjectID: projectID, ID: experienceID,
		ExpectedVersion: request.ExpectedVersion, From: from, To: to,
		Reason: strings.TrimSpace(request.Reason), ActorID: actor.Principal.ID,
		Now: s.now(), AuditID: auditID,
	})
}

// ReviseExperience appends a new revision to the lineage rather than editing
// the confirmed conclusion in place.
func (s Service) ReviseExperience(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, experienceID string, request ReviseExperienceRequest) (Experience, error) {
	if err := s.ready(actor, projectID, ScopeWrite); err != nil {
		return Experience{}, err
	}
	if err := request.validate(); err != nil {
		return Experience{}, err
	}
	source, err := s.Repository.GetExperience(ctx, actor.OrganizationID, projectID, experienceID)
	if err != nil {
		return Experience{}, err
	}
	if source.Version != request.ExpectedVersion {
		return Experience{}, ErrVersionConflict
	}
	if source.Status == ExperienceRetired || source.SupersededByID != "" {
		return Experience{}, ErrInvalidState
	}
	lineage, err := s.Repository.ListExperienceLineage(ctx, actor.OrganizationID, projectID, source.LineageID)
	if err != nil {
		return Experience{}, err
	}
	next := source.Revision
	for _, item := range lineage {
		if item.Revision > next {
			next = item.Revision
		}
		// A lineage may hold only one open revision at a time.
		if item.Status == ExperiencePending && item.ID != source.ID {
			return Experience{}, ErrInvalidState
		}
	}
	id, err := s.idGenerator()("experience")
	if err != nil {
		return Experience{}, err
	}
	auditID, err := s.idGenerator()("experienceaudit")
	if err != nil {
		return Experience{}, err
	}
	now := s.now()
	reason := strings.TrimSpace(request.Reason)
	if reason == "" {
		reason = fmt.Sprintf("修订自 %s 第 %d 版。", source.ID, source.Revision)
	}
	card := request.card()
	value := Experience{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		LineageID: source.LineageID, Revision: next + 1, SupersedesID: source.ID,
		ReportID:          source.ReportID,
		SourceExecutionID: source.SourceExecutionID, SourceEvidenceID: source.SourceEvidenceID,
		SourceMetricSnapshotID: source.SourceMetricSnapshotID,
		Conclusion:             strings.TrimSpace(request.Conclusion), Conditions: append([]string{}, request.Conditions...),
		Counterexamples: append([]string{}, request.Counterexamples...), Status: ExperiencePending,
		CardType: card.CardType, Confidence: card.Confidence,
		RecommendedAction: strings.TrimSpace(card.RecommendedAction),
		Applicability:     card.Applicability, DataBasis: card.DataBasis, ContentBasis: card.ContentBasis,
		StatusReason: reason, StatusChangedBy: actor.Principal.ID, StatusChangedAt: &now,
		Version: 1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now,
	}
	return s.Repository.CreateExperience(ctx, value, ExperienceAudit{
		ID: auditID, OrganizationID: actor.OrganizationID, ProjectID: projectID, ExperienceID: id,
		FromStatus: "", ToStatus: ExperiencePending, Reason: reason,
		ActorID: actor.Principal.ID, CreatedAt: now,
	})
}

// RecordExperienceReference closes the AM-014 loop: a downstream consumer says
// whether it adopted, modified or rejected the quoted conclusion.
func (s Service) RecordExperienceReference(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, experienceID string, request RecordExperienceReferenceRequest) (ExperienceReference, error) {
	if err := s.ready(actor, projectID, ScopeWrite); err != nil {
		return ExperienceReference{}, err
	}
	if err := request.validate(); err != nil {
		return ExperienceReference{}, err
	}
	experience, err := s.Repository.GetExperience(ctx, actor.OrganizationID, projectID, experienceID)
	if err != nil {
		return ExperienceReference{}, err
	}
	if !experience.Reusable() {
		return ExperienceReference{}, ErrInvalidState
	}
	id, err := s.idGenerator()("experienceref")
	if err != nil {
		return ExperienceReference{}, err
	}
	now := s.now()
	return s.Repository.CreateExperienceReference(ctx, ExperienceReference{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID, ExperienceID: experience.ID,
		ConsumerKind: strings.TrimSpace(request.ConsumerKind), ConsumerID: strings.TrimSpace(request.ConsumerID),
		Outcome: request.Outcome, Note: strings.TrimSpace(request.Note),
		Version: 1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now,
	})
}

func (s Service) ListExperienceReferences(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, experienceID string, limit int) ([]ExperienceReference, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return nil, err
	}
	if _, err := s.Repository.GetExperience(ctx, actor.OrganizationID, projectID, experienceID); err != nil {
		return nil, err
	}
	return s.Repository.ListExperienceReferences(ctx, actor.OrganizationID, projectID, experienceID, normalizeLimit(limit))
}

// ListProjectExperienceReferences answers "which of our experiences were used
// downstream, and were they adopted" without walking every experience in turn.
func (s Service) ListProjectExperienceReferences(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]ExperienceReference, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return nil, err
	}
	return s.Repository.ListExperienceReferences(ctx, actor.OrganizationID, projectID, "", normalizeLimit(limit))
}

func (s Service) ListExperienceAudits(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, experienceID string, limit int) ([]ExperienceAudit, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return nil, err
	}
	if _, err := s.Repository.GetExperience(ctx, actor.OrganizationID, projectID, experienceID); err != nil {
		return nil, err
	}
	return s.Repository.ListExperienceAudits(ctx, actor.OrganizationID, projectID, experienceID, normalizeLimit(limit))
}

func (s Service) ListExperienceLineage(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, experienceID string) ([]Experience, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return nil, err
	}
	value, err := s.Repository.GetExperience(ctx, actor.OrganizationID, projectID, experienceID)
	if err != nil {
		return nil, err
	}
	return s.Repository.ListExperienceLineage(ctx, actor.OrganizationID, projectID, value.LineageID)
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

// ListExperiences returns every lifecycle state when status is empty, so the
// experience library can show 待确认 / 已确认 / 待复审 / 已失效 side by side.
func (s Service) ListExperiences(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, status ExperienceStatus, limit int) ([]Experience, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return nil, err
	}
	if status != "" && !status.valid() {
		return nil, ErrInvalidRequest
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.Repository.ListExperiences(ctx, actor.OrganizationID, projectID, status, normalizeLimit(limit))
}

// GetPreLaunch 在 prelaunch.go。

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
