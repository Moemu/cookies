ALTER TABLE strategy_artifact_proposals
  ADD COLUMN finding_ids JSON NULL AFTER source_message_id,
  ADD COLUMN source_research_run_id VARCHAR(96) NULL AFTER finding_ids,
  ADD COLUMN stale_reason VARCHAR(500) NOT NULL DEFAULT '' AFTER status,
  ADD COLUMN supersedes_proposal_id VARCHAR(96) NULL AFTER stale_reason,
  ADD KEY idx_strategy_research_proposals
    (organization_id, project_id, source_research_run_id, status, created_at);
