package assets_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/platform/project"
)

func TestMySQLAssetGate2(t *testing.T) {
	dsn := os.Getenv("COOKIES_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("COOKIES_TEST_MYSQL_DSN is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping MySQL: %v", err)
	}
	var assetsTable string
	if err := db.QueryRowContext(ctx, "SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='assets'").Scan(&assetsTable); err != nil {
		t.Fatalf("assets migrations must be applied before integration test: %v", err)
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	organizationID := contract.OrganizationID("org_it_" + suffix)
	projectID := contract.ProjectID("project_it_" + suffix)
	userID := "user_it_" + suffix
	actor := contract.ActorContext{OrganizationID: organizationID, Principal: contract.Principal{Kind: contract.PrincipalUser, ID: userID}, Scopes: []contract.Scope{"project.read", "project.write", "assets.read", "assets.write"}}
	defer cleanupOrganization(t, db, organizationID, userID)
	identityStore := identity.MySQLStore{DB: db}
	if err := identityStore.EnsureLocalActor(ctx, actor); err != nil {
		t.Fatalf("seed actor: %v", err)
	}
	projectStore := project.MySQLStore{DB: db}
	if err := projectStore.EnsureLocalProject(ctx, actor, projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	projectService := &project.Service{Store: projectStore, Authorizer: projectStore}
	repository := assets.MySQLRepository{DB: db}
	blobs := assets.NewMemoryBlobStore()
	now := time.Now().UTC()
	uploads := assets.UploadService{Repository: repository, Projects: projectService, Blobs: blobs, Scanner: assets.NoopScanner{}, QuarantineBucket: "integration-quarantine", AssetsBucket: "integration-assets"}
	rc := contract.RequestContext{RequestID: "req_it_" + suffix, TraceID: "trace_it_" + suffix, Actor: actor}
	data := integrationPNG(t)
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	created, err := uploads.Create(ctx, rc, projectID, contract.IdempotencyKey("upload_"+suffix), assets.CreateUploadRequest{Filename: "integration.png", DeclaredMIMEType: "image/png", DeclaredSizeBytes: int64(len(data)), DeclaredSHA256: &digest})
	if err != nil {
		t.Fatalf("create upload: %v", err)
	}
	if err := uploads.PutContent(ctx, actor, projectID, created.Session.ID, bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("put upload: %v", err)
	}
	completed, err := uploads.Finalize(ctx, rc, projectID, created.Session.ID)
	if err != nil {
		t.Fatalf("finalize upload: %v", err)
	}
	if completed.ProjectAssetRef == nil {
		t.Fatal("upload did not create ProjectAssetRef")
	}

	request := assets.GeneratedAssetIntakeRequest{ProviderJobID: "job_" + suffix, Output: contract.ProviderOutputRef{ProviderCode: "integration", ProviderJobID: "job_" + suffix, OutputID: "output_" + suffix, RetrievalExpiresAt: now.Add(time.Hour), DeclaredMIMEType: "image/png", DeclaredSizeBytes: int64(len(data)), DeclaredSHA256: &digest}, Provenance: assets.GenerationProvenance{Capability: "image.generate", ProviderCode: "integration", ModelAlias: "integration.image", ModelVersion: "v1", SourceAssetRefs: []contract.AssetVersionRef{}, ProjectContextVersion: 1, GeneratedAt: now}}
	intakes := assets.GeneratedIntakeService{Repository: repository, Projects: projectService}
	intake, err := intakes.Create(ctx, rc, projectID, contract.IdempotencyKey("intake_"+suffix), request)
	if err != nil {
		t.Fatalf("create intake: %v", err)
	}
	worker := assets.GeneratedIntakeWorker{Repository: repository, Projects: projectService, Fetcher: integrationFetcher{data: data, metadata: contract.OutputMetadata{MIMEType: "image/png", SizeBytes: int64(len(data)), SHA256: digest}}, Upload: uploads, Actor: actor}
	processed, err := worker.ProcessOnce(ctx, "worker_"+suffix)
	if err != nil || !processed {
		t.Fatalf("process intake: processed=%v err=%v", processed, err)
	}
	stored, err := intakes.Get(ctx, actor, projectID, intake.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != assets.GeneratedIntakeSucceeded || stored.ProjectAssetRef == nil {
		t.Fatalf("unexpected intake: %#v", stored)
	}
	items, err := uploads.List(ctx, actor, projectID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("asset count=%d, want 2", len(items))
	}
	if err := uploads.Remove(ctx, actor, projectID, completed.ProjectAssetRef.AssetVersion); err != nil {
		t.Fatalf("remove uploaded asset: %v", err)
	}
	items, err = uploads.List(ctx, actor, projectID, 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("asset count after remove=%d err=%v, want 1", len(items), err)
	}
	if _, err := uploads.Preview(ctx, actor, projectID, completed.ProjectAssetRef.AssetVersion); !errors.Is(err, assets.ErrNotFound) {
		t.Fatalf("preview removed asset error=%v, want ErrNotFound", err)
	}
	if err := uploads.Remove(ctx, actor, projectID, completed.ProjectAssetRef.AssetVersion); err != nil {
		t.Fatalf("idempotent remove: %v", err)
	}
	other := actor
	other.OrganizationID = contract.OrganizationID("org_other_" + suffix)
	if _, err := uploads.List(ctx, other, projectID, 10); err == nil {
		t.Fatal("cross-organization project access was allowed")
	}

	video := append([]byte{0, 0, 0, 16, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}, bytes.Repeat([]byte{0}, 16)...)
	externalUpload := uploads
	externalUpload.VideoProbe = integrationVideoProbe{}
	idSequence := 0
	externalImports := assets.ExternalImportService{
		Repository: repository, Projects: projectService, Upload: externalUpload, QuarantineBucket: "integration-quarantine",
		Now: func() time.Time { return now },
		NewID: func(prefix string) (string, error) {
			idSequence++
			return prefix + "_" + suffix + "_" + strconv.Itoa(idSequence), nil
		},
	}
	firstExternal, err := externalImports.Import(ctx, rc, projectID, contract.IdempotencyKey("external_1_"+suffix), assets.ExternalMediaImportRequest{
		SourceProvider: "miyun", SourceObjectID: "remote_1_" + suffix, MIMEType: "video/mp4", SizeBytes: int64(len(video)),
	}, func(context.Context) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(video)), nil })
	if err != nil {
		t.Fatalf("first external import: %v", err)
	}
	secondExternal, err := externalImports.Import(ctx, rc, projectID, contract.IdempotencyKey("external_2_"+suffix), assets.ExternalMediaImportRequest{
		SourceProvider: "miyun", SourceObjectID: "remote_2_" + suffix, MIMEType: "video/mp4", SizeBytes: int64(len(video)),
	}, func(context.Context) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(video)), nil })
	if err != nil {
		t.Fatalf("deduplicated external import: %v", err)
	}
	if firstExternal.AssetVersion != secondExternal.AssetVersion {
		t.Fatalf("same content created different assets: first=%#v second=%#v", firstExternal, secondExternal)
	}
	projectAsset, err := repository.GetProjectAsset(ctx, organizationID, projectID, firstExternal.AssetVersion)
	if err != nil || projectAsset.Version.SourceType != contract.AssetSourceImported {
		t.Fatalf("external asset=%#v err=%v", projectAsset, err)
	}
	ledger, err := repository.GetExternalImportBySource(ctx, organizationID, projectID, "miyun", "remote_2_"+suffix)
	if err != nil || ledger.Status != assets.ExternalImportSucceeded || ledger.CommittedAssetID != firstExternal.AssetVersion.AssetID {
		t.Fatalf("external ledger=%#v err=%v", ledger, err)
	}
	retryRequest := assets.ExternalMediaImportRequest{SourceProvider: "miyun", SourceObjectID: "remote_retry_" + suffix, MIMEType: "video/mp4", SizeBytes: int64(len(video))}
	if _, err := externalImports.Import(ctx, rc, projectID, contract.IdempotencyKey("external_retry_"+suffix), retryRequest, func(context.Context) (io.ReadCloser, error) {
		return nil, errors.New("temporary source failure")
	}); err == nil {
		t.Fatal("temporary external source failure was not recorded")
	}
	retriedExternal, err := externalImports.Import(ctx, rc, projectID, contract.IdempotencyKey("external_retry_"+suffix), retryRequest,
		func(context.Context) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(video)), nil })
	if err != nil || retriedExternal.AssetVersion != firstExternal.AssetVersion {
		t.Fatalf("external retry=%#v err=%v", retriedExternal, err)
	}
	retryLedger, err := repository.GetExternalImportBySource(ctx, organizationID, projectID, "miyun", retryRequest.SourceObjectID)
	if err != nil || retryLedger.Status != assets.ExternalImportSucceeded || retryLedger.AttemptCount != 2 {
		t.Fatalf("external retry ledger=%#v err=%v", retryLedger, err)
	}
}

