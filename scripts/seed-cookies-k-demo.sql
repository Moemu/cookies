-- 本地演示数据：把参考产品 cookies-k 的 mock（src/data/api.ts 的 agencyWorkbenchSample）
-- 搬进本地数据库，让侧栏、项目切换、投前洞察、投后分析、报告中心、经验库都有真实可点的内容。
--
-- 所有新增行的主键都以 k_ 开头（k = cookies-k），清理执行 scripts/clear-cookies-k-demo.sql。
-- 仅用于本地开发环境，不要在任何共享环境执行。
--
-- 没有搬过来的部分（我们的库里没有对应的表或字段，不是漏了）：
--   1. 客户（clients）：诺瓦生活科技 / 环域健康 —— 我们没有「客户」这一层，只有组织→品牌→项目；
--      客户名只能留在下面的注释里。
--   2. 组织：mock 里是「星河增长代理商 org-demo-agency」，但本地登录身份固定绑在 org_local 上，
--      换组织会直接登不进去，所以这三个项目挂在 org_local 下面。
--   3. 项目的运行态字段：阶段、进度百分比、风险状态、阻塞原因、预算、负责人、时区 —— projects 表里没有这些列。
--      预算改挂到投放计划的 budget_cents 上，其余信息写进了项目/计划的名称和文案里。
--   4. 广告账户绑定、素材质检记录、素材人工确认、素材版本指针 —— 这四类在我们的库里还没有建表。
--      它们的内容没有丢：质检结论和确认意见被翻译成了下面的复盘报告与经验条目。

SET @org = 'org_local';
SET @actor = 'user_local';
SET @now = '2026-07-27 08:00:00.000000';

-- ---------------------------------------------------------------------------
-- 1. 人：mock 里的负责人
-- ---------------------------------------------------------------------------

INSERT INTO users (id, display_name, status, created_at, updated_at) VALUES
  ('k_user_mia',    'Mia Chen',    'active', @now, @now),
  ('k_user_amelia', 'Amelia Meng', 'active', @now, @now),
  ('k_user_noah',   'Noah Xu',     'active', @now, @now),
  ('k_user_lin',    'Lin Wei',     'active', @now, @now),
  ('k_user_sofia',  'Sofia Chen',  'active', @now, @now);

INSERT INTO organization_memberships (organization_id, user_id, role, status, created_at, updated_at) VALUES
  (@org, 'k_user_mia',    'owner',  'active', @now, @now),
  (@org, 'k_user_amelia', 'admin',  'active', @now, @now),
  (@org, 'k_user_noah',   'member', 'active', @now, @now),
  (@org, 'k_user_lin',    'member', 'active', @now, @now),
  (@org, 'k_user_sofia',  'member', 'active', @now, @now);

-- ---------------------------------------------------------------------------
-- 2. 品牌与品牌规范
--    guidelineStatus 用规范版本的状态表达：ready → approved，outdated → archived。
--    Nova Kids 的项目仍然挂在一个已归档的规范版本上，这就是 mock 里「规范已过期」的意思。
-- ---------------------------------------------------------------------------

INSERT INTO brands (id, organization_id, name, status, created_at, updated_at) VALUES
  ('k_brand_nova_home',  @org, 'Nova Home',  'active', @now, @now),   -- 客户：诺瓦生活科技 / 类目：智能清洁
  ('k_brand_nova_kids',  @org, 'Nova Kids',  'active', @now, @now),   -- 客户：诺瓦生活科技 / 类目：儿童陪伴硬件
  ('k_brand_orbit_care', @org, 'Orbit Care', 'active', @now, @now);   -- 客户：环域健康 / 类目：健康管理

INSERT INTO brand_guideline_versions (id, organization_id, brand_id, version, status, created_at) VALUES
  ('k_guideline_nova_home_v1',  @org, 'k_brand_nova_home',  1, 'approved', @now),
  ('k_guideline_nova_kids_v1',  @org, 'k_brand_nova_kids',  1, 'archived', @now),
  ('k_guideline_orbit_care_v1', @org, 'k_brand_orbit_care', 1, 'approved', @now);

INSERT INTO products (id, organization_id, name, status, created_at, updated_at) VALUES
  ('k_product_nova_s8',   @org, '全屋扫拖机器人 S8', 'active', @now, @now),
  ('k_product_nova_l2',   @org, 'AI 学习灯 L2',      'active', @now, @now),
  ('k_product_orbit_pro', @org, '睡眠监测贴片 Pro',  'active', @now, @now);

