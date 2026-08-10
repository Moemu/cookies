package insights

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// MiyunRepository owns the Goal 1 persistence foundation plus Goal 2's
// product-profile review and manual intake transaction. Crawl execution and
// later handoff workflows stay behind future, workflow-specific services.
type MiyunRepository interface {
	CreateMiyunConnection(context.Context, MiyunConnection) (MiyunConnection, error)
	GetMiyunConnection(context.Context, contract.OrganizationID, contract.ProjectID, string) (MiyunConnection, error)
	GetProjectMiyunConnection(context.Context, contract.OrganizationID, contract.ProjectID) (MiyunConnection, error)
	UpdateMiyunConnection(context.Context, MiyunConnection, int64) (MiyunConnection, error)
	CreateMiyunProductProfile(context.Context, MiyunProductProfile) (MiyunProductProfile, error)
	CreateMiyunProductProfileDraft(context.Context, MiyunProductProfile) (MiyunProductProfile, error)
	GetMiyunProductProfile(context.Context, contract.OrganizationID, contract.ProjectID, string) (MiyunProductProfile, error)
	ListMiyunProductProfiles(context.Context, contract.OrganizationID, contract.ProjectID, int) ([]MiyunProductProfile, error)
	ConfirmMiyunProductProfile(context.Context, MiyunProductProfile, int64) (MiyunProductProfile, error)
	CreateMiyunCrawlJob(context.Context, MiyunCrawlJob) (MiyunCrawlJob, error)
	GetMiyunCrawlJob(context.Context, contract.OrganizationID, contract.ProjectID, string) (MiyunCrawlJob, error)
	CreateMiyunMaterial(context.Context, MiyunMaterial) (MiyunMaterial, error)
	GetMiyunMaterial(context.Context, contract.OrganizationID, contract.ProjectID, string) (MiyunMaterial, error)
	AppendMiyunMaterialSnapshot(context.Context, MiyunMaterialSnapshot) (MiyunMaterialSnapshot, error)
	ListMiyunMaterialSnapshots(context.Context, contract.OrganizationID, contract.ProjectID, string) ([]MiyunMaterialSnapshot, error)
	CreateManualMiyunMaterial(context.Context, MiyunManualImportRecord) (MiyunManualImportResult, error)
	CreateMiyunHandoff(context.Context, MiyunHandoff) (MiyunHandoff, error)
	GetMiyunHandoff(context.Context, contract.OrganizationID, contract.ProjectID, string) (MiyunHandoff, error)
}

const miyunConnectionSelect = `SELECT id, organization_id, project_id, status,
	session_ciphertext, session_key_version, session_expires_at,
	last_verified_at, last_successful_request_at, cooldown_until, last_error_kind, last_error_code, last_error_at,
	version, created_by, created_at, updated_at FROM insight_miyun_connections`

func (r MySQLRepository) CreateMiyunConnection(ctx context.Context, value MiyunConnection) (MiyunConnection, error) {
	if err := value.Validate(); err != nil {
		return MiyunConnection{}, err
	}
	_, err := r.DB.ExecContext(ctx, `INSERT INTO insight_miyun_connections (
		id, organization_id, project_id, status, session_ciphertext, session_key_version, session_expires_at,
		last_verified_at, last_successful_request_at, last_error_kind, last_error_code, last_error_at,
		cooldown_until, version, created_by, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		value.ID, value.OrganizationID, value.ProjectID, value.Status, value.SessionCiphertext,
		value.SessionKeyVersion, value.SessionExpiresAt, value.LastVerifiedAt, value.LastSuccessfulRequestAt,
		nullableString(value.LastErrorKind), nullableString(value.LastErrorCode), value.LastErrorAt,
		value.CooldownUntil, value.Version, value.CreatedBy, value.CreatedAt, value.UpdatedAt)
	if isDuplicateKey(err) {
		return MiyunConnection{}, fmt.Errorf("%w: Miyun connection identity already exists", ErrInvalidState)
	}
	return value, err
}

func (r MySQLRepository) GetMiyunConnection(ctx context.Context, organizationID contract.OrganizationID,
	projectID contract.ProjectID, id string) (MiyunConnection, error) {
	value, err := scanMiyunConnection(r.DB.QueryRowContext(ctx,
		miyunConnectionSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`, organizationID, projectID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return MiyunConnection{}, ErrNotFound
	}
	return value, err
}

