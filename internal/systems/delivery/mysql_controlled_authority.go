package delivery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/computeruse"
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
	v.SchemaVersion = RemoteWriteApprovalSchemaV1
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

func (r MySQLRepository) AttachComputerUseRun(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, id string, expectedVersion int64, runID string, now time.Time) (ControlledExecution, error) {
	result, err := r.DB.ExecContext(ctx, `UPDATE delivery_controlled_executions SET computer_use_run_id=?,status='running',version=version+1,updated_at=? WHERE organization_id=? AND project_id=? AND id=? AND version=? AND computer_use_run_id IS NULL AND status='pending'`, runID, now, org, project, id, expectedVersion)
	if err != nil {
		return ControlledExecution{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return ControlledExecution{}, err
	}
	if affected != 1 {
		return ControlledExecution{}, ErrVersionConflict
	}
	return r.GetControlledExecution(ctx, org, project, id)
}

func (r MySQLRepository) CreatePlatformEntityMapping(ctx context.Context, value PlatformEntityMapping) (PlatformEntityMapping, error) {
	_, err := r.DB.ExecContext(ctx, `INSERT INTO delivery_platform_entity_mappings (id,organization_id,project_id,account_reference_id,plan_id,configuration_id,business_execution_id,computer_use_run_id,internal_object_kind,internal_object_id,platform_object_kind,platform_object_id,platform_status,result_evidence_id,list_evidence_id,status,version,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, value.ID, value.OrganizationID, value.ProjectID, value.AccountReferenceID, value.PlanID, value.ConfigurationID, value.BusinessExecutionID, value.ComputerUseRunID, value.InternalObjectKind, value.InternalObjectID, value.PlatformObjectKind, nullableString(value.PlatformObjectID), nullableString(value.PlatformStatus), nullableString(value.ResultEvidenceID), nullableString(value.ListEvidenceID), value.Status, value.Version, value.CreatedAt)
	if err != nil {
		return PlatformEntityMapping{}, err
	}
	return value, nil
}

func (r MySQLRepository) GetPlatformEntityMapping(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (PlatformEntityMapping, error) {
	return scanPlatformEntityMapping(r.DB.QueryRowContext(ctx, platformEntityMappingSelect+` WHERE organization_id=? AND project_id=? AND id=?`, org, project, id))
}

func (r MySQLRepository) ConfirmPlatformEntityMapping(ctx context.Context, org contract.OrganizationID, project contract.ProjectID, id string, expectedVersion int64, resultEvidenceID, listEvidenceID string) (PlatformEntityMapping, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return PlatformEntityMapping{}, err
	}
	defer tx.Rollback()
	value, err := scanPlatformEntityMapping(tx.QueryRowContext(ctx, platformEntityMappingSelect+` WHERE organization_id=? AND project_id=? AND id=? FOR UPDATE`, org, project, id))
	if err != nil {
		return PlatformEntityMapping{}, err
	}
	if value.Version != expectedVersion {
		return PlatformEntityMapping{}, ErrVersionConflict
	}
	if value.Status != PlatformEntityMappingPending && value.Status != PlatformEntityMappingConfirmed {
		return PlatformEntityMapping{}, ErrInvalidState
	}
	resultEvidence, err := loadPlatformMappingEvidence(ctx, tx, value, resultEvidenceID)
	if err != nil {
		return PlatformEntityMapping{}, err
	}
	listEvidence, err := loadPlatformMappingEvidence(ctx, tx, value, listEvidenceID)
	if err != nil {
		return PlatformEntityMapping{}, err
	}
	platformObjectID, platformStatus, err := validatePlatformMappingEvidence(value, resultEvidence, listEvidence)
	if err != nil {
		return PlatformEntityMapping{}, err
	}
	if value.Status == PlatformEntityMappingPending {
		result, err := tx.ExecContext(ctx, `UPDATE delivery_platform_entity_mappings SET platform_object_id=?,platform_status=?,result_evidence_id=?,list_evidence_id=?,status='confirmed',version=version+1 WHERE organization_id=? AND project_id=? AND id=? AND status='pending_verification' AND version=?`, platformObjectID, platformStatus, resultEvidenceID, listEvidenceID, org, project, id, expectedVersion)
		if err != nil {
			return PlatformEntityMapping{}, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return PlatformEntityMapping{}, err
		}
		if affected != 1 {
			return PlatformEntityMapping{}, ErrVersionConflict
		}
	} else if value.PlatformObjectID != platformObjectID || value.PlatformStatus != platformStatus || value.ResultEvidenceID != resultEvidenceID || value.ListEvidenceID != listEvidenceID {
		return PlatformEntityMapping{}, ErrApprovalContentMismatch
	}

	var changeSetID, executionRunID, executionStatus string
	if err := tx.QueryRowContext(ctx, `SELECT controlled_change_set_id,COALESCE(computer_use_run_id,''),status FROM delivery_controlled_executions WHERE organization_id=? AND project_id=? AND id=? FOR UPDATE`, org, project, value.BusinessExecutionID).Scan(&changeSetID, &executionRunID, &executionStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PlatformEntityMapping{}, ErrNotFound
		}
		return PlatformEntityMapping{}, err
	}
	if executionRunID != value.ComputerUseRunID {
		return PlatformEntityMapping{}, ErrApprovalContentMismatch
	}
	completedAt := listEvidence.Evidence.CreatedAt
	if executionStatus == "running" {
		result, err := tx.ExecContext(ctx, `UPDATE delivery_controlled_executions SET status='succeeded',version=version+1,updated_at=? WHERE organization_id=? AND project_id=? AND id=? AND status='running' AND computer_use_run_id=?`, completedAt, org, project, value.BusinessExecutionID, value.ComputerUseRunID)
		if err != nil {
			return PlatformEntityMapping{}, err
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			return PlatformEntityMapping{}, ErrVersionConflict
		}
	} else if executionStatus != "succeeded" {
		return PlatformEntityMapping{}, ErrInvalidState
	}
	result, err := tx.ExecContext(ctx, `UPDATE delivery_controlled_change_sets SET status='executed',version=version+1,updated_at=? WHERE organization_id=? AND project_id=? AND id=? AND status='executing'`, completedAt, org, project, changeSetID)
	if err != nil {
		return PlatformEntityMapping{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return PlatformEntityMapping{}, err
	}
	if affected == 0 {
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM delivery_controlled_change_sets WHERE organization_id=? AND project_id=? AND id=? FOR UPDATE`, org, project, changeSetID).Scan(&status); err != nil {
			return PlatformEntityMapping{}, err
		}
		if status != string(ControlledChangeSetExecuted) {
			return PlatformEntityMapping{}, ErrInvalidState
		}
	}
	if err := tx.Commit(); err != nil {
		return PlatformEntityMapping{}, err
	}
	return r.GetPlatformEntityMapping(ctx, org, project, id)
}

