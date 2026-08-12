package computeruse

import (
	"context"
	"sync"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type MemoryRepository struct {
	mu             sync.Mutex
	runs           map[string]ComputerUseRun
	runKeys        map[string]string
	leases         map[string]SessionLease
	activeProfiles map[string]string
	killSwitches   map[string]KillSwitch
	confirmations  map[string]FinalConfirmation
	attempts       map[string]ControlledActionAttempt
	events         []RunEvent
	evidence       []Evidence
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{runs: map[string]ComputerUseRun{}, runKeys: map[string]string{}, leases: map[string]SessionLease{}, activeProfiles: map[string]string{}, killSwitches: map[string]KillSwitch{}, confirmations: map[string]FinalConfirmation{}, attempts: map[string]ControlledActionAttempt{}}
}

func scopeKey(org contract.OrganizationID, project contract.ProjectID, id string) string {
	return string(org) + "\x00" + string(project) + "\x00" + id
}
func killKey(scope KillSwitchScope, value string) string { return string(scope) + "\x00" + value }

func (r *MemoryRepository) CreateRun(_ context.Context, value ComputerUseRun) (ComputerUseRun, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := scopeKey(value.OrganizationID, value.ProjectID, value.IdempotencyKey)
	if id, ok := r.runKeys[key]; ok {
		existing := r.runs[scopeKey(value.OrganizationID, value.ProjectID, id)]
		if existing.RequestHash != value.RequestHash {
			return ComputerUseRun{}, false, ErrIdempotencyConflict
		}
		return existing, true, nil
	}
	r.runs[scopeKey(value.OrganizationID, value.ProjectID, value.ID)] = value
	r.runKeys[key] = value.ID
	return value, false, nil
}

func (r *MemoryRepository) GetRun(_ context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (ComputerUseRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.runs[scopeKey(org, project, id)]
	if !ok {
		return ComputerUseRun{}, ErrNotFound
	}
	return value, nil
}

func (r *MemoryRepository) TransitionRun(_ context.Context, org contract.OrganizationID, project contract.ProjectID, id string, expected int64, state RunState, reason BlockingReason, now time.Time) (ComputerUseRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := scopeKey(org, project, id)
	value, ok := r.runs[key]
	if !ok {
		return ComputerUseRun{}, ErrNotFound
	}
	if value.Version != expected {
		return ComputerUseRun{}, ErrVersionConflict
	}
	value.State, value.BlockingReason, value.Version, value.UpdatedAt = state, reason, value.Version+1, now
	r.runs[key] = value
	return value, nil
}

func (r *MemoryRepository) AcquireLease(_ context.Context, value SessionLease) (SessionLease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	profileKey := scopeKey(value.OrganizationID, value.ProjectID, value.ProfileID)
	if id, ok := r.activeProfiles[profileKey]; ok {
		if lease := r.leases[scopeKey(value.OrganizationID, value.ProjectID, id)]; lease.ReleasedAt == nil {
			return SessionLease{}, ErrLeaseUnavailable
		}
	}
	r.leases[scopeKey(value.OrganizationID, value.ProjectID, value.ID)] = value
	r.activeProfiles[profileKey] = value.ID
	return value, nil
}

func (r *MemoryRepository) GetLease(_ context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (SessionLease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.leases[scopeKey(org, project, id)]
	if !ok {
		return SessionLease{}, ErrNotFound
	}
	return value, nil
}

func (r *MemoryRepository) PutKillSwitch(_ context.Context, value KillSwitch, expected int64) (KillSwitch, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	keyValue := "*"
	if value.Scope == KillSwitchPlatform {
		keyValue = string(value.Platform)
	}
	if value.Scope == KillSwitchOrganization {
		keyValue = string(value.OrganizationID)
	}
	key := killKey(value.Scope, keyValue)
	current, ok := r.killSwitches[key]
	if ok && current.Version != expected {
		return KillSwitch{}, ErrVersionConflict
	}
	if !ok && expected != 0 {
		return KillSwitch{}, ErrVersionConflict
	}
	r.killSwitches[key] = value
	return value, nil
}

func (r *MemoryRepository) ActiveKillSwitch(_ context.Context, org contract.OrganizationID, platform Platform) (KillSwitch, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, key := range []string{killKey(KillSwitchGlobal, "*"), killKey(KillSwitchPlatform, string(platform)), killKey(KillSwitchOrganization, string(org))} {
		if value, ok := r.killSwitches[key]; ok && value.Active {
			return value, true, nil
		}
	}
	return KillSwitch{}, false, nil
}

func (r *MemoryRepository) IssueConfirmation(_ context.Context, value FinalConfirmation) (FinalConfirmation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, existing := range r.confirmations {
		if existing.OrganizationID == value.OrganizationID && existing.ProjectID == value.ProjectID && existing.RunID == value.RunID && existing.ConsumedAt == nil && existing.RejectedAt == nil && existing.InvalidatedAt == nil {
			invalidatedAt := value.IssuedAt
			existing.InvalidatedAt = &invalidatedAt
			existing.Version++
			r.confirmations[key] = existing
		}
	}
	r.confirmations[scopeKey(value.OrganizationID, value.ProjectID, value.ID)] = value
	return value, nil
}

func (r *MemoryRepository) AuthorizeControlledAction(_ context.Context, identity FinalConfirmation, digest string, lease SessionLease, attempt ControlledActionAttempt, now time.Time) (ControlledActionAttempt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := scopeKey(identity.OrganizationID, identity.ProjectID, identity.ID)
	confirmation, ok := r.confirmations[key]
	if !ok || confirmation.RunID != identity.RunID || confirmation.BindingHash != identity.BindingHash || confirmation.TokenDigest != digest || !confirmation.UsableAt(now) {
		return ControlledActionAttempt{}, ErrConfirmationInvalid
	}
	storedLease, ok := r.leases[scopeKey(lease.OrganizationID, lease.ProjectID, lease.ID)]
	if !ok || storedLease.FencingToken != lease.FencingToken || !storedLease.ValidAt(now) {
		return ControlledActionAttempt{}, ErrLeaseUnavailable
	}
	attemptKey := scopeKey(attempt.OrganizationID, attempt.ProjectID, attempt.IdempotencyKey)
	if existing, ok := r.attempts[attemptKey]; ok {
		if existing.ActionHash != attempt.ActionHash {
			return ControlledActionAttempt{}, ErrIdempotencyConflict
		}
		return existing, nil
	}
	confirmation.ConsumedAt = &now
	confirmation.Version++
	r.confirmations[key] = confirmation
	r.attempts[attemptKey] = attempt
	return attempt, nil
}

func (r *MemoryRepository) AppendEvent(_ context.Context, value RunEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, value)
	return nil
}
func (r *MemoryRepository) AppendEvidence(_ context.Context, value Evidence) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evidence = append(r.evidence, value)
	return nil
}
