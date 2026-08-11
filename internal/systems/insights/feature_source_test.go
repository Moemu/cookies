package insights

import (
	"strings"
	"testing"
)

// 变量分三类，是因为它们的可信度根本不是一回事：时长是从文件里量出来的，
// 「情绪是否高涨」是模型猜的。把模型猜的东西放进归因结论，等于用一个猜测
// 去解释另一个猜测。
func TestOnlyMeasuredAndHumanFeaturesAreAdmissibleForAttribution(t *testing.T) {
	t.Parallel()

	cases := map[FeatureSource]bool{
		SourceDerived: true,
		SourceHuman:   true,
		SourceAI:      false,
	}
	for source, admissible := range cases {
		if got := source.AdmissibleForAttribution(); got != admissible {
			t.Errorf("%s 的归因准入是 %v，期望 %v", source, got, admissible)
		}
		if !source.valid() {
			t.Errorf("%s 应该是合法来源", source)
		}
	}
	if FeatureSource("guessed").valid() {
		t.Error("未知来源不该通过校验")
	}
}

func TestFeatureSourceLabels(t *testing.T) {
	t.Parallel()

	cases := map[FeatureSource]string{
		SourceDerived: "客观可测",
		SourceHuman:   "人工标注",
		SourceAI:      "模型推断",
	}
	for source, label := range cases {
		if got := source.Label(); got != label {
			t.Errorf("%s 的名字是 %q，期望 %q", source, got, label)
		}
	}
}

// 展示和归因用同一份特征，但准入不同：AI 提取的特征可以摆在页面上，
// 不能进结论。两个读取口分开，才不会有人顺手用错。
func TestAttributableFeatureRejectsModelGuesses(t *testing.T) {
	t.Parallel()

	slice := &assetSlice{features: map[string]featureCell{
		"duration": {value: "15s", source: SourceDerived},
		"tone":     {value: "高涨", source: SourceAI},
		"hook":     {value: "疑问句", source: SourceHuman},
	}}

	for key, want := range map[string]string{"duration": "15s", "tone": "高涨", "hook": "疑问句"} {
		if got, ok := slice.featureValue(key); !ok || got != want {
			t.Errorf("展示口读 %s 得到 (%q,%v)，期望 %q", key, got, ok, want)
		}
	}
	if _, ok := slice.attributableFeature("tone"); ok {
		t.Error("模型推断的特征不该进归因")
	}
	if got, ok := slice.attributableFeature("duration"); !ok || got != "15s" {
		t.Errorf("客观可测的特征应该能进归因，得到 (%q,%v)", got, ok)
	}
	if got, ok := slice.attributableFeature("hook"); !ok || got != "疑问句" {
		t.Errorf("人工标注的特征应该能进归因，得到 (%q,%v)", got, ok)
	}
}

// 「人工复核」这道工序必须对归因有意义：人看过并认可的推断，从此按人工标注算。
// 否则复核完了特征还是进不了结论，那道工序就白做了。
func TestConfirmedModelGuessCountsAsHumanForAttribution(t *testing.T) {
	t.Parallel()

	slices := map[string]*assetSlice{"a1": {features: map[string]featureCell{}}}
	assignFeatures(slices, []AssetFeature{{
		AssetID: "a1", Key: "hook_type", Source: SourceAI, ReviewState: ReviewConfirmed,
		Value: FeatureValue{Kind: FeatureKindEnum, Terms: []string{"问题"}},
	}})

	if got, ok := slices["a1"].attributableFeature("hook_type"); !ok || got != "问题" {
		t.Fatalf("人工认可过的推断应当能进归因，得到 (%q,%v)", got, ok)
	}
}

// 反过来，没人看过的推断只能展示。它和上面那条的差别只有一个 ReviewState——
// 这正是整条规则的支点，所以两条测试要成对存在。
func TestUnreviewedModelGuessStaysOutOfAttribution(t *testing.T) {
	t.Parallel()

	slices := map[string]*assetSlice{"a1": {features: map[string]featureCell{}}}
	assignFeatures(slices, []AssetFeature{{
		AssetID: "a1", Key: "hook_type", Source: SourceAI, ReviewState: ReviewPending,
		Value: FeatureValue{Kind: FeatureKindEnum, Terms: []string{"问题"}},
	}})

	if _, ok := slices["a1"].featureValue("hook_type"); !ok {
		t.Fatal("待复核的推断仍然要能展示")
	}
	if _, ok := slices["a1"].attributableFeature("hook_type"); ok {
		t.Fatal("没人看过的推断不该进归因")
	}
}

// 两个素材只在一个模型推断的变量上不同：差异照样列出来给人看，但不能因此
// 判成「可归因」。这是本条规则在页面上唯一看得见的地方。
func TestComparisonDoesNotAttributeToModelGuessedVariable(t *testing.T) {
	t.Parallel()

	guessed := func(term string) map[string]featureCell {
		return map[string]featureCell{"preroll_hook_type": {value: term, source: SourceAI}}
	}
	left := &assetSlice{assetID: "a1", title: "A", kind: AssetTypePrerollAd,
		total: MetricCounts{Impressions: 60000, Clicks: 3000}, features: guessed("问题")}
	right := &assetSlice{assetID: "a2", title: "B", kind: AssetTypePrerollAd,
		total: MetricCounts{Impressions: 60000, Clicks: 1200}, features: guessed("陈述")}

	result := compareAssets(left, right, true)

	if len(result.ChangedFeatures) != 1 {
		t.Fatalf("差异要照常列出来给人看，实际 %d 条", len(result.ChangedFeatures))
	}
	if diff := result.ChangedFeatures[0]; diff.Source != SourceAI || diff.Admissible {
		t.Errorf("这条差异应标为模型推断且不可归因，实际 source=%s admissible=%v", diff.Source, diff.Admissible)
	}
	if result.VariantVerdict == VerdictAttributable {
		t.Error("差异只出现在模型推断的变量上时不能判为可归因")
	}
	if result.Verdict == VerdictExplained {
		t.Error("三档也不能是「能归因」")
	}
	if !strings.Contains(result.Note, "模型推断") {
		t.Errorf("理由里要说清为什么归不了因，实际是 %q", result.Note)
	}
}