-- ---------------------------------------------------------------------------
-- 3. 三个项目
-- ---------------------------------------------------------------------------

INSERT INTO projects
  (id, organization_id, name, status, primary_brand_id, brand_guideline_version_id,
   project_context_version, created_at, updated_at)
VALUES
  ('k_project_nova_home_launch',  @org, 'Nova Home 夏季清洁增长', 'active',
   'k_brand_nova_home',  'k_guideline_nova_home_v1',  3,
   '2026-07-18 02:00:00.000000', '2026-07-27 07:50:00.000000'),
  ('k_project_nova_kids_presale', @org, 'Nova Kids 开学季预售',   'active',
   'k_brand_nova_kids',  'k_guideline_nova_kids_v1',  2,
   '2026-07-20 03:10:00.000000', '2026-07-27 06:20:00.000000'),
  ('k_project_orbit_care_sleep',  @org, 'Orbit Care 睡眠健康线索', 'active',
   'k_brand_orbit_care', 'k_guideline_orbit_care_v1', 4,
   '2026-07-12 01:30:00.000000', '2026-07-27 07:10:00.000000');

-- 没有成员关系就打不开项目：登录身份 user_local 必须在每个项目里。
INSERT INTO project_memberships
  (organization_id, project_id, principal_kind, principal_id, role, status, created_at, updated_at)
VALUES
  (@org, 'k_project_nova_home_launch',  'user', @actor,       'owner',  'active', @now, @now),
  (@org, 'k_project_nova_home_launch',  'user', 'k_user_lin',  'owner',  'active', @now, @now),
  (@org, 'k_project_nova_kids_presale', 'user', @actor,       'owner',  'active', @now, @now),
  (@org, 'k_project_nova_kids_presale', 'user', 'k_user_sofia','owner',  'active', @now, @now),
  (@org, 'k_project_orbit_care_sleep',  'user', @actor,       'owner',  'active', @now, @now),
  (@org, 'k_project_orbit_care_sleep',  'user', 'k_user_noah', 'owner',  'active', @now, @now);

INSERT INTO project_products (organization_id, project_id, product_id, created_at) VALUES
  (@org, 'k_project_nova_home_launch',  'k_product_nova_s8',   @now),
  (@org, 'k_project_nova_kids_presale', 'k_product_nova_l2',   @now),
  (@org, 'k_project_orbit_care_sleep',  'k_product_orbit_pro', @now);

INSERT INTO project_context_versions
  (organization_id, project_id, version, brand_id, brand_guideline_version_id, product_ids, created_at)
VALUES
  (@org, 'k_project_nova_home_launch',  3, 'k_brand_nova_home',  'k_guideline_nova_home_v1',
   JSON_ARRAY('k_product_nova_s8'), '2026-07-27 07:50:00.000000'),
  (@org, 'k_project_nova_kids_presale', 2, 'k_brand_nova_kids',  'k_guideline_nova_kids_v1',
   JSON_ARRAY('k_product_nova_l2'), '2026-07-27 06:20:00.000000'),
  (@org, 'k_project_orbit_care_sleep',  4, 'k_brand_orbit_care', 'k_guideline_orbit_care_v1',
   JSON_ARRAY('k_product_orbit_pro'), '2026-07-27 07:10:00.000000');

-- ---------------------------------------------------------------------------
-- 4. 投放链路：计划 → 变更集 → 执行 → 证据 → 指标快照
--    投后分析和报告中心读的是这条链路，不灌它这两页切到新项目就是空的。
--    预算沿用 mock 里项目的 budget（元 → 分）。
-- ---------------------------------------------------------------------------

INSERT INTO delivery_plans
  (id, organization_id, project_id, creative_package_id, creative_package_hash, creative_version_id,
   name, objective, budget_cents, start_at, end_at, status, version, created_by, created_at, updated_at)
