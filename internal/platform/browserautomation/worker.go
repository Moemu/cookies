package browserautomation

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type WorkerOutcome string

const (
	WorkerSuccess       WorkerOutcome = "success"
	WorkerFailed        WorkerOutcome = "failed"
	WorkerPartial       WorkerOutcome = "partial"
	WorkerResultUnknown WorkerOutcome = "result_unknown"
)

type PreparedPage struct {
	BeforeFacts map[string]string
	Readback    map[string]string
	DiffKeys    []string
	PageRef     string
	// These values identify the exact Cookies draft selected by a staged
	// create plan. The server compiler supplies them.
	InternalObjectKind string
	InternalObjectID   string
	// Evidence metadata supplied by the executing adapter. Empty values fall
	// back to the deterministic-fake provenance used by test adapters.
	ScreenshotRef   string
	SelectorVersion string
	ActionVersion   string
}

// Typed adapter failures. Worker.Prepare classifies them into stable blocking
// reasons; adapters must wrap these instead of returning free-form text.
var (
	ErrAccountMismatch        = errors.New("browser rpa account mismatch")
	ErrPageDrift              = errors.New("browser rpa page drift")
	ErrEnvironmentUnavailable = errors.New("browser rpa environment unavailable")
	ErrResultUnknown          = errors.New("browser rpa result unknown")
)

type WorkerAdapter interface {
	Prepare(context.Context, BrowserRpaRun) (PreparedPage, error)
	Submit(context.Context, BrowserRpaRun, ControlledActionAttempt, string) (WorkerOutcome, PreparedPage, error)
}

// WorkerPlanAdapter is optional. Real Runner v3 adapters implement it to
// expose the exact prepare plan without opening a page or changing a run.
type WorkerPlanAdapter interface {
	Plan(context.Context, BrowserRpaRun) (json.RawMessage, error)
}

const EdgeSessionProbeSchemaV1 = "browser-rpa-edge-session-probe/v1"

// EdgeSessionProbe contains only safe session facts. It never exposes a CDP
// endpoint, page URL, browser target, cookie, or account value from the page.
type EdgeSessionProbe struct {
	SchemaVersion            string    `json:"schema_version"`
	CheckedAt                time.Time `json:"checked_at"`
	Status                   string    `json:"status"`
	Reason                   string    `json:"reason"`
	CDPAvailable             bool      `json:"cdp_available"`
	OceanEnginePageAvailable bool      `json:"oceanengine_page_available"`
	LoggedIn                 bool      `json:"logged_in"`
	AccountMatched           bool      `json:"account_matched"`
}

func (p EdgeSessionProbe) Ready() bool {
	return p.Status == "ready" && p.CDPAvailable && p.OceanEnginePageAvailable && p.LoggedIn && p.AccountMatched
}

// WorkerSessionProbeAdapter is optional for legacy adapters. The production
// Runner v3 adapter implements it. Worker.Prepare repeats the probe so a UI
// result cannot become an unsafe, stale authorization.
type WorkerSessionProbeAdapter interface {
	CheckSession(context.Context, BrowserRpaRun) (EdgeSessionProbe, error)
}

// CreatedObjectRecorder saves a reconciled platform identity before the
// worker advances to the next staged form. A false complete value means that
// the same run has another approved form to execute.
type CreatedObjectRecorder interface {
	RecordCreatedObject(context.Context, AuthorityBinding, string, PreparedPage, string, string, time.Time) (complete bool, err error)
}

type DeterministicFakeAdapter struct {
	Outcome   WorkerOutcome
	AccountID string
}

func (a DeterministicFakeAdapter) CheckSession(_ context.Context, run BrowserRpaRun) (EdgeSessionProbe, error) {
	matched := a.AccountID == "" || a.AccountID == run.AccountID
	status, reason := "ready", "session_ready"
	if !matched {
		status, reason = "blocked", "account_mismatch"
	}
	return EdgeSessionProbe{SchemaVersion: EdgeSessionProbeSchemaV1, CheckedAt: time.Now().UTC(), Status: status, Reason: reason, CDPAvailable: true, OceanEnginePageAvailable: true, LoggedIn: true, AccountMatched: matched}, nil
}

