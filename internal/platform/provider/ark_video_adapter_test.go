package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestArkVideoAdapterSubmitsPollsAndCachesMP4(t *testing.T) {
	t.Parallel()
	handles := &memoryOutputHandles{}
	adapter, err := NewArkVideoAdapter(ArkVideoConfig{
		APIKey: "test-key", Model: "doubao-seedance-2-0-fast-260128",
	}, handles)
	if err != nil {
		t.Fatalf("NewArkVideoAdapter() error = %v", err)
	}
	now := time.Date(2026, time.July, 27, 9, 0, 0, 0, time.UTC)
	adapter.now = func() time.Time { return now }
	polls := 0
	adapter.client = &http.Client{Transport: roundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "ark.cn-beijing.volces.com" && request.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/v3/contents/generations/tasks":
			var body struct {
				Model   string `json:"model"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
				Duration   int    `json:"duration"`
				Ratio      string `json:"ratio"`
				Resolution string `json:"resolution"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatalf("decode create request: %v", err)
			}
			if body.Model != "doubao-seedance-2-0-fast-260128" || len(body.Content) != 1 || body.Content[0].Type != "text" || body.Duration != 5 || body.Ratio != "9:16" || body.Resolution != "720p" {
				t.Fatalf("unexpected create request: %+v", body)
			}
			return jsonHTTPResponse(http.StatusOK, `{"id":"task_ark_video_1","status":"queued"}`), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/v3/contents/generations/tasks/task_ark_video_1":
			polls++
			if polls == 1 {
				return jsonHTTPResponse(http.StatusOK, `{"id":"task_ark_video_1","model":"doubao-seedance-2-0-fast-260128","status":"running"}`), nil
			}
			return jsonHTTPResponse(http.StatusOK, `{"id":"task_ark_video_1","model":"doubao-seedance-2-0-fast-260128","status":"succeeded","content":{"video_url":"https://video.example/output.mp4"}}`), nil
		case request.Method == http.MethodGet && request.URL.String() == "https://video.example/output.mp4":
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"video/mp4"}}, Body: io.NopCloser(bytes.NewReader(fakeVideoMP4))}, nil
		default:
			t.Fatalf("unexpected Ark request: %s %s", request.Method, request.URL)
			return nil, nil
		}
	})}

	submission, err := adapter.Submit(context.Background(), VideoGenerationRequest{
		OrganizationID: "org_1", ProjectID: "project_1", ProviderJobID: "provider_video_1",
		ModelAlias: "cookies.video.standard", IdempotencyKey: "ark-video-1",
		Input: VideoGenerationInput{Prompt: "five-second product pre-roll", DurationSeconds: 5, AspectRatio: "9:16", Resolution: "720p"},
	})
	if err != nil || submission.Status != VideoSubmissionAccepted || submission.ExternalTaskID != "task_ark_video_1" {
		t.Fatalf("Submit() = %+v, %v", submission, err)
	}
	reference := VideoTaskReference{
		OrganizationID: "org_1", ProjectID: "project_1", ProviderJobID: "provider_video_1",
		ProviderCode: arkVideoProviderCode, ModelAlias: "cookies.video.standard", ModelVersion: submission.ModelVersion, ExternalTaskID: submission.ExternalTaskID,
	}
	running, err := adapter.Poll(context.Background(), reference)
	if err != nil || running.Status != VideoTaskRunning {
		t.Fatalf("first Poll() = %+v, %v", running, err)
	}
	completed, err := adapter.Poll(context.Background(), reference)
	if err != nil || completed.Status != VideoTaskSucceeded || len(completed.Outputs) != 1 {
		t.Fatalf("second Poll() = %+v, %v", completed, err)
	}
	output := completed.Outputs[0]
	if output.DeclaredMIMEType != "video/mp4" || !bytes.Equal(handles.contents, fakeVideoMP4) {
		t.Fatalf("output = %+v cached=%x", output, handles.contents)
	}
	stream, metadata, err := adapter.Open(context.Background(), contract.ProjectRef{OrganizationID: "org_1", ProjectID: "project_1", ProjectContextVersion: 7}, output)
	if err != nil || metadata.MIMEType != "video/mp4" {
		t.Fatalf("Open() metadata=%+v err=%v", metadata, err)
	}
	_ = stream.Close()
}

