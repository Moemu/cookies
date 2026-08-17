package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type staticGatewayCredential string

func (c staticGatewayCredential) ResolveGatewayCredential(context.Context, string, int64) (string, error) {
	return string(c), nil
}

func TestAdapterGatewayImageAdapterUsesSnapshotAuthAndProtocol(t *testing.T) {
	t.Parallel()
	handles := &memoryOutputHandles{}
	adapter, err := NewAdapterGatewayImageAdapter(staticGatewayCredential("service-token"), handles)
	if err != nil {
		t.Fatalf("NewAdapterGatewayImageAdapter() error = %v", err)
	}
	now := time.Date(2026, time.July, 23, 2, 0, 0, 0, time.UTC)
	adapter.now = func() time.Time { return now }
	adapter.client = &http.Client{Transport: roundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://adapter.example/v1/images/generations" {
			t.Fatalf("request URL = %q", request.URL)
		}
		if request.Header.Get("Authorization") != "Bearer service-token" ||
			request.Header.Get("Idempotency-Key") != "gateway-image-1" ||
			request.Header.Get("X-Request-Id") != "provider_job_gateway_1" {
			t.Fatalf("unexpected gateway headers: %#v", request.Header)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["model"] != "gpt-image-2" || body["size"] != "1024x1024" ||
			body["response_format"] != "b64_json" || body["output_format"] != "png" || body["n"] != float64(1) {
			t.Fatalf("unexpected gateway body: %#v", body)
		}
		response := `{"model":"gpt-image-2","data":[{"b64_json":"` + base64.StdEncoding.EncodeToString(fakeImagePNG) + `"}]}`
		headers := make(http.Header)
		headers.Set("X-Request-Id", "adapter-request-123")
		headers.Set("X-Actual-Provider", "openai")
		return &http.Response{StatusCode: http.StatusOK, Header: headers, Body: io.NopCloser(bytes.NewBufferString(response))}, nil
	})}
	submission, err := adapter.Submit(context.Background(), ImageGenerationRequest{
		OrganizationID: "org_1", ProjectID: "project_1", ProviderJobID: "provider_job_gateway_1",
		ModelAlias: "cookies.image.standard", IdempotencyKey: "gateway-image-1",
		Input: ImageGenerationInput{Prompt: "launch poster", Width: 1024, Height: 1024},
		Route: testGatewayRoute(),
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if submission.ProviderCode != adapterGatewayProviderCode || submission.AdapterRequestID != "adapter-request-123" ||
		submission.ActualProvider != "openai" || submission.ActualModel != "gpt-image-2" || len(submission.Outputs) != 1 {
		t.Fatalf("unexpected submission: %+v", submission)
	}
	if !bytes.Equal(handles.contents, fakeImagePNG) {
		t.Fatal("gateway output was not retained through the opaque handle store")
	}
}

func TestAdapterGatewayImageAdapterTreatsServerErrorAsUnknown(t *testing.T) {
	t.Parallel()
	adapter, _ := NewAdapterGatewayImageAdapter(staticGatewayCredential("service-token"), &memoryOutputHandles{})
	adapter.client = &http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusBadGateway, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(`{"error":{"code":"upstream_error","message":"failed"}}`))}, nil
	})}
	_, err := adapter.Submit(context.Background(), ImageGenerationRequest{
		OrganizationID: "org_1", ProjectID: "project_1", ProviderJobID: "provider_job_gateway_1",
		ModelAlias: "cookies.image.standard", IdempotencyKey: "gateway-image-1",
		Input: ImageGenerationInput{Prompt: "launch poster", Width: 1024, Height: 1024},
		Route: testGatewayRoute(),
	})
	executionError, ok := err.(ExecutionError)
	if !ok || executionError.JobError.Code != "MODEL_SUBMISSION_UNKNOWN" || executionError.JobError.Retryable {
		t.Fatalf("Submit() error = %#v, want non-retryable unknown submission", err)
	}
}

func TestAdapterGatewayImageAdapterClassifiesStructuredImageErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		status    int
		body      string
		wantCode  string
		retryable bool
	}{
		{name: "input text policy", status: http.StatusBadRequest, body: `{"error":{"code":"InputTextSensitiveContentDetected","message":"sensitive input"}}`, wantCode: "MODEL_INPUT_POLICY_REJECTED"},
		{name: "output image policy", status: http.StatusBadRequest, body: `{"error":{"code":"OutputImageSensitiveContentDetected","message":"sensitive output"}}`, wantCode: "MODEL_OUTPUT_POLICY_REJECTED"},
		{name: "generic bad request", status: http.StatusBadRequest, body: `{"error":{"code":"BadRequest","message":"invalid"}}`, wantCode: "MODEL_REQUEST_REJECTED"},
		{name: "quota", status: http.StatusTooManyRequests, body: `{"error":{"code":"QuotaExceeded","message":"queued tasks exceeded"}}`, wantCode: "MODEL_RATE_LIMITED", retryable: true},
	}
	for _, item := range tests {
		item := item
		t.Run(item.name, func(t *testing.T) {
			t.Parallel()
			adapter, _ := NewAdapterGatewayImageAdapter(staticGatewayCredential("service-token"), &memoryOutputHandles{})
			adapter.client = &http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: item.status, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(item.body))}, nil
			})}
			_, err := adapter.Submit(context.Background(), ImageGenerationRequest{
				OrganizationID: "org_1", ProjectID: "project_1", ProviderJobID: "provider_job_gateway_error",
				ModelAlias: "cookies.image.standard", IdempotencyKey: "gateway-image-error",
				Input: ImageGenerationInput{Prompt: "launch poster", Width: 1024, Height: 1024}, Route: testGatewayRoute(),
			})
			executionError, ok := err.(ExecutionError)
			if !ok || executionError.JobError.Code != item.wantCode || executionError.JobError.Retryable != item.retryable {
				t.Fatalf("Submit() error = %#v, want code=%s retryable=%t", err, item.wantCode, item.retryable)
			}
			if strings.Contains(executionError.JobError.Message, "sensitive input") || strings.Contains(executionError.JobError.Message, "sensitive output") {
				t.Fatalf("raw upstream message leaked: %#v", executionError.JobError)
			}
		})
	}
}

