package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	adapterGatewayVideoProviderCode = "adapter_gateway_video"
	adapterGatewayVideoOutputTTL    = 15 * time.Minute
	adapterGatewayVideoMaxBytes     = 200 << 20
)

// AdapterGatewayVideoAdapter executes the database-selected GlobalRouter
// video route. Creative only supplies the stable cookies.video.standard alias;
// endpoint, upstream model and credential remain frozen in the Provider job.
type AdapterGatewayVideoAdapter struct {
	credentials       GatewayCredentialResolver
	handles           OutputHandleStore
	client            *http.Client
	now               func() time.Time
	allowInsecureHTTP bool
}

func NewAdapterGatewayVideoAdapter(credentials GatewayCredentialResolver, handles OutputHandleStore, allowInsecureHTTP bool) (*AdapterGatewayVideoAdapter, error) {
	if credentials == nil || handles == nil {
		return nil, fmt.Errorf("adapter gateway video credential resolver and output handle store are required")
	}
	return &AdapterGatewayVideoAdapter{
		credentials: credentials, handles: handles, client: &http.Client{}, now: time.Now,
		allowInsecureHTTP: allowInsecureHTTP,
	}, nil
}

func (*AdapterGatewayVideoAdapter) ProviderCode() string { return adapterGatewayVideoProviderCode }

func (a *AdapterGatewayVideoAdapter) Submit(ctx context.Context, request VideoGenerationRequest) (VideoSubmission, error) {
	if err := request.Validate(); err != nil {
		return VideoSubmission{}, err
	}
	route, token, err := a.resolveInvocation(ctx, request.Route)
	if err != nil {
		return VideoSubmission{}, err
	}
	payload, err := adapterGatewayVideoPayload(request, route.UpstreamModel)
	if err != nil {
		return VideoSubmission{}, err
	}
	endpoint, err := gatewayRouteURL(route.BaseURL, route.VideoSubmitPath, "/v1/videos/generations")
	if err != nil {
		return VideoSubmission{}, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(route.TimeoutSeconds)*time.Second)
	defer cancel()
	httpRequest, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return VideoSubmission{}, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+token)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("Idempotency-Key", string(request.IdempotencyKey))
	httpRequest.Header.Set("X-Request-Id", request.ProviderJobID)
	response, err := a.client.Do(httpRequest)
	if err != nil {
		return VideoSubmission{}, gatewayExecutionError("MODEL_SUBMISSION_UNKNOWN", "Adapter video submission outcome is unknown and will not be retried automatically")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, route.MaxResponseBytes+1))
	if err != nil || int64(len(body)) > route.MaxResponseBytes {
		return VideoSubmission{}, gatewayExecutionError("MODEL_RESPONSE_INVALID", "Adapter video response exceeded the configured safety limit")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return VideoSubmission{}, mapAdapterGatewayVideoHTTPError(response.StatusCode, body)
	}
	decoded, err := decodeAdapterGatewayVideoResponse(body)
	if err != nil || strings.TrimSpace(decoded.ID) == "" {
		return VideoSubmission{}, gatewayExecutionError("MODEL_RESPONSE_INVALID", "Adapter video response did not contain a task ID")
	}
	model := firstNonEmpty(decoded.Gateway.Model, decoded.Model, route.UpstreamModel)
	return VideoSubmission{Status: VideoSubmissionAccepted, ProviderCode: adapterGatewayVideoProviderCode, ModelVersion: model, ExternalTaskID: decoded.ID}, nil
}

func adapterGatewayVideoPayload(request VideoGenerationRequest, model string) ([]byte, error) {
	body := map[string]any{
		"model": model, "prompt": request.Input.Prompt,
		"duration": request.Input.DurationSeconds, "ratio": request.Input.AspectRatio,
		"resolution": request.Input.Resolution,
	}
	if request.Input.AudioPolicy != "" {
		body["generate_audio"] = request.Input.AudioPolicy == VideoAudioGenerated
	}
	for index, source := range request.Sources {
		contents, err := io.ReadAll(io.LimitReader(source.Content, arkVideoMaxImageBytes+1))
		if err != nil || len(contents) == 0 || int64(len(contents)) > arkVideoMaxImageBytes {
			return nil, gatewayExecutionError("MODEL_INPUT_UNSUPPORTED", fmt.Sprintf("video conditioning image %d could not be read safely", index+1))
		}
		dataURL := "data:" + source.MIMEType + ";base64," + base64.StdEncoding.EncodeToString(contents)
		switch source.Role {
		case VideoConditioningReferenceImage:
			body["image_url"] = dataURL
		case VideoConditioningFirstFrame:
			body["first_frame_url"] = dataURL
		case VideoConditioningLastFrame:
			body["last_frame_url"] = dataURL
		default:
			return nil, gatewayExecutionError("MODEL_INPUT_UNSUPPORTED", "video conditioning role is unsupported by Adapter")
		}
	}
	return json.Marshal(body)
}

