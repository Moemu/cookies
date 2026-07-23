package assets

import (
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestGeneratedAssetIntakeValidatesSingleOpaqueOutput(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	request := GeneratedAssetIntakeRequest{
		ProviderJobID: "job_1",
		Output: contract.ProviderOutputRef{
			ProviderCode:       "volcengine",
			ProviderJobID:      "job_1",
			OutputID:           "output_1",
			RetrievalExpiresAt: now.Add(2 * time.Hour),
			DeclaredMIMEType:   "image/png",
			DeclaredSizeBytes:  1024,
		},
		Provenance: GenerationProvenance{
			Capability:            "image.generate",
			ProviderCode:          "volcengine",
			ModelAlias:            "cookies.image.standard",
			ModelVersion:          "image-v1",
			SourceAssetRefs:       []contract.AssetVersionRef{},
			ProjectContextVersion: 7,
			GeneratedAt:           now,
		},
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	request.Output.ProviderJobID = "job_2"
	if err := request.Validate(); err == nil {
		t.Fatal("expected mismatched provider job IDs to be rejected")
	}
}

func TestProviderOutputAllowsMissingDeclaredChecksum(t *testing.T) {
	t.Parallel()
	output := contract.ProviderOutputRef{
		ProviderCode:       "volcengine",
		ProviderJobID:      "job_1",
		OutputID:           "output_1",
		RetrievalExpiresAt: time.Now().UTC().Add(2 * time.Hour),
		DeclaredMIMEType:   "image/png",
		DeclaredSizeBytes:  10,
		DeclaredSHA256:     nil,
	}
	if err := output.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestGeneratedAssetIntakeResponseTracksOneProjectAsset(t *testing.T) {
	t.Parallel()
	projectAsset := contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "asset_1", Version: 1}}
	response := GeneratedAssetIntakeResponse{
		ID:              "intake_1",
		ProviderJobID:   "job_1",
		OutputID:        "output_1",
		Status:          GeneratedIntakeSucceeded,
		ProjectAssetRef: &projectAsset,
	}
	if err := response.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
