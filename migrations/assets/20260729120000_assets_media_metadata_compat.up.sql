-- Reconcile the Kanon media-probe metadata with databases created by the
-- earlier cookies-platform asset schema. MySQL does not support portable
-- ADD COLUMN IF NOT EXISTS syntax, so each column is guarded through
-- information_schema and a prepared DDL statement.
SET @ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'asset_versions' AND column_name = 'duration_seconds'),
  'SELECT 1',
  'ALTER TABLE asset_versions ADD COLUMN duration_seconds DOUBLE NULL'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'asset_versions' AND column_name = 'fps'),
  'SELECT 1',
  'ALTER TABLE asset_versions ADD COLUMN fps DOUBLE NULL'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'asset_versions' AND column_name = 'codec'),
  'SELECT 1',
  'ALTER TABLE asset_versions ADD COLUMN codec VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'asset_versions' AND column_name = 'bitrate_bps'),
  'SELECT 1',
  'ALTER TABLE asset_versions ADD COLUMN bitrate_bps BIGINT UNSIGNED NULL'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'asset_versions' AND column_name = 'audio_codec'),
  'SELECT 1',
  'ALTER TABLE asset_versions ADD COLUMN audio_codec VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'asset_versions' AND column_name = 'audio_channels'),
  'SELECT 1',
  'ALTER TABLE asset_versions ADD COLUMN audio_channels INT UNSIGNED NULL'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'asset_versions' AND column_name = 'audio_sample_rate'),
  'SELECT 1',
  'ALTER TABLE asset_versions ADD COLUMN audio_sample_rate INT UNSIGNED NULL'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'asset_versions' AND column_name = 'poster_frame_ref'),
  'SELECT 1',
  'ALTER TABLE asset_versions ADD COLUMN poster_frame_ref VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NULL'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'asset_versions' AND column_name = 'probe_status'),
  'SELECT 1',
  'ALTER TABLE asset_versions ADD COLUMN probe_status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT ''not_required'''
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'asset_versions' AND column_name = 'probe_error'),
  'SELECT 1',
  'ALTER TABLE asset_versions ADD COLUMN probe_error VARCHAR(1024) NULL'
);
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
