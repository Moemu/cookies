package computeruse

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const FinalConfirmationTTL = 5 * time.Minute

type IDGenerator func(string) (string, error)

type Service struct {
	Repository Repository
	NewID      IDGenerator
	Now        func() time.Time
}

type CreateRunRequest struct {
	Run ComputerUseRun
}

func (s Service) CreateRun(ctx context.Context, request CreateRunRequest) (ComputerUseRun, bool, error) {
	if s.Repository == nil || request.Run.State != RunQueued || request.Run.BlockingReason != "" || request.Run.Paused || request.Run.TakeoverActive {
		return ComputerUseRun{}, false, ErrInvalidContract
	}
	if err := request.Run.Validate(); err != nil {
		return ComputerUseRun{}, false, err
	}
	if _, active, err := s.Repository.ActiveKillSwitch(ctx, request.Run.OrganizationID, request.Run.Platform); err != nil {
		return ComputerUseRun{}, false, err
	} else if active {
		return ComputerUseRun{}, false, ErrKillSwitchActive
	}
	return s.Repository.CreateRun(ctx, request.Run)
}

func (s Service) TransitionRun(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, runID string, expectedVersion int64, next RunState, reason BlockingReason) (ComputerUseRun, error) {
	current, err := s.Repository.GetRun(ctx, organizationID, projectID, runID)
	if err != nil {
		return ComputerUseRun{}, err
	}
	if current.Version != expectedVersion {
		return ComputerUseRun{}, ErrVersionConflict
	}
	if !CanTransition(current.State, next) {
		return ComputerUseRun{}, ErrInvalidTransition
	}
	if _, active, err := s.Repository.ActiveKillSwitch(ctx, organizationID, current.Platform); err != nil {
		return ComputerUseRun{}, err
	} else if active && next != RunCancelled && next != RunFailed {
		return ComputerUseRun{}, ErrKillSwitchActive
	}
	return s.Repository.TransitionRun(ctx, organizationID, projectID, runID, expectedVersion, next, reason, s.now())
}

type IssuedConfirmation struct {
	Confirmation FinalConfirmation `json:"confirmation"`
	Token        string            `json:"token"`
}

func (s Service) IssueFinalConfirmation(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, runID, bindingHash, actor string) (IssuedConfirmation, error) {
	run, err := s.Repository.GetRun(ctx, organizationID, projectID, runID)
	if err != nil {
		return IssuedConfirmation{}, err
	}
	if run.State != RunAwaitingConfirmation || bindingHash != run.Authority.ApprovalActionHash || strings.TrimSpace(actor) == "" {
		return IssuedConfirmation{}, ErrConfirmationInvalid
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return IssuedConfirmation{}, err
	}
	token := hex.EncodeToString(tokenBytes)
	digest := sha256.Sum256([]byte(token))
	id, err := s.newID("cuc")
	if err != nil {
		return IssuedConfirmation{}, err
	}
	now := s.now()
	confirmation := FinalConfirmation{SchemaVersion: ConfirmationSchemaV1, ID: id, OrganizationID: organizationID, ProjectID: projectID, RunID: runID, BindingHash: bindingHash, TokenDigest: hex.EncodeToString(digest[:]), IssuedBy: actor, IssuedAt: now, ExpiresAt: now.Add(FinalConfirmationTTL), Version: 1}
	confirmation, err = s.Repository.IssueConfirmation(ctx, confirmation)
	if err != nil {
		return IssuedConfirmation{}, err
	}
	return IssuedConfirmation{Confirmation: confirmation, Token: token}, nil
}

type AuthorizeActionRequest struct {
	OrganizationID contract.OrganizationID
	ProjectID      contract.ProjectID
	RunID          string
	StepID         string
	ConfirmationID string
	Token          string
	LeaseID        string
	FencingToken   int64
	IdempotencyKey string
}

// AuthorizeAction performs the final fail-closed check. The repository must
// consume the one-time token and persist the attempt in the same transaction.
func (s Service) AuthorizeAction(ctx context.Context, request AuthorizeActionRequest) (ControlledActionAttempt, error) {
	run, err := s.Repository.GetRun(ctx, request.OrganizationID, request.ProjectID, request.RunID)
	if err != nil {
		return ControlledActionAttempt{}, err
	}
	now := s.now()
	if run.State != RunAwaitingConfirmation || run.Paused || run.TakeoverActive {
		return ControlledActionAttempt{}, ErrConfirmationInvalid
	}
	if _, active, err := s.Repository.ActiveKillSwitch(ctx, request.OrganizationID, run.Platform); err != nil {
		return ControlledActionAttempt{}, err
	} else if active {
		return ControlledActionAttempt{}, ErrKillSwitchActive
	}
	lease, err := s.Repository.GetLease(ctx, request.OrganizationID, request.ProjectID, request.LeaseID)
	if err != nil || lease.RunID != run.ID || lease.FencingToken != request.FencingToken || !lease.ValidAt(now) {
		return ControlledActionAttempt{}, ErrLeaseUnavailable
	}
	digest := sha256.Sum256([]byte(request.Token))
	attemptID, err := s.newID("cua")
	if err != nil {
		return ControlledActionAttempt{}, err
	}
	attempt := ControlledActionAttempt{ID: attemptID, OrganizationID: request.OrganizationID, ProjectID: request.ProjectID, RunID: run.ID, StepID: request.StepID, ConfirmationID: request.ConfirmationID, ApprovalID: run.Authority.ApprovalID, LeaseID: lease.ID, FencingToken: lease.FencingToken, ActionHash: run.Authority.ApprovalActionHash, IdempotencyKey: request.IdempotencyKey, Status: "authorized", CreatedAt: now}
	return s.Repository.AuthorizeControlledAction(ctx, FinalConfirmation{ID: request.ConfirmationID, OrganizationID: request.OrganizationID, ProjectID: request.ProjectID, RunID: run.ID, BindingHash: run.Authority.ApprovalActionHash}, hex.EncodeToString(digest[:]), lease, attempt, now)
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s Service) newID(prefix string) (string, error) {
	if s.NewID != nil {
		return s.NewID(prefix)
	}
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(bytes), nil
}
