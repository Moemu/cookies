package insights

import (
	"encoding/json"
	"strings"
	"testing"
)

// decodeSchema 把生成出来的 JSON Schema 解回来查。测试直接查 JSON 而不是查
// map[string]any 的中间结构——真正发给模型的是这段 JSON，查它才算查到了。
func decodeSchema(t *testing.T, assetType AssetType) map[string]any {
	t.Helper()
	schema, ok := FeatureSchemaFor(assetType)
	if !ok {
		t.Fatalf("%s 没有特征体系", assetType)
	}
	encoded, err := featureOutputSchema(schema)
	if err != nil {
		t.Fatalf("生成 %s 的输出格式失败：%v", assetType, err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("生成的不是合法 JSON：%v", err)
	}
	return decoded
}

func schemaProperties(t *testing.T, root map[string]any) map[string]any {
	t.Helper()
	properties, ok := root["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema 里没有 properties")
	}
	return properties
}

// 这是 03 §15② 的明文验收：不把视频钩子字段套到公众号文章。
// ValidateFeatureValue 已经在写库那一步挡了一道，但那是事后拒绝——
// 输出格式这一道是事前不允许，模型连返回都返回不了。两道都要有：
// 前一道保证脏数据进不来，这一道保证模型不用去试。
func TestOutputSchemaKeepsAssetTypesApart(t *testing.T) {
	articleProps := schemaProperties(t, decodeSchema(t, AssetTypeWechatArticle))
	if _, leaked := articleProps["hook_type"]; leaked {
		t.Fatal("公众号图文的输出格式里出现了视频的钩子字段")
	}

	adProps := schemaProperties(t, decodeSchema(t, AssetTypeDigitalHumanAd))
	if _, ok := adProps["hook_type"]; !ok {
		t.Fatal("数字人广告的输出格式里反而没有钩子字段")
	}
	if _, leaked := adProps["section_count"]; leaked {
		t.Fatal("数字人广告的输出格式里出现了公众号的章节数字段")
	}
}

// additionalProperties: false 是这套约束的地基。少了它，白名单只是建议：
// 模型可以在白名单之外再加字段，而多出来的那个字段没有任何一处会拦。
func TestOutputSchemaForbidsExtraFields(t *testing.T) {
	for _, schema := range AllFeatureSchemas() {
		root := decodeSchema(t, schema.AssetType)
		if root["additionalProperties"] != false {
			t.Fatalf("%s 的输出格式允许了额外字段", schema.Label)
		}
		for key, raw := range schemaProperties(t, root) {
			field, ok := raw.(map[string]any)
			if !ok {
				t.Fatalf("%s 的 %s 不是一个对象", schema.Label, key)
			}
			if field["additionalProperties"] != false {
				t.Fatalf("%s 的 %s 允许了额外字段", schema.Label, key)
			}
		}
	}
}

// 每个字段都必须要求 confidence 和 evidence。
//
// 没有置信度，复核队列没法排序——人只能从头看到尾，等于没有队列；
// 没有依据，复核的人只能凭印象点确认，那不叫复核。
func TestEveryFieldRequiresConfidenceAndEvidence(t *testing.T) {
	for _, schema := range AllFeatureSchemas() {
		for key, raw := range schemaProperties(t, decodeSchema(t, schema.AssetType)) {
			field := raw.(map[string]any)
			required := map[string]bool{}
			for _, item := range field["required"].([]any) {
				required[item.(string)] = true
			}
			if !required["confidence"] || !required["evidence"] {
				t.Fatalf("%s 的 %s 没有强制要求置信度和依据", schema.Label, key)
			}
			properties := field["properties"].(map[string]any)
			confidence := properties["confidence"].(map[string]any)
			values := confidence["enum"].([]any)
			if len(values) != 3 {
				t.Fatalf("%s 的 %s 置信度不是三档", schema.Label, key)
			}
		}
	}
}

// 有受控词表的字段，模型只能从词表里选；没有词表的字段保持开放。
//
// 后者是有意的：03 §5 末把词表发布权交给了能力运营的管理员，词表没发布之前
// 不能因此不让提取——那会让「先有数据才好定词表」这条路彻底走不通。
func TestVocabularyBecomesEnumOnlyWhenPublished(t *testing.T) {
	schema, _ := FeatureSchemaFor(AssetTypeDigitalHumanAd)
	properties := schemaProperties(t, decodeSchema(t, AssetTypeDigitalHumanAd))

	var checkedControlled, checkedOpen bool
	for _, field := range schema.Fields {
		terms, ok := properties[field.Key].(map[string]any)["properties"].(map[string]any)["terms"].(map[string]any)
		if !ok {
			continue
		}
		items := terms["items"].(map[string]any)
		if len(field.Vocabulary) > 0 {
			values, ok := items["enum"].([]any)
			if !ok {
				t.Fatalf("%s 有受控词表，输出格式却没有限制取值", field.Key)
			}
			if len(values) != len(field.Vocabulary) {
				t.Fatalf("%s 的词表少了几个取值：%d ≠ %d", field.Key, len(values), len(field.Vocabulary))
			}
			checkedControlled = true
			continue
		}
		if _, limited := items["enum"]; limited {
			t.Fatalf("%s 还没有发布词表，输出格式却限制了取值", field.Key)
		}
		description, _ := terms["description"].(string)
		if !strings.Contains(description, "同一个意思每次要写成同一个词") {
			t.Fatalf("%s 没有词表时应提醒模型保持用词稳定，否则同义词会被当成两个组", field.Key)
		}
		checkedOpen = true
	}
	if !checkedControlled || !checkedOpen {
		t.Fatal("这个类型里没有同时覆盖到有词表和无词表的字段，测试没测到东西")
	}
}

