package contract

import "testing"

func TestAssetRefRequiresProjectScopedVersion(t *testing.T) {
	t.Parallel()
	asset := AssetRef{
		AssetID:        "asset_1",
		Version:        1,
		OrganizationID: "org_1",
		ProjectID:      "project_1",
	}
	if err := asset.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	asset.Version = 0
	if err := asset.Validate(); err == nil {
		t.Fatal("expected invalid asset version to be rejected")
	}
}
