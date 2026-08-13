-- 经验四态收成三态，「待复审」变成「在用」上的一个标记。
--
-- 「待复审」从来就不是一个独立状态：一条被标了待复审的经验，仍然在用、仍然
-- 能被引用，只是有人觉得该重新看一眼。把它做成状态，代价是每个读经验的地方
-- 都要判断「confirmed 或者 needs_review」——漏一处，那条经验就在某个页面上
-- 凭空消失了。做成标记之后，读的地方只认 confirmed，标记只影响怎么显示。
--
-- 数据迁移的方向是「进而不是退」：原来 needs_review 的行变成 confirmed 加标记，
-- 它们仍然在用。反过来把它们降级成 pending 会让一批已经在被引用的经验突然
-- 失去引用资格，而没有人做过这个决定。
ALTER TABLE insight_experiences
  ADD COLUMN needs_review TINYINT(1) NOT NULL DEFAULT 0 AFTER status;

UPDATE insight_experiences SET needs_review = 1, status = 'confirmed'
  WHERE status = 'needs_review';

ALTER TABLE insight_experiences
  DROP CHECK chk_insight_experiences_status;

ALTER TABLE insight_experiences
  ADD CONSTRAINT chk_insight_experiences_status
  CHECK (status IN ('pending', 'confirmed', 'retired'));

-- 审计表 insight_experience_audits 的历史记录原样保留，不改写。
-- 那些 to_status = 'needs_review' 的行是真实发生过的事，改掉它们等于伪造历史；
-- 那张表本来就没有 status 的 CHECK 约束，历史取值还读得出来。
