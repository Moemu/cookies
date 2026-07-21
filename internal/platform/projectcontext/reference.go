// Package projectcontext defines the minimal, authorized projection that the
// vertical systems may retain. It intentionally excludes complete brand and
// product facts; those remain owned by the Project & Brand platform module.
package projectcontext

import (
	"fmt"
	"strings"

	"github.com/Cecillia803/cookies/internal/platform/contract"
)

type BrandID string
type ProductID string

type Reference struct {
	OrganizationID          contract.OrganizationID `json:"organization_id"`
	ProjectID               contract.ProjectID      `json:"project_id"`
	BrandID                 BrandID                 `json:"brand_id"`
	ProductIDs              []ProductID             `json:"product_ids"`
	BrandGuidelineVersionID string                  `json:"brand_guideline_version_id,omitempty"`
	Version                 int64                   `json:"version"`
}

func (r Reference) Validate() error {
	if strings.TrimSpace(string(r.OrganizationID)) == "" {
		return fmt.Errorf("organization_id is required")
	}
	if strings.TrimSpace(string(r.ProjectID)) == "" {
		return fmt.Errorf("project_id is required")
	}
	if r.Version < 1 {
		return fmt.Errorf("project context version must be positive")
	}
	products := make(map[ProductID]struct{}, len(r.ProductIDs))
	for _, productID := range r.ProductIDs {
		if strings.TrimSpace(string(productID)) == "" {
			return fmt.Errorf("product_id must not be empty")
		}
		if _, exists := products[productID]; exists {
			return fmt.Errorf("product_id %q is duplicated", productID)
		}
		products[productID] = struct{}{}
	}
	return nil
}