VALUES
  ('k_plan_nova_home_1', @org, 'k_project_nova_home_launch',
   'k_package_nova_home_1', 'sha256:knovahome1', 'k_package_nova_home_1_v1',
   '巨量引擎信息流首轮测试', '验证家庭清洁痛点素材，提升搜索与信息流线索效率。', 26000000,
   '2026-07-18 00:00:00.000000', '2026-07-25 00:00:00.000000', 'draft', 1, 'k_user_lin', @now, @now),
  ('k_plan_nova_home_2', @org, 'k_project_nova_home_launch',
   'k_package_nova_home_2', 'sha256:knovahome2', 'k_package_nova_home_2_v1',
   '巨量引擎信息流复投', '钩子视频返修后复投，验证去掉价格承诺是否影响点击。', 14000000,
   '2026-07-21 00:00:00.000000', '2026-07-27 00:00:00.000000', 'draft', 1, 'k_user_lin', @now, @now),
  ('k_plan_nova_kids_1', @org, 'k_project_nova_kids_presale',
   'k_package_nova_kids_1', 'sha256:knovakids1', 'k_package_nova_kids_1_v1',
   '快手磁力预售种草测试', '围绕护眼与陪伴场景准备预售素材和账户测试计划。', 18000000,
   '2026-07-21 00:00:00.000000', '2026-07-26 00:00:00.000000', 'draft', 1, 'k_user_sofia', @now, @now),
  ('k_plan_orbit_sleep_1', @org, 'k_project_orbit_care_sleep',
   'k_package_orbit_sleep_1', 'sha256:korbitsleep1', 'k_package_orbit_sleep_1_v1',
   '腾讯广告睡眠教育线索测试', '验证睡眠监测教育素材，建立可扩量的获客计划。', 32000000,
   '2026-07-19 00:00:00.000000', '2026-07-26 00:00:00.000000', 'draft', 1, 'k_user_noah', @now, @now);

INSERT INTO delivery_change_sets
  (id, organization_id, project_id, plan_id, plan_version, status, risk_level, preflight_notes,
   approved_by, approved_at, version, created_by, created_at, updated_at)
VALUES
  ('k_changeset_nova_home_1', @org, 'k_project_nova_home_launch', 'k_plan_nova_home_1', 1, 'executed', 'medium',
   JSON_ARRAY('预算在演示上限内', '钩子视频 v2 质检未通过，仅证明段进入投放'),
   'k_user_lin', @now, 3, 'k_user_lin', @now, @now),
  ('k_changeset_nova_home_2', @org, 'k_project_nova_home_launch', 'k_plan_nova_home_2', 1, 'executed', 'low',
   JSON_ARRAY('预算在演示上限内', '钩子视频 v3 已移除价格承诺'),
   'k_user_lin', @now, 3, 'k_user_lin', @now, @now),
  ('k_changeset_nova_kids_1', @org, 'k_project_nova_kids_presale', 'k_plan_nova_kids_1', 1, 'executed', 'medium',
   JSON_ARRAY('预算在演示上限内', '快手账户登录状态告警，投放期间需人工盯盘'),
   'k_user_sofia', @now, 3, 'k_user_sofia', @now, @now),
  ('k_changeset_orbit_sleep_1', @org, 'k_project_orbit_care_sleep', 'k_plan_orbit_sleep_1', 1, 'executed', 'high',
   JSON_ARRAY('预算在演示上限内', '腾讯广告账户追踪状态已过期，转化回传可能缺失'),
   'k_user_noah', @now, 3, 'k_user_noah', @now, @now);

INSERT INTO delivery_executions
  (id, organization_id, project_id, change_set_id, status, execution_mode, executed_by, started_at, completed_at)
VALUES
  ('k_execution_nova_home_1', @org, 'k_project_nova_home_launch', 'k_changeset_nova_home_1',
   'succeeded', 'local_simulation', 'k_user_lin',
   '2026-07-18 00:00:00.000000', '2026-07-25 00:00:00.000000'),
  ('k_execution_nova_home_2', @org, 'k_project_nova_home_launch', 'k_changeset_nova_home_2',
   'succeeded', 'local_simulation', 'k_user_lin',
   '2026-07-21 00:00:00.000000', '2026-07-27 00:00:00.000000'),
  ('k_execution_nova_kids_1', @org, 'k_project_nova_kids_presale', 'k_changeset_nova_kids_1',
   'succeeded', 'local_simulation', 'k_user_sofia',
   '2026-07-21 00:00:00.000000', '2026-07-26 00:00:00.000000'),
  ('k_execution_orbit_sleep_1', @org, 'k_project_orbit_care_sleep', 'k_changeset_orbit_sleep_1',
   'succeeded', 'local_simulation', 'k_user_noah',
   '2026-07-19 00:00:00.000000', '2026-07-26 00:00:00.000000');

