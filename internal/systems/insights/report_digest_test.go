package insights

import (
	"strings"
	"testing"
)

func TestDigestOrdersByStrength(t *testing.T) {
	// 可归因的排在方向性前面，方向性排在混杂前面。
	// 报告是给人扫一眼的，最强的证据必须在最上面。
	analysis := PerformanceAnalysis{Comparable: true, Comparisons: []VariantComparison{
		{VariantTitle: "混", BaselineTitle: "基", VariantVerdict: VerdictConfounded, Judgement: judge(ConfidenceSufficient, "混杂的一条")},
		{VariantTitle: "归", BaselineTitle: "基", VariantVerdict: VerdictAttributable, Judgement: judge(ConfidenceSufficient, "可归因的一条")},
		{VariantTitle: "方", BaselineTitle: "基", VariantVerdict: VerdictDirectional, Judgement: judge(ConfidenceSufficient, "方向性的一条")},
	}}
	got := buildReportDigest(analysis, nil, nil)
	if len(got) == 0 {
		t.Fatal("汇总不该是空的")
	}
	if got[0].Strength != VerdictAttributable {
		t.Fatalf("最强的证据必须排在最上面，实际第一条是 %q", got[0].Strength)
	}
}

func TestDigestSkipsLowSampleFindings(t *testing.T) {
	// 样本不足的配对不进报告。带进去等于让人在复盘会上引用一条算不出来的结论。
	analysis := PerformanceAnalysis{Comparable: true, Comparisons: []VariantComparison{
		{VariantTitle: "甲", BaselineTitle: "乙", VariantVerdict: VerdictLowSample, Judgement: judge(ConfidenceSufficient, "样本不够")},
	}}
	got := buildReportDigest(analysis, nil, nil)
	for _, finding := range got {
		if finding.Strength == VerdictLowSample {
			t.Fatal("样本不足的结论不该自动带进报告")
		}
	}
}

func TestDigestCountsSkippedFindings(t *testing.T) {
	// 略过的条数必须写出来。静默截断读起来像「就这么多」，实际不是。
	analysis := PerformanceAnalysis{Comparable: true, Comparisons: []VariantComparison{
		{VariantTitle: "甲", BaselineTitle: "乙", VariantVerdict: VerdictLowSample, Judgement: judge(ConfidenceSufficient, "样本不够")},
		{VariantTitle: "丙", BaselineTitle: "丁", VariantVerdict: VerdictNoFeatures, Judgement: judge(ConfidenceSufficient, "没填特征")},
	}}
	got := buildReportDigest(analysis, nil, nil)
	var mentioned bool
	for _, finding := range got {
		if strings.Contains(finding.Text, "2 组素材") {
			mentioned = true
		}
	}
	if !mentioned {
		t.Fatal("被略过的对比条数没有出现在报告里，读者会以为算过的就这些")
	}
}

func TestDigestAlwaysHasExperimentSection(t *testing.T) {
	// 实验中心一个实验都没有时，这一块要明写「没有」，不能隐藏。
	// 隐藏了以后没人记得这块该有。
	got := buildReportDigest(PerformanceAnalysis{}, nil, nil)
	var found bool
	for _, finding := range got {
		if finding.Kind == SectionExperiment {
			found = true
		}
	}
	if !found {
		t.Fatal("实验结论这一块必须出现，哪怕内容是「本轮没有实验」")
	}
}

func TestDigestCoversFourSections(t *testing.T) {
	// 四块都必须出现。少一块，读的人不会发现它缺了——只会以为那块本来就没内容。
	got := buildReportDigest(PerformanceAnalysis{}, nil, nil)
	for _, kind := range ReportSectionOrder {
		var found bool
		for _, finding := range got {
			if finding.Kind == kind {
				found = true
			}
		}
		if !found {
			t.Fatalf("报告缺了「%s」这一块", ReportSectionLabels[kind])
		}
	}
}

