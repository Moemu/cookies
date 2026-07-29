package insights

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// 投后分析的五个解释性视图：素材对比、趋势、疲劳、异常、驱动因素
// （22 §6.2「MVP 只保留真实指标总览，随后实现素材对比、趋势、疲劳、异常和驱动因素」）。
//
// 五个视图共用一次事实读取，因为它们回答的是同一批数据的五个不同问题——分五次拉会
// 出现「趋势里看到的和疲劳里算的不是同一份数据」这种没法解释的情况。
//
// 全篇的纪律只有一条：**能不能归因，和差异有多大，是两件事**。差异大而变量混杂时，
// 这里只出方向性观察，不出结论（03 §7.3）。因此每个结构都带 Confidence 和一段说明
// 混杂来源的文字，前端不允许只显示数字。

// PerformanceAnalysis 是 GET /projects/{id}/performance-analysis 的返回。
type PerformanceAnalysis struct {
	Window  MetricWindow  `json:"window"`
	Caliber MetricCaliber `json:"caliber"`
	// Comparable=false 时所有对比类结论都只是方向性的：口径不一致的数据放在
	// 一起比，比出来的差异可能全部来自口径本身。
	Comparable       bool   `json:"comparable"`
	ComparableReason string `json:"comparable_reason,omitempty"`

	Comparisons []VariantComparison `json:"comparisons"`
	Trends      []AssetTrend        `json:"trends"`
	Fatigue     []FatigueSignal     `json:"fatigue"`
	Anomalies   []MetricAnomaly     `json:"anomalies"`
	Drivers     []FeatureDriver     `json:"drivers"`

	// FeatureCoverage 说明有多少参与分析的素材真的有特征数据。
	// 没有特征就没有变量，素材对比会退化成「两个素材谁的数字大」。
	AssetsInWindow  int      `json:"assets_in_window"`
	AssetsWithFeats int      `json:"assets_with_features"`
	Notes           []string `json:"notes,omitempty"`
}

// FeatureDiff 是两个素材之间一个特征的取值差异，也就是 AM-009 说的「实验变量」。
type FeatureDiff struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Group     string `json:"group"`
	Baseline  string `json:"baseline"`
	Variant   string `json:"variant"`
	HumanOnly bool   `json:"human_only"`
}

// VariantVerdict 说明这一对素材的差异能不能归到某个变量上。
type VariantVerdict string

const (
	// VerdictAttributable：恰好一个变量不同，且样本足够、区间不重叠。
	VerdictAttributable VariantVerdict = "attributable"
	// VerdictDirectional：变量单一但区间重叠，只能说方向。
	VerdictDirectional VariantVerdict = "directional"
	// VerdictConfounded：不止一个变量不同，差异归不到任何单一变量上。
	VerdictConfounded VariantVerdict = "confounded"
	// VerdictLowSample：样本不够，先不谈差异。
	VerdictLowSample VariantVerdict = "low_sample"
	// VerdictNoFeatures：至少一边没有特征数据，连变量是什么都不知道。
	VerdictNoFeatures VariantVerdict = "no_features"
)

// VariantComparison 是素材对比的一行，同时承载 AM-009 变体分析。
type VariantComparison struct {
	BaselineAssetID string    `json:"baseline_asset_id"`
	BaselineTitle   string    `json:"baseline_title"`
	VariantAssetID  string    `json:"variant_asset_id"`
	VariantTitle    string    `json:"variant_title"`
	AssetType       AssetType `json:"asset_type,omitempty"`

	ChangedFeatures []FeatureDiff `json:"changed_features"`
	// ControlledCount 是两边取值相同的特征数——它是「控制住了多少」的度量。
	ControlledCount int `json:"controlled_count"`

	BaselineCounts MetricCounts `json:"baseline_counts"`
	VariantCounts  MetricCounts `json:"variant_counts"`
	BaselineRates  MetricRates  `json:"baseline_rates"`
	VariantRates   MetricRates  `json:"variant_rates"`

	BaselineCTRInterval *RateInterval `json:"baseline_ctr_interval,omitempty"`
	VariantCTRInterval  *RateInterval `json:"variant_ctr_interval,omitempty"`
	// IntervalsOverlap=true 表示两条置信区间重叠，差异可能只是噪声。
	IntervalsOverlap bool     `json:"intervals_overlap"`
	CTRLift          *float64 `json:"ctr_lift,omitempty"`

	Verdict    VariantVerdict  `json:"verdict"`
	Confidence ConfidenceLevel `json:"confidence"`
	Note       string          `json:"note"`
}

// AssetTrend 是一个素材在窗口内的逐日走势。
type AssetTrend struct {
	AssetID    string             `json:"asset_id"`
	AssetTitle string             `json:"asset_title"`
	AssetType  AssetType          `json:"asset_type,omitempty"`
	Points     []PerformancePoint `json:"points"`
	// ActiveDays 是真的有数据的天数。窗口 30 天但只投了 3 天，走势图会骗人。
	ActiveDays int    `json:"active_days"`
	Direction  string `json:"direction"`
	// CTRChange 是后半段相对前半段的相对变化。分母为零时为空，不退化成 0。
	CTRChange  *float64        `json:"ctr_change,omitempty"`
	Confidence ConfidenceLevel `json:"confidence"`
	Note       string          `json:"note"`
}

// FatigueSeverity 分三档，`none` 也会返回：知道「查过了，没有」比看不到这一行有用。
type FatigueSeverity string

const (
	FatigueNone   FatigueSeverity = "none"
	FatigueWatch  FatigueSeverity = "watch"
	FatigueLikely FatigueSeverity = "likely"
)

