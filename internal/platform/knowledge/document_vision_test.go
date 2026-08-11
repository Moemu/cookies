package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
)

func TestNormalizeVisionPageSelectionRequiresExplicitPagesForLongOrUnknownDocuments(t *testing.T) {
	if _, err := normalizeVisionPageSelection(nil, nil); err != ErrDocumentVisionPageSelectionRequired {
		t.Fatalf("unknown page selection error = %v", err)
	}
	total := 25
	if _, err := normalizeVisionPageSelection(nil, &total); err != ErrDocumentVisionPageSelectionRequired {
		t.Fatalf("long document page selection error = %v", err)
	}
	pages, err := normalizeVisionPageSelection([]int{5, 2, 5, 3}, &total)
	if err != nil {
		t.Fatalf("normalizeVisionPageSelection() error = %v", err)
	}
	if encoded, _ := json.Marshal(pages); string(encoded) != `[2,3,5]` {
		t.Fatalf("normalized pages = %s", encoded)
	}
}

func TestNormalizeVisionPageSelectionDefaultsOnlyForBoundedDocuments(t *testing.T) {
	total := 3
	pages, err := normalizeVisionPageSelection(nil, &total)
	if err != nil {
		t.Fatalf("normalizeVisionPageSelection() error = %v", err)
	}
	if encoded, _ := json.Marshal(pages); string(encoded) != `[1,2,3]` {
		t.Fatalf("default pages = %s", encoded)
	}
}

func TestValidateDocumentVisionResultRejectsUnselectedAndDuplicatePages(t *testing.T) {
	base := DocumentVisionParseResult{ProviderCode: "las", ModelVersion: "vision-v1"}
	base.Pages = []DocumentVisionPage{{PageNumber: 3, Markdown: "outside selection"}}
	if err := validateDocumentVisionResult(base, []int{1, 2}); err == nil {
		t.Fatal("unselected provider page must be rejected")
	}
	base.Pages = []DocumentVisionPage{{PageNumber: 1, Markdown: "one"}, {PageNumber: 1, Markdown: "duplicate"}}
	if err := validateDocumentVisionResult(base, []int{1, 2}); err == nil {
		t.Fatal("duplicate provider page must be rejected")
	}
}

func TestValidateDocumentVisionResultRejectsOversizedOrUnsafeProviderOutput(t *testing.T) {
	result := DocumentVisionParseResult{
		ProviderCode: "las", ModelVersion: "vision-v1",
		Pages: []DocumentVisionPage{{PageNumber: 1, Markdown: strings.Repeat("a", maxDocumentVisionPageBytes+1)}},
	}
	if err := validateDocumentVisionResult(result, []int{1}); err == nil {
		t.Fatal("oversized page must be rejected")
	}
	result.Pages = []DocumentVisionPage{{
		PageNumber: 1, Markdown: "usable text",
		Locator: map[string]any{"source_url": "https://signed.example.test/document"},
	}}
	if err := validateDocumentVisionResult(result, []int{1}); err == nil {
		t.Fatal("provider locator with a URL must be rejected")
	}
}

func TestSanitizeDocumentVisionLocatorKeepsOnlyBoundedTraceabilityFields(t *testing.T) {
	locator, err := sanitizeDocumentVisionLocator(2, map[string]any{
		"page_number": 2,
		"section":     "market table",
		"bounding_boxes": []any{
			map[string]any{"kind": "table", "bbox": []any{0.1, 0.2, 0.8, 0.9}},
		},
	})
	if err != nil {
		t.Fatalf("sanitizeDocumentVisionLocator() error = %v", err)
	}
	if locator["page_number"] != 2 || locator["section"] != "market table" {
		t.Fatalf("sanitized locator = %#v", locator)
	}
}

