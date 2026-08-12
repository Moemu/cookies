package computeruse

import (
	"context"
	"fmt"

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
}

type WorkerAdapter interface {
	Prepare(context.Context, ComputerUseRun) (PreparedPage, error)
	Submit(context.Context, ComputerUseRun, ControlledActionAttempt) (WorkerOutcome, PreparedPage, error)
}

type DeterministicFakeAdapter struct {
	Outcome   WorkerOutcome
	AccountID string
}

func (a DeterministicFakeAdapter) Prepare(_ context.Context, run ComputerUseRun) (PreparedPage, error) {
	if a.AccountID != "" && a.AccountID != run.AccountID {
		return PreparedPage{}, fmt.Errorf("%s", BlockAccountMismatch)
	}
	return PreparedPage{BeforeFacts: map[string]string{"account_id": run.AccountID, "page_kind": "review"}, Readback: map[string]string{"account_id": run.AccountID, "object_fingerprint": run.Authority.ObjectFingerprint}, DiffKeys: []string{}, PageRef: "fake://oceanengine/review"}, nil
}
func (a DeterministicFakeAdapter) Submit(_ context.Context, run ComputerUseRun, _ ControlledActionAttempt) (WorkerOutcome, PreparedPage, error) {
	outcome := a.Outcome
	if outcome == "" {
		outcome = WorkerSuccess
	}
	page := PreparedPage{BeforeFacts: map[string]string{"object_fingerprint": run.Authority.ObjectFingerprint}, Readback: map[string]string{"platform_object_id": "fake-object-" + run.ID, "status": string(outcome)}, DiffKeys: []string{}, PageRef: "fake://oceanengine/result"}
	return outcome, page, nil
}

type Worker struct {
	Service Service
	Adapter WorkerAdapter
}

func (w Worker) Prepare(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, runID string) (ComputerUseRun, error) {
	run, err := w.Service.Repository.GetRun(ctx, org, project, runID)
	if err != nil {
		return ComputerUseRun{}, err
	}
	if run.Paused || run.TakeoverActive {
		return ComputerUseRun{}, ErrInvalidTransition
	}
	if run.State == RunQueued {
		run, err = w.Service.TransitionRun(ctx, org, project, runID, run.Version, RunEnvironmentCheck, "")
		if err != nil {
			return ComputerUseRun{}, err
		}
	}
	if run.State != RunEnvironmentCheck {
		return ComputerUseRun{}, ErrInvalidTransition
	}
	run, err = w.Service.TransitionRun(ctx, org, project, runID, run.Version, RunPreparing, "")
	if err != nil {
		return ComputerUseRun{}, err
	}
	prepared, err := w.Adapter.Prepare(ctx, run)
	if err != nil {
		reason := BlockPageDrift
		if err.Error() == string(BlockAccountMismatch) {
			reason = BlockAccountMismatch
		}
		return w.Service.TransitionRun(ctx, org, project, runID, run.Version, RunFailed, reason)
	}
	step := RunStep{ID: run.ID + "-prepare", RunID: run.ID, Sequence: 1, WorkflowStepID: run.Authority.WorkflowStepID, Action: "prepare_and_readback", Status: StepSucceeded, Attempt: 1, Version: 1}
	if err := w.Service.Repository.PutStep(ctx, org, project, step); err != nil {
		return ComputerUseRun{}, err
	}
	if err := w.appendEvidence(ctx, run, step, prepared); err != nil {
		return ComputerUseRun{}, err
	}
	return w.Service.TransitionRun(ctx, org, project, runID, run.Version, RunAwaitingConfirmation, BlockFinalConfirmationRequired)
}

type WorkerSubmitRequest struct{ Authorize AuthorizeActionRequest }

