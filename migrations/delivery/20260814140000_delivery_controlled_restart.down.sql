ALTER TABLE delivery_platform_entity_mapping_revisions
  DROP CHECK chk_delivery_platform_mapping_revision_action,
  DROP CHECK chk_delivery_platform_mapping_revision_state_hash,
  ADD CONSTRAINT chk_delivery_platform_mapping_revision_action CHECK (
    action IN (
      'create_project_and_promotions',
      'create_promotions_in_existing_project',
      'update_promotion_budget',
      'update_promotion_schedule',
      'update_promotion_materials',
      'pause_promotion'
    )
  ),
  ADD CONSTRAINT chk_delivery_platform_mapping_revision_state_hash CHECK (
    ((previous_state_action IS NULL AND previous_state_hash IS NULL) OR (previous_state_action IN ('update_promotion_budget', 'update_promotion_schedule', 'update_promotion_materials', 'pause_promotion') AND previous_state_hash REGEXP '^[0-9a-f]{64}$'))
    AND ((current_state_action IS NULL AND current_state_hash IS NULL) OR (current_state_action IN ('update_promotion_budget', 'update_promotion_schedule', 'update_promotion_materials', 'pause_promotion') AND current_state_hash REGEXP '^[0-9a-f]{64}$'))
  );

ALTER TABLE delivery_platform_entity_mappings
  DROP CHECK chk_delivery_platform_mapping_state_hash,
  ADD CONSTRAINT chk_delivery_platform_mapping_state_hash CHECK (
    (current_state_action IS NULL AND current_state_hash IS NULL)
    OR (
      current_state_action IN ('update_promotion_budget', 'update_promotion_schedule', 'update_promotion_materials', 'pause_promotion')
      AND current_state_hash REGEXP '^[0-9a-f]{64}$'
    )
  );

ALTER TABLE delivery_remote_write_approvals
  DROP CHECK chk_delivery_remote_write_approval_authority,
  ADD CONSTRAINT chk_delivery_remote_write_approval_authority CHECK (
    action IN (
      'create_project_and_promotions',
      'create_promotions_in_existing_project',
      'update_promotion_budget',
      'update_promotion_schedule',
      'update_promotion_materials',
      'pause_promotion'
    )
    AND scope = 'controlled_remote_write'
  );

ALTER TABLE delivery_controlled_change_sets
  DROP CHECK chk_delivery_controlled_change_set_action,
  ADD CONSTRAINT chk_delivery_controlled_change_set_action CHECK (
    action IN (
      'create_project_and_promotions',
      'create_promotions_in_existing_project',
      'update_promotion_budget',
      'update_promotion_schedule',
      'update_promotion_materials',
      'pause_promotion'
    )
  );
