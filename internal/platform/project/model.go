package project

import (
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type Status string

const (
	StatusDraft    Status = "draft"
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
)

type Project struct {
	ID                      contract.ProjectID      `json:"id"`
	OrganizationID          contract.OrganizationID `json:"organization_id"`
	Name                    string                  `json:"name"`
	Status                  Status                  `json:"status"`
	PrimaryBrandID          *contract.BrandID       `json:"primary_brand_id"`
	PrimaryBrandStatus      string                  `json:"-"`
	BrandGuidelineVersionID string                  `json:"brand_guideline_version_id,omitempty"`
	ProjectContextVersion   int64                   `json:"project_context_version"`
	CreatedAt               time.Time               `json:"created_at"`
	UpdatedAt               time.Time               `json:"updated_at"`
}

type Brand struct {
	ID             contract.BrandID        `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	Name           string                  `json:"name"`
	Status         string                  `json:"status"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

type CreateProjectRequest struct {
	Name           string               `json:"name"`
	PrimaryBrandID *contract.BrandID    `json:"primary_brand_id"`
	ProductIDs     []contract.ProductID `json:"product_ids"`
	Activate       bool                 `json:"activate"`
}

func (r CreateProjectRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" || len(r.Name) > 255 {
		return fmt.Errorf("project name must be between 1 and 255 characters")
	}
	if r.PrimaryBrandID != nil && strings.TrimSpace(string(*r.PrimaryBrandID)) == "" {
		return fmt.Errorf("primary_brand_id must be null or non-empty")
	}
	if r.Activate && r.PrimaryBrandID == nil {
		return fmt.Errorf("active project requires primary_brand_id")
	}
	if r.ProductIDs == nil {
		return fmt.Errorf("product_ids must be an array")
	}
	seen := make(map[contract.ProductID]struct{}, len(r.ProductIDs))
	for _, productID := range r.ProductIDs {
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