type platformMappingEvidence struct {
	Evidence computeruse.Evidence
	Step     computeruse.RunStep
}

func loadPlatformMappingEvidence(ctx context.Context, tx *sql.Tx, mapping PlatformEntityMapping, evidenceID string) (platformMappingEvidence, error) {
	var payload []byte
	var value platformMappingEvidence
	err := tx.QueryRowContext(ctx, `SELECT e.evidence_json,s.id,s.run_id,s.sequence_number,s.action,s.status FROM computer_use_evidence e JOIN computer_use_run_steps s ON s.organization_id=e.organization_id AND s.project_id=e.project_id AND s.run_id=e.run_id AND s.id=e.step_id WHERE e.organization_id=? AND e.project_id=? AND e.run_id=? AND e.id=? FOR UPDATE`, mapping.OrganizationID, mapping.ProjectID, mapping.ComputerUseRunID, evidenceID).Scan(&payload, &value.Step.ID, &value.Step.RunID, &value.Step.Sequence, &value.Step.Action, &value.Step.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return platformMappingEvidence{}, ErrNotFound
	}
	if err != nil {
		return platformMappingEvidence{}, err
	}
	if err := json.Unmarshal(payload, &value.Evidence); err != nil {
		return platformMappingEvidence{}, fmt.Errorf("decode computer-use evidence: %w", err)
	}
	return value, nil
}

