package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifyMiyunConnectionUsesAcceptedProductProbe(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload struct {
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.Variables["order"] != "_score_desc" {
			t.Fatalf("order=%#v", payload.Variables["order"])
		}
		if materialIDs := payload.Variables["materialIds"]; materialIDs != nil {
			t.Fatalf("materialIds must be null, got %#v", materialIDs)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":{"leafletMaterialList":{"data":{"material":[]},"total":0,"limit":20,"maxTotal":0,"page":1}}}`))
	}))
	defer server.Close()

	adapter := miyunProtocolAdapter{endpoint: server.URL, client: server.Client()}
	if err := adapter.VerifyMiyunConnection(context.Background(), []byte("sessionId=test")); err != nil {
		t.Fatal(err)
	}
}
