package jobruntime

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

type MySQLStore struct{ DB *sql.DB }

func (s MySQLStore) Enqueue(ctx context.Context, request CreateRequest) (contract.Job, bool, error) {
	if s.DB == nil {
		return contract.Job{}, false, fmt.Errorf("MySQL database is required")
	}
	if err := request.Validate(); err != nil {
		return contract.Job{}, false, err
	}
	job := request.Job
	_, err := s.DB.ExecContext(ctx, `INSERT INTO platform_jobs (
		id, kind, organization_id, project_id, status, progress, payload, cancellable,
		version, idempotency_key, request_hash, attempt_count, max_attempts, available_at, created_at, updated_at
	) VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.Kind, job.OrganizationID, job.ProjectID, job.Status, job.Progress, request.Payload,
		job.Cancellable, job.Version, request.IdempotencyKey, request.RequestHash, job.AttemptCount,
		job.MaxAttempts, job.CreatedAt, job.CreatedAt, job.UpdatedAt)
	if err == nil {
		return job, false, nil
	}
	var mysqlError *mysqlDriver.MySQLError
	if !errors.As(err, &mysqlError) || mysqlError.Number != 1062 {
		return contract.Job{}, false, err
	}
	var storedHash string
	existing, getErr := s.getByIdempotency(ctx, job.OrganizationID, request.IdempotencyKey, &storedHash)
	if getErr != nil {
		return contract.Job{}, false, err
	}
	if storedHash != request.RequestHash {
		return contract.Job{}, false, ErrIdempotencyConflict
	}
	return existing, true, nil
}

func (s MySQLStore) Claim(ctx context.Context, workerID string, now time.Time) (Claim, bool, error) {
	if s.DB == nil || workerID == "" {
		return Claim{}, false, fmt.Errorf("MySQL database and worker ID are required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Claim{}, false, err
	}
	defer tx.Rollback()
	row := tx.QueryRowContext(ctx, `SELECT id, kind, organization_id, project_id, status, progress, payload, cancellable, version, attempt_count, max_attempts, created_at, updated_at
		FROM platform_jobs WHERE status = 'queued' AND available_at <= ? ORDER BY available_at, created_at LIMIT 1 FOR UPDATE SKIP LOCKED`, now)
	job, payload, err := scanJob(row)
	if err == sql.ErrNoRows {
		return Claim{}, false, nil
	}
	if err != nil {
		return Claim{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE platform_jobs SET status = 'running', lock_owner = ?, locked_at = ?, attempt_count = attempt_count + 1, version = version + 1, updated_at = ? WHERE id = ?`, workerID, now, now, job.ID); err != nil {
		return Claim{}, false, err
	}
	job.Status = contract.JobRunning
	job.AttemptCount++
	job.Version++
	job.UpdatedAt = now
	if err := tx.Commit(); err != nil {
		return Claim{}, false, err
	}
	return Claim{Job: job, Payload: payload, LockOwner: workerID}, true, nil
}

func (s MySQLStore) Succeed(ctx context.Context, claim Claim, result Result, now time.Time) error {
	if result.Ref != nil {
		if err := result.Ref.Validate(); err != nil {
			return err
		}
	}
	var resultType, resultID any
	var resultVersion any
	if result.Ref != nil {
		resultType, resultID, resultVersion = result.Ref.Type, result.Ref.ID, result.Ref.Version
	}
	return s.transition(ctx, claim, contract.JobSucceeded, nil, resultType, resultID, resultVersion, now)
}

func (s MySQLStore) Fail(ctx context.Context, claim Claim, problem contract.JobError, now time.Time) error {
	if problem.Code == "" {
		return fmt.Errorf("job failure code is required")
	}
	return s.transition(ctx, claim, contract.JobFailed, &problem, nil, nil, nil, now)
}