func (w Worker) Submit(ctx context.Context, request WorkerSubmitRequest) (ComputerUseRun, error) {
	run, err := w.Service.Repository.GetRun(ctx, request.Authorize.OrganizationID, request.Authorize.ProjectID, request.Authorize.RunID)
	if err != nil {
		return ComputerUseRun{}, err
	}
	step := RunStep{ID: request.Authorize.StepID, RunID: run.ID, Sequence: 2, WorkflowStepID: run.Authority.WorkflowStepID, Action: "submit_platform_configuration", Status: StepPending, Attempt: 1, Version: 1}
	if err := w.Service.Repository.PutStep(ctx, run.OrganizationID, run.ProjectID, step); err != nil {
		return ComputerUseRun{}, err
	}
	attempt, err := w.Service.AuthorizeAction(ctx, request.Authorize)
	if err != nil {
		return ComputerUseRun{}, err
	}
	run, err = w.Service.TransitionRun(ctx, run.OrganizationID, run.ProjectID, run.ID, run.Version, RunSubmitting, "")
	if err != nil {
		return ComputerUseRun{}, err
	}
	step.Status = StepRunning
	step.Version++
	if err := w.Service.Repository.PutStep(ctx, run.OrganizationID, run.ProjectID, step); err != nil {
		return ComputerUseRun{}, err
	}
	outcome, page, adapterErr := w.Adapter.Submit(ctx, run, attempt)
	if adapterErr != nil {
		step.Status = StepFailed
		step.Version++
		_ = w.Service.Repository.PutStep(ctx, run.OrganizationID, run.ProjectID, step)
		return w.Service.TransitionRun(ctx, run.OrganizationID, run.ProjectID, run.ID, run.Version, RunFailed, BlockResultReconciliation)
	}
	if outcome == WorkerResultUnknown {
		step.Status = StepResultUnknown
		step.Version++
		_ = w.Service.Repository.PutStep(ctx, run.OrganizationID, run.ProjectID, step)
		_ = w.appendEvidence(ctx, run, step, page)
		return w.Service.TransitionRun(ctx, run.OrganizationID, run.ProjectID, run.ID, run.Version, RunResultUnknown, BlockResultReconciliation)
	}
	run, err = w.Service.TransitionRun(ctx, run.OrganizationID, run.ProjectID, run.ID, run.Version, RunVerifying, "")
	if err != nil {
		return ComputerUseRun{}, err
	}
	step.Version++
	step.Status = StepSucceeded
	if outcome == WorkerFailed {
		step.Status = StepFailed
	}
	if err := w.Service.Repository.PutStep(ctx, run.OrganizationID, run.ProjectID, step); err != nil {
		return ComputerUseRun{}, err
	}
	if err := w.appendEvidence(ctx, run, step, page); err != nil {
		return ComputerUseRun{}, err
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

func (w Worker) appendEvidence(ctx context.Context, run ComputerUseRun, step RunStep, page PreparedPage) error {
	now := w.Service.now()
	id, err := w.Service.newID("cuevidence")
	if err != nil {
		return err
	}
	return w.Service.Repository.AppendEvidence(ctx, Evidence{SchemaVersion: EvidenceSchemaV1, ID: id, OrganizationID: run.OrganizationID, ProjectID: run.ProjectID, RunID: run.ID, StepID: step.ID, BeforePageFacts: page.BeforeFacts, AfterPageFacts: page.Readback, FieldReadback: page.Readback, DiffKeys: page.DiffKeys, PageReference: page.PageRef, ObjectFingerprint: run.Authority.ObjectFingerprint, SkillVersion: run.Authority.SkillVersion, SelectorVersion: "fake-selector/v1", ActionVersion: "fake-action/v1", RedactionVersion: "computer-use-redaction/v1", CreatedAt: now})
}

type ControlAction string

const (
	ControlPause           ControlAction = "pause"
	ControlResume          ControlAction = "resume"
	ControlCancel          ControlAction = "cancel"
	ControlTakeover        ControlAction = "takeover"
	ControlReleaseTakeover ControlAction = "release_takeover"
)

func (s Service) ControlRun(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, runID string, expected int64, action ControlAction) (ComputerUseRun, error) {
	run, err := s.Repository.GetRun(ctx, org, project, runID)
	if err != nil {
		return ComputerUseRun{}, err
	}
	if run.Version != expected {
		return ComputerUseRun{}, ErrVersionConflict
	}
	switch action {
	case ControlPause:
		if terminalState(run.State) {
			return ComputerUseRun{}, ErrInvalidTransition
		}
		return s.setRunControl(ctx, run, expected, run.State, true, false, run.BlockingReason, action)
	case ControlTakeover:
		if terminalState(run.State) {
			return ComputerUseRun{}, ErrInvalidTransition
		}
		return s.setRunControl(ctx, run, expected, RunAwaitingTakeover, true, true, run.BlockingReason, action)
	case ControlReleaseTakeover, ControlResume:
		if !run.Paused && !run.TakeoverActive {
			return ComputerUseRun{}, ErrInvalidTransition
		}
		return s.setRunControl(ctx, run, expected, RunEnvironmentCheck, false, false, "", action)
	case ControlCancel:
		if terminalState(run.State) {
			return ComputerUseRun{}, ErrInvalidTransition
		}
		return s.setRunControl(ctx, run, expected, RunCancelled, false, false, "", action)
	default:
		return ComputerUseRun{}, ErrInvalidContract
	}
}
func (s Service) setRunControl(ctx context.Context, run ComputerUseRun, expected int64, state RunState, paused, takeover bool, reason BlockingReason, action ControlAction) (ComputerUseRun, error) {
	updated, err := s.Repository.SetRunControl(ctx, run.OrganizationID, run.ProjectID, run.ID, expected, state, paused, takeover, reason, s.now())
	if err != nil {
		return ComputerUseRun{}, err
	}
	if err := s.recordEvent(ctx, updated, "control_"+string(action), string(action), run.CreatedBy); err != nil {
		return ComputerUseRun{}, err
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
