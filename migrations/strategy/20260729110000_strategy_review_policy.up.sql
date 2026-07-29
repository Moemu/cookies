CREATE TABLE strategy_review_policies (
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  mode VARCHAR(32) NOT NULL,
  approver_user_ids JSON NOT NULL,
  allow_self_approval BOOLEAN NOT NULL DEFAULT FALSE,
  version BIGINT NOT NULL,
  updated_by VARCHAR(96) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (organization_id, project_id),
  CONSTRAINT chk_strategy_review_policy_mode
    CHECK (mode IN ('self_confirmation', 'leader_approval', 'designated_approvers')),
  CONSTRAINT fk_strategy_review_policy_project
    FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id)
);

CREATE TABLE strategy_review_assignments (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  review_id VARCHAR(96) NOT NULL,
  reviewer_user_id VARCHAR(96) NOT NULL,
  review_mode VARCHAR(32) NOT NULL,
  status VARCHAR(24) NOT NULL,
  policy_snapshot JSON NOT NULL,
  decision_reason TEXT NULL,
  decided_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_strategy_review_assignment (organization_id, project_id, review_id, reviewer_user_id),
  KEY idx_strategy_review_assignment_reviewer
    (organization_id, reviewer_user_id, status, updated_at),
  CONSTRAINT chk_strategy_review_assignment_mode
    CHECK (review_mode IN ('self_confirmation', 'leader_approval', 'designated_approvers')),
  CONSTRAINT chk_strategy_review_assignment_status
    CHECK (status IN ('pending', 'approved', 'returned', 'cancelled')),
  CONSTRAINT fk_strategy_review_assignment_review
    FOREIGN KEY (review_id) REFERENCES strategy_reviews(id),
  CONSTRAINT fk_strategy_review_assignment_membership
    FOREIGN KEY (organization_id, reviewer_user_id)
    REFERENCES organization_memberships(organization_id, user_id)
);
