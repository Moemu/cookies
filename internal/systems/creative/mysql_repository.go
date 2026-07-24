package creative

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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
	strategyPackageID := ""
	strategyPackageVersion := int64(0)
	strategyPackageHash := ""
	if intake.Request.StrategyPackage != nil {
		strategyPackageID = intake.Request.StrategyPackage.PackageID
		strategyPackageVersion = intake.Request.StrategyPackage.PackageVersion
		strategyPackageHash = intake.Request.StrategyPackage.ExpectedContentHash
	}
	_, err = r.DB.ExecContext(ctx, `INSERT INTO creative_intakes (
		id, organization_id, project_id, principal_kind, principal_id, source_type, status,
		request_payload, missing_fields, warnings, confirmed_by, idempotency_key, request_hash,
		strategy_package_id, strategy_package_version, strategy_package_content_hash, version, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''), NULLIF(?, 0), NULLIF(?, ''), ?, ?, ?)`,
		intake.ID, intake.OrganizationID, intake.ProjectID, intake.Principal.Kind, intake.Principal.ID, intake.Source, intake.Status,
		request, missing, warnings, intake.ConfirmedBy, intake.IdempotencyKey, intake.RequestHash,
		strategyPackageID, strategyPackageVersion, strategyPackageHash, intake.Version, intake.CreatedAt, intake.UpdatedAt)
	if err == nil {
		return intake, false, nil
	}
	var mysqlError *mysqlDriver.MySQLError
	if !errors.As(err, &mysqlError) || mysqlError.Number != 1062 {
		return CreativeIntake{}, false, err
	}
	existing, getErr := r.getIntakeByIdempotency(ctx, intake)
	if getErr == nil {
		if existing.RequestHash != intake.RequestHash {
			return CreativeIntake{}, false, ErrIdempotencyConflict
		}
		return existing, true, nil
	}
	if intake.Source == IntakeSourceStrategyPackage && intake.Request.StrategyPackage != nil {
		existing, packageErr := r.getIntakeByStrategyPackage(ctx, intake.OrganizationID, intake.ProjectID, *intake.Request.StrategyPackage)
		if packageErr == nil {
			return existing, true, nil
		}
		if !errors.Is(packageErr, sql.ErrNoRows) {
			return CreativeIntake{}, false, packageErr
		}
	}
	return CreativeIntake{}, false, getErr
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
	rows, err := r.DB.QueryContext(ctx, creativeTaskSelect+` WHERE organization_id = ? AND project_id = ? AND status <> ? ORDER BY created_at DESC LIMIT ?`, organizationID, projectID, TaskArchived, limit)
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

