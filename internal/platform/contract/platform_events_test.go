package contract

import "testing"

func TestCompletedModelEventRequiresDurableProjectAssets(t *testing.T) {
	t.Parallel()
	data := ModelJobCompletedData{
		Status: ProviderJobSucceeded, Capability: "image.generate",
		ProjectAssetRefs: []ProjectAssetRef{{ProjectID: "project_1", AssetVersion: AssetVersionRef{AssetID: "asset_1", Version: 1}}},
	}
	if err := data.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	data.ProjectAssetRefs = nil
	if err := data.Validate(); err == nil {
		t.Fatal("expected event without durable assets to be rejected")
	}
}
