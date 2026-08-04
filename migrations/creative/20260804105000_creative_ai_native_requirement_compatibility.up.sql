-- Forward-only compatibility repair for databases that applied the original
-- Phase A migration before workspace lineage and revision metadata were added.
-- New databases already contain these fields, so every DDL step is conditional.

SET @ddl = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'creative_ai_native_requirement_workspaces' AND column_name = 'creative_intake_id') = 0,
    'ALTER TABLE creative_ai_native_requirement_workspaces ADD COLUMN creative_intake_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER workspace_id',
    'SELECT 1'
);
PREPARE migration_stmt FROM @ddl; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @ddl = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'creative_ai_native_requirement_workspaces' AND column_name = 'creative_task_id') = 0,
    'ALTER TABLE creative_ai_native_requirement_workspaces ADD COLUMN creative_task_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER creative_intake_id',
    'SELECT 1'
);
PREPARE migration_stmt FROM @ddl; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @ddl = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'creative_ai_native_requirement_workspaces' AND column_name = 'current_stage') = 0,
    'ALTER TABLE creative_ai_native_requirement_workspaces ADD COLUMN current_stage VARCHAR(32) NULL AFTER status',
    'SELECT 1'
);
PREPARE migration_stmt FROM @ddl; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @ddl = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'creative_ai_native_requirement_workspaces' AND column_name = 'workspace_version') = 0,
    'ALTER TABLE creative_ai_native_requirement_workspaces ADD COLUMN workspace_version BIGINT NULL AFTER current_stage',
    'SELECT 1'
);
PREPARE migration_stmt FROM @ddl; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @ddl = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'creative_ai_native_requirement_workspaces' AND column_name = 'active_operation_id') = 0,
    'ALTER TABLE creative_ai_native_requirement_workspaces ADD COLUMN active_operation_id VARCHAR(96) NULL AFTER workspace_version',
    'SELECT 1'
);
PREPARE migration_stmt FROM @ddl; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @ddl = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'creative_ai_native_requirement_workspaces' AND column_name = 'active_operation_version') = 0,
    'ALTER TABLE creative_ai_native_requirement_workspaces ADD COLUMN active_operation_version BIGINT NULL AFTER active_operation_id',
    'SELECT 1'
);
PREPARE migration_stmt FROM @ddl; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

UPDATE creative_ai_native_requirement_workspaces
SET creative_intake_id = COALESCE(creative_intake_id, CONCAT('creativeintake_legacy_', workspace_id)),
    creative_task_id = COALESCE(creative_task_id, CONCAT('creativetask_legacy_', workspace_id)),
    current_stage = COALESCE(current_stage, 'requirement'),
    workspace_version = COALESCE(workspace_version, 1);

ALTER TABLE creative_ai_native_requirement_workspaces
    MODIFY creative_intake_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    MODIFY creative_task_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    MODIFY current_stage VARCHAR(32) NOT NULL,
    MODIFY workspace_version BIGINT NOT NULL;

