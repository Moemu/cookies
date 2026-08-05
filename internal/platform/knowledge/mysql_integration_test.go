package knowledge_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
	"github.com/shikanon/cookies/internal/platform/knowledge"
	"github.com/shikanon/cookies/internal/platform/project"
)

func TestKnowledgeCenterMySQLProjection(t *testing.T) {
	dsn := os.Getenv("COOKIES_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("COOKIES_TEST_MYSQL_DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	organizationID := contract.OrganizationID("org_knowledge_it_" + suffix)
	projectID := contract.ProjectID("project_knowledge_it_" + suffix)
	userID := "user_knowledge_it_" + suffix
	actor := contract.ActorContext{
		OrganizationID: organizationID,
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: userID},
		Scopes:         []contract.Scope{"project.read", "project.write", "assets.read", "assets.write"},
	}
	t.Cleanup(func() { cleanupKnowledgeIntegration(t, db, organizationID, userID) })
	identityStore := identity.MySQLStore{DB: db}
	if err := identityStore.EnsureLocalActor(ctx, actor); err != nil {
		t.Fatal(err)
	}
	projectStore := project.MySQLStore{DB: db}
	if err := projectStore.EnsureLocalProject(ctx, actor, projectID); err != nil {
		t.Fatal(err)
	}
	service := knowledge.Service{
		DB: db, Projects: &project.Service{Store: projectStore, Authorizer: projectStore},
		Blobs: assets.NewMemoryBlobStore(), Scanner: assets.NoopScanner{},
		AssetsBucket: "knowledge-integration", Runner: researchRunner{},
	}

	document, err := service.ImportDocument(ctx, actor, projectID, knowledge.ImportDocumentRequest{
		Title: "投前洞察证据", SourceURI: "cookies://prelaunch-insights/PRE-001",
		SourceType: "prelaunch_insight", Text: "历史项目验证了当前主张。",
	})
	if err != nil {
		t.Fatal(err)
	}
	documents, err := service.ListDocuments(ctx, actor, projectID, 10)
	if err != nil || len(documents) != 1 ||
		documents[0].Title != document.Title ||
		documents[0].SourceURI != document.SourceURI ||
		documents[0].SourceType != "prelaunch_insight" {
		t.Fatalf("persisted knowledge metadata=%#v err=%v", documents, err)
	}
	secondDocument, err := service.ImportDocument(ctx, actor, projectID, knowledge.ImportDocumentRequest{
		Title: "品牌边界", SourceURI: "cookies://brand-guardrails/BG-001",
		SourceType: "docs", Text: "品牌表达必须克制，不使用治疗承诺。",
	})
	if err != nil {
		t.Fatal(err)
	}
	conversationChunks, err := service.SelectConversationChunks(
		ctx, actor, projectID, []string{document.ID, secondDocument.ID}, "历史项目验证",
	)
	if err != nil {
		t.Fatalf("SelectConversationChunks() error=%v", err)
	}
	coveredDocuments := map[string]bool{}
	for _, result := range conversationChunks {
		coveredDocuments[result.Chunk.DocumentID] = true
	}
	if !coveredDocuments[document.ID] || !coveredDocuments[secondDocument.ID] {
		t.Fatalf("conversation chunks did not cover every ready attachment: %#v", coveredDocuments)
	}

	run, err := service.RunResearch(ctx, actor, projectID, knowledge.ResearchRequest{
		Mode: "web", Category: "audience", Query: "研发负责人决策因素",
		DocumentIDs: []string{document.ID}, DisclosedFields: []string{"query", "document_content"},
		Confirmed: true,
	})
	if err != nil || run.Status != "succeeded" || run.Category != "audience" ||
		len(run.Artifacts) != 1 || len(run.DisclosedChunkIDs) != 1 ||
		run.DisclosedChunkIDs[0] == document.ID {
		t.Fatalf("research run=%#v err=%v", run, err)
	}
	artifacts, err := service.ListResearchArtifacts(ctx, actor, projectID, "audience", 10)
	if err != nil || len(artifacts) != 1 || artifacts[0].Category != "audience" {
		t.Fatalf("research artifacts=%#v err=%v", artifacts, err)
	}
	if _, err := service.ListResearchArtifacts(ctx, actor, projectID, "creative", 10); !errors.Is(err, knowledge.ErrInvalidResearchRequest) {
		t.Fatalf("invalid category error=%v", err)
	}

	service.DocumentParser = parserStub{}
	service.DocumentScheduler = parseSchedulerStub{}
	pdfBytes := []byte("%PDF-1.7 fake integration payload")
	pdf, err := service.CreateDocument(
		ctx, actor, projectID, "market-report.pdf", "application/pdf",
		bytes.NewReader(pdfBytes), int64(len(pdfBytes)),
	)
	if err != nil || pdf.Status != "parse_queued" {
		t.Fatalf("queued PDF=%#v err=%v", pdf, err)
	}
	payload, _ := json.Marshal(map[string]string{"document_id": pdf.ID})
	if _, err := service.HandleDocumentParseJob(ctx, jobruntime.Claim{
		Job: contract.Job{
			Kind:           knowledge.DocumentParseJobKind,
			OrganizationID: actor.OrganizationID, ProjectID: projectID,
		},
		Payload: payload,
	}); err != nil {
		t.Fatalf("HandleDocumentParseJob() error=%v", err)
	}
	parsedPDF, err := service.GetDocument(ctx, actor, projectID, pdf.ID)
	if err != nil || parsedPDF.Status != "ready" || parsedPDF.ParserCode != "tika" ||
		parsedPDF.ChunkCount < 1 {
		t.Fatalf("parsed PDF=%#v err=%v", parsedPDF, err)
	}
}