type integrationFetcher struct {
	data     []byte
	metadata contract.OutputMetadata
}

type integrationVideoProbe struct{}

func (integrationVideoProbe) Probe(context.Context, []byte) (assets.VideoMetadata, error) {
	return assets.VideoMetadata{DurationMS: 1000, WidthPixels: 16, HeightPixels: 16, FrameRate: "25/1", VideoCodec: "h264"}, nil
}

func (f integrationFetcher) Open(context.Context, contract.ProjectRef, contract.ProviderOutputRef) (io.ReadCloser, contract.OutputMetadata, error) {
	return io.NopCloser(bytes.NewReader(f.data)), f.metadata, nil
}

func integrationPNG(t *testing.T) []byte {
	t.Helper()
	var result bytes.Buffer
	value := image.NewRGBA(image.Rect(0, 0, 2, 2))
	value.Set(0, 0, color.RGBA{R: 20, G: 80, B: 160, A: 255})
	if err := png.Encode(&result, value); err != nil {
		t.Fatal(err)
	}
	return result.Bytes()
}

func cleanupOrganization(t *testing.T, db *sql.DB, organizationID contract.OrganizationID, userID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	statements := []string{
		"DELETE FROM asset_features WHERE organization_id=?", "DELETE FROM assets_outbox WHERE organization_id=?", "DELETE FROM asset_external_imports WHERE organization_id=?", "DELETE FROM project_assets WHERE organization_id=?", "DELETE FROM asset_versions WHERE organization_id=?", "DELETE FROM assets WHERE organization_id=?", "DELETE FROM asset_blobs WHERE organization_id=?", "DELETE FROM generated_intakes WHERE organization_id=?", "DELETE FROM upload_sessions WHERE organization_id=?",
		// 建项目时会顺手写一行运行时，它外键指向 projects。不先删它，后面每一条
		// 清理都连环失败，测试跑一次就往开发库里留一份脏数据。
		"DELETE FROM platform_project_runtimes WHERE organization_id=?", "DELETE FROM project_context_versions WHERE organization_id=?", "DELETE FROM project_products WHERE organization_id=?", "DELETE FROM project_memberships WHERE organization_id=?", "DELETE FROM projects WHERE organization_id=?", "DELETE FROM brand_guideline_versions WHERE organization_id=?", "DELETE FROM products WHERE organization_id=?", "DELETE FROM brands WHERE organization_id=?", "DELETE FROM service_identity_scopes WHERE organization_id=?", "DELETE FROM service_identities WHERE organization_id=?", "DELETE FROM organization_memberships WHERE organization_id=?", "DELETE FROM organizations WHERE id=?",
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement, organizationID); err != nil {
			t.Errorf("cleanup %q: %v", statement, err)
		}
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM users WHERE id=?", userID); err != nil {
		t.Errorf("cleanup user: %v", err)
	}
}
