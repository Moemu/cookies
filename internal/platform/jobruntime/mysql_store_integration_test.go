package jobruntime

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestMySQLWorkerRestartAcrossProcesses(t *testing.T) {
	dsn := os.Getenv("COOKIES_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("COOKIES_TEST_MYSQL_DSN is not configured")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("ping MySQL: %v", err)
	}

	wallClock := time.Now().UTC().Truncate(time.Microsecond)
	jobID := "job_process_restart_" + strings.ReplaceAll(wallClock.Format("20060102150405.000000"), ".", "")
	now := wallClock.Add(24 * time.Hour)
	request := CreateRequest{
		Job: contract.Job{
			ID: jobID, Kind: "strategy.restart_probe",
			OrganizationID: contract.OrganizationID("org_" + jobID),
			ProjectID:      contract.ProjectID("project_" + jobID),
			Status:         contract.JobQueued, Cancellable: true, Version: 1, MaxAttempts: 2,
			CreatedAt: now, UpdatedAt: now,
		},
		Payload: []byte(`{"checkpoint":"durable"}`), IdempotencyKey: contract.IdempotencyKey("restart-" + jobID),
		RequestHash: strings.Repeat("d", 64),
	}
	store := MySQLStore{DB: db, ClaimOrganizationID: request.Job.OrganizationID, ClaimProjectID: request.Job.ProjectID, ClaimJobID: jobID}
	if _, _, err := store.Enqueue(t.Context(), request); err != nil {
		t.Fatalf("enqueue restart probe: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM platform_jobs WHERE id = ?", jobID)
	})

	runJobRuntimeHelperProcess(t, "crash_after_claim", dsn, request.Job, now.Add(time.Second))
	var status, lockOwner string
	if err := db.QueryRowContext(t.Context(), "SELECT status, lock_owner FROM platform_jobs WHERE id = ?", jobID).Scan(&status, &lockOwner); err != nil {
		t.Fatalf("read abandoned claim: %v", err)
	}
	if status != string(contract.JobRunning) || lockOwner != "worker-process-a" {
		t.Fatalf("abandoned claim = (status=%q, owner=%q)", status, lockOwner)
	}

	recovery, err := store.ReclaimExpired(t.Context(), now.Add(2*time.Minute), time.Minute)
	if err != nil || recovery.Rescheduled != 1 {
		t.Fatalf("reclaim abandoned process = (%+v, err=%v)", recovery, err)
	}
	runJobRuntimeHelperProcess(t, "complete", dsn, request.Job, now.Add(2*time.Minute+time.Second))

	var attempts int
	var persistedPayload string
	if err := db.QueryRowContext(t.Context(), "SELECT status, attempt_count, payload FROM platform_jobs WHERE id = ?", jobID).Scan(&status, &attempts, &persistedPayload); err != nil {
		t.Fatalf("read restarted process result: %v", err)
	}
	var restoredPayload map[string]string
	if err := json.Unmarshal([]byte(persistedPayload), &restoredPayload); err != nil {
		t.Fatalf("decode restarted process payload: %v", err)
	}
	if status != string(contract.JobSucceeded) || attempts != 2 || restoredPayload["checkpoint"] != "durable" {
		t.Fatalf("restarted process result = (status=%q, attempts=%d, payload=%q)", status, attempts, persistedPayload)
	}
}

