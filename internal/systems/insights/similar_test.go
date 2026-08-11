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
