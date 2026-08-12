package strategy

import (
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestNormalizeMessageV2BuildsLegacyProjectionAndCanonicalBlocks(t *testing.T) {
	t.Parallel()
	hash := strings.Repeat("a", 64)
	result, err := normalizeMessageV2(SendMessageV2Request{
		ContractVersion: MessageCreateContractV2,
		Content: []MessageContentBlock{
			{Type: "text", Text: "  参考附件做一条新品短片  "},
			{Type: "document_ref", DocumentID: " doc_01 ", ExpectedContentSHA256: hash},
			{Type: "asset_ref", AssetKind: "image", AssetID: contract.AssetID(" asset_01 "), AssetVersion: 2},
			{Type: "asset_ref", AssetKind: "video", AssetID: contract.AssetID("asset_02"), AssetVersion: 1},
			{Type: "research_ref", ResearchArtifactID: " research_01 ", ExpectedContentHash: hash},
		},
		RequestedPolicy: &MessageRequestedPolicy{ReasoningMode: "deep", WebSearch: "allowed", MCPServerIDs: []string{}},
	})
	if err != nil {
		t.Fatalf("normalize message: %v", err)
	}
	wantProjection := "参考附件做一条新品短片\n[文档 doc_01]\n[图片 asset_01#2]\n[视频 asset_02#1]\n[联网证据 research_01]"
	if result.Projection != wantProjection {
		t.Fatalf("projection=%q want=%q", result.Projection, wantProjection)
	}
	if result.ContentBlocks[0].Text != "参考附件做一条新品短片" || result.ContentBlocks[1].DocumentID != "doc_01" || result.ContentBlocks[2].AssetID != "asset_01" {
		t.Fatalf("blocks were not normalized: %#v", result.ContentBlocks)
	}
	if result.RequestedPolicy == nil || result.RequestedPolicy.ReasoningMode != "deep" || result.RequestedPolicy.WebSearch != "allowed" {
		t.Fatalf("policy=%#v", result.RequestedPolicy)
	}
}

func TestNormalizeMessageV2RejectsUnsupportedOrAmbiguousInput(t *testing.T) {
	t.Parallel()
	hash := strings.Repeat("a", 64)
	tests := map[string]SendMessageV2Request{
		"missing contract": {
			Content: []MessageContentBlock{{Type: "text", Text: "hello"}},
		},
		"uppercase content hash": {
			ContractVersion: MessageCreateContractV2,
			Content:         []MessageContentBlock{{Type: "document_ref", DocumentID: "doc_01", ExpectedContentSHA256: strings.Repeat("A", 64)}},
		},
		"duplicate immutable ref": {
			ContractVersion: MessageCreateContractV2,
			Content: []MessageContentBlock{
				{Type: "document_ref", DocumentID: "doc_01", ExpectedContentSHA256: hash},
				{Type: "document_ref", DocumentID: "doc_01", ExpectedContentSHA256: hash},
			},
		},
		"mixed block payload": {
			ContractVersion: MessageCreateContractV2,
			Content:         []MessageContentBlock{{Type: "text", Text: "hello", DocumentID: "doc_01"}},
		},
		"unsupported asset kind": {
			ContractVersion: MessageCreateContractV2,
			Content:         []MessageContentBlock{{Type: "asset_ref", AssetKind: "audio", AssetID: "asset_01", AssetVersion: 1}},
		},
		"research evidence without visible search policy": {
			ContractVersion: MessageCreateContractV2,
			Content:         []MessageContentBlock{{Type: "research_ref", ResearchArtifactID: "research_01", ExpectedContentHash: hash}},
		},
		"mixed research payload": {
			ContractVersion: MessageCreateContractV2,
			Content:         []MessageContentBlock{{Type: "research_ref", ResearchArtifactID: "research_01", ExpectedContentHash: hash, Text: "hidden"}},
			RequestedPolicy: &MessageRequestedPolicy{WebSearch: "allowed"},
		},
		"remote mcp before security review": {
			ContractVersion: MessageCreateContractV2,
			Content:         []MessageContentBlock{{Type: "text", Text: "hello"}},
			RequestedPolicy: &MessageRequestedPolicy{MCPServerIDs: []string{"unreviewed-server"}},
		},
		"unsupported policy": {
			ContractVersion: MessageCreateContractV2,
			Content:         []MessageContentBlock{{Type: "text", Text: "hello"}},
			RequestedPolicy: &MessageRequestedPolicy{ReasoningMode: "always_max"},
		},
	}
	for name, request := range tests {
		request := request
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := normalizeMessageV2(request); err == nil {
				t.Fatal("expected invalid request")
			}
		})
	}
}

func TestNormalizeMessageV2AllowsBackgroundSearchIntentBeforeEvidence(t *testing.T) {
	t.Parallel()
	result, err := normalizeMessageV2(SendMessageV2Request{
		ContractVersion: MessageCreateContractV2,
		Content:         []MessageContentBlock{{Type: "text", Text: "please verify"}},
		RequestedPolicy: &MessageRequestedPolicy{WebSearch: "allowed"},
	})
	if err != nil {
		t.Fatalf("normalize background search message: %v", err)
	}
	if result.RequestedPolicy == nil || result.RequestedPolicy.WebSearch != "allowed" {
		t.Fatalf("policy=%#v", result.RequestedPolicy)
	}
}

func TestNormalizeMessageV2BindsAssistantSurfaceOutsideFrozenBody(t *testing.T) {
	t.Parallel()
	result, err := normalizeMessageV2(SendMessageV2Request{
		ContractVersion:   MessageCreateContractV2,
		Content:           []MessageContentBlock{{Type: "text", Text: "品牌：轻氧"}},
		ContextStage:      "strategy",
		ContextSurface:    "assistant",
		ExcludedSourceIDs: []string{"source_b", "source_a"},
	})
	if err != nil {
		t.Fatalf("normalize assistant message: %v", err)
	}
	if result.ContextStage != "strategy" || result.ContextSurface != "assistant" {
		t.Fatalf("context=%s/%s", result.ContextStage, result.ContextSurface)
	}
	if len(result.ExcludedSourceIDs) != 2 || result.ExcludedSourceIDs[0] != "source_a" || result.ExcludedSourceIDs[1] != "source_b" {
		t.Fatalf("excluded source IDs=%#v", result.ExcludedSourceIDs)
	}
	if _, err := normalizeMessageV2(SendMessageV2Request{
		ContractVersion: MessageCreateContractV2,
		Content:         []MessageContentBlock{{Type: "text", Text: "hello"}},
		ContextSurface:  "floating-widget",
	}); err == nil {
		t.Fatal("unsupported context surface must be rejected")
	}
	if _, err := normalizeMessageV2(SendMessageV2Request{
		ContractVersion:   MessageCreateContractV2,
		Content:           []MessageContentBlock{{Type: "text", Text: "hello"}},
		ContextSurface:    "workspace",
		ExcludedSourceIDs: []string{"source_1"},
	}); err == nil {
		t.Fatal("direct workspace message accepted Assistant-only source exclusions")
	}
	if _, err := normalizeMessageV2(SendMessageV2Request{
		ContractVersion:   MessageCreateContractV2,
		Content:           []MessageContentBlock{{Type: "text", Text: "hello"}},
		ContextSurface:    "assistant",
		ExcludedSourceIDs: []string{"source_1", "source_1"},
	}); err == nil {
		t.Fatal("duplicate source exclusion was accepted")
	}
}