func (a DeterministicFakeAdapter) Prepare(_ context.Context, run BrowserRpaRun) (PreparedPage, error) {
	if a.AccountID != "" && a.AccountID != run.AccountID {
		return PreparedPage{}, ErrAccountMismatch
	}
	readback := map[string]string{"account_id": run.AccountID, "object_fingerprint": run.Authority.ObjectFingerprint}
	if changesExistingPromotionAction(run.Authority.Action) {
		readback["platform_object_id"] = run.Authority.TargetPlatformObjectID
		if currentStateHash, targetStateHash, err := run.Authority.existingPromotionStateHashes(); err == nil {
			readback["current_state_hash"] = currentStateHash
			readback["target_state_hash"] = targetStateHash
		}
		if restart := run.Authority.PromotionRestart; restart != nil {
			scheduleHash, materialsHash, err := restart.readbackHashes()
			if err != nil {
				return PreparedPage{}, err
			}
			readback["platform_project_id"] = run.Authority.ParentPlatformProjectID
			readback["platform_status"] = restart.CurrentPlatformStatus
			readback["daily_budget_minor"] = strconv.FormatInt(restart.ApprovedDailyBudgetMinor, 10)
			readback["schedule_hash"] = scheduleHash
			readback["material_references_hash"] = materialsHash
			readback["landing_page_reference_id"] = restart.LandingPage.ReferenceID
			readback["materials_available"] = "true"
			readback["landing_page_available"] = "true"
		}
	}
	return PreparedPage{BeforeFacts: map[string]string{"account_id": run.AccountID, "page_kind": "review"}, Readback: readback, DiffKeys: []string{}, PageRef: "fake://oceanengine/review"}, nil
}
func (a DeterministicFakeAdapter) Submit(_ context.Context, run BrowserRpaRun, _ ControlledActionAttempt, _ string) (WorkerOutcome, PreparedPage, error) {
	outcome := a.Outcome
	if outcome == "" {
		outcome = WorkerSuccess
	}
	objectID := "fake-object-" + run.ID
	if changesExistingPromotionAction(run.Authority.Action) {
		objectID = run.Authority.TargetPlatformObjectID
	}
	targetStateHash := ""
	if _, stateHash, err := run.Authority.existingPromotionStateHashes(); err == nil {
		targetStateHash = stateHash
	}
	platformStatus := string(outcome)
	if targetStatus := run.Authority.existingPromotionTargetStatus(); targetStatus != "" && outcome == WorkerSuccess {
		platformStatus = targetStatus
	}
	page := PreparedPage{BeforeFacts: map[string]string{"object_fingerprint": run.Authority.ObjectFingerprint}, Readback: map[string]string{"platform_object_id": objectID, "platform_status": platformStatus, "target_state_hash": targetStateHash}, DiffKeys: []string{}, PageRef: "fake://oceanengine/result"}
	return outcome, page, nil
}

type Worker struct {
	Service Service
	Adapter WorkerAdapter
}

func (w Worker) CheckSession(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, runID string) (EdgeSessionProbe, error) {
	run, err := w.Service.Repository.GetRun(ctx, org, project, runID)
	if err != nil {
		return EdgeSessionProbe{}, err
	}
	probe, ok := w.Adapter.(WorkerSessionProbeAdapter)
	if !ok {
		return EdgeSessionProbe{}, ErrEnvironmentUnavailable
	}
	return probe.CheckSession(ctx, run)
}

func (w Worker) Plan(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, runID string) (json.RawMessage, error) {
	run, err := w.Service.Repository.GetRun(ctx, org, project, runID)
	if err != nil {
		return nil, err
	}
	if run.Paused || run.TakeoverActive || terminalState(run.State) {
		return nil, ErrInvalidTransition
	}
	planner, ok := w.Adapter.(WorkerPlanAdapter)
	if !ok {
		return nil, ErrEnvironmentUnavailable
	}
	return planner.Plan(ctx, run)
}

