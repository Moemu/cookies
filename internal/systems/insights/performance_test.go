package insights

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// 这一组测试钉住投后分析里唯一容易被悄悄放宽的东西：**什么时候才敢说「是这个变量」**。
// 判定放宽一格，页面上就会出现看起来很确定、实际归不了因的结论——比没有这一页更糟。

func testWindow(days int) MetricWindow {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	return MetricWindow{Start: start, End: start.AddDate(0, 0, days-1)}
}

func factsFor(assetID, title string, kind AssetType, objectID string, days int, daily MetricCounts) []MetricFactWithMapping {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	facts := make([]MetricFactWithMapping, 0, days)
	for index := 0; index < days; index++ {
		facts = append(facts, MetricFactWithMapping{
			MetricFact: MetricFact{
				Platform: PlatformDouyin, PlatformObjectKind: "creative", PlatformObjectID: objectID,
				StatDate: start.AddDate(0, 0, index), Counts: daily,
				Caliber: MetricCaliber{Currency: "CNY", AttributionWindow: "7d", MetricSchemaVersion: "v1"},
			},
			AssetID: assetID, AssetTitle: title, AssetType: kind, MappingStatus: MappingMatched,
		})
	}
	return facts
}

func enumFeature(assetID string, kind AssetType, key, term string) AssetFeature {
	return AssetFeature{
		AssetID: assetID, AssetType: kind, Key: key,
		Value:       FeatureValue{Kind: FeatureKindEnum, Terms: []string{term}},
		Source:      SourceAI,
		ReviewState: ReviewConfirmed,
	}
}

func findComparison(t *testing.T, analysis PerformanceAnalysis, left, right string) VariantComparison {
	t.Helper()
	for _, comparison := range analysis.Comparisons {
		if comparison.BaselineAssetID == left && comparison.VariantAssetID == right {
			return comparison
		}
		if comparison.BaselineAssetID == right && comparison.VariantAssetID == left {
			return comparison
		}
	}
	t.Fatalf("没有 %s 和 %s 的对比行", left, right)
	return VariantComparison{}
}

// 只改一个变量、样本充分、区间不重叠 —— 这是唯一能说「归到这个变量」的情形。
func TestVariantComparisonAttributableOnlyWhenSingleVariable(t *testing.T) {
	facts := append(
		factsFor("asset_a", "露脸版", AssetTypePrerollAd, "obj_a", 10, MetricCounts{Impressions: 4000, Clicks: 200, SpendCents: 10000}),
		factsFor("asset_b", "不露脸版", AssetTypePrerollAd, "obj_b", 10, MetricCounts{Impressions: 4000, Clicks: 80, SpendCents: 10000})...,
	)
	features := []AssetFeature{
		enumFeature("asset_a", AssetTypePrerollAd, "preroll_hook_type", "露脸"),
		enumFeature("asset_b", AssetTypePrerollAd, "preroll_hook_type", "不露脸"),
		enumFeature("asset_a", AssetTypePrerollAd, "cta_type", "立即购买"),
		enumFeature("asset_b", AssetTypePrerollAd, "cta_type", "立即购买"),
	}

	analysis := buildPerformanceAnalysis(testWindow(10), facts, features)
	comparison := findComparison(t, analysis, "asset_a", "asset_b")

	if comparison.Verdict != VerdictAttributable {
		t.Fatalf("单变量且样本充分时应判为可归因，实际 %q（%s）", comparison.Verdict, comparison.Note)
	}
	if len(comparison.ChangedFeatures) != 1 {
		t.Fatalf("变量应只有一个，实际 %d", len(comparison.ChangedFeatures))
	}
	if comparison.ControlledCount != 1 {
		t.Fatalf("受控特征应为 1，实际 %d", comparison.ControlledCount)
	}
}