// FatigueSignal 对应 03 §7.4：识别曝光增大但点击/转化下降、成本恶化的趋势，
// 并且**必须区分数据延迟、受众变化、预算变化和真正的素材衰退**。
// 我们手上没有受众数据，区分不了的就明写在 AlternativeExplanations 里，
// 不假装排除过。
type FatigueSignal struct {
	AssetID    string    `json:"asset_id"`
	AssetTitle string    `json:"asset_title"`
	AssetType  AssetType `json:"asset_type,omitempty"`

	FirstHalf  MetricCounts `json:"first_half"`
	SecondHalf MetricCounts `json:"second_half"`
	FirstRates MetricRates  `json:"first_rates"`
	LastRates  MetricRates  `json:"last_rates"`

	CTRChange        *float64 `json:"ctr_change,omitempty"`
	CPAChange        *float64 `json:"cpa_change,omitempty"`
	ImpressionChange *float64 `json:"impression_change,omitempty"`

	Severity FatigueSeverity `json:"severity"`
	// AlternativeExplanations 是这次没能排除的其他解释。为空表示确实没有别的解释，
	// 不是「没检查」——检查项是固定的四类。
	AlternativeExplanations []string        `json:"alternative_explanations,omitempty"`
	Confidence              ConfidenceLevel `json:"confidence"`
	Note                    string          `json:"note"`
}

// AnomalyKind 说明这一天是怎么不对劲的。
type AnomalyKind string

const (
	AnomalySpike AnomalyKind = "spike"
	AnomalyDrop  AnomalyKind = "drop"
	AnomalyGap   AnomalyKind = "gap"
)

// MetricAnomaly 是窗口内某一天偏离常态的记录。判定用中位数和 MAD 而不是均值和
// 标准差：广告数据里一次大促就能把均值和标准差同时拉走，之后什么都不算异常了。
type MetricAnomaly struct {
	Date       string      `json:"date"`
	Scope      string      `json:"scope"`
	AssetID    string      `json:"asset_id,omitempty"`
	AssetTitle string      `json:"asset_title,omitempty"`
	Metric     string      `json:"metric"`
	Kind       AnomalyKind `json:"kind"`

	Observed float64 `json:"observed"`
	Median   float64 `json:"median"`
	// Deviation 是偏离中位数多少个 MAD。
	Deviation float64 `json:"deviation"`
	Note      string  `json:"note"`
}

// FeatureDriver 是「哪一类内容特征伴随更好的表现」。
//
// 它是相关，不是因果——同一个特征取值的素材往往还共享别的特征。
// CovaryingFeatures 就是这件事的证据：这一组素材在这些特征上也整齐地
// 和其他素材不同，差异不能只算到 FeatureKey 头上。
type FeatureDriver struct {
	AssetType AssetType `json:"asset_type,omitempty"`
	Key       string    `json:"key"`
	Label     string    `json:"label"`
	Group     string    `json:"group"`
	Value     string    `json:"value"`

	Assets     int          `json:"assets"`
	RestAssets int          `json:"rest_assets"`
	Counts     MetricCounts `json:"counts"`
	RestCounts MetricCounts `json:"rest_counts"`
	Rates      MetricRates  `json:"rates"`
	RestRates  MetricRates  `json:"rest_rates"`

	CTRInterval      *RateInterval `json:"ctr_interval,omitempty"`
	RestCTRInterval  *RateInterval `json:"rest_ctr_interval,omitempty"`
	IntervalsOverlap bool          `json:"intervals_overlap"`
	CTRLift          *float64      `json:"ctr_lift,omitempty"`

	CovaryingFeatures []string        `json:"covarying_features,omitempty"`
	Confidence        ConfidenceLevel `json:"confidence"`
	Note              string          `json:"note"`
}

// maxComparisonAssets 限制两两配对的规模。取花费最高的若干个素材配对，
// 其余的会在 Notes 里说清楚被排除了多少个——静默截断等于谎报覆盖面。
const maxComparisonAssets = 8

// GetPerformanceAnalysis 组装五个解释性视图。
func (s Service) GetPerformanceAnalysis(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, window MetricWindow) (PerformanceAnalysis, error) {
	if err := s.connectorsReady(actor, projectID, ScopeRead); err != nil {
		return PerformanceAnalysis{}, err
	}
	if err := s.assetsReady(actor, projectID, ScopeRead); err != nil {
		return PerformanceAnalysis{}, err
	}
	if window.End.Before(window.Start) {
		return PerformanceAnalysis{}, fmt.Errorf("%w: 数据窗口的结束日期早于开始日期", ErrInvalidRequest)
	}
	if window.Days() > maxWindowDays {
		return PerformanceAnalysis{}, fmt.Errorf("%w: 数据窗口最长 %d 天", ErrInvalidRequest, maxWindowDays)
	}
	facts, err := s.Connectors.ListMetricFacts(ctx, actor.OrganizationID, projectID, window)
	if err != nil {
		return PerformanceAnalysis{}, err
	}

	assetIDs := attributableAssetIDs(facts)
	var features []AssetFeature
	if len(assetIDs) > 0 {
		features, err = s.Assets.ListAssetFeatures(ctx, actor.OrganizationID, projectID, assetIDs, len(assetIDs)*64)
		if err != nil {
			return PerformanceAnalysis{}, err
		}
	}
	return buildPerformanceAnalysis(window, facts, features), nil
}

func attributableAssetIDs(facts []MetricFactWithMapping) []string {
	seen := map[string]struct{}{}
	ids := make([]string, 0, 16)
	for _, fact := range facts {
		if fact.AssetID == "" {
			continue
		}
		if _, ok := seen[fact.AssetID]; ok {
			continue
		}
		seen[fact.AssetID] = struct{}{}
		ids = append(ids, fact.AssetID)
	}
	sort.Strings(ids)
	return ids
}

// assetSlice 是一个素材在窗口内的全部事实，按日聚合后供五个视图复用。
type assetSlice struct {
	assetID string
	title   string
	kind    AssetType
	total   MetricCounts
	byDate  map[string]MetricCounts
	objects map[string]struct{}
	// features 只收「人工确认过的」和「AI 提取但没被拒绝的」，见 pickFeatures。
	features map[string]string
}

