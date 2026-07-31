-- 本地演示数据：填充素材洞察的经验库，让四种状态和版本链都能在界面上看到。
-- 所有 ID 都以 demo_ 开头，删除时执行 scripts/clear-insight-demo.sql 即可。
-- 仅用于本地开发环境，不要在任何共享环境执行。

SET @org = 'org_local';
SET @project = 'project_local';
SET @actor = 'user_local';
SET @now = '2026-07-28 10:00:00.000000';

-- 投后分析和报告中心读的是投放侧的执行证据，不是 insight_* 表。
-- 只灌经验数据的话这两页永远是空的，所以这里把投放链路一起补上：
-- 计划 → 变更集 → 执行 → 证据 → 指标快照。
-- 第 1 次执行已经有复盘报告，第 2 次没有，用来演示「生成复盘草稿」。
INSERT INTO delivery_plans
  (id, organization_id, project_id, creative_package_id, creative_package_hash, creative_version_id,
   name, objective, budget_cents, start_at, end_at, status, version, created_by, created_at, updated_at)
VALUES
  ('demo_plan_1', @org, @project, 'demo_package_1', 'sha256:demopackage1', 'demo_package_1_v1',
   '春季新品小红书图文', '验证真人出镜封面对点击率的影响', 800000,
   '2026-07-15 00:00:00.000000', '2026-07-22 00:00:00.000000', 'draft', 1, @actor, @now, @now),
  ('demo_plan_2', @org, @project, 'demo_package_2', 'sha256:demopackage2', 'demo_package_2_v1',
   '夏季续投小红书图文', '复用封面结论，验证短标题的稳定性', 600000,
   '2026-07-19 00:00:00.000000', '2026-07-26 00:00:00.000000', 'draft', 1, @actor, @now, @now);

INSERT INTO delivery_change_sets
  (id, organization_id, project_id, plan_id, plan_version, status, risk_level, preflight_notes,
   approved_by, approved_at, version, created_by, created_at, updated_at)
VALUES
  ('demo_changeset_1', @org, @project, 'demo_plan_1', 1, 'executed', 'low',
   JSON_ARRAY('预算在演示上限内', '素材包哈希与计划一致'), @actor, @now, 3, @actor, @now, @now),
  ('demo_changeset_2', @org, @project, 'demo_plan_2', 1, 'executed', 'low',
   JSON_ARRAY('预算在演示上限内', '素材包哈希与计划一致'), @actor, @now, 3, @actor, @now, @now);

INSERT INTO delivery_executions
  (id, organization_id, project_id, change_set_id, status, execution_mode, executed_by, started_at, completed_at)
VALUES
  ('demo_execution_1', @org, @project, 'demo_changeset_1', 'succeeded', 'local_simulation', @actor,
   '2026-07-15 00:00:00.000000', '2026-07-22 00:00:00.000000'),
  ('demo_execution_2', @org, @project, 'demo_changeset_2', 'succeeded', 'local_simulation', @actor,
   '2026-07-19 00:00:00.000000', '2026-07-26 00:00:00.000000');

INSERT INTO delivery_evidence
  (id, organization_id, project_id, execution_id, summary, evidence_mode, reversible, created_at)
VALUES
  ('demo_evidence_1', @org, @project, 'demo_execution_1',
   '本地模拟投放证据：3 条小红书图文素材，投放 7 天，数据来自 preroll-demo/v1 演示数据集。',
   'local_simulation', 1, '2026-07-22 00:00:00.000000'),
  ('demo_evidence_2', @org, @project, 'demo_execution_2',
   '本地模拟投放证据：2 条小红书图文素材沿用真人出镜封面，投放 7 天，数据来自 preroll-demo/v1 演示数据集。',
   'local_simulation', 1, '2026-07-26 00:00:00.000000');

INSERT INTO delivery_metric_snapshots
  (id, organization_id, project_id, execution_id, plan_id, creative_package_id, source, is_simulated,
   dataset_version, currency, window_start, window_end, impressions, clicks, conversions, spend_cents,
   created_by, created_at)
VALUES
  ('demo_metric_1', @org, @project, 'demo_execution_1', 'demo_plan_1', 'demo_package_1',
   'demo_fixture', 1, 'preroll-demo/v1', 'CNY',
   '2026-07-15 00:00:00.000000', '2026-07-22 00:00:00.000000', 128000, 4096, 512, 786400, @actor, @now),
  ('demo_metric_2', @org, @project, 'demo_execution_2', 'demo_plan_2', 'demo_package_2',
   'demo_fixture', 1, 'preroll-demo/v1', 'CNY',
   '2026-07-19 00:00:00.000000', '2026-07-26 00:00:00.000000', 96000, 2880, 317, 592300, @actor, @now);

