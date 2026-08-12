package main

import (
	"path/filepath"
	"strings"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
)

func TestValidateRehearsalTargetFailsClosed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{name: "production-looking schema", dsn: "user:pass@tcp(127.0.0.1:3306)/cookies?multiStatements=true", want: "cookies_rehearsal_"},
		{name: "remote host", dsn: "user:pass@tcp(mysql.example.com:3306)/cookies_rehearsal_test?multiStatements=true", want: "not loopback"},
		{name: "unix socket", dsn: "user:pass@unix(/tmp/mysql.sock)/cookies_rehearsal_test?multiStatements=true", want: "not tcp"},
		{name: "single statement", dsn: "user:pass@tcp(localhost:3306)/cookies_rehearsal_test", want: "multiStatements=true"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			config, err := mysqldriver.ParseDSN(test.dsn)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateRehearsalTarget(config); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validate error = %v, want %q", err, test.want)
			}
		})
	}
	config, err := mysqldriver.ParseDSN("user:pass@tcp(127.0.0.1:3307)/cookies_rehearsal_safe?multiStatements=true")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRehearsalTarget(config); err != nil {
		t.Fatalf("safe target rejected: %v", err)
	}
}

func TestPartitionMigrationsRequiresEveryStagedMigration(t *testing.T) {
	t.Parallel()
	paths := make([]string, 0, len(stagedMigrationSuffixes)+1)
	for _, suffix := range stagedMigrationSuffixes {
		paths = append(paths, filepath.FromSlash(suffix))
	}
	paths = append(paths, filepath.FromSlash("migrations/platform/20260721170000_platform_jobs.up.sql"))
	baseline, staged, err := partitionMigrations(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(baseline) != 1 || len(staged) != len(stagedMigrationSuffixes) {
		t.Fatalf("partition = %d baseline, %d staged", len(baseline), len(staged))
	}
	if _, _, err := partitionMigrations(paths[:len(paths)-2]); err == nil {
		t.Fatal("missing staged migration was accepted")
	}
}

func TestMigrationProbeTablesAreStaticIdentifiers(t *testing.T) {
	t.Parallel()
	allowed := map[string]bool{
		"platform_research_runs": true, "platform_knowledge_documents": true,
		"strategy_conversation_memories": true, "strategy_artifact_proposals": true,
		"strategy_review_analyses": true, "provider_connections": true,
		"strategy_product_events":    true,
		"platform_schema_migrations": true,
	}
	for _, suffix := range stagedMigrationSuffixes {
		if table := migrationProbeTable(filepath.FromSlash(suffix)); !allowed[table] {
			t.Fatalf("unsafe probe table %q for %s", table, suffix)
		}
	}
}

func TestRehearsalLockNameFitsMySQLLimitAndIsSchemaBound(t *testing.T) {
	t.Parallel()
	first := rehearsalLockName("cookies_rehearsal_" + strings.Repeat("a", 80))
	second := rehearsalLockName("cookies_rehearsal_" + strings.Repeat("b", 80))
	if len(first) > 64 || first == second || !strings.HasPrefix(first, "cookies:migration:") {
		t.Fatalf("lock names = %q and %q", first, second)
	}
}
