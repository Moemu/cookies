package provider

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestFakeImageAdapterReturnsOpaqueOutputAfterPolling(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 22, 6, 0, 0, 0, time.UTC)
	adapter := NewFakeImageAdapter(func() time.Time { return now })
	submission, err := adapter.Submit(context.Background(), ImageGenerationRequest{
		OrganizationID: "org_1", ProjectID: "project_1", ProviderJobID: "provider_job_1", ModelAlias: "cookies.image.standard", IdempotencyKey: "fake-image-1",
		Input: ImageGenerationInput{Prompt: "launch poster", Width: 1024, Height: 1024},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	reference := ImageTaskReference{ProviderCode: submission.ProviderCode, ModelAlias: "cookies.image.standard", ModelVersion: submission.ModelVersion, ExternalTaskID: submission.ExternalTaskID}
	running, err := adapter.Poll(context.Background(), reference)
	if err != nil || running.Status != ImageTaskRunning || running.Progress != 50 {
		t.Fatalf("first Poll() = %+v, %v", running, err)
	}
	completed, err := adapter.Poll(context.Background(), reference)
	if err != nil || completed.Status != ImageTaskSucceeded || len(completed.Outputs) != 1 {
		t.Fatalf("second Poll() = %+v, %v", completed, err)
	}
	output := completed.Outputs[0]
	if output.ProviderCode != fakeProviderCode || output.ProviderJobID != "provider_job_1" || output.OutputID != "output_1" || output.DeclaredMIMEType != "image/png" {
		t.Fatalf("fake output is not an opaque image reference: %+v", output)
	}
	if err := output.Validate(); err != nil {
		t.Fatalf("fake output Validate() = %v", err)
	}
	var fetcher assets.GeneratedOutputFetcher = adapter
	stream, metadata, err := fetcher.Open(context.Background(), contract.ProjectRef{OrganizationID: "org_1", ProjectID: "project_1", ProjectContextVersion: 1}, output)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	contents, readErr := io.ReadAll(stream)
	closeErr := stream.Close()
	if readErr != nil || closeErr != nil || metadata.SizeBytes != int64(len(contents)) || metadata.MIMEType != "image/png" {
		t.Fatalf("invalid fake output stream: bytes=%d metadata=%+v read=%v close=%v", len(contents), metadata, readErr, closeErr)
	}
	if _, _, err := fetcher.Open(context.Background(), contract.ProjectRef{OrganizationID: "org_1", ProjectID: "other_project", ProjectContextVersion: 1}, output); err == nil {
		t.Fatal("Open() cross-project error = nil, want rejection")
	}
}
