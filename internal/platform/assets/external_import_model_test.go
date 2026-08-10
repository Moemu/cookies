package assets

import (
	"testing"
	"time"
)

func TestExternalImportStatusContract(t *testing.T) {
	now := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	value := ExternalImport{
		ID: "external_import_1", OrganizationID: "org_1", ProjectID: "project_1",
		SourceProvider: "miyun", SourceObjectID: "remote_material_1",
		IdempotencyKey: "import-1", RequestHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		RequestSnapshot: []byte(`{"source_provider":"miyun","source_object_id":"remote_material_1"}`),
		AttemptCount:    0, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	for _, status := range []ExternalImportStatus{
		ExternalImportQueued, ExternalImportRunning, ExternalImportSucceeded, ExternalImportFailed,
	} {
		value.Status = status
		value.CommittedAssetID, value.CommittedAssetVersion, value.LastErrorCode = "", 0, ""
		if status == ExternalImportSucceeded {
			value.CommittedAssetID, value.CommittedAssetVersion = "asset_1", 1
		}
		if status == ExternalImportFailed {
			value.LastErrorCode = "SOURCE_UNAVAILABLE"
		}
		if err := value.Validate(); err != nil {
			t.Fatalf("status %q: %v", status, err)
		}
	}

	value.Status = "imported"
	value.CommittedAssetID, value.CommittedAssetVersion = "", 0
	if err := value.Validate(); err == nil {
		t.Fatal("Insights material-import vocabulary must not leak into Assets ledger")
	}
}
