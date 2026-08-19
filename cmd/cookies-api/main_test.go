package main

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestHTTPServerWriteTimeoutCoversLongRunningModelRequests(t *testing.T) {
	server := newHTTPServer(":0", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	if server.WriteTimeout < 10*time.Minute {
		t.Fatalf("WriteTimeout = %s, want at least 10m for configured text model routes", server.WriteTimeout)
	}
	if server.WriteTimeout <= server.ReadTimeout {
		t.Fatalf("WriteTimeout = %s must exceed ReadTimeout = %s", server.WriteTimeout, server.ReadTimeout)
	}
}

func TestProductionBrowserRPAMountIsSafeByDefault(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	mainSource := string(source)
	if !strings.Contains(mainSource, "browserautomationhttp.NewTakeoverOnly") {
		t.Fatal("production API must mount the takeover-only Browser RPA control plane")
	}
	if !strings.Contains(mainSource, "cfg.BrowserRPA.Enabled") {
		t.Fatal("automated Browser RPA worker must be gated on COOKIES_BROWSER_RPA_ENABLED")
	}
	if !strings.Contains(mainSource, "rparunner.NewPlaywrightRPAAdapter") {
		t.Fatal("automated Browser RPA worker must wire the real Playwright adapter")
	}
	if strings.Contains(mainSource, "browserautomation.DeterministicFakeAdapter") {
		t.Fatal("production API must not wire the deterministic fake Browser RPA adapter")
	}
}