// 同时改两个变量就是混杂，不管差异多大。这一条是 AM-009 的全部意义。
func TestVariantComparisonConfoundedWhenTwoVariablesChange(t *testing.T) {
	facts := append(
		factsFor("asset_a", "A", AssetTypePrerollAd, "obj_a", 10, MetricCounts{Impressions: 4000, Clicks: 400, SpendCents: 10000}),
		factsFor("asset_b", "B", AssetTypePrerollAd, "obj_b", 10, MetricCounts{Impressions: 4000, Clicks: 40, SpendCents: 10000})...,
	)
	features := []AssetFeature{
		enumFeature("asset_a", AssetTypePrerollAd, "preroll_hook_type", "露脸"),
		enumFeature("asset_b", AssetTypePrerollAd, "preroll_hook_type", "不露脸"),
		enumFeature("asset_a", AssetTypePrerollAd, "cta_type", "立即购买"),
		enumFeature("asset_b", AssetTypePrerollAd, "cta_type", "了解更多"),
	}

	comparison := findComparison(t, buildPerformanceAnalysis(testWindow(10), facts, features), "asset_a", "asset_b")
	if comparison.Verdict != VerdictConfounded {
		t.Fatalf("两个变量同时变化时必须判为混杂，实际 %q", comparison.Verdict)
	}
	if comparison.Confidence != ConfidenceConfounded {
		t.Fatalf("置信应为 confounded，实际 %q", comparison.Confidence)
	}
}

// 样本不足时不谈差异，哪怕变量是单一的。
func TestVariantComparisonLowSampleBeatsSingleVariable(t *testing.T) {
	facts := append(
		factsFor("asset_a", "A", AssetTypePrerollAd, "obj_a", 5, MetricCounts{Impressions: 50, Clicks: 25}),
		factsFor("asset_b", "B", AssetTypePrerollAd, "obj_b", 5, MetricCounts{Impressions: 50, Clicks: 1})...,
	)
	features := []AssetFeature{
		enumFeature("asset_a", AssetTypePrerollAd, "preroll_hook_type", "露脸"),
		enumFeature("asset_b", AssetTypePrerollAd, "preroll_hook_type", "不露脸"),
	}

	comparison := findComparison(t, buildPerformanceAnalysis(testWindow(5), facts, features), "asset_a", "asset_b")
	if comparison.Verdict != VerdictLowSample {
		t.Fatalf("样本不足时必须先判低样本，实际 %q", comparison.Verdict)
	}
}

// 没有特征就没有变量。这时数字差异再大也不能算到任何东西头上。
func TestVariantComparisonWithoutFeaturesIsNotAttributable(t *testing.T) {
	facts := append(
		factsFor("asset_a", "A", AssetTypePrerollAd, "obj_a", 10, MetricCounts{Impressions: 40000, Clicks: 4000}),
		factsFor("asset_b", "B", AssetTypePrerollAd, "obj_b", 10, MetricCounts{Impressions: 40000, Clicks: 400})...,
	)

	comparison := findComparison(t, buildPerformanceAnalysis(testWindow(10), facts, nil), "asset_a", "asset_b")
	if comparison.Verdict != VerdictNoFeatures {
		t.Fatalf("无特征时应判为 no_features，实际 %q", comparison.Verdict)
	}
}

// 自由文本不是变量：它每个素材都不同，当变量用会让每一对都变成「改了很多个」。
func TestFreeTextFeatureIsNotTreatedAsVariable(t *testing.T) {
	facts := append(
		factsFor("asset_a", "A", AssetTypePrerollAd, "obj_a", 10, MetricCounts{Impressions: 4000, Clicks: 200}),
		factsFor("asset_b", "B", AssetTypePrerollAd, "obj_b", 10, MetricCounts{Impressions: 4000, Clicks: 80})...,
	)
	features := []AssetFeature{
		enumFeature("asset_a", AssetTypePrerollAd, "preroll_hook_type", "露脸"),
		enumFeature("asset_b", AssetTypePrerollAd, "preroll_hook_type", "不露脸"),
		{AssetID: "asset_a", AssetType: AssetTypePrerollAd, Key: "summary",
			Value: FeatureValue{Kind: FeatureKindText, Text: "开头是人脸特写"}, Source: SourceAI, ReviewState: ReviewConfirmed},
		{AssetID: "asset_b", AssetType: AssetTypePrerollAd, Key: "summary",
			Value: FeatureValue{Kind: FeatureKindText, Text: "开头是产品空镜"}, Source: SourceAI, ReviewState: ReviewConfirmed},
	}

	comparison := findComparison(t, buildPerformanceAnalysis(testWindow(10), facts, features), "asset_a", "asset_b")
	if len(comparison.ChangedFeatures) != 1 {
		t.Fatalf("自由文本不应计入变量，实际变量 %d 个", len(comparison.ChangedFeatures))
	}
}

