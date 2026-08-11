package knowledge_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
	"github.com/shikanon/cookies/internal/platform/knowledge"
	"github.com/shikanon/cookies/internal/platform/project"
	systemsstrategy "github.com/shikanon/cookies/internal/systems/strategy"
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
	integrationQueueNow := time.Now().UTC().Add(24 * time.Hour)
	organizationID := contract.OrganizationID("org_knowledge_it_" + suffix)
	projectID := contract.ProjectID("project_knowledge_it_" + suffix)
	userID := "user_knowledge_it_" + suffix
	secondUserID := "user_knowledge_reviewer_it_" + suffix
	actor := contract.ActorContext{
		OrganizationID: organizationID,
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: userID},
		Scopes:         []contract.Scope{"project.read", "project.write", "assets.read", "assets.write", systemsstrategy.ScopeRead},
	}
	t.Cleanup(func() { cleanupKnowledgeIntegration(t, db, organizationID, userID, secondUserID) })
	identityStore := identity.MySQLStore{DB: db}
	if err := identityStore.EnsureLocalActor(ctx, actor); err != nil {
		t.Fatal(err)
	}
	projectStore := project.MySQLStore{DB: db}
	if err := projectStore.EnsureLocalProject(ctx, actor, projectID); err != nil {
		t.Fatal(err)
	}
	projectService := &project.Service{Store: projectStore, Authorizer: projectStore}
	service := knowledge.Service{
		DB: db, Projects: projectService,
		Blobs: assets.NewMemoryBlobStore(), Scanner: assets.NoopScanner{},
		AssetsBucket: "knowledge-integration", Runner: researchRunner{},
		SourceVerifier: researchVerifier{},
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
		Mode: "web", RunMode: "quick", Category: "audience", Purpose: "conversation_web_search",
		SourceRef: &contract.ResourceRef{Type: "strategy_message", ID: "message_" + suffix}, Query: "研发负责人决策因素",
		DocumentIDs: []string{document.ID}, DisclosedFields: []string{"query", "document_content"},
		Confirmed: true,
	})
	if err != nil || run.Status != "completed" || run.Category != "audience" ||
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

	deepRun, err := service.RunResearch(ctx, actor, projectID, knowledge.ResearchRequest{
		Mode: "web", RunMode: "deep", Category: "audience", Purpose: "deep_research",
		SourceRef:        &contract.ResourceRef{Type: "strategy_workspace", ID: "workspace_deep_" + suffix},
		InputSnapshotRef: "strategy_workspace:workspace_deep_" + suffix + ":v1",
		InputSnapshot:    json.RawMessage(`{"contract_version":"strategy-project-context-manifest/v1","workspace_ref":{"id":"workspace_deep"}}`),
		Query:            "研发负责人最看重哪类决策证据", DocumentIDs: []string{},
		DisclosedFields: []string{"query"}, Confirmed: true,
	})
	if err != nil || deepRun.Status != "completed" || deepRun.CurrentRound != 1 ||
		len(deepRun.Iterations) != 1 || len(deepRun.Findings) != 1 ||
		deepRun.Findings[0].Status != "verified" || deepRun.ReportArtifactID == nil {
		t.Fatalf("deep research run=%#v err=%v", deepRun, err)
	}
	report, err := service.GetResearchReport(ctx, actor, projectID, deepRun.ID)
	if err != nil || report.ID != *deepRun.ReportArtifactID || len(report.Sources) != 2 {
		t.Fatalf("deep research report=%#v err=%v", report, err)
	}
	for _, source := range report.Sources {
		if source.VerificationStatus != "content_verified" || source.SupportLevel != "content_verified" {
			t.Fatalf("verified report source remained model-only: %#v", source)
		}
	}

	retryRunner := &failFirstResearchRunner{}
	service.Runner = retryRunner
	service.Scheduler = researchRetrySchedulerStub{}
	retryRun, err := service.RunResearch(ctx, actor, projectID, knowledge.ResearchRequest{
		Mode: "web", RunMode: "deep", Category: "industry", Purpose: "deep_research",
		SourceRef:        &contract.ResourceRef{Type: "strategy_workspace", ID: "workspace_retry_" + suffix},
		InputSnapshotRef: "strategy_workspace:workspace_retry_" + suffix + ":v1",
		InputSnapshot:    json.RawMessage(`{"contract_version":"strategy-project-context-manifest/v1"}`),
		Query:            "失败轮次恢复", DocumentIDs: []string{}, DisclosedFields: []string{"query"}, Confirmed: true,
	})
	if err != nil || retryRun.Status != "queued" {
		t.Fatalf("retry fixture run=%#v err=%v", retryRun, err)
	}
	claim := jobruntime.Claim{Job: contract.Job{
		Kind: knowledge.ResearchJobKind, OrganizationID: organizationID, ProjectID: projectID,
		AttemptCount: 1, MaxAttempts: 2,
	}, Payload: mustJSON(t, map[string]string{"research_run_id": retryRun.ID})}
	if _, err := service.HandleResearchJob(ctx, claim); err == nil {
		t.Fatal("first failed provider attempt was not deferred")
	} else {
		var deferred jobruntime.DeferredError
		if !errors.As(err, &deferred) || !deferred.AvailableAt.After(time.Now().UTC()) {
			t.Fatalf("first provider failure was not retryable: %v", err)
		}
	}
	retryRun, err = service.GetResearchRun(ctx, actor, projectID, retryRun.ID)
	if err != nil || retryRun.Status != "planning" || retryRun.StopReason != "provider_retry" ||
		len(retryRun.Iterations) != 1 || retryRun.Iterations[0].Status != "failed" {
		t.Fatalf("failed checkpoint=%#v err=%v", retryRun, err)
	}
	claim.Job.AttemptCount = 2
	if _, err := service.HandleResearchJob(ctx, claim); err != nil {
		t.Fatalf("resumed research attempt: %v", err)
	}
	retryRun, err = service.GetResearchRun(ctx, actor, projectID, retryRun.ID)
	if err != nil || retryRun.Status != "completed" || retryRun.CurrentRound != 1 ||
		len(retryRun.Iterations) != 1 || retryRun.Iterations[0].Status != "completed" || retryRunner.calls != 2 {
		t.Fatalf("resumed checkpoint duplicated or failed: run=%#v calls=%d err=%v", retryRun, retryRunner.calls, err)
	}
	service.Runner = researchRunner{}
	service.Scheduler = nil

	checkpointRunner := &failAfterThirdRoundResearchRunner{}
	service.Runner = checkpointRunner
	service.Scheduler = researchRetrySchedulerStub{}
	checkpointRun, err := service.RunResearch(ctx, actor, projectID, knowledge.ResearchRequest{
		Mode: "web", RunMode: "deep", Category: "competitor", Purpose: "deep_research",
		SourceRef:        &contract.ResourceRef{Type: "strategy_workspace", ID: "workspace_checkpoint_" + suffix},
		InputSnapshotRef: "strategy_workspace:workspace_checkpoint_" + suffix + ":v1",
		InputSnapshot:    json.RawMessage(`{"contract_version":"strategy-project-context-manifest/v1"}`),
		Query:            "resume after the third completed round", DocumentIDs: []string{},
		DisclosedFields: []string{"query"}, Confirmed: true,
	})
	if err != nil || checkpointRun.Status != "queued" {
		t.Fatalf("checkpoint fixture run=%#v err=%v", checkpointRun, err)
	}
	checkpointClaim := jobruntime.Claim{Job: contract.Job{
		Kind: knowledge.ResearchJobKind, OrganizationID: organizationID, ProjectID: projectID,
		AttemptCount: 1, MaxAttempts: 2,
	}, Payload: mustJSON(t, map[string]string{"research_run_id": checkpointRun.ID})}
	if _, err := service.HandleResearchJob(ctx, checkpointClaim); err == nil {
		t.Fatal("fourth-round provider failure was not deferred")
	} else {
		var deferred jobruntime.DeferredError
		if !errors.As(err, &deferred) {
			t.Fatalf("fourth-round provider failure was not retryable: %v", err)
		}
	}
	checkpointRun, err = service.GetResearchRun(ctx, actor, projectID, checkpointRun.ID)
	if err != nil || checkpointRun.Status != "planning" || checkpointRun.CurrentRound != 3 ||
		len(checkpointRun.Findings) != 3 || len(checkpointRun.Iterations) != 4 ||
		checkpointRun.Iterations[3].Round != 4 || checkpointRun.Iterations[3].Status != "failed" ||
		!reflect.DeepEqual(checkpointRunner.rounds, []int{1, 2, 3, 4}) {
		t.Fatalf("third-round checkpoint was not preserved: run=%#v rounds=%v err=%v", checkpointRun, checkpointRunner.rounds, err)
	}
	checkpointClaim.Job.AttemptCount = 2
	if _, err := service.HandleResearchJob(ctx, checkpointClaim); err != nil {
		t.Fatalf("resume from third-round checkpoint: %v", err)
	}
	checkpointRun, err = service.GetResearchRun(ctx, actor, projectID, checkpointRun.ID)
	if err != nil || checkpointRun.Status != "completed" || checkpointRun.CurrentRound != 4 ||
		len(checkpointRun.Iterations) != 4 || checkpointRun.Iterations[3].Status != "completed" ||
		!reflect.DeepEqual(checkpointRunner.rounds, []int{1, 2, 3, 4, 4}) {
		t.Fatalf("checkpoint replay repeated completed rounds: run=%#v rounds=%v err=%v", checkpointRun, checkpointRunner.rounds, err)
	}
	service.Runner = researchRunner{}
	service.Scheduler = nil

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
	if err != nil || parsedPDF.Status != "partial" || parsedPDF.ParserCode != "tika" ||
		parsedPDF.ChunkCount < 1 || parsedPDF.QualityTier != "low" || parsedPDF.PreviewStatus != "partial" {
		t.Fatalf("parsed PDF=%#v err=%v", parsedPDF, err)
	}
	preview, err := service.GetDocumentPreview(ctx, actor, projectID, pdf.ID)
	if err != nil || preview.DocumentID != pdf.ID || preview.Text == "" || len(preview.Chunks) < 1 || !preview.OriginalAvailable {
		t.Fatalf("document preview=%#v err=%v", preview, err)
	}
	stream, info, filename, err := service.OpenDocumentContent(ctx, actor, projectID, pdf.ID)
	if err != nil {
		t.Fatalf("OpenDocumentContent() error=%v", err)
	}
	original, readErr := io.ReadAll(stream)
	_ = stream.Close()
	if readErr != nil || filename != "market-report.pdf" || info.SizeBytes != int64(len(pdfBytes)) || !bytes.Equal(original, pdfBytes) {
		t.Fatalf("original filename=%q info=%#v bytes=%q readErr=%v", filename, info, original, readErr)
	}
	duplicatePDF, err := service.CreateDocument(
		ctx, actor, projectID, "market-report-copy.pdf", "application/pdf",
		bytes.NewReader(pdfBytes), int64(len(pdfBytes)),
	)
	if err != nil || duplicatePDF.ID != parsedPDF.ID || duplicatePDF.Status != "partial" {
		t.Fatalf("duplicate PDF did not reuse ready parse: duplicate=%#v parsed=%#v err=%v", duplicatePDF, parsedPDF, err)
	}
	service.DocumentVision = documentVisionParserStub{}
	service.VisionScheduler = visionSchedulerStub{}
	service.VisionModelAlias = "cookies.document.vision.standard"
	visionCapability, err := service.GetDocumentVisionFallbackCapability(ctx, actor, projectID, parsedPDF.ID)
	if err != nil || !visionCapability.Eligible || !visionCapability.Available || visionCapability.RequiresPageSelection {
		t.Fatalf("document vision capability=%#v err=%v", visionCapability, err)
	}
	queuedVision, err := service.RunDocumentVisionFallback(
		ctx, actor, projectID, parsedPDF.ID, knowledge.RunDocumentVisionFallbackRequest{},
	)
	if err != nil || queuedVision.VisionFallbackStatus != "queued" || !reflect.DeepEqual(queuedVision.VisionSelectedPages, []int{1, 2}) {
		t.Fatalf("queued visual fallback=%#v err=%v", queuedVision, err)
	}
	visionPayload, _ := json.Marshal(map[string]any{
		"document_id": parsedPDF.ID, "attempt_id": queuedVision.VisionAttemptID, "page_numbers": []int{1, 2},
	})
	for attempt := 1; attempt <= 3; attempt++ {
		_, handleErr := service.HandleDocumentVisionFallbackJob(ctx, jobruntime.Claim{
			Job: contract.Job{
				Kind: knowledge.DocumentVisionFallbackJobKind, OrganizationID: actor.OrganizationID,
				ProjectID: projectID, AttemptCount: attempt, MaxAttempts: 1000,
			},
			Payload: visionPayload,
		})
		var deferred jobruntime.DeferredError
		if attempt < 3 && !errors.As(handleErr, &deferred) {
			t.Fatalf("HandleDocumentVisionFallbackJob() attempt %d error=%v", attempt, handleErr)
		}
		if attempt == 3 && handleErr != nil {
			t.Fatalf("HandleDocumentVisionFallbackJob() final error=%v", handleErr)
		}
	}
	visionDocument, err := service.GetDocument(ctx, actor, projectID, parsedPDF.ID)
	if err != nil || visionDocument.Status != "ready" || visionDocument.ParseStrategy != "hybrid" ||
		visionDocument.VisionFallbackStatus != "succeeded" || len(visionDocument.VisionCompletedPages) != 2 ||
		visionDocument.ChunkCount <= parsedPDF.ChunkCount {
		t.Fatalf("visual document=%#v err=%v", visionDocument, err)
	}
	var persistedVisionPages int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM platform_knowledge_document_pages
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND status = 'ready'`,
		actor.OrganizationID, projectID, parsedPDF.ID).Scan(&persistedVisionPages); err != nil || persistedVisionPages != 2 {
		t.Fatalf("persisted visual pages=%d err=%v", persistedVisionPages, err)
	}
	nonContiguousBytes := []byte("%PDF-1.7 non-contiguous visual selection " + suffix)
	nonContiguousPDF, err := service.CreateDocument(
		ctx, actor, projectID, "non-contiguous-report.pdf", "application/pdf",
		bytes.NewReader(nonContiguousBytes), int64(len(nonContiguousBytes)),
	)
	if err != nil {
		t.Fatalf("create non-contiguous visual fixture: %v", err)
	}
	nonContiguousParsePayload, _ := json.Marshal(map[string]string{"document_id": nonContiguousPDF.ID})
	if _, err := service.HandleDocumentParseJob(ctx, jobruntime.Claim{
		Job:     contract.Job{Kind: knowledge.DocumentParseJobKind, OrganizationID: organizationID, ProjectID: projectID},
		Payload: nonContiguousParsePayload,
	}); err != nil {
		t.Fatalf("parse non-contiguous visual fixture: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE platform_knowledge_documents SET total_pages = 8
		WHERE organization_id = ? AND project_id = ? AND id = ?`, organizationID, projectID, nonContiguousPDF.ID); err != nil {
		t.Fatalf("expand non-contiguous fixture pages: %v", err)
	}
	nonContiguousPDF, err = service.GetDocument(ctx, actor, projectID, nonContiguousPDF.ID)
	if err != nil {
		t.Fatalf("reload non-contiguous fixture: %v", err)
	}
	nonContiguousPDF, err = service.RunDocumentVisionFallback(ctx, actor, projectID, nonContiguousPDF.ID,
		knowledge.RunDocumentVisionFallbackRequest{PageNumbers: []int{1, 3, 4, 8}})
	if err != nil {
		t.Fatalf("queue non-contiguous visual fallback: %v", err)
	}
	rows, err := db.QueryContext(ctx, `SELECT page_numbers FROM platform_knowledge_document_vision_tasks
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ? ORDER BY task_index`,
		organizationID, projectID, nonContiguousPDF.ID, nonContiguousPDF.VisionAttemptID)
	if err != nil {
		t.Fatalf("read split visual tasks: %v", err)
	}
	var splitTasks []string
	for rows.Next() {
		var encoded []byte
		if err := rows.Scan(&encoded); err != nil {
			t.Fatal(err)
		}
		splitTasks = append(splitTasks, string(encoded))
	}
	_ = rows.Close()
	if !reflect.DeepEqual(splitTasks, []string{"[1]", "[3, 4]", "[8]"}) &&
		!reflect.DeepEqual(splitTasks, []string{"[1]", "[3,4]", "[8]"}) {
		t.Fatalf("split external tasks = %#v", splitTasks)
	}
	service.DocumentVision = unknownSubmissionDocumentVisionParserStub{documentVisionParserStub: documentVisionParserStub{}}
	unknownPayload, _ := json.Marshal(map[string]any{
		"document_id": nonContiguousPDF.ID, "attempt_id": nonContiguousPDF.VisionAttemptID,
		"page_numbers": []int{1, 3, 4, 8},
	})
	if _, err := service.HandleDocumentVisionFallbackJob(ctx, jobruntime.Claim{
		Job:     contract.Job{Kind: knowledge.DocumentVisionFallbackJobKind, OrganizationID: organizationID, ProjectID: projectID, AttemptCount: 2, MaxAttempts: 1000},
		Payload: unknownPayload,
	}); err == nil {
		t.Fatal("uncertain external submission must stop automatic resubmission")
	}
	service.DocumentVision = documentVisionParserStub{}
	var unknownTaskStatus, intentID, providerCode, modelVersion, routeRevision string
	var intentCheckpoint []byte
	if err := db.QueryRowContext(ctx, `SELECT status, intent_id, provider_code, model_version,
		route_revision_id, checkpoint_json FROM platform_knowledge_document_vision_tasks
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ? AND task_index = 0`,
		organizationID, projectID, nonContiguousPDF.ID, nonContiguousPDF.VisionAttemptID,
	).Scan(&unknownTaskStatus, &intentID, &providerCode, &modelVersion, &routeRevision, &intentCheckpoint); err != nil {
		t.Fatalf("read uncertain submission intent: %v", err)
	}
	if unknownTaskStatus != "unknown" || len(intentID) != 64 || providerCode != "las" ||
		modelVersion != "las-pdf-test" || routeRevision != "route_document_vision_test" || !json.Valid(intentCheckpoint) {
		t.Fatalf("uncertain submission lost its intent: status=%q intent=%q provider=%q model=%q route=%q checkpoint=%s",
			unknownTaskStatus, intentID, providerCode, modelVersion, routeRevision, intentCheckpoint)
	}
	nonContiguousPDF, err = service.GetDocument(ctx, actor, projectID, nonContiguousPDF.ID)
	if err != nil || nonContiguousPDF.VisionFallbackStatus != "failed" ||
		nonContiguousPDF.VisionErrorCode != "DOCUMENT_VISION_SUBMISSION_UNKNOWN" {
		t.Fatalf("uncertain visual submission document=%#v err=%v", nonContiguousPDF, err)
	}
	if _, err := service.RunDocumentVisionFallback(ctx, actor, projectID, nonContiguousPDF.ID,
		knowledge.RunDocumentVisionFallbackRequest{PageNumbers: []int{1, 3, 4, 8}}); !errors.Is(err, knowledge.ErrDocumentVisionReconciliationRequired) {
		t.Fatalf("uncertain visual submission retry error = %v", err)
	}
	reconciliationActor := actor
	reconciliationActor.Scopes = append(append([]contract.Scope{}, actor.Scopes...), knowledge.ScopeDocumentVisionReconcile)
	secondReconciliationActor := contract.ActorContext{
		OrganizationID: organizationID,
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: secondUserID},
		Scopes:         append(append([]contract.Scope{}, actor.Scopes...), knowledge.ScopeDocumentVisionReconcile),
	}
	if err := identityStore.EnsureLocalActor(ctx, secondReconciliationActor); err != nil {
		t.Fatalf("create second reconciliation operator: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO project_memberships
		(organization_id, project_id, principal_kind, principal_id, role, status)
		VALUES (?, ?, 'user', ?, 'owner', 'active')`, organizationID, projectID, secondUserID); err != nil {
		t.Fatalf("grant second reconciliation operator project access: %v", err)
	}
	candidates, err := service.ListDocumentVisionReconciliationCandidates(ctx, reconciliationActor, projectID, 50)
	if err != nil {
		t.Fatalf("list document vision reconciliation candidates: %v", err)
	}
	var candidateFound bool
	for _, candidate := range candidates {
		if candidate.DocumentID == nonContiguousPDF.ID && candidate.TaskIndex == 0 {
			candidateFound = candidate.IntentID == intentID && candidate.Status == "unknown" &&
				reflect.DeepEqual(candidate.PageNumbers, []int{1}) && candidate.ProviderCode == "las"
		}
	}
	if !candidateFound {
		t.Fatalf("uncertain task was not safely discoverable: %#v", candidates)
	}
	reconciliationStore := jobruntime.MySQLStore{DB: db}
	acceptedReconciliationJobID := "visionreconciliationjob_" + suffix
	service.VisionScheduler = failingVisionSchedulerStub{}
	acceptedReconciliation, err := service.ProposeDocumentVisionReconciliation(
		ctx, reconciliationActor, projectID, nonContiguousPDF.ID,
		knowledge.ProposeDocumentVisionReconciliationRequest{
			TaskIndex: 0, ExpectedIntentID: intentID, Decision: "accepted",
			ExternalTaskID: "las_external_reconciled_" + suffix, EvidenceRef: "ticket:LAS-ACCEPTED-" + suffix,
		},
	)
	if err != nil || acceptedReconciliation.Status != "proposed" || acceptedReconciliation.ProposedBy != userID {
		t.Fatalf("accepted reconciliation proposal=%#v err=%v", acceptedReconciliation, err)
	}
	visibleProposal, err := service.GetDocumentVisionReconciliation(ctx, secondReconciliationActor, projectID, acceptedReconciliation.ID)
	if err != nil || visibleProposal.Decision != "accepted" || visibleProposal.EvidenceRef != "ticket:LAS-ACCEPTED-"+suffix {
		t.Fatalf("second operator proposal view=%#v err=%v", visibleProposal, err)
	}
	if _, err := service.ConfirmDocumentVisionReconciliation(
		ctx, reconciliationActor, projectID, acceptedReconciliation.ID,
		knowledge.ConfirmDocumentVisionReconciliationRequest{Approve: boolPointer(true)},
	); !errors.Is(err, knowledge.ErrDocumentVisionReconciliationSameActor) {
		t.Fatalf("same actor confirmation error=%v", err)
	}
	acceptedReconciliation, err = service.ConfirmDocumentVisionReconciliation(
		ctx, secondReconciliationActor, projectID, acceptedReconciliation.ID,
		knowledge.ConfirmDocumentVisionReconciliationRequest{Approve: boolPointer(true)},
	)
	if err != nil || acceptedReconciliation.Status != "applied" || acceptedReconciliation.ConfirmedBy != secondUserID ||
		acceptedReconciliation.ScheduledAt != nil {
		t.Fatalf("accepted reconciliation confirmation=%#v err=%v", acceptedReconciliation, err)
	}
	service.VisionScheduler = knowledge.JobRuntimeDocumentVisionFallbackScheduler{
		Store: reconciliationStore, NewID: func() (string, error) { return acceptedReconciliationJobID, nil },
		Now: func() time.Time { return integrationQueueNow },
	}
	if processed, err := service.ReconcileJobStates(ctx, 10); err != nil || !processed {
		t.Fatalf("recover accepted reconciliation schedule processed=%t err=%v", processed, err)
	}
	var acceptedTaskStatus, acceptedExternalTaskID, acceptedJobStatus string
	var acceptedJobPayload []byte
	var acceptedScheduled bool
	if err := db.QueryRowContext(ctx, `SELECT status, external_task_id
		FROM platform_knowledge_document_vision_tasks
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ? AND task_index = 0`,
		organizationID, projectID, nonContiguousPDF.ID, nonContiguousPDF.VisionAttemptID,
	).Scan(&acceptedTaskStatus, &acceptedExternalTaskID); err != nil {
		t.Fatalf("read accepted reconciled task: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT status, payload FROM platform_jobs
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		organizationID, projectID, acceptedReconciliationJobID,
	).Scan(&acceptedJobStatus, &acceptedJobPayload); err != nil {
		t.Fatalf("read accepted reconciliation job: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT scheduled_at IS NOT NULL
		FROM platform_knowledge_document_vision_reconciliations
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		organizationID, projectID, acceptedReconciliation.ID,
	).Scan(&acceptedScheduled); err != nil {
		t.Fatalf("read accepted reconciliation scheduling marker: %v", err)
	}
	var acceptedPayload struct {
		ScheduleKey string `json:"schedule_key"`
	}
	if err := json.Unmarshal(acceptedJobPayload, &acceptedPayload); err != nil {
		t.Fatalf("decode accepted reconciliation job payload: %v", err)
	}
	if acceptedTaskStatus != "submitted" || acceptedExternalTaskID != "las_external_reconciled_"+suffix ||
		acceptedJobStatus != "queued" || !acceptedScheduled ||
		acceptedPayload.ScheduleKey != acceptedReconciliation.ID {
		t.Fatalf("accepted task status=%q external=%q job=%q scheduled=%t payload=%s",
			acceptedTaskStatus, acceptedExternalTaskID, acceptedJobStatus, acceptedScheduled, acceptedJobPayload)
	}
	candidates, err = service.ListDocumentVisionReconciliationCandidates(ctx, reconciliationActor, projectID, 50)
	if err != nil {
		t.Fatalf("list reconciliation candidates after apply: %v", err)
	}
	for _, candidate := range candidates {
		if candidate.DocumentID == nonContiguousPDF.ID && candidate.TaskIndex == 0 {
			t.Fatalf("applied task remained an uncertain candidate: %#v", candidate)
		}
	}

	notAcceptedBytes := []byte("%PDF-1.7 not-accepted reconciliation " + suffix)
	notAcceptedPDF, err := service.CreateDocument(
		ctx, actor, projectID, "not-accepted-report.pdf", "application/pdf",
		bytes.NewReader(notAcceptedBytes), int64(len(notAcceptedBytes)),
	)
	if err != nil {
		t.Fatalf("create not-accepted reconciliation fixture: %v", err)
	}
	notAcceptedParsePayload, _ := json.Marshal(map[string]string{"document_id": notAcceptedPDF.ID})
	if _, err := service.HandleDocumentParseJob(ctx, jobruntime.Claim{
		Job:     contract.Job{Kind: knowledge.DocumentParseJobKind, OrganizationID: organizationID, ProjectID: projectID},
		Payload: notAcceptedParsePayload,
	}); err != nil {
		t.Fatalf("parse not-accepted reconciliation fixture: %v", err)
	}
	service.VisionScheduler = visionSchedulerStub{}
	notAcceptedPDF, err = service.RunDocumentVisionFallback(ctx, actor, projectID, notAcceptedPDF.ID, knowledge.RunDocumentVisionFallbackRequest{})
	if err != nil {
		t.Fatalf("queue not-accepted reconciliation fixture: %v", err)
	}
	service.DocumentVision = unknownSubmissionDocumentVisionParserStub{documentVisionParserStub: documentVisionParserStub{}}
	notAcceptedVisionPayload, _ := json.Marshal(map[string]any{
		"document_id": notAcceptedPDF.ID, "attempt_id": notAcceptedPDF.VisionAttemptID, "page_numbers": []int{1, 2},
	})
	if _, err := service.HandleDocumentVisionFallbackJob(ctx, jobruntime.Claim{
		Job:     contract.Job{Kind: knowledge.DocumentVisionFallbackJobKind, OrganizationID: organizationID, ProjectID: projectID, AttemptCount: 1, MaxAttempts: 1000},
		Payload: notAcceptedVisionPayload,
	}); err == nil {
		t.Fatal("not-accepted fixture must enter uncertain submission state")
	}
	var notAcceptedIntentID string
	if err := db.QueryRowContext(ctx, `SELECT intent_id FROM platform_knowledge_document_vision_tasks
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ? AND task_index = 0`,
		organizationID, projectID, notAcceptedPDF.ID, notAcceptedPDF.VisionAttemptID,
	).Scan(&notAcceptedIntentID); err != nil {
		t.Fatalf("read not-accepted intent: %v", err)
	}
	if _, err := service.ProposeDocumentVisionReconciliation(
		ctx, reconciliationActor, projectID, notAcceptedPDF.ID,
		knowledge.ProposeDocumentVisionReconciliationRequest{
			TaskIndex: 0, ExpectedIntentID: notAcceptedIntentID, Decision: "accepted",
			ExternalTaskID: "las_external_reconciled_" + suffix, EvidenceRef: "ticket:LAS-DUPLICATE-" + suffix,
		},
	); !errors.Is(err, knowledge.ErrDocumentVisionReconciliationConflict) {
		t.Fatalf("duplicate external task binding error=%v", err)
	}
	notAcceptedReconciliation, err := service.ProposeDocumentVisionReconciliation(
		ctx, reconciliationActor, projectID, notAcceptedPDF.ID,
		knowledge.ProposeDocumentVisionReconciliationRequest{
			TaskIndex: 0, ExpectedIntentID: notAcceptedIntentID, Decision: "not_accepted",
			EvidenceRef: "ticket:LAS-NOT-ACCEPTED-" + suffix,
		},
	)
	if err != nil {
		t.Fatalf("propose not-accepted reconciliation: %v", err)
	}
	notAcceptedReconciliation, err = service.ConfirmDocumentVisionReconciliation(
		ctx, secondReconciliationActor, projectID, notAcceptedReconciliation.ID,
		knowledge.ConfirmDocumentVisionReconciliationRequest{Approve: boolPointer(true)},
	)
	if err != nil || notAcceptedReconciliation.Status != "applied" || notAcceptedReconciliation.ScheduledAt != nil {
		t.Fatalf("not-accepted reconciliation=%#v err=%v", notAcceptedReconciliation, err)
	}
	notAcceptedPDF, err = service.GetDocument(ctx, actor, projectID, notAcceptedPDF.ID)
	if err != nil || notAcceptedPDF.Status != "partial" || notAcceptedPDF.VisionFallbackStatus != "failed" ||
		notAcceptedPDF.VisionErrorCode != "DOCUMENT_VISION_RECONCILED_NOT_ACCEPTED" || notAcceptedPDF.ChunkCount < 1 {
		t.Fatalf("not-accepted document=%#v err=%v", notAcceptedPDF, err)
	}
	priorAttemptID := notAcceptedPDF.VisionAttemptID
	service.DocumentVision = documentVisionParserStub{}
	service.VisionScheduler = visionSchedulerStub{}
	notAcceptedPDF, err = service.RunDocumentVisionFallback(ctx, actor, projectID, notAcceptedPDF.ID, knowledge.RunDocumentVisionFallbackRequest{})
	if err != nil || notAcceptedPDF.VisionFallbackStatus != "queued" || notAcceptedPDF.VisionAttemptID == priorAttemptID {
		t.Fatalf("explicit retry after not-accepted reconciliation=%#v err=%v", notAcceptedPDF, err)
	}
	orphanBytes := []byte("%PDF-1.7 orphaned visual fallback " + suffix)
	orphanPDF, err := service.CreateDocument(
		ctx, actor, projectID, "orphaned-visual-report.pdf", "application/pdf",
		bytes.NewReader(orphanBytes), int64(len(orphanBytes)),
	)
	if err != nil {
		t.Fatalf("create orphan recovery fixture: %v", err)
	}
	orphanParsePayload, _ := json.Marshal(map[string]string{"document_id": orphanPDF.ID})
	if _, err := service.HandleDocumentParseJob(ctx, jobruntime.Claim{
		Job:     contract.Job{Kind: knowledge.DocumentParseJobKind, OrganizationID: organizationID, ProjectID: projectID},
		Payload: orphanParsePayload,
	}); err != nil {
		t.Fatalf("parse orphan recovery fixture: %v", err)
	}
	orphanPDF, err = service.GetDocument(ctx, actor, projectID, orphanPDF.ID)
	if err != nil || orphanPDF.Status != "partial" || orphanPDF.ChunkCount < 1 {
		t.Fatalf("orphan recovery baseline=%#v err=%v", orphanPDF, err)
	}
	orphanedAt := time.Now().UTC().Add(-10 * time.Second)
	orphanAttemptID := "visionattempt_orphan_" + suffix
	if _, err := db.ExecContext(ctx, `UPDATE platform_knowledge_documents
		SET vision_fallback_status = 'queued', vision_attempt_id = ?, vision_selected_pages = JSON_ARRAY(1, 2),
			vision_completed_pages = JSON_ARRAY(), vision_model_alias = ?,
			vision_route_revision_id = ?, updated_at = ?, heartbeat_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		orphanAttemptID, "cookies.document.vision.standard", "route_document_vision_test", orphanedAt, orphanedAt,
		organizationID, projectID, orphanPDF.ID); err != nil {
		t.Fatalf("create orphaned visual state: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO platform_knowledge_document_vision_tasks
		(organization_id, project_id, document_id, attempt_id, task_index, page_numbers,
		 status, model_alias, route_revision_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, JSON_ARRAY(1, 2), 'prepared', ?, ?, ?, ?)`,
		organizationID, projectID, orphanPDF.ID, orphanAttemptID,
		"cookies.document.vision.standard", "route_document_vision_test", orphanedAt, orphanedAt); err != nil {
		t.Fatalf("create orphaned visual task checkpoint: %v", err)
	}
	recoveryStore := jobruntime.MySQLStore{DB: db}
	recoveryJobID := "visionrecoveryjob_" + suffix
	service.VisionScheduler = knowledge.JobRuntimeDocumentVisionFallbackScheduler{
		Store: recoveryStore, NewID: func() (string, error) { return recoveryJobID, nil },
		Now: func() time.Time { return integrationQueueNow },
	}
	if processed, err := service.ReconcileJobStates(ctx, 10); err != nil || !processed {
		t.Fatalf("recover orphaned visual job processed=%t err=%v", processed, err)
	}
	var recoveredJobStatus string
	if err := db.QueryRowContext(ctx, `SELECT status FROM platform_jobs
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		organizationID, projectID, recoveryJobID).Scan(&recoveredJobStatus); err != nil || recoveredJobStatus != "queued" {
		t.Fatalf("recovered visual job status=%q err=%v", recoveredJobStatus, err)
	}
	terminalVisionAt := time.Now().UTC().Add(time.Second)
	if _, err := db.ExecContext(ctx, `UPDATE platform_jobs
		SET status = 'failed', error_code = 'VISION_FAULT_INJECTED', error_message = 'worker stopped', updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		terminalVisionAt, organizationID, projectID, recoveryJobID); err != nil {
		t.Fatalf("fail recovered visual job: %v", err)
	}
	if processed, err := service.ReconcileJobStates(ctx, 10); err != nil || !processed {
		t.Fatalf("reconcile terminal visual job processed=%t err=%v", processed, err)
	}
	orphanPDF, err = service.GetDocument(ctx, actor, projectID, orphanPDF.ID)
	if err != nil || orphanPDF.VisionFallbackStatus != "failed" || orphanPDF.Status != "partial" ||
		orphanPDF.ChunkCount < 1 || orphanPDF.VisionErrorCode != "VISION_FAULT_INJECTED" {
		t.Fatalf("terminal visual recovery document=%#v err=%v", orphanPDF, err)
	}
	service.VisionScheduler = visionSchedulerStub{}
	presentationConverter := &presentationConverterStub{failuresRemaining: 1}
	service.DocumentConverter = presentationConverter
	presentationBytes := []byte("PK presentation fixture " + suffix)
	presentation, err := service.CreateDocument(
		ctx, actor, projectID, "strategy-deck.pptx", knowledge.PowerPointOpenXMLMIME,
		bytes.NewReader(presentationBytes), int64(len(presentationBytes)),
	)
	if err != nil {
		t.Fatalf("create presentation fixture: %v", err)
	}
	presentationParsePayload, _ := json.Marshal(map[string]string{"document_id": presentation.ID})
	if _, err := service.HandleDocumentParseJob(ctx, jobruntime.Claim{
		Job:     contract.Job{Kind: knowledge.DocumentParseJobKind, OrganizationID: organizationID, ProjectID: projectID},
		Payload: presentationParsePayload,
	}); err != nil {
		t.Fatalf("parse presentation fixture: %v", err)
	}
	presentation, err = service.GetDocument(ctx, actor, projectID, presentation.ID)
	if err != nil || presentation.Status != "partial" {
		t.Fatalf("presentation text baseline=%#v err=%v", presentation, err)
	}
	presentationCapability, err := service.GetDocumentVisionFallbackCapability(ctx, actor, projectID, presentation.ID)
	if err != nil || !presentationCapability.Available || !presentationCapability.ConversionRequired ||
		presentationCapability.ConverterCode != "presentation-test" {
		t.Fatalf("presentation capability=%#v err=%v", presentationCapability, err)
	}
	presentation, err = service.RunDocumentVisionFallback(ctx, actor, projectID, presentation.ID, knowledge.RunDocumentVisionFallbackRequest{})
	if err != nil || presentation.VisionFallbackStatus != "queued" {
		t.Fatalf("queue presentation visual fallback=%#v err=%v", presentation, err)
	}
	presentationVisionPayload, _ := json.Marshal(map[string]any{
		"document_id": presentation.ID, "attempt_id": presentation.VisionAttemptID, "page_numbers": []int{1, 2},
	})
	for attempt := 1; attempt <= 4; attempt++ {
		_, handleErr := service.HandleDocumentVisionFallbackJob(ctx, jobruntime.Claim{
			Job: contract.Job{
				ID: "presentation-vision-job", Kind: knowledge.DocumentVisionFallbackJobKind,
				OrganizationID: actor.OrganizationID, ProjectID: projectID, AttemptCount: attempt, MaxAttempts: 1000,
			},
			Payload: presentationVisionPayload,
		})
		var deferred jobruntime.DeferredError
		if attempt < 4 && !errors.As(handleErr, &deferred) {
			t.Fatalf("presentation visual attempt %d error=%v", attempt, handleErr)
		}
		if attempt == 4 && handleErr != nil {
			t.Fatalf("presentation visual final error=%v", handleErr)
		}
		if attempt == 1 || attempt == 2 {
			phaseDocument, phaseErr := service.GetDocument(ctx, actor, projectID, presentation.ID)
			wantPhase := "visual_conversion"
			if attempt == 2 {
				wantPhase = "visual_fallback"
			}
			if phaseErr != nil || phaseDocument.ParsePhase != wantPhase {
				t.Fatalf("presentation attempt %d phase=%q want=%q err=%v", attempt, phaseDocument.ParsePhase, wantPhase, phaseErr)
			}
		}
	}
	if presentationConverter.convertCalls != 2 {
		t.Fatalf("presentation converter calls=%d, want one retry then success", presentationConverter.convertCalls)
	}
	presentation, err = service.GetDocument(ctx, actor, projectID, presentation.ID)
	if err != nil || presentation.Status != "ready" || presentation.VisionFallbackStatus != "succeeded" ||
		presentation.MIMEType != knowledge.PowerPointOpenXMLMIME {
		t.Fatalf("converted presentation=%#v err=%v", presentation, err)
	}
	var conversionStatus, sourceSHA256, derivedSHA256, derivedBucket, derivedKey string
	var derivedSize int64
	if err := db.QueryRowContext(ctx, `SELECT status, source_sha256, derived_sha256, derived_size_bytes,
		derived_bucket, derived_object_key FROM platform_knowledge_document_vision_input_conversions
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ?`,
		organizationID, projectID, presentation.ID, presentation.VisionAttemptID,
	).Scan(&conversionStatus, &sourceSHA256, &derivedSHA256, &derivedSize, &derivedBucket, &derivedKey); err != nil ||
		conversionStatus != "ready" || sourceSHA256 != presentation.ContentSHA256 || len(derivedSHA256) != 64 ||
		derivedSize < 8 || derivedBucket != service.AssetsBucket ||
		!strings.HasPrefix(derivedKey, fmt.Sprintf("assets/%s/%s/knowledge/%s/derived/document-vision/", organizationID, projectID, presentation.ID)) {
		t.Fatalf("presentation conversion lineage status=%q source=%q derived=%q size=%d bucket=%q key=%q err=%v",
			conversionStatus, sourceSHA256, derivedSHA256, derivedSize, derivedBucket, derivedKey, err)
	}
	presentationStream, _, _, readErr := service.OpenDocumentContent(ctx, actor, projectID, presentation.ID)
	if readErr != nil {
		t.Fatalf("open preserved presentation source: %v", readErr)
	}
	originalPresentation, readErr := io.ReadAll(presentationStream)
	_ = presentationStream.Close()
	if readErr != nil || !bytes.Equal(originalPresentation, presentationBytes) {
		t.Fatalf("presentation source was not preserved: bytes=%q err=%v", originalPresentation, readErr)
	}
	service.DocumentConverter = rejectingPresentationConverterStub{}
	rejectedBytes := []byte("PK rejected presentation fixture " + suffix)
	rejectedPresentation, err := service.CreateDocument(
		ctx, actor, projectID, "rejected-deck.pptx", knowledge.PowerPointOpenXMLMIME,
		bytes.NewReader(rejectedBytes), int64(len(rejectedBytes)),
	)
	if err != nil {
		t.Fatalf("create rejected presentation fixture: %v", err)
	}
	rejectedParsePayload, _ := json.Marshal(map[string]string{"document_id": rejectedPresentation.ID})
	if _, err := service.HandleDocumentParseJob(ctx, jobruntime.Claim{
		Job:     contract.Job{Kind: knowledge.DocumentParseJobKind, OrganizationID: organizationID, ProjectID: projectID},
		Payload: rejectedParsePayload,
	}); err != nil {
		t.Fatalf("parse rejected presentation fixture: %v", err)
	}
	rejectedPresentation, err = service.GetDocument(ctx, actor, projectID, rejectedPresentation.ID)
	if err != nil || rejectedPresentation.Status != "partial" || rejectedPresentation.ChunkCount < 1 {
		t.Fatalf("rejected presentation baseline=%#v err=%v", rejectedPresentation, err)
	}
	rejectedPresentation, err = service.RunDocumentVisionFallback(ctx, actor, projectID, rejectedPresentation.ID, knowledge.RunDocumentVisionFallbackRequest{})
	if err != nil {
		t.Fatalf("queue rejected presentation fallback: %v", err)
	}
	rejectedVisionPayload, _ := json.Marshal(map[string]any{
		"document_id": rejectedPresentation.ID, "attempt_id": rejectedPresentation.VisionAttemptID, "page_numbers": []int{1, 2},
	})
	if _, err := service.HandleDocumentVisionFallbackJob(ctx, jobruntime.Claim{
		Job: contract.Job{
			ID: "rejected-presentation-job", Kind: knowledge.DocumentVisionFallbackJobKind,
			OrganizationID: organizationID, ProjectID: projectID, AttemptCount: 1, MaxAttempts: 1000,
		},
		Payload: rejectedVisionPayload,
	}); err == nil {
		t.Fatal("deterministically rejected presentation conversion must fail the visual job")
	}
	rejectedPresentation, err = service.GetDocument(ctx, actor, projectID, rejectedPresentation.ID)
	if err != nil || rejectedPresentation.Status != "partial" || rejectedPresentation.ChunkCount < 1 ||
		rejectedPresentation.VisionFallbackStatus != "failed" || rejectedPresentation.VisionErrorCode != "DOCUMENT_VISION_CONVERSION_REJECTED" {
		t.Fatalf("rejected presentation result=%#v err=%v", rejectedPresentation, err)
	}
	var rejectedConversionStatus string
	if err := db.QueryRowContext(ctx, `SELECT status FROM platform_knowledge_document_vision_input_conversions
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ?`,
		organizationID, projectID, rejectedPresentation.ID, rejectedPresentation.VisionAttemptID,
	).Scan(&rejectedConversionStatus); err != nil || rejectedConversionStatus != "failed" {
		t.Fatalf("rejected conversion status=%q err=%v", rejectedConversionStatus, err)
	}
	service.DocumentConverter = alwaysRetryingPresentationConverterStub{}
	exhaustedBytes := []byte("PK retry-exhausted presentation fixture " + suffix)
	exhaustedPresentation, err := service.CreateDocument(
		ctx, actor, projectID, "retry-exhausted-deck.ppt", knowledge.PowerPointLegacyMIME,
		bytes.NewReader(exhaustedBytes), int64(len(exhaustedBytes)),
	)
	if err != nil {
		t.Fatalf("create retry-exhausted presentation fixture: %v", err)
	}
	exhaustedParsePayload, _ := json.Marshal(map[string]string{"document_id": exhaustedPresentation.ID})
	if _, err := service.HandleDocumentParseJob(ctx, jobruntime.Claim{
		Job: contract.Job{Kind: knowledge.DocumentParseJobKind, OrganizationID: organizationID, ProjectID: projectID}, Payload: exhaustedParsePayload,
	}); err != nil {
		t.Fatalf("parse retry-exhausted presentation: %v", err)
	}
	exhaustedPresentation, err = service.GetDocument(ctx, actor, projectID, exhaustedPresentation.ID)
	if err != nil {
		t.Fatalf("load retry-exhausted presentation: %v", err)
	}
	exhaustedPresentation, err = service.RunDocumentVisionFallback(ctx, actor, projectID, exhaustedPresentation.ID, knowledge.RunDocumentVisionFallbackRequest{})
	if err != nil {
		t.Fatalf("queue retry-exhausted presentation: %v", err)
	}
	exhaustedVisionPayload, _ := json.Marshal(map[string]any{
		"document_id": exhaustedPresentation.ID, "attempt_id": exhaustedPresentation.VisionAttemptID, "page_numbers": []int{1, 2},
	})
	for attempt := 1; attempt <= 3; attempt++ {
		_, handleErr := service.HandleDocumentVisionFallbackJob(ctx, jobruntime.Claim{
			Job: contract.Job{
				ID: "retry-exhausted-presentation-job", Kind: knowledge.DocumentVisionFallbackJobKind,
				OrganizationID: organizationID, ProjectID: projectID, AttemptCount: attempt, MaxAttempts: 1000,
			}, Payload: exhaustedVisionPayload,
		})
		var deferred jobruntime.DeferredError
		if attempt < 3 && !errors.As(handleErr, &deferred) {
			t.Fatalf("retry-exhausted presentation attempt %d error=%v", attempt, handleErr)
		}
		if attempt == 3 && (handleErr == nil || errors.As(handleErr, &deferred)) {
			t.Fatalf("third conversion failure must exhaust its independent budget: %v", handleErr)
		}
	}
	exhaustedPresentation, err = service.GetDocument(ctx, actor, projectID, exhaustedPresentation.ID)
	if err != nil || exhaustedPresentation.Status != "partial" || exhaustedPresentation.VisionErrorCode != "DOCUMENT_VISION_CONVERSION_RETRY_EXHAUSTED" {
		t.Fatalf("retry-exhausted presentation result=%#v err=%v", exhaustedPresentation, err)
	}
	var exhaustedAttempts int
	if err := db.QueryRowContext(ctx, `SELECT attempt_count FROM platform_knowledge_document_vision_input_conversions
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ?`,
		organizationID, projectID, exhaustedPresentation.ID, exhaustedPresentation.VisionAttemptID,
	).Scan(&exhaustedAttempts); err != nil || exhaustedAttempts != 3 {
		t.Fatalf("conversion attempt count=%d err=%v", exhaustedAttempts, err)
	}
	service.DocumentConverter = nil
	service.DocumentVision = nil
	service.VisionScheduler = nil
	service.VisionModelAlias = ""

	runtimeStore := jobruntime.MySQLStore{DB: db}
	service.JobCanceller = runtimeStore
	service.DocumentScheduler = knowledge.JobRuntimeDocumentParseScheduler{
		Store: runtimeStore, NewID: func() (string, error) { return "documentjob_" + suffix, nil },
		Now: func() time.Time { return integrationQueueNow },
	}
	controlledPDFBytes := []byte("%PDF-1.7 controlled retry payload " + suffix)
	controlledPDF, err := service.CreateDocument(
		ctx, actor, projectID, "controlled-report.pdf", "application/pdf",
		bytes.NewReader(controlledPDFBytes), int64(len(controlledPDFBytes)),
	)
	if err != nil || controlledPDF.Status != "parse_queued" {
		t.Fatalf("controlled PDF=%#v err=%v", controlledPDF, err)
	}
	activitySnapshot, err := (systemsstrategy.Service{DB: db, Projects: projectService}).ListTaskActivities(ctx, actor, projectID, "", 20)
	if err != nil {
		t.Fatalf("ListTaskActivities() error=%v", err)
	}
	var documentActivity *systemsstrategy.TaskActivity
	for index := range activitySnapshot.Items {
		if activitySnapshot.Items[index].ResourceRef.ID == controlledPDF.ID {
			documentActivity = &activitySnapshot.Items[index]
			break
		}
	}
	if documentActivity == nil || documentActivity.Kind != "document_parse" || documentActivity.Phase != "queued" || documentActivity.Progress.Value == nil || *documentActivity.Progress.Value != 0 {
		t.Fatalf("document activity=%#v", documentActivity)
	}
	documentControl, err := service.CancelDocumentParse(ctx, actor, projectID, controlledPDF.ID)
	if err != nil || documentControl.ExecutionStatus != contract.JobCancelled {
		t.Fatalf("document cancel=%#v err=%v", documentControl, err)
	}
	service.DocumentScheduler = knowledge.JobRuntimeDocumentParseScheduler{
		Store: runtimeStore, NewID: func() (string, error) { return "documentretryjob_" + suffix, nil },
		Now: func() time.Time { return integrationQueueNow },
	}
	controlledPDF, err = service.RetryDocumentParse(ctx, actor, projectID, controlledPDF.ID)
	if err != nil || controlledPDF.Status != "parse_queued" {
		t.Fatalf("document retry=%#v err=%v", controlledPDF, err)
	}

	service.Scheduler = knowledge.JobRuntimeResearchScheduler{
		Store: runtimeStore, NewID: func() (string, error) { return "researchjob_" + suffix, nil },
		Now: func() time.Time { return integrationQueueNow },
	}
	queuedResearch, err := service.RunResearch(ctx, actor, projectID, knowledge.ResearchRequest{
		Mode: "web", RunMode: "deep", Category: "competitor", Purpose: "deep_research",
		SourceRef:        &contract.ResourceRef{Type: "strategy_workspace", ID: "workspace_" + suffix},
		InputSnapshotRef: "strategy_workspace:workspace_" + suffix + ":v1",
		InputSnapshot:    json.RawMessage(`{"contract_version":"strategy-project-context-manifest/v1"}`), Query: "可恢复研究任务",
		DocumentIDs: []string{}, DisclosedFields: []string{"query"}, Confirmed: true,
	})
	if err != nil || queuedResearch.Status != "queued" {
		t.Fatalf("queued research=%#v err=%v", queuedResearch, err)
	}
	researchControl, err := service.CancelResearch(ctx, actor, projectID, queuedResearch.ID)
	if err != nil || researchControl.ExecutionStatus != contract.JobCancelled {
		t.Fatalf("research cancel=%#v err=%v", researchControl, err)
	}
	service.Scheduler = knowledge.JobRuntimeResearchScheduler{
		Store: runtimeStore, NewID: func() (string, error) { return "researchretryjob_" + suffix, nil },
		Now: func() time.Time { return integrationQueueNow },
	}
	queuedResearch, err = service.RetryResearch(ctx, actor, projectID, queuedResearch.ID)
	if err != nil || queuedResearch.Status != "planning" {
		t.Fatalf("research retry=%#v err=%v", queuedResearch, err)
	}

	terminalAt := time.Now().UTC().Add(time.Second)
	if _, err := db.ExecContext(ctx, `UPDATE platform_jobs
		SET status = 'failed', error_code = 'FAULT_INJECTED', error_message = 'worker stopped',
			retryable = TRUE, updated_at = ?
		WHERE organization_id = ? AND id IN (?, ?)`, terminalAt, organizationID,
		"documentretryjob_"+suffix, "researchretryjob_"+suffix); err != nil {
		t.Fatal(err)
	}
	processed, err := service.ReconcileJobStates(ctx, 10)
	if err != nil || !processed {
		t.Fatalf("reconcile processed=%t err=%v", processed, err)
	}
	controlledPDF, err = service.GetDocument(ctx, actor, projectID, controlledPDF.ID)
	if err != nil || controlledPDF.Status != "parse_failed" || controlledPDF.ParseErrorCode != "FAULT_INJECTED" {
		t.Fatalf("reconciled document=%#v err=%v", controlledPDF, err)
	}
	queuedResearch, err = service.GetResearchRun(ctx, actor, projectID, queuedResearch.ID)
	if err != nil || queuedResearch.Status != "failed" || queuedResearch.ErrorCode != "FAULT_INJECTED" {
		t.Fatalf("reconciled research=%#v err=%v", queuedResearch, err)
	}
}

type researchRunner struct{}

type failFirstResearchRunner struct{ calls int }

type failAfterThirdRoundResearchRunner struct {
	rounds       []int
	failedRound4 bool
}

type researchRetrySchedulerStub struct{}

type researchVerifier struct{}

type parserStub struct{}

func (researchRetrySchedulerStub) Schedule(context.Context, knowledge.ResearchRun) error { return nil }

func (researchRetrySchedulerStub) ScheduleResearchRetry(context.Context, knowledge.ResearchRun) error {
	return nil
}

func (runner *failFirstResearchRunner) Run(ctx context.Context, input knowledge.ExternalResearchInput) ([]knowledge.ExternalResearchResult, error) {
	runner.calls++
	if runner.calls == 1 {
		return nil, errors.New("injected provider failure")
	}
	return (researchRunner{}).Run(ctx, input)
}

func (runner *failAfterThirdRoundResearchRunner) Run(_ context.Context, input knowledge.ExternalResearchInput) ([]knowledge.ExternalResearchResult, error) {
	runner.rounds = append(runner.rounds, input.Round)
	if input.Round == 4 && !runner.failedRound4 {
		runner.failedRound4 = true
		return nil, errors.New("injected failure after round-three checkpoint")
	}
	primaryURL := fmt.Sprintf("https://example.com/checkpoint/%d", input.Round)
	secondaryURL := fmt.Sprintf("https://example.org/checkpoint/%d", input.Round)
	return []knowledge.ExternalResearchResult{{
		Title:     fmt.Sprintf("checkpoint round %d", input.Round),
		Content:   fmt.Sprintf("independently verified decision evidence from round %d", input.Round),
		Citations: []string{primaryURL, secondaryURL},
		Sources: []knowledge.ExternalResearchSource{
			{SourceClass: "web", MediaType: "article", Title: "primary", URL: primaryURL},
			{SourceClass: "web", MediaType: "article", Title: "secondary", URL: secondaryURL},
		},
		Findings: []knowledge.ExternalResearchFinding{{
			Claim: fmt.Sprintf("verified checkpoint finding %d", input.Round), TimeScope: "2026-H1", Confidence: "high",
			TargetArtifact: "strategy", TargetFieldPath: "executive_summary",
			Implication:   fmt.Sprintf("apply independently verified finding %d", input.Round),
			ProposedValue: json.RawMessage(strconv.Quote(fmt.Sprintf("checkpoint decision %d", input.Round))),
			SupportingEvidence: []knowledge.ExternalResearchEvidence{
				{URL: primaryURL, Excerpt: "independently verified"},
				{URL: secondaryURL, Excerpt: "decision evidence"},
			},
		}},
		Coverage:        map[string]bool{fmt.Sprintf("round_%d", input.Round): true},
		OpenGaps:        []string{fmt.Sprintf("continue_after_round_%d", input.Round)},
		RecommendedStop: input.Round == 4,
		ActionSummary:   fmt.Sprintf("completed round %d", input.Round),
	}}, nil
}

func (parserStub) Parse(_ context.Context, request knowledge.DocumentParseRequest) (knowledge.ParsedDocument, error) {
	mimeType := request.MIMEType
	if mimeType == "" {
		mimeType = "application/pdf"
	}
	return knowledge.ParsedDocument{
		Text:     "市场规模持续增长。\n\n品牌事实需要逐项验证。",
		MIMEType: mimeType, ParserCode: "tika", ParserVersion: "test",
		Metadata: json.RawMessage(fmt.Sprintf(`{"Content-Type":%q,"xmpTPg:NPages":"2"}`, mimeType)),
	}, nil
}

type parseSchedulerStub struct{}

func (parseSchedulerStub) ScheduleDocumentParse(context.Context, knowledge.Document) error {
	return nil
}

type visionSchedulerStub struct{}

func (visionSchedulerStub) ScheduleDocumentVisionFallback(context.Context, knowledge.Document, []int, string) error {
	return nil
}

type failingVisionSchedulerStub struct{}

func (failingVisionSchedulerStub) ScheduleDocumentVisionFallback(context.Context, knowledge.Document, []int, string) error {
	return errors.New("fault-injected reconciliation scheduler outage")
}

type documentVisionParserStub struct{}

func (documentVisionParserStub) Inspect(context.Context, contract.OrganizationID, string) (knowledge.DocumentVisionCapability, error) {
	return knowledge.DocumentVisionCapability{
		Available: true, ModelAlias: "cookies.document.vision.standard", UpstreamModel: "las-pdf-test",
		RouteRevisionID: "route_document_vision_test", SupportedMIMEs: []string{"application/pdf"},
	}, nil
}

func (documentVisionParserStub) PrepareSubmission(_ context.Context, request knowledge.DocumentVisionParseRequest) (knowledge.DocumentVisionSubmissionIntent, error) {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s:%v", request.DocumentID, request.PageNumbers)))
	intentID := fmt.Sprintf("%x", digest[:])
	return knowledge.DocumentVisionSubmissionIntent{
		IntentID: intentID, ProviderCode: "las", ModelVersion: "las-pdf-test",
		RouteRevisionID: "route_document_vision_test",
		Checkpoint:      json.RawMessage(fmt.Sprintf(`{"version":"test/v2","intent_id":%q}`, intentID)),
	}, nil
}

func (documentVisionParserStub) SubmitPrepared(_ context.Context, request knowledge.DocumentVisionParseRequest, intent knowledge.DocumentVisionSubmissionIntent) (knowledge.DocumentVisionSubmission, error) {
	_, _ = io.ReadAll(request.Source)
	return knowledge.DocumentVisionSubmission{
		ExternalTaskID: "las_task_" + intent.IntentID[:24], ProviderCode: "las", ModelVersion: "las-pdf-test",
		RouteRevisionID: "route_document_vision_test", Checkpoint: intent.Checkpoint,
		PollAfter: 500 * time.Millisecond,
	}, nil
}

func (documentVisionParserStub) Poll(context.Context, knowledge.DocumentVisionSubmission) (knowledge.DocumentVisionPollResult, error) {
	result := knowledge.DocumentVisionParseResult{
		ProviderCode: "las", ModelVersion: "las-pdf-test", RouteRevisionID: "route_document_vision_test",
		Pages: []knowledge.DocumentVisionPage{
			{PageNumber: 1, Markdown: "# 市场规模\n2026 年市场规模达到 100 亿元，数据表格和标题均已恢复。", Locator: map[string]any{"page_number": 1}},
			{PageNumber: 2, Markdown: "# 品牌事实\n品牌核心主张、证据来源和适用范围均需逐项核验后使用。", Locator: map[string]any{"page_number": 2}},
		},
		Latency: 1200 * time.Millisecond, Usage: json.RawMessage(`{"input_tokens":120,"output_tokens":80,"total_tokens":200}`),
	}
	billable := 2
	return knowledge.DocumentVisionPollResult{
		Status: knowledge.DocumentVisionPollCompleted, Result: &result, BillablePages: &billable,
	}, nil
}

type unknownSubmissionDocumentVisionParserStub struct{ documentVisionParserStub }

func (unknownSubmissionDocumentVisionParserStub) SubmitPrepared(_ context.Context, request knowledge.DocumentVisionParseRequest, _ knowledge.DocumentVisionSubmissionIntent) (knowledge.DocumentVisionSubmission, error) {
	_, _ = io.ReadAll(request.Source)
	return knowledge.DocumentVisionSubmission{}, fmt.Errorf("fault-injected timeout after request transmission")
}

type presentationConverterStub struct {
	failuresRemaining int
	convertCalls      int
}

func (*presentationConverterStub) Inspect(context.Context) (knowledge.DocumentVisionInputConversionCapability, error) {
	return knowledge.DocumentVisionInputConversionCapability{
		Available: true, ConverterCode: "presentation-test", Version: "presentation-test-v1",
	}, nil
}

func (s *presentationConverterStub) Convert(_ context.Context, request knowledge.DocumentVisionInputConversionRequest) (knowledge.DocumentVisionInputConversionResult, error) {
	s.convertCalls++
	if s.failuresRemaining > 0 {
		s.failuresRemaining--
		return knowledge.DocumentVisionInputConversionResult{}, knowledge.NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERTER_UNAVAILABLE", "converter temporarily unavailable", true,
		)
	}
	if request.MIMEType != knowledge.PowerPointOpenXMLMIME {
		return knowledge.DocumentVisionInputConversionResult{}, fmt.Errorf("unexpected input MIME %q", request.MIMEType)
	}
	if _, err := io.ReadAll(request.Source); err != nil {
		return knowledge.DocumentVisionInputConversionResult{}, err
	}
	return knowledge.DocumentVisionInputConversionResult{
		PDF:           []byte("%PDF-1.7\npresentation conversion fixture\n%%EOF"),
		ConverterCode: "presentation-test", Version: "presentation-test-v1",
	}, nil
}

type rejectingPresentationConverterStub struct{}

func (rejectingPresentationConverterStub) Inspect(context.Context) (knowledge.DocumentVisionInputConversionCapability, error) {
	return knowledge.DocumentVisionInputConversionCapability{
		Available: true, ConverterCode: "presentation-reject-test", Version: "presentation-reject-v1",
	}, nil
}

type alwaysRetryingPresentationConverterStub struct{}

func (alwaysRetryingPresentationConverterStub) Inspect(context.Context) (knowledge.DocumentVisionInputConversionCapability, error) {
	return knowledge.DocumentVisionInputConversionCapability{
		Available: true, ConverterCode: "presentation-retry-test", Version: "presentation-retry-v1",
	}, nil
}

func (alwaysRetryingPresentationConverterStub) Convert(context.Context, knowledge.DocumentVisionInputConversionRequest) (knowledge.DocumentVisionInputConversionResult, error) {
	return knowledge.DocumentVisionInputConversionResult{}, knowledge.NewDocumentVisionInputConversionError(
		"DOCUMENT_VISION_CONVERTER_UNAVAILABLE", "converter temporarily unavailable", true,
	)
}

func (rejectingPresentationConverterStub) Convert(context.Context, knowledge.DocumentVisionInputConversionRequest) (knowledge.DocumentVisionInputConversionResult, error) {
	return knowledge.DocumentVisionInputConversionResult{}, knowledge.NewDocumentVisionInputConversionError(
		"DOCUMENT_VISION_CONVERSION_REJECTED", "presentation could not be converted", false,
	)
}

func (researchVerifier) Verify(context.Context, string, string) (knowledge.VerifiedResearchSource, error) {
	return knowledge.VerifiedResearchSource{ContentHash: strings.Repeat("a", 64), ExcerptFound: true}, nil
}

func (researchRunner) Run(_ context.Context, input knowledge.ExternalResearchInput) ([]knowledge.ExternalResearchResult, error) {
	if input.RunMode == "deep" {
		return []knowledge.ExternalResearchResult{{
			Title: "研发负责人决策证据研究", Content: "两类独立来源都强调可量化的交付证据。",
			Citations: []string{"https://example.com/research", "https://example.org/report"},
			Sources: []knowledge.ExternalResearchSource{
				{SourceClass: "web", MediaType: "article", Title: "研究一", URL: "https://example.com/research"},
				{SourceClass: "web", MediaType: "article", Title: "研究二", URL: "https://example.org/report"},
			},
			Findings: []knowledge.ExternalResearchFinding{{
				Claim: "研发负责人优先采用可量化的交付证据", TimeScope: "2026-H1", Confidence: "high",
				TargetArtifact: "brief", TargetFieldPath: "proposition",
				Implication: "核心主张应包含可验证的精度与交付结果。", ProposedValue: json.RawMessage(`"以可验证精度与交付结果降低研发决策风险"`),
				SupportingEvidence: []knowledge.ExternalResearchEvidence{
					{URL: "https://example.com/research", Excerpt: "可量化的交付证据优先"},
					{URL: "https://example.org/report", Excerpt: "精度与交付结果降低决策风险"},
				},
			}},
			Coverage: map[string]bool{"决策证据偏好": true}, RecommendedStop: true,
			ActionSummary: "交叉搜索两个独立域并提取支持片段",
		}}, nil
	}
	return []knowledge.ExternalResearchResult{{
		Title:     "研发负责人重视可验证证据",
		Content:   "研究样本显示，明确的精度与交付证据优先于泛化品牌表达。",
		Citations: []string{"https://example.com/research"},
	}}, nil
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func boolPointer(value bool) *bool { return &value }

func cleanupKnowledgeIntegration(t *testing.T, db *sql.DB, organizationID contract.OrganizationID, userIDs ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	statements := []string{
		"DELETE FROM platform_jobs WHERE organization_id=?",
		"DELETE FROM platform_research_iterations WHERE organization_id=?",
		"DELETE FROM platform_research_findings WHERE organization_id=?",
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
	for _, userID := range userIDs {
		if _, err := db.ExecContext(ctx, "DELETE FROM users WHERE id=?", userID); err != nil {
			t.Errorf("cleanup user %q: %v", userID, err)
		}
	}
}
