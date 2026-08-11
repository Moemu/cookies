package insights

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// 复盘报告的四块内容（20 §118：报告中心不产生新分析，只组织已有分析）。
//
// 这个文件里没有任何计算。强度是投后分析算的，置信是投后分析标的，实验判定是实验
// 中心按事先定的门槛给的——这里只做**选择和定格**。一旦这里开始自己算，报告里的
// 数字和投后分析页上的数字就会开始不一样，而没人分得清哪个对。
type ReportSectionKind string

const (
	SectionAssetPerformance ReportSectionKind = "asset_performance"
	SectionExperiment       ReportSectionKind = "experiment"
	SectionExperience       ReportSectionKind = "experience"
	SectionRecommendation   ReportSectionKind = "recommendation"
)

// ReportSectionOrder 是四块在报告里的固定顺序：先看这一轮的素材表现，再看事先设计的
// 实验怎么说，再看过去的经验是否印证，最后才是下一轮建议。建议放最后，是因为它的
// 说服力全部来自前面三块——放最前面会被当成结论本身。
var ReportSectionOrder = []ReportSectionKind{
	SectionAssetPerformance, SectionExperiment, SectionExperience, SectionRecommendation,
}

// ReportSectionLabels 给人看的块名。放在后端是为了让报告导出、前端展示和以后可能的
// 邮件摘要用同一套措辞。
var ReportSectionLabels = map[ReportSectionKind]string{
	SectionAssetPerformance: "素材表现",
	SectionExperiment:       "实验结论",
	SectionExperience:       "相关经验",
	SectionRecommendation:   "下一轮建议",
}

// FindingOrigin 区分这条发现是人自己挑的，还是系统按规则补的。
//
// 复盘页要把两者分开标（● 我记的 / ○ 系统补的）。混在一起显示，人就分不清
// 哪几条是自己看着数据决定要留的——而那几条恰恰是这次复盘真正的产出。
type FindingOrigin string

const (
	OriginSystem FindingOrigin = "system"
	OriginPinned FindingOrigin = "pinned"
)

func (o FindingOrigin) Label() string {
	if o == OriginPinned {
		return "我记的"
	}
	return "系统补的"
}

// ReportFinding 是报告里的一条发现。它是**定格**的：投后分析是活的，今天打开和
// 下周打开数字就不一样；报告要被引用、被追溯，必须固化，不能实时现算
// （基线文档 §7.9.8「定格」）。
type ReportFinding struct {
	Kind ReportSectionKind `json:"kind"`
	Text string            `json:"text"`

	// Strength 是投后分析已经算好的强度，报告不重算，只挑。
	Strength VariantVerdict `json:"strength,omitempty"`

	// Judgement 是这条发现的三档与理由，同样是定格的。内嵌而不是摆一个 confidence
	// 字段：档位和它的理由必须一起搬运，分开搬迟早只搬一半。
	Judgement

	// Origin 说明这条是谁放进来的。
	Origin FindingOrigin `json:"origin"`
	// Dimension 是这条出自六个视图里的哪一个（comparisons / trends / fatigue /
	// anomalies / drivers / overview），Variable 是它说的哪个变量。
	// 这两个字段是复盘合并时的去重键——人记过的，系统不再补一条。
	Dimension string `json:"dimension,omitempty"`
	Variable  string `json:"variable,omitempty"`

	PinnedBy string     `json:"pinned_by,omitempty"`
	PinnedAt *time.Time `json:"pinned_at,omitempty"`

	// SourceRef 指回算出这条的东西：素材 ID、实验 ID、经验 ID。可追溯（03 §MVP⑫）。
	SourceRef string `json:"source_ref,omitempty"`

	// Dropped 为 true 表示人把它删掉了。**不物理删除**——留着才知道
	// 「系统带了什么、人不要什么」，这是评估自动带入好不好用的唯一依据。
	Dropped bool `json:"dropped"`
}

// normalize 补齐旧数据。digest 是 JSON 列，老行里没有 origin 也没有 verdict；
// 补齐放在读取路径上而不是刷一次数据，是因为刷完还会有更旧的备份被恢复回来。
func (f *ReportFinding) normalize() {
	if f.Origin == "" {
		f.Origin = OriginSystem
	}
	if f.Verdict == "" {
		f.Judgement = judge(f.Confidence, f.Note)
	}
}

