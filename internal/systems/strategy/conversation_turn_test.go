package strategy

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/agent"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/knowledge"
	"github.com/shikanon/cookies/internal/platform/mediaunderstanding"
)

func TestConversationTurnDoesNotRepeatCapturedFields(t *testing.T) {
	t.Parallel()
	draft := BriefDraft{
		ID: "brief_draft_1", Status: "open", Version: 1,
		Document: EmptyBriefDocumentV2(), FieldStates: map[string]FieldState{},
	}
	message := Message{ID: "msg_1", CreatedBy: "user_1", Content: "品牌名称是灵裁，产品是电商创作，行业是信息服务业"}
	decision := sanitizeConversationDecision(draft, message, ConversationTurnDecision{
		Intent:         "provide_requirements",
		AssistantReply: "收到，我已经记录了品牌、产品和行业信息",
		Patch: BriefPatch{Operations: []BriefPatchOperation{
			{Op: "set", FieldPath: "brand.name", Value: json.RawMessage(`"灵裁"`), Confidence: "high"},
			{Op: "set", FieldPath: "product.name", Value: json.RawMessage(`"电商创作"`), Confidence: "high"},
			{Op: "set", FieldPath: "industry", Value: json.RawMessage(`"信息服务业"`), Confidence: "high"},
		}},
		FollowUpQuestions: []ConversationQuestion{
			{FieldPath: "brand.name", Text: "品牌名称是什么？"},
			{FieldPath: "product.name", Text: "本次推广的产品是什么？"},
		},
	})
	for index := range decision.Patch.Operations {
		decision.Patch.Operations[index].Source = FieldSource{Type: "conversation_message", ID: message.ID}
		decision.Patch.Operations[index].Confirmation = "unconfirmed"
	}
	updated, err := ApplyBriefPatch(draft, decision.Patch, PatchFromModel, "agent_1", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	decision = reconcileConversationTurn(updated, decision)
	if len(decision.FollowUpQuestions) != 2 ||
		decision.FollowUpQuestions[0].FieldPath != "campaign.objective" ||
		decision.FollowUpQuestions[1].FieldPath != "audience.primary" {
		t.Fatalf("questions = %#v", decision.FollowUpQuestions)
	}
	if strings.Contains(decision.AssistantReply, "品牌名称是什么") ||
		strings.Contains(decision.AssistantReply, "产品是什么") {
		t.Fatalf("assistant repeated captured fields: %q", decision.AssistantReply)
	}
}

func TestConversationTurnRejectsInferredAndUngroundedFields(t *testing.T) {
	t.Parallel()
	draft := BriefDraft{
		ID: "brief_draft_1", Status: "open", Version: 1,
		Document: EmptyBriefDocumentV2(), FieldStates: map[string]FieldState{},
	}
	message := Message{ID: "msg_1", Content: "我要给一个新护肤品牌做推广"}
	decision := sanitizeConversationDecision(draft, message, ConversationTurnDecision{
		Intent: "provide_requirements", AssistantReply: "明白",
		Patch: BriefPatch{Operations: []BriefPatchOperation{
			{Op: "set", FieldPath: "brand.name", Value: json.RawMessage(`"新护肤品牌（名称未指定）"`), Confidence: "low"},
			{Op: "set", FieldPath: "industry", Value: json.RawMessage(`"美妆个护"`), Confidence: "medium"},
			{Op: "set", FieldPath: "channels", Value: json.RawMessage(`["xiaohongshu"]`), Confidence: "high"},
			{Op: "set", FieldPath: "constraints", Value: json.RawMessage(`["预算有限"]`), Confidence: "high"},
		}},
	})
	if len(decision.Patch.Operations) != 0 {
		t.Fatalf("inferred operations were retained: %#v", decision.Patch.Operations)
	}
}

func TestConversationTurnRequiresEverySelectedChannelToBeGrounded(t *testing.T) {
	t.Parallel()
	message := Message{ID: "msg_1", Content: "首期只在小红书投放"}
	draft := BriefDraft{
		ID: "brief_draft_1", Status: "open", Version: 1,
		Document: EmptyBriefDocumentV2(), FieldStates: map[string]FieldState{},
	}
	decision := sanitizeConversationDecision(draft, message, ConversationTurnDecision{
		Intent: "provide_requirements",
		Patch: BriefPatch{Operations: []BriefPatchOperation{
			{Op: "set", FieldPath: "channels", Value: json.RawMessage(`["xiaohongshu","douyin"]`), Confidence: "high"},
		}},
	})
	if len(decision.Patch.Operations) != 0 {
		t.Fatalf("partly invented channels were retained: %#v", decision.Patch.Operations)
	}

	decision = sanitizeConversationDecision(draft, message, ConversationTurnDecision{
		Intent: "provide_requirements",
		Patch: BriefPatch{Operations: []BriefPatchOperation{
			{Op: "set", FieldPath: "channels", Value: json.RawMessage(`["xiaohongshu"]`), Confidence: "high"},
		}},
	})
	if len(decision.Patch.Operations) != 1 {
		t.Fatalf("grounded channel was rejected: %#v", decision.Patch.Operations)
	}
}

func TestConversationTurnAcceptsOnlyDocumentGroundedFactsAndKeepsChunkSource(t *testing.T) {
	t.Parallel()
	draft := BriefDraft{
		ID: "brief_draft_1", Status: "open", Version: 1,
		Document: EmptyBriefDocumentV2(), FieldStates: map[string]FieldState{},
	}
	message := Message{ID: "msg_1", Content: "请读取附件并整理需求"}
	grounding := []conversationGrounding{{
		Source:  FieldSource{Type: "knowledge_chunk", ID: "chunk_7", Locator: "产品说明:12-18"},
		Content: "产品：FlowKit\n核心受众：效率工具用户",
	}}
	decision := sanitizeConversationDecisionWithGrounding(draft, message, ConversationTurnDecision{
		Intent: "provide_requirements", AssistantReply: "已读取资料",
		Patch: BriefPatch{Operations: []BriefPatchOperation{
			{Op: "set", FieldPath: "product.name", Value: json.RawMessage(`"FlowKit"`), Confidence: "high"},
			{Op: "set", FieldPath: "brand.name", Value: json.RawMessage(`"资料中不存在的品牌"`), Confidence: "high"},
		}},
	}, grounding)
	if len(decision.Patch.Operations) != 1 || decision.Patch.Operations[0].FieldPath != "product.name" {
		t.Fatalf("document grounding filter = %#v", decision.Patch.Operations)
	}
	source := conversationOperationSource(message, grounding, decision.Patch.Operations[0])
	if source.Type != "knowledge_chunk" || source.ID != "chunk_7" || source.Locator != "产品说明:12-18" {
		t.Fatalf("document source = %#v", source)
	}
}

func TestConversationTurnAcceptsMultiProductCandidatesAcrossChunks(t *testing.T) {
	t.Parallel()
	draft := BriefDraft{Status: "open", Version: 1, Document: EmptyBriefDocumentV2(), FieldStates: map[string]FieldState{}}
	message := Message{ID: "msg_1", Content: "Please parse the attached Brief"}
	grounding := []conversationGrounding{
		{Source: FieldSource{Type: "knowledge_chunk", ID: "chunk_1", Locator: "lines:1-20"}, Content: "Product Alpha\nrepair and radiance\nAlpha must be shown"},
		{Source: FieldSource{Type: "knowledge_chunk", ID: "chunk_2", Locator: "lines:21-40"}, Content: "Product Beta\nhydration and firmness\ndo not claim allergy treatment"},
	}
	operation := BriefPatchOperation{
		Op: "set", FieldPath: "product.candidates", Confidence: "high",
		Value: json.RawMessage(`[
			{"name":"Product Alpha","category":"","selling_points":["repair and radiance"],"evidence":[],"mandatory_elements":["Alpha must be shown"],"prohibited_claims":[]},
			{"name":"Product Beta","category":"","selling_points":["hydration and firmness"],"evidence":[],"mandatory_elements":[],"prohibited_claims":["do not claim allergy treatment"]}
		]`),
	}
	if !productCandidatesAreGrounded(message.Content, grounding, operation.Value) {
		t.Fatalf("test product candidates should be grounded: %s", operation.Value)
	}
	decision := sanitizeConversationDecisionWithGrounding(draft, message, ConversationTurnDecision{
		Intent: "provide_requirements", AssistantReply: "Parsed", Patch: BriefPatch{Operations: []BriefPatchOperation{operation}},
	}, grounding)
	if len(decision.Patch.Operations) != 1 {
		t.Fatalf("grounded product candidates were rejected: %#v", decision.Patch.Operations)
	}
	enriched := enrichProductCandidateSources(message, grounding, decision.Patch.Operations[0].Value)
	var candidates []BriefProductCandidate
	if err := json.Unmarshal(enriched, &candidates); err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 || len(candidates[0].SourceRefs) != 1 || candidates[0].SourceRefs[0].ID != "chunk_1" ||
		len(candidates[1].SourceRefs) != 1 || candidates[1].SourceRefs[0].ID != "chunk_2" {
		t.Fatalf("candidate source refs = %#v", candidates)
	}

	operation.Value = json.RawMessage(`[{"name":"Product Alpha","category":"","selling_points":["invented claim"],"evidence":[],"mandatory_elements":[],"prohibited_claims":[]}]`)
	decision = sanitizeConversationDecisionWithGrounding(draft, message, ConversationTurnDecision{
		Intent: "provide_requirements", AssistantReply: "Parsed", Patch: BriefPatch{Operations: []BriefPatchOperation{operation}},
	}, grounding)
	if len(decision.Patch.Operations) != 1 {
		t.Fatalf("grounded candidate was discarded with its ungrounded fact: %#v", decision.Patch.Operations)
	}
	var sanitized []BriefProductCandidate
	if err := json.Unmarshal(decision.Patch.Operations[0].Value, &sanitized); err != nil {
		t.Fatal(err)
	}
	if len(sanitized) != 1 || len(sanitized[0].SellingPoints) != 0 {
		t.Fatalf("ungrounded candidate fact was retained: %#v", sanitized)
	}
}

func TestConversationGroundingReadsOnlyDirectMediaEvidence(t *testing.T) {
	t.Parallel()
	timestamp := int64(2400)
	ref := contract.AssetVersionRef{AssetID: "asset_video", Version: 2}
	artifact := mediaunderstanding.Artifact{
		ID: "media_1", Status: mediaunderstanding.StatusReady,
		VisibleText: []mediaunderstanding.Evidence{{
			ID: "visible_01", Text: "FlowKit", Confidence: .94,
			Locator: mediaunderstanding.Locator{Kind: "video_frame", AssetRef: contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: ref}, TimestampMS: &timestamp},
		}},
		Observations: []mediaunderstanding.Evidence{},
		Inferences: []mediaunderstanding.Evidence{{
			ID: "inference_01", Text: "适合年轻人", Confidence: .6,
			Locator: mediaunderstanding.Locator{Kind: "video_frame", AssetRef: contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: ref}, TimestampMS: &timestamp},
		}},
	}
	service := Service{ConversationMedia: staticConversationMedia{artifact: artifact}}
	grounding, err := service.loadConversationGrounding(context.Background(), agent.Task{
		OrganizationID: "org_1", ProjectID: "project_1", CreatedBy: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
	}, Message{ContentBlocks: []MessageContentBlock{{Type: "asset_ref", AssetKind: "video", AssetID: "asset_video", AssetVersion: 2}}})
	if err != nil || len(grounding) != 1 {
		t.Fatalf("media grounding=%#v err=%v", grounding, err)
	}
	if grounding[0].Content != "FlowKit" || grounding[0].Source.Type != "media_artifact" || grounding[0].Source.Locator != "video:2400ms" {
		t.Fatalf("media grounding source=%#v", grounding[0])
	}
}

