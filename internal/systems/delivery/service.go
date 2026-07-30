package delivery

import (
	"context"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/ids"
)

type Service struct {
	Store Store
	NewID ids.Generator
	Now   func() time.Time
}

func (s Service) CreatePlan(ctx context.Context, actor contract.ActorContext, request CreatePlanRequest) (DeliveryPlan, error) {
	if s.Store == nil {
		return DeliveryPlan{}, fmt.Errorf("delivery store is required")
	}
	if err := actor.Validate(); err != nil {
		return DeliveryPlan{}, err
	}
	if err := request.Validate(); err != nil {
		return DeliveryPlan{}, err
	}
	id, err := s.newID()("deliveryplan")
	if err != nil {
		return DeliveryPlan{}, err
	}
	now := s.now()
	scenario := scenarioFor(request.PlanDraft)
	plan := DeliveryPlan{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: request.ProjectID,
		Status: PlanStatusDraft, Platform: "ocean_engine_mock", Source: SourceMock,
		Scenario: scenario, CurrentVersionNumber: 1, CreatedBy: actor.Principal,
		CreatedAt: now, UpdatedAt: now,
	}
	version := versionFromDraft(plan, 1, request.PlanDraft, actor.Principal, now)
	return s.Store.Create(ctx, plan, version)
}

func (s Service) GetPlan(ctx context.Context, actor contract.ActorContext, planID string) (DeliveryPlan, error) {
	if s.Store == nil {
		return DeliveryPlan{}, fmt.Errorf("delivery store is required")
	}
	if err := actor.Validate(); err != nil {
		return DeliveryPlan{}, err
	}
	return s.Store.Get(ctx, actor.OrganizationID, planID)
}

func (s Service) ListPlans(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) ([]DeliveryPlan, error) {
	if s.Store == nil {
		return nil, fmt.Errorf("delivery store is required")
	}
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	return s.Store.List(ctx, actor.OrganizationID, projectID)
}

func (s Service) UpdatePlan(ctx context.Context, actor contract.ActorContext, planID string, request UpdatePlanRequest) (DeliveryPlan, error) {
	if s.Store == nil {
		return DeliveryPlan{}, fmt.Errorf("delivery store is required")
	}
	if err := actor.Validate(); err != nil {
		return DeliveryPlan{}, err
	}
	if err := request.Validate(); err != nil {
		return DeliveryPlan{}, err
	}
	plan, err := s.Store.Get(ctx, actor.OrganizationID, planID)
	if err != nil {
		return DeliveryPlan{}, err
	}
	now := s.now()
	next := versionFromDraft(plan, request.ExpectedVersion+1, request.PlanDraft, actor.Principal, now)
	return s.Store.Update(ctx, actor.OrganizationID, planID, request.ExpectedVersion, next)
}

func (s Service) PreflightPlan(ctx context.Context, actor contract.ActorContext, planID string) (PreflightResult, error) {
	plan, err := s.GetPlan(ctx, actor, planID)
	if err != nil {
		return PreflightResult{}, err
	}
	checks := RunPreflight(plan.CurrentVersion)
	blocked := false
	for _, item := range checks {
		if !item.Passed && item.Severity == CheckSeverityError {
			blocked = true
			break
		}
	}
	return PreflightResult{
		PlanID: plan.ID, PlanVersion: plan.CurrentVersionNumber,
		Passed: !blocked, Blocked: blocked, Checks: checks,
		Source: SourceMock, Scenario: plan.Scenario, CheckedAt: s.now(),
	}, nil
}

func (s Service) newID() ids.Generator {
	if s.NewID != nil {
		return s.NewID
	}
	return ids.New
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