func (a *assetSlice) dates() []string {
	values := make([]string, 0, len(a.byDate))
	for date := range a.byDate {
		values = append(values, date)
	}
	sort.Strings(values)
	return values
}

func buildPerformanceAnalysis(window MetricWindow, facts []MetricFactWithMapping, features []AssetFeature) PerformanceAnalysis {
	analysis := PerformanceAnalysis{Window: window, Comparable: true}

	slices := map[string]*assetSlice{}
	projectByDate := map[string]MetricCounts{}
	currencies := map[string]struct{}{}
	attributions := map[string]struct{}{}
	schemas := map[string]struct{}{}
	platforms := map[Platform]struct{}{}

	for _, fact := range facts {
		date := fact.StatDate.Format("2006-01-02")
		projectByDate[date] = projectByDate[date].add(fact.Counts)
		currencies[fact.Caliber.Currency] = struct{}{}
		attributions[fact.Caliber.AttributionWindow] = struct{}{}
		schemas[fact.Caliber.MetricSchemaVersion] = struct{}{}
		platforms[fact.Platform] = struct{}{}
		if fact.AssetID == "" {
			continue
		}
		slice, ok := slices[fact.AssetID]
		if !ok {
			slice = &assetSlice{
				assetID: fact.AssetID, title: fact.AssetTitle, kind: fact.AssetType,
				byDate: map[string]MetricCounts{}, objects: map[string]struct{}{},
				features: map[string]string{},
			}
			slices[fact.AssetID] = slice
		}
		slice.total = slice.total.add(fact.Counts)
		slice.byDate[date] = slice.byDate[date].add(fact.Counts)
		slice.objects[string(fact.Platform)+"\x00"+fact.PlatformObjectKind+"\x00"+fact.PlatformObjectID] = struct{}{}
	}

	analysis.Caliber = MetricCaliber{
		Currency:            singleOr(currencies, "多币种"),
		AttributionWindow:   singleOr(attributions, "多归因窗口"),
		MetricSchemaVersion: singleOr(schemas, "多版本"),
		TimeZone:            "按各数据源账户时区聚合",
	}
	var reasons []string
	if len(currencies) > 1 {
		reasons = append(reasons, "窗口内混合了多种币种")
	}
	if len(attributions) > 1 {
		reasons = append(reasons, "窗口内混合了多种归因窗口")
	}
	if len(schemas) > 1 {
		reasons = append(reasons, "窗口内混合了多个指标口径版本")
	}
	if len(platforms) > 1 {
		reasons = append(reasons, "窗口内包含多个平台，平台之间的指标定义不同")
	}
	if len(reasons) > 0 {
		analysis.Comparable = false
		analysis.ComparableReason = strings.Join(reasons, "；")
	}

	assignFeatures(slices, features)

	ordered := make([]*assetSlice, 0, len(slices))
	for _, slice := range slices {
		ordered = append(ordered, slice)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].total.SpendCents != ordered[j].total.SpendCents {
			return ordered[i].total.SpendCents > ordered[j].total.SpendCents
		}
		return ordered[i].assetID < ordered[j].assetID
	})

	analysis.AssetsInWindow = len(ordered)
	for _, slice := range ordered {
		if len(slice.features) > 0 {
			analysis.AssetsWithFeats++
		}
	}

	analysis.Comparisons = buildComparisons(ordered, analysis.Comparable, &analysis.Notes)
	analysis.Trends = buildTrends(ordered)
	analysis.Fatigue = buildFatigue(ordered, window)
	analysis.Anomalies = buildAnomalies(projectByDate, ordered)
	analysis.Drivers = buildDrivers(ordered, analysis.Comparable)

	if analysis.AssetsInWindow == 0 {
		analysis.Notes = append(analysis.Notes, "窗口内没有任何能归到素材上的投放数据，五个视图都无从算起。先去数据接入把平台对象和素材对应起来。")
	} else if analysis.AssetsWithFeats == 0 {
		analysis.Notes = append(analysis.Notes, "窗口内的素材都还没有内容特征，素材对比只能比数字、驱动因素无从谈起。先去内容分析把特征提出来。")
	}
	if !analysis.Comparable {
		analysis.Notes = append(analysis.Notes, "口径不一致："+analysis.ComparableReason+"。这一页所有对比结论都只能当方向性观察。")
	}
	return analysis
}

// assignFeatures 把特征贴到素材上。同一个 key 有 AI 行和人工行时以人工为准
// （AM-006「人工结果不被后台覆盖」）；被拒绝的 AI 行直接丢掉——被人否掉的
// 推断不该继续参与变量识别。
func assignFeatures(slices map[string]*assetSlice, features []AssetFeature) {
	human := map[string]map[string]struct{}{}
	for _, feature := range features {
		slice, ok := slices[feature.AssetID]
		if !ok || feature.ReviewState == ReviewRejected {
			continue
		}
		if !comparableKind(feature.Value.Kind) {
			continue
		}
		text := featureValueText(feature.Value)
		if text == "" {
			continue
		}
		confirmed := feature.Source == SourceHuman || feature.ReviewState == ReviewConfirmed
		if _, taken := human[feature.AssetID][feature.Key]; taken && !confirmed {
			continue
		}
		if confirmed {
			if human[feature.AssetID] == nil {
				human[feature.AssetID] = map[string]struct{}{}
			}
			human[feature.AssetID][feature.Key] = struct{}{}
		}
		slice.features[feature.Key] = text
	}
}

