package insights

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const MiyunHandoffManifestV1 = "miyun-handoff-manifest/cookies-draft-v1"
const MiyunHandoffManifestV2 = "miyun-handoff-manifest/cookies-draft-v2"
const MiyunHandoffManifestVersion = "miyun-handoff-manifest/cookies-draft-v3"
const MiyunHandoffParameterVersion = "miyun-handoff-parameters/v1"

type CreateMiyunHandoffRequest struct {
	SourceMaterialIDs []string `json:"source_material_ids"`
	ProductProfileID  string   `json:"product_profile_id"`
	CrawlJobID        string   `json:"crawl_job_id"`
}

type miyunHandoffSourceSnapshot struct {
	Material MiyunMaterial            `json:"material"`
	DataCard MiyunMaterialSnapshot    `json:"data_card"`
	AssetRef contract.AssetVersionRef `json:"asset_ref"`
	File     MiyunHandoffExportFile   `json:"file"`
}
type miyunHandoffSourcesSnapshot struct {
	Sources []miyunHandoffSourceSnapshot `json:"sources"`
}
type miyunHandoffProfileSnapshot struct {
	Profile        MiyunProductProfile `json:"profile"`
	ProfileVersion int64               `json:"profile_version"`
}
type miyunHandoffProductFilesSnapshot struct {
	Media     []MiyunHandoffExportFile `json:"media"`
	Documents []MiyunHandoffExportFile `json:"documents"`
}
type MiyunHandoffContentOpener interface {
	OpenMiyunHandoffAsset(context.Context, contract.ActorContext, contract.ProjectID, contract.AssetVersionRef) (io.ReadCloser, error)
	OpenMiyunHandoffDocument(context.Context, contract.ActorContext, contract.ProjectID, string) (io.ReadCloser, error)
}

func (s Service) handoffRepository() (MiyunHandoffRepository, error) {
	repository, ok := s.Miyun.(MiyunHandoffRepository)
	if !ok || repository == nil {
		return nil, fmt.Errorf("Miyun handoff repository is unavailable")
	}
	return repository, nil
}

