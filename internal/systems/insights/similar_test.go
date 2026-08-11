package insights

import "testing"

func probeFeature(key, value string, source FeatureSource) AssetFeature {
	return AssetFeature{
		Key: key, Source: source, ReviewState: ReviewConfirmed,
		Value: FeatureValue{Kind: FeatureKindEnum, Terms: []string{value}},
	}
}

// 相似度是「重叠了几个变量」，不是一个不可解释的分数。
//
// 用向量的话结果只有一个 0.87，人问「像在哪」答不上来——而这批素材是要被拿去
// 做归因的，说不出像在哪就等于说不出为什么能凑成一组。
func TestSimilarityCountsOverlappingFeatures(t *testing.T) {
	t.Parallel()

	probe := FeatureProbe{"duration": "15s", "opening": "face", "bgm": "upbeat"}
	candidate := map[string]AssetFeature{
		"duration": probeFeature("duration", "15s", SourceHuman),
		"opening":  probeFeature("opening", "face", SourceHuman),
		"bgm":      probeFeature("bgm", "quiet", SourceHuman),
	}

	got := scoreSimilarity(probe, candidate)
	if got.Overlap != 2 {
		t.Errorf("重叠数应该是 2，得到 %d", got.Overlap)
	}
	if len(got.Reasons) != 2 {
		t.Fatalf("每个重叠都要给出理由，得到 %d 条", len(got.Reasons))
	}
	// 理由必须说清是哪个变量取什么值，「相似度 0.67」不算理由。
	if got.Reasons[0].Key == "" || got.Reasons[0].Value == "" {
		t.Errorf("理由缺变量或取值：%+v", got.Reasons[0])
	}
	// 变量在特征体系里没登记时，标签兜底成键名——界面上宁可显示一个英文键，
	// 也不能显示一个空白的「 = face」。
	if got.Reasons[0].Label == "" {
		t.Errorf("理由缺标签：%+v", got.Reasons[0])
	}
}

// 模型推断的变量能用来找候选，但不能算进「可归因重叠」。
//
// 找候选和做归因是两件事：找的时候宽一点没坏处，最多多看几个；
// 归因的时候松一格，结论就建立在一个没人核过的推断上。
func TestSimilarityCountsAdmissibleOverlapSeparately(t *testing.T) {
	t.Parallel()

	probe := FeatureProbe{"duration": "15s", "mood": "warm"}
	candidate := map[string]AssetFeature{
		"duration": probeFeature("duration", "15s", SourceDerived),
		"mood":     probeFeature("mood", "warm", SourceAI),
	}

	got := scoreSimilarity(probe, candidate)
	if got.Overlap != 2 {
		t.Errorf("总重叠应该是 2，得到 %d", got.Overlap)
	}
	if got.AdmissibleOverlap != 1 {
		t.Errorf("可归因重叠应该只算 duration 一个，得到 %d", got.AdmissibleOverlap)
	}
}

// 完全不重叠的候选分数为 0，不进结果。给一个「最不差的」出来比不给更糟：
// 人会以为系统找到了东西。
func TestSimilarityIsZeroWhenNothingOverlaps(t *testing.T) {
	t.Parallel()

	got := scoreSimilarity(FeatureProbe{"duration": "15s"}, map[string]AssetFeature{
		"duration": probeFeature("duration", "60s", SourceHuman),
	})
	if got.Overlap != 0 || got.Score != 0 {
		t.Errorf("不重叠应该是 0 分，得到 %+v", got)
	}
}

// 自由文本不当变量用。两条素材的文案一字不差才算「重叠」，那种巧合没有意义；
// 而只要差一个字就不算，这一栏对找相似永远不起作用。素材对比那边同样把它排除
// 在外（comparableKind），两处得一致，否则同一条素材在两个页面上像不像会打架。
func TestSimilarityIgnoresFreeText(t *testing.T) {
	t.Parallel()

	got := scoreSimilarity(FeatureProbe{"cover_subject": "一只猫"}, map[string]AssetFeature{
		"cover_subject": {Key: "cover_subject", Source: SourceHuman,
			Value: FeatureValue{Kind: FeatureKindText, Text: "一只猫"}},
	})
	if got.Overlap != 0 {
		t.Errorf("自由文本不该算重叠，得到 %d", got.Overlap)
	}
}

// 可归因重叠多的排前面，哪怕总重叠一样多。人拿这批素材去做归因，
// 排前面的应该是最能支撑结论的那些。
func TestRankPrefersAdmissibleOverlap(t *testing.T) {
	t.Parallel()

	ranked := rankSimilar([]SimilarAsset{
		{AssetID: "b", Overlap: 3, AdmissibleOverlap: 1},
		{AssetID: "a", Overlap: 3, AdmissibleOverlap: 3},
	}, 10)
	if ranked[0].AssetID != "a" {
		t.Errorf("可归因重叠多的应该排前面，得到 %q", ranked[0].AssetID)
	}
}

// 排序必须稳定。同分的两条今天这个在前、明天那个在前，人会以为数据变了。
func TestRankIsStableForTies(t *testing.T) {
	t.Parallel()

	first := rankSimilar([]SimilarAsset{
		{AssetID: "b", Overlap: 2, AdmissibleOverlap: 2},
		{AssetID: "a", Overlap: 2, AdmissibleOverlap: 2},
	}, 10)
	if first[0].AssetID != "a" {
		t.Errorf("同分按素材 ID 排，得到 %q", first[0].AssetID)
	}
}

