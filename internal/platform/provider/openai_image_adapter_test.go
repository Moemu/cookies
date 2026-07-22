package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestOpenAIImageAdapterUsesV1ImagesGenerationEndpoint(t *testing.T) {
	t.Parallel()
	handles := &memoryOutputHandles{}
	adapter, err := NewOpenAIImageAdapter(OpenAIImageConfig{APIKey: "test-key", Model: "gpt-image-2", BaseURL: "http://gateway.example"}, handles)
	if err != nil {
		t.Fatalf("NewOpenAIImageAdapter() error = %v", err)
	}
	adapter.delegate.now = func() time.Time { return time.Date(2026, time.July, 22, 9, 0, 0, 0, time.UTC) }
	responseBody := fmt.Sprintf(`{"model":"gpt-image-2","data":[{"b64_json":"%s"}]}`, base64.StdEncoding.EncodeToString(fakeImagePNG))
	adapter.delegate.client = &http.Client{Transport: roundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "http://gateway.example/v1/images/generations" || request.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected OpenAI-compatible request: %s", request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(responseBody))}, nil
	})}
	submission, err := adapter.Submit(context.Background(), ImageGenerationRequest{OrganizationID: "org_1", ProjectID: "project_1", ProviderJobID: "provider_job_1", ModelAlias: "cookies.image.standard", IdempotencyKey: "openai-image-1", Input: ImageGenerationInput{Prompt: "poster", Width: 1024, Height: 1024}})
	if err != nil || submission.Status != ImageSubmissionCompleted || submission.ProviderCode != openAIImageProviderCode || len(submission.Outputs) != 1 {
		t.Fatalf("Submit() = %+v, %v", submission, err)
	}
	if submission.Outputs[0].ProviderCode != openAIImageProviderCode || submission.ModelVersion != "gpt-image-2" {
		t.Fatalf("unexpected OpenAI output: %+v", submission)
	}
	if _, _, err := adapter.Open(context.Background(), contract.ProjectRef{OrganizationID: "org_1", ProjectID: "project_1", ProjectContextVersion: 1}, submission.Outputs[0]); err != nil {
		t.Fatalf("Open() error = %v", err)
	}
}

func TestOpenAIImageAdapterDoesNotDuplicateVersionedBasePath(t *testing.T) {
	t.Parallel()
	adapter, err := NewOpenAIImageAdapter(OpenAIImageConfig{APIKey: "test-key", Model: "gpt-image-2", BaseURL: "http://gateway.example/api/v1"}, &memoryOutputHandles{})
	if err != nil {
		t.Fatalf("NewOpenAIImageAdapter() error = %v", err)
	}
	if got, want := adapter.delegate.baseURL+adapter.delegate.endpointPath, "http://gateway.example/api/v1/images/generations"; got != want {
		t.Fatalf("OpenAI-compatible endpoint = %q, want %q", got, want)
	}
}
