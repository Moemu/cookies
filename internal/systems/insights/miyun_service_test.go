package insights

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
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

func TestUpdateMiyunConnectionExplainsConfigurationAndCookieBounds(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	service := newMiyunTestService(newMiyunServiceRepository(now), now)
	request := UpdateMiyunConnectionRequest{Session: "cookie=value", ExpectedVersion: 0}
	if _, err := service.UpdateMiyunConnection(context.Background(), miyunTestActor(), "project_1", request); !errors.Is(err, ErrInvalidState) || !strings.Contains(err.Error(), "会话加密") {
		t.Fatalf("missing server encryption error=%v", err)
	}

	service.MiyunSecrets = miyunCipherTestDouble{}
	request.Session = "short"
	if _, err := service.UpdateMiyunConnection(context.Background(), miyunTestActor(), "project_1", request); !errors.Is(err, ErrInvalidRequest) || !strings.Contains(err.Error(), "Cookie 值不完整") {
		t.Fatalf("short cookie error=%v", err)
	}
	request.Session = strings.Repeat("x", maxMiyunSessionBytes+1)
	if _, err := service.UpdateMiyunConnection(context.Background(), miyunTestActor(), "project_1", request); !errors.Is(err, ErrInvalidRequest) || !strings.Contains(err.Error(), "超过 16 KiB") {
		t.Fatalf("oversized cookie error=%v", err)
	}
}

