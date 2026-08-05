package insights

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// MySQL implementation of ConnectorRepository. Tables are created by
// migrations/insights/20260729143000_insight_data_ingestion.up.sql.

const dataSourceSelect = `SELECT id, organization_id, project_id, platform, account_label, account_ref,
	ingest_mode, credential_ref, status, quality_status, quality_note,
	time_zone, currency, attribution_window, metric_schema_version, field_mapping,
	data_through, last_synced_at, version, created_by, created_at, updated_at
	FROM insight_data_sources`

const importBatchSelect = `SELECT id, organization_id, project_id, data_source_id, kind, status,
	source_label, window_start, window_end, content_hash,
	requested_rows, accepted_rows, rejected_rows, error_summary, errors, corrects_batch_id,
	started_at, finished_at, version, created_by, created_at, updated_at
	FROM insight_import_batches`

func (r MySQLRepository) CreateDataSource(ctx context.Context, value DataSource) (DataSource, error) {
	mapping, err := encodeFieldMapping(value.FieldMapping)
	if err != nil {
		return DataSource{}, err
	}
	_, err = r.DB.ExecContext(ctx, `INSERT INTO insight_data_sources (
		id, organization_id, project_id, platform, account_label, account_ref,
		ingest_mode, credential_ref, status, quality_status, quality_note,
		time_zone, currency, attribution_window, metric_schema_version, field_mapping,
		data_through, last_synced_at, version, created_by, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		value.ID, value.OrganizationID, value.ProjectID, value.Platform, value.AccountLabel, value.AccountRef,
		value.IngestMode, value.CredentialRef, value.Status, value.QualityStatus, value.QualityNote,
		value.Caliber.TimeZone, value.Caliber.Currency, value.Caliber.AttributionWindow, value.Caliber.MetricSchemaVersion,
		mapping, nullableDate(value.DataThrough), value.LastSyncedAt,
		value.Version, value.CreatedBy, value.CreatedAt, value.UpdatedAt)
	if isDuplicateKey(err) {
		return DataSource{}, fmt.Errorf("%w: 该项目下这个平台账户已经接入过了", ErrInvalidState)
	}
	if err != nil {
		return DataSource{}, err
	}
	return value, nil
}

func (r MySQLRepository) ListDataSources(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, filter DataSourceFilter) ([]DataSource, error) {
	query := dataSourceSelect + ` WHERE organization_id = ? AND project_id = ?`
	args := []any{organizationID, projectID}
	if len(filter.Statuses) > 0 {
		query += ` AND status IN (` + placeholders(len(filter.Statuses)) + `)`
		for _, status := range filter.Statuses {
			args = append(args, status)
		}
	}
	if len(filter.Platforms) > 0 {
		query += ` AND platform IN (` + placeholders(len(filter.Platforms)) + `)`
		for _, platform := range filter.Platforms {
			args = append(args, platform)
		}
	}
	query += ` ORDER BY updated_at DESC, id DESC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]DataSource, 0)
	for rows.Next() {
		value, scanErr := scanDataSource(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (r MySQLRepository) GetDataSource(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (DataSource, error) {
	value, err := scanDataSource(r.DB.QueryRowContext(ctx,
		dataSourceSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`, organizationID, projectID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return DataSource{}, ErrNotFound
	}
	return value, err
}

// UpdateDataSource is optimistic on version. expectedVersion 0 means the caller
// did not supply one — the current version is used, so internal callers such as
// the freshness bump after an import do not need to round-trip for it.
func (r MySQLRepository) UpdateDataSource(ctx context.Context, value DataSource, expectedVersion int64) (DataSource, error) {
	if expectedVersion == 0 {
		expectedVersion = value.Version
	}
	mapping, err := encodeFieldMapping(value.FieldMapping)
	if err != nil {
		return DataSource{}, err
	}
	result, err := r.DB.ExecContext(ctx, `UPDATE insight_data_sources SET
		account_label = ?, status = ?, quality_status = ?, quality_note = ?,
		time_zone = ?, currency = ?, attribution_window = ?, metric_schema_version = ?,
		field_mapping = ?, data_through = ?, last_synced_at = ?,
		version = version + 1, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ?`,
		value.AccountLabel, value.Status, value.QualityStatus, value.QualityNote,
		value.Caliber.TimeZone, value.Caliber.Currency, value.Caliber.AttributionWindow, value.Caliber.MetricSchemaVersion,
		mapping, nullableDate(value.DataThrough), value.LastSyncedAt, value.UpdatedAt,
		value.OrganizationID, value.ProjectID, value.ID, expectedVersion)
	if err != nil {
		return DataSource{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return DataSource{}, err
	}
	if affected == 0 {
		return DataSource{}, ErrVersionConflict
	}
	value.Version = expectedVersion + 1
	return value, nil
}

func (r MySQLRepository) CreateImportBatch(ctx context.Context, value ImportBatch) (ImportBatch, error) {
	encodedErrors, err := encodeStringList(value.Errors)
	if err != nil {
		return ImportBatch{}, err
	}
	_, err = r.DB.ExecContext(ctx, `INSERT INTO insight_import_batches (
		id, organization_id, project_id, data_source_id, kind, status,
		source_label, window_start, window_end, content_hash,
		requested_rows, accepted_rows, rejected_rows, error_summary, errors, corrects_batch_id,
		started_at, finished_at, version, created_by, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		value.ID, value.OrganizationID, value.ProjectID, value.DataSourceID, value.Kind, value.Status,
		value.SourceLabel, nullableDate(value.WindowStart), nullableDate(value.WindowEnd),
		nullableString(value.ContentHash),
		value.RequestedRows, value.AcceptedRows, value.RejectedRows, value.ErrorSummary, encodedErrors,
		nullableString(value.CorrectsBatchID), value.StartedAt, value.FinishedAt,
		value.Version, value.CreatedBy, value.CreatedAt, value.UpdatedAt)
	if isDuplicateKey(err) {
		// doc10 §8：相同文件哈希 + 相同导入范围默认阻止重复。
		return ImportBatch{}, fmt.Errorf("%w: 相同内容和范围的批次已经导入过，如需重导请创建更正批次", ErrInvalidState)
	}
	if err != nil {
		return ImportBatch{}, err
	}
	return value, nil
}

func (r MySQLRepository) FinishImportBatch(ctx context.Context, value ImportBatch, expectedVersion int64) (ImportBatch, error) {
	if expectedVersion == 0 {
		expectedVersion = value.Version
	}
	encodedErrors, err := encodeStringList(value.Errors)
	if err != nil {
		return ImportBatch{}, err
	}
	result, err := r.DB.ExecContext(ctx, `UPDATE insight_import_batches SET
		status = ?, window_start = ?, window_end = ?,
		requested_rows = ?, accepted_rows = ?, rejected_rows = ?,
		error_summary = ?, errors = ?, finished_at = ?,
		version = version + 1, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ?`,
		value.Status, nullableDate(value.WindowStart), nullableDate(value.WindowEnd),
		value.RequestedRows, value.AcceptedRows, value.RejectedRows,
		value.ErrorSummary, encodedErrors, value.FinishedAt, value.UpdatedAt,
		value.OrganizationID, value.ProjectID, value.ID, expectedVersion)
	if err != nil {
		return ImportBatch{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return ImportBatch{}, err
	}
	if affected == 0 {
		return ImportBatch{}, ErrVersionConflict
	}
	value.Version = expectedVersion + 1
	return value, nil
}

func (r MySQLRepository) ListImportBatches(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, filter ImportBatchFilter) ([]ImportBatch, error) {
	query := importBatchSelect + ` WHERE organization_id = ? AND project_id = ?`
	args := []any{organizationID, projectID}
	if strings.TrimSpace(filter.DataSourceID) != "" {
		query += ` AND data_source_id = ?`
		args = append(args, strings.TrimSpace(filter.DataSourceID))
	}
	if len(filter.Statuses) > 0 {
		query += ` AND status IN (` + placeholders(len(filter.Statuses)) + `)`
		for _, status := range filter.Statuses {
			args = append(args, status)
		}
	}
	// 同步记录按发生时间倒序：最近一次同步是不是坏的，是这一页唯一要先回答的问题。
	query += ` ORDER BY created_at DESC, id DESC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]ImportBatch, 0)
	for rows.Next() {
		value, scanErr := scanImportBatch(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

// UpsertMetricFacts is idempotent on the doc10 §7 key. Re-running the same
// batch overwrites the same rows rather than doubling the spend; a later batch
// covering the same day wins, which is how 最近窗口定期重拉 absorbs late
// attribution without creating duplicates.
func (r MySQLRepository) UpsertMetricFacts(ctx context.Context, facts []MetricFact) (int, error) {
	if len(facts) == 0 {
		return 0, nil
	}
	written := 0
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		for _, fact := range facts {
			raw, encodeErr := encodeRaw(fact.Raw)
			if encodeErr != nil {
				return encodeErr
			}
			_, execErr := tx.ExecContext(ctx, `INSERT INTO insight_metric_daily (
				id, organization_id, project_id, data_source_id, import_batch_id,
				platform, platform_object_kind, platform_object_id,
				stat_date, attribution_window, metric_schema_version, currency,
				impressions, clicks, conversions, video_views, video_completions,
				spend_cents, revenue_cents, raw, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				data_source_id = VALUES(data_source_id),
				import_batch_id = VALUES(import_batch_id),
				currency = VALUES(currency),
				impressions = VALUES(impressions),
				clicks = VALUES(clicks),
				conversions = VALUES(conversions),
				video_views = VALUES(video_views),
				video_completions = VALUES(video_completions),
				spend_cents = VALUES(spend_cents),
				revenue_cents = VALUES(revenue_cents),
				raw = VALUES(raw),
				updated_at = VALUES(updated_at)`,
				fact.ID, fact.OrganizationID, fact.ProjectID, fact.DataSourceID, fact.ImportBatchID,
				fact.Platform, fact.PlatformObjectKind, fact.PlatformObjectID,
				fact.StatDate.Format("2006-01-02"), fact.Caliber.AttributionWindow, fact.Caliber.MetricSchemaVersion,
				fact.Caliber.Currency,
				fact.Counts.Impressions, fact.Counts.Clicks, fact.Counts.Conversions,
				fact.Counts.VideoViews, fact.Counts.VideoCompletions,
				fact.Counts.SpendCents, fact.Counts.RevenueCents, raw,
				fact.CreatedAt, fact.UpdatedAt)
			if execErr != nil {
				return execErr
			}
			written++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return written, nil
}

// ListMetricFacts left-joins the mapping table so unmatched objects come back
// with an empty asset id instead of disappearing (doc10 §5, AM-003).
func (r MySQLRepository) ListMetricFacts(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, window MetricWindow) ([]MetricFactWithMapping, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT
		m.id, m.organization_id, m.project_id, m.data_source_id, m.import_batch_id,
		m.platform, m.platform_object_kind, m.platform_object_id,
		m.stat_date, m.attribution_window, m.metric_schema_version, m.currency,
		m.impressions, m.clicks, m.conversions, m.video_views, m.video_completions,
		m.spend_cents, m.revenue_cents, m.created_at, m.updated_at,
		COALESCE(p.status, 'unmatched'), COALESCE(a.id, ''), COALESCE(a.title, ''), COALESCE(a.asset_type, '')
		FROM insight_metric_daily m
		LEFT JOIN insight_asset_mappings p
			ON p.organization_id = m.organization_id AND p.project_id = m.project_id
			AND p.platform = m.platform AND p.platform_object_kind = m.platform_object_kind
			AND p.platform_object_id = m.platform_object_id
		LEFT JOIN insight_assets a
			ON a.organization_id = p.organization_id AND a.id = p.insight_asset_id
		WHERE m.organization_id = ? AND m.project_id = ? AND m.stat_date BETWEEN ? AND ?
		ORDER BY m.stat_date ASC, m.platform_object_id ASC`,
		organizationID, projectID, window.Start.Format("2006-01-02"), window.End.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]MetricFactWithMapping, 0)
	for rows.Next() {
		var value MetricFactWithMapping
		var statDate string
		if err := rows.Scan(
			&value.ID, &value.OrganizationID, &value.ProjectID, &value.DataSourceID, &value.ImportBatchID,
			&value.Platform, &value.PlatformObjectKind, &value.PlatformObjectID,
			&statDate, &value.Caliber.AttributionWindow, &value.Caliber.MetricSchemaVersion, &value.Caliber.Currency,
			&value.Counts.Impressions, &value.Counts.Clicks, &value.Counts.Conversions,
			&value.Counts.VideoViews, &value.Counts.VideoCompletions,
			&value.Counts.SpendCents, &value.Counts.RevenueCents,
			&value.CreatedAt, &value.UpdatedAt,
			&value.MappingStatus, &value.AssetID, &value.AssetTitle, &value.AssetType,
		); err != nil {
			return nil, err
		}
		parsed, parseErr := parseDate(statDate)
		if parseErr != nil {
			return nil, parseErr
		}
		value.StatDate = parsed
		values = append(values, value)
	}
	return values, rows.Err()
}

func scanDataSource(scanner rowScanner) (DataSource, error) {
	var value DataSource
	var accountLabel, accountRef, credentialRef, qualityNote sql.NullString
	var mapping []byte
	var dataThrough sql.NullString
	if err := scanner.Scan(
		&value.ID, &value.OrganizationID, &value.ProjectID, &value.Platform, &accountLabel, &accountRef,
		&value.IngestMode, &credentialRef, &value.Status, &value.QualityStatus, &qualityNote,
		&value.Caliber.TimeZone, &value.Caliber.Currency, &value.Caliber.AttributionWindow, &value.Caliber.MetricSchemaVersion,
		&mapping, &dataThrough, &value.LastSyncedAt,
		&value.Version, &value.CreatedBy, &value.CreatedAt, &value.UpdatedAt,
	); err != nil {
		return DataSource{}, err
	}
	value.AccountLabel = accountLabel.String
	value.AccountRef = accountRef.String
	value.CredentialRef = credentialRef.String
	value.QualityNote = qualityNote.String
	if len(mapping) > 0 {
		decoded := map[string]string{}
		if err := json.Unmarshal(mapping, &decoded); err != nil {
			return DataSource{}, err
		}
		value.FieldMapping = decoded
	}
	if dataThrough.Valid {
		parsed, err := parseDate(dataThrough.String)
		if err != nil {
			return DataSource{}, err
		}
		value.DataThrough = &parsed
	}
	return value, nil
}

func scanImportBatch(scanner rowScanner) (ImportBatch, error) {
	var value ImportBatch
	var sourceLabel, contentHash, errorSummary, correctsBatchID sql.NullString
	var windowStart, windowEnd sql.NullString
	var encodedErrors []byte
	if err := scanner.Scan(
		&value.ID, &value.OrganizationID, &value.ProjectID, &value.DataSourceID, &value.Kind, &value.Status,
		&sourceLabel, &windowStart, &windowEnd, &contentHash,
		&value.RequestedRows, &value.AcceptedRows, &value.RejectedRows, &errorSummary, &encodedErrors,
		&correctsBatchID, &value.StartedAt, &value.FinishedAt,
		&value.Version, &value.CreatedBy, &value.CreatedAt, &value.UpdatedAt,
	); err != nil {
		return ImportBatch{}, err
	}
	value.SourceLabel = sourceLabel.String
	value.ContentHash = contentHash.String
	value.ErrorSummary = errorSummary.String
	value.CorrectsBatchID = correctsBatchID.String
	for _, pair := range []struct {
		raw    sql.NullString
		target **time.Time
	}{{windowStart, &value.WindowStart}, {windowEnd, &value.WindowEnd}} {
		if !pair.raw.Valid {
			continue
		}
		parsed, err := parseDate(pair.raw.String)
		if err != nil {
			return ImportBatch{}, err
		}
		*pair.target = &parsed
	}
	if len(encodedErrors) > 0 {
		decoded := []string{}
		if err := json.Unmarshal(encodedErrors, &decoded); err != nil {
			return ImportBatch{}, err
		}
		value.Errors = decoded
	}
	return value, nil
}

// parseDate accepts both what the driver hands back for a DATE column with
// parseTime off ("2006-01-02") and the full timestamp form, so the repository
// does not depend on the DSN's parseTime setting.
func parseDate(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) > 10 {
		trimmed = trimmed[:10]
	}
	return time.ParseInLocation("2006-01-02", trimmed, time.UTC)
}

func nullableDate(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format("2006-01-02")
}

func encodeFieldMapping(value map[string]string) (any, error) {
	if len(value) == 0 {
		return nil, nil
	}
	return json.Marshal(value)
}

func encodeStringList(value []string) (any, error) {
	if len(value) == 0 {
		return nil, nil
	}
	return json.Marshal(value)
}

func encodeRaw(value map[string]any) (any, error) {
	if len(value) == 0 {
		return nil, nil
	}
	return json.Marshal(value)
}

func isDuplicateKey(err error) bool {
	var mysqlError *mysql.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}

const qualityDispositionSelect = `SELECT id, organization_id, project_id, fingerprint, issue_kind,
	state, note, observed_through, version, decided_by, created_at, updated_at
	FROM insight_quality_dispositions`

// ListQualityDispositions 一次取全 Project 的处置记录。数量天然有界——
// 一条问题只有一行，而问题类型和影响对象都是有限的——所以不分页，
// 让检测器在内存里按 fingerprint 对上即可。
func (r MySQLRepository) ListQualityDispositions(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) ([]QualityDisposition, error) {
	rows, err := r.DB.QueryContext(ctx, qualityDispositionSelect+
		` WHERE organization_id = ? AND project_id = ? ORDER BY updated_at DESC, id DESC`,
		organizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]QualityDisposition, 0)
	for rows.Next() {
		var value QualityDisposition
		if scanErr := rows.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.Fingerprint,
			&value.IssueKind, &value.State, &value.Note, &value.ObservedThrough,
			&value.Version, &value.DecidedBy, &value.CreatedAt, &value.UpdatedAt); scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

// UpsertQualityDisposition 一条问题在一个 Project 里只有一条当前处置：
// 再次处置是覆盖同一行并抬高 version，而不是堆一串历史。
// 需要完整处置历史时应该另建审计表，这里刻意只保留"现在算什么状态"。
func (r MySQLRepository) UpsertQualityDisposition(ctx context.Context, value QualityDisposition) (QualityDisposition, error) {
	_, err := r.DB.ExecContext(ctx, `INSERT INTO insight_quality_dispositions (
		id, organization_id, project_id, fingerprint, issue_kind, state, note,
		observed_through, version, decided_by, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE
		issue_kind = VALUES(issue_kind),
		state = VALUES(state),
		note = VALUES(note),
		observed_through = VALUES(observed_through),
		version = version + 1,
		decided_by = VALUES(decided_by),
		updated_at = VALUES(updated_at)`,
		value.ID, value.OrganizationID, value.ProjectID, value.Fingerprint, value.IssueKind,
		value.State, value.Note, value.ObservedThrough, value.Version, value.DecidedBy,
		value.CreatedAt, value.UpdatedAt)
	if err != nil {
		return QualityDisposition{}, err
	}
	rows, err := r.DB.QueryContext(ctx, qualityDispositionSelect+
		` WHERE organization_id = ? AND project_id = ? AND fingerprint = ?`,
		value.OrganizationID, value.ProjectID, value.Fingerprint)
	if err != nil {
		return QualityDisposition{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return QualityDisposition{}, sql.ErrNoRows
	}
	var stored QualityDisposition
	if err := rows.Scan(&stored.ID, &stored.OrganizationID, &stored.ProjectID, &stored.Fingerprint,
		&stored.IssueKind, &stored.State, &stored.Note, &stored.ObservedThrough,
		&stored.Version, &stored.DecidedBy, &stored.CreatedAt, &stored.UpdatedAt); err != nil {
		return QualityDisposition{}, err
	}
	return stored, rows.Err()
}
