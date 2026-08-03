package assets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"strconv"
	"strings"
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

func TestUploadPersistsVideoProbeMetadata(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	metadata := MediaMetadata{
		DurationSeconds: 12.4,
		FPS:             29.97,
		Codec:           "h264",
		BitrateBPS:      3_200_000,
		AudioCodec:      "aac",
		AudioChannels:   2,
		AudioSampleRate: 48000,
		PosterFrameRef:  "poster://asset/video_1/frame_0",
	}
	service := UploadService{
		Repository: repo, Projects: fakeProjects{organization: "org_1", project: "project_1", version: 4},
		Blobs: NewMemoryBlobStore(), Scanner: NoopScanner{}, MediaProbe: StaticMediaProbe{Metadata: metadata},
		QuarantineBucket: "quarantine", AssetsBucket: "assets", Now: func() time.Time { return now }, NewID: sequenceIDs(),
	}
	data := testMP4()
	digest := sha256.Sum256(data)
	hash := hex.EncodeToString(digest[:])
	rc := testRequestContext("org_1", "project_1")
	created, err := service.Create(context.Background(), rc, "project_1", "video-upload-key", CreateUploadRequest{Filename: "hero.mp4", DeclaredMIMEType: "video/mp4", DeclaredSizeBytes: int64(len(data)), DeclaredSHA256: &hash})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.PutContent(context.Background(), rc.Actor, "project_1", created.Session.ID, bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatal(err)
	}
	result, err := service.Finalize(context.Background(), rc, "project_1", created.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.ProjectAssetRef == nil {
		t.Fatal("expected project asset ref")
	}
	stored, err := repo.GetProjectAsset(context.Background(), "org_1", "project_1", result.ProjectAssetRef.AssetVersion)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Asset.Kind != contract.AssetVideo || stored.Version.MIMEType != "video/mp4" {
		t.Fatalf("unexpected video asset: %#v", stored)
	}
	if stored.Version.Media.ProbeStatus != MediaProbeSucceeded || stored.Version.Media.DurationSeconds != 12.4 || stored.Version.Media.Codec != "h264" || stored.Version.Media.AudioCodec != "aac" || stored.Version.Media.PosterFrameRef == "" {
		t.Fatalf("unexpected media metadata: %#v", stored.Version.Media)
	}
	items, err := service.List(context.Background(), rc.Actor, "project_1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Version.Media.FPS != 29.97 {
		t.Fatalf("list did not return media metadata: %#v", items)
	}
}

func TestUploadKeepsVideoWhenProbeFails(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	service := UploadService{
		Repository: repo, Projects: fakeProjects{organization: "org_1", project: "project_1", version: 4},
		Blobs: NewMemoryBlobStore(), Scanner: NoopScanner{}, MediaProbe: StaticMediaProbe{Err: context.DeadlineExceeded},
		QuarantineBucket: "quarantine", AssetsBucket: "assets", Now: func() time.Time { return now }, NewID: sequenceIDs(),
	}
	data := testMP4()
	rc := testRequestContext("org_1", "project_1")
	created, err := service.Create(context.Background(), rc, "project_1", "video-upload-key", CreateUploadRequest{Filename: "hero.mp4", DeclaredMIMEType: "video/mp4", DeclaredSizeBytes: int64(len(data))})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.PutContent(context.Background(), rc.Actor, "project_1", created.Session.ID, bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatal(err)
	}
	result, err := service.Finalize(context.Background(), rc, "project_1", created.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := repo.GetProjectAsset(context.Background(), "org_1", "project_1", result.ProjectAssetRef.AssetVersion)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Version.Media.ProbeStatus != MediaProbeFailed || stored.Version.Media.ProbeError == "" {
		t.Fatalf("unexpected failed probe metadata: %#v", stored.Version.Media)
	}
}

func TestAssetFeatureUpsertValidatesAndIsolatesProjectScope(t *testing.T) {
	now := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	service := UploadService{Repository: repo, Projects: fakeProjects{organization: "org_1", project: "project_1", version: 4}, Blobs: NewMemoryBlobStore(), Scanner: NoopScanner{}, QuarantineBucket: "quarantine", AssetsBucket: "assets", Now: func() time.Time { return now }, NewID: sequenceIDs()}
	ref := repo.commit(AssetCommit{BlobID: "blob_1", OrganizationID: "org_1", ProjectID: "project_1", AssetID: "asset_1", Version: 1, Kind: contract.AssetVideo, SourceType: contract.AssetSourceUpload, OwnerSystem: "assets", MIMEType: "video/mp4", SizeBytes: 128, SHA256: strings.Repeat("a", 64), Media: MediaMetadata{ProbeStatus: MediaProbeSucceeded}, Location: ObjectLocation{Provider: "memory", Bucket: "assets", Key: "asset_1"}, Event: contract.EventEnvelope{EventID: "event_1"}}, now)
	actor := testRequestContext("org_1", "project_1").Actor
	feature := testAssetFeature(ref.AssetVersion)

	stored, err := service.UpsertFeature(context.Background(), actor, "project_1", feature)
	if err != nil {
		t.Fatal(err)
	}
	feature.HookStrength = 0.93
	updated, err := service.UpsertFeature(context.Background(), actor, "project_1", feature)
	if err != nil {
		t.Fatal(err)
	}
	if stored.CreatedAt.IsZero() || !stored.CreatedAt.Equal(updated.CreatedAt) || updated.HookStrength != 0.93 {
		t.Fatalf("unexpected upsert timestamps/value: stored=%#v updated=%#v", stored, updated)
	}
	got, err := service.GetFeature(context.Background(), actor, "project_1", ref.AssetVersion, "vlm-2026-07-26")
	if err != nil {
		t.Fatal(err)
	}
	if got.SellingPoints[0] != "0.01mm 精度" || got.SimilarityRisk != AssetFeatureRiskMedium {
		t.Fatalf("unexpected feature: %#v", got)
	}
	if _, err := service.GetFeature(context.Background(), actor, "project_other", ref.AssetVersion, "vlm-2026-07-26"); err == nil {
		t.Fatal("expected cross-project feature read to fail")
	}
	feature.SchemaVersion = "asset_feature_v0"
	if _, err := service.UpsertFeature(context.Background(), actor, "project_1", feature); err == nil {
		t.Fatal("expected schema validation error")
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
	sourceVersion := int64(3)
	request := GeneratedAssetIntakeRequest{ProviderJobID: "job_1", Output: contract.ProviderOutputRef{ProviderCode: "fake", ProviderJobID: "job_1", OutputID: "output_1", RetrievalExpiresAt: now.Add(time.Hour), DeclaredMIMEType: "image/png", DeclaredSizeBytes: int64(len(data)), DeclaredSHA256: &hash}, Provenance: GenerationProvenance{Capability: "image.generate", ProviderCode: "fake", ModelAlias: "image.standard", ModelVersion: "v1", SourceAssetRefs: []contract.AssetVersionRef{{AssetID: "input_asset_1", Version: sourceVersion}}, SourceResourceRefs: []contract.ResourceRef{{Type: "remix_plan", ID: "remixplan_1"}, {Type: "remix_render_job", ID: "remixrender_1"}}, ProjectContextVersion: 7, GeneratedAt: now}}
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
	relations, err := repo.ListAssetRelations(context.Background(), "org_1", "project_1", stored.ProjectAssetRef.AssetVersion)
	if err != nil {
		t.Fatal(err)
	}
	if len(relations) != 3 {
		t.Fatalf("relations count=%d relations=%#v", len(relations), relations)
	}
	sources := map[string]bool{}
	for _, relation := range relations {
		if relation.OutputAsset != stored.ProjectAssetRef.AssetVersion || relation.RelationType != AssetRelationGeneratedFrom {
			t.Fatalf("unexpected relation: %#v", relation)
		}
		version := int64(0)
		if relation.Source.Version != nil {
			version = *relation.Source.Version
		}
		sources[relation.Source.Type+"/"+relation.Source.ID+"/"+strconv.FormatInt(version, 10)] = true
	}
	for _, want := range []string{"asset_version/input_asset_1/3", "remix_plan/remixplan_1/0", "remix_render_job/remixrender_1/0"} {
		if !sources[want] {
			t.Fatalf("missing relation source %s in %#v", want, relations)
		}
	}
}

func TestGeneratedIntakeUsesFilesystemBlobStoreWithoutLeakingStorageHandles(t *testing.T) {
	now := time.Date(2026, 7, 22, 11, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	projects := fakeProjects{organization: "org_1", project: "project_1", version: 7}
	ids := sequenceIDs()
	service := GeneratedIntakeService{Repository: repo, Projects: projects, Now: func() time.Time { return now }, NewID: ids}
	data := testPNG(t)
	digest := sha256.Sum256(data)
	hash := hex.EncodeToString(digest[:])
	request := GeneratedAssetIntakeRequest{
		ProviderJobID: "job_1",
		Output: contract.ProviderOutputRef{
			ProviderCode:       "fake",
			ProviderJobID:      "job_1",
			OutputID:           "output_1",
			RetrievalExpiresAt: now.Add(time.Hour),
			DeclaredMIMEType:   "image/png",
			DeclaredSizeBytes:  int64(len(data)),
			DeclaredSHA256:     &hash,
		},
		Provenance: GenerationProvenance{
			Capability:            "image.generate",
			ProviderCode:          "fake",
			ModelAlias:            "image.standard",
			ModelVersion:          "v1",
			SourceAssetRefs:       []contract.AssetVersionRef{},
			ProjectContextVersion: 7,
			GeneratedAt:           now,
		},
	}
	rc := testRequestContext("org_1", "project_1")
	intake, err := service.Create(context.Background(), rc, "project_1", "provider-job-1-output-1", request)
	if err != nil {
		t.Fatal(err)
	}
	blobs, err := NewFilesystemBlobStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	upload := UploadService{Repository: repo, Projects: projects, Blobs: blobs, Scanner: NoopScanner{}, QuarantineBucket: "quarantine", AssetsBucket: "assets", Now: func() time.Time { return now }, NewID: ids}
	worker := GeneratedIntakeWorker{Repository: repo, Projects: projects, Fetcher: fakeFetcher{data: data, metadata: contract.OutputMetadata{MIMEType: "image/png", SizeBytes: int64(len(data)), SHA256: hash}}, Upload: upload, Actor: rc.Actor, Now: func() time.Time { return now }}

	processed, err := worker.ProcessOnce(context.Background(), "worker_1")
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Fatal("expected queued intake")
	}
	stored, err := service.Get(context.Background(), rc.Actor, "project_1", intake.ID)
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
	if asset.Version.Blob.Provider != "filesystem" || asset.Version.Blob.Bucket != "assets" || asset.Version.Blob.Key == "" {
		t.Fatalf("generated asset was not committed to filesystem storage: %#v", asset.Version.Blob)
	}
	responseJSON, err := json.Marshal(stored.Response())
	if err != nil {
		t.Fatal(err)
	}
	response := string(responseJSON)
	for _, forbidden := range []string{"bucket", "object_key", "storage", "tos://", "filesystem://", "https://vendor.example"} {
		if strings.Contains(response, forbidden) {
			t.Fatalf("generated intake response leaked %q: %s", forbidden, response)
		}
	}
}

func TestGeneratedIntakeRetriesTransientFailureThenRecordsAsset(t *testing.T) {
	now := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	projects := fakeProjects{organization: "org_1", project: "project_1", version: 7}
	ids := sequenceIDs()
	service := GeneratedIntakeService{Repository: repo, Projects: projects, Now: func() time.Time { return now }, NewID: ids, MaxAttempts: 2}
	data := testPNG(t)
	digest := sha256.Sum256(data)
	hash := hex.EncodeToString(digest[:])
	request := GeneratedAssetIntakeRequest{ProviderJobID: "job_1", Output: contract.ProviderOutputRef{ProviderCode: "fake", ProviderJobID: "job_1", OutputID: "output_1", RetrievalExpiresAt: now.Add(time.Hour), DeclaredMIMEType: "image/png", DeclaredSizeBytes: int64(len(data)), DeclaredSHA256: &hash}, Provenance: GenerationProvenance{Capability: "image.generate", ProviderCode: "fake", ModelAlias: "image.standard", ModelVersion: "v1", SourceAssetRefs: []contract.AssetVersionRef{}, SourceResourceRefs: []contract.ResourceRef{{Type: "remix_render_job", ID: "remixrender_1"}}, ProjectContextVersion: 7, GeneratedAt: now}}
	rc := testRequestContext("org_1", "project_1")
	intake, err := service.Create(context.Background(), rc, "project_1", "provider-job-1-output-1", request)
	if err != nil {
		t.Fatal(err)
	}
	upload := UploadService{Repository: repo, Projects: projects, Blobs: NewMemoryBlobStore(), Scanner: NoopScanner{}, QuarantineBucket: "quarantine", AssetsBucket: "assets", Now: func() time.Time { return now }, NewID: ids}
	firstWorker := GeneratedIntakeWorker{Repository: repo, Projects: projects, Fetcher: fakeFetcher{err: transientFetcherError{}}, Upload: upload, Actor: rc.Actor, Now: func() time.Time { return now }}
	processed, err := firstWorker.ProcessOnce(context.Background(), "worker_1")
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Fatal("expected first attempt to claim intake")
	}
	requeued, err := service.Get(context.Background(), rc.Actor, "project_1", intake.ID)
	if err != nil {
		t.Fatal(err)
	}
	if requeued.Status != GeneratedIntakeQueued || requeued.Error == nil || !requeued.Error.Retryable {
		t.Fatalf("expected retryable queued intake, got %#v", requeued)
	}
	secondWorker := GeneratedIntakeWorker{Repository: repo, Projects: projects, Fetcher: fakeFetcher{data: data, metadata: contract.OutputMetadata{MIMEType: "image/png", SizeBytes: int64(len(data)), SHA256: hash}}, Upload: upload, Actor: rc.Actor, Now: func() time.Time { return now.Add(time.Second) }}
	processed, err = secondWorker.ProcessOnce(context.Background(), "worker_2")
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Fatal("expected second attempt to claim intake")
	}
	done, err := service.Get(context.Background(), rc.Actor, "project_1", intake.ID)
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != GeneratedIntakeSucceeded || done.ProjectAssetRef == nil {
		t.Fatalf("expected successful retry, got %#v", done)
	}
}

func TestGeneratedIntakeCompletesMP4AsVideoAsset(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 27, 8, 0, 0, 0, time.UTC)
	repo := newFakeRepository()
	projects := fakeProjects{organization: "org_1", project: "project_1", version: 7}
	ids := sequenceIDs()
	data := []byte{
		0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm',
		0x00, 0x00, 0x02, 0x00, 'i', 's', 'o', 'm', 'm', 'p', '4', '2',
	}
	digest := sha256.Sum256(data)
	hash := hex.EncodeToString(digest[:])
	request := GeneratedAssetIntakeRequest{
		ProviderJobID: "video_job_1",
		Output: contract.ProviderOutputRef{
			ProviderCode: "fake-video", ProviderJobID: "video_job_1", OutputID: "output_1",
			RetrievalExpiresAt: now.Add(time.Hour), DeclaredMIMEType: "video/mp4",
			DeclaredSizeBytes: int64(len(data)), DeclaredSHA256: &hash,
		},
		Provenance: GenerationProvenance{
			Capability: "video.generate", ProviderCode: "fake-video", ModelAlias: "cookies.video.standard", ModelVersion: "fake-video-v1",
			SourceAssetRefs: []contract.AssetVersionRef{}, ProjectContextVersion: 7, GeneratedAt: now,
		},
	}
	rc := testRequestContext("org_1", "project_1")
	service := GeneratedIntakeService{Repository: repo, Projects: projects, Now: func() time.Time { return now }, NewID: ids}
	intake, err := service.Create(context.Background(), rc, "project_1", "provider-video-output-1", request)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	upload := UploadService{
		Repository: repo, Projects: projects, Blobs: NewMemoryBlobStore(), Scanner: NoopScanner{},
		QuarantineBucket: "quarantine", AssetsBucket: "assets", Now: func() time.Time { return now }, NewID: ids,
		VideoProbe: fakeVideoProbe{metadata: VideoMetadata{
			DurationMS: 5000, WidthPixels: 720, HeightPixels: 1280, FrameRate: "25/1", VideoCodec: "h264", AudioCodec: "aac",
		}},
	}
	worker := GeneratedIntakeWorker{
		Repository: repo, Projects: projects,
		Fetcher: fakeFetcher{data: data, metadata: contract.OutputMetadata{MIMEType: "video/mp4", SizeBytes: int64(len(data)), SHA256: hash}},
		Upload:  upload, Actor: rc.Actor, Now: func() time.Time { return now },
	}
	if processed, err := worker.ProcessOnce(context.Background(), "worker_video_1"); err != nil || !processed {
		t.Fatalf("ProcessOnce() = (%v, %v)", processed, err)
	}
	stored, err := service.Get(context.Background(), rc.Actor, "project_1", intake.ID)
	if err != nil || stored.ProjectAssetRef == nil {
		t.Fatalf("Get() = (%+v, %v)", stored, err)
	}
	asset, err := repo.GetProjectAsset(context.Background(), "org_1", "project_1", stored.ProjectAssetRef.AssetVersion)
	if err != nil {
		t.Fatal(err)
	}
	if asset.Asset.Kind != contract.AssetVideo || asset.Version.MIMEType != "video/mp4" {
		t.Fatalf("generated MP4 asset = %+v", asset)
	}
	if asset.Version.DurationMS != 5000 || asset.Version.VideoCodec != "h264" || asset.Version.WidthPixels != 720 {
		t.Fatalf("generated video metadata = %+v", asset.Version)
	}
}