func (r MySQLRepository) GetProjectMiyunConnection(ctx context.Context, organizationID contract.OrganizationID,
	projectID contract.ProjectID) (MiyunConnection, error) {
	value, err := scanMiyunConnection(r.DB.QueryRowContext(ctx,
		miyunConnectionSelect+` WHERE organization_id = ? AND project_id = ? ORDER BY created_at DESC, id DESC LIMIT 1`, organizationID, projectID))
	if errors.Is(err, sql.ErrNoRows) {
		return MiyunConnection{}, ErrNotFound
	}
	return value, err
}

func (r MySQLRepository) UpdateMiyunConnection(ctx context.Context, value MiyunConnection, expectedVersion int64) (MiyunConnection, error) {
	if err := value.Validate(); err != nil {
		return MiyunConnection{}, err
	}
	if expectedVersion < 1 {
		return MiyunConnection{}, ErrInvalidRequest
	}
	result, err := r.DB.ExecContext(ctx, `UPDATE insight_miyun_connections SET
		status = ?, session_ciphertext = ?, session_key_version = ?, session_expires_at = ?,
		last_verified_at = ?, last_successful_request_at = ?, cooldown_until = ?, last_error_kind = ?, last_error_code = ?, last_error_at = ?,
		version = version + 1, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ?`,
		value.Status, value.SessionCiphertext, value.SessionKeyVersion, value.SessionExpiresAt,
		value.LastVerifiedAt, value.LastSuccessfulRequestAt, value.CooldownUntil, nullableString(value.LastErrorKind),
		nullableString(value.LastErrorCode), value.LastErrorAt, value.UpdatedAt,
		value.OrganizationID, value.ProjectID, value.ID, expectedVersion)
	if err != nil {
		return MiyunConnection{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return MiyunConnection{}, err
	}
	if affected == 0 {
		return MiyunConnection{}, ErrVersionConflict
	}
	value.Version = expectedVersion + 1
	return value, nil
}

func scanMiyunConnection(row rowScanner) (MiyunConnection, error) {
	var value MiyunConnection
	var sessionExpiresAt, lastVerifiedAt, lastSuccessfulRequestAt, cooldownUntil, lastErrorAt sql.NullTime
	var lastErrorKind, lastErrorCode sql.NullString
	err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.Status,
		&value.SessionCiphertext, &value.SessionKeyVersion, &sessionExpiresAt,
		&lastVerifiedAt, &lastSuccessfulRequestAt, &cooldownUntil, &lastErrorKind, &lastErrorCode, &lastErrorAt,
		&value.Version, &value.CreatedBy, &value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return MiyunConnection{}, err
	}
	value.SessionExpiresAt = nullTimePointer(sessionExpiresAt)
	value.LastVerifiedAt = nullTimePointer(lastVerifiedAt)
	value.LastSuccessfulRequestAt = nullTimePointer(lastSuccessfulRequestAt)
	value.CooldownUntil = nullTimePointer(cooldownUntil)
	value.LastErrorAt = nullTimePointer(lastErrorAt)
	value.LastErrorKind, value.LastErrorCode = lastErrorKind.String, lastErrorCode.String
	return value, nil
}

const miyunProductProfileSelect = `SELECT id, organization_id, project_id, connection_id, status,
	product_id, product_name, brand_name, category_id, category_name, keywords, material_content_types, window_start, window_end,
	project_context_version, product_asset_refs, knowledge_document_ids, rule_version,
	COALESCE(model_version, ''), analysis_method, input_hash, input_snapshot, field_sources, analysis_warnings,
	confirmed_by, confirmed_at, version, created_by, created_at, updated_at FROM insight_miyun_product_profiles`

