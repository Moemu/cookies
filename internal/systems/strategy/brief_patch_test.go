package strategy

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestApplyBriefPatchProtectsConfirmedFacts(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 23, 8, 0, 0, 0, time.UTC)
	draft := BriefDraft{
		ID: "draft_1", Status: "open", Version: 3, Document: EmptyBriefDocument(),
		FieldStates: map[string]FieldState{
			"audience.primary": {
				FieldPath: "audience.primary", Confirmation: "confirmed",
				Source: FieldSource{Type: "user_edit", ID: "user_1"},
			},
		},
	}
	draft.Document.Audience.Primary = "研发负责人"
	patch := BriefPatch{BaseVersion: 3, Operations: []BriefPatchOperation{{
		Op: "set", FieldPath: "audience.primary", Value: json.RawMessage(`"采购负责人"`),
		Source: FieldSource{Type: "conversation_message", ID: "msg_2"}, Confidence: "high",
	}}}
	updated, err := ApplyBriefPatch(draft, patch, PatchFromModel, "agent_1", now)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Document.Audience.Primary != "研发负责人" {
		t.Fatalf("confirmed value was overwritten: %q", updated.Document.Audience.Primary)
	}
	if len(updated.FieldStates["audience.primary"].Conflicts) != 1 {
		t.Fatalf("expected one conflict, got %#v", updated.FieldStates["audience.primary"].Conflicts)
	}
}

func TestUserPatchProducesConfirmableCompleteBrief(t *testing.T) {
	t.Parallel()
	draft := BriefDraft{ID: "draft_1", Status: "open", Version: 1, Document: EmptyBriefDocument(), FieldStates: map[string]FieldState{}}
	patch := BriefPatch{ExpectedVersion: 1, Operations: []BriefPatchOperation{
		{Op: "set", FieldPath: "campaign.objective", Value: json.RawMessage(`"新品认知"`)},
		{Op: "set", FieldPath: "audience.primary", Value: json.RawMessage(`"研发负责人"`)},
		{Op: "set", FieldPath: "proposition", Value: json.RawMessage(`"缩短研发周期"`)},
		{Op: "set", FieldPath: "channels", Value: json.RawMessage(`["xiaohongshu"]`)},
	}}
	updated, err := ApplyBriefPatch(draft, patch, PatchFromUser, "user_1", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Completeness.Ready || updated.Version != 2 {
		t.Fatalf("updated draft = %#v", updated)
	}
	for _, path := range []string{"campaign.objective", "audience.primary", "proposition", "channels"} {
		if updated.FieldStates[path].Confirmation != "confirmed" {
			t.Fatalf("%s was not confirmed", path)
		}
	}
}

func TestApplyBriefPatchRejectsStaleAndUnknownFields(t *testing.T) {
	t.Parallel()
	draft := BriefDraft{Status: "open", Version: 2, Document: EmptyBriefDocument()}
	_, err := ApplyBriefPatch(draft, BriefPatch{ExpectedVersion: 1, Operations: []BriefPatchOperation{{Op: "set", FieldPath: "campaign.objective", Value: json.RawMessage(`"x"`)}}}, PatchFromUser, "user_1", time.Now())
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("error = %v, want version conflict", err)
	}
	_, err = ApplyBriefPatch(draft, BriefPatch{ExpectedVersion: 2, Operations: []BriefPatchOperation{{Op: "set", FieldPath: "organization_id", Value: json.RawMessage(`"other"`)}}}, PatchFromUser, "user_1", time.Now())
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("error = %v, want invalid request", err)
	}
}

func TestDeterministicBriefPatchSeparatesChineseLabeledFields(t *testing.T) {
	t.Parallel()
	draft := BriefDraft{Version: 1, Document: EmptyBriefDocument()}
	patch := deterministicBriefPatch(draft, Message{
		ID: "msg_1", Content: "目标：新品认知；受众：研发负责人；卖点：缩短研发周期",
	})
	values := make(map[string]string)
	for _, operation := range patch.Operations {
		if operation.FieldPath == "channels" {
			continue
		}
		var value string
		if err := json.Unmarshal(operation.Value, &value); err != nil {
			t.Fatal(err)
		}
		values[operation.FieldPath] = value
	}
	if values["campaign.objective"] != "新品认知" ||
		values["audience.primary"] != "研发负责人" ||
		values["proposition"] != "缩短研发周期" {
		t.Fatalf("extracted fields = %#v", values)
	}
}
