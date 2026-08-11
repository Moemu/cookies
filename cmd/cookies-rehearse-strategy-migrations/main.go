// cookies-rehearse-strategy-migrations stages the Strategy workspace migration
// set against a populated, disposable local MySQL schema and emits lock/latency
// evidence. It refuses non-loopback hosts, non-rehearsal schema names, and any
// schema that already contains tables.
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"

	"github.com/shikanon/cookies/internal/platform/migration"
)

const rehearsalDSNEnv = "COOKIES_REHEARSAL_MYSQL_DSN"

var stagedMigrationSuffixes = []string{
	"migrations/platform/20260810100000_research_orchestration_v2.up.sql",
	"migrations/platform/20260810101000_document_parse_pipeline_v2.up.sql",
	"migrations/strategy/20260810102000_strategy_assistant_memory_v2.up.sql",
	"migrations/strategy/20260810103000_strategy_product_events.up.sql",
	"migrations/strategy/20260810103100_strategy_product_event_type_constraint.up.sql",
	"migrations/strategy/20260810104000_strategy_research_adoption_v1.up.sql",
	"migrations/strategy/20260810104100_strategy_research_proposal_fingerprint.up.sql",
	"migrations/strategy/20260810105000_strategy_perspective_analysis.up.sql",
	"migrations/platform/20260811100000_document_vision_fallback.up.sql",
	"migrations/platform/20260811101000_document_vision_external_tasks.up.sql",
	"migrations/platform/20260811101100_document_vision_external_task_intents.up.sql",
	"migrations/provider/20260811101000_provider_document_vision_routes.up.sql",
	"migrations/platform/20260811102000_document_vision_input_conversions.up.sql",
}

type rehearsalConfig struct {
	Rows             int
	ResearchRows     int
	DocumentRows     int
	MemoryRows       int
	ProviderRows     int
	ProposalRows     int
	ProductEventRows int
	AnalysisRows     int
	MaxMigration     time.Duration
	MaxReadBlock     time.Duration
	BaselineLabel    string
	ProductionLike   bool
	OutputPath       string
}

type migrationMeasurement struct {
	Migration        string `json:"migration"`
	ProbeTable       string `json:"probe_table"`
	RowsBefore       int64  `json:"rows_before"`
	DurationMS       int64  `json:"duration_ms"`
	MaxReadLatencyMS int64  `json:"max_read_latency_ms"`
	ReadAttempts     int    `json:"read_attempts"`
	ReadErrors       int    `json:"read_errors"`
	WithinThreshold  bool   `json:"within_threshold"`
}

type rehearsalReport struct {
	ContractVersion string                 `json:"contract_version"`
	Schema          string                 `json:"schema"`
	ServerVersion   string                 `json:"server_version"`
	BaselineLabel   string                 `json:"baseline_label,omitempty"`
	ProductionLike  bool                   `json:"production_like"`
	StartedAt       time.Time              `json:"started_at"`
	CompletedAt     time.Time              `json:"completed_at"`
	SeedRows        map[string]int         `json:"seed_rows"`
	FinalRows       map[string]int64       `json:"final_rows"`
	MaxMigrationMS  int64                  `json:"max_migration_ms"`
	MaxReadBlockMS  int64                  `json:"max_read_block_ms"`
	Measurements    []migrationMeasurement `json:"measurements"`
	Warnings        []string               `json:"warnings"`
	Passed          bool                   `json:"passed"`
}

func main() {
	config := parseFlags()
	dsn := strings.TrimSpace(os.Getenv(rehearsalDSNEnv))
	if dsn == "" {
		fatalf("%s is required", rehearsalDSNEnv)
	}
	parsed, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		fatalf("parse rehearsal DSN: %v", err)
	}
	if err := validateRehearsalTarget(parsed); err != nil {
		fatalf("unsafe rehearsal target: %v", err)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fatalf("open rehearsal database: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(8)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		fatalf("ping rehearsal database: %v", err)
	}
	if err := requireEmptySchema(ctx, db, parsed.DBName); err != nil {
		fatalf("rehearsal schema preflight: %v", err)
	}
	if err := acquireRehearsalLock(ctx, db); err != nil {
		fatalf("acquire rehearsal lock: %v", err)
	}
	defer db.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", rehearsalLockName(parsed.DBName))

	report, err := runRehearsal(ctx, db, parsed.DBName, config)
	if err != nil {
		fatalf("migration rehearsal failed: %v", err)
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fatalf("encode report: %v", err)
	}
	if config.OutputPath != "" {
		if err := os.WriteFile(config.OutputPath, append(encoded, '\n'), 0o600); err != nil {
			fatalf("write report: %v", err)
		}
	}
	fmt.Println(string(encoded))
	if !report.Passed {
		os.Exit(2)
	}
}

