package insights

import (
	"strings"
	"testing"
)

// 判定只能由 judge() 从置信度收敛出来，测试里也不例外：
// 手拼一个 Verdict 能造出「置信充分但只是观察」这种现实中不存在的组合，
// 拿它验出来的筛选行为不作数。
func usable(applicability Applicability, confidence ConfidenceLevel) Experience {
	return Experience{
		Status:        ExperienceConfirmed,
		Applicability: applicability,
		Judgement:     judge(confidence, ""),
	}
}

// 适用条件是「这条经验在什么范围内成立」，不是标签。
// 筛的时候：经验没写这一格 = 不限，能匹配任何取值；写了就必须对上。
//
// 反过来做（没写就不匹配）会把绝大多数经验筛没——大部分经验只写了两三格。
func TestBlankApplicabilityMeansUnrestricted(t *testing.T) {
	t.Parallel()

	value := usable(Applicability{Channels: []string{"抖音"}}, ConfidenceSufficient)
	if _, ok := matchApplicability(value, ExperienceLookup{Channel: "抖音", Brand: "某美妆"}); !ok {
		t.Error("经验没写品牌就等于不限品牌，应该匹配上")
	}
	if _, ok := matchApplicability(value, ExperienceLookup{Channel: "小红书"}); ok {
		t.Error("写了抖音就不该匹配小红书")
	}
}

// 一格里可以有多个取值：一条经验同时适用于抖音和小红书是常态，
// 按相等去比会把它判成只适用于第一个。
func TestOneConditionCanHoldSeveralValues(t *testing.T) {
	t.Parallel()

	value := usable(Applicability{Channels: []string{"抖音", "小红书"}}, ConfidenceSufficient)
	for _, channel := range []string{"抖音", "小红书"} {
		if _, ok := matchApplicability(value, ExperienceLookup{Channel: channel}); !ok {
			t.Errorf("%s 在适用渠道里，应该匹配上", channel)
		}
	}
	if _, ok := matchApplicability(value, ExperienceLookup{Channel: "视频号"}); ok {
		t.Error("不在这组里的渠道不该匹配")
	}
}

// 匹配上了哪几格要说出来。人看到一条经验被推荐，第一个问题就是
// 「凭什么推给我」——答案是「因为渠道和广告类型都对上了」。
func TestMatchTellsWhichConditionsHit(t *testing.T) {
	t.Parallel()

	value := usable(Applicability{
		Channels: []string{"抖音"}, CreativeTypes: []string{"效果广告"},
	}, ConfidenceSufficient)
	match, ok := matchApplicability(value, ExperienceLookup{Channel: "抖音", AdType: "效果广告"})
	if !ok {
		t.Fatal("应该匹配上")
	}
	if len(match.Matched) != 2 {
		t.Errorf("应该报出两格匹配，得到 %v", match.Matched)
	}
}

// 默认只给能归因的。只是观察的要显式要（IncludeObserved），
// 而且要在结果里标出来它不是默认集里的。
func TestObservedExperiencesNeedAnExplicitAsk(t *testing.T) {
	t.Parallel()

	observed := usable(Applicability{Channels: []string{"抖音"}}, ConfidenceDirectional)
	if observed.Verdict != VerdictObserved {
		t.Fatalf("前提不成立：方向性应该收敛成只是观察，得到 %s", observed.Verdict)
	}

	if _, ok := matchApplicability(observed, ExperienceLookup{Channel: "抖音"}); ok {
		t.Error("默认不该给出只是观察的经验")
	}

	match, ok := matchApplicability(observed, ExperienceLookup{Channel: "抖音", IncludeObserved: true})
	if !ok {
		t.Fatal("显式要了就该给")
	}
	if match.Default {
		t.Error("只是观察的经验即使给出来，也不能标成默认可引用")
	}
}

// 停用的和待定的永远不进「查」。查的人要的是「能照着做的」，
// 把没确认的混进去，他分不出哪条是已经有人背过书的。
func TestLookupExcludesPendingAndRetired(t *testing.T) {
	t.Parallel()

	for _, status := range []ExperienceStatus{ExperiencePending, ExperienceRetired} {
		value := usable(Applicability{Channels: []string{"抖音"}}, ConfidenceSufficient)
		value.Status = status
		if _, ok := matchApplicability(value, ExperienceLookup{Channel: "抖音"}); ok {
			t.Errorf("%s 的经验不该出现在「查」里", status)
		}
	}
}

// 标了「该看一眼了」的仍然出现在「查」里，但要能被界面认出来。
// 藏起来的话，等于悄悄拿掉了一条正在被引用的经验。
func TestFlaggedExperienceStillShowsUpInLookup(t *testing.T) {
	t.Parallel()

	value := usable(Applicability{Channels: []string{"抖音"}}, ConfidenceSufficient)
	value.NeedsReview = true
	match, ok := matchApplicability(value, ExperienceLookup{Channel: "抖音"})
	if !ok {
		t.Fatal("标了复审的经验还在用，应该查得到")
	}
	if !match.Default {
		t.Error("标记不影响它是不是默认可引用")
	}
}

