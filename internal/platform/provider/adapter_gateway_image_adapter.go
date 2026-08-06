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
	adapterGatewayProviderCode = "adapter_gateway"
	adapterGatewayOutputTTL    = 15 * time.Minute
	adapterGatewayMaxImage     = 20 << 20
)

// AdapterGatewayImageAdapter invokes the separately deployed adapter service.
// Endpoint, model and credential identity come only from the durable job
// snapshot; callers cannot override them in the public request.
type AdapterGatewayImageAdapter struct {
	credentials       GatewayCredentialResolver
	handles           OutputHandleStore
	client            *http.Client
	now               func() time.Time
	allowInsecureHTTP bool
	submissions       chan struct{}
}

func NewAdapterGatewayImageAdapter(credentials GatewayCredentialResolver, handles OutputHandleStore) (*AdapterGatewayImageAdapter, error) {
	return NewAdapterGatewayImageAdapterWithPolicy(credentials, handles, false)
}

func NewAdapterGatewayImageAdapterWithPolicy(credentials GatewayCredentialResolver, handles OutputHandleStore, allowInsecureHTTP bool) (*AdapterGatewayImageAdapter, error) {
	if credentials == nil || handles == nil {
		return nil, fmt.Errorf("adapter gateway credential resolver and output handle store are required")
	}
	return &AdapterGatewayImageAdapter{
		credentials:       credentials,
		handles:           handles,
		client:            &http.Client{},
		now:               time.Now,
		allowInsecureHTTP: allowInsecureHTTP,
		submissions:       make(chan struct{}, 1),
	}, nil
}

func (a *AdapterGatewayImageAdapter) Prepare(ctx context.Context, request ImageGenerationRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	if request.Route == nil {
		return fmt.Errorf("adapter gateway route snapshot is required")
	}
	if err := request.Route.ValidateWithPolicy(a.allowInsecureHTTP); err != nil {
		return err
	}
	if !adapterGatewayImageSizeSupported(request.Input.Width, request.Input.Height) {
		return gatewayExecutionError("MODEL_INPUT_UNSUPPORTED", "Adapter gateway image dimensions are unsupported")
	}
	if _, err := a.credentials.ResolveGatewayCredential(ctx, request.Route.CredentialID, request.Route.CredentialVersion); err != nil {
		return gatewayExecutionError("MODEL_AUTH_UNAVAILABLE", "Adapter gateway credential could not be resolved")
	}
	return nil
}

func (a *AdapterGatewayImageAdapter) Submit(ctx context.Context, request ImageGenerationRequest) (ImageSubmission, error) {
	if err := a.Prepare(ctx, request); err != nil {
		return ImageSubmission{}, err
	}
	token, err := a.credentials.ResolveGatewayCredential(ctx, request.Route.CredentialID, request.Route.CredentialVersion)
	if err != nil {
		return ImageSubmission{}, gatewayExecutionError("MODEL_AUTH_UNAVAILABLE", "Adapter gateway credential could not be resolved")
	}
	body, err := json.Marshal(map[string]any{
		"model":           request.Route.UpstreamModel,
		"prompt":          request.Input.Prompt,
		"n":               1,
		"size":            fmt.Sprintf("%dx%d", request.Input.Width, request.Input.Height),
		"response_format": "b64_json",
		"output_format":   "png",
	})
	if err != nil {
		return ImageSubmission{}, err
	}
	select {
	case a.submissions <- struct{}{}:
		defer func() { <-a.submissions }()
	case <-ctx.Done():
		return ImageSubmission{}, ctx.Err()
	}
	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(request.Route.TimeoutSeconds)*time.Second)
	defer cancel()
	endpoint := strings.TrimRight(request.Route.BaseURL, "/")
	if strings.HasSuffix(endpoint, "/v1") {
		endpoint += "/images/generations"
	} else {
		endpoint += "/v1/images/generations"
	}
	httpRequest, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ImageSubmission{}, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+token)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("Idempotency-Key", string(request.IdempotencyKey))
	httpRequest.Header.Set("X-Request-Id", request.ProviderJobID)
	response, err := a.client.Do(httpRequest)
	if err != nil {
		return ImageSubmission{}, gatewayExecutionError("MODEL_SUBMISSION_UNKNOWN", "Adapter gateway submission outcome is unknown and will not be retried automatically")
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, request.Route.MaxResponseBytes+1))
	if err != nil || int64(len(responseBody)) > request.Route.MaxResponseBytes {
		return ImageSubmission{}, gatewayExecutionError("MODEL_RESPONSE_INVALID", "Adapter gateway response exceeded the configured safety limit")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return ImageSubmission{}, mapGatewayImageHTTPError(response.StatusCode)
	}
	var decoded struct {
		Model string `json:"model"`
		Data  []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &decoded); err != nil || len(decoded.Data) != 1 {
		return ImageSubmission{}, gatewayExecutionError("MODEL_RESPONSE_INVALID", "Adapter gateway response did not contain exactly one verifiable output")
	}
	contents, err := base64.StdEncoding.DecodeString(decoded.Data[0].B64JSON)
	if err != nil || len(contents) == 0 || len(contents) > adapterGatewayMaxImage {
		return ImageSubmission{}, gatewayExecutionError("MODEL_OUTPUT_INVALID", "Adapter gateway image output is invalid or too large")
	}
	mimeType := http.DetectContentType(contents)
	if mimeType != "image/png" && mimeType != "image/jpeg" && mimeType != "image/webp" {
		return ImageSubmission{}, gatewayExecutionError("MODEL_OUTPUT_UNSUPPORTED", "Adapter gateway output is not PNG, JPEG, or WebP")
	}
	expiresAt := a.now().UTC().Add(adapterGatewayOutputTTL)
	ref, err := NewOutputRef(adapterGatewayProviderCode, request.ProviderJobID, "output_1", mimeType, contents, expiresAt)
	if err != nil {
		return ImageSubmission{}, gatewayExecutionError("MODEL_OUTPUT_INVALID", "Adapter gateway output metadata is invalid")
	}
	project := contract.ProjectRef{OrganizationID: request.OrganizationID, ProjectID: request.ProjectID, ProjectContextVersion: 1}
	if err := a.handles.Put(ctx, project, ref, contents); err != nil {
		return ImageSubmission{}, gatewayExecutionError("MODEL_SUBMISSION_UNKNOWN", "Adapter gateway completed generation but its output could not be retained")
	}
	actualModel := strings.TrimSpace(decoded.Model)
	if actualModel == "" {
		actualModel = request.Route.UpstreamModel
	}
	requestID := strings.TrimSpace(response.Header.Get("X-Request-Id"))
	return ImageSubmission{
		Status:           ImageSubmissionCompleted,
		ProviderCode:     adapterGatewayProviderCode,
		ModelVersion:     actualModel,
		Outputs:          []contract.ProviderOutputRef{ref},
		AdapterRequestID: requestID,
		ActualProvider:   strings.TrimSpace(response.Header.Get("X-Actual-Provider")),
		ActualModel:      actualModel,
	}, nil
}