// 一个重叠都没有的候选不进结果。0 分的东西被排在最后面和不出现，
// 在界面上是两件事：出现了，人就会去点它。
func TestRankDropsZeroOverlap(t *testing.T) {
	t.Parallel()

	ranked := rankSimilar([]SimilarAsset{
		{AssetID: "a", Overlap: 0, AdmissibleOverlap: 0},
		{AssetID: "b", Overlap: 1, AdmissibleOverlap: 0},
	}, 10)
	if len(ranked) != 1 || ranked[0].AssetID != "b" {
		t.Errorf("只该留下有重叠的那条，得到 %+v", ranked)
	}
}

// 被人否掉的推断不算重叠。人看过、说了「不是这样」，它就不该再把两条素材
// 凑成一组——素材对比那边也是这么办的。
func TestEffectiveFeaturesDropRejectedRows(t *testing.T) {
	t.Parallel()

	rejected := probeFeature("opening", "face", SourceHuman)
	rejected.AssetID, rejected.ReviewState = "a", ReviewRejected

	byAsset := effectiveAssetFeatures([]AssetFeature{rejected})
	if _, present := byAsset["a"]["opening"]; present {
		t.Error("被否掉的标注不该进有效变量")
	}
}

// 人复核认可过的 AI 推断，从此按人工标注算——有人为它背书了。
// 不这么办的话，「人工复核」这道工序对归因毫无意义：复核完了还是进不了结论。
func TestEffectiveFeaturesUpgradeConfirmedInference(t *testing.T) {
	t.Parallel()

	inferred := probeFeature("opening", "face", SourceAI)
	inferred.AssetID, inferred.ReviewState = "a", ReviewConfirmed

	got := effectiveAssetFeatures([]AssetFeature{inferred})["a"]["opening"]
	if !got.Source.AdmissibleForAttribution() {
		t.Errorf("认可过的推断应该能进归因，得到 source=%s", got.Source)
	}
}

// 同一个变量人标过也 AI 猜过，以人标的为准。
func TestEffectiveFeaturesPreferTheHumanLayer(t *testing.T) {
	t.Parallel()

	inferred := probeFeature("opening", "face", SourceAI)
	inferred.AssetID, inferred.ReviewState = "a", ReviewPending
	authored := probeFeature("opening", "product", SourceHuman)
	authored.AssetID, authored.ReviewState = "a", ReviewAuthored

	got := effectiveAssetFeatures([]AssetFeature{inferred, authored})["a"]["opening"]
	if featureValueText(got.Value) != "product" {
		t.Errorf("人标的应该盖过推断，得到 %q", featureValueText(got.Value))
	}
}

func TestSimilarAssetRequestValidation(t *testing.T) {
	t.Parallel()

	byAsset := SimilarAssetRequest{AssetID: "asset_1"}
	if err := byAsset.Validate(); err != nil {
		t.Fatalf("按素材找相似应该合法：%v", err)
	}

	byFeature := SimilarAssetRequest{Features: map[string]string{"duration": "15s"}}
	if err := byFeature.Validate(); err != nil {
		t.Fatalf("按变量找相似应该合法：%v", err)
	}

	// 两个都不给就等于「把库里所有素材列出来」——那是素材列表，不是找相似。
	if err := (SimilarAssetRequest{}).Validate(); err == nil {
		t.Error("既没有素材也没有变量的请求应该被拒")
	}

	// 变量太多会退化成「找那一条素材自己」，没有意义。
	tooMany := SimilarAssetRequest{Features: map[string]string{}}
	for index := 0; index < maxProbeFeatures+1; index++ {
		tooMany.Features[string(rune('a'+index))] = "x"
	}
	if err := tooMany.Validate(); err == nil {
		t.Error("变量超过 20 个应该被拒")
	}
}

// 默认条数要有上限。不限的话一个常见取值能拉回几百条，人在界面上根本挑不过来，
// 而且这批素材是要被拿去重算归因的，几百条会让那次计算变得很慢。
func TestSimilarAssetRequestDefaultsTheLimit(t *testing.T) {
	t.Parallel()

	request := SimilarAssetRequest{AssetID: "asset_1"}
	if got := request.effectiveLimit(); got != defaultSimilarLimit {
		t.Errorf("默认条数应该是 %d，得到 %d", defaultSimilarLimit, got)
	}
	over := SimilarAssetRequest{AssetID: "asset_1", Limit: 500}
	if got := over.effectiveLimit(); got != maxSimilarLimit {
		t.Errorf("超上限应该压到 %d，得到 %d", maxSimilarLimit, got)
	}
}

// 一条相似都建立在模型推断上时，必须说出来。不说的话，人会把它们当成和
// 人工标注一样可靠的样本，然后拿去撑一个结论。
func TestSimilarNoteWarnsWhenNothingIsAdmissible(t *testing.T) {
	t.Parallel()

	if note := similarNote([]SimilarAsset{{AssetID: "a", Overlap: 2}}); note == "" {
		t.Error("全靠模型推断的结果必须带一句提醒")
	}
	if note := similarNote([]SimilarAsset{{AssetID: "a", Overlap: 2, AdmissibleOverlap: 1}}); note != "" {
		t.Errorf("有可归因重叠时不该再提醒，得到 %q", note)
	}
	if note := similarNote(nil); note == "" {
		t.Error("一条都没找到时要说清是「没有」，不是界面坏了")
	}
}

func TestRankRespectsLimit(t *testing.T) {
	t.Parallel()

	values := make([]SimilarAsset, 0, 20)
	for index := 0; index < 20; index++ {
		values = append(values, SimilarAsset{AssetID: string(rune('a' + index)), Overlap: 1, AdmissibleOverlap: 1})
	}
	if got := rankSimilar(values, 5); len(got) != 5 {
		t.Errorf("limit 没生效，得到 %d 条", len(got))
	}
}
