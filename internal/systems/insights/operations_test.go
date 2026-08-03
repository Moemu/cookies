package insights

import (
	"encoding/json"
	"testing"
	"time"
)

// 能力运营是治理面：它算出来的数字被用来判断别处的数字可不可信。所以这一组测试
// 钉的不是「有没有输出」，而是几件**一放宽就会让治理面自己变得不可信**的事：
//
//   1. 取值分布必须用生效值。人改过的旧机器值不能再算成一个取值，否则词表看上去
//      比实际更碎，而碎掉的那一半根本没人在用。
//   2. 准确率必须有样本门槛。一条复核算 100% 放在治理面上比没有更糟。
//   3. 没人看过的提取不算样本。算成「对了」，准确率就会随提取量自动上涨。
//   4. 派生指标只能总量除总量。日均比率是另一个数，而且几乎总是错的。

func operationsWindow(days int) MetricWindow {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	return MetricWindow{Start: start, End: start.AddDate(0, 0, days-1)}
}

func typedAsset(id, title string, kind AssetType) Asset {
	return Asset{ID: id, Title: title, AssetType: kind, AnalysisStatus: AnalysisConfirmed}
}

func layeredFeature(assetID string, kind AssetType, key, term string, source FeatureSource, state ReviewState) AssetFeature {
	return AssetFeature{
		AssetID: assetID, AssetType: kind, Key: key,
		Value:        FeatureValue{Kind: FeatureKindEnum, Terms: []string{term}},
		Source:       source,
		ReviewState:  state,
		SkillID:      "skill_preroll",
		SkillVersion: "v1",
	}
}

func fieldUsage(t *testing.T, systems []FeatureSystemHealth, kind AssetType, key string) FeatureFieldUsage {
	t.Helper()
	for _, system := range systems {
		if system.AssetType != kind {
			continue
		}
		for _, field := range system.Fields {
			if field.Key == key {
				return field
			}
		}
	}
	t.Fatalf("特征体系里没有 %s 的 %s 字段", kind, key)
	return FeatureFieldUsage{}
}

// 人写了新值之后，机器的旧值就不再是「有人在用的取值」。
func TestHumanConclusionReplacesAIValueInVocabularyStats(t *testing.T) {
	assets := []Asset{typedAsset("asset_a", "前贴 A", AssetTypePrerollAd)}
	features := []AssetFeature{
		layeredFeature("asset_a", AssetTypePrerollAd, "opening_structure", "悬念开场", SourceAI, ReviewPending),
		layeredFeature("asset_a", AssetTypePrerollAd, "opening_structure", "利益开场", SourceHuman, ReviewRejected),
	}
	// 人推翻并给了自己的取值：这一行既是新值，也是对机器的否定。
	features[1].ReviewState = ReviewConfirmed

	report := buildCapabilityOperations(operationsWindow(7), assets, features, nil, nil, time.Now())
	field := fieldUsage(t, report.FeatureSystems, AssetTypePrerollAd, "opening_structure")
	if field.DistinctValues != 1 {
		t.Fatalf("同一条素材同一个字段只能贡献一个生效取值，实际 %d 个：%+v", field.DistinctValues, field.Values)
	}
	if field.Values[0].Value != "利益开场" {
		t.Fatalf("生效值应当是人写的那个，实际 %q", field.Values[0].Value)
	}
}

// 只被一条素材用过的取值进待归并队列——是候选，不是结论。
func TestSingleUseValuesBecomeMergeCandidates(t *testing.T) {
	assets := []Asset{
		typedAsset("asset_a", "前贴 A", AssetTypePrerollAd),
		typedAsset("asset_b", "前贴 B", AssetTypePrerollAd),
		typedAsset("asset_c", "前贴 C", AssetTypePrerollAd),
	}
	features := []AssetFeature{
		layeredFeature("asset_a", AssetTypePrerollAd, "opening_structure", "悬念开场", SourceAI, ReviewPending),
		layeredFeature("asset_b", AssetTypePrerollAd, "opening_structure", "悬念开场", SourceAI, ReviewPending),
		layeredFeature("asset_c", AssetTypePrerollAd, "opening_structure", "悬念式开场", SourceAI, ReviewPending),
	}
	report := buildCapabilityOperations(operationsWindow(7), assets, features, nil, nil, time.Now())
	field := fieldUsage(t, report.FeatureSystems, AssetTypePrerollAd, "opening_structure")
	if len(field.MergeCandidates) != 1 || field.MergeCandidates[0] != "悬念式开场" {
		t.Fatalf("只出现一次的取值应当进待归并队列，实际 %+v", field.MergeCandidates)
	}
	// 用过两次的那个不能进：它已经是事实上的通用取值。
	for _, candidate := range field.MergeCandidates {
		if candidate == "悬念开场" {
			t.Fatalf("用过两次的取值不该被当成碎片")
		}
	}
}

