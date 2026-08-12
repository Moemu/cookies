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
	AcquireLease(context.Context, SessionLease) (SessionLease, error)
	GetLease(context.Context, contract.OrganizationID, contract.ProjectID, string) (SessionLease, error)
	PutKillSwitch(context.Context, KillSwitch, int64) (KillSwitch, error)
	ActiveKillSwitch(context.Context, contract.OrganizationID, Platform) (KillSwitch, bool, error)
	IssueConfirmation(context.Context, FinalConfirmation) (FinalConfirmation, error)
	AuthorizeControlledAction(context.Context, FinalConfirmation, string, SessionLease, ControlledActionAttempt, time.Time) (ControlledActionAttempt, error)
	AppendEvent(context.Context, RunEvent) error
	AppendEvidence(context.Context, Evidence) error
}