// dedupeKey 是「哪个维度上的哪个变量」。两者都空表示这条是自由文本
// （比如口径警告），不参与去重——拿空键去重会把它们全折成一条。
func (f ReportFinding) dedupeKey() string {
	if f.Dimension == "" && f.Variable == "" {
		return ""
	}
	return f.Dimension + "\x00" + f.Variable
}

// strengthRank 决定发现在报告里的先后。报告是给人扫一眼的，最强的证据必须在最上面
// ——排在后面的会被跳过，所以排序本身就是一次编辑决定。
//
// low_sample 和 no_features 给了最大的秩，但它们其实根本不会进报告（见 keepVerdict）。
// 留在这里是为了排序函数对任何输入都有确定结果，而不是遇到意外值就随机排。
func strengthRank(verdict VariantVerdict) int {
	switch verdict {
	case VerdictAttributable:
		return 0
	case VerdictDirectional:
		return 1
	case VerdictConfounded:
		return 2
	default:
		return 3
	}
}

// keepVerdict 决定一条对比结论能不能自动带进报告。
//
// low_sample 是「算不出来」，no_features 是「没东西可比」。这两种带进去，等于让人
// 在复盘会上引用一条根本没有结论的结论——而报告里的东西一旦被念出来，就没人再回去
// 看它旁边那行小字了。人仍然可以自己在报告里补写，那是他明知道的选择。
func keepVerdict(verdict VariantVerdict) bool {
	return verdict == VerdictAttributable || verdict == VerdictDirectional || verdict == VerdictConfounded
}

// maxPerCategory 是每一类发现自动带入的条数上限。四条对比 + 三条驱动 + 三条疲劳
// 已经够一次复盘会讲的了，再多人会开始跳着读。被略过的条数一定会写进文本里
// ——静默截断读起来像「就这么多」，实际不是。
const maxPerCategory = 3

// buildReportDigest 把四块内容汇总成定格的发现列表。
// 它不产生新分析，只做选择——数据清洗在数据接入那层，强度判定在投后分析已经算完。
//
// 实验这一块收的是 []Experiment 而不是 ExperimentDetail：已结论的实验，判定和解读
// 在结论那一刻就冻住了，不需要再算一遍读数。真要现算，算出来的还可能和当初登记的
// 判定不一样——那样报告里的实验结论就成了活的，而实验结论恰恰是最该定死的东西。
func buildReportDigest(analysis PerformanceAnalysis, experiments []Experiment,
	experiences []Experience) []ReportFinding {
	findings := make([]ReportFinding, 0, 16)
	findings = append(findings, performanceFindings(analysis)...)
	findings = append(findings, experimentFindings(experiments)...)
	findings = append(findings, experienceFindings(experiences)...)
	findings = append(findings, recommendationFindings(analysis, experiments)...)
	// 这里出来的每一条都是系统按规则补的，统一在出口标一次 Origin，而不是让四个
	// 子函数各自在二十来个构造点上抄一遍——抄漏一条，复盘页上它就会顶着「我记的」
	// 出现，而人根本没记过它。
	for index := range findings {
		findings[index].normalize()
	}
	return findings
}