// 自由文本每条都不一样是设计意图，不是碎片化。
func TestFreeTextFieldsDoNotProduceMergeCandidates(t *testing.T) {
	assets := []Asset{
		typedAsset("asset_a", "数字人 A", AssetTypeDigitalHumanAd),
		typedAsset("asset_b", "数字人 B", AssetTypeDigitalHumanAd),
	}
	// pain_point 是 text：两条素材各写各的痛点，本来就该不一样。
	features := []AssetFeature{
		{AssetID: "asset_a", AssetType: AssetTypeDigitalHumanAd, Key: "pain_point",
			Value: FeatureValue{Kind: FeatureKindText, Text: "厨房油污反复擦不干净"}, Source: SourceAI, ReviewState: ReviewPending},
		{AssetID: "asset_b", AssetType: AssetTypeDigitalHumanAd, Key: "pain_point",
			Value: FeatureValue{Kind: FeatureKindText, Text: "地板拖完还是黏脚"}, Source: SourceAI, ReviewState: ReviewPending},
	}
	report := buildCapabilityOperations(operationsWindow(7), assets, features, nil, nil, time.Now())
	field := fieldUsage(t, report.FeatureSystems, AssetTypeDigitalHumanAd, "pain_point")
	if field.DistinctValues != 2 {
		t.Fatalf("两条素材写了两个痛点，取值数应当是 2，实际 %d", field.DistinctValues)
	}
	if len(field.MergeCandidates) != 0 {
		t.Fatalf("自由文本不该进待归并队列，否则队列会被必然唯一的值塞满：%+v", field.MergeCandidates)
	}
	if report.Dashboard.MergeCandidateCount != 0 {
		t.Fatalf("看板计数也不该把自由文本算进去，实际 %d", report.Dashboard.MergeCandidateCount)
	}
}

// 已发布词表的字段，取值全在表内时不应报「词表外存量」。
func TestGovernedFieldWithInVocabularyValuesHasNoBacklog(t *testing.T) {
	assets := []Asset{typedAsset("asset_a", "前贴 A", AssetTypePrerollAd)}
	features := []AssetFeature{{
		AssetID: "asset_a", AssetType: AssetTypePrerollAd, Key: "hook_type",
		Value:  FeatureValue{Kind: FeatureKindEnumMul, Terms: []string{"问题", "反差"}},
		Source: SourceAI, ReviewState: ReviewPending,
	}}
	report := buildCapabilityOperations(operationsWindow(7), assets, features, nil, nil, time.Now())
	field := fieldUsage(t, report.FeatureSystems, AssetTypePrerollAd, "hook_type")
	if !field.Governed {
		t.Fatalf("hook_type 在 features.go 里带受控词表，这里却判成未治理")
	}
	if len(field.OffVocabulary) != 0 {
		t.Fatalf("取值都在词表内，不该报词表外存量：%+v", field.OffVocabulary)
	}
}

// 样本不足时不给准确率，并且明说为什么。
func TestEvaluationWithFewSamplesRefusesToReportAccuracy(t *testing.T) {
	assets := []Asset{typedAsset("asset_a", "前贴 A", AssetTypePrerollAd)}
	features := []AssetFeature{
		layeredFeature("asset_a", AssetTypePrerollAd, "opening_structure", "悬念开场", SourceAI, ReviewPending),
		layeredFeature("asset_a", AssetTypePrerollAd, "opening_structure", "悬念开场", SourceHuman, ReviewConfirmed),
	}
	report := buildCapabilityOperations(operationsWindow(7), assets, features, nil, nil, time.Now())
	if len(report.Evaluations) != 1 {
		t.Fatalf("应当有一条评测结果，实际 %d 条", len(report.Evaluations))
	}
	evaluation := report.Evaluations[0]
	if evaluation.Confidence != ConfidenceLowSample {
		t.Fatalf("1 条样本必须判为样本不足，实际 %s", evaluation.Confidence)
	}
	if evaluation.Accuracy != 0 {
		t.Fatalf("样本不足时不能给准确率，实际 %.2f", evaluation.Accuracy)
	}
}

