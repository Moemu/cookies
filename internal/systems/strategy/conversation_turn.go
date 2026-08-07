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
	"product.name", "campaign.objective", "audience.primary", "proposition",
}

var requiredConversationFieldsV1 = []string{
	"campaign.objective", "audience.primary", "proposition", "channels",
}

type conversationGrounding struct {
	Source  FieldSource
	Content string
}

func sanitizeConversationDecision(draft BriefDraft, message Message, decision ConversationTurnDecision) ConversationTurnDecision {
	return sanitizeConversationDecisionWithGrounding(draft, message, decision, nil)
}

func sanitizeConversationDecisionWithGrounding(
	draft BriefDraft,
	message Message,
	decision ConversationTurnDecision,
	grounding []conversationGrounding,
) ConversationTurnDecision {
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
		if operation.FieldPath == "product.candidates" {
			grounded, ok := sanitizeGroundedProductCandidates(message.Content, grounding, operation.Value)
			if operation.Confidence == "high" && ok {
				operation.Value = grounded
				operations = append(operations, operation)
			}
			continue
		}
		// Only explicit, high-confidence facts are written into the Brief.
		// Lower-confidence inferences remain questions instead of becoming data.
		// Attached document chunks are explicit evidence, but only when the
		// operation value can be found in that exact chunk.
		if operation.Confidence != "high" || !conversationOperationHasGrounding(message.Content, grounding, operation) {
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

func sanitizeGroundedProductCandidates(messageContent string, grounding []conversationGrounding, raw json.RawMessage) (json.RawMessage, bool) {
	var candidates []BriefProductCandidate
	if json.Unmarshal(raw, &candidates) != nil || len(candidates) == 0 || len(candidates) > 12 {
		return nil, false
	}
	contents := []string{messageContent}
	for _, source := range grounding {
		if source.Source.Type != "research_artifact" {
			contents = append(contents, source.Content)
		}
	}
	isGrounded := func(value string) bool {
		for _, content := range contents {
			if conversationValueIsGrounded(content, value) {
				return true
			}
		}
		return false
	}
	filter := func(values []string) []string {
		result := make([]string, 0, len(values))
		for _, value := range values {
			if isGrounded(value) {
				result = appendUnique(result, strings.TrimSpace(value))
			}
		}
		return result
	}
	result := make([]BriefProductCandidate, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate.Name = strings.TrimSpace(candidate.Name)
		key := strings.ToLower(candidate.Name)
		if candidate.Name == "" || !isGrounded(candidate.Name) {
			continue
		}
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		if !isGrounded(candidate.Category) {
			candidate.Category = ""
		}
		candidate.SellingPoints = filter(candidate.SellingPoints)
		candidate.Evidence = filter(candidate.Evidence)
		candidate.MandatoryElements = filter(candidate.MandatoryElements)
		candidate.ProhibitedClaims = filter(candidate.ProhibitedClaims)
		candidate.SourceRefs = nil
		result = append(result, candidate)
	}
	if len(result) == 0 {
		return nil, false
	}
	return mustJSON(result), true
}

func conversationOperationHasGrounding(messageContent string, grounding []conversationGrounding, operation BriefPatchOperation) bool {
	if operation.FieldPath == "product.candidates" {
		return productCandidatesAreGrounded(messageContent, grounding, operation.Value)
	}
	if conversationOperationIsGrounded(messageContent, operation) {
		return true
	}
	for _, source := range grounding {
		// Web findings may answer the user's question, but must never become a
		// confirmed business requirement without an explicit user statement.
		if source.Source.Type == "research_artifact" {
			continue
		}
		if conversationOperationIsGrounded(source.Content, operation) {
			return true
		}
	}
	return false
}

func conversationOperationSource(message Message, grounding []conversationGrounding, operation BriefPatchOperation) FieldSource {
	if operation.FieldPath == "product.candidates" {
		var candidates []BriefProductCandidate
		if json.Unmarshal(operation.Value, &candidates) == nil && len(candidates) > 0 {
			if conversationValueIsGrounded(message.Content, candidates[0].Name) {
				return FieldSource{Type: "conversation_message", ID: message.ID}
			}
			for _, source := range grounding {
				if source.Source.Type != "research_artifact" && conversationValueIsGrounded(source.Content, candidates[0].Name) {
					return source.Source
				}
			}
		}
	}
	if conversationOperationIsGrounded(message.Content, operation) {
		return FieldSource{Type: "conversation_message", ID: message.ID}
	}
	for _, source := range grounding {
		if conversationOperationIsGrounded(source.Content, operation) {
			return source.Source
		}
	}
	return FieldSource{Type: "conversation_message", ID: message.ID}
}

func productCandidatesAreGrounded(messageContent string, grounding []conversationGrounding, raw json.RawMessage) bool {
	var candidates []BriefProductCandidate
	if json.Unmarshal(raw, &candidates) != nil || len(candidates) == 0 || len(candidates) > 12 {
		return false
	}
	contents := []string{messageContent}
	for _, source := range grounding {
		if source.Source.Type != "research_artifact" {
			contents = append(contents, source.Content)
		}
	}
	for _, candidate := range candidates {
		values := []string{candidate.Name}
		if strings.TrimSpace(candidate.Category) != "" {
			values = append(values, candidate.Category)
		}
		values = append(values, candidate.SellingPoints...)
		values = append(values, candidate.Evidence...)
		values = append(values, candidate.MandatoryElements...)
		values = append(values, candidate.ProhibitedClaims...)
		for _, value := range values {
			grounded := false
			for _, content := range contents {
				if conversationValueIsGrounded(content, value) {
					grounded = true
					break
				}
			}
			if !grounded {
				return false
			}
		}
	}
	return true
}

func enrichProductCandidateSources(message Message, grounding []conversationGrounding, raw json.RawMessage) json.RawMessage {
	var candidates []BriefProductCandidate
	if json.Unmarshal(raw, &candidates) != nil {
		return raw
	}
	for index := range candidates {
		candidate := &candidates[index]
		values := append([]string{candidate.Name, candidate.Category}, candidate.SellingPoints...)
		values = append(values, candidate.Evidence...)
		values = append(values, candidate.MandatoryElements...)
		values = append(values, candidate.ProhibitedClaims...)
		if candidateValuesMatchContent(values, message.Content) {
			candidate.SourceRefs = append(candidate.SourceRefs, FieldSource{Type: "conversation_message", ID: message.ID})
		}
		for _, source := range grounding {
			if source.Source.Type != "research_artifact" && candidateValuesMatchContent(values, source.Content) {
				candidate.SourceRefs = append(candidate.SourceRefs, source.Source)
			}
		}
		candidate.SourceRefs = uniqueFieldSources(candidate.SourceRefs)
	}
	return mustJSON(candidates)
}

func candidateValuesMatchContent(values []string, content string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" && conversationValueIsGrounded(content, value) {
			return true
		}
	}
	return false
}

