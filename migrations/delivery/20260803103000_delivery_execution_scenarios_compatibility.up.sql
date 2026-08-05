-- Repair databases that applied the first A04 draft before queued executions
-- and Project-scoped idempotency were finalized. This migration is also safe
-- on a fresh A04 database: it reasserts the finalized column and index shape.
ALTER TABLE delivery_executions
  MODIFY COLUMN completed_at DATETIME(6) NULL,
  DROP INDEX uq_delivery_execution_idempotency,
  ADD UNIQUE KEY uq_delivery_execution_idempotency (organization_id, project_id, idempotency_key);