// 没有人工结论对照的提取不进样本：算成「机器对了」会让准确率随提取量自动上涨。
func TestUnreviewedExtractionsAreNotCountedAsCorrect(t *testing.T) {
	assets := make([]Asset, 0, 30)
	features := make([]AssetFeature, 0, 60)
	for index := 0; index < 30; index++ {
		id := "asset_" + string(rune('a'+index%26)) + string(rune('0'+index/26))
		assets = append(assets, typedAsset(id, "前贴", AssetTypePrerollAd))
		features = append(features, layeredFeature(id, AssetTypePrerollAd, "opening_structure", "悬念开场", SourceAI, ReviewPending))
		// 只有前 12 条有人复核过，其中 3 条被改写。
		if index >= 12 {
			continue
		}
		term := "悬念开场"
		if index < 3 {
			term = "利益开场"
		}
		features = append(features, layeredFeature(id, AssetTypePrerollAd, "opening_structure", term, SourceHuman, ReviewConfirmed))
	}
	report := buildCapabilityOperations(operationsWindow(7), assets, features, nil, nil, time.Now())
	if len(report.Evaluations) != 1 {
		t.Fatalf("应当只有一条评测结果，实际 %d 条", len(report.Evaluations))
	}
	evaluation := report.Evaluations[0]
	if evaluation.Reviewed != 12 {
		t.Fatalf("样本量只能数被复核过的 12 条，实际 %d 条", evaluation.Reviewed)
	}
	if evaluation.Agreed != 9 || evaluation.Disagreed != 3 {
		t.Fatalf("一致 9 / 不一致 3，实际 %d / %d", evaluation.Agreed, evaluation.Disagreed)
	}
	if evaluation.Accuracy < 0.74 || evaluation.Accuracy > 0.76 {
		t.Fatalf("准确率应当是 9/12=0.75，实际 %.3f", evaluation.Accuracy)
	}
	if len(evaluation.Examples) != 3 {
		t.Fatalf("三条被改写的应当都能举出例子，实际 %d 条", len(evaluation.Examples))
	}
}

// 人明确 rejected 时算不一致，哪怕取值碰巧相同。
func TestRejectedReviewCountsAsDisagreementEvenWithSameValue(t *testing.T) {
	assets := []Asset{typedAsset("asset_a", "前贴 A", AssetTypePrerollAd)}
	features := []AssetFeature{
		layeredFeature("asset_a", AssetTypePrerollAd, "opening_structure", "悬念开场", SourceAI, ReviewPending),
		layeredFeature("asset_a", AssetTypePrerollAd, "opening_structure", "悬念开场", SourceHuman, ReviewRejected),
	}
	report := buildCapabilityOperations(operationsWindow(7), assets, features, nil, nil, time.Now())
	if len(report.Evaluations) != 1 || report.Evaluations[0].Disagreed != 1 {
		t.Fatalf("人推翻了就算不一致，实际 %+v", report.Evaluations)
	}
}

// 派生指标必须用总量除总量，不能把每天的比率平均。
func TestDerivedMetricUsesTotalsNotDailyAverage(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	// 第一天 1000 曝光 100 点击（10%），第二天 9000 曝光 90 点击（1%）。
	// 日均比率 5.5%，总量口径 190/10000 = 1.9%。差得很远，正好用来分辨算法。
	facts := []MetricFactWithMapping{
		{MetricFact: MetricFact{StatDate: start, Counts: MetricCounts{Impressions: 1000, Clicks: 100}}},
		{MetricFact: MetricFact{StatDate: start.AddDate(0, 0, 1), Counts: MetricCounts{Impressions: 9000, Clicks: 90}}},
	}
	report := buildCapabilityOperations(operationsWindow(7), nil, nil, nil, facts, time.Now())
	var ctr MetricDictionaryEntry
	for _, entry := range report.Metrics {
		if entry.Key == "ctr" {
			ctr = entry
		}
	}
	if ctr.Key == "" {
		t.Fatal("指标字典里没有点击率")
	}
	if ctr.Total < 0.0189 || ctr.Total > 0.0191 {
		t.Fatalf("点击率应当是 190/10000=1.9%%，实际 %.4f（像是把日比率平均了）", ctr.Total)
	}
}