func TestJobRuntimeWorkerHelperProcess(t *testing.T) {
	mode := os.Getenv("COOKIES_JOBRUNTIME_HELPER_MODE")
	if mode == "" {
		return
	}
	dsn := os.Getenv("COOKIES_JOBRUNTIME_HELPER_DSN")
	now, err := time.Parse(time.RFC3339Nano, os.Getenv("COOKIES_JOBRUNTIME_HELPER_NOW"))
	if err != nil {
		t.Fatalf("parse helper clock: %v", err)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open helper MySQL: %v", err)
	}
	defer db.Close()
	store := MySQLStore{
		DB:                  db,
		ClaimOrganizationID: contract.OrganizationID(os.Getenv("COOKIES_JOBRUNTIME_HELPER_ORGANIZATION_ID")),
		ClaimProjectID:      contract.ProjectID(os.Getenv("COOKIES_JOBRUNTIME_HELPER_PROJECT_ID")),
		ClaimJobID:          os.Getenv("COOKIES_JOBRUNTIME_HELPER_JOB_ID"),
	}
	if mode == "crash_after_claim" {
		claim, found, err := store.Claim(t.Context(), "worker-process-a", now)
		if err != nil || !found || claim.Job.ID != store.ClaimJobID {
			t.Fatalf("helper claim = (%+v, found=%t, err=%v)", claim, found, err)
		}
		return
	}
	if mode != "complete" {
		t.Fatalf("unknown helper mode %q", mode)
	}
	worker := Worker{
		Store: store, Now: func() time.Time { return now },
		Handlers: map[string]Handler{
			"strategy.restart_probe": func(_ context.Context, claim Claim) (Result, error) {
				var payload map[string]string
				if err := json.Unmarshal(claim.Payload, &payload); err != nil || payload["checkpoint"] != "durable" {
					t.Fatalf("helper restored payload %q", claim.Payload)
				}
				return Result{}, nil
			},
		},
	}
	processed, err := worker.RunOnce(t.Context(), "worker-process-b")
	if err != nil || !processed {
		t.Fatalf("replacement helper = (processed=%t, err=%v)", processed, err)
	}
}

func runJobRuntimeHelperProcess(t *testing.T, mode, dsn string, job contract.Job, now time.Time) {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestJobRuntimeWorkerHelperProcess$", "-test.v")
	command.Env = append(os.Environ(),
		"COOKIES_JOBRUNTIME_HELPER_MODE="+mode,
		"COOKIES_JOBRUNTIME_HELPER_DSN="+dsn,
		"COOKIES_JOBRUNTIME_HELPER_NOW="+now.Format(time.RFC3339Nano),
		"COOKIES_JOBRUNTIME_HELPER_ORGANIZATION_ID="+string(job.OrganizationID),
		"COOKIES_JOBRUNTIME_HELPER_PROJECT_ID="+string(job.ProjectID),
		"COOKIES_JOBRUNTIME_HELPER_JOB_ID="+job.ID,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("helper process %s failed: %v\n%s", mode, err, output)
	}
}

