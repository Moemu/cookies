package insights

import "testing"

// 四个统计档位往三个行动档位收敛的规则，是整个模块的中轴。这张表写死在测试里，
// 改代码改不动它——要改必须先改这张表，那时候就得解释为什么。
func TestConfidenceMapsToExactlyThreeVerdicts(t *testing.T) {
	t.Parallel()

	want := map[ConfidenceLevel]Verdict{
		ConfidenceSufficient:  VerdictExplained,
		ConfidenceDirectional: VerdictObserved,
		ConfidenceConfounded:  VerdictObserved,
		ConfidenceLowSample:   VerdictUnclear,
	}
	for level, expected := range want {
		if got := level.Verdict(); got != expected {
			t.Errorf("%s 收敛成 %s，期望 %s", level, got, expected)
		}
	}
	// 新加一个 ConfidenceLevel 却忘了给它三档归属，会静默落到默认分支，
	// 屏幕上就多出一批莫名其妙的「算不出来」。这一条拦住它。
	if len(want) != len(allConfidenceLevels) {
		t.Fatalf("ConfidenceLevel 有 %d 个取值，但只有 %d 个有三档归属", len(allConfidenceLevels), len(want))
	}
}

// 三档不是终点，是「现在还缺什么」。缺的东西不一样，下一步就不一样。
func TestEachVerdictKnowsItsOnlyUpgradePath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		verdict Verdict
		upgrade UpgradePath
	}{
		{VerdictExplained, UpgradeNone},
		{VerdictObserved, UpgradeExperiment},
		{VerdictUnclear, UpgradeSimilar},
	}
	for _, c := range cases {
		if got := c.verdict.Upgrade(); got != c.upgrade {
			t.Errorf("%s 的升级通道是 %q，期望 %q", c.verdict, got, c.upgrade)
		}
	}
}

func TestVerdictLabelAndIconAreFixed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		verdict Verdict
		label   string
		icon    string
	}{
		{VerdictExplained, "能归因", "✅"},
		{VerdictObserved, "只是观察", "👁"},
		{VerdictUnclear, "算不出来", "❓"},
	}
	for _, c := range cases {
		if got := c.verdict.Label(); got != c.label {
			t.Errorf("%s 的文案是 %q，期望 %q", c.verdict, got, c.label)
		}
		if got := c.verdict.Icon(); got != c.icon {
			t.Errorf("%s 的图标是 %q，期望 %q", c.verdict, got, c.icon)
		}
	}
}

// 一屏上有一条算不出来，这屏就不能整体标成能归因。
func TestScreenVerdictTakesTheWeakestItem(t *testing.T) {
	t.Parallel()

	if got := weakestVerdict(VerdictExplained, VerdictExplained); got != VerdictExplained {
		t.Errorf("全是能归因时屏级应为能归因，得到 %s", got)
	}
	if got := weakestVerdict(VerdictExplained, VerdictObserved); got != VerdictObserved {
		t.Errorf("混了只是观察时屏级应为只是观察，得到 %s", got)
	}
	if got := weakestVerdict(VerdictExplained, VerdictUnclear, VerdictObserved); got != VerdictUnclear {
		t.Errorf("混了算不出来时屏级应为算不出来，得到 %s", got)
	}
	// 一条结论都没有，是「算不出来」，不是「能归因」。空屏默认成 ✅ 会让
	// 没有数据的页面显得比有数据的页面更可信。
	if got := weakestVerdict(); got != VerdictUnclear {
		t.Errorf("空输入应为算不出来，得到 %s", got)
	}
}

func TestJudgeFillsEveryFieldFromTheConfidence(t *testing.T) {
	t.Parallel()

	got := judge(ConfidenceConfounded, "两组素材在时长上也整齐不同。")
	if got.Confidence != ConfidenceConfounded {
		t.Errorf("Confidence 被改写成了 %s", got.Confidence)
	}
	if got.Verdict != VerdictObserved {
		t.Errorf("Verdict 是 %s，期望 %s", got.Verdict, VerdictObserved)
	}
	if got.VerdictLabel != "只是观察" {
		t.Errorf("VerdictLabel 是 %q", got.VerdictLabel)
	}
	if got.Upgrade != UpgradeExperiment {
		t.Errorf("Upgrade 是 %q，期望 %q", got.Upgrade, UpgradeExperiment)
	}
	if got.Note != "两组素材在时长上也整齐不同。" {
		t.Errorf("Note 被改写成了 %q", got.Note)
	}
}
