package migration

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscoverOrdersByMigrationVersionAcrossModules(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := []string{
		filepath.Join(root, "assets", "20260721180000_assets.up.sql"),
		filepath.Join(root, "platform", "20260721170000_platform.up.sql"),
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

	got, err := discover(root)
	if err != nil {
		t.Fatalf("discover() error = %v", err)
	}
	want := []string{
		filepath.Join(root, "platform", "20260721170000_platform.up.sql"),
		filepath.Join(root, "assets", "20260721180000_assets.up.sql"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discover() = %v, want %v", got, want)
	}
}
