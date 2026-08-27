ALTER TABLE connector_platform_objects
  DROP CONSTRAINT chk_connector_platform_object_preview_pair,
  DROP CONSTRAINT chk_connector_platform_object_preview_kind,
  DROP COLUMN preview_observed_at,
  DROP COLUMN preview_expires_at,
  DROP COLUMN preview_key_version,
  DROP COLUMN preview_url_ciphertext,
  DROP COLUMN preview_kind;