func (s Service) CreateMiyunHandoff(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreateMiyunHandoffRequest) (MiyunHandoff, error) {
	if err := s.miyunReady(actor, projectID, ScopeWrite); err != nil {
		return MiyunHandoff{}, err
	}
	repository, err := s.handoffRepository()
	if err != nil {
		return MiyunHandoff{}, err
	}
	request.ProductProfileID = strings.TrimSpace(request.ProductProfileID)
	request.CrawlJobID = strings.TrimSpace(request.CrawlJobID)
	sourceMaterialIDs, err := normalizeMiyunHandoffSourceMaterialIDs(request.SourceMaterialIDs)
	if err != nil || request.ProductProfileID == "" || request.CrawlJobID == "" {
		return MiyunHandoff{}, ErrInvalidRequest
	}
	job, err := repository.GetMiyunCrawlJob(ctx, actor.OrganizationID, projectID, request.CrawlJobID)
	if err != nil {
		return MiyunHandoff{}, err
	}
	if job.ProductProfileID != request.ProductProfileID {
		return MiyunHandoff{}, fmt.Errorf("%w: crawl job and product profile do not match", ErrInvalidRequest)
	}
	profile, err := repository.GetMiyunProductProfile(ctx, actor.OrganizationID, projectID, request.ProductProfileID)
	if err != nil {
		return MiyunHandoff{}, err
	}
	if profile.Status != MiyunProfileConfirmed {
		return MiyunHandoff{}, fmt.Errorf("%w: product profile must be confirmed", ErrInvalidState)
	}
	if s.MiyunAssets == nil || s.MiyunKnowledge == nil {
		return MiyunHandoff{}, fmt.Errorf("Miyun handoff sources are unavailable")
	}
	sources := make([]miyunHandoffSourceSnapshot, 0, len(sourceMaterialIDs))
	for _, sourceMaterialID := range sourceMaterialIDs {
		material, readErr := repository.GetMiyunMaterial(ctx, actor.OrganizationID, projectID, sourceMaterialID)
		if readErr != nil {
			return MiyunHandoff{}, readErr
		}
		if material.SelectionStatus != MiyunMaterialConfirmed || (material.ImportStatus != MiyunMaterialImportImported && material.ImportStatus != MiyunMaterialImportDeduplicated) || material.PlatformAssetID == "" || material.PlatformAssetVersion < 1 {
			return MiyunHandoff{}, fmt.Errorf("%w: every source material must be confirmed and imported", ErrInvalidState)
		}
		snapshots, readErr := repository.ListMiyunMaterialSnapshots(ctx, actor.OrganizationID, projectID, material.ID)
		if readErr != nil {
			return MiyunHandoff{}, readErr
		}
		var jobSnapshot *MiyunMaterialSnapshot
		for index := range snapshots {
			if snapshots[index].CrawlJobID == request.CrawlJobID {
				jobSnapshot = &snapshots[index]
			}
		}
		if jobSnapshot == nil {
			return MiyunHandoff{}, fmt.Errorf("%w: source material does not belong to the selected crawl job", ErrInvalidState)
		}
		sourceRef := contract.AssetVersionRef{AssetID: material.PlatformAssetID, Version: material.PlatformAssetVersion}
		sourceAsset, readErr := s.MiyunAssets.ReadMiyunAssetSource(ctx, actor, projectID, sourceRef)
		if readErr != nil {
			return MiyunHandoff{}, readErr
		}
		if !sourceAsset.Ready || sourceAsset.MIMEType != "video/mp4" || len(sourceAsset.SHA256) != 64 {
			return MiyunHandoff{}, fmt.Errorf("%w: source video is unavailable", ErrInvalidState)
		}
		sources = append(sources, miyunHandoffSourceSnapshot{Material: material, DataCard: *jobSnapshot, AssetRef: sourceRef, File: MiyunHandoffExportFile{Reference: "asset:" + string(sourceRef.AssetID) + ":" + fmt.Sprint(sourceRef.Version), Name: "source_" + string(sourceRef.AssetID) + "_v" + fmt.Sprint(sourceRef.Version) + ".mp4", SHA256: sourceAsset.SHA256}})
	}
	sourceSnapshot, err := json.Marshal(miyunHandoffSourcesSnapshot{Sources: sources})
	if err != nil {
		return MiyunHandoff{}, err
	}
	profileSnapshot, err := json.Marshal(miyunHandoffProfileSnapshot{Profile: profile, ProfileVersion: profile.Version})
	if err != nil {
		return MiyunHandoff{}, err
	}
	files := miyunHandoffProductFilesSnapshot{Media: []MiyunHandoffExportFile{}, Documents: []MiyunHandoffExportFile{}}
	for _, ref := range profile.ProductAssetRefs {
		source, readErr := s.MiyunAssets.ReadMiyunAssetSource(ctx, actor, projectID, ref)
		if readErr != nil {
			return MiyunHandoff{}, readErr
		}
		if !source.Ready || !miyunHandoffMediaMIME(source.MIMEType) {
			return MiyunHandoff{}, fmt.Errorf("%w: unsupported or unavailable product media", ErrInvalidState)
		}
		files.Media = append(files.Media, MiyunHandoffExportFile{Reference: "asset:" + string(ref.AssetID) + ":" + fmt.Sprint(ref.Version), Name: "asset_" + string(ref.AssetID) + "_v" + fmt.Sprint(ref.Version) + miyunHandoffExtension(source.MIMEType), SHA256: source.SHA256})
	}
	for _, documentID := range profile.KnowledgeDocumentIDs {
		source, readErr := s.MiyunKnowledge.ReadMiyunKnowledgeSource(ctx, actor, projectID, documentID)
		if readErr != nil {
			return MiyunHandoff{}, readErr
		}
		if source.Status != "ready" || !miyunHandoffDocumentMIME(source.MIMEType) || len(source.ContentSHA256) != 64 {
			return MiyunHandoff{}, fmt.Errorf("%w: unsupported or unavailable product document", ErrInvalidState)
		}
		files.Documents = append(files.Documents, MiyunHandoffExportFile{Reference: "document:" + source.ID, Name: source.Filename, SHA256: source.ContentSHA256})
	}
	productFiles, err := json.Marshal(files)
	if err != nil {
		return MiyunHandoff{}, err
	}
	identity, err := json.Marshal(struct {
		CrawlJobID       string          `json:"crawl_job_id"`
		Source           json.RawMessage `json:"source"`
		Profile          json.RawMessage `json:"profile"`
		ProductFiles     json.RawMessage `json:"product_files"`
		ManifestVersion  string          `json:"manifest_version"`
		ParameterVersion string          `json:"parameter_version"`
	}{request.CrawlJobID, sourceSnapshot, profileSnapshot, productFiles, MiyunHandoffManifestVersion, MiyunHandoffParameterVersion})
	if err != nil {
		return MiyunHandoff{}, err
	}
	digest := sha256.Sum256(identity)
	inputHash := hex.EncodeToString(digest[:])
	if existing, err := repository.FindMiyunHandoffByInputHash(ctx, actor.OrganizationID, projectID, inputHash); err == nil {
		return existing, nil
	} else if !errors.Is(err, ErrNotFound) {
		return MiyunHandoff{}, err
	}
	id, err := s.idGenerator()("miyunhandoff")
	if err != nil {
		return MiyunHandoff{}, err
	}
	now := s.now()
	value := MiyunHandoff{ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID, CrawlJobID: request.CrawlJobID, SourceMaterialID: sourceMaterialIDs[0], SourceMaterialIDs: sourceMaterialIDs, ProductProfileID: profile.ID, Status: MiyunHandoffExporting, ManifestVersion: MiyunHandoffManifestVersion, ParameterVersion: MiyunHandoffParameterVersion, ProductFilesSnapshot: productFiles, SourceSnapshot: sourceSnapshot, ProfileSnapshot: profileSnapshot, InputHash: inputHash, Version: 1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now}
	created, err := repository.CreateMiyunHandoff(ctx, value)
	if err != nil {
		if existing, lookupErr := repository.FindMiyunHandoffByInputHash(ctx, actor.OrganizationID, projectID, inputHash); lookupErr == nil {
			return existing, nil
		}
		return MiyunHandoff{}, err
	}
	return created, nil
}

