package insights

import "strings"

// 「记一笔」是分析页唯一的写操作。
//
// 分析页是自由探索的地方：换窗口、换视图、来回比。这种地方不能承担「确认结论」的
// 职责——人还在看，一个误点就把一条没想清楚的东西沉淀成了经验。所以分析页只能做
// 一件事：把这一条钉进本轮复盘草稿，等复盘的时候再逐条决定要不要提交。
//
// 判定不收前端的。请求只说「哪个窗口、哪个视图、哪条」，档位由后端回头去那次分析
// 结果里找回来。能传判定的话，页面上标的三档就是装饰。

// analysisDimensions 是六个视图的键。记一笔只能记在这六个里面——
// 记在别的地方，复盘页就不知道该把它归到哪一块。
var analysisDimensions = map[string]ReportSectionKind{
	"overview":    SectionAssetPerformance,
	"comparisons": SectionAssetPerformance,
	"trends":      SectionAssetPerformance,
	"fatigue":     SectionAssetPerformance,
	"anomalies":   SectionAssetPerformance,
	"drivers":     SectionAssetPerformance,
}

// findJudgement 在一次分析结果里找回某一条的判定和它自己的措辞。
//
// 返回措辞而不是让前端把屏幕上的文字传上来：屏幕上的文字是前端拼的，
// 传上来就等于把措辞的权威交给了前端，两处措辞迟早不一样。
func findJudgement(analysis PerformanceAnalysis, dimension, sourceRef, variable string) (Judgement, string, bool) {
	switch dimension {
	case "overview":
		if analysis.Judgement.Verdict == "" {
			return Judgement{}, "", false
		}
		return analysis.Judgement, analysis.Judgement.Note, true
	case "comparisons":
		for _, item := range analysis.Comparisons {
			if !matchesSubject(sourceRef, variable, item.VariantAssetID, firstFeatureKey(item.ChangedFeatures)) {
				continue
			}
			return item.Judgement, comparisonText(item), true
		}
	case "trends":
		for _, item := range analysis.Trends {
			if !matchesSubject(sourceRef, variable, item.AssetID, "") {
				continue
			}
			return item.Judgement, item.AssetTitle + "：" + item.Note, true
		}
	case "fatigue":
		for _, item := range analysis.Fatigue {
			if !matchesSubject(sourceRef, variable, item.AssetID, "") {
				continue
			}
			return item.Judgement, item.AssetTitle + "：" + item.Note, true
		}
	case "anomalies":
		for _, item := range analysis.Anomalies {
			if !matchesSubject(sourceRef, variable, item.AssetID, item.Date) {
				continue
			}
			return item.Judgement, item.Date + " " + item.Metric + "：" + item.Note, true
		}
	case "drivers":
		for _, item := range analysis.Drivers {
			if !matchesSubject(sourceRef, variable, "", item.Key) {
				continue
			}
			return item.Judgement, item.Label + " = " + item.Value + "：" + item.Note, true
		}
	}
	return Judgement{}, "", false
}

// matchesSubject：请求给了哪个就按哪个匹配，两个都给就都要对上。
func matchesSubject(sourceRef, variable, itemRef, itemVariable string) bool {
	if sourceRef != "" && sourceRef != itemRef {
		return false
	}
	if variable != "" && variable != itemVariable {
		return false
	}
	return sourceRef != "" || variable != ""
}

func firstFeatureKey(diffs []FeatureDiff) string {
	if len(diffs) == 0 {
		return ""
	}
	return diffs[0].Key
}

func comparisonText(item VariantComparison) string {
	changed := make([]string, 0, len(item.ChangedFeatures))
	for _, diff := range item.ChangedFeatures {
		changed = append(changed, diff.Label)
	}
	subject := "无差异"
	if len(changed) > 0 {
		subject = strings.Join(changed, "、")
	}
	return item.BaselineTitle + " vs " + item.VariantTitle + "（改了：" + subject + "）：" + item.Note
}
