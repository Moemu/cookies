package rparunner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/browserautomation"
	"github.com/shikanon/cookies/internal/platform/contract"
)

// PlanCompiler turns an authorized run (and, for submit, the authorized
// attempt) into the deterministic plan executed by the Playwright runner.
type PlanCompiler interface {
	CompilePrepare(run browserautomation.BrowserRpaRun, policy browserautomation.SitePolicy) (RpaPlan, error)
	CompileSubmit(run browserautomation.BrowserRpaRun, attempt browserautomation.ControlledActionAttempt, policy browserautomation.SitePolicy) (RpaPlan, error)
}

type V3PlanCompiler interface {
	CompilePrepareV3(context.Context, browserautomation.BrowserRpaRun, browserautomation.SitePolicy) (json.RawMessage, error)
	CompileSubmitV3(context.Context, browserautomation.BrowserRpaRun, browserautomation.ControlledActionAttempt, browserautomation.SitePolicy, string) (json.RawMessage, error)
}

const (
	ProtocolV3     = "v3"
	ProtocolLegacy = "legacy"
)

// LeaseHeartbeater keeps the run lease alive while the subprocess executes.
// browserautomation.Service satisfies this interface.
type LeaseHeartbeater interface {
	HeartbeatRunLease(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, runID, leaseID string, expectedVersion, fencingToken int64) (browserautomation.SessionLease, error)
}

const heartbeatInterval = 30 * time.Second

// AdapterConfig carries the process-level executor settings. Per-run values
// (account, CDP endpoint, policy) come from the control-plane records.
type AdapterConfig struct {
	Protocol            string
	Command             []string
	ScriptPath          string
	WorkDir             string
	EvidenceRoot        string
	EdgeSessionFile     string
	AuthorityStateRoot  string
	V3Compiler          V3PlanCompiler
	PrepareTimeout      time.Duration
	SubmitTimeout       time.Duration
	FallbackCDPEndpoint string
}

// NewPlaywrightRPAAdapter wires the real browser executor into the control
// plane worker.
func NewPlaywrightRPAAdapter(cfg AdapterConfig, store browserautomation.Repository, heartbeat LeaseHeartbeater, compiler PlanCompiler) PlaywrightRPAAdapter {
	return PlaywrightRPAAdapter{
		Runner: Runner{
			Command:        cfg.Command,
			ScriptPath:     cfg.ScriptPath,
			WorkDir:        cfg.WorkDir,
			PrepareTimeout: cfg.PrepareTimeout,
			SubmitTimeout:  cfg.SubmitTimeout,
		},
		Protocol:           cfg.Protocol,
		Compiler:           compiler,
		V3Compiler:         cfg.V3Compiler,
		Store:              store,
		Heartbeat:          heartbeat,
		EvidenceRoot:       cfg.EvidenceRoot,
		EdgeSessionFile:    cfg.EdgeSessionFile,
		AuthorityStateRoot: cfg.AuthorityStateRoot,
		FallbackCDP:        cfg.FallbackCDPEndpoint,
	}
}

// PlaywrightRPAAdapter is the real WorkerAdapter: it compiles an authorized
// run into a deterministic Playwright plan and executes it in a subprocess
// attached to the externally authenticated browser session over CDP.
type PlaywrightRPAAdapter struct {
	Runner             Runner
	Protocol           string
	Compiler           PlanCompiler
	V3Compiler         V3PlanCompiler
	Store              browserautomation.Repository
	Heartbeat          LeaseHeartbeater
	EvidenceRoot       string
	EdgeSessionFile    string
	AuthorityStateRoot string
	FallbackCDP        string
}

var _ browserautomation.WorkerAdapter = PlaywrightRPAAdapter{}

