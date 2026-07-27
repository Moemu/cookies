ALTER TABLE asset_versions
  ADD COLUMN duration_ms BIGINT NULL AFTER height_pixels,
  ADD COLUMN frame_rate VARCHAR(32) NULL AFTER duration_ms,
  ADD COLUMN video_codec VARCHAR(64) NULL AFTER frame_rate,
  ADD COLUMN audio_codec VARCHAR(64) NULL AFTER video_codec;
