package insights

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// MySQL implementation of AnalysisRunRepository. Table is created by
// migrations/insights/20260730103000_insight_analysis_runs.up.sql.

const analysisRunSelect = `SELECT id, organization_id, project_id, run_kind, insight_asset_id, asset_type, status,
	skill_id, skill_version, skill_content_hash, prompt_version,
	provider_code, model_alias, model_version, route_revision_id, response_mode, generation_mode,
	input_hash, input_summary, result_hash, feature_count, dropped_fields, data_through,
	prompt_tokens, completion_tokens, latency_ms, error_code, error_message,
	started_at, finished_at, created_by, created_at, updated_at
	FROM insight_analysis_runs`

// CreateAnalysisRun 在任务开始时就落一行（status=running）。
//
// **不等结果出来再写。** 等结果的话，进程在调模型期间挂掉就什么都不剩，
// 而那正是最需要被看见的一类失败——它不会出现在任何成功率统计里。
func (r MySQLRepository) CreateAnalysisRun(ctx context.Context, value AnalysisRun) (AnalysisRun, error) {
	if err := value.validate(); err != nil {
		return AnalysisRun{}, err
	}
	inputSummary, err := marshalJSONColumn(value.InputSummary)
	if err != nil {
		return AnalysisRun{}, err
	}
	dropped, err := marshalStringsColumn(value.DroppedFields)
	if err != nil {
		return AnalysisRun{}, err
	}
	_, err = r.DB.ExecContext(ctx, `INSERT INTO insight_analysis_runs (
		id, organization_id, project_id, run_kind, insight_asset_id, asset_type, status,
		skill_id, skill_version, skill_content_hash, prompt_version,
		provider_code, model_alias, model_version, route_revision_id, response_mode, generation_mode,
		input_hash, input_summary, result_hash, feature_count, dropped_fields, data_through,
		prompt_tokens, completion_tokens, latency_ms, error_code, error_message,
		started_at, finished_at, created_by, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		value.ID, value.OrganizationID, value.ProjectID, value.Kind, value.AssetID, value.AssetType, value.Status,
		value.SkillID, value.SkillVersion, value.SkillContentHash, value.PromptVersion,
		value.ProviderCode, value.ModelAlias, value.ModelVersion, value.RouteRevisionID, value.ResponseMode, value.GenerationMode,
		value.InputHash, inputSummary, value.ResultHash, value.FeatureCount, dropped, value.DataThrough,
		value.PromptTokens, value.CompletionTokens, value.LatencyMS, value.ErrorCode, value.ErrorMessage,
		value.StartedAt, value.FinishedAt, value.CreatedBy, value.CreatedAt, value.UpdatedAt)
	if err != nil {
		return AnalysisRun{}, err
	}
	return value, nil
}

// FinishAnalysisRun 把 running 的那一行改成 succeeded 或 failed。
//
// WHERE 里带 status = 'running'：**一次运行只能被结束一次**。
// 少了这个条件，重试逻辑写错时会把已经失败的记录改成成功，
// 而成功率是拿这张表算的——那等于允许统计被覆盖掉。
func (r MySQLRepository) FinishAnalysisRun(ctx context.Context, value AnalysisRun) (AnalysisRun, error) {
	if err := value.validate(); err != nil {
		return AnalysisRun{}, err
	}
	if value.Status == AnalysisRunRunning {
		return AnalysisRun{}, ErrInvalidRequest
	}
	dropped, err := marshalStringsColumn(value.DroppedFields)
	if err != nil {
		return AnalysisRun{}, err
	}
	result, err := r.DB.ExecContext(ctx, `UPDATE insight_analysis_runs SET
		status = ?, provider_code = ?, model_alias = ?, model_version = ?, route_revision_id = ?,
		response_mode = ?, generation_mode = ?, result_hash = ?, feature_count = ?, dropped_fields = ?,
		prompt_tokens = ?, completion_tokens = ?, latency_ms = ?, error_code = ?, error_message = ?,
		finished_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND status = 'running'`,
		value.Status, value.ProviderCode, value.ModelAlias, value.ModelVersion, value.RouteRevisionID,
		value.ResponseMode, value.GenerationMode, value.ResultHash, value.FeatureCount, dropped,
		value.PromptTokens, value.CompletionTokens, value.LatencyMS, value.ErrorCode, value.ErrorMessage,
		value.FinishedAt, value.UpdatedAt,
		value.OrganizationID, value.ProjectID, value.ID)
	if err != nil {
		return AnalysisRun{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return AnalysisRun{}, err
	}
	if affected == 0 {
		return AnalysisRun{}, ErrNotFound
	}
	return value, nil
}

func (r MySQLRepository) ListAnalysisRuns(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, filter AnalysisRunFilter) ([]AnalysisRun, error) {
	query := analysisRunSelect + ` WHERE organization_id = ? AND project_id = ?`
	args := []any{organizationID, projectID}
	if filter.AssetID != "" {
		query += ` AND insight_asset_id = ?`
		args = append(args, filter.AssetID)
	}
	if filter.Kind != "" {
		query += ` AND run_kind = ?`
		args = append(args, filter.Kind)
	}
	if len(filter.Statuses) > 0 {
		query += ` AND status IN (` + placeholders(len(filter.Statuses)) + `)`
		for _, status := range filter.Statuses {
			args = append(args, status)
		}
	}
	// 最近的排在前面：「数据与方法」要看的是这条素材最后一次是怎么分析的。
	query += ` ORDER BY started_at DESC, id DESC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]AnalysisRun, 0)
	for rows.Next() {
		value, scanErr := scanAnalysisRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func scanAnalysisRun(row rowScanner) (AnalysisRun, error) {
	var value AnalysisRun
	var inputSummary, dropped []byte
	var dataThrough, finishedAt sql.NullTime
	if err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.Kind, &value.AssetID, &value.AssetType, &value.Status,
		&value.SkillID, &value.SkillVersion, &value.SkillContentHash, &value.PromptVersion,
		&value.ProviderCode, &value.ModelAlias, &value.ModelVersion, &value.RouteRevisionID, &value.ResponseMode, &value.GenerationMode,
		&value.InputHash, &inputSummary, &value.ResultHash, &value.FeatureCount, &dropped, &dataThrough,
		&value.PromptTokens, &value.CompletionTokens, &value.LatencyMS, &value.ErrorCode, &value.ErrorMessage,
		&value.StartedAt, &finishedAt, &value.CreatedBy, &value.CreatedAt, &value.UpdatedAt); err != nil {
		return AnalysisRun{}, err
	}
	if len(inputSummary) > 0 {
		value.InputSummary = json.RawMessage(inputSummary)
	}
	if len(dropped) > 0 {
		if err := json.Unmarshal(dropped, &value.DroppedFields); err != nil {
			return AnalysisRun{}, err
		}
	}
	if dataThrough.Valid {
		value.DataThrough = &dataThrough.Time
	}
	if finishedAt.Valid {
		value.FinishedAt = &finishedAt.Time
	}
	return value, nil
}

func marshalJSONColumn(value json.RawMessage) (any, error) {
	if len(value) == 0 {
		return nil, nil
	}
	return []byte(value), nil
}

// 空列表写 NULL 而不是 []。这一列的语义是「有没有东西被丢掉」，
// NULL 读作「没有」比空数组更直接，也省掉一行 JSON。
func marshalStringsColumn(values []string) (any, error) {
	if len(values) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}
