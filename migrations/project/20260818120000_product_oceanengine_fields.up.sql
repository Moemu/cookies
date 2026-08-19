-- Align product catalog fields with OceanEngine product attributes.
-- Category splits the two broad OceanEngine product kinds: ordinary
-- products (product) and promotional activities (activity). The fine-grained
-- OceanEngine category tree (local services, recycling, finance, ...) is not
-- enumerable, so it is intentionally not modeled here.
ALTER TABLE products
  ADD COLUMN category VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'product' AFTER name,
  ADD COLUMN product_image VARCHAR(512) NULL AFTER activity_name,
  ADD COLUMN price_band VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER product_image,
  ADD COLUMN brand_type VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER brand_name,
  ADD COLUMN description TEXT NULL AFTER brand_type;