func parseFlags() rehearsalConfig {
	var config rehearsalConfig
	flag.IntVar(&config.Rows, "rows", 10_000, "default synthetic row count per populated table")
	flag.IntVar(&config.ResearchRows, "research-rows", 0, "platform_research_runs rows (defaults to -rows)")
	flag.IntVar(&config.DocumentRows, "document-rows", 0, "platform_knowledge_documents rows (defaults to -rows)")
	flag.IntVar(&config.MemoryRows, "memory-rows", 0, "strategy_conversation_memories rows (defaults to -rows)")
	flag.IntVar(&config.ProviderRows, "provider-rows", 0, "provider_connections rows (defaults to -rows)")
	flag.IntVar(&config.ProposalRows, "proposal-rows", 0, "strategy_artifact_proposals rows (defaults to -rows)")
	flag.IntVar(&config.ProductEventRows, "product-event-rows", 0, "strategy_product_events rows (defaults to -rows)")
	flag.IntVar(&config.AnalysisRows, "analysis-rows", 0, "strategy_review_analyses rows (defaults to -rows)")
	flag.DurationVar(&config.MaxMigration, "max-migration", 30*time.Second, "maximum allowed duration for one staged migration")
	flag.DurationVar(&config.MaxReadBlock, "max-read-block", 2*time.Second, "maximum allowed concurrent read latency")
	flag.StringVar(&config.BaselineLabel, "baseline-label", "", "non-sensitive production baseline identifier")
	flag.BoolVar(&config.ProductionLike, "production-like", false, "assert row counts came from an approved production baseline")
	flag.StringVar(&config.OutputPath, "output", "", "optional JSON report path")
	flag.Parse()
	if config.Rows < 1 || config.Rows > 2_000_000 {
		fatalf("-rows must be between 1 and 2000000")
	}
	for _, target := range []*int{
		&config.ResearchRows, &config.DocumentRows, &config.MemoryRows,
		&config.ProviderRows, &config.ProposalRows, &config.ProductEventRows,
		&config.AnalysisRows,
	} {
		if *target == 0 {
			*target = config.Rows
		}
		if *target < 1 || *target > 2_000_000 {
			fatalf("table row counts must be between 1 and 2000000")
		}
	}
	if config.MaxMigration <= 0 || config.MaxReadBlock <= 0 {
		fatalf("migration and read-block thresholds must be positive")
	}
	if config.ProductionLike && strings.TrimSpace(config.BaselineLabel) == "" {
		fatalf("-production-like requires a non-sensitive -baseline-label")
	}
	return config
}

func validateRehearsalTarget(config *mysqldriver.Config) error {
	if config == nil {
		return errors.New("DSN config is required")
	}
	if !strings.HasPrefix(config.DBName, "cookies_rehearsal_") {
		return fmt.Errorf("database %q must start with cookies_rehearsal_", config.DBName)
	}
	if config.Net != "tcp" {
		return fmt.Errorf("network %q is not tcp loopback", config.Net)
	}
	host, _, err := net.SplitHostPort(config.Addr)
	if err != nil {
		return fmt.Errorf("parse address %q: %w", config.Addr, err)
	}
	if host != "localhost" {
		ip := net.ParseIP(strings.Trim(host, "[]"))
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("host %q is not loopback", host)
		}
	}
	if !config.MultiStatements {
		return errors.New("multiStatements=true is required for repository migrations")
	}
	return nil
}

