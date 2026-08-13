package strategy

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestBriefDraftProposalContentHashChangesWithFactsAndConfirmation(t *testing.T) {
	t.Parallel()
	draft := BriefDraft{
		ID: "draft_1", Status: "open", Version: 1,
		Document: EmptyBriefDocumentV2(), FieldStates: map[string]FieldState{},
	}
	first, err := briefDraftProposalContentHash(draft)
	if err != nil {
		t.Fatal(err)
	}
	draft.Document.Brand.Name = "轻氧"
	second, err := briefDraftProposalContentHash(draft)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("document change must invalidate a proposal")
	}
	draft.FieldStates["brand.name"] = FieldState{
		FieldPath: "brand.name", Confirmation: "confirmed", UpdatedAt: time.Now().UTC(),
	}
	third, err := briefDraftProposalContentHash(draft)
	if err != nil {
		t.Fatal(err)
	}
	if second == third {
		t.Fatal("field-state change must invalidate a proposal")
	}
}

func TestValidateEditedProposalOperationsCannotExpandScope(t *testing.T) {
	t.Parallel()
	original := []BriefPatchOperation{
		{Op: "set", FieldPath: "brand.name", Value: json.RawMessage(`"Old"`)},
		{Op: "set", FieldPath: "channels", Value: json.RawMessage(`["xiaohongshu"]`)},
	}
	edited := []BriefPatchOperation{
		{Op: "set", FieldPath: "brand.name", Value: json.RawMessage(`"New"`)},
		{Op: "set", FieldPath: "channels", Value: json.RawMessage(`["douyin"]`)},
	}
	if err := validateEditedProposalOperations(original, edited); err != nil {
		t.Fatalf("same-scope edit: %v", err)
	}
	edited[1].FieldPath = "budget.total"
	if err := validateEditedProposalOperations(original, edited); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expanded scope error=%v", err)
	}
}
