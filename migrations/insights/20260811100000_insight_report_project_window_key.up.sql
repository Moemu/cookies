-- 复盘草稿要按 (项目 + 窗口) 唯一，唯一键必须带上 project_id。
--
-- 现在的键是 uq_insight_reports_execution_window (organization_id, execution_id,
-- window_start, window_end)。它是按「报告一定挂在一次投放执行上」建的，那时候
-- execution_id 非空，而执行本身就是项目独占的，所以不带 project_id 也不会撞。
--
-- 「记一笔」改变了这个前提：人在分析页看到一条值得留的结论就按一下，这时候还没有
-- 选投放执行——草稿的 execution_id 是空串。于是同一个组织里两个项目、同一个窗口，
-- 键都是 (org, '', '2026-07-01', '2026-07-30')，第二个项目的第一次记一笔会直接
-- 撞键失败，而错误信息只会说重复键，看不出是跨项目撞的。
--
-- 加上 project_id 之后，execution_id 非空的老行行为完全不变（执行本来就属于一个
-- 项目），空 execution_id 的草稿按项目分开。
--
-- execution_id 同时补上 DEFAULT ''：这一列原本是 NOT NULL 无默认值，因为过去每份
-- 报告都必须挂一次执行。记一笔建出来的草稿在提交之前不属于任何一次执行，让它显式
-- 存空串，而不是留一个「必须由调用方记得填」的坑。
ALTER TABLE insight_reports
  MODIFY COLUMN execution_id VARCHAR(96) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  DROP INDEX uq_insight_reports_execution_window,
  ADD UNIQUE KEY uq_insight_reports_project_execution_window
    (organization_id, project_id, execution_id, window_start, window_end);
