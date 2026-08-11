package insights

import (
	"fmt"
	"strings"
)

// 组间比较。**这个文件是全模块唯一的组间判定口径。**
//
// 驱动因素（事后按特征把素材分成两堆）和实验中心（事先定好哪些素材进哪一组）问的是
// 同一个问题：这两组的差异是真的吗，能不能归到那个变量上。两处各写一套统计，迟早会
// 出现「实验页说可归因、素材对比页说混杂」，而没人解释得清哪个对。
//
// 两者的区别不在统计，在**证据强度**：事后凑对排除不掉选择效应——凑在一起的素材
// 往往还共享别的特征，最多说到「和这个变量相关」；事先实验把变量、分组、样本门槛
// 在看到结果之前定死，才够得上因果说法。那个区别体现在措辞上（preRegistered），
// 不体现在算法上。

// GroupComparison 是「一组素材 vs 另一组素材」的判定结果。
type GroupComparison struct {
	Counts     MetricCounts `json:"counts"`
	RestCounts MetricCounts `json:"rest_counts"`
	Rates      MetricRates  `json:"rates"`
	RestRates  MetricRates  `json:"rest_rates"`

	CTRInterval      *RateInterval `json:"ctr_interval,omitempty"`
	RestCTRInterval  *RateInterval `json:"rest_ctr_interval,omitempty"`
	IntervalsOverlap bool          `json:"intervals_overlap"`
	CTRLift          *float64      `json:"ctr_lift,omitempty"`

	// CovaryingFeatures 是「组内整齐一致、且与组外整齐不同」的其他特征。
	// 它们和目标变量绑在一起变化，是这条结论最直接的混杂来源。空切片而非 nil。
	CovaryingFeatures []string `json:"covarying_features"`
	// Judgement 内嵌而不是摆两个字段：档位和它的理由必须一起产生、一起搬运。
	Judgement
}

// groupCompareInput 把「比什么」和「怎么措辞」分开传，算法部分对两个调用方是同一套。
type groupCompareInput struct {
	InGroup []*assetSlice
	Rest    []*assetSlice

	// CovaryKey 是被比较的那个变量的键。为空时跳过协变特征检查——
	// 实验中心的分组是事先定的，是否要查协变由调用方决定。
	CovaryKey string
	// SubjectLabel 是这个变量给人看的名字，只用于措辞。
	SubjectLabel string

	// Comparable 说明窗口内的口径是否一致。口径不一致时任何组间差异都可能来自口径。
	Comparable bool
	// PreRegistered 为 true 表示分组在看到结果之前就定死了（实验）。
	// false 表示是事后按特征凑的（驱动因素），结论只能说到相关。
	PreRegistered bool

	// Thresholds 是这次判定用的那一套。零值表示调用方没传，
	// compareGroups 会退回出厂设定——判定是公共函数，调用点很多，
	// 漏传一处就拿 0 去比的话，任何样本量都会被判成充分，那是最坏的一种错。
	Thresholds ResolvedThresholds
}

func compareGroups(input groupCompareInput) GroupComparison {
	thresholds := input.Thresholds.orDefaults()
	result := GroupComparison{CovaryingFeatures: make([]string, 0, 2)}
	for _, slice := range input.InGroup {
		result.Counts = result.Counts.add(slice.total)
	}
	for _, slice := range input.Rest {
		result.RestCounts = result.RestCounts.add(slice.total)
	}
	result.Rates, result.RestRates = RatesOf(result.Counts), RatesOf(result.RestCounts)
	result.CTRInterval = WilsonInterval(result.Counts.Clicks, result.Counts.Impressions)
	result.RestCTRInterval = WilsonInterval(result.RestCounts.Clicks, result.RestCounts.Impressions)
	result.IntervalsOverlap = intervalsOverlap(result.CTRInterval, result.RestCTRInterval)
	result.CTRLift = relativeChange(result.RestRates.CTR, result.Rates.CTR)
	if input.CovaryKey != "" && len(input.InGroup) > 0 {
		result.CovaryingFeatures = append(result.CovaryingFeatures,
			covaryingFeatures(input.CovaryKey, input.InGroup, input.Rest)...)
	}

	minImpressions := result.Counts.Impressions
	if result.RestCounts.Impressions < minImpressions {
		minImpressions = result.RestCounts.Impressions
	}

	// 判定顺序是有意的：样本 → 混杂 → 区间 → 口径 → 充分度。
	// 先卡样本，因为样本不够时后面几项都算不出有意义的东西；混杂排在区间前面，
	// 因为区间不重叠但存在协变特征时，结论仍然归不到目标变量上。
	switch {
	case minImpressions < int64(thresholds.DirectionalImpressions):
		result.Judgement = judge(ConfidenceLowSample,
			fmt.Sprintf("样本较少的一侧只有 %s 次展示，这个分组还比不出东西。", countText(minImpressions)))
	case len(result.CovaryingFeatures) > 0:
		result.Judgement = judge(ConfidenceConfounded,
			fmt.Sprintf("这一组素材在「%s」上也整齐地和其他素材不同，差异不能只算到「%s」头上。",
				strings.Join(result.CovaryingFeatures, "」「"), input.SubjectLabel))
	case result.IntervalsOverlap:
		result.Judgement = judge(ConfidenceDirectional, "两组的点击率置信区间重叠，差异可能只是波动。")
	case !input.Comparable:
		result.Judgement = judge(ConfidenceConfounded, "窗口内口径不一致，两组之间的差异可能来自口径而不是内容。")
	case minImpressions < int64(thresholds.SufficientImpressions):
		result.Judgement = judge(ConfidenceDirectional, directionalNote(input.PreRegistered))
	default:
		result.Judgement = judge(ConfidenceSufficient, sufficientNote(input.PreRegistered))
	}
	return result
}

// 事先设计的实验和事后凑对，在这里分道扬镳：同样的数字，能说的话不一样。
func directionalNote(preRegistered bool) string {
	if preRegistered {
		return "区间不重叠，但样本还没到充分门槛。变量和分组是事先定的，所以差异能归到这个变量上，只是还不够稳。"
	}
	return "区间不重叠，但样本还没到充分门槛；而且这是相关不是因果，要确认得做只改这一个变量的实验。"
}

func sufficientNote(preRegistered bool) string {
	if preRegistered {
		return "样本充分、区间不重叠、也没发现同向变化的其他特征。变量和分组是事先定的，这条结论能归因到这个变量。"
	}
	return "样本充分、区间不重叠、也没发现同向变化的其他特征。但这仍然是相关：要确认因果得做只改这一个变量的实验。"
}
