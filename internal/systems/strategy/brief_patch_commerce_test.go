package strategy

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestApplyBriefPatchPersistsCommerceCreativeFacts(t *testing.T) {
	t.Parallel()
	draft := BriefDraft{
		Status: "open", Version: 1, Document: EmptyBriefDocumentV2(),
		FieldStates: map[string]FieldState{},
	}
	operations := []BriefPatchOperation{
		{Op: "set", FieldPath: "product.category", Value: json.RawMessage(`"skincare"`)},
		{Op: "set", FieldPath: "product.selling_points", Value: json.RawMessage(`["hydration","radiance"]`)},
		{Op: "set", FieldPath: "product.evidence", Value: json.RawMessage(`["approved brief claim"]`)},
		{Op: "set", FieldPath: "product.asset_refs", Value: json.RawMessage(`[{"asset_id":"asset_product","version":2}]`)},
		{Op: "set", FieldPath: "creative.tone", Value: json.RawMessage(`["premium","warm"]`)},
		{Op: "set", FieldPath: "creative.mandatory_elements", Value: json.RawMessage(`["front label"]`)},
		{Op: "set", FieldPath: "creative.prohibited_claims", Value: json.RawMessage(`["medical claim"]`)},
	}
	updated, err := ApplyBriefPatch(draft, BriefPatch{
		ExpectedVersion: 1,
		Operations:      operations,
	}, PatchFromUser, "user_1", time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ApplyBriefPatch() error = %v", err)
	}
	if updated.Document.Product.Category != "skincare" ||
		len(updated.Document.Product.SellingPoints) != 2 ||
		len(updated.Document.Product.Evidence) != 1 ||
		len(updated.Document.Product.AssetRefs) != 1 ||
		updated.Document.Product.AssetRefs[0] != (contract.AssetVersionRef{AssetID: "asset_product", Version: 2}) ||
		len(updated.Document.Creative.Tone) != 2 ||
		len(updated.Document.Creative.MandatoryElements) != 1 ||
		len(updated.Document.Creative.ProhibitedClaims) != 1 {
		t.Fatalf("commerce facts were not persisted: %+v", updated.Document)
	}
	for _, operation := range operations {
		if updated.FieldStates[operation.FieldPath].Confirmation != "confirmed" {
			t.Fatalf("%s was not user-confirmed", operation.FieldPath)
		}
	}
}
