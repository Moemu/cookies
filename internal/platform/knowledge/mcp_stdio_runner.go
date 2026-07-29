package knowledge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// MCPStdioRunner is a backend-only MCP client. It starts one isolated server
// process per research run so protocol state, cancellation, and disclosure are
// bounded to that explicitly confirmed request.
type MCPStdioRunner struct {
	Command         string
	Args            []string
	ToolName        string
	ProtocolVersion string
	EnvAllowlist    []string
	Timeout         time.Duration
	MaxOutputBytes  int
}

type mcpRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpError struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type mcpToolResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	} `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent,omitempty"`
	IsError           bool            `json:"isError,omitempty"`
}

func (r MCPStdioRunner) Run(ctx context.Context, input ExternalResearchInput) ([]ExternalResearchResult, error) {
	if strings.TrimSpace(r.Command) == "" || strings.TrimSpace(r.ToolName) == "" {
		return nil, ErrExternalRunnerUnavailable
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	maxOutput := r.MaxOutputBytes
	if maxOutput <= 0 {
		maxOutput = 4 * 1024 * 1024
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	command := exec.CommandContext(runCtx, r.Command, r.Args...)
	command.Env = allowedProcessEnvironment(r.EnvAllowlist)
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create MCP stdin: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create MCP stdout: %w", err)
	}
	stderr := &boundedBuffer{limit: 32 * 1024}
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start MCP server: %w", err)
	}
	defer func() {
		_ = stdin.Close()
		if command.Process != nil {
			_ = command.Process.Kill()
		}
		_ = command.Wait()
	}()

	encoder := json.NewEncoder(stdin)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), maxOutput)
	protocolVersion := strings.TrimSpace(r.ProtocolVersion)
	if protocolVersion == "" {
		protocolVersion = "2025-11-25"
	}
	if err := encoder.Encode(mcpRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{},
			"clientInfo": map[string]string{
				"name": "cookies-knowledge", "version": "1.0.0",
			},
		},
	}); err != nil {
		return nil, fmt.Errorf("send MCP initialize: %w", err)
	}
	if _, err := readMCPResponse(scanner, 1, stderr); err != nil {
		return nil, fmt.Errorf("initialize MCP server: %w", err)
	}
	if err := encoder.Encode(mcpRequest{
		JSONRPC: "2.0", Method: "notifications/initialized",
	}); err != nil {
		return nil, fmt.Errorf("send MCP initialized notification: %w", err)
	}
	if err := encoder.Encode(mcpRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: map[string]any{
			"name": r.ToolName,
			"arguments": map[string]any{
				"mode": input.Mode, "query": input.Query, "documents": input.Documents,
			},
		},
	}); err != nil {
		return nil, fmt.Errorf("call MCP research tool: %w", err)
	}
	raw, err := readMCPResponse(scanner, 2, stderr)
	if err != nil {
		return nil, fmt.Errorf("read MCP research result: %w", err)
	}
	return parseMCPToolResults(raw)
}

func readMCPResponse(scanner *bufio.Scanner, id int64, stderr *boundedBuffer) (json.RawMessage, error) {
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var response mcpResponse
		if err := json.Unmarshal(line, &response); err != nil {
			return nil, fmt.Errorf("MCP stdout must contain newline-delimited JSON-RPC only: %w", err)
		}
		if response.ID != id {
			continue
		}
		if response.Error != nil {
			return nil, fmt.Errorf("MCP error %d: %s", response.Error.Code, response.Error.Message)
		}
		return response.Result, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan MCP stdout: %w", err)
	}
	detail := strings.TrimSpace(stderr.String())
	if detail != "" {
		return nil, fmt.Errorf("MCP server exited before response: %s", detail)
	}
	return nil, io.ErrUnexpectedEOF
}

func parseMCPToolResults(raw json.RawMessage) ([]ExternalResearchResult, error) {
	var toolResult mcpToolResult
	if err := json.Unmarshal(raw, &toolResult); err != nil {
		return nil, fmt.Errorf("decode MCP tool result: %w", err)
	}
	if toolResult.IsError {
		detail := make([]string, 0, len(toolResult.Content))
		for _, item := range toolResult.Content {
			if item.Type == "text" && strings.TrimSpace(item.Text) != "" {
				detail = append(detail, strings.TrimSpace(item.Text))
			}
		}
		if len(detail) == 0 {
			return nil, errors.New("MCP research tool returned an error")
		}
		return nil, fmt.Errorf("MCP research tool returned an error: %s", strings.Join(detail, "\n"))
	}
	if len(toolResult.StructuredContent) > 0 && string(toolResult.StructuredContent) != "null" {
		if results, ok := decodeExternalResearchResults(toolResult.StructuredContent); ok {
			return results, nil
		}
	}
	results := []ExternalResearchResult{}
	for _, item := range toolResult.Content {
		if item.Type != "text" || strings.TrimSpace(item.Text) == "" {
			continue
		}
		text := strings.TrimSpace(item.Text)
		if decoded, ok := decodeExternalResearchResults(json.RawMessage(text)); ok {
			results = append(results, decoded...)
			continue
		}
		results = append(results, ExternalResearchResult{
			Title: "MCP 研究结果", Content: text, Citations: []string{},
		})
	}
	if len(results) == 0 {
		return nil, errors.New("MCP research tool returned no text or structured evidence")
	}
	return results, nil
}

func decodeExternalResearchResults(raw json.RawMessage) ([]ExternalResearchResult, bool) {
	var values []ExternalResearchResult
	if err := json.Unmarshal(raw, &values); err == nil && len(values) > 0 {
		return values, true
	}
	var wrapper struct {
		Results []ExternalResearchResult `json:"results"`
	}
	if err := json.Unmarshal(raw, &wrapper); err == nil && len(wrapper.Results) > 0 {
		return wrapper.Results, true
	}
	var value ExternalResearchResult
	if err := json.Unmarshal(raw, &value); err == nil &&
		strings.TrimSpace(value.Title) != "" && strings.TrimSpace(value.Content) != "" {
		return []ExternalResearchResult{value}, true
	}
	return nil, false
}

func allowedProcessEnvironment(allowlist []string) []string {
	result := make([]string, 0, len(allowlist))
	seen := map[string]struct{}{}
	for _, name := range allowlist {
		name = strings.TrimSpace(name)
		key := strings.ToUpper(name)
		if name == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		if value, ok := os.LookupEnv(name); ok {
			result = append(result, name+"="+value)
		}
	}
	return result
}

type boundedBuffer struct {
	mu    sync.Mutex
	value bytes.Buffer
	limit int
}

func (b *boundedBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.limit - b.value.Len()
	if remaining > 0 {
		if remaining > len(value) {
			remaining = len(value)
		}
		_, _ = b.value.Write(value[:remaining])
	}
	return len(value), nil
}

func (b *boundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.value.String()
}
