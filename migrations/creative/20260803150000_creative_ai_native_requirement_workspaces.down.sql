CREATE TEMPORARY TABLE rollback_ai_native_parent_ids AS
SELECT organization_id, creative_intake_id, creative_task_id
FROM creative_ai_native_requirement_workspaces;

DROP TABLE IF EXISTS creative_ai_native_requirement_revisions;
DROP TABLE IF EXISTS creative_ai_native_requirement_workspaces;

DELETE creative_tasks
FROM creative_tasks
JOIN rollback_ai_native_parent_ids ids
  ON ids.organization_id = creative_tasks.organization_id
 AND ids.creative_task_id = creative_tasks.id;

DELETE creative_intakes
FROM creative_intakes
JOIN rollback_ai_native_parent_ids ids
  ON ids.organization_id = creative_intakes.organization_id
 AND ids.creative_intake_id = creative_intakes.id;

DROP TEMPORARY TABLE rollback_ai_native_parent_ids;
