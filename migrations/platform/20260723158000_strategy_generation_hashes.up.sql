ALTER TABLE platform_skill_runs
  ADD COLUMN skill_snapshot_hashes JSON NULL AFTER skill_versions,
  ADD COLUMN generation_context_hash CHAR(64) NULL AFTER skill_snapshot_hashes,
  ADD COLUMN output_hash CHAR(64) NULL AFTER output_snapshot;
