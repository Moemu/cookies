package strategy

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

type GenerationReadiness struct {
	Ready           bool                      `json:"ready"`
	GenerationMode  string                    `json:"generation_mode"`
	ModelAlias      string                    `json:"model_alias,omitempty"`
	UpstreamModel   string                    `json:"upstream_model,omitempty"`
	RouteRevisionID string                    `json:"route_revision_id,omitempty"`
	ResponseMode    provider.TextResponseMode `json:"response_mode,omitempty"`
	APIMode         provider.TextAPIMode      `json:"api_mode,omitempty"`
	Background      bool                      `json:"background,omitempty"`
	PromptVersion   string                    `json:"prompt_version"`
	ReasonCode      string                    `json:"reason_code,omitempty"`
}

type GenerationProbe struct {
	Ready           bool                      `json:"ready"`
	ProviderCode    string                    `json:"provider_code"`
	ModelAlias      string                    `json:"model_alias"`
	ModelVersion    string                    `json:"model_version"`
	RouteRevisionID string                    `json:"route_revision_id,omitempty"`
	ResponseMode    provider.TextResponseMode `json:"response_mode,omitempty"`
	APIMode         provider.TextAPIMode      `json:"api_mode,omitempty"`
	Background      bool                      `json:"background,omitempty"`
	Usage           *provider.TokenUsage      `json:"usage,omitempty"`
	LatencyMS       int64                     `json:"latency_ms"`
}

type GenerationMetadata struct {
	GenerationMode        string                    `json:"generation_mode"`
	ProviderCode          string                    `json:"provider_code,omitempty"`
	ModelAlias            string                    `json:"model_alias,omitempty"`
	ModelVersion          string                    `json:"model_version,omitempty"`
	RouteRevisionID       string                    `json:"route_revision_id,omitempty"`
	ResponseMode          provider.TextResponseMode `json:"response_mode,omitempty"`
	PromptVersion         string                    `json:"prompt_version,omitempty"`
	SkillVersions         map[string]string         `json:"skill_versions"`
	SkillSnapshotHashes   map[string]string         `json:"skill_snapshot_hashes"`
	GenerationContextHash string                    `json:"generation_context_hash,omitempty"`
	OutputHash            string                    `json:"output_hash,omitempty"`
	Usage                 *provider.TokenUsage      `json:"usage,omitempty"`
	LatencyMS             int64                     `json:"latency_ms,omitempty"`
	ValidationAttempts    int                       `json:"validation_attempts"`
	QualityReport         *QualityReport            `json:"quality_report,omitempty"`
}

func (s Service) GetGenerationReadiness(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) (GenerationReadiness, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return GenerationReadiness{}, err
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return GenerationReadiness{}, err
	}
	promptVersion := strings.TrimSpace(s.PromptVersion)
	if promptVersion == "" {
		promptVersion = "strategy.generate.v2"
	}
	if s.Text == nil {
		return GenerationReadiness{
			Ready: true, GenerationMode: "deterministic", PromptVersion: promptVersion,
			ReasonCode: "REAL_PROVIDER_DISABLED",
		}, nil
	}
	inspection, err := s.Text.InspectTextRoute(ctx, actor.OrganizationID, s.TextModelAlias)
	if err != nil {
		return GenerationReadiness{
			Ready: false, GenerationMode: "provider", ModelAlias: s.TextModelAlias,
			PromptVersion: promptVersion, ReasonCode: generationReadinessReason(err),
		}, nil
	}
	return GenerationReadiness{
		Ready: inspection.Ready, GenerationMode: "provider", ModelAlias: inspection.ModelAlias,
		UpstreamModel: inspection.UpstreamModel, RouteRevisionID: inspection.RouteRevisionID,
		ResponseMode: inspection.ResponseMode, APIMode: inspection.APIMode,
		Background: inspection.Background, PromptVersion: promptVersion,
	}, nil
}

func (s Service) ProbeGeneration(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
) (GenerationProbe, error) {
	return s.ProbeGenerationProfile(ctx, actor, projectID, "")
}

