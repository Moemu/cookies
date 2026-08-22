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
	"github.com/shikanon/cookies/internal/systems/insights"
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
	timeout := 2 * time.Minute
	if os.Args[1] == "import-xlsx" {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}
	if os.Args[1] == "backtest-xlsx" {
		if err = runBacktestXLSX(ctx, os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		return
	}
	db, err := database.Open(ctx, cfg.MySQL)
	if err != nil {
		log.Fatalf("open MySQL: %v", err)
	}
	defer db.Close()
	repository := connector.MySQLRepository{DB: db}
	switch os.Args[1] {
	case "import-xlsx":
		err = runImportXLSX(ctx, cfg, repository, os.Args[2:])
	case "backtest-launch-batches":
		err = runLaunchBatchBacktest(ctx, repository, os.Args[2:])
	case "audit":
		err = runAudit(ctx, repository, os.Args[2:])
	case "export":
		err = runExport(ctx, repository, os.Args[2:])
	case "backtest":
		err = runBacktest(ctx, repository, os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func runLaunchBatchBacktest(ctx context.Context, repository connector.MySQLRepository, args []string) error {
	flags := flag.NewFlagSet("backtest-launch-batches", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	common := commonFlags{}
	bindCommon(flags, &common)
	corePath := flags.String("core", "", "core custom XLSX export")
	supplementPath := flags.String("supplement", "", "supplement custom XLSX export")
	cutoffText := flags.String("cutoff", "", "RFC3339 Connector knowledge cutoff")
	persist := flags.Bool("persist", false, "persist the safe calibrated prior for Delivery simulation")
	if err := flags.Parse(args); err != nil {
		return err
	}
	accountRef, err := validateCommon(common)
	if err != nil {
		return err
	}
	cutoff, err := parseTime(*cutoffText, "cutoff")
	if err != nil {
		return err
	}
	externalAccount, err := repository.ResolveAnyExternalAccountID(ctx, strings.TrimSpace(common.organization), strings.TrimSpace(common.project), strings.TrimSpace(common.account))
	if err != nil {
		return fmt.Errorf("resolve registered Connector account: %w", err)
	}
	sources, err := readOfflineSources([]string{*corePath, *supplementPath})
	if err != nil {
		return err
	}
	for index := range sources {
		defer clear(sources[index].Content)
	}
	snapshot, err := repository.Snapshot(ctx, connector.Query{OrganizationID: strings.TrimSpace(common.organization), ProjectID: strings.TrimSpace(common.project), SourceRef: accountRef, PredictionCutoff: cutoff})
	if err != nil {
		return fmt.Errorf("read Connector object index: %w", err)
	}
	result, err := connector.BuildLaunchBatchBacktest(connector.LaunchBatchBacktestRequest{ExternalAccount: externalAccount, Core: sources[0], Supplement: sources[1], Inventory: snapshot, TimeZone: "Asia/Shanghai"})
	if err != nil {
		return fmt.Errorf("build launch batch backtest: %w", err)
	}
	if *persist {
		prior, priorErr := connector.NewLaunchBatchCalibrationSnapshot(strings.TrimSpace(common.organization), strings.TrimSpace(common.account), result, sources, time.Now().UTC())
		if priorErr != nil {
			return fmt.Errorf("build launch batch calibration snapshot: %w", priorErr)
		}
		if _, priorErr = repository.AppendLaunchBatchCalibration(ctx, prior); priorErr != nil {
			return fmt.Errorf("persist launch batch calibration snapshot: %w", priorErr)
		}
	}
	return writeJSON(common.output, result)
}

type staticSnapshotReader struct{ snapshot connector.CanonicalSnapshot }

func (r staticSnapshotReader) Snapshot(context.Context, connector.Query) (connector.CanonicalSnapshot, error) {
	return r.snapshot, nil
}

func runBacktestXLSX(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("backtest-xlsx", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	common := commonFlags{}
	bindCommon(flags, &common)
	accountDaily := flags.String("account-daily", "", "account daily XLSX export")
	projectDaily := flags.String("project-daily", "", "project daily XLSX export")
	promotionDaily := flags.String("promotion-daily", "", "promotion daily XLSX export")
	materialAggregate := flags.String("material-aggregate", "", "material aggregate XLSX export")
	lookbackDays := flags.Int("lookback-days", 7, "history window in days")
	horizonDays := flags.Int("horizon-days", 1, "prediction horizon in days")
	stepDays := flags.Int("step-days", 1, "days between rolling cutoffs")
	minimumHistory := flags.Int("minimum-history-windows", 2, "minimum observed history windows per case")
	keyVersion := flags.String("key-version", "", "non-secret export key version")
	if err := flags.Parse(args); err != nil {
		return err
	}
	common.organization = strings.TrimSpace(common.organization)
	common.project = strings.TrimSpace(common.project)
	if common.organization == "" || strings.TrimSpace(*keyVersion) == "" || *lookbackDays < 1 || *horizonDays < 1 || *stepDays < 1 || *minimumHistory < 1 {
		return connector.ErrInvalidFact
	}
	sources, err := readOfflineSources([]string{*accountDaily, *projectDaily, *promotionDaily, *materialAggregate})
	if err != nil {
		return err
	}
	for index := range sources {
		defer clear(sources[index].Content)
	}
	externalAccount, err := connector.DetectOfflineXLSXExternalAccount(sources)
	if err != nil {
		return err
	}
	accountID := connector.PlatformAccountID(common.organization, common.project, externalAccount)
	knowledgeCutoff := time.Now().UTC()
	snapshot, audit, err := connector.BuildOfflineXLSXSnapshot(connector.OfflineXLSXImportRequest{
		OrganizationID: common.organization, ProjectID: common.project, AccountID: accountID, ExternalAccount: externalAccount,
		IdempotencyKey: "offline-backtest-xlsx-v1", TimeZone: "Asia/Shanghai", Currency: "CNY", Sources: sources,
	}, knowledgeCutoff)
	if err != nil {
		return fmt.Errorf("build offline Connector snapshot: %w", err)
	}
	key, err := readExportKey()
	if err != nil {
		return err
	}
	defer clear(key)
	result, err := (connector.RetrospectiveCalibrationBuilder{Reader: staticSnapshotReader{snapshot: snapshot}, Key: key, Now: func() time.Time { return knowledgeCutoff }}).Build(ctx, connector.RetrospectiveCalibrationRequest{
		OrganizationID: common.organization, ProjectID: common.project, AccountRef: connector.AnonymizeRef(accountID), KnowledgeCutoff: knowledgeCutoff,
		ReplayStart: audit.DateStart.AddDate(0, 0, *lookbackDays), ReplayEnd: audit.DateEnd, LookbackDays: *lookbackDays, HorizonDays: *horizonDays, StepDays: *stepDays, MinimumHistoryWindows: *minimumHistory, KeyVersion: strings.TrimSpace(*keyVersion),
	})
	if err != nil {
		return fmt.Errorf("build offline retrospective calibration backtest: %w", err)
	}
	result.Limitations = append(result.Limitations, "offline_export_account_registration_not_verified", "material_aggregate_excluded_missing_daily_promotion_binding_and_duplicate_attribution")
	return writeJSON(common.output, result)
}

func runImportXLSX(ctx context.Context, cfg config.Config, repository connector.MySQLRepository, args []string) error {
	flags := flag.NewFlagSet("import-xlsx", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	common := commonFlags{}
	bindCommon(flags, &common)
	accountDaily := flags.String("account-daily", "", "account daily XLSX export")
	projectDaily := flags.String("project-daily", "", "project daily XLSX export")
	promotionDaily := flags.String("promotion-daily", "", "promotion daily XLSX export")
	materialAggregate := flags.String("material-aggregate", "", "material aggregate XLSX export")
	idempotencyKey := flags.String("idempotency-key", "", "stable offline import idempotency key")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if _, err := validateCommon(common); err != nil || strings.TrimSpace(*idempotencyKey) == "" {
		return connector.ErrInvalidFact
	}
	externalAccount, err := repository.ResolveAnyExternalAccountID(ctx, strings.TrimSpace(common.organization), strings.TrimSpace(common.project), strings.TrimSpace(common.account))
	if err != nil {
		return fmt.Errorf("resolve registered Connector account: %w", err)
	}
	paths := []string{*accountDaily, *projectDaily, *promotionDaily, *materialAggregate}
	sources, err := readOfflineSources(paths)
	if err != nil {
		return err
	}
	for index := range sources {
		defer clear(sources[index].Content)
	}
	cipher, err := insights.NewAESGCMSecretCipher(cfg.OceanEngine.MasterKey, cfg.OceanEngine.MasterKeyVersion)
	if err != nil {
		return fmt.Errorf("configure offline evidence encryption: %w", err)
	}
	result, err := (connector.OfflineXLSXImporter{Writer: repository, Cipher: cipher}).Import(ctx, connector.OfflineXLSXImportRequest{
		OrganizationID: strings.TrimSpace(common.organization), ProjectID: strings.TrimSpace(common.project), AccountID: strings.TrimSpace(common.account), ExternalAccount: externalAccount,
		IdempotencyKey: strings.TrimSpace(*idempotencyKey), TimeZone: "Asia/Shanghai", Currency: "CNY", Sources: sources,
	})
	if err != nil {
		return fmt.Errorf("import offline Connector evidence: %w", err)
	}
	return writeJSON(common.output, result)
}

func readOfflineSources(paths []string) ([]connector.OfflineXLSXSource, error) {
	sources := make([]connector.OfflineXLSXSource, 0, len(paths))
	for index, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			return nil, fmt.Errorf("offline source %d is required", index+1)
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			var pathErr *os.PathError
			if errors.As(readErr, &pathErr) {
				return nil, fmt.Errorf("read offline source %d: %v", index+1, pathErr.Err)
			}
			return nil, fmt.Errorf("read offline source %d", index+1)
		}
		sources = append(sources, connector.OfflineXLSXSource{Name: path, Content: content})
	}
	return sources, nil
}

func runBacktest(ctx context.Context, reader connector.SnapshotReader, args []string) error {
	flags := flag.NewFlagSet("backtest", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	common := commonFlags{}
	bindCommon(flags, &common)
	knowledgeText := flags.String("knowledge-cutoff", "", "RFC3339 Connector knowledge cutoff")
	startText := flags.String("replay-start", "", "RFC3339 first rolling prediction cutoff")
	endText := flags.String("replay-end", "", "RFC3339 exclusive replay label boundary")
	lookbackDays := flags.Int("lookback-days", 14, "history window in days")
	horizonDays := flags.Int("horizon-days", 7, "prediction horizon in days")
	stepDays := flags.Int("step-days", 7, "days between rolling cutoffs")
	minimumHistory := flags.Int("minimum-history-windows", 7, "minimum observed history windows per case")
	keyVersion := flags.String("key-version", "", "non-secret export key version")
	if err := flags.Parse(args); err != nil {
		return err
	}
	knowledge, err := parseTime(*knowledgeText, "knowledge-cutoff")
	if err != nil {
		return err
	}
	replayStart, err := parseTime(*startText, "replay-start")
	if err != nil {
		return err
	}
	replayEnd, err := parseTime(*endText, "replay-end")
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
	builder := connector.RetrospectiveCalibrationBuilder{Reader: reader, Key: key}
	result, err := builder.Build(ctx, connector.RetrospectiveCalibrationRequest{OrganizationID: common.organization, ProjectID: common.project, AccountRef: accountRef, KnowledgeCutoff: knowledge, ReplayStart: replayStart, ReplayEnd: replayEnd, LookbackDays: *lookbackDays, HorizonDays: *horizonDays, StepDays: *stepDays, MinimumHistoryWindows: *minimumHistory, KeyVersion: strings.TrimSpace(*keyVersion)})
	if err != nil {
		return fmt.Errorf("build retrospective calibration backtest: %w", err)
	}
	return writeJSON(common.output, result)
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
	fmt.Fprintln(os.Stderr, "usage: cookies-delivery-calibration <import-xlsx|backtest-xlsx|backtest-launch-batches|audit|export|backtest> [flags]")
}
