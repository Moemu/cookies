package migration

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

func TestApplyPathsRejectsChangedAppliedMigration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("COOKIES_TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("COOKIES_TEST_MYSQL_DSN is not configured")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	path := filepath.Join(t.TempDir(), "20990101000000_checksum_probe.up.sql")
	if err := os.WriteFile(path, []byte("SELECT 1;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	migrationID := filepath.ToSlash(path)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM "+trackingTable+" WHERE migration_id = ?", migrationID)
	})
	if err := ApplyPaths(t.Context(), db, []string{path}); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	crlfContents := []byte("SELECT 1;\r\n")
	if err := os.WriteFile(path, crlfContents, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ApplyPaths(t.Context(), db, []string{path}); err != nil {
		t.Fatalf("equivalent CRLF apply: %v", err)
	}
	legacyCRLFChecksum := sha256Hex(crlfContents)
	if _, err := db.ExecContext(t.Context(), "UPDATE "+trackingTable+" SET checksum_sha256 = ? WHERE migration_id = ?", legacyCRLFChecksum, migrationID); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("SELECT 1;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ApplyPaths(t.Context(), db, []string{path}); err != nil {
		t.Fatalf("legacy CRLF checksum upgrade: %v", err)
	}
	var upgradedChecksum string
	if err := db.QueryRowContext(t.Context(), "SELECT checksum_sha256 FROM "+trackingTable+" WHERE migration_id = ?", migrationID).Scan(&upgradedChecksum); err != nil {
		t.Fatal(err)
	}
	if want := migrationChecksumsFor([]byte("SELECT 1;\n")).canonical; upgradedChecksum != want {
		t.Fatalf("upgraded checksum = %s, want %s", upgradedChecksum, want)
	}
	if err := os.WriteFile(path, []byte("SELECT 2;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ApplyPaths(t.Context(), db, []string{path}); err == nil || !strings.Contains(err.Error(), "checksum changed after apply") {
		t.Fatalf("changed migration error = %v", err)
	}
}
