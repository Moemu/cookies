ALTER TABLE platform_knowledge_documents
  ADD INDEX idx_knowledge_document_content_ready
    (organization_id, project_id, content_sha256, status, parsed_at);

UPDATE platform_knowledge_documents AS document
INNER JOIN platform_jobs AS job
  ON job.organization_id = document.organization_id
 AND job.project_id = document.project_id
 AND job.kind = 'knowledge.document.parse'
 AND JSON_UNQUOTE(JSON_EXTRACT(job.payload, '$.document_id')) = document.id
SET document.status = 'parse_failed',
    document.parse_error_code = COALESCE(job.error_code, 'DOCUMENT_PARSE_JOB_FAILED'),
    document.parse_error_message = COALESCE(job.error_message, 'Document parse job did not complete'),
    document.updated_at = job.updated_at
WHERE document.status IN ('parse_queued', 'parsing')
  AND job.status IN ('failed', 'cancelled');
