package strategy

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/strategy/creativecatalog"
)

type CreativeTaskPlanCompleteness struct {
	Ready    bool              `json:"ready"`
	Blockers []ValidationError `json:"blockers"`
	Warnings []ValidationError `json:"warnings"`
}

type CreativeTaskPlan struct {
	ID                     string                       `json:"id"`
	OrganizationID         contract.OrganizationID      `json:"organization_id"`
	ProjectID              contract.ProjectID           `json:"project_id"`
	BriefID                string                       `json:"brief_id"`
	BriefVersion           int64                        `json:"brief_version"`
	BriefContentHash       string                       `json:"brief_content_hash"`
	SourceStrategyID       string                       `json:"source_strategy_id,omitempty"`
	SourceStrategyRevision int64                        `json:"source_strategy_revision,omitempty"`
	SourceStrategyHash     string                       `json:"source_strategy_content_hash,omitempty"`
	Status                 string                       `json:"status"`
	BusinessCode           string                       `json:"business_code"`
	BusinessGeneration     int64                        `json:"business_generation"`
	BusinessVersion        string                       `json:"business_version"`
	BusinessContentHash    string                       `json:"business_content_hash"`
	SkillName              string                       `json:"skill_name"`
	SkillVersion           string                       `json:"skill_version"`
	SkillContentHash       string                       `json:"skill_content_hash"`
	SelectionSource        string                       `json:"selection_source"`
	RecommendationSnapshot RecommendationSnapshot       `json:"recommendation_snapshot"`
	Answers                map[string]json.RawMessage   `json:"answers"`
	Completeness           CreativeTaskPlanCompleteness `json:"completeness"`
	CurrentRevision        int64                        `json:"current_revision"`
	CurrentStrategyVersion int64                        `json:"current_strategy_version"`
	CurrentAgentTaskID     string                       `json:"current_agent_task_id,omitempty"`
	Version                int64                        `json:"version"`
	CreatedBy              string                       `json:"created_by"`
	CreatedAt              time.Time                    `json:"created_at"`
	UpdatedAt              time.Time                    `json:"updated_at"`
	Profile                *creativecatalog.Profile     `json:"profile,omitempty"`
	CurrentStrategy        *CreativeTaskStrategyVersion `json:"current_strategy,omitempty"`
}

type CreativeTaskPlanRevision struct {
	PlanID       string                   `json:"plan_id"`
	Revision     int64                    `json:"revision"`
	BaseRevision *int64                   `json:"base_revision,omitempty"`
	Snapshot     CreativeTaskPlanSnapshot `json:"snapshot"`
	ContentHash  string                   `json:"content_hash"`
	ChangeReason string                   `json:"change_reason"`
	CreatedBy    string                   `json:"created_by"`
	CreatedAt    time.Time                `json:"created_at"`
}

type CreativeTaskPlanSnapshot struct {
	BriefID                string                       `json:"brief_id"`
	BriefVersion           int64                        `json:"brief_version"`
	BriefContentHash       string                       `json:"brief_content_hash"`
	SourceStrategyID       string                       `json:"source_strategy_id,omitempty"`
	SourceStrategyRevision int64                        `json:"source_strategy_revision,omitempty"`
	SourceStrategyHash     string                       `json:"source_strategy_content_hash,omitempty"`
	BusinessCode           string                       `json:"business_code"`
	BusinessGeneration     int64                        `json:"business_generation"`
	BusinessVersion        string                       `json:"business_version"`
	BusinessContentHash    string                       `json:"business_content_hash"`
	SkillName              string                       `json:"skill_name"`
	SkillVersion           string                       `json:"skill_version"`
	SkillContentHash       string                       `json:"skill_content_hash"`
	SelectionSource        string                       `json:"selection_source"`
	RecommendationSnapshot RecommendationSnapshot       `json:"recommendation_snapshot"`
	Answers                map[string]json.RawMessage   `json:"answers"`
	Completeness           CreativeTaskPlanCompleteness `json:"completeness"`
}

type SourceStrategyRequest struct {
	StrategyID string `json:"strategy_id"`
	Revision   int64  `json:"revision"`
}

