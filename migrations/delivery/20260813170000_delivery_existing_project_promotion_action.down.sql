ALTER TABLE delivery_remote_write_approvals
  DROP CHECK chk_delivery_remote_write_approval_authority,
  ADD CONSTRAINT chk_delivery_remote_write_approval_authority CHECK (
    action = 'create_project_and_promotions'
    AND scope = 'controlled_remote_write'
  );

ALTER TABLE delivery_controlled_change_sets
  DROP CHECK chk_delivery_controlled_change_set_action,
  ADD CONSTRAINT chk_delivery_controlled_change_set_action CHECK (
    action = 'create_project_and_promotions'
  );
