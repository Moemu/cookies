package main

import (
	"encoding/base64"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/config"
	"github.com/shikanon/cookies/internal/platform/provider"
)

func TestAssessConfigurationAcceptsOneBucketCanaryConfiguration(t *testing.T) {
	cfg := readyConfiguration()
	report := assessConfiguration(cfg)
	if !report.ConfigurationReady {
		t.Fatalf("expected configuration to be ready, blockers=%v", report.Blockers)
	}
	if len(report.Blockers) != 0 {
		t.Fatalf("expected no blockers, got %v", report.Blockers)
	}
}

func TestAssessConfigurationReportsOnlySafeBlockerCodes(t *testing.T) {
	cfg := readyConfiguration()
	cfg.ObjectStorage.Provider = "filesystem"
	cfg.ObjectStorage.SecretKey = ""
	cfg.Provider.OutputBucket = "another-bucket"
	cfg.Provider.MasterKey = "not-base64"
	cfg.Research.DocumentVisionEnabled = false

	report := assessConfiguration(cfg)
	want := []string{
		"DOCUMENT_VISION_DISABLED",
		"PROVIDER_MASTER_KEY_INVALID",
		"SINGLE_TOS_BUCKET_NOT_ENFORCED",
		"TOS_CREDENTIAL_MISSING",
		"TOS_PROVIDER_DISABLED",
	}
	if !slices.Equal(report.Blockers, want) {
		t.Fatalf("blockers mismatch\n got: %v\nwant: %v", report.Blockers, want)
	}
	if report.ConfigurationReady {
		t.Fatal("invalid configuration must not be ready")
	}
}

func TestAssessCapabilitiesRequiresExactCapabilityAndAlias(t *testing.T) {
	capabilities := []provider.CapabilityStatus{
		{Capability: "vision.understand", ModelAlias: "cookies.document.vision.standard", Available: true, CredentialConfigured: true},
		{Capability: "document.vision.parse", ModelAlias: "another-model", Available: true, CredentialConfigured: true},
		{Capability: "document.vision.parse", ModelAlias: "cookies.document.vision.standard", Available: true, CredentialConfigured: true},
	}
	available, credential := assessCapabilities(capabilities, "cookies.document.vision.standard")
	if !available || !credential {
		t.Fatalf("expected exact document route to be ready, available=%v credential=%v", available, credential)
	}
	available, credential = assessCapabilities(capabilities, "missing-model")
	if available || credential {
		t.Fatalf("unexpected fallback route, available=%v credential=%v", available, credential)
	}
}

func TestReadinessReportCannotSerializeConfigurationSecrets(t *testing.T) {
	cfg := readyConfiguration()
	report := assessConfiguration(cfg)
	contents, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(contents)
	for _, forbidden := range []string{
		cfg.ObjectStorage.Endpoint,
		cfg.ObjectStorage.AccessKey,
		cfg.ObjectStorage.SecretKey,
		cfg.ObjectStorage.AssetsBucket,
		cfg.Provider.MasterKey,
	} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("readiness report leaked a configuration value: %s", serialized)
		}
	}
}

func readyConfiguration() config.Config {
	bucket := "cookies-shared-private"
	return config.Config{
		ObjectStorage: config.ObjectStorage{
			Provider:         "tos",
			Endpoint:         "https://tos-cn-beijing.volces.com",
			Region:           "cn-beijing",
			AccessKey:        "access-key-present",
			SecretKey:        "secret-key-present",
			AssetsBucket:     bucket,
			QuarantineBucket: bucket,
		},
		Provider: config.Provider{
			MasterKey:        base64.StdEncoding.EncodeToString(make([]byte, 32)),
			MasterKeyVersion: "v1",
			OutputBucket:     bucket,
		},
		Research: config.Research{
			DocumentVisionEnabled:    true,
			DocumentVisionModelAlias: "cookies.document.vision.standard",
		},
	}
}
