CREATE TABLE event_outbox (
  event_id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  event_type VARCHAR(128) NOT NULL,
  subject_type VARCHAR(128) NOT NULL,
  subject_id VARCHAR(128) NOT NULL,
  subject_version BIGINT NOT NULL,
  payload JSON NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'pending',
  available_at DATETIME(6) NOT NULL,
  attempt_count INT NOT NULL DEFAULT 0,
  last_error VARCHAR(1024) NULL,
  created_at DATETIME(6) NOT NULL,
  published_at DATETIME(6) NULL,
  UNIQUE KEY uq_outbox_subject_version (organization_id, event_type, subject_type, subject_id, subject_version),
  KEY idx_outbox_claim (status, available_at, created_at),
  KEY idx_outbox_scope (organization_id, project_id, event_id)
);

CREATE TABLE event_consumptions (
  consumer_name VARCHAR(128) NOT NULL,
  event_id VARCHAR(96) NOT NULL,
  consumed_at DATETIME(6) NOT NULL,
  PRIMARY KEY (consumer_name, event_id)
);
