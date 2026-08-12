package delivery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func (r MySQLRepository) CreateControlledChangeSet(ctx context.Context, value ControlledChangeSet) (ControlledChangeSet, bool, error) {
	binding, err := json.Marshal(value.Binding)
	if err != nil {
		return ControlledChangeSet{}, false, err
	}
	_, err = r.DB.ExecContext(ctx, `INSERT INTO delivery_controlled_change_sets (id,organization_id,project_id,binding_json,action,budget_limit_minor,currency,status,canonical_hash,version,created_by,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, value.ID, value.OrganizationID, value.ProjectID, binding, value.Action, value.BudgetLimitMinor, value.Currency, value.Status, value.CanonicalHash, value.Version, value.CreatedBy, value.CreatedAt, value.UpdatedAt)
	if err == nil {
		return value, false, nil
	}
	existing, getErr := r.getControlledChangeSetByHash(ctx, value.OrganizationID, value.ProjectID, value.CanonicalHash)
	if getErr != nil {
		return ControlledChangeSet{}, false, err
	}
	return existing, true, nil
}

func (r MySQLRepository) GetControlledChangeSet(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (ControlledChangeSet, error) {
	return scanControlledChangeSet(r.DB.QueryRowContext(ctx, controlledChangeSetSelect+` WHERE organization_id=? AND project_id=? AND id=?`, org, project, id))
}
func (r MySQLRepository) getControlledChangeSetByHash(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, hash string) (ControlledChangeSet, error) {
	return scanControlledChangeSet(r.DB.QueryRowContext(ctx, controlledChangeSetSelect+` WHERE organization_id=? AND project_id=? AND canonical_hash=?`, org, project, hash))
}

func (r MySQLRepository) ApproveControlledChangeSet(ctx context.Context, change ControlledChangeSet, approval RemoteWriteApproval) (ControlledChangeSet, RemoteWriteApproval, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return ControlledChangeSet{}, RemoteWriteApproval{}, err
	}
	defer tx.Rollback()
	var status string
	var version int64
	var hash string
	err = tx.QueryRowContext(ctx, `SELECT status,version,canonical_hash FROM delivery_controlled_change_sets WHERE organization_id=? AND project_id=? AND id=? FOR UPDATE`, change.OrganizationID, change.ProjectID, change.ID).Scan(&status, &version, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return ControlledChangeSet{}, RemoteWriteApproval{}, ErrNotFound
	}
	if err != nil {
		return ControlledChangeSet{}, RemoteWriteApproval{}, err
	}
	if status != string(ControlledChangeSetReady) {
		return ControlledChangeSet{}, RemoteWriteApproval{}, ErrInvalidState
	}
	if version != change.Version {
		return ControlledChangeSet{}, RemoteWriteApproval{}, ErrVersionConflict
	}
	if hash != approval.ControlledChangeSetHash {
		return ControlledChangeSet{}, RemoteWriteApproval{}, ErrApprovalContentMismatch
	}
	binding, err := json.Marshal(approval.Binding)
	if err != nil {
		return ControlledChangeSet{}, RemoteWriteApproval{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO delivery_remote_write_approvals (id,organization_id,project_id,controlled_change_set_id,controlled_change_set_hash,binding_json,action,scope,budget_limit_minor,currency,action_hash,approved_by,approved_at,expires_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, approval.ID, approval.OrganizationID, approval.ProjectID, approval.ControlledChangeSetID, approval.ControlledChangeSetHash, binding, approval.Action, approval.Scope, approval.BudgetLimitMinor, approval.Currency, approval.ActionHash, approval.ApprovedBy, approval.ApprovedAt, approval.ExpiresAt)
	if err != nil {
		return ControlledChangeSet{}, RemoteWriteApproval{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE delivery_controlled_change_sets SET status='approved',version=version+1,updated_at=? WHERE organization_id=? AND project_id=? AND id=? AND version=?`, approval.ApprovedAt, change.OrganizationID, change.ProjectID, change.ID, change.Version)
	if err != nil {
		return ControlledChangeSet{}, RemoteWriteApproval{}, err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return ControlledChangeSet{}, RemoteWriteApproval{}, ErrVersionConflict
	}
	if err := tx.Commit(); err != nil {
		return ControlledChangeSet{}, RemoteWriteApproval{}, err
	}
	change.Status = ControlledChangeSetApproved
	change.Version++
	change.UpdatedAt = approval.ApprovedAt
	return change, approval, nil
}

