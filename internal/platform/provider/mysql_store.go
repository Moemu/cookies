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
type MySQLStore struct {
	DB                *sql.DB
	AllowInsecureHTTP bool
}

func (s MySQLStore) Create(ctx context.Context, record JobRecord) (JobRecord, bool, error) {
	if s.DB == nil {
		return JobRecord{}, false, fmt.Errorf("MySQL database is required")
	}
	if err := validateRecord(record, s.AllowInsecureHTTP); err != nil {
		return JobRecord{}, false, err
	}
	payload, err := json.Marshal(record.Input)
	if err != nil {
		return JobRecord{}, false, fmt.Errorf("encode provider input: %w", err)
	}
	job := record.Job
	routeSnapshot, err := marshalRouteSnapshot(record.Route, s.AllowInsecureHTTP)
	if err != nil {
		return JobRecord{}, false, err
	}
	submissionState := record.SubmissionState
	if submissionState == "" {
		submissionState = SubmissionNotStarted
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO provider_jobs (
		id, organization_id, project_id, principal_kind, principal_id, operation_name,
		idempotency_key, request_hash, kind, model_alias, route_snapshot, source_system, source_task_id,
		project_context_version, execution_status, provider_status, progress, provider_code, model_version, external_task_id, input_payload,
		submission_state, adapter_request_id, actual_provider, actual_model, execution_deadline_at, submitted_at, response_received_at,
		attempt_count, max_attempts, version, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?,
		?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.OrganizationID, job.ProjectID, record.Principal.Kind, record.Principal.ID, record.Operation,
		record.IdempotencyKey, record.RequestHash, job.Kind, record.ModelAlias, routeSnapshot, record.SourceSystem, record.SourceTaskID,
		record.ProjectContextVersion, job.ExecutionStatus, job.ProviderStatus, job.Progress,
		record.ProviderCode, record.ModelVersion, record.ExternalTaskID, payload,
		submissionState, record.AdapterRequestID, record.ActualProvider, record.ActualModel,
		record.ExecutionDeadlineAt, record.SubmittedAt, record.ResponseReceivedAt,
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
		return JobRecord{}, false, fmt.Errorf("load existing provider idempotency record: %w", getErr)
	}
	if existing.RequestHash != record.RequestHash {
		return JobRecord{}, false, ErrIdempotencyConflict
	}
	return existing, true, nil
}

func (s MySQLStore) Get(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, jobID string) (JobRecord, error) {
	if s.DB == nil {
		return JobRecord{}, fmt.Errorf("MySQL database is required")
	}
	row := s.DB.QueryRowContext(ctx, `SELECT
		id, organization_id, project_id, principal_kind, principal_id, operation_name,
		idempotency_key, request_hash, kind, model_alias, route_snapshot, source_system, source_task_id,
		project_context_version, execution_status, provider_status, progress, provider_code, model_version, external_task_id, input_payload,
		submission_state, adapter_request_id, actual_provider, actual_model, execution_deadline_at, submitted_at, response_received_at,
		error_code, error_message, retryable, attempt_count, max_attempts, version, created_at, updated_at
		FROM provider_jobs WHERE id = ? AND organization_id = ? AND project_id = ?`, jobID, organizationID, projectID)
	record, err := scanRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return JobRecord{}, ErrJobNotFound
	}
	if err != nil {
		return JobRecord{}, err
	}
	outputs, err := s.loadOutputs(ctx, record.Job.ID, record.Job.ProjectID)
	if err != nil {
		return JobRecord{}, err
	}
	record.Outputs = outputs
	record.Job.ProjectAssetRefs = projectAssetRefs(outputs)
	return record, nil
}

