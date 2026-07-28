ALTER TABLE strategy_creative_handoffs
  ADD CONSTRAINT fk_strategy_creative_handoff_package
  FOREIGN KEY (organization_id, project_id, package_id, package_version)
  REFERENCES strategy_package_versions (organization_id, project_id, package_id, version);
