package knowledge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMCPStdioRunnerPerformsInitializeAndToolCall(t *testing.T) {
	t.Setenv("GO_WANT_MCP_HELPER", "1")
	runner := MCPStdioRunner{
		Command:         os.Args[0],
		Args:            []string{"-test.run=TestMCPHelperProcess", "--"},
		ToolName:        "research",
		ProtocolVersion: "2025-11-25",
		EnvAllowlist:    []string{"GO_WANT_MCP_HELPER"},
		Timeout:         5 * time.Second,
		MaxOutputBytes:  1024 * 1024,
	}
	results, err := runner.Run(context.Background(), ExternalResearchInput{
		Mode: "mcp", Query: "测试查询",
	})
	if err != nil {
		t.Fatalf("run MCP stdio client: %v", err)
	}
	if len(results) != 1 || results[0].Title != "helper evidence" {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestMCPHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_MCP_HELPER") != "1" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() || !strings.Contains(scanner.Text(), `"method":"initialize"`) {
		_, _ = fmt.Fprintln(os.Stdout, `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"missing initialize"}}`)
		os.Exit(0)
	}
	_, _ = fmt.Fprintln(os.Stdout, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25","capabilities":{},"serverInfo":{"name":"test","version":"1"}}}`)
	if !scanner.Scan() || !strings.Contains(scanner.Text(), `"method":"notifications/initialized"`) {
		os.Exit(2)
	}
	if !scanner.Scan() || !strings.Contains(scanner.Text(), `"method":"tools/call"`) ||
		!strings.Contains(scanner.Text(), `"测试查询"`) {
		_, _ = fmt.Fprintln(os.Stdout, `{"jsonrpc":"2.0","id":2,"error":{"code":-32602,"message":"bad arguments"}}`)
		os.Exit(0)
	}
	_, _ = fmt.Fprintln(os.Stdout, `{"jsonrpc":"2.0","id":2,"result":{"structuredContent":{"results":[{"title":"helper evidence","content":"verified","citations":[]}]}}}`)
	os.Exit(0)
}

func TestParseMCPToolResultsAcceptsStructuredEvidence(t *testing.T) {
	raw := json.RawMessage(`{
		"structuredContent": {
			"results": [{
				"title": "竞品投放观察",
				"source_url": "https://example.test/report",
				"content": "近七日短视频素材强调价格锚点。",
				"citations": ["https://example.test/report"]
			}]
		}
	}`)
	results, err := parseMCPToolResults(raw)
	if err != nil {
		t.Fatalf("parse structured result: %v", err)
	}
	if len(results) != 1 || results[0].Title != "竞品投放观察" || len(results[0].Citations) != 1 {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestParseMCPToolResultsFallsBackToPlainText(t *testing.T) {
	raw := json.RawMessage(`{"content":[{"type":"text","text":"一条可追溯的研究摘要"}]}`)
	results, err := parseMCPToolResults(raw)
	if err != nil {
		t.Fatalf("parse plain text result: %v", err)
	}
	if len(results) != 1 || results[0].Content != "一条可追溯的研究摘要" {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestParseMCPToolResultsRejectsToolError(t *testing.T) {
	raw := json.RawMessage(`{"isError":true,"content":[{"type":"text","text":"upstream unavailable"}]}`)
	if _, err := parseMCPToolResults(raw); err == nil {
		t.Fatal("expected MCP tool error")
	}
}
