package strategy

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/shikanon/cookies/internal/platform/contract"
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
