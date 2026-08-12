ALTER TABLE insight_miyun_product_profiles
  ADD COLUMN material_types JSON NULL AFTER keywords;

UPDATE insight_miyun_product_profiles
SET material_types = JSON_ARRAY()
WHERE material_types IS NULL;

ALTER TABLE insight_miyun_product_profiles
  MODIFY COLUMN material_types JSON NOT NULL;
