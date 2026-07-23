ALTER TABLE provider_job_output_handles
  MODIFY COLUMN contents LONGBLOB NULL,
  ADD COLUMN storage_provider VARCHAR(32) NULL AFTER sha256,
  ADD COLUMN storage_bucket VARCHAR(255) NULL AFTER storage_provider,
  ADD COLUMN storage_key VARCHAR(1024) NULL AFTER storage_bucket,
  ADD COLUMN storage_version_id VARCHAR(255) NULL AFTER storage_key,
  ADD CONSTRAINT chk_provider_output_handle_storage CHECK (
    (contents IS NOT NULL AND storage_provider IS NULL AND storage_bucket IS NULL AND storage_key IS NULL)
    OR
    (contents IS NULL AND storage_provider IS NOT NULL AND storage_bucket IS NOT NULL AND storage_key IS NOT NULL)
  );