func (r MySQLRepository) GetRemoteWriteApproval(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, changeSetID string) (RemoteWriteApproval, error) {
	var v RemoteWriteApproval
	var binding []byte
	err := r.DB.QueryRowContext(ctx, `SELECT id,organization_id,project_id,controlled_change_set_id,controlled_change_set_hash,binding_json,action,scope,budget_limit_minor,currency,action_hash,approved_by,approved_at,expires_at FROM delivery_remote_write_approvals WHERE organization_id=? AND project_id=? AND controlled_change_set_id=?`, org, project, changeSetID).Scan(&v.ID, &v.OrganizationID, &v.ProjectID, &v.ControlledChangeSetID, &v.ControlledChangeSetHash, &binding, &v.Action, &v.Scope, &v.BudgetLimitMinor, &v.Currency, &v.ActionHash, &v.ApprovedBy, &v.ApprovedAt, &v.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return RemoteWriteApproval{}, ErrNotFound
	}
	if err != nil {
		return RemoteWriteApproval{}, err
	}
	if err := json.Unmarshal(binding, &v.Binding); err != nil {
		return RemoteWriteApproval{}, fmt.Errorf("decode controlled approval binding: %w", err)
	}
	return v, nil
}

func (r MySQLRepository) CreateControlledExecution(ctx context.Context, value ControlledExecution) (ControlledExecution, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return ControlledExecution{}, err
	}
	defer tx.Rollback()
	var status, approvalID string
	var expiresAt sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT c.status,a.id,a.expires_at FROM delivery_controlled_change_sets c JOIN delivery_remote_write_approvals a ON a.organization_id=c.organization_id AND a.project_id=c.project_id AND a.controlled_change_set_id=c.id WHERE c.organization_id=? AND c.project_id=? AND c.id=? FOR UPDATE`, value.OrganizationID, value.ProjectID, value.ControlledChangeSetID).Scan(&status, &approvalID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ControlledExecution{}, ErrNotFound
	}
	if err != nil {
		return ControlledExecution{}, err
	}
	if status != string(ControlledChangeSetApproved) || approvalID != value.RemoteWriteApprovalID || !expiresAt.Valid || !value.CreatedAt.Before(expiresAt.Time) {
		return ControlledExecution{}, ErrApprovalExpired
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO delivery_controlled_executions (id,organization_id,project_id,controlled_change_set_id,remote_write_approval_id,computer_use_run_id,status,version,created_by,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, value.ID, value.OrganizationID, value.ProjectID, value.ControlledChangeSetID, value.RemoteWriteApprovalID, nullableString(value.ComputerUseRunID), value.Status, value.Version, value.CreatedBy, value.CreatedAt, value.UpdatedAt)
	if err != nil {
		return ControlledExecution{}, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE delivery_controlled_change_sets SET status='executing',version=version+1,updated_at=? WHERE organization_id=? AND project_id=? AND id=? AND status='approved'`, value.CreatedAt, value.OrganizationID, value.ProjectID, value.ControlledChangeSetID)
	if err != nil {
		return ControlledExecution{}, err
	}
	if err := tx.Commit(); err != nil {
		return ControlledExecution{}, err
	}
	return value, nil
}

func (r MySQLRepository) GetControlledExecution(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (ControlledExecution, error) {
	var v ControlledExecution
	err := r.DB.QueryRowContext(ctx, `SELECT id,organization_id,project_id,controlled_change_set_id,remote_write_approval_id,COALESCE(computer_use_run_id,''),status,version,created_by,created_at,updated_at FROM delivery_controlled_executions WHERE organization_id=? AND project_id=? AND id=?`, org, project, id).Scan(&v.ID, &v.OrganizationID, &v.ProjectID, &v.ControlledChangeSetID, &v.RemoteWriteApprovalID, &v.ComputerUseRunID, &v.Status, &v.Version, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ControlledExecution{}, ErrNotFound
	}
	return v, err
}

const controlledChangeSetSelect = `SELECT id,organization_id,project_id,binding_json,action,budget_limit_minor,currency,status,canonical_hash,version,created_by,created_at,updated_at FROM delivery_controlled_change_sets`

func scanControlledChangeSet(row rowScanner) (ControlledChangeSet, error) {
	var v ControlledChangeSet
	var binding []byte
	err := row.Scan(&v.ID, &v.OrganizationID, &v.ProjectID, &binding, &v.Action, &v.BudgetLimitMinor, &v.Currency, &v.Status, &v.CanonicalHash, &v.Version, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ControlledChangeSet{}, ErrNotFound
	}
	if err != nil {
		return ControlledChangeSet{}, err
	}
	if err := json.Unmarshal(binding, &v.Binding); err != nil {
		return ControlledChangeSet{}, fmt.Errorf("decode controlled change-set binding: %w", err)
	}
	return v, nil
}
