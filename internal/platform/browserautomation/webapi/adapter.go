package webapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/shikanon/cookies/internal/platform/browserautomation"
)

const (
	ContractVersion = "oceanengine-web-api-contract/v1"
	SelectorVersion = "oceanengine-web-api/session/v1"
	ActionVersion   = "oceanengine-web-api/action/v1"
)

var (
	ErrWriteDisabled       = errors.New("Ocean Engine Web API write is disabled")
	ErrAccountNotAllowed   = errors.New("Ocean Engine Web API account is not allowed")
	ErrContractNotCaptured = errors.New("Ocean Engine Web API write contract is not captured")
)

type PlanCompiler interface {
	CompilePrepareV3(context.Context, browserautomation.BrowserRpaRun, browserautomation.SitePolicy) (json.RawMessage, error)
}

type SessionChecker interface {
	Check(context.Context, browserautomation.BrowserRpaRun) error
}

type Adapter struct {
	Compiler         PlanCompiler
	Policies         browserautomation.Repository
	Sessions         SessionChecker
	WriteEnabled     bool
	AccountAllowlist []string
}

var _ browserautomation.WorkerAdapter = Adapter{}
var _ browserautomation.WorkerPlanAdapter = Adapter{}
var _ browserautomation.WorkerSubmitGate = Adapter{}

func (a Adapter) Plan(ctx context.Context, run browserautomation.BrowserRpaRun) (json.RawMessage, error) {
	if a.Compiler == nil || a.Policies == nil {
		return nil, browserautomation.ErrEnvironmentUnavailable
	}
	policy, err := a.Policies.GetSitePolicy(ctx, run.OrganizationID, run.ProjectID, run.PolicyID)
	if err != nil {
		return nil, err
	}
	return a.Compiler.CompilePrepareV3(ctx, run, policy)
}

func (a Adapter) Prepare(ctx context.Context, run browserautomation.BrowserRpaRun) (browserautomation.PreparedPage, error) {
	if a.Sessions == nil {
		return browserautomation.PreparedPage{}, browserautomation.ErrEnvironmentUnavailable
	}
	if err := a.Sessions.Check(ctx, run); err != nil {
		return browserautomation.PreparedPage{}, err
	}
	plan, err := a.Plan(ctx, run)
	if err != nil {
		return browserautomation.PreparedPage{}, err
	}
	digest := sha256.Sum256(plan)
	return browserautomation.PreparedPage{
		BeforeFacts: map[string]string{
			"execution_driver": string(browserautomation.ExecutionDriverOceanEngineWebAPI),
			"contract_version": ContractVersion,
			"account_match":    "true",
		},
		Readback: map[string]string{
			"compiled_input_sha256": hex.EncodeToString(digest[:]),
			"write_gate":            a.writeGate(run),
		},
		DiffKeys:        []string{},
		PageRef:         "oceanengine-web-api://prepare/" + run.ID,
		SelectorVersion: SelectorVersion,
		ActionVersion:   ActionVersion,
	}, nil
}

func (a Adapter) CheckSubmit(run browserautomation.BrowserRpaRun) error {
	if !a.WriteEnabled {
		return ErrWriteDisabled
	}
	if !slices.Contains(a.AccountAllowlist, run.AccountID) {
		return ErrAccountNotAllowed
	}
	return ErrContractNotCaptured
}

func (a Adapter) Submit(context.Context, browserautomation.BrowserRpaRun, browserautomation.ControlledActionAttempt, string) (browserautomation.WorkerOutcome, browserautomation.PreparedPage, error) {
	return browserautomation.WorkerFailed, browserautomation.PreparedPage{}, ErrContractNotCaptured
}

func (a Adapter) writeGate(run browserautomation.BrowserRpaRun) string {
	if err := a.CheckSubmit(run); err != nil {
		return strings.TrimPrefix(fmt.Sprint(err), "Ocean Engine ")
	}
	return "ready"
}
