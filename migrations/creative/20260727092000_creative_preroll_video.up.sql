ALTER TABLE creative_tasks
  ADD COLUMN video_purpose VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER channel,
  ADD COLUMN performance_mode VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER video_purpose,
  DROP CHECK chk_creative_tasks_format,
  DROP CHECK chk_creative_tasks_channel,
  DROP CHECK chk_creative_tasks_status,
  ADD CONSTRAINT chk_creative_tasks_format CHECK (creative_format IN ('image_text', 'video')),
  ADD CONSTRAINT chk_creative_tasks_channel CHECK (channel IN ('xiaohongshu', 'douyin', 'kuaishou')),
  ADD CONSTRAINT chk_creative_tasks_status CHECK (
    status IN ('draft', 'in_progress', 'generating', 'generated', 'rendering', 'ready_for_review', 'approved', 'delivered', 'archived')
  );

CREATE TABLE creative_video_drafts (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  task_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  revision BIGINT NOT NULL,
  content_payload JSON NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (organization_id, task_id, revision),
  CONSTRAINT fk_creative_video_drafts_task
    FOREIGN KEY (organization_id, task_id) REFERENCES creative_tasks(organization_id, id),
  CONSTRAINT chk_creative_video_drafts_revision CHECK (revision > 0)
);

ALTER TABLE creative_production_jobs
  DROP CHECK chk_creative_production_kind,
  ADD CONSTRAINT chk_creative_production_kind
  CHECK (
    job_kind = 'cover_image'
    OR job_kind = 'video_generate'
    OR job_kind REGEXP '^image_plan_([1-9]|1[0-2])_job_[A-Za-z0-9_-]+$'
  );
