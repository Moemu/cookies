package strategy

import (
	"strings"
	"testing"
	"time"
)

func TestProjectContextManifestRejectsUnboundedOrInvalidContext(t *testing.T) {
	hash, err := contextContentHash(strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	manifest := ProjectContextManifest{
		ContractVersion:    ProjectContextManifestContractV1,
		WorkspaceRef:       VersionedContextRef{Type: "strategy_workspace", ID: "workspace_1", Version: 2},
		ProjectContextRef:  VersionedContextRef{Type: "project_context", ID: "project_1", Version: 7},
		Stage:              "brief",
		BriefRef:           &SnapshotContextRef{Type: "brief_draft", ID: "briefdraft_1", Version: 4, ContentHash: hash},
		SelectedSourceRefs: []SnapshotContextRef{}, MemoryVersion: 3, GeneratedAt: time.Now().UTC(),
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("valid manifest: %v", err)
	}
	manifest.Stage = "hidden_reasoning"
	if err := manifest.Validate(); err == nil {
		t.Fatal("invalid stage was accepted")
	}
	manifest.Stage = "brief"
	manifest.SelectedSourceRefs = make([]SnapshotContextRef, 33)
	if err := manifest.Validate(); err == nil {
		t.Fatal("unbounded selected sources were accepted")
	}
}

func TestContextContentHashAcceptsRawOrPrefixedSHA256(t *testing.T) {
	raw := strings.Repeat("b", 64)
	first, err := contextContentHash(raw)
	if err != nil {
		t.Fatal(err)
	}
	second, err := contextContentHash("sha256:" + raw)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || string(first) != "sha256:"+raw {
		t.Fatalf("normalized hashes = %q, %q", first, second)
	}
}

func TestContextSourceExclusionAffectsOnlyTheCopiedManifest(t *testing.T) {
	hash, err := contextContentHash(strings.Repeat("c", 64))
	if err != nil {
		t.Fatal(err)
	}
	original := []SnapshotContextRef{
		{Type: "knowledge_document", ID: "document_1", ContentHash: hash},
		{Type: "knowledge_research_artifact", ID: "research_1", ContentHash: hash},
	}
	filtered := contextSourcesWithout(original, []string{"document_1", "unknown_stale_source"})
	if len(filtered) != 1 || filtered[0].ID != "research_1" {
		t.Fatalf("filtered refs=%#v", filtered)
	}
	if len(original) != 2 || original[0].ID != "document_1" {
		t.Fatalf("source exclusion mutated authoritative refs=%#v", original)
	}
}
