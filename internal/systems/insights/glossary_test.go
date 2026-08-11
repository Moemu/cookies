package insights

import (
	"strings"
	"testing"
)

// 名词表不是一张给人读的表，它是一条约束：表上写了「不要再叫」的词，
// 就不许再出现在任何用户能看见的文案里。没有这条，名词表两周后就作废了。
func TestNoBannedAliasAppearsInUserFacingLabels(t *testing.T) {
	t.Parallel()

	labels := map[string]string{}
	for _, v := range []Verdict{VerdictExplained, VerdictObserved, VerdictUnclear} {
		labels["Verdict."+string(v)] = v.Label()
	}
	for _, u := range []UpgradePath{UpgradeExperiment, UpgradeSimilar} {
		labels["UpgradePath."+string(u)] = u.Label()
	}
	for _, s := range []FeatureSource{SourceAI, SourceHuman, SourceDerived} {
		labels["FeatureSource."+string(s)] = s.Label()
	}
	for _, c := range allConfidenceLevels {
		labels["ConfidenceLevel."+string(c)] = c.Label()
	}

	for _, alias := range bannedAliases() {
		for where, label := range labels {
			if strings.Contains(label, alias) {
				t.Errorf("%s 的文案 %q 里还有被废弃的说法「%s」", where, label, alias)
			}
		}
	}
}

// 每个词都得说清楚三件事，缺一件这一行就没用：只有名字是废话，
// 只有解释没有「不要再叫」就拦不住旧说法回流。
func TestEveryGlossaryTermIsComplete(t *testing.T) {
	t.Parallel()

	if len(insightGlossary) == 0 {
		t.Fatal("名词表是空的")
	}
	seen := map[string]bool{}
	for _, term := range insightGlossary {
		if strings.TrimSpace(term.Term) == "" {
			t.Error("有一行没有名字")
			continue
		}
		if seen[term.Term] {
			t.Errorf("「%s」在名词表里出现了两次", term.Term)
		}
		seen[term.Term] = true
		if strings.TrimSpace(term.Means) == "" {
			t.Errorf("「%s」没有解释", term.Term)
		}
	}
}

// 三档、两条升级通道、三类变量必须在名词表上——它们是这个模块的中轴词，
// 中轴词不在表上，表就只是装饰。
func TestSpineTermsAreOnTheGlossary(t *testing.T) {
	t.Parallel()

	required := []string{"能归因", "只是观察", "算不出来", "做个实验", "找相似素材",
		"客观可测", "人工标注", "模型推断", "记一笔", "发现", "经验"}
	present := map[string]bool{}
	for _, term := range insightGlossary {
		present[term.Term] = true
	}
	for _, want := range required {
		if !present[want] {
			t.Errorf("中轴词「%s」不在名词表上", want)
		}
	}
}
