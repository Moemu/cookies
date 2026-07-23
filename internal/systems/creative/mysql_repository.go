package creative

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	mysqlDriver "github.com/go-sql-driver/mysql"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// MySQLRepository persists only Creative-owned objects. It never joins
// Strategy, Provider, or Assets tables; those systems expose their own seams.
type MySQLRepository struct{ DB *sql.DB }

func (r MySQLRepository) CreateIntake(ctx context.Context, intake CreativeIntake) (CreativeIntake, bool, error) {
	if r.DB == nil {
		return CreativeIntake{}, false, fmt.Errorf("creative MySQL database is required")
	}
	request, err := json.Marshal(intake.Request)
	if err != nil {
		return CreativeIntake{}, false, fmt.Errorf("encode creative intake request: %w", err)
	}
	missing, err := json.Marshal(intake.MissingFields)
	if err != nil {
		return CreativeIntake{}, false, err
	}
	warnings, err := json.Marshal(intake.Warnings)
	if err != nil {
		return CreativeIntake{}, false, err
	}
	_, err = r.DB.ExecContext(ctx, `INSERT INTO creative_intakes (
		id, organization_id, project_id, principal_kind, principal_id, source_type, status,
		request_payload, missing_fields, warnings, confirmed_by, idempotency_key, request_hash, version, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?)`,
		intake.ID, intake.OrganizationID, intake.ProjectID, intake.Principal.Kind, intake.Principal.ID, intake.Source, intake.Status,
		request, missing, warnings, intake.ConfirmedBy, intake.IdempotencyKey, intake.RequestHash, intake.Version, intake.CreatedAt, intake.UpdatedAt)
	if err == nil {
		return intake, false, nil
	}
	var mysqlError *mysqlDriver.MySQLError
	if !errors.As(err, &mysqlError) || mysqlError.Number != 1062 {
		return CreativeIntake{}, false, err
	}
	existing, getErr := r.getIntakeByIdempotency(ctx, intake)
	if getErr != nil {
		return CreativeIntake{}, false, getErr
	}
	if existing.RequestHash != intake.RequestHash {
		return CreativeIntake{}, false, ErrIdempotencyConflict
	}
	return existing, true, nil
}

func (r MySQLRepository) ListIntakes(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, limit int) ([]CreativeIntake, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("creative MySQL database is required")
	}
	rows, err := r.DB.QueryContext(ctx, creativeIntakeSelect+` WHERE organization_id = ? AND project_id = ? ORDER BY created_at DESC LIMIT ?`, organizationID, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]CreativeIntake, 0)
	for rows.Next() {
		value, scanErr := scanIntake(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r MySQLRepository) GetIntake(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, intakeID string) (CreativeIntake, error) {
	if r.DB == nil {
		return CreativeIntake{}, fmt.Errorf("creative MySQL database is required")
	}
	value, err := scanIntake(r.DB.QueryRowContext(ctx, creativeIntakeSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`, organizationID, projectID, intakeID))
	if errors.Is(err, sql.ErrNoRows) {
		return CreativeIntake{}, ErrNotFound
	}
	return value, err
}

func (r MySQLRepository) CreateTask(ctx context.Context, task CreativeTask, draft ImageTextDraft) (CreativeTask, error) {
	if r.DB == nil {
		return CreativeTask{}, fmt.Errorf("creative MySQL database is required")
	}
	direction, err := json.Marshal(task.Direction)
	if err != nil {
		return CreativeTask{}, err
	}
	content, err := json.Marshal(draft)
	if err != nil {
		return CreativeTask{}, err
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return CreativeTask{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO creative_tasks (
		id, organization_id, project_id, intake_id, creative_format, channel, status, direction_payload, version, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.OrganizationID, task.ProjectID, task.IntakeID, task.Format, task.Channel, task.Status, direction, task.Version, task.CreatedAt, task.UpdatedAt)
	if err != nil {
		var mysqlError *mysqlDriver.MySQLError
		if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
			_ = tx.Rollback()
			return r.getTaskByIntake(ctx, task.OrganizationID, task.ProjectID, task.IntakeID)
		}
		return CreativeTask{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO creative_image_text_drafts (organization_id, task_id, version, status, content_payload, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		task.OrganizationID, task.ID, draft.Version, draft.Status, content, draft.CreatedAt)
	if err != nil {
		return CreativeTask{}, err
	}
	if err := tx.Commit(); err != nil {
		return CreativeTask{}, err
	}
	return task, nil
}

func (r MySQLRepository) ListTasks(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, limit int) ([]CreativeTask, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("creative MySQL database is required")
	}
	rows, err := r.DB.QueryContext(ctx, creativeTaskSelect+` WHERE organization_id = ? AND project_id = ? ORDER BY created_at DESC LIMIT ?`, organizationID, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]CreativeTask, 0)
	for rows.Next() {
		value, scanErr := scanTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r MySQLRepository) GetTaskDetail(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, taskID string) (TaskDetail, error) {
	if r.DB == nil {
		return TaskDetail{}, fmt.Errorf("creative MySQL database is required")
	}
	task, err := scanTask(r.DB.QueryRowContext(ctx, creativeTaskSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`, organizationID, projectID, taskID))
	if errors.Is(err, sql.ErrNoRows) {
		return TaskDetail{}, ErrNotFound
	}
	if err != nil {
		return TaskDetail{}, err
	}
	intake, err := r.GetIntake(ctx, organizationID, projectID, task.IntakeID)
	if err != nil {
		return TaskDetail{}, err
	}
	draft, err := r.getLatestDraft(ctx, organizationID, taskID)
	if err != nil {
		return TaskDetail{}, err
	}
	jobs, err := r.productionJobs(ctx, organizationID, projectID, taskID)
	if err != nil {
		return TaskDetail{}, err
	}
	return TaskDetail{Task: task, Intake: intake, Draft: draft, ProductionJobs: jobs}, nil
}

func (r MySQLRepository) RegisterProductionJob(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, taskID string, job ProductionJob) error {
	if r.DB == nil {
		return fmt.Errorf("creative MySQL database is required")
	}
	_, err := r.DB.ExecContext(ctx, `INSERT INTO creative_production_jobs (organization_id, project_id, task_id, job_kind, provider_job_id, created_at) VALUES (?, ?, ?, ?, ?, ?)`, organizationID, projectID, taskID, job.Kind, job.ProviderJobID, job.CreatedAt)
	if err == nil {
		return nil
	}
	var mysqlError *mysqlDriver.MySQLError
	if !errors.As(err, &mysqlError) || mysqlError.Number != 1062 {
		return err
	}
	var existing string
	readErr := r.DB.QueryRowContext(ctx, `SELECT provider_job_id FROM creative_production_jobs WHERE organization_id = ? AND task_id = ? AND job_kind = ?`, organizationID, taskID, job.Kind).Scan(&existing)
	if readErr != nil {
		return readErr
	}
	if existing != job.ProviderJobID {
		return ErrProviderJobConflict
	}
	return nil
}

const creativeIntakeSelect = `SELECT id, organization_id, project_id, principal_kind, principal_id, source_type, status,
	request_payload, missing_fields, warnings, confirmed_by, idempotency_key, request_hash, version, created_at, updated_at FROM creative_intakes`
const creativeTaskSelect = `SELECT id, organization_id, project_id, intake_id, creative_format, channel, status, direction_payload, version, created_at, updated_at FROM creative_tasks`

type rowScanner interface{ Scan(...any) error }

func scanIntake(row rowScanner) (CreativeIntake, error) {
	var value CreativeIntake
	var request, missing, warnings []byte
	var confirmed sql.NullString
	err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.Principal.Kind, &value.Principal.ID, &value.Source, &value.Status,
		&request, &missing, &warnings, &confirmed, &value.IdempotencyKey, &value.RequestHash, &value.Version, &value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return CreativeIntake{}, err
	}
	if err := json.Unmarshal(request, &value.Request); err != nil {
		return CreativeIntake{}, fmt.Errorf("decode creative intake request: %w", err)
	}
	if err := json.Unmarshal(missing, &value.MissingFields); err != nil {
		return CreativeIntake{}, fmt.Errorf("decode creative intake missing fields: %w", err)
	}
	if err := json.Unmarshal(warnings, &value.Warnings); err != nil {
		return CreativeIntake{}, fmt.Errorf("decode creative intake warnings: %w", err)
	}
	value.ConfirmedBy = confirmed.String
	return value, nil
}

