package assets

import (
	"testing"
	"time"

	"github.com/Cecillia803/cookies/internal/platform/contract"
)

func TestGeneratedAssetIntakeValidatesMultiOutputRequest(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	request := GeneratedAssetIntakeRequest{
		ProviderJobID: "job_1",
		Outputs: []GeneratedOutput{
			{
				OutputID:       "output_1",
				TemporaryURI:   "https://provider.example/output/1",
				TemporaryUntil: now.Add(time.Hour),
				MediaType:      "image/png",
				SizeBytes:      1024,
				SHA256:         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			{
				OutputID:       "output_2",
				TemporaryURI:   "provider://volcengine/task/output-2",
				TemporaryUntil: now.Add(time.Hour),
				MediaType:      "image/png",
				SizeBytes:      2048,
				SHA256:         "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
		},
		Provenance: GenerationProvenance{
			Capability:   "image.generate",
			ProviderCode: "volcengine",
			ModelAlias:   "cookies.image.standard",
			ModelVersion: "image-v1",
			GeneratedAt:  now,
		},
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	request.Outputs[1].OutputID = "output_1"
	if err := request.Validate(); err == nil {
		t.Fatal("expected duplicate output IDs to be rejected")
	}
}

func TestGeneratedAssetIntakeRejectsUnsafeOrUnverifiableOutput(t *testing.T) {
	t.Parallel()
	output := GeneratedOutput{
		OutputID:       "output_1",
		TemporaryURI:   "http://untrusted.example/output",
		TemporaryUntil: time.Now().UTC().Add(time.Hour),
		MediaType:      "image/png",
		SizeBytes:      10,
		SHA256:         "not-a-digest",
	}
	if err := output.Validate(); err == nil {
		t.Fatal("expected unsafe output to be rejected")
	}
}

func TestGeneratedAssetIntakeResponseRequiresProjectAssets(t *testing.T) {
	t.Parallel()
	response := GeneratedAssetIntakeResponse{Assets: []contract.ProjectAssetRef{
		{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "asset_1", Version: 1}},
	}}
	if err := response.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