INSERT INTO insight_reports
  (id, organization_id, project_id, execution_id, delivery_mode, evidence_id, evidence_summary,
   metric_snapshot_id, creative_package_id, is_simulated, dataset_version, status, summary, findings,
   version, created_by, confirmed_by, confirmed_at, created_at, updated_at)
VALUES
  ('demo_report_1', @org, @project, 'demo_execution_1', 'local_simulation', 'demo_evidence_1',
   '本地模拟投放证据：3 条小红书图文素材，投放 7 天，数据来自 preroll-demo/v1 演示数据集。',
   'demo_metric_1', 'demo_package_1', 1, 'preroll-demo/v1', 'confirmed',
   '短标题 + 真人出镜封面的组合，点击率明显高于纯产品图。',
   JSON_ARRAY('真人出镜封面点击率 3.2%，纯产品图 1.4%', '标题超过 18 字后完播率下降', '转化集中在投放第 2-4 天'),
   2, @actor, @actor, @now, @now, @now),
  -- 第二份留在待确认：报告中心的「待确认」视图要有真东西可确认，
  -- 否则那个视图只能证明它自己是空的。
  ('demo_report_2', @org, @project, 'demo_execution_2', 'local_simulation', 'demo_evidence_2',
   '本地模拟投放证据：抖音短视频前贴 2 条素材，投放 7 天，数据来自 preroll-demo/v1 演示数据集。',
   'demo_metric_2', 'demo_package_2', 1, 'preroll-demo/v1', 'draft',
   '前 3 秒露脸的素材完播率更高，但这一轮同时改了字幕样式，两个变量没分开。',
   JSON_ARRAY('露脸组完播率 34%，未露脸组 21%', '同批次改了字幕加粗，效果无法拆分到单一变量', '第 5 天起点击率下滑，疑似素材疲劳'),
   1, @actor, NULL, NULL, @now, @now);

-- 洞察卡九字段（03 §8.1）。card_type 是最要紧的一格：事实 / 统计观察 / 假设 /
-- 建议是四种强度不同的东西，这份演示数据刻意四种都放了一条，好让投前洞察的
-- 「策略证据 / 创意建议 / 风险与反例」三个视图各自拿到真东西，而不是同一批卡
-- 换个标题重排一遍。
INSERT INTO insight_experiences
  (id, organization_id, project_id, lineage_id, revision, supersedes_id, superseded_by_id, report_id,
   source_execution_id, source_evidence_id, source_metric_snapshot_id, conclusion,
   card_type, confidence, recommended_action,
   conditions, counterexamples, applicability, data_basis, content_basis,
   status, status_reason, status_changed_by, status_changed_at, confirmed_by, confirmed_at,
   version, created_by, created_at, updated_at)
