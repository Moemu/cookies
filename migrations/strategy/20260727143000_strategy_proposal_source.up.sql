ALTER TABLE strategy_proposals
  ADD COLUMN source_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER project_id,
  ADD COLUMN source_object_uri VARCHAR(1024) NULL AFTER source_type,
  ADD KEY idx_strategy_proposals_source (source_type, source_object_uri(191));
