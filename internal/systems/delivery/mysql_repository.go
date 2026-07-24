package delivery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type MySQLRepository struct {
	DB *sql.DB
}

func (r MySQLRepository) CreatePlan(ctx context.Context, value DeliveryPlan) (DeliveryPlan, error) {
	_, err := r.DB.ExecContext(ctx, `INSERT INTO delivery_plans (
		id, organization_id, project_id, creative_package_id, creative_package_hash, creative_version_id,
		name, objective, budget_cents, start_at, end_at, status, version, created_by, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		value.ID, value.OrganizationID, value.ProjectID, value.CreativePackageID, value.CreativePackageHash,
		value.CreativeVersionID, value.Name, value.Objective, value.BudgetCents, value.StartAt, value.EndAt,
		value.Status, value.Version, value.CreatedBy, value.CreatedAt, value.UpdatedAt)
	return value, err
}

func (r MySQLRepository) ListPlans(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, limit int) ([]DeliveryPlan, error) {
	rows, err := r.DB.QueryContext(ctx, deliveryPlanSelect+` WHERE organization_id = ? AND project_id = ? ORDER BY updated_at DESC, id DESC LIMIT ?`, organizationID, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]DeliveryPlan, 0)
	for rows.Next() {
		value, scanErr := scanDeliveryPlan(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r MySQLRepository) GetPlan(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (DeliveryPlan, error) {
	value, err := scanDeliveryPlan(r.DB.QueryRowContext(ctx, deliveryPlanSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`, organizationID, projectID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return DeliveryPlan{}, ErrNotFound
	}
	return value, err
}

func (r MySQLRepository) CreateChangeSet(ctx context.Context, value ChangeSet) (ChangeSet, error) {
	notes, err := json.Marshal(value.PreflightNotes)
	if err != nil {
		return ChangeSet{}, err
	}
	_, err = r.DB.ExecContext(ctx, `INSERT INTO delivery_change_sets (
		id, organization_id, project_id, plan_id, plan_version, status, risk_level, preflight_notes,
		approved_by, approved_at, version, created_by, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, ?, ?, ?, ?)`,
		value.ID, value.OrganizationID, value.ProjectID, value.PlanID, value.PlanVersion, value.Status,
		value.RiskLevel, notes, value.Version, value.CreatedBy, value.CreatedAt, value.UpdatedAt)
	return value, err
}

