-- 报告存结构化的四块发现与定格的数据窗口。
--
-- findings（旧的 []string）保留不动：已经存在的报告要能继续读出来。旧报告的内容
-- 全部出自投放数字的换算，删掉它等于把已经发生过的复盘从历史里抹掉。
--
-- digest 是新的四块结构（素材表现 / 实验结论 / 相关经验 / 下一轮建议），旧报告这一
-- 列为空数组。
--
-- window_start / window_end 是定格的数据窗口。报告存的是「在这个时点、基于这个窗口
-- 的数据、做了这个判断」——没有窗口，报告里的每个数字都无法复核。旧报告可空：
-- 它们本来就没有窗口概念，编一个进去比留空更糟。
ALTER TABLE insight_reports
  ADD COLUMN digest JSON NOT NULL DEFAULT (JSON_ARRAY()),
  ADD COLUMN window_start VARCHAR(10) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD COLUMN window_end VARCHAR(10) CHARACTER SET ascii COLLATE ascii_bin NULL;
