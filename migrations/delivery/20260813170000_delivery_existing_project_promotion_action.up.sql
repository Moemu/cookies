ALTER TABLE delivery_controlled_change_sets
  DROP CHECK chk_delivery_controlled_change_set_action,
  ADD CONSTRAINT chk_delivery_controlled_change_set_action CHECK (
    action IN ('create_project_and_promotions', 'create_promotions_in_existing_project')
  );

ALTER TABLE delivery_remote_write_approvals
  DROP CHECK chk_delivery_remote_write_approval_authority,
  ADD CONSTRAINT chk_delivery_remote_write_approval_authority CHECK (
    action IN ('create_project_and_promotions', 'create_promotions_in_existing_project')
    AND scope = 'controlled_remote_write'
  );
