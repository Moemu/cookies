-- 分析任务留痕（03 §113 AssetAnalysisRun、§304 可重放、§344 验收 12）。
--
-- 这张表存在的理由只有一条：**任一分析结论能回溯到素材、数据截止时间、指标和
-- Skill 版本**（§344 验收 12）。特征表里每条 AI 特征只带 skill_id / skill_version，
-- 那回答不了「这次提取用的是哪个模型、哪一版提示词、输入是什么、耗时多久、
-- 有没有出错」。没有这张表，模型换了一版之后，库里新旧两批特征混在一起，
-- 谁也说不清哪条是哪一版提出来的——而特征是后面所有对比结论的地基。
--
-- 也是 §310 运营指标（分析任务成功率、P95 时长）的唯一数据来源。
--
-- **一次运行一行，成功失败都留。** 只记成功的话，成功率永远是 100%。
--
-- 关于输入快照：本表存 input_hash（发给模型的完整载荷的 sha256）和
-- input_summary（指针与规模，不含正文），**不存输入全文**。
-- 依据 09 §7「日志默认不记录内容全文和凭据」。代价要说清楚：
-- 这让「同样的输入是否产生同样的结果」可判定（比哈希），但不支持凭本表
-- 逐字重建当时的输入——素材正文另存于 insight_assets 及其来源系统。

CREATE TABLE insight_analysis_runs (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  -- 目前只有一种：从素材内容提取特征。后面接入指标类分析时在这里加值，
  -- 而不是另建一张表——运营面要的是「所有分析任务」的成功率，不是分门别类的。
  run_kind VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  insight_asset_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  -- 运行当时的素材类型快照。素材类型事后可以被人改（AM-002），改了以后
  -- 这次运行用的是哪套特征体系就查不出来了——而输出格式正是由它生成的。
  asset_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  status VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  -- 方法：Skill 与提示词。content_hash 让「Skill 版本号没动但内容改了」也能被发现。
  skill_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  skill_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  skill_content_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  prompt_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',

  -- 模型与路由。model_version 是供应商实际返回的那一版，不是我们请求的别名——
  -- 别名（如 default）背后换了模型时，只有它能把两批结果区分开。
  provider_code VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  model_alias VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  model_version VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  route_revision_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  response_mode VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  -- 'model' = 真调了模型；'template' = 没有可用供应商时的兜底产出。
  -- 这两者绝不能混着统计成功率，兜底路径的成功率是恒定的 100%。
  generation_mode VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',

  -- 输入指纹与规模。全文不入表，理由见表头。
  input_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  input_summary JSON NULL,

  -- 结果指纹：同样输入 + 同样方法 → 同样 result_hash，可重放（§304）。
  result_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  -- 写入了几条特征，以及被丢掉的字段（越界 / 空值）。丢弃必须留痕：
  -- 静悄悄少几条会被读成「模型没看出来」，而真实原因是格式没对上。
  feature_count INT NOT NULL DEFAULT 0,
  dropped_fields JSON NULL,

  -- 指标类分析的数据截止时间（§304）。内容特征提取不读投放数据，因此为 NULL；
  -- 这个 NULL 是有含义的：它说明这次运行的结论与投放数据无关。
  data_through DATETIME(6) NULL,

  prompt_tokens INT NOT NULL DEFAULT 0,
  completion_tokens INT NOT NULL DEFAULT 0,
  latency_ms INT NOT NULL DEFAULT 0,

  -- 失败原因。message 是给人看的，不放模型返回的原文（09 §7）。
  error_code VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  error_message VARCHAR(1000) NOT NULL DEFAULT '',

  started_at DATETIME(6) NOT NULL,
  finished_at DATETIME(6) NULL,
  created_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,

  UNIQUE KEY uq_insight_analysis_runs_scope (organization_id, id),
  -- 素材详情页的「数据与方法」按时间倒序读这一条链（03 §82）。
  KEY idx_insight_analysis_runs_asset (organization_id, insight_asset_id, started_at),
  -- 运营面按项目算成功率与 P95 时长（§310）。
  KEY idx_insight_analysis_runs_project (organization_id, project_id, run_kind, started_at),
  KEY idx_insight_analysis_runs_status (organization_id, project_id, status, started_at),
  CONSTRAINT fk_insight_analysis_runs_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT fk_insight_analysis_runs_asset FOREIGN KEY (organization_id, insight_asset_id) REFERENCES insight_assets(organization_id, id),
  CONSTRAINT chk_insight_analysis_runs_kind CHECK (run_kind IN ('feature_extraction')),
  CONSTRAINT chk_insight_analysis_runs_type CHECK (asset_type IN ('xiaohongshu_note', 'wechat_article', 'brand_ad', 'digital_human_ad', 'preroll_ad', 'hit_replica_ad')),
  CONSTRAINT chk_insight_analysis_runs_status CHECK (status IN ('running', 'succeeded', 'failed')),
  CONSTRAINT chk_insight_analysis_runs_mode CHECK (generation_mode IN ('', 'model', 'template')),
  -- 结束了就必须有结束时间；还在跑就不能有。否则 P95 时长会算进没跑完的任务。
  CONSTRAINT chk_insight_analysis_runs_finished CHECK ((status = 'running' AND finished_at IS NULL) OR (status <> 'running' AND finished_at IS NOT NULL)),
  -- 失败必须写清原因，否则运营面只能看到一个数字，查不下去。
  CONSTRAINT chk_insight_analysis_runs_error CHECK (status <> 'failed' OR CHAR_LENGTH(error_message) > 0),
  -- 成功不留错误信息，避免把上一次重试的错误读成本次的。
  CONSTRAINT chk_insight_analysis_runs_success CHECK (status <> 'succeeded' OR (error_code = '' AND error_message = ''))
);
