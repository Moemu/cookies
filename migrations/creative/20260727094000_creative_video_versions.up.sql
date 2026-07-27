ALTER TABLE creative_versions
  ADD COLUMN creative_format VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'image_text' AFTER draft_version,
  ADD COLUMN video_snapshot_payload JSON NULL AFTER snapshot_payload,
  ADD CONSTRAINT chk_creative_versions_format CHECK (creative_format IN ('image_text', 'video'));

ALTER TABLE creative_packages
  ADD COLUMN creative_format VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'image_text' AFTER creative_version_id,
  ADD COLUMN video_snapshot_payload JSON NULL AFTER snapshot_payload,
  ADD CONSTRAINT chk_creative_packages_format CHECK (creative_format IN ('image_text', 'video'));