func (r MySQLRepository) CreateMiyunProductProfile(ctx context.Context, value MiyunProductProfile) (MiyunProductProfile, error) {
	if err := value.Validate(); err != nil {
		return MiyunProductProfile{}, err
	}
	keywords, err := json.Marshal(value.Keywords)
	if err != nil {
		return MiyunProductProfile{}, err
	}
	contentTypes, err := json.Marshal(value.MaterialContentTypes)
	if err != nil {
		return MiyunProductProfile{}, err
	}
	assetRefs, err := json.Marshal(value.ProductAssetRefs)
	if err != nil {
		return MiyunProductProfile{}, err
	}
	documentIDs, err := json.Marshal(value.KnowledgeDocumentIDs)
	if err != nil {
		return MiyunProductProfile{}, err
	}
	fieldSources, err := json.Marshal(value.FieldSources)
	if err != nil {
		return MiyunProductProfile{}, err
	}
	warnings, err := json.Marshal(value.AnalysisWarnings)
	if err != nil {
		return MiyunProductProfile{}, err
	}
	_, err = r.DB.ExecContext(ctx, `INSERT INTO insight_miyun_product_profiles (
		id, organization_id, project_id, connection_id, status, product_id, product_name, brand_name, category_id, category_name,
		keywords, material_content_types, window_start, window_end, project_context_version,
		product_asset_refs, knowledge_document_ids, rule_version, model_version, analysis_method, input_hash,
		input_snapshot, field_sources, analysis_warnings, confirmed_by, confirmed_at,
		version, created_by, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		value.ID, value.OrganizationID, value.ProjectID, value.ConnectionID, value.Status, value.ProductID, value.ProductName, value.BrandName,
		value.CategoryID, value.CategoryName, keywords, contentTypes, value.WindowStart.Format("2006-01-02"),
		value.WindowEnd.Format("2006-01-02"), value.ProjectContextVersion, assetRefs, documentIDs,
		value.RuleVersion, nullableString(value.ModelVersion), value.AnalysisMethod, value.InputHash,
		value.InputSnapshot, fieldSources, warnings, nullableString(value.ConfirmedBy), value.ConfirmedAt,
		value.Version, value.CreatedBy, value.CreatedAt, value.UpdatedAt)
	if isDuplicateKey(err) {
		return MiyunProductProfile{}, fmt.Errorf("%w: Miyun product profile identity already exists", ErrInvalidState)
	}
	return value, err
}

func (r MySQLRepository) GetMiyunProductProfile(ctx context.Context, organizationID contract.OrganizationID,
	projectID contract.ProjectID, id string) (MiyunProductProfile, error) {
	value, err := scanMiyunProductProfile(r.DB.QueryRowContext(ctx,
		miyunProductProfileSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`, organizationID, projectID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return MiyunProductProfile{}, ErrNotFound
	}
	return value, err
}