func (r MySQLRepository) ListChangeSets(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, limit int) ([]ChangeSet, error) {
	rows, err := r.DB.QueryContext(ctx, changeSetSelect+` WHERE organization_id = ? AND project_id = ? ORDER BY updated_at DESC, id DESC LIMIT ?`, organizationID, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]ChangeSet, 0)
	for rows.Next() {
		value, scanErr := scanChangeSet(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r MySQLRepository) GetChangeSet(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (ChangeSet, error) {
	value, err := scanChangeSet(r.DB.QueryRowContext(ctx, changeSetSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`, organizationID, projectID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return ChangeSet{}, ErrNotFound
	}
	return value, err
}

func (r MySQLRepository) TransitionChangeSet(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string, expectedVersion int64, next ChangeSetStatus, actorID string, now time.Time) (ChangeSet, error) {
	var result sql.Result
	var err error
	if next == ChangeSetApproved {
		result, err = r.DB.ExecContext(ctx, `UPDATE delivery_change_sets SET status = ?, approved_by = ?, approved_at = ?, version = version + 1, updated_at = ? WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ?`,
			next, actorID, now, now, organizationID, projectID, id, expectedVersion)
	} else {
		result, err = r.DB.ExecContext(ctx, `UPDATE delivery_change_sets SET status = ?, version = version + 1, updated_at = ? WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ?`,
			next, now, organizationID, projectID, id, expectedVersion)
	}
	if err != nil {
		return ChangeSet{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return ChangeSet{}, err
	}
	if affected == 0 {
		if _, getErr := r.GetChangeSet(ctx, organizationID, projectID, id); getErr != nil {
			return ChangeSet{}, getErr
		}
		return ChangeSet{}, ErrVersionConflict
	}
	return r.GetChangeSet(ctx, organizationID, projectID, id)
}

func (r MySQLRepository) RecordExecution(ctx context.Context, changeSet ChangeSet, execution Execution, evidence Evidence) (ExecutionResult, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return ExecutionResult{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE delivery_change_sets SET status = ?, version = version + 1, updated_at = ? WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ? AND status = ?`,
		ChangeSetExecuted, execution.CompletedAt, changeSet.OrganizationID, changeSet.ProjectID, changeSet.ID, changeSet.Version, ChangeSetApproved)
	if err != nil {
		return ExecutionResult{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return ExecutionResult{}, err
	}
	if affected == 0 {
		return ExecutionResult{}, ErrVersionConflict
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO delivery_executions (id, organization_id, project_id, change_set_id, status, execution_mode, executed_by, started_at, completed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		execution.ID, execution.OrganizationID, execution.ProjectID, execution.ChangeSetID, execution.Status, execution.Mode, execution.ExecutedBy, execution.StartedAt, execution.CompletedAt)
	if err != nil {
		return ExecutionResult{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO delivery_evidence (id, organization_id, project_id, execution_id, summary, evidence_mode, reversible, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		evidence.ID, evidence.OrganizationID, evidence.ProjectID, evidence.ExecutionID, evidence.Summary, evidence.Mode, evidence.Reversible, evidence.CreatedAt)
	if err != nil {
		return ExecutionResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ExecutionResult{}, err
	}
	changeSet.Status = ChangeSetExecuted
	changeSet.Version++
	changeSet.UpdatedAt = execution.CompletedAt
	return ExecutionResult{ChangeSet: changeSet, Execution: execution, Evidence: evidence}, nil
}

func (r MySQLRepository) ListExecutions(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, limit int) ([]ExecutionResult, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT
		c.id, c.organization_id, c.project_id, c.plan_id, c.plan_version, c.status, c.risk_level, c.preflight_notes, c.approved_by, c.approved_at, c.version, c.created_by, c.created_at, c.updated_at,
		x.id, x.organization_id, x.project_id, x.change_set_id, x.status, x.execution_mode, x.executed_by, x.started_at, x.completed_at,
		e.id, e.organization_id, e.project_id, e.execution_id, e.summary, e.evidence_mode, e.reversible, e.created_at
		FROM delivery_executions x
		JOIN delivery_change_sets c ON c.organization_id = x.organization_id AND c.id = x.change_set_id
		JOIN delivery_evidence e ON e.organization_id = x.organization_id AND e.execution_id = x.id
		WHERE x.organization_id = ? AND x.project_id = ?
		ORDER BY x.completed_at DESC, x.id DESC LIMIT ?`, organizationID, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]ExecutionResult, 0)
	for rows.Next() {
		var value ExecutionResult
		var notes []byte
		var approvedBy sql.NullString
		var approvedAt sql.NullTime
		if err := rows.Scan(
			&value.ChangeSet.ID, &value.ChangeSet.OrganizationID, &value.ChangeSet.ProjectID, &value.ChangeSet.PlanID,
			&value.ChangeSet.PlanVersion, &value.ChangeSet.Status, &value.ChangeSet.RiskLevel, &notes,
			&approvedBy, &approvedAt, &value.ChangeSet.Version, &value.ChangeSet.CreatedBy, &value.ChangeSet.CreatedAt, &value.ChangeSet.UpdatedAt,
			&value.Execution.ID, &value.Execution.OrganizationID, &value.Execution.ProjectID, &value.Execution.ChangeSetID,
			&value.Execution.Status, &value.Execution.Mode, &value.Execution.ExecutedBy, &value.Execution.StartedAt, &value.Execution.CompletedAt,
			&value.Evidence.ID, &value.Evidence.OrganizationID, &value.Evidence.ProjectID, &value.Evidence.ExecutionID,
			&value.Evidence.Summary, &value.Evidence.Mode, &value.Evidence.Reversible, &value.Evidence.CreatedAt,
		); err != nil {
			return nil, err
		}
		if err := decodeChangeSetOptional(&value.ChangeSet, notes, approvedBy, approvedAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

const deliveryPlanSelect = `SELECT id, organization_id, project_id, creative_package_id, creative_package_hash, creative_version_id, name, objective, budget_cents, start_at, end_at, status, version, created_by, created_at, updated_at FROM delivery_plans`
const changeSetSelect = `SELECT id, organization_id, project_id, plan_id, plan_version, status, risk_level, preflight_notes, approved_by, approved_at, version, created_by, created_at, updated_at FROM delivery_change_sets`

type rowScanner interface {
	Scan(...any) error
}

func scanDeliveryPlan(row rowScanner) (DeliveryPlan, error) {
	var value DeliveryPlan
	err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.CreativePackageID,
		&value.CreativePackageHash, &value.CreativeVersionID, &value.Name, &value.Objective,
		&value.BudgetCents, &value.StartAt, &value.EndAt, &value.Status, &value.Version,
		&value.CreatedBy, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}

func scanChangeSet(row rowScanner) (ChangeSet, error) {
	var value ChangeSet
	var notes []byte
	var approvedBy sql.NullString
	var approvedAt sql.NullTime
	err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.PlanID,
		&value.PlanVersion, &value.Status, &value.RiskLevel, &notes, &approvedBy, &approvedAt,
		&value.Version, &value.CreatedBy, &value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return ChangeSet{}, err
	}
	if err := decodeChangeSetOptional(&value, notes, approvedBy, approvedAt); err != nil {
		return ChangeSet{}, err
	}
	return value, nil
}

func decodeChangeSetOptional(value *ChangeSet, notes []byte, approvedBy sql.NullString, approvedAt sql.NullTime) error {
	if err := json.Unmarshal(notes, &value.PreflightNotes); err != nil {
		return fmt.Errorf("decode delivery preflight notes: %w", err)
	}
	if approvedBy.Valid {
		value.ApprovedBy = approvedBy.String
	}
	if approvedAt.Valid {
		value.ApprovedAt = &approvedAt.Time
	}
	return nil
}