type CreateCreativeTaskPlanRequest struct {
	BriefID         string                 `json:"brief_id"`
	BriefVersion    int64                  `json:"brief_version"`
	SourceStrategy  *SourceStrategyRequest `json:"source_strategy,omitempty"`
	BusinessCode    string                 `json:"business_code"`
	SelectionSource string                 `json:"selection_source"`
	CatalogHash     string                 `json:"catalog_hash"`
}

type CreativeTaskAnswerOperation struct {
	Op         string          `json:"op"`
	QuestionID string          `json:"question_id"`
	Value      json.RawMessage `json:"value,omitempty"`
}

type CreativeTaskAnswerPatch struct {
	ExpectedVersion int64                         `json:"expected_version"`
	Operations      []CreativeTaskAnswerOperation `json:"operations"`
}

const creativeTaskPlanSelect = `SELECT id, organization_id, project_id, brief_id,
	brief_version, brief_content_hash, COALESCE(source_strategy_id, ''),
	COALESCE(source_strategy_revision, 0), COALESCE(source_strategy_content_hash, ''),
	status, business_code, business_generation, business_version, business_content_hash,
	skill_name, skill_version, skill_content_hash, selection_source,
	recommendation_snapshot, answers, completeness, current_revision,
	current_strategy_version, COALESCE(current_agent_task_id, ''), version, created_by,
	created_at, updated_at FROM strategy_creative_task_plans`

