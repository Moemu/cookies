ALTER TABLE platform_knowledge_documents
  ADD COLUMN parse_strategy VARCHAR(24) NOT NULL DEFAULT 'text_native' AFTER status,
  ADD COLUMN parse_phase VARCHAR(24) NOT NULL DEFAULT 'queued' AFTER parse_strategy,
  ADD COLUMN parse_progress TINYINT UNSIGNED NULL AFTER parse_phase,
  ADD COLUMN progress_kind VARCHAR(24) NOT NULL DEFAULT 'milestone' AFTER parse_progress,
  ADD COLUMN processed_pages INT NULL AFTER progress_kind,
  ADD COLUMN total_pages INT NULL AFTER processed_pages,
  ADD COLUMN quality_score DECIMAL(5,4) NULL AFTER total_pages,
  ADD COLUMN quality_tier VARCHAR(16) NOT NULL DEFAULT 'unknown' AFTER quality_score,
  ADD COLUMN fallback_reason VARCHAR(500) NOT NULL DEFAULT '' AFTER quality_tier,
  ADD COLUMN preview_status VARCHAR(24) NOT NULL DEFAULT 'unavailable' AFTER fallback_reason,
  ADD COLUMN page_quality_summary JSON NULL AFTER preview_status,
  ADD COLUMN heartbeat_at DATETIME(6) NULL AFTER page_quality_summary,
  ADD INDEX idx_knowledge_document_parse_activity
    (organization_id, project_id, parse_phase, heartbeat_at, updated_at);

UPDATE platform_knowledge_documents
SET parse_strategy = CASE
      WHEN parser_code = 'tika' OR mime_type IN (
        'application/pdf',
        'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
        'application/vnd.openxmlformats-officedocument.presentationml.presentation'
      ) THEN 'tika_text'
      ELSE 'text_native'
    END,
    parse_phase = CASE status
      WHEN 'parse_queued' THEN 'queued'
      WHEN 'parsing' THEN 'extracting'
      WHEN 'ready' THEN 'ready'
      ELSE 'failed'
    END,
    parse_progress = CASE status
      WHEN 'parse_queued' THEN 0
      WHEN 'parsing' THEN 35
      WHEN 'ready' THEN 100
      ELSE NULL
    END,
    progress_kind = 'milestone',
    quality_tier = 'unknown',
    preview_status = CASE status
      WHEN 'parse_queued' THEN 'building'
      WHEN 'parsing' THEN 'building'
      WHEN 'ready' THEN 'ready'
      ELSE 'failed'
    END,
    heartbeat_at = updated_at;
