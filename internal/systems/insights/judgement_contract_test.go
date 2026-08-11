package insights

import (
	"encoding/json"
	"testing"
	"time"
)

// 三档要成为中轴，靠的不是「大家记得填」，而是这一条：JSON 里凡是出现了
// confidence，同一个对象里就必须出现 verdict。有人新加一个带 confidence 的
// 结构体却忘了内嵌 Judgement，这里会直接报出它在 JSON 里的路径。
func TestEveryConfidenceCarriesAVerdict(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	window := MetricWindow{Start: now.AddDate(0, 0, -30), End: now}

	overview := buildMetricOverview(window, nil, []DataSource{{ID: "ds-1"}}, now, ResolvedThresholds{})
	analysis := buildPerformanceAnalysis(window, nil, nil, ResolvedThresholds{})

	for name, value := range map[string]any{"MetricOverview": overview, "PerformanceAnalysis": analysis} {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("%s 序列化失败：%v", name, err)
		}
		var tree any
		if err := json.Unmarshal(raw, &tree); err != nil {
			t.Fatalf("%s 反序列化失败：%v", name, err)
		}
		assertVerdictAccompaniesConfidence(t, name, tree)
	}
}

func assertVerdictAccompaniesConfidence(t *testing.T, path string, node any) {
	t.Helper()
	switch typed := node.(type) {
	case map[string]any:
		if _, hasConfidence := typed["confidence"]; hasConfidence {
			if _, hasVerdict := typed["verdict"]; !hasVerdict {
				t.Errorf("%s 有 confidence 但没有 verdict——这个结构体没内嵌 Judgement", path)
			}
			if _, hasLabel := typed["verdict_label"]; !hasLabel {
				t.Errorf("%s 有 confidence 但没有 verdict_label", path)
			}
			// 阈值可写之后，一条结论不带版本号就说不清它是按什么标准算出来的。
			// 这个键必须出现在**每一处**判定上，不能只出现在屏级：发现是从各视图
			// 抄进复盘草稿的，中间那一层漏了，存下来的那条就永远查不到依据。
			if _, hasVersion := typed["threshold_version"]; !hasVersion {
				t.Errorf("%s 有 confidence 但没有 threshold_version——判定没记下它用的那一版阈值", path)
			}
		}
		for key, child := range typed {
			assertVerdictAccompaniesConfidence(t, path+"."+key, child)
		}
	case []any:
		for index, child := range typed {
			assertVerdictAccompaniesConfidence(t, path+"[]", child)
			_ = index
		}
	}
}

// verdict 这个键只能是三档。VariantComparison 原来也叫 verdict，装的是
// attributable/no_features 那五档；两个 verdict 挤在一个对象里，Go 会让外层
// 那个赢，屏幕上就出现「verdict=attributable」而三档静默消失——上面那条契约
// 测试还会照常通过。所以这里把取值也钉死。
func TestVerdictKeyOnlyEverHoldsTheThreeVerdicts(t *testing.T) {
	t.Parallel()

	// 要用真的有对比行的数据：Comparisons 为空的话，被遮蔽的 verdict 根本不会
	// 出现在 JSON 里，这条测试就成了摆设。
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

	analysis := buildPerformanceAnalysis(testWindow(10), facts, features, ResolvedThresholds{})
	if len(analysis.Comparisons) == 0 {
		t.Fatal("这组数据应该产出对比行，否则这条测试什么也没检查")
	}
	raw, err := json.Marshal(analysis)
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatalf("反序列化失败：%v", err)
	}
	allowed := map[string]bool{
		string(VerdictExplained): true,
		string(VerdictObserved):  true,
		string(VerdictUnclear):   true,
	}
	assertVerdictValuesAreThreeWay(t, "PerformanceAnalysis", tree, allowed)
}

func assertVerdictValuesAreThreeWay(t *testing.T, path string, node any, allowed map[string]bool) {
	t.Helper()
	switch typed := node.(type) {
	case map[string]any:
		if value, ok := typed["verdict"].(string); ok && !allowed[value] {
			t.Errorf("%s.verdict = %q，不是三档之一——多半是被同名字段遮蔽了", path, value)
		}
		for key, child := range typed {
			assertVerdictValuesAreThreeWay(t, path+"."+key, child, allowed)
		}
	case []any:
		for _, child := range typed {
			assertVerdictValuesAreThreeWay(t, path+"[]", child, allowed)
		}
	}
}

// 空窗口是最常见的情况：一条数据都没有。此时屏级档位必须是「算不出来」，
// 不能因为「没有任何一条弱结论」就默认成能归因。
func TestEmptyWindowScreenVerdictIsUnclear(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	window := MetricWindow{Start: now.AddDate(0, 0, -30), End: now}

	analysis := buildPerformanceAnalysis(window, nil, nil, ResolvedThresholds{})
	if analysis.Judgement.Verdict != VerdictUnclear {
		t.Errorf("空窗口的屏级档位是 %s，期望 %s", analysis.Judgement.Verdict, VerdictUnclear)
	}
}

// MetricOverview 原来用 confidence_note，别的结构体用 note。同一个意思两个键名，
// 前端就得写两套渲染。统一成 note。
func TestMetricOverviewUsesTheSharedNoteKey(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	window := MetricWindow{Start: now.AddDate(0, 0, -30), End: now}

	raw, err := json.Marshal(buildMetricOverview(window, nil, []DataSource{{ID: "ds-1"}}, now, ResolvedThresholds{}))
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("反序列化失败：%v", err)
	}
	if _, stale := decoded["confidence_note"]; stale {
		t.Error("confidence_note 还在——应该已经并进 Judgement 的 note")
	}
	if _, ok := decoded["note"]; !ok {
		t.Error("缺少 note")
	}
}
