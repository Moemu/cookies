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

func (r MySQLRepository) EnsureCommerceFixtureWorkspace(
	ctx context.Context,
	intake CreativeIntake,
	task CreativeTask,
	draft VideoDraft,
	fixtureID string,
	fixtureVersion int64,
	templateID string,
) (TaskDetail, bool, error) {
	if r.DB == nil {
		return TaskDetail{}, false, fmt.Errorf("creative MySQL database is required")
	}
	existing, err := r.commerceWorkspaceTaskID(
		ctx, intake.OrganizationID, intake.ProjectID, fixtureID, fixtureVersion,
	)
	if err == nil {
		value, readErr := r.GetCommerceWorkspace(ctx, intake.OrganizationID, intake.ProjectID, existing)
		return value, false, readErr
	}
	if !errors.Is(err, ErrNotFound) {
		return TaskDetail{}, false, err
	}
	requestPayload, err := json.Marshal(intake.Request)
	if err != nil {
		return TaskDetail{}, false, fmt.Errorf("encode commerce intake: %w", err)
	}
	missingFields, _ := json.Marshal(intake.MissingFields)
	warnings, _ := json.Marshal(intake.Warnings)
	direction, err := json.Marshal(task.Direction)
	if err != nil {
		return TaskDetail{}, false, fmt.Errorf("encode commerce direction: %w", err)
	}
	draftPayload, err := json.Marshal(draft)
	if err != nil {
		return TaskDetail{}, false, fmt.Errorf("encode commerce draft: %w", err)
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return TaskDetail{}, false, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO creative_intakes (
		id, organization_id, project_id, principal_kind, principal_id, source_type, status,
		request_payload, missing_fields, warnings, confirmed_by, idempotency_key, request_hash,
		version, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		intake.ID, intake.OrganizationID, intake.ProjectID, intake.Principal.Kind, intake.Principal.ID,
		intake.Source, intake.Status, requestPayload, missingFields, warnings, intake.ConfirmedBy,
		intake.IdempotencyKey, intake.RequestHash, intake.Version, intake.CreatedAt, intake.UpdatedAt,
	)
	if err == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO creative_tasks (
			id, organization_id, project_id, intake_id, creative_format, channel, video_purpose,
			performance_mode, status, direction_payload, version, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			task.ID, task.OrganizationID, task.ProjectID, task.IntakeID, task.Format, task.Channel,
			task.VideoPurpose, task.PerformanceMode, task.Status, direction, task.Version,
			task.CreatedAt, task.UpdatedAt,
		)
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO creative_video_drafts
			(organization_id, task_id, revision, content_payload, created_at)
			VALUES (?, ?, ?, ?, ?)`,
			task.OrganizationID, task.ID, draft.Revision, draftPayload, draft.CreatedAt,
		)
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO creative_commerce_preroll_workspaces (
			organization_id, project_id, fixture_id, fixture_version, template_id,
			intake_id, task_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			task.OrganizationID, task.ProjectID, fixtureID, fixtureVersion, templateID,
			intake.ID, task.ID, task.CreatedAt, task.UpdatedAt,
		)
	}
	if err != nil {
		var mysqlError *mysqlDriver.MySQLError
		if !errors.As(err, &mysqlError) || mysqlError.Number != 1062 {
			return TaskDetail{}, false, err
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return TaskDetail{}, false, rollbackErr
		}
		existing, readErr := r.commerceWorkspaceTaskID(
			ctx, intake.OrganizationID, intake.ProjectID, fixtureID, fixtureVersion,
		)
		if readErr != nil {
			return TaskDetail{}, false, readErr
		}
		value, readErr := r.GetCommerceWorkspace(ctx, intake.OrganizationID, intake.ProjectID, existing)
		return value, false, readErr
	}
	if err := tx.Commit(); err != nil {
		return TaskDetail{}, false, err
	}
	value, err := r.GetCommerceWorkspace(ctx, intake.OrganizationID, intake.ProjectID, task.ID)
	return value, true, err
}

func (r MySQLRepository) GetLatestCommerceWorkspace(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
) (TaskDetail, error) {
	if r.DB == nil {
		return TaskDetail{}, fmt.Errorf("creative MySQL database is required")
	}
	var taskID string
	err := r.DB.QueryRowContext(ctx, `SELECT task_id
		FROM creative_commerce_preroll_workspaces
		WHERE organization_id = ? AND project_id = ?
		ORDER BY updated_at DESC LIMIT 1`,
		organizationID, projectID,
	).Scan(&taskID)
	if errors.Is(err, sql.ErrNoRows) {
		return TaskDetail{}, ErrNotFound
	}
	if err != nil {
		return TaskDetail{}, err
	}
	return r.GetCommerceWorkspace(ctx, organizationID, projectID, taskID)
}

func (r MySQLRepository) GetCommerceWorkspace(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	taskID string,
) (TaskDetail, error) {
	value, err := r.GetTaskDetail(ctx, organizationID, projectID, taskID)
	if err != nil {
		return TaskDetail{}, err
	}
	attempts, err := r.commerceGenerationAttempts(ctx, organizationID, projectID, taskID)
	if err != nil {
		return TaskDetail{}, err
	}
	value.CommerceGenerationAttempts = attempts
	return value, nil
}

func (r MySQLRepository) CreateCommerceGenerationAttempt(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	attempt CommerceGenerationAttempt,
) (CommerceGenerationAttempt, error) {
	if r.DB == nil {
		return CommerceGenerationAttempt{}, fmt.Errorf("creative MySQL database is required")
	}
	_, err := r.DB.ExecContext(ctx, `INSERT INTO creative_commerce_preroll_generation_attempts (
		id, organization_id, project_id, task_id, draft_revision, template_id, template_version,
		prompt_hash, generation_spec_hash, provider_job_id, retry_of_attempt_id, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?)`,
		attempt.ID, organizationID, projectID, attempt.TaskID, attempt.DraftRevision,
		attempt.Template.ID, attempt.Template.Version, attempt.PromptHash,
		attempt.GenerationSpecHash, attempt.ProviderJobID, attempt.RetryOfAttemptID,
		attempt.CreatedAt, attempt.CreatedAt,
	)
	if err == nil {
		return attempt, nil
	}
	var mysqlError *mysqlDriver.MySQLError
	if !errors.As(err, &mysqlError) || mysqlError.Number != 1062 {
		return CommerceGenerationAttempt{}, err
	}
	return r.commerceGenerationAttemptByProviderJob(
		ctx, organizationID, projectID, attempt.ProviderJobID,
	)
}

func (r MySQLRepository) commerceWorkspaceTaskID(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	fixtureID string,
	fixtureVersion int64,
) (string, error) {
	var taskID string
	err := r.DB.QueryRowContext(ctx, `SELECT task_id
		FROM creative_commerce_preroll_workspaces
		WHERE organization_id = ? AND project_id = ? AND fixture_id = ? AND fixture_version = ?`,
		organizationID, projectID, fixtureID, fixtureVersion,
	).Scan(&taskID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return taskID, err
}

func (r MySQLRepository) commerceGenerationAttempts(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	taskID string,
) ([]CommerceGenerationAttempt, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT
		id, task_id, draft_revision, template_id, template_version, prompt_hash,
		generation_spec_hash, provider_job_id, COALESCE(retry_of_attempt_id, ''),
		output_asset_id, output_asset_version, created_at
		FROM creative_commerce_preroll_generation_attempts
		WHERE organization_id = ? AND project_id = ? AND task_id = ?
		ORDER BY created_at`,
		organizationID, projectID, taskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]CommerceGenerationAttempt, 0)
	for rows.Next() {
		value, scanErr := scanCommerceGenerationAttempt(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r MySQLRepository) commerceGenerationAttemptByProviderJob(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	providerJobID string,
) (CommerceGenerationAttempt, error) {
	return scanCommerceGenerationAttempt(r.DB.QueryRowContext(ctx, `SELECT
		id, task_id, draft_revision, template_id, template_version, prompt_hash,
		generation_spec_hash, provider_job_id, COALESCE(retry_of_attempt_id, ''),
		output_asset_id, output_asset_version, created_at
		FROM creative_commerce_preroll_generation_attempts
		WHERE organization_id = ? AND project_id = ? AND provider_job_id = ?`,
		organizationID, projectID, providerJobID,
	))
}

type commerceGenerationAttemptScanner interface {
	Scan(...any) error
}

func scanCommerceGenerationAttempt(scanner commerceGenerationAttemptScanner) (CommerceGenerationAttempt, error) {
	var value CommerceGenerationAttempt
	var outputAssetID sql.NullString
	var outputAssetVersion sql.NullInt64
	if err := scanner.Scan(
		&value.ID, &value.TaskID, &value.DraftRevision,
		&value.Template.ID, &value.Template.Version, &value.PromptHash,
		&value.GenerationSpecHash, &value.ProviderJobID, &value.RetryOfAttemptID,
		&outputAssetID, &outputAssetVersion, &value.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CommerceGenerationAttempt{}, ErrNotFound
		}
		return CommerceGenerationAttempt{}, err
	}
	if outputAssetID.Valid && outputAssetVersion.Valid {
		value.OutputAssetVersion = &contract.AssetVersionRef{
			AssetID: contract.AssetID(outputAssetID.String),
			Version: outputAssetVersion.Int64,
		}
	}
	return value, nil
}
