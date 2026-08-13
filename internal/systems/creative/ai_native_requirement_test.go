package creative

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

type aiNativeProductResolverStub struct {
	product AINativeProductSnapshot
	err     error
	called  bool
}

func (r *aiNativeProductResolverStub) Resolve(context.Context, string) (AINativeProductSnapshot, error) {
	r.called = true
	return r.product, r.err
}

type aiNativeTextGeneratorStub struct {
	response provider.SynchronousResponse
	err      error
	request  provider.TextGenerateRequest
}

type aiNativeProductMediaImporterStub struct{ called bool }

type aiNativeProductMediaImporterErrorStub struct{}

func (aiNativeProductMediaImporterErrorStub) ImportProductMedia(context.Context, contract.ActorContext, contract.ProjectID, string, []AINativeRequirementMedia) ([]AINativeRequirementMedia, error) {
	return nil, errors.New("image source unavailable")
}

func (s *aiNativeProductMediaImporterStub) ImportProductMedia(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, _ string, media []AINativeRequirementMedia) ([]AINativeRequirementMedia, error) {
	s.called = true
	result := append([]AINativeRequirementMedia{}, media...)
	for index := range result {
		result[index].AssetRef = &contract.AssetVersionRef{AssetID: contract.AssetID("asset_product_1"), Version: 1}
	}
	return result, nil
}

