package provider

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestFakeVideoAdapterReturnsOpaqueMP4AfterPolling(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 27, 6, 0, 0, 0, time.UTC)
	adapter := NewFakeVideoAdapter(func() time.Time { return now })
	submission, err := adapter.Submit(context.Background(), VideoGenerationRequest{
		OrganizationID: "org_1", ProjectID: "project_1", ProviderJobID: "provider_job_1", ModelAlias: "cookies.video.standard", IdempotencyKey: "fake-video-1",
		Input: VideoGenerationInput{Prompt: "five-second product pre-roll", DurationSeconds: 5, AspectRatio: "9:16", Resolution: "720p"},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	reference := VideoTaskReference{
		OrganizationID: "org_1", ProjectID: "project_1", ProviderJobID: "provider_job_1",
		ProviderCode: submission.ProviderCode, ModelAlias: "cookies.video.standard", ModelVersion: submission.ModelVersion, ExternalTaskID: submission.ExternalTaskID,
	}
	running, err := adapter.Poll(context.Background(), reference)
	if err != nil || running.Status != VideoTaskRunning || running.Progress != 50 {
		t.Fatalf("first Poll() = %+v, %v", running, err)
	}
	completed, err := adapter.Poll(context.Background(), reference)
	if err != nil || completed.Status != VideoTaskSucceeded || len(completed.Outputs) != 1 {
		t.Fatalf("second Poll() = %+v, %v", completed, err)
	}
	output := completed.Outputs[0]
	if output.ProviderCode != fakeVideoProviderCode || output.ProviderJobID != "provider_job_1" || output.OutputID != "output_1" || output.DeclaredMIMEType != "video/mp4" {
		t.Fatalf("fake output is not an opaque video reference: %+v", output)
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
	if readErr != nil || closeErr != nil || metadata.SizeBytes != int64(len(contents)) || metadata.MIMEType != "video/mp4" {
		t.Fatalf("invalid fake output stream: bytes=%d metadata=%+v read=%v close=%v", len(contents), metadata, readErr, closeErr)
	}
	if len(contents) < 12 || string(contents[4:8]) != "ftyp" {
		t.Fatalf("fake video does not contain an MP4 file type box: %x", contents)
	}
	if len(contents) < 1000 {
		t.Fatalf("fake video must be a complete deterministic media fixture, got only %d bytes", len(contents))
	}
}