INSERT INTO delivery_evidence
  (id, organization_id, project_id, execution_id, summary, evidence_mode, reversible, created_at)
VALUES
  ('k_evidence_nova_home_1', @org, 'k_project_nova_home_launch', 'k_execution_nova_home_1',
   '本地模拟投放证据：巨量引擎演示账户 JLY-DEMO-NOVA-001，投放 1 条产品证明段素材（钩子视频因质检未通过被剔除），投放 7 天。',
   'local_simulation', 1, '2026-07-25 00:00:00.000000'),
  ('k_evidence_nova_home_2', @org, 'k_project_nova_home_launch', 'k_execution_nova_home_2',
   '本地模拟投放证据：巨量引擎演示账户 JLY-DEMO-NOVA-001，投放 2 条素材（返修后的钩子视频 v3 与产品证明段 v1），投放 6 天。',
   'local_simulation', 1, '2026-07-27 00:00:00.000000'),
  ('k_evidence_nova_kids_1', @org, 'k_project_nova_kids_presale', 'k_execution_nova_kids_1',
   '本地模拟投放证据：快手磁力演示账户 KS-DEMO-NOVA-018，投放 1 条护眼场景素材，投放 5 天，期间账户登录状态曾告警。',
   'local_simulation', 1, '2026-07-26 00:00:00.000000'),
  ('k_evidence_orbit_sleep_1', @org, 'k_project_orbit_care_sleep', 'k_execution_orbit_sleep_1',
   '本地模拟投放证据：腾讯广告演示账户 TX-DEMO-ORBIT-026，投放 2 条睡眠教育素材，投放 7 天，账户追踪状态过期期间转化回传不完整。',
   'local_simulation', 1, '2026-07-26 00:00:00.000000');

INSERT INTO delivery_metric_snapshots
  (id, organization_id, project_id, execution_id, plan_id, creative_package_id, source, is_simulated,
   dataset_version, currency, window_start, window_end, impressions, clicks, conversions, spend_cents,
   created_by, created_at)
VALUES
  ('k_metric_nova_home_1', @org, 'k_project_nova_home_launch', 'k_execution_nova_home_1',
   'k_plan_nova_home_1', 'k_package_nova_home_1', 'demo_fixture', 1, 'agency-demo/v1', 'CNY',
   '2026-07-18 00:00:00.000000', '2026-07-25 00:00:00.000000', 214000, 6420, 733, 2450000, 'k_user_lin', @now),
  ('k_metric_nova_home_2', @org, 'k_project_nova_home_launch', 'k_execution_nova_home_2',
   'k_plan_nova_home_2', 'k_package_nova_home_2', 'demo_fixture', 1, 'agency-demo/v1', 'CNY',
   '2026-07-21 00:00:00.000000', '2026-07-27 00:00:00.000000', 132000, 4180, 461, 1610000, 'k_user_lin', @now),
  ('k_metric_nova_kids_1', @org, 'k_project_nova_kids_presale', 'k_execution_nova_kids_1',
   'k_plan_nova_kids_1', 'k_package_nova_kids_1', 'demo_fixture', 1, 'agency-demo/v1', 'CNY',
   '2026-07-21 00:00:00.000000', '2026-07-26 00:00:00.000000', 88000, 2110, 168, 940000, 'k_user_sofia', @now),
  ('k_metric_orbit_sleep_1', @org, 'k_project_orbit_care_sleep', 'k_execution_orbit_sleep_1',
   'k_plan_orbit_sleep_1', 'k_package_orbit_sleep_1', 'demo_fixture', 1, 'agency-demo/v1', 'CNY',
   '2026-07-19 00:00:00.000000', '2026-07-26 00:00:00.000000', 156000, 3900, 289, 1880000, 'k_user_noah', @now);

-- ---------------------------------------------------------------------------
-- 5. 复盘报告
--    内容来自 mock 的质检结论（qualityCheckRuns）和人工确认意见（materialConfirmations）。
--    Nova Home 第 2 次执行故意不带报告，用来在界面上演示「生成复盘草稿」。
--    Nova Kids 的报告是草稿状态，用来演示「待确认」的报告长什么样。
-- ---------------------------------------------------------------------------

INSERT INTO insight_reports
  (id, organization_id, project_id, execution_id, delivery_mode, evidence_id, evidence_summary,
   metric_snapshot_id, creative_package_id, is_simulated, dataset_version, status, summary, findings,
   version, created_by, confirmed_by, confirmed_at, created_at, updated_at)