func scanMiyunProductProfile(row rowScanner) (MiyunProductProfile, error) {
	var value MiyunProductProfile
	var keywords, contentTypes, assetRefs, documentIDs, inputSnapshot, fieldSources, warnings []byte
	var confirmedBy sql.NullString
	var confirmedAt sql.NullTime
	err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.ConnectionID, &value.Status,
		&value.ProductID, &value.ProductName, &value.BrandName, &value.CategoryID, &value.CategoryName, &keywords, &contentTypes,
		&value.WindowStart, &value.WindowEnd, &value.ProjectContextVersion, &assetRefs, &documentIDs,
		&value.RuleVersion, &value.ModelVersion, &value.AnalysisMethod, &value.InputHash,
		&inputSnapshot, &fieldSources, &warnings, &confirmedBy, &confirmedAt, &value.Version,
		&value.CreatedBy, &value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return MiyunProductProfile{}, err
	}
	if err := json.Unmarshal(keywords, &value.Keywords); err != nil {
		return MiyunProductProfile{}, err
	}
	if err := json.Unmarshal(contentTypes, &value.MaterialContentTypes); err != nil {
		return MiyunProductProfile{}, err
	}
	if err := json.Unmarshal(assetRefs, &value.ProductAssetRefs); err != nil {
		return MiyunProductProfile{}, err
	}
	if err := json.Unmarshal(documentIDs, &value.KnowledgeDocumentIDs); err != nil {
		return MiyunProductProfile{}, err
	}
	value.InputSnapshot = json.RawMessage(inputSnapshot)
	if err := json.Unmarshal(fieldSources, &value.FieldSources); err != nil {
		return MiyunProductProfile{}, err
	}
	if err := json.Unmarshal(warnings, &value.AnalysisWarnings); err != nil {
		return MiyunProductProfile{}, err
	}
	value.ConfirmedBy, value.ConfirmedAt = confirmedBy.String, nullTimePointer(confirmedAt)
	return value, nil
}

const miyunCrawlJobSelect = `SELECT id, organization_id, project_id, connection_id, product_profile_id,
	status, operation, query_schema_version, query_snapshot, idempotency_key, request_hash, runtime_job_id, completed_pages, discovered_count,
	deduplicated_count, downloaded_count, failed_count, cooldown_until, last_error_kind, last_error_code,
	version, created_by, created_at, updated_at FROM insight_miyun_crawl_jobs`

func (r MySQLRepository) CreateMiyunCrawlJob(ctx context.Context, value MiyunCrawlJob) (MiyunCrawlJob, error) {
	if err := value.Validate(); err != nil {
		return MiyunCrawlJob{}, err
	}
	_, err := r.DB.ExecContext(ctx, `INSERT INTO insight_miyun_crawl_jobs (
		id, organization_id, project_id, connection_id, product_profile_id, status, operation,
		query_schema_version, query_snapshot, idempotency_key, request_hash, runtime_job_id, completed_pages, discovered_count, deduplicated_count,
		downloaded_count, failed_count, cooldown_until, last_error_kind, last_error_code,
		version, created_by, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		value.ID, value.OrganizationID, value.ProjectID, value.ConnectionID, value.ProductProfileID,
		value.Status, value.Operation, value.QuerySchemaVersion, value.QuerySnapshot, value.IdempotencyKey, value.RequestHash, value.RuntimeJobID, value.CompletedPages,
		value.DiscoveredCount, value.DeduplicatedCount, value.DownloadedCount, value.FailedCount,
		value.CooldownUntil, nullableString(value.LastErrorKind), nullableString(value.LastErrorCode),
		value.Version, value.CreatedBy, value.CreatedAt, value.UpdatedAt)
	if isDuplicateKey(err) {
		return MiyunCrawlJob{}, fmt.Errorf("%w: Miyun crawl job identity already exists", ErrInvalidState)
	}
	return value, err
}

func (r MySQLRepository) GetMiyunCrawlJob(ctx context.Context, organizationID contract.OrganizationID,
	projectID contract.ProjectID, id string) (MiyunCrawlJob, error) {
	value, err := scanMiyunCrawlJob(r.DB.QueryRowContext(ctx,
		miyunCrawlJobSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`, organizationID, projectID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return MiyunCrawlJob{}, ErrNotFound
	}
	return value, err
}

