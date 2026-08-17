package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestAdapterGatewayVideoSubmitsPollsAndCachesOutput(t *testing.T) {
	t.Parallel()
	handles := &memoryOutputHandles{}
	adapter, err := NewAdapterGatewayVideoAdapter(staticGatewayCredential("service-token"), handles, true)
	if err != nil {
		t.Fatal(err)
	}
	polls := 0
	adapter.client = &http.Client{Transport: roundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "adapter.example" && request.Header.Get("Authorization") != "Bearer service-token" {
			t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
		}
		switch {
		case request.Method == http.MethodPost && request.URL.String() == "http://adapter.example/v1/videos/generations":
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["model"] != "doubao-seedance-2-0-fast-260128" || body["duration"] != float64(5) || body["ratio"] != "9:16" {
				t.Fatalf("body = %#v", body)
			}
			if body["input_mode"] != "text_only" || body["content"] != nil {
				t.Fatalf("text-only body must not contain media content: %#v", body)
			}
			return jsonHTTPResponse(http.StatusAccepted, `{"id":"task_adapter_video_1","status":"queued","gateway":{"provider":"globalrouter_main","model":"doubao-seedance-2-0-fast-260128"}}`), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://adapter.example/v1/videos/generations/task_adapter_video_1":
			polls++
			if polls == 1 {
				return jsonHTTPResponse(http.StatusOK, `{"id":"task_adapter_video_1","status":"running"}`), nil
			}
			return jsonHTTPResponse(http.StatusOK, `{"id":"task_adapter_video_1","status":"completed","content":{"video_url":"https://cdn.example/output.mp4"}}`), nil
		case request.Method == http.MethodGet && request.URL.String() == "https://cdn.example/output.mp4":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(fakeVideoMP4)), Header: make(http.Header)}, nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
			return nil, nil
		}
	})}
	route := &VideoRouteSnapshot{
		RouteID: "route_video", RouteRevisionID: "route_video_r1", ConnectionID: "connection_adapter", ConnectionRevisionID: "connection_adapter_r1",
		ConnectionType: "adapter_gateway", BaseURL: "http://adapter.example/v1", UpstreamModel: "doubao-seedance-2-0-fast-260128",
		CredentialID: "credential_adapter", CredentialVersion: 1, TimeoutSeconds: 120, MaxResponseBytes: 4 << 20,
		VideoSubmitPath: "/v1/videos/generations", VideoPollPath: "/v1/videos/generations/{task_id}",
		VideoInputModes: []VideoInputMode{VideoInputTextOnly}, VideoAudioPolicies: []VideoAudioPolicy{VideoAudioSilent},
	}
	submission, err := adapter.Submit(context.Background(), VideoGenerationRequest{
		OrganizationID: "org_1", ProjectID: "project_1", ProviderJobID: "provider_video_1", ModelAlias: "cookies.video.standard",
		IdempotencyKey: "adapter-video-1", Input: VideoGenerationInput{Prompt: "brand film", DurationSeconds: 5, AspectRatio: "9:16", Resolution: "720p", AudioPolicy: VideoAudioSilent}, Route: route,
	})
	if err != nil || submission.ExternalTaskID != "task_adapter_video_1" || submission.ProviderCode != adapterGatewayVideoProviderCode {
		t.Fatalf("Submit() = %+v, %v", submission, err)
	}
	reference := VideoTaskReference{OrganizationID: "org_1", ProjectID: "project_1", ProviderJobID: "provider_video_1", ProviderCode: submission.ProviderCode, ModelAlias: "cookies.video.standard", ModelVersion: submission.ModelVersion, ExternalTaskID: submission.ExternalTaskID, Route: route}
	if result, err := adapter.Poll(context.Background(), reference); err != nil || result.Status != VideoTaskRunning {
		t.Fatalf("first Poll() = %+v, %v", result, err)
	}
	result, err := adapter.Poll(context.Background(), reference)
	if err != nil || result.Status != VideoTaskSucceeded || len(result.Outputs) != 1 || !bytes.Equal(handles.contents, fakeVideoMP4) {
		t.Fatalf("second Poll() = %+v, %v", result, err)
	}
	stream, _, err := adapter.Open(context.Background(), contract.ProjectRef{OrganizationID: "org_1", ProjectID: "project_1", ProjectContextVersion: 1}, result.Outputs[0])
	if err != nil {
		t.Fatal(err)
	}
	_ = stream.Close()
}

