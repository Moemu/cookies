package media

import (
	"fmt"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestPreRollCompositionRequestRequiresStableAssetVersions(t *testing.T) {
	t.Parallel()
	valid := PreRollCompositionRequest{
		OrganizationID: "org_1",
		ProjectID:      "project_1",
		PreRollVideo:   contract.AssetVersionRef{AssetID: "asset_preroll", Version: 1},
		MainVideo:      contract.AssetVersionRef{AssetID: "asset_main", Version: 3},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	valid.MainVideo.Version = 0
	if err := valid.Validate(); err == nil {
		t.Fatal("mutable/non-versioned main video reference was accepted")
	}
}

func TestSegmentCompositionRequestAcceptsThirtySecondBrandFilm(t *testing.T) {
	t.Parallel()
	request := SegmentCompositionRequest{
		OrganizationID: "org_1",
		ProjectID:      "project_1",
		Segments: []SegmentComposition{
			{Asset: contract.AssetVersionRef{AssetID: "asset_1", Version: 1}, DurationSeconds: 5},
			{Asset: contract.AssetVersionRef{AssetID: "asset_2", Version: 1}, DurationSeconds: 5},
			{Asset: contract.AssetVersionRef{AssetID: "asset_3", Version: 1}, DurationSeconds: 5},
			{Asset: contract.AssetVersionRef{AssetID: "asset_4", Version: 1}, DurationSeconds: 5},
			{Asset: contract.AssetVersionRef{AssetID: "asset_5", Version: 1}, DurationSeconds: 5},
			{Asset: contract.AssetVersionRef{AssetID: "asset_6", Version: 1}, DurationSeconds: 5},
		},
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("30-second brand film request rejected: %v", err)
	}
}

func TestSegmentCompositionRequestRejectsUnboundedTimeline(t *testing.T) {
	t.Parallel()
	segments := make([]SegmentComposition, 9)
	for index := range segments {
		segments[index] = SegmentComposition{Asset: contract.AssetVersionRef{AssetID: contract.AssetID(fmt.Sprintf("asset_%d", index)), Version: 1}, DurationSeconds: 15}
	}
	request := SegmentCompositionRequest{OrganizationID: "org_1", ProjectID: "project_1", Segments: segments}
	if err := request.Validate(); err == nil {
		t.Fatal("unbounded segment timeline was accepted")
	}
}

func TestFFconcatPathUsesForwardSlashes(t *testing.T) {
	t.Parallel()
	if got, want := ffconcatPath(`C:\work\clip.mp4`), "C:/work/clip.mp4"; got != want {
		t.Fatalf("ffconcatPath() = %q, want %q", got, want)
	}
	if got, want := ffconcatPath(`\\server\share\clip.mp4`), "//server/share/clip.mp4"; got != want {
		t.Fatalf("ffconcatPath() UNC path = %q, want %q", got, want)
	}
}
