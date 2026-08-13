package computeruse

import (
	"context"
	"errors"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

var (
	ErrNotFound            = errors.New("computer-use resource not found")
	ErrVersionConflict     = errors.New("computer-use resource version conflict")
	ErrIdempotencyConflict = errors.New("computer-use idempotency key conflict")
	ErrLeaseUnavailable    = errors.New("computer-use profile lease unavailable")
	ErrConfirmationInvalid = errors.New("computer-use final confirmation invalid")
	ErrKillSwitchActive    = errors.New("computer-use kill switch active")
)

type Repository interface {
	CreateRun(context.Context, ComputerUseRun) (ComputerUseRun, bool, error)
	GetRun(context.Context, contract.OrganizationID, contract.ProjectID, string) (ComputerUseRun, error)
	TransitionRun(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, RunState, BlockingReason, time.Time) (ComputerUseRun, error)
	SetRunControl(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, RunState, bool, bool, BlockingReason, time.Time) (ComputerUseRun, error)
	CreateEnvironment(context.Context, ExecutionEnvironment) (ExecutionEnvironment, error)
	GetEnvironment(context.Context, contract.OrganizationID, contract.ProjectID, string) (ExecutionEnvironment, error)
	CreateBrowserProfile(context.Context, BrowserProfile) (BrowserProfile, error)
	GetBrowserProfile(context.Context, contract.OrganizationID, contract.ProjectID, string) (BrowserProfile, error)
	CreateSitePolicy(context.Context, SitePolicy) (SitePolicy, error)
	GetSitePolicy(context.Context, contract.OrganizationID, contract.ProjectID, string) (SitePolicy, error)
	PutStep(context.Context, contract.OrganizationID, contract.ProjectID, RunStep) error
	ListSteps(context.Context, contract.OrganizationID, contract.ProjectID, string) ([]RunStep, error)
	AcquireLease(context.Context, SessionLease) (SessionLease, error)
	AcquireRunLease(context.Context, ComputerUseRun, int64, SessionLease, time.Time) (ComputerUseRun, SessionLease, error)
	GetLease(context.Context, contract.OrganizationID, contract.ProjectID, string) (SessionLease, error)
	HeartbeatLease(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, int64, time.Time, time.Time, time.Time) (SessionLease, error)
	ReleaseLease(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, int64, time.Time) (SessionLease, error)
	ReleaseRunLease(context.Context, ComputerUseRun, int64, SessionLease, int64, int64, time.Time) (ComputerUseRun, SessionLease, error)
	PutKillSwitch(context.Context, KillSwitch, int64) (KillSwitch, error)
	ActiveKillSwitch(context.Context, contract.OrganizationID, Platform) (KillSwitch, bool, error)
	IssueConfirmation(context.Context, FinalConfirmation) (FinalConfirmation, error)
	AuthorizeControlledAction(context.Context, FinalConfirmation, string, SessionLease, ControlledActionAttempt, time.Time) (ControlledActionAttempt, error)
	AppendEvent(context.Context, RunEvent) error
	AppendEvidence(context.Context, Evidence) error
	RecordTakeoverEvidence(context.Context, ComputerUseRun, int64, RunStep, Evidence, RunEvent, time.Time) (ComputerUseRun, error)
	ListEvents(context.Context, contract.OrganizationID, contract.ProjectID, string) ([]RunEvent, error)
	ListEvidence(context.Context, contract.OrganizationID, contract.ProjectID, string) ([]Evidence, error)
}
