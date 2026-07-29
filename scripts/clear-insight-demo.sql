-- 清除 scripts/seed-insight-demo.sql 写入的本地演示数据。
-- 按 experience_id 而不是自己的 id 删：在页面上点过确认/复审之后，
-- 审计和引用会多出真实生成的行（id 是 experienceaudit_xxx），
-- 只按 demo_ 前缀删会把它们留下，外键随即拦住经验行的删除。
--
-- 经验行同理：在报告中心「沉淀成经验」出来的行 id 是 experience_xxx，
-- 不带 demo_ 前缀，但它挂在 demo_report_x 上，留着就会拦住报告的删除。
-- 所以先把这批经验的 id 收进临时表，引用/审计/经验三张表都照它删。
DROP TEMPORARY TABLE IF EXISTS demo_experience_ids;
CREATE TEMPORARY TABLE demo_experience_ids (id VARCHAR(64) PRIMARY KEY);
INSERT INTO demo_experience_ids (id)
  SELECT id FROM insight_experiences WHERE id LIKE 'demo\_%' OR report_id LIKE 'demo\_%';

DELETE FROM insight_experience_references
  WHERE id LIKE 'demo\_%' OR experience_id IN (SELECT id FROM demo_experience_ids);
DELETE FROM insight_experience_audits
  WHERE id LIKE 'demo\_%' OR experience_id IN (SELECT id FROM demo_experience_ids);
DELETE FROM insight_experiences WHERE id IN (SELECT id FROM demo_experience_ids);
DROP TEMPORARY TABLE demo_experience_ids;

DELETE FROM insight_reports WHERE id LIKE 'demo\_%';

-- 投放链路按外键反序删除：指标快照 → 证据 → 执行 → 变更集 → 计划。
DELETE FROM delivery_metric_snapshots WHERE id LIKE 'demo\_%';
DELETE FROM delivery_evidence WHERE id LIKE 'demo\_%';
DELETE FROM delivery_executions WHERE id LIKE 'demo\_%';
DELETE FROM delivery_change_sets WHERE id LIKE 'demo\_%';
DELETE FROM delivery_plans WHERE id LIKE 'demo\_%';