func TestArkVideoAdapterEncodesConfirmedFirstAndLastFrames(t *testing.T) {
	t.Parallel()
	adapter, err := NewArkVideoAdapter(ArkVideoConfig{
		APIKey: "test-key", Model: "doubao-seedance-2-0-fast-260128",
	}, &memoryOutputHandles{})
	if err != nil {
		t.Fatalf("NewArkVideoAdapter() error = %v", err)
	}
	firstRef := contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "asset_first", Version: 1}}
	lastRef := contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "asset_last", Version: 2}}
	adapter.client = &http.Client{Transport: roundTripper(func(request *http.Request) (*http.Response, error) {
		var body struct {
			Content []struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				Role     string `json:"role"`
				ImageURL struct {
					URL string `json:"url"`
				} `json:"image_url"`
			} `json:"content"`
			GenerateAudio *bool `json:"generate_audio"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode create request: %v", err)
		}
		if len(body.Content) != 3 || body.Content[0].Type != "text" {
			t.Fatalf("content = %+v", body.Content)
		}
		if body.Content[1].Role != "first_frame" || body.Content[1].ImageURL.URL != "data:image/png;base64,Zmlyc3Q=" {
			t.Fatalf("first frame content = %+v", body.Content[1])
		}
		if body.Content[2].Role != "last_frame" || body.Content[2].ImageURL.URL != "data:image/png;base64,bGFzdA==" {
			t.Fatalf("last frame content = %+v", body.Content[2])
		}
		if body.GenerateAudio == nil || *body.GenerateAudio {
			t.Fatalf("generate_audio = %v, want false", body.GenerateAudio)
		}
		return jsonHTTPResponse(http.StatusOK, `{"id":"task_first_last","status":"queued"}`), nil
	})}

	submission, err := adapter.Submit(context.Background(), VideoGenerationRequest{
		OrganizationID: "org_1", ProjectID: "project_1", ProviderJobID: "provider_video_first_last",
		ModelAlias: "cookies.video.standard", IdempotencyKey: "ark-video-first-last-1",
		Input: VideoGenerationInput{
			Prompt:          "one wipe reveals the exact product",
			DurationSeconds: 6,
			AspectRatio:     "9:16",
			Resolution:      "720p",
			AudioPolicy:     VideoAudioSilent,
			InputMode:       VideoInputFirstLastFrame,
			ConditioningAssets: []VideoConditioningAsset{
				{Role: VideoConditioningFirstFrame, Reference: firstRef},
				{Role: VideoConditioningLastFrame, Reference: lastRef},
			},
		},
		Sources: []VideoSource{
			{Role: VideoConditioningFirstFrame, Reference: firstRef, MIMEType: "image/png", Content: io.NopCloser(bytes.NewBufferString("first"))},
			{Role: VideoConditioningLastFrame, Reference: lastRef, MIMEType: "image/png", Content: io.NopCloser(bytes.NewBufferString("last"))},
		},
	})
	if err != nil || submission.ExternalTaskID != "task_first_last" {
		t.Fatalf("Submit() = %+v, %v", submission, err)
	}
}

func TestEncodeArkVideoContentUsesAuthorizedAssetURIWithoutUploadingBytes(t *testing.T) {
	t.Parallel()
	first := contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "first", Version: 1}}
	input := VideoGenerationInput{
		Prompt: "authorized actor", DurationSeconds: 6, AspectRatio: "9:16", Resolution: "720p", InputMode: VideoInputReferenceImage,
		ConditioningAssets: []VideoConditioningAsset{{Role: VideoConditioningReferenceImage, Reference: first, AuthorizedAsset: &VideoAuthorizedAssetReference{ProviderCode: arkVideoProviderCode, AssetID: "asset-20260222234430-mxpgh"}}},
	}
	content, err := encodeArkVideoContent(VideoGenerationRequest{
		OrganizationID: "org_1", ProjectID: "project_1", ProviderJobID: "job_1", ModelAlias: "cookies.video.standard", IdempotencyKey: "authorized-asset-1",
		Input: input, Sources: []VideoSource{{Role: VideoConditioningReferenceImage, Reference: first, AuthorizedAsset: input.ConditioningAssets[0].AuthorizedAsset}},
	})
	if err != nil {
		t.Fatalf("encode authorized content: %v", err)
	}
	if len(content) != 2 || content[1].ImageURL == nil || content[1].ImageURL.URL != "asset://asset-20260222234430-mxpgh" || content[1].Role != "reference_image" {
		t.Fatalf("authorized content = %#v", content)
	}
}

func TestArkVideoHTTPErrorPreservesSafeUpstreamDetails(t *testing.T) {
	t.Parallel()

	got := arkVideoHTTPError(
		"submission",
		http.StatusBadRequest,
		[]byte(`{"error":{"code":"InvalidParameter","message":"duration is not supported"}}`),
	)

	if got.JobError.Code != "InvalidParameter" {
		t.Fatalf("code = %q, want InvalidParameter", got.JobError.Code)
	}
	if got.JobError.Message != "duration is not supported" {
		t.Fatalf("message = %q, want upstream message", got.JobError.Message)
	}
	if got.JobError.Retryable {
		t.Fatal("400 response must not be retryable")
	}
}

func jsonHTTPResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewBufferString(body))}
}
