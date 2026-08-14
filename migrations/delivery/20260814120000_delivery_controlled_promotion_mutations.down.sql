DROP TABLE delivery_platform_entity_mapping_revisions;

ALTER TABLE delivery_platform_entity_mappings
  DROP CHECK chk_delivery_platform_mapping_state_hash,
  DROP COLUMN current_state_action,
  DROP COLUMN current_state_hash,
  DROP COLUMN updated_at;

ALTER TABLE delivery_remote_write_approvals
  DROP CHECK chk_delivery_remote_write_approval_authority,
  ADD CONSTRAINT chk_delivery_remote_write_approval_authority CHECK (
    action IN ('create_project_and_promotions', 'create_promotions_in_existing_project')
    AND scope = 'controlled_remote_write'
  );

ALTER TABLE delivery_controlled_change_sets
  DROP CHECK chk_delivery_controlled_change_set_action,
  ADD CONSTRAINT chk_delivery_controlled_change_set_action CHECK (
    action IN ('create_project_and_promotions', 'create_promotions_in_existing_project')
  );