// performanceFindings 汇总素材表现那一块：对比、驱动因素、疲劳、异常。
//
// 四类分开截断而不是合起来取前 N 条：合起来取的话，对比结论通常强度最高，会把疲劳
// 和异常整类挤掉——而「有没有素材在衰退」是复盘会上一定要回答的问题，不能因为它
// 强度天生低就整块消失。
func performanceFindings(analysis PerformanceAnalysis) []ReportFinding {
	findings := make([]ReportFinding, 0, 12)

	// 口径不一致会让这一整块的每条结论都可能是口径造成的。它必须排在所有数字前面，
	// 而不是作为某条结论的脚注——脚注会被跳过。
	if !analysis.Comparable && strings.TrimSpace(analysis.ComparableReason) != "" {
		findings = append(findings, ReportFinding{
			Kind: SectionAssetPerformance,
			Text: fmt.Sprintf("先说边界：%s 下面这一块的每条结论都可能是口径造成的，不是素材造成的。", analysis.ComparableReason),
		})
	}

	kept := make([]VariantComparison, 0, len(analysis.Comparisons))
	for _, item := range analysis.Comparisons {
		if keepVerdict(item.VariantVerdict) {
			kept = append(kept, item)
		}
	}
	// SliceStable：强度相同的两条保持投后分析给的原顺序。用不稳定排序的话，同一份
	// 数据定格两次可能得到不同的报告，而报告是要被追溯的。
	sort.SliceStable(kept, func(i, j int) bool {
		return strengthRank(kept[i].VariantVerdict) < strengthRank(kept[j].VariantVerdict)
	})
	for index, item := range kept {
		if index >= maxPerCategory {
			findings = append(findings, ReportFinding{
				Kind: SectionAssetPerformance,
				Text: fmt.Sprintf("另有 %d 条素材对比结论没有带进来，强度低于上面几条。要看全部请回投后分析的「素材对比」。", len(kept)-maxPerCategory),
			})
			break
		}
		// 去重键要的是「这条说的哪个变量」。一组对比可能改了不止一个变量，取第一个
		// ——多变量的对比本来就归不了因，它进报告只是作为观察，键撞不撞无所谓。
		changedKey := ""
		if len(item.ChangedFeatures) > 0 {
			changedKey = item.ChangedFeatures[0].Key
		}
		findings = append(findings, ReportFinding{
			Kind:      SectionAssetPerformance,
			Text:      fmt.Sprintf("「%s」对比「%s」：%s", item.VariantTitle, item.BaselineTitle, item.Note),
			Strength:  item.VariantVerdict,
			Judgement: item.Judgement,
			Dimension: "comparisons",
			Variable:  changedKey,
			SourceRef: item.VariantAssetID,
		})
	}
	if skipped := len(analysis.Comparisons) - len(kept); skipped > 0 {
		findings = append(findings, ReportFinding{
			Kind: SectionAssetPerformance,
			Text: fmt.Sprintf("还有 %d 组素材因为样本不够或者没填特征而算不出结论，它们没有进报告。算不出来不等于没差别。", skipped),
		})
	}

	for index, driver := range analysis.Drivers {
		if index >= maxPerCategory {
			findings = append(findings, ReportFinding{
				Kind: SectionAssetPerformance,
				Text: fmt.Sprintf("另有 %d 条特征层面的观察没有带进来。", len(analysis.Drivers)-maxPerCategory),
			})
			break
		}
		findings = append(findings, ReportFinding{
			Kind:      SectionAssetPerformance,
			Text:      fmt.Sprintf("特征「%s = %s」：%s", driver.Label, driver.Value, driver.Note),
			Judgement: driver.Judgement,
			Dimension: "drivers",
			Variable:  driver.Key,
			SourceRef: driver.Key,
		})
	}

	fatigued := make([]FatigueSignal, 0, len(analysis.Fatigue))
	for _, signal := range analysis.Fatigue {
		// none 档不进报告，但它在投后分析页上仍然显示——那里的价值是「查过了，没有」，
		// 报告里放一堆「没事」只会把真正在衰退的那条淹掉。
		if signal.Severity != FatigueNone {
			fatigued = append(fatigued, signal)
		}
	}
	for index, signal := range fatigued {
		if index >= maxPerCategory {
			findings = append(findings, ReportFinding{
				Kind: SectionAssetPerformance,
				Text: fmt.Sprintf("另有 %d 条疲劳信号没有带进来。", len(fatigued)-maxPerCategory),
			})
			break
		}
		text := fmt.Sprintf("「%s」有衰退迹象：%s", signal.AssetTitle, signal.Note)
		// 排除不了的其他解释要跟着信号一起走。只写「在衰退」而不写「也可能是预算变了」，
		// 下一步就会变成换素材，而真正的原因没人查。
		if len(signal.AlternativeExplanations) > 0 {
			text += fmt.Sprintf("（没能排除的其他解释：%s）", strings.Join(signal.AlternativeExplanations, "；"))
		}
		findings = append(findings, ReportFinding{
			Kind:      SectionAssetPerformance,
			Text:      text,
			Judgement: signal.Judgement,
			Dimension: "fatigue",
			Variable:  signal.AssetID,
			SourceRef: signal.AssetID,
		})
	}

	for index, anomaly := range analysis.Anomalies {
		if index >= maxPerCategory {
			findings = append(findings, ReportFinding{
				Kind: SectionAssetPerformance,
				Text: fmt.Sprintf("另有 %d 条数据异常没有带进来。", len(analysis.Anomalies)-maxPerCategory),
			})
			break
		}
		findings = append(findings, ReportFinding{
			Kind:      SectionAssetPerformance,
			Text:      fmt.Sprintf("%s 数据异常：%s", anomaly.Date, anomaly.Note),
			Judgement: anomaly.Judgement,
			Dimension: "anomalies",
			Variable:  anomaly.AssetID,
			SourceRef: anomaly.AssetID,
		})
	}

	if len(findings) == 0 {
		findings = append(findings, ReportFinding{
			Kind: SectionAssetPerformance,
			Text: "这个窗口内没有算得出来的素材表现结论。可能是素材还没填特征，也可能是投放数据还没到——这两件事去投后分析页能分清。",
		})
	}
	return findings
}

