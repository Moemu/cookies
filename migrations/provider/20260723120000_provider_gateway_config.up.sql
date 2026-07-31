CREATE TABLE provider_connections (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  connection_code VARCHAR(96) NOT NULL,
  connection_type VARCHAR(32) NOT NULL,
  current_revision_id VARCHAR(96) NULL,
  status VARCHAR(16) NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_provider_connections_code (connection_code),
  CONSTRAINT chk_provider_connection_type CHECK (connection_type IN ('adapter_gateway')),
  CONSTRAINT chk_provider_connection_status CHECK (status IN ('enabled', 'disabled'))
);

CREATE TABLE provider_connection_revisions (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  connection_id VARCHAR(96) NOT NULL,
  revision_number BIGINT NOT NULL,
  base_url VARCHAR(1024) NOT NULL,
  timeout_seconds INT NOT NULL DEFAULT 210,
  max_response_bytes BIGINT NOT NULL DEFAULT 41943040,
  config_json JSON NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_provider_connection_revision (connection_id, revision_number),
  CONSTRAINT fk_provider_connection_revision_connection
    FOREIGN KEY (connection_id) REFERENCES provider_connections(id)
);

ALTER TABLE provider_connections
  ADD CONSTRAINT fk_provider_connection_current_revision
  FOREIGN KEY (current_revision_id) REFERENCES provider_connection_revisions(id);

CREATE TABLE provider_credentials (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  connection_id VARCHAR(96) NOT NULL,
  credential_version BIGINT NOT NULL,
  ciphertext VARBINARY(4096) NOT NULL,
  nonce VARBINARY(32) NOT NULL,
  key_version VARCHAR(64) NOT NULL,
  status VARCHAR(16) NOT NULL,
  active_from DATETIME(6) NOT NULL,
  active_until DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_provider_credential_version (connection_id, credential_version),
  KEY idx_provider_credentials_active (connection_id, status, active_from, active_until),
  CONSTRAINT fk_provider_credential_connection FOREIGN KEY (connection_id) REFERENCES provider_connections(id),
  CONSTRAINT chk_provider_credential_status CHECK (status IN ('active', 'retired', 'revoked'))
);

CREATE TABLE provider_model_routes (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) NULL,
  organization_scope VARCHAR(96) GENERATED ALWAYS AS (COALESCE(organization_id, '*')) STORED,
  capability VARCHAR(64) NOT NULL,
  model_alias VARCHAR(255) NOT NULL,
  current_revision_id VARCHAR(96) NULL,
  status VARCHAR(16) NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_provider_model_route (organization_scope, capability, model_alias),
  KEY idx_provider_model_route_lookup (capability, model_alias, status),
  CONSTRAINT chk_provider_route_status CHECK (status IN ('enabled', 'disabled'))
);

CREATE TABLE provider_model_route_revisions (
  id VARCHAR(96) NOT NULL PRIMARY KEY,
  route_id VARCHAR(96) NOT NULL,
  revision_number BIGINT NOT NULL,
  connection_id VARCHAR(96) NOT NULL,
  connection_revision_id VARCHAR(96) NOT NULL,
  upstream_model VARCHAR(255) NOT NULL,
  constraints_json JSON NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_provider_model_route_revision (route_id, revision_number),
  CONSTRAINT fk_provider_route_revision_route FOREIGN KEY (route_id) REFERENCES provider_model_routes(id),
  CONSTRAINT fk_provider_route_revision_connection FOREIGN KEY (connection_id) REFERENCES provider_connections(id),
  CONSTRAINT fk_provider_route_revision_connection_revision FOREIGN KEY (connection_revision_id) REFERENCES provider_connection_revisions(id)
);

ALTER TABLE provider_model_routes
  ADD CONSTRAINT fk_provider_route_current_revision
  FOREIGN KEY (current_revision_id) REFERENCES provider_model_route_revisions(id);

ALTER TABLE provider_jobs
  ADD COLUMN route_snapshot JSON NULL AFTER model_alias,
  ADD COLUMN submission_state VARCHAR(16) NOT NULL DEFAULT 'not_started' AFTER external_task_id,
  ADD COLUMN adapter_request_id VARCHAR(255) NULL AFTER submission_state,
  ADD COLUMN actual_provider VARCHAR(96) NULL AFTER adapter_request_id,
  ADD COLUMN actual_model VARCHAR(255) NULL AFTER actual_provider,
  ADD COLUMN execution_deadline_at DATETIME(6) NULL AFTER actual_model,
  ADD COLUMN submitted_at DATETIME(6) NULL AFTER execution_deadline_at,
  ADD COLUMN response_received_at DATETIME(6) NULL AFTER submitted_at,
  ADD CONSTRAINT chk_provider_submission_state CHECK (submission_state IN ('not_started', 'in_flight', 'completed', 'unknown'));
