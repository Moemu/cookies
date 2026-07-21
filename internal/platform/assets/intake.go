package assets

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Cecillia803/cookies/internal/platform/contract"
)

// GeneratedAssetIntakeRequest admits successful provider outputs into the
// project asset library. Organization and project scope come from the trusted
// request context and are deliberately not accepted in this payload.
type GeneratedAssetIntakeRequest struct {
	ProviderJobID string               `json:"provider_job_id"`
	Outputs       []GeneratedOutput    `json:"outputs"`
	Provenance    GenerationProvenance `json:"provenance"`
}

// GeneratedOutput describes a short-lived provider result. The Assets module
// must download it through an allow-listed provider adapter, verify it, and
// replace the temporary URI with its own durable storage location.
type GeneratedOutput struct {
	OutputID       string    `json:"output_id"`
	TemporaryURI   string    `json:"temporary_uri"`
	TemporaryUntil time.Time `json:"temporary_until"`
	MediaType      string    `json:"media_type"`
	SizeBytes      int64     `json:"size_bytes"`
	SHA256         string    `json:"sha256"`
}

type GenerationProvenance struct {
	Capability      string                     `json:"capability"`
	ProviderCode    string                     `json:"provider_code"`
	ModelAlias      string                     `json:"model_alias"`
	ModelVersion    string                     `json:"model_version"`
	PromptRef       *contract.ResourceRef      `json:"prompt_ref,omitempty"`
	SourceAssetRefs []contract.AssetVersionRef `json:"source_asset_refs,omitempty"`
	GeneratedAt     time.Time                  `json:"generated_at"`
}

type GeneratedAssetIntakeResponse struct {
	Assets []contract.ProjectAssetRef `json:"assets"`
}

func (r GeneratedAssetIntakeRequest) Validate() error {
	if strings.TrimSpace(r.ProviderJobID) == "" {
		return fmt.Errorf("provider_job_id is required")
	}
	if len(r.Outputs) == 0 {
		return fmt.Errorf("at least one generated output is required")
	}
	seen := make(map[string]struct{}, len(r.Outputs))
	for index, output := range r.Outputs {
		if err := output.Validate(); err != nil {
			return fmt.Errorf("invalid output at index %d: %w", index, err)
		}
		if _, exists := seen[output.OutputID]; exists {
			return fmt.Errorf("output_id %q is duplicated", output.OutputID)
		}
		seen[output.OutputID] = struct{}{}
	}
	if err := r.Provenance.Validate(); err != nil {
		return fmt.Errorf("invalid provenance: %w", err)
	}
	return nil
}

func (o GeneratedOutput) Validate() error {
	if strings.TrimSpace(o.OutputID) == "" {
		return fmt.Errorf("output_id is required")
	}
	parsed, err := url.ParseRequestURI(o.TemporaryURI)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "provider") {
		return fmt.Errorf("temporary_uri must use https or provider scheme")
	}
	if o.TemporaryUntil.IsZero() {
		return fmt.Errorf("temporary_until is required")
	}
	if strings.TrimSpace(o.MediaType) == "" {
		return fmt.Errorf("media_type is required")
	}
	if o.SizeBytes < 1 {
		return fmt.Errorf("size_bytes must be positive")
	}
	if !validSHA256(o.SHA256) {
		return fmt.Errorf("sha256 must be a lowercase hexadecimal SHA-256 digest")
	}
	return nil
}

func (p GenerationProvenance) Validate() error {
	if strings.TrimSpace(p.Capability) == "" || strings.TrimSpace(p.ProviderCode) == "" {
		return fmt.Errorf("capability and provider_code are required")
	}
	if strings.TrimSpace(p.ModelAlias) == "" || strings.TrimSpace(p.ModelVersion) == "" {
		return fmt.Errorf("model_alias and model_version are required")
	}
	if p.GeneratedAt.IsZero() {
		return fmt.Errorf("generated_at is required")
	}
	if p.PromptRef != nil {
		if err := p.PromptRef.Validate(); err != nil {
			return fmt.Errorf("invalid prompt_ref: %w", err)
		}
	}
	for index, assetRef := range p.SourceAssetRefs {
		if err := assetRef.Validate(); err != nil {
			return fmt.Errorf("invalid source_asset_ref at index %d: %w", index, err)
		}
	}
	return nil
}

func (r GeneratedAssetIntakeResponse) Validate() error {
	if len(r.Assets) == 0 {
		return fmt.Errorf("at least one admitted asset is required")
	}
	for index, assetRef := range r.Assets {
		if err := assetRef.Validate(); err != nil {
			return fmt.Errorf("invalid asset at index %d: %w", index, err)
		}
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}