func featureValueText(value FeatureValue) string {
	switch value.Kind {
	case FeatureKindTags, FeatureKindEnum, FeatureKindEnumMul:
		if len(value.Terms) == 0 {
			return strings.TrimSpace(value.Text)
		}
		terms := append([]string(nil), value.Terms...)
		sort.Strings(terms)
		return strings.Join(terms, "、")
	case FeatureKindBool:
		if value.Bool {
			return "是"
		}
		return "否"
	case FeatureKindNumber, FeatureKindDuration:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", value.Number), "0"), ".")
	default:
		return strings.TrimSpace(value.Text)
	}
}

// comparableKind 决定一个特征能不能当变量用。自由文本每个素材都不一样，
// 拿它当变量的话每一对素材都会「改了 N 个变量」，AM-009 的判定就永远是混杂。
// features.go 对 FeatureKindText 的注释写的就是 "not comparable across assets"。
func comparableKind(kind FeatureValueKind) bool {
	return kind != FeatureKindText
}

func fieldOf(kind AssetType, key string) FeatureField {
	if schema, ok := FeatureSchemaFor(kind); ok {
		if field, found := schema.Field(key); found {
			return field
		}
	}
	return FeatureField{Key: key, Label: key, Group: "未登记特征"}
}

// --- 素材对比 / 变体分析（AM-008、AM-009，03 §7.2 §7.3）---

func buildComparisons(ordered []*assetSlice, comparable bool, notes *[]string) []VariantComparison {
	pool := ordered
	if len(pool) > maxComparisonAssets {
		pool = pool[:maxComparisonAssets]
		*notes = append(*notes, fmt.Sprintf(
			"素材对比只配对了花费最高的 %d 个素材，窗口内另有 %d 个素材没有参与配对。",
			maxComparisonAssets, len(ordered)-maxComparisonAssets))
	}
	comparisons := make([]VariantComparison, 0, len(pool)*(len(pool)-1)/2)
	for i := 0; i < len(pool); i++ {
		for j := i + 1; j < len(pool); j++ {
			left, right := pool[i], pool[j]
			// 类型不同的素材没有共同的特征体系，比出来的差异连变量都对不齐。
			if left.kind != right.kind {
				continue
			}
			comparisons = append(comparisons, compareAssets(left, right, comparable))
		}
	}
	sort.Slice(comparisons, func(i, j int) bool {
		return verdictRank(comparisons[i].Verdict) < verdictRank(comparisons[j].Verdict)
	})
	return comparisons
}

// verdictRank 让能归因的排在前面，样本不足和无特征的沉底。
func verdictRank(verdict VariantVerdict) int {
	switch verdict {
	case VerdictAttributable:
		return 0
	case VerdictDirectional:
		return 1
	case VerdictConfounded:
		return 2
	case VerdictLowSample:
		return 3
	default:
		return 4
	}
}

func compareAssets(baseline, variant *assetSlice, comparable bool) VariantComparison {
	result := VariantComparison{
		BaselineAssetID: baseline.assetID, BaselineTitle: baseline.title,
		VariantAssetID: variant.assetID, VariantTitle: variant.title,
		AssetType:      baseline.kind,
		BaselineCounts: baseline.total, VariantCounts: variant.total,
		BaselineRates: RatesOf(baseline.total), VariantRates: RatesOf(variant.total),
	}
	result.BaselineCTRInterval = WilsonInterval(baseline.total.Clicks, baseline.total.Impressions)
	result.VariantCTRInterval = WilsonInterval(variant.total.Clicks, variant.total.Impressions)
	result.IntervalsOverlap = intervalsOverlap(result.BaselineCTRInterval, result.VariantCTRInterval)
	result.CTRLift = relativeChange(result.BaselineRates.CTR, result.VariantRates.CTR)

	keys := map[string]struct{}{}
	for key := range baseline.features {
		keys[key] = struct{}{}
	}
	for key := range variant.features {
		keys[key] = struct{}{}
	}
	sorted := make([]string, 0, len(keys))
	for key := range keys {
		sorted = append(sorted, key)
	}
	sort.Strings(sorted)
	for _, key := range sorted {
		left, right := baseline.features[key], variant.features[key]
		if left == right {
			if left != "" {
				result.ControlledCount++
			}
			continue
		}
		field := fieldOf(baseline.kind, key)
		result.ChangedFeatures = append(result.ChangedFeatures, FeatureDiff{
			Key: key, Label: field.Label, Group: field.Group,
			Baseline: orDash(left), Variant: orDash(right),
		})
	}

	minImpressions := baseline.total.Impressions
	if variant.total.Impressions < minImpressions {
		minImpressions = variant.total.Impressions
	}
	switch {
	case len(baseline.features) == 0 || len(variant.features) == 0:
		result.Verdict, result.Confidence = VerdictNoFeatures, ConfidenceConfounded
		result.Note = "至少一边没有内容特征，两个素材之间到底改了什么无从判断。数字上的差异不能算到任何变量头上。"
	case minImpressions < directionalSampleImpressions:
		result.Verdict, result.Confidence = VerdictLowSample, ConfidenceLowSample
		result.Note = fmt.Sprintf("样本较少的一边只有 %d 次展示，不到 %d 次的方向性门槛，先不谈差异。",
			minImpressions, directionalSampleImpressions)
	case len(result.ChangedFeatures) == 0:
		result.Verdict, result.Confidence = VerdictConfounded, ConfidenceConfounded
		result.Note = "两个素材在已记录的特征上完全一致，差异来自特征体系没覆盖到的地方——可能是投放设置、时段或受众，不是内容。"
	case len(result.ChangedFeatures) > 1:
		result.Verdict, result.Confidence = VerdictConfounded, ConfidenceConfounded
		result.Note = fmt.Sprintf("这一对同时改了 %d 个变量（%s），差异归不到其中任何一个上。要归因得再做一组只改一个变量的素材。",
			len(result.ChangedFeatures), joinFeatureLabels(result.ChangedFeatures))
	case result.IntervalsOverlap:
		result.Verdict, result.Confidence = VerdictDirectional, ConfidenceDirectional
		result.Note = fmt.Sprintf("只改了「%s」，但两边的点击率置信区间重叠，差异可能只是波动。方向可以参考，不能当结论。",
			result.ChangedFeatures[0].Label)
	case minImpressions < sufficientSampleImpressions:
		result.Verdict, result.Confidence = VerdictDirectional, ConfidenceDirectional
		result.Note = fmt.Sprintf("只改了「%s」，区间也不重叠，但样本还没到 %d 次展示的充分门槛。",
			result.ChangedFeatures[0].Label, sufficientSampleImpressions)
	default:
		result.Verdict, result.Confidence = VerdictAttributable, ConfidenceSufficient
		result.Note = fmt.Sprintf("只改了「%s」，其余 %d 个特征取值相同，样本充分且区间不重叠——这个差异可以归到这个变量上。",
			result.ChangedFeatures[0].Label, result.ControlledCount)
	}
	// 口径不一致会把所有归因打回方向性：差异可能全部来自口径本身。
	if !comparable && result.Verdict == VerdictAttributable {
		result.Verdict, result.Confidence = VerdictDirectional, ConfidenceConfounded
		result.Note += "（但窗口内口径不一致，这一条降级为方向性观察。）"
	}
	return result
}

