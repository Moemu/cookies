-- 特征复核状态新增 authored：AI 从来没提过这一项，人是第一个填的。
--
-- 和 rejected 的区别是刚性的：rejected 说的是「有个推断，我不认」，authored 说的是
-- 「没有推断，我来填」。混成一个值有两处后果——投后分析会把人手填的特征当成被否掉
-- 的推断丢掉（于是内容分析里填的东西在素材对比和驱动因素里一条都看不见），技能提取
-- 准确率也会把 AI 压根没提过的项算成它提错了。
--
-- MySQL 改不了已有的 CHECK，只能先删后建。约束名与
-- 20260729103000_insight_asset_index.up.sql 里的定义一致。

ALTER TABLE insight_asset_features
  DROP CHECK chk_insight_asset_features_review_state;

ALTER TABLE insight_asset_features
  ADD CONSTRAINT chk_insight_asset_features_review_state
  CHECK (review_state IN ('pending', 'confirmed', 'rejected', 'authored'));
