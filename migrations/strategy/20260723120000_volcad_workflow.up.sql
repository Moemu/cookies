CREATE TABLE IF NOT EXISTS strategy_proposals (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  input_json JSON NOT NULL,
  input_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  template_version VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'draft',
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_strategy_proposals_input (organization_id, project_id, input_hash, template_version),
  KEY idx_strategy_proposals_scope (organization_id, project_id, created_at),
  CONSTRAINT fk_strategy_proposals_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT chk_strategy_proposals_status CHECK (status IN ('draft', 'generated', 'approved'))
);

CREATE TABLE IF NOT EXISTS strategy_outputs (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  proposal_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  strategy_json JSON NOT NULL,
  model_alias VARCHAR(255) NOT NULL,
  model_version VARCHAR(255) NOT NULL,
  provider_code VARCHAR(64) NOT NULL,
  approved_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_strategy_outputs_scope_id (organization_id, project_id, id),
  KEY idx_strategy_outputs_proposal (organization_id, project_id, proposal_id, created_at),
  CONSTRAINT fk_strategy_outputs_proposal FOREIGN KEY (proposal_id) REFERENCES strategy_proposals(id)
);

CREATE TABLE IF NOT EXISTS creative_plans (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  strategy_output_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  media_type VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  variant INT NOT NULL,
  prompt TEXT NOT NULL,
  model_alias VARCHAR(255) NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_creative_plans_scope_id (organization_id, project_id, id),
  KEY idx_creative_plans_strategy (organization_id, project_id, strategy_output_id, created_at),
  CONSTRAINT fk_creative_plans_strategy FOREIGN KEY (strategy_output_id) REFERENCES strategy_outputs(id),
  CONSTRAINT chk_creative_plans_media_type CHECK (media_type IN ('image', 'video')),
  CONSTRAINT chk_creative_plans_variant CHECK (variant > 0)
);