func scanTask(row rowScanner) (CreativeTask, error) {
	var value CreativeTask
	var direction []byte
	err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.IntakeID, &value.Format, &value.Channel, &value.Status, &direction, &value.Version, &value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return CreativeTask{}, err
	}
	if err := json.Unmarshal(direction, &value.Direction); err != nil {
		return CreativeTask{}, fmt.Errorf("decode creative task direction: %w", err)
	}
	return value, nil
}

func (r MySQLRepository) getIntakeByIdempotency(ctx context.Context, intake CreativeIntake) (CreativeIntake, error) {
	return scanIntake(r.DB.QueryRowContext(ctx, creativeIntakeSelect+` WHERE organization_id = ? AND project_id = ? AND principal_kind = ? AND principal_id = ? AND idempotency_key = ?`, intake.OrganizationID, intake.ProjectID, intake.Principal.Kind, intake.Principal.ID, intake.IdempotencyKey))
}

func (r MySQLRepository) getLatestDraft(ctx context.Context, organizationID contract.OrganizationID, taskID string) (ImageTextDraft, error) {
	var payload []byte
	var value ImageTextDraft
	err := r.DB.QueryRowContext(ctx, `SELECT content_payload FROM creative_image_text_drafts WHERE organization_id = ? AND task_id = ? ORDER BY version DESC LIMIT 1`, organizationID, taskID).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return ImageTextDraft{}, ErrNotFound
	}
	if err != nil {
		return ImageTextDraft{}, err
	}
	if err := json.Unmarshal(payload, &value); err != nil {
		return ImageTextDraft{}, fmt.Errorf("decode creative draft: %w", err)
	}
	return value, nil
}

func (r MySQLRepository) getTaskByIntake(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, intakeID string) (CreativeTask, error) {
	value, err := scanTask(r.DB.QueryRowContext(ctx, creativeTaskSelect+` WHERE organization_id = ? AND project_id = ? AND intake_id = ?`, organizationID, projectID, intakeID))
	if errors.Is(err, sql.ErrNoRows) {
		return CreativeTask{}, ErrNotFound
	}
	return value, err
}

func (r MySQLRepository) productionJobs(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, taskID string) ([]ProductionJob, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT task_id, job_kind, provider_job_id, created_at FROM creative_production_jobs WHERE organization_id = ? AND project_id = ? AND task_id = ? ORDER BY created_at`, organizationID, projectID, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]ProductionJob, 0)
	for rows.Next() {
		var job ProductionJob
		if err := rows.Scan(&job.TaskID, &job.Kind, &job.ProviderJobID, &job.CreatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}
