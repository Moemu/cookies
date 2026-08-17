package calibrationmanifest

import "testing"

func TestCurrentManifestIsTheCanonicalTypedProjection(t *testing.T) {
	manifest, err := Current()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Fields) != 40 || len(manifest.CoverageCases) != 26 {
		t.Fatalf("unexpected frozen coverage: %d fields, %d cases", len(manifest.Fields), len(manifest.CoverageCases))
	}
	if err := manifest.ValidateBinding(manifest.SchemaVersion, manifest.ManifestID); err != nil {
		t.Fatal(err)
	}
	if err := manifest.ValidateBinding(manifest.SchemaVersion, "old-manifest"); err == nil {
		t.Fatal("old binding must fail closed")
	}
	projections := manifest.Project(OceanEngineConfiguration, map[string]string{
		"marketing_purpose": "application",
		"delivery_mode":     "manual",
	})
	if len(projections) == 0 {
		t.Fatal("configuration must receive Manifest projections")
	}
	var applicationDownload, pending bool
	for _, projection := range projections {
		if projection.Field.Key == "project.application_download_mode" {
			applicationDownload = projection.Blocked && projection.Reason == "platform_pending"
		}
		if projection.Field.Key == "project.marketing_scenario" {
			pending = projection.Blocked && projection.Reason == "platform_pending"
		}
		if projection.Mapping.Treatment == EvidenceOnly && projection.Executable {
			t.Fatalf("evidence-only field became executable: %s", projection.Field.Key)
		}
	}
	if !applicationDownload || !pending {
		t.Fatal("missing conditions and dependency-only fields must remain platform_pending")
	}
}

func TestManifestRejectsDuplicateMappingAndWriteAuthority(t *testing.T) {
	manifest, err := Current()
	if err != nil {
		t.Fatal(err)
	}
	manifest.ConsumerMappings = append(manifest.ConsumerMappings, manifest.ConsumerMappings[0])
	if err := manifest.Validate(); err == nil {
		t.Fatal("duplicate field/consumer mapping must fail")
	}
	manifest, err = Current()
	if err != nil {
		t.Fatal(err)
	}
	manifest.ObservationBoundary.RemoteWriteAuthorized = true
	if err := manifest.Validate(); err == nil {
		t.Fatal("Manifest must never grant remote write authority")
	}
}