// experimentFindings 汇总实验结论那一块。
//
// 一个实验都没有时会明写「本轮没有实验」，而不是把这一块隐藏掉。隐藏了以后没人记得
// 这块该有，「事后对比」就会被当成「实验验证过」——这两件事的说服力差着一个量级。
func experimentFindings(experiments []Experiment) []ReportFinding {
	findings := make([]ReportFinding, 0, len(experiments)+1)
	for _, experiment := range experiments {
		if experiment.Status != ExperimentConcluded {
			continue
		}
		text := fmt.Sprintf("实验「%s」：围绕「%s」，结论是%s。",
			experiment.Title, experiment.VariableLabel, experimentVerdictText(experiment.Verdict))
		if strings.TrimSpace(experiment.Hypothesis) != "" {
			text += fmt.Sprintf("原假设：%s", experiment.Hypothesis)
		}
		if strings.TrimSpace(experiment.Interpretation) != "" {
			text += fmt.Sprintf(" 解读：%s", experiment.Interpretation)
		}
		findings = append(findings, ReportFinding{
			Kind:      SectionExperiment,
			Text:      text,
			SourceRef: experiment.ID,
		})
	}
	if len(findings) == 0 {
		findings = append(findings, ReportFinding{
			Kind: SectionExperiment,
			Text: "本轮没有已结论的实验。上面的归因来自事后对比，不是事先设计的实验——它能指出方向，但撑不起「因为改了这个所以变好」。",
		})
	}
	return findings
}