func TestRenderedVideoIntakeIsIdempotentByRenderJob(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 27, 8, 30, 0, 0, time.UTC)
	repository := newFakeRepository()
	service := UploadService{
		Repository:       repository,
		Projects:         fakeProjects{organization: "org_1", project: "project_1", version: 7},
		Blobs:            NewMemoryBlobStore(),
		Scanner:          NoopScanner{},
		QuarantineBucket: "quarantine",
		AssetsBucket:     "assets",
		Now:              func() time.Time { return now },
		NewID:            sequenceIDs(),
		VideoProbe: fakeVideoProbe{metadata: VideoMetadata{
			DurationMS: 17000, WidthPixels: 720, HeightPixels: 1280, FrameRate: "25/1", VideoCodec: "h264", AudioCodec: "aac",
		}},
	}
	contents := append([]byte{0, 0, 0, 24}, []byte("ftypisom-rendered")...)
	requestContext := testRequestContext("org_1", "project_1")
	first, err := service.IngestRenderedVideo(context.Background(), requestContext, "project_1", "renderjob_1", bytes.NewReader(contents), int64(len(contents)))
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.IngestRenderedVideo(context.Background(), requestContext, "project_1", "renderjob_1", bytes.NewReader(contents), int64(len(contents)))
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("render retry created a second asset: first=%+v second=%+v", first, second)
	}
	asset, err := repository.GetProjectAsset(context.Background(), "org_1", "project_1", first.AssetVersion)
	if err != nil {
		t.Fatal(err)
	}
	if asset.Version.SourceType != contract.AssetSourceRendered || asset.Version.RenderJobID != "renderjob_1" {
		t.Fatalf("render lineage was not preserved: %+v", asset.Version)
	}
}

