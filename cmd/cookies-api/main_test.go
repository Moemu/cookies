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

func TestProductionComputerUseMountIsTakeoverOnly(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	mainSource := string(source)
	if !strings.Contains(mainSource, "computerusehttp.NewTakeoverOnly") {
		t.Fatal("production API must mount the takeover-only Computer Use control plane")
	}
	if strings.Contains(mainSource, "computeruse.DeterministicFakeAdapter") {
		t.Fatal("production API must not wire the deterministic fake Computer Use adapter")
	}
}
