package insights

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/integrations/crawler"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
)

func TestMiyunSecretCipherRoundTripAndKeyVersionIsolation(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	cipher, err := NewAESGCMMiyunSecretCipher(key, "key-v1")
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("session material")
	ciphertext, version, err := cipher.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if string(ciphertext) == string(plaintext) || version != "key-v1" {
		t.Fatal("session was not encrypted with versioned key material")
	}
	decrypted, err := cipher.Decrypt(ciphertext, version)
	if err != nil || string(decrypted) != string(plaintext) {
		t.Fatalf("decrypt = %q, %v", decrypted, err)
	}
	if _, err := cipher.Decrypt(ciphertext, "key-v2"); err == nil {
		t.Fatal("ciphertext decrypted under a different key version")
	}
}

func TestMiyunWorkerActorPreservesConfirmedUserPrincipal(t *testing.T) {
	actor := miyunWorkerActor(miyunRuntimePayload{OrganizationID: "org_1", ActorID: "user_1"})
	if actor.Principal.Kind != contract.PrincipalUser || actor.Principal.ID != "user_1" || !actor.HasScope("assets.write") {
		t.Fatalf("worker actor = %#v", actor)
	}
}

func TestMiyunConnectionVerificationPersistsSafeUpstreamFailureState(t *testing.T) {
	service, repository, _, _ := newMiyunCrawlTestService(t)
	service.MiyunVerifier = miyunVerifierTestDouble{err: errors.New("tls handshake failed")}

	value, err := service.VerifyMiyunConnection(context.Background(), miyunTestActor(), "project_1", VerifyMiyunConnectionRequest{ExpectedVersion: repository.connection.Version})
	if err != nil {
		t.Fatalf("verification should return persisted state, not an internal error: %v", err)
	}
	if value.LastErrorKind != string(crawler.YouShuTransport) || value.LastErrorCode != "UNCLASSIFIED" || repository.connection.Version != 2 {
		t.Fatalf("persisted verification state=%#v", value)
	}
}

func TestMiyunConnectionVerificationClearsStaleAuthStateOnNonAuthFailure(t *testing.T) {
	service, repository, _, _ := newMiyunCrawlTestService(t)
	repository.connection.Status = MiyunConnectionAuthRequired
	service.MiyunVerifier = miyunVerifierTestDouble{err: &crawler.YouShuError{
		Kind: crawler.YouShuInvalidRequest, Strategy: crawler.YouShuCorrectRequest, Source: "graphql", Code: "00:400999",
	}}

	value, err := service.VerifyMiyunConnection(context.Background(), miyunTestActor(), "project_1", VerifyMiyunConnectionRequest{ExpectedVersion: repository.connection.Version})
	if err != nil {
		t.Fatal(err)
	}
	if value.Status != MiyunConnectionUnverified || value.LastErrorKind != string(crawler.YouShuInvalidRequest) || value.LastErrorCode != "00:400999" {
		t.Fatalf("verification state=%#v", value)
	}
}

