-- M02 adds explainable product-profile lineage and a manual material intake
-- path. Manual intake points at an existing Project AssetVersion and therefore
-- has no crawl job; crawler rows keep their existing foreign-key requirement.

ALTER TABLE insight_miyun_product_profiles
  ADD COLUMN product_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER status,
  ADD COLUMN brand_name VARCHAR(255) NOT NULL DEFAULT '' AFTER product_name,
  ADD COLUMN model_version VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER rule_version,
  ADD COLUMN analysis_method VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'rules' AFTER model_version,
  ADD COLUMN input_snapshot JSON NULL AFTER input_hash,
  ADD COLUMN field_sources JSON NULL AFTER input_snapshot,
  ADD COLUMN analysis_warnings JSON NULL AFTER field_sources,
  ADD CONSTRAINT chk_insight_miyun_product_profiles_analysis_method
    CHECK (analysis_method = 'rules'),
  ADD CONSTRAINT chk_insight_miyun_product_profiles_model_lineage
    CHECK (analysis_method <> 'rules' OR model_version IS NULL);

-- Preserve any Goal 1 draft rows with explicit legacy/unknown provenance. New
-- Goal 2 writes always supply their selected Project product and full snapshot.
UPDATE insight_miyun_product_profiles
SET product_id = id,
    input_snapshot = JSON_OBJECT('legacy_profile_id', id, 'migration', 'goal2-intake'),
    field_sources = JSON_ARRAY(JSON_OBJECT(
      'field', 'legacy_profile',
      'source_kind', 'migration',
      'source_refs', JSON_ARRAY(id),
      'confidence', 'unknown',
      'review_state', 'unknown',
      'explanation', 'Created before Goal 2 product intake lineage was available.'
    )),
    analysis_warnings = JSON_ARRAY('legacy_profile_input_unknown')
WHERE product_id IS NULL;

ALTER TABLE insight_miyun_product_profiles
  MODIFY COLUMN product_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  MODIFY COLUMN input_snapshot JSON NOT NULL,
  MODIFY COLUMN field_sources JSON NOT NULL,
  MODIFY COLUMN analysis_warnings JSON NOT NULL;

ALTER TABLE insight_miyun_material_snapshots
  DROP FOREIGN KEY fk_insight_miyun_material_snapshots_job;

ALTER TABLE insight_miyun_materials
  DROP FOREIGN KEY fk_insight_miyun_materials_job;

ALTER TABLE insight_miyun_materials
  MODIFY COLUMN first_seen_crawl_job_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN import_method VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'crawler' AFTER first_seen_crawl_job_id,
  ADD COLUMN manual_idempotency_key VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER import_method,
  ADD COLUMN manual_request_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER manual_idempotency_key,
  ADD UNIQUE KEY uq_insight_miyun_materials_manual_idempotency
    (organization_id, project_id, manual_idempotency_key),
  ADD CONSTRAINT fk_insight_miyun_materials_job
    FOREIGN KEY (organization_id, project_id, first_seen_crawl_job_id)
    REFERENCES insight_miyun_crawl_jobs(organization_id, project_id, id),
  ADD CONSTRAINT chk_insight_miyun_materials_import_method
    CHECK (import_method IN ('crawler', 'manual')),
  ADD CONSTRAINT chk_insight_miyun_materials_origin
    CHECK (
      (import_method = 'crawler' AND first_seen_crawl_job_id IS NOT NULL
        AND manual_idempotency_key IS NULL AND manual_request_hash IS NULL)
      OR
      (import_method = 'manual' AND first_seen_crawl_job_id IS NULL
        AND manual_idempotency_key IS NOT NULL AND manual_request_hash IS NOT NULL
        AND platform_asset_id IS NOT NULL AND platform_asset_version IS NOT NULL
        AND insight_asset_id IS NOT NULL)
    );

ALTER TABLE insight_miyun_material_snapshots
  MODIFY COLUMN crawl_job_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN import_method VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'crawler' AFTER crawl_job_id,
  ADD COLUMN cumulative_impressions_raw VARCHAR(64) NOT NULL DEFAULT '0' AFTER cumulative_impressions,
  ADD CONSTRAINT fk_insight_miyun_material_snapshots_job
    FOREIGN KEY (organization_id, project_id, crawl_job_id)
    REFERENCES insight_miyun_crawl_jobs(organization_id, project_id, id),
  ADD CONSTRAINT chk_insight_miyun_material_snapshots_import_method
    CHECK (import_method IN ('crawler', 'manual')),
  ADD CONSTRAINT chk_insight_miyun_material_snapshots_origin
    CHECK (
      (import_method = 'crawler' AND crawl_job_id IS NOT NULL)
      OR
      (import_method = 'manual' AND crawl_job_id IS NULL
        AND schema_version = 'miyun-data-card/v1')
    );
