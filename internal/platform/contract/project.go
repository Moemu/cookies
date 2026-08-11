package contract

import (
	"fmt"
	"strings"
)

type BrandID string
type ProductID string

// ProjectRef is the stable cross-module pointer to an authorized project
// context snapshot.
type ProjectRef struct {
	OrganizationID        OrganizationID `json:"organization_id"`
	ProjectID             ProjectID      `json:"project_id"`
	ProjectContextVersion int64          `json:"project_context_version"`
}

func (r ProjectRef) Validate() error {
	if strings.TrimSpace(string(r.OrganizationID)) == "" {
		return fmt.Errorf("organization_id is required")
	}
	if strings.TrimSpace(string(r.ProjectID)) == "" {
		return fmt.Errorf("project_id is required")
	}
	if r.ProjectContextVersion < 1 {
		return fmt.Errorf("project_context_version must be positive")
	}
	return nil
}

// ProjectContext is the minimal project projection shared with Provider and
// Assets. BrandID may be nil while a project is a draft.
type ProjectContext struct {
	OrganizationID          OrganizationID `json:"organization_id"`
	ProjectID               ProjectID      `json:"project_id"`
	BrandID                 *BrandID       `json:"brand_id"`
	ProductIDs              []ProductID    `json:"product_ids"`
	BrandGuidelineVersionID string         `json:"brand_guideline_version_id,omitempty"`
	ProjectContextVersion   int64          `json:"project_context_version"`
}

// ProjectBusinessContext is the human-readable identity used to prevent a
// Brief from crossing into a different client's Project. It is deliberately
// separate from ProjectContext: names are validation metadata and must never
// become Provider lineage identifiers.
type ProjectBusinessContext struct {
	ProjectID   ProjectID                `json:"project_id"`
	ProjectName string                   `json:"project_name"`
	BrandID     *BrandID                 `json:"brand_id"`
	BrandName   string                   `json:"brand_name"`
	Products    []ProjectBusinessProduct `json:"products"`
}

type ProjectBusinessProduct struct {
	ID   ProductID `json:"id"`
	Name string    `json:"name"`
}

func (c ProjectContext) Validate() error {
	if err := (ProjectRef{OrganizationID: c.OrganizationID, ProjectID: c.ProjectID, ProjectContextVersion: c.ProjectContextVersion}).Validate(); err != nil {
		return err
	}
	if c.BrandID != nil && strings.TrimSpace(string(*c.BrandID)) == "" {
		return fmt.Errorf("brand_id must be null or non-empty")
	}
	if c.ProductIDs == nil {
		return fmt.Errorf("product_ids must be an array")
	}
	seen := make(map[ProductID]struct{}, len(c.ProductIDs))
	for _, productID := range c.ProductIDs {
		if strings.TrimSpace(string(productID)) == "" {
			return fmt.Errorf("product_id must not be empty")
		}
		if _, exists := seen[productID]; exists {
			return fmt.Errorf("product_id %q is duplicated", productID)
		}
		seen[productID] = struct{}{}
	}
	return nil
}

// ValidateBrandBound enforces the active/generation/intake invariant.
func (c ProjectContext) ValidateBrandBound() error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.BrandID == nil {
		return fmt.Errorf("brand_id is required for an active project operation")
	}
	return nil
}