func (w Worker) Prepare(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, runID string) (BrowserRpaRun, error) {
	run, err := w.Service.Repository.GetRun(ctx, org, project, runID)
	if err != nil {
		return BrowserRpaRun{}, err
	}
	if run.Paused || run.TakeoverActive {
		return BrowserRpaRun{}, ErrInvalidTransition
	}
	if run.State == RunQueued {
		run, err = w.Service.TransitionRun(ctx, org, project, runID, run.Version, RunEnvironmentCheck, "")
		if err != nil {
			return BrowserRpaRun{}, err
		}
	}
	if run.State != RunEnvironmentCheck {
		return BrowserRpaRun{}, ErrInvalidTransition
	}
	run, err = w.Service.TransitionRun(ctx, org, project, runID, run.Version, RunPreparing, "")
	if err != nil {
		return BrowserRpaRun{}, err
	}
	if probeAdapter, ok := w.Adapter.(WorkerSessionProbeAdapter); ok {
		probe, probeErr := probeAdapter.CheckSession(ctx, run)
		if probeErr != nil || !probe.Ready() {
			reason := BlockPageDrift
			if probe.Reason == "account_mismatch" || errors.Is(probeErr, ErrAccountMismatch) {
				reason = BlockAccountMismatch
			}
			return w.Service.TransitionRun(ctx, org, project, runID, run.Version, RunFailed, reason)
		}
	}
	prepared, err := w.Adapter.Prepare(ctx, run)
	if err != nil {
		reason := BlockPageDrift
		if errors.Is(err, ErrAccountMismatch) {
			reason = BlockAccountMismatch
		}
		return w.Service.TransitionRun(ctx, org, project, runID, run.Version, RunFailed, reason)
	}
	if changesExistingPromotionAction(run.Authority.Action) && run.Authority.validatePreSubmitReadback(prepared.Readback, w.Service.now()) != nil {
		return w.Service.TransitionRun(ctx, org, project, runID, run.Version, RunFailed, BlockPageDrift)
	}
	step := RunStep{ID: run.ID + "-prepare-v" + strconv.FormatInt(run.Version, 10), RunID: run.ID, Sequence: int(run.Version)*10 + 1, WorkflowStepID: run.Authority.WorkflowStepID, Action: "prepare_and_readback", Status: StepSucceeded, Attempt: 1, Version: 1}
	if err := w.Service.Repository.PutStep(ctx, org, project, step); err != nil {
		return BrowserRpaRun{}, err
	}
	if err := w.appendEvidence(ctx, run, step, prepared); err != nil {
		return BrowserRpaRun{}, err
	}
	return w.Service.TransitionRun(ctx, org, project, runID, run.Version, RunAwaitingConfirmation, BlockFinalConfirmationRequired)
}

type WorkerSubmitRequest struct{ Authorize AuthorizeActionRequest }

