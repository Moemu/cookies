CREATE TABLE provider_job_output_handles (
  provider_job_id VARCHAR(96) NOT NULL,
  output_id VARCHAR(128) NOT NULL,
  organization_id VARCHAR(96) NOT NULL,
  project_id VARCHAR(96) NOT NULL,
  provider_code VARCHAR(64) NOT NULL,
  retrieval_expires_at DATETIME(6) NOT NULL,
  mime_type VARCHAR(255) NOT NULL,
  size_bytes BIGINT NOT NULL,
  sha256 CHAR(64) NOT NULL,
  contents LONGBLOB NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (provider_job_id, output_id),
  KEY idx_provider_output_handles_expiry (retrieval_expires_at),
  CONSTRAINT fk_provider_output_handles_job FOREIGN KEY (provider_job_id) REFERENCES provider_jobs(id)
);
