package strategy

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

var conversationFieldQuestions = map[string]string{
	"brand.name":              "品牌名称是什么？",
	"product.name":            "本次推广的产品或服务是什么？",
	"industry":                "所属行业是什么？",
	"region":                  "主要投放地区是哪里？",
	"language":                "内容使用什么语言？",
	"campaign.objective":      "这次最重要的业务目标是什么？",
	"audience.primary":        "最希望影响哪一类核心人群？",
	"proposition":             "最希望用户记住的核心卖点是什么？",
	"channels":                "首期希望覆盖哪些平台？",
	"budget.total":            "首期预算大致是多少？",
	"schedule.window":         "计划在什么时间范围内推进？",
	"measurement.primary_kpi": "最关注哪个核心指标？",
}

var requiredConversationFieldsV2 = []string{
	"brand.name", "product.name", "industry", "region", "language",
	"campaign.objective", "audience.primary", "proposition", "channels",
}

var requiredConversationFieldsV1 = []string{
	"campaign.objective", "audience.primary", "proposition", "channels",
}

func sanitizeConversationDecision(draft BriefDraft, message Message, decision ConversationTurnDecision) ConversationTurnDecision {
	decision.Intent = strings.TrimSpace(decision.Intent)
	switch decision.Intent {
	case "greeting", "provide_requirements", "answer_question", "correct_information",
		"confirm_information", "ask_question", "off_topic":
	default:
		decision.Intent = "provide_requirements"
	}
	decision.AssistantReply = strings.TrimSpace(decision.AssistantReply)
	if len([]rune(decision.AssistantReply)) > 800 {
		decision.AssistantReply = string([]rune(decision.AssistantReply)[:800])
	}

	operations := make([]BriefPatchOperation, 0, len(decision.Patch.Operations))
	for _, operation := range decision.Patch.Operations {
		// Only explicit, high-confidence facts are written into the Brief.
		// Lower-confidence inferences remain questions instead of becoming data.
		if operation.Confidence != "high" || !conversationOperationIsGrounded(message.Content, operation) {
			continue
		}
		operations = append(operations, operation)
	}
	decision.Patch.Operations = operations
	decision.Patch.BaseVersion = draft.Version
	decision.Patch.ContractVersion = "strategy-brief-patch/v1"
	if draft.Document.ContractVersion == "strategy-brief-version/v2" {
		decision.Patch.ContractVersion = "strategy-brief-patch/v2"
	}

	pending := pendingConversationFields(draft)
	pendingSet := make(map[string]struct{}, len(pending))
	for _, field := range pending {
		pendingSet[field] = struct{}{}
	}
	confirmed := make([]string, 0, len(decision.ConfirmFields))
	if explicitConversationConfirmation(message.Content) && len(decision.ConfirmFields) == 0 {
		decision.ConfirmFields = pending
	}
	for _, field := range decision.ConfirmFields {
		if _, ok := pendingSet[field]; ok {
			confirmed = appendUnique(confirmed, field)
		}
	}
	decision.ConfirmFields = confirmed
	return decision
}

func conversationOperationIsGrounded(content string, operation BriefPatchOperation) bool {
	content = strings.ToLower(strings.TrimSpace(content))
	if content == "" {
		return false
	}
	if operation.FieldPath == "channels" {
		for _, marker := range []string{
			"小红书", "rednote", "red note", "redbook", "red book",
			"抖音", "douyin", "淘宝", "天猫", "taobao", "tmall",
			"微信", "公众号", "视频号", "wechat",
		} {
			if strings.Contains(content, marker) {
				return true
			}
		}
		return false
	}
	var value string
	if json.Unmarshal(operation.Value, &value) == nil {
		return conversationValueIsGrounded(content, value)
	}

	var values []string
	if json.Unmarshal(operation.Value, &values) != nil || len(values) == 0 {
		return false
	}
	for _, item := range values {
		if !conversationValueIsGrounded(content, item) {
			return false
		}
	}
	return true
}

func conversationValueIsGrounded(content, value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	lowerValue := strings.ToLower(value)
	for _, marker := range []string{"未指定", "未提供", "未知", "待确认", "unspecified", "unknown", "not provided"} {
		if strings.Contains(lowerValue, marker) {
			return false
		}
	}
	return strings.Contains(content, lowerValue)
}

