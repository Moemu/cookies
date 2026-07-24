ALTER TABLE creative_intakes
  ADD COLUMN strategy_package_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER request_hash,
  ADD COLUMN strategy_package_version BIGINT NULL AFTER strategy_package_id,
  ADD COLUMN strategy_package_content_hash CHAR(71) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER strategy_package_version,
  ADD UNIQUE KEY uq_creative_intakes_strategy_package (organization_id, project_id, source_type, strategy_package_id, strategy_package_version, strategy_package_content_hash);