func conversationOperationIsGrounded(content string, operation BriefPatchOperation) bool {
	content = strings.ToLower(strings.TrimSpace(content))
	if content == "" {
		return false
	}
	if operation.FieldPath == "channels" {
		return conversationChannelsAreGrounded(content, operation.Value)
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

func conversationChannelsAreGrounded(content string, raw json.RawMessage) bool {
	var values []string
	if json.Unmarshal(raw, &values) != nil || len(values) == 0 {
		return false
	}
	aliases := map[string][]string{
		"xiaohongshu": {"小红书", "xiaohongshu", "rednote", "red note", "redbook", "red book"},
		"rednote":     {"小红书", "xiaohongshu", "rednote", "red note", "redbook", "red book"},
		"douyin":      {"抖音", "douyin"},
		"taobao":      {"淘宝", "taobao"},
		"tmall":       {"天猫", "tmall"},
		"wechat":      {"微信", "公众号", "视频号", "wechat"},
	}
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		markers := aliases[normalized]
		if len(markers) == 0 {
			markers = []string{normalized}
		}
		grounded := false
		for _, marker := range markers {
			if marker != "" && strings.Contains(content, marker) {
				grounded = true
				break
			}
		}
		if !grounded {
			return false
		}
	}
	return true
}

func conversationValueIsGrounded(content, value string) bool {
	content = strings.ToLower(strings.TrimSpace(content))
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
	// Confirmation follows what the user has actually supplied, not only the
	// minimum fields that block Creative intake. This keeps optional evidence
	// and context trustworthy without turning them into form requirements.
	candidates := append([]string{}, required...)
	for _, field := range []string{
		"brand.name", "product.name", "industry", "region", "language",
		"campaign.objective", "audience.primary", "proposition", "channels",
		"budget.total", "schedule.window", "measurement.primary_kpi",
	} {
		candidates = appendUnique(candidates, field)
	}
	result := make([]string, 0, len(candidates))
	for _, field := range candidates {
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
		"required":["intent","assistant_reply","product_candidates","operations","confirm_fields","follow_up_questions","warnings"],
		"properties":{
			"intent":{"type":"string","enum":["greeting","provide_requirements","answer_question","correct_information","confirm_information","ask_question","off_topic"]},
			"assistant_reply":{"type":"string","minLength":1,"maxLength":800},
			"product_candidates":{"type":"array","maxItems":12,"items":{"type":"object","additionalProperties":false,
				"required":["name","category","selling_points","evidence","mandatory_elements","prohibited_claims"],
				"properties":{"name":{"type":"string","minLength":1,"maxLength":160},"category":{"type":"string","maxLength":160},
				"selling_points":{"type":"array","maxItems":20,"items":{"type":"string","minLength":1,"maxLength":500}},
				"evidence":{"type":"array","maxItems":20,"items":{"type":"string","minLength":1,"maxLength":500}},
				"mandatory_elements":{"type":"array","maxItems":20,"items":{"type":"string","minLength":1,"maxLength":500}},
				"prohibited_claims":{"type":"array","maxItems":20,"items":{"type":"string","minLength":1,"maxLength":500}}}}},
			"operations":{"type":"array","maxItems":32,"items":{"type":"object",
				"additionalProperties":false,"required":["op","field_path","value","confidence"],
				"properties":{"op":{"type":"string","const":"set"},"field_path":{"type":"string","enum":%s},
				"value":{"anyOf":[{"type":"string"},{"type":"array","items":{"type":"string"}}]},
				"confidence":{"type":"string","enum":["low","medium","high"]}}}},
			"confirm_fields":{"type":"array","maxItems":16,"items":{"type":"string","enum":%s}},
			"follow_up_questions":{"type":"array","maxItems":2,"items":{"type":"object",
				"additionalProperties":false,"required":["field_path","text"],
				"properties":{"field_path":{"type":"string","enum":%s},"text":{"type":"string","minLength":1,"maxLength":160}}}},
			"warnings":{"type":"array","maxItems":8,"items":{"type":"string","maxLength":200}}
		}
	}`, fieldPaths, fieldPaths, fieldPaths))
}
