-- 实验中心（03 §7.3 / AM-009；22 §234「先建立 Experiment、Variant 和 SampleCheck 对象」）。
--
-- 投后分析里的「驱动因素」和「素材对比」是事后从已投数据里认出来的对照：一组素材
-- 恰好在某个特征上一致，表现也恰好更好。它排除不掉选择效应——这批素材本来就是被人
-- 挑出来投的，挑的动作本身可能就和表现相关。所以那边的结论最多说到「相关」。
--
-- 这两张表补的是另一半：**变量、分组、样本门槛在看到结果之前定死**。
--
-- 「定死」由 status 落实：draft 时分组随便改，一旦 running 就冻住。少了这个冻结，
-- 「事先登记」只是个说法——谁都可以在看完数据之后，往表现好的那一组里再补两条素材。
--
-- 没有 SampleCheck 表：样本量每次从 insight_metric_facts 现算。
-- 落库会出现「页面显示的样本数是三天前的」，而样本够不够正是这一页唯一要回答的问题。
-- 事先定的那个门槛（min_impressions）才需要持久化，而它在实验行上。

CREATE TABLE insight_experiments (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  title VARCHAR(200) NOT NULL,
  hypothesis VARCHAR(2000) NOT NULL DEFAULT '',

  -- 假设的出处：投前洞察的假设卡「拿去验证」过来时填这里。
  -- 为空表示空白新建，也就是「这条假设没有出处」——留着这个空值，
  -- 实验有结论之后才知道该回头更新哪张卡。
  source_experience_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',

  -- 被测变量。asset_type 决定用哪套特征体系，variable_key 必须是那套里的字段；
  -- 这一条由服务层对着 features.go 校验，数据库这边只保证类型合法。
  asset_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  variable_key VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  variable_label VARCHAR(128) NOT NULL DEFAULT '',

  -- 要求各组之间保持一致的其他特征。不一致不拦，只在入组时给黄牌：
  -- 真实素材很难只差一个变量，硬拦会让实验一个也建不起来；但不说出来，
  -- 结论就会被当成干净的对照。
  controlled_keys JSON NULL,

  -- 事先定的每组最低展示量。事后再定门槛等于允许「凑够了就说显著」，
  -- 所以它一旦开跑就不能改（服务层只在 draft 状态接受写入）。
  min_impressions BIGINT NOT NULL,

  window_start DATETIME(6) NOT NULL,
  window_end DATETIME(6) NOT NULL,

  status VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  -- verdict 是系统按门槛和区间判的，interpretation 是人写的。
  -- 两者分开存：混在一起就分不清哪句话是谁说的，而这正是这个模块最要紧的区分。
  verdict VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  interpretation VARCHAR(2000) NOT NULL DEFAULT '',
  concluded_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  concluded_at DATETIME(6) NULL,

  started_at DATETIME(6) NULL,
  version BIGINT NOT NULL DEFAULT 1,
  created_by VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,

  UNIQUE KEY uq_insight_experiments_scope (organization_id, id),
  KEY idx_insight_experiments_project (organization_id, project_id, status, created_at),
  CONSTRAINT fk_insight_experiments_project FOREIGN KEY (organization_id, project_id) REFERENCES projects(organization_id, id),
  CONSTRAINT chk_insight_experiments_type CHECK (asset_type IN ('xiaohongshu_note', 'wechat_article', 'brand_ad', 'digital_human_ad', 'preroll_ad', 'hit_replica_ad')),
  CONSTRAINT chk_insight_experiments_status CHECK (status IN ('draft', 'running', 'concluded')),
  CONSTRAINT chk_insight_experiments_verdict CHECK (verdict IN ('', 'supported', 'refuted', 'inconclusive')),
  -- 门槛为 0 等于没有门槛，那样这张表就只是个记事本。
  CONSTRAINT chk_insight_experiments_threshold CHECK (min_impressions > 0),
  CONSTRAINT chk_insight_experiments_window CHECK (window_end >= window_start),
  -- 下了结论就必须有人写的解读。只留一个判定，读的人无从知道这条结论该怎么用。
  CONSTRAINT chk_insight_experiments_conclusion CHECK (
    status <> 'concluded' OR (CHAR_LENGTH(interpretation) > 0 AND verdict <> '' AND concluded_at IS NOT NULL)
  )
);

CREATE TABLE insight_experiment_variants (
  id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  project_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  experiment_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,

  name VARCHAR(64) NOT NULL,
  -- 这一组在被测变量上的取值。入组的素材必须和它对得上，对不上直接拦——
  -- 放错组的素材会让整次实验比的都不是登记的那个变量。
  variable_value VARCHAR(200) NOT NULL,
  is_baseline TINYINT(1) NOT NULL DEFAULT 0,

  -- 素材清单直接存在这里，不另建关联表：这份清单只被它自己的实验读，
  -- 也从不按素材反查（「这条素材参加过哪些实验」不是这个模块要回答的问题）。
  -- 另建一张表只会多一次 JOIN 和一套一致性维护。
  asset_ids JSON NULL,
  position INT NOT NULL DEFAULT 0,

  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,

  UNIQUE KEY uq_insight_experiment_variants_scope (organization_id, id),
  -- 同一个实验里两组不能重名，也不能取同一个值——取值一样就不是两组。
  UNIQUE KEY uq_insight_experiment_variants_name (organization_id, experiment_id, name),
  UNIQUE KEY uq_insight_experiment_variants_value (organization_id, experiment_id, variable_value),
  KEY idx_insight_experiment_variants_experiment (organization_id, experiment_id, position),
  CONSTRAINT fk_insight_experiment_variants_experiment FOREIGN KEY (organization_id, experiment_id) REFERENCES insight_experiments(organization_id, id),
  CONSTRAINT chk_insight_experiment_variants_name CHECK (CHAR_LENGTH(name) > 0),
  CONSTRAINT chk_insight_experiment_variants_value CHECK (CHAR_LENGTH(variable_value) > 0)
);