func (s Service) ProbeGenerationProfile(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	profile string,
) (GenerationProbe, error) {
	if err := requireScope(actor, ScopeWrite); err != nil {
		return GenerationProbe{}, err
	}
	project, err := s.project(ctx, actor, projectID)
	if err != nil {
		return GenerationProbe{}, err
	}
	if s.Text == nil {
		return GenerationProbe{}, ErrGenerationUnavailable
	}
	profile = strings.TrimSpace(profile)
	var modelAlias string
	switch profile {
	case "":
		modelAlias = strings.TrimSpace(s.TextModelAlias)
		if modelAlias == "" {
			modelAlias = "cookies.text.standard"
		}
	case "deep_review":
		modelAlias = strings.TrimSpace(s.DeepReviewModelAlias)
		if modelAlias == "" {
			modelAlias = "cookies.text.deep_review"
		}
	default:
		return GenerationProbe{}, ErrInvalidRequest
	}
	invocationID, err := s.newID("generationprobe")
	if err != nil {
		return GenerationProbe{}, err
	}
	started := time.Now()
	response, err := s.Text.GenerateText(ctx, provider.TextGenerateRequest{
		Actor: actor, Project: project, ModelAlias: modelAlias,
		InvocationKey: contract.IdempotencyKey(invocationID),
		Messages: []provider.TextMessage{
			{Role: provider.TextRoleSystem, Content: "Return the requested structured health check only."},
			{Role: provider.TextRoleUser, Content: "Return ok=true."},
		},
		OutputJSONSchema: json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}},"required":["ok"],"additionalProperties":false}`),
	})
	if err != nil {
		return GenerationProbe{}, err
	}
	candidate := response.StructuredOutput
	if len(candidate) == 0 {
		candidate = normalizeJSONCandidate(response.Text)
	}
	var value struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(candidate, &value); err != nil || !value.OK {
		return GenerationProbe{}, ErrGenerationUnavailable
	}
	return GenerationProbe{
		Ready: true, ProviderCode: response.ProviderCode, ModelAlias: response.ModelAlias,
		ModelVersion: response.ModelVersion, RouteRevisionID: response.RouteRevisionID,
		ResponseMode: response.ResponseMode, APIMode: response.APIMode,
		Background: response.Background, Usage: response.Usage,
		LatencyMS: time.Since(started).Milliseconds(),
	}, nil
}

func generationReadinessReason(err error) string {
	var execution provider.ExecutionError
	if errors.As(err, &execution) && execution.JobError.Code == "MODEL_AUTH_UNAVAILABLE" {
		return "PROVIDER_CREDENTIAL_UNAVAILABLE"
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "no enabled adapter gateway"):
		return "MODEL_ROUTE_NOT_FOUND"
	case strings.Contains(message, "master key"), strings.Contains(message, "credential"):
		return "PROVIDER_CREDENTIAL_UNAVAILABLE"
	case strings.Contains(message, "base url"), strings.Contains(message, "response mode"),
		strings.Contains(message, "api mode"), strings.Contains(message, "output tokens"),
		strings.Contains(message, "temperature"):
		return "MODEL_ROUTE_POLICY_INVALID"
	default:
		return "PROVIDER_PREFLIGHT_FAILED"
	}
}

func (s Service) ensureTextProviderReady(ctx context.Context, organizationID contract.OrganizationID) error {
	if s.Text == nil {
		return nil
	}
	inspection, err := s.Text.InspectTextRoute(ctx, organizationID, s.TextModelAlias)
	if err != nil || !inspection.Ready {
		return ErrGenerationUnavailable
	}
	return nil
}

func (s Service) GetGenerationMetadata(ctx context.Context, actor contract.ActorContext, strategyID string) (GenerationMetadata, error) {
	draft, err := s.GetDraft(ctx, actor, strategyID)
	if err != nil {
		return GenerationMetadata{}, err
	}
	var value GenerationMetadata
	var providerCode, modelVersion, generationMode, modelAlias, routeRevisionID, responseMode, promptVersion sql.NullString
	var generationContextHash, outputHash sql.NullString
	var inputTokens, outputTokens, totalTokens, latencyMS sql.NullInt64
	var skillsJSON, skillHashesJSON, qualityJSON []byte
	err = s.DB.QueryRowContext(ctx, `SELECT
			sr.provider_code, sr.model_version, sr.generation_mode, sr.model_alias,
			sr.route_revision_id, sr.response_mode, sr.prompt_version,
			COALESCE(sr.skill_versions, JSON_OBJECT()),
			COALESCE(sr.skill_snapshot_hashes, JSON_OBJECT()),
			sr.generation_context_hash, sr.output_hash,
			sr.input_tokens, sr.output_tokens, sr.total_tokens, sr.latency_ms,
			sr.validation_attempts, sr.quality_report
		FROM platform_skill_runs sr
		JOIN platform_agent_tasks at ON at.id = sr.agent_task_id
			AND at.organization_id = sr.organization_id AND at.project_id = sr.project_id
		WHERE sr.organization_id = ? AND sr.project_id = ?
			AND at.source_system = 'strategy' AND at.source_type = 'strategy_draft'
			AND at.source_id = ?
			AND sr.skill_name IN ('strategy.strategy.generate', 'strategy.strategy.revise')
			AND sr.status = 'succeeded'
		ORDER BY sr.completed_at DESC, sr.id DESC LIMIT 1`,
		actor.OrganizationID, draft.ProjectID, strategyID,
	).Scan(
		&providerCode, &modelVersion, &generationMode, &modelAlias,
		&routeRevisionID, &responseMode, &promptVersion, &skillsJSON, &skillHashesJSON,
		&generationContextHash, &outputHash,
		&inputTokens, &outputTokens, &totalTokens, &latencyMS,
		&value.ValidationAttempts, &qualityJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return GenerationMetadata{}, ErrNotFound
	}
	if err != nil {
		return GenerationMetadata{}, err
	}
	value.ProviderCode = providerCode.String
	value.ModelVersion = modelVersion.String
	value.GenerationMode = generationMode.String
	value.ModelAlias = modelAlias.String
	value.RouteRevisionID = routeRevisionID.String
	value.ResponseMode = provider.TextResponseMode(responseMode.String)
	value.PromptVersion = promptVersion.String
	value.GenerationContextHash = generationContextHash.String
	value.OutputHash = outputHash.String
	value.LatencyMS = latencyMS.Int64
	value.SkillVersions = map[string]string{}
	if len(skillsJSON) > 0 {
		if err := json.Unmarshal(skillsJSON, &value.SkillVersions); err != nil {
			return GenerationMetadata{}, err
		}
	}
	value.SkillSnapshotHashes = map[string]string{}
	if len(skillHashesJSON) > 0 {
		if err := json.Unmarshal(skillHashesJSON, &value.SkillSnapshotHashes); err != nil {
			return GenerationMetadata{}, err
		}
	}
	if inputTokens.Valid || outputTokens.Valid || totalTokens.Valid {
		value.Usage = &provider.TokenUsage{
			InputTokens: inputTokens.Int64, OutputTokens: outputTokens.Int64, TotalTokens: totalTokens.Int64,
		}
	}
	if len(qualityJSON) > 0 && string(qualityJSON) != "null" {
		var report QualityReport
		if err := json.Unmarshal(qualityJSON, &report); err != nil {
			return GenerationMetadata{}, err
		}
		value.QualityReport = &report
	}
	return value, nil
}
