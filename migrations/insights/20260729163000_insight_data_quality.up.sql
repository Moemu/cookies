-- 数据质量（建设顺序第 3 批，PRD §一级导航「阻止错误数据产生强结论」）。
--
-- 这里只有一张表，而且它**不存问题本身**。
--
-- 原因：质量问题全部可以从已有数据算出来——数据源截止到哪天、窗口里哪天没数据、
-- 有多少平台对象没认领、几个数据源的币种是不是一致。如果再把问题也存一份，
-- 就会出现「表里说有问题、数据里已经修好了」或者反过来的两边不一致，而这恰恰是
-- 一个数据质量模块最不该犯的错。
--
-- 所以：问题由 internal/systems/insights/quality.go 的检测器每次实时算出，
-- 本表只记录**人对某条问题做过什么处置**（认领 / 标记已修 / 判定可忽略 + 理由 + 操作人）。
-- 两者靠 fingerprint 关联，fingerprint 由检测器按「问题类型 + 影响对象 + 窗口」拼出，
-- 定义在 Go 侧（权威源），数据库只当它是个字符串。
--
-- observed_through 是这张表的关键：它记下「处置发生时，这个问题最新一次被观测到是什么时候」。
-- 下次检测如果发现问题的最新观测时间晚于它，说明处置之后问题又发生了，此时自动重新算作
-- 未处理，而不是继续显示「已修复」。没有这一列，任何人点一次「已修复」就能让问题永久消失。

CREATE TABLE insight_quality_dispositions (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  -- 检测器算出的稳定指纹，拼法见 Go 侧 QualityIssue.Fingerprint()。
  -- 同一个问题反复出现时指纹不变，处置记录才跟得住。
  fingerprint VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  -- 冗余问题类型，便于按类型统计处置情况而不必回到检测器重算。
  issue_kind VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  -- 只有这三种。「未处理」不入表——没有记录就是未处理，避免为每个新问题预写一行。
  state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  -- PRD §12 可追溯：处置必须写清依据，后面看到某个数字时得能回溯它凭什么算数。
  note VARCHAR(1000) NOT NULL,

  -- 处置发生时该问题的最新观测时间。晚于它的新观测会让问题重新变成未处理。
  observed_through DATETIME(6) NOT NULL,

  version BIGINT NOT NULL DEFAULT 1,
  decided_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,

  UNIQUE KEY uq_insight_quality_dispositions_scope (organization_id, id),
  -- 一条问题在一个 Project 里只有一条当前处置；再次处置是更新这一行并抬高 version。
  UNIQUE KEY uq_insight_quality_dispositions_fingerprint (organization_id, project_id, fingerprint),
  KEY idx_insight_quality_dispositions_project (organization_id, project_id, state, updated_at),
  KEY idx_insight_quality_dispositions_kind (organization_id, project_id, issue_kind, updated_at),
  CONSTRAINT fk_insight_quality_dispositions_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT chk_insight_quality_dispositions_version CHECK (version > 0),
  CONSTRAINT chk_insight_quality_dispositions_kind CHECK (issue_kind IN ('freshness', 'missing', 'anomaly', 'caliber', 'reconciliation')),
  CONSTRAINT chk_insight_quality_dispositions_state CHECK (state IN ('acknowledged', 'resolved', 'ignored')),
  -- 三种处置都要写理由：认领要说谁在跟、标记已修要说怎么修的、判定可忽略更要说为什么。
  CONSTRAINT chk_insight_quality_dispositions_note CHECK (CHAR_LENGTH(note) > 0)
);