// 从没被填过的指标要能看出来，不能和「填了但恰好是 0」混在一起。
func TestMetricWithoutDataReportsZeroDays(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	facts := []MetricFactWithMapping{
		{MetricFact: MetricFact{StatDate: start, Counts: MetricCounts{Impressions: 1000, Clicks: 100}}},
	}
	report := buildCapabilityOperations(operationsWindow(7), nil, nil, nil, facts, time.Now())
	for _, entry := range report.Metrics {
		if entry.Key != "revenue_cents" {
			continue
		}
		if entry.DayCount != 0 {
			t.Fatalf("本项目一条收入都没导过，天数必须是 0，实际 %d", entry.DayCount)
		}
		return
	}
	t.Fatal("指标字典里没有收入")
}

// 口径冲突只影响依赖它的指标，不能一冲突就把整本字典标成不可比。
func TestCaliberConflictOnlyMarksDependentMetrics(t *testing.T) {
	sources := []DataSource{
		{ID: "src_a", Platform: PlatformDouyin, AccountLabel: "抖音主号", Status: DataSourceActive,
			Caliber: MetricCaliber{TimeZone: "Asia/Shanghai", Currency: "CNY", AttributionWindow: "7d", MetricSchemaVersion: "v1"}},
		{ID: "src_b", Platform: PlatformKuaishou, AccountLabel: "快手主号", Status: DataSourceActive,
			Caliber: MetricCaliber{TimeZone: "Asia/Shanghai", Currency: "CNY", AttributionWindow: "1d", MetricSchemaVersion: "v1"}},
	}
	report := buildCapabilityOperations(operationsWindow(7), nil, nil, sources, nil, time.Now())
	if len(report.CaliberConflicts) != 1 || report.CaliberConflicts[0].Factor != CaliberAttribution {
		t.Fatalf("只有归因窗口不一致，实际 %+v", report.CaliberConflicts)
	}
	byKey := map[string]MetricDictionaryEntry{}
	for _, entry := range report.Metrics {
		byKey[entry.Key] = entry
	}
	if byKey["conversions"].Comparable {
		t.Fatal("转化依赖归因窗口，归因窗口不一致时必须标成不可跨源比较")
	}
	if !byKey["impressions"].Comparable {
		t.Fatal("曝光不依赖归因窗口，不该被这次冲突牵连")
	}
	if report.Dashboard.CaliberConflictCount != 1 {
		t.Fatalf("质量看板上的冲突数应当和口径视图一致，实际 %d", report.Dashboard.CaliberConflictCount)
	}
}

// 只有一个数据源时不存在「跨源不可比」。
func TestSingleDataSourceProducesNoCaliberConflict(t *testing.T) {
	sources := []DataSource{
		{ID: "src_a", Platform: PlatformDouyin, AccountLabel: "抖音主号", Status: DataSourceActive,
			Caliber: MetricCaliber{TimeZone: "Asia/Shanghai", Currency: "CNY", AttributionWindow: "7d", MetricSchemaVersion: "v1"}},
	}
	report := buildCapabilityOperations(operationsWindow(7), nil, nil, sources, nil, time.Now())
	if len(report.CaliberConflicts) != 0 {
		t.Fatalf("单数据源不该有口径冲突，实际 %+v", report.CaliberConflicts)
	}
}

// 草稿数据源导不进数据，它的默认口径不能拿来报冲突。
func TestDraftDataSourceIsIgnoredInCaliberCheck(t *testing.T) {
	sources := []DataSource{
		{ID: "src_a", Platform: PlatformDouyin, AccountLabel: "抖音主号", Status: DataSourceActive,
			Caliber: MetricCaliber{TimeZone: "Asia/Shanghai", Currency: "CNY", AttributionWindow: "7d", MetricSchemaVersion: "v1"}},
		{ID: "src_b", Platform: PlatformKuaishou, AccountLabel: "快手待接", Status: DataSourceDraft,
			Caliber: MetricCaliber{TimeZone: "UTC", Currency: "USD", AttributionWindow: "1d", MetricSchemaVersion: "v2"}},
	}
	report := buildCapabilityOperations(operationsWindow(7), nil, nil, sources, nil, time.Now())
	if len(report.CaliberConflicts) != 0 {
		t.Fatalf("草稿源还没导入过数据，不该报冲突：%+v", report.CaliberConflicts)
	}
}