func (a PlaywrightRPAAdapter) Prepare(ctx context.Context, run browserautomation.BrowserRpaRun) (browserautomation.PreparedPage, error) {
	env, policy, err := a.resolveSession(ctx, run)
	if err != nil {
		return browserautomation.PreparedPage{}, err
	}
	var result RpaResult
	if a.protocol() == ProtocolV3 {
		if a.V3Compiler == nil {
			return browserautomation.PreparedPage{}, fmt.Errorf("%w: runner v3 compiler is not configured", browserautomation.ErrEnvironmentUnavailable)
		}
		plan, compileErr := a.V3Compiler.CompilePrepareV3(ctx, run, policy)
		if compileErr != nil {
			return browserautomation.PreparedPage{}, fmt.Errorf("%w: %v", browserautomation.ErrPageDrift, compileErr)
		}
		result, err = a.sessionRunner(env).RunV3(ctx, plan, "", a.AuthorityStateRoot)
	} else {
		plan, compileErr := a.Compiler.CompilePrepare(run, policy)
		if compileErr != nil {
			return browserautomation.PreparedPage{}, fmt.Errorf("%w: %v", browserautomation.ErrPageDrift, compileErr)
		}
		plan.EvidenceRoot = a.EvidenceRoot
		result, err = a.sessionRunner(env).Run(ctx, plan)
	}
	if err != nil {
		return browserautomation.PreparedPage{}, fmt.Errorf("%w: %v", browserautomation.ErrEnvironmentUnavailable, err)
	}
	if result.Outcome != OutcomeSuccess {
		return browserautomation.PreparedPage{}, classifyResult(result)
	}
	page := preparedPageFromResult(result)
	if err := completePrepareReadback(run, result, &page); err != nil {
		return browserautomation.PreparedPage{}, err
	}
	return page, nil
}

// completePrepareReadback promotes the runner-observed object identity into
// the contract key, verifies it against the bound authority, and injects the
// server-owned values (account reference and immutable state hashes) that the
// page cannot supply.
func completePrepareReadback(run browserautomation.BrowserRpaRun, result RpaResult, page *browserautomation.PreparedPage) error {
	if page.Readback == nil {
		page.Readback = map[string]string{}
	}
	if objectID, ok := observedObjectID(result); ok {
		page.Readback["platform_object_id"] = objectID
	} else if result.SchemaVersion == ResultSchemaV2 && changesExistingPromotion(run.Authority.Action) {
		// Runner v3 identifies the exact edit URL before it touches a field.
		// Its prepare result has no created_object_id because no object is created.
		page.Readback["platform_object_id"] = run.Authority.TargetPlatformObjectID
	}
	if !changesExistingPromotion(run.Authority.Action) {
		return nil
	}
	if page.Readback["platform_object_id"] != run.Authority.TargetPlatformObjectID {
		return fmt.Errorf("%w: observed object %q does not match the bound target %q", browserautomation.ErrPageDrift, page.Readback["platform_object_id"], run.Authority.TargetPlatformObjectID)
	}
	currentStateHash, targetStateHash, err := run.Authority.ExistingPromotionStateHashes()
	if err != nil {
		return fmt.Errorf("%w: %v", browserautomation.ErrPageDrift, err)
	}
	page.Readback["account_id"] = run.Authority.AccountReferenceID
	page.Readback["current_state_hash"] = currentStateHash
	page.Readback["target_state_hash"] = targetStateHash
	return nil
}

func observedObjectID(result RpaResult) (string, bool) {
	for _, step := range result.Steps {
		if value, ok := step.Readback["object_id"].(string); ok && value != "" {
			return value, true
		}
	}
	return "", false
}

func changesExistingPromotion(action string) bool {
	switch action {
	case "update_promotion_budget", "update_promotion_materials", "pause_promotion", "resume_promotion":
		return true
	default:
		return false
	}
}