func (s Service) CreateCreativeTaskPlan(
	ctx context.Context,
	actor contract.ActorContext,
	key contract.IdempotencyKey,
	projectID contract.ProjectID,
	request CreateCreativeTaskPlanRequest,
) (CreativeTaskPlan, bool, error) {
	if !s.CreativeTaskPlanningEnabled {
		return CreativeTaskPlan{}, false, ErrFeatureDisabled
	}
	if err := requireScope(actor, ScopeWrite); err != nil {
		return CreativeTaskPlan{}, false, err
	}
	request.BriefID = strings.TrimSpace(request.BriefID)
	request.BusinessCode = strings.TrimSpace(request.BusinessCode)
	request.SelectionSource = strings.TrimSpace(request.SelectionSource)
	request.CatalogHash = strings.TrimSpace(request.CatalogHash)
	if err := key.Validate(); err != nil || request.BriefID == "" || request.BriefVersion < 1 ||
		request.BusinessCode == "" || request.CatalogHash == "" ||
		(request.SelectionSource != "recommended" && request.SelectionSource != "manual") {
		return CreativeTaskPlan{}, false, ErrInvalidRequest
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return CreativeTaskPlan{}, false, err
	}
	brief, err := s.GetBriefVersion(ctx, actor, request.BriefID, request.BriefVersion)
	if err != nil {
		return CreativeTaskPlan{}, false, err
	}
	if brief.ProjectID != projectID {
		return CreativeTaskPlan{}, false, ErrProjectAccessDenied
	}
	recommendation, err := s.RecommendCreativeBusinesses(
		ctx, actor, projectID, request.BriefID, request.BriefVersion, 3,
	)
	if err != nil {
		return CreativeTaskPlan{}, false, err
	}
	if recommendation.CatalogHash != request.CatalogHash {
		return CreativeTaskPlan{}, false, ErrCatalogChanged
	}
	selected, recommended := findRecommendation(recommendation, request.BusinessCode)
	if selected == nil {
		return CreativeTaskPlan{}, false, ErrBusinessNotSelectable
	}
	actualSource := "manual"
	if recommended {
		actualSource = "recommended"
	}
	if actualSource != request.SelectionSource {
		return CreativeTaskPlan{}, false, ErrInvalidRequest
	}
	profile, err := s.getCreativeBusinessVersion(
		ctx, selected.ProfileRef.BusinessCode, selected.ProfileRef.Generation,
	)
	if err != nil {
		return CreativeTaskPlan{}, false, err
	}
	if !profile.Selectable || profile.Lifecycle != "active" {
		return CreativeTaskPlan{}, false, ErrBusinessNotSelectable
	}
	var sourceStrategyID, sourceStrategyHash string
	var sourceStrategyRevision int64
	if request.SourceStrategy != nil {
		if request.SourceStrategy.StrategyID == "" || request.SourceStrategy.Revision < 1 {
			return CreativeTaskPlan{}, false, ErrInvalidRequest
		}
		draft, err := s.GetDraft(ctx, actor, request.SourceStrategy.StrategyID)
		if err != nil {
			return CreativeTaskPlan{}, false, err
		}
		if draft.ProjectID != projectID || draft.BriefID != brief.BriefID {
			return CreativeTaskPlan{}, false, ErrInvalidRequest
		}
		revision, err := s.GetDraftRevision(
			ctx, actor, request.SourceStrategy.StrategyID, request.SourceStrategy.Revision,
		)
		if err != nil {
			return CreativeTaskPlan{}, false, err
		}
		sourceStrategyID = request.SourceStrategy.StrategyID
		sourceStrategyRevision = request.SourceStrategy.Revision
		sourceStrategyHash = string(revision.ContentHash)
	}
	requestHash, _ := contract.CanonicalJSONHash(struct {
		ProjectID contract.ProjectID            `json:"project_id"`
		Request   CreateCreativeTaskPlanRequest `json:"request"`
	}{projectID, request})
	var prior CreativeTaskPlan
	found, err := s.loadReceipt(
		ctx, actor, projectID, "strategy.creative_task_plan.create", key, requestHash, &prior,
	)
	if found || err != nil {
		return prior, found, err
	}
	id, err := s.newID("strategycreativeplan")
	if err != nil {
		return CreativeTaskPlan{}, false, err
	}
	now := s.now()
	answers := map[string]json.RawMessage{}
	completeness := evaluateCreativeTaskPlanCompleteness(brief.Snapshot, profile, answers)
	status := "collecting"
	if completeness.Ready {
		status = "ready"
	}
	plan := CreativeTaskPlan{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		BriefID: brief.BriefID, BriefVersion: brief.Version,
		BriefContentHash: string(brief.ContentHash),
		SourceStrategyID: sourceStrategyID, SourceStrategyRevision: sourceStrategyRevision,
		SourceStrategyHash: sourceStrategyHash, Status: status,
		BusinessCode: profile.BusinessCode, BusinessGeneration: profile.Generation,
		BusinessVersion: profile.Version, BusinessContentHash: profile.ContentHash,
		SkillName: profile.SkillName, SkillVersion: profile.SkillVersion,
		SkillContentHash: profile.SkillContentHash, SelectionSource: actualSource,
		RecommendationSnapshot: recommendation, Answers: answers, Completeness: completeness,
		CurrentRevision: 1, Version: 1, CreatedBy: actor.Principal.ID,
		CreatedAt: now, UpdatedAt: now,
	}
	plan.Profile = &profile
	snapshot := plan.snapshot()
	contentHash, err := contract.NewContentHash(snapshot)
	if err != nil {
		return CreativeTaskPlan{}, false, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return CreativeTaskPlan{}, false, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_creative_task_plans
		(id, organization_id, project_id, brief_id, brief_version, brief_content_hash,
		 source_strategy_id, source_strategy_revision, source_strategy_content_hash,
		 status, business_code, business_generation, business_version, business_content_hash,
		 skill_name, skill_version, skill_content_hash, selection_source,
		 recommendation_snapshot, answers, completeness, current_revision,
		 current_strategy_version, current_agent_task_id, version, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, 0), NULLIF(?, ''), ?, ?, ?, ?, ?,
		 ?, ?, ?, ?, ?, ?, ?, 1, 0, NULL, 1, ?, ?, ?)`,
		plan.ID, plan.OrganizationID, plan.ProjectID, plan.BriefID, plan.BriefVersion,
		plan.BriefContentHash, plan.SourceStrategyID, plan.SourceStrategyRevision,
		plan.SourceStrategyHash, plan.Status, plan.BusinessCode, plan.BusinessGeneration,
		plan.BusinessVersion, plan.BusinessContentHash, plan.SkillName, plan.SkillVersion,
		plan.SkillContentHash, plan.SelectionSource, mustJSON(plan.RecommendationSnapshot),
		mustJSON(plan.Answers), mustJSON(plan.Completeness), plan.CreatedBy, now, now); err != nil {
		return CreativeTaskPlan{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_creative_task_plan_revisions
		(plan_id, revision, organization_id, project_id, base_revision, snapshot, content_hash,
		 change_reason, created_by, created_at)
		VALUES (?, 1, ?, ?, NULL, ?, ?, 'selected', ?, ?)`,
		plan.ID, plan.OrganizationID, plan.ProjectID, mustJSON(snapshot), contentHash,
		plan.CreatedBy, now); err != nil {
		return CreativeTaskPlan{}, false, err
	}
	if err := insertReceipt(
		ctx, tx, actor, projectID, "strategy.creative_task_plan.create", key,
		requestHash, 201, plan, now,
	); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(
				ctx, actor, projectID, "strategy.creative_task_plan.create",
				key, requestHash, &prior,
			)
			return prior, found, readErr
		}
		return CreativeTaskPlan{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return CreativeTaskPlan{}, false, err
	}
	return plan, false, nil
}