func TestDigestOnlyRecommendsFromAttributable(t *testing.T) {
	// 方向性的结论不能推建议。方向性的意思就是「看着像但说不准」，
	// 拿它指导下一轮等于把一次没验证的观察变成一条要照着做的规定。
	analysis := PerformanceAnalysis{Comparable: true, Comparisons: []VariantComparison{{
		VariantTitle: "甲", BaselineTitle: "乙", VariantVerdict: VerdictDirectional, Judgement: judge(ConfidenceSufficient, "方向性"),
		ChangedFeatures: []FeatureDiff{{Label: "开场", Baseline: "产品", Variant: "人脸"}},
	}}}
	for _, finding := range buildReportDigest(analysis, nil, nil) {
		if finding.Kind == SectionRecommendation && finding.Strength == VerdictDirectional {
			t.Fatal("方向性的结论不该被推成下一轮建议")
		}
	}
}

func TestDigestKeepsAlternativeExplanations(t *testing.T) {
	// 疲劳信号排除不了的其他解释要跟着一起进报告。只写「在衰退」，
	// 下一步就会变成换素材，而真正的原因没人查。
	analysis := PerformanceAnalysis{Comparable: true, Fatigue: []FatigueSignal{{
		AssetTitle: "甲素材", Severity: FatigueLikely,
		Judgement:               judge(ConfidenceDirectional, "后半段点击率明显下滑"),
		AlternativeExplanations: []string{"这段时间预算也调过"},
	}}}
	var found bool
	for _, finding := range buildReportDigest(analysis, nil, nil) {
		if strings.Contains(finding.Text, "这段时间预算也调过") {
			found = true
		}
	}
	if !found {
		t.Fatal("没能排除的其他解释必须跟着疲劳信号一起进报告")
	}
}

func TestDigestReturnsEmptySliceNotNil(t *testing.T) {
	got := buildReportDigest(PerformanceAnalysis{}, nil, nil)
	if got == nil {
		t.Fatal("空切片必须初始化，nil 会序列化成 null 并崩掉前端")
	}
}

func TestDigestMergesSameDirection(t *testing.T) {
	// 同一个变量朝同一个方向改，在几对素材上各出现一次，报告里只能是一条建议。
	// 逐条推出去就是同一句话抄三遍，读的人分不清这是三条独立发现还是一条被重复了。
	// 而「有几组对比支持它」必须留在句子里——那才是这条建议值多少钱的关键。
	diff := []FeatureDiff{{Key: "hook", Label: "钩子类型", Baseline: "问题", Variant: "利益"}}
	// 三对都是 variant 赢，所以归并出来的方向都是「问题 → 利益」。
	// 点击率不能省：方向是按哪边赢定的，没有点击率就定不出方向。
	analysis := PerformanceAnalysis{Comparable: true, Comparisons: []VariantComparison{
		{VariantTitle: "v2", BaselineTitle: "v1", VariantVerdict: VerdictAttributable, ChangedFeatures: diff,
			BaselineRates: ratesWithCTR(0.02), VariantRates: ratesWithCTR(0.03)},
		{VariantTitle: "v3", BaselineTitle: "v1", VariantVerdict: VerdictAttributable, ChangedFeatures: diff,
			BaselineRates: ratesWithCTR(0.02), VariantRates: ratesWithCTR(0.031)},
		{VariantTitle: "v5", BaselineTitle: "v3", VariantVerdict: VerdictAttributable, ChangedFeatures: diff,
			BaselineRates: ratesWithCTR(0.021), VariantRates: ratesWithCTR(0.033)},
	}}
	var recommendations []string
	for _, finding := range buildReportDigest(analysis, nil, nil) {
		if finding.Kind == SectionRecommendation {
			recommendations = append(recommendations, finding.Text)
		}
	}
	if len(recommendations) != 1 {
		t.Fatalf("同一个方向只该推一条建议，实际推了 %d 条：%v", len(recommendations), recommendations)
	}
	if !strings.Contains(recommendations[0], "3 组") {
		t.Fatalf("建议里必须写清有几组对比支持它，实际是：%s", recommendations[0])
	}
}

