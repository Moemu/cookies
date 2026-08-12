ALTER TABLE insight_miyun_handoff_returns
  DROP CONSTRAINT chk_insight_miyun_return_association,
  DROP COLUMN container_filename,
  DROP COLUMN association_source,
  DROP COLUMN source_material_id,
  DROP COLUMN crawl_job_id;
