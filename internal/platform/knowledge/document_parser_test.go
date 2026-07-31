package knowledge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTikaParserUsesBoundedRecursiveMetadataEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPut || request.URL.Path != "/rmeta/text" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("maxEmbeddedResources") != "0" {
			t.Fatalf("maxEmbeddedResources = %q", request.Header.Get("maxEmbeddedResources"))
		}
		if request.Header.Get("writeLimit") != "4096" {
			t.Fatalf("writeLimit = %q", request.Header.Get("writeLimit"))
		}
		if request.Header.Get("Content-Type") != "application/pdf" {
			t.Fatalf("Content-Type = %q", request.Header.Get("Content-Type"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`[{"X-TIKA:content":"第一段\n\n第二段","Content-Type":"application/pdf"}]`))
	}))
	defer server.Close()

	parsed, err := (TikaParser{
		BaseURL: server.URL, Version: "3.2.3.0",
		Timeout: time.Second, MaxOutputBytes: 4096,
	}).Parse(context.Background(), DocumentParseRequest{
		Filename: "brief.pdf", MIMEType: "application/pdf",
		Size: 4, Source: strings.NewReader("test"),
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.Text != "第一段\n\n第二段" || parsed.ParserCode != "tika" || parsed.ParserVersion != "3.2.3.0" {
		t.Fatalf("parsed = %#v", parsed)
	}
}

func TestChunksForParsedDocumentAreDeterministicAndBounded(t *testing.T) {
	document := Document{
		ID: "doc_1", OrganizationID: "org_1", ProjectID: "project_1",
		ContentSHA256: strings.Repeat("a", 64), UpdatedAt: time.Unix(1, 0).UTC(),
	}
	parsed := ParsedDocument{
		Text:       strings.Repeat("市场信号 ", 180) + "\n" + strings.Repeat("品牌事实 ", 180),
		ParserCode: "tika", ParserVersion: "3.2.3.0",
	}
	first := chunksForParsedDocument(document, parsed)
	second := chunksForParsedDocument(document, parsed)
	if len(first) < 2 || len(first) != len(second) {
		t.Fatalf("chunk counts = %d and %d", len(first), len(second))
	}
	for index := range first {
		if first[index].ID != second[index].ID || first[index].TextSHA256 != second[index].TextSHA256 {
			t.Fatalf("chunk %d is not deterministic", index)
		}
		if first[index].ParserCode != "tika" || first[index].Locator["start_line"] == nil {
			t.Fatalf("chunk %d metadata = %#v", index, first[index])
		}
		if len([]rune(first[index].Text)) > 800 {
			t.Fatalf("chunk %d has %d runes", index, len([]rune(first[index].Text)))
		}
	}
}
