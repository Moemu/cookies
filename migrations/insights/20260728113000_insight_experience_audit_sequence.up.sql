-- created_at alone cannot order the audit trail: confirming a revision retires
-- its predecessor inside one transaction, so two rows can share a timestamp.
-- A monotonic sequence keeps the trail readable in the order it happened.
ALTER TABLE insight_experience_audits
  ADD COLUMN sequence BIGINT NOT NULL AUTO_INCREMENT UNIQUE KEY FIRST;
