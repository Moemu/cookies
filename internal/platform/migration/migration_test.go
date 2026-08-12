package migration

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDiscoverOrdersByMigrationVersionAcrossModules(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := []string{
		filepath.Join(root, "assets", "20260721180000_assets.up.sql"),
		filepath.Join(root, "platform", "20260721170000_platform.up.sql"),
		filepath.Join(root, "strategy", "20260721180000_strategy.up.sql"),
		filepath.Join(root, "platform", "ignore.down.sql"),
	}
	for _, path := range paths {
		if filepath.Ext(path) != ".sql" || filepath.Base(path) == "ignore.down.sql" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("SELECT 1;"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := Discover(root)
	if err != nil {
		t.Fatalf("discover() error = %v", err)
	}
	want := []string{
		filepath.Join(root, "platform", "20260721170000_platform.up.sql"),
		filepath.Join(root, "assets", "20260721180000_assets.up.sql"),
		filepath.Join(root, "strategy", "20260721180000_strategy.up.sql"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discover() = %v, want %v", got, want)
	}
}

func TestMigrationChecksumsTreatLFAndCRLFAsOneIdentity(t *testing.T) {
	t.Parallel()

	lf := []byte("CREATE TABLE example (\n  id BIGINT NOT NULL\n);\n")
	crlf := []byte(strings.ReplaceAll(string(lf), "\n", "\r\n"))
	lfChecksums := migrationChecksumsFor(lf)
	crlfChecksums := migrationChecksumsFor(crlf)
	if lfChecksums.canonical != crlfChecksums.canonical {
		t.Fatalf("canonical checksums differ: lf=%s crlf=%s", lfChecksums.canonical, crlfChecksums.canonical)
	}
	if _, ok := lfChecksums.compatible[sha256Hex(crlf)]; !ok {
		t.Fatal("LF checkout must recognize a historical CRLF checksum")
	}
	if _, ok := crlfChecksums.compatible[sha256Hex(lf)]; !ok {
		t.Fatal("CRLF checkout must recognize a historical LF checksum")
	}
}

func TestMigrationChecksumCompatibilityAcceptsOnlyTheExactHistoricalVariant(t *testing.T) {
	const migrationID = "migrations/platform/20260810101000_document_parse_pipeline_v2.up.sql"
	checksums := migrationChecksumsFor([]byte("SELECT 1;\n"))
	if !migrationChecksumCompatible(migrationID, "2faab7a8e9c11652c8d8e321ee7ce126d5d349f5d424d90eb600c13c59de0df0", checksums) {
		t.Fatal("exact pre-candidate checksum should be compatible")
	}
	if migrationChecksumCompatible("migrations/platform/other.up.sql", "2faab7a8e9c11652c8d8e321ee7ce126d5d349f5d424d90eb600c13c59de0df0", checksums) {
		t.Fatal("historical checksum must not apply to another migration")
	}
	if migrationChecksumCompatible(migrationID, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", checksums) {
		t.Fatal("unknown content change must remain fail-closed")
	}
}

func TestMigrationChecksumsStillRejectContentAndLoneCRChanges(t *testing.T) {
	t.Parallel()

	original := migrationChecksumsFor([]byte("SELECT 1;\n"))
	for name, changed := range map[string][]byte{
		"statement": []byte("SELECT 2;\n"),
		"bom":       append([]byte{0xef, 0xbb, 0xbf}, []byte("SELECT 1;\n")...),
		"lone-cr":   []byte("SELECT 1;\r"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, ok := original.compatible[migrationChecksumsFor(changed).canonical]; ok {
				t.Fatalf("%s change was accepted", name)
			}
		})
	}
}