func (r MySQLRepository) ArchiveTask(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, taskID string, now time.Time) error {
	if r.DB == nil {
		return fmt.Errorf("creative MySQL database is required")
	}
	result, err := r.DB.ExecContext(ctx, `UPDATE creative_tasks SET status = ?, updated_at = ? WHERE organization_id = ? AND project_id = ? AND id = ? AND status <> ?`, TaskArchived, now, organizationID, projectID, taskID, TaskArchived)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
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

func (r MySQLRepository) ReviseDraft(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, taskID string, expectedVersion int64, draft ImageTextDraft) (ImageTextDraft, error) {
	if r.DB == nil {
		return ImageTextDraft{}, fmt.Errorf("creative MySQL database is required")
	}
	payload, err := json.Marshal(draft)
	if err != nil {
		return ImageTextDraft{}, fmt.Errorf("encode creative draft revision: %w", err)
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return ImageTextDraft{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE creative_tasks SET version = ?, status = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ?`,
		draft.Version, TaskDraft, draft.CreatedAt, organizationID, projectID, taskID, expectedVersion)
	if err != nil {
		return ImageTextDraft{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return ImageTextDraft{}, err
	}
	if affected != 1 {
		var exists int
		err = tx.QueryRowContext(ctx, `SELECT 1 FROM creative_tasks WHERE organization_id = ? AND project_id = ? AND id = ?`, organizationID, projectID, taskID).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return ImageTextDraft{}, ErrNotFound
		}
		if err != nil {
			return ImageTextDraft{}, err
		}
		return ImageTextDraft{}, ErrVersionConflict
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO creative_image_text_drafts (organization_id, task_id, version, status, content_payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, organizationID, taskID, draft.Version, draft.Status, payload, draft.CreatedAt)
	if err != nil {
		return ImageTextDraft{}, err
	}
	if err := tx.Commit(); err != nil {
		return ImageTextDraft{}, err
	}
	return draft, nil
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

func (r MySQLRepository) CreateVersion(ctx context.Context, value CreativeVersion) (CreativeVersion, bool, error) {
	if r.DB == nil {
		return CreativeVersion{}, false, fmt.Errorf("creative MySQL database is required")
	}
	snapshot, err := json.Marshal(value.Snapshot)
	if err != nil {
		return CreativeVersion{}, false, fmt.Errorf("encode creative version snapshot: %w", err)
	}
	_, err = r.DB.ExecContext(ctx, `INSERT INTO creative_versions (
		id, organization_id, project_id, task_id, version, draft_version, status,
		snapshot_payload, content_hash, created_by, idempotency_key, request_hash, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		value.ID, value.OrganizationID, value.ProjectID, value.TaskID, value.Version, value.DraftVersion,
		value.Status, snapshot, value.ContentHash, value.CreatedBy, value.IdempotencyKey, value.RequestHash, value.CreatedAt)
	if err == nil {
		return value, false, nil
	}
	var mysqlError *mysqlDriver.MySQLError
	if !errors.As(err, &mysqlError) || mysqlError.Number != 1062 {
		return CreativeVersion{}, false, err
	}
	replayed, readErr := r.getVersionByIdempotency(ctx, value)
	if readErr == nil {
		if replayed.RequestHash != value.RequestHash {
			return CreativeVersion{}, false, ErrIdempotencyConflict
		}
		return replayed, true, nil
	}
	existing, readErr := r.getVersionByTaskDraft(ctx, value.OrganizationID, value.ProjectID, value.TaskID, value.DraftVersion)
	if readErr != nil {
		return CreativeVersion{}, false, readErr
	}
	if !existing.ContentHash.Equal(value.ContentHash) {
		return CreativeVersion{}, false, ErrVersionConflict
	}
	return existing, false, nil
}

const creativeIntakeSelect = `SELECT id, organization_id, project_id, principal_kind, principal_id, source_type, status,
	request_payload, missing_fields, warnings, confirmed_by, idempotency_key, request_hash, version, created_at, updated_at FROM creative_intakes`
const creativeTaskSelect = `SELECT id, organization_id, project_id, intake_id, creative_format, channel, status, direction_payload, version, created_at, updated_at FROM creative_tasks`
const creativeVersionSelect = `SELECT id, organization_id, project_id, task_id, version, draft_version, status,
	snapshot_payload, content_hash, created_by, idempotency_key, request_hash, created_at FROM creative_versions`

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

func scanCreativeVersion(row rowScanner) (CreativeVersion, error) {
	var value CreativeVersion
	var snapshot []byte
	err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.TaskID, &value.Version, &value.DraftVersion,
		&value.Status, &snapshot, &value.ContentHash, &value.CreatedBy, &value.IdempotencyKey, &value.RequestHash, &value.CreatedAt)
	if err != nil {
		return CreativeVersion{}, err
	}
	if err := json.Unmarshal(snapshot, &value.Snapshot); err != nil {
		return CreativeVersion{}, fmt.Errorf("decode creative version snapshot: %w", err)
	}
	return value, nil
}

func (r MySQLRepository) getIntakeByIdempotency(ctx context.Context, intake CreativeIntake) (CreativeIntake, error) {
	return scanIntake(r.DB.QueryRowContext(ctx, creativeIntakeSelect+` WHERE organization_id = ? AND project_id = ? AND principal_kind = ? AND principal_id = ? AND idempotency_key = ?`, intake.OrganizationID, intake.ProjectID, intake.Principal.Kind, intake.Principal.ID, intake.IdempotencyKey))
}

func (r MySQLRepository) getIntakeByStrategyPackage(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, reference StrategyPackageReference) (CreativeIntake, error) {
	return scanIntake(r.DB.QueryRowContext(ctx, creativeIntakeSelect+` WHERE organization_id = ? AND project_id = ? AND source_type = ? AND strategy_package_id = ? AND strategy_package_version = ? AND strategy_package_content_hash = ?`, organizationID, projectID, IntakeSourceStrategyPackage, reference.PackageID, reference.PackageVersion, reference.ExpectedContentHash))
}

func (r MySQLRepository) getVersionByIdempotency(ctx context.Context, value CreativeVersion) (CreativeVersion, error) {
	version, err := scanCreativeVersion(r.DB.QueryRowContext(ctx, creativeVersionSelect+` WHERE organization_id = ? AND project_id = ? AND created_by = ? AND idempotency_key = ?`, value.OrganizationID, value.ProjectID, value.CreatedBy, value.IdempotencyKey))
	if errors.Is(err, sql.ErrNoRows) {
		return CreativeVersion{}, ErrNotFound
	}
	return version, err
}

func (r MySQLRepository) getVersionByTaskDraft(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, taskID string, draftVersion int64) (CreativeVersion, error) {
	version, err := scanCreativeVersion(r.DB.QueryRowContext(ctx, creativeVersionSelect+` WHERE organization_id = ? AND project_id = ? AND task_id = ? AND draft_version = ?`, organizationID, projectID, taskID, draftVersion))
	if errors.Is(err, sql.ErrNoRows) {
		return CreativeVersion{}, ErrNotFound
	}
	return version, err
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
