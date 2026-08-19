package rparunner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// ErrRunnerInfrastructure reports that the subprocess itself failed (could not
// start, produced no parseable result, or was killed). Callers decide whether
// that maps to a failed run or result_unknown based on click progress.
var ErrRunnerInfrastructure = errors.New("rpa runner infrastructure failure")

const stderrCap = 8 << 10

type Runner struct {
	// Command launches the TypeScript runner script, typically
	// []string{"npx", "tsx"} or a repo-local tsx binary path.
	Command []string
	// ScriptPath is the runner script, relative to WorkDir.
	ScriptPath string
	// WorkDir is the process working directory, normally the repository root.
	WorkDir string
	// CDPEndpoint is the DevTools endpoint of the externally authenticated
	// browser session.
	CDPEndpoint string

	PrepareTimeout time.Duration
	SubmitTimeout  time.Duration
}

// WithCDPEndpoint returns a copy of the runner bound to a specific DevTools
// endpoint, so one configured runner can serve many environments.
func (r Runner) WithCDPEndpoint(endpoint string) Runner {
	r.CDPEndpoint = endpoint
	return r
}

// Run executes one plan in a subprocess and returns the parsed result. The
// plan travels over stdin; only the result document comes back over stdout.
func (r Runner) Run(ctx context.Context, plan RpaPlan) (RpaResult, error) {
	payload, err := json.Marshal(plan)
	if err != nil {
		return RpaResult{}, fmt.Errorf("%w: encode plan: %v", ErrRunnerInfrastructure, err)
	}
	timeout := r.PrepareTimeout
	if plan.Mode == "submit" {
		timeout = r.SubmitTimeout
	}
	if timeout <= 0 {
		timeout = 3 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if len(r.Command) == 0 {
		return RpaResult{}, fmt.Errorf("%w: runner command is not configured", ErrRunnerInfrastructure)
	}
	args := append(append([]string{}, r.Command[1:]...), r.ScriptPath, r.CDPEndpoint)
	cmd := exec.CommandContext(ctx, r.Command[0], args...)
	cmd.Dir = r.WorkDir
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = limitedWriter{Buffer: &stderr, Cap: stderrCap}
	runErr := cmd.Run()

	result, parseErr := parseResult(stdout.Bytes())
	if parseErr == nil {
		return result, nil
	}
	if runErr != nil {
		return RpaResult{}, fmt.Errorf("%w: %v: %s", ErrRunnerInfrastructure, runErr, tailMessage(stderr.String()))
	}
	return RpaResult{}, fmt.Errorf("%w: %v: %s", ErrRunnerInfrastructure, parseErr, tailMessage(stderr.String()))
}

func parseResult(payload []byte) (RpaResult, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return RpaResult{}, errors.New("empty result")
	}
	var result RpaResult
	if err := json.Unmarshal(trimmed, &result); err != nil {
		return RpaResult{}, err
	}
	if result.SchemaVersion != ResultSchemaV1 {
		return RpaResult{}, fmt.Errorf("unexpected result schema %q", result.SchemaVersion)
	}
	return result, nil
}

func tailMessage(value string) string {
	const cap = 512
	if len(value) <= cap {
		return value
	}
	return value[len(value)-cap:]
}

type limitedWriter struct {
	Buffer *bytes.Buffer
	Cap    int
}

func (w limitedWriter) Write(p []byte) (int, error) {
	if w.Buffer.Len() >= w.Cap {
		return len(p), nil
	}
	room := w.Cap - w.Buffer.Len()
	if len(p) > room {
		p = p[:room]
	}
	return w.Buffer.Write(p)
}
