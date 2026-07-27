package remix

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

var ErrNotFound = errors.New("remix plan not found")

type Service struct {
	mu      sync.RWMutex
	plans   map[string]Plan
	renders map[string]RenderJob
	newID   func() (string, error)
	nowUTC  func() time.Time
}

func NewMemoryService(newID func() (string, error)) *Service {
	if newID == nil {
		newID = defaultID
	}
	return &Service{
		plans:   make(map[string]Plan),
		renders: make(map[string]RenderJob),
		newID:   newID,
		nowUTC:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) Create(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreatePlanRequest) (Plan, error) {
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	if err := request.Validate(); err != nil {
		return Plan{}, err
	}
	id, err := s.newID()
	if err != nil {
		return Plan{}, err
	}
	now := s.nowUTC()
	plan := Plan{
		ID:             id,
		OrganizationID: actor.OrganizationID,
		ProjectID:      projectID,
		CreatedBy:      actor.Principal,
		ClientPlanID:   request.ClientPlanID,
		TargetSeconds:  request.TargetSeconds,
		ActualSeconds:  request.ActualSeconds,
		Pace:           request.Pace,
		Segments:       cloneSegments(request.Segments),
		Warnings:       append([]string(nil), request.Warnings...),
		Summary:        request.Summary,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := plan.Validate(); err != nil {
		return Plan{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.plans[id] = plan
	return clonePlan(plan), nil
}

func (s *Service) Get(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (Plan, error) {
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	plan, ok := s.plans[id]
	if !ok || plan.OrganizationID != actor.OrganizationID || plan.ProjectID != projectID {
		return Plan{}, ErrNotFound
	}
	return clonePlan(plan), nil
}

func (s *Service) List(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]Plan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("limit must be between 1 and 100")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	plans := make([]Plan, 0, len(s.plans))
	for _, plan := range s.plans {
		if plan.OrganizationID == actor.OrganizationID && plan.ProjectID == projectID {
			plans = append(plans, clonePlan(plan))
		}
	}
	sort.Slice(plans, func(i, j int) bool {
		return plans[i].CreatedAt.After(plans[j].CreatedAt)
	})
	if len(plans) > limit {
		plans = plans[:limit]
	}
	return plans, nil
}

func (s *Service) CreateRenderJob(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreateRenderJobRequest) (RenderJob, error) {
	if err := ctx.Err(); err != nil {
		return RenderJob{}, err
	}
	if err := request.Validate(); err != nil {
		return RenderJob{}, err
	}
	id, err := s.newID()
	if err != nil {
		return RenderJob{}, err
	}
	if request.TargetFormat == "" {
		request.TargetFormat = "mp4"
	}
	if request.TargetQuality == "" {
		request.TargetQuality = "standard"
	}
	now := s.nowUTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	plan, ok := s.plans[request.PlanID]
	if !ok || plan.OrganizationID != actor.OrganizationID || plan.ProjectID != projectID {
		return RenderJob{}, ErrNotFound
	}
	job := RenderJob{
		ID:             id,
		OrganizationID: actor.OrganizationID,
		ProjectID:      projectID,
		PlanID:         request.PlanID,
		Status:         RenderQueued,
		TargetFormat:   request.TargetFormat,
		TargetQuality:  request.TargetQuality,
		CreatedBy:      actor.Principal,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := job.Validate(); err != nil {
		return RenderJob{}, err
	}
	s.renders[id] = job
	return cloneRenderJob(job), nil
}

func (s *Service) GetRenderJob(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (RenderJob, error) {
	if err := ctx.Err(); err != nil {
		return RenderJob{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.renders[id]
	if !ok || job.OrganizationID != actor.OrganizationID || job.ProjectID != projectID {
		return RenderJob{}, ErrNotFound
	}
	return cloneRenderJob(job), nil
}

func defaultID() (string, error) {
	return fmt.Sprintf("remixplan_%d", time.Now().UTC().UnixNano()), nil
}

func clonePlan(plan Plan) Plan {
	plan.Segments = cloneSegments(plan.Segments)
	plan.Warnings = append([]string(nil), plan.Warnings...)
	return plan
}

func cloneRenderJob(job RenderJob) RenderJob {
	if job.OutputAsset != nil {
		output := *job.OutputAsset
		job.OutputAsset = &output
	}
	return job
}

func cloneSegments(segments []SegmentPlan) []SegmentPlan {
	cloned := make([]SegmentPlan, len(segments))
	for index, segment := range segments {
		cloned[index] = segment
		cloned[index].Clips = append([]Clip(nil), segment.Clips...)
	}
	return cloned
}