// 被人工拒绝的 AI 特征不参与变量识别。
func TestRejectedFeatureIsIgnored(t *testing.T) {
	facts := append(
		factsFor("asset_a", "A", AssetTypePrerollAd, "obj_a", 10, MetricCounts{Impressions: 4000, Clicks: 200}),
		factsFor("asset_b", "B", AssetTypePrerollAd, "obj_b", 10, MetricCounts{Impressions: 4000, Clicks: 80})...,
	)
	rejected := enumFeature("asset_a", AssetTypePrerollAd, "cta_type", "立即购买")
	rejected.ReviewState = ReviewRejected
	features := []AssetFeature{
		enumFeature("asset_a", AssetTypePrerollAd, "preroll_hook_type", "露脸"),
		enumFeature("asset_b", AssetTypePrerollAd, "preroll_hook_type", "不露脸"),
		rejected,
		enumFeature("asset_b", AssetTypePrerollAd, "cta_type", "了解更多"),
	}

	comparison := findComparison(t, buildPerformanceAnalysis(testWindow(10), facts, features), "asset_a", "asset_b")
	for _, changed := range comparison.ChangedFeatures {
		if changed.Key == "cta_type" && changed.Baseline != "（未记录）" {
			t.Fatalf("被拒绝的 AI 特征不该出现在变量里：%+v", changed)
		}
	}
}

// 口径不一致时，可归因降级为方向性——差异可能全部来自口径。
func TestCaliberConflictDowngradesAttribution(t *testing.T) {
	facts := append(
		factsFor("asset_a", "A", AssetTypePrerollAd, "obj_a", 10, MetricCounts{Impressions: 4000, Clicks: 200}),
		factsFor("asset_b", "B", AssetTypePrerollAd, "obj_b", 10, MetricCounts{Impressions: 4000, Clicks: 80})...,
	)
	facts[0].Caliber.Currency = "USD"
	features := []AssetFeature{
		enumFeature("asset_a", AssetTypePrerollAd, "preroll_hook_type", "露脸"),
		enumFeature("asset_b", AssetTypePrerollAd, "preroll_hook_type", "不露脸"),
	}

	analysis := buildPerformanceAnalysis(testWindow(10), facts, features)
	if analysis.Comparable {
		t.Fatal("混了两种币种时 Comparable 应为 false")
	}
	comparison := findComparison(t, analysis, "asset_a", "asset_b")
	if comparison.Verdict == VerdictAttributable {
		t.Fatalf("口径不一致时不该出现可归因判定，实际 %q", comparison.Verdict)
	}
}

