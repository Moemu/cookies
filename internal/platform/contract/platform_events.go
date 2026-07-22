package contract

import (
	"fmt"
	"strings"
)

type AssetKind string

const (
	AssetImage    AssetKind = "image"
	AssetVideo    AssetKind = "video"
	AssetAudio    AssetKind = "audio"
	AssetDocument AssetKind = "document"
	AssetText     AssetKind = "text"
)

type AssetSourceType string

const (
	AssetSourceUpload            AssetSourceType = "upload"
	AssetSourceProviderGenerated AssetSourceType = "provider_generated"
	AssetSourceImported          AssetSourceType = "imported"
	AssetSourceCaptured          AssetSourceType = "captured"
)

type AssetReadyData struct {
	AssetKind     AssetKind       `json:"asset_kind"`
	MIMEType      string          `json:"mime_type"`
	SizeBytes     int64           `json:"size_bytes"`
	SourceType    AssetSourceType `json:"source_type"`
	ProviderJobID string          `json:"provider_job_id,omitempty"`
	OutputID      string          `json:"output_id,omitempty"`
}

func (d AssetReadyData) Validate() error {
	if !d.AssetKind.valid() || strings.TrimSpace(d.MIMEType) == "" || d.SizeBytes < 1 || !d.SourceType.valid() {
		return fmt.Errorf("asset kind, MIME type, positive size, and source type are required")
	}
	if d.SourceType == AssetSourceProviderGenerated && (strings.TrimSpace(d.ProviderJobID) == "" || strings.TrimSpace(d.OutputID) == "") {
		return fmt.Errorf("provider-generated asset requires provider_job_id and output_id")
	}
	return nil
}

func (k AssetKind) valid() bool {
	switch k {
	case AssetImage, AssetVideo, AssetAudio, AssetDocument, AssetText:
		return true
	default:
		return false
	}
}

func (s AssetSourceType) valid() bool {
	switch s {
	case AssetSourceUpload, AssetSourceProviderGenerated, AssetSourceImported, AssetSourceCaptured:
		return true
	default:
		return false
	}
}

type ModelJobCompletedData struct {
	Status           ProviderJobStatus `json:"status"`
	Capability       string            `json:"capability"`
	ProjectAssetRefs []ProjectAssetRef `json:"project_asset_refs"`
}

func (d ModelJobCompletedData) Validate() error {
	if d.Status != ProviderJobSucceeded && d.Status != ProviderJobPartiallySucceeded {
		return fmt.Errorf("completed model job status must be succeeded or partially_succeeded")
	}
	if strings.TrimSpace(d.Capability) == "" || len(d.ProjectAssetRefs) == 0 {
		return fmt.Errorf("capability and at least one durable project asset are required")
	}
	for index, ref := range d.ProjectAssetRefs {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("invalid project_asset_ref at index %d: %w", index, err)
		}
	}
	return nil
}
