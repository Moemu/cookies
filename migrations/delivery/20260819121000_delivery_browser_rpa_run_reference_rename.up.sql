-- Rename the delivery-side ComputerUseRun reference columns to the
-- browser-rpa namespace. CHANGE COLUMN renames only: row values, canonical
-- hashes and evidence references are untouched, and index definitions follow
-- the renamed columns automatically.
ALTER TABLE delivery_controlled_executions
  CHANGE COLUMN computer_use_run_id browser_rpa_run_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL;

ALTER TABLE delivery_executions
  CHANGE COLUMN computer_use_run_id browser_rpa_run_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL;

ALTER TABLE delivery_platform_entity_mappings
  CHANGE COLUMN computer_use_run_id browser_rpa_run_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL;

ALTER TABLE delivery_platform_entity_mapping_revisions
  CHANGE COLUMN computer_use_run_id browser_rpa_run_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL;