func containsText(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

type aiNativeOperationCancellerStub struct {
	id      string
	version int64
}

func (s *aiNativeOperationCancellerStub) CancelAINativeOperation(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, id string, version int64) error {
	s.id, s.version = id, version
	return nil
}

type memoryAINativeRequirementRepository struct {
	workspace AINativeRequirementWorkspace
	revisions map[int64]AINativeRequirementDraft
	statuses  map[int64]string
}

func (r *memoryAINativeRequirementRepository) CreateAINativeRequirementWorkspace(_ context.Context, value AINativeRequirementWorkspace) (AINativeRequirementWorkspace, error) {
	r.workspace = value
	if r.revisions == nil {
		r.revisions = map[int64]AINativeRequirementDraft{}
		r.statuses = map[int64]string{}
	}
	r.revisions[value.CurrentRevision] = value.Requirement
	r.statuses[value.CurrentRevision] = AINativeRequirementDraftStatus
	return value, nil
}
func (r *memoryAINativeRequirementRepository) GetAINativeRequirementWorkspace(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string) (AINativeRequirementWorkspace, error) {
	if r.workspace.WorkspaceID != workspaceID || r.workspace.OrganizationID != organizationID || r.workspace.ProjectID != projectID {
		return AINativeRequirementWorkspace{}, ErrNotFound
	}
	return r.workspace, nil
}
func (r *memoryAINativeRequirementRepository) GetLatestAINativeRequirementWorkspace(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) (AINativeRequirementWorkspace, error) {
	if r.workspace.OrganizationID != organizationID || r.workspace.ProjectID != projectID || r.workspace.WorkspaceID == "" {
		return AINativeRequirementWorkspace{}, ErrNotFound
	}
	return r.workspace, nil
}
func (r *memoryAINativeRequirementRepository) ListAINativeAdWorkspaces(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) ([]AINativeAdWorkspaceSummary, error) {
	if r.workspace.OrganizationID != organizationID || r.workspace.ProjectID != projectID || r.workspace.WorkspaceID == "" {
		return []AINativeAdWorkspaceSummary{}, nil
	}
	return []AINativeAdWorkspaceSummary{{
		WorkspaceID: r.workspace.WorkspaceID, DisplayName: r.workspace.DisplayName,
		ProductName: r.workspace.Requirement.ProductName, CurrentStage: r.workspace.CurrentStage,
		Status: r.workspace.Status, ScriptStatus: r.workspace.ScriptStatus,
		StoryboardStatus: r.workspace.StoryboardStatus, ProductionStatus: r.workspace.ProductionStatus,
		CreatedAt: r.workspace.CreatedAt, UpdatedAt: r.workspace.UpdatedAt,
	}}, nil
}
func (r *memoryAINativeRequirementRepository) RenameAINativeAdWorkspace(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID, displayName string, now time.Time) (AINativeRequirementWorkspace, error) {
	if r.workspace.WorkspaceID != workspaceID || r.workspace.OrganizationID != organizationID || r.workspace.ProjectID != projectID {
		return AINativeRequirementWorkspace{}, ErrNotFound
	}
	r.workspace.DisplayName = displayName
	r.workspace.UpdatedAt = now
	return r.workspace, nil
}
func (r *memoryAINativeRequirementRepository) AppendAINativeRequirementRevision(_ context.Context, next AINativeRequirementWorkspace, expectedRevision int64, _ string) (AINativeRequirementWorkspace, error) {
	if r.workspace.CurrentRevision != expectedRevision {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	if r.workspace.Status != AINativeRequirementDraftStatus {
		return AINativeRequirementWorkspace{}, ErrInvalidState
	}
	r.workspace = next
	r.revisions[next.CurrentRevision] = next.Requirement
	r.statuses[next.CurrentRevision] = AINativeRequirementDraftStatus
	return next, nil
}
func (r *memoryAINativeRequirementRepository) ConfirmAINativeRequirement(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string, expectedRevision int64, actorID string, now time.Time) (AINativeRequirementWorkspace, error) {
	if r.workspace.CurrentRevision != expectedRevision {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	r.workspace.Status = AINativeRequirementConfirmedStatus
	r.workspace.ConfirmedRevision = &expectedRevision
	r.workspace.ConfirmedBy = actorID
	r.workspace.WorkspaceVersion++
	r.workspace.UpdatedAt = now
	r.statuses[expectedRevision] = AINativeRequirementConfirmedStatus
	return r.workspace, nil
}

func (r *memoryAINativeRequirementRepository) GetAINativeReopenImpact(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID, stage string) (AINativeReopenImpact, error) {
	if stage != AINativeStageRequirement || r.workspace.WorkspaceID != workspaceID || r.workspace.OrganizationID != organizationID || r.workspace.ProjectID != projectID {
		return AINativeReopenImpact{}, ErrNotFound
	}
	if r.workspace.Status != AINativeRequirementConfirmedStatus || r.workspace.ConfirmedRevision == nil {
		return AINativeReopenImpact{}, ErrInvalidState
	}
	resources := []AINativeInvalidatedResource{}
	if r.workspace.ActiveOperationID != "" && r.workspace.ActiveOperationVersion != nil {
		resources = append(resources, AINativeInvalidatedResource{Type: "operation", ID: r.workspace.ActiveOperationID, Status: "cancel_requested", Version: *r.workspace.ActiveOperationVersion})
	}
	return AINativeReopenImpact{
		WorkspaceID: workspaceID, Stage: stage, ExpectedWorkspaceVersion: r.workspace.WorkspaceVersion,
		SupersededRequirementRevisions: []int64{*r.workspace.ConfirmedRevision}, InvalidatedResources: resources,
	}, nil
}

func (r *memoryAINativeRequirementRepository) ReopenAINativeRequirement(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, workspaceID string, expectedWorkspaceVersion int64, actorID string, now time.Time) (AINativeRequirementWorkspace, error) {
	if r.workspace.WorkspaceID != workspaceID || r.workspace.OrganizationID != organizationID || r.workspace.ProjectID != projectID {
		return AINativeRequirementWorkspace{}, ErrNotFound
	}
	if r.workspace.WorkspaceVersion != expectedWorkspaceVersion {
		return AINativeRequirementWorkspace{}, ErrVersionConflict
	}
	previous := r.workspace.CurrentRevision
	r.statuses[previous] = AINativeRequirementSupersededStatus
	r.workspace.CurrentRevision++
	r.workspace.WorkspaceVersion++
	r.workspace.Status = AINativeRequirementDraftStatus
	r.workspace.ConfirmedRevision = nil
	r.workspace.ConfirmedBy = ""
	r.workspace.ActiveOperationID = ""
	r.workspace.ActiveOperationVersion = nil
	r.workspace.Requirement.Revision = r.workspace.CurrentRevision
	r.workspace.Requirement.Status = AINativeRequirementDraftStatus
	r.workspace.UpdatedAt = now
	r.revisions[r.workspace.CurrentRevision] = r.workspace.Requirement
	r.statuses[r.workspace.CurrentRevision] = AINativeRequirementDraftStatus
	return r.workspace, nil
}

func (g *aiNativeTextGeneratorStub) GenerateText(_ context.Context, request provider.TextGenerateRequest) (provider.SynchronousResponse, error) {
	g.request = request
	return g.response, g.err
}

func TestGetLatestAINativeRequirementWorkspaceRestoresProjectWorkspace(t *testing.T) {
	t.Parallel()
	value := AINativeRequirementWorkspace{WorkspaceID: "ainativeworkspace_1", OrganizationID: "org_1", ProjectID: "project_1", CurrentStage: AINativeStageScript}
	repository := &memoryAINativeRequirementRepository{workspace: value}
	service := Service{Projects: testProjects{}, AINativeRequirements: repository}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeRead}}

	restored, err := service.GetLatestAINativeRequirementWorkspace(context.Background(), actor, value.ProjectID)
	if err != nil {
		t.Fatalf("GetLatestAINativeRequirementWorkspace() error = %v", err)
	}
	if restored.WorkspaceID != value.WorkspaceID || restored.CurrentStage != value.CurrentStage {
		t.Fatalf("restored workspace = %+v", restored)
	}
}

func TestAINativeWorkspaceCatalogListsAndRenamesSavedAds(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	value := AINativeRequirementWorkspace{
		WorkspaceID: "ainativeworkspace_1", OrganizationID: "org_1", ProjectID: "project_1",
		Status: AINativeRequirementDraftStatus, CurrentStage: AINativeStageRequirement,
		Requirement: AINativeRequirementDraft{ProductName: "施美乐钛杯"}, CreatedAt: now, UpdatedAt: now,
	}
	repository := &memoryAINativeRequirementRepository{workspace: value}
	service := Service{Projects: testProjects{}, AINativeRequirements: repository, Now: func() time.Time { return now.Add(time.Minute) }}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeRead, ScopeWrite}}

	items, err := service.ListAINativeAdWorkspaces(context.Background(), actor, value.ProjectID)
	if err != nil || len(items) != 1 || items[0].ProductName != "施美乐钛杯" {
		t.Fatalf("ListAINativeAdWorkspaces() = %#v, %v", items, err)
	}
	renamed, err := service.RenameAINativeAdWorkspace(context.Background(), actor, value.ProjectID, value.WorkspaceID, RenameAINativeAdWorkspaceRequest{DisplayName: "钛杯通勤版"})
	if err != nil || renamed.DisplayName != "钛杯通勤版" {
		t.Fatalf("RenameAINativeAdWorkspace() = %#v, %v", renamed, err)
	}
	if _, err := service.RenameAINativeAdWorkspace(context.Background(), actor, value.ProjectID, value.WorkspaceID, RenameAINativeAdWorkspaceRequest{DisplayName: "   "}); !errors.Is(err, ErrInvalidAINativeRequirement) {
		t.Fatalf("blank display name should fail, got %v", err)
	}
}

