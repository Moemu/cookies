-- 分析素材库与内容分析的地基（建设顺序第 1、2 批）。
--
-- insight_assets          素材索引与稳定版本 ID（AM-001）、内容类型（AM-004）、分析状态链（PRD §11.1）
-- insight_asset_mappings  平台创意/广告对象与素材版本的映射，未命中即待处理队列（AM-003）
-- insight_asset_features  按类型提取的内容特征（AM-005），AI 推断与人工结论分行存储、互不覆盖（AM-006、PRD §14）
--
-- 素材类型取值与特征字段的权威定义在 internal/systems/insights/features.go，
-- 由 PRD §5 逐条转录。数据库只约束类型枚举，不复制字段清单，避免两处漂移。

CREATE TABLE insight_assets (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  -- AM-001：稳定素材与版本 ID，不依赖文件名。lineage_id 串起同一素材的多个版本。
  lineage_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  revision INT NOT NULL DEFAULT 1,

  title VARCHAR(255) NOT NULL,

  -- AM-001 三种来源：创意模块产物、上传文件、外部引用。
  source_kind VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_ref VARCHAR(512) NOT NULL DEFAULT '',
  source_job_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,

  -- PRD §14：素材二进制与预览统一走媒体资产平台的 Asset ID/Version。
  -- 两列同时为 NULL 表示尚未登记到媒体资产平台（外部引用常见）；一旦声明引用，
  -- 复合外键即强制该 Asset 版本真实存在，杜绝指向不存在版本的悬空引用。
  platform_asset_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  platform_asset_version BIGINT NULL,

  -- AM-004：六类之一；空串表示待识别，此时不允许写入任何特征。
  asset_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  asset_type_source VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  asset_type_confidence VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',

  -- PRD §11.1 分析状态链：待数据 → 待匹配 → 可分析 → 分析中 → 待确认 → 已确认/待复审/已失效
  analysis_status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'awaiting_data',
  analysis_status_reason VARCHAR(1000) NOT NULL DEFAULT '',
  analysis_status_changed_at DATETIME(6) NULL,

  version BIGINT NOT NULL DEFAULT 1,
  created_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,

  UNIQUE KEY uq_insight_assets_scope (organization_id, id),
  UNIQUE KEY uq_insight_assets_lineage (organization_id, lineage_id, revision),
  KEY idx_insight_assets_project (organization_id, project_id, updated_at),
  KEY idx_insight_assets_status (organization_id, project_id, analysis_status, updated_at),
  KEY idx_insight_assets_type (organization_id, project_id, asset_type, updated_at),
  KEY idx_insight_assets_platform (organization_id, platform_asset_id, platform_asset_version),
  CONSTRAINT fk_insight_assets_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT fk_insight_assets_platform_version FOREIGN KEY (organization_id, platform_asset_id, platform_asset_version) REFERENCES asset_versions(organization_id, asset_id, version),
  CONSTRAINT chk_insight_assets_version CHECK (version > 0),
  CONSTRAINT chk_insight_assets_revision CHECK (revision > 0),
  CONSTRAINT chk_insight_assets_source_kind CHECK (source_kind IN ('creative', 'upload', 'external')),
  CONSTRAINT chk_insight_assets_type CHECK (asset_type IN ('', 'xiaohongshu_note', 'wechat_article', 'brand_ad', 'digital_human_ad', 'preroll_ad', 'hit_replica_ad')),
  CONSTRAINT chk_insight_assets_type_source CHECK (asset_type_source IN ('', 'ai', 'human')),
  CONSTRAINT chk_insight_assets_type_confidence CHECK (asset_type_confidence IN ('', 'low', 'medium', 'high')),
  -- 类型未识别时不得声明来源或置信度，避免出现"没有类型却有类型置信度"的记录。
  CONSTRAINT chk_insight_assets_type_pair CHECK ((asset_type = '' AND asset_type_source = '') OR (asset_type <> '' AND asset_type_source <> '')),
  -- AI 识别必须带置信级别（PRD §5 末）；人工识别不需要。
  CONSTRAINT chk_insight_assets_type_ai_confidence CHECK (asset_type_source <> 'ai' OR asset_type_confidence <> ''),
  CONSTRAINT chk_insight_assets_analysis_status CHECK (analysis_status IN ('awaiting_data', 'awaiting_match', 'analysable', 'analysing', 'pending_confirmation', 'confirmed', 'needs_review', 'retired')),
  -- 媒体资产引用要么两列都给，要么都不给。
  CONSTRAINT chk_insight_assets_platform_pair CHECK ((platform_asset_id IS NULL AND platform_asset_version IS NULL) OR (platform_asset_id IS NOT NULL AND platform_asset_version IS NOT NULL))
);

