package contract

import (
	"testing"
	"time"
)

func TestProviderJobSeparatesExecutionAndDomainStatuses(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	job := ProviderJob{
		ID: "job_1", Kind: "provider.image.generate", OrganizationID: "org_1", ProjectID: "project_1",
		ExecutionStatus: JobRunning, ProviderStatus: ProviderJobIngesting, Progress: 80,
		ProjectAssetRefs: []ProjectAssetRef{}, AttemptCount: 1, MaxAttempts: 3, Version: 5,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := job.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	job.ExecutionStatus = JobStatus(ProviderJobOutputsReady)
	if err := job.Validate(); err == nil {
		t.Fatal("provider state must not be accepted as execution_status")
	}
}

func TestProviderOutputRefValidatesOptionalDeclaredHash(t *testing.T) {
	t.Parallel()
	ref := ProviderOutputRef{
		ProviderCode: "volcengine", ProviderJobID: "job_1", OutputID: "output_1",
		RetrievalExpiresAt: time.Now().UTC().Add(2 * time.Hour), DeclaredMIMEType: "image/png", DeclaredSizeBytes: 1024,
	}
	if err := ref.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	bad := "not-a-sha256"
	ref.DeclaredSHA256 = &bad
	if err := ref.Validate(); err == nil {
		t.Fatal("expected invalid declared hash to be rejected")
	}
}