// 疲劳判定必须带上没能排除的其他解释，其中受众变化永远排除不了。
func TestFatigueAlwaysListsAudienceAsUnexcluded(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	var facts []MetricFactWithMapping
	for index := 0; index < 10; index++ {
		counts := MetricCounts{Impressions: 2000, Clicks: 200, SpendCents: 5000}
		if index >= 5 {
			// 后半段：曝光放大、点击率腰斩——03 §7.4 点名的典型疲劳形态。
			counts = MetricCounts{Impressions: 4000, Clicks: 80, SpendCents: 10000}
		}
		facts = append(facts, MetricFactWithMapping{
			MetricFact: MetricFact{
				Platform: PlatformDouyin, PlatformObjectKind: "creative", PlatformObjectID: "obj_a",
				StatDate: start.AddDate(0, 0, index), Counts: counts,
				Caliber: MetricCaliber{Currency: "CNY", AttributionWindow: "7d", MetricSchemaVersion: "v1"},
			},
			AssetID: "asset_a", AssetTitle: "A", AssetType: AssetTypePrerollAd, MappingStatus: MappingMatched,
		})
	}

	analysis := buildPerformanceAnalysis(testWindow(10), facts, nil)
	if len(analysis.Fatigue) != 1 {
		t.Fatalf("应有一条疲劳记录，实际 %d", len(analysis.Fatigue))
	}
	signal := analysis.Fatigue[0]
	if signal.Severity != FatigueLikely {
		t.Fatalf("曝光放大 + 点击率下滑应判为 likely，实际 %q（%s）", signal.Severity, signal.Note)
	}
	var mentionsAudience bool
	for _, reason := range signal.AlternativeExplanations {
		if strings.Contains(reason, "受众变化") {
			mentionsAudience = true
		}
	}
	if !mentionsAudience {
		t.Fatalf("疲劳结论必须写明受众变化排除不了，实际 %v", signal.AlternativeExplanations)
	}
}

// 天数不足时不能出现「没有疲劳迹象 · 置信充分」——曝光量再大也换不来天数。
func TestFatigueWithoutEnoughDaysIsLowSample(t *testing.T) {
	facts := factsFor("asset_a", "A", AssetTypePrerollAd, "obj_a", 3,
		MetricCounts{Impressions: 40000, Clicks: 2000, SpendCents: 100000, Conversions: 100})

	analysis := buildPerformanceAnalysis(testWindow(3), facts, nil)
	if len(analysis.Fatigue) != 1 {
		t.Fatalf("应有一条疲劳记录，实际 %d", len(analysis.Fatigue))
	}
	signal := analysis.Fatigue[0]
	if signal.Severity != FatigueNone {
		t.Fatalf("天数不足时不该给出疲劳判定，实际 %q", signal.Severity)
	}
	if signal.Confidence != ConfidenceLowSample {
		t.Fatalf("天数不足时置信必须是 low_sample，实际 %q——曝光 4 万也不代表查过了", signal.Confidence)
	}
}

// 驱动因素必须报告同向变化的其他特征，否则它会被当成因果读。
func TestDriverReportsCovaryingFeatures(t *testing.T) {
	var facts []MetricFactWithMapping
	// 两组各两个素材：组内 hook 和 cta 完全绑在一起变化。
	for _, item := range []struct {
		assetID string
		clicks  int64
	}{{"asset_a", 200}, {"asset_b", 190}, {"asset_c", 80}, {"asset_d", 70}} {
		facts = append(facts, factsFor(item.assetID, item.assetID, AssetTypePrerollAd,
			"obj_"+item.assetID, 10, MetricCounts{Impressions: 4000, Clicks: item.clicks, SpendCents: 10000})...)
	}
	features := []AssetFeature{
		enumFeature("asset_a", AssetTypePrerollAd, "preroll_hook_type", "露脸"),
		enumFeature("asset_b", AssetTypePrerollAd, "preroll_hook_type", "露脸"),
		enumFeature("asset_c", AssetTypePrerollAd, "preroll_hook_type", "不露脸"),
		enumFeature("asset_d", AssetTypePrerollAd, "preroll_hook_type", "不露脸"),
		enumFeature("asset_a", AssetTypePrerollAd, "cta_type", "立即购买"),
		enumFeature("asset_b", AssetTypePrerollAd, "cta_type", "立即购买"),
		enumFeature("asset_c", AssetTypePrerollAd, "cta_type", "了解更多"),
		enumFeature("asset_d", AssetTypePrerollAd, "cta_type", "了解更多"),
	}

	analysis := buildPerformanceAnalysis(testWindow(10), facts, features)
	var checked int
	for _, driver := range analysis.Drivers {
		if driver.Key != "preroll_hook_type" {
			continue
		}
		checked++
		if len(driver.CovaryingFeatures) == 0 {
			t.Fatalf("cta_type 与 hook 完全同向变化，必须登记为混杂来源，实际 %+v", driver)
		}
		if driver.Confidence != ConfidenceConfounded {
			t.Fatalf("存在同向变化的特征时置信应为 confounded，实际 %q", driver.Confidence)
		}
	}
	if checked == 0 {
		t.Fatal("没有生成 preroll_hook_type 的驱动因素行")
	}
}

