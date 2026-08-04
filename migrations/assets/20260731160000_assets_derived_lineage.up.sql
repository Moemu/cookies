ALTER TABLE asset_versions
  ADD COLUMN derivation_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER render_job_id,
  DROP CHECK chk_asset_versions_source,
  ADD CONSTRAINT chk_asset_versions_source
    CHECK (source_type IN ('upload', 'provider_generated', 'imported', 'captured', 'rendered', 'derived')),
  ADD UNIQUE KEY uq_asset_versions_derivation (organization_id, derivation_id);

ALTER TABLE asset_relations
  DROP CHECK chk_asset_relations_type,
  ADD CONSTRAINT chk_asset_relations_type CHECK (relation_type IN ('generated_from', 'derived_from'));
