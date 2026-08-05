-- Experience lifecycle: 待确认 -> 已确认 -> 待复审 -> 已失效 (PRD §11.1),
-- version chains that never silently overwrite a prior conclusion (PRD §7.6),
-- and reference records that carry downstream adoption feedback (AM-014).
ALTER TABLE insight_experiences
  DROP CHECK chk_insight_experiences_status;

-- fk_insight_experiences_report currently leans on the unique key. Give the
-- foreign key a plain index first so a report can carry several revisions.
ALTER TABLE insight_experiences
  ADD KEY idx_insight_experiences_report (organization_id, report_id);

ALTER TABLE insight_experiences
  DROP INDEX uq_insight_experiences_report;

ALTER TABLE insight_experiences
  ADD COLUMN lineage_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' AFTER project_id,
  ADD COLUMN revision INT NOT NULL DEFAULT 1 AFTER lineage_id,
  ADD COLUMN supersedes_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER revision,
  ADD COLUMN superseded_by_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER supersedes_id,
  ADD COLUMN status_reason VARCHAR(1000) NOT NULL DEFAULT '' AFTER status,
  ADD COLUMN status_changed_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '' AFTER status_reason,
  ADD COLUMN status_changed_at DATETIME(6) NULL AFTER status_changed_by,
  ADD COLUMN confirmed_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL AFTER status_changed_at,
  ADD COLUMN confirmed_at DATETIME(6) NULL AFTER confirmed_by;

UPDATE insight_experiences
SET lineage_id = id,
    revision = 1,
    status_changed_by = created_by,
    status_changed_at = updated_at,
    confirmed_by = created_by,
    confirmed_at = created_at
WHERE lineage_id = '';

ALTER TABLE insight_experiences
  ADD CONSTRAINT chk_insight_experiences_status CHECK (status IN ('pending', 'confirmed', 'needs_review', 'retired')),
  ADD CONSTRAINT chk_insight_experiences_revision CHECK (revision > 0),
  ADD UNIQUE KEY uq_insight_experiences_lineage (organization_id, lineage_id, revision),
  ADD KEY idx_insight_experiences_status (organization_id, project_id, status, updated_at);

CREATE TABLE insight_experience_audits (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  experience_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  from_status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  to_status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  reason VARCHAR(1000) NOT NULL,
  actor_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  KEY idx_insight_experience_audits_experience (organization_id, experience_id, created_at),
  CONSTRAINT fk_insight_experience_audits_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT fk_insight_experience_audits_experience FOREIGN KEY (organization_id, experience_id) REFERENCES insight_experiences(organization_id, id)
);

CREATE TABLE insight_experience_references (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  experience_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  consumer_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  consumer_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  outcome VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  note VARCHAR(1000) NOT NULL,
  version BIGINT NOT NULL,
  created_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uq_insight_experience_references_scope (organization_id, id),
  UNIQUE KEY uq_insight_experience_references_consumer (organization_id, experience_id, consumer_kind, consumer_id),
  KEY idx_insight_experience_references_experience (organization_id, experience_id, created_at),
  CONSTRAINT fk_insight_experience_references_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT fk_insight_experience_references_experience FOREIGN KEY (organization_id, experience_id) REFERENCES insight_experiences(organization_id, id),
  CONSTRAINT chk_insight_experience_references_version CHECK (version > 0),
  CONSTRAINT chk_insight_experience_references_outcome CHECK (outcome IN ('referenced', 'adopted', 'modified', 'rejected'))
);
