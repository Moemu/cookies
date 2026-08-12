package computeruse

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type MySQLRepository struct{ DB *sql.DB }

func (r MySQLRepository) CreateRun(ctx context.Context, value ComputerUseRun) (ComputerUseRun, bool, error) {
	authority, err := json.Marshal(value.Authority)
	if err != nil {
		return ComputerUseRun{}, false, err
	}
	_, err = r.DB.ExecContext(ctx, `INSERT INTO computer_use_runs
		(id,organization_id,project_id,platform,account_id,authority_json,environment_id,profile_id,lease_id,policy_id,state,blocking_reason,paused,takeover_active,version,idempotency_key,request_hash,created_by,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, value.ID, value.OrganizationID, value.ProjectID, value.Platform, value.AccountID, authority, value.EnvironmentID, value.ProfileID, nullableString(value.LeaseID), value.PolicyID, value.State, nullableString(string(value.BlockingReason)), value.Paused, value.TakeoverActive, value.Version, value.IdempotencyKey, value.RequestHash, value.CreatedBy, value.CreatedAt, value.UpdatedAt)
	if err == nil {
		return value, false, nil
	}
	existing, getErr := r.getRunByIdempotency(ctx, value.OrganizationID, value.ProjectID, value.IdempotencyKey)
	if getErr != nil {
		return ComputerUseRun{}, false, err
	}
	if existing.RequestHash != value.RequestHash {
		return ComputerUseRun{}, false, ErrIdempotencyConflict
	}
	return existing, true, nil
}

func (r MySQLRepository) GetRun(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (ComputerUseRun, error) {
	return scanRun(r.DB.QueryRowContext(ctx, runSelect+` WHERE organization_id=? AND project_id=? AND id=?`, org, project, id))
}

func (r MySQLRepository) getRunByIdempotency(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, key string) (ComputerUseRun, error) {
	return scanRun(r.DB.QueryRowContext(ctx, runSelect+` WHERE organization_id=? AND project_id=? AND idempotency_key=?`, org, project, key))
}

func (r MySQLRepository) TransitionRun(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, id string, expected int64, state RunState, reason BlockingReason, now time.Time) (ComputerUseRun, error) {
	result, err := r.DB.ExecContext(ctx, `UPDATE computer_use_runs SET state=?,blocking_reason=?,version=version+1,updated_at=? WHERE organization_id=? AND project_id=? AND id=? AND version=?`, state, nullableString(string(reason)), now, org, project, id, expected)
	if err != nil {
		return ComputerUseRun{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return ComputerUseRun{}, err
	}
	if affected != 1 {
		return ComputerUseRun{}, ErrVersionConflict
	}
	return r.GetRun(ctx, org, project, id)
}

func (r MySQLRepository) SetRunControl(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, id string, expected int64, state RunState, paused, takeover bool, reason BlockingReason, now time.Time) (ComputerUseRun, error) {
	result, err := r.DB.ExecContext(ctx, `UPDATE computer_use_runs SET state=?,paused=?,takeover_active=?,blocking_reason=?,version=version+1,updated_at=? WHERE organization_id=? AND project_id=? AND id=? AND version=?`, state, paused, takeover, nullableString(string(reason)), now, org, project, id, expected)
	if err != nil {
		return ComputerUseRun{}, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return ComputerUseRun{}, ErrVersionConflict
	}
	return r.GetRun(ctx, org, project, id)
}

func (r MySQLRepository) PutStep(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, value RunStep) error {
	_, err := r.DB.ExecContext(ctx, `INSERT INTO computer_use_run_steps (id,organization_id,project_id,run_id,sequence_number,workflow_step_id,action,status,blocking_reason,attempt,version) VALUES (?,?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE status=VALUES(status),blocking_reason=VALUES(blocking_reason),attempt=VALUES(attempt),version=VALUES(version)`, value.ID, org, project, value.RunID, value.Sequence, value.WorkflowStepID, value.Action, value.Status, nullableString(string(value.BlockingReason)), value.Attempt, value.Version)
	return err
}
func (r MySQLRepository) ListSteps(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, runID string) ([]RunStep, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id,run_id,sequence_number,workflow_step_id,action,status,COALESCE(blocking_reason,''),attempt,version FROM computer_use_run_steps WHERE organization_id=? AND project_id=? AND run_id=? ORDER BY sequence_number`, org, project, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []RunStep{}
	for rows.Next() {
		var v RunStep
		if err := rows.Scan(&v.ID, &v.RunID, &v.Sequence, &v.WorkflowStepID, &v.Action, &v.Status, &v.BlockingReason, &v.Attempt, &v.Version); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

func (r MySQLRepository) AcquireLease(ctx context.Context, value SessionLease) (SessionLease, error) {
	activeKey := string(value.OrganizationID) + ":" + string(value.ProjectID) + ":" + string(value.Platform) + ":" + value.AccountID + ":" + value.ProfileID
	_, err := r.DB.ExecContext(ctx, `INSERT INTO computer_use_session_leases (id,organization_id,project_id,run_id,environment_id,profile_id,platform,account_id,holder,active_lock_key,fencing_token,version,expires_at,heartbeat_deadline,released_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, value.ID, value.OrganizationID, value.ProjectID, value.RunID, value.EnvironmentID, value.ProfileID, value.Platform, value.AccountID, value.Holder, activeKey, value.FencingToken, value.Version, value.ExpiresAt, value.HeartbeatDeadline, value.ReleasedAt)
	if err != nil {
		return SessionLease{}, ErrLeaseUnavailable
	}
	return value, nil
}

func (r MySQLRepository) GetLease(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (SessionLease, error) {
	return scanLease(r.DB.QueryRowContext(ctx, leaseSelect+` WHERE organization_id=? AND project_id=? AND id=?`, org, project, id))
}

func (r MySQLRepository) HeartbeatLease(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, id string, expectedVersion, fencingToken int64, now, expiresAt, heartbeatDeadline time.Time) (SessionLease, error) {
	result, err := r.DB.ExecContext(ctx, `UPDATE computer_use_session_leases SET expires_at=?,heartbeat_deadline=?,version=version+1 WHERE organization_id=? AND project_id=? AND id=? AND version=? AND fencing_token=? AND released_at IS NULL AND expires_at>? AND heartbeat_deadline>?`, expiresAt, heartbeatDeadline, org, project, id, expectedVersion, fencingToken, now, now)
	if err != nil {
		return SessionLease{}, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return SessionLease{}, ErrVersionConflict
	}
	return r.GetLease(ctx, org, project, id)
}

func (r MySQLRepository) ReleaseLease(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, id string, expectedVersion, fencingToken int64, now time.Time) (SessionLease, error) {
	result, err := r.DB.ExecContext(ctx, `UPDATE computer_use_session_leases SET active_lock_key=NULL,released_at=?,version=version+1 WHERE organization_id=? AND project_id=? AND id=? AND version=? AND fencing_token=? AND released_at IS NULL`, now, org, project, id, expectedVersion, fencingToken)
	if err != nil {
		return SessionLease{}, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return SessionLease{}, ErrVersionConflict
	}
	return r.GetLease(ctx, org, project, id)
}

func (r MySQLRepository) PutKillSwitch(ctx context.Context, value KillSwitch, expected int64) (KillSwitch, error) {
	scopeKey := "*"
	if value.Scope == KillSwitchPlatform {
		scopeKey = string(value.Platform)
	}
	if value.Scope == KillSwitchOrganization {
		scopeKey = string(value.OrganizationID)
	}
	if expected == 0 {
		_, err := r.DB.ExecContext(ctx, `INSERT INTO computer_use_kill_switches (id,scope,scope_key,organization_id,platform,active,reason,version,updated_by,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`, value.ID, value.Scope, scopeKey, nullableString(string(value.OrganizationID)), nullableString(string(value.Platform)), value.Active, value.Reason, value.Version, value.UpdatedBy, value.UpdatedAt)
		if err != nil {
			return KillSwitch{}, ErrVersionConflict
		}
		return value, nil
	}
	result, err := r.DB.ExecContext(ctx, `UPDATE computer_use_kill_switches SET active=?,reason=?,version=version+1,updated_by=?,updated_at=? WHERE scope=? AND scope_key=? AND version=?`, value.Active, value.Reason, value.UpdatedBy, value.UpdatedAt, value.Scope, scopeKey, expected)
	if err != nil {
		return KillSwitch{}, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return KillSwitch{}, ErrVersionConflict
	}
	value.Version = expected + 1
	return value, nil
}

func (r MySQLRepository) ActiveKillSwitch(ctx context.Context, org contract.OrganizationID, platform Platform) (KillSwitch, bool, error) {
	row := r.DB.QueryRowContext(ctx, killSelect+` WHERE active=TRUE AND ((scope='global' AND scope_key='*') OR (scope='platform' AND scope_key=?) OR (scope='organization' AND scope_key=?)) ORDER BY FIELD(scope,'global','platform','organization') LIMIT 1`, platform, org)
	value, err := scanKill(row)
	if errors.Is(err, ErrNotFound) {
		return KillSwitch{}, false, nil
	}
	return value, err == nil, err
}

func (r MySQLRepository) IssueConfirmation(ctx context.Context, value FinalConfirmation) (FinalConfirmation, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return FinalConfirmation{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `UPDATE computer_use_final_confirmations SET invalidated_at=?,version=version+1 WHERE organization_id=? AND project_id=? AND run_id=? AND consumed_at IS NULL AND rejected_at IS NULL AND invalidated_at IS NULL`, value.IssuedAt, value.OrganizationID, value.ProjectID, value.RunID)
	if err != nil {
		return FinalConfirmation{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO computer_use_final_confirmations (id,organization_id,project_id,run_id,binding_hash,token_digest,issued_by,issued_at,expires_at,consumed_at,rejected_at,invalidated_at,version) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, value.ID, value.OrganizationID, value.ProjectID, value.RunID, value.BindingHash, value.TokenDigest, value.IssuedBy, value.IssuedAt, value.ExpiresAt, value.ConsumedAt, value.RejectedAt, value.InvalidatedAt, value.Version)
	if err != nil {
		return FinalConfirmation{}, err
	}
	if err := tx.Commit(); err != nil {
		return FinalConfirmation{}, err
	}
	return value, nil
}

func (r MySQLRepository) AuthorizeControlledAction(ctx context.Context, identity FinalConfirmation, digest string, lease SessionLease, attempt ControlledActionAttempt, now time.Time) (ControlledActionAttempt, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return ControlledActionAttempt{}, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id FROM computer_use_kill_switches WHERE active=TRUE AND ((scope='global' AND scope_key='*') OR (scope='platform' AND scope_key=?) OR (scope='organization' AND scope_key=?)) FOR UPDATE`, lease.Platform, identity.OrganizationID)
	if err != nil {
		return ControlledActionAttempt{}, err
	}
	active := rows.Next()
	rows.Close()
	if active {
		return ControlledActionAttempt{}, ErrKillSwitchActive
	}
	storedLease, err := scanLease(tx.QueryRowContext(ctx, leaseSelect+` WHERE organization_id=? AND project_id=? AND id=? FOR UPDATE`, identity.OrganizationID, identity.ProjectID, lease.ID))
	if err != nil || storedLease.RunID != identity.RunID || storedLease.FencingToken != lease.FencingToken || !storedLease.ValidAt(now) {
		return ControlledActionAttempt{}, ErrLeaseUnavailable
	}
	var confirmation FinalConfirmation
	var consumed, rejected, invalidated sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT id,organization_id,project_id,run_id,binding_hash,token_digest,issued_by,issued_at,expires_at,consumed_at,rejected_at,invalidated_at,version FROM computer_use_final_confirmations WHERE organization_id=? AND project_id=? AND id=? FOR UPDATE`, identity.OrganizationID, identity.ProjectID, identity.ID).Scan(&confirmation.ID, &confirmation.OrganizationID, &confirmation.ProjectID, &confirmation.RunID, &confirmation.BindingHash, &confirmation.TokenDigest, &confirmation.IssuedBy, &confirmation.IssuedAt, &confirmation.ExpiresAt, &consumed, &rejected, &invalidated, &confirmation.Version)
	if err != nil {
		return ControlledActionAttempt{}, ErrConfirmationInvalid
	}
	confirmation.SchemaVersion = ConfirmationSchemaV1
	confirmation.ConsumedAt = timePtr(consumed)
	confirmation.RejectedAt = timePtr(rejected)
	confirmation.InvalidatedAt = timePtr(invalidated)
	if confirmation.RunID != identity.RunID || confirmation.BindingHash != identity.BindingHash || confirmation.TokenDigest != digest || !confirmation.UsableAt(now) {
		return ControlledActionAttempt{}, ErrConfirmationInvalid
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO computer_use_controlled_action_attempts (id,organization_id,project_id,run_id,step_id,confirmation_id,approval_id,lease_id,fencing_token,action_hash,idempotency_key,status,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, attempt.ID, attempt.OrganizationID, attempt.ProjectID, attempt.RunID, attempt.StepID, attempt.ConfirmationID, attempt.ApprovalID, attempt.LeaseID, attempt.FencingToken, attempt.ActionHash, attempt.IdempotencyKey, attempt.Status, attempt.CreatedAt)
	if err != nil {
		var existingHash string
		scanErr := tx.QueryRowContext(ctx, `SELECT action_hash FROM computer_use_controlled_action_attempts WHERE organization_id=? AND project_id=? AND idempotency_key=?`, attempt.OrganizationID, attempt.ProjectID, attempt.IdempotencyKey).Scan(&existingHash)
		if scanErr == nil && existingHash != attempt.ActionHash {
			return ControlledActionAttempt{}, ErrIdempotencyConflict
		}
		if scanErr == nil {
			return ControlledActionAttempt{}, ErrConfirmationInvalid
		}
		return ControlledActionAttempt{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE computer_use_final_confirmations SET consumed_at=?,version=version+1 WHERE organization_id=? AND project_id=? AND id=? AND version=? AND consumed_at IS NULL AND rejected_at IS NULL AND invalidated_at IS NULL`, now, identity.OrganizationID, identity.ProjectID, identity.ID, confirmation.Version)
	if err != nil {
		return ControlledActionAttempt{}, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return ControlledActionAttempt{}, ErrConfirmationInvalid
	}
	if err := tx.Commit(); err != nil {
		return ControlledActionAttempt{}, err
	}
	return attempt, nil
}

func (r MySQLRepository) AppendEvent(ctx context.Context, value RunEvent) error {
	_, err := r.DB.ExecContext(ctx, `INSERT INTO computer_use_events (id,organization_id,project_id,run_id,sequence_number,kind,summary,actor,created_at) VALUES (?,?,?,?,?,?,?,?,?)`, value.ID, value.OrganizationID, value.ProjectID, value.RunID, value.Sequence, value.Kind, value.Summary, value.Actor, value.CreatedAt)
	return err
}
func (r MySQLRepository) AppendEvidence(ctx context.Context, value Evidence) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = r.DB.ExecContext(ctx, `INSERT INTO computer_use_evidence (id,organization_id,project_id,run_id,step_id,evidence_json,object_fingerprint,skill_version,selector_version,action_version,redaction_version,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, value.ID, value.OrganizationID, value.ProjectID, value.RunID, value.StepID, payload, value.ObjectFingerprint, value.SkillVersion, value.SelectorVersion, value.ActionVersion, value.RedactionVersion, value.CreatedAt)
	return err
}
func (r MySQLRepository) ListEvents(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, runID string) ([]RunEvent, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id,organization_id,project_id,run_id,sequence_number,kind,summary,actor,created_at FROM computer_use_events WHERE organization_id=? AND project_id=? AND run_id=? ORDER BY sequence_number`, org, project, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []RunEvent{}
	for rows.Next() {
		var v RunEvent
		if err := rows.Scan(&v.ID, &v.OrganizationID, &v.ProjectID, &v.RunID, &v.Sequence, &v.Kind, &v.Summary, &v.Actor, &v.CreatedAt); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, rows.Err()
}
func (r MySQLRepository) ListEvidence(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, runID string) ([]Evidence, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT evidence_json FROM computer_use_evidence WHERE organization_id=? AND project_id=? AND run_id=? ORDER BY created_at,id`, org, project, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []Evidence{}
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var v Evidence
		if err := json.Unmarshal(payload, &v); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

const runSelect = `SELECT id,organization_id,project_id,platform,account_id,authority_json,environment_id,profile_id,COALESCE(lease_id,''),policy_id,state,COALESCE(blocking_reason,''),paused,takeover_active,version,idempotency_key,request_hash,created_by,created_at,updated_at FROM computer_use_runs`

func scanRun(row interface{ Scan(...any) error }) (ComputerUseRun, error) {
	var v ComputerUseRun
	var authority []byte
	err := row.Scan(&v.ID, &v.OrganizationID, &v.ProjectID, &v.Platform, &v.AccountID, &authority, &v.EnvironmentID, &v.ProfileID, &v.LeaseID, &v.PolicyID, &v.State, &v.BlockingReason, &v.Paused, &v.TakeoverActive, &v.Version, &v.IdempotencyKey, &v.RequestHash, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ComputerUseRun{}, ErrNotFound
	}
	if err != nil {
		return ComputerUseRun{}, err
	}
	if err := json.Unmarshal(authority, &v.Authority); err != nil {
		return ComputerUseRun{}, fmt.Errorf("decode computer-use authority: %w", err)
	}
	return v, nil
}

const leaseSelect = `SELECT id,organization_id,project_id,run_id,environment_id,profile_id,platform,account_id,holder,fencing_token,version,expires_at,heartbeat_deadline,released_at FROM computer_use_session_leases`

func scanLease(row interface{ Scan(...any) error }) (SessionLease, error) {
	var v SessionLease
	var released sql.NullTime
	err := row.Scan(&v.ID, &v.OrganizationID, &v.ProjectID, &v.RunID, &v.EnvironmentID, &v.ProfileID, &v.Platform, &v.AccountID, &v.Holder, &v.FencingToken, &v.Version, &v.ExpiresAt, &v.HeartbeatDeadline, &released)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionLease{}, ErrNotFound
	}
	if err != nil {
		return SessionLease{}, err
	}
	v.ReleasedAt = timePtr(released)
	return v, nil
}

const killSelect = `SELECT id,scope,organization_id,platform,active,reason,version,updated_by,updated_at FROM computer_use_kill_switches`

func scanKill(row interface{ Scan(...any) error }) (KillSwitch, error) {
	var v KillSwitch
	var org, platform sql.NullString
	err := row.Scan(&v.ID, &v.Scope, &org, &platform, &v.Active, &v.Reason, &v.Version, &v.UpdatedBy, &v.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return KillSwitch{}, ErrNotFound
	}
	if err != nil {
		return KillSwitch{}, err
	}
	v.OrganizationID = contract.OrganizationID(org.String)
	v.Platform = Platform(platform.String)
	return v, nil
}
func nullableString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
func timePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	value := v.Time
	return &value
}
