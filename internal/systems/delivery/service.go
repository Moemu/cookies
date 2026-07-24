// Package delivery owns versioned advertising plans and their controlled,
// auditable execution. The MVP executor is deliberately a local simulation.
package delivery

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
	ScopeRead    contract.Scope = "delivery.read"
	ScopeWrite   contract.Scope = "delivery.write"
	ScopeApprove contract.Scope = "delivery.approve"
	ScopeExecute contract.Scope = "delivery.execute"
)

var (
	ErrNotFound        = errors.New("delivery resource not found")
	ErrInvalidRequest  = errors.New("delivery request is invalid")
	ErrInvalidState    = errors.New("delivery resource is not in a state that allows this action")
	ErrVersionConflict = errors.New("delivery resource version conflict")
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

type CreatePlanRequest struct {
	CreativePackageID string    `json:"creative_package_id"`
	Name              string    `json:"name"`
	Objective         string    `json:"objective"`
	BudgetCents       int64     `json:"budget_cents"`
	StartAt           time.Time `json:"start_at"`
	EndAt             time.Time `json:"end_at"`
}

func (r CreatePlanRequest) Validate() error {
	if strings.TrimSpace(r.CreativePackageID) == "" || strings.TrimSpace(r.Name) == "" ||
		strings.TrimSpace(r.Objective) == "" || r.BudgetCents <= 0 || r.StartAt.IsZero() ||
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

type DeliveryPlan struct {
	ID                  string                  `json:"id"`
	OrganizationID      contract.OrganizationID `json:"organization_id"`
	ProjectID           contract.ProjectID      `json:"project_id"`
	CreativePackageID   string                  `json:"creative_package_id"`
	CreativePackageHash string                  `json:"creative_package_hash"`
	CreativeVersionID   string                  `json:"creative_version_id"`
	Name                string                  `json:"name"`
	Objective           string                  `json:"objective"`
	BudgetCents         int64                   `json:"budget_cents"`
	StartAt             time.Time               `json:"start_at"`
	EndAt               time.Time               `json:"end_at"`
	Status              DeliveryPlanStatus      `json:"status"`
	Version             int64                   `json:"version"`
	CreatedBy           string                  `json:"created_by"`
	CreatedAt           time.Time               `json:"created_at"`
	UpdatedAt           time.Time               `json:"updated_at"`
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

type PlanDetail struct {
	Plan       DeliveryPlan      `json:"plan"`
	ChangeSets []ChangeSet       `json:"change_sets"`
	Executions []ExecutionResult `json:"executions"`
}

type ActiveProjectResolver interface {
	RequireActiveContext(context.Context, contract.ActorContext, contract.ProjectID) (contract.ProjectContext, error)
}

// CreativePackageReader is the only seam from Delivery to Creative.
type CreativePackageReader interface {
	ReadCreativePackage(context.Context, contract.ActorContext, contract.ProjectID, string) (CreativePackageSnapshot, error)
}

type Repository interface {
	CreatePlan(context.Context, DeliveryPlan) (DeliveryPlan, error)
	ListPlans(context.Context, contract.OrganizationID, contract.ProjectID, int) ([]DeliveryPlan, error)
	GetPlan(context.Context, contract.OrganizationID, contract.ProjectID, string) (DeliveryPlan, error)
	CreateChangeSet(context.Context, ChangeSet) (ChangeSet, error)
	ListChangeSets(context.Context, contract.OrganizationID, contract.ProjectID, int) ([]ChangeSet, error)
	GetChangeSet(context.Context, contract.OrganizationID, contract.ProjectID, string) (ChangeSet, error)
	TransitionChangeSet(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, ChangeSetStatus, string, time.Time) (ChangeSet, error)
	RecordExecution(context.Context, ChangeSet, Execution, Evidence) (ExecutionResult, error)
	ListExecutions(context.Context, contract.OrganizationID, contract.ProjectID, int) ([]ExecutionResult, error)
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
	pkg, err := s.Packages.ReadCreativePackage(ctx, actor, projectID, request.CreativePackageID)
	if err != nil {
		return DeliveryPlan{}, err
	}
	id, err := s.idGenerator()("deliveryplan")
	if err != nil {
		return DeliveryPlan{}, err
	}
	now := s.now()
	return s.Repository.CreatePlan(ctx, DeliveryPlan{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		CreativePackageID: pkg.ID, CreativePackageHash: pkg.ContentHash, CreativeVersionID: pkg.CreativeVersionID,
		Name: strings.TrimSpace(request.Name), Objective: strings.TrimSpace(request.Objective),
		BudgetCents: request.BudgetCents, StartAt: request.StartAt, EndAt: request.EndAt,
		Status: DeliveryPlanDraft, Version: 1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now,
	})
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

func (s Service) GetPlanDetail(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, planID string) (PlanDetail, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return PlanDetail{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return PlanDetail{}, err
	}
	plan, err := s.Repository.GetPlan(ctx, actor.OrganizationID, projectID, planID)
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
	if _, err := s.Repository.GetPlan(ctx, actor.OrganizationID, projectID, value.PlanID); err != nil {
		return ChangeSet{}, err
	}
	return s.Repository.TransitionChangeSet(ctx, actor.OrganizationID, projectID, changeSetID, expectedVersion, ChangeSetPreflightPassed, actor.Principal.ID, s.now())
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

// ListExecutionEvidence is the narrow internal projection used by the
// Delivery→Insights integration. The Insights service authorizes its own
// caller before using this method; Delivery still enforces tenant and active
// project boundaries here without requiring a second end-user Delivery scope.
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

func (s Service) ready(actor contract.ActorContext, projectID contract.ProjectID, scope contract.Scope) error {
	if s.Repository == nil || s.Projects == nil || s.Packages == nil {
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
