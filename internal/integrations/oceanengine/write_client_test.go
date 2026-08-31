package oceanengine

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestWriteClientUsesProtectedPostPathForHEADAndCachesToken(t *testing.T) {
	var heads, posts atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "session=secret" {
			t.Error("session cookie missing")
		}
		switch r.Method {
		case http.MethodHead:
			heads.Add(1)
			if r.URL.Path != ProjectCreatePath || r.Header.Get("x-secsdk-csrf-request") != "1" || r.Header.Get("x-secsdk-csrf-version") != SecSDKVersion {
				t.Errorf("invalid HEAD contract: %s %#v", r.URL.Path, r.Header)
			}
			w.Header().Set("x-ware-csrf-token", "0,token-value,60000,ok,session")
		case http.MethodPost:
			posts.Add(1)
			if r.URL.Path != ProjectCreatePath || r.Header.Get("x-secsdk-csrf-token") != "token-value" {
				t.Errorf("invalid POST contract: %s %#v", r.URL.Path, r.Header)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"data":{"id":"123"}}`))
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()
	client, err := NewWriteClient(server.URL, "10001", 7, Session{Cookies: "session=secret"}, server.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = client.SubmitJSON(context.Background(), ProjectCreatePath, map[string]any{"name": "redacted"}); err != nil {
			t.Fatal(err)
		}
	}
	if heads.Load() != 1 || posts.Load() != 2 {
		t.Fatalf("HEAD=%d POST=%d", heads.Load(), posts.Load())
	}
}

func TestWriteClientRejectsInvalidAndDowngradeToken(t *testing.T) {
	for _, header := range []string{"", "1,token,60000,error", "0,,60000,ok", "0,DOWNGRADE,60000,ok", "0,token,0,ok"} {
		t.Run(header, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Header().Set("x-ware-csrf-token", header) }))
			defer server.Close()
			client, _ := NewWriteClient(server.URL, "10001", 1, Session{Cookies: "session=secret"}, server.Client(), nil)
			_, err := client.SubmitJSON(context.Background(), ProjectCreatePath, map[string]any{})
			if !errors.Is(err, ErrCSRFTokenInvalid) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestWriteClientDoesNotRetryUnknownPost(t *testing.T) {
	var posts atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("x-ware-csrf-token", "0,token,60000,ok")
			return
		}
		posts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client, _ := NewWriteClient(server.URL, "10001", 1, Session{Cookies: "session=secret"}, server.Client(), nil)
	_, err := client.SubmitJSON(context.Background(), PromotionCreatePath, json.RawMessage(`{"name":"redacted"}`))
	if !errors.Is(err, ErrResultUnknown) || posts.Load() != 1 {
		t.Fatalf("error=%v posts=%d", err, posts.Load())
	}
}

func TestWriteClientCacheKeyIncludesAccountAndSessionVersion(t *testing.T) {
	var heads atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			heads.Add(1)
			w.Header().Set("x-ware-csrf-token", "0,token,60000,ok")
			return
		}
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	defer server.Close()
	cache := NewCSRFTokenCache()
	now := time.Now
	for _, item := range []struct {
		account string
		version int64
	}{{"1", 1}, {"2", 1}, {"1", 2}} {
		client, _ := NewWriteClient(server.URL, item.account, item.version, Session{Cookies: "secret"}, server.Client(), cache)
		client.Now = now
		if _, err := client.SubmitJSON(context.Background(), ProjectCreatePath, map[string]any{}); err != nil {
			t.Fatal(err)
		}
	}
	if heads.Load() != 3 {
		t.Fatalf("HEAD=%d", heads.Load())
	}
}

func TestWriteClientRefreshesExpiredToken(t *testing.T) {
	var heads atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			heads.Add(1)
			w.Header().Set("x-ware-csrf-token", "0,token,1000,ok")
			return
		}
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	defer server.Close()
	client, _ := NewWriteClient(server.URL, "1", 1, Session{Cookies: "secret"}, server.Client(), nil)
	now := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	client.Now = func() time.Time { return now }
	if _, err := client.SubmitJSON(context.Background(), ProjectCreatePath, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Second)
	if _, err := client.SubmitJSON(context.Background(), ProjectCreatePath, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if heads.Load() != 2 {
		t.Fatalf("HEAD=%d", heads.Load())
	}
}

func TestWriteClientMarksAuthenticationRequired(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer server.Close()
	client, _ := NewWriteClient(server.URL, "1", 1, Session{Cookies: "secret"}, server.Client(), nil)
	_, err := client.SubmitJSON(context.Background(), ProjectCreatePath, map[string]any{})
	if !errors.Is(err, ErrAuthRequired) {
		t.Fatalf("error=%v", err)
	}
}

func TestWriteClientErrorsDoNotExposeSessionOrAccount(t *testing.T) {
	client, err := NewWriteClient("https://127.0.0.1:1", "sensitive-account", 1, Session{Cookies: "sensitive-cookie"}, &http.Client{Timeout: 50 * time.Millisecond}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.SubmitJSON(context.Background(), ProjectCreatePath, map[string]any{})
	message := err.Error()
	if strings.Contains(message, "sensitive-account") || strings.Contains(message, "sensitive-cookie") {
		t.Fatalf("secret escaped through error: %s", message)
	}
}
