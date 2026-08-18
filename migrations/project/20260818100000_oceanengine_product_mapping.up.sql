ALTER TABLE products
  ADD COLUMN ocean_engine_product_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER status,
  ADD UNIQUE KEY uq_products_org_ocean_engine_id (organization_id, ocean_engine_product_id);
