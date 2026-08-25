package browserautomation

import (
	"context"
	"errors"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

var (
	ErrNotFound            = errors.New("browser-rpa resource not found")
	ErrVersionConflict     = errors.New("browser-rpa resource version conflict")
	ErrIdempotencyConflict = errors.New("browser-rpa idempotency key conflict")
	ErrLeaseUnavailable    = errors.New("browser-rpa profile lease unavailable")
	ErrConfirmationInvalid = errors.New("browser-rpa final confirmation invalid")
	ErrKillSwitchActive    = errors.New("browser-rpa kill switch active")
)

type Repository interface {
	CreateRun(context.Context, BrowserRpaRun) (BrowserRpaRun, bool, error)
	GetRun(context.Context, contract.OrganizationID, contract.ProjectID, string) (BrowserRpaRun, error)
	TransitionRun(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, RunState, BlockingReason, time.Time) (BrowserRpaRun, error)
	SetRunControl(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, RunState, bool, bool, BlockingReason, time.Time) (BrowserRpaRun, error)
	CreateEnvironment(context.Context, ExecutionEnvironment) (ExecutionEnvironment, error)
	GetEnvironment(context.Context, contract.OrganizationID, contract.ProjectID, string) (ExecutionEnvironment, error)
	CreateBrowserProfile(context.Context, BrowserProfile) (BrowserProfile, error)
	GetBrowserProfile(context.Context, contract.OrganizationID, contract.ProjectID, string) (BrowserProfile, error)
	CreateSitePolicy(context.Context, SitePolicy) (SitePolicy, error)
	GetSitePolicy(context.Context, contract.OrganizationID, contract.ProjectID, string) (SitePolicy, error)
	PutStep(context.Context, contract.OrganizationID, contract.ProjectID, RunStep) error
	ListSteps(context.Context, contract.OrganizationID, contract.ProjectID, string) ([]RunStep, error)
	AcquireLease(context.Context, SessionLease) (SessionLease, error)
	AcquireRunLease(context.Context, BrowserRpaRun, int64, SessionLease, time.Time) (BrowserRpaRun, SessionLease, error)
	GetLease(context.Context, contract.OrganizationID, contract.ProjectID, string) (SessionLease, error)
	HeartbeatLease(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, int64, time.Time, time.Time, time.Time) (SessionLease, error)
	ReleaseLease(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, int64, time.Time) (SessionLease, error)
	ReleaseRunLease(context.Context, BrowserRpaRun, int64, SessionLease, int64, int64, time.Time) (BrowserRpaRun, SessionLease, error)
	PutKillSwitch(context.Context, KillSwitch, int64) (KillSwitch, error)
	ActiveKillSwitch(context.Context, contract.OrganizationID, Platform) (KillSwitch, bool, error)
	IssueConfirmation(context.Context, FinalConfirmation) (FinalConfirmation, error)
	AuthorizeControlledAction(context.Context, FinalConfirmation, string, SessionLease, ControlledActionAttempt, time.Time) (ControlledActionAttempt, error)
	CompleteControlledAction(context.Context, contract.OrganizationID, contract.ProjectID, string, string) error
	AuthorizeTakeoverAction(context.Context, BrowserRpaRun, int64, FinalConfirmation, string, SessionLease, ControlledActionAttempt, RunStep, Evidence, RunEvent, time.Time) (BrowserRpaRun, ControlledActionAttempt, error)
	AppendEvent(context.Context, RunEvent) error
	AppendEvidence(context.Context, Evidence) error
	RecordTakeoverEvidence(context.Context, BrowserRpaRun, int64, RunStep, Evidence, RunEvent, time.Time) (BrowserRpaRun, error)
	RecordTakeoverOutcome(context.Context, BrowserRpaRun, int64, string, string, RunState, BlockingReason, RunStep, Evidence, RunEvent, time.Time) (BrowserRpaRun, error)
	ListEvents(context.Context, contract.OrganizationID, contract.ProjectID, string) ([]RunEvent, error)
	ListEvidence(context.Context, contract.OrganizationID, contract.ProjectID, string) ([]Evidence, error)
}
