ALTER TABLE platform_knowledge_documents
  ADD COLUMN title VARCHAR(160) NULL AFTER project_id,
  ADD COLUMN source_uri VARCHAR(512) NULL AFTER title,
  ADD COLUMN source_type VARCHAR(32) NOT NULL DEFAULT 'docs' AFTER source_uri,
  ADD COLUMN chunk_count INT NOT NULL DEFAULT 1 AFTER source_type,
  ADD KEY idx_knowledge_document_source
    (organization_id, project_id, source_type, created_at);

UPDATE platform_knowledge_documents
SET title = TRIM(TRAILING '.md' FROM filename)
WHERE title IS NULL OR title = '';

ALTER TABLE platform_research_runs
  ADD COLUMN category VARCHAR(24) NOT NULL DEFAULT 'general' AFTER mode,
  ADD KEY idx_research_run_center
    (organization_id, project_id, category, status, updated_at);

ALTER TABLE platform_research_artifacts
  ADD COLUMN category VARCHAR(24) NOT NULL DEFAULT 'general' AFTER source_type,
  ADD KEY idx_research_artifact_center
    (organization_id, project_id, category, source_type, created_at);
