package provider

import (
	"context"
	"io"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const openAIImageProviderCode = "openai"

// OpenAIImageConfig configures an OpenAI-compatible image gateway. BaseURL
// may be the gateway origin or an already versioned base URL ending in /v1.
type OpenAIImageConfig struct {
	APIKey  string
	Model   string
	BaseURL string
}

// OpenAIImageAdapter owns only the provider-specific OpenAI-compatible wire
// protocol. Its completed outputs still cross into Assets exclusively through
// the existing opaque GeneratedOutputFetcher seam.
type OpenAIImageAdapter struct{ delegate *ArkImageAdapter }

func NewOpenAIImageAdapter(config OpenAIImageConfig, handles OutputHandleStore) (*OpenAIImageAdapter, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	endpointPath := "/v1/images/generations"
	if strings.HasSuffix(baseURL, "/v1") {
		endpointPath = "/images/generations"
	}
	delegate, err := newCompatibleImageAdapter(config.APIKey, config.Model, baseURL, endpointPath, openAIImageProviderCode, "OpenAI-compatible", handles)
	if err != nil {
		return nil, err
	}
	return &OpenAIImageAdapter{delegate: delegate}, nil
}

func (a *OpenAIImageAdapter) Submit(ctx context.Context, request ImageGenerationRequest) (ImageSubmission, error) {
	return a.delegate.Submit(ctx, request)
}

func (a *OpenAIImageAdapter) Poll(ctx context.Context, reference ImageTaskReference) (ImageTaskResult, error) {
	return a.delegate.Poll(ctx, reference)
}

func (a *OpenAIImageAdapter) Open(ctx context.Context, project contract.ProjectRef, ref contract.ProviderOutputRef) (io.ReadCloser, contract.OutputMetadata, error) {
	return a.delegate.Open(ctx, project, ref)
}
