-- The legacy primary source preserves relational lineage. This column freezes
-- every selected source in a stable order for the versioned handoff.
ALTER TABLE insight_miyun_handoffs
  ADD COLUMN source_material_ids JSON NULL AFTER source_material_id;

UPDATE insight_miyun_handoffs
  SET source_material_ids = JSON_ARRAY(source_material_id)
  WHERE source_material_ids IS NULL;

ALTER TABLE insight_miyun_handoffs
  MODIFY COLUMN source_material_ids JSON NOT NULL;