func (s Service) GetCreativeTaskPlan(
	ctx context.Context,
	actor contract.ActorContext,
	planID string,
) (CreativeTaskPlan, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return CreativeTaskPlan{}, err
	}
	plan, err := scanCreativeTaskPlan(s.DB.QueryRowContext(
		ctx, creativeTaskPlanSelect+` WHERE organization_id = ? AND id = ?`,
		actor.OrganizationID, planID,
	))
	if err != nil {
		return CreativeTaskPlan{}, err
	}
	if _, err := s.project(ctx, actor, plan.ProjectID); err != nil {
		return CreativeTaskPlan{}, err
	}
	return s.hydrateCreativeTaskPlan(ctx, actor, plan)
}

func (s Service) hydrateCreativeTaskPlan(
	ctx context.Context,
	actor contract.ActorContext,
	plan CreativeTaskPlan,
) (CreativeTaskPlan, error) {
	profile, err := s.getCreativeBusinessVersion(ctx, plan.BusinessCode, plan.BusinessGeneration)
	if err != nil {
		return CreativeTaskPlan{}, err
	}
	plan.Profile = &profile
	if plan.CurrentStrategyVersion > 0 {
		version, err := s.getCreativeTaskStrategyVersion(
			ctx, actor, plan, plan.CurrentStrategyVersion,
		)
		if err != nil {
			return CreativeTaskPlan{}, err
		}
		plan.CurrentStrategy = &version
	}
	return plan, nil
}

func (s Service) ListCreativeTaskPlans(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	briefID string,
) ([]CreativeTaskPlan, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return nil, err
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return nil, err
	}
	query := creativeTaskPlanSelect + ` WHERE organization_id = ? AND project_id = ?`
	args := []any{actor.OrganizationID, projectID}
	if strings.TrimSpace(briefID) != "" {
		query += ` AND brief_id = ?`
		args = append(args, strings.TrimSpace(briefID))
	}
	query += ` ORDER BY updated_at DESC, created_at DESC`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []CreativeTaskPlan{}
	for rows.Next() {
		plan, err := scanCreativeTaskPlan(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, plan)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range result {
		hydrated, err := s.hydrateCreativeTaskPlan(ctx, actor, result[index])
		if err != nil {
			return nil, err
		}
		result[index] = hydrated
	}
	return result, nil
}

