package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestArkImageAdapterCachesOpaqueBase64Output(t *testing.T) {
	t.Parallel()
	contents := fakeImagePNG
	responseBody := fmt.Sprintf(`{"model":"seedream-test","data":[{"b64_json":"%s"}]}`, base64.StdEncoding.EncodeToString(contents))
	handles := &memoryOutputHandles{}
	adapter, err := NewArkImageAdapter(ArkImageConfig{APIKey: "test-key", Model: "seedream-test"}, handles)
	if err != nil {
		t.Fatalf("NewArkImageAdapter() error = %v", err)
	}
	now := time.Date(2026, time.July, 22, 8, 0, 0, 0, time.UTC)
	adapter.now = func() time.Time { return now }
	adapter.client = &http.Client{Transport: roundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/v3/images/generations" || request.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected Ark request: %s %q", request.URL, request.Header.Get("Authorization"))
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(responseBody))}, nil
	})}
	submission, err := adapter.Submit(context.Background(), ImageGenerationRequest{OrganizationID: "org_1", ProjectID: "project_1", ProviderJobID: "provider_job_1", ModelAlias: "cookies.image.standard", IdempotencyKey: "ark-image-1", Input: ImageGenerationInput{Prompt: "launch poster", Width: 1024, Height: 1024}})
	if err != nil || submission.Status != ImageSubmissionCompleted || len(submission.Outputs) != 1 {
		t.Fatalf("Submit() = %+v, %v", submission, err)
	}
	output := submission.Outputs[0]
	if output.ProviderCode != arkProviderCode || output.DeclaredMIMEType != "image/png" || output.DeclaredSHA256 == nil {
		t.Fatalf("unexpected output reference: %+v", output)
	}
	stream, metadata, err := adapter.Open(context.Background(), contract.ProjectRef{OrganizationID: "org_1", ProjectID: "project_1", ProjectContextVersion: 7}, output)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	got, readErr := io.ReadAll(stream)
	_ = stream.Close()
	if readErr != nil || !bytes.Equal(got, contents) || metadata.SizeBytes != int64(len(contents)) {
		t.Fatalf("Open() bytes=%d metadata=%+v read=%v", len(got), metadata, readErr)
	}
	if _, _, err := adapter.Open(context.Background(), contract.ProjectRef{OrganizationID: "org_1", ProjectID: "other", ProjectContextVersion: 7}, output); err == nil {
		t.Fatal("cross-project Open() error = nil, want rejection")
	}
}

func TestArkImageAdapterSendsSourceImageForEdit(t *testing.T) {
	t.Parallel()
	handles := &memoryOutputHandles{}
	adapter, err := NewArkImageAdapter(ArkImageConfig{APIKey: "test-key", Model: "seedream-edit-test"}, handles)
	if err != nil {
		t.Fatalf("NewArkImageAdapter() error = %v", err)
	}
	resultURL := "https://example.test/result.png"
	adapter.client = &http.Client{Transport: roundTripper(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodPost:
			var body struct {
				Model          string   `json:"model"`
				Prompt         string   `json:"prompt"`
				Image          []string `json:"image"`
				ResponseFormat string   `json:"response_format"`
				OutputFormat   string   `json:"output_format"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatalf("decode Ark request body: %v", err)
			}
			if body.Model != "seedream-edit-test" || body.Prompt == "" || len(body.Image) != 1 || body.ResponseFormat != "url" || body.OutputFormat != "png" {
				t.Fatalf("unexpected edit request body: %+v", body)
			}
			wantPrefix := "data:image/png;base64,"
			if got := body.Image[0]; len(got) <= len(wantPrefix) || got[:len(wantPrefix)] != wantPrefix {
				t.Fatalf("source image was not sent as PNG data URL: %q", body.Image[0])
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(fmt.Sprintf(`{"model":"seedream-edit-test","data":[{"url":%q}]}`, resultURL)))}, nil
		case request.Method == http.MethodGet && request.URL.String() == resultURL:
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(fakeImagePNG))}, nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
			return nil, nil
		}
	})}
	submission, err := adapter.Submit(context.Background(), ImageGenerationRequest{
		OrganizationID: "org_1", ProjectID: "project_1", ProviderJobID: "provider_job_1",
		ModelAlias: "cookies.image.standard", IdempotencyKey: "ark-image-edit-1",
		Input: ImageGenerationInput{
			Prompt: "换成清晨咖啡店背景", Width: 1024, Height: 1024,
			SourceAssets: []contract.ProjectAssetRef{{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "asset_1", Version: 1}}},
		},
		Sources: []VisionSource{{Reference: contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "asset_1", Version: 1}}, MIMEType: "image/png", Content: io.NopCloser(bytes.NewReader(fakeImagePNG))}},
	})
	if err != nil || submission.Status != ImageSubmissionCompleted || len(submission.Outputs) != 1 {
		t.Fatalf("Submit() = %+v, %v", submission, err)
	}
	if !bytes.Equal(handles.contents, fakeImagePNG) {
		t.Fatalf("downloaded edit output was not cached")
	}
}

type memoryOutputHandles struct {
	project  contract.ProjectRef
	ref      contract.ProviderOutputRef
	contents []byte
}

func (s *memoryOutputHandles) Put(_ context.Context, project contract.ProjectRef, ref contract.ProviderOutputRef, contents []byte) error {
	s.project, s.ref, s.contents = project, ref, append([]byte(nil), contents...)
	return nil
}
func (s *memoryOutputHandles) Open(_ context.Context, project contract.ProjectRef, ref contract.ProviderOutputRef) (io.ReadCloser, contract.OutputMetadata, error) {
	if project.OrganizationID != s.project.OrganizationID || project.ProjectID != s.project.ProjectID || ref.ProviderJobID != s.ref.ProviderJobID || ref.OutputID != s.ref.OutputID {
		return nil, contract.OutputMetadata{}, ErrOutputHandleNotFound
	}
	return io.NopCloser(bytes.NewReader(s.contents)), contract.OutputMetadata{MIMEType: ref.DeclaredMIMEType, SizeBytes: int64(len(s.contents)), SHA256: *ref.DeclaredSHA256}, nil
}
func (*memoryOutputHandles) Delete(context.Context, contract.OrganizationID, contract.ProjectID, string, string) error {
	return nil
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }
