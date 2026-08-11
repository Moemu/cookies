package insights

import "testing"

func intPtr(value int) *int { return &value }

// 没人调过的格子用代码默认值。
//
// 存成 0 或者在建表时把默认值写死进去，都会切断这条线：将来改了代码默认值，
// 那些从没被调过的部署不会跟着走，而它们本来应该跟着走。
func TestUnsetThresholdsFallBackToCodeDefaults(t *testing.T) {
	t.Parallel()

	got := resolve(ThresholdSet{Version: 3, Values: Thresholds{
		SufficientImpressions: intPtr(5000),
	}})

	if got.SufficientImpressions != 5000 {
		t.Errorf("调过的那格应该用调过的值，得到 %d", got.SufficientImpressions)
	}
	if got.DirectionalImpressions != directionalSampleImpressions {
		t.Errorf("没调过的那格应该用默认值 %d，得到 %d",
			directionalSampleImpressions, got.DirectionalImpressions)
	}
	if got.Version != 3 {
		t.Errorf("版本号要带上，得到 %d", got.Version)
	}
}

// 一条阈值都没存过时，解析出来的就是全套代码默认值，版本号为 0。
// 0 是有意义的：它表示「谁也没调过，跑的是出厂设定」。
func TestNoThresholdSetMeansVersionZero(t *testing.T) {
	t.Parallel()

	got := resolve(ThresholdSet{})
	if got.Version != 0 {
		t.Errorf("没存过应该是第 0 版，得到 %d", got.Version)
	}
	if got.SufficientImpressions != sufficientSampleImpressions {
		t.Errorf("应该是代码默认值，得到 %d", got.SufficientImpressions)
	}
}

// 「充分」的门槛不能低于「有方向」的门槛。
//
// 反过来设的话，一个样本量会同时满足「充分」和「不够充分只能看方向」，
// 判定顺序说了算——那意味着阈值页上两个看起来独立的数字，实际效果取决于
// 代码里 if 的先后。这种配置必须在保存时就拦下来。
func TestSufficientMustNotBeBelowDirectional(t *testing.T) {
	t.Parallel()

	bad := Thresholds{
		SufficientImpressions:  intPtr(500),
		DirectionalImpressions: intPtr(1000),
	}
	if err := bad.Validate(); err == nil {
		t.Error("充分门槛低于方向门槛应该被拒")
	}

	ok := Thresholds{SufficientImpressions: intPtr(8000), DirectionalImpressions: intPtr(1000)}
	if err := ok.Validate(); err != nil {
		t.Errorf("合法组合被拒了：%v", err)
	}
}

// 只调一格也要跟另一格比。只调「有方向」、把它抬到默认「充分」之上，
// 一样会造成两档重叠——校验只看填了的那两格的话，这种改法会被放过去。
func TestHalfSetThresholdsStillCompareAgainstTheDefault(t *testing.T) {
	t.Parallel()

	if err := (Thresholds{DirectionalImpressions: intPtr(sufficientSampleImpressions + 1)}).Validate(); err == nil {
		t.Error("方向门槛抬到默认充分门槛之上应该被拒")
	}
}

// 天数下限不能设成 1。一天的数据算不出趋势，也判不了异常——
// 允许设成 1，等于允许人把「拒绝下结论」这条规则关掉，而那是这个模块的灵魂。
func TestDayThresholdsHaveAFloor(t *testing.T) {
	t.Parallel()

	for _, value := range []Thresholds{
		{MinTrendDays: intPtr(1)},
		{MinAnomalyDays: intPtr(2)},
	} {
		if err := value.Validate(); err == nil {
			t.Errorf("天数低于下限应该被拒：%+v", value)
		}
	}
}

// 非正数一律拒。0 次曝光算充分等于取消样本门槛。
func TestNonPositiveThresholdsAreRejected(t *testing.T) {
	t.Parallel()

	if err := (Thresholds{SufficientImpressions: intPtr(0)}).Validate(); err == nil {
		t.Error("0 应该被拒")
	}
	if err := (Thresholds{MinDriverAssets: intPtr(-1)}).Validate(); err == nil {
		t.Error("负数应该被拒")
	}
}

// 理由必填。阈值本身合法但没写理由，照样存不进去——
// 三个月后回头看，一版没有理由的阈值和没记录过是一回事。
func TestSavingThresholdsNeedsAReason(t *testing.T) {
	t.Parallel()

	request := SaveThresholdsRequest{Values: Thresholds{SufficientImpressions: intPtr(8000)}}
	if err := request.Validate(); err == nil {
		t.Error("没写理由应该被拒")
	}
	request.Reason = "  "
	if err := request.Validate(); err == nil {
		t.Error("只有空白的理由应该被拒")
	}
	request.Reason = "本行业单条素材跑不到一万曝光，门槛降到八千。"
	if err := request.Validate(); err != nil {
		t.Errorf("写了理由的合法请求被拒了：%v", err)
	}
}

// 每条结论要盖上它用的那一版阈值。
//
// 没有这个号码，改完阈值之后所有历史结论都说不清是按什么标准算出来的。
// 经验库是「新增版本不覆盖、历史可审计」的——一批说不清依据的经验会永远
// 挂在账上，说不清是对是错。
func TestJudgementCarriesTheThresholdVersion(t *testing.T) {
	t.Parallel()

	thresholds := defaultThresholds()
	thresholds.Version = 7

	input := groupCompareInput{
		InGroup:      slicesWith(20000, 600),
		Rest:         slicesWith(20000, 400),
		SubjectLabel: "开场类型",
		Comparable:   true,
		Thresholds:   thresholds,
	}
	got := compareGroups(input).ThresholdVersion
	if got == nil {
		t.Fatal("判定没有记下阈值版本")
	}
	if *got != 7 {
		t.Errorf("判定应该记下第 7 版阈值，得到 %d", *got)
	}
}

// 出厂设定是第 0 版，也要如实盖上——空着的话，页面分不清
// 「跑的是出厂设定」和「这条结论早于阈值功能」这两种情况。
func TestDefaultThresholdsStampVersionZero(t *testing.T) {
	t.Parallel()

	input := groupCompareInput{
		InGroup:      slicesWith(20000, 600),
		Rest:         slicesWith(20000, 400),
		SubjectLabel: "开场类型", Comparable: true,
	}
	// 0 和「没有」是两回事：跑出厂设定要如实盖第 0 版，不能留空——
	// 留空的意思是「不知道这条按什么标准算的」，而这里明明知道。
	got := compareGroups(input).ThresholdVersion
	if got == nil {
		t.Fatal("跑出厂设定也要盖版本号，不能留空")
	}
	if *got != 0 {
		t.Errorf("出厂设定应该盖第 0 版，得到 %d", *got)
	}
}

// 屏级判定也要盖号。一屏上只出现一个标注，标的就是这个号——
// 屏级不盖的话，页面上唯一那处标注只能靠猜，或者干脆不标。
func TestScreenJudgementCarriesTheThresholdVersion(t *testing.T) {
	t.Parallel()

	thresholds := defaultThresholds()
	thresholds.Version = 4

	window := testWindow(10)
	analysis := buildPerformanceAnalysis(window, nil, nil, thresholds)
	got := analysis.Judgement.ThresholdVersion
	if got == nil {
		t.Fatal("屏级判定没有记下阈值版本")
	}
	if *got != 4 {
		t.Errorf("屏级判定应该记下第 4 版阈值，得到 %d", *got)
	}
}
