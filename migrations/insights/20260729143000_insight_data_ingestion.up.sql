-- 数据接入的地基（建设顺序第 1 批的前提，20 §4.1 标为 P0 核心闭环）。
--
-- insight_data_sources   数据源与其口径：平台、账户、接入方式、时区币种、归因窗口、质量状态、数据截止时间
-- insight_import_batches 每一次同步或导入的批次：范围、行数、错误、幂等哈希、更正关系
-- insight_metric_daily   Canonical 层日粒度指标：平台对象 × 日期 × 归因窗口 × 口径版本
--
-- 契约来自 10-ad-data-connectors.md：
--   §3 实体（DataSource / ImportBatch / MetricDefinition）
--   §4 分层（Raw 原字段留痕，Canonical 供分析）
--   §6 指标（金额定点数 + 币种；时区随账户；派生指标不落库，分母为零时算不出来就是算不出来）
--   §7 幂等（平台主键 + 数据日期 + 归因窗口 + Schema 版本）
--   §9 安全（凭据只存引用，密文由密钥服务持有，本库不落任何令牌）
--   §11 质量状态七值
--
-- 边界说明：doc10 描述的是一个独立的 Connector 平台层（migrations/connector/ 已占位但还是空的），
-- 负责 OAuth、Raw 分层、SyncCursor、限流退避。本文件建的是**素材洞察这一侧**的接入与 Canonical 指标，
-- 表前缀 insight_ 即为此意。Connector 平台层建成后，本层改为消费它的输出，
-- 表结构按 doc10 §3 的字段命名保持可迁移。
--
-- 指标与素材的连接不在本文件：走 insight_asset_mappings 的
-- (platform, platform_object_kind, platform_object_id)，未匹配的对象留在待处理队列，
-- 照样有指标、只是暂时归不到素材（AM-003、doc10 §5）。

CREATE TABLE insight_data_sources (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  platform VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  account_label VARCHAR(255) NOT NULL DEFAULT '',
  account_ref VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',

  -- doc10 §2 五种数据源类型。
  ingest_mode VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  -- doc10 §9：这里存的是密钥服务里的引用键，不是凭据本身。
  -- 任何令牌、API Key、刷新令牌都不得写进本列。
  credential_ref VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',

  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'draft',

  -- doc10 §11 七种质量状态。任何引用本数据源的洞察都必须把它显示出来（§11 末）。
  quality_status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'healthy',
  quality_note VARCHAR(1000) NOT NULL DEFAULT '',

  -- doc10 §6：日粒度按平台账户时区聚合，金额带币种，归因窗口是口径的一部分。
  time_zone VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'Asia/Shanghai',
  currency CHAR(3) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'CNY',
  attribution_window VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'platform_default',
  metric_schema_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'v1',

  -- doc10 §8：平台报表列名 → canonical 指标名。为空表示还没配字段映射，此时不能导入。
  field_mapping JSON NULL,

  -- 新鲜度（AM-002）：数据截到哪一天，以及上一次真正拉到数据是什么时候。
  data_through DATE NULL,
  last_synced_at DATETIME(6) NULL,

  version BIGINT NOT NULL DEFAULT 1,
  created_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,

  UNIQUE KEY uq_insight_data_sources_scope (organization_id, id),
  UNIQUE KEY uq_insight_data_sources_account (organization_id, project_id, platform, account_ref),
  KEY idx_insight_data_sources_project (organization_id, project_id, updated_at),
  KEY idx_insight_data_sources_quality (organization_id, project_id, quality_status, updated_at),
  CONSTRAINT fk_insight_data_sources_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT chk_insight_data_sources_version CHECK (version > 0),
  CONSTRAINT chk_insight_data_sources_platform CHECK (platform IN ('douyin', 'kuaishou', 'xiaohongshu', 'wechat', 'tencent_ads', 'other')),
  CONSTRAINT chk_insight_data_sources_ingest_mode CHECK (ingest_mode IN ('api', 'service_account', 'file_import', 'computer_use', 'business')),
  CONSTRAINT chk_insight_data_sources_status CHECK (status IN ('draft', 'active', 'paused', 'revoked')),
  CONSTRAINT chk_insight_data_sources_quality CHECK (quality_status IN ('healthy', 'delayed', 'partial', 'mapping_incomplete', 'tracking_broken', 'reconciling', 'blocked')),
  -- 非 healthy 必须写清楚为什么，否则页面上只剩一个没人能解释的黄灯。
  CONSTRAINT chk_insight_data_sources_quality_note CHECK (quality_status = 'healthy' OR quality_note <> '')
);

