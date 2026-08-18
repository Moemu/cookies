ALTER TABLE products
  DROP INDEX uq_products_org_ocean_engine_id,
  DROP COLUMN ocean_engine_product_id;
