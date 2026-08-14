ALTER TABLE delivery_controlled_change_sets
  DROP CHECK chk_delivery_controlled_change_set_action,
  ADD CONSTRAINT chk_delivery_controlled_change_set_action CHECK (
    action IN (
      'create_project_and_promotions',
      'create_promotions_in_existing_project',
      'update_promotion_budget',
      'update_promotion_materials'
    )
  );

ALTER TABLE delivery_remote_write_approvals
  DROP CHECK chk_delivery_remote_write_approval_authority,
  ADD CONSTRAINT chk_delivery_remote_write_approval_authority CHECK (
    action IN (
      'create_project_and_promotions',
      'create_promotions_in_existing_project',
      'update_promotion_budget',
      'update_promotion_materials'
    )
    AND scope = 'controlled_remote_write'
  );

ALTER TABLE delivery_platform_entity_mappings
  ADD COLUMN current_state_action VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER platform_status,
  ADD COLUMN current_state_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER platform_status,
  ADD COLUMN updated_at DATETIME(6) NULL AFTER created_at;

UPDATE delivery_platform_entity_mappings
SET updated_at = created_at
WHERE updated_at IS NULL;

ALTER TABLE delivery_platform_entity_mappings
  MODIFY COLUMN updated_at DATETIME(6) NOT NULL,
  ADD CONSTRAINT chk_delivery_platform_mapping_state_hash CHECK (
    (current_state_action IS NULL AND current_state_hash IS NULL)
    OR (
      current_state_action IN ('update_promotion_budget', 'update_promotion_materials')
      AND current_state_hash REGEXP '^[0-9a-f]{64}$'
    )
  );

CREATE TABLE delivery_platform_entity_mapping_revisions (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  mapping_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  mapping_version BIGINT NOT NULL,
  action VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  business_execution_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  computer_use_run_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  platform_object_id VARCHAR(160) CHARACTER SET ascii COLLATE ascii_bin NULL,
  platform_status VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  previous_state_action VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  previous_state_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  current_state_action VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  current_state_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  result_evidence_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  list_evidence_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, project_id, mapping_id, mapping_version),
  KEY idx_delivery_platform_mapping_revision_execution (organization_id, project_id, business_execution_id),
  CONSTRAINT fk_delivery_platform_mapping_revision_mapping FOREIGN KEY (organization_id, project_id, mapping_id) REFERENCES delivery_platform_entity_mappings(organization_id, project_id, id),
  CONSTRAINT fk_delivery_platform_mapping_revision_execution FOREIGN KEY (organization_id, project_id, business_execution_id) REFERENCES delivery_controlled_executions(organization_id, project_id, id),
  CONSTRAINT chk_delivery_platform_mapping_revision_version CHECK (mapping_version > 0),
  CONSTRAINT chk_delivery_platform_mapping_revision_action CHECK (
    action IN (
      'create_project_and_promotions',
      'create_promotions_in_existing_project',
      'update_promotion_budget',
      'update_promotion_materials'
    )
  ),
  CONSTRAINT chk_delivery_platform_mapping_revision_state_hash CHECK (
    ((previous_state_action IS NULL AND previous_state_hash IS NULL) OR (previous_state_action IN ('update_promotion_budget', 'update_promotion_materials') AND previous_state_hash REGEXP '^[0-9a-f]{64}$'))
    AND ((current_state_action IS NULL AND current_state_hash IS NULL) OR (current_state_action IN ('update_promotion_budget', 'update_promotion_materials') AND current_state_hash REGEXP '^[0-9a-f]{64}$'))
  )
);

INSERT INTO delivery_platform_entity_mapping_revisions (
  organization_id,
  project_id,
  mapping_id,
  mapping_version,
  action,
  business_execution_id,
  computer_use_run_id,
  platform_object_id,
  platform_status,
  current_state_action,
  current_state_hash,
  result_evidence_id,
  list_evidence_id,
  created_at
)
SELECT
  m.organization_id,
  m.project_id,
  m.id,
  m.version,
  c.action,
  m.business_execution_id,
  m.computer_use_run_id,
  m.platform_object_id,
  m.platform_status,
  m.current_state_action,
  m.current_state_hash,
  m.result_evidence_id,
  m.list_evidence_id,
  m.updated_at
FROM delivery_platform_entity_mappings m
JOIN delivery_controlled_executions e
  ON e.organization_id = m.organization_id
  AND e.project_id = m.project_id
  AND e.id = m.business_execution_id
JOIN delivery_controlled_change_sets c
  ON c.organization_id = e.organization_id
  AND c.project_id = e.project_id
  AND c.id = e.controlled_change_set_id;
