package projectcontext

import (
	"testing"
)

func TestReferenceRequiresStableIdentifiersAndVersion(t *testing.T) {
	t.Parallel()
	reference := Reference{
		OrganizationID: "org_1",
		ProjectID:      "project_1",
		BrandID:        "brand_1",
		ProductIDs:     []ProductID{"product_1"},
		Version:        1,
	}
	if err := reference.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	reference.ProductIDs = append(reference.ProductIDs, "product_1")
	if err := reference.Validate(); err == nil {
		t.Fatal("expected duplicated product ID to be rejected")
	}
}