// 单选字段必须 maxItems: 1。单选之所以是单选，是因为后面要拿它分组：
// 一条素材落进两个组，会把两个组的样本量各算多一次，对比结果直接失真。
func TestSingleChoiceFieldsAreCappedAtOne(t *testing.T) {
	for _, schema := range AllFeatureSchemas() {
		properties := schemaProperties(t, decodeSchema(t, schema.AssetType))
		for _, field := range schema.Fields {
			if field.Kind != FeatureKindEnum {
				continue
			}
			terms := properties[field.Key].(map[string]any)["properties"].(map[string]any)["terms"].(map[string]any)
			if terms["maxItems"] != float64(1) {
				t.Fatalf("%s 的 %s 是单选，却没有限制只能选一个", schema.Label, field.Key)
			}
		}
	}
}

// 数值字段收数字，不收字符串。
//
// 收字符串意味着「3」和「三张」都能通过，而它们排不了序也算不了平均，
// 而可比较正是内容分析存在的理由。
func TestNumericFieldsAcceptNumbersNotStrings(t *testing.T) {
	properties := schemaProperties(t, decodeSchema(t, AssetTypeXiaohongshuNote))
	number := properties["image_count"].(map[string]any)["properties"].(map[string]any)["number"].(map[string]any)
	if number["type"] != "number" {
		t.Fatalf("图片数量收的不是数字，而是 %v", number["type"])
	}
	if description, _ := number["description"].(string); !strings.Contains(description, "张") {
		t.Fatal("有单位的数值字段没有把单位告诉模型，模型可能按别的单位填")
	}
}

// 没有任何字段是必填的。逼模型每个字段都填，它就会开始编——
// 而编出来的那条会带着 high 置信度混进已确认层，比空着危险得多。
func TestNoFieldIsMandatory(t *testing.T) {
	for _, schema := range AllFeatureSchemas() {
		root := decodeSchema(t, schema.AssetType)
		if required, ok := root["required"]; ok {
			t.Fatalf("%s 的输出格式把字段设成了必填：%v", schema.Label, required)
		}
	}
}

func TestFeatureInputsFromOutputMapsEachKind(t *testing.T) {
	schema, _ := FeatureSchemaFor(AssetTypeXiaohongshuNote)
	count := 4.0
	output := extractionOutput{
		"cover_subject":  {Text: "  产品特写  ", Confid: "HIGH", Evidence: "封面正中是产品"},
		"title_keywords": {Terms: []string{"平价", " 学生党 ", ""}, Confid: "medium", Evidence: "标题原文"},
		"image_count":    {Number: &count, Confid: "high", Evidence: "共 4 张图"},
		"cta":            {Terms: []string{"引导评论"}, Confid: "low", Evidence: "结尾一句"},
	}
	inputs, dropped := featureInputsFromOutput(schema, output)
	if len(dropped) != 0 {
		t.Fatalf("不该丢任何字段，却丢了：%v", dropped)
	}
	if len(inputs) != 4 {
		t.Fatalf("应该翻出 4 条特征，实际 %d 条", len(inputs))
	}

	byKey := map[string]FeatureInput{}
	for _, input := range inputs {
		byKey[input.Key] = input
	}
	if got := byKey["cover_subject"].Value.Text; got != "产品特写" {
		t.Fatalf("文本没有去掉首尾空白：%q", got)
	}
	if got := byKey["cover_subject"].Confidence; got != ConfidenceHigh {
		t.Fatalf("大写的 HIGH 没有被认出来，成了 %v", got)
	}
	if got := byKey["title_keywords"].Value.Terms; len(got) != 2 || got[1] != "学生党" {
		t.Fatalf("标签没有清掉空白和空串：%v", got)
	}
	if got := byKey["image_count"].Value.Number; got != 4 {
		t.Fatalf("数值没翻对：%v", got)
	}
	if got := byKey["image_count"].Value.Kind; got != FeatureKindNumber {
		t.Fatalf("值的类型应该跟着字段走，实际 %v", got)
	}
}

