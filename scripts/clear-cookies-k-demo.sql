-- 撤销 scripts/seed-cookies-k-demo.sql 灌进去的演示数据。
-- 只删主键以 k_ 开头的行，不碰 demo_ 开头的那套（scripts/clear-insight-demo.sql 管那些），
-- 也不碰 org_local / project_local / user_local 这些本地基础数据。
-- 删除顺序按外键从下游到上游排。

SET @org = 'org_local';

DELETE FROM insight_experience_references WHERE organization_id = @org AND id LIKE 'k\_%';
DELETE FROM insight_experience_audits     WHERE organization_id = @org AND id LIKE 'k\_%';
DELETE FROM insight_experiences           WHERE organization_id = @org AND id LIKE 'k\_%';
DELETE FROM insight_reports               WHERE organization_id = @org AND id LIKE 'k\_%';

DELETE FROM delivery_metric_snapshots WHERE organization_id = @org AND id LIKE 'k\_%';
DELETE FROM delivery_evidence         WHERE organization_id = @org AND id LIKE 'k\_%';
DELETE FROM delivery_executions       WHERE organization_id = @org AND id LIKE 'k\_%';
DELETE FROM delivery_change_sets      WHERE organization_id = @org AND id LIKE 'k\_%';
DELETE FROM delivery_plans            WHERE organization_id = @org AND id LIKE 'k\_%';

DELETE FROM project_context_versions WHERE organization_id = @org AND project_id LIKE 'k\_%';
DELETE FROM project_products         WHERE organization_id = @org AND project_id LIKE 'k\_%';
DELETE FROM project_memberships      WHERE organization_id = @org AND project_id LIKE 'k\_%';
DELETE FROM projects                 WHERE organization_id = @org AND id LIKE 'k\_%';

DELETE FROM products                  WHERE organization_id = @org AND id LIKE 'k\_%';
DELETE FROM brand_guideline_versions  WHERE organization_id = @org AND id LIKE 'k\_%';
DELETE FROM brands                    WHERE organization_id = @org AND id LIKE 'k\_%';

DELETE FROM organization_memberships WHERE organization_id = @org AND user_id LIKE 'k\_%';
DELETE FROM users                    WHERE id LIKE 'k\_%';