type staticConversationMedia struct{ artifact mediaunderstanding.Artifact }

func (s staticConversationMedia) GetLatestForAsset(context.Context, contract.ActorContext, contract.ProjectID, contract.AssetVersionRef) (mediaunderstanding.Artifact, error) {
	return s.artifact, nil
}

func TestConversationConfirmationConfirmsCapturedFields(t *testing.T) {
	t.Parallel()
	now := time.Now()
	draft := BriefDraft{
		ID: "brief_draft_1", Status: "open", Version: 3,
		Document: EmptyBriefDocumentV2(),
		FieldStates: map[string]FieldState{
			"brand.name": {
				FieldPath: "brand.name", Confirmation: "unconfirmed",
				Source: FieldSource{Type: "conversation_message", ID: "msg_old"},
			},
		},
	}
	draft.Document.Brand.Name = "灵裁"
	draft.Completeness = ComputeCompleteness(draft.Document, draft.FieldStates)
	message := Message{ID: "msg_confirm", CreatedBy: "user_1", Content: "对"}
	decision := sanitizeConversationDecision(draft, message, ConversationTurnDecision{
		Intent: "confirm_information", AssistantReply: "好的", Patch: BriefPatch{Operations: []BriefPatchOperation{}},
	})
	if len(decision.ConfirmFields) != 1 || decision.ConfirmFields[0] != "brand.name" {
		t.Fatalf("confirm fields = %#v", decision.ConfirmFields)
	}
	updated, changed := applyConversationConfirmations(
		draft, decision.ConfirmFields, message.CreatedBy, message.ID, now, true,
	)
	if !changed || updated.Version != 4 || updated.FieldStates["brand.name"].Confirmation != "confirmed" {
		t.Fatalf("updated draft = %#v", updated)
	}
}

