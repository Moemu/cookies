ALTER TABLE asset_versions
  DROP INDEX uq_asset_versions_org_ocean_engine_id,
  DROP COLUMN ocean_engine_material_id;