VALUES
  -- 版本链：第 1 版已被第 2 版取代，仍留在库里可追溯。
  ('demo_exp_1_r1', @org, @project, 'demo_lineage_1', 1, NULL, 'demo_exp_1_r2', 'demo_report_1',
   'demo_execution_1', 'demo_evidence_1', 'demo_metric_1',
   '小红书图文用真人出镜封面，点击率高于纯产品图。',
   'statistic', 'directional', '',
   JSON_ARRAY('小红书图文', '美妆个护类目'),
   JSON_ARRAY('仅有模拟指标，未经真实平台验证'),
   JSON_OBJECT('channels', JSON_ARRAY('小红书'), 'creative_types', JSON_ARRAY('图文'),
               'objectives', JSON_ARRAY('种草'), 'products', JSON_ARRAY('美妆个护')),
   JSON_OBJECT('asset_count', 3, 'sample_size', 82000, 'metrics', JSON_ARRAY('点击率'),
               'baseline', '同期纯产品图封面'),
   JSON_OBJECT('features', JSON_ARRAY('真人出镜封面'), 'example_asset_versions', JSON_ARRAY('demo_asset_1_v1')),
   'retired', '已被第 2 版取代。', @actor, @now, @actor, @now, 4, @actor, @now, @now),

  ('demo_exp_1_r2', @org, @project, 'demo_lineage_1', 2, 'demo_exp_1_r1', NULL, 'demo_report_1',
   'demo_execution_1', 'demo_evidence_1', 'demo_metric_1',
   '小红书图文用真人出镜封面配 18 字以内短标题，点击率约为纯产品图的两倍。',
   'statistic', 'sufficient', '',
   JSON_ARRAY('小红书图文', '美妆个护类目', '单条预算 5000 元以内'),
   JSON_ARRAY('仅有模拟指标，真实投放前需复核', '标题超过 18 字时结论不成立'),
   JSON_OBJECT('channels', JSON_ARRAY('小红书'), 'creative_types', JSON_ARRAY('图文'),
               'objectives', JSON_ARRAY('种草'), 'products', JSON_ARRAY('美妆个护'),
               'time_range_note', '2026 年上半年的观察窗口'),
   JSON_OBJECT('asset_count', 3, 'sample_size', 128000, 'metrics', JSON_ARRAY('点击率', '完播率'),
               'baseline', '同期纯产品图封面'),
   JSON_OBJECT('features', JSON_ARRAY('真人出镜封面', '短标题'),
               'example_asset_versions', JSON_ARRAY('demo_asset_1_v2')),
   'confirmed', '补充了标题长度这一关键条件后确认。', @actor, @now, @actor, @now, 3, @actor, @now, @now),

  -- 待确认：刚沉淀出来，还没有人负责地认可它。
  ('demo_exp_2_r1', @org, @project, 'demo_lineage_2', 1, NULL, NULL, 'demo_report_1',
   'demo_execution_1', 'demo_evidence_1', 'demo_metric_1',
   '投放第 2-4 天是转化高峰，预算应该前置。',
   'hypothesis', 'low_sample', '',
   JSON_ARRAY('7 天以内的短周期投放'),
   JSON_ARRAY('只观察了一次投放，样本不足'),
   JSON_OBJECT('channels', JSON_ARRAY('小红书'), 'objectives', JSON_ARRAY('转化')),
   JSON_OBJECT('asset_count', 3, 'sample_size', 9600, 'metrics', JSON_ARRAY('转化数')),
   JSON_OBJECT('note', '只看了投放节奏，没有拆到素材特征。'),
   'pending', '从复盘报告沉淀。', @actor, @now, NULL, NULL, 1, @actor, @now, @now),

  -- 待复审：曾经被确认，后来有人对它提出了疑问。
  ('demo_exp_3_r1', @org, @project, 'demo_lineage_3', 1, NULL, NULL, 'demo_report_1',
   'demo_execution_1', 'demo_evidence_1', 'demo_metric_1',
   '素材上线后 48 小时内不要调整出价。',
   'recommendation', 'directional', '冷启动 48 小时内锁定出价，只调预算不调价。',
   JSON_ARRAY('小红书信息流'),
   JSON_ARRAY('大促期间不适用'),
   JSON_OBJECT('channels', JSON_ARRAY('小红书'), 'creative_types', JSON_ARRAY('信息流')),
   JSON_OBJECT('asset_count', 3, 'sample_size', 41000, 'metrics', JSON_ARRAY('转化成本')),
   JSON_OBJECT('note', '结论来自投放节奏，与素材特征无关。'),
   'needs_review', '平台近期调整了冷启动机制，该结论需要重新验证。', @actor, @now, @actor, @now, 3, @actor, @now, @now),

  -- 「事实」：数据里直接读得出来的，不含推断。投前洞察的策略证据视图靠这类卡。
  ('demo_exp_4_r1', @org, @project, 'demo_lineage_4', 1, NULL, NULL, 'demo_report_1',
   'demo_execution_1', 'demo_evidence_1', 'demo_metric_1',
   '抖音短视频中，前 3 秒出现人脸的素材完播率为 34%，未出现人脸的为 21%。',
   'fact', 'sufficient', '',
   JSON_ARRAY('抖音短视频', '15 秒以内'),
   JSON_ARRAY('母婴类目未观察到同样差距'),
   JSON_OBJECT('channels', JSON_ARRAY('抖音'), 'creative_types', JSON_ARRAY('短视频'),
               'objectives', JSON_ARRAY('曝光'), 'audiences', JSON_ARRAY('18-34 女性')),
   JSON_OBJECT('asset_count', 24, 'sample_size', 316000, 'metrics', JSON_ARRAY('完播率'),
               'baseline', '同期未出现人脸的同类素材'),
   JSON_OBJECT('features', JSON_ARRAY('开头露脸', '短标题'),
               'example_asset_versions', JSON_ARRAY('demo_asset_2_v1', 'demo_asset_3_v1')),
   'confirmed', '数据窗口与素材标注均已复核。', @actor, @now, @actor, @now, 2, @actor, @now, @now),

  -- 「建议」：明确要人去做什么，所以 recommended_action 不能为空。
  ('demo_exp_5_r1', @org, @project, 'demo_lineage_5', 1, NULL, NULL, 'demo_report_1',
   'demo_execution_1', 'demo_evidence_1', 'demo_metric_1',
   '抖音短视频的字幕应加粗描边，否则移动端小屏读不清。',
   'recommendation', 'directional', '短视频交付前统一加粗描边字幕，字号不小于 36px。',
   JSON_ARRAY('抖音短视频', '竖版 9:16'),
   JSON_ARRAY('纯口播且无信息点的素材加字幕反而降低完播'),
   JSON_OBJECT('channels', JSON_ARRAY('抖音'), 'creative_types', JSON_ARRAY('短视频'),
               'objectives', JSON_ARRAY('转化')),
   JSON_OBJECT('asset_count', 18, 'sample_size', 204000, 'metrics', JSON_ARRAY('完播率', '点击率')),
   JSON_OBJECT('features', JSON_ARRAY('字幕加粗'), 'example_asset_versions', JSON_ARRAY('demo_asset_2_v2')),
   'confirmed', '创意与投放双方都认可这条交付要求。', @actor, @now, @actor, @now, 2, @actor, @now, @now),

  -- 存在混杂：结论看起来成立，但同期换过受众包，分不清是哪一个造成的。
  -- 这条是「风险与反例」视图要突出的那类卡。
  ('demo_exp_6_r1', @org, @project, 'demo_lineage_6', 1, NULL, NULL, 'demo_report_1',
   'demo_execution_1', 'demo_evidence_1', 'demo_metric_1',
   '促销价签出现在首帧时，点击率提升约 15%。',
   'statistic', 'confounded', '',
   JSON_ARRAY('抖音短视频', '大促期间'),
   JSON_ARRAY('同期更换过受众包，提升可能来自受众而非价签', '非大促期间未复现'),
   JSON_OBJECT('channels', JSON_ARRAY('抖音'), 'creative_types', JSON_ARRAY('短视频'),
               'objectives', JSON_ARRAY('转化')),
   JSON_OBJECT('asset_count', 11, 'sample_size', 87000, 'metrics', JSON_ARRAY('点击率'),
               'baseline', '大促前两周同素材'),
   JSON_OBJECT('features', JSON_ARRAY('促销价签', '开头露脸'),
               'example_asset_versions', JSON_ARRAY('demo_asset_4_v1')),
   'confirmed', '保留结论但标注混杂，供下一轮实验设计参考。', @actor, @now, @actor, @now, 2, @actor, @now, @now);

