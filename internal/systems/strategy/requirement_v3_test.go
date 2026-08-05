package strategy

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestBriefV3RoundTripKeepsCanonicalShapeAndLegacyProjection(t *testing.T) {
	t.Parallel()
	document := EmptyBriefDocumentV3()
	document.Core = BriefCoreV3{
		Objective:         "新品认知",
		DeliverableIntent: "viral_remake",
		ProductOrSubject:  "产品 A",
		Audience:          "敏感肌用户",
	}
	document.Facts = []BriefFactV3{
		{ID: "fact_brand", Kind: "brand", Value: json.RawMessage(`"山海"`), SourceRefs: []FieldSource{{Type: "document", ID: "doc_1", Locator: "p.2"}}, Confidence: "high"},
		{ID: "fact_channels", Kind: "channel", Value: json.RawMessage(`["douyin","xiaohongshu"]`), SourceRefs: []FieldSource{}, Confidence: "medium"},
	}
	document.AssetRefs = []contract.AssetVersionRef{{AssetID: "asset_1", Version: 2}}

	payload, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal v3: %v", err)
	}
	for _, legacyField := range []string{`"campaign"`, `"product"`, `"platform_briefs"`} {
		if strings.Contains(string(payload), legacyField) {
			t.Fatalf("v3 payload leaked legacy field %s: %s", legacyField, payload)
		}
	}

	var restored BriefDocument
	if err := json.Unmarshal(payload, &restored); err != nil {
		t.Fatalf("unmarshal v3: %v", err)
	}
	if restored.ContractVersion != BriefContractVersionV3 || restored.Campaign.Objective != "新品认知" || restored.Product.Name != "产品 A" || restored.Audience.Primary != "敏感肌用户" {
		t.Fatalf("core projection=%#v", restored)
	}
	if restored.Brand.Name != "山海" || len(restored.Channels) != 2 || restored.AssetRefs[0].Version != 2 {
		t.Fatalf("fact projection=%#v", restored)
	}
}

func TestBriefV3CompletenessOnlyBlocksCreativeCriticalUnknowns(t *testing.T) {
	t.Parallel()
	document := EmptyBriefDocumentV3()
	document.Core = BriefCoreV3{Objective: "新品认知", DeliverableIntent: "viral_remake", ProductOrSubject: "产品 A", Audience: "敏感肌用户"}
	document.Assumptions = []BriefAssumptionV3{{ID: "assumption_1", Statement: "用户偏好真实测评", SourceRefs: []FieldSource{}}}
	document.Unknowns = []BriefUnknownV3{{ID: "unknown_production", Question: "最终字幕字号？", RequiredFor: "production"}}
	now := time.Now()
	states := map[string]FieldState{}
	for _, path := range []string{"core.objective", "core.deliverable_intent", "core.product_or_subject", "core.audience"} {
		states[path] = FieldState{FieldPath: path, Confirmation: "confirmed", UpdatedAt: now}
	}
	completeness := ComputeCompleteness(document, states)
	if !completeness.Ready || len(completeness.Blockers) != 0 || len(completeness.Warnings) != 1 {
		t.Fatalf("unexpected completeness: %#v", completeness)
	}

	document.Unknowns = append(document.Unknowns, BriefUnknownV3{ID: "unknown_intake", Question: "需要改编哪条参考视频？", RequiredFor: "creative_intake"})
	completeness = ComputeCompleteness(document, states)
	if completeness.Ready || len(completeness.Blockers) != 1 || completeness.Blockers[0].Field != "unknowns.unknown_intake" {
		t.Fatalf("critical unknown did not block intake: %#v", completeness)
	}
}

func TestBriefV3RejectsDuplicateSemanticItemIDs(t *testing.T) {
	t.Parallel()
	document := EmptyBriefDocumentV3()
	document.Facts = []BriefFactV3{{ID: "same", Kind: "brand", Value: json.RawMessage(`"山海"`), SourceRefs: []FieldSource{}, Confidence: "high"}}
	document.Unknowns = []BriefUnknownV3{{ID: "same", Question: "目标平台？", RequiredFor: "creative_intake"}}
	if _, err := json.Marshal(document); err == nil {
		t.Fatal("expected duplicate semantic ids to be rejected")
	}
}