func explicitConversationConfirmation(content string) bool {
	normalized := strings.TrimSpace(strings.ToLower(content))
	normalized = strings.Trim(normalized, "。！!，,；; ")
	switch normalized {
	case "对", "是", "确认", "正确", "没问题", "没有问题", "都对", "以上正确", "以上确认":
		return true
	default:
		return false
	}
}

func conversationQuestionsFromStrings(values []string) []ConversationQuestion {
	result := make([]ConversationQuestion, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		for field, question := range conversationFieldQuestions {
			if value == question {
				result = append(result, ConversationQuestion{FieldPath: field, Text: value})
				break
			}
		}
	}
	return result
}

func applyConversationConfirmations(draft BriefDraft, fields []string, actorID, messageID string, now time.Time, incrementVersion bool) (BriefDraft, bool) {
	changed := false
	for _, field := range fields {
		state, ok := draft.FieldStates[field]
		if !ok || state.Confirmation != "unconfirmed" || !briefFieldPresent(draft.Document, field) {
			continue
		}
		state.Confirmation = "confirmed"
		state.Source = FieldSource{Type: "conversation_message", ID: messageID}
		state.UpdatedBy = actorID
		state.UpdatedAt = now.UTC()
		draft.FieldStates[field] = state
		changed = true
	}
	if !changed {
		return draft, false
	}
	if incrementVersion {
		draft.Version++
	}
	draft.UpdatedBy = actorID
	draft.UpdatedAt = now.UTC()
	draft.Completeness = ComputeCompleteness(draft.Document, draft.FieldStates)
	return draft, true
}

func reconcileConversationTurn(draft BriefDraft, decision ConversationTurnDecision) ConversationTurnDecision {
	missing := missingConversationFields(draft)
	missingSet := make(map[string]struct{}, len(missing))
	for _, field := range missing {
		missingSet[field] = struct{}{}
	}
	questions := make([]ConversationQuestion, 0, 2)
	for _, question := range decision.FollowUpQuestions {
		question.FieldPath = strings.TrimSpace(question.FieldPath)
		question.Text = strings.TrimSpace(question.Text)
		if _, ok := missingSet[question.FieldPath]; !ok || question.Text == "" {
			continue
		}
		questions = append(questions, question)
		delete(missingSet, question.FieldPath)
		if len(questions) == 2 {
			break
		}
	}
	for _, field := range missing {
		if len(questions) == 2 {
			break
		}
		if _, ok := missingSet[field]; !ok {
			continue
		}
		if text := conversationFieldQuestions[field]; text != "" {
			questions = append(questions, ConversationQuestion{FieldPath: field, Text: text})
		}
	}
	decision.FollowUpQuestions = questions
	decision.Patch.Questions = make([]string, 0, len(questions))
	for _, question := range questions {
		decision.Patch.Questions = append(decision.Patch.Questions, question.Text)
	}

	reply := strings.TrimSpace(decision.AssistantReply)
	if reply == "" {
		reply = fallbackConversationAcknowledgement(decision)
	}
	if len(questions) > 0 {
		values := make([]string, 0, len(questions))
		for _, question := range questions {
			values = append(values, question.Text)
		}
		reply = strings.TrimRight(reply, "。！？!? ") + "。\n\n接下来想确认：" + strings.Join(values, "；")
	} else if len(pendingConversationFields(draft)) > 0 {
		reply = strings.TrimRight(reply, "。！？!? ") + "。\n\n这些信息已经记录在 Brief 中，请在右侧核对；确认无误后即可进入策略生成。"
	} else if draft.Completeness.Ready {
		reply = strings.TrimRight(reply, "。！？!? ") + "。\n\nBrief 信息已完整，可以确认并进入策略生成。"
	}
	decision.AssistantReply = reply
	return decision
}

