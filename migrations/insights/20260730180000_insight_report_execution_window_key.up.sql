-- 一次投放可以定格出多份复盘报告。
--
-- 原来的唯一键 uq_insight_reports_execution (organization_id, execution_id) 是按
-- 「一次执行产出一份报告」建的——那时报告只是执行指标快照的换算，同一次执行算两遍
-- 结果一样，所以按执行去重就是幂等。
--
-- 现在报告是「在投后分析页定格的这一屏」：窗口是人当时选的。投放跑了 30 天，第 7 天
-- 定格一次看初期结论、第 30 天再定格一次看最终结论，是两份合法的报告。按执行去重会
-- 让第二次定格直接撞键失败。
--
-- 窗口列同时改成 NOT NULL DEFAULT ''，空串表示「这份报告没有窗口」。留 NULL 不行：
-- MySQL 的唯一索引认为每个 NULL 互不相等，不带窗口的报告就能建无数份，反而把原来
-- 那条幂等保证丢掉了。
UPDATE insight_reports SET window_start = '' WHERE window_start IS NULL;
UPDATE insight_reports SET window_end = '' WHERE window_end IS NULL;

ALTER TABLE insight_reports
  MODIFY COLUMN window_start VARCHAR(10) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  MODIFY COLUMN window_end VARCHAR(10) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  DROP INDEX uq_insight_reports_execution,
  ADD UNIQUE KEY uq_insight_reports_execution_window (organization_id, execution_id, window_start, window_end);