func (s Service) PatchCreativeTaskPlanAnswers(
	ctx context.Context,
	actor contract.ActorContext,
	key contract.IdempotencyKey,
	planID string,
	patch CreativeTaskAnswerPatch,
) (CreativeTaskPlan, bool, error) {
	if !s.CreativeTaskPlanningEnabled {
		return CreativeTaskPlan{}, false, ErrFeatureDisabled
	}
	if err := requireScope(actor, ScopeWrite); err != nil {
		return CreativeTaskPlan{}, false, err
	}
	if err := key.Validate(); err != nil || patch.ExpectedVersion < 1 ||
		len(patch.Operations) == 0 || len(patch.Operations) > 32 {
		return CreativeTaskPlan{}, false, ErrInvalidRequest
	}
	plan, err := scanCreativeTaskPlan(s.DB.QueryRowContext(
		ctx, creativeTaskPlanSelect+` WHERE organization_id = ? AND id = ?`,
		actor.OrganizationID, planID,
	))
	if err != nil {
		return CreativeTaskPlan{}, false, err
	}
	if _, err := s.project(ctx, actor, plan.ProjectID); err != nil {
		return CreativeTaskPlan{}, false, err
	}
	requestHash, _ := contract.CanonicalJSONHash(patch)
	var prior CreativeTaskPlan
	found, err := s.loadReceipt(
		ctx, actor, plan.ProjectID, "strategy.creative_task_plan.answers", key,
		requestHash, &prior,
	)
	if found || err != nil {
		return prior, found, err
	}
	profile, err := s.getCreativeBusinessVersion(ctx, plan.BusinessCode, plan.BusinessGeneration)
	if err != nil {
		return CreativeTaskPlan{}, false, err
	}
	brief, err := s.GetBriefVersion(ctx, actor, plan.BriefID, plan.BriefVersion)
	if err != nil {
		return CreativeTaskPlan{}, false, err
	}
	answers := cloneRawMap(plan.Answers)
	if err := applyCreativeTaskAnswerOperations(brief.Snapshot, profile, answers, patch.Operations); err != nil {
		return CreativeTaskPlan{}, false, err
	}
	if err := s.validateCreativeTaskAssetAnswers(
		ctx, actor, plan.ProjectID, profile, answers,
	); err != nil {
		return CreativeTaskPlan{}, false, err
	}
	beforeHash, _ := contract.CanonicalJSONHash(plan.Answers)
	afterHash, _ := contract.CanonicalJSONHash(answers)
	if beforeHash == afterHash {
		return CreativeTaskPlan{}, false, ErrInvalidRequest
	}
	completeness := evaluateCreativeTaskPlanCompleteness(brief.Snapshot, profile, answers)
	status := "collecting"
	if completeness.Ready {
		status = "ready"
	}
	now := s.now()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return CreativeTaskPlan{}, false, err
	}
	defer tx.Rollback()
	locked, err := scanCreativeTaskPlan(tx.QueryRowContext(
		ctx, creativeTaskPlanSelect+` WHERE organization_id = ? AND project_id = ? AND id = ? FOR UPDATE`,
		actor.OrganizationID, plan.ProjectID, plan.ID,
	))
	if err != nil {
		return CreativeTaskPlan{}, false, err
	}
	if locked.Version != patch.ExpectedVersion {
		return CreativeTaskPlan{}, false, ErrVersionConflict
	}
	if locked.Status == "generating" || locked.Status == "superseded" {
		return CreativeTaskPlan{}, false, ErrInvalidState
	}
	baseRevision := locked.CurrentRevision
	locked.Answers = answers
	locked.Completeness = completeness
	locked.Status = status
	locked.CurrentRevision++
	locked.Version++
	locked.UpdatedAt = now
	snapshot := locked.snapshot()
	contentHash, err := contract.NewContentHash(snapshot)
	if err != nil {
		return CreativeTaskPlan{}, false, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE strategy_creative_task_plans
		SET status = ?, answers = ?, completeness = ?, current_revision = ?,
		    version = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ?`,
		locked.Status, mustJSON(locked.Answers), mustJSON(locked.Completeness),
		locked.CurrentRevision, locked.Version, now, actor.OrganizationID,
		locked.ProjectID, locked.ID, plan.Version)
	if err != nil {
		return CreativeTaskPlan{}, false, err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return CreativeTaskPlan{}, false, ErrVersionConflict
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_creative_task_plan_revisions
		(plan_id, revision, organization_id, project_id, base_revision, snapshot, content_hash,
		 change_reason, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'answers_updated', ?, ?)`,
		locked.ID, locked.CurrentRevision, locked.OrganizationID, locked.ProjectID,
		baseRevision, mustJSON(snapshot), contentHash, actor.Principal.ID, now); err != nil {
		return CreativeTaskPlan{}, false, err
	}
	if err := insertReceipt(
		ctx, tx, actor, locked.ProjectID, "strategy.creative_task_plan.answers",
		key, requestHash, 200, locked, now,
	); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(
				ctx, actor, locked.ProjectID, "strategy.creative_task_plan.answers",
				key, requestHash, &prior,
			)
			return prior, found, readErr
		}
		return CreativeTaskPlan{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return CreativeTaskPlan{}, false, err
	}
	locked.Profile = &profile
	return locked, false, nil
}

