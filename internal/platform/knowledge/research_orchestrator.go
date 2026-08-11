package knowledge

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const researchFindingContractVersion = "strategy-research-finding/v1"

var errResearchProviderRetryable = errors.New("research provider call can be retried")

type researchProgress func(round int, status, message string) error

func researchStatusActive(status string) bool {
	switch status {
	case "queued", "planning", "searching", "reading", "cross_checking", "drafting", "auditing":
		return true
	default:
		return false
	}
}

func researchStatusTerminal(status string) bool {
	switch status {
	case "completed", "partially_completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func (s Service) executeDeepResearch(
	ctx context.Context,
	run ResearchRun,
	documents []ExternalDocument,
	progress researchProgress,
	allowProviderRetry bool,
) (ResearchRun, error) {
	if run.RunMode != "deep" {
		return ResearchRun{}, ErrInvalidResearchRequest
	}
	if researchStatusTerminal(run.Status) {
		return run, nil
	}
	now := s.now()
	if run.StartedAt == nil {
		run.StartedAt = &now
	}
	run.DisclosedChunkIDs = make([]string, 0, len(documents))
	for _, document := range documents {
		run.DisclosedChunkIDs = append(run.DisclosedChunkIDs, document.ID)
	}
	if err := s.updateResearchPhase(ctx, run, "planning", now, run.CurrentRound, ""); err != nil {
		return ResearchRun{}, err
	}
	if progress != nil {
		if err := progress(run.CurrentRound, "planning", "正在确认研究目标、上下文快照与停止条件"); err != nil {
			return ResearchRun{}, err
		}
	}
	noUsefulRounds := 0
	for round := run.CurrentRound + 1; round <= run.MaxRounds; round++ {
		if err := ctx.Err(); err != nil {
			return s.finishInterruptedResearch(context.WithoutCancel(ctx), run, "cancelled", "RESEARCH_CANCELLED", "研究任务已取消")
		}
		current, err := s.getResearchRun(ctx, run.OrganizationID, run.ProjectID, run.ID)
		if err != nil {
			return ResearchRun{}, err
		}
		run = current
		if researchStatusTerminal(run.Status) {
			return run, nil
		}
		if reason := researchBudgetStopReason(run, s.now()); reason != "" {
			run.StopReason = reason
			break
		}
		if progress != nil {
			if err := progress(round, "searching", fmt.Sprintf("第 %d / %d 轮：正在搜索并读取候选来源", round, run.MaxRounds)); err != nil {
				return ResearchRun{}, err
			}
		}
		if err := s.updateResearchPhase(ctx, run, "searching", s.now(), round-1, ""); err != nil {
			return ResearchRun{}, err
		}
		input := ExternalResearchInput{
			OrganizationID: run.OrganizationID, ProjectID: run.ProjectID,
			Mode: run.Mode, Category: run.Category, Purpose: run.Purpose,
			Query: run.Query, Documents: documents, ResearchRunID: run.ID,
			RunMode: run.RunMode, Round: round, MaxRounds: run.MaxRounds,
			PriorFindings: summarizeResearchFindings(run.Findings), OpenGaps: append([]string(nil), run.OpenGaps...),
		}
		inputHash, err := contract.NewContentHash(input)
		if err != nil {
			return ResearchRun{}, err
		}
		iterationStarted := s.now()
		results, callErr := s.Runner.Run(ctx, input)
		if callErr != nil {
			if errors.Is(callErr, context.Canceled) || errors.Is(callErr, context.DeadlineExceeded) || ctx.Err() != nil {
				return s.finishInterruptedResearch(context.WithoutCancel(ctx), run, "cancelled", "RESEARCH_CANCELLED", "研究任务已取消")
			}
			failed, finishErr := s.finishProviderFailure(ctx, run, round, inputHash, iterationStarted, !allowProviderRetry)
			if finishErr != nil {
				return ResearchRun{}, finishErr
			}
			if allowProviderRetry {
				return failed, errResearchProviderRetryable
			}
			return failed, nil
		}
		verifiedEvidence := s.verifyRoundEvidence(ctx, results)
		beforeUseful := countUsefulFindings(run.Findings)
		checkpoint, err := s.persistResearchRound(ctx, run, round, input, inputHash, results, verifiedEvidence, iterationStarted)
		if err != nil {
			return ResearchRun{}, err
		}
		run = checkpoint
		if countUsefulFindings(run.Findings) <= beforeUseful {
			noUsefulRounds++
		} else {
			noUsefulRounds = 0
		}
		if progress != nil {
			if err := progress(round, "cross_checking", fmt.Sprintf(
				"第 %d / %d 轮完成：%d 条已确认或冲突结论，仍有 %d 个缺口",
				round, run.MaxRounds, countUsefulFindings(run.Findings), len(run.OpenGaps),
			)); err != nil {
				return ResearchRun{}, err
			}
		}
		stopRecommended := false
		for _, result := range results {
			stopRecommended = stopRecommended || result.RecommendedStop
		}
		switch {
		case len(run.OpenGaps) == 0 && countUsefulFindings(run.Findings) > 0:
			run.StopReason = "coverage_complete"
		case stopRecommended && countUsefulFindings(run.Findings) > 0:
			run.StopReason = "model_recommended_after_validation"
		case noUsefulRounds >= 2:
			run.StopReason = "diminishing_returns"
		case round == run.MaxRounds:
			run.StopReason = "max_rounds"
		default:
			continue
		}
		break
	}
	if run.StopReason == "" {
		run.StopReason = researchBudgetStopReason(run, s.now())
	}
	if run.StopReason == "" {
		run.StopReason = "max_rounds"
	}
	return s.finalizeDeepResearch(ctx, run, progress)
}

func (s Service) updateResearchPhase(ctx context.Context, run ResearchRun, status string, now time.Time, currentRound int, stopReason string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE platform_research_runs
		SET status = ?, current_round = ?, disclosed_chunk_ids = ?, stop_reason = ?,
			started_at = COALESCE(started_at, ?), heartbeat_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?
		  AND status IN ('queued', 'planning', 'searching', 'reading', 'cross_checking', 'drafting', 'auditing')`,
		status, currentRound, jsonBytes(run.DisclosedChunkIDs), stopReason, now, now, now,
		run.OrganizationID, run.ProjectID, run.ID)
	return err
}

func researchBudgetStopReason(run ResearchRun, now time.Time) string {
	if run.CurrentRound >= run.MaxRounds {
		return "max_rounds"
	}
	if run.Usage != nil && run.Usage.TotalTokens >= run.TokenBudget {
		return "token_budget"
	}
	if run.StartedAt != nil && now.Sub(*run.StartedAt) >= time.Duration(run.TimeBudgetSeconds)*time.Second {
		return "time_budget"
	}
	return ""
}

func summarizeResearchFindings(findings []ResearchFinding) []ExternalFindingSummary {
	values := make([]ExternalFindingSummary, 0, len(findings))
	for _, finding := range findings {
		if finding.Status == "invalid" {
			continue
		}
		values = append(values, ExternalFindingSummary{
			Claim: finding.Claim, Status: finding.Status,
			TargetPath: finding.Target.Artifact + "." + finding.Target.FieldPath,
		})
		if len(values) == 20 {
			break
		}
	}
	return values
}

type verifiedEvidenceKey struct {
	CanonicalURL string
	Excerpt      string
}

func (s Service) verifyRoundEvidence(ctx context.Context, results []ExternalResearchResult) map[verifiedEvidenceKey]VerifiedResearchSource {
	verified := map[verifiedEvidenceKey]VerifiedResearchSource{}
	if s.SourceVerifier == nil {
		return verified
	}
	for _, result := range results {
		for _, finding := range result.Findings {
			evidence := append(append([]ExternalResearchEvidence(nil), finding.SupportingEvidence...), finding.ConflictingEvidence...)
			for _, item := range evidence {
				canonical, _, err := canonicalResearchURL(item.URL)
				excerpt := strings.TrimSpace(item.Excerpt)
				key := verifiedEvidenceKey{CanonicalURL: canonical, Excerpt: excerpt}
				if err != nil || excerpt == "" {
					continue
				}
				if _, exists := verified[key]; exists {
					continue
				}
				value, err := s.SourceVerifier.Verify(ctx, canonical, excerpt)
				if err == nil && value.ExcerptFound && strings.TrimSpace(value.ContentHash) != "" {
					verified[key] = value
				}
			}
		}
	}
	return verified
}

type researchSourceVerification struct {
	Status      string
	ContentHash string
}

func researchSourceVerificationByURL(
	results []ExternalResearchResult,
	verified map[verifiedEvidenceKey]VerifiedResearchSource,
) map[string]researchSourceVerification {
	values := map[string]researchSourceVerification{}
	record := func(evidence ExternalResearchEvidence, status string) {
		canonical, _, err := canonicalResearchURL(evidence.URL)
		if err != nil {
			return
		}
		verification, ok := verified[verifiedEvidenceKey{
			CanonicalURL: canonical,
			Excerpt:      strings.TrimSpace(evidence.Excerpt),
		}]
		if !ok || !verification.ExcerptFound || strings.TrimSpace(verification.ContentHash) == "" {
			return
		}
		current := values[canonical]
		// A source used as verified counter-evidence is globally surfaced as
		// conflicted. This is deliberately stronger than content_verified so a
		// later supporting citation cannot hide the conflict from the UI.
		if current.Status == "conflicted" && status != "conflicted" {
			return
		}
		values[canonical] = researchSourceVerification{
			Status:      status,
			ContentHash: strings.TrimSpace(verification.ContentHash),
		}
	}
	for _, result := range results {
		for _, finding := range result.Findings {
			for _, evidence := range finding.SupportingEvidence {
				record(evidence, "content_verified")
			}
			for _, evidence := range finding.ConflictingEvidence {
				record(evidence, "conflicted")
			}
		}
	}
	return values
}

func (s Service) persistResearchRound(
	ctx context.Context,
	run ResearchRun,
	round int,
	input ExternalResearchInput,
	inputHash contract.ContentHash,
	results []ExternalResearchResult,
	verified map[verifiedEvidenceKey]VerifiedResearchSource,
	startedAt time.Time,
) (ResearchRun, error) {
	outputHash, err := contract.NewContentHash(results)
	if err != nil {
		return ResearchRun{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ResearchRun{}, err
	}
	defer tx.Rollback()
	artifactIDs := []string{}
	sourceByURL := map[string]ResearchSource{}
	usage := ResearchUsage{}
	coverage := cloneCoverage(run.Coverage)
	openGaps := []string{}
	actionSummary := []string{}
	for _, result := range results {
		artifact, insertErr := s.insertArtifact(ctx, tx, run, result)
		if insertErr != nil {
			return ResearchRun{}, insertErr
		}
		artifactIDs = append(artifactIDs, artifact.ID)
		for _, source := range artifact.Sources {
			sourceByURL[source.CanonicalURL] = source
		}
		if result.Usage != nil {
			usage.InputTokens += result.Usage.InputTokens
			usage.OutputTokens += result.Usage.OutputTokens
			usage.TotalTokens += result.Usage.TotalTokens
		}
		for key, value := range result.Coverage {
			if value {
				coverage[key] = true
			} else if _, exists := coverage[key]; !exists {
				coverage[key] = false
			}
		}
		openGaps = append(openGaps, result.OpenGaps...)
		if summary := strings.TrimSpace(result.ActionSummary); summary != "" {
			actionSummary = append(actionSummary, summary)
		}
		if run.ProviderCode == "" {
			run.ProviderCode = strings.TrimSpace(result.ProviderCode)
			run.ModelVersion = strings.TrimSpace(result.ModelVersion)
			run.ProviderResponseID = strings.TrimSpace(result.ProviderResponse)
		}
	}
	verificationByURL := researchSourceVerificationByURL(results, verified)
	for canonicalURL, verification := range verificationByURL {
		source, ok := sourceByURL[canonicalURL]
		if !ok {
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE platform_research_sources
			SET verification_status = ?, content_hash = ?
			WHERE organization_id = ? AND project_id = ? AND research_run_id = ? AND id = ?
			  AND verification_status IN ('model_cited', 'content_verified', 'conflicted')`,
			verification.Status, verification.ContentHash,
			run.OrganizationID, run.ProjectID, run.ID, source.ID); err != nil {
			return ResearchRun{}, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE platform_research_citations
			SET support_level = ?
			WHERE organization_id = ? AND project_id = ? AND research_source_id = ?`,
			verification.Status, run.OrganizationID, run.ProjectID, source.ID); err != nil {
			return ResearchRun{}, err
		}
		source.VerificationStatus = verification.Status
		source.SupportLevel = verification.Status
		source.ContentHash = verification.ContentHash
		sourceByURL[canonicalURL] = source
	}
	openGaps = normalizedLimitedStrings(openGaps, 32, 300)
	findingIDs := []string{}
	for _, result := range results {
		for _, candidate := range result.Findings {
			finding, upsertErr := s.upsertResearchFinding(ctx, tx, run, round, candidate, sourceByURL, verified)
			if upsertErr != nil {
				return ResearchRun{}, upsertErr
			}
			findingIDs = append(findingIDs, finding.ID)
		}
	}
	findingIDs = uniqueStrings(findingIDs)
	sourceIDs := make([]string, 0, len(sourceByURL))
	for _, source := range sourceByURL {
		sourceIDs = append(sourceIDs, source.ID)
	}
	sort.Strings(sourceIDs)
	iterationID, err := s.newID("researchiteration")
	if err != nil {
		return ResearchRun{}, err
	}
	now := s.now()
	if _, err := tx.ExecContext(ctx, `INSERT INTO platform_research_iterations
		(id, organization_id, project_id, research_run_id, round_number, status,
		 objective, query_text, action_summary, source_ids, artifact_ids, finding_ids,
		 coverage_json, open_gaps_json, usage_json, input_hash, output_hash,
		 started_at, completed_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'completed', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			status = 'completed', objective = VALUES(objective), query_text = VALUES(query_text),
			action_summary = VALUES(action_summary), source_ids = VALUES(source_ids),
			artifact_ids = VALUES(artifact_ids), finding_ids = VALUES(finding_ids),
			coverage_json = VALUES(coverage_json), open_gaps_json = VALUES(open_gaps_json),
			usage_json = VALUES(usage_json), input_hash = VALUES(input_hash), output_hash = VALUES(output_hash),
			error_code = NULL, error_message = NULL, started_at = VALUES(started_at),
			completed_at = VALUES(completed_at), updated_at = VALUES(updated_at)`,
		iterationID, run.OrganizationID, run.ProjectID, run.ID, round,
		researchRoundObjective(run, round), input.Query, strings.Join(actionSummary, "；"),
		jsonBytes(sourceIDs), jsonBytes(artifactIDs), jsonBytes(findingIDs),
		jsonBytes(coverage), jsonBytes(openGaps), nullableJSONValue(&usage), inputHash, outputHash,
		startedAt, now, now, now); err != nil {
		return ResearchRun{}, err
	}
	accumulated := ResearchUsage{}
	if run.Usage != nil {
		accumulated = *run.Usage
	}
	accumulated.InputTokens += usage.InputTokens
	accumulated.OutputTokens += usage.OutputTokens
	accumulated.TotalTokens += usage.TotalTokens
	checkpointResult, err := tx.ExecContext(ctx, `UPDATE platform_research_runs
		SET status = 'cross_checking', current_round = ?, coverage_json = ?, open_gaps_json = ?,
			provider_code = ?, model_version = ?, provider_response_id = ?, usage_json = ?,
			heartbeat_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?
		  AND current_round = ? AND status IN ('planning', 'searching', 'reading', 'cross_checking')`,
		round, jsonBytes(coverage), jsonBytes(openGaps), nullable(run.ProviderCode), nullable(run.ModelVersion),
		nullable(run.ProviderResponseID), nullableJSONValue(&accumulated), now, now,
		run.OrganizationID, run.ProjectID, run.ID, round-1)
	if err != nil {
		return ResearchRun{}, err
	}
	changed, err := checkpointResult.RowsAffected()
	if err != nil {
		return ResearchRun{}, err
	}
	if changed != 1 {
		return ResearchRun{}, fmt.Errorf("research run checkpoint lost optimistic lock")
	}
	if err := tx.Commit(); err != nil {
		return ResearchRun{}, err
	}
	return s.getResearchRun(ctx, run.OrganizationID, run.ProjectID, run.ID)
}

func researchRoundObjective(run ResearchRun, round int) string {
	if len(run.OpenGaps) > 0 {
		return truncateRunesForResearch("补查高影响缺口："+strings.Join(run.OpenGaps, "；"), 2000)
	}
	return truncateRunesForResearch(fmt.Sprintf("围绕决策问题建立第 %d 轮可核验证据：%s", round, run.Query), 2000)
}

func (s Service) upsertResearchFinding(
	ctx context.Context,
	tx *sql.Tx,
	run ResearchRun,
	round int,
	candidate ExternalResearchFinding,
	sourceByURL map[string]ResearchSource,
	verified map[verifiedEvidenceKey]VerifiedResearchSource,
) (ResearchFinding, error) {
	finding := normalizeResearchFinding(candidate, round, sourceByURL, verified)
	hash, err := contract.NewContentHash(struct {
		Claim         string                `json:"claim"`
		TimeScope     string                `json:"time_scope"`
		Target        ResearchFindingTarget `json:"target"`
		Implication   string                `json:"implication"`
		ProposedValue json.RawMessage       `json:"proposed_value,omitempty"`
	}{finding.Claim, finding.TimeScope, finding.Target, finding.Implication, finding.ProposedValue})
	if err != nil {
		return ResearchFinding{}, err
	}
	finding.ContentHash = hash
	var existingID string
	var existingRound int
	var existingStatus string
	var existingSupport, existingConflict []byte
	err = tx.QueryRowContext(ctx, `SELECT id, round_number, status, supporting_source_ids, conflicting_source_ids
		FROM platform_research_findings
		WHERE organization_id = ? AND project_id = ? AND research_run_id = ? AND content_hash = ? FOR UPDATE`,
		run.OrganizationID, run.ProjectID, run.ID, finding.ContentHash).Scan(
		&existingID, &existingRound, &existingStatus, &existingSupport, &existingConflict,
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ResearchFinding{}, err
	}
	now := s.now()
	if errors.Is(err, sql.ErrNoRows) {
		finding.ID, err = s.newID("researchfinding")
		if err != nil {
			return ResearchFinding{}, err
		}
		finding.ContractVersion = researchFindingContractVersion
		finding.ResearchRunID = run.ID
		_, err = tx.ExecContext(ctx, `INSERT INTO platform_research_findings
			(id, contract_version, organization_id, project_id, research_run_id, round_number,
			 claim, status, time_scope, confidence, supporting_source_ids, conflicting_source_ids,
			 target_artifact, target_field_path, implication, proposed_value, content_hash, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			finding.ID, finding.ContractVersion, run.OrganizationID, run.ProjectID, run.ID, round,
			finding.Claim, finding.Status, finding.TimeScope, finding.Confidence,
			jsonBytes(finding.SupportingSourceIDs), jsonBytes(finding.ConflictingSourceIDs),
			finding.Target.Artifact, finding.Target.FieldPath, finding.Implication,
			nullableJSON(finding.ProposedValue), finding.ContentHash, now, now)
		return finding, err
	}
	var priorSupport, priorConflict []string
	_ = json.Unmarshal(existingSupport, &priorSupport)
	_ = json.Unmarshal(existingConflict, &priorConflict)
	finding.ID = existingID
	finding.ContractVersion = researchFindingContractVersion
	finding.ResearchRunID = run.ID
	finding.Round = maxInt(existingRound, round)
	finding.SupportingSourceIDs = uniqueStrings(append(priorSupport, finding.SupportingSourceIDs...))
	finding.ConflictingSourceIDs = uniqueStrings(append(priorConflict, finding.ConflictingSourceIDs...))
	finding.Status = strongerFindingStatus(existingStatus, finding.Status, finding.SupportingSourceIDs, finding.ConflictingSourceIDs)
	_, err = tx.ExecContext(ctx, `UPDATE platform_research_findings
		SET round_number = ?, status = ?, confidence = ?, supporting_source_ids = ?,
			conflicting_source_ids = ?, proposed_value = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND research_run_id = ? AND id = ?`,
		finding.Round, finding.Status, finding.Confidence, jsonBytes(finding.SupportingSourceIDs),
		jsonBytes(finding.ConflictingSourceIDs), nullableJSON(finding.ProposedValue), now,
		run.OrganizationID, run.ProjectID, run.ID, finding.ID)
	return finding, err
}

func normalizeResearchFinding(
	candidate ExternalResearchFinding,
	round int,
	sourceByURL map[string]ResearchSource,
	verified map[verifiedEvidenceKey]VerifiedResearchSource,
) ResearchFinding {
	finding := ResearchFinding{
		Claim:      truncateRunesForResearch(strings.TrimSpace(candidate.Claim), 2000),
		TimeScope:  truncateRunesForResearch(strings.TrimSpace(candidate.TimeScope), 96),
		Confidence: normalizedFindingConfidence(candidate.Confidence),
		Target: ResearchFindingTarget{
			Artifact:  strings.ToLower(strings.TrimSpace(candidate.TargetArtifact)),
			FieldPath: strings.TrimSpace(candidate.TargetFieldPath),
		},
		Implication: truncateRunesForResearch(strings.TrimSpace(candidate.Implication), 2000),
		Round:       round,
	}
	if finding.TimeScope == "" {
		finding.TimeScope = "未注明"
	}
	if len(candidate.ProposedValue) > 0 && len(candidate.ProposedValue) <= 16*1024 && json.Valid(candidate.ProposedValue) {
		finding.ProposedValue = append(json.RawMessage(nil), candidate.ProposedValue...)
	}
	verifiedSupportingDomains := map[string]struct{}{}
	for _, evidence := range candidate.SupportingEvidence {
		canonical, domain, err := canonicalResearchURL(evidence.URL)
		if err != nil {
			continue
		}
		source, ok := sourceByURL[canonical]
		if !ok {
			continue
		}
		finding.SupportingSourceIDs = append(finding.SupportingSourceIDs, source.ID)
		if value, ok := verified[verifiedEvidenceKey{CanonicalURL: canonical, Excerpt: strings.TrimSpace(evidence.Excerpt)}]; ok && value.ExcerptFound {
			verifiedSupportingDomains[domain] = struct{}{}
		}
	}
	verifiedConflict := false
	for _, evidence := range candidate.ConflictingEvidence {
		canonical, _, err := canonicalResearchURL(evidence.URL)
		if err != nil {
			continue
		}
		source, ok := sourceByURL[canonical]
		if !ok {
			continue
		}
		finding.ConflictingSourceIDs = append(finding.ConflictingSourceIDs, source.ID)
		if value, ok := verified[verifiedEvidenceKey{CanonicalURL: canonical, Excerpt: strings.TrimSpace(evidence.Excerpt)}]; ok && value.ExcerptFound {
			verifiedConflict = true
		}
	}
	finding.SupportingSourceIDs = uniqueStrings(finding.SupportingSourceIDs)
	finding.ConflictingSourceIDs = uniqueStrings(finding.ConflictingSourceIDs)
	switch {
	case finding.Claim == "" || finding.Implication == "" || !validResearchFindingTarget(finding.Target):
		finding.Status = "invalid"
	case len(finding.SupportingSourceIDs) == 0:
		finding.Status = "invalid"
	case verifiedConflict && len(finding.ConflictingSourceIDs) > 0:
		finding.Status = "conflicting"
	case len(verifiedSupportingDomains) >= 2:
		finding.Status = "verified"
	default:
		finding.Status = "tentative"
	}
	return finding
}

func validResearchFindingTarget(target ResearchFindingTarget) bool {
	switch target.Artifact {
	case "brief":
		switch target.FieldPath {
		case "campaign.objective", "audience.primary", "proposition", "channels", "constraints",
			"measurement.primary_kpi", "creative.tone", "creative.mandatory_elements", "creative.prohibited_claims":
			return true
		}
	case "strategy":
		switch target.FieldPath {
		case "executive_summary", "objective", "audience", "proposition", "channel_strategy",
			"platform_plans", "content_strategy", "budget_and_cadence", "measurement", "experiments",
			"assumptions_and_gaps":
			return true
		}
	}
	return false
}

func strongerFindingStatus(left, right string, support, conflict []string) string {
	if len(conflict) > 0 && len(support) > 0 && (left == "conflicting" || right == "conflicting") {
		return "conflicting"
	}
	order := map[string]int{"invalid": 0, "tentative": 1, "verified": 2, "conflicting": 3}
	if order[right] > order[left] {
		return right
	}
	return left
}

func normalizedFindingConfidence(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low", "high":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "medium"
	}
}