-- doc10 §3 ImportBatch / §7 同步与回补 / §8 文件导入。
-- 一次同步、一次文件导入、一次历史回补、一次更正，都是这张表的一行。
CREATE TABLE insight_import_batches (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  data_source_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  kind VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'pending',

  source_label VARCHAR(255) NOT NULL DEFAULT '',
  window_start DATE NULL,
  window_end DATE NULL,

  -- doc10 §8：相同文件哈希 + 相同导入范围默认阻止重复。
  -- 允许为 NULL：周期性 API 同步没有文件哈希，NULL 不进唯一键，因此可以反复跑。
  content_hash VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,

  -- doc10 §7：每批记录返回数量、丢弃/错误数量。
  requested_rows INT NOT NULL DEFAULT 0,
  accepted_rows INT NOT NULL DEFAULT 0,
  rejected_rows INT NOT NULL DEFAULT 0,
  error_summary VARCHAR(1000) NOT NULL DEFAULT '',
  errors JSON NULL,

  -- doc10 §8：确需重导时创建更正批次，指向被更正的那一批，历史批次本身不改写。
  corrects_batch_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,

  started_at DATETIME(6) NULL,
  finished_at DATETIME(6) NULL,

  version BIGINT NOT NULL DEFAULT 1,
  created_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,

  UNIQUE KEY uq_insight_import_batches_scope (organization_id, id),
  UNIQUE KEY uq_insight_import_batches_dedup (organization_id, data_source_id, content_hash, window_start, window_end),
  KEY idx_insight_import_batches_project (organization_id, project_id, created_at),
  KEY idx_insight_import_batches_source (organization_id, data_source_id, created_at),
  KEY idx_insight_import_batches_status (organization_id, project_id, status, created_at),
  CONSTRAINT fk_insight_import_batches_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT fk_insight_import_batches_source FOREIGN KEY (organization_id, data_source_id) REFERENCES insight_data_sources(organization_id, id),
  CONSTRAINT fk_insight_import_batches_corrects FOREIGN KEY (organization_id, corrects_batch_id) REFERENCES insight_import_batches(organization_id, id),
  CONSTRAINT chk_insight_import_batches_version CHECK (version > 0),
  CONSTRAINT chk_insight_import_batches_kind CHECK (kind IN ('sync', 'backfill', 'file', 'correction')),
  CONSTRAINT chk_insight_import_batches_status CHECK (status IN ('pending', 'running', 'succeeded', 'partial', 'failed')),
  CONSTRAINT chk_insight_import_batches_rows CHECK (requested_rows >= 0 AND accepted_rows >= 0 AND rejected_rows >= 0),
  -- 窗口要么都给要么都不给，且不能倒序。
  CONSTRAINT chk_insight_import_batches_window CHECK ((window_start IS NULL AND window_end IS NULL) OR (window_start IS NOT NULL AND window_end IS NOT NULL AND window_start <= window_end)),
  -- 只有更正批次能指向别的批次。
  CONSTRAINT chk_insight_import_batches_correction CHECK (kind = 'correction' OR corrects_batch_id IS NULL),
  -- 失败与部分成功必须留下原因，否则「同步记录」页只剩一排红字没人知道发生了什么（20 §4.1 错误置顶）。
  CONSTRAINT chk_insight_import_batches_error CHECK (status NOT IN ('failed', 'partial') OR error_summary <> '')
);

-- Canonical 层日粒度指标（doc10 §4.3）。
--
-- 只存平台给的事实计数与金额，不存 CTR/CVR/CPA/ROAS：
-- doc10 §6 要求派生指标由版本化公式计算、分母为零时返回"不可用"而不是 0，
-- 一旦把比率也落库，"分母为零"这件事就在存储层被抹平了。
CREATE TABLE insight_metric_daily (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  data_source_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  import_batch_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  -- 与 insight_asset_mappings 同名同义，join 用这三列。
  platform VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  platform_object_kind VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  platform_object_id VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  -- doc10 §6：日粒度按平台账户时区聚合，这里存的是聚合后的当地日期。
  stat_date DATE NOT NULL,
  attribution_window VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  metric_schema_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  currency CHAR(3) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  impressions BIGINT NOT NULL DEFAULT 0,
  clicks BIGINT NOT NULL DEFAULT 0,
  conversions BIGINT NOT NULL DEFAULT 0,
  video_views BIGINT NOT NULL DEFAULT 0,
  video_completions BIGINT NOT NULL DEFAULT 0,
  -- doc10 §6：金额用定点数，不用浮点。以「分」为最小单位存整数，
  -- 与 delivery 的 RawMetrics.SpendCents 同一表示，跨系统对账时不需要换算。
  spend_cents BIGINT NOT NULL DEFAULT 0,
  revenue_cents BIGINT NOT NULL DEFAULT 0,

  -- doc10 §4.1 / §12.5：统一指标不得覆盖平台原始事实，原字段随行留痕以便回溯。
  raw JSON NULL,

  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,

  UNIQUE KEY uq_insight_metric_daily_scope (organization_id, id),
  -- doc10 §7：平台主键 + 数据日期 + 归因窗口 + Schema 版本 → 同一批重跑不产生重复。
  UNIQUE KEY uq_insight_metric_daily_fact (organization_id, project_id, platform, platform_object_kind, platform_object_id, stat_date, attribution_window, metric_schema_version),
  KEY idx_insight_metric_daily_window (organization_id, project_id, stat_date),
  KEY idx_insight_metric_daily_object (organization_id, project_id, platform, platform_object_id, stat_date),
  KEY idx_insight_metric_daily_batch (organization_id, import_batch_id),
  CONSTRAINT fk_insight_metric_daily_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT fk_insight_metric_daily_source FOREIGN KEY (organization_id, data_source_id) REFERENCES insight_data_sources(organization_id, id),
  CONSTRAINT fk_insight_metric_daily_batch FOREIGN KEY (organization_id, import_batch_id) REFERENCES insight_import_batches(organization_id, id),
  CONSTRAINT chk_insight_metric_daily_kind CHECK (platform_object_kind IN ('creative', 'ad')),
  CONSTRAINT chk_insight_metric_daily_counts CHECK (impressions >= 0 AND clicks >= 0 AND conversions >= 0 AND video_views >= 0 AND video_completions >= 0),
  CONSTRAINT chk_insight_metric_daily_money CHECK (spend_cents >= 0 AND revenue_cents >= 0),
  -- 完播不可能多于播放；点击不可能多于展示。数据一旦违反这两条，是口径错了而不是效果好。
  CONSTRAINT chk_insight_metric_daily_video CHECK (video_completions <= video_views),
  CONSTRAINT chk_insight_metric_daily_clicks CHECK (clicks <= impressions)
);