func (s MySQLStore) Update(ctx context.Context, record JobRecord) (JobRecord, error) {
	if s.DB == nil {
		return JobRecord{}, fmt.Errorf("MySQL database is required")
	}
	if err := validateRecord(record, s.AllowInsecureHTTP); err != nil {
		return JobRecord{}, err
	}
	payload, err := json.Marshal(record.Input)
	if err != nil {
		return JobRecord{}, fmt.Errorf("encode provider input: %w", err)
	}
	var errorCode, errorMessage any
	var retryable any
	if record.Job.Error != nil {
		errorCode = record.Job.Error.Code
		errorMessage = record.Job.Error.Message
		retryable = record.Job.Error.Retryable
	}
	routeSnapshot, err := marshalRouteSnapshot(record.Route, s.AllowInsecureHTTP)
	if err != nil {
		return JobRecord{}, err
	}
	submissionState := record.SubmissionState
	if submissionState == "" {
		submissionState = SubmissionNotStarted
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return JobRecord{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE provider_jobs SET
		model_alias = ?, route_snapshot = ?, source_system = NULLIF(?, ''), source_task_id = NULLIF(?, ''),
		project_context_version = ?, execution_status = ?, provider_status = ?, progress = ?,
		provider_code = NULLIF(?, ''), model_version = NULLIF(?, ''), external_task_id = NULLIF(?, ''),
		input_payload = ?, submission_state = ?, adapter_request_id = NULLIF(?, ''),
		actual_provider = NULLIF(?, ''), actual_model = NULLIF(?, ''), execution_deadline_at = ?,
		submitted_at = ?, response_received_at = ?, error_code = ?, error_message = ?, retryable = ?, attempt_count = ?,
		max_attempts = ?, version = version + 1, updated_at = ?
		WHERE id = ? AND organization_id = ? AND project_id = ? AND version = ?`,
		record.ModelAlias, routeSnapshot, record.SourceSystem, record.SourceTaskID, record.ProjectContextVersion,
		record.Job.ExecutionStatus, record.Job.ProviderStatus, record.Job.Progress,
		record.ProviderCode, record.ModelVersion, record.ExternalTaskID, payload,
		submissionState, record.AdapterRequestID, record.ActualProvider, record.ActualModel,
		record.ExecutionDeadlineAt, record.SubmittedAt, record.ResponseReceivedAt,
		errorCode, errorMessage, retryable, record.Job.AttemptCount, record.Job.MaxAttempts, record.Job.UpdatedAt,
		record.Job.ID, record.Job.OrganizationID, record.Job.ProjectID, record.Job.Version)
	if err != nil {
		return JobRecord{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return JobRecord{}, err
	}
	if changed != 1 {
		return JobRecord{}, ErrVersionConflict
	}
	for _, output := range record.Outputs {
		if err := upsertOutput(ctx, tx, record.Job.ID, output); err != nil {
			return JobRecord{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return JobRecord{}, err
	}
	record.Job.Version++
	return record, nil
}

func (s MySQLStore) getByIdempotency(ctx context.Context, record JobRecord) (JobRecord, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT
		id, organization_id, project_id, principal_kind, principal_id, operation_name,
		idempotency_key, request_hash, kind, model_alias, route_snapshot, source_system, source_task_id,
		project_context_version, execution_status, provider_status, progress, provider_code, model_version, external_task_id, input_payload,
		submission_state, adapter_request_id, actual_provider, actual_model, execution_deadline_at, submitted_at, response_received_at,
		error_code, error_message, retryable, attempt_count, max_attempts, version, created_at, updated_at
		FROM provider_jobs
		WHERE organization_id = ? AND project_id = ? AND principal_kind = ? AND principal_id = ?
			AND operation_name = ? AND idempotency_key = ?`,
		record.Job.OrganizationID, record.Job.ProjectID, record.Principal.Kind, record.Principal.ID,
		record.Operation, record.IdempotencyKey)
	existing, err := scanRecord(row)
	if err != nil {
		return JobRecord{}, err
	}
	outputs, err := s.loadOutputs(ctx, existing.Job.ID, existing.Job.ProjectID)
	if err != nil {
		return JobRecord{}, err
	}
	existing.Outputs = outputs
	existing.Job.ProjectAssetRefs = projectAssetRefs(outputs)
	return existing, nil
}

func projectAssetRefs(outputs []OutputRecord) []contract.ProjectAssetRef {
	refs := make([]contract.ProjectAssetRef, 0, len(outputs))
	for _, output := range outputs {
		if output.Status == OutputSucceeded && output.ProjectAssetRef != nil {
			refs = append(refs, *output.ProjectAssetRef)
		}
	}
	return refs
}

func (s MySQLStore) loadOutputs(ctx context.Context, jobID string, projectID contract.ProjectID) ([]OutputRecord, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT output_id, provider_code, retrieval_expires_at, declared_mime_type,
		declared_size_bytes, declared_sha256, output_status, intake_id, asset_id, asset_version,
		error_code, error_message, retryable
		FROM provider_job_outputs WHERE provider_job_id = ? ORDER BY output_id`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	outputs := make([]OutputRecord, 0)
	for rows.Next() {
		var output OutputRecord
		var declaredSHA256, intakeID, assetID, errorCode, errorMessage sql.NullString
		var assetVersion sql.NullInt64
		var retryable sql.NullBool
		if err := rows.Scan(&output.Ref.OutputID, &output.Ref.ProviderCode, &output.Ref.RetrievalExpiresAt,
			&output.Ref.DeclaredMIMEType, &output.Ref.DeclaredSizeBytes, &declaredSHA256, &output.Status,
			&intakeID, &assetID, &assetVersion, &errorCode, &errorMessage, &retryable); err != nil {
			return nil, err
		}
		output.Ref.ProviderJobID = jobID
		if declaredSHA256.Valid {
			value := declaredSHA256.String
			output.Ref.DeclaredSHA256 = &value
		}
		output.IntakeID = intakeID.String
		if assetID.Valid && assetVersion.Valid {
			output.ProjectAssetRef = &contract.ProjectAssetRef{ProjectID: projectID, AssetVersion: contract.AssetVersionRef{AssetID: contract.AssetID(assetID.String), Version: assetVersion.Int64}}
		}
		if errorCode.Valid {
			output.Error = &contract.JobError{Code: errorCode.String, Message: errorMessage.String, Retryable: retryable.Bool}
		}
		outputs = append(outputs, output)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return outputs, nil
}

func upsertOutput(ctx context.Context, tx *sql.Tx, jobID string, output OutputRecord) error {
	if err := validateOutput(jobID, output); err != nil {
		return err
	}
	var declaredSHA256 any
	if output.Ref.DeclaredSHA256 != nil {
		declaredSHA256 = *output.Ref.DeclaredSHA256
	}
	var assetID, assetVersion any
	if output.ProjectAssetRef != nil {
		assetID = output.ProjectAssetRef.AssetVersion.AssetID
		assetVersion = output.ProjectAssetRef.AssetVersion.Version
	}
	var errorCode, errorMessage, retryable any
	if output.Error != nil {
		errorCode, errorMessage, retryable = output.Error.Code, output.Error.Message, output.Error.Retryable
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO provider_job_outputs (
		provider_job_id, output_id, provider_code, retrieval_expires_at, declared_mime_type,
		declared_size_bytes, declared_sha256, output_status, intake_id, asset_id, asset_version,
		error_code, error_message, retryable
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE provider_code = VALUES(provider_code), retrieval_expires_at = VALUES(retrieval_expires_at),
		declared_mime_type = VALUES(declared_mime_type), declared_size_bytes = VALUES(declared_size_bytes),
		declared_sha256 = VALUES(declared_sha256), output_status = VALUES(output_status), intake_id = VALUES(intake_id),
		asset_id = VALUES(asset_id), asset_version = VALUES(asset_version), error_code = VALUES(error_code),
		error_message = VALUES(error_message), retryable = VALUES(retryable)`,
		jobID, output.Ref.OutputID, output.Ref.ProviderCode, output.Ref.RetrievalExpiresAt,
		output.Ref.DeclaredMIMEType, output.Ref.DeclaredSizeBytes, declaredSHA256, output.Status, output.IntakeID,
		assetID, assetVersion, errorCode, errorMessage, retryable)
	return err
}

