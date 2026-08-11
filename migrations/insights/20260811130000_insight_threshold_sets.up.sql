-- 判定阈值从代码常量改成可写配置。
--
-- 这些数字直接决定三档结论怎么判——多少曝光算「充分」、几天才给趋势、几条素材
-- 才谈驱动因素。不能改，等于这个模块的判断标准不可调；而不同行业、不同渠道的
-- 合理门槛本来就不一样，写死一套是错的。
--
-- **只增版本，从不原地改。** 每次保存插一行新的，旧行永远留着。没有这一条，
-- 改完阈值之后所有历史结论都说不清是按什么标准算出来的——而经验库是「新增版本
-- 不覆盖、历史可审计」的，一批说不清依据的经验会永远挂在账上，说不清是对是错。
--
-- 值列全部可空。**NULL 表示「没人调过这一格，用代码默认值」**，不是 0。
-- 存成 0 的话，将来改了代码默认值，那些从没被调过的部署不会跟着走，
-- 而它们本来应该跟着走。
CREATE TABLE insight_threshold_sets (
  id              VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  organization_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  -- version 在组织内单调递增。它就是盖在每条结论上的那个号码。
  version         BIGINT NOT NULL,

  sufficient_impressions  INT NULL,
  directional_impressions INT NULL,
  min_trend_days          INT NULL,
  min_anomaly_days        INT NULL,
  min_driver_assets       INT NULL,
  max_comparison_assets   INT NULL,
  quality_window_days     INT NULL,

  -- reason 必填。改判定标准是一件要负责的事，写不出理由的改动多半是试出来的，
  -- 而试出来的阈值会在三个月后没人说得清为什么是这个数。
  reason      VARCHAR(1000) NOT NULL,
  changed_by  VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  changed_at  DATETIME(6) NOT NULL,

  UNIQUE KEY uq_insight_threshold_sets_version (organization_id, version),
  CONSTRAINT chk_insight_threshold_sets_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
