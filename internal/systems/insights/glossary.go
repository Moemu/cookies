package insights

// 名词表。这个模块以前同一件事有三四个叫法——「置信充分」「可归因」「强结论」
// 说的是同一档，「洞察卡」「经验」「结论」混着用。词多了不是丰富，是每读一屏
// 都得先在脑子里做一次翻译。
//
// 这张表是有约束力的：Avoid 里的说法不许再出现在任何用户能看见的文案里，
// glossary_test.go 会拦。

type GlossaryTerm struct {
	// Term 是唯一批准的叫法。
	Term string `json:"term"`
	// Means 用一句人话解释它是什么，不解释它怎么算。
	Means string `json:"means"`
	// Avoid 是被这个词取代的旧说法。
	Avoid []string `json:"avoid,omitempty"`
}

var insightGlossary = []GlossaryTerm{
	{
		Term:  "能归因",
		Means: "差异是真的，而且能归到某一个变量上，可以直接拿去指导下一轮。",
		// 「置信充分」不在这里：那是 ConfidenceLevel 的说法，是统计口径，
		// 和「能归因」不是一回事——口径充分只是能归因的必要条件之一。
		Avoid: []string{"强结论", "高置信"},
	},
	{
		Term:  "只是观察",
		Means: "看见了差异，但说不清是不是那个变量造成的——可能样本还不够稳，也可能有别的特征跟着一起变。",
		Avoid: []string{"方向性结论", "弱结论", "存在混杂结论"},
	},
	{
		Term:  "算不出来",
		Means: "数据太少，连差异存不存在都判断不了。不是「没差异」，是「不知道」。",
		// 同理，「样本不足」是 ConfidenceLevel 自己的说法，保留。这里只废掉
		// 那些和它同义又各说各话的叫法。
		Avoid: []string{"无结论", "低置信"},
	},
	{
		Term:  "做个实验",
		Means: "把「只是观察」升成「能归因」的唯一办法：事先定好只改哪一个变量、分几组、样本到多少才看结果。",
		Avoid: []string{"AB测", "跑实验验证"},
	},
	{
		Term:  "找相似素材",
		Means: "把「算不出来」升上去的办法：从库里拉内容变量重合的素材，把样本做厚了再判一次。",
		Avoid: []string{"扩样本", "召回相似"},
	},
	{
		Term:  "客观可测",
		Means: "从素材文件本身量出来的变量：时长、分辨率、镜头数、语速。算两遍结果一样。",
		Avoid: []string{"结构化特征", "硬特征"},
	},
	{
		Term:  "人工标注",
		Means: "人填的变量。人会错，但人为自己填的东西负责。",
		Avoid: []string{"人工特征", "手工标签"},
	},
	{
		Term:  "模型推断",
		Means: "模型看着素材猜出来的变量。可以摆在页面上参考，不能进结论。",
		Avoid: []string{"AI特征", "智能标签"},
	},
	{
		Term:  "记一笔",
		Means: "在分析页看到一条值得留下的发现时，把它钉进本轮复盘草稿。这是分析页唯一的写操作。",
		Avoid: []string{"收藏", "加入报告", "标记"},
	},
	{
		Term:  "发现",
		Means: "一条还没被人确认的结论，带着它的三档和理由。发现攒在复盘里，确认之后才变成经验。",
		Avoid: []string{"洞察卡", "候选结论", "分析项"},
	},
	{
		Term:  "复盘",
		Means: "一轮投放结束后，把这一轮的发现收在一起、逐条勾选提交的地方。",
		// 光写「报告」两个字会误伤——数据源报告、导入报告都还是报告。
		// 废掉的是把复盘叫成报告的那几个具体说法。
		Avoid: []string{"报告中心", "任务复盘报告"},
	},
	{
		Term:  "经验",
		Means: "人工确认过的发现。下一轮投前直接查它，它是这个模块最终要交付的东西。",
		Avoid: []string{"知识库", "沉淀", "已确认洞察"},
	},
}

func bannedAliases() []string {
	aliases := make([]string, 0, len(insightGlossary)*2)
	for _, term := range insightGlossary {
		aliases = append(aliases, term.Avoid...)
	}
	return aliases
}