func TestAdapterGatewayImageAdapterSerializesSynchronousSubmissions(t *testing.T) {
	var active atomic.Int32
	var maximum atomic.Int32
	adapter, _ := NewAdapterGatewayImageAdapter(staticGatewayCredential("service-token"), &memoryOutputHandles{})
	adapter.client = &http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) {
		current := active.Add(1)
		for observed := maximum.Load(); current > observed && !maximum.CompareAndSwap(observed, current); observed = maximum.Load() {
		}
		time.Sleep(40 * time.Millisecond)
		active.Add(-1)
		response := `{"model":"gpt-image-2","data":[{"b64_json":"` + base64.StdEncoding.EncodeToString(fakeImagePNG) + `"}]}`
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(response))}, nil
	})}

	var group sync.WaitGroup
	for index := 1; index <= 2; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			_, _ = adapter.Submit(context.Background(), ImageGenerationRequest{
				OrganizationID: "org_1", ProjectID: "project_1", ProviderJobID: fmt.Sprintf("provider_job_%d", index),
				ModelAlias: "cookies.image.standard", IdempotencyKey: contract.IdempotencyKey(fmt.Sprintf("gateway-image-%d", index)),
				Input: ImageGenerationInput{Prompt: "launch poster", Width: 1024, Height: 1024}, Route: testGatewayRoute(),
			})
		}(index)
	}
	group.Wait()
	if maximum.Load() != 1 {
		t.Fatalf("synchronous gateway received %d concurrent image submissions, want 1", maximum.Load())
	}
}

func TestAdapterGatewayImageAdapterAllowsHTTPOnlyWithExplicitPolicy(t *testing.T) {
	t.Parallel()
	route := testGatewayRoute()
	route.BaseURL = "http://118.196.44.61:9060"
	request := ImageGenerationRequest{
		OrganizationID: "org_1", ProjectID: "project_1", ProviderJobID: "provider_job_gateway_1",
		ModelAlias: "cookies.image.standard", IdempotencyKey: "gateway-image-1",
		Input: ImageGenerationInput{Prompt: "launch poster", Width: 1024, Height: 1024},
		Route: route,
	}
	secure, _ := NewAdapterGatewayImageAdapter(staticGatewayCredential("service-token"), &memoryOutputHandles{})
	if err := secure.Prepare(context.Background(), request); err == nil {
		t.Fatal("secure adapter accepted an HTTP route")
	}
	local, _ := NewAdapterGatewayImageAdapterWithPolicy(staticGatewayCredential("service-token"), &memoryOutputHandles{}, true)
	if err := local.Prepare(context.Background(), request); err != nil {
		t.Fatalf("explicit local HTTP policy rejected route: %v", err)
	}
}

func TestAdapterGatewayImageAdapterAcceptsFrozenImageTextPortraitProfile(t *testing.T) {
	t.Parallel()
	adapter, err := NewAdapterGatewayImageAdapter(
		staticGatewayCredential("service-token"), &memoryOutputHandles{},
	)
	if err != nil {
		t.Fatalf("NewAdapterGatewayImageAdapter() error = %v", err)
	}
	request := ImageGenerationRequest{
		OrganizationID: "org_1", ProjectID: "project_1",
		ProviderJobID: "provider_job_portrait_1", ModelAlias: "cookies.image.standard",
		IdempotencyKey: "gateway-image-portrait-1",
		Input: ImageGenerationInput{
			Prompt: "portrait editorial product image", Width: 1024, Height: 1536,
		},
		Route: testGatewayRoute(),
	}
	if err := adapter.Prepare(context.Background(), request); err != nil {
		t.Fatalf("portrait Prepare() error = %v", err)
	}
	request.Input.Width = 1000
	if err := adapter.Prepare(context.Background(), request); err == nil {
		t.Fatal("Prepare() accepted a non-supported image size")
	}
}

func testGatewayRoute() *ImageRouteSnapshot {
	return &ImageRouteSnapshot{
		RouteID: "route_1", RouteRevisionID: "route_revision_1",
		ConnectionID: "connection_1", ConnectionRevisionID: "connection_revision_1",
		BaseURL: "https://adapter.example", UpstreamModel: "gpt-image-2",
		CredentialID: "credential_1", CredentialVersion: 3,
		TimeoutSeconds: 210, MaxResponseBytes: 40 << 20,
	}
}
