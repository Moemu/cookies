package provider

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	mysqlDriver "github.com/go-sql-driver/mysql"

	"github.com/shikanon/cookies/internal/platform/contract"
)

var ErrIdempotencyConflict = errors.New("provider idempotency key was reused with a different request")

// MySQLStore persists only Provider-owned data. Assets records and object
// storage details never appear in this repository.
type MySQLStore struct{ DB *sql.DB }

func (s MySQLStore) Create(ctx context.Context, record JobRecord) (JobRecord, bool, error) {
	if s.DB == nil {
		return JobRecord{}, false, fmt.Errorf("MySQL database is required")
	}
	if err := validateRecord(record); err != nil {
		return JobRecord{}, false, err
	}
	payload, err := json.Marshal(record.Input)
	if err != nil {
		return JobRecord{}, false, fmt.Errorf("encode provider input: %w", err)
	}
	job := record.Job
	_, err = s.DB.ExecContext(ctx, `INSERT INTO provider_jobs (
		id, organization_id, project_id, principal_kind, principal_id, operation_name,
		idempotency_key, request_hash, kind, model_alias, source_system, source_task_id,
		project_context_version, execution_status, provider_status, progress, input_payload,
		attempt_count, max_attempts, version, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.OrganizationID, job.ProjectID, record.Principal.Kind, record.Principal.ID, record.Operation,
		record.IdempotencyKey, record.RequestHash, job.Kind, record.ModelAlias, record.SourceSystem, record.SourceTaskID,
		record.ProjectContextVersion, job.ExecutionStatus, job.ProviderStatus, job.Progress, payload,
		job.AttemptCount, job.MaxAttempts, job.Version, job.CreatedAt, job.UpdatedAt)
	if err == nil {
		return record, false, nil
	}
	var mysqlError *mysqlDriver.MySQLError
	if !errors.As(err, &mysqlError) || mysqlError.Number != 1062 {
		return JobRecord{}, false, err
	}
	existing, getErr := s.getByIdempotency(ctx, record)
	if getErr != nil {
		return JobRecord{}, false, err
	}
	if existing.RequestHash != record.RequestHash {
		return JobRecord{}, false, ErrIdempotencyConflict
	}
	return existing, true, nil
}

func (s MySQLStore) getByIdempotency(ctx context.Context, record JobRecord) (JobRecord, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT
		id, organization_id, project_id, principal_kind, principal_id, operation_name,
		idempotency_key, request_hash, kind, model_alias, source_system, source_task_id,
		project_context_version, execution_status, provider_status, progress, input_payload,
		error_code, error_message, retryable, attempt_count, max_attempts, version, created_at, updated_at
		FROM provider_jobs
		WHERE organization_id = ? AND project_id = ? AND principal_kind = ? AND principal_id = ?
			AND operation_name = ? AND idempotency_key = ?`,
		record.Job.OrganizationID, record.Job.ProjectID, record.Principal.Kind, record.Principal.ID,
		record.Operation, record.IdempotencyKey)
	return scanRecord(row)
}

type rowScanner interface{ Scan(...any) error }

func scanRecord(row rowScanner) (JobRecord, error) {
	var record JobRecord
	var input json.RawMessage
	var sourceSystem, sourceTaskID, errorCode, errorMessage sql.NullString
	var retryable sql.NullBool
	err := row.Scan(
		&record.Job.ID, &record.Job.OrganizationID, &record.Job.ProjectID, &record.Principal.Kind, &record.Principal.ID, &record.Operation,
		&record.IdempotencyKey, &record.RequestHash, &record.Job.Kind, &record.ModelAlias, &sourceSystem, &sourceTaskID,
		&record.ProjectContextVersion, &record.Job.ExecutionStatus, &record.Job.ProviderStatus, &record.Job.Progress, &input,
		&errorCode, &errorMessage, &retryable, &record.Job.AttemptCount, &record.Job.MaxAttempts, &record.Job.Version,
		&record.Job.CreatedAt, &record.Job.UpdatedAt,
	)
	if err != nil {
		return JobRecord{}, err
	}
	record.SourceSystem = sourceSystem.String
	record.SourceTaskID = sourceTaskID.String
	record.Job.ProjectAssetRefs = []contract.ProjectAssetRef{}
	if errorCode.Valid {
		record.Job.Error = &contract.JobError{Code: errorCode.String, Message: errorMessage.String, Retryable: retryable.Bool}
	}
	if err := json.Unmarshal(input, &record.Input); err != nil {
		return JobRecord{}, fmt.Errorf("decode provider input: %w", err)
	}
	return record, nil
}

func validateRecord(record JobRecord) error {
	if err := record.Job.Validate(); err != nil {
		return err
	}
	if err := (contract.ActorContext{OrganizationID: record.Job.OrganizationID, Principal: record.Principal, Scopes: []contract.Scope{}}).Validate(); err != nil {
		return fmt.Errorf("invalid principal: %w", err)
	}
	if strings.TrimSpace(record.Operation) == "" {
		return fmt.Errorf("provider operation is required")
	}
	if err := record.IdempotencyKey.Validate(); err != nil {
		return err
	}
	if !validSHA256(record.RequestHash) {
		return fmt.Errorf("request hash must be a lowercase hexadecimal SHA-256 digest")
	}
	if record.ProjectContextVersion < 1 {
		return fmt.Errorf("project_context_version must be positive")
	}
	if strings.TrimSpace(record.ModelAlias) == "" {
		return fmt.Errorf("model alias is required")
	}
	return record.Input.Validate()
}