// directionFindings 把可归因的对比结论按「改的是哪个变量」归并成建议。
//
// 逐条推是不行的。同一个变量会在很多对素材上反复出现——v2 比 v1、v3 比 v1 改的都是
// 「钩子类型」，逐条推出去就是同一句话抄三遍，读的人分不清这是三条独立发现还是一条
// 被重复了，而「有几组对比支持它」恰恰是这条建议值多少钱的关键，反倒丢了。
//
// 更要命的是方向会打架：v2 比 v1 说「利益」赢，v4 比 v2 说「反差、问题」赢，两条都
// 标着「可归因」。逐条推出去，报告就在同一节里叫人往两个相反的方向走，还都盖着
// 「可归因」的章。这时候正确的做法不是挑一个更强的推出去——两边强度一样，挑哪个都是
// 编——而是明写它们对不上，并指出这正是实验中心存在的理由。
func directionFindings(comparisons []VariantComparison) []ReportFinding {
	type direction struct {
		text       string // 「反差、问题 → 利益」，箭头指向赢的那一边
		count      int
		sourceRef  string
		confidence ConfidenceLevel
		// lift 是赢的一边相对输的一边高多少，恒为正。它和 VariantComparison.CTRLift
		// 不是一个数：那个是 variant 相对 baseline，方向翻转后不能取负数——
		// 相对变化不对称（3.23% 比 1.89% 高 71%，1.89% 比 3.23% 低 41.5%）。
		lift *float64
	}
	type variable struct {
		label      string
		order      []string // 方向按首次出现排，保证同样的输入永远得到同样的报告
		directions map[string]*direction
	}
	order := make([]string, 0, 4)
	byVariable := make(map[string]*variable, 4)

	for _, item := range comparisons {
		if item.VariantVerdict != VerdictAttributable || len(item.ChangedFeatures) == 0 {
			continue
		}
		// 可归因按定义只有一个变量在动；真出现多个就把它们拼成一个复合变量，
		// 而不是拆开——拆开会把「同时改了两处」说成两条各自独立的建议。
		keys := make([]string, 0, len(item.ChangedFeatures))
		labels := make([]string, 0, len(item.ChangedFeatures))
		froms := make([]string, 0, len(item.ChangedFeatures))
		tos := make([]string, 0, len(item.ChangedFeatures))
		for _, diff := range item.ChangedFeatures {
			keys = append(keys, diff.Key)
			labels = append(labels, diff.Label)
			froms = append(froms, diff.Baseline)
			tos = append(tos, diff.Variant)
		}
		// 方向按「哪边赢」定，不能照抄 baseline → variant。
		//
		// baseline 只是配对时花费更高的那一个（buildComparisons 把素材按花费排序后
		// 两两配对，排在前面的当 baseline），和表现好坏毫无关系。照抄的结果是：
		// A 版点击率 3.23%、B 版 1.89%，报告却写「下一轮继续按「钩子类型（问题 → 利益）」
		// 这个方向做」——把输了 41% 的那一版推荐给下一轮，而且盖着「可归因」的章。
		high, low := item.VariantRates.CTR, item.BaselineRates.CTR
		winner, loser, sourceRef := tos, froms, item.VariantAssetID
		if high == nil || low == nil {
			// 判成可归因的前提是两边都过了充分样本门槛，CTR 不可能算不出来。
			// 真走到这里说明判定机器改过了，这时候宁可少一条建议也不能瞎猜方向。
			continue
		}
		if *high < *low {
			high, low = low, high
			winner, loser, sourceRef = froms, tos, item.BaselineAssetID
		}

		varKey := strings.Join(keys, "+")
		entry := byVariable[varKey]
		if entry == nil {
			entry = &variable{label: strings.Join(labels, "、"), directions: map[string]*direction{}}
			byVariable[varKey] = entry
			order = append(order, varKey)
		}
		dirText := fmt.Sprintf("%s → %s", strings.Join(loser, "、"), strings.Join(winner, "、"))
		if existing := entry.directions[dirText]; existing != nil {
			existing.count++
			continue
		}
		entry.directions[dirText] = &direction{
			text: dirText, count: 1, sourceRef: sourceRef, confidence: item.Confidence,
			lift: relativeChange(low, high),
		}
		entry.order = append(entry.order, dirText)
	}

	findings := make([]ReportFinding, 0, len(order))
	for _, varKey := range order {
		entry := byVariable[varKey]
		if len(entry.order) > 1 {
			conflicts := make([]string, 0, len(entry.order))
			for _, dirText := range entry.order {
				conflicts = append(conflicts, fmt.Sprintf("「%s」", dirText))
			}
			findings = append(findings, ReportFinding{
				Kind: SectionRecommendation,
				Text: fmt.Sprintf(
					"「%s」这个变量本轮给不出方向：%s 都被判成可归因，方向是反的。"+
						"说明它的效果还受别的东西影响（受众、时段、和它一起变的东西），"+
						"事后对比再做几组也解不开。要拿到方向，得在下一轮开跑前只拿它当变量、"+
						"其余控住——去实验中心建一个。",
					entry.label, strings.Join(conflicts, "、")),
			})
			continue
		}
		only := entry.directions[entry.order[0]]
		text := fmt.Sprintf("下一轮可以继续按「%s（%s）」这个方向做，它在本轮是可归因的差异。", entry.label, only.text)
		if only.count > 1 {
			text = fmt.Sprintf("下一轮可以继续按「%s（%s）」这个方向做：本轮有 %d 组素材对比都指向它，且都是可归因的差异。",
				entry.label, only.text, only.count)
		}
		// 把幅度写出来，读的人才知道这条建议值不值得为它改流程：
		// 「相对高 2%」和「相对高 71%」是两件事，光看方向分不出来。
		if only.lift != nil {
			text += fmt.Sprintf("箭头指向的那一边，本轮点击率相对高 %.1f%%。", *only.lift*100)
		}
		// 建议不带维度：它不是六个视图里的某一条，而是好几条对比归并出来的。
		// 给它一个维度会让复盘去重把它和它的来源折成一条。
		findings = append(findings, ReportFinding{
			Kind:      SectionRecommendation,
			Text:      text,
			Strength:  VerdictAttributable,
			Judgement: judge(only.confidence, ""),
			SourceRef: only.sourceRef,
		})
	}
	return findings
}

