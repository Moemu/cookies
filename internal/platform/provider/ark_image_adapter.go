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
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	arkProviderCode        = "ark"
	arkImageDefaultBaseURL = "https://ark.cn-beijing.volces.com/api/v3"
	arkImageOutputTTL      = 15 * time.Minute
	arkImageMaxBytes       = 20 << 20
)

// ArkImageConfig is deliberately limited to the one M1 capability. Model
// resolution stays server-side; callers can only send a stable model alias.
type ArkImageConfig struct {
	APIKey  string
	Model   string
	BaseURL string
}

// ArkImageAdapter implements Ark's OpenAI-compatible synchronous image API.
// It requests b64_json so vendor URLs never cross the Provider/Assets seam.
type ArkImageAdapter struct {
	apiKey       string
	model        string
	baseURL      string
	endpointPath string
	providerCode string
	providerName string
	client       *http.Client
	handles      OutputHandleStore
	now          func() time.Time
}

func NewArkImageAdapter(config ArkImageConfig, handles OutputHandleStore) (*ArkImageAdapter, error) {
	if strings.TrimSpace(config.APIKey) == "" || strings.TrimSpace(config.Model) == "" || handles == nil {
		return nil, fmt.Errorf("Ark image API key, model, and output handle store are required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		baseURL = arkImageDefaultBaseURL
	}
	return newCompatibleImageAdapter(config.APIKey, config.Model, baseURL, "/images/generations", arkProviderCode, "Ark", handles)
}

func (a *ArkImageAdapter) Submit(ctx context.Context, request ImageGenerationRequest) (ImageSubmission, error) {
	if err := request.Validate(); err != nil {
		return ImageSubmission{}, err
	}
	size, err := arkImageSize(request.Input.Width, request.Input.Height)
	if err != nil {
		return ImageSubmission{}, ExecutionError{JobError: contract.JobError{Code: "MODEL_INPUT_UNSUPPORTED", Message: err.Error(), Retryable: false}}
	}
	payload, err := a.buildRequestPayload(request, size)
	if err != nil {
		return ImageSubmission{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+a.endpointPath, strings.NewReader(string(payload)))
	if err != nil {
		return ImageSubmission{}, fmt.Errorf("build %s image request: %w", a.providerName, err)
	}
	httpRequest.Header.Set("Authorization", "Bearer "+a.apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	response, err := a.client.Do(httpRequest)
	if err != nil {
		return ImageSubmission{}, ExecutionError{JobError: contract.JobError{Code: "MODEL_SUBMISSION_UNKNOWN", Message: fmt.Sprintf("%s image submission outcome is unknown and will not be retried automatically", a.providerName), Retryable: false}}
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		problem := contract.JobError{Code: "MODEL_REQUEST_REJECTED", Message: fmt.Sprintf("%s image request returned HTTP %d", a.providerName, response.StatusCode), Retryable: false}
		// A rate-limit response is a confirmed non-acceptance. Other 5xx
		// responses may have reached the model after the gateway accepted the
		// request, so treating them as retryable could duplicate generation.
		if response.StatusCode == http.StatusTooManyRequests {
			problem = contract.JobError{Code: "MODEL_RATE_LIMITED", Message: fmt.Sprintf("%s image request was rate limited", a.providerName), Retryable: true}
		} else if response.StatusCode >= 500 {
			problem = contract.JobError{Code: "MODEL_SUBMISSION_UNKNOWN", Message: fmt.Sprintf("%s image submission outcome is unknown and will not be retried automatically", a.providerName), Retryable: false}
		}
		return ImageSubmission{}, ExecutionError{JobError: problem}
	}
	var decoded arkImageResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, arkImageMaxBytes*2)).Decode(&decoded); err != nil {
		return ImageSubmission{}, ExecutionError{JobError: contract.JobError{Code: "MODEL_RESPONSE_INVALID", Message: fmt.Sprintf("%s image response could not be verified", a.providerName), Retryable: false}}
	}
	if len(decoded.Data) == 0 {
		return ImageSubmission{}, ExecutionError{JobError: contract.JobError{Code: "MODEL_RESPONSE_INVALID", Message: fmt.Sprintf("%s image response contained no output", a.providerName), Retryable: false}}
	}
	expiresAt := a.now().UTC().Add(arkImageOutputTTL)
	project := contract.ProjectRef{OrganizationID: request.OrganizationID, ProjectID: request.ProjectID, ProjectContextVersion: 1}
	outputs := make([]contract.ProviderOutputRef, 0, len(decoded.Data))
	for index, result := range decoded.Data {
		contents, contentErr := a.readImageResult(ctx, result)
		if contentErr != nil || len(contents) == 0 || len(contents) > arkImageMaxBytes {
			return ImageSubmission{}, ExecutionError{JobError: contract.JobError{Code: "MODEL_OUTPUT_INVALID", Message: fmt.Sprintf("%s image output is invalid or exceeds the supported size", a.providerName), Retryable: false}}
		}
		mimeType := http.DetectContentType(contents)
		if mimeType != "image/png" && mimeType != "image/jpeg" {
			return ImageSubmission{}, ExecutionError{JobError: contract.JobError{Code: "MODEL_OUTPUT_UNSUPPORTED", Message: fmt.Sprintf("%s image output is not a supported PNG or JPEG image", a.providerName), Retryable: false}}
		}
		ref, refErr := NewOutputRef(a.providerCode, request.ProviderJobID, fmt.Sprintf("output_%d", index+1), mimeType, contents, expiresAt)
		if refErr != nil {
			return ImageSubmission{}, ExecutionError{JobError: contract.JobError{Code: "MODEL_OUTPUT_INVALID", Message: fmt.Sprintf("%s image output metadata is invalid", a.providerName), Retryable: false}}
		}
		if putErr := a.handles.Put(ctx, project, ref, contents); putErr != nil {
			return ImageSubmission{}, ExecutionError{JobError: contract.JobError{Code: "MODEL_SUBMISSION_UNKNOWN", Message: fmt.Sprintf("%s image completed but its output could not be retained safely", a.providerName), Retryable: false}}
		}
		outputs = append(outputs, ref)
	}
	modelVersion := strings.TrimSpace(decoded.Model)
	if modelVersion == "" {
		modelVersion = a.model
	}
	return ImageSubmission{Status: ImageSubmissionCompleted, ProviderCode: a.providerCode, ModelVersion: modelVersion, Outputs: outputs}, nil
}

func (a *ArkImageAdapter) buildRequestPayload(request ImageGenerationRequest, size string) ([]byte, error) {
	if len(request.Sources) == 0 {
		payload, err := json.Marshal(struct {
			Model          string `json:"model"`
			Prompt         string `json:"prompt"`
			Size           string `json:"size"`
			ResponseFormat string `json:"response_format"`
		}{Model: a.model, Prompt: request.Input.Prompt, Size: size, ResponseFormat: "b64_json"})
		if err != nil {
			return nil, fmt.Errorf("encode %s image request: %w", a.providerName, err)
		}
		return payload, nil
	}
	images := make([]string, 0, len(request.Sources))
	for index, source := range request.Sources {
		if source.MIMEType != "image/png" && source.MIMEType != "image/jpeg" {
			return nil, ExecutionError{JobError: contract.JobError{Code: "MODEL_INPUT_UNSUPPORTED", Message: fmt.Sprintf("%s image source %d is not a supported PNG or JPEG", a.providerName, index+1), Retryable: false}}
		}
		contents, err := io.ReadAll(io.LimitReader(source.Content, arkImageMaxBytes+1))
		if err != nil || len(contents) == 0 || len(contents) > arkImageMaxBytes {
			return nil, ExecutionError{JobError: contract.JobError{Code: "MODEL_INPUT_UNSUPPORTED", Message: fmt.Sprintf("%s image source %d could not be read safely", a.providerName, index+1), Retryable: false}}
		}
		images = append(images, fmt.Sprintf("data:%s;base64,%s", source.MIMEType, base64.StdEncoding.EncodeToString(contents)))
	}
	payload, err := json.Marshal(struct {
		Model                 string            `json:"model"`
		Prompt                string            `json:"prompt"`
		Image                 []string          `json:"image"`
		Size                  string            `json:"size"`
		ResponseFormat        string            `json:"response_format"`
		OutputFormat          string            `json:"output_format"`
		OptimizePromptOptions map[string]string `json:"optimize_prompt_options"`
	}{Model: a.model, Prompt: request.Input.Prompt, Image: images, Size: size, ResponseFormat: "url", OutputFormat: "png", OptimizePromptOptions: map[string]string{"mode": "standard"}})
	if err != nil {
		return nil, fmt.Errorf("encode %s image edit request: %w", a.providerName, err)
	}
	return payload, nil
}

func (a *ArkImageAdapter) readImageResult(ctx context.Context, result arkImageResult) ([]byte, error) {
	if strings.TrimSpace(result.B64JSON) != "" {
		return base64.StdEncoding.DecodeString(result.B64JSON)
	}
	if strings.TrimSpace(result.URL) == "" {
		return nil, fmt.Errorf("%s image response did not include bytes or URL", a.providerName)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, result.URL, nil)
	if err != nil {
		return nil, err
	}
	response, err := a.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%s image result URL returned HTTP %d", a.providerName, response.StatusCode)
	}
	var buffer bytes.Buffer
	if _, err := io.Copy(&buffer, io.LimitReader(response.Body, arkImageMaxBytes+1)); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func (a *ArkImageAdapter) Poll(context.Context, ImageTaskReference) (ImageTaskResult, error) {
	return ImageTaskResult{}, fmt.Errorf("%s synchronous image generation does not support polling", a.providerName)
}

// Open delegates to the Provider-private store and thereby implements the
// Assets-owned GeneratedOutputFetcher seam without giving Assets DB access.
func (a *ArkImageAdapter) Open(ctx context.Context, project contract.ProjectRef, ref contract.ProviderOutputRef) (io.ReadCloser, contract.OutputMetadata, error) {
	if ref.ProviderCode != a.providerCode {
		return nil, contract.OutputMetadata{}, ErrOutputHandleNotFound
	}
	return a.handles.Open(ctx, project, ref)
}

func newCompatibleImageAdapter(apiKey, model, baseURL, endpointPath, providerCode, providerName string, handles OutputHandleStore) (*ArkImageAdapter, error) {
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(model) == "" || strings.TrimSpace(baseURL) == "" || strings.TrimSpace(endpointPath) == "" || strings.TrimSpace(providerCode) == "" || handles == nil {
		return nil, fmt.Errorf("%s image API key, model, base URL, and output handle store are required", providerName)
	}
	return &ArkImageAdapter{apiKey: apiKey, model: model, baseURL: strings.TrimRight(baseURL, "/"), endpointPath: endpointPath, providerCode: providerCode, providerName: providerName, client: &http.Client{Timeout: 180 * time.Second}, handles: handles, now: time.Now}, nil
}

type arkImageResponse struct {
	Model string           `json:"model"`
	Data  []arkImageResult `json:"data"`
}

type arkImageResult struct {
	B64JSON string `json:"b64_json"`
	URL     string `json:"url"`
}

func arkImageSize(width, height int) (string, error) {
	size := fmt.Sprintf("%dx%d", width, height)
	switch size {
	case "1024x1024", "2048x2048", "1024x768", "768x1024", "1365x1024", "1024x1365":
		return size, nil
	default:
		return "", fmt.Errorf("image dimensions %s are not supported by this image adapter", size)
	}
}
