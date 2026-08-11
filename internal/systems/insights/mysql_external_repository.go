package insights

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// MySQL implementation of ExternalAssetRepository. 表由
// migrations/insights/20260811110000_insight_external_assets.up.sql 建。
//
// 这里的每一条语句都只碰 insight_external_assets 一张表。**永远不要**在这个
// 文件里出现 insight_assets——外部素材进了共享素材库，就没有任何机制拦住
// 它被投出去。

const externalAssetSelect = `SELECT id, organization_id, project_id, title, source_note, asset_type,
	purpose, purpose_note, storage_key, original_purged, features, retention_until,
	created_by, created_at, updated_at
	FROM insight_external_assets`

func (r MySQLRepository) CreateExternalAsset(ctx context.Context, value ExternalAsset) (ExternalAsset, error) {
	features, err := json.Marshal(nonNilFeatureValues(value.Features))
	if err != nil {
		return ExternalAsset{}, err
	}
	_, err = r.DB.ExecContext(ctx, `INSERT INTO insight_external_assets (
		id, organization_id, project_id, title, source_note, asset_type,
		purpose, purpose_note, storage_key, original_purged, features, retention_until,
		created_by, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		value.ID, value.OrganizationID, value.ProjectID, value.Title, value.SourceNote, value.AssetType,
		value.Purpose, value.PurposeNote, value.StorageKey, value.OriginalPurged, features, value.RetentionUntil,
		value.CreatedBy, value.CreatedAt, value.UpdatedAt)
	if err != nil {
		return ExternalAsset{}, err
	}
	return value, nil
}

func (r MySQLRepository) ListExternalAssets(ctx context.Context, organizationID contract.OrganizationID,
	projectID contract.ProjectID, limit int) ([]ExternalAsset, error) {
	rows, err := r.DB.QueryContext(ctx, externalAssetSelect+`
		WHERE organization_id = ? AND project_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT ?`, organizationID, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []ExternalAsset{}
	for rows.Next() {
		value, scanErr := scanExternalAsset(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

// PurgeExpiredOriginals 清掉到期的原件，返回它们的存储路径给调用方去对象
// 存储上删文件。
//
// 先查再改必须在一个事务里，并且查的时候要 FOR UPDATE：不然并发跑两次会返回
// 两批相同的 storage_key，对象存储那边删第二次会报不存在。
//
// 改的是 original_purged 和 storage_key，不是删整行——删了的话，引用过它的
// 那份复盘就成了「引用了一个不存在的东西」。features 留着，它们是派生物。
func (r MySQLRepository) PurgeExpiredOriginals(ctx context.Context, now time.Time) ([]string, error) {
	keys := []string{}
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		rows, txErr := tx.QueryContext(ctx, `SELECT id, storage_key
			FROM insight_external_assets
			WHERE retention_until <= ? AND original_purged = 0
			ORDER BY id
			FOR UPDATE`, now)
		if txErr != nil {
			return txErr
		}
		ids := []string{}
		for rows.Next() {
			var id, storageKey string
			if scanErr := rows.Scan(&id, &storageKey); scanErr != nil {
				rows.Close()
				return scanErr
			}
			ids = append(ids, id)
			if storageKey != "" {
				keys = append(keys, storageKey)
			}
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			rows.Close()
			return rowsErr
		}
		rows.Close()
		if len(ids) == 0 {
			return nil
		}
		args := make([]any, 0, len(ids)+1)
		args = append(args, now)
		for _, id := range ids {
			args = append(args, id)
		}
		_, txErr = tx.ExecContext(ctx, `UPDATE insight_external_assets
			SET original_purged = 1, storage_key = '', updated_at = ?
			WHERE id IN (`+placeholders(len(ids))+`)`, args...)
		return txErr
	})
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func scanExternalAsset(rows *sql.Rows) (ExternalAsset, error) {
	var value ExternalAsset
	var features []byte
	if err := rows.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.Title,
		&value.SourceNote, &value.AssetType, &value.Purpose, &value.PurposeNote,
		&value.StorageKey, &value.OriginalPurged, &features, &value.RetentionUntil,
		&value.CreatedBy, &value.CreatedAt, &value.UpdatedAt); err != nil {
		return ExternalAsset{}, err
	}
	value.Features = map[string]FeatureValue{}
	if len(features) > 0 {
		if err := json.Unmarshal(features, &value.Features); err != nil {
			return ExternalAsset{}, err
		}
	}
	return value, nil
}

// nonNilFeatureValues 保证写进 JSON 列的是 {} 而不是 null。
// null 在 MySQL 的 JSON 列里是合法值，读回来会变成一个空指针，
// 之后每个取用的地方都要多一次判空。
func nonNilFeatureValues(values map[string]FeatureValue) map[string]FeatureValue {
	if values == nil {
		return map[string]FeatureValue{}
	}
	return values
}
