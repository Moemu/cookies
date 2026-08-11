package provider

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestMySQLGatewayConfigStoreResolvesVersionedEncryptedRoute(t *testing.T) {
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
	suffix := strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000"), ".", "")
	connectionID, connectionRevisionID := "connection_"+suffix, "connection_revision_"+suffix
	credentialID, routeID, routeRevisionID := "credential_"+suffix, "route_"+suffix, "route_revision_"+suffix
	modelAlias := "cookies.image.integration." + suffix
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x35}, 32))
	cipher, err := NewAESGCMCredentialCipher(key, "integration-v1")
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, nonce, keyVersion, err := cipher.Encrypt([]byte("adapter-integration-token"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `INSERT INTO provider_connections
		(id, connection_code, connection_type, current_revision_id, status) VALUES (?, ?, 'adapter_gateway', NULL, 'enabled')`,
		connectionID, connectionID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("UPDATE provider_model_routes SET current_revision_id = NULL WHERE id = ?", routeID)
		_, _ = db.Exec("DELETE FROM provider_model_route_revisions WHERE id = ?", routeRevisionID)
		_, _ = db.Exec("DELETE FROM provider_model_routes WHERE id = ?", routeID)
		_, _ = db.Exec("DELETE FROM provider_credentials WHERE id = ?", credentialID)
		_, _ = db.Exec("UPDATE provider_connections SET current_revision_id = NULL WHERE id = ?", connectionID)
		_, _ = db.Exec("DELETE FROM provider_connection_revisions WHERE id = ?", connectionRevisionID)
		_, _ = db.Exec("DELETE FROM provider_connections WHERE id = ?", connectionID)
	})
	if _, err := db.ExecContext(t.Context(), `INSERT INTO provider_connection_revisions
		(id, connection_id, revision_number, base_url, timeout_seconds, max_response_bytes)
		VALUES (?, ?, 1, 'https://adapter.example/v1', 210, 41943040)`, connectionRevisionID, connectionID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), "UPDATE provider_connections SET current_revision_id = ? WHERE id = ?", connectionRevisionID, connectionID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `INSERT INTO provider_credentials
		(id, connection_id, credential_version, ciphertext, nonce, key_version, status, active_from)
		VALUES (?, ?, 1, ?, ?, ?, 'active', UTC_TIMESTAMP(6))`,
		credentialID, connectionID, ciphertext, nonce, keyVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `INSERT INTO provider_model_routes
		(id, organization_id, capability, model_alias, current_revision_id, status)
		VALUES (?, NULL, 'image.generate', ?, NULL, 'enabled')`, routeID, modelAlias); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `INSERT INTO provider_model_route_revisions
		(id, route_id, revision_number, connection_id, connection_revision_id, upstream_model)
		VALUES (?, ?, 1, ?, ?, 'gpt-image-2')`,
		routeRevisionID, routeID, connectionID, connectionRevisionID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), "UPDATE provider_model_routes SET current_revision_id = ? WHERE id = ?", routeRevisionID, routeID); err != nil {
		t.Fatal(err)
	}

	store := MySQLGatewayConfigStore{DB: db, Cipher: cipher}
	snapshot, err := store.ResolveImageRoute(t.Context(), "org_integration", modelAlias)
	if err != nil || snapshot.ConnectionRevisionID != connectionRevisionID || snapshot.CredentialID != credentialID {
		t.Fatalf("ResolveImageRoute() = %+v, %v", snapshot, err)
	}
	token, err := store.ResolveGatewayCredential(t.Context(), snapshot.CredentialID, snapshot.CredentialVersion)
	if err != nil || token != "adapter-integration-token" {
		t.Fatalf("ResolveGatewayCredential() = %q, %v", token, err)
	}
}

