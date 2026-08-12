package computeruse

import (
	"context"
	"sync"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type MemoryRepository struct {
	mu              sync.Mutex
	runs            map[string]ComputerUseRun
	runKeys         map[string]string
	leases          map[string]SessionLease
	activeProfiles  map[string]string
	environments    map[string]ExecutionEnvironment
	browserProfiles map[string]BrowserProfile
	policies        map[string]SitePolicy
	killSwitches    map[string]KillSwitch
	confirmations   map[string]FinalConfirmation
	attempts        map[string]ControlledActionAttempt
	events          []RunEvent
	evidence        []Evidence
	steps           []RunStep
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{runs: map[string]ComputerUseRun{}, runKeys: map[string]string{}, leases: map[string]SessionLease{}, activeProfiles: map[string]string{}, environments: map[string]ExecutionEnvironment{}, browserProfiles: map[string]BrowserProfile{}, policies: map[string]SitePolicy{}, killSwitches: map[string]KillSwitch{}, confirmations: map[string]FinalConfirmation{}, attempts: map[string]ControlledActionAttempt{}}
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

func (r *MemoryRepository) SetRunControl(_ context.Context, org contract.OrganizationID, project contract.ProjectID, id string, expected int64, state RunState, paused, takeover bool, reason BlockingReason, now time.Time) (ComputerUseRun, error) {
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
	value.State, value.Paused, value.TakeoverActive, value.BlockingReason, value.Version, value.UpdatedAt = state, paused, takeover, reason, value.Version+1, now
	r.runs[key] = value
	return value, nil
}

func (r *MemoryRepository) PutSitePolicy(value SitePolicy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.policies[scopeKey(value.OrganizationID, value.ProjectID, value.ID)] = value
}

func (r *MemoryRepository) PutEnvironment(value ExecutionEnvironment) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.environments[scopeKey(value.OrganizationID, value.ProjectID, value.ID)] = value
}

func (r *MemoryRepository) GetEnvironment(_ context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (ExecutionEnvironment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.environments[scopeKey(org, project, id)]
	if !ok {
		return ExecutionEnvironment{}, ErrNotFound
	}
	return value, nil
}

func (r *MemoryRepository) PutBrowserProfile(value BrowserProfile) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.browserProfiles[scopeKey(value.OrganizationID, value.ProjectID, value.ID)] = value
}

func (r *MemoryRepository) GetBrowserProfile(_ context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (BrowserProfile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.browserProfiles[scopeKey(org, project, id)]
	if !ok {
		return BrowserProfile{}, ErrNotFound
	}
	return value, nil
}

func (r *MemoryRepository) GetSitePolicy(_ context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (SitePolicy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.policies[scopeKey(org, project, id)]
	if !ok {
		return SitePolicy{}, ErrNotFound
	}
	return value, nil
}

func (r *MemoryRepository) PutStep(_ context.Context, org contract.OrganizationID, project contract.ProjectID, value RunStep) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.runs[scopeKey(org, project, value.RunID)]; !ok {
		return ErrNotFound
	}
	for index, step := range r.steps {
		if step.RunID == value.RunID && step.ID == value.ID {
			r.steps[index] = value
			return nil
		}
	}
	r.steps = append(r.steps, value)
	return nil
}

func (r *MemoryRepository) ListSteps(_ context.Context, org contract.OrganizationID, project contract.ProjectID, runID string) ([]RunStep, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.runs[scopeKey(org, project, runID)]; !ok {
		return nil, ErrNotFound
	}
	values := []RunStep{}
	for _, step := range r.steps {
		if step.RunID == runID {
			values = append(values, step)
		}
	}
	return values, nil
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

func (r *MemoryRepository) AcquireRunLease(_ context.Context, run ComputerUseRun, expected int64, lease SessionLease, now time.Time) (ComputerUseRun, SessionLease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	runKey := scopeKey(run.OrganizationID, run.ProjectID, run.ID)
	current, ok := r.runs[runKey]
	if !ok {
		return ComputerUseRun{}, SessionLease{}, ErrNotFound
	}
	if current.Version != expected || current.Version != run.Version || current.LeaseID != "" {
		return ComputerUseRun{}, SessionLease{}, ErrVersionConflict
	}
	profileKey := scopeKey(lease.OrganizationID, lease.ProjectID, lease.ProfileID)
	if id, ok := r.activeProfiles[profileKey]; ok {
		if active := r.leases[scopeKey(lease.OrganizationID, lease.ProjectID, id)]; active.ReleasedAt == nil {
			return ComputerUseRun{}, SessionLease{}, ErrLeaseUnavailable
		}
	}
	for _, existing := range r.leases {
		if existing.OrganizationID == lease.OrganizationID && existing.ProjectID == lease.ProjectID && existing.ProfileID == lease.ProfileID && existing.FencingToken >= lease.FencingToken {
			lease.FencingToken = existing.FencingToken + 1
		}
	}
	r.leases[scopeKey(lease.OrganizationID, lease.ProjectID, lease.ID)] = lease
	r.activeProfiles[profileKey] = lease.ID
	current.LeaseID = lease.ID
	current.Version++
	current.UpdatedAt = now
	r.runs[runKey] = current
	return current, lease, nil
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

func (r *MemoryRepository) HeartbeatLease(_ context.Context, org contract.OrganizationID, project contract.ProjectID, id string, expectedVersion, fencingToken int64, now, expiresAt, heartbeatDeadline time.Time) (SessionLease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := scopeKey(org, project, id)
	value, ok := r.leases[key]
	if !ok {
		return SessionLease{}, ErrNotFound
	}
	if value.Version != expectedVersion || value.FencingToken != fencingToken || !value.ValidAt(now) {
		return SessionLease{}, ErrVersionConflict
	}
	value.ExpiresAt = expiresAt
	value.HeartbeatDeadline = heartbeatDeadline
	value.Version++
	r.leases[key] = value
	return value, nil
}

func (r *MemoryRepository) ReleaseLease(_ context.Context, org contract.OrganizationID, project contract.ProjectID, id string, expectedVersion, fencingToken int64, now time.Time) (SessionLease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := scopeKey(org, project, id)
	value, ok := r.leases[key]
	if !ok {
		return SessionLease{}, ErrNotFound
	}
	if value.Version != expectedVersion || value.FencingToken != fencingToken || value.ReleasedAt != nil {
		return SessionLease{}, ErrVersionConflict
	}
	value.ReleasedAt = &now
	value.Version++
	r.leases[key] = value
	delete(r.activeProfiles, scopeKey(org, project, value.ProfileID))
	return value, nil
}

func (r *MemoryRepository) ReleaseRunLease(_ context.Context, run ComputerUseRun, expectedRunVersion int64, lease SessionLease, expectedLeaseVersion, fencingToken int64, now time.Time) (ComputerUseRun, SessionLease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	runKey := scopeKey(run.OrganizationID, run.ProjectID, run.ID)
	currentRun, ok := r.runs[runKey]
	if !ok {
		return ComputerUseRun{}, SessionLease{}, ErrNotFound
	}
	leaseKey := scopeKey(lease.OrganizationID, lease.ProjectID, lease.ID)
	currentLease, ok := r.leases[leaseKey]
	if !ok {
		return ComputerUseRun{}, SessionLease{}, ErrNotFound
	}
	if currentRun.Version != expectedRunVersion || currentRun.LeaseID != currentLease.ID || currentLease.RunID != currentRun.ID || currentLease.Version != expectedLeaseVersion || currentLease.FencingToken != fencingToken || currentLease.ReleasedAt != nil {
		return ComputerUseRun{}, SessionLease{}, ErrVersionConflict
	}
	currentLease.ReleasedAt = &now
	currentLease.Version++
	currentRun.LeaseID = ""
	currentRun.Version++
	currentRun.UpdatedAt = now
	r.leases[leaseKey] = currentLease
	r.runs[runKey] = currentRun
	profileKey := scopeKey(currentLease.OrganizationID, currentLease.ProjectID, currentLease.ProfileID)
	if r.activeProfiles[profileKey] == currentLease.ID {
		delete(r.activeProfiles, profileKey)
	}
	return currentRun, currentLease, nil
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

func (r *MemoryRepository) RecordTakeoverEvidence(_ context.Context, run ComputerUseRun, expected int64, step RunStep, evidence Evidence, event RunEvent, now time.Time) (ComputerUseRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := scopeKey(run.OrganizationID, run.ProjectID, run.ID)
	current, ok := r.runs[key]
	if !ok {
		return ComputerUseRun{}, ErrNotFound
	}
	if current.Version != expected || current.Version != run.Version {
		return ComputerUseRun{}, ErrVersionConflict
	}
	for _, existing := range r.steps {
		if existing.RunID == run.ID && (existing.ID == step.ID || existing.Sequence == step.Sequence) {
			return ComputerUseRun{}, ErrIdempotencyConflict
		}
	}
	current.Version++
	current.UpdatedAt = now
	r.runs[key] = current
	r.steps = append(r.steps, step)
	r.evidence = append(r.evidence, evidence)
	r.events = append(r.events, event)
	return current, nil
}
func (r *MemoryRepository) ListEvents(_ context.Context, org contract.OrganizationID, project contract.ProjectID, runID string) ([]RunEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.runs[scopeKey(org, project, runID)]; !ok {
		return nil, ErrNotFound
	}
	values := []RunEvent{}
	for _, value := range r.events {
		if value.OrganizationID == org && value.ProjectID == project && value.RunID == runID {
			values = append(values, value)
		}
	}
	return values, nil
}
func (r *MemoryRepository) ListEvidence(_ context.Context, org contract.OrganizationID, project contract.ProjectID, runID string) ([]Evidence, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.runs[scopeKey(org, project, runID)]; !ok {
		return nil, ErrNotFound
	}
	values := []Evidence{}
	for _, value := range r.evidence {
		if value.OrganizationID == org && value.ProjectID == project && value.RunID == runID {
			values = append(values, value)
		}
	}
	return values, nil
}
