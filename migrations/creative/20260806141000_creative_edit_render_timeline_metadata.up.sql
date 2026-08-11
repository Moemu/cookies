ALTER TABLE creative_edit_render_jobs
    ADD COLUMN timeline_created_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER timeline_hash,
    ADD COLUMN timeline_created_at DATETIME(6) NULL AFTER timeline_created_by;

UPDATE creative_edit_render_jobs AS render_job
JOIN creative_edit_timeline_versions AS timeline_version
  ON timeline_version.organization_id = render_job.organization_id
 AND timeline_version.project_id = render_job.project_id
 AND timeline_version.edit_task_id = render_job.edit_task_id
 AND timeline_version.version = render_job.timeline_version
SET render_job.timeline_created_by = timeline_version.created_by,
    render_job.timeline_created_at = timeline_version.created_at;

ALTER TABLE creative_edit_render_jobs
    MODIFY COLUMN timeline_created_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    MODIFY COLUMN timeline_created_at DATETIME(6) NOT NULL;