func joinFeatureLabels(diffs []FeatureDiff) string {
	labels := make([]string, 0, len(diffs))
	for _, diff := range diffs {
		labels = append(labels, diff.Label)
	}
	if len(labels) > 4 {
		return strings.Join(labels[:4], "、") + fmt.Sprintf(" 等 %d 个", len(labels))
	}
	return strings.Join(labels, "、")
}

func orDash(value string) string {
	if value == "" {
		return "（未记录）"
	}
	return value
}

func intervalsOverlap(left, right *RateInterval) bool {
	if left == nil || right == nil {
		// 有一边算不出区间就当作重叠：不知道差异是否显著时，默认它不显著。
		return true
	}
	return left.Low <= right.High && right.Low <= left.High
}

// relativeChange 是 (after-before)/before。before 为空或为 0 时返回 nil，
// 不退化成 0——「没有基线」和「没有变化」是两件事（doc10 §6）。
func relativeChange(before, after *float64) *float64 {
	if before == nil || after == nil || *before == 0 {
		return nil
	}
	value := (*after - *before) / *before
	return &value
}

// --- 趋势（03 §7.4 的前半）---

func buildTrends(ordered []*assetSlice) []AssetTrend {
	trends := make([]AssetTrend, 0, len(ordered))
	for _, slice := range ordered {
		dates := slice.dates()
		trend := AssetTrend{
			AssetID: slice.assetID, AssetTitle: slice.title, AssetType: slice.kind,
			ActiveDays: len(dates),
			Confidence: confidenceOf(slice.total, true, len(slice.objects)),
		}
		for _, date := range dates {
			counts := slice.byDate[date]
			trend.Points = append(trend.Points, PerformancePoint{Date: date, Counts: counts, Rates: RatesOf(counts)})
		}
		first, second := splitHalves(slice)
		trend.CTRChange = relativeChange(RatesOf(first).CTR, RatesOf(second).CTR)
		switch {
		case len(dates) < 4:
			trend.Direction, trend.Note = "unknown", fmt.Sprintf("窗口内只有 %d 天有数据，看不出走势。", len(dates))
		case trend.CTRChange == nil:
			trend.Direction, trend.Note = "unknown", "前半段没有展示，算不出变化，不能当成持平。"
		case *trend.CTRChange <= -0.15:
			trend.Direction, trend.Note = "declining", "后半段点击率明显低于前半段。"
		case *trend.CTRChange >= 0.15:
			trend.Direction, trend.Note = "rising", "后半段点击率明显高于前半段。"
		default:
			trend.Direction, trend.Note = "flat", "前后两段点击率变化在 ±15% 以内。"
		}
		trends = append(trends, trend)
	}
	return trends
}

// splitHalves 按有数据的日期数对半切，不按窗口天数——窗口 30 天只投了 6 天时，
// 按窗口切会把全部数据塞进其中一半。
func splitHalves(slice *assetSlice) (MetricCounts, MetricCounts) {
	dates := slice.dates()
	var first, second MetricCounts
	mid := len(dates) / 2
	for index, date := range dates {
		if index < mid {
			first = first.add(slice.byDate[date])
		} else {
			second = second.add(slice.byDate[date])
		}
	}
	return first, second
}

// --- 疲劳（03 §7.4）---

func buildFatigue(ordered []*assetSlice, window MetricWindow) []FatigueSignal {
	signals := make([]FatigueSignal, 0, len(ordered))
	for _, slice := range ordered {
		first, second := splitHalves(slice)
		firstRates, secondRates := RatesOf(first), RatesOf(second)
		signal := FatigueSignal{
			AssetID: slice.assetID, AssetTitle: slice.title, AssetType: slice.kind,
			FirstHalf: first, SecondHalf: second,
			FirstRates: firstRates, LastRates: secondRates,
			CTRChange:        relativeChange(firstRates.CTR, secondRates.CTR),
			CPAChange:        relativeChange(firstRates.CPACents, secondRates.CPACents),
			ImpressionChange: relativeChange(floatOf(first.Impressions), floatOf(second.Impressions)),
			Confidence:       confidenceOf(slice.total, true, len(slice.objects)),
			Severity:         FatigueNone,
		}

		ctrDown := signal.CTRChange != nil && *signal.CTRChange <= -0.2
		cpaUp := signal.CPAChange != nil && *signal.CPAChange >= 0.2
		impressionsUp := signal.ImpressionChange != nil && *signal.ImpressionChange >= 0.1

		switch {
		case len(slice.dates()) < 4:
			signal.Note = fmt.Sprintf("只有 %d 天数据，疲劳要看趋势，天数不够就没有趋势可看。", len(slice.dates()))
			// 曝光量再大也换不来天数。这里必须把置信压到 low_sample，否则页面上会
			// 出现「没有疲劳迹象 · 置信充分」——那是在说「查过了，没问题」，
			// 而实际情况是「压根没法查」。这两句话对读者的意义完全相反。
			signal.Confidence = ConfidenceLowSample
		case ctrDown && impressionsUp:
			// 03 §7.4 点名的典型形态：曝光继续放大，点击却掉下来。
			signal.Severity = FatigueLikely
			signal.Note = "曝光还在放大，点击率却明显下滑——这是素材疲劳最典型的形态。"
		case ctrDown && cpaUp:
			signal.Severity = FatigueLikely
			signal.Note = "点击率下滑的同时单次转化成本上升，效率在双向恶化。"
		case ctrDown || cpaUp:
			signal.Severity = FatigueWatch
			signal.Note = "有一项指标在恶化，但另一项没有同向变化，还不足以判定为素材衰退。"
		default:
			signal.Note = "后半段没有出现点击率下滑或成本上升，这一轮看不到疲劳迹象。"
		}

		if signal.Severity != FatigueNone {
			signal.AlternativeExplanations = fatigueAlternatives(signal, slice, window)
		}
		signals = append(signals, signal)
	}
	sort.Slice(signals, func(i, j int) bool {
		return fatigueRank(signals[i].Severity) < fatigueRank(signals[j].Severity)
	})
	return signals
}

