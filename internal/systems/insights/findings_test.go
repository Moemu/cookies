package insights

import (
	"testing"
	"time"
)

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

// 判定不能从请求里传。能传的话，页面上那个 ❓ 就是装饰——前端改一个字段
// 就能把「算不出来」记成「能归因」，而复盘会上没人会回去核。
func TestPinFindingRecomputesTheVerdictFromTheAnalysis(t *testing.T) {
	t.Parallel()

	analysis := PerformanceAnalysis{
		Drivers: []FeatureDriver{{
			Key:       "duration",
			Label:     "时长",
			Value:     "15s",
			Judgement: judge(ConfidenceLowSample, "样本较少的一侧只有 300 次展示。"),
		}},
	}
	got, text, ok := findJudgement(analysis, "drivers", "", "duration")
	if !ok {
		t.Fatal("应该能在驱动因素里找回这一条")
	}
	if got.Verdict != VerdictUnclear {
		t.Errorf("回查到的档位是 %s，期望 %s", got.Verdict, VerdictUnclear)
	}
	if text == "" {
		t.Error("回查应该同时给出这条自己的措辞，让人不用自己编")
	}
}

// 请求指到一条分析里不存在的结论时，必须拒绝，而不是记一条没有判定的发现。
// 记下去的话，复盘页上会出现一条既没有档位也没有出处的文字，谁也说不清它从哪来。
func TestPinFindingRejectsAConclusionThatIsNotOnTheScreen(t *testing.T) {
	t.Parallel()

	analysis := PerformanceAnalysis{}
	if _, _, ok := findJudgement(analysis, "drivers", "", "duration"); ok {
		t.Error("分析结果里没有这一条，不该回查成功")
	}
}

func TestPinFindingRequestValidation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	valid := PinFindingRequest{
		Window:    MetricWindow{Start: now.AddDate(0, 0, -30), End: now},
		Dimension: "drivers",
		Variable:  "duration",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("合法请求被拒了：%v", err)
	}

	noWindow := valid
	noWindow.Window = MetricWindow{}
	if err := noWindow.Validate(); err == nil {
		t.Error("没有窗口的记一笔应该被拒——不知道往哪份复盘草稿记")
	}

	badDimension := valid
	badDimension.Dimension = "whatever"
	if err := badDimension.Validate(); err == nil {
		t.Error("维度必须是六个视图之一")
	}

	noSubject := valid
	noSubject.Variable, noSubject.SourceRef = "", ""
	if err := noSubject.Validate(); err == nil {
		t.Error("变量和来源引用至少要有一个——两个都没有就回查不到任何一条")
	}
}
