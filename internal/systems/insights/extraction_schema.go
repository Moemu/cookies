package insights

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// 这个文件把 features.go 里的特征体系翻译成模型的输出格式约束。
//
// **为什么不写一份提示词让模型自由返回。** 03 §MVP② 有一条明文验收：
// 「不把视频钩子字段套到公众号文章」。靠提示词说「请只填公众号的字段」是软约束——
// 模型偶尔会越界，而越界的那条特征一旦写进库，后面所有按特征分组的对比都会
// 多出一个只有它自己有的取值，看上去像发现，其实是污染。
//
// 这里的做法是硬约束：输出格式**由该素材类型的特征体系生成**，字段名是白名单，
// 受控词表是枚举，数值字段只接受数字。模型物理上返回不了不属于这个类型的东西。
//
// 代价是每类素材的格式说明都不小（品牌广告有 40 多个字段）。这个代价值得付：
// 它把一条验收从「记得测」变成了「结构上做不到」。

// extractedFeature 是模型对一个字段给出的答案。
//
// **Confidence 是必填的，Evidence 也是。** 没有置信度的提取没法进评测集，
// 也没法在界面上区分「模型很确定」和「模型在猜」；没有依据的提取没法复核——
// 人得能看到模型是凭哪一段内容得出这个结论的，才谈得上确认或者否掉。
type extractedFeature struct {
	Text     string   `json:"text,omitempty"`
	Terms    []string `json:"terms,omitempty"`
	Number   *float64 `json:"number,omitempty"`
	Bool     *bool    `json:"bool,omitempty"`
	Absent   bool     `json:"absent,omitempty"`
	Confid   string   `json:"confidence"`
	Evidence string   `json:"evidence"`
}

// extractionOutput 是模型返回的整份答案：字段键 → 答案。
type extractionOutput map[string]extractedFeature

// 所有没有受控词表的取值型字段（tags，以及词表尚未发布的 enum）都带这句话。
//
// **同义词漂移是这类字段唯一的致命伤。** 后面所有对比都是按取值分组算的，
// 模型这次写「平价」下次写「便宜」，一个组就被拆成两个，每组样本量减半，
// 显著性检验直接失效——而页面上看不出任何异常，只会显示两个都不显著。
const vocabularyStabilityHint = "。这个字段还没有受控词表，用简短稳定的说法，同一个意思每次要写成同一个词"

// featureOutputSchema 生成一个素材类型的 JSON Schema。
//
// 顶层是「字段键 → 该字段的答案对象」，`additionalProperties: false`，
// 没有一个字段是必填的——**「这条素材没有这个特征」和「模型没看出来」都应该是
// 允许的答案**。逼模型每个字段都填，它就会开始编。
func featureOutputSchema(schema FeatureSchema) (json.RawMessage, error) {
	properties := make(map[string]any, len(schema.Fields))
	for _, field := range schema.Fields {
		properties[field.Key] = fieldOutputSchema(field)
	}
	root := map[string]any{
		"type":                 "object",
		"description":          fmt.Sprintf("%s的内容特征。只填能从素材内容看出来的字段，看不出来的整个省略。", schema.Label),
		"properties":           properties,
		"additionalProperties": false,
	}
	encoded, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("生成 %s 的输出格式失败：%w", schema.AssetType, err)
	}
	return encoded, nil
}

// fieldOutputSchema 把一个字段翻译成它自己的答案对象。
//
// 值的键随类型变（text / terms / number / bool），不是所有类型都给一个通用的
// `value` 字符串——通用字符串意味着「3 张图」和「三张」都能通过，而它们没法比较，
// 而可比较正是内容分析存在的理由（20 §4.1）。
func fieldOutputSchema(field FeatureField) map[string]any {
	value := map[string]any{}
	switch field.Kind {
	case FeatureKindText:
		value["text"] = map[string]any{"type": "string", "description": "原文或概括，一句话"}
	case FeatureKindTags:
		value["terms"] = map[string]any{
			"type": "array", "items": map[string]any{"type": "string"},
			"description": "若干个词，用素材里实际出现的说法" + vocabularyStabilityHint,
		}
	case FeatureKindEnum, FeatureKindEnumMul:
		item := map[string]any{"type": "string"}
		if len(field.Vocabulary) > 0 {
			// 词表已发布：只能从里面选。词表还没发布的字段（03 §5 末把发布权交给
			// 能力运营的管理员）这里保持开放——那是有意的，不是漏了。
			item["enum"] = append([]string(nil), field.Vocabulary...)
		}
		terms := map[string]any{"type": "array", "items": item}
		if field.Kind == FeatureKindEnum {
			terms["maxItems"] = 1
			terms["description"] = "只选一个"
		} else {
			terms["description"] = "可以多选"
		}
		if len(field.Vocabulary) == 0 {
			terms["description"] = terms["description"].(string) + vocabularyStabilityHint
		}
		value["terms"] = terms
	case FeatureKindNumber:
		description := "数字"
		if field.Unit != "" {
			description = "数字，单位：" + field.Unit
		}
		value["number"] = map[string]any{"type": "number", "description": description}
	case FeatureKindBool:
		value["bool"] = map[string]any{"type": "boolean"}
	case FeatureKindDuration:
		value["number"] = map[string]any{"type": "number", "description": "秒数"}
	}

	// absent 让模型能明确说「看过了，这条素材没有这个特征」，区别于整个字段省略
	// （＝没看出来）。这两件事在复核队列里要分开：前者不用人再看，后者要。
	value["absent"] = map[string]any{
		"type":        "boolean",
		"description": "确认这条素材没有这个特征时填 true，此时不要填值",
	}
	value["confidence"] = map[string]any{
		"type": "string", "enum": []string{"high", "medium", "low"},
		"description": "从素材内容能直接看出来填 high；需要推断填 medium；不确定填 low",
	}
	value["evidence"] = map[string]any{
		"type":        "string",
		"description": "支持这个结论的原文片段或画面描述，供人工复核。不要写推理过程",
	}
	return map[string]any{
		"type":                 "object",
		"description":          strings.TrimSpace(field.Label + " " + field.Note),
		"properties":           value,
		"required":             []string{"confidence", "evidence"},
		"additionalProperties": false,
	}
}