func TestMiyunCrawlCreateIsConfirmedAndIdempotent(t *testing.T) {
	service, repository, runtime, _ := newMiyunCrawlTestService(t)
	profile := confirmedMiyunCrawlTestProfile(t, &service)

	first, err := service.CreateMiyunCrawlJob(context.Background(), miyunTestActor(), "project_1", "crawl-key-1", CreateMiyunCrawlJobRequest{ProductProfileID: profile.ID, Operation: "product"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateMiyunCrawlJob(context.Background(), miyunTestActor(), "project_1", "crawl-key-1", CreateMiyunCrawlJobRequest{ProductProfileID: profile.ID, Operation: "product"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || len(repository.jobs) != 1 || len(runtime.byKey) != 1 {
		t.Fatalf("idempotent create duplicated state: first=%s second=%s jobs=%d runtime=%d", first.ID, second.ID, len(repository.jobs), len(runtime.byKey))
	}
	var snapshot MiyunQuerySnapshot
	if json.Unmarshal(first.QuerySnapshot, &snapshot) != nil || snapshot.ProfileID != profile.ID || snapshot.Query.Page != 1 || snapshot.SchemaVersion != MiyunQuerySchemaV1 || snapshot.FilterCatalogVersion != MiyunMaterialFilterCatalogVersion || snapshot.MaxPages != DefaultMiyunCrawlMaxPages || len(snapshot.Query.ProductID) != 0 || snapshot.Query.Order != "_score_desc" || snapshot.Query.MType == nil || *snapshot.Query.MType != "201,202" || snapshot.Query.MaterialTag == nil || *snapshot.Query.MaterialTag != "5" {
		t.Fatalf("query snapshot was not frozen: %#v", snapshot)
	}
	if _, err := service.CreateMiyunCrawlJob(context.Background(), miyunTestActor(), "project_1", "crawl-key-too-many-pages", CreateMiyunCrawlJobRequest{ProductProfileID: profile.ID, Operation: "product", MaxPages: DefaultMiyunCrawlMaxPages + 1}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("max page validation error = %v", err)
	}

	draft, err := service.AnalyzeMiyunProductProfile(context.Background(), miyunTestActor(), "project_1", AnalyzeMiyunProductProfileRequest{
		ConnectionID: "miyun_connection_1", ProductID: "product_1", ProductAssetRefs: []contract.AssetVersionRef{}, KnowledgeDocumentIDs: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateMiyunCrawlJob(context.Background(), miyunTestActor(), "project_1", "crawl-key-draft", CreateMiyunCrawlJobRequest{ProductProfileID: draft.ID, Operation: "product"}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("draft profile create error = %v", err)
	}
}

func TestMiyunCrawlResumesPagesAndAppendsSnapshots(t *testing.T) {
	service, repository, runtime, pages := newMiyunCrawlTestService(t)
	profile := confirmedMiyunCrawlTestProfile(t, &service)
	job, err := service.CreateMiyunCrawlJob(context.Background(), miyunTestActor(), "project_1", "crawl-pages", CreateMiyunCrawlJobRequest{ProductProfileID: profile.ID, Operation: "product"})
	if err != nil {
		t.Fatal(err)
	}
	pages.results = []crawler.YouShuPage{
		{Materials: []crawler.YouShuMaterial{miyunCrawlTestMaterial("remote-1")}, Page: 1, Limit: 1, Total: 2},
		{Materials: []crawler.YouShuMaterial{miyunCrawlTestMaterial("remote-1")}, Page: 2, Limit: 1, Total: 2},
	}
	claim := runtime.claim(job.RuntimeJobID)
	if _, err := service.HandleMiyunCrawlJob(context.Background(), claim); !isDeferredMiyunTestError(err) {
		t.Fatalf("first page should defer: %v", err)
	}
	claim = runtime.claim(job.RuntimeJobID)
	if _, err := service.HandleMiyunCrawlJob(context.Background(), claim); err != nil {
		t.Fatal(err)
	}
	stored := repository.jobs[job.ID]
	if stored.Status != MiyunCrawlJobSucceeded || stored.CompletedPages != 2 || stored.DiscoveredCount != 1 || stored.DeduplicatedCount != 1 {
		t.Fatalf("resumed job = %#v", stored)
	}
	if len(repository.materials) != 1 || len(repository.snapshots) != 2 || pages.calls != 2 || pages.requested[0] != 1 || pages.requested[1] != 2 {
		t.Fatalf("material/snapshot/page counts = %d/%d/%d pages=%v", len(repository.materials), len(repository.snapshots), pages.calls, pages.requested)
	}
	secondJob, err := service.CreateMiyunCrawlJob(context.Background(), miyunTestActor(), "project_1", "crawl-pages-second", CreateMiyunCrawlJobRequest{ProductProfileID: profile.ID, Operation: "product"})
	if err != nil {
		t.Fatal(err)
	}
	pages.results = append(pages.results, crawler.YouShuPage{Materials: []crawler.YouShuMaterial{miyunCrawlTestMaterial("remote-1")}, Page: 1, Limit: 1, Total: 1})
	if _, err := service.HandleMiyunCrawlJob(context.Background(), runtime.claim(secondJob.RuntimeJobID)); err != nil {
		t.Fatal(err)
	}
	if len(repository.materials) != 1 || len(repository.snapshots) != 3 {
		t.Fatalf("a second crawl should append a snapshot without duplicating current material: materials=%d snapshots=%d", len(repository.materials), len(repository.snapshots))
	}
}

func TestMiyunCrawlSnapshotPreservesRawAndNormalizedImpressions(t *testing.T) {
	service, _, _, _ := newMiyunCrawlTestService(t)
	record, err := service.miyunCrawlRecord(miyunRuntimePayload{OrganizationID: "org_1", ProjectID: "project_1", ActorID: "user_1"}, MiyunCrawlJob{ID: "crawl_1"}, 1, crawler.YouShuMaterial{
		MaterialID: "remote-units", ImpressionInc2Y: 44_220_000, ImpressionRaw: "4422万",
		Resource: crawler.YouShuResource{ID: "resource-1", URL: "https://cdn.example.test/video.mp4"},
	}, service.now())
	if err != nil {
		t.Fatal(err)
	}
	if record.Snapshot.CumulativeImpressions != 44_220_000 || record.Snapshot.CumulativeImpressionsRaw != "4422万" {
		t.Fatalf("snapshot impressions=%d raw=%q", record.Snapshot.CumulativeImpressions, record.Snapshot.CumulativeImpressionsRaw)
	}
}

func TestMiyunCrawlStopsAtFrozenMaximumPages(t *testing.T) {
	service, repository, runtime, pages := newMiyunCrawlTestService(t)
	profile := confirmedMiyunCrawlTestProfile(t, &service)
	job, err := service.CreateMiyunCrawlJob(context.Background(), miyunTestActor(), "project_1", "crawl-max-pages", CreateMiyunCrawlJobRequest{ProductProfileID: profile.ID, Operation: "product", MaxPages: 2})
	if err != nil {
		t.Fatal(err)
	}
	pages.results = []crawler.YouShuPage{
		{Materials: []crawler.YouShuMaterial{miyunCrawlTestMaterial("remote-max-1")}, Page: 1, Limit: 1, Total: 999},
		{Materials: []crawler.YouShuMaterial{miyunCrawlTestMaterial("remote-max-2")}, Page: 2, Limit: 1, Total: 999},
		{Materials: []crawler.YouShuMaterial{miyunCrawlTestMaterial("remote-max-3")}, Page: 3, Limit: 1, Total: 999},
	}
	if _, err := service.HandleMiyunCrawlJob(context.Background(), runtime.claim(job.RuntimeJobID)); !isDeferredMiyunTestError(err) {
		t.Fatalf("first page should defer: %v", err)
	}
	if _, err := service.HandleMiyunCrawlJob(context.Background(), runtime.claim(job.RuntimeJobID)); err != nil {
		t.Fatal(err)
	}
	stored := repository.jobs[job.ID]
	if stored.Status != MiyunCrawlJobSucceeded || stored.CompletedPages != 2 || pages.calls != 2 {
		t.Fatalf("max-pages job=%#v calls=%d", stored, pages.calls)
	}
}

func TestMiyunCrawlCancelImmediatelyPersistsAndStopsRequests(t *testing.T) {
	service, repository, runtime, pages := newMiyunCrawlTestService(t)
	profile := confirmedMiyunCrawlTestProfile(t, &service)
	job, err := service.CreateMiyunCrawlJob(context.Background(), miyunTestActor(), "project_1", "crawl-cancel-action", CreateMiyunCrawlJobRequest{ProductProfileID: profile.ID, Operation: "product", MaxPages: 20})
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := service.CancelMiyunCrawlJob(context.Background(), miyunTestActor(), "project_1", job.ID, job.Version)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != MiyunCrawlJobCancelled || !runtime.cancelled {
		t.Fatalf("cancelled job=%#v runtime_cancelled=%v", cancelled, runtime.cancelled)
	}
	if _, err := service.HandleMiyunCrawlJob(context.Background(), runtime.claim(job.RuntimeJobID)); err != nil {
		t.Fatal(err)
	}
	if pages.calls != 0 || repository.jobs[job.ID].Status != MiyunCrawlJobCancelled {
		t.Fatalf("cancelled job made %d calls and ended %s", pages.calls, repository.jobs[job.ID].Status)
	}
}

func TestMiyunCrawlCancellationAndUpstreamStops(t *testing.T) {
	for _, test := range []struct {
		name       string
		key        string
		cancelled  bool
		upstream   error
		wantStatus MiyunCrawlJobStatus
		deferred   bool
	}{
		{name: "cancelled", key: "cancelled", cancelled: true, wantStatus: MiyunCrawlJobCancelled},
		{name: "rate limited", key: "rate-limited", upstream: &crawler.YouShuError{Kind: crawler.YouShuRateLimited, Strategy: crawler.YouShuRetry, Source: "graphql", Code: "00:400998"}, wantStatus: MiyunCrawlJobCoolingDown, deferred: true},
		{name: "auth required", key: "auth-required", upstream: &crawler.YouShuError{Kind: crawler.YouShuAuthRequired, Strategy: crawler.YouShuDoNotRetry, Source: "graphql", Code: "00:403005"}, wantStatus: MiyunCrawlJobAuthRequired},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, repository, runtime, pages := newMiyunCrawlTestService(t)
			profile := confirmedMiyunCrawlTestProfile(t, &service)
			job, err := service.CreateMiyunCrawlJob(context.Background(), miyunTestActor(), "project_1", contract.IdempotencyKey("crawl-stop-"+test.key), CreateMiyunCrawlJobRequest{ProductProfileID: profile.ID, Operation: "product"})
			if err != nil {
				t.Fatal(err)
			}
			runtime.cancelled = test.cancelled
			pages.err = test.upstream
			_, err = service.HandleMiyunCrawlJob(context.Background(), runtime.claim(job.RuntimeJobID))
			if test.deferred != isDeferredMiyunTestError(err) {
				t.Fatalf("deferred=%v error=%v", test.deferred, err)
			}
			if repository.jobs[job.ID].Status != test.wantStatus {
				t.Fatalf("status=%s want=%s", repository.jobs[job.ID].Status, test.wantStatus)
			}
			if test.cancelled && pages.calls != 0 {
				t.Fatalf("cancelled job made %d upstream calls", pages.calls)
			}
			if test.wantStatus == MiyunCrawlJobAuthRequired && repository.connection.Status != MiyunConnectionAuthRequired {
				t.Fatalf("connection status=%s", repository.connection.Status)
			}
			if test.wantStatus == MiyunCrawlJobCoolingDown && repository.connection.CooldownUntil == nil {
				t.Fatal("connection cooldown was not persisted")
			}
		})
	}
}

func TestMiyunPendingConfirmedMaterialCanRecoverAMissedEnqueue(t *testing.T) {
	service, repository, runtime, _ := newMiyunCrawlTestService(t)
	now := service.now()
	decisionAt := now
	material := MiyunMaterial{
		ID: "material-pending", OrganizationID: "org_1", ProjectID: "project_1", MiyunMaterialID: "remote-pending",
		FirstSeenCrawlJobID: "crawl-original", ImportMethod: MiyunImportCrawler,
		ResourceURLCiphertext: []byte("encrypted"), ResourceURLKeyVersion: "key-v1", SourceRefStatus: "unknown",
		SelectionStatus: MiyunMaterialConfirmed, ImportStatus: MiyunMaterialImportPending,
		DecisionBy: "operator_1", DecisionAt: &decisionAt, Version: 2, CreatedBy: "operator_1", CreatedAt: now, UpdatedAt: now,
	}
	repository.materials[material.MiyunMaterialID] = material
	got, err := service.RetryMiyunMaterialImport(context.Background(), miyunTestActor(), "project_1", material.ID, material.Version)
	if err != nil || got.ID != material.ID || len(runtime.byKey) != 1 {
		t.Fatalf("pending retry material=%#v runtime_jobs=%d err=%v", got, len(runtime.byKey), err)
	}
}

type miyunCrawlTestRepository struct {
	*memoryMiyunServiceRepository
	jobs      map[string]MiyunCrawlJob
	jobKeys   map[string]string
	materials map[string]MiyunMaterial
	snapshots map[string]MiyunMaterialSnapshot
}

func (r *miyunCrawlTestRepository) UpdateMiyunConnection(_ context.Context, value MiyunConnection, expected int64) (MiyunConnection, error) {
	if r.connection.Version != expected {
		return MiyunConnection{}, ErrVersionConflict
	}
	value.Version = expected + 1
	r.connection = value
	return value, nil
}
func (r *miyunCrawlTestRepository) CreateMiyunCrawlJobIdempotent(_ context.Context, value MiyunCrawlJob) (MiyunCrawlJob, bool, error) {
	key := string(value.OrganizationID) + "/" + string(value.ProjectID) + "/" + value.IdempotencyKey
	if id, ok := r.jobKeys[key]; ok {
		existing := r.jobs[id]
		if existing.RequestHash != value.RequestHash {
			return MiyunCrawlJob{}, false, ErrIdempotencyConflict
		}
		return existing, true, nil
	}
	r.jobs[value.ID], r.jobKeys[key] = value, value.ID
	return value, false, nil
}
func (r *miyunCrawlTestRepository) ListMiyunCrawlJobs(context.Context, contract.OrganizationID, contract.ProjectID, int) ([]MiyunCrawlJob, error) {
	values := make([]MiyunCrawlJob, 0, len(r.jobs))
	for _, value := range r.jobs {
		values = append(values, value)
	}
	return values, nil
}
func (r *miyunCrawlTestRepository) GetMiyunCrawlJob(_ context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (MiyunCrawlJob, error) {
	value, ok := r.jobs[id]
	if !ok || value.OrganizationID != org || value.ProjectID != project {
		return MiyunCrawlJob{}, ErrNotFound
	}
	return value, nil
}
func (r *miyunCrawlTestRepository) UpdateMiyunCrawlJob(_ context.Context, value MiyunCrawlJob, expected int64) (MiyunCrawlJob, error) {
	current, ok := r.jobs[value.ID]
	if !ok || current.Version != expected {
		return MiyunCrawlJob{}, ErrVersionConflict
	}
	value.Version = expected + 1
	r.jobs[value.ID] = value
	return value, nil
}
func (r *miyunCrawlTestRepository) UpdateMiyunCrawlJobAndConnection(ctx context.Context, job MiyunCrawlJob, jobExpected int64, connection MiyunConnection, connectionExpected int64) (MiyunCrawlJob, MiyunConnection, error) {
	updatedJob, err := r.UpdateMiyunCrawlJob(ctx, job, jobExpected)
	if err != nil {
		return MiyunCrawlJob{}, MiyunConnection{}, err
	}
	updatedConnection, err := r.UpdateMiyunConnection(ctx, connection, connectionExpected)
	if err != nil {
		return MiyunCrawlJob{}, MiyunConnection{}, err
	}
	return updatedJob, updatedConnection, nil
}
func (r *miyunCrawlTestRepository) ApplyMiyunCrawlPage(_ context.Context, job MiyunCrawlJob, page int64, records []MiyunCrawlPageRecord, finished bool) (MiyunCrawlJob, error) {
	current := r.jobs[job.ID]
	if current.CompletedPages >= page {
		return current, nil
	}
	var discovered, deduplicated int64
	for _, record := range records {
		material := record.Material
		if existing, ok := r.materials[material.MiyunMaterialID]; ok {
			material.ID = existing.ID
			deduplicated++
		} else {
			r.materials[material.MiyunMaterialID] = material
			discovered++
		}
		snapshot := record.Snapshot
		snapshot.MaterialID = material.ID
		r.snapshots[snapshot.ID] = snapshot
	}
	current.CompletedPages, current.DiscoveredCount = page, current.DiscoveredCount+discovered
	current.DeduplicatedCount += deduplicated
	current.Status = MiyunCrawlJobRunning
	if finished {
		current.Status = MiyunCrawlJobSucceeded
	}
	current.Version++
	r.jobs[current.ID] = current
	return current, nil
}
func (r *miyunCrawlTestRepository) ListMiyunMaterials(context.Context, contract.OrganizationID, contract.ProjectID, MiyunMaterialListOptions) (MiyunMaterialListPage, error) {
	return MiyunMaterialListPage{}, nil
}
func (r *miyunCrawlTestRepository) GetMiyunMaterial(_ context.Context, org contract.OrganizationID, project contract.ProjectID, id string) (MiyunMaterial, error) {
	for _, material := range r.materials {
		if material.ID == id && material.OrganizationID == org && material.ProjectID == project {
			return material, nil
		}
	}
	return MiyunMaterial{}, ErrNotFound
}
func (r *miyunCrawlTestRepository) DecideMiyunMaterial(context.Context, MiyunMaterial, int64) (MiyunMaterial, error) {
	return MiyunMaterial{}, errors.New("unused")
}
func (r *miyunCrawlTestRepository) MarkMiyunMaterialImporting(context.Context, MiyunMaterial, int64, string) (MiyunMaterial, error) {
	return MiyunMaterial{}, errors.New("unused")
}
func (r *miyunCrawlTestRepository) CompleteMiyunMaterialImport(context.Context, MiyunMaterialImportCompletion) (MiyunMaterial, error) {
	return MiyunMaterial{}, errors.New("unused")
}
func (r *miyunCrawlTestRepository) FailMiyunMaterialImport(context.Context, MiyunMaterial, int64, string, string) (MiyunMaterial, error) {
	return MiyunMaterial{}, errors.New("unused")
}

type miyunRuntimeTestStore struct {
	byKey     map[string]jobruntime.CreateRequest
	byID      map[string]jobruntime.CreateRequest
	cancelled bool
}

func (s *miyunRuntimeTestStore) Enqueue(_ context.Context, request jobruntime.CreateRequest) (contract.Job, bool, error) {
	if err := request.Validate(); err != nil {
		return contract.Job{}, false, err
	}
	key := string(request.Job.OrganizationID) + "/" + string(request.Job.ProjectID) + "/" + string(request.IdempotencyKey)
	if existing, ok := s.byKey[key]; ok {
		if existing.RequestHash != request.RequestHash {
			return contract.Job{}, false, jobruntime.ErrIdempotencyConflict
		}
		return existing.Job, true, nil
	}
	s.byKey[key], s.byID[request.Job.ID] = request, request
	return request.Job, false, nil
}
func (s *miyunRuntimeTestStore) Get(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, id string) (contract.Job, error) {
	return s.byID[id].Job, nil
}
func (s *miyunRuntimeTestStore) RequestCancel(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, id string, _ int64, _ time.Time) (contract.Job, error) {
	s.cancelled = true
	return s.byID[id].Job, nil
}
func (s *miyunRuntimeTestStore) IsCancelRequested(context.Context, contract.OrganizationID, string) (bool, error) {
	return s.cancelled, nil
}
func (s *miyunRuntimeTestStore) claim(id string) jobruntime.Claim {
	request := s.byID[id]
	return jobruntime.Claim{Job: request.Job, Payload: request.Payload, LockOwner: "test-worker"}
}

type miyunPageTestClient struct {
	results   []crawler.YouShuPage
	err       error
	calls     int
	requested []int
}

type miyunVerifierTestDouble struct{ err error }

func (v miyunVerifierTestDouble) VerifyMiyunConnection(context.Context, []byte) error { return v.err }

func (c *miyunPageTestClient) FetchMiyunPage(_ context.Context, _ MiyunConnection, _ string, query crawler.YouShuQuery) (crawler.YouShuPage, error) {
	c.calls++
	c.requested = append(c.requested, query.Page)
	if c.err != nil {
		return crawler.YouShuPage{}, c.err
	}
	return c.results[c.calls-1], nil
}

type miyunCipherTestDouble struct{}

func (miyunCipherTestDouble) Encrypt(value []byte) ([]byte, string, error) {
	return append([]byte("cipher:"), value...), "key-v1", nil
}
func (miyunCipherTestDouble) Decrypt(value []byte, _ string) ([]byte, error) {
	return append([]byte(nil), value...), nil
}

func newMiyunCrawlTestService(t *testing.T) (Service, *miyunCrawlTestRepository, *miyunRuntimeTestStore, *miyunPageTestClient) {
	t.Helper()
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	base := newMiyunServiceRepository(now)
	repository := &miyunCrawlTestRepository{memoryMiyunServiceRepository: base, jobs: map[string]MiyunCrawlJob{}, jobKeys: map[string]string{}, materials: map[string]MiyunMaterial{}, snapshots: map[string]MiyunMaterialSnapshot{}}
	runtime := &miyunRuntimeTestStore{byKey: map[string]jobruntime.CreateRequest{}, byID: map[string]jobruntime.CreateRequest{}}
	pages := &miyunPageTestClient{}
	service := newMiyunTestService(base, now)
	service.Miyun, service.MiyunCrawl, service.MiyunJobs, service.MiyunPages, service.MiyunSecrets = repository, repository, runtime, pages, miyunCipherTestDouble{}
	return service, repository, runtime, pages
}

func confirmedMiyunCrawlTestProfile(t *testing.T, service *Service) MiyunProductProfile {
	t.Helper()
	draft, err := service.AnalyzeMiyunProductProfile(context.Background(), miyunTestActor(), "project_1", AnalyzeMiyunProductProfileRequest{
		ConnectionID: "miyun_connection_1", ProductID: "product_1", ProductAssetRefs: []contract.AssetVersionRef{}, KnowledgeDocumentIDs: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	profile, err := service.ConfirmMiyunProductProfile(context.Background(), miyunTestActor(), "project_1", draft.ID, ConfirmMiyunProductProfileRequest{ExpectedVersion: draft.Version, Query: MiyunProfileQuery{
		ProductName: "Test product", CategoryID: "category-1", CategoryName: "Category", Keywords: []string{"test"}, MaterialTypes: []string{"视频", "vertical_video"}, MaterialContentTypes: []string{"product_demo"}, WindowStart: service.now().AddDate(0, 0, -7), WindowEnd: service.now(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func miyunCrawlTestMaterial(id string) crawler.YouShuMaterial {
	return crawler.YouShuMaterial{MaterialID: id, MaterialType: "video", Resource: crawler.YouShuResource{ID: "resource-" + id, URL: "https://cdn.example.test/" + id + ".mp4", Size: 28}, Slogan: "test"}
}

func isDeferredMiyunTestError(err error) bool {
	var deferred jobruntime.DeferredError
	return errors.As(err, &deferred)
}
