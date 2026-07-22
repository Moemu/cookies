package provider

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestServiceCompletesImageJobOnlyAfterIntakeReturnsProjectAsset(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 22, 3, 0, 0, 0, time.UTC)
	version := int64(1)
	record := JobRecord{
		Job: contract.ProviderJob{
			ID:               "provider_job_1",
			Kind:             imageJobKind,
			OrganizationID:   "org_1",
			ProjectID:        "project_1",
			ExecutionStatus:  contract.JobRunning,
			ProviderStatus:   contract.ProviderJobOutputsReady,
			Progress:         70,
			ProjectAssetRefs: []contract.ProjectAssetRef{},
			MaxAttempts:      3,
			Version:          2,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		Principal:             contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
		Operation:             imageOperation,
		IdempotencyKey:        "create-image-1",
		RequestHash:           "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ProjectContextVersion: 7,
		ModelAlias:            "cookies.image.standard",
		ProviderCode:          "fake",
		ModelVersion:          "fake-image-v1",
		Input:                 ImageGenerationInput{Prompt: "生成一张新品海报", Width: 1024, Height: 1024},
		Outputs: []OutputRecord{{
			Ref: contract.ProviderOutputRef{
				ProviderCode: "fake", ProviderJobID: "provider_job_1", OutputID: "output_1",
				RetrievalExpiresAt: now.Add(time.Hour), DeclaredMIMEType: "image/png", DeclaredSizeBytes: 1024,
			},
			Status: OutputReady,
		}},
	}
	store := &processingStore{record: record}
	intake := &fakeIntakeClient{response: assets.GeneratedAssetIntakeResponse{
		ID: "intake_1", ProviderJobID: "provider_job_1", OutputID: "output_1", Status: assets.GeneratedIntakeSucceeded,
		ProjectAssetRef: &contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "asset_1", Version: version}},
	}}
	service := Service{Store: store, Intake: intake, Now: func() time.Time { return now }}

	updated, deferredUntil, err := service.ProcessImageJob(context.Background(), "org_1", "project_1", "provider_job_1")
	if err != nil || deferredUntil != nil {
		t.Fatalf("ProcessImageJob() deferred_until=%v err=%v", deferredUntil, err)
	}
	if updated.ProviderStatus != contract.ProviderJobSucceeded || updated.ExecutionStatus != contract.JobSucceeded || len(updated.ProjectAssetRefs) != 1 {
		t.Fatalf("unexpected completed job: %+v", updated)
	}
	if intake.key != "provider-job-provider_job_1-output-output_1" || intake.request.Output.OutputID != "output_1" {
		t.Fatalf("unexpected intake call: key=%q request=%+v", intake.key, intake.request)
	}
}

func TestServiceDefersUntilGeneratedIntakeCompletes(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 22, 3, 30, 0, 0, time.UTC)
	record := readyOutputJobRecord(now)
	store := &processingStore{record: record}
	version := int64(1)
	intake := &scriptedIntakeClient{
		create: assets.GeneratedAssetIntakeResponse{ID: "intake_1", ProviderJobID: record.Job.ID, OutputID: "output_1", Status: assets.GeneratedIntakeQueued},
		get: assets.GeneratedAssetIntakeResponse{
			ID: "intake_1", ProviderJobID: record.Job.ID, OutputID: "output_1", Status: assets.GeneratedIntakeSucceeded,
			ProjectAssetRef: &contract.ProjectAssetRef{ProjectID: record.Job.ProjectID, AssetVersion: contract.AssetVersionRef{AssetID: "asset_1", Version: version}},
		},
	}
	service := Service{Store: store, Intake: intake, Now: func() time.Time { return now }}

	job, deferredUntil, err := service.ProcessImageJob(context.Background(), record.Job.OrganizationID, record.Job.ProjectID, record.Job.ID)
	if err != nil || deferredUntil == nil || job.ProviderStatus != contract.ProviderJobIngesting {
		t.Fatalf("pending intake ProcessImageJob() = (%+v, deferred=%v, err=%v)", job, deferredUntil, err)
	}
	job, deferredUntil, err = service.ProcessImageJob(context.Background(), record.Job.OrganizationID, record.Job.ProjectID, record.Job.ID)
	if err != nil || deferredUntil != nil || job.ProviderStatus != contract.ProviderJobSucceeded || len(job.ProjectAssetRefs) != 1 {
		t.Fatalf("completed intake ProcessImageJob() = (%+v, deferred=%v, err=%v)", job, deferredUntil, err)
	}
	if intake.createCalls != 1 || intake.getCalls != 1 {
		t.Fatalf("expected create then get exactly once: %+v", intake)
	}
}