func scanMiyunCrawlJob(row rowScanner) (MiyunCrawlJob, error) {
	var value MiyunCrawlJob
	var cooldownUntil sql.NullTime
	var lastErrorKind, lastErrorCode sql.NullString
	err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.ConnectionID, &value.ProductProfileID,
		&value.Status, &value.Operation, &value.QuerySchemaVersion, &value.QuerySnapshot, &value.IdempotencyKey, &value.RequestHash, &value.RuntimeJobID, &value.CompletedPages,
		&value.DiscoveredCount, &value.DeduplicatedCount, &value.DownloadedCount, &value.FailedCount,
		&cooldownUntil, &lastErrorKind, &lastErrorCode, &value.Version, &value.CreatedBy, &value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return MiyunCrawlJob{}, err
	}
	value.CooldownUntil = nullTimePointer(cooldownUntil)
	value.LastErrorKind, value.LastErrorCode = lastErrorKind.String, lastErrorCode.String
	return value, nil
}

const miyunMaterialSelect = `SELECT id, organization_id, project_id, miyun_material_id, first_seen_crawl_job_id,
	import_method, manual_idempotency_key, manual_request_hash, resource_id, resource_url_ciphertext, resource_url_key_version, resource_expected_size,
	source_ref, source_ref_status, title, selection_status, import_status, decision_by, decision_at, decision_note,
	last_import_error_kind, last_import_error_code, external_import_id,
	platform_asset_id, platform_asset_version, insight_asset_id, version, created_by, created_at, updated_at
	FROM insight_miyun_materials`

func (r MySQLRepository) CreateMiyunMaterial(ctx context.Context, value MiyunMaterial) (MiyunMaterial, error) {
	if err := value.Validate(); err != nil {
		return MiyunMaterial{}, err
	}
	_, err := r.DB.ExecContext(ctx, `INSERT INTO insight_miyun_materials (
		id, organization_id, project_id, miyun_material_id, first_seen_crawl_job_id, import_method,
		manual_idempotency_key, manual_request_hash,
		resource_id, resource_url_ciphertext, resource_url_key_version, resource_expected_size, source_ref, source_ref_status, title, selection_status, import_status,
		decision_by, decision_at, decision_note, last_import_error_kind, last_import_error_code, external_import_id,
		platform_asset_id, platform_asset_version, insight_asset_id, version, created_by, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		value.ID, value.OrganizationID, value.ProjectID, value.MiyunMaterialID, value.FirstSeenCrawlJobID,
		value.ImportMethod, nullableString(value.ManualIdempotencyKey), nullableString(value.ManualRequestHash),
		value.ResourceID, value.ResourceURLCiphertext, nullableString(value.ResourceURLKeyVersion), value.ResourceExpectedSize, value.SourceRef, value.SourceRefStatus,
		value.Title, value.SelectionStatus, value.ImportStatus, nullableString(value.DecisionBy), value.DecisionAt, value.DecisionNote,
		nullableString(value.LastImportErrorKind), nullableString(value.LastImportErrorCode),
		nullableString(value.ExternalImportID), nullableString(string(value.PlatformAssetID)), nullableInt64(value.PlatformAssetVersion),
		nullableString(value.InsightAssetID), value.Version, value.CreatedBy, value.CreatedAt, value.UpdatedAt)
	if isDuplicateKey(err) {
		return MiyunMaterial{}, fmt.Errorf("%w: Miyun material already exists in this project", ErrInvalidState)
	}
	return value, err
}

func (r MySQLRepository) GetMiyunMaterial(ctx context.Context, organizationID contract.OrganizationID,
	projectID contract.ProjectID, id string) (MiyunMaterial, error) {
	value, err := scanMiyunMaterial(r.DB.QueryRowContext(ctx,
		miyunMaterialSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`, organizationID, projectID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return MiyunMaterial{}, ErrNotFound
	}
	return value, err
}