func TestAnalyzeAINativeRequirementAppliesP0Defaults(t *testing.T) {
	resolver := &aiNativeProductResolverStub{product: testAINativeProduct()}
	importer := &aiNativeProductMediaImporterStub{}
	repository := &memoryAINativeRequirementRepository{}
	service := Service{
		Projects: testProjects{}, AINativeProducts: resolver,
		AINativeRequirementPlanner: DeterministicAINativeRequirementPlanner{}, AINativeRequirements: repository,
		AINativeProductMediaImporter: importer,
		NewID:                        func(string) (string, error) { return "ainativeworkspace_1", nil },
	}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeWrite}}
	workspace, err := service.AnalyzeAINativeRequirement(context.Background(), actor, "project_1", AnalyzeAINativeRequirementRequest{
		ProductLink: "https://v.douyin.com/example/", SupplementalRequirement: "突出通勤场景",
	})
	if err != nil {
		t.Fatal(err)
	}
	draft := workspace.Requirement
	if !resolver.called || workspace.WorkspaceID != "ainativeworkspace_1" || draft.Channel != "douyin" || draft.AspectRatio != "9:16" || draft.DurationSeconds != 20 || draft.Language != "zh-CN" {
		t.Fatalf("P0 defaults were not applied: %#v", workspace)
	}
	if draft.ContractVersion != aiNativeRequirementContractV2 || draft.OutputPreset.ID != AINativeOutputPresetDouyinFeed9x16V1 || draft.OutputPreset.AspectRatio != "9:16" {
		t.Fatalf("output preset defaults were not frozen: %#v", draft.OutputPreset)
	}
	if draft.DeliveryTreatment != DefaultAINativeDeliveryTreatment() {
		t.Fatalf("delivery treatment defaults were not frozen: %#v", draft.DeliveryTreatment)
	}
	if draft.Generation.Mode != "deterministic_fallback" || len(draft.TargetAudiences) != 3 || len(draft.CoreSellingPoints) != 3 || len(draft.Media) != 1 || !importer.called || draft.Media[0].AssetRef == nil {
		t.Fatalf("unexpected deterministic draft: %#v", draft)
	}
	if err := draft.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestAINativeRequirementV2DraftMayBeIncompleteUntilConfirmation(t *testing.T) {
	draft := AINativeRequirementDraft{
		ContractVersion: aiNativeRequirementContractV2,
		Revision:        1,
		Status:          AINativeRequirementDraftStatus,
		Product: AINativeProductSnapshot{
			Source: "taobao", ProductID: "123456789", SourceURL: "https://item.taobao.com/item.htm?id=123456789",
			Price: AINativeProductPrice{Currency: "CNY", DisplayUnconfirmed: true},
		},
		ProductResolution: AINativeProductResolution{
			Status: AINativeProductResolutionManualRequired, Source: "taobao", ResourceType: AINativeProductResourceProduct,
			ExternalID: "123456789", SourceURL: "https://item.taobao.com/item.htm?id=123456789", MissingFields: []string{"product_name", "images"},
		},
		OutputPreset:      DefaultAINativeOutputPreset(),
		DeliveryTreatment: DefaultAINativeDeliveryTreatment(),
		Channel:           "douyin", AspectRatio: "9:16", DurationSeconds: 20, Language: "zh-CN",
		Generation: AINativeGenerationMetadata{Mode: "deterministic_fallback", ModelAlias: "fixture.deterministic", ModelVersion: "partial-v1", PromptVersion: aiNativeRequirementPromptVersion},
	}

	if err := draft.ValidateStructure(); err != nil {
		t.Fatalf("incomplete v2 draft should be structurally valid: %v", err)
	}
	issues := draft.ValidateForConfirmation()
	fields := map[string]bool{}
	for _, issue := range issues {
		fields[issue.Field] = true
	}
	for _, field := range []string{"product_name", "media", "core_selling_points", "target_audiences"} {
		if !fields[field] {
			t.Fatalf("confirmation issue for %s is missing: %#v", field, issues)
		}
	}
}

func TestAINativeDeliveryPresetRejectsContradictoryTreatment(t *testing.T) {
	treatment := DefaultAINativeDeliveryTreatment()
	treatment.VoiceoverMode = AINativeVoiceoverNone
	treatment.CaptionMode = AINativeCaptionEditorial
	if err := treatment.Validate(); err == nil {
		t.Fatal("full_ad preset should not accept a no-voiceover treatment")
	}
	treatment.Preset = AINativeDeliveryPresetCustom
	if err := treatment.Validate(); err != nil {
		t.Fatalf("the same advanced combination should be valid as custom: %v", err)
	}
}

func TestAINativeDeliveryPresetCatalogMapsTheThreeProductChoices(t *testing.T) {
	cases := []struct {
		preset, voiceover, caption, overlay, audio string
	}{
		{AINativeDeliveryPresetFullAd, AINativeVoiceoverGenerated, AINativeCaptionFromVoiceover, AINativeSalesOverlayKeyPoints, AINativeMusicSFXAuto},
		{AINativeDeliveryPresetNoVoiceover, AINativeVoiceoverNone, AINativeCaptionEditorial, AINativeSalesOverlayKeyPoints, AINativeMusicSFXAuto},
		{AINativeDeliveryPresetCleanMaterial, AINativeVoiceoverNone, AINativeCaptionNone, AINativeSalesOverlayNone, AINativeMusicSFXNone},
	}
	for _, tc := range cases {
		got, err := AINativeDeliveryTreatmentForPreset(tc.preset)
		if err != nil {
			t.Fatalf("preset %s: %v", tc.preset, err)
		}
		if got.VoiceoverMode != tc.voiceover || got.CaptionMode != tc.caption || got.SalesOverlayMode != tc.overlay || got.MusicSFXMode != tc.audio {
			t.Fatalf("preset %s = %#v", tc.preset, got)
		}
	}
}