INSERT INTO insight_experience_audits
  (id, organization_id, project_id, experience_id, from_status, to_status, reason, actor_id, created_at)
VALUES
  ('demo_audit_01', @org, @project, 'demo_exp_1_r1', '', 'pending', '从复盘报告沉淀。', @actor, @now),
  ('demo_audit_02', @org, @project, 'demo_exp_1_r1', 'pending', 'confirmed', '', @actor, @now),
  ('demo_audit_03', @org, @project, 'demo_exp_1_r2', '', 'pending', '补充标题长度条件后修订。', @actor, @now),
  ('demo_audit_04', @org, @project, 'demo_exp_1_r2', 'pending', 'confirmed', '', @actor, @now),
  ('demo_audit_05', @org, @project, 'demo_exp_1_r1', 'confirmed', 'retired', '已被第 2 版取代。', @actor, @now),
  ('demo_audit_06', @org, @project, 'demo_exp_2_r1', '', 'pending', '从复盘报告沉淀。', @actor, @now),
  ('demo_audit_07', @org, @project, 'demo_exp_3_r1', '', 'pending', '从复盘报告沉淀。', @actor, @now),
  ('demo_audit_08', @org, @project, 'demo_exp_3_r1', 'pending', 'confirmed', '', @actor, @now),
  ('demo_audit_09', @org, @project, 'demo_exp_3_r1', 'confirmed', 'needs_review',
   '平台近期调整了冷启动机制，该结论需要重新验证。', @actor, @now);

-- 引用记录挂在已失效的第 1 版上：结论失效了，但当初照着它做的投放依然查得到依据。
INSERT INTO insight_experience_references
  (id, organization_id, project_id, experience_id, consumer_kind, consumer_id, outcome, note,
   version, created_by, created_at, updated_at)
VALUES
  ('demo_ref_1', @org, @project, 'demo_exp_1_r1', 'strategy', 'demo_strategy_1', 'adopted',
   '春季新品策略直接采纳了封面建议。', 1, @actor, @now, @now),
  ('demo_ref_2', @org, @project, 'demo_exp_1_r2', 'strategy', 'demo_strategy_2', 'modified',
   '改成 22 字标题后使用，效果待观察。', 1, @actor, @now, @now);
