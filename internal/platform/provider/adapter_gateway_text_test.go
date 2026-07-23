package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestAdapterGatewayTextProducesStructuredOutput(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" || request.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("unexpected request path=%q auth=%q", request.URL.Path, request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"model":"text-v2","usage":{"prompt_tokens":12,"completion_tokens":4,"total_tokens":16},"choices":[{"message":{"content":"{\"operations\":[]}"}}]}`))
	}))
	defer server.Close()
	adapter, err := NewAdapterGatewayTextAdapter(
		textRouteStub{snapshot: textRouteSnapshot(server.URL)},
		credentialStub("test-token"),
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	adapter.client = server.Client()
	result, err := adapter.GenerateText(context.Background(), TextAdapterRequest{
		OrganizationID: "org_1", ModelAlias: "cookies.text.standard",
		Messages:         []TextMessage{{Role: TextRoleUser, Content: "extract"}},
		OutputJSONSchema: []byte(`{"type":"object"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ModelVersion != "text-v2" || string(result.StructuredOutput) != `{"operations":[]}` || result.Usage == nil || result.Usage.TotalTokens != 16 {
		t.Fatalf("result = %#v", result)
	}
}

func TestAdapterGatewayTextRejectsOversizedResponse(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(strings.Repeat("x", 40)))
	}))
	defer server.Close()
	snapshot := textRouteSnapshot(server.URL)
	snapshot.MaxResponseBytes = 16
	adapter, _ := NewAdapterGatewayTextAdapter(textRouteStub{snapshot: snapshot}, credentialStub("token"), false)
	adapter.client = server.Client()
	_, err := adapter.GenerateText(context.Background(), TextAdapterRequest{
		OrganizationID: "org_1", ModelAlias: "model",
		Messages: []TextMessage{{Role: TextRoleUser, Content: "test"}},
	})
	if err == nil {
		t.Fatal("expected oversized response rejection")
	}
}

func TestAdapterGatewayTextForwardsInvocationKey(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Idempotency-Key"); got != "agent-task-brief" {
			t.Fatalf("Idempotency-Key = %q", got)
		}
		_, _ = writer.Write([]byte(`{"model":"text-v2","choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()
	adapter, _ := NewAdapterGatewayTextAdapter(textRouteStub{snapshot: textRouteSnapshot(server.URL)}, credentialStub("token"), false)
	adapter.client = server.Client()
	_, err := adapter.GenerateText(context.Background(), TextAdapterRequest{
		OrganizationID: "org_1", ModelAlias: "model", InvocationKey: "agent-task-brief",
		Messages: []TextMessage{{Role: TextRoleUser, Content: "test"}},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAdapterGatewayTextReturnsStableRefusalCode(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"model":"text-v2","choices":[{"message":{"refusal":"policy"}}]}`))
	}))
	defer server.Close()
	adapter, _ := NewAdapterGatewayTextAdapter(textRouteStub{snapshot: textRouteSnapshot(server.URL)}, credentialStub("token"), false)
	adapter.client = server.Client()
	_, err := adapter.GenerateText(context.Background(), TextAdapterRequest{
		OrganizationID: "org_1", ModelAlias: "model",
		Messages: []TextMessage{{Role: TextRoleUser, Content: "test"}},
	})
	var executionError ExecutionError
	if !errors.As(err, &executionError) || executionError.JobError.Code != "MODEL_REFUSED" {
		t.Fatalf("error = %#v", err)
	}
}

type textRouteStub struct {
	snapshot ImageRouteSnapshot
	err      error
}

func (s textRouteStub) ResolveTextRoute(context.Context, contract.OrganizationID, string) (ImageRouteSnapshot, error) {
	return s.snapshot, s.err
}

type credentialStub string

func (s credentialStub) ResolveGatewayCredential(context.Context, string, int64) (string, error) {
	if s == "" {
		return "", fmt.Errorf("missing")
	}
	return string(s), nil
}

func textRouteSnapshot(baseURL string) ImageRouteSnapshot {
	return ImageRouteSnapshot{
		RouteID: "route_1", RouteRevisionID: "routerev_1", ConnectionID: "connection_1",
		ConnectionRevisionID: "connectionrev_1", BaseURL: baseURL, UpstreamModel: "text-v1",
		CredentialID: "credential_1", CredentialVersion: 1, TimeoutSeconds: 5, MaxResponseBytes: 1 << 20,
	}
}
