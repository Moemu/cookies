package knowledge

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestExtractMarkdownAndDOCX(t *testing.T) {
	t.Parallel()
	markdown, mimeType, err := extractDocument(".md", []byte("\xef\xbb\xbf# 标题\n\n正文"))
	if err != nil || markdown != "# 标题\n\n正文" || mimeType != "text/markdown" {
		t.Fatalf("markdown extraction = %q, %q, %v", markdown, mimeType, err)
	}

	var content bytes.Buffer
	writer := zip.NewWriter(&content)
	part, err := writer.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte(`<?xml version="1.0"?><w:document xmlns:w="urn:w"><w:body><w:p><w:r><w:t>第一段</w:t></w:r></w:p><w:p><w:r><w:t>第二段</w:t></w:r></w:p></w:body></w:document>`))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	docx, mimeType, err := extractDocument(".docx", content.Bytes())
	if err != nil || docx != "第一段\n第二段" ||
		mimeType != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		t.Fatalf("docx extraction = %q, %q, %v", docx, mimeType, err)
	}
}

func TestExtractDocumentRejectsUnsupportedOrMalformedContent(t *testing.T) {
	t.Parallel()
	if _, _, err := extractDocument(".pdf", []byte("pdf")); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("unsupported extension error = %v", err)
	}
	if _, _, err := extractDocument(".docx", []byte("not a zip")); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("malformed docx error = %v", err)
	}
}

func TestHTMLDocumentsUseTheBoundedTikaTextPath(t *testing.T) {
	t.Parallel()
	for _, extension := range []string{".html", ".htm"} {
		if !allowedMIME(extension, "text/html; charset=utf-8") ||
			!allowedMIME(extension, "application/xhtml+xml") ||
			defaultDocumentMIME(extension) != "text/html" ||
			documentParseStrategy(extension) != "tika_text" {
			t.Fatalf("HTML routing is incomplete for %s", extension)
		}
	}
	if allowedMIME(".html", "image/svg+xml") {
		t.Fatal("HTML extension accepted an unrelated active-content MIME")
	}
}

func TestPDFContainerValidationRejectsRenamedContent(t *testing.T) {
	t.Parallel()
	if !validPDFContainer([]byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\n%%EOF\n")) {
		t.Fatal("expected a bounded PDF container to pass admission")
	}
	for _, content := range [][]byte{[]byte("plain text renamed.pdf"), []byte("%PDF-1.7\nmissing eof")} {
		if validPDFContainer(content) {
			t.Fatalf("renamed or truncated PDF passed admission: %q", content)
		}
	}
}

func TestVerifyDocumentOriginalRejectsSubstitution(t *testing.T) {
	t.Parallel()
	content := []byte("original")
	sum := sha256.Sum256(content)
	location := assets.ObjectLocation{Provider: "memory", Bucket: "knowledge", Key: "doc/source.txt", ETag: "etag"}
	document := Document{
		MIMEType: "text/plain", SizeBytes: int64(len(content)), ContentSHA256: hex.EncodeToString(sum[:]), Blob: location,
	}
	info := assets.ObjectInfo{ObjectLocation: location, SizeBytes: int64(len(content)), MIMEType: "text/plain"}
	if verified, err := verifyDocumentOriginal(bytes.NewReader(content), info, document); err != nil || !bytes.Equal(verified, content) {
		t.Fatalf("verified=%q err=%v", verified, err)
	}
	if _, err := verifyDocumentOriginal(bytes.NewReader([]byte("changed!")), info, document); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("same-size substitution error=%v", err)
	}
	info.MIMEType = "application/pdf"
	if _, err := verifyDocumentOriginal(bytes.NewReader(content), info, document); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("MIME substitution error=%v", err)
	}
}

func TestResearchRequiresPerCallConfirmationBeforeDatabaseOrRunner(t *testing.T) {
	t.Parallel()
	service := Service{Projects: allowingProjects{}}
	actor := contract.ActorContext{
		OrganizationID: "org_1",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
	}
	_, err := service.RunResearch(context.Background(), actor, "project_1", ResearchRequest{
		Mode: "web", Query: "行业案例", Confirmed: false,
	})
	if !errors.Is(err, ErrExternalConfirmationRequired) {
		t.Fatalf("unconfirmed research error = %v", err)
	}
}

