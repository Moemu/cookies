package contract

import "testing"

func TestAssetVersionAndProjectAssetReferencesValidateIndependently(t *testing.T) {
	t.Parallel()
	asset := AssetVersionRef{
		AssetID: "asset_1",
		Version: 1,
	}
	if err := asset.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	asset.Version = 0
	if err := asset.Validate(); err == nil {
		t.Fatal("expected invalid asset version to be rejected")
	}

	projectAsset := ProjectAssetRef{
		ProjectID:    "project_1",
		AssetVersion: AssetVersionRef{AssetID: "asset_1", Version: 1},
	}
	if err := projectAsset.Validate(); err != nil {
		t.Fatalf("project asset validation error = %v", err)
	}
}
