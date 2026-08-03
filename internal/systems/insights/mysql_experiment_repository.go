package insights

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// MySQL implementation of ExperimentRepository.
// 表见 migrations/insights/20260730160000_insight_experiments.up.sql。

const experimentSelect = `SELECT id, organization_id, project_id, title, hypothesis, source_experience_id,
	asset_type, variable_key, variable_label, controlled_keys, min_impressions,
	window_start, window_end, status, verdict, interpretation, concluded_by, concluded_at,
	started_at, version, created_by, created_at, updated_at
	FROM insight_experiments`

const variantSelect = `SELECT id, organization_id, project_id, experiment_id, name, variable_value,
	is_baseline, asset_ids, position, created_at, updated_at
	FROM insight_experiment_variants`

// CreateExperiment 把实验和它的分组写在同一个事务里。
// 分开写会留下「有实验、没有分组」的半截数据，而那种实验在页面上看起来完全正常。
func (r MySQLRepository) CreateExperiment(ctx context.Context, value Experiment) (Experiment, error) {
	controlled, err := marshalStringsColumn(value.ControlledKeys)
	if err != nil {
		return Experiment{}, err
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return Experiment{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `INSERT INTO insight_experiments (
		id, organization_id, project_id, title, hypothesis, source_experience_id,
		asset_type, variable_key, variable_label, controlled_keys, min_impressions,
		window_start, window_end, status, verdict, interpretation, concluded_by, concluded_at,
		started_at, version, created_by, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', '', '', NULL, NULL, ?, ?, ?, ?)`,
		value.ID, value.OrganizationID, value.ProjectID, value.Title, value.Hypothesis, value.SourceExperienceID,
		value.AssetType, value.VariableKey, value.VariableLabel, controlled, value.MinImpressions,
		value.WindowStart, value.WindowEnd, value.Status,
		value.Version, value.CreatedBy, value.CreatedAt, value.UpdatedAt); err != nil {
		return Experiment{}, err
	}
	for _, variant := range value.Variants {
		assetIDs, marshalErr := json.Marshal(variant.AssetIDs)
		if marshalErr != nil {
			return Experiment{}, marshalErr
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO insight_experiment_variants (
			id, organization_id, project_id, experiment_id, name, variable_value,
			is_baseline, asset_ids, position, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			variant.ID, variant.OrganizationID, variant.ProjectID, variant.ExperimentID,
			variant.Name, variant.VariableValue, variant.IsBaseline, assetIDs, variant.Position,
			variant.CreatedAt, variant.UpdatedAt); err != nil {
			return Experiment{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return Experiment{}, err
	}
	return value, nil
}

func (r MySQLRepository) ListExperiments(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, filter ExperimentFilter) ([]Experiment, error) {
	query := experimentSelect + ` WHERE organization_id = ? AND project_id = ?`
	args := []any{organizationID, projectID}
	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, filter.Status)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, filter.Limit)

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]Experiment, 0)
	ids := make([]string, 0)
	for rows.Next() {
		value, scanErr := scanExperiment(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
		ids = append(ids, value.ID)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return values, nil
	}
	// 一次把所有分组读回来再贴上去。列表页要显示每个实验有几组、各组几条素材，
	// 逐个实验查一次分组会在列表长起来之后变成几十次往返。
	byExperiment, err := r.loadVariants(ctx, organizationID, ids)
	if err != nil {
		return nil, err
	}
	for index := range values {
		values[index].Variants = byExperiment[values[index].ID]
	}
	return values, nil
}

func (r MySQLRepository) GetExperiment(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (Experiment, error) {
	value, err := scanExperiment(r.DB.QueryRowContext(ctx,
		experimentSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`, organizationID, projectID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Experiment{}, ErrNotFound
	}
	if err != nil {
		return Experiment{}, err
	}
	byExperiment, err := r.loadVariants(ctx, organizationID, []string{id})
	if err != nil {
		return Experiment{}, err
	}
	value.Variants = byExperiment[id]
	return value, nil
}

func (r MySQLRepository) loadVariants(ctx context.Context, organizationID contract.OrganizationID, experimentIDs []string) (map[string][]ExperimentVariant, error) {
	args := make([]any, 0, len(experimentIDs)+1)
	args = append(args, organizationID)
	for _, id := range experimentIDs {
		args = append(args, id)
	}
	rows, err := r.DB.QueryContext(ctx,
		variantSelect+` WHERE organization_id = ? AND experiment_id IN (`+placeholders(len(experimentIDs))+`) ORDER BY position, id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string][]ExperimentVariant{}
	for rows.Next() {
		variant, scanErr := scanExperimentVariant(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result[variant.ExperimentID] = append(result[variant.ExperimentID], variant)
	}
	return result, rows.Err()
}

// UpdateVariantAssets 改一组的素材清单。
//
// WHERE 里带 status = 'draft'：**开跑之后不能再动分组**。服务层已经拦过一次，
// 这里再拦一次是因为那次拦截和这次写入之间隔着一个网络往返——两个人同时操作时，
// 一个人点了开跑、另一个人正好在加素材，只有这个条件能保证后者失败。
func (r MySQLRepository) UpdateVariantAssets(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, experimentID, variantID string, assetIDs []string, now time.Time) (ExperimentVariant, error) {
	if assetIDs == nil {
		assetIDs = []string{}
	}
	encoded, err := json.Marshal(assetIDs)
	if err != nil {
		return ExperimentVariant{}, err
	}
	result, err := r.DB.ExecContext(ctx, `UPDATE insight_experiment_variants SET asset_ids = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND experiment_id = ? AND id = ?
		  AND EXISTS (SELECT 1 FROM insight_experiments e
		              WHERE e.organization_id = insight_experiment_variants.organization_id
		                AND e.id = insight_experiment_variants.experiment_id
		                AND e.status = 'draft')`,
		encoded, now, organizationID, projectID, experimentID, variantID)
	if err != nil {
		return ExperimentVariant{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return ExperimentVariant{}, err
	}
	if affected == 0 {
		return ExperimentVariant{}, ErrInvalidState
	}
	variant, err := scanExperimentVariant(r.DB.QueryRowContext(ctx,
		variantSelect+` WHERE organization_id = ? AND id = ?`, organizationID, variantID))
	if errors.Is(err, sql.ErrNoRows) {
		return ExperimentVariant{}, ErrNotFound
	}
	return variant, err
}

func (r MySQLRepository) UpdateExperimentStatus(ctx context.Context, input UpdateExperimentStatusInput) (Experiment, error) {
	query := `UPDATE insight_experiments SET status = ?, version = version + 1, updated_at = ?`
	args := []any{input.To, input.Now}
	switch input.To {
	case ExperimentRunning:
		query += `, started_at = ?`
		args = append(args, input.Now)
	case ExperimentConcluded:
		query += `, verdict = ?, interpretation = ?, concluded_by = ?, concluded_at = ?`
		args = append(args, input.Verdict, input.Interpretation, input.ActorID, input.Now)
	}
	query += ` WHERE organization_id = ? AND project_id = ? AND id = ? AND status = ?`
	args = append(args, input.OrganizationID, input.ProjectID, input.ID, input.From)
	if input.ExpectedVersion > 0 {
		query += ` AND version = ?`
		args = append(args, input.ExpectedVersion)
	}
	result, err := r.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return Experiment{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Experiment{}, err
	}
	if affected == 0 {
		// 分不清是版本对不上还是状态已经变了，就都按版本冲突报：
		// 两种情况下用户要做的事一样——刷新，再看一眼现在是什么状态。
		return Experiment{}, ErrVersionConflict
	}
	return r.GetExperiment(ctx, input.OrganizationID, input.ProjectID, input.ID)
}

func scanExperiment(row rowScanner) (Experiment, error) {
	var value Experiment
	var controlled []byte
	var concludedAt, startedAt sql.NullTime
	if err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.Title, &value.Hypothesis,
		&value.SourceExperienceID, &value.AssetType, &value.VariableKey, &value.VariableLabel, &controlled,
		&value.MinImpressions, &value.WindowStart, &value.WindowEnd, &value.Status, &value.Verdict,
		&value.Interpretation, &value.ConcludedBy, &concludedAt, &startedAt,
		&value.Version, &value.CreatedBy, &value.CreatedAt, &value.UpdatedAt); err != nil {
		return Experiment{}, err
	}
	value.ControlledKeys = make([]string, 0)
	if len(controlled) > 0 {
		if err := json.Unmarshal(controlled, &value.ControlledKeys); err != nil {
			return Experiment{}, err
		}
	}
	if concludedAt.Valid {
		value.ConcludedAt = &concludedAt.Time
	}
	if startedAt.Valid {
		value.StartedAt = &startedAt.Time
	}
	// 空切片而非 nil：nil 序列化成 null，前端一个 .map 就白屏。
	value.Variants = make([]ExperimentVariant, 0)
	return value, nil
}

func scanExperimentVariant(row rowScanner) (ExperimentVariant, error) {
	var value ExperimentVariant
	var assetIDs []byte
	if err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.ExperimentID,
		&value.Name, &value.VariableValue, &value.IsBaseline, &assetIDs, &value.Position,
		&value.CreatedAt, &value.UpdatedAt); err != nil {
		return ExperimentVariant{}, err
	}
	value.AssetIDs = make([]string, 0)
	if len(assetIDs) > 0 {
		if err := json.Unmarshal(assetIDs, &value.AssetIDs); err != nil {
			return ExperimentVariant{}, err
		}
	}
	return value, nil
}