func TestConversationGreetingKeepsNaturalReplyAndLimitsQuestions(t *testing.T) {
	t.Parallel()
	draft := BriefDraft{
		ID: "brief_draft_1", Status: "open", Version: 1,
		Document: EmptyBriefDocumentV2(), FieldStates: map[string]FieldState{},
	}
	decision := reconcileConversationTurn(draft, ConversationTurnDecision{
		Intent: "greeting", AssistantReply: "你好，我会陪你把这次推广需求一步步梳理清楚。",
	})
	if !strings.HasPrefix(decision.AssistantReply, "你好") {
		t.Fatalf("reply = %q", decision.AssistantReply)
	}
	if len(decision.FollowUpQuestions) != 2 {
		t.Fatalf("questions = %#v", decision.FollowUpQuestions)
	}
}

func TestConversationDecisionOutputSchemaUsesStrictCompatibleTypes(t *testing.T) {
	t.Parallel()
	var schema map[string]any
	if err := json.Unmarshal(conversationDecisionOutputSchema(), &schema); err != nil {
		t.Fatal(err)
	}
	properties := schema["properties"].(map[string]any)
	if properties["intent"].(map[string]any)["type"] != "string" {
		t.Fatal("intent enum must declare its string type")
	}
	operationProperties := properties["operations"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	for _, name := range []string{"op", "field_path", "confidence"} {
		if operationProperties[name].(map[string]any)["type"] != "string" {
			t.Fatalf("operation property %q must declare its string type", name)
		}
	}
	value := operationProperties["value"].(map[string]any)
	if _, ok := value["anyOf"]; !ok {
		t.Fatal("operation value must use anyOf")
	}
	if _, ok := value["oneOf"]; ok {
		t.Fatal("operation value must not use oneOf")
	}
	if properties["confirm_fields"].(map[string]any)["items"].(map[string]any)["type"] != "string" {
		t.Fatal("confirm_fields items must declare their string type")
	}
	questionProperties := properties["follow_up_questions"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	if questionProperties["field_path"].(map[string]any)["type"] != "string" {
		t.Fatal("follow-up field_path must declare its string type")
	}
}

func TestConversationTurnMergesExplicitLabeledBriefFields(t *testing.T) {
	t.Parallel()
	draft := BriefDraft{
		ID: "brief_draft_1", Status: "open", Version: 1,
		Document: EmptyBriefDocumentV2(), FieldStates: map[string]FieldState{},
	}
	message := Message{
		ID: "msg_1", CreatedBy: "user_1",
		Content: `品牌：轻氧
产品：0 糖青柠气泡水
行业：饮料 / 即饮消费品
地区：中国大陆，一线与新一线城市
语言：简体中文
推广目标：夏季上新期间建立新品认知，提升品牌词搜索量和首购转化
核心受众：22–35 岁、一线和新一线城市女性
核心主张：0 糖清爽、青柠口感
渠道：小红书、抖音
预算：30 万元
周期：2026-08-01 至 2026-08-31
核心 KPI：品牌词搜索量提升 25%，首购转化率达到 3.5%
约束条件：不得使用减肥、治疗表述；不得虚构检测数据`,
	}
	patch := mergeExplicitLabeledBriefOperations(draft, message, BriefPatch{})
	updated, err := ApplyBriefPatch(draft, patch, PatchFromModel, "agent_1", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if updated.Document.Brand.Name != "轻氧" ||
		updated.Document.Product.Name != "0 糖青柠气泡水" ||
		updated.Document.Industry != "饮料 / 即饮消费品" ||
		updated.Document.Audience.Primary != "22–35 岁、一线和新一线城市女性" ||
		updated.Document.Proposition != "0 糖清爽、青柠口感" ||
		updated.Document.Budget.Total != "30 万元" ||
		updated.Document.Measurement.PrimaryKPI != "品牌词搜索量提升 25%，首购转化率达到 3.5%" {
		t.Fatalf("explicit fields were not retained: %#v", updated.Document)
	}
	if len(updated.Document.Channels) != 2 || len(updated.Document.Constraints) != 2 {
		t.Fatalf("channels or constraints were not retained: %#v", updated.Document)
	}
	for _, operation := range patch.Operations {
		if operation.Confidence != "high" {
			t.Fatalf("%s confidence = %q", operation.FieldPath, operation.Confidence)
		}
	}
}

func TestConversationTurnMergesExplicitNarrativeCNCBriefFields(t *testing.T) {
	t.Parallel()
	draft := BriefDraft{
		ID: "brief_draft_1", Status: "open", Version: 1,
		Document: EmptyBriefDocumentV2(), FieldStates: map[string]FieldState{},
	}
	message := Message{
		ID: "msg_cnc", CreatedBy: "user_1",
		Content: `白域精工希望面向研发负责人和采购负责人推广高精密 CNC 加工服务，在小红书建立专业认知并获取销售咨询。

核心能力包括：
1. 最高可实现 ±0.01mm 加工精度；
2. 历史订单准时交付率达到 98% 以上；
3. 可承接铝合金、不锈钢等材料的打样和小批量加工。

目前我们还不确定目标人群最关注哪些决策证据。`,
	}
	patch := mergeExplicitLabeledBriefOperations(draft, message, BriefPatch{Operations: []BriefPatchOperation{
		{Op: "set", FieldPath: "brand.name", Value: json.RawMessage(`"白域精工"`), Confidence: "high"},
		{Op: "set", FieldPath: "audience.primary", Value: json.RawMessage(`["研发负责人","采购负责人"]`), Confidence: "high"},
		{Op: "set", FieldPath: "channels", Value: json.RawMessage(`["xiaohongshu"]`), Confidence: "high"},
	}})
	if err := normalizeModelBriefPatch(&patch); err != nil {
		t.Fatal(err)
	}
	for index := range patch.Operations {
		patch.Operations[index].Source = FieldSource{Type: "conversation_message", ID: message.ID}
		patch.Operations[index].Confirmation = "unconfirmed"
	}
	updated, err := ApplyBriefPatch(draft, patch, PatchFromModel, "agent_1", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if updated.Document.Product.Name != "高精密 CNC 加工服务" ||
		updated.Document.Product.Category != "CNC 加工" ||
		updated.Document.Industry != "CNC 加工" {
		t.Fatalf("narrative product fields were not retained: %#v", updated.Document.Product)
	}
	if len(updated.Document.Product.SellingPoints) != 3 {
		t.Fatalf("selling points = %#v", updated.Document.Product.SellingPoints)
	}
	if len(updated.Document.Product.Evidence) != 2 {
		t.Fatalf("evidence = %#v", updated.Document.Product.Evidence)
	}
}

func TestConversationResearchGroundingAnswersButDoesNotMutateBrief(t *testing.T) {
	t.Parallel()
	hash := strings.Repeat("b", 64)
	service := Service{Knowledge: stubKnowledgeReader{values: map[string]knowledge.Reference{
		"research_1": {
			ID: "research_1", Kind: "research_artifact", Title: "行业查证",
			Content: "市场报告称核心受众是采购负责人", ContentHash: hash,
			Citations: []string{"https://example.com/report"},
		},
	}}}
	message := Message{
		ID: "message_1", Content: "请联网查证一下目标用户",
		ContentBlocks: []MessageContentBlock{{
			Type: "research_ref", ResearchArtifactID: "research_1", ExpectedContentHash: hash,
		}},
	}
	grounding, err := service.loadConversationGrounding(context.Background(), agent.Task{
		OrganizationID: "org_1", ProjectID: "project_1",
	}, message)
	if err != nil {
		t.Fatalf("load research grounding: %v", err)
	}
	if len(grounding) != 1 || grounding[0].Source.Type != "research_artifact" ||
		grounding[0].Source.Locator != "https://example.com/report" {
		t.Fatalf("grounding=%#v", grounding)
	}
	decision := sanitizeConversationDecisionWithGrounding(BriefDraft{
		Version: 1, Document: EmptyBriefDocumentV2(), FieldStates: map[string]FieldState{},
	}, message, ConversationTurnDecision{Patch: BriefPatch{Operations: []BriefPatchOperation{{
		Op: "set", FieldPath: "audience.primary", Value: json.RawMessage(`"采购负责人"`), Confidence: "high",
	}}}}, grounding)
	if len(decision.Patch.Operations) != 0 {
		t.Fatalf("research finding mutated Brief without user confirmation: %#v", decision.Patch.Operations)
	}
}
