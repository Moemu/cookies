package delivery

import (
	"context"
	"sort"
	"sync"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type MemoryStore struct {
	mu    sync.RWMutex
	plans map[string]DeliveryPlan
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{plans: make(map[string]DeliveryPlan)}
}

func (s *MemoryStore) Create(_ context.Context, plan DeliveryPlan, version DeliveryPlanVersion) (DeliveryPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := storeKey(plan.OrganizationID, plan.ID)
	if _, exists := s.plans[key]; exists {
		return DeliveryPlan{}, ErrVersionConflict
	}
	plan.CurrentVersionNumber = version.VersionNumber
	plan.CurrentVersion = cloneVersion(version)
	plan.Versions = []DeliveryPlanVersion{cloneVersion(version)}
	s.plans[key] = clonePlan(plan)
	return clonePlan(plan), nil
}

func (s *MemoryStore) Get(_ context.Context, organizationID contract.OrganizationID, planID string) (DeliveryPlan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	plan, exists := s.plans[storeKey(organizationID, planID)]
	if !exists {
		return DeliveryPlan{}, ErrNotFound
	}
	return clonePlan(plan), nil
}

func (s *MemoryStore) List(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) ([]DeliveryPlan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]DeliveryPlan, 0)
	for _, plan := range s.plans {
		if plan.OrganizationID == organizationID && plan.ProjectID == projectID {
			result = append(result, clonePlan(plan))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt.Before(result[j].UpdatedAt) })
	return result, nil
}

func (s *MemoryStore) Update(_ context.Context, organizationID contract.OrganizationID, planID string, expectedVersion int, version DeliveryPlanVersion) (DeliveryPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := storeKey(organizationID, planID)
	plan, exists := s.plans[key]
	if !exists {
		return DeliveryPlan{}, ErrNotFound
	}
	if plan.CurrentVersionNumber != expectedVersion {
		return DeliveryPlan{}, ErrVersionConflict
	}
	plan.CurrentVersionNumber = version.VersionNumber
	plan.CurrentVersion = cloneVersion(version)
	plan.Versions = append(plan.Versions, cloneVersion(version))
	plan.Scenario = version.Scenario
	plan.UpdatedAt = version.CreatedAt
	s.plans[key] = clonePlan(plan)
	return clonePlan(plan), nil
}

func storeKey(organizationID contract.OrganizationID, planID string) string {
	return string(organizationID) + "\x00" + planID
}

func clonePlan(plan DeliveryPlan) DeliveryPlan {
	plan.CurrentVersion = cloneVersion(plan.CurrentVersion)
	plan.Versions = append([]DeliveryPlanVersion(nil), plan.Versions...)
	for index := range plan.Versions {
		plan.Versions[index] = cloneVersion(plan.Versions[index])
	}
	return plan
}

func cloneVersion(version DeliveryPlanVersion) DeliveryPlanVersion {
	version.CreativeReferences = append([]CreativeReference(nil), version.CreativeReferences...)
	return version
}

var _ Store = (*MemoryStore)(nil)
