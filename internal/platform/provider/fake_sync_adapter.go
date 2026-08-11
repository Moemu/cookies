package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const fakeTextModelVersion = "fake-text-v1"
const fakeVisionModelVersion = "fake-vision-v1"

// FakeSyncAdapter supports consumer-contract tests and local wiring. It is
// deterministic, never calls a network, and never exposes a vendor-shaped
// response.
type FakeSyncAdapter struct{}

func (FakeSyncAdapter) GenerateText(_ context.Context, request TextAdapterRequest) (SynchronousResult, error) {
	if strings.TrimSpace(request.ModelAlias) == "" || len(request.Messages) == 0 {
		return SynchronousResult{}, fmt.Errorf("fake text request is invalid")
	}
	return SynchronousResult{ProviderCode: fakeProviderCode, ModelVersion: fakeTextModelVersion, Text: "Fake text response"}, nil
}

func (FakeSyncAdapter) InspectTextRoute(_ context.Context, _ contract.OrganizationID, modelAlias string) (TextRouteInspection, error) {
	if strings.TrimSpace(modelAlias) == "" {
		return TextRouteInspection{}, fmt.Errorf("fake text model alias is required")
	}
	return TextRouteInspection{
		ModelAlias: modelAlias, UpstreamModel: fakeTextModelVersion,
		RouteRevisionID: "fake-route-v1", ResponseMode: TextResponsePromptJSON, Ready: true,
	}, nil
}

func (FakeSyncAdapter) InspectVisionRoute(_ context.Context, _ contract.OrganizationID, modelAlias string) (CapabilityRouteInspection, error) {
	if strings.TrimSpace(modelAlias) == "" {
		return CapabilityRouteInspection{}, fmt.Errorf("fake vision model alias is required")
	}
	return CapabilityRouteInspection{
		ModelAlias: modelAlias, UpstreamModel: fakeVisionModelVersion,
		RouteRevisionID: "fake-vision-route-v1", Ready: true,
	}, nil
}

func (FakeSyncAdapter) UnderstandVision(_ context.Context, request VisionAdapterRequest) (SynchronousResult, error) {
	if request.OrganizationID == "" || strings.TrimSpace(request.ModelAlias) == "" || len(request.Input.SourceAssets) == 0 || len(request.Sources) != len(request.Input.SourceAssets) {
		return SynchronousResult{}, fmt.Errorf("fake vision request is invalid")
	}
	return SynchronousResult{ProviderCode: fakeProviderCode, ModelVersion: fakeVisionModelVersion, Text: "Fake vision analysis"}, nil
}
