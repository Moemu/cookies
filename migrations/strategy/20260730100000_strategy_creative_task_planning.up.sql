CREATE TABLE strategy_creative_business_profiles (
  business_code VARCHAR(64) NOT NULL,
  generation BIGINT NOT NULL,
  version VARCHAR(32) NOT NULL,
  display_name VARCHAR(120) NOT NULL,
  summary VARCHAR(500) NOT NULL,
  lifecycle VARCHAR(24) NOT NULL,
  selectable BOOLEAN NOT NULL,
  display_order INT NOT NULL,
  profile JSON NOT NULL,
  content_hash VARCHAR(71) NOT NULL,
  skill_name VARCHAR(120) NOT NULL,
  skill_version VARCHAR(32) NOT NULL,
  skill_content_hash VARCHAR(71) NOT NULL,
  owner VARCHAR(120) NOT NULL,
  reviewed_by VARCHAR(120) NULL,
  reviewed_at DATETIME(6) NULL,
  published_at DATETIME(6) NOT NULL,
  PRIMARY KEY (business_code, generation),
  UNIQUE KEY uq_strategy_creative_business_version (business_code, version),
  UNIQUE KEY uq_strategy_creative_business_hash (business_code, content_hash),
  KEY idx_strategy_creative_business_current
    (business_code, generation, lifecycle, selectable),
  CONSTRAINT chk_strategy_creative_business_lifecycle
    CHECK (lifecycle IN ('draft', 'active', 'deprecated', 'retired'))
);

CREATE TABLE strategy_creative_task_plans (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  brief_id VARCHAR(96) NOT NULL,
  brief_version BIGINT NOT NULL,
  brief_content_hash VARCHAR(71) NOT NULL,
  source_strategy_id VARCHAR(96) NULL,
  source_strategy_revision BIGINT NULL,
  source_strategy_content_hash VARCHAR(71) NULL,
  status VARCHAR(24) NOT NULL,
  business_code VARCHAR(64) NOT NULL,
  business_generation BIGINT NOT NULL,
  business_version VARCHAR(32) NOT NULL,
  business_content_hash VARCHAR(71) NOT NULL,
  skill_name VARCHAR(120) NOT NULL,
  skill_version VARCHAR(32) NOT NULL,
  skill_content_hash VARCHAR(71) NOT NULL,
  selection_source VARCHAR(24) NOT NULL,
  recommendation_snapshot JSON NOT NULL,
  answers JSON NOT NULL,
  completeness JSON NOT NULL,
  current_revision BIGINT NOT NULL,
  current_strategy_version BIGINT NOT NULL DEFAULT 0,
  current_agent_task_id VARCHAR(96) NULL,
  version BIGINT NOT NULL,
  created_by VARCHAR(96) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_strategy_creative_plan_scope (organization_id, project_id, id),
  KEY idx_strategy_creative_plan_brief
    (organization_id, project_id, brief_id, brief_version, created_at),
  CONSTRAINT chk_strategy_creative_plan_status
    CHECK (status IN (
      'collecting', 'ready', 'generating',
      'generated', 'failed', 'superseded'
    )),
  CONSTRAINT chk_strategy_creative_selection_source
    CHECK (selection_source IN ('recommended', 'manual')),
  CONSTRAINT chk_strategy_creative_source_strategy
    CHECK (
      (source_strategy_id IS NULL AND source_strategy_revision IS NULL
        AND source_strategy_content_hash IS NULL)
      OR
      (source_strategy_id IS NOT NULL AND source_strategy_revision > 0
        AND source_strategy_content_hash IS NOT NULL)
    )
);

CREATE TABLE strategy_creative_task_plan_revisions (
  plan_id VARCHAR(96) NOT NULL,
  revision BIGINT NOT NULL,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  base_revision BIGINT NULL,
  snapshot JSON NOT NULL,
  content_hash VARCHAR(71) NOT NULL,
  change_reason VARCHAR(120) NOT NULL,
  created_by VARCHAR(96) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, project_id, plan_id, revision),
  UNIQUE KEY uq_strategy_creative_plan_revision_hash
    (organization_id, project_id, plan_id, content_hash)
);

CREATE TABLE strategy_creative_task_strategy_versions (
  plan_id VARCHAR(96) NOT NULL,
  version BIGINT NOT NULL,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  plan_revision BIGINT NOT NULL,
  contract_version VARCHAR(64) NOT NULL,
  document JSON NOT NULL,
  content_hash VARCHAR(71) NOT NULL,
  generation_context_hash VARCHAR(71) NOT NULL,
  agent_task_id VARCHAR(96) NOT NULL,
  skill_name VARCHAR(120) NOT NULL,
  skill_version VARCHAR(32) NOT NULL,
  skill_content_hash VARCHAR(71) NOT NULL,
  created_by VARCHAR(96) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, project_id, plan_id, version),
  UNIQUE KEY uq_strategy_creative_task_strategy_hash
    (organization_id, project_id, plan_id, content_hash),
  UNIQUE KEY uq_strategy_creative_task_strategy_agent
    (organization_id, project_id, agent_task_id)
);
