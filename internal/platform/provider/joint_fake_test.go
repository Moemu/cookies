package provider

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

// This is the Provider-side consumer-contract test for the Gate 3 joint
// closure. The fake Intake behaves like Assets: it opens the opaque output
// under the trusted ProjectRef, verifies metadata, then returns one durable
// ProjectAssetRef. It does not share Provider storage.
func TestFakeImageProviderCompletesOnlyAfterScopedGeneratedIntake(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 22, 8, 0, 0, 0, time.UTC)
	record := executableImageJobRecord(now)
	store := &processingStore{record: record}
	adapter := NewFakeImageAdapter(func() time.Time { return now })
	intake := &fetchingGeneratedIntake{fetcher: adapter}
	service := Service{Store: store, ImageAdapter: adapter, Intake: intake, Now: func() time.Time { return now }}

	for attempt := 0; attempt < 2; attempt++ {
		job, deferredUntil, err := service.ExecuteImageJob(context.Background(), record.Job.OrganizationID, record.Job.ProjectID, record.Job.ID)
		if err != nil || deferredUntil == nil || job.ExecutionStatus != contract.JobRunning {
			t.Fatalf("attempt %d = (%+v, deferred=%v, err=%v), want a running deferred job", attempt, job, deferredUntil, err)
		}
	}
	completed, deferredUntil, err := service.ExecuteImageJob(context.Background(), record.Job.OrganizationID, record.Job.ProjectID, record.Job.ID)
	if err != nil || deferredUntil != nil || completed.ExecutionStatus != contract.JobSucceeded || completed.ProviderStatus != contract.ProviderJobSucceeded || len(completed.ProjectAssetRefs) != 1 {
		t.Fatalf("completion = (%+v, deferred=%v, err=%v)", completed, deferredUntil, err)
	}
	if intake.createCalls != 1 || intake.project.OrganizationID != record.Job.OrganizationID || intake.project.ProjectID != record.Job.ProjectID || intake.project.ProjectContextVersion != record.ProjectContextVersion {
		t.Fatalf("generated intake did not receive the trusted project scope: %+v", intake)
	}
}

type fetchingGeneratedIntake struct {
	fetcher     assets.GeneratedOutputFetcher
	project     contract.ProjectRef
	createCalls int
}

func (i *fetchingGeneratedIntake) Create(ctx context.Context, project contract.ProjectRef, request assets.GeneratedAssetIntakeRequest, _ contract.IdempotencyKey) (assets.GeneratedAssetIntakeResponse, error) {
	i.createCalls++
	i.project = project
	if err := request.Validate(); err != nil {
		return assets.GeneratedAssetIntakeResponse{}, err
	}
	stream, metadata, err := i.fetcher.Open(ctx, project, request.Output)
	if err != nil {
		return assets.GeneratedAssetIntakeResponse{}, err
	}
	contents, readErr := io.ReadAll(stream)
	closeErr := stream.Close()
	if readErr != nil || closeErr != nil || metadata.SizeBytes != int64(len(contents)) || metadata.MIMEType != request.Output.DeclaredMIMEType || metadata.SHA256 != dereferenceSHA256(request.Output.DeclaredSHA256) {
		return assets.GeneratedAssetIntakeResponse{}, fmt.Errorf("fake assets verification failed")
	}
	return assets.GeneratedAssetIntakeResponse{
		ID: "intake_" + request.Output.OutputID, ProviderJobID: request.ProviderJobID, OutputID: request.Output.OutputID, Status: assets.GeneratedIntakeSucceeded,
		ProjectAssetRef: &contract.ProjectAssetRef{ProjectID: project.ProjectID, AssetVersion: contract.AssetVersionRef{AssetID: contract.AssetID("asset_" + request.Output.OutputID), Version: 1}},
	}, nil
}

func (i *fetchingGeneratedIntake) Get(context.Context, contract.ProjectRef, string) (assets.GeneratedAssetIntakeResponse, error) {
	return assets.GeneratedAssetIntakeResponse{}, fmt.Errorf("fake intake is synchronous")
}

func dereferenceSHA256(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
