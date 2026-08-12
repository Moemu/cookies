-- M03 makes Miyun collection and authorized import durable and recoverable.
-- Signed media locators remain encrypted; stable source references stay
-- explicitly unknown until verified from a sanitized protocol fixture.

ALTER TABLE insight_miyun_connections
  ADD COLUMN cooldown_until DATETIME(6) NULL AFTER last_successful_request_at;

ALTER TABLE insight_miyun_crawl_jobs
  ADD COLUMN idempotency_key VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER query_snapshot,
  ADD COLUMN request_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER idempotency_key,
  ADD COLUMN runtime_job_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER request_hash,
  ADD UNIQUE KEY uq_insight_miyun_crawl_jobs_idempotency
    (organization_id, project_id, idempotency_key),
  ADD UNIQUE KEY uq_insight_miyun_crawl_jobs_runtime
    (organization_id, project_id, runtime_job_id);

UPDATE insight_miyun_crawl_jobs
SET idempotency_key = CONCAT('legacy_', id),
    request_hash = SHA2(CONCAT('legacy:', id), 256),
    runtime_job_id = id
WHERE idempotency_key IS NULL;

ALTER TABLE insight_miyun_crawl_jobs
  MODIFY COLUMN idempotency_key VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  MODIFY COLUMN request_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  MODIFY COLUMN runtime_job_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL;

ALTER TABLE insight_miyun_materials
  ADD COLUMN resource_url_ciphertext VARBINARY(4096) NULL AFTER resource_id,
  ADD COLUMN resource_url_key_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER resource_url_ciphertext,
  ADD COLUMN resource_expected_size BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER resource_url_key_version,
  ADD COLUMN source_ref_status VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'unknown' AFTER source_ref,
  ADD COLUMN decision_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER import_status,
  ADD COLUMN decision_at DATETIME(6) NULL AFTER decision_by,
  ADD COLUMN decision_note VARCHAR(1000) NOT NULL DEFAULT '' AFTER decision_at,
  ADD COLUMN last_import_error_kind VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER decision_note,
  ADD COLUMN last_import_error_code VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER last_import_error_kind,
  ADD CONSTRAINT chk_insight_miyun_materials_resource_cipher
    CHECK (
      (resource_url_ciphertext IS NULL AND resource_url_key_version IS NULL)
      OR (resource_url_ciphertext IS NOT NULL AND resource_url_key_version IS NOT NULL)
    ),
  ADD CONSTRAINT chk_insight_miyun_materials_source_ref_status
    CHECK (source_ref_status IN ('verified', 'unknown')),
  ADD CONSTRAINT chk_insight_miyun_materials_source_ref_verified
    CHECK (source_ref_status <> 'verified' OR source_ref <> ''),
  ADD CONSTRAINT chk_insight_miyun_materials_import_error
    CHECK (
      (last_import_error_kind IS NULL AND last_import_error_code IS NULL)
      OR
      (last_import_error_kind IS NOT NULL AND last_import_error_code IS NOT NULL)
    );

UPDATE insight_miyun_materials
SET source_ref_status = 'verified'
WHERE import_method = 'manual';

UPDATE insight_miyun_materials
SET decision_by = created_by,
    decision_at = created_at,
    decision_note = 'Legacy decision backfilled during Miyun authorized import migration.'
WHERE selection_status IN ('confirmed', 'rejected')
  AND (decision_by IS NULL OR decision_at IS NULL);

ALTER TABLE insight_miyun_materials
  ADD CONSTRAINT chk_insight_miyun_materials_decision
    CHECK (
      (selection_status = 'discovered' AND decision_by IS NULL AND decision_at IS NULL)
      OR
      (selection_status IN ('confirmed', 'rejected') AND decision_by IS NOT NULL AND decision_at IS NOT NULL)
    );

ALTER TABLE insight_miyun_material_snapshots
  ADD COLUMN source_page BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER crawl_job_id,
  ADD COLUMN related_creators_raw VARCHAR(64) NOT NULL DEFAULT 'unknown' AFTER related_creators,
  ADD COLUMN related_creators_known BOOLEAN NOT NULL DEFAULT FALSE AFTER related_creators_raw;

UPDATE insight_miyun_material_snapshots
SET source_page = 1
WHERE import_method = 'crawler' AND source_page = 0;

ALTER TABLE insight_miyun_material_snapshots
  ADD CONSTRAINT chk_insight_miyun_snapshot_source_page
    CHECK (
      (import_method = 'manual' AND source_page = 0)
      OR
      (import_method = 'crawler' AND source_page > 0)
    );
