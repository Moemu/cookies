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

func TestClientDerivesCSRFHeaderAndUsesBrowserUserAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "session=x; csrftoken=csrf" || r.Header.Get("x-csrftoken") != "csrf" {
			t.Errorf("session headers not derived from cookie: cookie=%q csrf=%q", r.Header.Get("Cookie"), r.Header.Get("x-csrftoken"))
		}
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "Mozilla/5.0") {
			t.Errorf("expected browser user agent, got %q", r.Header.Get("User-Agent"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	defer server.Close()
	client, err := NewSessionClient(server.URL, Session{Cookies: "session=x; csrftoken=csrf"}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	client.Delay = 0
	if _, err := client.AccountInfo(context.Background()); err != nil {
		t.Fatal(err)
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
