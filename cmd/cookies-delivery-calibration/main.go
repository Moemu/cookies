// cookies-delivery-calibration audits Connector facts and exports historical
// Delivery calibration cases. It never calls an advertising-platform writer.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/config"
	"github.com/shikanon/cookies/internal/platform/connector"
	"github.com/shikanon/cookies/internal/platform/database"
)

const exportKeyEnvironment = "COOKIES_CALIBRATION_EXPORT_KEY_BASE64"

type commonFlags struct {
	organization string
	project      string
	account      string
	output       string
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}
	db, err := database.Open(ctx, cfg.MySQL)
	if err != nil {
		log.Fatalf("open MySQL: %v", err)
	}
	defer db.Close()
	repository := connector.MySQLRepository{DB: db}
	switch os.Args[1] {
	case "audit":
		err = runAudit(ctx, repository, os.Args[2:])
	case "export":
		err = runExport(ctx, repository, os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func runAudit(ctx context.Context, reader connector.SnapshotReader, args []string) error {
	flags := flag.NewFlagSet("audit", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	common := commonFlags{}
	bindCommon(flags, &common)
	cutoffText := flags.String("cutoff", "", "RFC3339 knowledge cutoff")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cutoff, err := parseTime(*cutoffText, "cutoff")
	if err != nil {
		return err
	}
	accountRef, err := validateCommon(common)
	if err != nil {
		return err
	}
	snapshot, err := reader.Snapshot(ctx, connector.Query{OrganizationID: common.organization, ProjectID: common.project, SourceRef: accountRef, PredictionCutoff: cutoff})
	if err != nil {
		return fmt.Errorf("read Connector snapshot: %w", err)
	}
	return writeJSON(common.output, connector.AuditCalibrationSnapshot(snapshot))
}

func runExport(ctx context.Context, reader connector.SnapshotReader, args []string) error {
	flags := flag.NewFlagSet("export", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	common := commonFlags{}
	bindCommon(flags, &common)
	predictionText := flags.String("prediction-cutoff", "", "RFC3339 feature cutoff")
	labelText := flags.String("label-cutoff", "", "RFC3339 mature-label cutoff")
	horizonDays := flags.Int("horizon-days", 7, "prediction horizon in days")
	keyVersion := flags.String("key-version", "", "non-secret export key version")
	if err := flags.Parse(args); err != nil {
		return err
	}
	prediction, err := parseTime(*predictionText, "prediction-cutoff")
	if err != nil {
		return err
	}
	label, err := parseTime(*labelText, "label-cutoff")
	if err != nil {
		return err
	}
	accountRef, err := validateCommon(common)
	if err != nil {
		return err
	}
	key, err := readExportKey()
	if err != nil {
		return err
	}
	defer clear(key)
	exporter := connector.CalibrationCaseExporter{Reader: reader, Key: key}
	result, err := exporter.Export(ctx, connector.CalibrationExportRequest{OrganizationID: common.organization, ProjectID: common.project, AccountRef: accountRef, PredictionCutoff: prediction, LabelCutoff: label, HorizonDays: *horizonDays, KeyVersion: strings.TrimSpace(*keyVersion)})
	if err != nil {
		return fmt.Errorf("export calibration cases: %w", err)
	}
	return writeJSON(common.output, result)
}

func bindCommon(flags *flag.FlagSet, value *commonFlags) {
	flags.StringVar(&value.organization, "organization", "", "cookies organization ID")
	flags.StringVar(&value.project, "project", "", "optional legacy cookies project ID")
	flags.StringVar(&value.account, "account", "", "Connector local account ID")
	flags.StringVar(&value.output, "output", "-", "new output file, or - for stdout")
}

func validateCommon(value commonFlags) (string, error) {
	value.organization = strings.TrimSpace(value.organization)
	value.project = strings.TrimSpace(value.project)
	value.account = strings.TrimSpace(value.account)
	if value.organization == "" || !strings.HasPrefix(value.account, "oeacct_") {
		return "", errors.New("organization and a Connector local oeacct_ account are required")
	}
	return connector.AnonymizeRef(value.account), nil
}

func parseTime(value, name string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must use RFC3339", name)
	}
	return parsed.UTC(), nil
}

func readExportKey() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv(exportKeyEnvironment))
	if raw == "" {
		return nil, fmt.Errorf("%s is required", exportKeyEnvironment)
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		clear(key)
		return nil, fmt.Errorf("%s must contain exactly 32 base64-encoded bytes", exportKeyEnvironment)
	}
	return key, nil
}

func writeJSON(path string, value any) error {
	var output io.Writer = os.Stdout
	var file *os.File
	if path != "-" {
		var err error
		file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return fmt.Errorf("create output without overwrite: %w", err)
		}
		defer file.Close()
		output = file
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: cookies-delivery-calibration <audit|export> [flags]")
}