func (w Worker) Submit(ctx context.Context, request WorkerSubmitRequest) (BrowserRpaRun, error) {
	run, err := w.Service.Repository.GetRun(ctx, request.Authorize.OrganizationID, request.Authorize.ProjectID, request.Authorize.RunID)
	if err != nil {
		return BrowserRpaRun{}, err
	}
	stepAction := "submit_platform_configuration"
	if stagedCreateAction(run.Authority.Action) {
		stepAction = string(TakeoverResultObserved)
	}
	step := RunStep{ID: request.Authorize.StepID, RunID: run.ID, Sequence: int(run.Version)*10 + 2, WorkflowStepID: run.Authority.WorkflowStepID, Action: stepAction, Status: StepPending, Attempt: 1, Version: 1}
	if err := w.Service.Repository.PutStep(ctx, run.OrganizationID, run.ProjectID, step); err != nil {
		return BrowserRpaRun{}, err
	}
	attempt, err := w.Service.AuthorizeAction(ctx, request.Authorize)
	if err != nil {
		return BrowserRpaRun{}, err
	}
	run, err = w.Service.TransitionRun(ctx, run.OrganizationID, run.ProjectID, run.ID, run.Version, RunSubmitting, "")
	if err != nil {
		return BrowserRpaRun{}, err
	}
	step.Status = StepRunning
	step.Version++
	if err := w.Service.Repository.PutStep(ctx, run.OrganizationID, run.ProjectID, step); err != nil {
		return BrowserRpaRun{}, err
	}
	outcome, page, adapterErr := w.Adapter.Submit(ctx, run, attempt, request.Authorize.Token)
	if adapterErr != nil {
		step.Status = StepFailed
		step.Version++
		_ = w.Service.Repository.PutStep(ctx, run.OrganizationID, run.ProjectID, step)
		_ = w.appendEvidence(ctx, run, step, page)
		if err := w.Service.Repository.CompleteControlledAction(ctx, run.OrganizationID, run.ProjectID, attempt.ID, ControlledActionFailed); err != nil {
			return BrowserRpaRun{}, err
		}
		reason := BlockPageDrift
		if errors.Is(adapterErr, ErrAccountMismatch) {
			reason = BlockAccountMismatch
		} else if errors.Is(adapterErr, ErrEnvironmentUnavailable) {
			reason = BlockResultReconciliation
		}
		return w.Service.TransitionRun(ctx, run.OrganizationID, run.ProjectID, run.ID, run.Version, RunFailed, reason)
	}
	if outcome == WorkerResultUnknown {
		step.Status = StepResultUnknown
		step.Version++
		_ = w.Service.Repository.PutStep(ctx, run.OrganizationID, run.ProjectID, step)
		_ = w.appendEvidence(ctx, run, step, page)
		if err := w.Service.Repository.CompleteControlledAction(ctx, run.OrganizationID, run.ProjectID, attempt.ID, ControlledActionResultUnknown); err != nil {
			return BrowserRpaRun{}, err
		}
		return w.Service.TransitionRun(ctx, run.OrganizationID, run.ProjectID, run.ID, run.Version, RunResultUnknown, BlockResultReconciliation)
	}
	attemptStatus := ControlledActionVerified
	if outcome == WorkerFailed {
		attemptStatus = ControlledActionFailed
	}
	if err := w.Service.Repository.CompleteControlledAction(ctx, run.OrganizationID, run.ProjectID, attempt.ID, attemptStatus); err != nil {
		return BrowserRpaRun{}, err
	}
	run, err = w.Service.TransitionRun(ctx, run.OrganizationID, run.ProjectID, run.ID, run.Version, RunVerifying, "")
	if err != nil {
		return BrowserRpaRun{}, err
	}
	step.Version++
	step.Status = StepSucceeded
	if outcome == WorkerFailed {
		step.Status = StepFailed
	}
	if err := w.Service.Repository.PutStep(ctx, run.OrganizationID, run.ProjectID, step); err != nil {
		return BrowserRpaRun{}, err
	}
	stagedObjectObserved := stagedCreateAction(run.Authority.Action) && (outcome == WorkerSuccess || outcome == WorkerPartial) && page.InternalObjectID != ""
	if stagedObjectObserved {
		if page.Readback == nil {
			page.Readback = map[string]string{}
		}
		if page.Readback["platform_status"] == "" {
			page.Readback["platform_status"] = "pending_review"
		}
	}
	resultEvidenceID, err := w.appendEvidenceWithID(ctx, run, step, page)
	if err != nil {
		return BrowserRpaRun{}, err
	}
	if stagedObjectObserved {
		reconciliationStep := RunStep{ID: step.ID + "-reconcile", RunID: run.ID, Sequence: step.Sequence + 1, WorkflowStepID: run.Authority.WorkflowStepID, Action: string(TakeoverListConfirmed), Status: StepSucceeded, Attempt: 1, Version: 1}
		if err := w.Service.Repository.PutStep(ctx, run.OrganizationID, run.ProjectID, reconciliationStep); err != nil {
			return BrowserRpaRun{}, err
		}
		listEvidenceID, evidenceErr := w.appendEvidenceWithID(ctx, run, reconciliationStep, page)
		if evidenceErr != nil {
			return BrowserRpaRun{}, evidenceErr
		}
		recorder, ok := w.Service.AuthorityProvider.(CreatedObjectRecorder)
		if !ok {
			return w.Service.TransitionRun(ctx, run.OrganizationID, run.ProjectID, run.ID, run.Version, RunFailed, BlockResultReconciliation)
		}
		complete, recordErr := recorder.RecordCreatedObject(ctx, run.Authority, run.ID, page, resultEvidenceID, listEvidenceID, w.Service.now())
		if recordErr != nil {
			return w.Service.TransitionRun(ctx, run.OrganizationID, run.ProjectID, run.ID, run.Version, RunFailed, BlockResultReconciliation)
		}
		if outcome == WorkerSuccess && !complete {
			return w.Service.TransitionRun(ctx, run.OrganizationID, run.ProjectID, run.ID, run.Version, RunEnvironmentCheck, "")
		}
	}
	terminal := RunSucceeded
	reason := BlockingReason("")
	if outcome == WorkerFailed {
		terminal = RunFailed
	}
	if outcome == WorkerPartial {
		terminal = RunPartial
		reason = BlockResultReconciliation
	}
	return w.Service.TransitionRun(ctx, run.OrganizationID, run.ProjectID, run.ID, run.Version, terminal, reason)
}