func TestServiceFailsImageJobWhenEveryIntakeFails(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 22, 3, 45, 0, 0, time.UTC)
	record := readyOutputJobRecord(now)
	store := &processingStore{record: record}
	intake := &fakeIntakeClient{response: assets.GeneratedAssetIntakeResponse{
		ID: "intake_1", ProviderJobID: record.Job.ID, OutputID: "output_1", Status: assets.GeneratedIntakeFailed,
		Error: &contract.JobError{Code: contract.ErrorAssetIntakeFailed, Message: "scan rejected output", Retryable: false},
	}}
	service := Service{Store: store, Intake: intake, Now: func() time.Time { return now }}

	job, deferredUntil, err := service.ProcessImageJob(context.Background(), record.Job.OrganizationID, record.Job.ProjectID, record.Job.ID)
	if err != nil || deferredUntil != nil || job.ProviderStatus != contract.ProviderJobFailed || job.ExecutionStatus != contract.JobFailed || job.Error == nil {
		t.Fatalf("failed intake ProcessImageJob() = (%+v, deferred=%v, err=%v)", job, deferredUntil, err)
	}
}

func readyOutputJobRecord(now time.Time) JobRecord {
	return JobRecord{
		Job: contract.ProviderJob{
			ID: "provider_job_1", Kind: imageJobKind, OrganizationID: "org_1", ProjectID: "project_1",
			ExecutionStatus: contract.JobRunning, ProviderStatus: contract.ProviderJobOutputsReady, Progress: 70,
			ProjectAssetRefs: []contract.ProjectAssetRef{}, MaxAttempts: 3, Version: 2, CreatedAt: now, UpdatedAt: now,
		},
		Principal:             contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
		Operation:             imageOperation,
		IdempotencyKey:        "create-image-1",
		RequestHash:           "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ProjectContextVersion: 7,
		ModelAlias:            "cookies.image.standard",
		ProviderCode:          "fake",
		ModelVersion:          "fake-image-v1",
		Input:                 ImageGenerationInput{Prompt: "launch poster", Width: 1024, Height: 1024},
		Outputs: []OutputRecord{{
			Ref:    contract.ProviderOutputRef{ProviderCode: "fake", ProviderJobID: "provider_job_1", OutputID: "output_1", RetrievalExpiresAt: now.Add(time.Hour), DeclaredMIMEType: "image/png", DeclaredSizeBytes: 1024},
			Status: OutputReady,
		}},
	}
}

type processingStore struct{ record JobRecord }

func (s *processingStore) Create(context.Context, JobRecord) (JobRecord, bool, error) {
	return JobRecord{}, false, nil
}

func (s *processingStore) Get(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, jobID string) (JobRecord, error) {
	if s.record.Job.OrganizationID != organizationID || s.record.Job.ProjectID != projectID || s.record.Job.ID != jobID {
		return JobRecord{}, ErrJobNotFound
	}
	return s.record, nil
}

func (s *processingStore) Update(_ context.Context, record JobRecord) (JobRecord, error) {
	record.Job.Version++
	s.record = record
	return record, nil
}

type fakeIntakeClient struct {
	request  assets.GeneratedAssetIntakeRequest
	key      string
	response assets.GeneratedAssetIntakeResponse
}

func (c *fakeIntakeClient) Create(_ context.Context, _ contract.ProjectID, request assets.GeneratedAssetIntakeRequest, key contract.IdempotencyKey) (assets.GeneratedAssetIntakeResponse, error) {
	c.request = request
	c.key = string(key)
	return c.response, nil
}

func (c *fakeIntakeClient) Get(context.Context, contract.ProjectID, string) (assets.GeneratedAssetIntakeResponse, error) {
	return c.response, nil
}

type scriptedIntakeClient struct {
	create      assets.GeneratedAssetIntakeResponse
	get         assets.GeneratedAssetIntakeResponse
	createCalls int
	getCalls    int
}

func (c *scriptedIntakeClient) Create(context.Context, contract.ProjectID, assets.GeneratedAssetIntakeRequest, contract.IdempotencyKey) (assets.GeneratedAssetIntakeResponse, error) {
	c.createCalls++
	return c.create, nil
}

func (c *scriptedIntakeClient) Get(context.Context, contract.ProjectID, string) (assets.GeneratedAssetIntakeResponse, error) {
	c.getCalls++
	return c.get, nil
}
