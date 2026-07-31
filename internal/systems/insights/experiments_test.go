package insights

import (
	"strings"
	"testing"
	"time"
)

// 这一组测试钉住实验中心区别于投后分析的那几条纪律。放宽任何一条，
// 这一页就退化成「看起来更正式的素材对比」——而它存在的全部理由就是不退化。

func testExperiment(minImpressions int64, variants ...ExperimentVariant) Experiment {
	window := testWindow(10)
	return Experiment{
		ID: "exp_1", Title: "露脸开场是否提升点击",
		AssetType: AssetTypePrerollAd, VariableKey: "preroll_hook_type", VariableLabel: "开场钩子类型",
		MinImpressions: minImpressions,
		WindowStart:    window.Start, WindowEnd: window.End,
		Status: ExperimentRunning, Variants: variants,
	}
}

func variantOf(id, name, value string, baseline bool, assetIDs ...string) ExperimentVariant {
	return ExperimentVariant{ID: id, Name: name, VariableValue: value, IsBaseline: baseline, AssetIDs: assetIDs}
}

func comparisonFor(t *testing.T, readout ExperimentReadout, variantID string) ExperimentComparison {
	t.Helper()
	for _, comparison := range readout.Comparisons {
		if comparison.VariantID == variantID {
			return comparison
		}
	}
	t.Fatalf("没有 %s 的对比行", variantID)
	return ExperimentComparison{}
}

// 样本不到事先定的门槛时，一个对比数字都不给。
//
// 这是整个实验中心最容易被「先显示出来再加个提示」替换掉的一条。人会先看见数字
// 再看见提示，然后记住数字——不给数字是唯一可靠的拦法。
func TestExperimentBelowThresholdShowsNoComparisonNumbers(t *testing.T) {
	facts := append(
		factsFor("asset_a", "露脸版", AssetTypePrerollAd, "obj_a", 10, MetricCounts{Impressions: 100, Clicks: 30}),
		factsFor("asset_b", "不露脸版", AssetTypePrerollAd, "obj_b", 10, MetricCounts{Impressions: 100, Clicks: 3})...,
	)
	experiment := testExperiment(50000,
		variantOf("var_base", "不露脸", "不露脸", true, "asset_b"),
		variantOf("var_test", "露脸", "露脸", false, "asset_a"),
	)

	readout := buildExperimentReadout(experiment, facts)
	comparison := comparisonFor(t, readout, "var_test")

	if !comparison.Blocked {
		t.Fatalf("样本不到门槛必须拦下，实际没拦")
	}
	if comparison.Result != nil {
		t.Fatalf("样本不到门槛时不能给任何对比数字，实际给了 %+v", *comparison.Result)
	}
	if comparison.Verdict != "" {
		t.Fatalf("样本不到门槛时不能给判定，实际 %q", comparison.Verdict)
	}
	if readout.Ready {
		t.Fatalf("样本不到门槛时不能允许下结论")
	}
	// 样本数本身要照常给：不知道差多少就不知道还要投多久。
	sample := findSample(readout.Samples, "var_test")
	if sample.Impressions != 1000 {
		t.Fatalf("样本量应照常返回，实际 %d", sample.Impressions)
	}
	if sample.Short != 49000 {
		t.Fatalf("应告诉用户还差多少，实际 %d", sample.Short)
	}
}

// 样本充分、区间不重叠时给出判定，并且措辞够得上因果——
// 分组是事先定的，这正是它和投后分析「相关」措辞的唯一区别。
func TestExperimentPreRegisteredWordingReachesCausal(t *testing.T) {
	facts := append(
		factsFor("asset_a", "露脸版", AssetTypePrerollAd, "obj_a", 10, MetricCounts{Impressions: 4000, Clicks: 200}),
		factsFor("asset_b", "不露脸版", AssetTypePrerollAd, "obj_b", 10, MetricCounts{Impressions: 4000, Clicks: 80})...,
	)
	experiment := testExperiment(10000,
		variantOf("var_base", "不露脸", "不露脸", true, "asset_b"),
		variantOf("var_test", "露脸", "露脸", false, "asset_a"),
	)

	readout := buildExperimentReadout(experiment, facts)
	comparison := comparisonFor(t, readout, "var_test")

	if comparison.Blocked {
		t.Fatalf("样本充分时不该拦：%s", comparison.Blocker)
	}
	if comparison.Result == nil {
		t.Fatal("样本充分时应给出对比结果")
	}
	if comparison.Verdict != VerdictSupported {
		t.Fatalf("变体明显更好时应判为成立，实际 %q（%s）", comparison.Verdict, comparison.Result.Note)
	}
	if !readout.Ready {
		t.Fatal("两组都过门槛时应允许下结论")
	}
	if readout.Verdict != VerdictSupported {
		t.Fatalf("主结论应为成立，实际 %q", readout.Verdict)
	}
	// 措辞对照：事先登记说「能归因」，事后凑对只能说「相关」。
	if !strings.Contains(comparison.Result.Note, "事先定的") {
		t.Fatalf("事先登记的实验措辞应点明分组是事先定的，实际 %q", comparison.Result.Note)
	}
	if strings.Contains(comparison.Result.Note, "相关不是因果") {
		t.Fatalf("事先登记的实验不该沿用事后分析的「相关」措辞，实际 %q", comparison.Result.Note)
	}
}