type rowScanner interface{ Scan(...any) error }

func scanRecord(row rowScanner) (JobRecord, error) {
	var record JobRecord
	var input json.RawMessage
	var sourceSystem, sourceTaskID, routeSnapshot, providerCode, modelVersion, externalTaskID, adapterRequestID, actualProvider, actualModel, errorCode, errorMessage sql.NullString
	var executionDeadlineAt, submittedAt, responseReceivedAt sql.NullTime
	var retryable sql.NullBool
	err := row.Scan(
		&record.Job.ID, &record.Job.OrganizationID, &record.Job.ProjectID, &record.Principal.Kind, &record.Principal.ID, &record.Operation,
		&record.IdempotencyKey, &record.RequestHash, &record.Job.Kind, &record.ModelAlias, &routeSnapshot, &sourceSystem, &sourceTaskID,
		&record.ProjectContextVersion, &record.Job.ExecutionStatus, &record.Job.ProviderStatus, &record.Job.Progress,
		&providerCode, &modelVersion, &externalTaskID, &input,
		&record.SubmissionState, &adapterRequestID, &actualProvider, &actualModel, &executionDeadlineAt, &submittedAt, &responseReceivedAt,
		&errorCode, &errorMessage, &retryable, &record.Job.AttemptCount, &record.Job.MaxAttempts, &record.Job.Version,
		&record.Job.CreatedAt, &record.Job.UpdatedAt,
	)
	if err != nil {
		return JobRecord{}, err
	}
	record.SourceSystem = sourceSystem.String
	record.SourceTaskID = sourceTaskID.String
	record.ProviderCode = providerCode.String
	record.ModelVersion = modelVersion.String
	record.ExternalTaskID = externalTaskID.String
	record.AdapterRequestID = adapterRequestID.String
	record.ActualProvider = actualProvider.String
	record.ActualModel = actualModel.String
	if executionDeadlineAt.Valid {
		value := executionDeadlineAt.Time
		record.ExecutionDeadlineAt = &value
	}
	if submittedAt.Valid {
		value := submittedAt.Time
		record.SubmittedAt = &value
	}
	if responseReceivedAt.Valid {
		value := responseReceivedAt.Time
		record.ResponseReceivedAt = &value
	}
	if routeSnapshot.Valid && routeSnapshot.String != "" && routeSnapshot.String != "null" {
		var snapshot ImageRouteSnapshot
		if err := json.Unmarshal([]byte(routeSnapshot.String), &snapshot); err != nil {
			return JobRecord{}, fmt.Errorf("decode provider route snapshot: %w", err)
		}
		record.Route = &snapshot
	}
	record.Job.ProjectAssetRefs = []contract.ProjectAssetRef{}
	if errorCode.Valid {
		record.Job.Error = &contract.JobError{Code: errorCode.String, Message: errorMessage.String, Retryable: retryable.Bool}
	}
	if err := json.Unmarshal(input, &record.Input); err != nil {
		return JobRecord{}, fmt.Errorf("decode provider input: %w", err)
	}
	return record, nil
}

