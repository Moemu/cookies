ALTER TABLE platform_skill_runs
  ADD COLUMN generation_mode VARCHAR(24) NULL AFTER model_version,
  ADD COLUMN model_alias VARCHAR(255) NULL AFTER generation_mode,
  ADD COLUMN route_revision_id VARCHAR(96) NULL AFTER model_alias,
  ADD COLUMN response_mode VARCHAR(24) NULL AFTER route_revision_id,
  ADD COLUMN prompt_version VARCHAR(96) NULL AFTER response_mode,
  ADD COLUMN skill_versions JSON NULL AFTER prompt_version,
  ADD COLUMN latency_ms BIGINT NULL AFTER total_tokens,
  ADD COLUMN validation_attempts INT NOT NULL DEFAULT 0 AFTER latency_ms,
  ADD COLUMN quality_report JSON NULL AFTER validation_attempts;