func (s Service) finalizeDeepResearch(ctx context.Context, run ResearchRun, progress researchProgress) (ResearchRun, error) {
	current, err := s.getResearchRun(ctx, run.OrganizationID, run.ProjectID, run.ID)
	if err != nil {
		return ResearchRun{}, err
	}
	run = current
	if progress != nil {
		if err := progress(run.CurrentRound, "drafting", "正在生成带来源、冲突与采纳映射的研究报告"); err != nil {
			return ResearchRun{}, err
		}
	}
	if err := s.updateResearchPhase(ctx, run, "drafting", s.now(), run.CurrentRound, run.StopReason); err != nil {
		return ResearchRun{}, err
	}
	report, reportSources := buildResearchReport(run)
	result := ExternalResearchResult{
		Title: "深度研究报告", Content: report, Citations: sourceURLs(reportSources), Sources: reportSources,
		ProviderCode: run.ProviderCode, ModelVersion: run.ModelVersion,
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ResearchRun{}, err
	}
	defer tx.Rollback()
	artifact, err := s.insertArtifact(ctx, tx, run, result)
	if err != nil {
		return ResearchRun{}, err
	}
	now := s.now()
	status := "partially_completed"
	if hasAdoptableFinding(run.Findings) && researchReportCitationAudit(run.Findings, artifact.Sources) {
		status = "completed"
	} else if run.StopReason == "" {
		if hasAdoptableFinding(run.Findings) {
			run.StopReason = "report_citation_audit_failed"
		} else {
			run.StopReason = "no_adoptable_verified_finding"
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE platform_research_runs
		SET status = ?, report_artifact_id = ?, stop_reason = ?, heartbeat_at = ?,
			completed_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?
		  AND status IN ('planning', 'searching', 'reading', 'cross_checking', 'drafting', 'auditing')`,
		status, artifact.ID, run.StopReason, now, now, now, run.OrganizationID, run.ProjectID, run.ID); err != nil {
		return ResearchRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return ResearchRun{}, err
	}
	completed, err := s.getResearchRun(ctx, run.OrganizationID, run.ProjectID, run.ID)
	if err != nil {
		return ResearchRun{}, err
	}
	if completed.Status == "completed" && s.ResearchCompletion != nil {
		if sinkErr := s.ResearchCompletion.OnResearchCompleted(ctx, completed); sinkErr != nil {
			now = s.now()
			_, _ = s.DB.ExecContext(ctx, `UPDATE platform_research_runs
				SET status = 'partially_completed', stop_reason = 'proposal_generation_failed',
					error_code = 'RESEARCH_PROPOSAL_GENERATION_FAILED',
					error_message = '研究报告已保存，但采纳建议生成失败', updated_at = ?
				WHERE organization_id = ? AND project_id = ? AND id = ? AND status = 'completed'`,
				now, run.OrganizationID, run.ProjectID, run.ID)
			return s.getResearchRun(ctx, run.OrganizationID, run.ProjectID, run.ID)
		}
	}
	return completed, nil
}

func hasAdoptableFinding(findings []ResearchFinding) bool {
	for _, finding := range findings {
		if (finding.Status == "verified" || finding.Status == "conflicting") &&
			validResearchFindingTarget(finding.Target) && len(finding.ProposedValue) > 0 {
			return true
		}
	}
	return false
}

func researchReportCitationAudit(findings []ResearchFinding, sources []ResearchSource) bool {
	verifiedSources := make(map[string]struct{}, len(sources))
	conflictingSources := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		switch source.VerificationStatus {
		case "content_verified":
			verifiedSources[source.ID] = struct{}{}
		case "conflicted":
			verifiedSources[source.ID] = struct{}{}
			conflictingSources[source.ID] = struct{}{}
		}
	}
	adoptable := 0
	for _, finding := range findings {
		if (finding.Status != "verified" && finding.Status != "conflicting") ||
			!validResearchFindingTarget(finding.Target) || len(finding.ProposedValue) == 0 {
			continue
		}
		adoptable++
		if !containsResearchSource(finding.SupportingSourceIDs, verifiedSources) {
			return false
		}
		if finding.Status == "conflicting" && !containsResearchSource(finding.ConflictingSourceIDs, conflictingSources) {
			return false
		}
	}
	return adoptable > 0
}

func containsResearchSource(ids []string, available map[string]struct{}) bool {
	for _, id := range ids {
		if _, ok := available[id]; ok {
			return true
		}
	}
	return false
}

func buildResearchReport(run ResearchRun) (string, []ExternalResearchSource) {
	var report strings.Builder
	fmt.Fprintf(&report, "# 深度研究报告\n\n研究问题：%s\n\n", run.Query)
	report.WriteString("## 可用于决策的发现\n\n")
	useful := 0
	for _, finding := range run.Findings {
		if finding.Status != "verified" && finding.Status != "conflicting" {
			continue
		}
		useful++
		fmt.Fprintf(&report, "%d. %s\n   - 状态：%s；置信度：%s；时间范围：%s\n   - 目标：%s.%s\n   - 对决策的影响：%s\n\n",
			useful, finding.Claim, finding.Status, finding.Confidence, finding.TimeScope,
			finding.Target.Artifact, finding.Target.FieldPath, finding.Implication)
	}
	if useful == 0 {
		report.WriteString("目前没有达到交叉核验门槛且可直接映射到 Brief/策略字段的结论。研究结果不会自动改写任何业务对象。\n\n")
	}
	if len(run.OpenGaps) > 0 {
		report.WriteString("## 尚未解决的缺口\n\n")
		for _, gap := range run.OpenGaps {
			fmt.Fprintf(&report, "- %s\n", gap)
		}
		report.WriteString("\n")
	}
	fmt.Fprintf(&report, "## 停止说明\n\n%s；共完成 %d / %d 轮。\n", run.StopReason, run.CurrentRound, run.MaxRounds)
	sourceMap := map[string]ExternalResearchSource{}
	for _, artifact := range run.Artifacts {
		for _, source := range artifact.Sources {
			sourceMap[source.CanonicalURL] = ExternalResearchSource{
				SourceClass: source.SourceClass, MediaType: source.MediaType, Title: source.Title,
				URL: source.URL, PublishedAt: source.PublishedAt,
			}
		}
	}
	sources := make([]ExternalResearchSource, 0, len(sourceMap))
	for _, source := range sourceMap {
		sources = append(sources, source)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].URL < sources[j].URL })
	return report.String(), sources
}

func sourceURLs(sources []ExternalResearchSource) []string {
	values := make([]string, 0, len(sources))
	for _, source := range sources {
		values = append(values, source.URL)
	}
	return values
}

func (s Service) finishProviderFailure(
	ctx context.Context,
	run ResearchRun,
	round int,
	inputHash contract.ContentHash,
	startedAt time.Time,
	terminal bool,
) (ResearchRun, error) {
	now := s.now()
	status, stopReason := "planning", "provider_retry"
	var completedAt any
	if terminal {
		status, stopReason, completedAt = "failed", "provider_error", now
		if countUsefulFindings(run.Findings) > 0 {
			status = "partially_completed"
		}
	}
	iterationID, err := s.newID("researchiteration")
	if err != nil {
		return ResearchRun{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ResearchRun{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO platform_research_iterations
		(id, organization_id, project_id, research_run_id, round_number, status,
		 objective, query_text, action_summary, source_ids, artifact_ids, finding_ids,
		 coverage_json, open_gaps_json, input_hash, output_hash, error_code, error_message,
		 started_at, completed_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'failed', ?, ?, '', JSON_ARRAY(), JSON_ARRAY(), JSON_ARRAY(),
			?, ?, ?, ?, 'EXTERNAL_RESEARCH_FAILED', '外部研究调用失败', ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			status = 'failed', objective = VALUES(objective), query_text = VALUES(query_text),
			action_summary = '', source_ids = JSON_ARRAY(), artifact_ids = JSON_ARRAY(),
			finding_ids = JSON_ARRAY(), coverage_json = VALUES(coverage_json),
			open_gaps_json = VALUES(open_gaps_json), usage_json = NULL,
			input_hash = VALUES(input_hash), output_hash = VALUES(output_hash),
			error_code = 'EXTERNAL_RESEARCH_FAILED', error_message = '外部研究调用失败',
			started_at = VALUES(started_at), completed_at = VALUES(completed_at), updated_at = VALUES(updated_at)`,
		iterationID, run.OrganizationID, run.ProjectID, run.ID, round,
		researchRoundObjective(run, round), run.Query, jsonBytes(run.Coverage), jsonBytes(run.OpenGaps),
		inputHash, inputHash, startedAt, now, now, now)
	if err != nil {
		return ResearchRun{}, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE platform_research_runs
		SET status = ?, stop_reason = ?, error_code = 'EXTERNAL_RESEARCH_FAILED',
			error_message = '外部研究调用失败', heartbeat_at = ?, completed_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		status, stopReason, now, completedAt, now, run.OrganizationID, run.ProjectID, run.ID)
	if err != nil {
		return ResearchRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return ResearchRun{}, err
	}
	return s.getResearchRun(ctx, run.OrganizationID, run.ProjectID, run.ID)
}

func (s Service) finishInterruptedResearch(ctx context.Context, run ResearchRun, status, code, message string) (ResearchRun, error) {
	now := s.now()
	_, err := s.DB.ExecContext(ctx, `UPDATE platform_research_runs
		SET status = ?, stop_reason = 'user_cancelled', error_code = ?, error_message = ?,
			heartbeat_at = ?, completed_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?
		  AND status IN ('queued', 'planning', 'searching', 'reading', 'cross_checking', 'drafting', 'auditing')`,
		status, code, message, now, now, now, run.OrganizationID, run.ProjectID, run.ID)
	if err != nil {
		return ResearchRun{}, err
	}
	return s.getResearchRun(ctx, run.OrganizationID, run.ProjectID, run.ID)
}

func countUsefulFindings(findings []ResearchFinding) int {
	count := 0
	for _, finding := range findings {
		if finding.Status == "verified" || finding.Status == "conflicting" {
			count++
		}
	}
	return count
}

func cloneCoverage(source map[string]bool) map[string]bool {
	result := map[string]bool{}
	for key, value := range source {
		result[key] = value
	}
	return result
}

func normalizedLimitedStrings(values []string, maxItems, maxRunes int) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, minInt(len(values), maxItems))
	for _, value := range values {
		value = truncateRunesForResearch(strings.TrimSpace(value), maxRunes)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
		if len(result) == maxItems {
			break
		}
	}
	return result
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func truncateRunesForResearch(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func (s Service) listResearchIterations(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, runID string) ([]ResearchIteration, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, research_run_id, round_number, status,
		objective, query_text, action_summary, source_ids, artifact_ids, finding_ids,
		coverage_json, open_gaps_json, usage_json, input_hash, output_hash,
		COALESCE(error_code, ''), COALESCE(error_message, ''), started_at, completed_at
		FROM platform_research_iterations
		WHERE organization_id = ? AND project_id = ? AND research_run_id = ? ORDER BY round_number`,
		organizationID, projectID, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []ResearchIteration{}
	for rows.Next() {
		var value ResearchIteration
		var sourceIDs, artifactIDs, findingIDs, coverage, gaps, usage []byte
		var completed sql.NullTime
		if err := rows.Scan(&value.ID, &value.ResearchRunID, &value.Round, &value.Status,
			&value.Objective, &value.Query, &value.ActionSummary, &sourceIDs, &artifactIDs, &findingIDs,
			&coverage, &gaps, &usage, &value.InputHash, &value.OutputHash,
			&value.ErrorCode, &value.ErrorMessage, &value.StartedAt, &completed); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(sourceIDs, &value.SourceIDs)
		_ = json.Unmarshal(artifactIDs, &value.ArtifactIDs)
		_ = json.Unmarshal(findingIDs, &value.FindingIDs)
		_ = json.Unmarshal(coverage, &value.Coverage)
		_ = json.Unmarshal(gaps, &value.OpenGaps)
		if len(usage) > 0 {
			value.Usage = &ResearchUsage{}
			_ = json.Unmarshal(usage, value.Usage)
		}
		value.CompletedAt = nullTimePointer(completed)
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s Service) listResearchFindings(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, runID string) ([]ResearchFinding, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT contract_version, id, research_run_id, claim, status,
		time_scope, confidence, supporting_source_ids, conflicting_source_ids,
		target_artifact, target_field_path, implication, proposed_value, round_number, content_hash
		FROM platform_research_findings
		WHERE organization_id = ? AND project_id = ? AND research_run_id = ?
		ORDER BY round_number, created_at`, organizationID, projectID, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []ResearchFinding{}
	for rows.Next() {
		var value ResearchFinding
		var support, conflict []byte
		var proposed []byte
		if err := rows.Scan(&value.ContractVersion, &value.ID, &value.ResearchRunID,
			&value.Claim, &value.Status, &value.TimeScope, &value.Confidence,
			&support, &conflict, &value.Target.Artifact, &value.Target.FieldPath,
			&value.Implication, &proposed, &value.Round, &value.ContentHash); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(support, &value.SupportingSourceIDs)
		_ = json.Unmarshal(conflict, &value.ConflictingSourceIDs)
		if len(proposed) > 0 && string(proposed) != "null" {
			value.ProposedValue = append(json.RawMessage(nil), proposed...)
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