func experimentVerdictText(verdict ExperimentVerdict) string {
	switch verdict {
	case VerdictSupported:
		return "支持这条假设"
	case VerdictRefuted:
		return "推翻这条假设"
	case VerdictInconclusive:
		return "看不出差别"
	default:
		return "没有登记判定"
	}
}

// experienceFindings 汇总相关经验那一块。
//
// 全带，不自动判断印证还是推翻。判断一条老经验在这一轮是被印证还是被推翻，要看这轮
// 的素材是不是落在它的适用范围内——那是人的判断。系统自动标一个，人就会照着签字。
func experienceFindings(experiences []Experience) []ReportFinding {
	findings := make([]ReportFinding, 0, len(experiences)+1)
	for _, experience := range experiences {
		if experience.Status != ExperienceConfirmed {
			continue
		}
		findings = append(findings, ReportFinding{
			Kind:      SectionExperience,
			Text:      fmt.Sprintf("已有经验（%s）：%s", cardTypeText(experience.CardType), experience.Conclusion),
			Judgement: judge(experience.Confidence, ""),
			SourceRef: experience.ID,
		})
	}
	if len(findings) == 0 {
		findings = append(findings, ReportFinding{
			Kind: SectionExperience,
			Text: "经验库里还没有已确认的结论可以对照。这一轮的结论确认之后可以沉淀成第一条。",
		})
	}
	return findings
}

func cardTypeText(cardType InsightCardType) string {
	switch cardType {
	case CardFact:
		return "事实"
	case CardStatistic:
		return "统计观察"
	case CardHypothesis:
		return "假设"
	case CardRecommendation:
		return "建议"
	default:
		return string(cardType)
	}
}

// recommendationFindings 推下一轮建议。
//
// 只有两种东西够格推建议：**可归因**的对比结论，和已结论的实验。方向性的不算——
// 方向性的意思就是「看着像但说不准」，拿它指导下一轮，等于把一次没验证的观察变成
// 一条要照着做的规定。一条都推不出时明写，不硬凑。
func recommendationFindings(analysis PerformanceAnalysis, experiments []Experiment) []ReportFinding {
	findings := make([]ReportFinding, 0, 6)
	findings = append(findings, directionFindings(analysis.Comparisons)...)
	for _, experiment := range experiments {
		if experiment.Status != ExperimentConcluded || experiment.Verdict != VerdictSupported {
			continue
		}
		findings = append(findings, ReportFinding{
			Kind:      SectionRecommendation,
			Text:      fmt.Sprintf("「%s」这条假设被实验支持了，可以进经验库，下一轮当默认做法。", experiment.Title),
			SourceRef: experiment.ID,
		})
	}
	// 在衰退的素材要换掉，这是唯一一条不需要归因就能给的建议：不管衰退的原因是素材
	// 本身还是受众耗尽，继续投它都不划算。
	for _, signal := range analysis.Fatigue {
		if signal.Severity != FatigueLikely {
			continue
		}
		findings = append(findings, ReportFinding{
			Kind:      SectionRecommendation,
			Text:      fmt.Sprintf("「%s」建议下一轮替换或降权，它在窗口后半段已经明显衰退。", signal.AssetTitle),
			SourceRef: signal.AssetID,
		})
	}
	if len(findings) == 0 {
		findings = append(findings, ReportFinding{
			Kind: SectionRecommendation,
			Text: "本轮没有强到可以指导下一轮的结论。要拿到这样的结论，需要在下一轮开跑前就把变量和分组定下来——那是实验中心做的事。",
		})
	}
	return findings
}