VALUES
  ('k_report_nova_home_1', @org, 'k_project_nova_home_launch', 'k_execution_nova_home_1', 'local_simulation',
   'k_evidence_nova_home_1',
   '本地模拟投放证据：巨量引擎演示账户 JLY-DEMO-NOVA-001，投放 1 条产品证明段素材（钩子视频因质检未通过被剔除），投放 7 天。',
   'k_metric_nova_home_1', 'k_package_nova_home_1', 1, 'agency-demo/v1', 'confirmed',
   '产品证明段单素材跑满 7 天，线索率稳定；钩子视频因价格权益口径被质检打回，这一轮没拿到对比数据。',
   JSON_ARRAY(
     '产品证明段点击率 3.0%，线索转化率 11.4%，全程无异常波动',
     '钩子视频 v2 第 6 秒字幕出现未在 Brief 中确认的限时优惠，质检判定为 major 问题',
     '因为只有一条素材在跑，「真人出镜 vs 产品图」的对比这一轮无法回答'),
   2, 'k_user_lin', 'k_user_amelia', '2026-07-26 02:00:00.000000', '2026-07-25 06:00:00.000000', '2026-07-26 02:00:00.000000'),

  ('k_report_nova_kids_1', @org, 'k_project_nova_kids_presale', 'k_execution_nova_kids_1', 'local_simulation',
   'k_evidence_nova_kids_1',
   '本地模拟投放证据：快手磁力演示账户 KS-DEMO-NOVA-018，投放 1 条护眼场景素材，投放 5 天，期间账户登录状态曾告警。',
   'k_metric_nova_kids_1', 'k_package_nova_kids_1', 1, 'agency-demo/v1', 'draft',
   '护眼场景开场的完播率高于产品参数开场，但线索量只有 168 条，还不足以下结论。',
   JSON_ARRAY(
     '护眼场景开场 3 秒完播率 62%，产品参数开场 41%',
     '账户登录告警期间有约 8 小时投放中断，曝光量偏低',
     '线索样本 168 条，低于我们自己定的 300 条起判线'),
   1, 'k_user_sofia', NULL, NULL, '2026-07-26 09:00:00.000000', '2026-07-26 09:00:00.000000'),

  ('k_report_orbit_sleep_1', @org, 'k_project_orbit_care_sleep', 'k_execution_orbit_sleep_1', 'local_simulation',
   'k_evidence_orbit_sleep_1',
   '本地模拟投放证据：腾讯广告演示账户 TX-DEMO-ORBIT-026，投放 2 条睡眠教育素材，投放 7 天，账户追踪状态过期期间转化回传不完整。',
   'k_metric_orbit_sleep_1', 'k_package_orbit_sleep_1', 1, 'agency-demo/v1', 'confirmed',
   '医学表述改成「辅助观察」后素材顺利过检；但账户追踪已过期，这一轮的转化数只能当下限看。',
   JSON_ARRAY(
     '把「诊断」改成「辅助观察」后，质检从 3 次驳回变为一次通过',
     '教育向素材的点击率 2.5%，明显高于纯产品展示素材',
     '账户追踪状态 expired，转化 289 条是回传下限，真实值应更高'),
   2, 'k_user_noah', 'k_user_noah', '2026-07-27 03:00:00.000000', '2026-07-26 08:00:00.000000', '2026-07-27 03:00:00.000000');

-- ---------------------------------------------------------------------------
-- 6. 经验条目：四种状态都覆盖到，附带一条版本链
-- ---------------------------------------------------------------------------

INSERT INTO insight_experiences
  (id, organization_id, project_id, lineage_id, revision, supersedes_id, superseded_by_id, report_id,
   source_execution_id, source_evidence_id, source_metric_snapshot_id, conclusion, conditions, counterexamples,
   status, status_reason, status_changed_by, status_changed_at, confirmed_by, confirmed_at,
   version, created_by, created_at, updated_at)
