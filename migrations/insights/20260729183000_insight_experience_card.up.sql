-- 洞察卡九字段（03 §8.1 / 功能基线 §3.1）。
--
-- 经验表原本只有九字段里的四个：结论、适用范围（conditions）、风险与反例
-- （counterexamples）、状态。剩下五个——类型、数据依据、内容依据、置信提示、
-- 建议动作——一直没有落库，所以投前洞察拿不到合格的洞察卡，只能在前端写死示例。
--
-- 其中「类型」最关键：事实 / 统计观察 / 假设 / 建议是四种完全不同的东西，
-- 03 §2 目标⑥ 要求保住这条边界，避免把相关性写成因果。它不是展示用的标签，
-- 是这个模块区别于普通报表工具的地方，所以进表而不是靠前端推断。

ALTER TABLE insight_experiences
  -- 事实 / 统计观察 / 假设 / 建议。
  ADD COLUMN card_type VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'hypothesis' AFTER conclusion,

  -- 充分 / 方向性 / 样本不足 / 存在混杂。
  -- 取值沿用 connectors.go 里已有的 ConfidenceLevel（sufficient / directional /
  -- low_sample / confounded），不另起一套——指标总览和洞察卡说的是同一件事。
  ADD COLUMN confidence VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'directional' AFTER card_type,

  -- 建议动作：下一步在策略、创意还是投放上做什么。
  ADD COLUMN recommended_action VARCHAR(1000) NOT NULL DEFAULT '' AFTER confidence,

  -- 适用范围的结构化部分：品牌、产品、渠道、广告类型、目标、受众、时间。
  -- conditions 里的自由文本继续保留，两者不重复——一个用来筛选，一个用来读。
  ADD COLUMN applicability JSON NULL AFTER counterexamples,

  -- 数据依据：素材数量、样本量、时间窗口、指标、对比基线。
  ADD COLUMN data_basis JSON NULL AFTER applicability,

  -- 内容依据：关键素材特征与示例素材版本。
  ADD COLUMN content_basis JSON NULL AFTER data_basis;

-- 历史行一律落在最保守的一格：假设 + 方向性。
-- 它们当初录入时没人填过类型和置信，把它们当成「事实」或「充分」是替录入的人
-- 做了一个他没做过的判断，正是 §3.1 要防的那件事。
UPDATE insight_experiences
SET card_type = 'hypothesis', confidence = 'directional'
WHERE card_type = '' OR confidence = '';

ALTER TABLE insight_experiences
  ADD CONSTRAINT chk_insight_experiences_card_type
    CHECK (card_type IN ('fact', 'statistic', 'hypothesis', 'recommendation')),
  ADD CONSTRAINT chk_insight_experiences_confidence
    CHECK (confidence IN ('sufficient', 'directional', 'low_sample', 'confounded')),
  -- 「建议」类卡片没有建议动作就是一句空话。
  ADD CONSTRAINT chk_insight_experiences_recommended_action
    CHECK (card_type <> 'recommendation' OR recommended_action <> '');

-- 投前洞察按渠道和创意类型筛选，走的是 applicability 里的值。
-- MySQL 不能直接给 JSON 建索引，这里靠 status + updated_at 先收窄，
-- 剩下的在内存里筛——一个 Project 的已确认经验是几十条量级，不是几万条。
