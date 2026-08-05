package provider

import (
	"context"
	"fmt"
	"io"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

// OutputFetcherRouter gives the single Assets worker a stable fetch seam while
// image and video adapters remain independently replaceable.
type OutputFetcherRouter map[string]assets.GeneratedOutputFetcher

type OutputProviderCoder interface {
	ProviderCode() string
}

func NewOutputFetcherRouter(fetchers ...assets.GeneratedOutputFetcher) (OutputFetcherRouter, error) {
	router := make(OutputFetcherRouter, len(fetchers))
	for _, fetcher := range fetchers {
		coded, ok := fetcher.(OutputProviderCoder)
		if !ok || coded.ProviderCode() == "" {
			return nil, fmt.Errorf("generated output fetcher must expose its provider code")
		}
		if _, exists := router[coded.ProviderCode()]; exists {
			return nil, fmt.Errorf("generated output provider code %q is duplicated", coded.ProviderCode())
		}
		router[coded.ProviderCode()] = fetcher
	}
	return router, nil
}

func (r OutputFetcherRouter) Open(ctx context.Context, project contract.ProjectRef, ref contract.ProviderOutputRef) (io.ReadCloser, contract.OutputMetadata, error) {
	fetcher := r[ref.ProviderCode]
	if fetcher == nil {
		return nil, contract.OutputMetadata{}, fmt.Errorf("%w: provider %q", ErrOutputHandleNotFound, ref.ProviderCode)
	}
	return fetcher.Open(ctx, project, ref)
}