// Reschedule releases a running claim for a later attempt. It deliberately
// keeps the attempt count: an execution attempt already happened and retry
// policy belongs to the domain handler that selected the next time.
func (s MySQLStore) Reschedule(ctx context.Context, claim Claim, availableAt time.Time, now time.Time) error {
	if availableAt.IsZero() || !availableAt.After(now) {
		return fmt.Errorf("reschedule time must be after now")
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE platform_jobs
		SET status = 'queued', available_at = ?, lock_owner = NULL, locked_at = NULL,
			version = version + 1, updated_at = ?
		WHERE id = ? AND status = 'running' AND lock_owner = ?`, availableAt, now, claim.Job.ID, claim.LockOwner)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return fmt.Errorf("job %s is no longer owned by worker %s", claim.Job.ID, claim.LockOwner)
	}
	return nil
}

// ReclaimExpired returns abandoned running jobs to the queue. It does not mark
// a job terminal at recovery time: the domain handler must get one final
// chance to synchronize its public state before Worker applies the attempt
// policy to a deferred or failed execution.
func (s MySQLStore) ReclaimExpired(ctx context.Context, now time.Time, leaseDuration time.Duration) (LeaseRecovery, error) {
	if s.DB == nil {
		return LeaseRecovery{}, fmt.Errorf("MySQL database is required")
	}
	if leaseDuration <= 0 {
		return LeaseRecovery{}, fmt.Errorf("lease duration must be positive")
	}
	deadline := now.Add(-leaseDuration)
	rescheduled, err := s.DB.ExecContext(ctx, `UPDATE platform_jobs
		SET status = 'queued', available_at = ?, lock_owner = NULL, locked_at = NULL,
			version = version + 1, updated_at = ?
		WHERE status = 'running' AND locked_at <= ?`, now, now, deadline)
	if err != nil {
		return LeaseRecovery{}, err
	}
	rescheduledCount, err := rescheduled.RowsAffected()
	if err != nil {
		return LeaseRecovery{}, err
	}
	return LeaseRecovery{Rescheduled: rescheduledCount}, nil
}

func (s MySQLStore) transition(ctx context.Context, claim Claim, status contract.JobStatus, problem *contract.JobError, resultType, resultID, resultVersion any, now time.Time) error {
	var code, message any
	var retryable any
	if problem != nil {
		code, message, retryable = problem.Code, problem.Message, problem.Retryable
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE platform_jobs SET status = ?, result_type = ?, result_id = ?, result_version = ?, error_code = ?, error_message = ?, retryable = ?, lock_owner = NULL, locked_at = NULL, version = version + 1, updated_at = ? WHERE id = ? AND status = 'running' AND lock_owner = ?`, status, resultType, resultID, resultVersion, code, message, retryable, now, claim.Job.ID, claim.LockOwner)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return fmt.Errorf("job %s is no longer owned by worker %s", claim.Job.ID, claim.LockOwner)
	}
	return nil
}

func (s MySQLStore) getByIdempotency(ctx context.Context, organizationID contract.OrganizationID, key contract.IdempotencyKey, requestHash *string) (contract.Job, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id, kind, organization_id, project_id, status, progress, payload, cancellable, version, attempt_count, max_attempts, created_at, updated_at, request_hash FROM platform_jobs WHERE organization_id = ? AND idempotency_key = ?`, organizationID, key)
	job, _, err := scanJobWithHash(row, requestHash)
	return job, err
}

type scanner interface{ Scan(...any) error }

func scanJob(row scanner) (contract.Job, json.RawMessage, error) {
	return scanJobWithHash(row, nil)
}

func scanJobWithHash(row scanner, requestHash *string) (contract.Job, json.RawMessage, error) {
	var job contract.Job
	var projectID sql.NullString
	var payload json.RawMessage
	if requestHash == nil {
		err := row.Scan(&job.ID, &job.Kind, &job.OrganizationID, &projectID, &job.Status, &job.Progress, &payload, &job.Cancellable, &job.Version, &job.AttemptCount, &job.MaxAttempts, &job.CreatedAt, &job.UpdatedAt)
		job.ProjectID = contract.ProjectID(projectID.String)
		return job, payload, err
	}
	err := row.Scan(&job.ID, &job.Kind, &job.OrganizationID, &projectID, &job.Status, &job.Progress, &payload, &job.Cancellable, &job.Version, &job.AttemptCount, &job.MaxAttempts, &job.CreatedAt, &job.UpdatedAt, requestHash)
	job.ProjectID = contract.ProjectID(projectID.String)
	return job, payload, err
}