func TestMiyunAnalyzeCreatesPendingIdentityWhenProjectHasNoRegisteredProduct(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	repository := newMiyunServiceRepository(now)
	service := newMiyunTestService(repository, now)
	projectReader := service.MiyunProjects.(fakeMiyunProjectReader)
	projectReader.source.Context.ProductIDs = []contract.ProductID{}
	projectReader.source.Products = []MiyunProjectProduct{}
	service.MiyunProjects = projectReader

	draft, err := service.AnalyzeMiyunProductProfile(context.Background(), miyunTestActor(), "project_1", AnalyzeMiyunProductProfileRequest{
		ConnectionID: "miyun_connection_1", ProductName: "手冲咖啡套装", CategoryName: "咖啡器具",
		ProductAssetRefs: []contract.AssetVersionRef{}, KnowledgeDocumentIDs: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(draft.ProductID), "project_input:") || draft.ProductName != "手冲咖啡套装" || draft.CategoryName != "咖啡器具" || !containsString(draft.AnalysisWarnings, "product_identity_pending_confirmation") {
		t.Fatalf("pending product identity was not explicit: %#v", draft)
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
	materials  map[string]MiyunMaterial
	snapshots  map[string][]MiyunMaterialSnapshot
	handoffs   map[string]MiyunHandoff
}

func newMiyunServiceRepository(now time.Time) *memoryMiyunServiceRepository {
	connection := validMiyunConnection(now)
	connection.Status = MiyunConnectionReady
	return &memoryMiyunServiceRepository{
		connection: connection, profiles: map[string]MiyunProductProfile{}, manual: map[string]MiyunManualImportResult{}, materials: map[string]MiyunMaterial{}, snapshots: map[string][]MiyunMaterialSnapshot{}, handoffs: map[string]MiyunHandoff{},
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
func (r *memoryMiyunServiceRepository) GetMiyunMaterial(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (MiyunMaterial, error) {
	value, ok := r.materials[id]
	if !ok || value.OrganizationID != organizationID || value.ProjectID != projectID {
		return MiyunMaterial{}, ErrNotFound
	}
	return value, nil
}
func (r *memoryMiyunServiceRepository) AppendMiyunMaterialSnapshot(context.Context, MiyunMaterialSnapshot) (MiyunMaterialSnapshot, error) {
	return MiyunMaterialSnapshot{}, errors.New("unused")
}
func (r *memoryMiyunServiceRepository) ListMiyunMaterialSnapshots(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, materialID string) ([]MiyunMaterialSnapshot, error) {
	material, err := r.GetMiyunMaterial(context.Background(), organizationID, projectID, materialID)
	if err != nil {
		return nil, err
	}
	return append([]MiyunMaterialSnapshot(nil), r.snapshots[material.ID]...), nil
}
func (r *memoryMiyunServiceRepository) CreateMiyunHandoff(_ context.Context, value MiyunHandoff) (MiyunHandoff, error) {
	for _, existing := range r.handoffs {
		if existing.OrganizationID == value.OrganizationID && existing.ProjectID == value.ProjectID && existing.InputHash == value.InputHash {
			return MiyunHandoff{}, ErrIdempotencyConflict
		}
	}
	r.handoffs[value.ID] = value
	return value, nil
}
func (r *memoryMiyunServiceRepository) GetMiyunHandoff(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (MiyunHandoff, error) {
	value, ok := r.handoffs[id]
	if !ok || value.OrganizationID != organizationID || value.ProjectID != projectID {
		return MiyunHandoff{}, ErrNotFound
	}
	return value, nil
}
func (r *memoryMiyunServiceRepository) ListMiyunHandoffs(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, _ int) ([]MiyunHandoff, error) {
	result := make([]MiyunHandoff, 0, len(r.handoffs))
	for _, value := range r.handoffs {
		if value.OrganizationID == organizationID && value.ProjectID == projectID {
			result = append(result, value)
		}
	}
	return result, nil
}
func (r *memoryMiyunServiceRepository) FindMiyunHandoffByInputHash(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, inputHash string) (MiyunHandoff, error) {
	for _, value := range r.handoffs {
		if value.OrganizationID == organizationID && value.ProjectID == projectID && value.InputHash == inputHash {
			return value, nil
		}
	}
	return MiyunHandoff{}, ErrNotFound
}
func (r *memoryMiyunServiceRepository) UpdateMiyunHandoffStatus(_ context.Context, value MiyunHandoff, expectedVersion int64) (MiyunHandoff, error) {
	current, ok := r.handoffs[value.ID]
	if !ok {
		return MiyunHandoff{}, ErrNotFound
	}
	if current.Version != expectedVersion {
		return MiyunHandoff{}, ErrVersionConflict
	}
	value.Version = expectedVersion + 1
	r.handoffs[value.ID] = value
	return value, nil
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

func TestMiyunHandoffCreateFreezesInputAndReplays(t *testing.T) {
	now := time.Date(2026, 8, 11, 1, 0, 0, 0, time.UTC)
	repository := newMiyunServiceRepository(now)
	service := newMiyunTestService(repository, now)
	asset := service.MiyunAssets.(*fakeMiyunAssetReader)
	asset.source.SHA256 = miyunHandoffTestHash([]byte("video"))
	material := validMiyunMaterial(now)
	material.SelectionStatus, material.ImportStatus = MiyunMaterialConfirmed, MiyunMaterialImportImported
	material.PlatformAssetID, material.PlatformAssetVersion = "asset_1", 1
	repository.materials[material.ID] = material
	snapshot := validMiyunMaterialSnapshot(now)
	snapshot.MaterialID = material.ID
	repository.snapshots[material.ID] = []MiyunMaterialSnapshot{snapshot}
	profile := validMiyunProductProfile(now)
	profile.Status, profile.Version = MiyunProfileConfirmed, 2
	profile.ProductAssetRefs = []contract.AssetVersionRef{{AssetID: "asset_1", Version: 1}}
	profile.KnowledgeDocumentIDs = []string{}
	repository.profiles[profile.ID] = profile

	first, err := service.CreateMiyunHandoff(context.Background(), miyunTestActor(), "project_1", CreateMiyunHandoffRequest{SourceMaterialIDs: []string{material.ID}, ProductProfileID: profile.ID})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateMiyunHandoff(context.Background(), miyunTestActor(), "project_1", CreateMiyunHandoffRequest{SourceMaterialIDs: []string{material.ID}, ProductProfileID: profile.ID})
	if err != nil || first.ID != second.ID || first.Status != MiyunHandoffExporting {
		t.Fatalf("idempotent create = %#v, %#v, %v", first, second, err)
	}
	frozen := append([]byte(nil), first.ProfileSnapshot...)
	profile.ProductName = "Changed after handoff"
	repository.profiles[profile.ID] = profile
	got, err := service.GetMiyunHandoff(context.Background(), miyunTestActor(), "project_1", first.ID)
	if err != nil || !bytes.Equal(got.ProfileSnapshot, frozen) || got.InputHash != first.InputHash {
		t.Fatalf("handoff was not frozen: %#v, %v", got, err)
	}
	material.SelectionStatus = MiyunMaterialDiscovered
	repository.materials[material.ID] = material
	if _, err := service.CreateMiyunHandoff(context.Background(), miyunTestActor(), "project_1", CreateMiyunHandoffRequest{SourceMaterialIDs: []string{material.ID}, ProductProfileID: profile.ID}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("unconfirmed material create error=%v", err)
	}
}

func TestMiyunHandoffFreezesMultipleSourcesInStableOrder(t *testing.T) {
	now := time.Date(2026, 8, 11, 1, 30, 0, 0, time.UTC)
	repository := newMiyunServiceRepository(now)
	service := newMiyunTestService(repository, now)
	asset := service.MiyunAssets.(*fakeMiyunAssetReader)
	asset.source.SHA256 = miyunHandoffTestHash([]byte("video"))
	profile := validMiyunProductProfile(now)
	profile.Status = MiyunProfileConfirmed
	profile.ProductAssetRefs = []contract.AssetVersionRef{{AssetID: "asset_1", Version: 1}}
	profile.KnowledgeDocumentIDs = []string{}
	repository.profiles[profile.ID] = profile
	first := validMiyunMaterial(now)
	first.ID, first.MiyunMaterialID = "miyunmaterial_b", "remote-b"
	first.SelectionStatus, first.ImportStatus = MiyunMaterialConfirmed, MiyunMaterialImportImported
	first.PlatformAssetID, first.PlatformAssetVersion = "asset_1", 1
	second := first
	second.ID, second.MiyunMaterialID = "miyunmaterial_a", "remote-a"
	for _, material := range []MiyunMaterial{first, second} {
		repository.materials[material.ID] = material
		snapshot := validMiyunMaterialSnapshot(now)
		snapshot.MaterialID = material.ID
		repository.snapshots[material.ID] = []MiyunMaterialSnapshot{snapshot}
	}
	created, err := service.CreateMiyunHandoff(context.Background(), miyunTestActor(), "project_1", CreateMiyunHandoffRequest{SourceMaterialIDs: []string{first.ID, second.ID}, ProductProfileID: profile.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := created.SourceMaterialIDs, []string{second.ID, first.ID}; !reflect.DeepEqual(got, want) || created.SourceMaterialID != second.ID {
		t.Fatalf("frozen sources = %#v, primary=%s", got, created.SourceMaterialID)
	}
	var frozen miyunHandoffSourcesSnapshot
	if err := json.Unmarshal(created.SourceSnapshot, &frozen); err != nil || len(frozen.Sources) != 2 || frozen.Sources[0].Material.ID != second.ID {
		t.Fatalf("frozen source snapshot = %#v, %v", frozen, err)
	}
	replayed, err := service.CreateMiyunHandoff(context.Background(), miyunTestActor(), "project_1", CreateMiyunHandoffRequest{SourceMaterialIDs: []string{second.ID, first.ID}, ProductProfileID: profile.ID})
	if err != nil || replayed.ID != created.ID {
		t.Fatalf("unordered replay = %#v, %v", replayed, err)
	}
}

func TestMiyunHandoffExportAndExplicitDeliveryState(t *testing.T) {
	now := time.Date(2026, 8, 11, 2, 0, 0, 0, time.UTC)
	repository := newMiyunServiceRepository(now)
	service := newMiyunTestService(repository, now)
	asset := service.MiyunAssets.(*fakeMiyunAssetReader)
	asset.source.SHA256 = miyunHandoffTestHash([]byte("video"))
	material := validMiyunMaterial(now)
	material.SelectionStatus, material.ImportStatus = MiyunMaterialConfirmed, MiyunMaterialImportDeduplicated
	material.PlatformAssetID, material.PlatformAssetVersion = "asset_1", 1
	repository.materials[material.ID] = material
	snapshot := validMiyunMaterialSnapshot(now)
	snapshot.MaterialID = material.ID
	repository.snapshots[material.ID] = []MiyunMaterialSnapshot{snapshot}
	profile := validMiyunProductProfile(now)
	profile.Status, profile.Version = MiyunProfileConfirmed, 2
	profile.ProductAssetRefs = []contract.AssetVersionRef{{AssetID: "asset_1", Version: 1}}
	profile.KnowledgeDocumentIDs = []string{}
	repository.profiles[profile.ID] = profile
	handoff, err := service.CreateMiyunHandoff(context.Background(), miyunTestActor(), "project_1", CreateMiyunHandoffRequest{SourceMaterialIDs: []string{material.ID}, ProductProfileID: profile.ID})
	if err != nil {
		t.Fatal(err)
	}
	service.MiyunHandoffContent = miyunHandoffContentTestDouble{content: map[string][]byte{"asset:asset_1:1": []byte("video")}}
	if _, err := service.MarkMiyunHandoffDelivered(context.Background(), miyunTestActor(), "project_1", handoff.ID, handoff.Version); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("delivery before export error=%v", err)
	}
	var output bytes.Buffer
	if err := service.ExportMiyunHandoff(context.Background(), miyunTestActor(), "project_1", handoff.ID, &output); err != nil || output.Len() == 0 {
		t.Fatalf("export error=%v size=%d", err, output.Len())
	}
	exported, _ := service.GetMiyunHandoff(context.Background(), miyunTestActor(), "project_1", handoff.ID)
	if exported.Status != MiyunHandoffExported {
		t.Fatalf("status after export=%s", exported.Status)
	}
	if _, err := service.MarkMiyunHandoffDelivered(context.Background(), miyunTestActor(), "project_1", handoff.ID, handoff.Version); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale delivery version error=%v", err)
	}
	delivered, err := service.MarkMiyunHandoffDelivered(context.Background(), miyunTestActor(), "project_1", handoff.ID, exported.Version)
	if err != nil || delivered.Status != MiyunHandoffDelivered {
		t.Fatalf("delivery=%#v err=%v", delivered, err)
	}
	if repeated, err := service.MarkMiyunHandoffDelivered(context.Background(), miyunTestActor(), "project_1", handoff.ID, delivered.Version); err != nil || repeated.ID != delivered.ID {
		t.Fatalf("delivery replay=%#v err=%v", repeated, err)
	}
	profile.ProductName = "second frozen profile"
	repository.profiles[profile.ID] = profile
	failing, err := service.CreateMiyunHandoff(context.Background(), miyunTestActor(), "project_1", CreateMiyunHandoffRequest{SourceMaterialIDs: []string{material.ID}, ProductProfileID: profile.ID})
	if err != nil || failing.ID == handoff.ID {
		t.Fatalf("second handoff=%#v err=%v", failing, err)
	}
	if err := service.ExportMiyunHandoff(context.Background(), miyunTestActor(), "project_1", failing.ID, miyunHandoffFailingWriter{}); err == nil {
		t.Fatal("failed response writer exported a handoff")
	}
	failed, _ := service.GetMiyunHandoff(context.Background(), miyunTestActor(), "project_1", failing.ID)
	if failed.Status != MiyunHandoffFailed {
		t.Fatalf("failed export status=%s", failed.Status)
	}
}

type miyunHandoffContentTestDouble struct{ content map[string][]byte }

type miyunHandoffFailingWriter struct{}

func (miyunHandoffFailingWriter) Write([]byte) (int, error) {
	return 0, errors.New("client disconnected")
}

func (d miyunHandoffContentTestDouble) OpenMiyunHandoffAsset(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, ref contract.AssetVersionRef) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(d.content["asset:"+string(ref.AssetID)+":"+fmt.Sprint(ref.Version)])), nil
}
func (d miyunHandoffContentTestDouble) OpenMiyunHandoffDocument(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, id string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(d.content["document:"+id])), nil
}
func miyunHandoffTestHash(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
