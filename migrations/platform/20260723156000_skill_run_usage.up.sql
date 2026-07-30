ALTER TABLE platform_skill_runs
  ADD COLUMN input_tokens BIGINT NULL AFTER model_version,
  ADD COLUMN output_tokens BIGINT NULL AFTER input_tokens,
  ADD COLUMN total_tokens BIGINT NULL AFTER output_tokens;
