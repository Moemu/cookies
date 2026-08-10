package insights_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/platform/project"
	"github.com/shikanon/cookies/internal/systems/insights"
)

func TestMiyunFoundationAgainstMySQL(t *testing.T) {
	dsn := os.Getenv("COOKIES_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("COOKIES_TEST_MYSQL_DSN is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	organizationID := contract.OrganizationID("org_miyun_it_" + suffix)
	projectID := contract.ProjectID("project_miyun_it_" + suffix)
	otherProjectID := contract.ProjectID("project_miyun_other_it_" + suffix)
	userID := "user_miyun_it_" + suffix
	actor := contract.ActorContext{
		OrganizationID: organizationID,
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: userID},
		Scopes:         []contract.Scope{"project.read", "project.write"},
	}
	t.Cleanup(func() {
		cleanupMiyunIntegration(t, db, organizationID)
		cleanupInsightsIntegration(t, db, organizationID, userID)
	})
	if err := (identity.MySQLStore{DB: db}).EnsureLocalActor(ctx, actor); err != nil {
		t.Fatal(err)
	}
	projects := project.MySQLStore{DB: db}
	if err := projects.EnsureLocalProject(ctx, actor, projectID); err != nil {
		t.Fatal(err)
	}
	if err := projects.EnsureLocalProject(ctx, actor, otherProjectID); err != nil {
		t.Fatal(err)
	}

	repository := insights.MySQLRepository{DB: db}
	now := time.Now().UTC().Truncate(time.Microsecond)
	connection := insights.MiyunConnection{
		ID: "miyun_connection_it_" + suffix, OrganizationID: organizationID, ProjectID: projectID,
		Status: insights.MiyunConnectionUnverified, SessionCiphertext: []byte("encrypted-test-envelope"),
		SessionKeyVersion: "key-v1", Version: 1, CreatedBy: userID, CreatedAt: now, UpdatedAt: now,
	}
	connection, err = repository.CreateMiyunConnection(ctx, connection)
	if err != nil {
		t.Fatal(err)
	}
	connection.Status = insights.MiyunConnectionReady
	connection.LastVerifiedAt = &now
	connection.UpdatedAt = now.Add(time.Second)
	connection, err = repository.UpdateMiyunConnection(ctx, connection, 1)
	if err != nil || connection.Version != 2 {
		t.Fatalf("connection=%#v err=%v", connection, err)
	}
	if _, err := repository.UpdateMiyunConnection(ctx, connection, 1); !errors.Is(err, insights.ErrVersionConflict) {
		t.Fatalf("stale connection update should conflict: %v", err)
	}
	if _, err := repository.GetMiyunConnection(ctx, organizationID, otherProjectID, connection.ID); !errors.Is(err, insights.ErrNotFound) {
		t.Fatalf("cross-project connection read should be hidden: %v", err)
	}

	profile := insights.MiyunProductProfile{
		ID: "miyun_profile_it_" + suffix, OrganizationID: organizationID, ProjectID: projectID,
		ConnectionID: connection.ID, Status: insights.MiyunProfileDraft, ProductID: "product_test", ProductName: "Test product",
		CategoryID: "category-test", CategoryName: "Test category", Keywords: []string{"test keyword"},
		MaterialContentTypes: []string{"product_demo"}, WindowStart: now.AddDate(0, -1, 0), WindowEnd: now,
		ProjectContextVersion: 1, ProductAssetRefs: []contract.AssetVersionRef{}, KnowledgeDocumentIDs: []string{},
		RuleVersion: insights.MiyunProductProfileRuleVersion, AnalysisMethod: "rules",
		InputSnapshot:    []byte(`{"version":"1"}`),
		FieldSources:     []insights.MiyunProfileFieldSource{{Field: "keywords", SourceKind: "deterministic_rules", SourceRefs: []string{"product:product_1"}, Confidence: "medium", ReviewState: "suggested", Explanation: "fixture"}},
		AnalysisWarnings: []string{"model_not_used:deterministic_rules"},
		InputHash:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Version:          1, CreatedBy: userID, CreatedAt: now, UpdatedAt: now,
	}
	profile, err = repository.CreateMiyunProductProfileDraft(ctx, profile)
	if err != nil {
		t.Fatal(err)
	}
	replacement := profile
	replacement.ID = "miyun_profile_replacement_it_" + suffix
	replacement.CreatedAt, replacement.UpdatedAt = now.Add(time.Second), now.Add(time.Second)
	replacement, err = repository.CreateMiyunProductProfileDraft(ctx, replacement)
	if err != nil {
		t.Fatal(err)
	}
	if superseded, err := repository.GetMiyunProductProfile(ctx, organizationID, projectID, profile.ID); err != nil || superseded.Status != insights.MiyunProfileSuperseded || superseded.Version != 2 {
		t.Fatalf("superseded profile=%#v err=%v", superseded, err)
	}
	confirmedAt := now.Add(2 * time.Second)
	replacement.Status, replacement.ConfirmedBy, replacement.ConfirmedAt = insights.MiyunProfileConfirmed, userID, &confirmedAt
	replacement.UpdatedAt = confirmedAt
	for index := range replacement.FieldSources {
		replacement.FieldSources[index].ReviewState = "human_confirmed"
	}
	profile, err = repository.ConfirmMiyunProductProfile(ctx, replacement, 1)
	if err != nil || profile.Version != 2 {
		t.Fatalf("confirmed profile=%#v err=%v", profile, err)
	}
	if reloaded, err := repository.GetMiyunProductProfile(ctx, organizationID, projectID, profile.ID); err != nil || reloaded.Keywords[0] != "test keyword" || reloaded.Status != insights.MiyunProfileConfirmed {
		t.Fatalf("profile=%#v err=%v", reloaded, err)
	}

	job := insights.MiyunCrawlJob{
		ID: "miyun_job_it_" + suffix, OrganizationID: organizationID, ProjectID: projectID,
		ConnectionID: connection.ID, ProductProfileID: profile.ID, Status: insights.MiyunCrawlJobQueued,
		Operation: "product", QuerySchemaVersion: "youshu-query-v1",
		QuerySnapshot: []byte(`{"keyword":"test keyword","page":1}`), Version: 1,
		CreatedBy: userID, CreatedAt: now, UpdatedAt: now,
	}
	job, err = repository.CreateMiyunCrawlJob(ctx, job)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetMiyunCrawlJob(ctx, organizationID, projectID, job.ID); err != nil {
		t.Fatal(err)
	}

	material := insights.MiyunMaterial{
		ID: "miyun_material_it_" + suffix, OrganizationID: organizationID, ProjectID: projectID,
		MiyunMaterialID: "remote-material-test", FirstSeenCrawlJobID: job.ID,
		ImportMethod:    insights.MiyunImportCrawler,
		SelectionStatus: insights.MiyunMaterialDiscovered, ImportStatus: insights.MiyunMaterialImportPending,
		Version: 1, CreatedBy: userID, CreatedAt: now, UpdatedAt: now,
	}
	material, err = repository.CreateMiyunMaterial(ctx, material)
	if err != nil {
		t.Fatal(err)
	}
	duplicate := material
	duplicate.ID = "miyun_material_duplicate_it_" + suffix
	if _, err := repository.CreateMiyunMaterial(ctx, duplicate); !errors.Is(err, insights.ErrInvalidState) {
		t.Fatalf("duplicate remote identity should fail: %v", err)
	}
	if _, err := repository.GetMiyunMaterial(ctx, organizationID, otherProjectID, material.ID); !errors.Is(err, insights.ErrNotFound) {
		t.Fatalf("cross-project material read should be hidden: %v", err)
	}

	for index := range 2 {
		capturedAt := now.Add(time.Duration(index) * time.Hour)
		_, err := repository.AppendMiyunMaterialSnapshot(ctx, insights.MiyunMaterialSnapshot{
			ID:             "miyun_snapshot_" + strconv.Itoa(index) + "_it_" + suffix,
			OrganizationID: organizationID, ProjectID: projectID, MaterialID: material.ID, CrawlJobID: job.ID, ImportMethod: insights.MiyunImportCrawler,
			SchemaVersion: "miyun-card-v1", CapturedAt: capturedAt,
			CumulativeImpressions: int64(100 + index), CumulativeImpressionsRaw: strconv.Itoa(100 + index), RelatedAds: int64(5 + index),
			SanitizedRaw: []byte(`{"schema_version":"miyun-card-v1"}`), CreatedAt: capturedAt,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	snapshots, err := repository.ListMiyunMaterialSnapshots(ctx, organizationID, projectID, material.ID)
	if err != nil || len(snapshots) != 2 || snapshots[0].CumulativeImpressions == snapshots[1].CumulativeImpressions {
		t.Fatalf("append-only snapshots=%#v err=%v", snapshots, err)
	}

	handoff := insights.MiyunHandoff{
		ID: "miyun_handoff_it_" + suffix, OrganizationID: organizationID, ProjectID: projectID,
		SourceMaterialID: material.ID, ProductProfileID: profile.ID, Status: insights.MiyunHandoffExporting,
		ManifestVersion: "miyun-manifest-v1", ParameterVersion: "parameters-v1",
		ProductFilesSnapshot: []byte(`[]`), SourceSnapshot: []byte(`{"material_id":"remote-material-test"}`),
		Version: 1, CreatedBy: userID, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := repository.CreateMiyunHandoff(ctx, handoff); err != nil {
		t.Fatal(err)
	}
	if reloaded, err := repository.GetMiyunHandoff(ctx, organizationID, projectID, handoff.ID); err != nil || reloaded.ManifestVersion != handoff.ManifestVersion {
		t.Fatalf("handoff=%#v err=%v", reloaded, err)
	}
}

func cleanupMiyunIntegration(t *testing.T, db *sql.DB, organizationID contract.OrganizationID) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for _, statement := range []string{
		"DELETE FROM insight_miyun_handoffs WHERE organization_id=?",
		"DELETE FROM insight_miyun_material_snapshots WHERE organization_id=?",
		"DELETE FROM insight_miyun_materials WHERE organization_id=?",
		"DELETE FROM insight_miyun_crawl_jobs WHERE organization_id=?",
		"DELETE FROM insight_miyun_product_profiles WHERE organization_id=?",
		"DELETE FROM insight_miyun_connections WHERE organization_id=?",
		"DELETE FROM asset_external_imports WHERE organization_id=?",
		"DELETE FROM platform_project_runtimes WHERE organization_id=?",
	} {
		if _, err := db.ExecContext(ctx, statement, organizationID); err != nil {
			t.Errorf("cleanup %q: %v", statement, err)
		}
	}
}