-- AM-003 关系匹配：平台创意/广告 ID 与 cookies 素材版本对应，无法匹配的进入待处理队列。
CREATE TABLE insight_asset_mappings (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  platform VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  platform_object_kind VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  platform_object_id VARCHAR(191) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  platform_object_name VARCHAR(255) NOT NULL DEFAULT '',

  -- NULL 即"待匹配"队列的一行：平台有这个对象，但还不知道对应哪个素材版本。
  insight_asset_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'unmatched',
  match_source VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  matched_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NULL,
  matched_at DATETIME(6) NULL,
  note VARCHAR(1000) NOT NULL DEFAULT '',

  version BIGINT NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,

  UNIQUE KEY uq_insight_asset_mappings_scope (organization_id, id),
  UNIQUE KEY uq_insight_asset_mappings_object (organization_id, project_id, platform, platform_object_kind, platform_object_id),
  KEY idx_insight_asset_mappings_status (organization_id, project_id, status, updated_at),
  KEY idx_insight_asset_mappings_asset (organization_id, insight_asset_id),
  CONSTRAINT fk_insight_asset_mappings_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT fk_insight_asset_mappings_asset FOREIGN KEY (organization_id, insight_asset_id) REFERENCES insight_assets(organization_id, id),
  CONSTRAINT chk_insight_asset_mappings_version CHECK (version > 0),
  CONSTRAINT chk_insight_asset_mappings_kind CHECK (platform_object_kind IN ('creative', 'ad')),
  CONSTRAINT chk_insight_asset_mappings_status CHECK (status IN ('unmatched', 'matched', 'ignored')),
  CONSTRAINT chk_insight_asset_mappings_source CHECK (match_source IN ('', 'auto', 'human')),
  -- 已匹配必须指向素材且说明是自动还是人工匹配；未匹配不得留下素材指针。
  CONSTRAINT chk_insight_asset_mappings_matched CHECK ((status = 'matched' AND insight_asset_id IS NOT NULL AND match_source <> '') OR (status <> 'matched' AND insight_asset_id IS NULL))
);

-- AM-005 特征提取 / AM-006 特征确认。
--
-- 关键设计：唯一键包含 source，因此同一素材同一特征的「AI 推断」与「人工结论」
-- 是两行，各自独立存在。后台重新提取只会覆盖 source='ai' 的行，人工确认的行
-- 永远不会被后台写回覆盖（AM-006），这也是 PRD §14 四层数据分离在本表的落点。
CREATE TABLE insight_asset_features (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  insight_asset_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  -- 冗余素材类型，使"特征键必须属于该类型的特征体系"这条约束在本表可直接校验，
  -- 也让特征矩阵查询不必回表。写入时由服务层与 insight_assets.asset_type 对齐。
  asset_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  feature_key VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  value_kind VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  value_text VARCHAR(2000) NULL,
  value_number DECIMAL(18, 4) NULL,
  value_bool TINYINT(1) NULL,
  value_terms JSON NULL,

  -- 'ai' = AI 推断层，'human' = 人工结论层。
  source VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  confidence VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  review_state VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'pending',

  -- 可追溯（PRD §MVP⑫）：结论要能回到 Skill 与算法版本。
  skill_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  skill_version VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  extracted_at DATETIME(6) NULL,

  version BIGINT NOT NULL DEFAULT 1,
  created_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,

  UNIQUE KEY uq_insight_asset_features_scope (organization_id, id),
  UNIQUE KEY uq_insight_asset_features_layer (organization_id, insight_asset_id, feature_key, source),
  KEY idx_insight_asset_features_asset (organization_id, insight_asset_id, feature_key),
  KEY idx_insight_asset_features_matrix (organization_id, project_id, asset_type, feature_key),
  KEY idx_insight_asset_features_review (organization_id, project_id, review_state, updated_at),
  CONSTRAINT fk_insight_asset_features_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT fk_insight_asset_features_asset FOREIGN KEY (organization_id, insight_asset_id) REFERENCES insight_assets(organization_id, id),
  CONSTRAINT chk_insight_asset_features_version CHECK (version > 0),
  CONSTRAINT chk_insight_asset_features_type CHECK (asset_type IN ('xiaohongshu_note', 'wechat_article', 'brand_ad', 'digital_human_ad', 'preroll_ad', 'hit_replica_ad')),
  CONSTRAINT chk_insight_asset_features_kind CHECK (value_kind IN ('text', 'tags', 'enum', 'enum_multi', 'number', 'bool', 'duration_seconds')),
  CONSTRAINT chk_insight_asset_features_source CHECK (source IN ('ai', 'human')),
  CONSTRAINT chk_insight_asset_features_confidence CHECK (confidence IN ('', 'low', 'medium', 'high')),
  CONSTRAINT chk_insight_asset_features_review_state CHECK (review_state IN ('pending', 'confirmed', 'rejected')),
  -- PRD §5 末：自动特征必须标识为 AI 提取并保存置信级别。
  CONSTRAINT chk_insight_asset_features_ai_confidence CHECK (source <> 'ai' OR confidence <> ''),
  -- 人工结论层不带机器置信度，避免把人工判断读成模型输出。
  CONSTRAINT chk_insight_asset_features_human_confidence CHECK (source <> 'human' OR confidence = '')
);