func (p CreativeTaskPlan) snapshot() CreativeTaskPlanSnapshot {
	return CreativeTaskPlanSnapshot{
		BriefID: p.BriefID, BriefVersion: p.BriefVersion,
		BriefContentHash: p.BriefContentHash, SourceStrategyID: p.SourceStrategyID,
		SourceStrategyRevision: p.SourceStrategyRevision, SourceStrategyHash: p.SourceStrategyHash,
		BusinessCode: p.BusinessCode, BusinessGeneration: p.BusinessGeneration,
		BusinessVersion: p.BusinessVersion, BusinessContentHash: p.BusinessContentHash,
		SkillName: p.SkillName, SkillVersion: p.SkillVersion,
		SkillContentHash: p.SkillContentHash, SelectionSource: p.SelectionSource,
		RecommendationSnapshot: p.RecommendationSnapshot, Answers: cloneRawMap(p.Answers),
		Completeness: p.Completeness,
	}
}

func findRecommendation(
	snapshot RecommendationSnapshot,
	businessCode string,
) (*CreativeBusinessRecommendation, bool) {
	for index := range snapshot.Recommended {
		if snapshot.Recommended[index].BusinessCode == businessCode {
			return &snapshot.Recommended[index], true
		}
	}
	for index := range snapshot.Alternatives {
		if snapshot.Alternatives[index].BusinessCode == businessCode {
			return &snapshot.Alternatives[index], false
		}
	}
	return nil, false
}

func evaluateCreativeTaskPlanCompleteness(
	brief BriefDocument,
	profile creativecatalog.Profile,
	answers map[string]json.RawMessage,
) CreativeTaskPlanCompleteness {
	result := CreativeTaskPlanCompleteness{
		Blockers: []ValidationError{}, Warnings: []ValidationError{},
	}
	for _, question := range profile.Questions {
		if !questionApplies(question, answers) {
			continue
		}
		present := false
		if question.BriefSourcePath != "" {
			present = briefPathPresent(brief, question.BriefSourcePath)
		}
		if !present {
			value, found := answers[question.ID]
			if found {
				present = rawAnswerPresent(value)
			}
		}
		if present {
			continue
		}
		problem := ValidationError{
			Field:  "answers." + question.ID,
			Reason: "请补充：" + question.Label,
		}
		switch question.RequiredFor {
		case "strategy", "recommendation":
			result.Blockers = append(result.Blockers, problem)
		case "production":
			result.Warnings = append(result.Warnings, problem)
		}
	}
	result.Ready = len(result.Blockers) == 0
	return result
}

func applyCreativeTaskAnswerOperations(
	brief BriefDocument,
	profile creativecatalog.Profile,
	answers map[string]json.RawMessage,
	operations []CreativeTaskAnswerOperation,
) error {
	questions := map[string]creativecatalog.QuestionDefinition{}
	for _, question := range profile.Questions {
		questions[question.ID] = question
	}
	for _, operation := range operations {
		question, found := questions[strings.TrimSpace(operation.QuestionID)]
		if !found || (question.BriefSourcePath != "" &&
			briefPathPresent(brief, question.BriefSourcePath)) {
			return ErrInvalidRequest
		}
		switch operation.Op {
		case "remove":
			delete(answers, question.ID)
		case "set":
			if err := validateCreativeTaskAnswer(question, operation.Value); err != nil {
				return err
			}
			answers[question.ID] = append(json.RawMessage(nil), operation.Value...)
		default:
			return ErrInvalidRequest
		}
	}
	return nil
}

