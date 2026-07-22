package provider

import (
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestMySQLStoreUsesProviderIdempotencyScope(t *testing.T) {
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
	store := MySQLStore{DB: db}
	record := testJobRecord(now, "provider_job_store_1")
	created, duplicate, err := store.Create(t.Context(), record)
	if err != nil || duplicate || created.Job.ID != record.Job.ID {
		t.Fatalf("Create() = (%+v, duplicate=%v, err=%v)", created, duplicate, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(t.Context(), "DELETE FROM provider_job_outputs WHERE provider_job_id = ?", record.Job.ID)
		_, _ = db.ExecContext(t.Context(), "DELETE FROM provider_jobs WHERE id = ?", record.Job.ID)
	})

	created, duplicate, err = store.Create(t.Context(), record)
	if err != nil || !duplicate || created.Job.ID != record.Job.ID {
		t.Fatalf("duplicate Create() = (%+v, duplicate=%v, err=%v)", created, duplicate, err)
	}

	conflicting := record
	conflicting.RequestHash = strings.Repeat("b", 64)
	_, _, err = store.Create(t.Context(), conflicting)
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflicting Create() err=%v, want ErrIdempotencyConflict", err)
	}

	created.Job.ExecutionStatus = contract.JobRunning
	created.Job.ProviderStatus = contract.ProviderJobOutputsReady
	created.Job.Progress = 70
	created.Job.UpdatedAt = now.Add(time.Second)
	created.ProviderCode = "fake"
	created.ModelVersion = "fake-image-v1"
	created.ExternalTaskID = "task_1"
	created.Outputs = []OutputRecord{{
		Ref: contract.ProviderOutputRef{
			ProviderCode: "fake", ProviderJobID: created.Job.ID, OutputID: "output_1",
			RetrievalExpiresAt: now.Add(time.Hour), DeclaredMIMEType: "image/png", DeclaredSizeBytes: 1024,
		},
		Status: OutputReady,
	}}
	updated, err := store.Update(t.Context(), created)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Job.Version != 2 {
		t.Fatalf("Update() version = %d, want 2", updated.Job.Version)
	}

	loaded, err := store.Get(t.Context(), record.Job.OrganizationID, record.Job.ProjectID, record.Job.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loaded.ProviderCode != "fake" || loaded.ModelVersion != "fake-image-v1" || loaded.ExternalTaskID != "task_1" {
		t.Fatalf("Get() lost provider metadata: %+v", loaded)
	}
	if len(loaded.Outputs) != 1 || loaded.Outputs[0].Ref.OutputID != "output_1" || loaded.Outputs[0].Status != OutputReady {
		t.Fatalf("Get() outputs = %+v, want persisted ready output", loaded.Outputs)
	}
}

func testJobRecord(now time.Time, id string) JobRecord {
	return JobRecord{
		Job: contract.ProviderJob{
			ID:               id,
			Kind:             imageJobKind,
			OrganizationID:   "org_store",
			ProjectID:        "project_store",
			ExecutionStatus:  contract.JobQueued,
			ProviderStatus:   contract.ProviderJobSubmitted,
			ProjectAssetRefs: []contract.ProjectAssetRef{},
			MaxAttempts:      3,
			Version:          1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		Principal:             contract.Principal{Kind: contract.PrincipalUser, ID: "user_store"},
		Operation:             imageOperation,
		IdempotencyKey:        "provider-store-create",
		RequestHash:           strings.Repeat("a", 64),
		ProjectContextVersion: 1,
		ModelAlias:            "cookies.image.standard",
		Input:                 ImageGenerationInput{Prompt: "test", Width: 512, Height: 512},
	}
}
