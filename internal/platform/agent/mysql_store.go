package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

var ErrNotFound = errors.New("agent task not found")
var ErrVersionConflict = errors.New("agent task version conflict")

type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type TransactionalTaskWriter interface {
	CreateIn(context.Context, DBTX, CreateRequest) error
}

type MySQLStore struct{ DB *sql.DB }

func (s MySQLStore) Create(ctx context.Context, request CreateRequest) (Task, error) {
	if s.DB == nil {
		return Task{}, fmt.Errorf("MySQL database is required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback()
	if err := s.CreateIn(ctx, tx, request); err != nil {
		return Task{}, err
	}
	if err := tx.Commit(); err != nil {
		return Task{}, err
	}
	return request.Task, nil
}

// CreateIn lets a domain atomically append its own fact and the dispatch
// record using the shared MySQL transaction.
func (MySQLStore) CreateIn(ctx context.Context, executor DBTX, request CreateRequest) error {
	if executor == nil {
		return fmt.Errorf("database executor is required")
	}
	task := request.Task
	if err := task.Validate(); err != nil {
		return err
	}
	if task.Status != TaskDispatchPending || task.JobID != "" {
		return fmt.Errorf("new agent task must await dispatch")
	}
	if _, err := executor.ExecContext(ctx, `INSERT INTO platform_agent_tasks (
		id, organization_id, project_id, source_system, source_type, source_id, kind,
		status, version, input_snapshot, created_by_kind, created_by_id, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.OrganizationID, task.ProjectID, task.SourceSystem, task.SourceType,
		task.SourceID, task.Kind, task.Status, task.Version, task.InputSnapshot,
		task.CreatedBy.Kind, task.CreatedBy.ID, task.CreatedAt, task.UpdatedAt); err != nil {
		return err
	}
	_, err := executor.ExecContext(ctx, `INSERT INTO platform_agent_dispatches (
		agent_task_id, organization_id, project_id, status, available_at, created_at, updated_at
	) VALUES (?, ?, ?, 'pending', ?, ?, ?)`,
		task.ID, task.OrganizationID, task.ProjectID, task.CreatedAt, task.CreatedAt, task.UpdatedAt)
	return err
}

func (s MySQLStore) Get(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (Task, error) {
	if s.DB == nil {
		return Task{}, fmt.Errorf("MySQL database is required")
	}
	return scanTask(s.DB.QueryRowContext(ctx, taskSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`, organizationID, projectID, id))
}

func (s MySQLStore) getByID(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (Task, error) {
	return scanTask(s.DB.QueryRowContext(ctx, taskSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`, organizationID, projectID, id))
}

func (s MySQLStore) MarkRunning(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string, now time.Time) error {
	return s.transition(ctx, organizationID, projectID, id, TaskQueued, TaskRunning, nil, nil, now)
}

func (s MySQLStore) MarkSucceeded(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string, ref *contract.ResourceRef, now time.Time) error {
	return s.transition(ctx, organizationID, projectID, id, TaskRunning, TaskSucceeded, ref, nil, now)
}

func (s MySQLStore) MarkFailed(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string, problem contract.JobError, now time.Time) error {
	return s.transition(ctx, organizationID, projectID, id, TaskRunning, TaskFailed, nil, &problem, now)
}

func (s MySQLStore) MarkRetrying(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string, now time.Time) error {
	return s.transition(ctx, organizationID, projectID, id, TaskRunning, TaskQueued, nil, nil, now)
}

func (s MySQLStore) MarkRunningCancelled(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string, now time.Time) error {
	return s.transition(ctx, organizationID, projectID, id, TaskRunning, TaskCancelled, nil, nil, now)
}

func (s MySQLStore) RequestCancel(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string, expectedVersion int64, now time.Time) (Task, error) {
	result, err := s.DB.ExecContext(ctx, `UPDATE platform_agent_tasks SET status = ?,
		version = version + 1, updated_at = ? WHERE organization_id = ? AND project_id = ?
		AND id = ? AND version = ? AND status IN (?, ?)`, TaskCancelled, now.UTC(),
		organizationID, projectID, id, expectedVersion, TaskDispatchPending, TaskQueued)
	if err != nil {
		return Task{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return Task{}, err
	}
	if changed != 1 {
		existing, getErr := s.Get(ctx, organizationID, projectID, id)
		if getErr != nil {
			return Task{}, getErr
		}
		if existing.Status == TaskRunning {
			return existing, nil
		}
		return Task{}, ErrVersionConflict
	}
	if _, err := s.DB.ExecContext(ctx, `UPDATE platform_agent_dispatches SET status = 'cancelled',
		updated_at = ? WHERE organization_id = ? AND project_id = ? AND agent_task_id = ?
		AND status = 'pending'`, now.UTC(), organizationID, projectID, id); err != nil {
		return Task{}, err
	}
	return s.Get(ctx, organizationID, projectID, id)
}

func (s MySQLStore) transition(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string, from, to TaskStatus, ref *contract.ResourceRef, problem *contract.JobError, now time.Time) error {
	var resultType, resultID, resultVersion, errorCode, errorMessage any
	if ref != nil {
		if err := ref.Validate(); err != nil {
			return err
		}
		resultType, resultID, resultVersion = ref.Type, ref.ID, ref.Version
	}
	if problem != nil {
		errorCode, errorMessage = problem.Code, problem.Message
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE platform_agent_tasks SET status = ?,
		result_type = ?, result_id = ?, result_version = ?, error_code = ?, error_message = ?,
		version = version + 1, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND status = ?`,
		to, resultType, resultID, resultVersion, errorCode, errorMessage, now.UTC(),
		organizationID, projectID, id, from)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return ErrVersionConflict
	}
	return nil
}

const taskSelect = `SELECT id, organization_id, project_id, source_system, source_type, source_id,
	kind, status, COALESCE(job_id, ''), version, input_snapshot,
	result_type, result_id, result_version, error_code, error_message,
	created_by_kind, created_by_id, created_at, updated_at FROM platform_agent_tasks`

func scanTask(row *sql.Row) (Task, error) {
	var task Task
	var resultType, resultID, errorCode, errorMessage sql.NullString
	var resultVersion sql.NullInt64
	if err := row.Scan(&task.ID, &task.OrganizationID, &task.ProjectID, &task.SourceSystem,
		&task.SourceType, &task.SourceID, &task.Kind, &task.Status, &task.JobID, &task.Version,
		&task.InputSnapshot, &resultType, &resultID, &resultVersion, &errorCode, &errorMessage,
		&task.CreatedBy.Kind, &task.CreatedBy.ID, &task.CreatedAt, &task.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Task{}, ErrNotFound
		}
		return Task{}, err
	}
	if resultType.Valid && resultID.Valid {
		task.ResultRef = &contract.ResourceRef{Type: resultType.String, ID: resultID.String}
		if resultVersion.Valid {
			task.ResultRef.Version = &resultVersion.Int64
		}
	}
	if errorCode.Valid {
		task.Error = &contract.JobError{Code: errorCode.String, Message: errorMessage.String}
	}
	return task, nil
}
