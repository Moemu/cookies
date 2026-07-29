package strategy

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/agent"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
	"github.com/shikanon/cookies/internal/platform/provider"
)

func (s Service) StartDeepReview(
	ctx context.Context,
	actor contract.ActorContext,
	key contract.IdempotencyKey,
	reviewID string,
	request StartDeepReviewRequest,
) (DeepReviewStartResult, bool, error) {
	if err := requireScope(actor, ScopeReview); err != nil {
		return DeepReviewStartResult{}, false, err
	}
	if err := key.Validate(); err != nil || strings.TrimSpace(reviewID) == "" ||
		request.ExpectedReviewStatus != "open" {
		return DeepReviewStartResult{}, false, ErrInvalidRequest
	}
	review, err := s.GetReview(ctx, actor, reviewID)
	if err != nil {
		return DeepReviewStartResult{}, false, err
	}
	if review.Status != request.ExpectedReviewStatus {
		return DeepReviewStartResult{}, false, ErrVersionConflict
	}
	if err := s.ensureConcurrencyLimit(ctx, actor.OrganizationID, review.ProjectID, 4); err != nil {
		return DeepReviewStartResult{}, false, err
	}
	modelAlias := strings.TrimSpace(s.DeepReviewModelAlias)
	if modelAlias == "" {
		modelAlias = "cookies.text.deep_review"
	}
	if s.Text == nil {
		return DeepReviewStartResult{}, false, ErrGenerationUnavailable
	}
	inspection, err := s.Text.InspectTextRoute(ctx, actor.OrganizationID, modelAlias)
	if err != nil || !inspection.Ready || inspection.APIMode != provider.TextAPIResponses || !inspection.Background {
		return DeepReviewStartResult{}, false, ErrGenerationUnavailable
	}
	requestHash, _ := contract.CanonicalJSONHash(struct {
		ReviewID string                 `json:"review_id"`
		Request  StartDeepReviewRequest `json:"request"`
	}{ReviewID: reviewID, Request: request})
	var prior DeepReviewStartResult
	found, err := s.loadReceipt(ctx, actor, review.ProjectID, "strategy.review.deep", key, requestHash, &prior)
	if found || err != nil {
		return prior, found, err
	}
	analysisID, err := s.newID("deepreview")
	if err != nil {
		return DeepReviewStartResult{}, false, err
	}
	taskID, err := s.newID("agenttask")
	if err != nil {
		return DeepReviewStartResult{}, false, err
	}
	now := s.now()
	input := map[string]any{
		"analysis_id": analysisID, "review_id": review.ID, "strategy_id": review.StrategyID,
		"candidate_revision": review.CandidateRevision, "candidate_content_hash": review.CandidateContentHash,
		"model_alias": modelAlias,
	}
	task := agent.Task{
		ID: taskID, OrganizationID: actor.OrganizationID, ProjectID: review.ProjectID,
		SourceSystem: "strategy", SourceType: "strategy_review", SourceID: review.ID,
		Kind: AgentKindReviewDeep, Status: agent.TaskDispatchPending, Version: 1,
		InputSnapshot: mustJSON(input), CreatedBy: actor.Principal, CreatedAt: now, UpdatedAt: now,
	}
	analysis := DeepReviewAnalysis{
		ID: analysisID, OrganizationID: actor.OrganizationID, ProjectID: review.ProjectID,
		ReviewID: review.ID, StrategyID: review.StrategyID, CandidateRevision: review.CandidateRevision,
		CandidateContentHash: review.CandidateContentHash, AgentTaskID: task.ID, Status: "pending",
		Findings: []DeepReviewFinding{}, ModelAlias: modelAlias, APIMode: inspection.APIMode,
		Background: inspection.Background, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now,
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return DeepReviewStartResult{}, false, err
	}
	defer tx.Rollback()
	writer, err := s.agentWriter()
	if err != nil {
		return DeepReviewStartResult{}, false, err
	}
	if err := writer.CreateIn(ctx, tx, agent.CreateRequest{Task: task}); err != nil {
		return DeepReviewStartResult{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_review_analyses
		(id, organization_id, project_id, review_id, strategy_id, candidate_revision,
		 candidate_content_hash, agent_task_id, status, findings, model_alias, api_mode,
		 background, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', JSON_ARRAY(), ?, ?, ?, ?, ?, ?)`,
		analysis.ID, analysis.OrganizationID, analysis.ProjectID, analysis.ReviewID, analysis.StrategyID,
		analysis.CandidateRevision, analysis.CandidateContentHash, analysis.AgentTaskID,
		analysis.ModelAlias, analysis.APIMode, analysis.Background, analysis.CreatedBy, now, now); err != nil {
		return DeepReviewStartResult{}, false, err
	}
	result := DeepReviewStartResult{Analysis: analysis, AgentTask: task}
	if err := insertReceipt(ctx, tx, actor, review.ProjectID, "strategy.review.deep", key, requestHash, 202, result, now); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(ctx, actor, review.ProjectID, "strategy.review.deep", key, requestHash, &prior)
			return prior, found, readErr
		}
		return DeepReviewStartResult{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return DeepReviewStartResult{}, false, err
	}
	return result, false, nil
}

func (s Service) GetLatestDeepReview(ctx context.Context, actor contract.ActorContext, reviewID string) (DeepReviewAnalysis, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return DeepReviewAnalysis{}, err
	}
	review, err := s.GetReview(ctx, actor, reviewID)
	if err != nil {
		return DeepReviewAnalysis{}, err
	}
	return scanDeepReview(s.DB.QueryRowContext(ctx, deepReviewSelect+`
		WHERE organization_id = ? AND project_id = ? AND review_id = ?
		ORDER BY created_at DESC LIMIT 1`, actor.OrganizationID, review.ProjectID, reviewID))
}

