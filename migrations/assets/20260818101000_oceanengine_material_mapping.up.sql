ALTER TABLE asset_versions
  ADD COLUMN ocean_engine_material_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER provider_output_id,
  ADD UNIQUE KEY uq_asset_versions_org_ocean_engine_id (organization_id, ocean_engine_material_id);