// featureInputsFromOutput 把模型的答案翻回领域类型。
//
// **越界的字段直接丢掉，不报错。** 输出格式已经是白名单了，真出现越界说明
// provider 没有强制 schema（有些模型是尽力而为）。这时候丢掉那一条、留下其余的，
// 比整次提取失败更有用——但丢了什么必须回报给调用方，不能悄悄少几条。
func featureInputsFromOutput(schema FeatureSchema, output extractionOutput) ([]FeatureInput, []string) {
	keys := make([]string, 0, len(output))
	for key := range output {
		keys = append(keys, key)
	}
	// 排序只为让同样的输入产生同样的顺序：结果哈希要能对得上（03 §304）。
	sort.Strings(keys)

	inputs := make([]FeatureInput, 0, len(keys))
	dropped := make([]string, 0)
	for _, key := range keys {
		answer := output[key]
		field, ok := schema.Field(key)
		if !ok {
			dropped = append(dropped, fmt.Sprintf("%s（不属于%s的特征体系）", key, schema.Label))
			continue
		}
		if answer.Absent {
			continue
		}
		value, ok := featureValueFromAnswer(field, answer)
		if !ok {
			dropped = append(dropped, fmt.Sprintf("%s（没有给出%s类型的值）", key, string(field.Kind)))
			continue
		}
		input := FeatureInput{
			Key:        key,
			Value:      value,
			Confidence: normalizeConfidence(answer.Confid),
		}
		// 最后再过一遍写入侧的校验。**模型不该有能力让整批提取失败**：
		// 受控词表在 JSON Schema 里已经约束过一次，但有的供应商把 Schema
		// 当事后校验，仍会返回词表外的词。那时候丢掉这一条并报出来，
		// 好过让另外十几条提对了的特征陪着一起失败。
		if err := input.validate(schema.AssetType); err != nil {
			dropped = append(dropped, fmt.Sprintf("%s（%s）", key, strippedReason(err)))
			continue
		}
		inputs = append(inputs, input)
	}
	return inputs, dropped
}

// strippedReason 去掉错误里的 "insights request is invalid: " 前缀。
// 丢弃原因是给人看的一行字，不是错误链——留着前缀只会让页面上多一串英文。
func strippedReason(err error) string {
	return strings.TrimPrefix(err.Error(), ErrInvalidRequest.Error()+": ")
}

// featureValueFromAnswer 按字段类型取对应的那个值。取不到就返回 false——
// 空值不写进库：一条 text 为空的特征在覆盖率里算「填了」，在页面上是空白，
// 比没有这条记录更难解释。
func featureValueFromAnswer(field FeatureField, answer extractedFeature) (FeatureValue, bool) {
	switch field.Kind {
	case FeatureKindText:
		text := strings.TrimSpace(answer.Text)
		if text == "" {
			return FeatureValue{}, false
		}
		return FeatureValue{Kind: field.Kind, Text: text}, true
	case FeatureKindTags, FeatureKindEnum, FeatureKindEnumMul:
		terms := make([]string, 0, len(answer.Terms))
		for _, term := range answer.Terms {
			if trimmed := strings.TrimSpace(term); trimmed != "" {
				terms = append(terms, trimmed)
			}
		}
		if len(terms) == 0 {
			return FeatureValue{}, false
		}
		// 单选字段拿到多个取值时只留第一个。这不是纠错，是止损：
		// 单选之所以是单选，是因为后面要拿它分组，一条素材落进两个组会把
		// 每组的样本量都算多一次。
		if field.Kind == FeatureKindEnum && len(terms) > 1 {
			terms = terms[:1]
		}
		return FeatureValue{Kind: field.Kind, Terms: terms}, true
	case FeatureKindNumber, FeatureKindDuration:
		if answer.Number == nil {
			return FeatureValue{}, false
		}
		return FeatureValue{Kind: field.Kind, Number: *answer.Number}, true
	case FeatureKindBool:
		if answer.Bool == nil {
			return FeatureValue{}, false
		}
		return FeatureValue{Kind: field.Kind, Bool: *answer.Bool}, true
	}
	return FeatureValue{}, false
}

// normalizeConfidence 认不出来的一律当 low。
//
// **方向是有意的：不认识就往低了算。** 往高了算会让这条提取绕过复核队列，
// 而它恰恰是最不该被信任的那一条——连置信度都没写对的输出。
func normalizeConfidence(value string) Confidence {
	switch Confidence(strings.ToLower(strings.TrimSpace(value))) {
	case ConfidenceHigh:
		return ConfidenceHigh
	case ConfidenceMedium:
		return ConfidenceMedium
	}
	return ConfidenceLow
}
