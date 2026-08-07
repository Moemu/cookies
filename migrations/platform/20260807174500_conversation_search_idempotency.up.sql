ALTER TABLE platform_research_runs
  ADD UNIQUE KEY uq_research_run_conversation_source
    (organization_id, project_id, purpose, source_type, source_id);
