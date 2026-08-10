package insights

import (
	"errors"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestMiyunStatusContracts(t *testing.T) {
	now := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	connection := validMiyunConnection(now)
	for _, status := range []MiyunConnectionStatus{
		MiyunConnectionUnverified, MiyunConnectionReady, MiyunConnectionAuthRequired, MiyunConnectionDisabled,
	} {
		connection.Status = status
		if err := connection.Validate(); err != nil {
			t.Fatalf("connection status %q: %v", status, err)
		}
	}
	connection.Status = "active"
	if err := connection.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("connector status vocabulary must not leak into Miyun: %v", err)
	}

	profile := validMiyunProductProfile(now)
	for _, status := range []MiyunProfileStatus{MiyunProfileDraft, MiyunProfileConfirmed, MiyunProfileSuperseded} {
		profile.Status = status
		if status == MiyunProfileConfirmed {
			profile.ConfirmedBy, profile.ConfirmedAt = "operator_1", &now
		} else {
			profile.ConfirmedBy, profile.ConfirmedAt = "", nil
		}
		if err := profile.Validate(); err != nil {
			t.Fatalf("profile status %q: %v", status, err)
		}
	}

	job := validMiyunCrawlJob(now)
	for _, status := range []MiyunCrawlJobStatus{
		MiyunCrawlJobQueued, MiyunCrawlJobRunning, MiyunCrawlJobCoolingDown, MiyunCrawlJobAuthRequired,
		MiyunCrawlJobPartial, MiyunCrawlJobSucceeded, MiyunCrawlJobFailed, MiyunCrawlJobCancelled,
	} {
		job.Status = status
		if status == MiyunCrawlJobCoolingDown {
			cooldown := now.Add(5 * time.Minute)
			job.CooldownUntil = &cooldown
		} else {
			job.CooldownUntil = nil
		}
		if err := job.Validate(); err != nil {
			t.Fatalf("job status %q: %v", status, err)
		}
	}

	material := validMiyunMaterial(now)
	for _, selection := range []MiyunMaterialSelectionStatus{
		MiyunMaterialDiscovered, MiyunMaterialConfirmed, MiyunMaterialRejected,
	} {
		for _, importStatus := range []MiyunMaterialImportStatus{
			MiyunMaterialImportPending, MiyunMaterialImportDownloading, MiyunMaterialImportImported,
			MiyunMaterialImportDeduplicated, MiyunMaterialImportFailed, MiyunMaterialImportSkipped,
		} {
			material.SelectionStatus, material.ImportStatus = selection, importStatus
			if err := material.Validate(); err != nil {
				t.Fatalf("material statuses %q/%q: %v", selection, importStatus, err)
			}
		}
	}

	handoff := validMiyunHandoff(now)
	for _, status := range []MiyunHandoffStatus{
		MiyunHandoffExporting, MiyunHandoffExported, MiyunHandoffDelivered, MiyunHandoffReturned, MiyunHandoffFailed,
	} {
		handoff.Status = status
		if err := handoff.Validate(); err != nil {
			t.Fatalf("handoff status %q: %v", status, err)
		}
	}
}

func TestMiyunValidationProtectsFrozenDataContracts(t *testing.T) {
	now := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)

	profile := validMiyunProductProfile(now)
	profile.Keywords = []string{"cup", "cup"}
	if err := profile.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("duplicate query inputs must fail: %v", err)
	}

	job := validMiyunCrawlJob(now)
	job.QuerySnapshot = []byte("not-json")
	if err := job.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("unversioned or malformed query snapshots must fail: %v", err)
	}

	material := validMiyunMaterial(now)
	material.PlatformAssetID = "asset_1"
	if err := material.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("partial asset references must fail: %v", err)
	}

	snapshot := validMiyunMaterialSnapshot(now)
	snapshot.CumulativeImpressions = -1
	if err := snapshot.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("negative cumulative data must fail: %v", err)
	}
	snapshot = validMiyunMaterialSnapshot(now)
	snapshot.SanitizedRaw = []byte(`{"` + "author" + `ization":"synthetic"}`)
	if err := snapshot.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("session metadata must not enter sanitized snapshots: %v", err)
	}
}

