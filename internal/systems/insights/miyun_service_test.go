package insights

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestMiyunAnalyzeIsDeterministicExplainableAndSupersedesDraft(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	repository := newMiyunServiceRepository(now)
	service := newMiyunTestService(repository, now)
	request := AnalyzeMiyunProductProfileRequest{
		ConnectionID: "miyun_connection_1", ProductID: "product_1",
		ProductAssetRefs:     []contract.AssetVersionRef{{AssetID: "asset_1", Version: 1}},
		KnowledgeDocumentIDs: []string{"document_1"},
	}
	first, err := service.AnalyzeMiyunProductProfile(context.Background(), miyunTestActor(), "project_1", request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.AnalyzeMiyunProductProfile(context.Background(), miyunTestActor(), "project_1", request)
	if err != nil {
		t.Fatal(err)
	}
	if first.InputHash != second.InputHash || first.ProductName != second.ProductName ||
		!reflect.DeepEqual(first.Keywords, second.Keywords) || !reflect.DeepEqual(first.MaterialContentTypes, second.MaterialContentTypes) {
		t.Fatalf("same frozen input produced different drafts:\nfirst=%#v\nsecond=%#v", first, second)
	}
	if repository.profiles[first.ID].Status != MiyunProfileSuperseded || repository.profiles[first.ID].Version != 2 || second.Status != MiyunProfileDraft {
		t.Fatalf("old draft=%s new=%s", repository.profiles[first.ID].Status, second.Status)
	}
	if second.AnalysisMethod != "rules" || second.ModelVersion != "" || !containsString(second.AnalysisWarnings, "model_not_used:deterministic_rules") {
		t.Fatalf("rule lineage is not explicit: %#v", second)
	}
	if len(second.FieldSources) != 5 || second.FieldSources[0].ReviewState != "suggested" || len(second.FieldSources[0].SourceRefs) == 0 {
		t.Fatalf("field sources=%#v", second.FieldSources)
	}
	projectReader := service.MiyunProjects.(fakeMiyunProjectReader)
	projectReader.source.Context.ProjectContextVersion++
	service.MiyunProjects = projectReader
	changedContext, err := service.AnalyzeMiyunProductProfile(context.Background(), miyunTestActor(), "project_1", request)
	if err != nil {
		t.Fatal(err)
	}
	if changedContext.InputHash == second.InputHash {
		t.Fatal("context version changed without changing the frozen input hash")
	}
}

func TestMiyunConfirmUsesExpectedVersionAndFreezesConfirmedProfile(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	repository := newMiyunServiceRepository(now)
	service := newMiyunTestService(repository, now)
	draft, err := service.AnalyzeMiyunProductProfile(context.Background(), miyunTestActor(), "project_1", AnalyzeMiyunProductProfileRequest{
		ConnectionID: "miyun_connection_1", ProductID: "product_1", ProductAssetRefs: []contract.AssetVersionRef{}, KnowledgeDocumentIDs: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	query := MiyunProfileQuery{
		ProductName: "Operator product", CategoryID: "cid_1", CategoryName: "Drinkware",
		Keywords: []string{"operator keyword"}, MaterialContentTypes: []string{"商品展示"},
		WindowStart: now.AddDate(0, 0, -6), WindowEnd: now,
	}
	if _, err := service.ConfirmMiyunProductProfile(context.Background(), miyunTestActor(), "project_1", draft.ID, ConfirmMiyunProductProfileRequest{ExpectedVersion: draft.Version + 1, Query: query}); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale confirmation error=%v", err)
	}
	confirmed, err := service.ConfirmMiyunProductProfile(context.Background(), miyunTestActor(), "project_1", draft.ID, ConfirmMiyunProductProfileRequest{ExpectedVersion: draft.Version, Query: query})
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Status != MiyunProfileConfirmed || confirmed.Version != 2 || confirmed.ProductName != "Operator product" || confirmed.FieldSources[0].ReviewState != "human_confirmed" {
		t.Fatalf("confirmed=%#v", confirmed)
	}
	if _, err := service.ConfirmMiyunProductProfile(context.Background(), miyunTestActor(), "project_1", draft.ID, ConfirmMiyunProductProfileRequest{ExpectedVersion: confirmed.Version, Query: query}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("confirmed profile changed in place: %v", err)
	}
}

func TestMiyunManualImportReferencesExistingMP4AndReplaysIdempotently(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	repository := newMiyunServiceRepository(now)
	service := newMiyunTestService(repository, now)
	request := ManualMiyunMaterialRequest{
		AssetRef:        contract.AssetVersionRef{AssetID: "asset_1", Version: 1},
		MiyunMaterialID: "remote_1", SourceRef: "https://example.test/miyun/material/remote_1", Title: "Manual material",
		DataCard: ManualMiyunDataCard{
			SchemaVersion: MiyunDataCardSchemaV1, CapturedAt: now,
			CumulativeImpressionsRaw: "1.2万", CumulativeImpressions: 12000, RelatedAds: 5,
			SourceFields: []byte(`{"fixture_version":"1"}`),
		},
	}
	first, err := service.ManualImportMiyunMaterial(context.Background(), miyunTestActor(), "project_1", "manual-key-1", request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.ManualImportMiyunMaterial(context.Background(), miyunTestActor(), "project_1", "manual-key-1", request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Material.ID != second.Material.ID || !second.Replayed || first.Material.ImportMethod != MiyunImportManual ||
		first.Material.FirstSeenCrawlJobID != "" || first.Snapshot.CrawlJobID != "" ||
		first.Snapshot.CumulativeImpressionsRaw != "1.2万" || first.InsightAsset.SourceKind != AssetSourceExternal {
		t.Fatalf("manual replay/result mismatch: first=%#v second=%#v", first, second)
	}
	request.Title = "different request"
	if _, err := service.ManualImportMiyunMaterial(context.Background(), miyunTestActor(), "project_1", "manual-key-1", request); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("idempotency conflict=%v", err)
	}

	assetReader := service.MiyunAssets.(*fakeMiyunAssetReader)
	assetReader.source.MIMEType = "image/png"
	request.Title = "new request"
	if _, err := service.ManualImportMiyunMaterial(context.Background(), miyunTestActor(), "project_1", "manual-key-2", request); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("non-MP4 manual import=%v", err)
	}
}

func TestMiyunAnalysisBoundsAndManualSourceRefRejectsSecrets(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	repository := newMiyunServiceRepository(now)
	service := newMiyunTestService(repository, now)
	documentIDs := make([]string, maxMiyunAnalysisSources+1)
	for index := range documentIDs {
		documentIDs[index] = fmt.Sprintf("document_%d", index)
	}
	_, err := service.AnalyzeMiyunProductProfile(context.Background(), miyunTestActor(), "project_1", AnalyzeMiyunProductProfileRequest{
		ConnectionID: "miyun_connection_1", ProductID: "product_1",
		ProductAssetRefs: []contract.AssetVersionRef{}, KnowledgeDocumentIDs: documentIDs,
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("unbounded analysis sources error=%v", err)
	}

	request := ManualMiyunMaterialRequest{
		AssetRef: contract.AssetVersionRef{AssetID: "asset_1", Version: 1}, MiyunMaterialID: "remote_secret",
		SourceRef: "https://example.test/material?id=1&session_token=secret",
		DataCard:  ManualMiyunDataCard{SchemaVersion: MiyunDataCardSchemaV1, CapturedAt: now, CumulativeImpressionsRaw: "0"},
	}
	if _, err := service.ManualImportMiyunMaterial(context.Background(), miyunTestActor(), "project_1", "manual-secret", request); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("secret-bearing source URL error=%v", err)
	}
}

func TestMiyunManualImportRequiresRemoteIdentityAndSource(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	base := ManualMiyunMaterialRequest{
		AssetRef: contract.AssetVersionRef{AssetID: "asset_1", Version: 1}, MiyunMaterialID: "remote_1",
		SourceRef: "https://example.test/material/1",
		DataCard:  ManualMiyunDataCard{SchemaVersion: MiyunDataCardSchemaV1, CapturedAt: now, CumulativeImpressionsRaw: "0"},
	}
	for _, mutate := range []func(*ManualMiyunMaterialRequest){
		func(request *ManualMiyunMaterialRequest) { request.MiyunMaterialID = "" },
		func(request *ManualMiyunMaterialRequest) { request.SourceRef = "" },
	} {
		request := base
		mutate(&request)
		service := newMiyunTestService(newMiyunServiceRepository(now), now)
		if _, err := service.ManualImportMiyunMaterial(context.Background(), miyunTestActor(), "project_1", "required-fields", request); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("missing manual identity/source error=%v", err)
		}
	}
}

type memoryMiyunServiceRepository struct {
	connection MiyunConnection
	profiles   map[string]MiyunProductProfile
	manual     map[string]MiyunManualImportResult
}

func newMiyunServiceRepository(now time.Time) *memoryMiyunServiceRepository {
	connection := validMiyunConnection(now)
	connection.Status = MiyunConnectionReady
	return &memoryMiyunServiceRepository{
		connection: connection, profiles: map[string]MiyunProductProfile{}, manual: map[string]MiyunManualImportResult{},
	}
}

func (r *memoryMiyunServiceRepository) CreateMiyunProductProfileDraft(_ context.Context, value MiyunProductProfile) (MiyunProductProfile, error) {
	for id, existing := range r.profiles {
		if existing.ProjectID == value.ProjectID && existing.ProductID == value.ProductID && existing.Status == MiyunProfileDraft {
			existing.Status, existing.Version = MiyunProfileSuperseded, existing.Version+1
			r.profiles[id] = existing
		}
	}
	r.profiles[value.ID] = value
	return value, nil
}

func (r *memoryMiyunServiceRepository) GetMiyunProductProfile(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (MiyunProductProfile, error) {
	value, ok := r.profiles[id]
	if !ok || value.OrganizationID != organizationID || value.ProjectID != projectID {
		return MiyunProductProfile{}, ErrNotFound
	}
	return value, nil
}

func (r *memoryMiyunServiceRepository) ListMiyunProductProfiles(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, _ int) ([]MiyunProductProfile, error) {
	result := []MiyunProductProfile{}
	for _, value := range r.profiles {
		if value.OrganizationID == organizationID && value.ProjectID == projectID {
			result = append(result, value)
		}
	}
	return result, nil
}

func (r *memoryMiyunServiceRepository) ConfirmMiyunProductProfile(_ context.Context, value MiyunProductProfile, expectedVersion int64) (MiyunProductProfile, error) {
	current, ok := r.profiles[value.ID]
	if !ok {
		return MiyunProductProfile{}, ErrNotFound
	}
	if current.Status != MiyunProfileDraft {
		return MiyunProductProfile{}, ErrInvalidState
	}
	if current.Version != expectedVersion {
		return MiyunProductProfile{}, ErrVersionConflict
	}
	value.Version = expectedVersion + 1
	r.profiles[value.ID] = value
	return value, nil
}

func (r *memoryMiyunServiceRepository) CreateManualMiyunMaterial(_ context.Context, record MiyunManualImportRecord) (MiyunManualImportResult, error) {
	key := record.Material.ManualIdempotencyKey
	if existing, ok := r.manual[key]; ok {
		if existing.Material.ManualRequestHash != record.Material.ManualRequestHash {
			return MiyunManualImportResult{}, ErrIdempotencyConflict
		}
		existing.Replayed = true
		return existing, nil
	}
	result := MiyunManualImportResult{Material: record.Material, Snapshot: record.Snapshot, InsightAsset: record.InsightAsset}
	r.manual[key] = result
	return result, nil
}

func (r *memoryMiyunServiceRepository) GetMiyunConnection(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (MiyunConnection, error) {
	if r.connection.OrganizationID != organizationID || r.connection.ProjectID != projectID || r.connection.ID != id {
		return MiyunConnection{}, ErrNotFound
	}
	return r.connection, nil
}

func (r *memoryMiyunServiceRepository) GetProjectMiyunConnection(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) (MiyunConnection, error) {
	if r.connection.OrganizationID != organizationID || r.connection.ProjectID != projectID {
		return MiyunConnection{}, ErrNotFound
	}
	return r.connection, nil
}

func (r *memoryMiyunServiceRepository) CreateMiyunConnection(context.Context, MiyunConnection) (MiyunConnection, error) {
	return MiyunConnection{}, errors.New("unused")
}
func (r *memoryMiyunServiceRepository) UpdateMiyunConnection(context.Context, MiyunConnection, int64) (MiyunConnection, error) {
	return MiyunConnection{}, errors.New("unused")
}
func (r *memoryMiyunServiceRepository) CreateMiyunProductProfile(context.Context, MiyunProductProfile) (MiyunProductProfile, error) {
	return MiyunProductProfile{}, errors.New("unused")
}
func (r *memoryMiyunServiceRepository) CreateMiyunCrawlJob(context.Context, MiyunCrawlJob) (MiyunCrawlJob, error) {
	return MiyunCrawlJob{}, errors.New("unused")
}
func (r *memoryMiyunServiceRepository) GetMiyunCrawlJob(context.Context, contract.OrganizationID, contract.ProjectID, string) (MiyunCrawlJob, error) {
	return MiyunCrawlJob{}, errors.New("unused")
}
func (r *memoryMiyunServiceRepository) CreateMiyunMaterial(context.Context, MiyunMaterial) (MiyunMaterial, error) {
	return MiyunMaterial{}, errors.New("unused")
}
func (r *memoryMiyunServiceRepository) GetMiyunMaterial(context.Context, contract.OrganizationID, contract.ProjectID, string) (MiyunMaterial, error) {
	return MiyunMaterial{}, errors.New("unused")
}
func (r *memoryMiyunServiceRepository) AppendMiyunMaterialSnapshot(context.Context, MiyunMaterialSnapshot) (MiyunMaterialSnapshot, error) {
	return MiyunMaterialSnapshot{}, errors.New("unused")
}
func (r *memoryMiyunServiceRepository) ListMiyunMaterialSnapshots(context.Context, contract.OrganizationID, contract.ProjectID, string) ([]MiyunMaterialSnapshot, error) {
	return nil, errors.New("unused")
}
func (r *memoryMiyunServiceRepository) CreateMiyunHandoff(context.Context, MiyunHandoff) (MiyunHandoff, error) {
	return MiyunHandoff{}, errors.New("unused")
}
func (r *memoryMiyunServiceRepository) GetMiyunHandoff(context.Context, contract.OrganizationID, contract.ProjectID, string) (MiyunHandoff, error) {
	return MiyunHandoff{}, errors.New("unused")
}

type fakeMiyunProjectReader struct{ source MiyunProjectSource }

func (r fakeMiyunProjectReader) ReadMiyunProjectSource(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID) (MiyunProjectSource, error) {
	if actor.OrganizationID != r.source.Context.OrganizationID || projectID != r.source.Context.ProjectID {
		return MiyunProjectSource{}, ErrNotFound
	}
	return r.source, nil
}

type fakeMiyunAssetReader struct{ source MiyunAssetSource }

func (r *fakeMiyunAssetReader) ReadMiyunAssetSource(_ context.Context, _ contract.ActorContext, projectID contract.ProjectID, ref contract.AssetVersionRef) (MiyunAssetSource, error) {
	if projectID != "project_1" || ref != r.source.Ref {
		return MiyunAssetSource{}, ErrNotFound
	}
	return r.source, nil
}

type fakeMiyunKnowledgeReader struct{ source MiyunKnowledgeSource }

func (r fakeMiyunKnowledgeReader) ReadMiyunKnowledgeSource(_ context.Context, _ contract.ActorContext, projectID contract.ProjectID, id string) (MiyunKnowledgeSource, error) {
	if projectID != "project_1" || id != r.source.ID {
		return MiyunKnowledgeSource{}, ErrNotFound
	}
	return r.source, nil
}

type fakeMiyunMediaReader struct{ evidence MiyunMediaEvidence }

func (r fakeMiyunMediaReader) ReadLatestMiyunMediaEvidence(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, _ contract.AssetVersionRef) (MiyunMediaEvidence, bool, error) {
	return r.evidence, true, nil
}

func newMiyunTestService(repository *memoryMiyunServiceRepository, now time.Time) Service {
	sequence := 0
	return Service{
		Miyun: repository,
		MiyunProjects: fakeMiyunProjectReader{source: MiyunProjectSource{
			Context:     contract.ProjectContext{OrganizationID: "org_1", ProjectID: "project_1", BrandID: miyunBrandID("brand_1"), ProductIDs: []contract.ProductID{"product_1"}, ProjectContextVersion: 7},
			ProjectName: "Campaign", BrandName: "Thermos", CategoryName: "Drinkware",
			Products: []MiyunProjectProduct{{ID: "product_1", Name: "Insulated cup"}},
		}},
		MiyunAssets:    &fakeMiyunAssetReader{source: MiyunAssetSource{Ref: contract.AssetVersionRef{AssetID: "asset_1", Version: 1}, Kind: contract.AssetVideo, MIMEType: "video/mp4", SHA256: fmt.Sprintf("%064d", 1), Ready: true}},
		MiyunKnowledge: fakeMiyunKnowledgeReader{source: MiyunKnowledgeSource{ID: "document_1", Filename: "brief.txt", MIMEType: "text/plain", Status: "ready", Text: "商品展示 轻量保温", TextSHA256: fmt.Sprintf("%064d", 2)}},
		MiyunMedia:     fakeMiyunMediaReader{evidence: MiyunMediaEvidence{ArtifactID: "evidence_1", Status: "partial", ContentHash: fmt.Sprintf("%064d", 3), Evidence: []string{"商品展示"}}},
		Now:            func() time.Time { return now },
		NewID:          func(prefix string) (string, error) { sequence++; return fmt.Sprintf("%s_%d", prefix, sequence), nil },
	}
}

func miyunTestActor() contract.ActorContext {
	return contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "operator_1"}, Scopes: []contract.Scope{ScopeRead, ScopeWrite, ScopeConfirm}}
}

func miyunBrandID(value contract.BrandID) *contract.BrandID { return &value }