func fatigueRank(severity FatigueSeverity) int {
	switch severity {
	case FatigueLikely:
		return 0
	case FatigueWatch:
		return 1
	default:
		return 2
	}
}

// fatigueAlternatives 列出这次**没能排除**的其他解释（03 §7.4 要求区分
// 数据延迟、受众变化、预算变化和真正的素材衰退）。能排除的就不列，
// 排除不了的必须列出来——把「排除不了」写成「已排除」是最坏的一种假精确。
func fatigueAlternatives(signal FatigueSignal, slice *assetSlice, window MetricWindow) []string {
	var reasons []string
	if signal.SecondHalf.SpendCents > 0 && signal.FirstHalf.SpendCents > 0 {
		change := float64(signal.SecondHalf.SpendCents-signal.FirstHalf.SpendCents) / float64(signal.FirstHalf.SpendCents)
		if math.Abs(change) >= 0.2 {
			reasons = append(reasons, fmt.Sprintf("后半段花费变化了 %.0f%%，预算调整本身就会改变流量结构和竞价环境。", change*100))
		}
	}
	dates := slice.dates()
	if len(dates) > 0 {
		last := dates[len(dates)-1]
		if last < window.End.Format("2006-01-02") {
			reasons = append(reasons, fmt.Sprintf("这个素材最后一天有数据是 %s，晚于它的日期都还没回流，末段下滑可能只是数据没到齐。", last))
		}
	}
	if len(slice.objects) > 1 {
		reasons = append(reasons, fmt.Sprintf("这个素材同时投在 %d 个平台对象上，各自的受众和出价不同，合并看趋势会互相稀释。", len(slice.objects)))
	}
	// 受众变化永远排除不了：我们手上没有受众构成数据。
	reasons = append(reasons, "受众变化排除不了——当前没有接入受众构成数据，人群变了和素材看腻了在数字上长得一样。")
	return reasons
}

func floatOf(value int64) *float64 {
	result := float64(value)
	return &result
}

// --- 异常（20 §4.1「错误与延迟置顶」）---

