package knowledge

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestServiceImportsAndSearchesProjectKnowledgeWithCitations(t *testing.T) {
	t.Parallel()
	nextID := 0
	service := NewMemoryService(func(prefix string) (string, error) {
		nextID++
		return prefix + "_" + string(rune('0'+nextID)), nil
	})
	service.nowUTC = func() time.Time { return time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC) }
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}

	document, err := service.ImportDocument(context.Background(), actor, "project_1", ImportDocumentRequest{
		Title:      "电商前贴策略",
		SourceURI:  "docs/策略/06-电商广告前贴与钩子视频生成策略.md",
		SourceType: "strategy",
		Text:       "# Hook\n首屏需要商品露出和强钩子。\n\n# Quality\n质检必须保留 citation。",
	})
	if err != nil {
		t.Fatalf("ImportDocument() error = %v", err)
	}
	if document.ID != "knowledgedoc_1" || document.ChunkCount != 2 || document.SourceType != "strategy" {
		t.Fatalf("document = %#v", document)
	}

	results, err := service.Search(context.Background(), actor, "project_1", SearchRequest{Query: "商品 露出 citation", Limit: 5})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results length = %d, want 2: %#v", len(results), results)
	}
	if results[0].Score < results[1].Score {
		t.Fatalf("results are not sorted by score: %#v", results)
	}
	citation := results[0].Citations[0]
	if citation.DocumentID != document.ID || citation.ChunkID == "" || citation.Title != "电商前贴策略" {
		t.Fatalf("citation = %#v", citation)
	}
	if citation.SourceURI != "docs/策略/06-电商广告前贴与钩子视频生成策略.md" || citation.StartLine < 1 || citation.EndLine < citation.StartLine {
		t.Fatalf("citation source range = %#v", citation)
	}
	if citation.Snippet == "" {
		t.Fatal("citation snippet is required")
	}
}

func TestServiceKeepsKnowledgeScopedToProjectAndOrganization(t *testing.T) {
	t.Parallel()
	nextID := 0
	service := NewMemoryService(func(prefix string) (string, error) {
		nextID++
		return prefix + "_" + string(rune('a'+nextID)), nil
	})
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}
	otherActor := contract.ActorContext{OrganizationID: "org_2", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_2"}}
	_, err := service.ImportDocument(context.Background(), actor, "project_1", ImportDocumentRequest{Title: "项目一策略", Text: "钩子 素材 商品"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ImportDocument(context.Background(), actor, "project_2", ImportDocumentRequest{Title: "项目二策略", Text: "钩子 素材 商品"})
	if err != nil {
		t.Fatal(err)
	}

	documents, err := service.ListDocuments(context.Background(), actor, "project_1", 10)
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if len(documents) != 1 || documents[0].ProjectID != "project_1" {
		t.Fatalf("documents = %#v", documents)
	}
	projectResults, err := service.Search(context.Background(), actor, "project_1", SearchRequest{Query: "钩子", Limit: 10})
	if err != nil {
		t.Fatalf("Search(project_1) error = %v", err)
	}
	if len(projectResults) != 1 || projectResults[0].Chunk.ProjectID != "project_1" {
		t.Fatalf("project results = %#v", projectResults)
	}
	orgResults, err := service.Search(context.Background(), otherActor, "project_1", SearchRequest{Query: "钩子", Limit: 10})
	if err != nil {
		t.Fatalf("Search(other org) error = %v", err)
	}
	if len(orgResults) != 0 {
		t.Fatalf("cross-org results = %#v", orgResults)
	}
}