VALUES
  -- Nova Home：版本链，第 1 版说得太绝对，第 2 版补了条件后取代它。
  ('k_exp_nova_price_r1', @org, 'k_project_nova_home_launch', 'k_lineage_nova_price', 1, NULL, 'k_exp_nova_price_r2',
   'k_report_nova_home_1', 'k_execution_nova_home_1', 'k_evidence_nova_home_1', 'k_metric_nova_home_1',
   '短视频素材里不要出现价格承诺。',
   JSON_ARRAY('巨量引擎信息流'),
   JSON_ARRAY('只看了一条被打回的素材'),
   'retired', '说法太绝对，已被第 2 版取代。', 'k_user_amelia', '2026-07-27 05:50:00.000000',
   'k_user_amelia', '2026-07-26 02:10:00.000000', 4, 'k_user_lin', '2026-07-26 02:05:00.000000', '2026-07-27 05:50:00.000000'),

  ('k_exp_nova_price_r2', @org, 'k_project_nova_home_launch', 'k_lineage_nova_price', 2, 'k_exp_nova_price_r1', NULL,
   'k_report_nova_home_1', 'k_execution_nova_home_1', 'k_evidence_nova_home_1', 'k_metric_nova_home_1',
   '短视频钩子里出现未在 Brief 中确认的价格或权益承诺，会在素材质检环节被判为 major 问题打回。',
   JSON_ARRAY('巨量引擎信息流', '带权益字幕的短视频', '智能家居类目'),
   JSON_ARRAY('客户已在 Brief 中明确写入的限时活动不受此限', '仅有模拟指标，真实投放前需复核'),
   'confirmed', '补上「未在 Brief 中确认」这个前提后确认。', 'k_user_amelia', '2026-07-27 05:50:00.000000',
   'k_user_amelia', '2026-07-27 05:50:00.000000', 3, 'k_user_lin', '2026-07-27 05:45:00.000000', '2026-07-27 05:50:00.000000'),

  -- Nova Home：刚沉淀出来，还没人认领。
  ('k_exp_nova_proof_r1', @org, 'k_project_nova_home_launch', 'k_lineage_nova_proof', 1, NULL, NULL,
   'k_report_nova_home_1', 'k_execution_nova_home_1', 'k_evidence_nova_home_1', 'k_metric_nova_home_1',
   '「清洁前后对比 + 标准 CTA」的产品证明段，单素材连投 7 天线索率不衰减。',
   JSON_ARRAY('巨量引擎信息流', '扫拖机器人产品线', '单条素材连续投放 7 天以内'),
   JSON_ARRAY('只跑了一条素材，没有对照组'),
   'pending', '从复盘报告沉淀。', 'k_user_lin', '2026-07-26 02:20:00.000000',
   NULL, NULL, 1, 'k_user_lin', '2026-07-26 02:20:00.000000', '2026-07-26 02:20:00.000000'),

  -- Orbit Care：已确认，来自医学口径返修。
  ('k_exp_orbit_wording_r1', @org, 'k_project_orbit_care_sleep', 'k_lineage_orbit_wording', 1, NULL, NULL,
   'k_report_orbit_sleep_1', 'k_execution_orbit_sleep_1', 'k_evidence_orbit_sleep_1', 'k_metric_orbit_sleep_1',
   '消费医疗类素材用「辅助观察」替代「诊断」类表述，可以一次通过素材质检。',
   JSON_ARRAY('消费医疗类目', '睡眠监测 / 康复训练产品线', '腾讯广告教育向素材'),
   JSON_ARRAY('涉及疗效对比的素材仍需法务单独复核'),
   'confirmed', '连续三次驳回后改口径一次通过，结论可用。', 'k_user_noah', '2026-07-27 03:10:00.000000',
   'k_user_noah', '2026-07-27 03:10:00.000000', 3, 'k_user_noah', '2026-07-27 02:50:00.000000', '2026-07-27 03:10:00.000000'),

  -- Orbit Care：曾经确认，现在被打回复审。
  ('k_exp_orbit_authorization_r1', @org, 'k_project_orbit_care_sleep', 'k_lineage_orbit_authorization', 1, NULL, NULL,
   'k_report_orbit_sleep_1', 'k_execution_orbit_sleep_1', 'k_evidence_orbit_sleep_1', 'k_metric_orbit_sleep_1',
   '素材授权没有覆盖到的平台，不能把它设为该平台的交付版本。',
   JSON_ARRAY('外部提供脚本 / 素材的项目'),
   JSON_ARRAY('客户书面追加授权后可放行'),
   'needs_review', '睡眠教育素材的授权 2026-08-31 到期，且当前授权未覆盖腾讯广告，需要重新确认覆盖范围。',
   'k_user_noah', '2026-07-27 04:10:00.000000',
   'k_user_noah', '2026-07-27 03:20:00.000000', 3, 'k_user_noah', '2026-07-27 03:15:00.000000', '2026-07-27 04:10:00.000000');