func TestDocumentVisionSchedulerUsesDurableAttemptIdentityAndPollingBudget(t *testing.T) {
	store := &capturingKnowledgeJobStore{}
	now := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	document := Document{ID: "document_1", OrganizationID: "org_1", ProjectID: "project_1", UpdatedAt: now, DocumentVisionState: DocumentVisionState{VisionAttemptID: "vision_attempt_1"}}
	textScheduler := JobRuntimeDocumentParseScheduler{
		Store: store, NewID: func() (string, error) { return "parse_job_1", nil }, Now: func() time.Time { return now },
	}
	if err := textScheduler.ScheduleDocumentParse(context.Background(), document); err != nil {
		t.Fatalf("ScheduleDocumentParse() error = %v", err)
	}
	if store.last.Job.MaxAttempts != 2 || store.last.Job.Kind != DocumentParseJobKind {
		t.Fatalf("text parse job = %#v", store.last.Job)
	}
	visionScheduler := JobRuntimeDocumentVisionFallbackScheduler{
		Store: store, NewID: func() (string, error) { return "vision_job_1", nil }, Now: func() time.Time { return now },
	}
	if err := visionScheduler.ScheduleDocumentVisionFallback(context.Background(), document, []int{1, 3}, ""); err != nil {
		t.Fatalf("ScheduleDocumentVisionFallback() error = %v", err)
	}
	if store.last.Job.MaxAttempts != 1000 || store.last.Job.Kind != DocumentVisionFallbackJobKind || store.last.Job.Progress != 72 {
		t.Fatalf("vision fallback job = %#v", store.last.Job)
	}
	baseKey := store.last.IdempotencyKey
	if !strings.HasPrefix(string(baseKey), "knowledge_vision_") || len(baseKey) != len("knowledge_vision_")+64 {
		t.Fatalf("vision fallback idempotency key = %q", store.last.IdempotencyKey)
	}
	if err := visionScheduler.ScheduleDocumentVisionFallback(context.Background(), document, []int{1, 3}, "reconciliation_1"); err != nil {
		t.Fatalf("ScheduleDocumentVisionFallback() reconciliation error = %v", err)
	}
	if store.last.IdempotencyKey == baseKey || !strings.Contains(string(store.last.Payload), `"schedule_key":"reconciliation_1"`) {
		t.Fatalf("reconciliation schedule was not independently idempotent: key=%q payload=%s", store.last.IdempotencyKey, store.last.Payload)
	}
}

func TestSplitContiguousVisionPagesPreservesExactBillableSelection(t *testing.T) {
	groups := splitContiguousVisionPages([]int{1, 3, 4, 5, 8})
	encoded, _ := json.Marshal(groups)
	if string(encoded) != `[[1],[3,4,5],[8]]` {
		t.Fatalf("contiguous groups = %s", encoded)
	}
}

func TestDocumentVisionCapabilityRequiresExplicitMIMESupport(t *testing.T) {
	capability := DocumentVisionCapability{Available: true, SupportedMIMEs: []string{"application/pdf"}}
	if !documentVisionSupportsMIME(capability, "application/pdf") {
		t.Fatal("PDF should be supported by the LAS route")
	}
	if documentVisionSupportsMIME(capability, "application/vnd.openxmlformats-officedocument.presentationml.presentation") {
		t.Fatal("PPTX must remain unavailable until a converter is configured")
	}
}

func TestDocumentVisionConversionCoversLegacyAndOpenXMLPresentations(t *testing.T) {
	if !documentVisionNeedsConversion(PowerPointLegacyMIME) || !documentVisionNeedsConversion(PowerPointOpenXMLMIME) {
		t.Fatal("both .ppt and .pptx require the explicit PDF conversion boundary")
	}
	if documentVisionNeedsConversion("application/pdf") {
		t.Fatal("PDF must bypass the presentation converter")
	}
}

func TestDocumentVisionDerivedPDFKeyIsDeterministicAndAttemptScoped(t *testing.T) {
	document := Document{ID: "document_1", OrganizationID: "org_1", ProjectID: "project_1", ContentSHA256: strings.Repeat("a", 64)}
	first := documentVisionDerivedPDFKey(document, "attempt_1")
	if first != documentVisionDerivedPDFKey(document, "attempt_1") ||
		first == documentVisionDerivedPDFKey(document, "attempt_2") ||
		!strings.HasPrefix(first, "assets/org_1/project_1/knowledge/document_1/derived/document-vision/") ||
		!strings.HasSuffix(first, ".pdf") || strings.Contains(first, "attempt_1") {
		t.Fatalf("derived PDF key = %q", first)
	}
}

func TestDocumentVisionConversionLineageRejectsCrossBucketOrChangedSource(t *testing.T) {
	document := Document{
		ID: "document_1", OrganizationID: "org_1", ProjectID: "project_1",
		ContentSHA256: strings.Repeat("a", 64), MIMEType: PowerPointOpenXMLMIME,
		Blob:                assets.ObjectLocation{Provider: "tos", Bucket: "cookies", Key: "assets/org_1/project_1/knowledge/document_1/source.pptx", ETag: "source-etag"},
		DocumentVisionState: DocumentVisionState{VisionAttemptID: "attempt_1"},
	}
	conversion := documentVisionInputConversion{
		AttemptID: "attempt_1", SourceMIMEType: document.MIMEType, SourceSHA256: document.ContentSHA256,
		Source: document.Blob, ConverterCode: "gotenberg_libreoffice", ConverterVersion: "gotenberg-8.34.0",
		Derived: assets.ObjectLocation{Bucket: "cookies", Key: documentVisionDerivedPDFKey(document, "attempt_1")},
	}
	if err := validateDocumentVisionInputConversion(document, conversion, "cookies"); err != nil {
		t.Fatalf("valid conversion lineage rejected: %v", err)
	}
	conversion.Derived.Bucket = "other"
	if err := validateDocumentVisionInputConversion(document, conversion, "cookies"); err == nil {
		t.Fatal("cross-bucket derived input must be rejected")
	}
	conversion.Derived.Bucket = "cookies"
	conversion.Derived.Key = "assets/org_1/project_2/knowledge/document_1/derived/document-vision/x.pdf"
	if err := validateDocumentVisionInputConversion(document, conversion, "cookies"); err == nil {
		t.Fatal("cross-project derived key must be rejected")
	}
	conversion.Derived.Key = documentVisionDerivedPDFKey(document, "attempt_1")
	conversion.Source.ETag = "changed"
	if err := validateDocumentVisionInputConversion(document, conversion, "cookies"); err == nil {
		t.Fatal("changed source lineage must be rejected")
	}
}

