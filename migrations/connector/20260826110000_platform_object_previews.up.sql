ALTER TABLE connector_platform_objects
  ADD COLUMN preview_kind VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' AFTER display_name,
  ADD COLUMN preview_url_ciphertext VARBINARY(16384) NULL AFTER preview_kind,
  ADD COLUMN preview_key_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER preview_url_ciphertext,
  ADD COLUMN preview_expires_at DATETIME(6) NULL AFTER preview_key_version,
  ADD COLUMN preview_observed_at DATETIME(6) NULL AFTER preview_expires_at,
  ADD CONSTRAINT chk_connector_platform_object_preview_kind CHECK (preview_kind IN ('', 'image', 'video_poster', 'landing_page')),
  ADD CONSTRAINT chk_connector_platform_object_preview_pair CHECK ((preview_url_ciphertext IS NULL AND preview_key_version IS NULL AND preview_observed_at IS NULL) OR (preview_url_ciphertext IS NOT NULL AND preview_key_version IS NOT NULL AND preview_observed_at IS NOT NULL));
