package media

import (
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestVideoNormalizationRequestRejectsOddCanvas(t *testing.T) {
	t.Parallel()

	request := VideoNormalizationRequest{
		OrganizationID: "org_1", ProjectID: "project_1",
		SourceVideo: contract.AssetVersionRef{AssetID: "asset_1", Version: 1}, Width: 1280, Height: 545, FrameRate: 25,
	}
	if err := request.Validate(); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestVideoNormalizationRequestAcceptsSourceRatioCanvas(t *testing.T) {
	t.Parallel()

	request := VideoNormalizationRequest{
		OrganizationID: "org_1", ProjectID: "project_1",
		SourceVideo: contract.AssetVersionRef{AssetID: "asset_1", Version: 1}, Width: 1280, Height: 546, FrameRate: 25,
	}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
}