func fallbackConversationAcknowledgement(decision ConversationTurnDecision) string {
	switch decision.Intent {
	case "greeting":
		return "你好，我会通过几轮简短对话帮你把广告需求整理成可确认的 Brief"
	case "confirm_information":
		return "好的，已按你的确认更新当前 Brief"
	case "ask_question":
		return "我先回答你的问题，再继续梳理广告需求"
	case "off_topic":
		return "我可以继续帮你梳理这次广告的目标、受众和核心卖点"
	default:
		if len(decision.Patch.Operations) > 0 {
			return "收到，我已经把你刚才提供的信息整理进 Brief"
		}
		return "我理解了，我们继续把这次推广需求梳理清楚"
	}
}

func missingConversationFields(draft BriefDraft) []string {
	required := requiredConversationFieldsV1
	if draft.Document.ContractVersion == "strategy-brief-version/v2" {
		required = requiredConversationFieldsV2
	}
	result := make([]string, 0, len(required))
	for _, field := range required {
		if !briefFieldPresent(draft.Document, field) {
			result = append(result, field)
		}
	}
	return result
}

func pendingConversationFields(draft BriefDraft) []string {
	required := requiredConversationFieldsV1
	if draft.Document.ContractVersion == "strategy-brief-version/v2" {
		required = requiredConversationFieldsV2
	}
	result := make([]string, 0, len(required))
	for _, field := range required {
		if briefFieldPresent(draft.Document, field) && draft.FieldStates[field].Confirmation == "unconfirmed" {
			result = append(result, field)
		}
	}
	return result
}

func briefFieldPresent(document BriefDocument, field string) bool {
	switch field {
	case "brand.name":
		return strings.TrimSpace(document.Brand.Name) != ""
	case "product.name":
		return strings.TrimSpace(document.Product.Name) != ""
	case "industry":
		return strings.TrimSpace(document.Industry) != ""
	case "region":
		return strings.TrimSpace(document.Region) != ""
	case "language":
		return strings.TrimSpace(document.Language) != ""
	case "campaign.objective":
		return strings.TrimSpace(document.Campaign.Objective) != ""
	case "audience.primary":
		return strings.TrimSpace(document.Audience.Primary) != ""
	case "proposition":
		return strings.TrimSpace(document.Proposition) != ""
	case "channels":
		return len(document.Channels) > 0
	case "budget.total":
		return strings.TrimSpace(document.Budget.Total) != ""
	case "schedule.window":
		return strings.TrimSpace(document.Schedule.Window) != ""
	case "measurement.primary_kpi":
		return strings.TrimSpace(document.Measurement.PrimaryKPI) != ""
	default:
		return false
	}
}

func conversationDecisionOutputSchema() json.RawMessage {
	fieldPaths := `["brand.name","product.name","product.category","product.selling_points","product.evidence","industry","region","language","campaign.objective","audience.primary","proposition","channels","budget.total","schedule.window","constraints","measurement.primary_kpi","reference_ids","creative.tone","creative.mandatory_elements","creative.prohibited_claims"]`
	return json.RawMessage(fmt.Sprintf(`{
		"type":"object","additionalProperties":false,
		"required":["intent","assistant_reply","operations","confirm_fields","follow_up_questions","warnings"],
		"properties":{
			"intent":{"enum":["greeting","provide_requirements","answer_question","correct_information","confirm_information","ask_question","off_topic"]},
			"assistant_reply":{"type":"string","minLength":1,"maxLength":800},
			"operations":{"type":"array","maxItems":32,"items":{"type":"object",
				"additionalProperties":false,"required":["op","field_path","value","confidence"],
				"properties":{"op":{"const":"set"},"field_path":{"enum":%s},
				"value":{"oneOf":[{"type":"string"},{"type":"array","items":{"type":"string"}}]},
				"confidence":{"enum":["low","medium","high"]}}}},
			"confirm_fields":{"type":"array","maxItems":16,"items":{"enum":%s}},
			"follow_up_questions":{"type":"array","maxItems":2,"items":{"type":"object",
				"additionalProperties":false,"required":["field_path","text"],
				"properties":{"field_path":{"enum":%s},"text":{"type":"string","minLength":1,"maxLength":160}}}},
			"warnings":{"type":"array","maxItems":8,"items":{"type":"string","maxLength":200}}
		}
	}`, fieldPaths, fieldPaths, fieldPaths))
}
