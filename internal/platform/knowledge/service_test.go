package knowledge

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"testing"

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
				Mode: "mcp", Query: "竞品研究", DocumentIDs: []string{"doc_1"},
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

type allowingProjects struct{}

func (allowingProjects) GetContext(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID) (contract.ProjectContext, error) {
	return contract.ProjectContext{OrganizationID: actor.OrganizationID, ProjectID: projectID, ProjectContextVersion: 1}, nil
}
