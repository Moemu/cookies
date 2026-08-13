-- 外部素材单独一张表，不进 assets。
--
-- 素材库归创意创作那边，他们不接受平台外的素材（2026-08-04 确认）。这不是流程
-- 洁癖：共享素材库里的东西是可以被拿去投放的，而外部素材没有授权，混进去之后
-- 没有任何机制拦住它被投出去。
--
-- 洞察这边仍然需要它——「行业里同类素材长什么样」是解释本轮结果时绕不开的参照。
-- 所以它以**证据**的身份存在：只读、有用途声明、有留存期限、到期删原件。
--
-- retention_until 由导入时那个复盘窗口的结束日期 + 90 天算出来，存下来而不是
-- 每次现算：现算的话改一次常量，所有历史素材的到期日一起变，而人是按导入时
-- 告知的期限做的决定。
CREATE TABLE insight_external_assets (
  id              VARCHAR(64)  NOT NULL,
  organization_id VARCHAR(64)  NOT NULL,
  project_id      VARCHAR(64)  NOT NULL,

  title           VARCHAR(255) NOT NULL,
  -- source_note 是「这东西哪来的」，人自己写。不做成下拉：来源千奇百怪，
  -- 硬套选项只会让人全选「其他」，那一栏就废了。
  source_note     VARCHAR(512) NOT NULL DEFAULT '',
  asset_type      VARCHAR(64)  NOT NULL DEFAULT '',

  -- purpose 是用途声明，导入时必须选。它不是分类标签，是一份记录：
  -- 到了要解释「为什么留着这个」的时候，这一栏就是答案。
  purpose         VARCHAR(32)  NOT NULL,
  purpose_note    VARCHAR(512) NOT NULL DEFAULT '',

  -- storage_key 前缀固定为 insights/external/，和平台素材的存储路径物理隔开。
  -- 同一个前缀下的东西迟早会被某个批处理当成同类对待。
  storage_key     VARCHAR(512) NOT NULL DEFAULT '',
  -- original_purged 为 true 表示原件已删、只剩派生物（变量、截图）。
  -- 到期删的是原件不是整行：删整行的话，引用过它的那份复盘就变成了
  -- 「引用了一个不存在的东西」。
  original_purged TINYINT(1)   NOT NULL DEFAULT 0,

  features        JSON         NOT NULL,
  retention_until DATETIME     NOT NULL,

  created_by      VARCHAR(64)  NOT NULL,
  created_at      DATETIME     NOT NULL,
  updated_at      DATETIME     NOT NULL,

  PRIMARY KEY (id),
  KEY idx_external_project (organization_id, project_id, created_at),
  KEY idx_external_retention (retention_until, original_purged)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
