package contract

import (
	"fmt"
	"strings"
)

type AssetID string

// AssetRef is the cross-module reference returned by the Assets platform
// module after an upload or generated result has been admitted to a project
// library. It intentionally excludes URLs, prompts, rights metadata, and
// source classification, which are owned by the Assets module.
type AssetRef struct {
	AssetID        AssetID        `json:"asset_id"`
	Version        int64          `json:"version"`
	OrganizationID OrganizationID `json:"organization_id"`
	ProjectID      ProjectID      `json:"project_id"`
}

func (r AssetRef) Validate() error {
	if strings.TrimSpace(string(r.AssetID)) == "" {
		return fmt.Errorf("asset_id is required")
	}
	if r.Version < 1 {
		return fmt.Errorf("asset version must be positive")
	}
	if strings.TrimSpace(string(r.OrganizationID)) == "" {
		return fmt.Errorf("organization_id is required")
	}
	if strings.TrimSpace(string(r.ProjectID)) == "" {
		return fmt.Errorf("project_id is required")
	}
	return nil
}
