CREATE TABLE IF NOT EXISTS asset_relations (
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  output_asset_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  output_asset_version BIGINT NOT NULL,
  relation_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_type VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_version BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (organization_id, project_id, output_asset_id, output_asset_version, relation_type, source_type, source_id, source_version),
  KEY idx_asset_relations_source (organization_id, project_id, source_type, source_id, source_version),
  CONSTRAINT fk_asset_relations_output FOREIGN KEY (organization_id, project_id, output_asset_id, output_asset_version) REFERENCES project_assets(organization_id, project_id, asset_id, asset_version),
  CONSTRAINT chk_asset_relations_type CHECK (relation_type IN ('generated_from')),
  CONSTRAINT chk_asset_relations_output_version CHECK (output_asset_version > 0),
  CONSTRAINT chk_asset_relations_source_version CHECK (source_version >= 0)
);
