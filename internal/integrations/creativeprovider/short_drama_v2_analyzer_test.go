package creativeprovider

import (
	"context"
	"io"
	"testing"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

type shortDramaV2AssetOpenerStub struct{}

func (shortDramaV2AssetOpenerStub) OpenPreview(context.Context, contract.ActorContext, contract.ProjectID, contract.AssetVersionRef) (io.ReadCloser, assets.ObjectInfo, error) {
	return nil, assets.ObjectInfo{}, nil
}

type shortDramaV2TextRouteStub struct{}

func (shortDramaV2TextRouteStub) ResolveTextRoute(context.Context, contract.OrganizationID, string) (provider.GatewayRouteSnapshot, error) {
	return provider.GatewayRouteSnapshot{}, nil
}

type shortDramaV2CredentialStub struct{}

func (shortDramaV2CredentialStub) ResolveGatewayCredential(context.Context, string, int64) (string, error) {
	return "", nil
}

func TestNewShortDramaV2AnalyzerUsesNormalizedViralAnalyzerConfig(t *testing.T) {
	t.Parallel()

	analyzer, err := NewShortDramaV2Analyzer(ViralAnalyzerConfig{
		Assets: shortDramaV2AssetOpenerStub{}, Routes: shortDramaV2TextRouteStub{}, Credentials: shortDramaV2CredentialStub{},
		FFmpegPath: "ffmpeg", ModelAlias: "cookies.text.standard", PromptVersion: "short-drama-analysis/v1",
		ASR: ASRConfig{Endpoint: "https://example.com/asr"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if analyzer.config.Client == nil {
		t.Fatal("short drama analyzer did not inherit the normalized HTTP client")
	}
	if analyzer.config.WorkRoot != ".data/video-work" {
		t.Fatalf("short drama analyzer did not inherit the normalized work root: %q", analyzer.config.WorkRoot)
	}
}