func TestAdapterGatewayVideoSubmitsConditioningImagesThroughUnifiedContent(t *testing.T) {
	t.Parallel()
	handles := &memoryOutputHandles{}
	adapter, err := NewAdapterGatewayVideoAdapter(staticGatewayCredential("service-token"), handles, true)
	if err != nil {
		t.Fatal(err)
	}
	reference := contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "reference_board", Version: 3}}
	adapter.client = &http.Client{Transport: roundTripper(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.String() != "http://adapter.example/v1/videos/generations" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
		}
		var body struct {
			InputMode string `json:"input_mode"`
			ImageURL  any    `json:"image_url"`
			Content   []struct {
				Type     string `json:"type"`
				Role     string `json:"role"`
				ImageURL struct {
					URL string `json:"url"`
				} `json:"image_url"`
			} `json:"content"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.InputMode != "reference_image" {
			t.Fatalf("input_mode = %q", body.InputMode)
		}
		if body.ImageURL != nil {
			t.Fatalf("legacy top-level image_url must not be sent: %#v", body.ImageURL)
		}
		if len(body.Content) != 1 || body.Content[0].Type != "image_url" || body.Content[0].Role != "reference_image" || body.Content[0].ImageURL.URL != "data:image/png;base64,cmVmZXJlbmNlLWJvYXJk" {
			t.Fatalf("content = %#v", body.Content)
		}
		return jsonHTTPResponse(http.StatusAccepted, `{"id":"task_adapter_reference_1","status":"queued"}`), nil
	})}
	route := &VideoRouteSnapshot{
		RouteID: "route_video", RouteRevisionID: "route_video_r2", ConnectionID: "connection_adapter", ConnectionRevisionID: "connection_adapter_r1",
		ConnectionType: "adapter_gateway", BaseURL: "http://adapter.example/v1", UpstreamModel: "doubao-seedance-2-0-fast-260128",
		CredentialID: "credential_adapter", CredentialVersion: 1, TimeoutSeconds: 120, MaxResponseBytes: 4 << 20,
		VideoSubmitPath: "/v1/videos/generations", VideoPollPath: "/v1/videos/generations/{task_id}",
		VideoInputModes: []VideoInputMode{VideoInputReferenceImage}, VideoAudioPolicies: []VideoAudioPolicy{VideoAudioGenerated},
	}
	submission, err := adapter.Submit(context.Background(), VideoGenerationRequest{
		OrganizationID: "org_1", ProjectID: "project_1", ProviderJobID: "provider_video_reference_1", ModelAlias: "cookies.video.standard",
		IdempotencyKey: "adapter-video-reference-1",
		Input: VideoGenerationInput{
			Prompt: "cinematic fictional character", DurationSeconds: 15, AspectRatio: "9:16", Resolution: "720p",
			AudioPolicy: VideoAudioGenerated, InputMode: VideoInputReferenceImage,
			ConditioningAssets: []VideoConditioningAsset{{Role: VideoConditioningReferenceImage, Reference: reference}},
		},
		Sources: []VideoSource{{Role: VideoConditioningReferenceImage, Reference: reference, MIMEType: "image/png", Content: io.NopCloser(bytes.NewBufferString("reference-board"))}},
		Route:   route,
	})
	if err != nil || submission.ExternalTaskID != "task_adapter_reference_1" {
		t.Fatalf("Submit() = %+v, %v", submission, err)
	}
}

func TestAdapterGatewayVideoClassifiesReferenceImagePrivacyRejection(t *testing.T) {
	t.Parallel()
	adapter, err := NewAdapterGatewayVideoAdapter(staticGatewayCredential("service-token"), &memoryOutputHandles{}, true)
	if err != nil {
		t.Fatal(err)
	}
	adapter.client = &http.Client{Transport: roundTripper(func(request *http.Request) (*http.Response, error) {
		return jsonHTTPResponse(http.StatusBadRequest, `{"error":{"code":"content_rejected","message":"The request failed because the input image 'content[1]' may contain real person."}}`), nil
	})}
	route := &VideoRouteSnapshot{
		RouteID: "route_video", RouteRevisionID: "route_video_r3", ConnectionID: "connection_adapter", ConnectionRevisionID: "connection_adapter_r1",
		ConnectionType: "adapter_gateway", BaseURL: "http://adapter.example/v1", UpstreamModel: "doubao-seedance-2-0-fast-260128",
		CredentialID: "credential_adapter", CredentialVersion: 1, TimeoutSeconds: 120, MaxResponseBytes: 4 << 20,
		VideoSubmitPath: "/v1/videos/generations", VideoPollPath: "/v1/videos/generations/{task_id}",
		VideoInputModes: []VideoInputMode{VideoInputReferenceImage}, VideoAudioPolicies: []VideoAudioPolicy{VideoAudioGenerated},
	}
	_, err = adapter.Submit(context.Background(), VideoGenerationRequest{
		OrganizationID: "org_1", ProjectID: "project_1", ProviderJobID: "provider_video_reference_rejected", ModelAlias: "cookies.video.standard",
		IdempotencyKey: "adapter-video-reference-rejected",
		Input: VideoGenerationInput{
			Prompt: "fictional character", DurationSeconds: 15, AspectRatio: "9:16", Resolution: "720p", AudioPolicy: VideoAudioGenerated, InputMode: VideoInputReferenceImage,
			ConditioningAssets: []VideoConditioningAsset{{Role: VideoConditioningReferenceImage, Reference: contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "reference_board", Version: 1}}}},
		},
		Sources: []VideoSource{{Role: VideoConditioningReferenceImage, Reference: contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "reference_board", Version: 1}}, MIMEType: "image/png", Content: io.NopCloser(bytes.NewBufferString("reference-board"))}},
		Route:   route,
	})
	var executionError ExecutionError
	if !errors.As(err, &executionError) {
		t.Fatalf("Submit() error = %T %v", err, err)
	}
	if executionError.JobError.Code != "REFERENCE_ASSET_CONTENT_REJECTED" || executionError.JobError.Retryable {
		t.Fatalf("JobError = %+v", executionError.JobError)
	}
}

func TestAdapterGatewayVideoClassifiesAsyncReferenceImagePrivacyRejection(t *testing.T) {
	t.Parallel()
	adapter, err := NewAdapterGatewayVideoAdapter(staticGatewayCredential("service-token"), &memoryOutputHandles{}, true)
	if err != nil {
		t.Fatal(err)
	}
	adapter.client = &http.Client{Transport: roundTripper(func(request *http.Request) (*http.Response, error) {
		return jsonHTTPResponse(http.StatusOK, `{"id":"task_reference_rejected","status":"failed","error":{"code":"content_rejected","message":"The request failed because the input image 'content[1]' may contain real person."}}`), nil
	})}
	route := &VideoRouteSnapshot{
		RouteID: "route_video", RouteRevisionID: "route_video_r4", ConnectionID: "connection_adapter", ConnectionRevisionID: "connection_adapter_r1",
		ConnectionType: "adapter_gateway", BaseURL: "http://adapter.example/v1", UpstreamModel: "doubao-seedance-2-0-fast-260128",
		CredentialID: "credential_adapter", CredentialVersion: 1, TimeoutSeconds: 120, MaxResponseBytes: 4 << 20,
		VideoSubmitPath: "/v1/videos/generations", VideoPollPath: "/v1/videos/generations/{task_id}",
	}
	result, err := adapter.Poll(context.Background(), VideoTaskReference{
		OrganizationID: "org_1", ProjectID: "project_1", ProviderJobID: "provider_video_reference_rejected", ProviderCode: adapterGatewayVideoProviderCode,
		ModelAlias: "cookies.video.standard", ModelVersion: "doubao-seedance-2-0-fast-260128", ExternalTaskID: "task_reference_rejected", Route: route,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != VideoTaskFailed || result.Error == nil || result.Error.Code != "REFERENCE_ASSET_CONTENT_REJECTED" || result.Error.Retryable {
		t.Fatalf("Poll() = %#v", result)
	}
}
