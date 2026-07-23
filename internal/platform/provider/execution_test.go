package provider

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestServiceExecutesImageJobFromSubmitThroughAssetIntake(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 22, 4, 0, 0, 0, time.UTC)
	version := int64(1)
	record := executableImageJobRecord(now)
	store := &processingStore{record: record}
	adapter := &scriptedImageAdapter{
		submission: ImageSubmission{Status: ImageSubmissionAccepted, ProviderCode: "fake", ModelVersion: "fake-image-v1", ExternalTaskID: "fake-task-1"},
		polls: []ImageTaskResult{
			{Status: ImageTaskRunning, Progress: 50},
			{Status: ImageTaskSucceeded, Outputs: []contract.ProviderOutputRef{{
				ProviderCode: "fake", ProviderJobID: record.Job.ID, OutputID: "output_1",
				RetrievalExpiresAt: now.Add(time.Hour), DeclaredMIMEType: "image/png", DeclaredSizeBytes: 1024,
			}}},
		},
	}
	intake := &fakeIntakeClient{response: assets.GeneratedAssetIntakeResponse{
		ID: "intake_1", ProviderJobID: record.Job.ID, OutputID: "output_1", Status: assets.GeneratedIntakeSucceeded,
		ProjectAssetRef: &contract.ProjectAssetRef{ProjectID: record.Job.ProjectID, AssetVersion: contract.AssetVersionRef{AssetID: "asset_1", Version: version}},
	}}
	service := Service{Store: store, ImageAdapter: adapter, Intake: intake, Now: func() time.Time { return now }}

	job, deferredUntil, err := service.ExecuteImageJob(context.Background(), record.Job.OrganizationID, record.Job.ProjectID, record.Job.ID)
	if err != nil || deferredUntil == nil {
		t.Fatalf("submit ExecuteImageJob() = (%+v, deferred=%v, err=%v)", job, deferredUntil, err)
	}
	if job.ProviderStatus != contract.ProviderJobRunning || job.ExecutionStatus != contract.JobRunning || adapter.submitCalls != 1 {
		t.Fatalf("submit did not persist the running state: job=%+v calls=%d", job, adapter.submitCalls)
	}

	job, deferredUntil, err = service.ExecuteImageJob(context.Background(), record.Job.OrganizationID, record.Job.ProjectID, record.Job.ID)
	if err != nil || deferredUntil == nil || job.Progress != 50 || adapter.pollCalls != 1 {
		t.Fatalf("running poll ExecuteImageJob() = (%+v, deferred=%v, polls=%d, err=%v)", job, deferredUntil, adapter.pollCalls, err)
	}

	job, deferredUntil, err = service.ExecuteImageJob(context.Background(), record.Job.OrganizationID, record.Job.ProjectID, record.Job.ID)
	if err != nil || deferredUntil != nil {
		t.Fatalf("completion ExecuteImageJob() = (%+v, deferred=%v, err=%v)", job, deferredUntil, err)
	}
	if job.ProviderStatus != contract.ProviderJobSucceeded || job.ExecutionStatus != contract.JobSucceeded || len(job.ProjectAssetRefs) != 1 {
		t.Fatalf("completed job = %+v, want succeeded job with one durable project asset", job)
	}
	if adapter.pollCalls != 2 || intake.request.Output.OutputID != "output_1" {
		t.Fatalf("expected one ready output to reach Assets: polls=%d intake=%+v", adapter.pollCalls, intake.request)
	}
}

func TestServiceHandsSynchronousSubmissionToAssetsWithoutPolling(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 22, 4, 30, 0, 0, time.UTC)
	record := executableImageJobRecord(now)
	store := &processingStore{record: record}
	adapter := &scriptedImageAdapter{submission: ImageSubmission{Status: ImageSubmissionCompleted, ProviderCode: "ark", ModelVersion: "seedream-test", Outputs: []contract.ProviderOutputRef{{
		ProviderCode: "ark", ProviderJobID: record.Job.ID, OutputID: "output_1", RetrievalExpiresAt: now.Add(time.Hour), DeclaredMIMEType: "image/png", DeclaredSizeBytes: 1024,
	}}}}
	version := int64(1)
	intake := &fakeIntakeClient{response: assets.GeneratedAssetIntakeResponse{ID: "intake_1", ProviderJobID: record.Job.ID, OutputID: "output_1", Status: assets.GeneratedIntakeSucceeded, ProjectAssetRef: &contract.ProjectAssetRef{ProjectID: record.Job.ProjectID, AssetVersion: contract.AssetVersionRef{AssetID: "asset_1", Version: version}}}}
	service := Service{Store: store, ImageAdapter: adapter, Intake: intake, Now: func() time.Time { return now }}

	job, deferredUntil, err := service.ExecuteImageJob(context.Background(), record.Job.OrganizationID, record.Job.ProjectID, record.Job.ID)
	if err != nil || deferredUntil != nil || job.ProviderStatus != contract.ProviderJobSucceeded {
		t.Fatalf("synchronous ExecuteImageJob() = (%+v, deferred=%v, err=%v)", job, deferredUntil, err)
	}
	if adapter.submitCalls != 1 || adapter.pollCalls != 0 || intake.request.Output.OutputID != "output_1" {
		t.Fatalf("synchronous submission did not take the direct intake path: submit=%d poll=%d intake=%+v", adapter.submitCalls, adapter.pollCalls, intake.request)
	}
}

func executableImageJobRecord(now time.Time) JobRecord {
	return JobRecord{
		Job: contract.ProviderJob{
			ID:               "provider_job_execute_1",
			Kind:             imageJobKind,
			OrganizationID:   "org_1",
			ProjectID:        "project_1",
			ExecutionStatus:  contract.JobQueued,
			ProviderStatus:   contract.ProviderJobSubmitted,
			ProjectAssetRefs: []contract.ProjectAssetRef{},
			MaxAttempts:      3,
			Version:          1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		Principal:             contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
		Operation:             imageOperation,
		IdempotencyKey:        "execute-image-1",
		RequestHash:           "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ProjectContextVersion: 7,
		ModelAlias:            "cookies.image.standard",
		Input:                 ImageGenerationInput{Prompt: "launch image", Width: 1024, Height: 1024},
	}
}

type scriptedImageAdapter struct {
	submission  ImageSubmission
	polls       []ImageTaskResult
	submitCalls int
	pollCalls   int
}

func (a *scriptedImageAdapter) Submit(context.Context, ImageGenerationRequest) (ImageSubmission, error) {
	a.submitCalls++
	return a.submission, nil
}

func (a *scriptedImageAdapter) Poll(context.Context, ImageTaskReference) (ImageTaskResult, error) {
	result := a.polls[a.pollCalls]
	a.pollCalls++
	return result, nil
}
