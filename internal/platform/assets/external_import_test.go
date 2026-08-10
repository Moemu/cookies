package assets

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestExternalImportSuccessReplayAndConflicts(t *testing.T) {
	service, repo, requestContext := testExternalImportService(t)
	request := ExternalMediaImportRequest{SourceProvider: "catalog", SourceObjectID: "video_1", MIMEType: "video/mp4", SizeBytes: int64(len(externalTestMP4()))}
	first, err := service.Import(context.Background(), requestContext, "project_1", "key_1", request, testExternalOpener(externalTestMP4()))
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Import(context.Background(), requestContext, "project_1", "key_1", request, func(context.Context) (io.ReadCloser, error) { t.Fatal("replay opened source"); return nil, nil })
	if err != nil || first != second {
		t.Fatalf("replay = %#v, %v", second, err)
	}
	if len(repo.commits) != 1 {
		t.Fatalf("commits = %d", len(repo.commits))
	}
	_, err = service.Import(context.Background(), requestContext, "project_1", "other_key", request, testExternalOpener(externalTestMP4()))
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("source conflict = %v", err)
	}
}

func TestExternalImportRejectsInvalidVideoAndRecordsFailure(t *testing.T) {
	service, repo, requestContext := testExternalImportService(t)
	invalid := []byte("not-an-mp4!")
	request := ExternalMediaImportRequest{SourceProvider: "catalog", SourceObjectID: "invalid", MIMEType: "video/mp4", SizeBytes: int64(len(invalid))}
	_, err := service.Import(context.Background(), requestContext, "project_1", "key_invalid", request, testExternalOpener(invalid))
	if !errors.Is(err, ErrInvalidAssetContent) {
		t.Fatalf("invalid import = %v", err)
	}
	if repo.only().Status != ExternalImportFailed {
		t.Fatalf("status = %s", repo.only().Status)
	}
	request.SourceObjectID, request.SizeBytes = "too-large", MaxVideoBytes+1
	_, err = service.Import(context.Background(), requestContext, "project_1", "key_large", request, testExternalOpener(nil))
	if err == nil {
		t.Fatal("oversize request accepted")
	}
}

func TestExternalImportRetriesAResolvedFailureWithTheSameIdentity(t *testing.T) {
	service, repo, requestContext := testExternalImportService(t)
	video := externalTestMP4()
	request := ExternalMediaImportRequest{SourceProvider: "catalog", SourceObjectID: "retry", MIMEType: "video/mp4", SizeBytes: int64(len(video))}
	_, err := service.Import(context.Background(), requestContext, "project_1", "key_retry", request, func(context.Context) (io.ReadCloser, error) {
		return nil, errors.New("source temporarily unavailable")
	})
	if err == nil || repo.only().Status != ExternalImportFailed || repo.only().AttemptCount != 1 {
		t.Fatalf("first attempt status=%s attempts=%d err=%v", repo.only().Status, repo.only().AttemptCount, err)
	}
	got, err := service.Import(context.Background(), requestContext, "project_1", "key_retry", request, testExternalOpener(video))
	if err != nil || got.AssetVersion.AssetID == "" || repo.only().AttemptCount != 2 || repo.only().Status != ExternalImportSucceeded {
		t.Fatalf("retry result=%#v ledger=%#v err=%v", got, repo.only(), err)
	}
}

func TestExternalImportReusesProjectHashAndRecoversUnknownCommit(t *testing.T) {
	service, repo, requestContext := testExternalImportService(t)
	request := ExternalMediaImportRequest{SourceProvider: "catalog", SourceObjectID: "reuse", MIMEType: "video/mp4", SizeBytes: int64(len(externalTestMP4()))}
	repo.reuse = &contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "asset_old", Version: 1}}
	got, err := service.Import(context.Background(), requestContext, "project_1", "key_reuse", request, testExternalOpener(externalTestMP4()))
	if err != nil || got != *repo.reuse {
		t.Fatalf("reuse = %#v, %v", got, err)
	}
	if len(repo.commits) != 1 || repo.commits[0].BlobID != "" {
		t.Fatal("reuse did not use ledger completion")
	}

	service, repo, requestContext = testExternalImportService(t)
	request.SourceObjectID = "unknown"
	repo.ambiguous = true
	got, err = service.Import(context.Background(), requestContext, "project_1", "key_unknown", request, testExternalOpener(externalTestMP4()))
	if err != nil || got.AssetVersion.AssetID == "" {
		t.Fatalf("unknown recovery = %#v, %v", got, err)
	}
	if repo.only().ResultUnknownAt == nil {
		t.Fatal("unknown result was not recorded before readback")
	}
}

func externalTestMP4() []byte {
	return append([]byte{0, 0, 0, 16, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}, bytes.Repeat([]byte{0}, 16)...)
}
func testExternalOpener(data []byte) ExternalImportOpener {
	return func(context.Context) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }
}