func requireEmptySchema(ctx context.Context, db *sql.DB, schema string) error {
	var tables int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ?`, schema).Scan(&tables); err != nil {
		return err
	}
	if tables != 0 {
		return fmt.Errorf("schema %q contains %d tables; use a newly created disposable schema", schema, tables)
	}
	return nil
}

func acquireRehearsalLock(ctx context.Context, db *sql.DB) error {
	var acquired int
	if err := db.QueryRowContext(ctx, "SELECT GET_LOCK(?, 0)", rehearsalLockName(databaseName(ctx, db))).Scan(&acquired); err != nil {
		return err
	}
	if acquired != 1 {
		return errors.New("another rehearsal owns this schema")
	}
	return nil
}

func databaseName(ctx context.Context, db *sql.DB) string {
	var name string
	_ = db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&name)
	return name
}

func rehearsalLockName(schema string) string {
	digest := sha256.Sum256([]byte(schema))
	return fmt.Sprintf("cookies:migration:%x", digest[:20])
}

func runRehearsal(ctx context.Context, db *sql.DB, schema string, config rehearsalConfig) (rehearsalReport, error) {
	started := time.Now().UTC()
	paths, err := migration.Discover("migrations")
	if err != nil {
		return rehearsalReport{}, err
	}
	baseline, staged, err := partitionMigrations(paths)
	if err != nil {
		return rehearsalReport{}, err
	}
	if err := migration.ApplyPaths(ctx, db, baseline); err != nil {
		return rehearsalReport{}, fmt.Errorf("apply baseline: %w", err)
	}
	if err := seedBaselineRows(ctx, db, config); err != nil {
		return rehearsalReport{}, fmt.Errorf("seed baseline volume: %w", err)
	}

	report := rehearsalReport{
		ContractVersion: "strategy-migration-rehearsal-report/v1",
		Schema:          schema, BaselineLabel: strings.TrimSpace(config.BaselineLabel),
		ProductionLike: config.ProductionLike, StartedAt: started,
		SeedRows: map[string]int{
			"platform_research_runs":         config.ResearchRows,
			"platform_knowledge_documents":   config.DocumentRows,
			"strategy_conversation_memories": config.MemoryRows,
			"provider_connections":           config.ProviderRows,
			"strategy_artifact_proposals":    config.ProposalRows,
			"strategy_product_events":        config.ProductEventRows,
			"strategy_review_analyses":       config.AnalysisRows,
		},
		MaxMigrationMS: config.MaxMigration.Milliseconds(),
		MaxReadBlockMS: config.MaxReadBlock.Milliseconds(), Passed: true,
	}
	if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&report.ServerVersion); err != nil {
		return rehearsalReport{}, err
	}
	if !config.ProductionLike {
		report.Warnings = append(report.Warnings, "synthetic volume only; do not use this report as production-like lock evidence")
	}

	for _, path := range staged {
		measurement, err := measureMigration(ctx, db, path, config)
		if err != nil {
			return rehearsalReport{}, err
		}
		report.Measurements = append(report.Measurements, measurement)
		if !measurement.WithinThreshold {
			report.Passed = false
		}
		if hasMigrationSuffix(path, "strategy/20260810102000_strategy_assistant_memory_v2.up.sql") {
			if err := seedArtifactProposals(ctx, db, config.ProposalRows); err != nil {
				return rehearsalReport{}, err
			}
		}
		if hasMigrationSuffix(path, "strategy/20260810103000_strategy_product_events.up.sql") {
			if err := seedProductEvents(ctx, db, config.ProductEventRows); err != nil {
				return rehearsalReport{}, err
			}
		}
	}
	report.FinalRows = map[string]int64{}
	for table := range report.SeedRows {
		count, err := tableRows(ctx, db, table)
		if err != nil {
			return rehearsalReport{}, err
		}
		report.FinalRows[table] = count
	}
	report.CompletedAt = time.Now().UTC()
	return report, nil
}

func partitionMigrations(paths []string) ([]string, []string, error) {
	wanted := make(map[string]bool, len(stagedMigrationSuffixes))
	for _, suffix := range stagedMigrationSuffixes {
		wanted[filepath.ToSlash(suffix)] = false
	}
	var baseline, staged []string
	for _, path := range paths {
		normalized := filepath.ToSlash(path)
		matched := false
		for suffix := range wanted {
			if strings.HasSuffix(normalized, suffix) {
				wanted[suffix] = true
				staged = append(staged, path)
				matched = true
				break
			}
		}
		if !matched {
			baseline = append(baseline, path)
		}
	}
	for suffix, found := range wanted {
		if !found {
			return nil, nil, fmt.Errorf("staged migration not found: %s", suffix)
		}
	}
	sort.Slice(staged, func(i, j int) bool {
		left, right := filepath.Base(staged[i]), filepath.Base(staged[j])
		if left != right {
			return left < right
		}
		return filepath.ToSlash(staged[i]) < filepath.ToSlash(staged[j])
	})
	return baseline, staged, nil
}

func seedBaselineRows(ctx context.Context, db *sql.DB, config rehearsalConfig) error {
	maxRows := max(
		config.ResearchRows, config.DocumentRows, config.MemoryRows,
		config.ProviderRows, config.ProposalRows, config.ProductEventRows,
		config.AnalysisRows,
	)
	if _, err := db.ExecContext(ctx, `CREATE TABLE rehearsal_seed_numbers (n INT NOT NULL PRIMARY KEY)`); err != nil {
		return err
	}
	for start := 1; start <= maxRows; start += 1000 {
		end := min(maxRows, start+999)
		var query strings.Builder
		query.WriteString("INSERT INTO rehearsal_seed_numbers (n) VALUES ")
		args := make([]any, 0, end-start+1)
		for value := start; value <= end; value++ {
			if value > start {
				query.WriteByte(',')
			}
			query.WriteString("(?)")
			args = append(args, value)
		}
		if _, err := db.ExecContext(ctx, query.String(), args...); err != nil {
			return err
		}
	}
	statements := []string{
		fmt.Sprintf(`INSERT INTO platform_research_runs
			(id, organization_id, project_id, mode, category, purpose, query_text, document_ids,
			 disclosed_fields, disclosed_chunk_ids, status, confirmed_by, confirmed_at, created_at, updated_at)
			SELECT CONCAT('research_rehearsal_', LPAD(n, 12, '0')), 'org_rehearsal', 'project_rehearsal',
			 'web', 'general', 'deep_research', CONCAT('query ', n), JSON_ARRAY(), JSON_ARRAY(), JSON_ARRAY(),
			 'succeeded', 'user_rehearsal', NOW(6), NOW(6), NOW(6)
			FROM rehearsal_seed_numbers WHERE n <= %d`, config.ResearchRows),
		fmt.Sprintf(`INSERT INTO platform_knowledge_documents
			(id, organization_id, project_id, title, filename, mime_type, size_bytes, content_sha256,
			 text_sha256, extracted_text, object_provider, object_bucket, object_key, status,
			 parser_code, parser_version, parsed_at, created_by, created_at, updated_at)
			SELECT CONCAT('document_rehearsal_', LPAD(n, 12, '0')), 'org_rehearsal', 'project_rehearsal',
			 CONCAT('Document ', n), CONCAT('document-', n, '.pdf'), 'application/pdf', 1024,
			 SHA2(CONCAT('content-', n), 256), SHA2(CONCAT('text-', n), 256), CONCAT('extracted ', n),
			 'tos', 'rehearsal', CONCAT('assets/org/project/', n), 'ready', 'tika', '3.2.3', NOW(6),
			 'user_rehearsal', NOW(6), NOW(6)
			FROM rehearsal_seed_numbers WHERE n <= %d`, config.DocumentRows),
		fmt.Sprintf(`INSERT INTO strategy_conversation_memories
			(conversation_id, organization_id, project_id, summary, open_questions, last_message_id, version, updated_at)
			SELECT CONCAT('conversation_rehearsal_', LPAD(n, 12, '0')), 'org_rehearsal', 'project_rehearsal',
			 CONCAT('summary ', n), JSON_ARRAY(), CONCAT('message_', n), 1, NOW(6)
			FROM rehearsal_seed_numbers WHERE n <= %d`, config.MemoryRows),
		fmt.Sprintf(`INSERT INTO provider_connections
			(id, connection_code, connection_type, status, version, created_at, updated_at)
			SELECT CONCAT('connection_rehearsal_', LPAD(n, 12, '0')), CONCAT('rehearsal.connection.', n),
			 'adapter_gateway', 'enabled', 1, NOW(6), NOW(6)
			FROM rehearsal_seed_numbers WHERE n <= %d`, config.ProviderRows),
		fmt.Sprintf(`INSERT INTO platform_agent_tasks
			(id, organization_id, project_id, source_system, source_type, source_id, kind, status,
			 version, input_snapshot, result_type, result_id, result_version, created_by_kind,
			 created_by_id, created_at, updated_at)
			SELECT CONCAT('agenttask_rehearsal_', LPAD(n, 12, '0')), 'org_rehearsal', 'project_rehearsal',
			 'strategy', 'review', CONCAT('review_rehearsal_', LPAD(n, 12, '0')), 'strategy.review.deep',
			 'succeeded', 1, JSON_OBJECT(), 'strategy.review_analysis',
			 CONCAT('analysis_rehearsal_', LPAD(n, 12, '0')), 1, 'service', 'rehearsal', NOW(6), NOW(6)
			FROM rehearsal_seed_numbers WHERE n <= %d`, config.AnalysisRows),
		fmt.Sprintf(`INSERT INTO strategy_reviews
			(id, organization_id, project_id, strategy_id, candidate_revision, candidate_content_hash,
			 brief_id, brief_version, project_context_version, status, created_by, created_at, updated_at)
			SELECT CONCAT('review_rehearsal_', LPAD(n, 12, '0')), 'org_rehearsal', 'project_rehearsal',
			 CONCAT('strategy_rehearsal_', LPAD(n, 12, '0')), 1,
			 CONCAT('sha256:', SHA2(CONCAT('candidate-', n), 256)), CONCAT('brief_rehearsal_', n),
			 1, 1, 'approved', 'user_rehearsal', NOW(6), NOW(6)
			FROM rehearsal_seed_numbers WHERE n <= %d`, config.AnalysisRows),
		fmt.Sprintf(`INSERT INTO strategy_review_analyses
			(id, organization_id, project_id, review_id, strategy_id, candidate_revision,
			 candidate_content_hash, agent_task_id, status, summary, findings, model_alias,
			 model_version, route_revision_id, response_mode, api_mode, background, usage_json,
			 latency_ms, created_by, created_at, updated_at)
			SELECT CONCAT('analysis_rehearsal_', LPAD(n, 12, '0')), 'org_rehearsal', 'project_rehearsal',
			 CONCAT('review_rehearsal_', LPAD(n, 12, '0')), CONCAT('strategy_rehearsal_', LPAD(n, 12, '0')),
			 1, CONCAT('sha256:', SHA2(CONCAT('candidate-', n), 256)),
			 CONCAT('agenttask_rehearsal_', LPAD(n, 12, '0')), 'succeeded', 'rehearsal analysis',
			 JSON_ARRAY(), 'cookies.text.standard', 'rehearsal', 'route_rehearsal', 'sync', 'responses',
			 FALSE, JSON_OBJECT(), 1, 'service', NOW(6), NOW(6)
			FROM rehearsal_seed_numbers WHERE n <= %d`, config.AnalysisRows),
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func seedArtifactProposals(ctx context.Context, db *sql.DB, rows int) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO strategy_artifact_proposals
		(id, organization_id, project_id, workspace_id, conversation_id, proposal_kind, target_type,
		 target_id, target_version, base_content_hash, operations, rationale, risk, status,
		 source_message_id, created_by, version, created_at, updated_at)
		SELECT CONCAT('proposal_rehearsal_', LPAD(n, 12, '0')), 'org_rehearsal', 'project_rehearsal',
		 'workspace_rehearsal', 'conversation_rehearsal', 'assistant', 'brief_draft',
		 CONCAT('brief_', n), 1, CONCAT('sha256:', SHA2(CONCAT('brief-', n), 256)), JSON_ARRAY(),
		 'rehearsal proposal', 'low', 'proposed', NULL, 'user_rehearsal', 1, NOW(6), NOW(6)
		FROM rehearsal_seed_numbers WHERE n <= %d`, rows))
	return err
}