func validateRecord(record JobRecord, allowInsecureHTTP bool) error {
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
	if err := record.Input.Validate(); err != nil {
		return err
	}
	if record.Route != nil {
		if err := record.Route.ValidateWithPolicy(allowInsecureHTTP); err != nil {
			return err
		}
		switch record.SubmissionState {
		case SubmissionNotStarted, SubmissionInFlight, SubmissionCompleted, SubmissionUnknown:
		default:
			return fmt.Errorf("provider submission state is invalid")
		}
	}
	for _, output := range record.Outputs {
		if err := validateOutput(record.Job.ID, output); err != nil {
			return err
		}
	}
	return nil
}

func marshalRouteSnapshot(snapshot *ImageRouteSnapshot, allowInsecureHTTP bool) (any, error) {
	if snapshot == nil {
		return nil, nil
	}
	if err := snapshot.ValidateWithPolicy(allowInsecureHTTP); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("encode provider route snapshot: %w", err)
	}
	return encoded, nil
}

func validateOutput(jobID string, output OutputRecord) error {
	if err := output.Ref.Validate(); err != nil {
		return fmt.Errorf("invalid provider output: %w", err)
	}
	if output.Ref.ProviderJobID != jobID {
		return fmt.Errorf("provider output belongs to another job")
	}
	switch output.Status {
	case OutputReady, OutputIngesting:
		if output.ProjectAssetRef != nil || output.Error != nil {
			return fmt.Errorf("pending output cannot include an asset or error")
		}
	case OutputSucceeded:
		if output.ProjectAssetRef == nil || output.Error != nil {
			return fmt.Errorf("succeeded output requires one project asset and no error")
		}
		if err := output.ProjectAssetRef.Validate(); err != nil {
			return err
		}
	case OutputFailed:
		if output.ProjectAssetRef != nil || output.Error == nil || strings.TrimSpace(output.Error.Code) == "" {
			return fmt.Errorf("failed output requires one error and no project asset")
		}
	default:
		return fmt.Errorf("provider output status is invalid")
	}
	return nil
}