type researchRunner struct{}

type parserStub struct{}

func (parserStub) Parse(context.Context, knowledge.DocumentParseRequest) (knowledge.ParsedDocument, error) {
	return knowledge.ParsedDocument{
		Text:     "市场规模持续增长。\n\n品牌事实需要逐项验证。",
		MIMEType: "application/pdf", ParserCode: "tika", ParserVersion: "test",
		Metadata: json.RawMessage(`{"Content-Type":"application/pdf"}`),
	}, nil
}

type parseSchedulerStub struct{}

func (parseSchedulerStub) ScheduleDocumentParse(context.Context, knowledge.Document) error {
	return nil
}

func (researchRunner) Run(context.Context, knowledge.ExternalResearchInput) ([]knowledge.ExternalResearchResult, error) {
	return []knowledge.ExternalResearchResult{{
		Title:     "研发负责人重视可验证证据",
		Content:   "研究样本显示，明确的精度与交付证据优先于泛化品牌表达。",
		Citations: []string{"https://example.com/research"},
	}}, nil
}

func cleanupKnowledgeIntegration(t *testing.T, db *sql.DB, organizationID contract.OrganizationID, userID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	statements := []string{
		"DELETE FROM platform_research_citations WHERE organization_id=?",
		"DELETE FROM platform_research_sources WHERE organization_id=?",
		"DELETE FROM platform_research_artifacts WHERE organization_id=?",
		"DELETE FROM platform_research_runs WHERE organization_id=?",
		"DELETE FROM platform_knowledge_chunks WHERE organization_id=?",
		"DELETE FROM platform_knowledge_documents WHERE organization_id=?",
		"DELETE FROM project_context_versions WHERE organization_id=?",
		"DELETE FROM project_products WHERE organization_id=?",
		"DELETE FROM project_memberships WHERE organization_id=?",
		"DELETE FROM platform_project_runtimes WHERE organization_id=?",
		"DELETE FROM projects WHERE organization_id=?",
		"DELETE FROM brand_guideline_versions WHERE organization_id=?",
		"DELETE FROM products WHERE organization_id=?",
		"DELETE FROM brands WHERE organization_id=?",
		"DELETE FROM organization_memberships WHERE organization_id=?",
		"DELETE FROM organizations WHERE id=?",
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement, organizationID); err != nil {
			t.Errorf("cleanup %q: %v", statement, err)
		}
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM users WHERE id=?", userID); err != nil {
		t.Errorf("cleanup user: %v", err)
	}
}