func (a *AdapterGatewayVideoAdapter) Poll(ctx context.Context, reference VideoTaskReference) (VideoTaskResult, error) {
	if err := reference.Validate(); err != nil {
		return VideoTaskResult{}, err
	}
	if reference.ProviderCode != adapterGatewayVideoProviderCode {
		return VideoTaskResult{}, fmt.Errorf("Adapter video task reference targets another provider")
	}
	route, token, err := a.resolveInvocation(ctx, reference.Route)
	if err != nil {
		return VideoTaskResult{}, err
	}
	pollPath := route.VideoPollPath
	if pollPath == "" {
		pollPath = "/v1/videos/generations/{task_id}"
	}
	pollPath = strings.ReplaceAll(pollPath, "{task_id}", url.PathEscape(reference.ExternalTaskID))
	endpoint, err := gatewayRouteURL(route.BaseURL, pollPath, "")
	if err != nil {
		return VideoTaskResult{}, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(route.TimeoutSeconds)*time.Second)
	defer cancel()
	httpRequest, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return VideoTaskResult{}, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+token)
	httpRequest.Header.Set("Accept", "application/json")
	response, err := a.client.Do(httpRequest)
	if err != nil {
		return VideoTaskResult{}, fmt.Errorf("poll Adapter video task: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, route.MaxResponseBytes+1))
	if err != nil || int64(len(body)) > route.MaxResponseBytes {
		return VideoTaskResult{}, gatewayExecutionError("MODEL_RESPONSE_INVALID", "Adapter video polling response exceeded the configured safety limit")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return VideoTaskResult{}, mapAdapterGatewayVideoHTTPError(response.StatusCode, body)
	}
	decoded, err := decodeAdapterGatewayVideoResponse(body)
	if err != nil {
		return VideoTaskResult{}, gatewayExecutionError("MODEL_RESPONSE_INVALID", "Adapter video polling response is invalid")
	}
	switch strings.ToLower(strings.TrimSpace(decoded.Status)) {
	case "queued", "accepted", "pending":
		return VideoTaskResult{Status: VideoTaskRunning, Progress: 25}, nil
	case "running", "processing", "in_progress":
		progress := decoded.Progress
		if progress < 1 || progress > 70 {
			progress = 50
		}
		return VideoTaskResult{Status: VideoTaskRunning, Progress: progress}, nil
	case "succeeded", "completed", "success":
		target := decoded.outputURL()
		if target == "" {
			return VideoTaskResult{}, gatewayExecutionError("MODEL_RESPONSE_INVALID", "Adapter video task completed without an output URL")
		}
		contents, err := a.downloadVideo(ctx, target)
		if err != nil {
			return VideoTaskResult{}, err
		}
		ref, err := newOutputRef(adapterGatewayVideoProviderCode, reference.ProviderJobID, "output_1", "video/mp4", contents, a.now().UTC().Add(adapterGatewayVideoOutputTTL))
		if err != nil {
			return VideoTaskResult{}, err
		}
		project := contract.ProjectRef{OrganizationID: reference.OrganizationID, ProjectID: reference.ProjectID, ProjectContextVersion: 1}
		if err := a.handles.Put(ctx, project, ref, contents); err != nil {
			return VideoTaskResult{}, fmt.Errorf("retain Adapter video output: %w", err)
		}
		return VideoTaskResult{Status: VideoTaskSucceeded, Outputs: []contract.ProviderOutputRef{ref}}, nil
	case "failed", "cancelled", "canceled", "expired":
		message := firstNonEmpty(decoded.Error.Message, decoded.FailedReason, "Adapter video generation failed")
		code := firstNonEmpty(decoded.Error.Code, "MODEL_GENERATION_FAILED")
		return VideoTaskResult{Status: VideoTaskFailed, Error: &contract.JobError{Code: code, Message: message, Retryable: false}}, nil
	default:
		return VideoTaskResult{}, gatewayExecutionError("MODEL_RESPONSE_INVALID", "Adapter video task status is invalid")
	}
}

type adapterGatewayVideoResponse struct {
	ID           string `json:"id"`
	TaskID       string `json:"task_id"`
	Status       string `json:"status"`
	Model        string `json:"model"`
	Progress     int    `json:"progress"`
	VideoURL     string `json:"video_url"`
	URL          string `json:"url"`
	FailedReason string `json:"failed_reason"`
	Content      struct {
		VideoURL string   `json:"video_url"`
		URL      string   `json:"url"`
		Results  []string `json:"results"`
	} `json:"content"`
	Output struct {
		VideoURL string `json:"video_url"`
		URL      string `json:"url"`
	} `json:"output"`
	Data []struct {
		URL      string `json:"url"`
		VideoURL string `json:"video_url"`
	} `json:"data"`
	Gateway struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
	} `json:"gateway"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeAdapterGatewayVideoResponse(body []byte) (adapterGatewayVideoResponse, error) {
	var decoded adapterGatewayVideoResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return decoded, err
	}
	if decoded.ID == "" {
		decoded.ID = decoded.TaskID
	}
	return decoded, nil
}

func (r adapterGatewayVideoResponse) outputURL() string {
	values := []string{r.Content.VideoURL, r.Content.URL, r.Output.VideoURL, r.Output.URL, r.VideoURL, r.URL}
	if len(r.Content.Results) > 0 {
		values = append(values, r.Content.Results[0])
	}
	if len(r.Data) > 0 {
		values = append(values, r.Data[0].VideoURL, r.Data[0].URL)
	}
	return firstNonEmpty(values...)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func gatewayRouteURL(baseURL, configuredPath, fallbackPath string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("Adapter gateway video base URL is invalid")
	}
	path := strings.TrimSpace(configuredPath)
	if path == "" {
		path = fallbackPath
	}
	if path == "" || !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("Adapter gateway video endpoint path is invalid")
	}
	parsed.Path, parsed.RawPath, parsed.RawQuery, parsed.Fragment = path, "", "", ""
	return parsed.String(), nil
}

func (a *AdapterGatewayVideoAdapter) resolveInvocation(ctx context.Context, route *VideoRouteSnapshot) (VideoRouteSnapshot, string, error) {
	if route == nil || route.ConnectionType != "adapter_gateway" {
		return VideoRouteSnapshot{}, "", fmt.Errorf("Adapter gateway video route is required")
	}
	if err := route.ValidateVideoWithPolicy(a.allowInsecureHTTP); err != nil {
		return VideoRouteSnapshot{}, "", err
	}
	token, err := a.credentials.ResolveGatewayCredential(ctx, route.CredentialID, route.CredentialVersion)
	if err != nil {
		return VideoRouteSnapshot{}, "", gatewayExecutionError("MODEL_AUTH_UNAVAILABLE", "Adapter video credential could not be resolved")
	}
	return *route, token, nil
}

func (a *AdapterGatewayVideoAdapter) downloadVideo(ctx context.Context, target string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	response, err := a.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download Adapter video output: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Adapter video output returned HTTP %d", response.StatusCode)
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, adapterGatewayVideoMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(contents) < 12 || int64(len(contents)) > adapterGatewayVideoMaxBytes || string(contents[4:8]) != "ftyp" {
		return nil, gatewayExecutionError("MODEL_OUTPUT_UNSUPPORTED", "Adapter video output is not a supported MP4")
	}
	return contents, nil
}

func (a *AdapterGatewayVideoAdapter) Open(ctx context.Context, project contract.ProjectRef, ref contract.ProviderOutputRef) (io.ReadCloser, contract.OutputMetadata, error) {
	if ref.ProviderCode != adapterGatewayVideoProviderCode {
		return nil, contract.OutputMetadata{}, ErrOutputHandleNotFound
	}
	return a.handles.Open(ctx, project, ref)
}

func mapAdapterGatewayVideoHTTPError(status int, body []byte) error {
	var decoded struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &decoded)
	code, message, retryable := decoded.Error.Code, decoded.Error.Message, false
	if code == "" {
		code = "MODEL_REQUEST_REJECTED"
	}
	if message == "" {
		message = fmt.Sprintf("Adapter video request returned HTTP %d", status)
	}
	if status == http.StatusTooManyRequests || status >= 500 {
		retryable = true
	}
	return ExecutionError{JobError: contract.JobError{Code: code, Message: message, Retryable: retryable}}
}