func validateCreativeTaskAnswer(
	question creativecatalog.QuestionDefinition,
	value json.RawMessage,
) error {
	if len(value) == 0 || len(value) > 64*1024 {
		return ErrInvalidRequest
	}
	switch question.Type {
	case "asset_ref":
		var answer contract.AssetVersionRef
		if json.Unmarshal(value, &answer) != nil || answer.Validate() != nil {
			return ErrInvalidRequest
		}
	case "text", "textarea", "reference_locator", "single_select":
		var answer string
		if json.Unmarshal(value, &answer) != nil {
			return ErrInvalidRequest
		}
		answer = strings.TrimSpace(answer)
		if question.Validation != nil && question.Validation.MaxLength > 0 &&
			len([]rune(answer)) > question.Validation.MaxLength {
			return ErrInvalidRequest
		}
		if question.Type == "single_select" && !questionOptionExists(question, answer) {
			return ErrInvalidRequest
		}
	case "multi_select":
		var answer []string
		if json.Unmarshal(value, &answer) != nil {
			return ErrInvalidRequest
		}
		if question.Validation != nil && question.Validation.MaxItems > 0 &&
			len(answer) > question.Validation.MaxItems {
			return ErrInvalidRequest
		}
		for _, item := range answer {
			if !questionOptionExists(question, item) {
				return ErrInvalidRequest
			}
		}
	case "boolean":
		var answer bool
		if json.Unmarshal(value, &answer) != nil {
			return ErrInvalidRequest
		}
	default:
		return ErrInvalidRequest
	}
	return nil
}

func questionOptionExists(question creativecatalog.QuestionDefinition, value string) bool {
	for _, option := range question.Options {
		if option.Value == value {
			return true
		}
	}
	return false
}

func questionApplies(
	question creativecatalog.QuestionDefinition,
	answers map[string]json.RawMessage,
) bool {
	if question.DependsOn == nil {
		return true
	}
	value, found := answers[question.DependsOn.QuestionID]
	if !found {
		return false
	}
	expected, err := json.Marshal(question.DependsOn.Equals)
	return err == nil && string(value) == string(expected)
}

func briefPathPresent(brief BriefDocument, path string) bool {
	switch path {
	case "audience.scenarios":
		return len(brief.Audience.Scenarios) > 0
	case "product.selling_points":
		return len(brief.Product.SellingPoints) > 0
	case "product.evidence":
		return len(brief.Product.Evidence) > 0
	case "audience.primary":
		return strings.TrimSpace(brief.Audience.Primary) != ""
	case "campaign.objective":
		return strings.TrimSpace(brief.Campaign.Objective) != ""
	case "proposition":
		return strings.TrimSpace(brief.Proposition) != ""
	default:
		return false
	}
}

func rawAnswerPresent(value json.RawMessage) bool {
	var decoded any
	if json.Unmarshal(value, &decoded) != nil {
		return false
	}
	switch item := decoded.(type) {
	case string:
		return strings.TrimSpace(item) != ""
	case []any:
		return len(item) > 0
	case nil:
		return false
	default:
		return true
	}
}

func cloneRawMap(values map[string]json.RawMessage) map[string]json.RawMessage {
	cloned := make(map[string]json.RawMessage, len(values))
	for key, value := range values {
		cloned[key] = append(json.RawMessage(nil), value...)
	}
	return cloned
}

func scanCreativeTaskPlan(row interface{ Scan(...any) error }) (CreativeTaskPlan, error) {
	var plan CreativeTaskPlan
	var recommendation, answers, completeness json.RawMessage
	if err := row.Scan(
		&plan.ID, &plan.OrganizationID, &plan.ProjectID, &plan.BriefID,
		&plan.BriefVersion, &plan.BriefContentHash, &plan.SourceStrategyID,
		&plan.SourceStrategyRevision, &plan.SourceStrategyHash, &plan.Status,
		&plan.BusinessCode, &plan.BusinessGeneration, &plan.BusinessVersion,
		&plan.BusinessContentHash, &plan.SkillName, &plan.SkillVersion,
		&plan.SkillContentHash, &plan.SelectionSource, &recommendation, &answers,
		&completeness, &plan.CurrentRevision, &plan.CurrentStrategyVersion,
		&plan.CurrentAgentTaskID, &plan.Version, &plan.CreatedBy, &plan.CreatedAt,
		&plan.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return CreativeTaskPlan{}, ErrNotFound
		}
		return CreativeTaskPlan{}, err
	}
	if err := json.Unmarshal(recommendation, &plan.RecommendationSnapshot); err != nil {
		return CreativeTaskPlan{}, err
	}
	if err := json.Unmarshal(answers, &plan.Answers); err != nil {
		return CreativeTaskPlan{}, err
	}
	if err := json.Unmarshal(completeness, &plan.Completeness); err != nil {
		return CreativeTaskPlan{}, err
	}
	if plan.Answers == nil {
		plan.Answers = map[string]json.RawMessage{}
	}
	return plan, nil
}
