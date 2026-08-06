CREATE TABLE creative_edit_tasks (
    organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    edit_task_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    display_name VARCHAR(120) NOT NULL,
    status VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    entry_source VARCHAR(48) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    source_creative_task_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
    current_timeline_version BIGINT NOT NULL,
    created_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (organization_id, project_id, edit_task_id),
    UNIQUE KEY uq_creative_edit_task_source (
        organization_id, project_id, entry_source, source_creative_task_id
    ),
    KEY idx_creative_edit_tasks_project_updated (
        organization_id, project_id, updated_at, edit_task_id
    ),
    CONSTRAINT chk_creative_edit_task_status CHECK (status IN ('draft')),
    CONSTRAINT chk_creative_edit_task_entry_source CHECK (
        entry_source IN ('manual', 'short_drama_preroll_v2', 'creative_version')
    ),
    CONSTRAINT chk_creative_edit_task_source_link CHECK (
        (entry_source = 'manual' AND source_creative_task_id IS NULL)
        OR (entry_source IN ('short_drama_preroll_v2', 'creative_version') AND source_creative_task_id IS NOT NULL)
    ),
    CONSTRAINT chk_creative_edit_task_current_version CHECK (current_timeline_version > 0)
);

CREATE TABLE creative_edit_timeline_versions (
    organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    edit_task_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    version BIGINT NOT NULL,
    content_payload JSON NOT NULL,
    content_hash CHAR(71) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    created_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (organization_id, project_id, edit_task_id, version),
    CONSTRAINT fk_creative_edit_timeline_task
        FOREIGN KEY (organization_id, project_id, edit_task_id)
        REFERENCES creative_edit_tasks (organization_id, project_id, edit_task_id),
    CONSTRAINT chk_creative_edit_timeline_version CHECK (version > 0)
);