func TestMySQLStoreLifecycleIntegration(t *testing.T) {
	dsn := os.Getenv("COOKIES_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("COOKIES_TEST_MYSQL_DSN is not configured")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("ping MySQL: %v", err)
	}

	wallClock := time.Now().UTC().Truncate(time.Microsecond)
	jobID := "job_integration_" + strings.ReplaceAll(wallClock.Format("20060102150405.000000"), ".", "")
	// Production workers use the global queue by default. Keep this fixture in
	// the future and use a matching test clock so a live local worker sharing
	// the schema cannot claim it before the scoped test worker.
	now := wallClock.Add(24 * time.Hour)
	request := CreateRequest{
		Job: contract.Job{
			ID:             jobID,
			Kind:           "provider.generate_image",
			OrganizationID: contract.OrganizationID("org_integration_" + jobID),
			ProjectID:      contract.ProjectID("project_integration_" + jobID),
			Status:         contract.JobQueued,
			Cancellable:    true,
			Version:        1,
			MaxAttempts:    1,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		Payload:        []byte(`{"prompt":"integration test"}`),
		IdempotencyKey: contract.IdempotencyKey("integration-" + string(jobID)),
		RequestHash:    strings.Repeat("a", 64),
	}
	store := MySQLStore{
		DB: db, ClaimOrganizationID: request.Job.OrganizationID, ClaimProjectID: request.Job.ProjectID,
	}
	created, duplicate, err := store.Enqueue(t.Context(), request)
	if err != nil || duplicate || created.ID != jobID {
		t.Fatalf("enqueue = (%+v, duplicate=%t, err=%v)", created, duplicate, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM platform_jobs WHERE id = ?", jobID)
	})

	claim, found, err := store.Claim(t.Context(), "integration-worker", now.Add(time.Second))
	if err != nil || !found || claim.Job.ID != jobID {
		t.Fatalf("claim = (%+v, found=%t, err=%v)", claim, found, err)
	}
	version := int64(1)
	if err := store.Succeed(t.Context(), claim, Result{Ref: &contract.ResourceRef{Type: "provider_output", ID: "output_integration", Version: &version}}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("succeed: %v", err)
	}

	var status, resultType, resultID string
	var resultVersion int64
	if err := db.QueryRowContext(t.Context(), "SELECT status, result_type, result_id, result_version FROM platform_jobs WHERE id = ?", jobID).Scan(&status, &resultType, &resultID, &resultVersion); err != nil {
		t.Fatalf("read persisted job: %v", err)
	}
	if status != string(contract.JobSucceeded) || resultType != "provider_output" || resultID != "output_integration" || resultVersion != 1 {
		t.Fatalf("unexpected persisted job: status=%q type=%q id=%q version=%d", status, resultType, resultID, resultVersion)
	}
}

func TestMySQLStoreReclaimsExpiredLease(t *testing.T) {
	dsn := os.Getenv("COOKIES_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("COOKIES_TEST_MYSQL_DSN is not configured")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("ping MySQL: %v", err)
	}

	wallClock := time.Now().UTC().Truncate(time.Microsecond)
	jobID := "job_reclaim_" + strings.ReplaceAll(wallClock.Format("20060102150405.000000"), ".", "")
	// See TestMySQLStoreLifecycleIntegration: the production default remains
	// global while this future-scheduled fixture uses its own scoped test clock.
	now := wallClock.Add(24 * time.Hour)
	cancelJobID := jobID + "_cancel"
	exhaustedJobID := jobID + "_exhausted"
	request := CreateRequest{
		Job: contract.Job{
			ID:             jobID,
			Kind:           "provider.generate_image",
			OrganizationID: contract.OrganizationID("org_integration_" + jobID),
			ProjectID:      contract.ProjectID("project_integration_" + jobID),
			Status:         contract.JobQueued,
			Cancellable:    true,
			Version:        1,
			MaxAttempts:    2,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		Payload:        []byte(`{"prompt":"integration test"}`),
		IdempotencyKey: contract.IdempotencyKey("reclaim-" + string(jobID)),
		RequestHash:    strings.Repeat("b", 64),
	}
	store := MySQLStore{
		DB: db, ClaimOrganizationID: request.Job.OrganizationID, ClaimProjectID: request.Job.ProjectID,
	}
	if _, _, err := store.Enqueue(t.Context(), request); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM platform_jobs WHERE id IN (?, ?, ?)", jobID, cancelJobID, exhaustedJobID)
	})

	claim, found, err := store.Claim(t.Context(), "expired-worker", now.Add(time.Second))
	if err != nil || !found || claim.Job.ID != jobID {
		t.Fatalf("claim = (%+v, found=%t, err=%v)", claim, found, err)
	}

	recovery, err := store.ReclaimExpired(t.Context(), now.Add(2*time.Minute), time.Minute)
	if err != nil || recovery.Rescheduled != 1 {
		t.Fatalf("reclaim = (%+v, err=%v)", recovery, err)
	}

	var status string
	if err := db.QueryRowContext(t.Context(), "SELECT status FROM platform_jobs WHERE id = ?", jobID).Scan(&status); err != nil {
		t.Fatalf("read recovered job: %v", err)
	}
	if status != string(contract.JobQueued) {
		t.Fatalf("status after reclaim = %q, want %q", status, contract.JobQueued)
	}
	workerB := Worker{
		Store: store,
		Now:   func() time.Time { return now.Add(2*time.Minute + time.Second) },
		Handlers: map[string]Handler{
			request.Job.Kind: func(context.Context, Claim) (Result, error) {
				return Result{}, nil
			},
		},
	}
	processed, err := workerB.RunOnce(t.Context(), "replacement-worker")
	if err != nil || !processed {
		t.Fatalf("replacement worker = (processed=%t, err=%v)", processed, err)
	}
	var attempts int
	if err := db.QueryRowContext(t.Context(), "SELECT status, attempt_count FROM platform_jobs WHERE id = ?", jobID).Scan(&status, &attempts); err != nil {
		t.Fatalf("read replacement worker result: %v", err)
	}
	if status != string(contract.JobSucceeded) || attempts != 2 {
		t.Fatalf("replacement worker result = (status=%q, attempts=%d)", status, attempts)
	}

	cancelRequest := request
	cancelRequest.Job.ID = cancelJobID
	cancelRequest.IdempotencyKey = contract.IdempotencyKey("reclaim-cancel-" + string(cancelJobID))
	if _, _, err := store.Enqueue(t.Context(), cancelRequest); err != nil {
		t.Fatalf("enqueue cancellable recovery job: %v", err)
	}
	cancelClaim, found, err := store.Claim(t.Context(), "cancelled-worker", now.Add(3*time.Minute))
	if err != nil || !found || cancelClaim.Job.ID != cancelJobID {
		t.Fatalf("cancel claim = (%+v, found=%t, err=%v)", cancelClaim, found, err)
	}
	if _, err := store.RequestCancel(
		t.Context(), cancelClaim.Job.OrganizationID, cancelClaim.Job.ProjectID,
		cancelClaim.Job.ID, cancelClaim.Job.Version, now.Add(3*time.Minute+time.Second),
	); err != nil {
		t.Fatalf("request cancel: %v", err)
	}
	recovery, err = store.ReclaimExpired(t.Context(), now.Add(5*time.Minute), time.Minute)
	if err != nil || recovery.Cancelled != 1 || recovery.Rescheduled != 0 {
		t.Fatalf("cancel reclaim = (%+v, err=%v)", recovery, err)
	}
	if err := db.QueryRowContext(t.Context(), "SELECT status FROM platform_jobs WHERE id = ?", cancelJobID).Scan(&status); err != nil {
		t.Fatalf("read cancelled recovered job: %v", err)
	}
	if status != string(contract.JobCancelled) {
		t.Fatalf("status after cancelled reclaim = %q, want %q", status, contract.JobCancelled)
	}

	exhaustedRequest := request
	exhaustedRequest.Job.ID = exhaustedJobID
	exhaustedRequest.Job.MaxAttempts = 1
	exhaustedRequest.IdempotencyKey = contract.IdempotencyKey("reclaim-exhausted-" + string(exhaustedJobID))
	if _, _, err := store.Enqueue(t.Context(), exhaustedRequest); err != nil {
		t.Fatalf("enqueue exhausted recovery job: %v", err)
	}
	exhaustedClaim, found, err := store.Claim(t.Context(), "crashing-final-worker", now.Add(6*time.Minute))
	if err != nil || !found || exhaustedClaim.Job.ID != exhaustedJobID {
		t.Fatalf("exhausted claim = (%+v, found=%t, err=%v)", exhaustedClaim, found, err)
	}
	recovery, err = store.ReclaimExpired(t.Context(), now.Add(8*time.Minute), time.Minute)
	if err != nil || recovery.Failed != 1 || recovery.Rescheduled != 0 || recovery.Cancelled != 0 {
		t.Fatalf("exhausted reclaim = (%+v, err=%v)", recovery, err)
	}
	var errorCode string
	var retryable bool
	if err := db.QueryRowContext(t.Context(), "SELECT status, error_code, retryable FROM platform_jobs WHERE id = ?", exhaustedJobID).Scan(&status, &errorCode, &retryable); err != nil {
		t.Fatalf("read exhausted recovered job: %v", err)
	}
	if status != string(contract.JobFailed) || errorCode != "JOB_ATTEMPT_LIMIT_EXCEEDED" || retryable {
		t.Fatalf("exhausted recovery result = (status=%q, code=%q, retryable=%t)", status, errorCode, retryable)
	}
}
