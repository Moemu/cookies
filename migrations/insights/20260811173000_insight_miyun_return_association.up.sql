ALTER TABLE insight_miyun_handoff_returns
  ADD COLUMN crawl_job_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER product_profile_id,
  ADD COLUMN source_material_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER crawl_job_id,
  ADD COLUMN association_source VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'crawl_job' AFTER source_material_id,
  ADD COLUMN container_filename VARCHAR(512) NULL AFTER association_source,
  ADD CONSTRAINT chk_insight_miyun_return_association CHECK (association_source IN ('crawl_job','filename','manifest_xlsx'));

UPDATE insight_miyun_handoff_returns r
JOIN insight_miyun_handoffs h
  ON h.organization_id = r.organization_id
 AND h.project_id = r.project_id
 AND h.id = r.handoff_id
SET r.crawl_job_id = h.crawl_job_id
WHERE r.crawl_job_id IS NULL;