func TestDerivedImageIntakeIsIdempotentAndPreservesSourceLineage(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 31, 16, 0, 0, 0, time.UTC)
	repository := newFakeRepository()
	service := UploadService{
		Repository: repository, Projects: fakeProjects{organization: "org_1", project: "project_1", version: 7},
		Blobs: NewMemoryBlobStore(), Scanner: NoopScanner{}, QuarantineBucket: "quarantine", AssetsBucket: "assets",
		Now: func() time.Time { return now }, NewID: sequenceIDs(),
	}
	source := repository.commit(AssetCommit{
		BlobID: "source_blob", OrganizationID: "org_1", ProjectID: "project_1", AssetID: "source_video", Version: 1,
		Kind: contract.AssetVideo, SourceType: contract.AssetSourceUpload, OwnerSystem: "assets", MIMEType: "video/mp4",
		SizeBytes: 128, SHA256: strings.Repeat("a", 64), Location: ObjectLocation{Provider: "memory", Bucket: "assets", Key: "source"},
	}, now).AssetVersion
	frame := testPNG(t)
	rc := testRequestContext("org_1", "project_1")
	first, err := service.IngestDerivedImage(context.Background(), rc, "project_1", "game-frame-source_video-v1-21271ms-v1", source, bytes.NewReader(frame), int64(len(frame)), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.IngestDerivedImage(context.Background(), rc, "project_1", "game-frame-source_video-v1-21271ms-v1", source, bytes.NewReader(frame), int64(len(frame)), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("derivation retry created a second asset: first=%+v second=%+v", first, second)
	}
	stored, err := repository.GetProjectAsset(context.Background(), "org_1", "project_1", first.AssetVersion)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Asset.Kind != contract.AssetImage || stored.Version.SourceType != contract.AssetSourceDerived || stored.Version.DerivationID == "" {
		t.Fatalf("derived asset metadata = %+v", stored.Version)
	}
	relations, err := repository.ListAssetRelations(context.Background(), "org_1", "project_1", first.AssetVersion)
	if err != nil {
		t.Fatal(err)
	}
	if len(relations) != 1 || relations[0].RelationType != AssetRelationDerivedFrom || relations[0].Source.ID != string(source.AssetID) || relations[0].Source.Version == nil || *relations[0].Source.Version != source.Version {
		t.Fatalf("derived source lineage = %+v", relations)
	}
}