func TestKnowledgeDocumentObjectScopeRejectsSiblingProjectsAndDocuments(t *testing.T) {
	document := Document{
		ID: "document_1", OrganizationID: "org_1", ProjectID: "project_1",
		Blob: assets.ObjectLocation{Bucket: "cookies", Key: "assets/org_1/project_1/knowledge/document_1/source.pdf"},
	}
	if !knowledgeDocumentBlobInScope(document, "cookies") {
		t.Fatal("valid knowledge document object scope rejected")
	}
	for _, key := range []string{
		"assets/org_1/project_2/knowledge/document_1/source.pdf",
		"assets/org_1/project_1/knowledge/document_10/source.pdf",
		"provider-output/org_1/project_1/object",
	} {
		document.Blob.Key = key
		if knowledgeDocumentBlobInScope(document, "cookies") {
			t.Fatalf("out-of-scope key accepted: %s", key)
		}
	}
}

func TestDocumentVisionUncertainSubmissionRequiresReconciliation(t *testing.T) {
	for _, code := range []string{
		"DOCUMENT_VISION_SUBMISSION_UNKNOWN",
		"DOCUMENT_VISION_SUBMISSION_INVALID",
		"DOCUMENT_VISION_CHECKPOINT_FAILED",
	} {
		if !documentVisionRequiresReconciliation(code) {
			t.Fatalf("%s should block a duplicate paid submission", code)
		}
	}
	if documentVisionRequiresReconciliation("DOCUMENT_VISION_UPSTREAM_FAILED") {
		t.Fatal("a confirmed upstream failure may be retried after explicit user action")
	}
}

func TestDocumentVisionReconciliationRequiresAuthorizedHuman(t *testing.T) {
	service := Service{}
	for _, actor := range []contract.ActorContext{
		{
			OrganizationID: "org_1",
			Principal:      contract.Principal{Kind: contract.PrincipalService, ID: "service_1"},
			Scopes:         []contract.Scope{ScopeDocumentVisionReconcile},
		},
		{
			OrganizationID: "org_1",
			Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "member_1"},
			Scopes:         []contract.Scope{ScopeRead},
		},
	} {
		_, err := service.ProposeDocumentVisionReconciliation(
			context.Background(), actor, "project_1", "document_1", ProposeDocumentVisionReconciliationRequest{},
		)
		if !errors.Is(err, ErrDocumentVisionReconciliationForbidden) {
			t.Fatalf("actor %#v error = %v", actor.Principal, err)
		}
	}
}

func TestDocumentVisionReconciliationRejectsUnsafeOperatorEvidence(t *testing.T) {
	service := Service{Projects: staticVisionReconciliationProjectReader{}}
	actor := contract.ActorContext{
		OrganizationID: "org_1",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "admin_1"},
		Scopes:         []contract.Scope{ScopeDocumentVisionReconcile},
	}
	for _, request := range []ProposeDocumentVisionReconciliationRequest{
		{TaskIndex: 0, ExpectedIntentID: strings.Repeat("a", 64), Decision: "accepted", ExternalTaskID: "task_1", EvidenceRef: "https://console.example/task_1"},
		{TaskIndex: 0, ExpectedIntentID: strings.Repeat("a", 64), Decision: "accepted", ExternalTaskID: "task_1", EvidenceRef: "token:secret-value"},
		{TaskIndex: 0, ExpectedIntentID: strings.Repeat("a", 64), Decision: "accepted", ExternalTaskID: "task id", EvidenceRef: "ticket:LAS-123"},
	} {
		_, err := service.ProposeDocumentVisionReconciliation(context.Background(), actor, "project_1", "document_1", request)
		if !errors.Is(err, ErrDocumentVisionReconciliationInvalid) {
			t.Fatalf("request %#v error = %v", request, err)
		}
	}
}

type staticVisionReconciliationProjectReader struct{}

func (staticVisionReconciliationProjectReader) GetContext(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID) (contract.ProjectContext, error) {
	return contract.ProjectContext{OrganizationID: actor.OrganizationID, ProjectID: projectID}, nil
}

type capturingKnowledgeJobStore struct {
	last jobruntime.CreateRequest
}

func (s *capturingKnowledgeJobStore) Enqueue(_ context.Context, request jobruntime.CreateRequest) (contract.Job, bool, error) {
	s.last = request
	return request.Job, false, nil
}
