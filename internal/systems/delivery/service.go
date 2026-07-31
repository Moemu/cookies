// Package delivery owns versioned advertising plans and their controlled,
// auditable execution. The MVP executor is deliberately a local simulation.
package delivery

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/ids"
)

const (
	ScopeRead    contract.Scope = "delivery.read"
	ScopeWrite   contract.Scope = "delivery.write"
	ScopeApprove contract.Scope = "delivery.approve"
	ScopeExecute contract.Scope = "delivery.execute"
)

var (
	ErrNotFound            = errors.New("delivery resource not found")
	ErrInvalidRequest      = errors.New("delivery request is invalid")
	ErrInvalidState        = errors.New("delivery resource is not in a state that allows this action")
	ErrVersionConflict     = errors.New("delivery resource version conflict")
	ErrPlanVersionConflict = errors.New("delivery plan version conflict")
	ErrStalePlanVersion    = errors.New("delivery change set references a stale plan version")
)

type DeliveryPlanStatus string

const DeliveryPlanDraft DeliveryPlanStatus = "draft"

type ChangeSetStatus string

const (
	ChangeSetDraft           ChangeSetStatus = "draft"
	ChangeSetPreflightPassed ChangeSetStatus = "preflight_passed"
	ChangeSetPreflightFailed ChangeSetStatus = "preflight_failed"
	ChangeSetApproved        ChangeSetStatus = "approved"
	ChangeSetRejected        ChangeSetStatus = "rejected"
	ChangeSetExecuted        ChangeSetStatus = "executed"
	ChangeSetRolledBack      ChangeSetStatus = "rolled_back"
)

const ExecutionModeLocalSimulation = "local_simulation"

const (
	DemoMetricDatasetVersion = "preroll-demo/v1"
	MetricSourceDemoFixture  = "demo_fixture"
)

// CreatePlanRequest accepts the #21 package-oriented fields and the mock
// lifecycle draft fields. A request using PlanDraft is always explicitly mock.
type CreatePlanRequest struct {
	CreativePackageID string    `json:"creative_package_id,omitempty"`
	BudgetCents       int64     `json:"budget_cents,omitempty"`
	StartAt           time.Time `json:"start_at,omitempty"`
	EndAt             time.Time `json:"end_at,omitempty"`
	PlanDraft
}

func (r CreatePlanRequest) usesLifecycleDraft() bool {
	return r.PlanDraft.Advertiser.ID != "" || r.PlanDraft.Schedule.Timezone != "" ||
		r.PlanDraft.Budget.Currency != "" || len(r.PlanDraft.CreativeReferences) > 0
}

func (r CreatePlanRequest) Validate() error {
	if r.usesLifecycleDraft() {
		return r.PlanDraft.Validate()
	}
	if strings.TrimSpace(r.CreativePackageID) == "" || strings.TrimSpace(r.Name) == "" ||
		strings.TrimSpace(r.Objective) == "" || r.BudgetCents < 0 || r.StartAt.IsZero() ||
		r.EndAt.IsZero() || !r.EndAt.After(r.StartAt) {
		return ErrInvalidRequest
	}
	if len(r.Name) > 160 || len(r.Objective) > 1000 {
		return ErrInvalidRequest
	}
	return nil
}

type CreativePackageSnapshot struct {
	ID                string `json:"id"`
	CreativeVersionID string `json:"creative_version_id"`
	ContentHash       string `json:"content_hash"`
}