func validMiyunConnection(now time.Time) MiyunConnection {
	return MiyunConnection{
		ID: "miyun_connection_1", OrganizationID: "org_1", ProjectID: "project_1",
		Status: MiyunConnectionUnverified, SessionCiphertext: []byte("encrypted-envelope"),
		SessionKeyVersion: "key-v1", Version: 1, CreatedBy: "operator_1", CreatedAt: now, UpdatedAt: now,
	}
}

func validMiyunProductProfile(now time.Time) MiyunProductProfile {
	return MiyunProductProfile{
		ID: "miyun_profile_1", OrganizationID: "org_1", ProjectID: "project_1", Status: MiyunProfileDraft,
		ConnectionID: "miyun_connection_1", ProductID: "product_1",
		ProductName: "Insulated cup", CategoryID: "category_1", CategoryName: "Drinkware",
		Keywords: []string{"insulated cup"}, MaterialContentTypes: []string{"product_demo"},
		WindowStart: now.AddDate(0, -1, 0), WindowEnd: now, ProjectContextVersion: 3,
		ProductAssetRefs:     []contract.AssetVersionRef{{AssetID: "asset_1", Version: 2}},
		KnowledgeDocumentIDs: []string{"document_1"}, RuleVersion: MiyunProductProfileRuleVersion,
		AnalysisMethod: "rules", InputSnapshot: []byte(`{"version":"1"}`),
		FieldSources:     []MiyunProfileFieldSource{{Field: "keywords", SourceKind: "deterministic_rules", SourceRefs: []string{"product:product_1"}, Confidence: "medium", ReviewState: "suggested", Explanation: "fixture"}},
		AnalysisWarnings: []string{"model_not_used:deterministic_rules"},
		InputHash:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Version:          1, CreatedBy: "operator_1", CreatedAt: now, UpdatedAt: now,
	}
}

func validMiyunCrawlJob(now time.Time) MiyunCrawlJob {
	return MiyunCrawlJob{
		ID: "miyun_job_1", OrganizationID: "org_1", ProjectID: "project_1",
		ConnectionID: "miyun_connection_1", ProductProfileID: "miyun_profile_1",
		Status: MiyunCrawlJobQueued, Operation: "product", QuerySchemaVersion: "youshu-query-v1",
		QuerySnapshot: []byte(`{"keyword":"insulated cup","page":1}`), Version: 1,
		CreatedBy: "operator_1", CreatedAt: now, UpdatedAt: now,
	}
}

func validMiyunMaterial(now time.Time) MiyunMaterial {
	return MiyunMaterial{
		ID: "miyun_material_1", OrganizationID: "org_1", ProjectID: "project_1",
		MiyunMaterialID: "remote_material_1", FirstSeenCrawlJobID: "miyun_job_1",
		ImportMethod:    MiyunImportCrawler,
		SelectionStatus: MiyunMaterialDiscovered, ImportStatus: MiyunMaterialImportPending,
		Version: 1, CreatedBy: "operator_1", CreatedAt: now, UpdatedAt: now,
	}
}

func validMiyunMaterialSnapshot(now time.Time) MiyunMaterialSnapshot {
	return MiyunMaterialSnapshot{
		ID: "miyun_snapshot_1", OrganizationID: "org_1", ProjectID: "project_1",
		MaterialID: "miyun_material_1", CrawlJobID: "miyun_job_1", ImportMethod: MiyunImportCrawler, SchemaVersion: "miyun-card-v1",
		CumulativeImpressionsRaw: "0",
		CapturedAt:               now, SanitizedRaw: []byte(`{"fixture_version":"1"}`), CreatedAt: now,
	}
}

func validMiyunHandoff(now time.Time) MiyunHandoff {
	return MiyunHandoff{
		ID: "miyun_handoff_1", OrganizationID: "org_1", ProjectID: "project_1",
		SourceMaterialID: "miyun_material_1", ProductProfileID: "miyun_profile_1",
		Status: MiyunHandoffExporting, ManifestVersion: "miyun-manifest-v1", ParameterVersion: "parameters-v1",
		ProductFilesSnapshot: []byte(`[]`), SourceSnapshot: []byte(`{"snapshot_id":"miyun_snapshot_1"}`),
		Version: 1, CreatedBy: "operator_1", CreatedAt: now, UpdatedAt: now,
	}
}
