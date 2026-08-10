package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMiyunPipelineMigrationContract(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "..")
	assets := readMiyunMigration(t, filepath.Join(root, "migrations", "assets", "20260810100000_asset_external_imports.up.sql"))
	insights := readMiyunMigration(t, filepath.Join(root, "migrations", "insights", "20260810101000_insight_miyun_pipeline.up.sql"))

	for _, table := range []string{
		"asset_external_imports",
		"insight_miyun_connections",
		"insight_miyun_product_profiles",
		"insight_miyun_crawl_jobs",
		"insight_miyun_materials",
		"insight_miyun_material_snapshots",
		"insight_miyun_handoffs",
	} {
		if !strings.Contains(assets+insights, "CREATE TABLE IF NOT EXISTS "+table) {
			t.Errorf("missing Miyun table %q", table)
		}
	}

	for _, status := range []string{
		"'unverified', 'ready', 'auth_required', 'disabled'",
		"'draft', 'confirmed', 'superseded'",
		"'queued', 'running', 'cooling_down', 'auth_required', 'partial', 'succeeded', 'failed', 'cancelled'",
		"'discovered', 'confirmed', 'rejected'",
		"'pending', 'downloading', 'imported', 'deduplicated', 'failed', 'skipped'",
		"'exporting', 'exported', 'delivered', 'returned', 'failed'",
		"'queued', 'running', 'succeeded', 'failed'",
	} {
		if !strings.Contains(assets+insights, status) {
			t.Errorf("missing required status set %q", status)
		}
	}

	for _, required := range []string{
		"UNIQUE KEY uq_insight_miyun_materials_source (organization_id, project_id, miyun_material_id)",
		"FOREIGN KEY (organization_id, platform_asset_id, platform_asset_version) REFERENCES asset_versions(organization_id, asset_id, version)",
		"FOREIGN KEY (organization_id, project_id, platform_asset_id, platform_asset_version) REFERENCES project_assets(organization_id, project_id, asset_id, asset_version)",
		"FOREIGN KEY (organization_id, project_id, insight_asset_id) REFERENCES insight_assets(organization_id, project_id, id)",
		"FOREIGN KEY (organization_id, project_id, material_id) REFERENCES insight_miyun_materials(organization_id, project_id, id)",
		"UNIQUE KEY uq_asset_external_imports_source (organization_id, project_id, source_provider, source_object_id)",
		"completed_pages BIGINT UNSIGNED NOT NULL DEFAULT 0",
		"deduplicated_count BIGINT UNSIGNED NOT NULL DEFAULT 0",
		"downloaded_count BIGINT UNSIGNED NOT NULL DEFAULT 0",
		"schema_version VARCHAR(64)",
		"cumulative_impressions BIGINT UNSIGNED NOT NULL DEFAULT 0",
		"manifest_version VARCHAR(64)",
		"CONSTRAINT chk_asset_external_imports_succeeded CHECK (status <> 'succeeded' OR committed_asset_id IS NOT NULL)",
	} {
		if !strings.Contains(assets+insights, required) {
			t.Errorf("missing required migration contract %q", required)
		}
	}

	for _, forbidden := range []string{"provider_job_id", "provider_output_id", "plaintext_cookie", "cookie_plaintext"} {
		if strings.Contains(assets+insights, forbidden) {
			t.Errorf("forbidden migration field %q", forbidden)
		}
	}
}

func readMiyunMigration(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