func validatePlatformMappingEvidence(mapping PlatformEntityMapping, result, list platformMappingEvidence) (string, string, error) {
	if result.Evidence.ID == "" || list.Evidence.ID == "" || result.Evidence.ID == list.Evidence.ID ||
		result.Evidence.OrganizationID != mapping.OrganizationID || list.Evidence.OrganizationID != mapping.OrganizationID ||
		result.Evidence.ProjectID != mapping.ProjectID || list.Evidence.ProjectID != mapping.ProjectID ||
		result.Evidence.RunID != mapping.ComputerUseRunID || list.Evidence.RunID != mapping.ComputerUseRunID ||
		result.Evidence.StepID != result.Step.ID || list.Evidence.StepID != list.Step.ID ||
		result.Evidence.ObjectFingerprint != mapping.InternalObjectID || list.Evidence.ObjectFingerprint != mapping.InternalObjectID ||
		result.Step.RunID != mapping.ComputerUseRunID || list.Step.RunID != mapping.ComputerUseRunID ||
		result.Step.Action != string(computeruse.TakeoverResultObserved) || list.Step.Action != string(computeruse.TakeoverListConfirmed) ||
		result.Step.Status != computeruse.StepSucceeded || list.Step.Status != computeruse.StepSucceeded ||
		result.Step.Sequence >= list.Step.Sequence {
		return "", "", ErrApprovalContentMismatch
	}
	resultObjectID, resultStatus := result.Evidence.FieldReadback["platform_object_id"], result.Evidence.FieldReadback["platform_status"]
	listObjectID, listStatus := list.Evidence.FieldReadback["platform_object_id"], list.Evidence.FieldReadback["platform_status"]
	if resultObjectID == "" || resultStatus == "" || resultObjectID != listObjectID || resultStatus != listStatus {
		return "", "", ErrApprovalContentMismatch
	}
	return resultObjectID, resultStatus, nil
}

func scanPlatformEntityMapping(row rowScanner) (PlatformEntityMapping, error) {
	var value PlatformEntityMapping
	err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.AccountReferenceID, &value.PlanID, &value.ConfigurationID, &value.BusinessExecutionID, &value.ComputerUseRunID, &value.InternalObjectKind, &value.InternalObjectID, &value.PlatformObjectKind, &value.PlatformObjectID, &value.PlatformStatus, &value.ResultEvidenceID, &value.ListEvidenceID, &value.Status, &value.Version, &value.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return PlatformEntityMapping{}, ErrNotFound
	}
	if err != nil {
		return PlatformEntityMapping{}, err
	}
	value.SchemaVersion = PlatformEntityMappingV1
	return value, nil
}

const controlledChangeSetSelect = `SELECT id,organization_id,project_id,binding_json,action,budget_limit_minor,currency,status,canonical_hash,version,created_by,created_at,updated_at FROM delivery_controlled_change_sets`
const platformEntityMappingSelect = `SELECT id,organization_id,project_id,account_reference_id,plan_id,configuration_id,business_execution_id,computer_use_run_id,internal_object_kind,internal_object_id,platform_object_kind,COALESCE(platform_object_id,''),COALESCE(platform_status,''),COALESCE(result_evidence_id,''),COALESCE(list_evidence_id,''),status,version,created_at FROM delivery_platform_entity_mappings`

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
	v.SchemaVersion = ControlledChangeSetSchemaV1
	return v, nil
}
