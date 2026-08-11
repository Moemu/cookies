ALTER TABLE asset_relations
  DROP CHECK chk_asset_relations_type,
  ADD CONSTRAINT chk_asset_relations_type CHECK (relation_type IN ('generated_from', 'derived_from', 'returned_from'));