func TestLegacyAINativeRequirementV1RemainsStructurallyValid(t *testing.T) {
	draft := AINativeRequirementDraft{
		ContractVersion: aiNativeRequirementContractV1, Revision: 1, Status: AINativeRequirementDraftStatus,
		Product: testAINativeProduct(), ProductName: testAINativeProduct().Name, ProductDescription: "legacy description",
		TargetAudiences:   []AINativeEditableText{{ID: "audience_1", Text: "通勤人群"}},
		Media:             []AINativeRequirementMedia{{ID: "media_1", URL: testAINativeProduct().Images[0].URL, Role: "main", Source: "douyin_mall", AssetRef: &contract.AssetVersionRef{AssetID: "asset_1", Version: 1}}},
		CoreSellingPoints: []AINativeEditableText{{ID: "selling_point_1", Text: "便携随行"}},
		Channel:           "douyin", AspectRatio: "9:16", DurationSeconds: 20, Language: "zh-CN",
		Generation: AINativeGenerationMetadata{Mode: "deterministic_fallback", ModelAlias: "fixture.deterministic", ModelVersion: "legacy-v1", PromptVersion: aiNativeRequirementPromptVersion},
	}

	if err := draft.ValidateStructure(); err != nil {
		t.Fatalf("legacy v1 requirement should remain readable: %v", err)
	}
}

func TestUpdatingLegacyAINativeRequirementWritesV2Defaults(t *testing.T) {
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	product := testAINativeProduct()
	assetRef := &contract.AssetVersionRef{AssetID: "asset_product_1", Version: 1}
	draft := AINativeRequirementDraft{
		ContractVersion: aiNativeRequirementContractV1, Revision: 1, Status: AINativeRequirementDraftStatus,
		Product: product, ProductName: product.Name, ProductDescription: "legacy description",
		TargetAudiences:   []AINativeEditableText{{ID: "audience_1", Text: "通勤人群"}},
		Media:             []AINativeRequirementMedia{{ID: "media_1", URL: product.Images[0].URL, Role: "main", Source: product.Source, AssetRef: assetRef}},
		CoreSellingPoints: []AINativeEditableText{{ID: "selling_point_1", Text: "便携随行"}},
		Channel:           "douyin", AspectRatio: "9:16", DurationSeconds: 20, Language: "zh-CN",
		Generation: AINativeGenerationMetadata{Mode: "deterministic_fallback", ModelAlias: "fixture.deterministic", ModelVersion: "legacy-v1", PromptVersion: aiNativeRequirementPromptVersion},
	}
	repository := &memoryAINativeRequirementRepository{workspace: AINativeRequirementWorkspace{
		WorkspaceID: "workspace_1", CreativeIntakeID: "intake_1", CreativeTaskID: "task_1", OrganizationID: "org_1", ProjectID: "project_1",
		Status: AINativeRequirementDraftStatus, CurrentStage: AINativeStageRequirement, WorkspaceVersion: 1, CurrentRevision: 1,
		Requirement: draft, CreatedBy: "user_1", CreatedAt: now, UpdatedAt: now,
	}, revisions: map[int64]AINativeRequirementDraft{1: draft}, statuses: map[int64]string{1: AINativeRequirementDraftStatus}}
	service := Service{Projects: testProjects{}, AINativeRequirements: repository, Now: func() time.Time { return now.Add(time.Minute) }}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeWrite}}

	updated, err := service.UpdateAINativeRequirement(context.Background(), actor, "project_1", "workspace_1", UpdateAINativeRequirementRequest{
		ExpectedRevision: 1, ProductName: draft.ProductName, ProductDescription: draft.ProductDescription,
		TargetAudiences: draft.TargetAudiences, Media: draft.Media, CoreSellingPoints: draft.CoreSellingPoints,
		AspectRatio: "9:16", DurationSeconds: 20, Language: "zh-CN",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Requirement.ContractVersion != aiNativeRequirementContractV2 || updated.Requirement.OutputPreset.ID != AINativeOutputPresetDouyinFeed9x16V1 {
		t.Fatalf("legacy update was not upgraded to v2: %#v", updated.Requirement)
	}
	if updated.Requirement.DeliveryTreatment != DefaultAINativeDeliveryTreatment() || updated.Requirement.ProductResolution.Source != product.Source {
		t.Fatalf("v2 defaults were not populated during upgrade: %#v", updated.Requirement)
	}
}

func TestUpdatingRequirementRejectsUploadedAssetOutsideProjectImageAuthority(t *testing.T) {
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	product := testAINativeProduct()
	draft := AINativeRequirementDraft{
		ContractVersion: aiNativeRequirementContractV2, Revision: 1, Status: AINativeRequirementDraftStatus,
		Product: product, ProductResolution: AINativeProductResolution{Status: AINativeProductResolutionManualRequired, Source: product.Source, ResourceType: AINativeProductResourceProduct, ExternalID: product.ProductID, SourceURL: product.SourceURL, MissingFields: []string{"images"}},
		ProductName: product.Name, ProductDescription: product.Name,
		TargetAudiences: []AINativeEditableText{{ID: "audience_1", Text: "通勤人群"}}, CoreSellingPoints: []AINativeEditableText{{ID: "selling_point_1", Text: "便携随行"}},
		Channel: "douyin", AspectRatio: "9:16", DurationSeconds: 20, Language: "zh-CN", OutputPreset: DefaultAINativeOutputPreset(), DeliveryTreatment: DefaultAINativeDeliveryTreatment(),
		Generation: AINativeGenerationMetadata{Mode: "deterministic_fallback", ModelAlias: "fixture.deterministic", ModelVersion: "partial-v1", PromptVersion: aiNativeRequirementPromptVersion},
	}
	repository := &memoryAINativeRequirementRepository{workspace: AINativeRequirementWorkspace{
		WorkspaceID: "workspace_1", OrganizationID: "org_1", ProjectID: "project_1", Status: AINativeRequirementDraftStatus, CurrentStage: AINativeStageRequirement,
		WorkspaceVersion: 1, CurrentRevision: 1, Requirement: draft, CreatedBy: "user_1", CreatedAt: now, UpdatedAt: now,
	}, revisions: map[int64]AINativeRequirementDraft{1: draft}, statuses: map[int64]string{1: AINativeRequirementDraftStatus}}
	service := Service{Projects: testProjects{}, AINativeRequirements: repository, Assets: testAssetReader{snapshots: map[contract.AssetID]CreativeAssetSnapshot{}}}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeWrite}}

	_, err := service.UpdateAINativeRequirement(context.Background(), actor, "project_1", "workspace_1", UpdateAINativeRequirementRequest{
		ExpectedRevision: 1, ProductName: product.Name, ProductDescription: product.Name,
		TargetAudiences: draft.TargetAudiences, CoreSellingPoints: draft.CoreSellingPoints,
		Media:       []AINativeRequirementMedia{{ID: "media_upload", URL: "https://assets.cookies.local/asset_missing/1", Role: "main", Source: "user_upload", AssetRef: &contract.AssetVersionRef{AssetID: "asset_missing", Version: 1}}},
		AspectRatio: "9:16", DurationSeconds: 20, Language: "zh-CN",
	})
	if !errors.Is(err, ErrInvalidAINativeRequirement) {
		t.Fatalf("unreadable uploaded asset must be rejected, got %v", err)
	}
}

