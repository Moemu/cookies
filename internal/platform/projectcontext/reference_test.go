package projectcontext

import (
	"testing"
)

func TestReferenceRequiresStableIdentifiersAndVersion(t *testing.T) {
	t.Parallel()
	brandID := BrandID("brand_1")
	reference := Reference{
		OrganizationID:        "org_1",
		ProjectID:             "project_1",
		BrandID:               &brandID,
		ProductIDs:            []ProductID{"product_1"},
		ProjectContextVersion: 1,
	}
	if err := reference.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	reference.ProductIDs = append(reference.ProductIDs, "product_1")
	if err := reference.Validate(); err == nil {
		t.Fatal("expected duplicated product ID to be rejected")
	}
}

func TestDraftReferenceAllowsNullBrandButActiveOperationDoesNot(t *testing.T) {
	t.Parallel()
	reference := Reference{OrganizationID: "org_1", ProjectID: "project_1", ProductIDs: []ProductID{}, ProjectContextVersion: 1}
	if err := reference.Validate(); err != nil {
		t.Fatalf("draft Validate() error = %v", err)
	}
	if err := reference.ValidateBrandBound(); err == nil {
		t.Fatal("expected active operation to require a brand")
	}
}
