package insights

import (
	"context"
	"database/sql"
	"errors"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// MySQL implementation of ThresholdRepository. 表由
// migrations/insights/20260811130000_insight_threshold_sets.up.sql 建。
//
// 这个文件里**没有 UPDATE**，这不是遗漏。阈值集只增版本：每次保存插一行新的，
// 旧行永远留着。留一个更新语句在这里，早晚有人用它「顺手改一下上一版的理由」，
// 而那一改就会让所有盖着那一版号的历史结论对不上账。

const thresholdSetSelect = `SELECT id, organization_id, version,
	sufficient_impressions, directional_impressions, min_trend_days, min_anomaly_days,
	min_driver_assets, max_comparison_assets, quality_window_days,
	reason, changed_by, changed_at
	FROM insight_threshold_sets`

func (r MySQLRepository) LatestThresholdSet(ctx context.Context,
	organizationID contract.OrganizationID) (ThresholdSet, error) {
	row := r.DB.QueryRowContext(ctx, thresholdSetSelect+`
		WHERE organization_id = ?
		ORDER BY version DESC
		LIMIT 1`, organizationID)
	value, err := scanThresholdSet(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ThresholdSet{}, ErrNotFound
	}
	return value, err
}

func (r MySQLRepository) AppendThresholdSet(ctx context.Context, value ThresholdSet) (ThresholdSet, error) {
	_, err := r.DB.ExecContext(ctx, `INSERT INTO insight_threshold_sets (
		id, organization_id, version,
		sufficient_impressions, directional_impressions, min_trend_days, min_anomaly_days,
		min_driver_assets, max_comparison_assets, quality_window_days,
		reason, changed_by, changed_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		value.ID, value.OrganizationID, value.Version,
		nullableInt(value.Values.SufficientImpressions), nullableInt(value.Values.DirectionalImpressions),
		nullableInt(value.Values.MinTrendDays), nullableInt(value.Values.MinAnomalyDays),
		nullableInt(value.Values.MinDriverAssets), nullableInt(value.Values.MaxComparisonAssets),
		nullableInt(value.Values.QualityWindowDays),
		value.Reason, value.ChangedBy, value.ChangedAt)
	// 版本号撞车说明有人在这中间也存了一版。报冲突让后存的人重来一次，
	// 不要自己 +1 重试——两个人同时改判定标准，后来的那个必须先看到
	// 前一个改成了什么，否则他会在一个自己没见过的基准上再改一刀。
	if isDuplicateKey(err) {
		return ThresholdSet{}, ErrVersionConflict
	}
	if err != nil {
		return ThresholdSet{}, err
	}
	return value, nil
}

func (r MySQLRepository) ListThresholdSets(ctx context.Context,
	organizationID contract.OrganizationID, limit int) ([]ThresholdSet, error) {
	rows, err := r.DB.QueryContext(ctx, thresholdSetSelect+`
		WHERE organization_id = ?
		ORDER BY version DESC
		LIMIT ?`, organizationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []ThresholdSet{}
	for rows.Next() {
		value, scanErr := scanThresholdSet(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

// scanner 让单行查询和多行遍历共用一份列顺序。列顺序抄两遍的话，
// 加一列的时候漏改一处，读出来的是两列互换的值而不是一个报错。
type scanner interface {
	Scan(dest ...any) error
}

func scanThresholdSet(row scanner) (ThresholdSet, error) {
	var value ThresholdSet
	var sufficient, directional, trendDays, anomalyDays sql.NullInt64
	var driverAssets, comparisonAssets, windowDays sql.NullInt64
	err := row.Scan(&value.ID, &value.OrganizationID, &value.Version,
		&sufficient, &directional, &trendDays, &anomalyDays,
		&driverAssets, &comparisonAssets, &windowDays,
		&value.Reason, &value.ChangedBy, &value.ChangedAt)
	if err != nil {
		return ThresholdSet{}, err
	}
	value.Values = Thresholds{
		SufficientImpressions:  intFromNull(sufficient),
		DirectionalImpressions: intFromNull(directional),
		MinTrendDays:           intFromNull(trendDays),
		MinAnomalyDays:         intFromNull(anomalyDays),
		MinDriverAssets:        intFromNull(driverAssets),
		MaxComparisonAssets:    intFromNull(comparisonAssets),
		QualityWindowDays:      intFromNull(windowDays),
	}
	return value, nil
}

// NULL 与 nil 必须一一对应，不能在这里把 NULL 读成 0。
// 读成 0 的话，一个「没人调过」的格子会变成「有人把它调成了 0」，
// 而 0 是校验会拒绝的值——库里于是躺着一份存不回去的配置。
func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func intFromNull(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	converted := int(value.Int64)
	return &converted
}