func TestResolveAINativeProductPreviewDoesNotCreateWorkspace(t *testing.T) {
	resolver := &aiNativeProductResolverStub{product: testAINativeProduct()}
	service := Service{Projects: testProjects{}, AINativeProducts: resolver}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeRead}}

	preview, err := service.ResolveAINativeProductPreview(context.Background(), actor, "project_1", ResolveAINativeProductPreviewRequest{ProductLink: "https://v.douyin.com/example/"})
	if err != nil {
		t.Fatal(err)
	}
	if !resolver.called || preview.ProductID == "" || preview.ProductName != testAINativeProduct().Name || preview.SourceURL != testAINativeProduct().SourceURL {
		t.Fatalf("unexpected product preview: %#v", preview)
	}
}

func TestResolveAINativeProductPreviewAcceptsManualRequiredProduct(t *testing.T) {
	product := AINativeProductSnapshot{
		Source: "taobao", ProductID: "123456789", Name: "", SourceURL: "https://item.taobao.com/item.htm?id=123456789",
		ResolutionStatus: AINativeProductResolutionManualRequired, ResourceType: AINativeProductResourceProduct,
		MissingFields: []string{"product_name", "images", "core_selling_points"},
		Price:         AINativeProductPrice{Currency: "CNY", DisplayUnconfirmed: true},
	}
	service := Service{Projects: testProjects{}, AINativeProducts: &aiNativeProductResolverStub{product: product}}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeRead}}

	preview, err := service.ResolveAINativeProductPreview(context.Background(), actor, "project_1", ResolveAINativeProductPreviewRequest{ProductLink: product.SourceURL})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Status != AINativeProductResolutionManualRequired || preview.Source != "taobao" || preview.ProductID != "123456789" || len(preview.MissingFields) != 3 {
		t.Fatalf("unexpected partial preview: %#v", preview)
	}
}

func TestAnalyzeAINativeRequirementKeepsDraftWhenAutomaticImageImportFails(t *testing.T) {
	product := testAINativeProduct()
	product.ResolutionStatus = AINativeProductResolutionPartial
	product.ResourceType = AINativeProductResourceProduct
	product.MissingFields = []string{"description", "core_selling_points"}
	repository := &memoryAINativeRequirementRepository{}
	service := Service{
		Projects: testProjects{}, AINativeProducts: &aiNativeProductResolverStub{product: product},
		AINativeRequirementPlanner: DeterministicAINativeRequirementPlanner{}, AINativeRequirements: repository,
		AINativeProductMediaImporter: aiNativeProductMediaImporterErrorStub{},
		NewID:                        func(prefix string) (string, error) { return prefix + "_1", nil },
	}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeWrite}}

	workspace, err := service.AnalyzeAINativeRequirement(context.Background(), actor, "project_1", AnalyzeAINativeRequirementRequest{ProductLink: product.SourceURL})
	if err != nil {
		t.Fatal(err)
	}
	if workspace.Requirement.ProductResolution.Status != AINativeProductResolutionManualRequired || len(workspace.Requirement.Media) != 0 || !containsText(workspace.Requirement.ProductResolution.MissingFields, "images") {
		t.Fatalf("failed image import should become manual upload state: %#v", workspace.Requirement)
	}
}