func seedProductEvents(ctx context.Context, db *sql.DB, rows int) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO strategy_product_events
		(id, organization_id, project_id, event_type, actor_kind, actor_id_hash, attributes_json, occurred_at)
		SELECT CONCAT('event_rehearsal_', LPAD(n, 12, '0')), 'org_rehearsal', 'project_rehearsal',
		 'workspace.opened', 'service', SHA2(CONCAT('actor-', n), 256), JSON_OBJECT(), NOW(6)
		FROM rehearsal_seed_numbers WHERE n <= %d`, rows))
	return err
}

func measureMigration(ctx context.Context, db *sql.DB, path string, config rehearsalConfig) (migrationMeasurement, error) {
	probeTable := migrationProbeTable(path)
	rowsBefore, err := tableRows(ctx, db, probeTable)
	if err != nil {
		return migrationMeasurement{}, fmt.Errorf("count %s before %s: %w", probeTable, path, err)
	}
	probeCtx, cancel := context.WithCancel(ctx)
	ready := make(chan struct{})
	result := make(chan readProbeResult, 1)
	go runReadProbe(probeCtx, db, probeTable, ready, result)
	<-ready
	started := time.Now()
	err = migration.ApplyPaths(ctx, db, []string{path})
	duration := time.Since(started)
	cancel()
	probe := <-result
	if err != nil {
		return migrationMeasurement{}, fmt.Errorf("apply staged migration %s after %s: %w", filepath.ToSlash(path), duration, err)
	}
	measurement := migrationMeasurement{
		Migration: filepath.ToSlash(path), ProbeTable: probeTable, RowsBefore: rowsBefore,
		DurationMS: duration.Milliseconds(), MaxReadLatencyMS: probe.maxLatency.Milliseconds(),
		ReadAttempts: probe.attempts, ReadErrors: probe.errors,
	}
	measurement.WithinThreshold = duration <= config.MaxMigration &&
		probe.maxLatency <= config.MaxReadBlock && probe.errors == 0
	return measurement, nil
}

type readProbeResult struct {
	maxLatency time.Duration
	attempts   int
	errors     int
}

func runReadProbe(ctx context.Context, db *sql.DB, table string, ready chan<- struct{}, result chan<- readProbeResult) {
	var value readProbeResult
	var once sync.Once
	for {
		started := time.Now()
		var marker int
		err := db.QueryRowContext(ctx, "SELECT 1 FROM "+table+" LIMIT 1").Scan(&marker)
		latency := time.Since(started)
		if latency > value.maxLatency {
			value.maxLatency = latency
		}
		value.attempts++
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, sql.ErrNoRows) {
			value.errors++
		}
		once.Do(func() { close(ready) })
		select {
		case <-ctx.Done():
			result <- value
			return
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func migrationProbeTable(path string) string {
	normalized := filepath.ToSlash(path)
	switch {
	case strings.Contains(normalized, "provider_document_vision_routes"):
		return "provider_connections"
	case strings.Contains(normalized, "research_orchestration"):
		return "platform_research_runs"
	case strings.Contains(normalized, "document_"):
		return "platform_knowledge_documents"
	case strings.Contains(normalized, "assistant_memory"):
		return "strategy_conversation_memories"
	case strings.Contains(normalized, "product_event_type_constraint"):
		return "strategy_product_events"
	case strings.Contains(normalized, "strategy_product_events.up.sql"):
		return "strategy_conversation_memories"
	case strings.Contains(normalized, "research_adoption"),
		strings.Contains(normalized, "proposal_fingerprint"):
		return "strategy_artifact_proposals"
	case strings.Contains(normalized, "perspective_analysis"):
		return "strategy_review_analyses"
	default:
		return "platform_schema_migrations"
	}
}

func tableRows(ctx context.Context, db *sql.DB, table string) (int64, error) {
	var count int64
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count)
	return count, err
}

func hasMigrationSuffix(path, suffix string) bool {
	return strings.HasSuffix(filepath.ToSlash(path), filepath.ToSlash(suffix))
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