func testExternalImportService(t *testing.T) (ExternalImportService, *fakeExternalImportRepository, contract.RequestContext) {
	t.Helper()
	now := time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)
	repo := newFakeExternalImportRepository()
	upload := UploadService{Blobs: NewMemoryBlobStore(), Scanner: NoopScanner{}, QuarantineBucket: "quarantine", AssetsBucket: "assets", VideoProbe: fakeVideoProbe{metadata: VideoMetadata{WidthPixels: 16, HeightPixels: 16}}, Now: func() time.Time { return now }, NewID: sequenceIDs()}
	service := ExternalImportService{Repository: repo, Projects: fakeProjects{organization: "org_1", project: "project_1", version: 3}, Upload: upload, QuarantineBucket: "quarantine", Now: func() time.Time { return now }, NewID: sequenceIDs()}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: "user", ID: "user_1"}, Scopes: []contract.Scope{"assets.write"}}
	return service, repo, contract.RequestContext{Actor: actor, RequestID: "request_1", TraceID: "trace_1"}
}

type fakeExternalImportRepository struct {
	values      map[string]ExternalImport
	source      map[string]string
	idempotency map[string]string
	commits     []AssetCommit
	reuse       *contract.ProjectAssetRef
	ambiguous   bool
}

func newFakeExternalImportRepository() *fakeExternalImportRepository {
	return &fakeExternalImportRepository{values: map[string]ExternalImport{}, source: map[string]string{}, idempotency: map[string]string{}}
}
func externalKey(v ExternalImport) string {
	return string(v.OrganizationID) + "/" + string(v.ProjectID) + "/" + v.SourceProvider + "/" + v.SourceObjectID
}
func (r *fakeExternalImportRepository) CreateExternalImport(_ context.Context, v ExternalImport) (ExternalImport, bool, error) {
	if id, ok := r.source[externalKey(v)]; ok {
		old := r.values[id]
		if old.IdempotencyKey != v.IdempotencyKey || old.RequestHash != v.RequestHash {
			return ExternalImport{}, false, ErrIdempotencyConflict
		}
		return old, false, nil
	}
	if _, ok := r.idempotency[string(v.OrganizationID)+"/"+string(v.ProjectID)+"/"+v.IdempotencyKey]; ok {
		return ExternalImport{}, false, ErrIdempotencyConflict
	}
	r.values[v.ID] = v
	r.source[externalKey(v)] = v.ID
	r.idempotency[string(v.OrganizationID)+"/"+string(v.ProjectID)+"/"+v.IdempotencyKey] = v.ID
	return v, true, nil
}
func (r *fakeExternalImportRepository) GetExternalImport(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, id string) (ExternalImport, error) {
	v, ok := r.values[id]
	if !ok {
		return ExternalImport{}, ErrNotFound
	}
	return v, nil
}
func (r *fakeExternalImportRepository) GetExternalImportBySource(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, provider, objectID string) (ExternalImport, error) {
	for _, value := range r.values {
		if value.SourceProvider == provider && value.SourceObjectID == objectID {
			return value, nil
		}
	}
	return ExternalImport{}, ErrNotFound
}
func (r *fakeExternalImportRepository) MarkExternalImportRunning(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, id string, now time.Time) error {
	v := r.values[id]
	v.Status = ExternalImportRunning
	v.AttemptCount++
	v.LastErrorCode, v.LastErrorMessage = "", ""
	v.UpdatedAt = now
	r.values[id] = v
	return nil
}
func (r *fakeExternalImportRepository) CompleteExternalImport(_ context.Context, id string, c AssetCommit, now time.Time) (contract.ProjectAssetRef, error) {
	v := r.values[id]
	r.commits = append(r.commits, c)
	if r.ambiguous {
		v.Status = ExternalImportSucceeded
		v.CommittedAssetID = c.AssetID
		v.CommittedAssetVersion = c.Version
		r.values[id] = v
		return contract.ProjectAssetRef{}, errors.New("commit connection lost")
	}
	if c.BlobID == "" && r.reuse != nil {
		c.AssetID, c.Version = r.reuse.AssetVersion.AssetID, r.reuse.AssetVersion.Version
	}
	v.Status = ExternalImportSucceeded
	v.CommittedAssetID = c.AssetID
	v.CommittedAssetVersion = c.Version
	v.UpdatedAt = now
	r.values[id] = v
	return contract.ProjectAssetRef{ProjectID: v.ProjectID, AssetVersion: contract.AssetVersionRef{AssetID: c.AssetID, Version: c.Version}}, nil
}
func (r *fakeExternalImportRepository) FailExternalImport(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, id, code, message string, now time.Time) error {
	v := r.values[id]
	v.Status = ExternalImportFailed
	v.LastErrorCode = code
	v.LastErrorMessage = message
	v.UpdatedAt = now
	r.values[id] = v
	return nil
}
func (r *fakeExternalImportRepository) MarkExternalImportResultUnknown(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, id, _ string, now time.Time) error {
	v := r.values[id]
	v.ResultUnknownAt = &now
	r.values[id] = v
	return nil
}
func (r *fakeExternalImportRepository) FindProjectAssetBySHA256(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _ string) (contract.ProjectAssetRef, error) {
	if r.reuse != nil {
		return *r.reuse, nil
	}
	return contract.ProjectAssetRef{}, ErrNotFound
}
func (r *fakeExternalImportRepository) only() ExternalImport {
	for _, v := range r.values {
		return v
	}
	panic("empty")
}