func TestAnalyzeAINativeRequirementPersistsIncompleteCommerceDraft(t *testing.T) {
	product := AINativeProductSnapshot{
		Source: "taobao", ProductID: "123456789", SourceURL: "https://item.taobao.com/item.htm?id=123456789",
		ResolutionStatus: AINativeProductResolutionManualRequired, ResourceType: AINativeProductResourceProduct,
		MissingFields: []string{"product_name", "images", "description", "core_selling_points"},
		Price:         AINativeProductPrice{Currency: "CNY", DisplayUnconfirmed: true},
	}
	service := Service{
		Projects: testProjects{}, AINativeProducts: &aiNativeProductResolverStub{product: product},
		AINativeRequirementPlanner: DeterministicAINativeRequirementPlanner{}, AINativeRequirements: &memoryAINativeRequirementRepository{},
		NewID: func(prefix string) (string, error) { return prefix + "_1", nil },
	}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeWrite}}

	workspace, err := service.AnalyzeAINativeRequirement(context.Background(), actor, "project_1", AnalyzeAINativeRequirementRequest{ProductLink: product.SourceURL})
	if err != nil {
		t.Fatal(err)
	}
	missing := workspace.Requirement.ProductResolution.MissingFields
	if workspace.Requirement.ProductResolution.Status != AINativeProductResolutionManualRequired || len(missing) != 4 {
		t.Fatalf("incomplete link must remain an editable, de-duplicated draft: %#v", workspace.Requirement.ProductResolution)
	}
	if issues := workspace.Requirement.ValidateForConfirmation(); len(issues) != 4 {
		t.Fatalf("confirmation must report all user-fillable blockers, got %#v", issues)
	}
}

func TestAnalyzeAINativeRequirementRejectsChannelThatConflictsWithPresetBeforeResolve(t *testing.T) {
	resolver := &aiNativeProductResolverStub{product: testAINativeProduct()}
	service := Service{Projects: testProjects{}, AINativeProducts: resolver, AINativeRequirementPlanner: DeterministicAINativeRequirementPlanner{}, AINativeRequirements: &memoryAINativeRequirementRepository{}}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeWrite}}
	_, err := service.AnalyzeAINativeRequirement(context.Background(), actor, "project_1", AnalyzeAINativeRequirementRequest{ProductLink: "https://v.douyin.com/example/", Channel: "kuaishou"})
	if err == nil || resolver.called {
		t.Fatalf("conflicting channel and default preset should fail before product resolution, err=%v called=%v", err, resolver.called)
	}
}

func TestAnalyzeAINativeRequirementAcceptsSupportedCreationSurface(t *testing.T) {
	resolver := &aiNativeProductResolverStub{product: testAINativeProduct()}
	service := Service{
		Projects: testProjects{}, AINativeProducts: resolver,
		AINativeRequirementPlanner: DeterministicAINativeRequirementPlanner{}, AINativeRequirements: &memoryAINativeRequirementRepository{},
		NewID: func(prefix string) (string, error) { return prefix + "_surface", nil },
	}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeWrite}}

	workspace, err := service.AnalyzeAINativeRequirement(context.Background(), actor, "project_1", AnalyzeAINativeRequirementRequest{
		ProductLink: testAINativeProduct().SourceURL, OutputPresetID: "kuaishou_feed_9x16_v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resolver.called || workspace.Requirement.Channel != "kuaishou" || workspace.Requirement.AspectRatio != "9:16" || workspace.Requirement.OutputPreset.ID != "kuaishou_feed_9x16_v1" {
		t.Fatalf("supported creation surface was not frozen into the requirement: called=%v requirement=%#v", resolver.called, workspace.Requirement)
	}
}

func TestAINativeRequirementRevisionAndConfirmationStateMachine(t *testing.T) {
	repository := &memoryAINativeRequirementRepository{}
	service := Service{
		Projects: testProjects{}, AINativeProducts: &aiNativeProductResolverStub{product: testAINativeProduct()},
		AINativeRequirementPlanner: DeterministicAINativeRequirementPlanner{}, AINativeRequirements: repository,
		AINativeProductMediaImporter: &aiNativeProductMediaImporterStub{},
		NewID:                        func(string) (string, error) { return "ainativeworkspace_1", nil },
		Now:                          func() time.Time { return time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC) },
	}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeRead, ScopeWrite}}
	created, err := service.AnalyzeAINativeRequirement(context.Background(), actor, "project_1", AnalyzeAINativeRequirementRequest{ProductLink: "https://v.douyin.com/example/"})
	if err != nil {
		t.Fatal(err)
	}
	request := UpdateAINativeRequirementRequest{
		ExpectedRevision: 1, ProductName: created.Requirement.ProductName, ProductDescription: "用户确认后的商品描述",
		TargetAudiences: created.Requirement.TargetAudiences, Media: created.Requirement.Media,
		CoreSellingPoints: created.Requirement.CoreSellingPoints, SupplementalRequirement: "突出通勤场景",
		AspectRatio: "9:16", DurationSeconds: 25, Language: "zh-CN",
	}
	updated, err := service.UpdateAINativeRequirement(context.Background(), actor, "project_1", created.WorkspaceID, request)
	if err != nil {
		t.Fatal(err)
	}
	if updated.CurrentRevision != 2 || len(repository.revisions) != 2 || updated.Requirement.ProductDescription != "用户确认后的商品描述" {
		t.Fatalf("revision was not appended: %#v", updated)
	}
	request.ExpectedRevision = 1
	if _, err := service.UpdateAINativeRequirement(context.Background(), actor, "project_1", created.WorkspaceID, request); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale update should conflict, got %v", err)
	}
	withoutAssets := request
	withoutAssets.ExpectedRevision = 2
	withoutAssets.Media = append([]AINativeRequirementMedia{}, request.Media...)
	withoutAssets.Media[0].AssetRef = nil
	if _, err := service.UpdateAINativeRequirement(context.Background(), actor, "project_1", created.WorkspaceID, withoutAssets); !errors.Is(err, ErrInvalidAINativeRequirement) {
		t.Fatalf("persisted product media must retain an asset_ref, got %v", err)
	}
	confirmed, err := service.ConfirmAINativeRequirement(context.Background(), actor, "project_1", created.WorkspaceID, ConfirmAINativeRequirementRequest{ExpectedRevision: 2})
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Status != AINativeRequirementConfirmedStatus || confirmed.ConfirmedRevision == nil || *confirmed.ConfirmedRevision != 2 {
		t.Fatalf("requirement was not frozen: %#v", confirmed)
	}
	request.ExpectedRevision = 2
	if _, err := service.UpdateAINativeRequirement(context.Background(), actor, "project_1", created.WorkspaceID, request); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("confirmed requirement must be immutable, got %v", err)
	}
}