// 分母为零时相对变化必须是空，不能退化成 0（doc10 §6）。
func TestRelativeChangeReturnsNilWithoutBaseline(t *testing.T) {
	zero := 0.0
	one := 1.0
	if relativeChange(&zero, &one) != nil {
		t.Fatal("基线为 0 时相对变化应为空")
	}
	if relativeChange(nil, &one) != nil {
		t.Fatal("没有基线时相对变化应为空")
	}
}

// 区间算不出来时按「重叠」处理：不知道差异是否显著，就不能说它显著。
func TestUnknownIntervalCountsAsOverlap(t *testing.T) {
	if !intervalsOverlap(nil, &RateInterval{Low: 0.1, High: 0.2}) {
		t.Fatal("缺少一侧区间时应视为重叠")
	}
}

// 单条素材某天曝光暴涨，项目总量看不出来（被其他素材摊平），但它必须被报出来——
// 那一天是解释这条素材表现时最先要排除的干扰。
func TestAssetLevelSpikeIsReportedEvenWhenProjectTotalLooksNormal(t *testing.T) {
	// 大素材：量级高、每天小幅抖动，负责把项目总量的波动撑起来，
	// 好让小素材那一天的暴涨在项目口径上看不出来。
	big := factsFor("asset_big", "大盘素材", AssetTypePrerollAd, "obj_big", 20,
		MetricCounts{Impressions: 100000, Clicks: 3000, SpendCents: 700000, Conversions: 150})
	for index := range big {
		big[index].Counts.Impressions += int64(index%5) * 9000
		big[index].Counts.SpendCents += int64(index%5) * 63000
	}
	small := factsFor("asset_small", "小素材", AssetTypePrerollAd, "obj_small", 20,
		MetricCounts{Impressions: 4000, Clicks: 120, SpendCents: 29000, Conversions: 6})
	for index := range small {
		small[index].Counts.Impressions += int64(index%4) * 130
	}
	// 第 12 天被转了一次，曝光 4.5 倍。
	small[12].Counts.Impressions = 18000
	facts := append(append([]MetricFactWithMapping{}, big...), small...)

	analysis := buildPerformanceAnalysis(testWindow(20), facts, nil)

	var found *MetricAnomaly
	for index := range analysis.Anomalies {
		item := analysis.Anomalies[index]
		if item.AssetID == "asset_small" && item.Kind == AnomalySpike {
			found = &analysis.Anomalies[index]
		}
		if item.AssetID == "asset_big" {
			t.Fatalf("大素材只是常规抖动，不该被判为异常：%+v", item)
		}
	}
	if found == nil {
		t.Fatal("小素材第 12 天曝光 4.5 倍，必须报为素材级 spike")
	}
	if found.Scope != "asset" || found.Metric != "impressions" {
		t.Fatalf("素材级异常的 scope/metric 应为 asset/impressions，实际 %q/%q", found.Scope, found.Metric)
	}
	if found.Deviation < 3.5 {
		t.Fatalf("报出来的偏离度必须真的过阈值，实际 %.2f", found.Deviation)
	}
}

