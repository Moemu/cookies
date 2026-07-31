CREATE TABLE strategy_creative_handoffs (
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  package_id VARCHAR(96) NOT NULL,
  package_version BIGINT NOT NULL,
  contract_version VARCHAR(64) NOT NULL,
  snapshot JSON NOT NULL,
  content_hash VARCHAR(71) NOT NULL,
  published_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, project_id, package_id, package_version),
  KEY idx_strategy_creative_handoff_published (organization_id, project_id, published_at)
);