func miyunHandoffMediaMIME(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "image/jpeg", "image/png", "image/webp", "video/mp4":
		return true
	}
	return false
}

func normalizeMiyunHandoffSourceMaterialIDs(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, ErrInvalidRequest
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, ErrInvalidRequest
		}
		if _, exists := seen[value]; exists {
			return nil, ErrInvalidRequest
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}
func miyunHandoffDocumentMIME(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "application/pdf", "text/plain", "text/markdown":
		return true
	}
	return false
}
func miyunHandoffExtension(mime string) string {
	switch strings.ToLower(mime) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	}
	return filepath.Ext(mime)
}

func (s Service) ListMiyunHandoffs(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]MiyunHandoff, error) {
	if err := s.miyunReady(actor, projectID, ScopeRead); err != nil {
		return nil, err
	}
	r, err := s.handoffRepository()
	if err != nil {
		return nil, err
	}
	values, err := r.ListMiyunHandoffs(ctx, actor.OrganizationID, projectID, limit)
	if err != nil {
		return nil, err
	}
	if returns, ok := s.Miyun.(MiyunReturnRepository); ok {
		for index := range values {
			values[index].Returns, err = returns.ListMiyunHandoffReturns(ctx, actor.OrganizationID, projectID, values[index].ID)
			if err != nil {
				return nil, err
			}
		}
	}
	return values, nil
}
func (s Service) GetMiyunHandoff(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (MiyunHandoff, error) {
	if err := s.miyunReady(actor, projectID, ScopeRead); err != nil {
		return MiyunHandoff{}, err
	}
	value, err := s.Miyun.GetMiyunHandoff(ctx, actor.OrganizationID, projectID, strings.TrimSpace(id))
	if err != nil {
		return MiyunHandoff{}, err
	}
	if returns, ok := s.Miyun.(MiyunReturnRepository); ok {
		value.Returns, err = returns.ListMiyunHandoffReturns(ctx, actor.OrganizationID, projectID, value.ID)
		if err != nil {
			return MiyunHandoff{}, err
		}
	}
	return value, nil
}
func (s Service) MarkMiyunHandoffDelivered(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string, expectedVersion int64) (MiyunHandoff, error) {
	if err := s.miyunReady(actor, projectID, ScopeConfirm); err != nil {
		return MiyunHandoff{}, err
	}
	r, err := s.handoffRepository()
	if err != nil {
		return MiyunHandoff{}, err
	}
	value, err := s.GetMiyunHandoff(ctx, actor, projectID, id)
	if err != nil {
		return MiyunHandoff{}, err
	}
	if value.Status == MiyunHandoffDelivered {
		if value.Version == expectedVersion {
			return value, nil
		}
		return MiyunHandoff{}, ErrVersionConflict
	}
	if value.Status != MiyunHandoffExported {
		return MiyunHandoff{}, fmt.Errorf("%w: only a successfully exported handoff can be delivered", ErrInvalidState)
	}
	value.Status, value.UpdatedAt = MiyunHandoffDelivered, s.now()
	return r.UpdateMiyunHandoffStatus(ctx, value, expectedVersion)
}

func (s Service) ExportMiyunHandoff(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string, packageKind MiyunHandoffPackageKind, output io.Writer) error {
	if err := s.miyunReady(actor, projectID, ScopeRead); err != nil {
		return err
	}
	if s.MiyunHandoffContent == nil {
		return fmt.Errorf("Miyun handoff content reader is unavailable")
	}
	r, err := s.handoffRepository()
	if err != nil {
		return err
	}
	value, err := s.GetMiyunHandoff(ctx, actor, projectID, id)
	if err != nil {
		return err
	}
	transitionVersion := value.Version
	if value.Status == MiyunHandoffFailed {
		value.Status, value.UpdatedAt = MiyunHandoffExporting, s.now()
		value, err = r.UpdateMiyunHandoffStatus(ctx, value, transitionVersion)
		if err != nil {
			return err
		}
		transitionVersion = value.Version
	}
	if value.Status != MiyunHandoffExporting && value.Status != MiyunHandoffExported && value.Status != MiyunHandoffDelivered {
		return fmt.Errorf("%w: handoff cannot be exported", ErrInvalidState)
	}
	var sourceValues miyunHandoffSourcesSnapshot
	var files miyunHandoffProductFilesSnapshot
	var profile miyunHandoffProfileSnapshot
	if json.Unmarshal(value.SourceSnapshot, &sourceValues) != nil || json.Unmarshal(value.ProductFilesSnapshot, &files) != nil || json.Unmarshal(value.ProfileSnapshot, &profile) != nil {
		return fmt.Errorf("%w: invalid frozen handoff snapshot", ErrInvalidState)
	}
	if len(sourceValues.Sources) == 0 {
		var legacy miyunHandoffSourceSnapshot
		if json.Unmarshal(value.SourceSnapshot, &legacy) != nil || legacy.File.Reference == "" {
			return fmt.Errorf("%w: invalid frozen handoff sources", ErrInvalidState)
		}
		sourceValues.Sources = []miyunHandoffSourceSnapshot{legacy}
	}
	sources := make([]MiyunHandoffExportFile, 0, len(sourceValues.Sources))
	manifestSources := miyunHandoffManifestSources(sourceValues.Sources)
	for _, source := range sourceValues.Sources {
		sources = append(sources, source.File)
	}
	sourceMaterialIDs := make([]string, 0, len(sourceValues.Sources))
	for _, source := range sourceValues.Sources {
		sourceMaterialIDs = append(sourceMaterialIDs, source.Material.MiyunMaterialID)
	}
	snapshot := MiyunHandoffExportSnapshot{ManifestVersion: value.ManifestVersion, Manifest: MiyunHandoffManifest{HandoffID: value.ID, HandoffVersion: fmt.Sprint(value.Version), SourceMaterialName: manifestSources.names, MiyunMaterialID: manifestSources.materialIDs, SourceMaterialIDs: sourceMaterialIDs, SourceURL: manifestSources.urls, Source: "miyun", DeliveryDays: manifestSources.deliveryDays, CumulativeImpressions: manifestSources.impressions, RelatedAds: manifestSources.relatedAds, RelatedCreators: manifestSources.relatedCreators, TargetProduct: profile.Profile.ProductName, TargetCategory: profile.Profile.CategoryName, ParameterVersion: value.ParameterVersion, InputHash: value.InputHash}, Sources: sources, ProductMedia: files.Media, ProductDocs: files.Documents}
	writeErr := ExportMiyunHandoffPackageZIP(ctx, output, snapshot, packageKind, miyunHandoffContentReader{opener: s.MiyunHandoffContent, actor: actor, projectID: projectID})
	if writeErr != nil {
		if value.Status == MiyunHandoffExporting {
			value.Status, value.UpdatedAt = MiyunHandoffFailed, s.now()
			_, _ = r.UpdateMiyunHandoffStatus(context.Background(), value, transitionVersion)
		}
		return writeErr
	}
	if value.Status == MiyunHandoffExporting {
		value.Status, value.UpdatedAt = MiyunHandoffExported, s.now()
		_, err = r.UpdateMiyunHandoffStatus(ctx, value, transitionVersion)
		return err
	}
	return nil
}

type miyunHandoffManifestSourceValues struct {
	names, materialIDs, urls, deliveryDays, impressions, relatedAds, relatedCreators string
}

func miyunHandoffManifestSources(sources []miyunHandoffSourceSnapshot) miyunHandoffManifestSourceValues {
	values := make([]miyunHandoffManifestSourceValues, 0, len(sources))
	for _, source := range sources {
		values = append(values, miyunHandoffManifestSourceValues{source.Material.Title, source.Material.MiyunMaterialID, source.Material.SourceRef, fmt.Sprint(source.DataCard.DeliveryDays), source.DataCard.CumulativeImpressionsRaw, fmt.Sprint(source.DataCard.RelatedAds), source.DataCard.RelatedCreatorsRaw})
	}
	join := func(selectValue func(miyunHandoffManifestSourceValues) string) string {
		result := make([]string, len(values))
		for index, value := range values {
			result[index] = selectValue(value)
		}
		return strings.Join(result, ";")
	}
	return miyunHandoffManifestSourceValues{join(func(value miyunHandoffManifestSourceValues) string { return value.names }), join(func(value miyunHandoffManifestSourceValues) string { return value.materialIDs }), join(func(value miyunHandoffManifestSourceValues) string { return value.urls }), join(func(value miyunHandoffManifestSourceValues) string { return value.deliveryDays }), join(func(value miyunHandoffManifestSourceValues) string { return value.impressions }), join(func(value miyunHandoffManifestSourceValues) string { return value.relatedAds }), join(func(value miyunHandoffManifestSourceValues) string { return value.relatedCreators })}
}

type miyunHandoffContentReader struct {
	opener    MiyunHandoffContentOpener
	actor     contract.ActorContext
	projectID contract.ProjectID
}

func (r miyunHandoffContentReader) OpenMiyunHandoffExportFile(ctx context.Context, file MiyunHandoffExportFile) (io.ReadCloser, error) {
	parts := strings.Split(file.Reference, ":")
	if len(parts) == 3 && parts[0] == "asset" {
		version, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || version < 1 {
			return nil, ErrInvalidState
		}
		return r.opener.OpenMiyunHandoffAsset(ctx, r.actor, r.projectID, contract.AssetVersionRef{AssetID: contract.AssetID(parts[1]), Version: version})
	}
	if len(parts) == 2 && parts[0] == "document" {
		return r.opener.OpenMiyunHandoffDocument(ctx, r.actor, r.projectID, parts[1])
	}
	return nil, ErrInvalidState
}
