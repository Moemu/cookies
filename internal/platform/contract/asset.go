package contract

import (
	"fmt"
	"strings"
)

type AssetID string

// AssetVersionRef identifies an immutable media version. Authorization is
// evaluated from the request and business relationship, not from a URL.
type AssetVersionRef struct {
	AssetID AssetID `json:"asset_id"`
	Version int64   `json:"version"`
}

func (r AssetVersionRef) Validate() error {
	if strings.TrimSpace(string(r.AssetID)) == "" {
		return fmt.Errorf("asset_id is required")
	}
	if r.Version < 1 {
		return fmt.Errorf("asset version must be positive")
	}
	return nil
}

// ProjectAssetRef records a project-library relationship to an immutable
// asset version. The Assets module owns the relationship's lifecycle.
type ProjectAssetRef struct {
	ProjectID    ProjectID       `json:"project_id"`
	AssetVersion AssetVersionRef `json:"asset_version"`
}

func (r ProjectAssetRef) Validate() error {
	if strings.TrimSpace(string(r.ProjectID)) == "" {
		return fmt.Errorf("project_id is required")
	}
	return r.AssetVersion.Validate()
}
