package oceanengine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientRejectsNonReadOnlyEndpoint(t *testing.T) {
	client, err := NewClient("https://example.test", "123", Session{Cookies: "session=x"}, http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.do(context.Background(), http.MethodPost, "/ad/api/promotion/ads/update", strings.NewReader(`{}`), "application/json")
	if err == nil || !strings.Contains(err.Error(), ErrForbiddenEndpoint.Error()) {
		t.Fatalf("expected forbidden endpoint, got %v", err)
	}
}

func TestClientSetsSessionHeadersAndChecksBusinessCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/ad/api/account/info" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("aadvid") != "123" {
			t.Errorf("missing advertiser ID")
		}
		if r.Header.Get("Cookie") != "session=x" || r.Header.Get("x-csrftoken") != "csrf" {
			t.Errorf("session headers not forwarded")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"data":{"currency":"CNY"}}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "123", Session{Cookies: "session=x", CSRFToken: "csrf"}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	client.Delay = 0
	payload, err := client.do(context.Background(), http.MethodGet, "/ad/api/account/info", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if payload["code"] != float64(0) {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestClientMapsBusinessFailureToSessionInvalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":401,"msg":"expired"}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "123", Session{Cookies: "session=x"}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	client.Delay = 0
	_, err = client.do(context.Background(), http.MethodGet, "/ad/api/account/info", nil, "")
	if err == nil || !strings.Contains(err.Error(), ErrSessionInvalid.Error()) {
		t.Fatalf("expected session invalid, got %v", err)
	}
}