func TestResearchDisclosureMustExactlyMatchPayload(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		request   ResearchRequest
		wantError bool
	}{
		{
			name: "query only",
			request: ResearchRequest{
				Mode: "web", Query: "行业案例", DisclosedFields: []string{"query"},
			},
		},
		{
			name: "document content declared",
			request: ResearchRequest{
				Mode: "web", Query: "竞品研究", DocumentIDs: []string{"doc_1"},
				DisclosedFields: []string{"query", "document_content"},
			},
		},
		{
			name: "document content omitted",
			request: ResearchRequest{
				Mode: "web", Query: "行业案例", DocumentIDs: []string{"doc_1"},
				DisclosedFields: []string{"query"},
			},
			wantError: true,
		},
		{
			name: "mcp is not a public research mode",
			request: ResearchRequest{
				Mode: "mcp", Query: "竞品研究", DisclosedFields: []string{"query"},
			},
			wantError: true,
		},
		{
			name: "disclosure overstates payload",
			request: ResearchRequest{
				Mode: "web", Query: "行业案例",
				DisclosedFields: []string{"query", "document_content"},
			},
			wantError: true,
		},
		{
			name: "unknown disclosure field",
			request: ResearchRequest{
				Mode: "web", Query: "行业案例", DisclosedFields: []string{"query", "cookies"},
			},
			wantError: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := validateResearchRequest(test.request)
			if test.wantError != errors.Is(err, ErrInvalidResearchRequest) {
				t.Fatalf("error = %v, want invalid=%t", err, test.wantError)
			}
		})
	}
}

func TestResearchPurposeSeparatesConversationSearchFromDeepResearch(t *testing.T) {
	t.Parallel()
	messageRef := &contract.ResourceRef{Type: "strategy_message", ID: "message_1"}
	workspaceRef := &contract.ResourceRef{Type: "strategy_workspace", ID: "workspace_1"}
	tests := []struct {
		name        string
		purpose     string
		sourceRef   *contract.ResourceRef
		wantPurpose string
		wantError   bool
	}{
		{name: "deep research binds one workspace", sourceRef: workspaceRef, wantPurpose: "deep_research"},
		{name: "explicit deep research binds one workspace", purpose: "deep_research", sourceRef: workspaceRef, wantPurpose: "deep_research"},
		{name: "deep research requires workspace", purpose: "deep_research", wantError: true},
		{name: "conversation search binds one message", purpose: "conversation_web_search", sourceRef: messageRef, wantPurpose: "conversation_web_search"},
		{name: "conversation search requires message", purpose: "conversation_web_search", wantError: true},
		{name: "deep research rejects message source", purpose: "deep_research", sourceRef: messageRef, wantError: true},
		{name: "unknown purpose is rejected", purpose: "chat_research", wantError: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			purpose, sourceRef, err := validateResearchContext(test.purpose, test.sourceRef)
			if test.wantError != errors.Is(err, ErrInvalidResearchRequest) {
				t.Fatalf("error = %v, want invalid=%t", err, test.wantError)
			}
			if test.wantError {
				return
			}
			if purpose != test.wantPurpose {
				t.Fatalf("purpose = %q, want %q", purpose, test.wantPurpose)
			}
			if test.sourceRef != nil && (sourceRef == nil || sourceRef.ID != test.sourceRef.ID) {
				t.Fatalf("source ref = %#v", sourceRef)
			}
		})
	}
}

func TestResearchCategoryIsExplicitAndBounded(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "general", "audience", "competitor", "industry", " Audience "} {
		if !validResearchCategory(value, true) {
			t.Fatalf("category %q should be valid", value)
		}
	}
	for _, value := range []string{"creative", "brand", "unknown"} {
		if validResearchCategory(value, true) {
			t.Fatalf("category %q should be rejected", value)
		}
	}
	if normalizedResearchCategory("") != "general" ||
		normalizedResearchCategory(" Competitor ") != "competitor" {
		t.Fatal("research category normalization changed unexpectedly")
	}
}

type allowingProjects struct{}

func (allowingProjects) GetContext(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID) (contract.ProjectContext, error) {
	return contract.ProjectContext{OrganizationID: actor.OrganizationID, ProjectID: projectID, ProjectContextVersion: 1}, nil
}