// 变体反而更差时是「假设被推翻」，不是「分不出来」。
// 把推翻混进分不出来，等于让实验只能证实不能证伪。
func TestExperimentRefutedWhenVariantIsWorse(t *testing.T) {
	facts := append(
		factsFor("asset_a", "露脸版", AssetTypePrerollAd, "obj_a", 10, MetricCounts{Impressions: 4000, Clicks: 80}),
		factsFor("asset_b", "不露脸版", AssetTypePrerollAd, "obj_b", 10, MetricCounts{Impressions: 4000, Clicks: 200})...,
	)
	experiment := testExperiment(10000,
		variantOf("var_base", "不露脸", "不露脸", true, "asset_b"),
		variantOf("var_test", "露脸", "露脸", false, "asset_a"),
	)

	comparison := comparisonFor(t, buildExperimentReadout(experiment, facts), "var_test")
	if comparison.Verdict != VerdictRefuted {
		t.Fatalf("变体更差时应判为被推翻，实际 %q", comparison.Verdict)
	}
}

// 区间重叠就是分不出来，样本再多也一样。
func TestExperimentInconclusiveWhenIntervalsOverlap(t *testing.T) {
	facts := append(
		factsFor("asset_a", "露脸版", AssetTypePrerollAd, "obj_a", 10, MetricCounts{Impressions: 4000, Clicks: 200}),
		factsFor("asset_b", "不露脸版", AssetTypePrerollAd, "obj_b", 10, MetricCounts{Impressions: 4000, Clicks: 202})...,
	)
	experiment := testExperiment(10000,
		variantOf("var_base", "不露脸", "不露脸", true, "asset_b"),
		variantOf("var_test", "露脸", "露脸", false, "asset_a"),
	)

	comparison := comparisonFor(t, buildExperimentReadout(experiment, facts), "var_test")
	if comparison.Verdict != VerdictInconclusive {
		t.Fatalf("区间重叠时应判为分不出来，实际 %q", comparison.Verdict)
	}
}

// 基线组不达标会把它参与的每一条对比都拦下，而不只是拦下它自己。
func TestExperimentBaselineShortfallBlocksEveryComparison(t *testing.T) {
	facts := append(append(
		factsFor("asset_a", "A", AssetTypePrerollAd, "obj_a", 10, MetricCounts{Impressions: 4000, Clicks: 200}),
		factsFor("asset_c", "C", AssetTypePrerollAd, "obj_c", 10, MetricCounts{Impressions: 4000, Clicks: 150})...),
		factsFor("asset_b", "B", AssetTypePrerollAd, "obj_b", 10, MetricCounts{Impressions: 100, Clicks: 3})...,
	)
	experiment := testExperiment(10000,
		variantOf("var_base", "不露脸", "不露脸", true, "asset_b"),
		variantOf("var_a", "露脸", "露脸", false, "asset_a"),
		variantOf("var_c", "字幕开场", "字幕", false, "asset_c"),
	)

	readout := buildExperimentReadout(experiment, facts)
	for _, id := range []string{"var_a", "var_c"} {
		comparison := comparisonFor(t, readout, id)
		if !comparison.Blocked || comparison.Result != nil {
			t.Fatalf("基线组不达标时 %s 也必须拦下", id)
		}
	}
	if readout.Ready {
		t.Fatal("有对比被拦下时不能允许下结论")
	}
}

// 已下结论的实验显示当时定下的判定，而不是此刻重算的：
// 数据还在往里进，重算会让一条已经沉淀成经验的结论在页面上悄悄改口。
func TestConcludedExperimentKeepsTheVerdictItWasConcludedWith(t *testing.T) {
	facts := append(
		factsFor("asset_a", "A", AssetTypePrerollAd, "obj_a", 10, MetricCounts{Impressions: 4000, Clicks: 200}),
		factsFor("asset_b", "B", AssetTypePrerollAd, "obj_b", 10, MetricCounts{Impressions: 4000, Clicks: 80})...,
	)
	experiment := testExperiment(10000,
		variantOf("var_base", "不露脸", "不露脸", true, "asset_b"),
		variantOf("var_test", "露脸", "露脸", false, "asset_a"),
	)
	experiment.Status = ExperimentConcluded
	experiment.Verdict = VerdictInconclusive
	experiment.Interpretation = "当时样本刚够，没看出差别"

	readout := buildExperimentReadout(experiment, facts)
	if readout.Verdict != VerdictInconclusive {
		t.Fatalf("已下结论的实验应保留当时的判定，实际 %q", readout.Verdict)
	}
}

