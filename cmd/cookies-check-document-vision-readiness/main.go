// cookies-check-document-vision-readiness performs a read-only admission check
// for the optional LAS document-vision canary. It never submits provider work
// and never prints credentials, bucket names, object keys, DSNs, or route URLs.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/config"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/database"
	"github.com/shikanon/cookies/internal/platform/provider"
)

const readinessContractVersion = "document-vision-readiness/v1"

type readinessReport struct {
	ContractVersion      string   `json:"contract_version"`
	Ready                bool     `json:"ready"`
	ConfigurationReady   bool     `json:"configuration_ready"`
	DatabaseChecked      bool     `json:"database_checked"`
	RouteAvailable       bool     `json:"route_available"`
	CredentialConfigured bool     `json:"credential_configured"`
	Blockers             []string `json:"blockers"`
}

func main() {
	configOnly := flag.Bool("config-only", false, "validate process/.env configuration without connecting to MySQL")
	timeout := flag.Duration("timeout", 8*time.Second, "maximum duration for the read-only MySQL readiness check")
	flag.Parse()

	report := readinessReport{ContractVersion: readinessContractVersion}
	cfg, err := config.Load()
	if err != nil {
		report.Blockers = []string{"CONFIG_INVALID"}
		writeReportAndExit(report, 2)
	}
	report = assessConfiguration(cfg)
	if *configOnly {
		report.Ready = report.ConfigurationReady
		writeReportAndExit(report, readinessExitCode(report.Ready))
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	db, err := database.Open(ctx, cfg.MySQL)
	if err != nil {
		report.Blockers = appendUnique(report.Blockers, "DATABASE_UNAVAILABLE")
		writeReportAndExit(report, 2)
	}
	defer db.Close()
	report.DatabaseChecked = true

	store := provider.MySQLGatewayConfigStore{DB: db}
	capabilities, err := store.ListCapabilities(ctx, contract.OrganizationID("document_vision_readiness"))
	if err != nil {
		report.Blockers = appendUnique(report.Blockers, "ROUTE_CHECK_FAILED")
		writeReportAndExit(report, 2)
	}
	report.RouteAvailable, report.CredentialConfigured = assessCapabilities(capabilities, cfg.Research.DocumentVisionModelAlias)
	if !report.RouteAvailable {
		report.Blockers = appendUnique(report.Blockers, "DOCUMENT_VISION_ROUTE_UNAVAILABLE")
	}
	if !report.CredentialConfigured {
		report.Blockers = appendUnique(report.Blockers, "DOCUMENT_VISION_CREDENTIAL_UNAVAILABLE")
	}
	report.Ready = report.ConfigurationReady && report.RouteAvailable && report.CredentialConfigured
	writeReportAndExit(report, readinessExitCode(report.Ready))
}

func assessConfiguration(cfg config.Config) readinessReport {
	report := readinessReport{ContractVersion: readinessContractVersion}
	if cfg.ObjectStorage.Provider != "tos" {
		report.Blockers = append(report.Blockers, "TOS_PROVIDER_DISABLED")
	}
	if strings.TrimSpace(cfg.ObjectStorage.Endpoint) == "" || strings.TrimSpace(cfg.ObjectStorage.Region) == "" {
		report.Blockers = append(report.Blockers, "TOS_LOCATION_MISSING")
	}
	if strings.TrimSpace(cfg.ObjectStorage.AccessKey) == "" || strings.TrimSpace(cfg.ObjectStorage.SecretKey) == "" {
		report.Blockers = append(report.Blockers, "TOS_CREDENTIAL_MISSING")
	}
	if strings.TrimSpace(cfg.ObjectStorage.AssetsBucket) == "" ||
		strings.TrimSpace(cfg.ObjectStorage.QuarantineBucket) == "" ||
		strings.TrimSpace(cfg.Provider.OutputBucket) == "" ||
		cfg.ObjectStorage.AssetsBucket != cfg.ObjectStorage.QuarantineBucket ||
		cfg.ObjectStorage.AssetsBucket != cfg.Provider.OutputBucket {
		report.Blockers = append(report.Blockers, "SINGLE_TOS_BUCKET_NOT_ENFORCED")
	}
	if _, err := provider.NewAESGCMCredentialCipher(cfg.Provider.MasterKey, cfg.Provider.MasterKeyVersion); err != nil {
		report.Blockers = append(report.Blockers, "PROVIDER_MASTER_KEY_INVALID")
	}
	if !cfg.Research.DocumentVisionEnabled {
		report.Blockers = append(report.Blockers, "DOCUMENT_VISION_DISABLED")
	}
	if strings.TrimSpace(cfg.Research.DocumentVisionModelAlias) == "" {
		report.Blockers = append(report.Blockers, "DOCUMENT_VISION_MODEL_ALIAS_MISSING")
	}
	sort.Strings(report.Blockers)
	report.ConfigurationReady = len(report.Blockers) == 0
	return report
}

func assessCapabilities(capabilities []provider.CapabilityStatus, modelAlias string) (bool, bool) {
	for _, capability := range capabilities {
		if capability.Capability != "document.vision.parse" || capability.ModelAlias != modelAlias {
			continue
		}
		return capability.Available, capability.CredentialConfigured
	}
	return false, false
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	values = append(values, value)
	sort.Strings(values)
	return values
}

func readinessExitCode(ready bool) int {
	if ready {
		return 0
	}
	return 2
}

func writeReportAndExit(report readinessReport, code int) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, "could not encode readiness report")
		os.Exit(1)
	}
	os.Exit(code)
}
