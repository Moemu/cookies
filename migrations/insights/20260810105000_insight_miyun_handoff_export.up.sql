-- Immutable source/profile snapshots make a handoff reproducible after the
-- current product profile or its Project files change.
ALTER TABLE insight_miyun_handoffs
  ADD COLUMN profile_snapshot JSON NOT NULL AFTER source_snapshot,
  ADD COLUMN input_hash CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL AFTER profile_snapshot,
  ADD UNIQUE KEY uq_insight_miyun_handoffs_input (organization_id, project_id, input_hash);