// DeliveryPlan remains the #21 current projection and also exposes the
// immutable lifecycle snapshots needed by the plan editor.
type DeliveryPlan struct {
	ID                   string                  `json:"id"`
	OrganizationID       contract.OrganizationID `json:"organization_id"`
	ProjectID            contract.ProjectID      `json:"project_id"`
	CreativePackageID    string                  `json:"creative_package_id"`
	CreativePackageHash  string                  `json:"creative_package_hash"`
	CreativeVersionID    string                  `json:"creative_version_id"`
	Name                 string                  `json:"name"`
	Objective            string                  `json:"objective"`
	BudgetCents          int64                   `json:"budget_cents"`
	StartAt              time.Time               `json:"start_at"`
	EndAt                time.Time               `json:"end_at"`
	Status               DeliveryPlanStatus      `json:"status"`
	Version              int64                   `json:"version"`
	Platform             string                  `json:"platform"`
	Source               Source                  `json:"source"`
	Scenario             Scenario                `json:"scenario"`
	CurrentVersionNumber int                     `json:"current_version_number"`
	CurrentVersion       DeliveryPlanVersion     `json:"current_version"`
	Versions             []DeliveryPlanVersion   `json:"versions"`
	CreatedBy            string                  `json:"created_by"`
	CreatedAt            time.Time               `json:"created_at"`
	UpdatedAt            time.Time               `json:"updated_at"`
}

