package assets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"io"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestUploadCreatesImmutableProjectAsset(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	blobs := NewMemoryBlobStore()
	projects := fakeProjects{organization: "org_1", project: "project_1", version: 4}
	service := UploadService{Repository: repo, Projects: projects, Blobs: blobs, Scanner: NoopScanner{}, QuarantineBucket: "quarantine", AssetsBucket: "assets", Now: func() time.Time { return now }, NewID: sequenceIDs()}
	data := testPNG(t)
	digest := sha256.Sum256(data)
	hash := hex.EncodeToString(digest[:])
	rc := testRequestContext("org_1", "project_1")
	created, err := service.Create(context.Background(), rc, "project_1", "upload-key", CreateUploadRequest{Filename: "../hero.png", DeclaredMIMEType: "image/png", DeclaredSizeBytes: int64(len(data)), DeclaredSHA256: &hash})
	if err != nil {
		t.Fatal(err)
	}
	if created.Session.Filename != "hero.png" {
		t.Fatalf("filename=%q", created.Session.Filename)
	}
	if err := service.PutContent(context.Background(), rc.Actor, "project_1", created.Session.ID, bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatal(err)
	}
	result, err := service.Finalize(context.Background(), rc, "project_1", created.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != UploadSucceeded || result.ProjectAssetRef == nil || result.ProjectAssetRef.AssetVersion.Version != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	asset, err := repo.GetProjectAsset(context.Background(), "org_1", "project_1", result.ProjectAssetRef.AssetVersion)
	if err != nil {
		t.Fatal(err)
	}
	if asset.Version.SHA256 != hash || asset.Version.SourceType != contract.AssetSourceUpload || asset.Version.ProjectContextVersion != 4 {
		t.Fatalf("unexpected version: %#v", asset.Version)
	}
	preview, info, err := service.OpenPreview(context.Background(), rc.Actor, "project_1", result.ProjectAssetRef.AssetVersion)
	if err != nil {
		t.Fatalf("open preview: %v", err)
	}
	previewData, readErr := io.ReadAll(preview)
	closeErr := preview.Close()
	if readErr != nil || closeErr != nil || !bytes.Equal(previewData, data) || info.MIMEType != "image/png" {
		t.Fatalf("unexpected preview: size=%d mime=%q read=%v close=%v", len(previewData), info.MIMEType, readErr, closeErr)
	}
	if err := service.Remove(context.Background(), rc.Actor, "project_1", result.ProjectAssetRef.AssetVersion); err != nil {
		t.Fatalf("remove project asset: %v", err)
	}
	items, err := service.List(context.Background(), rc.Actor, "project_1", 10)
	if err != nil || len(items) != 0 {
		t.Fatalf("list after remove: count=%d err=%v", len(items), err)
	}
	if err := service.Remove(context.Background(), rc.Actor, "project_1", result.ProjectAssetRef.AssetVersion); err != nil {
		t.Fatalf("idempotent remove: %v", err)
	}
}

func TestUploadRejectsDeclaredMetadataMismatch(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	service := UploadService{Repository: repo, Projects: fakeProjects{organization: "org_1", project: "project_1", version: 1}, Blobs: NewMemoryBlobStore(), Scanner: NoopScanner{}, QuarantineBucket: "quarantine", AssetsBucket: "assets", Now: func() time.Time { return now }, NewID: sequenceIDs()}
	data := testPNG(t)
	rc := testRequestContext("org_1", "project_1")
	created, err := service.Create(context.Background(), rc, "project_1", "key", CreateUploadRequest{Filename: "x.png", DeclaredMIMEType: "image/png", DeclaredSizeBytes: int64(len(data)) + 1})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a direct signed upload whose actual object size differs from the declaration.
	if _, err := service.Blobs.Put(context.Background(), created.Session.Quarantine.Bucket, created.Session.Quarantine.Key, bytes.NewReader(data), int64(len(data)), "image/png"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Finalize(context.Background(), rc, "project_1", created.Session.ID); err == nil {
		t.Fatal("expected metadata mismatch")
	}
	stored, _ := repo.GetUpload(context.Background(), "org_1", "project_1", created.Session.ID)
	if stored.Status != UploadFailed || stored.ErrorCode != "OUTPUT_METADATA_MISMATCH" {
		t.Fatalf("unexpected failed upload: %#v", stored)
	}
}

func TestGeneratedIntakeIsIdempotentAndCompletesOneOutput(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	projects := fakeProjects{organization: "org_1", project: "project_1", version: 7}
	ids := sequenceIDs()
	service := GeneratedIntakeService{Repository: repo, Projects: projects, Now: func() time.Time { return now }, NewID: ids}
	data := testPNG(t)
	digest := sha256.Sum256(data)
	hash := hex.EncodeToString(digest[:])
	request := GeneratedAssetIntakeRequest{ProviderJobID: "job_1", Output: contract.ProviderOutputRef{ProviderCode: "fake", ProviderJobID: "job_1", OutputID: "output_1", RetrievalExpiresAt: now.Add(time.Hour), DeclaredMIMEType: "image/png", DeclaredSizeBytes: int64(len(data)), DeclaredSHA256: &hash}, Provenance: GenerationProvenance{Capability: "image.generate", ProviderCode: "fake", ModelAlias: "image.standard", ModelVersion: "v1", SourceAssetRefs: []contract.AssetVersionRef{}, ProjectContextVersion: 7, GeneratedAt: now}}
	rc := testRequestContext("org_1", "project_1")
	first, err := service.Create(context.Background(), rc, "project_1", "provider-job-1-output-1", request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Create(context.Background(), rc, "project_1", "provider-job-1-output-1", request)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("idempotency returned %q and %q", first.ID, second.ID)
	}
	blobs := NewMemoryBlobStore()
	upload := UploadService{Repository: repo, Projects: projects, Blobs: blobs, Scanner: NoopScanner{}, QuarantineBucket: "quarantine", AssetsBucket: "assets", Now: func() time.Time { return now }, NewID: ids}
	worker := GeneratedIntakeWorker{Repository: repo, Projects: projects, Fetcher: fakeFetcher{data: data, metadata: contract.OutputMetadata{MIMEType: "image/png", SizeBytes: int64(len(data)), SHA256: hash}}, Upload: upload, Actor: rc.Actor, Now: func() time.Time { return now }}
	processed, err := worker.ProcessOnce(context.Background(), "worker_1")
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Fatal("expected queued intake")
	}
	stored, err := service.Get(context.Background(), rc.Actor, "project_1", first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != GeneratedIntakeSucceeded || stored.ProjectAssetRef == nil {
		t.Fatalf("unexpected intake: %#v", stored)
	}
	asset, err := repo.GetProjectAsset(context.Background(), "org_1", "project_1", stored.ProjectAssetRef.AssetVersion)
	if err != nil {
		t.Fatal(err)
	}
	if asset.Version.ProviderJobID != "job_1" || asset.Version.ProviderOutputID != "output_1" || asset.Version.SourceType != contract.AssetSourceProviderGenerated {
		t.Fatalf("unexpected generated asset: %#v", asset.Version)
	}
}

func TestGeneratedIntakeCannotCrossOrganization(t *testing.T) {
	service := GeneratedIntakeService{Repository: newFakeRepository(), Projects: fakeProjects{organization: "org_1", project: "project_1", version: 1}}
	request := GeneratedAssetIntakeRequest{ProviderJobID: "job", Output: contract.ProviderOutputRef{ProviderCode: "fake", ProviderJobID: "job", OutputID: "out", RetrievalExpiresAt: time.Now().Add(time.Hour), DeclaredMIMEType: "image/png", DeclaredSizeBytes: 1}, Provenance: GenerationProvenance{Capability: "image.generate", ProviderCode: "fake", ModelAlias: "m", ModelVersion: "v", SourceAssetRefs: []contract.AssetVersionRef{}, ProjectContextVersion: 1, GeneratedAt: time.Now()}}
	_, err := service.Create(context.Background(), testRequestContext("org_2", "project_1"), "project_1", "key", request)
	if err == nil {
		t.Fatal("expected tenant authorization failure")
	}
}

type fakeProjects struct {
	organization contract.OrganizationID
	project      contract.ProjectID
	version      int64
}

func (p fakeProjects) RequireActiveContext(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID) (contract.ProjectContext, error) {
	if actor.OrganizationID != p.organization || projectID != p.project {
		return contract.ProjectContext{}, context.Canceled
	}
	brand := contract.BrandID("brand_1")
	return contract.ProjectContext{OrganizationID: p.organization, ProjectID: p.project, BrandID: &brand, ProductIDs: []contract.ProductID{}, ProjectContextVersion: p.version}, nil
}

type fakeFetcher struct {
	data     []byte
	metadata contract.OutputMetadata
	err      error
}

func (f fakeFetcher) Open(context.Context, contract.ProviderOutputRef) (io.ReadCloser, contract.OutputMetadata, error) {
	return io.NopCloser(bytes.NewReader(f.data)), f.metadata, f.err
}

type fakeRepository struct {
	mu         sync.Mutex
	uploads    map[string]UploadSession
	uploadKeys map[string]string
	intakes    map[string]GeneratedIntake
	intakeKeys map[string]string
	assets     map[string]ProjectAsset
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{uploads: map[string]UploadSession{}, uploadKeys: map[string]string{}, intakes: map[string]GeneratedIntake{}, intakeKeys: map[string]string{}, assets: map[string]ProjectAsset{}}
}
func (r *fakeRepository) CreateUpload(_ context.Context, v UploadSession) (UploadSession, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := string(v.OrganizationID) + "/" + string(v.ProjectID) + "/" + string(v.Principal.Kind) + "/" + v.Principal.ID + "/" + string(v.IdempotencyKey)
	if id, ok := r.uploadKeys[key]; ok {
		old := r.uploads[id]
		if old.RequestHash != v.RequestHash {
			return UploadSession{}, false, ErrIdempotencyConflict
		}
		return old, false, nil
	}
	r.uploads[v.ID] = v
	r.uploadKeys[key] = v.ID
	return v, true, nil
}
func (r *fakeRepository) GetUpload(_ context.Context, o contract.OrganizationID, p contract.ProjectID, id string) (UploadSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.uploads[id]
	if !ok || v.OrganizationID != o || v.ProjectID != p {
		return UploadSession{}, ErrNotFound
	}
	return v, nil
}
func (r *fakeRepository) MarkUploadUploaded(_ context.Context, o contract.OrganizationID, p contract.ProjectID, id string, now time.Time) error {
	return r.setUploadStatus(o, p, id, UploadUploaded, now)
}
func (r *fakeRepository) MarkUploadProcessing(_ context.Context, o contract.OrganizationID, p contract.ProjectID, id string, now time.Time) error {
	return r.setUploadStatus(o, p, id, UploadProcessing, now)
}
func (r *fakeRepository) setUploadStatus(o contract.OrganizationID, p contract.ProjectID, id string, status UploadStatus, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.uploads[id]
	if !ok || v.OrganizationID != o || v.ProjectID != p {
		return ErrNotFound
	}
	v.Status = status
	v.UpdatedAt = now
	r.uploads[id] = v
	return nil
}
func (r *fakeRepository) CompleteUpload(_ context.Context, id string, c AssetCommit, now time.Time) (contract.ProjectAssetRef, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.uploads[id]
	if !ok {
		return contract.ProjectAssetRef{}, ErrNotFound
	}
	ref := r.commit(c, now)
	v.Status = UploadSucceeded
	v.ProjectAssetRef = &ref
	r.uploads[id] = v
	return ref, nil
}
func (r *fakeRepository) FailUpload(_ context.Context, o contract.OrganizationID, p contract.ProjectID, id, code string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := r.uploads[id]
	v.Status = UploadFailed
	v.ErrorCode = code
	v.UpdatedAt = now
	r.uploads[id] = v
	return nil
}
func (r *fakeRepository) CreateIntake(_ context.Context, v GeneratedIntake) (GeneratedIntake, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := string(v.OrganizationID) + "/" + string(v.ProjectID) + "/" + v.ProviderJobID + "/" + v.OutputID
	if id, ok := r.intakeKeys[key]; ok {
		old := r.intakes[id]
		if old.IdempotencyKey != v.IdempotencyKey || old.RequestHash != v.RequestHash {
			return GeneratedIntake{}, false, ErrIdempotencyConflict
		}
		return old, false, nil
	}
	r.intakes[v.ID] = v
	r.intakeKeys[key] = v.ID
	return v, true, nil
}
func (r *fakeRepository) GetIntake(_ context.Context, o contract.OrganizationID, p contract.ProjectID, id string) (GeneratedIntake, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.intakes[id]
	if !ok || v.OrganizationID != o || v.ProjectID != p {
		return GeneratedIntake{}, ErrNotFound
	}
	return v, nil
}
func (r *fakeRepository) ClaimIntake(_ context.Context, actor contract.ActorContext, worker string, now time.Time) (GeneratedIntake, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, v := range r.intakes {
		if v.OrganizationID == actor.OrganizationID && v.Status == GeneratedIntakeQueued && !v.AvailableAt.After(now) {
			v.Status = GeneratedIntakeRunning
			v.LockOwner = worker
			v.AttemptCount++
			v.UpdatedAt = now
			r.intakes[id] = v
			return v, true, nil
		}
	}
	return GeneratedIntake{}, false, nil
}
func (r *fakeRepository) CompleteIntake(_ context.Context, v GeneratedIntake, c AssetCommit, now time.Time) (contract.ProjectAssetRef, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	stored := r.intakes[v.ID]
	ref := r.commit(c, now)
	stored.Status = GeneratedIntakeSucceeded
	stored.ProjectAssetRef = &ref
	stored.Error = nil
	r.intakes[v.ID] = stored
	return ref, nil
}
func (r *fakeRepository) RetryIntake(_ context.Context, v GeneratedIntake, e contract.JobError, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v.Status = GeneratedIntakeQueued
	v.Error = &e
	v.AvailableAt = now
	r.intakes[v.ID] = v
	return nil
}
func (r *fakeRepository) FailIntake(_ context.Context, v GeneratedIntake, e contract.JobError, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v.Status = GeneratedIntakeFailed
	v.Error = &e
	v.UpdatedAt = now
	r.intakes[v.ID] = v
	return nil
}
func (r *fakeRepository) commit(c AssetCommit, now time.Time) contract.ProjectAssetRef {
	ref := contract.ProjectAssetRef{ProjectID: c.ProjectID, AssetVersion: contract.AssetVersionRef{AssetID: c.AssetID, Version: c.Version}}
	r.assets[string(c.OrganizationID)+"/"+string(c.ProjectID)+"/"+string(c.AssetID)] = ProjectAsset{Ref: ref, Asset: Asset{ID: c.AssetID, OrganizationID: c.OrganizationID, Kind: c.Kind, Status: AssetReady, OwnerSystem: c.OwnerSystem, LatestVersion: c.Version, CreatedAt: now, UpdatedAt: now}, Version: AssetVersion{OrganizationID: c.OrganizationID, AssetID: c.AssetID, Version: c.Version, Status: AssetReady, SourceType: c.SourceType, MIMEType: c.MIMEType, SizeBytes: c.SizeBytes, SHA256: c.SHA256, WidthPixels: c.WidthPixels, HeightPixels: c.HeightPixels, ProviderJobID: c.ProviderJobID, ProviderOutputID: c.ProviderOutputID, ProjectContextVersion: c.ProjectContextVersion, Blob: c.Location, CreatedAt: now}, CreatedAt: now}
	return ref
}
func (r *fakeRepository) GetProjectAsset(_ context.Context, o contract.OrganizationID, p contract.ProjectID, ref contract.AssetVersionRef) (ProjectAsset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.assets[string(o)+"/"+string(p)+"/"+string(ref.AssetID)]
	if !ok || v.Ref.AssetVersion.Version != ref.Version {
		return ProjectAsset{}, ErrNotFound
	}
	return v, nil
}
func (r *fakeRepository) ListProjectAssets(_ context.Context, o contract.OrganizationID, p contract.ProjectID, _ int) ([]ProjectAsset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []ProjectAsset{}
	for _, v := range r.assets {
		if v.Asset.OrganizationID == o && v.Ref.ProjectID == p {
			out = append(out, v)
		}
	}
	return out, nil
}
func (r *fakeRepository) RemoveProjectAsset(_ context.Context, o contract.OrganizationID, p contract.ProjectID, ref contract.AssetVersionRef) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.assets, string(o)+"/"+string(p)+"/"+string(ref.AssetID))
	return nil
}

func testRequestContext(org contract.OrganizationID, _ contract.ProjectID) contract.RequestContext {
	return contract.RequestContext{RequestID: "req_1", TraceID: "trace_1", Actor: contract.ActorContext{OrganizationID: org, Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{"project.read", "project.write", "assets.read", "assets.write"}}}
}
func testPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func sequenceIDs() func(string) (string, error) {
	var mu sync.Mutex
	n := 0
	return func(prefix string) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		n++
		return prefix + "_test_" + strconv.Itoa(n), nil
	}
}
