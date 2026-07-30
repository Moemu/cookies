package strategy

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/systems/strategy/creativecatalog"
)

func deterministicCreativeBusinessStrategy(
	generation CreativeTaskGenerationContext,
) map[string]any {
	brief := generation.Brief.Snapshot
	answers := generation.Plan.Answers
	audience := firstNonEmpty(brief.Audience.Primary, "已确认目标人群")
	proposition := firstNonEmpty(brief.Proposition, firstString(brief.Product.SellingPoints), "已确认核心主张")
	scene := firstNonEmpty(
		firstString(brief.Audience.Scenarios),
		answerText(answers, "audience_scene"),
		firstString(brief.Audience.PainPoints),
	)
	result := map[string]any{}

	switch generation.Profile.BusinessCode {
	case "xiaohongshu_image_text":
		angle := fmt.Sprintf("围绕%s在%s中的真实决策问题，用%s完成场景—证据—行动的连接",
			audience, firstNonEmpty(scene, "典型使用场景"), proposition)
		result["content_angle"] = truncateCreativeText(angle, 300)
		result["message_priority"] = compactCreativeItems(5,
			"先呈现用户问题："+firstNonEmpty(firstString(brief.Audience.PainPoints), scene),
			"再建立单一主张："+proposition,
			"用已确认卖点解释价值："+strings.Join(brief.Product.SellingPoints, "、"),
			"用已确认事实降低疑虑："+strings.Join(brief.Product.Evidence, "、"),
			"最后承接互动："+answerOptionLabel(generation.Profile, answers, "interaction_goal"),
		)
		intents := answerOptionLabels(generation.Profile, answers, "search_intent")
		if interaction := answerOptionLabel(generation.Profile, answers, "interaction_goal"); interaction != "" {
			intents = append(intents, "主要互动目标："+interaction)
		}
		result["search_and_interaction_intent"] = compactCreativeItems(6, intents...)

	case "wechat_official_article":
		position := answerText(answers, "brand_position")
		result["core_argument"] = truncateCreativeText(
			firstNonEmpty(position, proposition)+"；让"+audience+"获得明确、可验证的判断依据",
			300,
		)
		result["argument_structure"] = compactCreativeItems(6,
			"读者价值："+answerText(answers, "reader_value"),
			"界定目标人群当前问题："+firstString(brief.Audience.PainPoints),
			"提出核心论点："+firstNonEmpty(position, proposition),
			"按信息优先级解释卖点："+strings.Join(brief.Product.SellingPoints, "、"),
			"用来源和证据支撑："+strings.Join(brief.Product.Evidence, "、"),
			"收束到业务目标："+brief.Campaign.Objective,
		)
		result["evidence_boundary"] = compactCreativeItems(8,
			"必须标注来源："+answerText(answers, "source_requirements"),
			"只使用 Brief 已确认事实："+strings.Join(brief.Product.Evidence, "、"),
			"没有来源的数据、功效或比较结论保持为待确认，不写成事实",
		)

	case "short_drama_preroll":
		result["audience_bridge"] = truncateCreativeText(
			firstNonEmpty(
				answerText(answers, "audience_connection"),
				fmt.Sprintf("%s与%s受众之间的共同问题或兴趣", proposition, audience),
			),
			400,
		)
		result["hook_mechanisms"] = compactCreativeItems(4,
			"从目标人群问题进入，不提前泄露具体剧情",
			"用产品价值与剧情情绪的冲突或悬念建立连接",
			"在极短时间内完成主张识别并自然交还正片",
		)
		result["continuity_guardrails"] = compactCreativeItems(6,
			"剧情类型与氛围："+answerText(answers, "drama_genre"),
			"连续性要求："+answerText(answers, "continuity_requirement"),
			"不得冒充正片内容或破坏人物、声音和叙事连续性",
		)

	case "game_preroll":
		result["player_motivation"] = truncateCreativeText(
			firstNonEmpty(answerText(answers, "player_motivation"), audience+"的真实玩法兴趣"),
			400,
		)
		result["gameplay_priority"] = compactCreativeItems(5,
			"优先突出已确认真实玩法："+answerText(answers, "gameplay"),
			"围绕玩家动机组织玩法信息："+answerText(answers, "player_motivation"),
			"转化动作："+answerOptionLabel(generation.Profile, answers, "conversion_action"),
		)
		result["truth_guardrails"] = compactCreativeItems(8,
			"只表达已确认且真实存在的玩法、角色、奖励和福利",
			"不得用生成画面暗示游戏内不存在的操作或结果",
			"所有奖励概率、领取条件和活动期限必须有事实依据",
			"Brief 证据："+strings.Join(brief.Product.Evidence, "、"),
		)

	case "commerce_preroll":
		offer := answerText(answers, "offer_facts")
		result["conversion_message"] = truncateCreativeText(
			proposition+joinCreativeClause("，已确认优惠事实为", offer),
			300,
		)
		result["message_sequence"] = compactCreativeItems(6,
			"商品识别："+firstNonEmpty(brief.Product.Name, "当前商品/SKU"),
			"核心转化主张："+proposition,
			"卖点优先级："+firstNonEmpty(strings.Join(brief.Product.SellingPoints, "、"), answerText(answers, "selling_point_priority")),
			"证据："+strings.Join(brief.Product.Evidence, "、"),
			"优惠事实："+offer,
			"行动："+answerOptionLabel(generation.Profile, answers, "conversion_action"),
		)
		result["opening_mechanisms"] = compactCreativeItems(4,
			"商品或关键使用动作先行，尽快建立品类识别",
			"从高优先级痛点或结果进入，但结果表达必须有证据",
			"优惠仅在事实完整且条件可见时作为开篇机制",
		)
		result["product_fidelity"] = compactCreativeItems(8,
			"包装外观、Logo、文字、颜色和 SKU 必须与已确认商品素材一致",
			"不得生成素材中不存在的包装结构、配件或功效标识",
			"价格、赠品、期限和适用条件以用户确认内容为准："+offer,
		)

	case "viral_remake":
		mechanisms := answerOptionLabels(generation.Profile, answers, "mechanism_focus")
		result["transferable_mechanisms"] = compactCreativeItems(
			6, append(mechanisms, "仅迁移可解释的抽象机制，不迁移具体表达")...,
		)
		result["product_mapping"] = compactCreativeItems(6,
			"目标产品："+firstNonEmpty(brief.Product.Name, proposition),
			"目标人群："+audience,
			"核心卖点："+strings.Join(brief.Product.SellingPoints, "、"),
			"业务目标："+brief.Campaign.Objective,
			"机制映射："+strings.Join(mechanisms, "、"),
		)
		result["non_copyable_elements"] = []string{
			"原视频的具体画面、人物形象、声音、台词、音乐、商标和可识别场景",
			"只有参考内容才拥有的剧情关系、镜头组合与独特表达",
			"没有证据支持的产品功效、结果、销量或用户评价",
		}
		result["originality_guardrails"] = compactCreativeItems(8,
			"保留机制层启发，重新建立与当前产品、人群、卖点和 CTA 的因果关系",
			"不得把参考链接或公开可访问等同于可下载、可训练或可商用",
			"生产性使用任何原始要素前，按实际用途确认权利范围",
		)

	case "brand_video":
		role := firstNonEmpty(answerText(answers, "brand_role"), "围绕本次品牌命题建立长期价值角色")
		result["brand_role"] = truncateCreativeText(role, 400)
		result["story_territories"] = compactCreativeItems(4,
			"从目标人群文化语境与生活张力进入："+firstNonEmpty(scene, audience),
			"围绕品牌长期主张与本次单一主张的关系展开："+answerText(answers, "brand_proposition"),
			"让产品在故事中承担品牌角色，而不是只做功能露出："+role,
		)
		result["emotional_direction"] = compactCreativeItems(4,
			"目标情绪连接："+answerText(answers, "emotional_goal"),
			"情绪必须服务于品牌主张："+proposition,
			"避免只追求短期点击而损害长期品牌记忆",
		)
		result["brand_memory_requirements"] = compactCreativeItems(8,
			"必须使用的记忆资产："+answerText(answers, "memory_assets"),
			"Brief 必须元素："+strings.Join(brief.Creative.MandatoryElements, "、"),
			"品牌语气："+strings.Join(brief.Creative.Tone, "、"),
		)
	}

	for _, field := range generation.Profile.OutputFields {
		if strategyFieldPresent(result[field.Key]) {
			continue
		}
		switch field.Type {
		case "string":
			result[field.Key] = truncateCreativeText(
				"基于已确认目标“"+brief.Campaign.Objective+"”与核心主张“"+proposition+"”形成的策略判断",
				field.MaxLength,
			)
		case "string_array":
			result[field.Key] = []string{
				"围绕已确认目标、人群、主张和约束形成可验证方向",
			}
		case "boolean":
			result[field.Key] = true
		}
	}
	return result
}

