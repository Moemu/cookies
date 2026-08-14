ALTER TABLE delivery_controlled_change_sets
  ADD COLUMN object_fingerprint CHAR(64) CHARACTER SET ascii COLLATE ascii_bin
    GENERATED ALWAYS AS (JSON_UNQUOTE(JSON_EXTRACT(binding_json, '$.object_fingerprint'))) STORED,
  ADD UNIQUE KEY uq_delivery_controlled_change_set_fingerprint (organization_id, project_id, object_fingerprint);

ALTER TABLE delivery_platform_entity_mappings
  ADD CONSTRAINT chk_delivery_platform_mapping_distinct_evidence
    CHECK (status <> 'confirmed' OR result_evidence_id <> list_evidence_id);