// 按内容特征找的时候，结论、建议动作、内容依据里的特征都算。
// 人问「有没有关于开场的经验」，想不起来那条当初把「开场」写在了哪一栏。
func TestFeatureLookupSearchesEverywhereTheWordCouldLive(t *testing.T) {
	t.Parallel()

	cases := map[string]Experience{
		"结论里": {Status: ExperienceConfirmed, Judgement: judge(ConfidenceSufficient, ""),
			Conclusion: "开场三秒露脸能拉完播。"},
		"建议动作里": {Status: ExperienceConfirmed, Judgement: judge(ConfidenceSufficient, ""),
			RecommendedAction: "下一版把开场改成人物正脸。"},
		"内容依据里": {Status: ExperienceConfirmed, Judgement: judge(ConfidenceSufficient, ""),
			ContentBasis: ContentBasis{Features: []string{"开场露脸"}}},
	}
	for where, value := range cases {
		if _, ok := matchApplicability(value, ExperienceLookup{Feature: "开场"}); !ok {
			t.Errorf("「开场」写在%s也该被找到", where)
		}
	}
	none := Experience{Status: ExperienceConfirmed, Judgement: judge(ConfidenceSufficient, ""),
		Conclusion: "字幕加粗能提升完播。"}
	if _, ok := matchApplicability(none, ExperienceLookup{Feature: "开场"}); ok {
		t.Error("哪都没提到「开场」的经验不该被找出来")
	}
}

// 引用一条经验时必须带上适用条件和来源。
//
// 只把结论抄走，下游拿到的是一句没有边界的断言：「痛点开场点击率高 38%」
// ——在什么渠道？什么广告类型？基于哪次投放？答不上来的话，这句话会被用到
// 它根本不成立的地方去，而用它的人没有任何线索能发现这一点。
func TestCitationCarriesScopeAndSource(t *testing.T) {
	t.Parallel()

	value := usable(Applicability{
		Channels: []string{"抖音"}, CreativeTypes: []string{"效果广告"},
	}, ConfidenceSufficient)
	value.Conclusion = "痛点开场比产品开场点击率高 38%"
	value.ReportID = "report_1"

	text := ExperienceMatch{Experience: value, Default: true}.CitationText()
	for _, want := range []string{"痛点开场", "抖音", "效果广告", "report_1"} {
		if !strings.Contains(text, want) {
			t.Errorf("引用文本里少了 %q：%s", want, text)
		}
	}
}

// 只是观察的经验被引用时，那句提醒必须跟着走。
// 它留在界面上而没进引用文本的话，抄到下游就变成了一条看起来同样可靠的结论。
func TestCitationOfObservedCarriesTheCaveat(t *testing.T) {
	t.Parallel()

	value := usable(Applicability{Channels: []string{"抖音"}}, ConfidenceDirectional)
	value.Conclusion = "15 秒整体好过 30 秒"
	text := ExperienceMatch{Experience: value, Default: false}.CitationText()
	if !strings.Contains(text, "只是观察") {
		t.Errorf("提醒没跟着引用走：%s", text)
	}
}

// 适用条件摆成一行文本的规则必须和前端 formatScope 一致：同一格的多个取值
// 用「/」连，格与格之间用「·」隔。都用同一个分隔符的话，
// 「抖音 · 小红书 · 效果广告」读起来像三个并列的限制，实际上是两格。
func TestApplicabilitySummaryMatchesTheFrontendRule(t *testing.T) {
	t.Parallel()

	summary := Applicability{
		Channels: []string{"抖音", "小红书"}, CreativeTypes: []string{"效果广告"},
	}.Summary()
	if summary != "抖音/小红书 · 效果广告" {
		t.Errorf("适用条件拼法和前端对不上：%s", summary)
	}
	if empty := (Applicability{}).Summary(); empty != "不限" {
		t.Errorf("一格都没写就是不限，得到：%s", empty)
	}
}

// 命中结果里要直接带着引用文本。下游拿到 JSON 就能原样抄走，
// 不用自己再拼一遍——各拼各的，迟早出现界面上和抄出去的不是一句话。
func TestMatchCarriesCitationText(t *testing.T) {
	t.Parallel()

	value := usable(Applicability{Channels: []string{"抖音"}}, ConfidenceSufficient)
	value.Conclusion = "痛点开场更好"
	match, ok := matchApplicability(value, ExperienceLookup{Channel: "抖音"})
	if !ok {
		t.Fatal("应该匹配上")
	}
	if match.Citation != match.CitationText() {
		t.Errorf("命中结果里的引用文本和方法算出来的不一致：%q / %q", match.Citation, match.CitationText())
	}
}
