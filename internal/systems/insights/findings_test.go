package insights

import (
	"errors"
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

// 人记的那几笔之间按「哪个维度上的哪个变量、说的哪个素材」区分，比去重键多一个
// 出处。趋势和疲劳这两个维度没有变量（说的是素材本身，不是某个变量），A 版和 B 版
// 的趋势拿去重键一比就是同一条——记完 A 版再记 B 版，A 版会被无声顶掉，而按钮上
// 两条都写着「已记一笔」。人以为记了两笔，复盘会上只剩一笔。
func TestPinKeyKeepsTwoAssetsOnTheSameDimensionApart(t *testing.T) {
	t.Parallel()

	a := ReportFinding{Dimension: "trends", SourceRef: "asset_a"}
	b := ReportFinding{Dimension: "trends", SourceRef: "asset_b"}

	if a.dedupeKey() != b.dedupeKey() {
		t.Error("去重键本来就只看维度和变量，这里两条应该撞键——撞键正是系统不再补的依据")
	}
	if a.pinKey() == b.pinKey() {
		t.Error("两个素材在同一个维度上的两笔，不该被当成同一笔")
	}
	if a.pinKey() != (ReportFinding{Dimension: "trends", SourceRef: "asset_a"}).pinKey() {
		t.Error("同维度同出处的两笔应该是同一笔——第二次记的是他现在想说的那句")
	}
	// 自由文本三样都空，同样不参与顶替：拿空键去顶，第二条自由文本会吃掉第一条。
	if (ReportFinding{}).pinKey() != "" {
		t.Error("没有维度、变量和出处的发现不该参与顶替")
	}
}

// 同一笔记两次是覆盖，别的几笔原样留着，顺序也不变——复盘页上那几行的先后
// 是人自己记出来的，第二次补一句话不该让它跳到最后一行去。
func TestMergePinnedFindingReplacesTheSamePinAndKeepsTheRest(t *testing.T) {
	t.Parallel()

	digest := []ReportFinding{
		{Origin: OriginPinned, Dimension: "trends", SourceRef: "asset_a", Text: "旧的说法"},
		{Origin: OriginPinned, Dimension: "trends", SourceRef: "asset_b", Text: "B 版"},
		{Origin: OriginSystem, Text: "本轮没有已结论的实验。"},
	}
	again := ReportFinding{Origin: OriginPinned, Dimension: "trends", SourceRef: "asset_a", Text: "他现在想说的那句"}

	merged := mergePinnedFinding(digest, again)
	if len(merged) != 3 {
		t.Fatalf("覆盖不该改变条数，得到 %d 条", len(merged))
	}
	if merged[0].Text != "他现在想说的那句" {
		t.Errorf("第一条应该被覆盖成新的说法，得到 %q", merged[0].Text)
	}
	if merged[1].Text != "B 版" || merged[2].Origin != OriginSystem {
		t.Error("其他几笔应该原地不动")
	}

	fresh := mergePinnedFinding(digest, ReportFinding{
		Origin: OriginPinned, Dimension: "fatigue", SourceRef: "asset_a", Text: "新的一笔",
	})
	if len(fresh) != 4 || fresh[3].Text != "新的一笔" {
		t.Error("没记过的一笔应该追加在最后")
	}
}

// 「记一笔」这个按钮不带版本号——人按下去的时候屏幕上根本没有「草稿」这个东西，
// 草稿是这一下按出来的。所以读到的版本过期不是两个人对着同一份报告各改各的，
// 是同一个人连按了两下，后一下读到了前一下写进去之前的版本。重读一次再写就对了；
// 直接报错的话人只看到「记一笔失败」，而他什么也没做错。
func TestPinFindingRetriesWhenTheDraftMovedUnderIt(t *testing.T) {
	t.Parallel()

	calls := 0
	value, err := retryContendedWrite(func() (InsightReport, error) {
		calls++
		if calls == 1 {
			return InsightReport{}, ErrVersionConflict
		}
		return InsightReport{ID: "insightreport_1", Version: 2}, nil
	})
	if err != nil {
		t.Fatalf("重试一次就该成功，却报了 %v", err)
	}
	if value.ID != "insightreport_1" || calls != 2 {
		t.Errorf("期望第二次拿到结果，实际调用 %d 次、结果 %+v", calls, value)
	}
}

// 这个窗口还没有草稿的时候连按两下，两下都会去建草稿，(项目 + 窗口) 的唯一键
// 只会让一下成功。输的那一下不该把「这个窗口已经定格过一份报告了」甩到人脸上——
// 他按的是记一笔，压根没打算建报告。退回去重读，把这一笔追加进那份草稿。
func TestPinFindingRetriesWhenAnotherPinCreatedTheDraftFirst(t *testing.T) {
	t.Parallel()

	calls := 0
	value, err := retryContendedWrite(func() (InsightReport, error) {
		calls++
		if calls == 1 {
			return InsightReport{}, errDraftRaced
		}
		return InsightReport{ID: "insightreport_1", Version: 2}, nil
	})
	if err != nil || value.ID != "insightreport_1" || calls != 2 {
		t.Fatalf("期望第二次追加成功，实际调用 %d 次、结果 %+v、错误 %v", calls, value, err)
	}
}

// 抢草稿一直抢不到也要停下来，而且不能把内部约定漏出去：errDraftRaced 不在
// insights 的四个错误值里，HTTP 层认不出来，人会看到一个 500。
func TestPinFindingNeverLeaksTheInternalRaceError(t *testing.T) {
	t.Parallel()

	_, err := retryContendedWrite(func() (InsightReport, error) {
		return InsightReport{}, errDraftRaced
	})
	if errors.Is(err, errDraftRaced) {
		t.Error("内部的抢建标记漏到外面了")
	}
	if !errors.Is(err, ErrVersionConflict) {
		t.Errorf("应该收敛成版本冲突让人再按一次，得到 %v", err)
	}
}

// 版本冲突之外的错误立刻抛出。草稿已经提交了还去重试，只会把同一个错误报三遍，
// 而人等的时间变成三倍。
func TestPinFindingDoesNotRetryOtherErrors(t *testing.T) {
	t.Parallel()

	calls := 0
	if _, err := retryContendedWrite(func() (InsightReport, error) {
		calls++
		return InsightReport{}, ErrInvalidState
	}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("期望原样抛出 ErrInvalidState，得到 %v", err)
	}
	if calls != 1 {
		t.Errorf("不该重试，实际调用 %d 次", calls)
	}
}

// 一直冲突也要停下来，并且把冲突本身报出去——无限重试会让一个坏掉的写路径
// 变成一个挂住的请求，比报错更难查。
func TestPinFindingGivesUpAfterEnoughConflicts(t *testing.T) {
	t.Parallel()

	calls := 0
	_, err := retryContendedWrite(func() (InsightReport, error) {
		calls++
		return InsightReport{}, ErrVersionConflict
	})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("放弃时应该报版本冲突，得到 %v", err)
	}
	if calls != pinAttempts {
		t.Errorf("期望试 %d 次，实际 %d 次", pinAttempts, calls)
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
