package media

import (
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

func TestFFconcatPathUsesForwardSlashes(t *testing.T) {
	t.Parallel()
	if got, want := ffconcatPath(`C:\work\clip.mp4`), "C:/work/clip.mp4"; got != want {
		t.Fatalf("ffconcatPath() = %q, want %q", got, want)
	}
	if got, want := ffconcatPath(`\\server\share\clip.mp4`), "//server/share/clip.mp4"; got != want {
		t.Fatalf("ffconcatPath() UNC path = %q, want %q", got, want)
	}
}