func (s Service) handleDeepReview(ctx context.Context, task agent.Task) (*contract.ResourceRef, error) {
	var input struct {
		AnalysisID           string               `json:"analysis_id"`
		ReviewID             string               `json:"review_id"`
		StrategyID           string               `json:"strategy_id"`
		CandidateRevision    int64                `json:"candidate_revision"`
		CandidateContentHash contract.ContentHash `json:"candidate_content_hash"`
		ModelAlias           string               `json:"model_alias"`
	}
	if err := json.Unmarshal(task.InputSnapshot, &input); err != nil {
		return nil, err
	}
	review, err := scanReview(s.DB.QueryRowContext(ctx, reviewSelect+`
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		task.OrganizationID, task.ProjectID, input.ReviewID))
	if err != nil {
		return nil, err
	}
	if review.StrategyID != input.StrategyID || review.CandidateRevision != input.CandidateRevision ||
		review.CandidateContentHash != input.CandidateContentHash {
		return nil, ErrReviewStale
	}
	revision, err := scanDraftRevision(s.DB.QueryRowContext(ctx, draftRevisionSelect+`
		WHERE organization_id = ? AND project_id = ? AND strategy_id = ? AND revision = ?`,
		task.OrganizationID, task.ProjectID, input.StrategyID, input.CandidateRevision))
	if err != nil {
		return nil, err
	}
	actor := contract.ActorContext{
		OrganizationID: task.OrganizationID, Principal: task.CreatedBy,
		Scopes: []contract.Scope{provider.ScopeTextGenerate},
	}
	project, err := s.Projects.GetContext(ctx, actor, task.ProjectID)
	if err != nil {
		return nil, err
	}
	started := time.Now()
	response, err := s.Text.GenerateText(ctx, provider.TextGenerateRequest{
		Actor: actor, Project: project, ModelAlias: input.ModelAlias,
		InvocationKey: contract.IdempotencyKey("agent-" + task.ID + "-deep-review"),
		Messages: []provider.TextMessage{
			{Role: provider.TextRoleSystem, Content: "You are a senior advertising strategy reviewer. Analyze the candidate rigorously, cite exact strategy sections, prioritize business risk, evidence gaps, channel coherence, measurability, and execution feasibility. Do not approve or reject; provide decision support for the human reviewer."},
			{Role: provider.TextRoleUser, Content: fmt.Sprintf(
				"Candidate revision %d with content hash %s:\n%s",
				revision.Revision, revision.ContentHash, mustJSON(revision.Document),
			)},
		},
		OutputJSONSchema: deepReviewOutputSchema(),
	})
	if err != nil {
		return nil, textGenerationError(err)
	}
	candidate := response.StructuredOutput
	if len(candidate) == 0 {
		candidate = normalizeJSONCandidate(response.Text)
	}
	var output struct {
		Summary  string              `json:"summary"`
		Findings []DeepReviewFinding `json:"findings"`
	}
	if err := json.Unmarshal(candidate, &output); err != nil {
		return nil, jobruntime.ExecutionError{JobError: contract.JobError{Code: "MODEL_OUTPUT_INVALID", Message: "Deep review output is invalid"}}
	}
	if strings.TrimSpace(output.Summary) == "" || len(output.Findings) == 0 {
		return nil, jobruntime.ExecutionError{JobError: contract.JobError{Code: "MODEL_OUTPUT_INVALID", Message: "Deep review output is incomplete"}}
	}
	for _, finding := range output.Findings {
		if finding.Severity != "blocker" && finding.Severity != "warning" && finding.Severity != "opportunity" {
			return nil, jobruntime.ExecutionError{JobError: contract.JobError{Code: "MODEL_OUTPUT_INVALID", Message: "Deep review severity is invalid"}}
		}
	}
	trace := SkillExecutionTrace{
		GenerationMode: "provider", ProviderCode: response.ProviderCode, ModelAlias: input.ModelAlias,
		ModelVersion: response.ModelVersion, RouteRevisionID: response.RouteRevisionID,
		ResponseMode: response.ResponseMode, PromptVersion: "strategy.review.deep.v1",
		SkillVersions: map[string]string{"strategy.review.deep": "v1.0.0"},
		Usage:         response.Usage, LatencyMS: time.Since(started).Milliseconds(), ValidationAttempts: 1,
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE strategy_review_analyses SET
		status = 'succeeded', summary = ?, findings = ?, model_version = ?, route_revision_id = ?,
		response_mode = ?, api_mode = ?, background = ?, usage_json = ?, latency_ms = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND status = 'pending'`,
		output.Summary, mustJSON(output.Findings), response.ModelVersion, response.RouteRevisionID,
		response.ResponseMode, response.APIMode, response.Background, nullableJSON(response.Usage),
		trace.LatencyMS, s.now(), task.OrganizationID, task.ProjectID, input.AnalysisID)
	if err != nil {
		return nil, err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return nil, ErrVersionConflict
	}
	if _, err := s.insertSkillRun(ctx, tx, task, "strategy.review.deep", "v1.0.0", trace, output); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	version := input.CandidateRevision
	return &contract.ResourceRef{Type: "strategy.review_analysis", ID: input.AnalysisID, Version: &version}, nil
}

const deepReviewSelect = `SELECT id, organization_id, project_id, review_id, strategy_id,
	candidate_revision, candidate_content_hash, agent_task_id, status, COALESCE(summary, ''),
	findings, COALESCE(model_alias, ''), COALESCE(model_version, ''),
	COALESCE(route_revision_id, ''), COALESCE(response_mode, ''), COALESCE(api_mode, ''),
	background, usage_json, COALESCE(latency_ms, 0), created_by, created_at, updated_at
	FROM strategy_review_analyses`

func scanDeepReview(row rowScanner) (DeepReviewAnalysis, error) {
	var value DeepReviewAnalysis
	var findingsJSON, usageJSON []byte
	if err := row.Scan(
		&value.ID, &value.OrganizationID, &value.ProjectID, &value.ReviewID, &value.StrategyID,
		&value.CandidateRevision, &value.CandidateContentHash, &value.AgentTaskID, &value.Status,
		&value.Summary, &findingsJSON, &value.ModelAlias, &value.ModelVersion,
		&value.RouteRevisionID, &value.ResponseMode, &value.APIMode, &value.Background,
		&usageJSON, &value.LatencyMS, &value.CreatedBy, &value.CreatedAt, &value.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DeepReviewAnalysis{}, ErrNotFound
		}
		return DeepReviewAnalysis{}, err
	}
	if err := json.Unmarshal(findingsJSON, &value.Findings); err != nil {
		return DeepReviewAnalysis{}, err
	}
	if len(usageJSON) > 0 && string(usageJSON) != "null" {
		value.Usage = &provider.TokenUsage{}
		if err := json.Unmarshal(usageJSON, value.Usage); err != nil {
			return DeepReviewAnalysis{}, err
		}
	}
	return value, nil
}

func deepReviewOutputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"summary":{"type":"string"},
			"findings":{"type":"array","minItems":1,"maxItems":12,"items":{
				"type":"object",
				"properties":{
					"severity":{"type":"string","enum":["blocker","warning","opportunity"]},
					"section":{"type":"string"},
					"title":{"type":"string"},
					"detail":{"type":"string"},
					"recommendation":{"type":"string"}
				},
				"required":["severity","section","title","detail","recommendation"],
				"additionalProperties":false
			}}
		},
		"required":["summary","findings"],
		"additionalProperties":false
	}`)
}

func nullableJSON(value any) any {
	if value == nil {
		return nil
	}
	return mustJSON(value)
}