SET @ddl = IF(
    (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'creative_ai_native_requirement_workspaces' AND index_name = 'uq_ai_native_workspace_task') = 0,
    'ALTER TABLE creative_ai_native_requirement_workspaces ADD UNIQUE KEY uq_ai_native_workspace_task (organization_id, creative_task_id)',
    'SELECT 1'
);
PREPARE migration_stmt FROM @ddl; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @ddl = IF(
    (SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema = DATABASE() AND table_name = 'creative_ai_native_requirement_workspaces' AND constraint_name = 'chk_ai_native_workspace_version') = 0,
    'ALTER TABLE creative_ai_native_requirement_workspaces ADD CONSTRAINT chk_ai_native_workspace_version CHECK (workspace_version > 0)',
    'SELECT 1'
);
PREPARE migration_stmt FROM @ddl; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @ddl = IF(
    (SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema = DATABASE() AND table_name = 'creative_ai_native_requirement_workspaces' AND constraint_name = 'chk_ai_native_active_operation') = 0,
    'ALTER TABLE creative_ai_native_requirement_workspaces ADD CONSTRAINT chk_ai_native_active_operation CHECK ((active_operation_id IS NULL AND active_operation_version IS NULL) OR (active_operation_id IS NOT NULL AND active_operation_version > 0))',
    'SELECT 1'
);
PREPARE migration_stmt FROM @ddl; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @ddl = IF(
    (SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema = DATABASE() AND table_name = 'creative_ai_native_requirement_workspaces' AND constraint_name = 'chk_ai_native_workspace_stage') = 0,
    'ALTER TABLE creative_ai_native_requirement_workspaces ADD CONSTRAINT chk_ai_native_workspace_stage CHECK (current_stage IN (''requirement''))',
    'SELECT 1'
);
PREPARE migration_stmt FROM @ddl; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @ddl = IF(
    (SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema = DATABASE() AND table_name = 'creative_ai_native_requirement_workspaces' AND constraint_name = 'chk_ai_native_workspace_status') = 0,
    'ALTER TABLE creative_ai_native_requirement_workspaces ADD CONSTRAINT chk_ai_native_workspace_status CHECK (status IN (''draft'', ''confirmed''))',
    'SELECT 1'
);
PREPARE migration_stmt FROM @ddl; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @ddl = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'creative_ai_native_requirement_revisions' AND column_name = 'status') = 0,
    'ALTER TABLE creative_ai_native_requirement_revisions ADD COLUMN status VARCHAR(32) NULL AFTER revision',
    'SELECT 1'
);
PREPARE migration_stmt FROM @ddl; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @ddl = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'creative_ai_native_requirement_revisions' AND column_name = 'content_hash') = 0,
    'ALTER TABLE creative_ai_native_requirement_revisions ADD COLUMN content_hash CHAR(64) NULL AFTER content_payload',
    'SELECT 1'
);
PREPARE migration_stmt FROM @ddl; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @ddl = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'creative_ai_native_requirement_revisions' AND column_name = 'based_on_revision') = 0,
    'ALTER TABLE creative_ai_native_requirement_revisions ADD COLUMN based_on_revision BIGINT NULL AFTER content_hash',
    'SELECT 1'
);
PREPARE migration_stmt FROM @ddl; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @ddl = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'creative_ai_native_requirement_revisions' AND column_name = 'confirmed_by') = 0,
    'ALTER TABLE creative_ai_native_requirement_revisions ADD COLUMN confirmed_by VARCHAR(96) NULL AFTER created_by',
    'SELECT 1'
);
PREPARE migration_stmt FROM @ddl; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @ddl = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'creative_ai_native_requirement_revisions' AND column_name = 'confirmed_at') = 0,
    'ALTER TABLE creative_ai_native_requirement_revisions ADD COLUMN confirmed_at DATETIME(6) NULL AFTER confirmed_by',
    'SELECT 1'
);
PREPARE migration_stmt FROM @ddl; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @ddl = IF(
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'creative_ai_native_requirement_revisions' AND column_name = 'superseded_at') = 0,
    'ALTER TABLE creative_ai_native_requirement_revisions ADD COLUMN superseded_at DATETIME(6) NULL AFTER confirmed_at',
    'SELECT 1'
);
PREPARE migration_stmt FROM @ddl; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

UPDATE creative_ai_native_requirement_revisions r
JOIN creative_ai_native_requirement_workspaces w
  ON w.organization_id = r.organization_id
 AND w.project_id = r.project_id
 AND w.workspace_id = r.workspace_id
SET r.status = COALESCE(r.status, IF(w.confirmed_revision = r.revision, 'confirmed', 'draft')),
    r.content_hash = COALESCE(r.content_hash, SHA2(CAST(r.content_payload AS CHAR), 256)),
    r.confirmed_by = COALESCE(r.confirmed_by, IF(w.confirmed_revision = r.revision, w.confirmed_by, NULL)),
    r.confirmed_at = COALESCE(r.confirmed_at, IF(w.confirmed_revision = r.revision, w.confirmed_at, NULL));

ALTER TABLE creative_ai_native_requirement_revisions
    MODIFY status VARCHAR(32) NOT NULL,
    MODIFY content_hash CHAR(64) NOT NULL;

SET @ddl = IF(
    (SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema = DATABASE() AND table_name = 'creative_ai_native_requirement_revisions' AND constraint_name = 'chk_ai_native_requirement_revision') = 0,
    'ALTER TABLE creative_ai_native_requirement_revisions ADD CONSTRAINT chk_ai_native_requirement_revision CHECK (revision > 0)',
    'SELECT 1'
);
PREPARE migration_stmt FROM @ddl; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;

SET @ddl = IF(
    (SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema = DATABASE() AND table_name = 'creative_ai_native_requirement_revisions' AND constraint_name = 'chk_ai_native_requirement_status') = 0,
    'ALTER TABLE creative_ai_native_requirement_revisions ADD CONSTRAINT chk_ai_native_requirement_status CHECK (status IN (''draft'', ''confirmed'', ''superseded''))',
    'SELECT 1'
);
PREPARE migration_stmt FROM @ddl; EXECUTE migration_stmt; DEALLOCATE PREPARE migration_stmt;