func buildAnomalies(projectByDate map[string]MetricCounts, ordered []*assetSlice) []MetricAnomaly {
	anomalies := make([]MetricAnomaly, 0, 8)
	dates := make([]string, 0, len(projectByDate))
	for date := range projectByDate {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	spends := make([]float64, 0, len(dates))
	for _, date := range dates {
		spends = append(spends, float64(projectByDate[date].SpendCents))
	}
	median, mad := medianAndMAD(spends)
	// 少于 5 天没有「常态」可言，算出来的异常全是噪声。
	if len(dates) >= 5 && mad > 0 {
		for index, date := range dates {
			deviation := math.Abs(spends[index]-median) / mad
			if deviation < 3.5 {
				continue
			}
			kind := AnomalySpike
			word := "高"
			if spends[index] < median {
				kind, word = AnomalyDrop, "低"
			}
			anomalies = append(anomalies, MetricAnomaly{
				Date: date, Scope: "project", Metric: "spend_cents", Kind: kind,
				Observed: spends[index], Median: median, Deviation: deviation,
				Note: fmt.Sprintf("这一天全项目花费明显%s于窗口内的常态水平，先确认是投放动作还是数据问题，再解释素材表现。", word),
			})
		}
	}

	// 素材级曝光突变。项目级花费能抓住「今天整体花超了」，但抓不住「某一条素材
	// 昨天被转了一次、曝光翻了四倍」——后者被其余素材的量摊平在总数里，而它恰恰是
	// 解释单条素材表现时最先要排除的那种事。所以两个尺度都要看。
	//
	// 看曝光而不是花费：素材级的花费还受出价和预算分配影响，曝光更接近「这条素材
	// 那天被推了多少」这件事本身。规则和项目级完全一致（MAD、3.5、至少 5 天），
	// 换个阈值只会让两处的「异常」不是同一个意思。
	for _, slice := range ordered {
		dates := slice.dates()
		if len(dates) < 5 {
			continue
		}
		impressions := make([]float64, 0, len(dates))
		for _, date := range dates {
			impressions = append(impressions, float64(slice.byDate[date].Impressions))
		}
		assetMedian, assetMAD := medianAndMAD(impressions)
		if assetMAD <= 0 {
			// 常态一点波动都没有，说明这是被四舍五入或补录填出来的序列，
			// 拿它当基准算偏离只会得到一堆假阳性。
			continue
		}
		// 每条素材每个方向只留偏离最大的那一天，其余的折进备注里。
		//
		// 这不是为了让列表短。整窗中位数对「台阶」不稳健：一条素材中途加了个投放
		// 计划、曝光整体抬了一级，台阶另一侧的每一天都会被判成「偏低」。逐条列出来
		// 会说成十几件事，而真相只有一件——量在中间变了一次。所以同方向多天命中时，
		// 报最极端的那天，并在备注里挑明这更像整体变化而不是当天出事。
		type worst struct {
			index     int
			deviation float64
			count     int
		}
		extremes := map[AnomalyKind]*worst{}
		for index := range dates {
			deviation := math.Abs(impressions[index]-assetMedian) / assetMAD
			if deviation < 3.5 {
				continue
			}
			kind := AnomalySpike
			if impressions[index] < assetMedian {
				kind = AnomalyDrop
			}
			current, seen := extremes[kind]
			if !seen {
				extremes[kind] = &worst{index: index, deviation: deviation, count: 1}
				continue
			}
			current.count++
			if deviation > current.deviation {
				current.index, current.deviation = index, deviation
			}
		}
		for _, kind := range []AnomalyKind{AnomalySpike, AnomalyDrop} {
			hit, seen := extremes[kind]
			if !seen {
				continue
			}
			word := "高"
			if kind == AnomalyDrop {
				word = "低"
			}
			note := fmt.Sprintf("这一天这条素材的曝光明显%s于它自己的常态。这一天的表现不要和其他日子放在一起算平均，先弄清楚那天发生了什么。", word)
			if hit.count > 1 {
				note = fmt.Sprintf("窗口内有 %d 天曝光明显%s于常态，这里列的是最极端的一天。这么多天同向偏离，通常说明投放量在窗口中间整体变过一次（加减计划、改预算），而不是某一天出了事——先去核对投放动作，再谈素材表现。",
					hit.count, word)
			}
			anomalies = append(anomalies, MetricAnomaly{
				Date: dates[hit.index], Scope: "asset", AssetID: slice.assetID, AssetTitle: slice.title,
				Metric: "impressions", Kind: kind,
				Observed: impressions[hit.index], Median: assetMedian, Deviation: hit.deviation,
				Note: note,
			})
		}
	}

	// 断档：某个素材中间有连续没有数据的日子。它不一定是异常，但会让趋势和疲劳算错，
	// 所以要在异常里说出来。
	for _, slice := range ordered {
		gaps := missingDates(slice.dates())
		if len(gaps) == 0 {
			continue
		}
		anomalies = append(anomalies, MetricAnomaly{
			Date: gaps[0], Scope: "asset", AssetID: slice.assetID, AssetTitle: slice.title,
			Metric: "impressions", Kind: AnomalyGap,
			Note: fmt.Sprintf("这个素材在投放期间有 %d 天没有数据（从 %s 起）。断档期间是停投还是没回流，这里分不出来，但趋势和疲劳都会因此算偏。",
				len(gaps), gaps[0]),
		})
	}

	sort.Slice(anomalies, func(i, j int) bool {
		if anomalies[i].Deviation != anomalies[j].Deviation {
			return anomalies[i].Deviation > anomalies[j].Deviation
		}
		return anomalies[i].Date < anomalies[j].Date
	})
	return anomalies
}

// medianAndMAD 返回中位数和中位数绝对偏差。用 MAD 而不是标准差：
// 一次大促就能把标准差撑大到之后什么都不算异常。
func medianAndMAD(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	median := percentile50(sorted)
	deviations := make([]float64, 0, len(values))
	for _, value := range values {
		deviations = append(deviations, math.Abs(value-median))
	}
	sort.Float64s(deviations)
	// 1.4826 把 MAD 换算成正态分布下的标准差当量，好让 3.5 这个阈值有通常的含义。
	return median, percentile50(deviations) * 1.4826
}

func percentile50(sorted []float64) float64 {
	count := len(sorted)
	if count == 0 {
		return 0
	}
	if count%2 == 1 {
		return sorted[count/2]
	}
	return (sorted[count/2-1] + sorted[count/2]) / 2
}

func missingDates(dates []string) []string {
	if len(dates) < 3 {
		return nil
	}
	start, err := parseDate(dates[0])
	if err != nil {
		return nil
	}
	end, err := parseDate(dates[len(dates)-1])
	if err != nil {
		return nil
	}
	present := map[string]struct{}{}
	for _, date := range dates {
		present[date] = struct{}{}
	}
	var gaps []string
	for cursor := start; !cursor.After(end); cursor = cursor.AddDate(0, 0, 1) {
		key := cursor.Format("2006-01-02")
		if _, ok := present[key]; !ok {
			gaps = append(gaps, key)
		}
	}
	return gaps
}

// --- 驱动因素（03 §7.5 模式归纳）---

// minDriverAssets 是一个特征取值至少要覆盖几个素材才值得比较。两个太少：
// 一个取值只对应一个素材时，比的是那个素材，不是那个特征。
const minDriverAssets = 2

func buildDrivers(ordered []*assetSlice, comparable bool) []FeatureDriver {
	byType := map[AssetType][]*assetSlice{}
	for _, slice := range ordered {
		if len(slice.features) == 0 {
			continue
		}
		byType[slice.kind] = append(byType[slice.kind], slice)
	}

	drivers := make([]FeatureDriver, 0, 16)
	for kind, group := range byType {
		if len(group) < minDriverAssets*2 {
			// 同类型素材不足 4 个时，任何分组都会退化成「一个对一个」。
			continue
		}
		keys := map[string]struct{}{}
		for _, slice := range group {
			for key := range slice.features {
				keys[key] = struct{}{}
			}
		}
		sortedKeys := make([]string, 0, len(keys))
		for key := range keys {
			sortedKeys = append(sortedKeys, key)
		}
		sort.Strings(sortedKeys)

		for _, key := range sortedKeys {
			buckets := map[string][]*assetSlice{}
			for _, slice := range group {
				value, ok := slice.features[key]
				if !ok {
					continue
				}
				buckets[value] = append(buckets[value], slice)
			}
			if len(buckets) < 2 {
				// 全都一样的特征不是变量，没有对照组。
				continue
			}
			sortedValues := make([]string, 0, len(buckets))
			for value := range buckets {
				sortedValues = append(sortedValues, value)
			}
			sort.Strings(sortedValues)
			// 只有两个取值时，两行是同一个发现的正反两面：「A 比其余高 40%」和
			// 「其余比 A 低 40%」。两行都列出来会被读成两个独立发现，所以只留第一行；
			// 它的对照组就是另一个取值，信息一点没少。三个取值起，每一行才各自成立。
			if len(sortedValues) == 2 {
				sortedValues = sortedValues[:1]
			}
			for _, value := range sortedValues {
				inGroup := buckets[value]
				if len(inGroup) < minDriverAssets {
					continue
				}
				rest := make([]*assetSlice, 0, len(group))
				for _, slice := range group {
					if slice.features[key] != value {
						rest = append(rest, slice)
					}
				}
				if len(rest) < minDriverAssets {
					continue
				}
				drivers = append(drivers, buildDriver(kind, key, value, inGroup, rest, comparable))
			}
		}
	}
	sort.Slice(drivers, func(i, j int) bool {
		return driverRank(drivers[i]) < driverRank(drivers[j])
	})
	return drivers
}

func driverRank(driver FeatureDriver) int {
	switch {
	case !driver.IntervalsOverlap && len(driver.CovaryingFeatures) == 0:
		return 0
	case !driver.IntervalsOverlap:
		return 1
	default:
		return 2
	}
}

func buildDriver(kind AssetType, key, value string, inGroup, rest []*assetSlice, comparable bool) FeatureDriver {
	field := fieldOf(kind, key)
	driver := FeatureDriver{
		AssetType: kind, Key: key, Label: field.Label, Group: field.Group, Value: value,
		Assets: len(inGroup), RestAssets: len(rest),
	}
	for _, slice := range inGroup {
		driver.Counts = driver.Counts.add(slice.total)
	}
	for _, slice := range rest {
		driver.RestCounts = driver.RestCounts.add(slice.total)
	}
	driver.Rates, driver.RestRates = RatesOf(driver.Counts), RatesOf(driver.RestCounts)
	driver.CTRInterval = WilsonInterval(driver.Counts.Clicks, driver.Counts.Impressions)
	driver.RestCTRInterval = WilsonInterval(driver.RestCounts.Clicks, driver.RestCounts.Impressions)
	driver.IntervalsOverlap = intervalsOverlap(driver.CTRInterval, driver.RestCTRInterval)
	driver.CTRLift = relativeChange(driver.RestRates.CTR, driver.Rates.CTR)
	driver.CovaryingFeatures = covaryingFeatures(key, inGroup, rest)

	minImpressions := driver.Counts.Impressions
	if driver.RestCounts.Impressions < minImpressions {
		minImpressions = driver.RestCounts.Impressions
	}
	switch {
	case minImpressions < directionalSampleImpressions:
		driver.Confidence = ConfidenceLowSample
		driver.Note = fmt.Sprintf("样本较少的一侧只有 %d 次展示，这个分组还比不出东西。", minImpressions)
	case len(driver.CovaryingFeatures) > 0:
		driver.Confidence = ConfidenceConfounded
		driver.Note = fmt.Sprintf("这一组素材在「%s」上也整齐地和其他素材不同，差异不能只算到「%s」头上。",
			strings.Join(driver.CovaryingFeatures, "」「"), field.Label)
	case driver.IntervalsOverlap:
		driver.Confidence = ConfidenceDirectional
		driver.Note = "两组的点击率置信区间重叠，差异可能只是波动。"
	case !comparable:
		driver.Confidence = ConfidenceConfounded
		driver.Note = "窗口内口径不一致，两组之间的差异可能来自口径而不是内容。"
	case minImpressions < sufficientSampleImpressions:
		driver.Confidence = ConfidenceDirectional
		driver.Note = "区间不重叠，但样本还没到充分门槛；而且这是相关不是因果，要确认得做只改这一个变量的实验。"
	default:
		driver.Confidence = ConfidenceSufficient
		driver.Note = "样本充分、区间不重叠、也没发现同向变化的其他特征。但这仍然是相关：要确认因果得做只改这一个变量的实验。"
	}
	return driver
}

// covaryingFeatures 找出那些「组内整齐一致、且与组外整齐不同」的其他特征。
// 它们和目标特征绑在一起变化，是这条驱动因素结论最直接的混杂来源。
func covaryingFeatures(target string, inGroup, rest []*assetSlice) []string {
	keys := map[string]struct{}{}
	for _, slice := range inGroup {
		for key := range slice.features {
			keys[key] = struct{}{}
		}
	}
	sorted := make([]string, 0, len(keys))
	for key := range keys {
		if key != target {
			sorted = append(sorted, key)
		}
	}
	sort.Strings(sorted)

	var found []string
	for _, key := range sorted {
		groupValue, uniform := uniformValue(inGroup, key)
		if !uniform {
			continue
		}
		differs := true
		for _, slice := range rest {
			value, ok := slice.features[key]
			if !ok || value == groupValue {
				differs = false
				break
			}
		}
		if differs {
			found = append(found, fieldOf(inGroup[0].kind, key).Label)
		}
	}
	return found
}

func uniformValue(slices []*assetSlice, key string) (string, bool) {
	var value string
	for index, slice := range slices {
		current, ok := slice.features[key]
		if !ok {
			return "", false
		}
		if index == 0 {
			value = current
			continue
		}
		if current != value {
			return "", false
		}
	}
	return value, value != ""
}
