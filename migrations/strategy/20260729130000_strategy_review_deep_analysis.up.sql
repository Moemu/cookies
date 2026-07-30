CREATE TABLE strategy_review_analyses (
  id VARCHAR(96) PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  review_id VARCHAR(96) NOT NULL,
  strategy_id VARCHAR(96) NOT NULL,
  candidate_revision BIGINT NOT NULL,
  candidate_content_hash VARCHAR(71) NOT NULL,
  agent_task_id VARCHAR(96) NOT NULL,
  status VARCHAR(32) NOT NULL,
  summary TEXT NULL,
  findings JSON NOT NULL,
  model_alias VARCHAR(160) NULL,
  model_version VARCHAR(160) NULL,
  route_revision_id VARCHAR(160) NULL,
  response_mode VARCHAR(32) NULL,
  api_mode VARCHAR(32) NULL,
  background BOOLEAN NOT NULL DEFAULT FALSE,
  usage_json JSON NULL,
  latency_ms BIGINT NULL,
  created_by VARCHAR(96) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_strategy_review_analysis_task (organization_id, agent_task_id),
  KEY idx_strategy_review_analysis_latest (organization_id, project_id, review_id, created_at),
  CONSTRAINT fk_strategy_review_analysis_review
    FOREIGN KEY (organization_id, project_id, review_id)
    REFERENCES strategy_reviews(organization_id, project_id, id),
  CONSTRAINT fk_strategy_review_analysis_task
    FOREIGN KEY (organization_id, project_id, agent_task_id)
    REFERENCES platform_agent_tasks(organization_id, project_id, id),
  CHECK (status IN ('pending', 'succeeded', 'failed')),
  CHECK (candidate_revision >= 1)
);