INSERT INTO insight_experience_audits
  (id, organization_id, project_id, experience_id, from_status, to_status, reason, actor_id, created_at)
VALUES
  ('k_audit_01', @org, 'k_project_nova_home_launch', 'k_exp_nova_price_r1', '', 'pending',
   '从复盘报告沉淀。', 'k_user_lin', '2026-07-26 02:05:00.000000'),
  ('k_audit_02', @org, 'k_project_nova_home_launch', 'k_exp_nova_price_r1', 'pending', 'confirmed',
   '', 'k_user_amelia', '2026-07-26 02:10:00.000000'),
  ('k_audit_03', @org, 'k_project_nova_home_launch', 'k_exp_nova_price_r2', '', 'pending',
   '补上「未在 Brief 中确认」这个前提后修订。', 'k_user_lin', '2026-07-27 05:45:00.000000'),
  ('k_audit_04', @org, 'k_project_nova_home_launch', 'k_exp_nova_price_r2', 'pending', 'confirmed',
   '', 'k_user_amelia', '2026-07-27 05:50:00.000000'),
  ('k_audit_05', @org, 'k_project_nova_home_launch', 'k_exp_nova_price_r1', 'confirmed', 'retired',
   '说法太绝对，已被第 2 版取代。', 'k_user_amelia', '2026-07-27 05:50:00.000000'),
  ('k_audit_06', @org, 'k_project_nova_home_launch', 'k_exp_nova_proof_r1', '', 'pending',
   '从复盘报告沉淀。', 'k_user_lin', '2026-07-26 02:20:00.000000'),
  ('k_audit_07', @org, 'k_project_orbit_care_sleep', 'k_exp_orbit_wording_r1', '', 'pending',
   '从复盘报告沉淀。', 'k_user_noah', '2026-07-27 02:50:00.000000'),
  ('k_audit_08', @org, 'k_project_orbit_care_sleep', 'k_exp_orbit_wording_r1', 'pending', 'confirmed',
   '连续三次驳回后改口径一次通过，结论可用。', 'k_user_noah', '2026-07-27 03:10:00.000000'),
  ('k_audit_09', @org, 'k_project_orbit_care_sleep', 'k_exp_orbit_authorization_r1', '', 'pending',
   '从复盘报告沉淀。', 'k_user_noah', '2026-07-27 03:15:00.000000'),
  ('k_audit_10', @org, 'k_project_orbit_care_sleep', 'k_exp_orbit_authorization_r1', 'pending', 'confirmed',
   '', 'k_user_noah', '2026-07-27 03:20:00.000000'),
  ('k_audit_11', @org, 'k_project_orbit_care_sleep', 'k_exp_orbit_authorization_r1', 'confirmed', 'needs_review',
   '睡眠教育素材的授权 2026-08-31 到期，且当前授权未覆盖腾讯广告，需要重新确认覆盖范围。',
   'k_user_noah', '2026-07-27 04:10:00.000000');

-- 引用记录：下游用了这条经验之后回来说好不好用。
INSERT INTO insight_experience_references
  (id, organization_id, project_id, experience_id, consumer_kind, consumer_id, outcome, note,
   version, created_by, created_at, updated_at)
VALUES
  ('k_ref_01', @org, 'k_project_nova_home_launch', 'k_exp_nova_price_r2', 'strategy', 'k_strategy_nova_home_2608',
   'adopted', '开学季策略直接采纳，Brief 里加了权益口径确认项。',
   1, 'k_user_lin', '2026-07-27 06:00:00.000000', '2026-07-27 06:00:00.000000'),
  ('k_ref_02', @org, 'k_project_nova_home_launch', 'k_exp_nova_price_r1', 'strategy', 'k_strategy_nova_kids_2608',
   'modified', '按第 1 版执行时把所有价格信息都删了，投放方反馈过于保守。',
   1, 'k_user_sofia', '2026-07-26 08:00:00.000000', '2026-07-26 08:00:00.000000'),
  ('k_ref_03', @org, 'k_project_orbit_care_sleep', 'k_exp_orbit_wording_r1', 'creative', 'k_creative_orbit_proof_v3',
   'adopted', '证明段素材照此改写，质检一次通过。',
   1, 'k_user_noah', '2026-07-27 03:30:00.000000', '2026-07-27 03:30:00.000000');
