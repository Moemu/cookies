package insights

import "testing"

// 旧报告的 digest 是 JSON 存的，里面只有 confidence 没有 verdict，也没有 origin。
// 读出来直接用会让复盘页上一半的发现没有档位、全部显示成「系统补的」都判断不了。
// 补齐必须发生在读取路径上，而不是靠一次性刷数据——刷完还会有更旧的备份被恢复回来。
func TestOldFindingsGetNormalizedOnRead(t *testing.T) {
	t.Parallel()

	old := ReportFinding{
		Kind:      SectionAssetPerformance,
		Text:      "15 秒版本的点击率比 30 秒版本高。",
		Judgement: Judgement{Confidence: ConfidenceDirectional},
	}
	old.normalize()

	if old.Origin != OriginSystem {
		t.Errorf("没有 origin 的旧发现应该算系统补的，得到 %q", old.Origin)
	}
	if old.Verdict != VerdictObserved {
		t.Errorf("verdict 应该由 confidence 收敛出来，得到 %q", old.Verdict)
	}
	if old.VerdictLabel != "只是观察" {
		t.Errorf("VerdictLabel 没补上，得到 %q", old.VerdictLabel)
	}
}

// 已经完整的发现不该被 normalize 改写——尤其是 Origin：把人记的那条改成系统补的，
// 复盘页上就再也分不清哪条是人自己挑的。
func TestNormalizeLeavesCompleteFindingsAlone(t *testing.T) {
	t.Parallel()

	pinned := ReportFinding{
		Kind:      SectionAssetPerformance,
		Text:      "15 秒版本的点击率比 30 秒版本高。",
		Origin:    OriginPinned,
		Judgement: judge(ConfidenceSufficient, "样本充分、区间不重叠。"),
	}
	before := pinned
	pinned.normalize()

	if pinned.Origin != before.Origin || pinned.Verdict != before.Verdict ||
		pinned.Note != before.Note {
		t.Errorf("完整的发现被 normalize 改写了：%+v -> %+v", before, pinned)
	}
}

// 去重键是「哪个维度上的哪个变量」。人在素材对比里记过「时长」，系统就不该在
// 同一份复盘里再补一条素材对比 · 时长——同一件事在会上被念两遍，第二遍会被当成
// 另一条独立证据。
func TestDedupeKeyIsDimensionPlusVariable(t *testing.T) {
	t.Parallel()

	a := ReportFinding{Dimension: "comparisons", Variable: "duration"}
	b := ReportFinding{Dimension: "comparisons", Variable: "duration"}
	c := ReportFinding{Dimension: "drivers", Variable: "duration"}

	if a.dedupeKey() != b.dedupeKey() {
		t.Error("同维度同变量应该是同一个去重键")
	}
	if a.dedupeKey() == c.dedupeKey() {
		t.Error("不同维度不该撞键——素材对比说的时长和驱动因素说的时长不是一回事")
	}
	// 自由文本类的发现（比如口径警告）没有维度也没有变量，不参与去重：
	// 拿空键去重会把它们全部折成一条。
	if (ReportFinding{}).dedupeKey() != "" {
		t.Error("没有维度和变量的发现不该参与去重")
	}
}
