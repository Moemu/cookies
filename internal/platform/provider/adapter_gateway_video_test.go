package provider

import (
	"bytes"
	"context"
	"encoding/json"
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