func scanMiyunMaterial(row rowScanner) (MiyunMaterial, error) {
	var value MiyunMaterial
	var firstSeenCrawlJobID, manualIdempotencyKey, manualRequestHash sql.NullString
	var resourceURLKeyVersion, decisionBy, lastImportErrorKind, lastImportErrorCode sql.NullString
	var externalImportID, platformAssetID, insightAssetID sql.NullString
	var decisionAt sql.NullTime
	var platformAssetVersion sql.NullInt64
	err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.MiyunMaterialID,
		&firstSeenCrawlJobID, &value.ImportMethod, &manualIdempotencyKey, &manualRequestHash,
		&value.ResourceID, &value.ResourceURLCiphertext, &resourceURLKeyVersion, &value.ResourceExpectedSize, &value.SourceRef, &value.SourceRefStatus, &value.Title,
		&value.SelectionStatus, &value.ImportStatus, &decisionBy, &decisionAt, &value.DecisionNote,
		&lastImportErrorKind, &lastImportErrorCode, &externalImportID, &platformAssetID,
		&platformAssetVersion, &insightAssetID, &value.Version, &value.CreatedBy, &value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return MiyunMaterial{}, err
	}
	value.FirstSeenCrawlJobID = firstSeenCrawlJobID.String
	value.ManualIdempotencyKey, value.ManualRequestHash = manualIdempotencyKey.String, manualRequestHash.String
	value.ResourceURLKeyVersion, value.DecisionBy, value.DecisionAt = resourceURLKeyVersion.String, decisionBy.String, nullTimePointer(decisionAt)
	value.LastImportErrorKind, value.LastImportErrorCode = lastImportErrorKind.String, lastImportErrorCode.String
	value.ExternalImportID = externalImportID.String
	value.PlatformAssetID = contract.AssetID(platformAssetID.String)
	value.PlatformAssetVersion = platformAssetVersion.Int64
	value.InsightAssetID = insightAssetID.String
	return value, nil
}

const miyunMaterialSnapshotSelect = `SELECT id, organization_id, project_id, material_id, crawl_job_id, import_method,
	source_page, schema_version, captured_at, first_published_at, last_published_at, delivery_days,
	cumulative_impressions, cumulative_impressions_raw, related_ads, related_creators, material_score, views, likes, comments,
	related_creators_raw, related_creators_known, shares, saves, sanitized_raw, created_at FROM insight_miyun_material_snapshots`

