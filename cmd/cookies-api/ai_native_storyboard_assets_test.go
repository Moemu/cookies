package main

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/systems/creative"
)

func TestStoryboardAssetPreparationAcceptsActorWithProviderCreateScope(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 5, 9, 0, 0, 0, time.UTC)
	store := &storyboardProviderStore{}
	service := &provider.Service{
		Store:     store,
		Scheduler: storyboardProviderScheduler{},
		NewID:     func() (string, error) { return "provider_job_storyboard_1", nil },
		Now:       func() time.Time { return now },
	}
	brandID := contract.BrandID("brand_1")
	actor := contract.ActorContext{
		OrganizationID: "org_1",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
		Scopes:         []contract.Scope{creative.ScopeWrite, provider.ScopeJobCreate},
	}
	project := contract.ProjectContext{
		OrganizationID:        "org_1",
		ProjectID:             "project_1",
		BrandID:               &brandID,
		ProductIDs:            []contract.ProductID{},
		ProjectContextVersion: 1,
	}
	operation := creative.AINativeStoryboardOperation{ID: "ainativestoryboardop_1"}
	asset := creative.AINativeStoryboardAsset{
		ID:              "person_1",
		Role:            creative.AINativeStoryboardAssetRolePersonIdentity,
		Name:            "商务男士",
		Source:          creative.AINativeStoryboardAssetSourceAIGenerated,
		GenerationBrief: "一位商务男士，正面半身，写实商业摄影",
		Status:          creative.AINativeStoryboardAssetPlanned,
	}

	ref, retryAt, err := (creativeAINativeStoryboardAssetPreparer{provider: service, now: func() time.Time { return now }}).
		PrepareAINativeStoryboardAsset(context.Background(), actor, project, operation, asset)
	if err != nil {
		t.Fatalf("PrepareAINativeStoryboardAsset() error = %v", err)
	}
	if ref != nil || retryAt == nil || !retryAt.Equal(now.Add(5*time.Second)) {
		t.Fatalf("PrepareAINativeStoryboardAsset() ref=%v retryAt=%v", ref, retryAt)
	}
}

type storyboardProviderStore struct {
	records []provider.JobRecord
}

func (s *storyboardProviderStore) Create(_ context.Context, record provider.JobRecord) (provider.JobRecord, bool, error) {
	s.records = append(s.records, record)
	return record, false, nil
}

func (s *storyboardProviderStore) Get(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, jobID string) (provider.JobRecord, error) {
	for _, record := range s.records {
		if record.Job.OrganizationID == organizationID && record.Job.ProjectID == projectID && record.Job.ID == jobID {
			return record, nil
		}
	}
	return provider.JobRecord{}, provider.ErrJobNotFound
}

func (s *storyboardProviderStore) Update(_ context.Context, record provider.JobRecord) (provider.JobRecord, error) {
	return record, nil
}

type storyboardProviderScheduler struct{}

func (storyboardProviderScheduler) Schedule(context.Context, contract.ProviderJob) error { return nil }
