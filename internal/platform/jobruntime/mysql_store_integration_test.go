package jobruntime

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

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

	now := time.Now().UTC().Truncate(time.Microsecond)
	jobID := "job_integration_" + strings.ReplaceAll(now.Format("20060102150405.000000"), ".", "")
	request := CreateRequest{
		Job: contract.Job{
			ID:             jobID,
			Kind:           "provider.generate_image",
			OrganizationID: "org_integration",
			ProjectID:      "project_integration",
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
	store := MySQLStore{DB: db}
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
