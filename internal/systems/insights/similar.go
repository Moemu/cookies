package insights

import "sort"

// 找相似素材：❓「算不出来」的升级通道。
//
// 一个变量在本轮只有三条素材、几百次展示，算不出任何东西。但库里可能还有十几条
// 同样是这个取值的素材——把它们拉进来，样本就够了。这是「样本永远不够」这个
// 前提下唯一能做的事。
//
// **用特征重叠，不用向量。** 向量能给出更细的相似，但只能给一个分数；
// 这批素材是要被拿去做归因的，说不出「像在哪」就等于说不出为什么能凑成一组，
// 而一条说不出理由的推荐，在复盘会上没人敢用。

// FeatureProbe 是检索的探针：变量键到取值。取值是 featureValueText 归一化之后的
// 那一份，和素材对比、驱动因素用的是同一份——否则同一条素材在两个页面上
// 「像不像」会打架。
type FeatureProbe map[string]string

// SimilarityReason 是「像在哪」的一条。带 Source 是因为读的人有权知道
// 这一条相似是量出来的、人标的，还是模型猜的。
type SimilarityReason struct {
	Key    string        `json:"key"`
	Label  string        `json:"label"`
	Value  string        `json:"value"`
	Source FeatureSource `json:"source"`
}

// SimilarAsset 是一条检索结果。
type SimilarAsset struct {
	AssetID string `json:"asset_id"`
	Title   string `json:"title"`

	// Overlap 是重叠的变量数，AdmissibleOverlap 是其中能进归因的那些
	// （derived / human）。两个都给，因为它们回答的是不同的问题：
	// 前者是「像不像」，后者是「拉进来之后能不能真的做归因」。
	Overlap           int `json:"overlap"`
	AdmissibleOverlap int `json:"admissible_overlap"`

	// Score 是重叠数占探针变量数的比例，只用于展示「像到什么程度」。
	Score   float64            `json:"score"`
	Reasons []SimilarityReason `json:"reasons"`
}

// scoreSimilarity 算一个候选和探针的重叠。它只填 Overlap / AdmissibleOverlap /
// Score / Reasons，素材 ID 与标题由调用方补——这里拿到的是一袋变量，
// 它不知道这袋变量是谁的。
func scoreSimilarity(probe FeatureProbe, candidate map[string]AssetFeature) SimilarAsset {
	result := SimilarAsset{Reasons: make([]SimilarityReason, 0, len(probe))}
	if len(probe) == 0 {
		return result
	}
	// 按键排序遍历，让理由的顺序稳定——同一次检索两次打开顺序不一样，
	// 人会以为结果变了。
	keys := make([]string, 0, len(probe))
	for key := range probe {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		feature, present := candidate[key]
		if !present || !comparableKind(feature.Value.Kind) {
			continue
		}
		text := featureValueText(feature.Value)
		if text == "" || text != probe[key] {
			continue
		}
		result.Overlap++
		if feature.Source.AdmissibleForAttribution() {
			result.AdmissibleOverlap++
		}
		result.Reasons = append(result.Reasons, SimilarityReason{
			Key: key, Label: fieldOf(feature.AssetType, key).Label,
			Value: text, Source: feature.Source,
		})
	}
	result.Score = float64(result.Overlap) / float64(len(probe))
	return result
}

// rankSimilar 排序并截断。可归因重叠优先——人拿这批素材去做归因，
// 排前面的应该是最能支撑结论的那些。同分按素材 ID 排，保证结果稳定。
//
// 一个重叠都没有的候选直接扔掉：给一个「最不差的」出来比不给更糟，
// 人会以为系统找到了东西。
func rankSimilar(values []SimilarAsset, limit int) []SimilarAsset {
	ranked := make([]SimilarAsset, 0, len(values))
	for _, value := range values {
		if value.Overlap > 0 {
			ranked = append(ranked, value)
		}
	}
	sort.SliceStable(ranked, func(left, right int) bool {
		if ranked[left].AdmissibleOverlap != ranked[right].AdmissibleOverlap {
			return ranked[left].AdmissibleOverlap > ranked[right].AdmissibleOverlap
		}
		if ranked[left].Overlap != ranked[right].Overlap {
			return ranked[left].Overlap > ranked[right].Overlap
		}
		return ranked[left].AssetID < ranked[right].AssetID
	})
	if limit > 0 && len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked
}