func TestConfirmAINativeRequirementRejectsIncompleteDraftBeforeRepositoryMutation(t *testing.T) {
	now := time.Date(2026, 8, 13, 9, 0, 0, 0, time.UTC)
	draft := AINativeRequirementDraft{
		ContractVersion: aiNativeRequirementContractV2, Revision: 1, Status: AINativeRequirementDraftStatus,
		Product:           AINativeProductSnapshot{Source: "taobao", ProductID: "123", SourceURL: "https://item.taobao.com/item.htm?id=123", Price: AINativeProductPrice{Currency: "CNY", DisplayUnconfirmed: true}},
		ProductResolution: AINativeProductResolution{Status: AINativeProductResolutionManualRequired, Source: "taobao", ResourceType: AINativeProductResourceProduct, ExternalID: "123", SourceURL: "https://item.taobao.com/item.htm?id=123"},
		OutputPreset:      DefaultAINativeOutputPreset(), DeliveryTreatment: DefaultAINativeDeliveryTreatment(),
		Channel: "douyin", AspectRatio: "9:16", DurationSeconds: 20, Language: "zh-CN",
		Generation: AINativeGenerationMetadata{Mode: "deterministic_fallback", ModelAlias: "fixture.deterministic", ModelVersion: "partial-v1", PromptVersion: aiNativeRequirementPromptVersion},
	}
	repository := &memoryAINativeRequirementRepository{workspace: AINativeRequirementWorkspace{
		WorkspaceID: "workspace_1", CreativeIntakeID: "intake_1", CreativeTaskID: "task_1", OrganizationID: "org_1", ProjectID: "project_1",
		Status: AINativeRequirementDraftStatus, CurrentStage: AINativeStageRequirement, WorkspaceVersion: 1, CurrentRevision: 1,
		Requirement: draft, CreatedBy: "user_1", CreatedAt: now, UpdatedAt: now,
	}}
	service := Service{Projects: testProjects{}, AINativeRequirements: repository, Now: func() time.Time { return now }}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeWrite}}

	_, err := service.ConfirmAINativeRequirement(context.Background(), actor, "project_1", "workspace_1", ConfirmAINativeRequirementRequest{ExpectedRevision: 1})
	if !errors.Is(err, ErrInvalidAINativeRequirement) {
		t.Fatalf("incomplete requirement should fail confirmation, got %v", err)
	}
	var confirmationErr AINativeRequirementConfirmationError
	if !errors.As(err, &confirmationErr) || len(confirmationErr.Issues) != 4 {
		t.Fatalf("confirmation error should expose four field issues, got %#v", err)
	}
	if repository.workspace.Status != AINativeRequirementDraftStatus || repository.workspace.ConfirmedRevision != nil {
		t.Fatalf("repository was mutated despite failed confirmation: %#v", repository.workspace)
	}
}

