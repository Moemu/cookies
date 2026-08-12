package insights

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

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

// effectiveAssetFeatures 把分层的特征行收敛成「每条素材每个变量到底算什么」。
//
// 三条规则和素材对比那边的 assignFeatures 一字不差，因为它们回答的是同一个问题：
//   - 被人否掉的推断出局——人看过、说了「不是这样」；
//   - 人标的盖过模型猜的——同一个键两层都有时以人为准；
//   - 人复核认可过的推断，从此按人工标注算，能进归因。不然「人工复核」这道
//     工序对归因就毫无意义：复核完了还是进不了结论。
//
// 两边没有合并成一个函数，是因为返回的形状不同：那边只要取值加来源，
// 这边还要留着整行，好从素材类型上查出变量的中文标签。改动其中一处时
// 另一处也要跟着改，similar_test.go 里的三条测试盯着这件事。
func effectiveAssetFeatures(features []AssetFeature) map[string]map[string]AssetFeature {
	byAsset := map[string]map[string]AssetFeature{}
	for _, feature := range features {
		if !feature.ReviewState.CountsTowardAnalysis() {
			continue
		}
		confirmed := feature.Source == SourceHuman || feature.ReviewState == ReviewConfirmed
		if byAsset[feature.AssetID] == nil {
			byAsset[feature.AssetID] = map[string]AssetFeature{}
		}
		existing, seen := byAsset[feature.AssetID][feature.Key]
		if seen && existing.Source == SourceHuman && !confirmed {
			continue
		}
		if confirmed {
			feature.Source = SourceHuman
		}
		byAsset[feature.AssetID][feature.Key] = feature
	}
	return byAsset
}

// 检索条数的上下限。不限的话一个常见取值能拉回几百条：界面上挑不过来，
// 而这批素材是要被拿去重算归因的，几百条会让那次计算变得很慢。
const (
	defaultSimilarLimit = 10
	maxSimilarLimit     = 50
	maxProbeFeatures    = 20
)

// similarCandidateLimit 是一次最多扫多少条素材。在库里素材上万之前，
// 全表扫一遍再在内存里算重叠，比先建一套索引简单得多，也不会算错。
// 超过这个数就该改成先按探针里最稀有的那个变量筛一次——留给那时候的人。
const similarCandidateLimit = 2000

// SimilarAssetRequest 有两种问法：
//   - 给 AssetID：「和这条素材像的还有哪些」，探针取这条素材自己的变量；
//   - 给 Features：「时长 15 秒的还有哪些」，这是 ❓ 升级通道用的那种。
type SimilarAssetRequest struct {
	AssetID  string            `json:"asset_id,omitempty"`
	Features map[string]string `json:"features,omitempty"`
	Limit    int               `json:"limit,omitempty"`
}

func (r SimilarAssetRequest) Validate() error {
	if r.AssetID == "" && len(r.Features) == 0 {
		return fmt.Errorf("%w: 找相似要么给一条素材，要么给一组变量", ErrInvalidRequest)
	}
	if len(r.Features) > maxProbeFeatures {
		return fmt.Errorf("%w: 一次最多按 %d 个变量找", ErrInvalidRequest, maxProbeFeatures)
	}
	return nil
}

func (r SimilarAssetRequest) effectiveLimit() int {
	if r.Limit <= 0 {
		return defaultSimilarLimit
	}
	if r.Limit > maxSimilarLimit {
		return maxSimilarLimit
	}
	return r.Limit
}

// SimilarAssetResult 把探针一起返回。人要看得见「系统是按哪几个变量找的」
// ——只给结果的话，找出一批看起来不像的东西时没人知道问题出在哪。
type SimilarAssetResult struct {
	Probe []SimilarityReason `json:"probe"`
	Items []SimilarAsset     `json:"items"`
	Note  string             `json:"note"`
}

