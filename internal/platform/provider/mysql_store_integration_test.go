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