func (w Worker) appendEvidence(ctx context.Context, run BrowserRpaRun, step RunStep, page PreparedPage) error {
	_, err := w.appendEvidenceWithID(ctx, run, step, page)
	return err
}

func (w Worker) appendEvidenceWithID(ctx context.Context, run BrowserRpaRun, step RunStep, page PreparedPage) (string, error) {
	now := w.Service.now()
	id, err := w.Service.newID(browserRpaEvidenceIDPrefix)
	if err != nil {
		return "", err
	}
	selectorVersion := page.SelectorVersion
	if selectorVersion == "" {
		selectorVersion = "deterministic-fake-selector/v1"
	}
	actionVersion := page.ActionVersion
	if actionVersion == "" {
		actionVersion = "deterministic-fake-action/v1"
	}
	fingerprint := run.Authority.ObjectFingerprint
	if page.InternalObjectID != "" {
		fingerprint = page.InternalObjectID
	}
	evidence := Evidence{SchemaVersion: EvidenceSchemaV1, ID: id, OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, RunID: run.ID, StepID: step.ID, BeforePageFacts: page.BeforeFacts, AfterPageFacts: page.Readback, FieldReadback: page.Readback, DiffKeys: page.DiffKeys, PageReference: page.PageRef, ScreenshotReference: page.ScreenshotRef, ObjectFingerprint: fingerprint, SelectorVersion: selectorVersion, ActionVersion: actionVersion, CreatedAt: now}
	if err := w.Service.Repository.AppendEvidence(ctx, RedactEvidence(evidence)); err != nil {
		return "", err
	}
	return id, nil
}

func stagedCreateAction(action string) bool {
	return action == "create_project_and_promotions" || action == "create_promotions_in_existing_project"
}

type ControlAction string

const (
	ControlPause           ControlAction = "pause"
	ControlResume          ControlAction = "resume"
	ControlCancel          ControlAction = "cancel"
	ControlTakeover        ControlAction = "takeover"
	ControlReleaseTakeover ControlAction = "release_takeover"
)

func (s Service) ControlRun(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, runID string, expected int64, action ControlAction) (BrowserRpaRun, error) {
	run, err := s.Repository.GetRun(ctx, org, project, runID)
	if err != nil {
		return BrowserRpaRun{}, err
	}
	if run.Version != expected {
		return BrowserRpaRun{}, ErrVersionConflict
	}
	switch action {
	case ControlPause:
		if terminalState(run.State) {
			return BrowserRpaRun{}, ErrInvalidTransition
		}
		return s.setRunControl(ctx, run, expected, run.State, true, false, run.BlockingReason, action)
	case ControlTakeover:
		if terminalState(run.State) {
			return BrowserRpaRun{}, ErrInvalidTransition
		}
		return s.setRunControl(ctx, run, expected, RunAwaitingTakeover, true, true, run.BlockingReason, action)
	case ControlReleaseTakeover, ControlResume:
		if !run.Paused && !run.TakeoverActive {
			return BrowserRpaRun{}, ErrInvalidTransition
		}
		return s.setRunControl(ctx, run, expected, RunEnvironmentCheck, false, false, "", action)
	case ControlCancel:
		if terminalState(run.State) {
			return BrowserRpaRun{}, ErrInvalidTransition
		}
		return s.setRunControl(ctx, run, expected, RunCancelled, false, false, "", action)
	default:
		return BrowserRpaRun{}, ErrInvalidContract
	}
}
func (s Service) setRunControl(ctx context.Context, run BrowserRpaRun, expected int64, state RunState, paused, takeover bool, reason BlockingReason, action ControlAction) (BrowserRpaRun, error) {
	updated, err := s.Repository.SetRunControl(ctx, run.OrganizationID, run.ProjectID, run.ID, expected, state, paused, takeover, reason, s.now())
	if err != nil {
		return BrowserRpaRun{}, err
	}
	if err := s.recordEvent(ctx, updated, "control_"+string(action), string(action), run.CreatedBy); err != nil {
		return BrowserRpaRun{}, err
	}
	return updated, nil
}
func terminalState(state RunState) bool {
	switch state {
	case RunSucceeded, RunFailed, RunPartial, RunResultUnknown, RunCancelled:
		return true
	default:
		return false
	}
}