func TestMySQLGatewayConfigStoreResolvesLASDocumentRoute(t *testing.T) {
	dsn := os.Getenv("COOKIES_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("COOKIES_TEST_MYSQL_DSN is not configured")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	suffix := strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000"), ".", "")
	connectionID, connectionRevisionID := "connection_las_"+suffix, "connection_las_revision_"+suffix
	credentialID, routeID, routeRevisionID := "credential_las_"+suffix, "route_las_"+suffix, "route_las_revision_"+suffix
	modelAlias := "cookies.document.vision.integration." + suffix
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x45}, 32))
	cipher, err := NewAESGCMCredentialCipher(key, "integration-v1")
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, nonce, keyVersion, err := cipher.Encrypt([]byte("las-integration-token"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `INSERT INTO provider_connections
		(id, connection_code, connection_type, current_revision_id, status)
		VALUES (?, ?, 'las_operator', NULL, 'enabled')`, connectionID, connectionID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("UPDATE provider_model_routes SET current_revision_id = NULL WHERE id = ?", routeID)
		_, _ = db.Exec("DELETE FROM provider_model_route_revisions WHERE id = ?", routeRevisionID)
		_, _ = db.Exec("DELETE FROM provider_model_routes WHERE id = ?", routeID)
		_, _ = db.Exec("DELETE FROM provider_credentials WHERE id = ?", credentialID)
		_, _ = db.Exec("UPDATE provider_connections SET current_revision_id = NULL WHERE id = ?", connectionID)
		_, _ = db.Exec("DELETE FROM provider_connection_revisions WHERE id = ?", connectionRevisionID)
		_, _ = db.Exec("DELETE FROM provider_connections WHERE id = ?", connectionID)
	})
	if _, err := db.ExecContext(t.Context(), `INSERT INTO provider_connection_revisions
		(id, connection_id, revision_number, base_url, timeout_seconds, max_response_bytes)
		VALUES (?, ?, 1, 'https://operator.las.cn-beijing.volces.com/api/v1', 900, 8388608)`,
		connectionRevisionID, connectionID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `UPDATE provider_connections SET current_revision_id = ? WHERE id = ?`, connectionRevisionID, connectionID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `INSERT INTO provider_credentials
		(id, connection_id, credential_version, ciphertext, nonce, key_version, status, active_from)
		VALUES (?, ?, 1, ?, ?, ?, 'active', UTC_TIMESTAMP(6))`,
		credentialID, connectionID, ciphertext, nonce, keyVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `INSERT INTO provider_model_routes
		(id, organization_id, capability, model_alias, current_revision_id, status)
		VALUES (?, NULL, 'document.vision.parse', ?, NULL, 'enabled')`, routeID, modelAlias); err != nil {
		t.Fatal(err)
	}
	constraints := json.RawMessage(`{
		"endpoint":"/submit","poll_endpoint":"/poll","operator_version":"v1",
		"parse_mode":"detail","full_result":true,"aspect_ratio_threshold":0.334,"poll_interval_ms":2000
	}`)
	if _, err := db.ExecContext(t.Context(), `INSERT INTO provider_model_route_revisions
		(id, route_id, revision_number, connection_id, connection_revision_id, upstream_model, constraints_json)
		VALUES (?, ?, 1, ?, ?, 'las_pdf_parse_doubao', ?)`,
		routeRevisionID, routeID, connectionID, connectionRevisionID, constraints); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `UPDATE provider_model_routes SET current_revision_id = ? WHERE id = ?`, routeRevisionID, routeID); err != nil {
		t.Fatal(err)
	}
	store := MySQLGatewayConfigStore{DB: db, Cipher: cipher}
	snapshot, err := store.ResolveDocumentVisionRoute(t.Context(), "org_integration", modelAlias)
	if err != nil || snapshot.ConnectionType != "las_operator" || snapshot.DocumentParseMode != "detail" ||
		snapshot.DocumentSubmitPath != "/submit" || snapshot.DocumentPollPath != "/poll" {
		t.Fatalf("ResolveDocumentVisionRoute() = %+v, %v", snapshot, err)
	}
	token, err := store.ResolveGatewayCredential(t.Context(), snapshot.CredentialID, snapshot.CredentialVersion)
	if err != nil || token != "las-integration-token" {
		t.Fatalf("ResolveGatewayCredential() = %q, %v", token, err)
	}
}

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
	testID := strings.ReplaceAll(now.Format("20060102150405.000000"), ".", "")
	record := testJobRecord(now, "provider_job_store_"+testID)
	record.IdempotencyKey = contract.IdempotencyKey("provider-store-" + testID)
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
	loaded.Route = testGatewayRoute()
	loaded.SubmissionState = SubmissionInFlight
	loaded.AdapterRequestID = "adapter-request-integration"
	deadline := now.Add(210 * time.Second)
	loaded.ExecutionDeadlineAt = &deadline
	loaded.SubmittedAt = &now
	loaded.Job.UpdatedAt = now.Add(2 * time.Second)
	if _, err := store.Update(t.Context(), loaded); err != nil {
		t.Fatalf("Update() gateway snapshot error = %v", err)
	}
	loaded, err = store.Get(t.Context(), record.Job.OrganizationID, record.Job.ProjectID, record.Job.ID)
	if err != nil || loaded.Route == nil || loaded.Route.UpstreamModel != "gpt-image-2" ||
		loaded.SubmissionState != SubmissionInFlight || loaded.ExecutionDeadlineAt == nil {
		t.Fatalf("Get() gateway snapshot = %+v, err=%v", loaded, err)
	}
	objectHandles := ObjectOutputHandleStore{DB: db, Blobs: assets.NewMemoryBlobStore(), Bucket: "provider-output-integration"}
	ref, err := NewOutputRef(adapterGatewayProviderCode, loaded.Job.ID, "object_output_1", "image/png", fakeImagePNG, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	projectRef := contract.ProjectRef{OrganizationID: loaded.Job.OrganizationID, ProjectID: loaded.Job.ProjectID, ProjectContextVersion: loaded.ProjectContextVersion}
	if err := objectHandles.Put(t.Context(), projectRef, ref, fakeImagePNG); err != nil {
		t.Fatalf("ObjectOutputHandleStore.Put() error = %v", err)
	}
	stream, metadata, err := objectHandles.Open(t.Context(), projectRef, ref)
	if err != nil {
		t.Fatalf("ObjectOutputHandleStore.Open() error = %v", err)
	}
	got, readErr := io.ReadAll(stream)
	_ = stream.Close()
	if readErr != nil || !bytes.Equal(got, fakeImagePNG) || metadata.SHA256 != *ref.DeclaredSHA256 {
		t.Fatalf("ObjectOutputHandleStore.Open() bytes=%d metadata=%+v err=%v", len(got), metadata, readErr)
	}
	if err := objectHandles.Delete(t.Context(), loaded.Job.OrganizationID, loaded.Job.ProjectID, loaded.Job.ID, ref.OutputID); err != nil {
		t.Fatalf("ObjectOutputHandleStore.Delete() error = %v", err)
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
