package strategy

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

func (s Service) ListSkillRuns(ctx context.Context, actor contract.ActorContext, agentTaskID string) ([]SkillRun, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return nil, err
	}
	var projectID contract.ProjectID
	if err := s.DB.QueryRowContext(ctx, `SELECT project_id FROM platform_agent_tasks
		WHERE organization_id = ? AND id = ?`, actor.OrganizationID, agentTaskID).Scan(&projectID); err != nil {
		return nil, err
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, skillRunSelect+` WHERE organization_id = ? AND project_id = ?
		AND agent_task_id = ? ORDER BY created_at ASC`, actor.OrganizationID, projectID, agentTaskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []SkillRun{}
	for rows.Next() {
		value, err := scanSkillRun(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range values {
		values[index].Attempts, err = s.listAttemptsForSkillRun(
			ctx, actor.OrganizationID, projectID, values[index].ID,
		)
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}

func (s Service) listAttemptsForSkillRun(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	skillRunID string,
) ([]SkillRunAttempt, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT attempt_no, purpose, provider_code,
		model_alias, model_version, route_revision_id, response_mode, api_mode, background,
		prompt_version, input_tokens, output_tokens, total_tokens, latency_ms,
		validation_passed, validation_errors, output_hash, created_at
		FROM platform_skill_run_attempts
		WHERE organization_id = ? AND project_id = ? AND skill_run_id = ?
		ORDER BY attempt_no ASC`,
		organizationID, projectID, skillRunID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []SkillRunAttempt{}
	for rows.Next() {
		var value SkillRunAttempt
		var modelAlias, modelVersion, routeRevisionID, responseMode, apiMode, promptVersion sql.NullString
		var inputTokens, outputTokens, totalTokens sql.NullInt64
		var validationErrors []byte
		var outputHash sql.NullString
		if err := rows.Scan(
			&value.AttemptNo, &value.Purpose, &value.ProviderCode,
			&modelAlias, &modelVersion, &routeRevisionID, &responseMode, &apiMode, &value.Background,
			&promptVersion, &inputTokens, &outputTokens, &totalTokens, &value.LatencyMS,
			&value.ValidationPassed, &validationErrors, &outputHash, &value.CreatedAt,
		); err != nil {
			return nil, err
		}
		value.ModelAlias = modelAlias.String
		value.ModelVersion = modelVersion.String
		value.RouteRevisionID = routeRevisionID.String
		value.ResponseMode = provider.TextResponseMode(responseMode.String)
		value.APIMode = provider.TextAPIMode(apiMode.String)
		value.PromptVersion = promptVersion.String
		value.OutputHash = outputHash.String
		value.ValidationErrors = []string{}
		if len(validationErrors) > 0 && string(validationErrors) != "null" {
			if err := json.Unmarshal(validationErrors, &value.ValidationErrors); err != nil {
				return nil, err
			}
		}
		if inputTokens.Valid || outputTokens.Valid || totalTokens.Valid {
			value.Usage = &provider.TokenUsage{
				InputTokens: inputTokens.Int64, OutputTokens: outputTokens.Int64,
				TotalTokens: totalTokens.Int64,
			}
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

const skillRunSelect = `SELECT id, organization_id, project_id, agent_task_id, skill_name,
	skill_version, status, input_hash, output_hash, provider_code, model_version,
	generation_mode, model_alias, prompt_version, generation_context_hash, latency_ms,
	validation_attempts, quality_report, started_at, completed_at FROM platform_skill_runs`

func scanSkillRun(row rowScanner) (SkillRun, error) {
	var value SkillRun
	var outputHash, providerCode, modelVersion, generationMode, modelAlias, promptVersion sql.NullString
	var generationContextHash sql.NullString
	var quality []byte
	err := row.Scan(
		&value.ID, &value.OrganizationID, &value.ProjectID, &value.AgentTaskID,
		&value.SkillName, &value.SkillVersion, &value.Status, &value.InputHash,
		&outputHash, &providerCode, &modelVersion, &generationMode, &modelAlias,
		&promptVersion, &generationContextHash, &value.LatencyMS,
		&value.ValidationAttempts, &quality, &value.StartedAt, &value.CompletedAt,
	)
	value.OutputHash = outputHash.String
	value.ProviderCode = providerCode.String
	value.ModelVersion = modelVersion.String
	value.GenerationMode = generationMode.String
	value.ModelAlias = modelAlias.String
	value.PromptVersion = promptVersion.String
	value.GenerationContextHash = generationContextHash.String
	if len(quality) > 0 {
		value.QualityReport = &QualityReport{}
		if json.Unmarshal(quality, value.QualityReport) != nil {
			return SkillRun{}, ErrInvalidState
		}
	}
	return value, err
}
