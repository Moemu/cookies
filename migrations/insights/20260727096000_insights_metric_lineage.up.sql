ALTER TABLE insight_reports
  ADD COLUMN metric_snapshot_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' AFTER evidence_summary,
  ADD COLUMN creative_package_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' AFTER metric_snapshot_id,
  ADD COLUMN is_simulated BOOLEAN NOT NULL DEFAULT FALSE AFTER creative_package_id,
  ADD COLUMN dataset_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' AFTER is_simulated,
  ADD KEY idx_insight_reports_metric (organization_id, metric_snapshot_id);

ALTER TABLE insight_experiences
  ADD COLUMN source_metric_snapshot_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' AFTER source_evidence_id;