func (r MySQLRepository) AppendMiyunMaterialSnapshot(ctx context.Context, value MiyunMaterialSnapshot) (MiyunMaterialSnapshot, error) {
	if err := value.Validate(); err != nil {
		return MiyunMaterialSnapshot{}, err
	}
	var raw any
	if len(value.SanitizedRaw) > 0 {
		raw = value.SanitizedRaw
	}
	_, err := r.DB.ExecContext(ctx, `INSERT INTO insight_miyun_material_snapshots (
		id, organization_id, project_id, material_id, crawl_job_id, source_page, import_method, schema_version, captured_at,
		first_published_at, last_published_at, delivery_days, cumulative_impressions, cumulative_impressions_raw, related_ads,
		related_creators, related_creators_raw, related_creators_known, material_score, views, likes, comments, shares, saves, sanitized_raw, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		value.ID, value.OrganizationID, value.ProjectID, value.MaterialID, value.CrawlJobID, value.SourcePage,
		value.ImportMethod, value.SchemaVersion, value.CapturedAt, value.FirstPublishedAt, value.LastPublishedAt,
		value.DeliveryDays, value.CumulativeImpressions, value.CumulativeImpressionsRaw, value.RelatedAds, value.RelatedCreators,
		value.RelatedCreatorsRaw, value.RelatedCreatorsKnown,
		value.MaterialScore, value.Views, value.Likes, value.Comments, value.Shares, value.Saves,
		raw, value.CreatedAt)
	if isDuplicateKey(err) {
		return MiyunMaterialSnapshot{}, fmt.Errorf("%w: Miyun material snapshot identity already exists", ErrInvalidState)
	}
	return value, err
}

func (r MySQLRepository) ListMiyunMaterialSnapshots(ctx context.Context, organizationID contract.OrganizationID,
	projectID contract.ProjectID, materialID string) ([]MiyunMaterialSnapshot, error) {
	rows, err := r.DB.QueryContext(ctx, miyunMaterialSnapshotSelect+`
		WHERE organization_id = ? AND project_id = ? AND material_id = ? ORDER BY captured_at, id`,
		organizationID, projectID, materialID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]MiyunMaterialSnapshot, 0)
	for rows.Next() {
		value, scanErr := scanMiyunMaterialSnapshot(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func scanMiyunMaterialSnapshot(row rowScanner) (MiyunMaterialSnapshot, error) {
	var value MiyunMaterialSnapshot
	var firstPublishedAt, lastPublishedAt sql.NullTime
	var crawlJobID sql.NullString
	var sanitizedRaw []byte
	err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.MaterialID, &crawlJobID, &value.ImportMethod,
		&value.SourcePage, &value.SchemaVersion, &value.CapturedAt, &firstPublishedAt, &lastPublishedAt, &value.DeliveryDays,
		&value.CumulativeImpressions, &value.CumulativeImpressionsRaw, &value.RelatedAds, &value.RelatedCreators, &value.MaterialScore,
		&value.Views, &value.Likes, &value.Comments, &value.RelatedCreatorsRaw, &value.RelatedCreatorsKnown,
		&value.Shares, &value.Saves, &sanitizedRaw, &value.CreatedAt)
	if err != nil {
		return MiyunMaterialSnapshot{}, err
	}
	value.CrawlJobID = crawlJobID.String
	value.FirstPublishedAt, value.LastPublishedAt = nullTimePointer(firstPublishedAt), nullTimePointer(lastPublishedAt)
	if len(sanitizedRaw) > 0 {
		value.SanitizedRaw = json.RawMessage(sanitizedRaw)
	}
	return value, nil
}

const miyunHandoffSelect = `SELECT id, organization_id, project_id, source_material_id, product_profile_id,
	status, manifest_version, parameter_version, product_files_snapshot, source_snapshot,
	version, created_by, created_at, updated_at FROM insight_miyun_handoffs`

func (r MySQLRepository) CreateMiyunHandoff(ctx context.Context, value MiyunHandoff) (MiyunHandoff, error) {
	if err := value.Validate(); err != nil {
		return MiyunHandoff{}, err
	}
	_, err := r.DB.ExecContext(ctx, `INSERT INTO insight_miyun_handoffs (
		id, organization_id, project_id, source_material_id, product_profile_id, status,
		manifest_version, parameter_version, product_files_snapshot, source_snapshot,
		version, created_by, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		value.ID, value.OrganizationID, value.ProjectID, value.SourceMaterialID, value.ProductProfileID,
		value.Status, value.ManifestVersion, value.ParameterVersion, value.ProductFilesSnapshot,
		value.SourceSnapshot, value.Version, value.CreatedBy, value.CreatedAt, value.UpdatedAt)
	if isDuplicateKey(err) {
		return MiyunHandoff{}, fmt.Errorf("%w: Miyun handoff identity already exists", ErrInvalidState)
	}
	return value, err
}

func (r MySQLRepository) GetMiyunHandoff(ctx context.Context, organizationID contract.OrganizationID,
	projectID contract.ProjectID, id string) (MiyunHandoff, error) {
	value, err := scanMiyunHandoff(r.DB.QueryRowContext(ctx,
		miyunHandoffSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`, organizationID, projectID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return MiyunHandoff{}, ErrNotFound
	}
	return value, err
}

func scanMiyunHandoff(row rowScanner) (MiyunHandoff, error) {
	var value MiyunHandoff
	var productFilesSnapshot, sourceSnapshot []byte
	err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.SourceMaterialID,
		&value.ProductProfileID, &value.Status, &value.ManifestVersion, &value.ParameterVersion,
		&productFilesSnapshot, &sourceSnapshot, &value.Version, &value.CreatedBy, &value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return MiyunHandoff{}, err
	}
	value.ProductFilesSnapshot = json.RawMessage(productFilesSnapshot)
	value.SourceSnapshot = json.RawMessage(sourceSnapshot)
	return value, nil
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}