func adapterGatewayImageSizeSupported(width, height int) bool {
	return (width == 1024 && height == 1024) ||
		(width == 1024 && height == 1536) ||
		(width == 1536 && height == 1024)
}

func (a *AdapterGatewayImageAdapter) Poll(context.Context, ImageTaskReference) (ImageTaskResult, error) {
	return ImageTaskResult{}, fmt.Errorf("adapter gateway image generation is synchronous and does not support polling")
}

func (a *AdapterGatewayImageAdapter) Open(ctx context.Context, project contract.ProjectRef, ref contract.ProviderOutputRef) (io.ReadCloser, contract.OutputMetadata, error) {
	if ref.ProviderCode != adapterGatewayProviderCode {
		return nil, contract.OutputMetadata{}, ErrOutputHandleNotFound
	}
	return a.handles.Open(ctx, project, ref)
}

func (*AdapterGatewayImageAdapter) ProviderCode() string { return adapterGatewayProviderCode }

func mapGatewayImageHTTPError(status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return gatewayExecutionError("MODEL_AUTH_REJECTED", "Adapter gateway rejected its service credential")
	case http.StatusBadRequest:
		return gatewayExecutionError("MODEL_REQUEST_REJECTED", "Adapter gateway rejected the image request")
	case http.StatusUnprocessableEntity:
		return gatewayExecutionError("MODEL_INPUT_UNSUPPORTED", "Adapter gateway could not process the image request")
	case http.StatusTooManyRequests:
		// Unless the adapter explicitly proves non-acceptance, retrying a
		// synchronous generation can duplicate cost.
		return gatewayExecutionError("MODEL_RATE_LIMITED", "Adapter gateway rate limited the image request")
	default:
		if status >= 500 {
			return gatewayExecutionError("MODEL_SUBMISSION_UNKNOWN", "Adapter gateway submission outcome is unknown and will not be retried automatically")
		}
		return gatewayExecutionError("MODEL_REQUEST_REJECTED", fmt.Sprintf("Adapter gateway returned HTTP %d", status))
	}
}

func mapGatewayTextHTTPError(status int, responseBody []byte) error {
	if status == http.StatusTooManyRequests && gatewayRejectedTextParameters(responseBody) {
		return gatewayExecutionError("MODEL_REQUEST_REJECTED", "Text model rejected the configured request parameters")
	}
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return gatewayExecutionError("MODEL_AUTH_REJECTED", "Adapter gateway rejected its service credential")
	case http.StatusBadRequest:
		return gatewayExecutionError("MODEL_REQUEST_REJECTED", "Adapter gateway rejected the text request")
	case http.StatusUnprocessableEntity:
		return gatewayExecutionError("MODEL_INPUT_UNSUPPORTED", "Adapter gateway could not process the text request")
	case http.StatusTooManyRequests:
		return ExecutionError{JobError: contract.JobError{
			Code: "MODEL_RATE_LIMITED", Message: "Adapter gateway rate limited the text request", Retryable: true,
		}}
	default:
		if status >= 500 {
			return gatewayExecutionError("MODEL_SUBMISSION_UNKNOWN", "Adapter gateway submission outcome is unknown and will not be retried automatically")
		}
		return gatewayExecutionError("MODEL_REQUEST_REJECTED", fmt.Sprintf("Adapter gateway returned HTTP %d", status))
	}
}

func gatewayRejectedTextParameters(responseBody []byte) bool {
	var payload struct {
		Error struct {
			Message      string `json:"message"`
			ProviderCode string `json:"provider_code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return false
	}
	providerCode := strings.ToLower(strings.TrimSpace(payload.Error.ProviderCode))
	message := strings.ToLower(strings.TrimSpace(payload.Error.Message))
	return providerCode == "invalidparameter" ||
		providerCode == "invalid_parameter" ||
		strings.Contains(message, "unsupported thinking type")
}

func gatewayExecutionError(code, message string) error {
	return ExecutionError{JobError: contract.JobError{Code: code, Message: message, Retryable: false}}
}