// FindSimilarAssets 按变量重叠找相似素材。
func (s Service) FindSimilarAssets(ctx context.Context, actor contract.ActorContext,
	projectID contract.ProjectID, request SimilarAssetRequest) (SimilarAssetResult, error) {
	if err := s.assetsReady(actor, projectID, ScopeRead); err != nil {
		return SimilarAssetResult{}, err
	}
	if err := request.Validate(); err != nil {
		return SimilarAssetResult{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return SimilarAssetResult{}, err
	}

	probe, probeReasons, err := s.buildProbe(ctx, actor, projectID, request)
	if err != nil {
		return SimilarAssetResult{}, err
	}
	if len(probe) == 0 {
		// 这条素材一个变量都没提取过。这不是「没有相似素材」，是「还没法找」
		// ——两者在界面上必须说成不同的话，否则人会以为库里真的没有像的。
		return SimilarAssetResult{
			Probe: probeReasons, Items: []SimilarAsset{},
			Note: "这条素材还没有能用来比对的变量，没法按内容找相似。先去「素材 · 变量」把它的变量填上。",
		}, nil
	}

	assets, err := s.Assets.ListAssets(ctx, actor.OrganizationID, projectID,
		AssetFilter{Limit: similarCandidateLimit})
	if err != nil {
		return SimilarAssetResult{}, err
	}
	ids := make([]string, 0, len(assets))
	titles := make(map[string]string, len(assets))
	for _, asset := range assets {
		if asset.ID == request.AssetID {
			continue // 自己永远和自己最像，列出来只是噪音
		}
		ids = append(ids, asset.ID)
		titles[asset.ID] = asset.Title
	}
	if len(ids) == 0 {
		return SimilarAssetResult{Probe: probeReasons, Items: []SimilarAsset{},
			Note: "这个 Project 里还没有别的素材可比。"}, nil
	}

	features, err := s.Assets.ListAssetFeatures(ctx, actor.OrganizationID, projectID, ids, 0)
	if err != nil {
		return SimilarAssetResult{}, err
	}
	byAsset := effectiveAssetFeatures(features)

	scored := make([]SimilarAsset, 0, len(ids))
	for _, id := range ids {
		item := scoreSimilarity(probe, byAsset[id])
		item.AssetID, item.Title = id, titles[id]
		scored = append(scored, item)
	}
	items := rankSimilar(scored, request.effectiveLimit())

	return SimilarAssetResult{Probe: probeReasons, Items: items, Note: similarNote(items)}, nil
}

// buildProbe 组出这一次要按哪些变量找。两种问法可以叠加：给了素材又给了变量时，
// 显式给的变量说了算——人手填的那一个是他真正想问的，不能被素材自己的取值盖掉。
func (s Service) buildProbe(ctx context.Context, actor contract.ActorContext,
	projectID contract.ProjectID, request SimilarAssetRequest) (FeatureProbe, []SimilarityReason, error) {
	probe := FeatureProbe{}
	reasons := make([]SimilarityReason, 0, maxProbeFeatures)

	if request.AssetID != "" {
		features, err := s.Assets.ListAssetFeatures(ctx, actor.OrganizationID, projectID,
			[]string{request.AssetID}, 0)
		if err != nil {
			return nil, nil, err
		}
		byKey := effectiveAssetFeatures(features)[request.AssetID]
		keys := make([]string, 0, len(byKey))
		for key := range byKey {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			feature := byKey[key]
			if !comparableKind(feature.Value.Kind) {
				continue
			}
			text := featureValueText(feature.Value)
			if text == "" {
				continue
			}
			probe[key] = text
			reasons = append(reasons, SimilarityReason{
				Key: key, Label: fieldOf(feature.AssetType, key).Label,
				Value: text, Source: feature.Source,
			})
		}
	}

	keys := make([]string, 0, len(request.Features))
	for key := range request.Features {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := strings.TrimSpace(request.Features[key])
		if key == "" || value == "" {
			continue
		}
		if _, taken := probe[key]; taken {
			// 素材自己也有这个变量，但人显式问的是另一个取值，以人问的为准。
			reasons = dropReason(reasons, key)
		}
		probe[key] = value
		// 显式给的变量没有来源可言——它是一个问题，不是一条标注。标成人给的，
		// 因为按它找回来的结果确实由人负责。
		reasons = append(reasons, SimilarityReason{
			Key: key, Label: labelOfKey(key), Value: value, Source: SourceHuman,
		})
	}
	return probe, reasons, nil
}

func dropReason(reasons []SimilarityReason, key string) []SimilarityReason {
	kept := reasons[:0]
	for _, reason := range reasons {
		if reason.Key != key {
			kept = append(kept, reason)
		}
	}
	return kept
}

// labelOfKey 找出一个变量键在名词表里的中文名，不看素材类型。
//
// 显式按变量找的时候没有类型上下文（问的是「时长 15 秒的还有哪些」，不是
// 「和这条素材像的」）。退回原始键名的话，探针那一行会写成
// 「按这些变量找的：target_duration=15」——人看不出那是「目标时长」。
// 六套特征体系里同名的键说的是同一件事，所以取第一个命中的即可。
func labelOfKey(key string) string {
	for _, schema := range AllFeatureSchemas() {
		if field, found := schema.Field(key); found {
			return field.Label
		}
	}
	return key
}

func similarNote(items []SimilarAsset) string {
	if len(items) == 0 {
		return "库里没有在这些变量上和它一致的素材。"
	}
	for _, item := range items {
		if item.AdmissibleOverlap > 0 {
			return ""
		}
	}
	// 这批的相似全建立在模型推断的变量上。拉进来能看，但不能拿去做归因
	// ——不说清楚的话，人会把它们当成和人工标注一样可靠的样本。
	return "找到的这些，相似之处全都建立在模型推断的变量上，只能参考，不能拿来做归因。"
}