func TestDigestRefusesToRecommendConflictingDirections(t *testing.T) {
	// 同一个变量的两个相反方向都被判成可归因时，绝不能各推一条建议——
	// 那是在同一节里叫人往两个相反的方向走，还都盖着「可归因」的章。
	// 两对都是 variant 赢，但赢的取值一个是「利益」一个是「问题」——
	// 归一到「哪边赢」之后方向依然相反，这才是真的打架。
	analysis := PerformanceAnalysis{Comparable: true, Comparisons: []VariantComparison{
		{VariantTitle: "v2", BaselineTitle: "v1", VariantVerdict: VerdictAttributable,
			BaselineRates:   ratesWithCTR(0.02),
			VariantRates:    ratesWithCTR(0.03),
			ChangedFeatures: []FeatureDiff{{Key: "hook", Label: "钩子类型", Baseline: "问题", Variant: "利益"}}},
		{VariantTitle: "v4", BaselineTitle: "v2", VariantVerdict: VerdictAttributable,
			BaselineRates:   ratesWithCTR(0.02),
			VariantRates:    ratesWithCTR(0.03),
			ChangedFeatures: []FeatureDiff{{Key: "hook", Label: "钩子类型", Baseline: "利益", Variant: "问题"}}},
	}}
	var recommendations []ReportFinding
	for _, finding := range buildReportDigest(analysis, nil, nil) {
		if finding.Kind == SectionRecommendation {
			recommendations = append(recommendations, finding)
		}
	}
	if len(recommendations) != 1 {
		t.Fatalf("方向打架时只该出一条说明，实际出了 %d 条", len(recommendations))
	}
	if strings.Contains(recommendations[0].Text, "可以继续按") {
		t.Fatalf("方向打架时不该推任何方向，实际是：%s", recommendations[0].Text)
	}
	if recommendations[0].Strength == VerdictAttributable {
		t.Fatal("这条说明本身不是可归因结论，不该盖可归因的章")
	}
	if !strings.Contains(recommendations[0].Text, "实验中心") {
		t.Fatalf("方向解不开时必须指向实验中心，实际是：%s", recommendations[0].Text)
	}
}

func ratesWithCTR(value float64) MetricRates {
	return MetricRates{CTR: &value}
}

func TestDigestRecommendsTheWinningDirection(t *testing.T) {
	// 这条盯的是一个真出过的错：建议的方向照抄了 baseline → variant，
	// 而 baseline 只是配对时花费更高的那一个，和表现好坏没有关系。
	// 结果报告写着「继续按（问题 → 利益）这个方向做」，而利益那一版点击率低了 41%。
	analysis := PerformanceAnalysis{Comparable: true, Comparisons: []VariantComparison{{
		VariantTitle: "B版", BaselineTitle: "A版", VariantVerdict: VerdictAttributable,
		VariantAssetID: "asset_b", BaselineAssetID: "asset_a",
		BaselineRates:   ratesWithCTR(0.0323),
		VariantRates:    ratesWithCTR(0.0189),
		ChangedFeatures: []FeatureDiff{{Key: "hook", Label: "钩子类型", Baseline: "问题", Variant: "利益"}},
	}}}
	var recommendation ReportFinding
	for _, finding := range buildReportDigest(analysis, nil, nil) {
		if finding.Kind == SectionRecommendation && finding.Strength == VerdictAttributable {
			recommendation = finding
		}
	}
	if recommendation.Text == "" {
		t.Fatal("可归因的对比必须推出一条建议")
	}
	if !strings.Contains(recommendation.Text, "利益 → 问题") {
		t.Fatalf("建议要指向赢的那一边（问题），实际是：%s", recommendation.Text)
	}
	if strings.Contains(recommendation.Text, "问题 → 利益") {
		t.Fatalf("建议指向了输的那一边，实际是：%s", recommendation.Text)
	}
	// 幅度按「赢的相对输的高多少」算，不是把 -41.5% 取个负号。
	// 相对变化不对称：3.23% 比 1.89% 高 70.9%，反过来只低 41.5%。
	if !strings.Contains(recommendation.Text, "70.9%") {
		t.Fatalf("幅度要按赢的一边相对输的一边算，实际是：%s", recommendation.Text)
	}
	if recommendation.SourceRef != "asset_a" {
		t.Fatalf("出处要指向赢的那条素材，实际是 %q", recommendation.SourceRef)
	}
}

