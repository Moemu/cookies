package strategy

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
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
		decision.FollowUpQuestions[0].FieldPath != "region" ||
		decision.FollowUpQuestions[1].FieldPath != "language" {
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
