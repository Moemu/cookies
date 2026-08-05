CREATE TABLE creative_image_prompt_packages (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  task_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  draft_revision BIGINT NOT NULL,
  image_plan_order INT NOT NULL,
  direction_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  direction_content_hash VARCHAR(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  input_identity_hash VARCHAR(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  compiler_version VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  prompt_payload JSON NOT NULL,
  content_hash VARCHAR(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_by VARCHAR(192) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, id),
  UNIQUE KEY uq_creative_image_prompt_content
    (organization_id, task_id, draft_revision, image_plan_order, content_hash),
  KEY idx_creative_image_prompt_task
    (organization_id, project_id, task_id, draft_revision, image_plan_order),
  CONSTRAINT fk_creative_image_prompt_task
    FOREIGN KEY (organization_id, task_id) REFERENCES creative_tasks(organization_id, id),
  CONSTRAINT chk_creative_image_prompt_revision CHECK (draft_revision > 0),
  CONSTRAINT chk_creative_image_prompt_order CHECK (image_plan_order BETWEEN 1 AND 3)
);
