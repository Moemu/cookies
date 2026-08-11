ALTER TABLE strategy_review_analyses
  DROP FOREIGN KEY fk_strategy_review_analysis_review,
  MODIFY COLUMN review_id VARCHAR(96) NULL,
  ADD COLUMN target_kind VARCHAR(32) NOT NULL DEFAULT 'review_candidate' AFTER project_id,
  ADD KEY idx_strategy_perspective_latest (organization_id, project_id, strategy_id, candidate_revision, created_at);

ALTER TABLE strategy_review_analyses
  ADD CONSTRAINT fk_strategy_review_analysis_review
    FOREIGN KEY (organization_id, project_id, review_id)
    REFERENCES strategy_reviews(organization_id, project_id, id),
  ADD CONSTRAINT chk_strategy_review_analysis_target
    CHECK (target_kind IN ('review_candidate', 'strategy_revision'));
