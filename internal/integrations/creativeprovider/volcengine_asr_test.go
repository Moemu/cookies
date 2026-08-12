package creativeprovider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVolcengineASRTranscribeUsesReusableClient(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Api-Key") != "test-key" || request.Header.Get("X-Api-Resource-Id") != "resource" {
			t.Fatalf("ASR headers=%v", request.Header)
		}
		writer.Header().Set("X-Api-Status-Code", "20000000")
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"result":{"text":"测试转写"}}`))
	}))
	defer server.Close()
	client := VolcengineASR{Config: ASRConfig{
		Endpoint: server.URL, AuthMode: "api_key", APIKey: "test-key", ResourceID: "resource", Model: "bigmodel",
	}, Client: server.Client()}
	text, err := client.Transcribe(context.Background(), []byte("wav"))
	if err != nil || text != "测试转写" {
		t.Fatalf("text=%q err=%v", text, err)
	}
}
