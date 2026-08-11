package insights

// 三档结论是素材洞察的中轴：任何一屏、任何一条结论，最后都要落到这三档之一。
// 这个文件是三档的唯一定义处。
//
// 为什么不直接用 ConfidenceLevel：那是四个值的**统计**口径，「样本不足」和
// 「存在混杂」是两种完全不同的毛病，算法里必须分开。但坐在屏幕前的人只需要
// 知道三件事——这条能拿去用（✅）、只能参考（👁）、还是根本算不出来（❓）。
// 四个统计档位往三个行动档位的收敛，只允许发生在本文件的 Verdict() 里。

// Verdict 是给人看的三档。
type Verdict string

const (
	// VerdictExplained ✅ 能归因：差异是真的，而且能归到那个变量上，可以拿去用。
	VerdictExplained Verdict = "explained"
	// VerdictObserved 👁 只是观察：看见了差异，但归不到变量上——可能是样本还不够稳，
	// 也可能是有别的特征跟着一起变。区别写在 Note 里，不占一个档位。
	VerdictObserved Verdict = "observed"
	// VerdictUnclear ❓ 算不出来：数据太少，连差异存不存在都判断不了。
	VerdictUnclear Verdict = "unclear"
)

func (v Verdict) Label() string {
	switch v {
	case VerdictExplained:
		return "能归因"
	case VerdictObserved:
		return "只是观察"
	case VerdictUnclear:
		return "算不出来"
	}
	return string(v)
}

func (v Verdict) Icon() string {
	switch v {
	case VerdictExplained:
		return "✅"
	case VerdictObserved:
		return "👁"
	case VerdictUnclear:
		return "❓"
	}
	return ""
}

// UpgradePath 是这一档往上走的唯一通道。
type UpgradePath string

const (
	UpgradeNone UpgradePath = ""
	// UpgradeExperiment：👁 缺的是「只改这一个变量」，那就得做实验。
	UpgradeExperiment UpgradePath = "experiment"
	// UpgradeSimilar：❓ 缺的是样本，那就得从库里拉相似素材把样本做厚。
	UpgradeSimilar UpgradePath = "similar_assets"
)

func (u UpgradePath) Label() string {
	switch u {
	case UpgradeExperiment:
		return "做个实验"
	case UpgradeSimilar:
		return "找相似素材"
	}
	return ""
}

func (v Verdict) Upgrade() UpgradePath {
	switch v {
	case VerdictObserved:
		return UpgradeExperiment
	case VerdictUnclear:
		return UpgradeSimilar
	}
	return UpgradeNone
}

// Verdict 是四档统计口径到三档行动口径的**唯一**收敛处。
//
// 未知取值落到 VerdictUnclear 而不是 VerdictObserved：不认识的档位应该表现为
// 「算不出来」，让人去查，而不是伪装成一条能参考的观察。
func (c ConfidenceLevel) Verdict() Verdict {
	switch c {
	case ConfidenceSufficient:
		return VerdictExplained
	case ConfidenceDirectional, ConfidenceConfounded:
		return VerdictObserved
	case ConfidenceLowSample:
		return VerdictUnclear
	}
	return VerdictUnclear
}

// Judgement 是一条结论对外的统一表达。要给人看档位的结构体一律内嵌它，
// 而不是各自摆一对 Confidence + Note 字段——摆散了迟早出现两页对同一份数据
// 给出不同档位，而没人解释得清哪个对。
type Judgement struct {
	Confidence   ConfidenceLevel `json:"confidence"`
	Verdict      Verdict         `json:"verdict"`
	VerdictLabel string          `json:"verdict_label"`
	Upgrade      UpgradePath     `json:"upgrade,omitempty"`
	Note         string          `json:"note"`

	// ThresholdVersion 是判出这条结论时生效的阈值版本。**0 和「没有」是两回事**：
	// 0 表示这条是按出厂设定判的，nil 表示手上没有那一版的号码。
	//
	// 用指针就是为了分开这两种。非指针的话，一条从库里按 Confidence 重建出来的
	// 判定（经验卡、投影）会带着 0 出去，页面上写「按出厂阈值判定」——
	// 而它实际上可能是第 5 版判的，只是那个号码没存下来。替一条来历不明的结论
	// 作保，比什么都不标更糟。
	//
	// 有号码的那些必须跟着结论走到最远的地方——记一笔存进复盘草稿、定格成报告
	// 之后仍然读得出来（发现是 JSON 列，这个字段跟着一起落库）。改完阈值之后
	// 回头看一条老结论，这个号码是唯一能说清「它是按什么标准算的」的东西。
	ThresholdVersion *int64 `json:"threshold_version,omitempty"`
}

// judge 是构造 Judgement 的唯一入口。手拼 Judgement 字面量会绕过收敛规则，
// 契约测试会在 JSON 层把它抓出来。
func judge(confidence ConfidenceLevel, note string) Judgement {
	verdict := confidence.Verdict()
	return Judgement{
		Confidence:   confidence,
		Verdict:      verdict,
		VerdictLabel: verdict.Label(),
		Upgrade:      verdict.Upgrade(),
		Note:         note,
	}
}

// judgeAt 是「拿着一套阈值算出来的」判定。凡是判定链路上有 thresholds 在手的
// 地方一律用它，不用 judge——用 judge 的话这条结论会盖成第 0 版，
// 看的人以为它跑的是出厂设定，而实际上是第 5 版。
//
// judge 留给另一种情况：从库里把 Confidence 读回来重建判定（经验、卡片投影）。
// 那些地方手上没有当初那一版的号码，编一个出来比留空更糟。
func judgeAt(thresholds ResolvedThresholds, confidence ConfidenceLevel, note string) Judgement {
	value := judge(confidence, note)
	version := thresholds.Version
	value.ThresholdVersion = &version
	return value
}

// NewJudgement 是 judge 的导出版本，给包外用（HTTP 层的测试替身等）。
// 包内一律用 judge。有了它，包外也没有理由手拼 Judgement 字面量——手拼就会
// 出现 confidence 和 verdict 对不上的组合。
func NewJudgement(confidence ConfidenceLevel, note string) Judgement {
	return judge(confidence, note)
}

// verdictStrength 越大越强。
//
// 不叫 verdictRank：那个名字归 performance.go 的 VariantVerdict 排序所有，
// 而且方向正相反（那边越小越靠前）。两个方向相反的同名函数迟早有人用错。
func verdictStrength(v Verdict) int {
	switch v {
	case VerdictExplained:
		return 2
	case VerdictObserved:
		return 1
	}
	return 0
}

// weakestVerdict 给一屏定档：取最弱的那一条。
func weakestVerdict(verdicts ...Verdict) Verdict {
	if len(verdicts) == 0 {
		return VerdictUnclear
	}
	weakest := VerdictExplained
	for _, v := range verdicts {
		if verdictStrength(v) < verdictStrength(weakest) {
			weakest = v
		}
	}
	return weakest
}