// 人在分析页记过「素材对比 · 时长」，系统就不该在同一份复盘里再补一条同样的。
// 复盘会上同一件事被念两遍，第二遍会被当成另一条独立证据——两条相互印证的错觉，
// 比一条孤证更容易让人下决心。
func TestMergeFindingsDropsSystemDuplicatesOfWhatSomeonePinned(t *testing.T) {
	t.Parallel()

	pinned := []ReportFinding{{
		Kind: SectionAssetPerformance, Text: "15 秒版本点击率更高。",
		Origin: OriginPinned, Dimension: "comparisons", Variable: "duration",
		Judgement: judge(ConfidenceSufficient, "样本充分、区间不重叠。"),
	}}
	system := []ReportFinding{
		{Kind: SectionAssetPerformance, Text: "时长 15s 组的点击率高于其余素材。",
			Origin: OriginSystem, Dimension: "comparisons", Variable: "duration",
			Judgement: judge(ConfidenceSufficient, "")},
		{Kind: SectionAssetPerformance, Text: "开场有人脸的一组转化更好。",
			Origin: OriginSystem, Dimension: "drivers", Variable: "opening_face",
			Judgement: judge(ConfidenceDirectional, "")},
	}

	merged := mergeFindings(pinned, system)
	if len(merged) != 2 {
		t.Fatalf("撞键的系统发现应该被丢掉，剩 2 条，得到 %d 条：%+v", len(merged), merged)
	}
	if merged[0].Origin != OriginPinned {
		t.Error("人记的应该排在前面——复盘先看自己留的，再看系统补的")
	}
	if merged[1].Variable != "opening_face" {
		t.Errorf("没撞键的系统发现应该留下，得到 %q", merged[1].Variable)
	}
}

// 没有维度和变量的发现（口径警告、下一轮建议）去重键是空的，不参与去重。
// 拿空键去重会把它们全折成一条。
func TestMergeFindingsKeepsEveryFreeTextFinding(t *testing.T) {
	t.Parallel()

	system := []ReportFinding{
		{Kind: SectionRecommendation, Text: "下一轮把时长压到 15 秒。", Origin: OriginSystem},
		{Kind: SectionRecommendation, Text: "补一组开场有人脸的素材。", Origin: OriginSystem},
	}
	merged := mergeFindings(nil, system)
	if len(merged) != 2 {
		t.Fatalf("自由文本不该被折叠，期望 2 条，得到 %d 条", len(merged))
	}
}

// 系统内部自己也可能重复（同一个变量在对比和驱动里各出现一次）。
// 这两条不该互相消掉——它们的维度不同，说的确实是两件事。
// 但同维度同变量的系统重复要消掉。
func TestMergeFindingsDedupesWithinTheSystemBatch(t *testing.T) {
	t.Parallel()

	system := []ReportFinding{
		{Text: "甲", Origin: OriginSystem, Dimension: "drivers", Variable: "duration"},
		{Text: "乙", Origin: OriginSystem, Dimension: "drivers", Variable: "duration"},
		{Text: "丙", Origin: OriginSystem, Dimension: "comparisons", Variable: "duration"},
	}
	merged := mergeFindings(nil, system)
	if len(merged) != 2 {
		t.Fatalf("同维度同变量的系统重复要消掉，期望 2 条，得到 %d 条：%+v", len(merged), merged)
	}
	if merged[0].Text != "甲" {
		t.Errorf("重复时保留先出现的那条，得到 %q", merged[0].Text)
	}
}

// 人删掉的那条也占着去重键。删掉不等于「这个维度还空着」——人是看过之后
// 决定不要的，系统再补一条一模一样的回来，等于否决他的决定。
func TestMergeFindingsRespectsWhatSomeoneDeleted(t *testing.T) {
	t.Parallel()

	pinned := []ReportFinding{{
		Text: "15 秒版本点击率更高。", Origin: OriginPinned,
		Dimension: "comparisons", Variable: "duration", Dropped: true,
	}}
	system := []ReportFinding{{
		Text: "时长 15s 组的点击率高于其余素材。", Origin: OriginSystem,
		Dimension: "comparisons", Variable: "duration",
	}}
	merged := mergeFindings(pinned, system)
	if len(merged) != 1 {
		t.Fatalf("被删掉的那条仍然占着去重键，期望 1 条，得到 %d 条：%+v", len(merged), merged)
	}
}