func (a PlaywrightRPAAdapter) Submit(ctx context.Context, run browserautomation.BrowserRpaRun, attempt browserautomation.ControlledActionAttempt, confirmToken string) (browserautomation.WorkerOutcome, browserautomation.PreparedPage, error) {
	env, policy, err := a.resolveSession(ctx, run)
	if err != nil {
		return browserautomation.WorkerFailed, browserautomation.PreparedPage{}, err
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopHeartbeat := a.keepLeaseAlive(runCtx, cancel, run, attempt)
	defer stopHeartbeat()

	var result RpaResult
	if a.protocol() == ProtocolV3 {
		if a.V3Compiler == nil {
			return browserautomation.WorkerFailed, browserautomation.PreparedPage{}, fmt.Errorf("%w: runner v3 compiler is not configured", browserautomation.ErrEnvironmentUnavailable)
		}
		plan, compileErr := a.V3Compiler.CompileSubmitV3(runCtx, run, attempt, policy, confirmToken)
		if compileErr != nil {
			return browserautomation.WorkerFailed, browserautomation.PreparedPage{}, fmt.Errorf("%w: %v", browserautomation.ErrPageDrift, compileErr)
		}
		result, err = a.sessionRunner(env).RunV3(runCtx, plan, confirmToken, a.AuthorityStateRoot)
	} else {
		plan, compileErr := a.Compiler.CompileSubmit(run, attempt, policy)
		if compileErr != nil {
			return browserautomation.WorkerFailed, browserautomation.PreparedPage{}, fmt.Errorf("%w: %v", browserautomation.ErrPageDrift, compileErr)
		}
		plan.EvidenceRoot = a.EvidenceRoot
		result, err = a.sessionRunner(env).Run(runCtx, plan)
	}
	if err != nil {
		// Infrastructure failure (crash, timeout, kill) leaves the platform
		// effect unproven either way; the contract only permits query,
		// re-identification or takeover from here.
		return browserautomation.WorkerResultUnknown, browserautomation.PreparedPage{}, nil
	}
	if result.Outcome == OutcomeSuccess && !result.FinalClickPerformed {
		return browserautomation.WorkerFailed, browserautomation.PreparedPage{}, fmt.Errorf("%w: runner reported success without performing the authorized click", browserautomation.ErrPageDrift)
	}
	switch result.Outcome {
	case OutcomeSuccess:
		return browserautomation.WorkerSuccess, preparedPageFromResult(result), nil
	case OutcomeSuccessWithDrift:
		return browserautomation.WorkerPartial, preparedPageFromResult(result), nil
	case OutcomePartial:
		return browserautomation.WorkerPartial, preparedPageFromResult(result), nil
	case OutcomeResultUnknown:
		return browserautomation.WorkerResultUnknown, preparedPageFromResult(result), nil
	default:
		return browserautomation.WorkerFailed, preparedPageFromResult(result), classifyResult(result)
	}
}

func (a PlaywrightRPAAdapter) protocol() string {
	if a.Protocol == "" {
		return ProtocolLegacy
	}
	return a.Protocol
}

func (a PlaywrightRPAAdapter) sessionRunner(env browserautomation.ExecutionEnvironment) Runner {
	runner := a.Runner.WithCDPEndpoint(env.CDPEndpoint)
	if a.protocol() == ProtocolV3 && a.EdgeSessionFile != "" {
		runner = runner.WithEdgeSessionFile(a.EdgeSessionFile)
	}
	return runner
}

func (a PlaywrightRPAAdapter) resolveSession(ctx context.Context, run browserautomation.BrowserRpaRun) (browserautomation.ExecutionEnvironment, browserautomation.SitePolicy, error) {
	env, err := a.Store.GetEnvironment(ctx, run.OrganizationID, run.ProjectID, run.EnvironmentID)
	if err != nil {
		return browserautomation.ExecutionEnvironment{}, browserautomation.SitePolicy{}, fmt.Errorf("%w: %v", browserautomation.ErrEnvironmentUnavailable, err)
	}
	if env.AccountID != run.AccountID {
		return browserautomation.ExecutionEnvironment{}, browserautomation.SitePolicy{}, browserautomation.ErrAccountMismatch
	}
	if env.CDPEndpoint == "" {
		env.CDPEndpoint = a.FallbackCDP
	}
	if !env.Healthy || (env.CDPEndpoint == "" && (a.protocol() != ProtocolV3 || a.EdgeSessionFile == "")) {
		return browserautomation.ExecutionEnvironment{}, browserautomation.SitePolicy{}, browserautomation.ErrEnvironmentUnavailable
	}
	policy, err := a.Store.GetSitePolicy(ctx, run.OrganizationID, run.ProjectID, run.PolicyID)
	if err != nil {
		return browserautomation.ExecutionEnvironment{}, browserautomation.SitePolicy{}, fmt.Errorf("%w: %v", browserautomation.ErrEnvironmentUnavailable, err)
	}
	return env, policy, nil
}

// keepLeaseAlive heartbeats the run lease every 30 seconds (the heartbeat
// deadline is one minute). A failed heartbeat cancels the subprocess
// context; if the final click has not happened yet, the runner stops before
// crossing the write boundary.
func (a PlaywrightRPAAdapter) keepLeaseAlive(ctx context.Context, cancel context.CancelFunc, run browserautomation.BrowserRpaRun, attempt browserautomation.ControlledActionAttempt) context.CancelFunc {
	if a.Heartbeat == nil || run.LeaseID == "" {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		lease, err := a.Store.GetLease(ctx, run.OrganizationID, run.ProjectID, run.LeaseID)
		if err != nil {
			cancel()
			return
		}
		leaseVersion := lease.Version
		missed := false
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				updated, err := a.Heartbeat.HeartbeatRunLease(ctx, run.OrganizationID, run.ProjectID, run.ID, run.LeaseID, leaseVersion, attempt.FencingToken)
				if err != nil {
					// One immediate retry absorbs transient database blips;
					// a second consecutive failure cancels the run.
					if missed {
						cancel()
						return
					}
					missed = true
					updated, err = a.Heartbeat.HeartbeatRunLease(ctx, run.OrganizationID, run.ProjectID, run.ID, run.LeaseID, leaseVersion, attempt.FencingToken)
					if err != nil {
						cancel()
						return
					}
				}
				missed = false
				leaseVersion = updated.Version
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func classifyResult(result RpaResult) error {
	switch result.ErrorCode {
	case CodeAccountMismatch:
		return fmt.Errorf("%w: %s", browserautomation.ErrAccountMismatch, result.ErrorMessage)
	case CodeCDPUnavailable, CodeEnvironmentUnavailable, CodeTimeout, CodeInternal:
		return fmt.Errorf("%w: %s", browserautomation.ErrEnvironmentUnavailable, result.ErrorMessage)
	default:
		return fmt.Errorf("%w: %s: %s", browserautomation.ErrPageDrift, result.ErrorCode, result.ErrorMessage)
	}
}

func preparedPageFromResult(result RpaResult) browserautomation.PreparedPage {
	page := browserautomation.PreparedPage{
		SelectorVersion: SelectorVersion,
		ActionVersion:   ActionVersion,
		Readback:        map[string]string{},
	}
	for _, step := range result.Steps {
		for key, value := range stringReadback(step.Readback) {
			page.Readback[key] = value
		}
		if len(step.BeforeFacts) > 0 {
			page.BeforeFacts = step.BeforeFacts
		}
		page.DiffKeys = append(page.DiffKeys, step.DiffKeys...)
		if step.PageReference != "" {
			page.PageRef = step.PageReference
		}
		if step.ScreenshotPath != "" {
			page.ScreenshotRef = step.ScreenshotPath
		}
	}
	if result.CreatedObjectID != "" {
		page.Readback["platform_object_id"] = result.CreatedObjectID
	}
	if result.Reconciliation != "" {
		page.Readback["reconciliation"] = result.Reconciliation
	}
	if result.FieldReconciliation != nil {
		page.Readback["field_reconciliation_status"] = result.FieldReconciliation.Status
		for _, field := range result.FieldReconciliation.Fields {
			if field.Expected != nil {
				page.Readback["field."+field.FieldKey+".expected"] = stringifyReadback(field.Expected)
			}
			if field.Observed != nil {
				page.Readback["field."+field.FieldKey+".observed"] = stringifyReadback(field.Observed)
			}
			if field.Status == "drifted" {
				page.DiffKeys = append(page.DiffKeys, field.FieldKey)
			}
		}
	}
	if page.DiffKeys == nil {
		page.DiffKeys = []string{}
	}
	return page
}

func stringReadback(values map[string]any) map[string]string {
	if values == nil {
		return nil
	}
	readback := make(map[string]string, len(values))
	for key, value := range values {
		readback[key] = stringifyReadback(value)
	}
	return readback
}

func stringifyReadback(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	payload, err := json.Marshal(value)
	if err == nil {
		return string(payload)
	}
	return fmt.Sprint(value)
}
