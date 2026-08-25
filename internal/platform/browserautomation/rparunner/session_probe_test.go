package rparunner

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionProbeRunnerPassesAccountOverStdinAndParsesSafeResult(t *testing.T) {
	script, err := filepath.Abs(filepath.Join("testdata", "fake-session-probe.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := (sessionProbeRunner{
		Command: []string{"node"}, ScriptPath: script, SessionFile: "session.json", Timeout: 5 * time.Second,
	}).Run(context.Background(), "1855554434276391")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Ready() || result.Reason != "session_ready" {
		t.Fatalf("probe=%#v", result)
	}
}
