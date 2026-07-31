package main

import (
	"net/http"
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
