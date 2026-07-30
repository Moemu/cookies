ALTER TABLE strategy_tasks
  ADD KEY idx_strategy_task_brief
    (organization_id, project_id, brief_id);

ALTER TABLE strategy_brief_drafts
  ADD KEY idx_strategy_brief_center
    (organization_id, project_id, status, updated_at);

ALTER TABLE strategy_drafts
  ADD KEY idx_strategy_draft_center
    (organization_id, project_id, status, archived_at, updated_at);

CREATE TABLE strategy_evidence_references (
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  evidence_type VARCHAR(32) NOT NULL,
  evidence_id VARCHAR(96) NOT NULL,
  target_type VARCHAR(32) NOT NULL,
  target_id VARCHAR(96) NOT NULL,
  target_version BIGINT NOT NULL,
  field_path VARCHAR(128) NOT NULL,
  content_hash VARCHAR(71) NOT NULL DEFAULT '',
  created_by VARCHAR(96) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (
    organization_id, project_id, evidence_id,
    target_type, target_id, target_version, field_path
  ),
  KEY idx_strategy_evidence_target (
    organization_id, project_id, target_type, target_id, target_version
  ),
  KEY idx_strategy_evidence_source (
    organization_id, project_id, evidence_type, evidence_id, created_at
  )
);

INSERT IGNORE INTO strategy_evidence_references
  (organization_id, project_id, evidence_type, evidence_id, target_type,
   target_id, target_version, field_path, content_hash, created_by, created_at)
SELECT
  draft.organization_id,
  draft.project_id,
  CASE
    WHEN artifact.id IS NOT NULL THEN 'research_artifact'
    WHEN document.id IS NOT NULL THEN 'knowledge_document'
    ELSE 'external_reference'
  END,
  reference_id.value,
  'brief_draft',
  draft.id,
  draft.version,
  'reference_ids',
  COALESCE(artifact.content_hash, document.text_sha256, ''),
  draft.updated_by,
  draft.updated_at
FROM strategy_brief_drafts AS draft
JOIN JSON_TABLE(
  draft.document,
  '$.reference_ids[*]' COLUMNS(value VARCHAR(96) PATH '$')
) AS reference_id
LEFT JOIN platform_research_artifacts AS artifact
  ON artifact.organization_id = draft.organization_id
  AND artifact.project_id = draft.project_id
  AND BINARY artifact.id = BINARY reference_id.value
LEFT JOIN platform_knowledge_documents AS document
  ON document.organization_id = draft.organization_id
  AND document.project_id = draft.project_id
  AND BINARY document.id = BINARY reference_id.value;

INSERT IGNORE INTO strategy_evidence_references
  (organization_id, project_id, evidence_type, evidence_id, target_type,
   target_id, target_version, field_path, content_hash, created_by, created_at)
SELECT
  version.organization_id,
  version.project_id,
  CASE
    WHEN artifact.id IS NOT NULL THEN 'research_artifact'
    WHEN document.id IS NOT NULL THEN 'knowledge_document'
    ELSE 'external_reference'
  END,
  reference_id.value,
  'brief_version',
  version.brief_id,
  version.version,
  'reference_ids',
  COALESCE(artifact.content_hash, document.text_sha256, ''),
  version.confirmed_by,
  version.confirmed_at
FROM strategy_brief_versions AS version
JOIN JSON_TABLE(
  version.snapshot,
  '$.document.reference_ids[*]' COLUMNS(value VARCHAR(96) PATH '$')
) AS reference_id
LEFT JOIN platform_research_artifacts AS artifact
  ON artifact.organization_id = version.organization_id
  AND artifact.project_id = version.project_id
  AND BINARY artifact.id = BINARY reference_id.value
LEFT JOIN platform_knowledge_documents AS document
  ON document.organization_id = version.organization_id
  AND document.project_id = version.project_id
  AND BINARY document.id = BINARY reference_id.value;

INSERT IGNORE INTO strategy_evidence_references
  (organization_id, project_id, evidence_type, evidence_id, target_type,
   target_id, target_version, field_path, content_hash, created_by, created_at)
SELECT
  revision.organization_id,
  revision.project_id,
  CASE
    WHEN artifact.id IS NOT NULL THEN 'research_artifact'
    WHEN document.id IS NOT NULL THEN 'knowledge_document'
    ELSE 'external_reference'
  END,
  reference_id.value,
  'strategy_revision',
  revision.strategy_id,
  revision.revision,
  'evidence_refs',
  COALESCE(artifact.content_hash, document.text_sha256, ''),
  revision.created_by,
  revision.created_at
FROM strategy_draft_revisions AS revision
JOIN JSON_TABLE(
  revision.document,
  '$.evidence_refs[*]' COLUMNS(value VARCHAR(96) PATH '$')
) AS reference_id
LEFT JOIN platform_research_artifacts AS artifact
  ON artifact.organization_id = revision.organization_id
  AND artifact.project_id = revision.project_id
  AND BINARY artifact.id = BINARY reference_id.value
LEFT JOIN platform_knowledge_documents AS document
  ON document.organization_id = revision.organization_id
  AND document.project_id = revision.project_id
  AND BINARY document.id = BINARY reference_id.value;