func TestAINativeWorkspaceCreatesAggregateIdentityAndReopensConfirmedRequirement(t *testing.T) {
	repository := &memoryAINativeRequirementRepository{}
	canceller := &aiNativeOperationCancellerStub{}
	ids := []string{"ainativeworkspace_1", "creativeintake_1", "creativetask_1"}
	service := Service{
		Projects: testProjects{}, AINativeProducts: &aiNativeProductResolverStub{product: testAINativeProduct()},
		AINativeRequirementPlanner: DeterministicAINativeRequirementPlanner{}, AINativeRequirements: repository,
		AINativeProductMediaImporter: &aiNativeProductMediaImporterStub{}, AINativeOperationCanceller: canceller,
		NewID: func(string) (string, error) { value := ids[0]; ids = ids[1:]; return value, nil },
		Now:   func() time.Time { return time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC) },
	}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{ScopeRead, ScopeWrite}}
	created, err := service.AnalyzeAINativeRequirement(context.Background(), actor, "project_1", AnalyzeAINativeRequirementRequest{ProductLink: "https://v.douyin.com/example/"})
	if err != nil {
		t.Fatal(err)
	}
	if created.CreativeTaskID != "creativetask_1" || created.WorkspaceVersion != 1 || created.CurrentStage != AINativeStageRequirement {
		t.Fatalf("aggregate identity was not established: %#v", created)
	}
	confirmed, err := service.ConfirmAINativeRequirement(context.Background(), actor, "project_1", created.WorkspaceID, ConfirmAINativeRequirementRequest{ExpectedRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	operationVersion := int64(3)
	repository.workspace.ActiveOperationID = "job_1"
	repository.workspace.ActiveOperationVersion = &operationVersion
	confirmed = repository.workspace
	impact, err := service.GetAINativeReopenImpact(context.Background(), actor, "project_1", created.WorkspaceID, AINativeStageRequirement)
	if err != nil {
		t.Fatal(err)
	}
	if impact.ExpectedWorkspaceVersion != confirmed.WorkspaceVersion || len(impact.SupersededRequirementRevisions) != 1 || impact.SupersededRequirementRevisions[0] != 1 || len(impact.InvalidatedResources) != 1 {
		t.Fatalf("unexpected reopen impact: %#v", impact)
	}
	reopened, err := service.ReopenAINativeRequirement(context.Background(), actor, "project_1", created.WorkspaceID, ReopenAINativeRequirementRequest{ExpectedWorkspaceVersion: confirmed.WorkspaceVersion, InvalidateDownstream: true})
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Status != AINativeRequirementDraftStatus || reopened.CurrentRevision != 2 || reopened.WorkspaceVersion != confirmed.WorkspaceVersion+1 || reopened.ConfirmedRevision != nil {
		t.Fatalf("requirement was not reopened as a new draft revision: %#v", reopened)
	}
	if repository.statuses[1] != AINativeRequirementSupersededStatus || repository.statuses[2] != AINativeRequirementDraftStatus {
		t.Fatalf("revision lineage was not preserved: %#v", repository.statuses)
	}
	if canceller.id != "job_1" || canceller.version != 3 {
		t.Fatalf("active operation cancellation was not requested: %#v", canceller)
	}
	if _, err := service.ReopenAINativeRequirement(context.Background(), actor, "project_1", created.WorkspaceID, ReopenAINativeRequirementRequest{ExpectedWorkspaceVersion: confirmed.WorkspaceVersion, InvalidateDownstream: true}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("an already reopened requirement cannot be reopened again, got %v", err)
	}
}

func TestModelAINativeRequirementPlannerUsesStructuredOutput(t *testing.T) {
	output, _ := json.Marshal(modelAINativeRequirement{
		ProductDescription: "面向通勤与随行饮用场景的纯钛保温杯。",
		TargetAudiences:    []string{"通勤人群", "通勤人群", "咖啡爱好者"},
		CoreSellingPoints:  []string{"纯钛材质", "便携随行"},
	})
	generator := &aiNativeTextGeneratorStub{response: provider.SynchronousResponse{
		ProviderCode: "ark", ModelAlias: "cookies.text.standard", ModelVersion: "seed-2-pro", StructuredOutput: output, RouteRevisionID: "route_1",
	}}
	planner := ModelAINativeRequirementPlanner{Text: generator, ModelAlias: "cookies.text.standard"}
	request, err := (AnalyzeAINativeRequirementRequest{ProductLink: "https://v.douyin.com/example/"}).normalized()
	if err != nil {
		t.Fatal(err)
	}
	draft, err := planner.Analyze(context.Background(), contract.ActorContext{}, contract.ProjectContext{}, testAINativeProduct(), request)
	if err != nil {
		t.Fatal(err)
	}
	if draft.Generation.Mode != "model" || draft.Generation.ModelVersion != "seed-2-pro" || draft.Generation.RouteRevisionID != "route_1" {
		t.Fatalf("model provenance missing: %#v", draft.Generation)
	}
	if len(draft.TargetAudiences) != 2 || len(generator.request.OutputJSONSchema) == 0 || len(generator.request.Messages) != 2 {
		t.Fatalf("structured model contract was not used: draft=%#v request=%#v", draft, generator.request)
	}
}

func TestFallbackAINativeRequirementPlannerRecoversFromModelFailure(t *testing.T) {
	failed := &aiNativeTextGeneratorStub{err: errors.New("route unavailable")}
	observed := false
	planner := FallbackAINativeRequirementPlanner{
		Primary:          ModelAINativeRequirementPlanner{Text: failed, ModelAlias: "cookies.text.standard"},
		Fallback:         DeterministicAINativeRequirementPlanner{},
		OnPrimaryFailure: func(error) { observed = true },
	}
	request, _ := (AnalyzeAINativeRequirementRequest{ProductLink: "https://v.douyin.com/example/"}).normalized()
	draft, err := planner.Analyze(context.Background(), contract.ActorContext{}, contract.ProjectContext{}, testAINativeProduct(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !observed || draft.Generation.Mode != "deterministic_fallback" {
		t.Fatalf("fallback was not recorded: observed=%v draft=%#v", observed, draft)
	}
}

func testAINativeProduct() AINativeProductSnapshot {
	return AINativeProductSnapshot{
		Source: "douyin_mall", ProductID: "3802315260866724312",
		Name:   "simelo施美乐纯钛保温杯随行咖啡杯便携钛杯",
		Images: []AINativeProductImage{{URL: "https://p26-item.ecombdimg.com/img/product.png", Role: "main"}},
		Price:  AINativeProductPrice{MinRaw: 79900, MaxRaw: 89900, Currency: "CNY", DisplayUnconfirmed: true},
		Sales:  3221, SourceURL: "https://v.douyin.com/example/",
	}
}
