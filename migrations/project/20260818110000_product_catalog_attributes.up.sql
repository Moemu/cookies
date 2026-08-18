-- Product catalog attributes: activity context mirrors the delivery
-- marketing_product reference so the product object can be the source of
-- truth for OceanEngine launches. All columns are optional; the product
-- name is the only required business field.
ALTER TABLE products
  ADD COLUMN activity_type VARCHAR(64) NULL AFTER name,
  ADD COLUMN activity_name VARCHAR(128) NULL AFTER activity_type,
  ADD COLUMN brand_name VARCHAR(128) NULL AFTER activity_name;