func answerText(answers map[string]json.RawMessage, key string) string {
	var value string
	if raw, found := answers[key]; found {
		_ = json.Unmarshal(raw, &value)
	}
	return strings.TrimSpace(value)
}

func answerOptionLabel(
	profile creativecatalog.Profile,
	answers map[string]json.RawMessage,
	key string,
) string {
	value := answerText(answers, key)
	if value == "" {
		return ""
	}
	for _, question := range profile.Questions {
		if question.ID != key {
			continue
		}
		for _, option := range question.Options {
			if option.Value == value {
				return option.Label
			}
		}
	}
	return value
}

func answerOptionLabels(
	profile creativecatalog.Profile,
	answers map[string]json.RawMessage,
	key string,
) []string {
	var values []string
	if raw, found := answers[key]; found {
		_ = json.Unmarshal(raw, &values)
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		label := value
		for _, question := range profile.Questions {
			if question.ID != key {
				continue
			}
			for _, option := range question.Options {
				if option.Value == value {
					label = option.Label
				}
			}
		}
		result = appendUniqueString(result, label)
	}
	return result
}

func compactCreativeItems(limit int, values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.HasSuffix(value, "：") {
			continue
		}
		value = strings.TrimSpace(strings.Trim(value, "；，"))
		result = appendUniqueString(result, value)
		if limit > 0 && len(result) == limit {
			break
		}
	}
	return result
}

func firstString(values []string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func joinCreativeClause(prefix, value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return prefix + strings.TrimSpace(value)
}

func truncateCreativeText(value string, maxLength int) string {
	value = strings.TrimSpace(value)
	if maxLength <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxLength {
		return value
	}
	return string(runes[:maxLength])
}
