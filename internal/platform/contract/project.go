package contract

import (
	"fmt"
	"strings"
)

type BrandID string
type ProductID string

// ProductCategory splits the two broad OceanEngine product kinds. The
// fine-grained platform category tree is not enumerable and is not modeled.
type ProductCategory string

const (
	ProductCategoryProduct  ProductCategory = "product"
	ProductCategoryActivity ProductCategory = "activity"
)

// ProductPriceBand mirrors the OceanEngine product price tiers.
type ProductPriceBand string

const (
	PriceBand0To9       ProductPriceBand = "0_9"
	PriceBand10To99     ProductPriceBand = "10_99"
	PriceBand100To999   ProductPriceBand = "100_999"
	PriceBand1000To9999 ProductPriceBand = "1000_9999"
	PriceBand10000To99999 ProductPriceBand = "10000_99999"
	PriceBand100000Plus ProductPriceBand = "100000_plus"
)

// BrandType indicates whether the brand is an OceanEngine standard brand
// (platform-recognized) or a custom brand.
type BrandType string

const (
	BrandTypeStandard BrandType = "standard"
	BrandTypeCustom   BrandType = "custom"
)

// Product is the organization-level business product object. It is the source
// of truth referenced by strategy briefs, insight crawling, and delivery
// marketing_product references. OceanEngineProductID is a platform mapping:
// a product without a bound OceanEngine object is treated as not yet created
// on the platform.
type Product struct {
	ID                   ProductID      `json:"id"`
	OrganizationID       OrganizationID `json:"organization_id"`
	Name                 string         `json:"name"`
	Category             ProductCategory `json:"category"`
	Status               string         `json:"status"`
	ProductImage         string         `json:"product_image,omitempty"`
	PriceBand            ProductPriceBand `json:"price_band,omitempty"`
	ActivityType         string         `json:"activity_type,omitempty"`
	ActivityName         string         `json:"activity_name,omitempty"`
	BrandType            BrandType      `json:"brand_type,omitempty"`
	BrandName            string         `json:"brand_name,omitempty"`
	Description          string         `json:"description,omitempty"`
	OceanEngineProductID string         `json:"ocean_engine_product_id,omitempty"`
	CreatedAt            string         `json:"created_at"`
	UpdatedAt            string         `json:"updated_at"`
}

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
	ID                   ProductID      `json:"id"`
	Name                 string         `json:"name"`
	Category             ProductCategory `json:"category"`
	ActivityType         string         `json:"activity_type,omitempty"`
	ActivityName         string         `json:"activity_name,omitempty"`
	BrandName            string         `json:"brand_name,omitempty"`
	OceanEngineProductID string         `json:"ocean_engine_product_id,omitempty"`
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