// 没人用过的字段，词表没发布也不该塞进治理待办——真正在散的那几个会被埋掉。
func TestUnusedOpenVocabularyFieldsStayOutOfTheTodoList(t *testing.T) {
	assets := []Asset{typedAsset("asset_a", "前贴 A", AssetTypePrerollAd)}
	features := []AssetFeature{
		layeredFeature("asset_a", AssetTypePrerollAd, "opening_structure", "悬念开场", SourceAI, ReviewPending),
	}
	report := buildCapabilityOperations(operationsWindow(7), assets, features, nil, nil, time.Now())
	vocabularyTodos := 0
	for _, todo := range report.Dashboard.Todos {
		if todo.Kind != "vocabulary" {
			continue
		}
		vocabularyTodos++
		if todo.FeatureKey != "opening_structure" {
			t.Fatalf("只有 opening_structure 被用过，不该为 %s 生成词表待办", todo.FeatureKey)
		}
	}
	if vocabularyTodos != 1 {
		t.Fatalf("应当只有一条词表待办，实际 %d 条", vocabularyTodos)
	}
	// 敞口计数是另一回事：没人用过的开放枚举仍然算敞口，只是不进待办。
	if report.Dashboard.OpenVocabularyFields <= 1 {
		t.Fatalf("开放枚举敞口应当把没用过的也数进去，实际 %d", report.Dashboard.OpenVocabularyFields)
	}
}

// Skill 版本按最近一次提取时间判在用，不按版本号字符串排序。
func TestLatestSkillVersionIsChosenByExtractionTime(t *testing.T) {
	older := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	assets := []Asset{typedAsset("asset_a", "前贴 A", AssetTypePrerollAd), typedAsset("asset_b", "前贴 B", AssetTypePrerollAd)}
	v9 := layeredFeature("asset_a", AssetTypePrerollAd, "opening_structure", "悬念开场", SourceAI, ReviewPending)
	v9.SkillVersion, v9.ExtractedAt = "v9", &older
	v10 := layeredFeature("asset_b", AssetTypePrerollAd, "opening_structure", "利益开场", SourceAI, ReviewPending)
	v10.SkillVersion, v10.ExtractedAt = "v10", &newer

	report := buildCapabilityOperations(operationsWindow(7), assets, []AssetFeature{v9, v10}, nil, nil, time.Now())
	if len(report.Skills) != 2 {
		t.Fatalf("两个版本都要列出来（历史版本提的特征还在库里），实际 %d 个", len(report.Skills))
	}
	for _, skill := range report.Skills {
		if skill.SkillVersion == "v10" && !skill.Latest {
			t.Fatal("v10 提取得更晚，应当标为在用")
		}
		if skill.SkillVersion == "v9" && skill.Latest {
			t.Fatal("v9 提取得更早，不该标为在用（字符串排序会把 v9 排在 v10 后面）")
		}
	}
}

// 空列表必须序列化成 []，不能是 null。nil 切片序列化成 null 之后，前端取 .length
// 会直接整页白屏——治理面看不见比数字算错更早暴露，但同样是「这一页不能用」。
// 而且「没有口径冲突」本身是一个要被读出来的结论，它得有个明确的零。
func TestEmptyCapabilityOperationsSerializesToEmptyArrays(t *testing.T) {
	report := buildCapabilityOperations(operationsWindow(30), nil, nil, nil, nil,
		time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC))

	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("序列化失败：%v", err)
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("反序列化失败：%v", err)
	}

	for _, field := range []string{"feature_systems", "metrics", "caliber_conflicts", "skills", "evaluations"} {
		raw, ok := decoded[field]
		if !ok {
			t.Fatalf("%s 字段整个不见了", field)
		}
		if string(raw) == "null" {
			t.Fatalf("%s 序列化成了 null，前端取 .length 会白屏", field)
		}
	}

	var dashboard struct {
		Todos json.RawMessage `json:"todos"`
	}
	if err := json.Unmarshal(decoded["dashboard"], &dashboard); err != nil {
		t.Fatalf("看板反序列化失败：%v", err)
	}
	if string(dashboard.Todos) == "null" {
		t.Fatal("dashboard.todos 序列化成了 null")
	}
}