// 序列完全没有波动时不能算异常：那说明它是被四舍五入或补录填出来的，
// 拿它当基准会把每一处正常起伏都判成异常。
func TestFlatAssetSeriesProducesNoSpike(t *testing.T) {
	facts := factsFor("asset_flat", "恒定素材", AssetTypePrerollAd, "obj_flat", 20,
		MetricCounts{Impressions: 5000, Clicks: 150, SpendCents: 35000, Conversions: 8})
	facts[7].Counts.Impressions = 25000

	analysis := buildPerformanceAnalysis(testWindow(20), facts, nil)
	for _, item := range analysis.Anomalies {
		if item.Kind == AnomalySpike || item.Kind == AnomalyDrop {
			t.Fatalf("常态零波动时不应产出突变异常，实际 %+v", item)
		}
	}
}

// 投放量在窗口中间上了一个台阶时，台阶另一侧的每一天都会偏离中位数。
// 那是一件事，不是十几件事：只报最极端的一天，并且备注必须指向「整体变过一次」。
func TestLevelShiftCollapsesIntoOneAnomalyWithAnExplanation(t *testing.T) {
	facts := factsFor("asset_step", "中途加投的素材", AssetTypePrerollAd, "obj_step", 20,
		MetricCounts{Impressions: 9000, Clicks: 380, SpendCents: 63000, Conversions: 20})
	for index := range facts {
		facts[index].Counts.Impressions += int64(index%3) * 300
		if index >= 8 {
			// 第 8 天起加了一个投放计划，曝光整体抬了一级。
			facts[index].Counts.Impressions += 6400
		}
	}

	analysis := buildPerformanceAnalysis(testWindow(20), facts, nil)

	drops := make([]MetricAnomaly, 0, 4)
	for _, item := range analysis.Anomalies {
		if item.AssetID == "asset_step" && item.Kind == AnomalyDrop {
			drops = append(drops, item)
		}
	}
	if len(drops) != 1 {
		t.Fatalf("台阶另一侧应该收敛成一条，实际 %d 条", len(drops))
	}
	if !strings.Contains(drops[0].Note, "整体变过一次") {
		t.Fatalf("多天同向偏离时备注必须指向投放量整体变化，实际 %q", drops[0].Note)
	}
}

// 一个特征只有两个取值时，「A 组高」和「其余组低」是同一句话说两遍。
// 只能出一行，否则读者会以为发现了两件事。
func TestTwoValuedFeatureProducesOneDriverRow(t *testing.T) {
	facts := make([]MetricFactWithMapping, 0, 80)
	features := make([]AssetFeature, 0, 8)
	for index, spec := range []struct {
		id    string
		hook  string
		daily MetricCounts
	}{
		{"asset_1", "利益", MetricCounts{Impressions: 9000, Clicks: 220, SpendCents: 63000, Conversions: 11}},
		{"asset_2", "利益", MetricCounts{Impressions: 8600, Clicks: 210, SpendCents: 60000, Conversions: 10}},
		{"asset_3", "问题", MetricCounts{Impressions: 9200, Clicks: 400, SpendCents: 64000, Conversions: 22}},
		{"asset_4", "问题", MetricCounts{Impressions: 8800, Clicks: 390, SpendCents: 61000, Conversions: 21}},
	} {
		facts = append(facts, factsFor(spec.id, spec.id, AssetTypePrerollAd,
			"obj_"+strconv.Itoa(index), 14, spec.daily)...)
		features = append(features, enumFeature(spec.id, AssetTypePrerollAd, "hook_type", spec.hook))
	}

	analysis := buildPerformanceAnalysis(testWindow(14), facts, features)

	rows := make([]FeatureDriver, 0, 2)
	for _, driver := range analysis.Drivers {
		if driver.Key == "hook_type" {
			rows = append(rows, driver)
		}
	}
	if len(rows) != 1 {
		values := make([]string, 0, len(rows))
		for _, row := range rows {
			values = append(values, row.Value)
		}
		t.Fatalf("两个取值只该出一行，实际 %d 行：%v", len(rows), values)
	}
	if rows[0].RestAssets != minDriverAssets {
		t.Fatalf("留下的那一行必须以另一个取值为对照组，实际对照 %d 个素材", rows[0].RestAssets)
	}
}