// 模型返回了不属于这个类型的字段时：丢掉，但必须回报。
// 静悄悄少几条比报错更糟——用户会以为提取过了，而那几个字段永远是空的。
func TestFeatureInputsFromOutputReportsWhatItDropped(t *testing.T) {
	schema, _ := FeatureSchemaFor(AssetTypeWechatArticle)
	output := extractionOutput{
		"hook_type":     {Terms: []string{"痛点"}, Confid: "high", Evidence: "开头"},
		"cta_link":      {Text: "", Confid: "high", Evidence: "没找到"},
		"section_count": {Number: nil, Confid: "medium", Evidence: "数不清"},
		"article_type":  {Terms: []string{"知识"}, Confid: "high", Evidence: "通篇是科普"},
	}
	inputs, dropped := featureInputsFromOutput(schema, output)
	if len(inputs) != 1 || inputs[0].Key != "article_type" {
		t.Fatalf("只有 article_type 该留下，实际留下 %v", inputs)
	}
	if len(dropped) != 3 {
		t.Fatalf("该回报 3 条被丢掉的，实际 %v", dropped)
	}
	joined := strings.Join(dropped, " / ")
	if !strings.Contains(joined, "hook_type") || !strings.Contains(joined, "不属于") {
		t.Fatalf("越界字段没有说清楚为什么被丢：%s", joined)
	}
	if !strings.Contains(joined, "cta_link") || !strings.Contains(joined, "section_count") {
		t.Fatalf("空值字段没有被回报：%s", joined)
	}
}

// absent 是「看过了，这条素材没有这个特征」，不是丢弃。
// 它不该产生特征记录，也不该出现在丢弃回报里——那会让人以为提取出了问题。
func TestAbsentIsNeitherAValueNorADrop(t *testing.T) {
	schema, _ := FeatureSchemaFor(AssetTypeXiaohongshuNote)
	inputs, dropped := featureInputsFromOutput(schema, extractionOutput{
		"cover_subject": {Absent: true, Confid: "high", Evidence: "封面是纯色背景，没有主体"},
	})
	if len(inputs) != 0 {
		t.Fatalf("明确说了没有的特征不该写进库：%v", inputs)
	}
	if len(dropped) != 0 {
		t.Fatalf("明确说了没有的特征不是被丢弃：%v", dropped)
	}
}

// 单选字段收到多个取值时只留第一个，不整条丢掉。
// 丢掉会让这个字段的覆盖率凭空低一截，而模型其实是看出来了的。
func TestSingleChoiceKeepsOnlyTheFirstTerm(t *testing.T) {
	schema, _ := FeatureSchemaFor(AssetTypeXiaohongshuNote)
	inputs, _ := featureInputsFromOutput(schema, extractionOutput{
		"opening_style": {Terms: []string{"提问", "场景"}, Confid: "medium", Evidence: "开头两句"},
	})
	if len(inputs) != 1 {
		t.Fatalf("应该留下 1 条，实际 %d 条", len(inputs))
	}
	if got := inputs[0].Value.Terms; len(got) != 1 || got[0] != "提问" {
		t.Fatalf("单选没有收敛到一个取值：%v", got)
	}
}

// 认不出来的置信度一律当 low。方向是有意的：往高了算会让这条提取
// 绕过复核，而它恰恰是最不该被信任的那条——连置信度都没写对的输出。
func TestUnknownConfidenceFallsToLow(t *testing.T) {
	for _, value := range []string{"", "很高", "0.9", "HIGHEST", "  "} {
		if got := normalizeConfidence(value); got != ConfidenceLow {
			t.Fatalf("置信度 %q 应该当成 low，实际 %v", value, got)
		}
	}
	if got := normalizeConfidence("  Medium "); got != ConfidenceMedium {
		t.Fatalf("带空白的 Medium 没被认出来：%v", got)
	}
}

// 翻出来的每一条都必须能通过写库那一道校验。
// 这两处要是对不上，提取会稳定地在最后一步失败，而失败点离原因很远。
func TestExtractedInputsPassTheWriteSideValidation(t *testing.T) {
	schema, _ := FeatureSchemaFor(AssetTypeDigitalHumanAd)
	hook, ok := schema.Field("hook_type")
	if !ok || len(hook.Vocabulary) == 0 {
		t.Fatal("钩子类型应该是个带受控词表的字段，测试的前提变了")
	}
	inputs, _ := featureInputsFromOutput(schema, extractionOutput{
		"hook_type": {Terms: []string{hook.Vocabulary[0]}, Confid: "high", Evidence: "开场三秒"},
		"voiceover": {Bool: boolPtr(true), Confid: "high", Evidence: "全程口播"},
	})
	if len(inputs) != 2 {
		t.Fatalf("应该翻出 2 条，实际 %d 条", len(inputs))
	}
	for _, input := range inputs {
		if err := ValidateFeatureValue(AssetTypeDigitalHumanAd, input.Key, input.Value.Terms); err != nil {
			t.Fatalf("%s 过不了写库校验：%v", input.Key, err)
		}
	}
}

func boolPtr(value bool) *bool { return &value }
