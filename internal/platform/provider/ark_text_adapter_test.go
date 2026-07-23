package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestArkTextAdapterSendsMessagesAndNormalizesTextResponse(t *testing.T) {
	t.Parallel()
	adapter, err := NewArkTextAdapter(ArkTextConfig{
		APIKey:  "test-key",
		Model:   "doubao-test",
		BaseURL: "https://ark.example.test/api/v3",
	})
	if err != nil {
		t.Fatalf("NewArkTextAdapter() error = %v", err)
	}
	adapter.client = &http.Client{Transport: roundTripper(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/v3/chat/completions" {
			t.Fatalf("unexpected Ark request: %s %s", request.Method, request.URL)
		}
		if got, want := request.Header.Get("Authorization"), "Bearer test-key"; got != want {
			t.Fatalf("Authorization = %q, want %q", got, want)
		}
		var body struct {
			Model    string        `json:"model"`
			Messages []TextMessage `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.Model != "doubao-test" || len(body.Messages) != 2 || body.Messages[0].Role != TextRoleSystem || body.Messages[1].Content != "Write a slogan." {
			t.Fatalf("unexpected request body: %#v", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{"model":"doubao-test-202607","choices":[{"message":{"content":"Fresh choices, simply made."}}]}`)),
		}, nil
	})}

	result, err := adapter.GenerateText(context.Background(), TextAdapterRequest{
		ModelAlias: "cookies.text.standard",
		Messages: []TextMessage{
			{Role: TextRoleSystem, Content: "You write concise advertising copy."},
			{Role: TextRoleUser, Content: "Write a slogan."},
		},
	})
	if err != nil {
		t.Fatalf("GenerateText() error = %v", err)
	}
	if result.ProviderCode != arkProviderCode || result.ModelVersion != "doubao-test-202607" || result.Text != "Fresh choices, simply made." || len(result.StructuredOutput) != 0 {
		t.Fatalf("unexpected normalized result: %#v", result)
	}
}

func TestArkTextAdapterRequestsAndReturnsStructuredOutput(t *testing.T) {
	t.Parallel()
	adapter, err := NewArkTextAdapter(ArkTextConfig{APIKey: "test-key", Model: "doubao-test", BaseURL: "https://ark.example.test"})
	if err != nil {
		t.Fatalf("NewArkTextAdapter() error = %v", err)
	}
	adapter.client = &http.Client{Transport: roundTripper(func(request *http.Request) (*http.Response, error) {
		var body struct {
			ResponseFormat *struct {
				Type       string `json:"type"`
				JSONSchema struct {
					Schema json.RawMessage `json:"schema"`
				} `json:"json_schema"`
			} `json:"response_format"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.ResponseFormat == nil || body.ResponseFormat.Type != "json_schema" || !json.Valid(body.ResponseFormat.JSONSchema.Schema) {
			t.Fatalf("unexpected response format: %#v", body.ResponseFormat)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{"choices":[{"message":{"content":"{\"headline\":\"Fresh choices\"}"}}]}`)),
		}, nil
	})}

	result, err := adapter.GenerateText(context.Background(), TextAdapterRequest{
		ModelAlias:       "cookies.text.standard",
		Messages:         []TextMessage{{Role: TextRoleUser, Content: "Return JSON."}},
		OutputJSONSchema: json.RawMessage(`{"type":"object","properties":{"headline":{"type":"string"}},"required":["headline"]}`),
	})
	if err != nil {
		t.Fatalf("GenerateText() error = %v", err)
	}
	if got, want := string(result.StructuredOutput), `{"headline":"Fresh choices"}`; got != want || result.Text != "" || result.ModelVersion != "doubao-test" {
		t.Fatalf("unexpected structured result: %#v", result)
	}
}

func TestArkTextAdapterRejectsInvalidResponseWithoutExposingCredential(t *testing.T) {
	t.Parallel()
	adapter, err := NewArkTextAdapter(ArkTextConfig{APIKey: "secret-key", Model: "doubao-test", BaseURL: "https://ark.example.test"})
	if err != nil {
		t.Fatalf("NewArkTextAdapter() error = %v", err)
	}
	adapter.client = &http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{"error":"invalid key"}`)),
		}, nil
	})}
	_, err = adapter.GenerateText(context.Background(), TextAdapterRequest{
		ModelAlias: "cookies.text.standard",
		Messages:   []TextMessage{{Role: TextRoleUser, Content: "Write a slogan."}},
	})
	if err == nil || bytes.Contains([]byte(err.Error()), []byte("secret-key")) {
		t.Fatalf("GenerateText() error = %v, want sanitized rejection", err)
	}
}