type ChangeSet struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	PlanID         string                  `json:"plan_id"`
	PlanVersion    int64                   `json:"plan_version"`
	Status         ChangeSetStatus         `json:"status"`
	RiskLevel      string                  `json:"risk_level"`
	PreflightNotes []string                `json:"preflight_notes"`
	ApprovedBy     string                  `json:"approved_by,omitempty"`
	ApprovedAt     *time.Time              `json:"approved_at,omitempty"`
	Version        int64                   `json:"version"`
	CreatedBy      string                  `json:"created_by"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

type Execution struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	ChangeSetID    string                  `json:"change_set_id"`
	Status         string                  `json:"status"`
	Mode           string                  `json:"mode"`
	ExecutedBy     string                  `json:"executed_by"`
	StartedAt      time.Time               `json:"started_at"`
	CompletedAt    time.Time               `json:"completed_at"`
}

type Evidence struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	ExecutionID    string                  `json:"execution_id"`
	Summary        string                  `json:"summary"`
	Mode           string                  `json:"mode"`
	Reversible     bool                    `json:"reversible"`
	CreatedAt      time.Time               `json:"created_at"`
}

type ExecutionResult struct {
	ChangeSet ChangeSet `json:"change_set"`
	Execution Execution `json:"execution"`
	Evidence  Evidence  `json:"evidence"`
}

type RawMetrics struct {
	Impressions int64 `json:"impressions"`
	Clicks      int64 `json:"clicks"`
	Conversions int64 `json:"conversions"`
	SpendCents  int64 `json:"spend_cents"`
}

type DeliveryMetricSnapshot struct {
	ID                string                  `json:"id"`
	OrganizationID    contract.OrganizationID `json:"organization_id"`
	ProjectID         contract.ProjectID      `json:"project_id"`
	ExecutionID       string                  `json:"execution_id"`
	PlanID            string                  `json:"plan_id"`
	CreativePackageID string                  `json:"creative_package_id"`
	Source            string                  `json:"source"`
	IsSimulated       bool                    `json:"is_simulated"`
	DatasetVersion    string                  `json:"dataset_version"`
	Currency          string                  `json:"currency"`
	WindowStart       time.Time               `json:"window_start"`
	WindowEnd         time.Time               `json:"window_end"`
	RawMetrics        RawMetrics              `json:"raw_metrics"`
	CreatedBy         string                  `json:"created_by"`
	CreatedAt         time.Time               `json:"created_at"`
}

type CreateMetricSnapshotRequest struct {
	DatasetVersion string `json:"dataset_version"`
}

func (r CreateMetricSnapshotRequest) Validate() error {
	if strings.TrimSpace(r.DatasetVersion) != DemoMetricDatasetVersion {
		return ErrInvalidRequest
	}
	return nil
}

type PlanDetail struct {
	Plan       DeliveryPlan      `json:"plan"`
	ChangeSets []ChangeSet       `json:"change_sets"`
	Executions []ExecutionResult `json:"executions"`
}

type ActiveProjectResolver interface {
	RequireActiveContext(context.Context, contract.ActorContext, contract.ProjectID) (contract.ProjectContext, error)
}

type CreativePackageReader interface {
	ReadCreativePackage(context.Context, contract.ActorContext, contract.ProjectID, string) (CreativePackageSnapshot, error)
}

type Repository interface {
	CreatePlan(context.Context, DeliveryPlan, DeliveryPlanVersion) (DeliveryPlan, error)
	UpdatePlan(context.Context, contract.OrganizationID, contract.ProjectID, string, int, DeliveryPlanVersion) (DeliveryPlan, error)
	ListPlans(context.Context, contract.OrganizationID, contract.ProjectID, int) ([]DeliveryPlan, error)
	GetPlan(context.Context, contract.OrganizationID, contract.ProjectID, string) (DeliveryPlan, error)
	ListPlanVersions(context.Context, contract.OrganizationID, contract.ProjectID, string) ([]DeliveryPlanVersion, error)
	GetPlanVersion(context.Context, contract.OrganizationID, contract.ProjectID, string, int) (DeliveryPlanVersion, error)
	CreateChangeSet(context.Context, ChangeSet) (ChangeSet, error)
	ListChangeSets(context.Context, contract.OrganizationID, contract.ProjectID, int) ([]ChangeSet, error)
	GetChangeSet(context.Context, contract.OrganizationID, contract.ProjectID, string) (ChangeSet, error)
	TransitionChangeSet(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, ChangeSetStatus, string, time.Time) (ChangeSet, error)
	RecordExecution(context.Context, ChangeSet, Execution, Evidence) (ExecutionResult, error)
	ListExecutions(context.Context, contract.OrganizationID, contract.ProjectID, int) ([]ExecutionResult, error)
	CreateMetricSnapshot(context.Context, DeliveryMetricSnapshot) (DeliveryMetricSnapshot, bool, error)
	ListMetricSnapshots(context.Context, contract.OrganizationID, contract.ProjectID, string, int) ([]DeliveryMetricSnapshot, error)
}

type Service struct {
	Repository Repository
	Projects   ActiveProjectResolver
	Packages   CreativePackageReader
	NewID      ids.Generator
	Now        func() time.Time
}

func (s Service) CreatePlan(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreatePlanRequest) (DeliveryPlan, error) {
	if err := s.ready(actor, projectID, ScopeWrite); err != nil {
		return DeliveryPlan{}, err
	}
	if err := request.Validate(); err != nil {
		return DeliveryPlan{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return DeliveryPlan{}, err
	}
	id, err := s.idGenerator()("deliveryplan")
	if err != nil {
		return DeliveryPlan{}, err
	}
	now := s.now()
	draft, pkg, err := s.createDraftAndPackage(ctx, actor, projectID, request)
	if err != nil {
		return DeliveryPlan{}, err
	}
	plan := DeliveryPlan{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		CreativePackageID: pkg.ID, CreativePackageHash: pkg.ContentHash, CreativeVersionID: pkg.CreativeVersionID,
		Name: draft.Name, Objective: draft.Objective, BudgetCents: draft.Budget.TotalMinor,
		StartAt: draft.Schedule.StartAt, EndAt: draft.Schedule.EndAt,
		Status: DeliveryPlanDraft, Version: 1, Platform: "ocean_engine_mock", Source: SourceMock,
		Scenario: scenarioFor(draft), CurrentVersionNumber: 1,
		CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now,
	}
	version := versionFromDraft(plan, 1, draft, actor.Principal, now)
	return s.Repository.CreatePlan(ctx, plan, version)
}

func (s Service) createDraftAndPackage(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreatePlanRequest) (PlanDraft, CreativePackageSnapshot, error) {
	if request.usesLifecycleDraft() {
		draft := normalizeDraft(request.PlanDraft, scenarioFor(request.PlanDraft))
		reference := CreativeReference{AssetID: "mock-unset", Version: 1}
		if len(draft.CreativeReferences) > 0 {
			reference = draft.CreativeReferences[0]
		}
		pkg := CreativePackageSnapshot{
			ID: reference.AssetID, CreativeVersionID: strconv.Itoa(reference.Version),
			ContentHash: fmt.Sprintf("mock:%s@%d", reference.AssetID, reference.Version),
		}
		return draft, pkg, nil
	}
	if s.Packages == nil {
		return PlanDraft{}, CreativePackageSnapshot{}, fmt.Errorf("delivery creative package reader is required")
	}
	pkg, err := s.Packages.ReadCreativePackage(ctx, actor, projectID, request.CreativePackageID)
	if err != nil {
		return PlanDraft{}, CreativePackageSnapshot{}, err
	}
	draft := PlanDraft{
		Name: request.Name, Objective: request.Objective,
		Advertiser:         AdvertiserInput{ID: "mock-advertiser-001", Name: "Cookies Mock 广告主", Platform: "ocean_engine"},
		Budget:             Budget{TotalMinor: request.BudgetCents, Currency: "CNY"},
		Schedule:           Schedule{StartAt: request.StartAt, EndAt: request.EndAt, Timezone: "Asia/Shanghai"},
		Tracking:           Tracking{LandingPage: "https://demo.cookies.local", PixelID: "PX-LOCAL", ConversionEvent: "conversion"},
		CreativeReferences: []CreativeReference{{AssetID: pkg.ID, Version: 1, Confirmed: true}},
	}
	return normalizeDraft(draft, scenarioFor(draft)), pkg, nil
}

func (s Service) UpdatePlan(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, planID string, request UpdatePlanRequest) (DeliveryPlan, error) {
	if err := s.ready(actor, projectID, ScopeWrite); err != nil {
		return DeliveryPlan{}, err
	}
	if err := request.Validate(); err != nil || strings.TrimSpace(planID) == "" {
		return DeliveryPlan{}, ErrInvalidRequest
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return DeliveryPlan{}, err
	}
	plan, err := s.Repository.GetPlan(ctx, actor.OrganizationID, projectID, planID)
	if err != nil {
		return DeliveryPlan{}, err
	}
	if plan.Status != DeliveryPlanDraft {
		return DeliveryPlan{}, ErrInvalidState
	}
	version := versionFromDraft(plan, request.ExpectedVersion+1, request.PlanDraft, actor.Principal, s.now())
	return s.Repository.UpdatePlan(ctx, actor.OrganizationID, projectID, planID, request.ExpectedVersion, version)
}

func (s Service) ListPlans(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]DeliveryPlan, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return nil, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.Repository.ListPlans(ctx, actor.OrganizationID, projectID, normalizeLimit(limit))
}

func (s Service) GetPlan(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, planID string) (DeliveryPlan, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return DeliveryPlan{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return DeliveryPlan{}, err
	}
	return s.Repository.GetPlan(ctx, actor.OrganizationID, projectID, planID)
}

func (s Service) ListPlanVersions(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, planID string) ([]DeliveryPlanVersion, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return nil, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.Repository.ListPlanVersions(ctx, actor.OrganizationID, projectID, planID)
}

func (s Service) GetPlanVersion(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, planID string, version int) (DeliveryPlanVersion, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return DeliveryPlanVersion{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return DeliveryPlanVersion{}, err
	}
	return s.Repository.GetPlanVersion(ctx, actor.OrganizationID, projectID, planID, version)
}

func (s Service) RunPlanPreflight(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, planID string) (PreflightResult, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return PreflightResult{}, err
	}
	plan, err := s.GetPlan(ctx, actor, projectID, planID)
	if err != nil {
		return PreflightResult{}, err
	}
	checks := RunPreflight(plan.CurrentVersion)
	return preflightResult(plan.ID, plan.CurrentVersion, checks, s.now()), nil
}

func preflightResult(planID string, version DeliveryPlanVersion, checks []PreflightCheck, checkedAt time.Time) PreflightResult {
	blocked := false
	for _, check := range checks {
		if !check.Passed && check.Severity == CheckSeverityError {
			blocked = true
			break
		}
	}
	return PreflightResult{
		PlanID: planID, PlanVersion: version.VersionNumber, Passed: !blocked, Blocked: blocked,
		Checks: checks, Source: SourceMock, Scenario: version.Scenario, CheckedAt: checkedAt,
	}
}

func (s Service) GetPlanDetail(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, planID string) (PlanDetail, error) {
	plan, err := s.GetPlan(ctx, actor, projectID, planID)
	if err != nil {
		return PlanDetail{}, err
	}
	changeSets, err := s.Repository.ListChangeSets(ctx, actor.OrganizationID, projectID, 100)
	if err != nil {
		return PlanDetail{}, err
	}
	filtered := make([]ChangeSet, 0)
	for _, value := range changeSets {
		if value.PlanID == planID {
			filtered = append(filtered, value)
		}
	}
	executions, err := s.Repository.ListExecutions(ctx, actor.OrganizationID, projectID, 100)
	if err != nil {
		return PlanDetail{}, err
	}
	filteredExecutions := make([]ExecutionResult, 0)
	for _, value := range executions {
		for _, changeSet := range filtered {
			if value.Execution.ChangeSetID == changeSet.ID {
				filteredExecutions = append(filteredExecutions, value)
			}
		}
	}
	return PlanDetail{Plan: plan, ChangeSets: filtered, Executions: filteredExecutions}, nil
}

func (s Service) CreateChangeSet(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, planID string, expectedPlanVersion int64) (ChangeSet, error) {
	if err := s.ready(actor, projectID, ScopeWrite); err != nil {
		return ChangeSet{}, err
	}
	plan, err := s.Repository.GetPlan(ctx, actor.OrganizationID, projectID, planID)
	if err != nil {
		return ChangeSet{}, err
	}
	if plan.Version != expectedPlanVersion {
		return ChangeSet{}, ErrVersionConflict
	}
	id, err := s.idGenerator()("deliverychangeset")
	if err != nil {
		return ChangeSet{}, err
	}
	now := s.now()
	return s.Repository.CreateChangeSet(ctx, ChangeSet{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID, PlanID: plan.ID,
		PlanVersion: plan.Version, Status: ChangeSetDraft, RiskLevel: "low",
		PreflightNotes: []string{}, Version: 1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now,
	})
}

func (s Service) Preflight(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, changeSetID string, expectedVersion int64) (ChangeSet, error) {
	if err := s.ready(actor, projectID, ScopeWrite); err != nil {
		return ChangeSet{}, err
	}
	value, err := s.Repository.GetChangeSet(ctx, actor.OrganizationID, projectID, changeSetID)
	if err != nil {
		return ChangeSet{}, err
	}
	if value.Status != ChangeSetDraft {
		return ChangeSet{}, ErrInvalidState
	}
	plan, err := s.Repository.GetPlan(ctx, actor.OrganizationID, projectID, value.PlanID)
	if err != nil {
		return ChangeSet{}, err
	}
	if plan.Version != value.PlanVersion {
		return ChangeSet{}, ErrStalePlanVersion
	}
	version, err := s.Repository.GetPlanVersion(ctx, actor.OrganizationID, projectID, value.PlanID, int(value.PlanVersion))
	if err != nil {
		return ChangeSet{}, err
	}
	next := ChangeSetPreflightPassed
	for _, check := range RunPreflight(version) {
		if !check.Passed && check.Severity == CheckSeverityError {
			next = ChangeSetPreflightFailed
			break
		}
	}
	return s.Repository.TransitionChangeSet(ctx, actor.OrganizationID, projectID, changeSetID, expectedVersion, next, actor.Principal.ID, s.now())
}

func (s Service) Approve(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, changeSetID string, expectedVersion int64) (ChangeSet, error) {
	if err := s.ready(actor, projectID, ScopeApprove); err != nil {
		return ChangeSet{}, err
	}
	value, err := s.Repository.GetChangeSet(ctx, actor.OrganizationID, projectID, changeSetID)
	if err != nil {
		return ChangeSet{}, err
	}
	if value.Status != ChangeSetPreflightPassed {
		return ChangeSet{}, ErrInvalidState
	}
	return s.Repository.TransitionChangeSet(ctx, actor.OrganizationID, projectID, changeSetID, expectedVersion, ChangeSetApproved, actor.Principal.ID, s.now())
}

func (s Service) Execute(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, changeSetID string, expectedVersion int64) (ExecutionResult, error) {
	if err := s.ready(actor, projectID, ScopeExecute); err != nil {
		return ExecutionResult{}, err
	}
	value, err := s.Repository.GetChangeSet(ctx, actor.OrganizationID, projectID, changeSetID)
	if err != nil {
		return ExecutionResult{}, err
	}
	if value.Version != expectedVersion {
		return ExecutionResult{}, ErrVersionConflict
	}
	if value.Status != ChangeSetApproved {
		return ExecutionResult{}, ErrInvalidState
	}
	plan, err := s.Repository.GetPlan(ctx, actor.OrganizationID, projectID, value.PlanID)
	if err != nil {
		return ExecutionResult{}, err
	}
	if plan.Version != value.PlanVersion {
		return ExecutionResult{}, ErrStalePlanVersion
	}
	executionID, err := s.idGenerator()("deliveryexecution")
	if err != nil {
		return ExecutionResult{}, err
	}
	evidenceID, err := s.idGenerator()("deliveryevidence")
	if err != nil {
		return ExecutionResult{}, err
	}
	now := s.now()
	execution := Execution{
		ID: executionID, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		ChangeSetID: value.ID, Status: "succeeded", Mode: ExecutionModeLocalSimulation,
		ExecutedBy: actor.Principal.ID, StartedAt: now, CompletedAt: now,
	}
	evidence := Evidence{
		ID: evidenceID, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		ExecutionID: execution.ID, Summary: "本地模拟执行完成，无真实广告平台写入。",
		Mode: ExecutionModeLocalSimulation, Reversible: true, CreatedAt: now,
	}
	return s.Repository.RecordExecution(ctx, value, execution, evidence)
}

func (s Service) Rollback(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, changeSetID string, expectedVersion int64) (ChangeSet, error) {
	if err := s.ready(actor, projectID, ScopeExecute); err != nil {
		return ChangeSet{}, err
	}
	value, err := s.Repository.GetChangeSet(ctx, actor.OrganizationID, projectID, changeSetID)
	if err != nil {
		return ChangeSet{}, err
	}
	if value.Status != ChangeSetExecuted {
		return ChangeSet{}, ErrInvalidState
	}
	return s.Repository.TransitionChangeSet(ctx, actor.OrganizationID, projectID, changeSetID, expectedVersion, ChangeSetRolledBack, actor.Principal.ID, s.now())
}

func (s Service) ListExecutions(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]ExecutionResult, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return nil, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.Repository.ListExecutions(ctx, actor.OrganizationID, projectID, normalizeLimit(limit))
}

func (s Service) CreateDemoMetricSnapshot(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, executionID string, request CreateMetricSnapshotRequest) (DeliveryMetricSnapshot, error) {
	if err := s.ready(actor, projectID, ScopeWrite); err != nil {
		return DeliveryMetricSnapshot{}, err
	}
	if strings.TrimSpace(executionID) == "" {
		return DeliveryMetricSnapshot{}, ErrInvalidRequest
	}
	if err := request.Validate(); err != nil {
		return DeliveryMetricSnapshot{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return DeliveryMetricSnapshot{}, err
	}
	execution, err := s.findExecution(ctx, actor.OrganizationID, projectID, executionID)
	if err != nil {
		return DeliveryMetricSnapshot{}, err
	}
	if execution.Execution.Mode != ExecutionModeLocalSimulation || execution.Execution.Status != "succeeded" {
		return DeliveryMetricSnapshot{}, ErrInvalidState
	}
	plan, err := s.Repository.GetPlan(ctx, actor.OrganizationID, projectID, execution.ChangeSet.PlanID)
	if err != nil {
		return DeliveryMetricSnapshot{}, err
	}
	id, err := s.idGenerator()("deliverymetric")
	if err != nil {
		return DeliveryMetricSnapshot{}, err
	}
	value := DeliveryMetricSnapshot{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		ExecutionID: execution.Execution.ID, PlanID: plan.ID, CreativePackageID: plan.CreativePackageID,
		Source: MetricSourceDemoFixture, IsSimulated: true, DatasetVersion: DemoMetricDatasetVersion,
		Currency: "CNY", WindowStart: plan.StartAt, WindowEnd: plan.EndAt,
		RawMetrics: RawMetrics{Impressions: 10000, Clicks: 420, Conversions: 31, SpendCents: 50000},
		CreatedBy:  actor.Principal.ID, CreatedAt: s.now(),
	}
	stored, _, err := s.Repository.CreateMetricSnapshot(ctx, value)
	return stored, err
}

func (s Service) ListMetricSnapshots(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, executionID string, limit int) ([]DeliveryMetricSnapshot, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return nil, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(executionID) == "" {
		return nil, ErrInvalidRequest
	}
	return s.Repository.ListMetricSnapshots(ctx, actor.OrganizationID, projectID, executionID, normalizeLimit(limit))
}

func (s Service) ListExecutionEvidence(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]ExecutionResult, error) {
	if s.Repository == nil || s.Projects == nil {
		return nil, fmt.Errorf("delivery evidence dependencies are incomplete")
	}
	if actor.OrganizationID == "" || projectID == "" {
		return nil, fmt.Errorf("organization and project are required")
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.Repository.ListExecutions(ctx, actor.OrganizationID, projectID, normalizeLimit(limit))
}

func (s Service) ReadExecutionEvidence(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, executionID string) (ExecutionResult, *DeliveryMetricSnapshot, DeliveryPlan, error) {
	if s.Repository == nil || s.Projects == nil {
		return ExecutionResult{}, nil, DeliveryPlan{}, fmt.Errorf("delivery evidence dependencies are incomplete")
	}
	if actor.OrganizationID == "" || projectID == "" || strings.TrimSpace(executionID) == "" {
		return ExecutionResult{}, nil, DeliveryPlan{}, ErrInvalidRequest
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return ExecutionResult{}, nil, DeliveryPlan{}, err
	}
	execution, err := s.findExecution(ctx, actor.OrganizationID, projectID, executionID)
	if err != nil {
		return ExecutionResult{}, nil, DeliveryPlan{}, err
	}
	plan, err := s.Repository.GetPlan(ctx, actor.OrganizationID, projectID, execution.ChangeSet.PlanID)
	if err != nil {
		return ExecutionResult{}, nil, DeliveryPlan{}, err
	}
	values, err := s.Repository.ListMetricSnapshots(ctx, actor.OrganizationID, projectID, executionID, 1)
	if err != nil {
		return ExecutionResult{}, nil, DeliveryPlan{}, err
	}
	if len(values) == 0 {
		return execution, nil, plan, nil
	}
	return execution, &values[0], plan, nil
}

func (s Service) findExecution(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, executionID string) (ExecutionResult, error) {
	values, err := s.Repository.ListExecutions(ctx, organizationID, projectID, 100)
	if err != nil {
		return ExecutionResult{}, err
	}
	for _, value := range values {
		if value.Execution.ID == executionID {
			return value, nil
		}
	}
	return ExecutionResult{}, ErrNotFound
}

func (s Service) ready(actor contract.ActorContext, projectID contract.ProjectID, scope contract.Scope) error {
	if s.Repository == nil || s.Projects == nil {
		return fmt.Errorf("delivery dependencies are incomplete")
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