type fakeVideoProbe struct {
	metadata VideoMetadata
	err      error
}

func (p fakeVideoProbe) Probe(context.Context, []byte) (VideoMetadata, error) {
	return p.metadata, p.err
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

func (f fakeFetcher) Open(context.Context, contract.ProjectRef, contract.ProviderOutputRef) (io.ReadCloser, contract.OutputMetadata, error) {
	return io.NopCloser(bytes.NewReader(f.data)), f.metadata, f.err
}

type transientFetcherError struct{}

func (transientFetcherError) Error() string   { return "temporary fetch failure" }
func (transientFetcherError) Retryable() bool { return true }

type fakeRepository struct {
	mu         sync.Mutex
	uploads    map[string]UploadSession
	uploadKeys map[string]string
	intakes    map[string]GeneratedIntake
	intakeKeys map[string]string
	assets     map[string]ProjectAsset
	relations  map[string][]AssetRelation
	features   map[string]AssetFeature
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{uploads: map[string]UploadSession{}, uploadKeys: map[string]string{}, intakes: map[string]GeneratedIntake{}, intakeKeys: map[string]string{}, assets: map[string]ProjectAsset{}, relations: map[string][]AssetRelation{}, features: map[string]AssetFeature{}}
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
func (r *fakeRepository) CompleteRender(_ context.Context, renderJobID string, c AssetCommit, now time.Time) (contract.ProjectAssetRef, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.assets {
		if existing.Asset.OrganizationID == c.OrganizationID && existing.Version.RenderJobID == renderJobID {
			return existing.Ref, nil
		}
	}
	return r.commit(c, now), nil
}
func (r *fakeRepository) CompleteDerived(_ context.Context, derivationID string, c AssetCommit, now time.Time) (contract.ProjectAssetRef, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.assets {
		if existing.Asset.OrganizationID == c.OrganizationID && existing.Version.DerivationID == derivationID {
			return existing.Ref, nil
		}
	}
	return r.commit(c, now), nil
}
func (r *fakeRepository) commit(c AssetCommit, now time.Time) contract.ProjectAssetRef {
	ref := contract.ProjectAssetRef{ProjectID: c.ProjectID, AssetVersion: contract.AssetVersionRef{AssetID: c.AssetID, Version: c.Version}}
	r.assets[string(c.OrganizationID)+"/"+string(c.ProjectID)+"/"+string(c.AssetID)] = ProjectAsset{Ref: ref, Asset: Asset{ID: c.AssetID, OrganizationID: c.OrganizationID, Kind: c.Kind, Status: AssetReady, OwnerSystem: c.OwnerSystem, LatestVersion: c.Version, CreatedAt: now, UpdatedAt: now}, Version: AssetVersion{OrganizationID: c.OrganizationID, AssetID: c.AssetID, Version: c.Version, Status: AssetReady, SourceType: c.SourceType, MIMEType: c.MIMEType, SizeBytes: c.SizeBytes, SHA256: c.SHA256, WidthPixels: c.WidthPixels, HeightPixels: c.HeightPixels, Media: c.Media, DurationMS: c.DurationMS, FrameRate: c.FrameRate, VideoCodec: c.VideoCodec, AudioCodec: c.AudioCodec, RenderJobID: c.RenderJobID, DerivationID: c.DerivationID, ProviderJobID: c.ProviderJobID, ProviderOutputID: c.ProviderOutputID, ProjectContextVersion: c.ProjectContextVersion, Blob: c.Location, CreatedAt: now}, CreatedAt: now}
	for _, relation := range c.Relations {
		relation.CreatedAt = now
		key := relationKey(c.OrganizationID, c.ProjectID, relation.OutputAsset)
		r.relations[key] = append(r.relations[key], relation)
	}
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
func (r *fakeRepository) ListAssetRelations(_ context.Context, o contract.OrganizationID, p contract.ProjectID, ref contract.AssetVersionRef) ([]AssetRelation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]AssetRelation(nil), r.relations[relationKey(o, p, ref)]...), nil
}
func (r *fakeRepository) UpsertAssetFeature(_ context.Context, v AssetFeature, now time.Time) (AssetFeature, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.assets[string(v.OrganizationID)+"/"+string(v.ProjectID)+"/"+string(v.AssetID)]; !ok {
		return AssetFeature{}, ErrNotFound
	}
	key := featureKey(v.OrganizationID, v.ProjectID, v.AssetID, v.AssetVersion, v.FeatureVersion)
	if old, ok := r.features[key]; ok {
		v.CreatedAt = old.CreatedAt
	} else {
		v.CreatedAt = now
	}
	v.UpdatedAt = now
	r.features[key] = v
	return v, nil
}
func (r *fakeRepository) GetAssetFeature(_ context.Context, o contract.OrganizationID, p contract.ProjectID, ref contract.AssetVersionRef, featureVersion string) (AssetFeature, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.features[featureKey(o, p, ref.AssetID, ref.Version, featureVersion)]
	if !ok {
		return AssetFeature{}, ErrNotFound
	}
	return v, nil
}
func (r *fakeRepository) ListAssetFeatures(_ context.Context, o contract.OrganizationID, p contract.ProjectID, _ int) ([]AssetFeature, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []AssetFeature{}
	for _, v := range r.features {
		if v.OrganizationID == o && v.ProjectID == p {
			out = append(out, v)
		}
	}
	return out, nil
}
func featureKey(o contract.OrganizationID, p contract.ProjectID, assetID contract.AssetID, version int64, featureVersion string) string {
	return string(o) + "/" + string(p) + "/" + string(assetID) + "/" + strconv.FormatInt(version, 10) + "/" + featureVersion
}
func relationKey(o contract.OrganizationID, p contract.ProjectID, ref contract.AssetVersionRef) string {
	return string(o) + "/" + string(p) + "/" + string(ref.AssetID) + "/" + strconv.FormatInt(ref.Version, 10)
}

func testAssetFeature(ref contract.AssetVersionRef) AssetFeature {
	return AssetFeature{
		AssetID: ref.AssetID, AssetVersion: ref.Version, SchemaVersion: AssetFeatureSchemaV1,
		FeatureVersion: "vlm-2026-07-26", HookStrength: 0.82, ProductVisibility: 0.76,
		SceneTags: []string{"factory"}, ProductTags: []string{"cnc"}, PersonTags: []string{"engineer"},
		ActionTags: []string{"cutting"}, EmotionTags: []string{"trust"}, SellingPoints: []string{"0.01mm 精度"},
		CTAPresence: true, SimilarityGroup: "precision-demo-a", SimilarityRisk: AssetFeatureRiskMedium,
		Evidence: []string{"00:00-00:03 强钩子"},
	}
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
func testMP4() []byte {
	return []byte{
		0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'm', 'p', '4', '2',
		0x00, 0x00, 0x00, 0x00, 'm', 'p', '4', '2', 'i', 's', 'o', 'm',
		0x00, 0x00, 0x00, 0x08, 'm', 'd', 'a', 't',
	}
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
