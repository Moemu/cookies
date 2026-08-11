ALTER TABLE strategy_artifact_proposals
  ADD COLUMN proposal_fingerprint VARCHAR(80) NULL AFTER source_research_run_id,
  ADD UNIQUE KEY uq_strategy_research_proposal_fingerprint
    (organization_id, project_id, proposal_fingerprint);
