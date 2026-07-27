ALTER TABLE asset_versions
  ADD COLUMN render_job_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER audio_codec,
  DROP CHECK chk_asset_versions_source,
  ADD CONSTRAINT chk_asset_versions_source
    CHECK (source_type IN ('upload', 'provider_generated', 'imported', 'captured', 'rendered')),
  ADD UNIQUE KEY uq_asset_versions_render_job (organization_id, render_job_id);
