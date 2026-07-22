package provider

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestServiceCreatesQueuedImageProviderJob(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 22, 2, 0, 0, 0, time.UTC)
	store := &memoryStore{}
	service := Service{
		Store: store,
		NewID: func() string { return "provider_job_1" },
		Now:   func() time.Time { return now },
	}
	brandID := contract.BrandID("brand_1")
	job, duplicate, err := service.CreateImageJob(context.Background(), CreateImageJobRequest{
		Actor: contract.ActorContext{
			OrganizationID: "org_1",
			Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
			Scopes:         []contract.Scope{ScopeJobCreate},
		},
		Project: contract.ProjectContext{
			OrganizationID:        "org_1",
			ProjectID:             "project_1",
			BrandID:               &brandID,
			ProductIDs:            []contract.ProductID{},
			ProjectContextVersion: 7,
		},
		IdempotencyKey: "create-image-1",
		RequestHash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ModelAlias:     "cookies.image.standard",
		Input:          ImageGenerationInput{Prompt: "生成一张新品海报", Width: 1024, Height: 1024},
	})
	if err != nil || duplicate {
		t.Fatalf("CreateImageJob() duplicate=%v err=%v", duplicate, err)
	}
	if job.ID != "provider_job_1" || job.Kind != "provider.image.generate" || job.ExecutionStatus != contract.JobQueued || job.ProviderStatus != contract.ProviderJobSubmitted {
		t.Fatalf("unexpected job: %+v", job)
	}
	if job.ProjectID != "project_1" || job.OrganizationID != "org_1" || job.ProjectAssetRefs == nil {
		t.Fatalf("unexpected public job state: %+v", job)
	}
	if len(store.records) != 1 || store.records[0].ProjectContextVersion != 7 || store.records[0].Input.Prompt != "生成一张新品海报" {
		t.Fatalf("unexpected stored record: %+v", store.records)
	}
}

func TestServiceRejectsImageJobForDraftProject(t *testing.T) {
	t.Parallel()
	service := Service{Store: &memoryStore{}, NewID: func() string { return "provider_job_1" }}
	_, _, err := service.CreateImageJob(context.Background(), CreateImageJobRequest{
		Actor: contract.ActorContext{
			OrganizationID: "org_1",
			Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
			Scopes:         []contract.Scope{ScopeJobCreate},
		},
		Project: contract.ProjectContext{
			OrganizationID:        "org_1",
			ProjectID:             "project_1",
			ProductIDs:            []contract.ProductID{},
			ProjectContextVersion: 1,
		},
		IdempotencyKey: "create-image-1",
		RequestHash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ModelAlias:     "cookies.image.standard",
		Input:          ImageGenerationInput{Prompt: "生成一张新品海报", Width: 1024, Height: 1024},
	})
	if err == nil {
		t.Fatal("CreateImageJob() error = nil, want draft project rejection")
	}
}

type memoryStore struct{ records []JobRecord }

func (s *memoryStore) Create(_ context.Context, record JobRecord) (JobRecord, bool, error) {
	s.records = append(s.records, record)
	return record, false, nil
}