// 没有基线组就没有参照物。这种实验建不出来（validateVariantRequests 拦），
// 但历史数据里可能有，读数这一层也要说清楚而不是默默比。
func TestExperimentWithoutBaselineProducesNoComparisons(t *testing.T) {
	experiment := testExperiment(1000,
		variantOf("var_a", "露脸", "露脸", false, "asset_a"),
		variantOf("var_b", "不露脸", "不露脸", false, "asset_b"),
	)
	readout := buildExperimentReadout(experiment, nil)
	if len(readout.Comparisons) != 0 {
		t.Fatalf("没有基线组时不该有对比行，实际 %d 行", len(readout.Comparisons))
	}
	if len(readout.Notes) == 0 {
		t.Fatal("没有基线组时应说明原因")
	}
}

func TestVariantRequestsRequireExactlyOneBaselineAndTwoGroups(t *testing.T) {
	cases := []struct {
		name     string
		variants []CreateVariantRequest
	}{
		{"只有一组", []CreateVariantRequest{{Name: "露脸", VariableValue: "露脸", IsBaseline: true}}},
		{"没有基线", []CreateVariantRequest{
			{Name: "露脸", VariableValue: "露脸"}, {Name: "不露脸", VariableValue: "不露脸"}}},
		{"两个基线", []CreateVariantRequest{
			{Name: "露脸", VariableValue: "露脸", IsBaseline: true},
			{Name: "不露脸", VariableValue: "不露脸", IsBaseline: true}}},
		{"取值重复", []CreateVariantRequest{
			{Name: "甲", VariableValue: "露脸", IsBaseline: true}, {Name: "乙", VariableValue: "露脸"}}},
		{"重名", []CreateVariantRequest{
			{Name: "露脸", VariableValue: "露脸", IsBaseline: true}, {Name: "露脸", VariableValue: "不露脸"}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := validateVariantRequests(testCase.variants); err == nil {
				t.Fatal("应该被拦下，实际通过了")
			}
		})
	}
	valid := []CreateVariantRequest{
		{Name: "露脸", VariableValue: "露脸"},
		{Name: "不露脸", VariableValue: "不露脸", IsBaseline: true},
	}
	if err := validateVariantRequests(valid); err != nil {
		t.Fatalf("合法分组不该被拦：%v", err)
	}
}

// 自由文本做不了对照变量：两条素材的取值几乎不可能一样，分组会全是单元素组。
func TestExperimentRejectsFreeTextVariable(t *testing.T) {
	schema, _ := FeatureSchemaFor(AssetTypePrerollAd)
	freeText := ""
	for _, field := range schema.Fields {
		if !comparableKind(field.Kind) {
			freeText = field.Key
			break
		}
	}
	if freeText == "" {
		t.Skip("这套特征体系里没有自由文本字段")
	}
	experiment := testExperiment(1000)
	experiment.VariableKey = freeText
	if err := experiment.validate(); err == nil {
		t.Fatal("自由文本不该能当被测变量")
	}
}

// 被测变量同时又被要求「控住」是自相矛盾的登记，建的时候就要拦。
func TestExperimentRejectsControllingTheVariableUnderTest(t *testing.T) {
	experiment := testExperiment(1000)
	experiment.ControlledKeys = []string{"preroll_hook_type"}
	if err := experiment.validate(); err == nil {
		t.Fatal("被测变量不能同时出现在受控变量里")
	}
}

func TestExperimentWindowRejectsBackwardsAndOverlongRanges(t *testing.T) {
	if _, err := parseExperimentWindow("2026-07-10", "2026-07-01"); err == nil {
		t.Fatal("结束早于开始应被拦下")
	}
	if _, err := parseExperimentWindow("2026-01-01", "2026-12-31"); err == nil {
		t.Fatal("超过 180 天应被拦下")
	}
	window, err := parseExperimentWindow("2026-07-01", "2026-07-10")
	if err != nil {
		t.Fatalf("合法窗口不该被拦：%v", err)
	}
	if !window.End.Equal(time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("窗口结束日期解析错误：%v", window.End)
	}
}

func TestCountTextGroupsThousands(t *testing.T) {
	// 「还差 9890020 次」和「还差 989002 次」肉眼分不出差了一位，
	// 而这个数字决定的是「再跑几天」还是「这次实验白做了」。
	cases := map[int64]string{0: "0", 7: "7", 100: "100", 1000: "1,000", 9890020: "9,890,020", 10000000: "10,000,000"}
	for value, want := range cases {
		if got := countText(value); got != want {
			t.Fatalf("countText(%d) = %s，期望 %s", value, got, want)
		}
	}
}
