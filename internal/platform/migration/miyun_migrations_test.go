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
	intake := readMiyunMigration(t, filepath.Join(root, "migrations", "insights", "20260810102000_insight_miyun_product_analysis_intake.up.sql"))
	authorizedImport := readMiyunMigration(t, filepath.Join(root, "migrations", "insights", "20260810103000_insight_miyun_crawl_authorized_import.up.sql"))
	all := assets + insights + intake + authorizedImport

	for _, table := range []string{
		"asset_external_imports",
		"insight_miyun_connections",
		"insight_miyun_product_profiles",
		"insight_miyun_crawl_jobs",
		"insight_miyun_materials",
		"insight_miyun_material_snapshots",
		"insight_miyun_handoffs",
	} {
		if !strings.Contains(all, "CREATE TABLE IF NOT EXISTS "+table) {
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
		if !strings.Contains(all, status) {
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
		"analysis_method VARCHAR(24)",
		"input_snapshot JSON NOT NULL",
		"field_sources JSON NOT NULL",
		"analysis_warnings JSON NOT NULL",
		"UNIQUE KEY uq_insight_miyun_materials_manual_idempotency",
		"(import_method = 'manual' AND first_seen_crawl_job_id IS NULL",
		"cumulative_impressions_raw VARCHAR(64) NOT NULL",
		"ADD COLUMN idempotency_key VARCHAR(128)",
		"ADD COLUMN runtime_job_id VARCHAR(96)",
		"ADD COLUMN resource_url_ciphertext VARBINARY(4096)",
		"ADD COLUMN source_ref_status VARCHAR(16)",
		"ADD COLUMN source_page BIGINT UNSIGNED",
		"ADD COLUMN related_creators_raw VARCHAR(64)",
		"CONSTRAINT chk_insight_miyun_materials_decision",
	} {
		if !strings.Contains(all, required) {
			t.Errorf("missing required migration contract %q", required)
		}
	}

	for _, forbidden := range []string{"provider_job_id", "provider_output_id", "plaintext_cookie", "cookie_plaintext"} {
		if strings.Contains(all, forbidden) {
			t.Errorf("forbidden migration field %q", forbidden)
		}
	}
}

func TestMiyunReturnMigrationContract(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "..")
	returns := readMiyunMigration(t, filepath.Join(root, "migrations", "insights", "20260811110000_insight_miyun_handoff_returns.up.sql"))
	lineage := readMiyunMigration(t, filepath.Join(root, "migrations", "assets", "20260811110000_asset_returned_lineage.up.sql"))
	for _, required := range []string{
		"CREATE TABLE insight_miyun_handoff_returns",
		"handoff_version BIGINT NOT NULL",
		"input_hash CHAR(64)",
		"asset_id VARCHAR(96)",
		"asset_version BIGINT",
		"upload_idempotency_key VARCHAR(128)",
		"upload_request_hash CHAR(64)",
		"mark_idempotency_key VARCHAR(128)",
		"returned_by VARCHAR(96)",
		"UNIQUE KEY uq_insight_miyun_return_idem",
		"FOREIGN KEY (organization_id, project_id, handoff_id) REFERENCES insight_miyun_handoffs",
		"CHECK (status IN ('created','uploaded','failed','returned'))",
	} {
		if !strings.Contains(returns, required) {
			t.Errorf("missing return migration contract %q", required)
		}
	}
	if !strings.Contains(lineage, "'returned_from'") {
		t.Error("returned Asset lineage relation is missing from the migration")
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
