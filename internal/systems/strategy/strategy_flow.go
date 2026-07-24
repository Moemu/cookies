package strategy

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/agent"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
	"github.com/shikanon/cookies/internal/platform/provider"
)

type SkillExecutionTrace struct {
	GenerationMode        string
	ProviderCode          string
	ModelAlias            string
	ModelVersion          string
	RouteRevisionID       string
	ResponseMode          provider.TextResponseMode
	PromptVersion         string
	SkillVersions         map[string]string
	SkillSnapshotHashes   map[string]string
	GenerationContextHash string
	Usage                 *provider.TokenUsage
	LatencyMS             int64
	ValidationAttempts    int
	QualityReport         *QualityReport
}

type CreateStrategyResult struct {
	Draft     Draft      `json:"strategy_draft"`
	AgentTask agent.Task `json:"agent_task"`
}

func (s Service) CreateStrategy(ctx context.Context, actor contract.ActorContext, key contract.IdempotencyKey, taskID, briefID string, briefVersion int64) (CreateStrategyResult, bool, error) {
	if err := requireScope(actor, ScopeWrite); err != nil {
		return CreateStrategyResult{}, false, err
	}
	if err := key.Validate(); err != nil || taskID == "" || briefID == "" || briefVersion < 1 {
		return CreateStrategyResult{}, false, ErrInvalidRequest
	}
	task, err := scanTask(s.DB.QueryRowContext(ctx, taskSelect+` WHERE organization_id = ? AND id = ?`, actor.OrganizationID, taskID))
	if err != nil {
		return CreateStrategyResult{}, false, err
	}
	if task.BriefID != briefID {
		return CreateStrategyResult{}, false, ErrInvalidRequest
	}
	projectContext, err := s.project(ctx, actor, task.ProjectID)
	if err != nil {
		return CreateStrategyResult{}, false, err
	}
	brief, err := s.GetBriefVersion(ctx, actor, briefID, briefVersion)
	if err != nil {
		return CreateStrategyResult{}, false, err
	}
	if err := s.ensureConcurrencyLimit(ctx, actor.OrganizationID, task.ProjectID, 4); err != nil {
		return CreateStrategyResult{}, false, err
	}
	request := struct {
		TaskID       string `json:"task_id"`
		BriefID      string `json:"brief_id"`
		BriefVersion int64  `json:"brief_version"`
	}{taskID, briefID, briefVersion}
	hash, _ := contract.CanonicalJSONHash(request)
	var prior CreateStrategyResult
	found, err := s.loadReceipt(ctx, actor, task.ProjectID, "strategy.create", key, hash, &prior)
	if found || err != nil {
		return prior, found, err
	}
	if err := s.ensureTextProviderReady(ctx, actor.OrganizationID); err != nil {
		return CreateStrategyResult{}, false, err
	}
	strategyID, err := s.newID("strategy")
	if err != nil {
		return CreateStrategyResult{}, false, err
	}
	agentTaskID, err := s.newID("agenttask")
	if err != nil {
		return CreateStrategyResult{}, false, err
	}
	now := s.now()
	skills, err := selectedSkillVersions(brief)
	if err != nil {
		return CreateStrategyResult{}, false, err
	}
	draft := Draft{
		ID: strategyID, OrganizationID: actor.OrganizationID, ProjectID: task.ProjectID,
		TaskID: taskID, BriefID: briefID, BriefVersion: briefVersion,
		ProjectContextVersion: projectContext.ProjectContextVersion, Status: "generating",
		CurrentRevision: 0, Version: 1, SkillVersions: skills, CreatedAt: now, UpdatedAt: now,
	}
	input := mustJSON(map[string]any{"strategy_id": strategyID, "brief_id": briefID, "brief_version": briefVersion})
	agentTask := agent.Task{
		ID: agentTaskID, OrganizationID: actor.OrganizationID, ProjectID: task.ProjectID,
		SourceSystem: "strategy", SourceType: "strategy_draft", SourceID: strategyID,
		Kind: AgentKindDraftGenerate, Status: agent.TaskDispatchPending, Version: 1,
		InputSnapshot: input, CreatedBy: actor.Principal, CreatedAt: now, UpdatedAt: now,
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return CreateStrategyResult{}, false, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_drafts
		(id, organization_id, project_id, task_id, brief_id, brief_version,
		 project_context_version, status, current_revision, version, skill_versions,
		 created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, 1, ?, ?, ?, ?)`,
		draft.ID, draft.OrganizationID, draft.ProjectID, draft.TaskID, draft.BriefID,
		draft.BriefVersion, draft.ProjectContextVersion, draft.Status, mustJSON(skills),
		actor.Principal.ID, now, now); err != nil {
		return CreateStrategyResult{}, false, err
	}
	writer, err := s.agentWriter()
	if err != nil {
		return CreateStrategyResult{}, false, err
	}
	if err := writer.CreateIn(ctx, tx, agent.CreateRequest{Task: agentTask}); err != nil {
		return CreateStrategyResult{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE strategy_tasks SET current_strategy_id = ?,
		current_agent_task_id = ?, version = version + 1, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		draft.ID, agentTask.ID, now, actor.OrganizationID, task.ProjectID, task.ID); err != nil {
		return CreateStrategyResult{}, false, err
	}
	result := CreateStrategyResult{Draft: draft, AgentTask: agentTask}
	if err := insertReceipt(ctx, tx, actor, task.ProjectID, "strategy.create", key, hash, 202, result, now); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(ctx, actor, task.ProjectID, "strategy.create", key, hash, &prior)
			return prior, found, readErr
		}
		return CreateStrategyResult{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return CreateStrategyResult{}, false, err
	}
	return result, false, nil
}

func (s Service) GetDraft(ctx context.Context, actor contract.ActorContext, id string) (Draft, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return Draft{}, err
	}
	draft, err := scanDraft(s.DB.QueryRowContext(ctx, draftSelect+` WHERE organization_id = ? AND id = ?`, actor.OrganizationID, id))
	if err != nil {
		return Draft{}, err
	}
	if _, err := s.project(ctx, actor, draft.ProjectID); err != nil {
		return Draft{}, err
	}
	if draft.CurrentRevision > 0 {
		revision, err := scanDraftRevision(s.DB.QueryRowContext(ctx, draftRevisionSelect+`
			WHERE organization_id = ? AND project_id = ? AND strategy_id = ? AND revision = ?`,
			actor.OrganizationID, draft.ProjectID, draft.ID, draft.CurrentRevision))
		if err != nil {
			return Draft{}, err
		}
		draft.Revision = &revision
	}
	return draft, nil
}

func (s Service) ListDraftRevisions(ctx context.Context, actor contract.ActorContext, strategyID string) ([]DraftRevision, error) {
	draft, err := s.GetDraft(ctx, actor, strategyID)
	if err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, draftRevisionSelect+` WHERE organization_id = ? AND project_id = ?
		AND strategy_id = ? ORDER BY revision DESC`, actor.OrganizationID, draft.ProjectID, strategyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var revisions []DraftRevision
	for rows.Next() {
		revision, err := scanDraftRevision(rows)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, revision)
	}
	if revisions == nil {
		revisions = []DraftRevision{}
	}
	return revisions, rows.Err()
}

func (s Service) GetDraftRevision(ctx context.Context, actor contract.ActorContext, strategyID string, revisionNumber int64) (DraftRevision, error) {
	draft, err := s.GetDraft(ctx, actor, strategyID)
	if err != nil {
		return DraftRevision{}, err
	}
	return scanDraftRevision(s.DB.QueryRowContext(ctx, draftRevisionSelect+` WHERE organization_id = ?
		AND project_id = ? AND strategy_id = ? AND revision = ?`, actor.OrganizationID,
		draft.ProjectID, strategyID, revisionNumber))
}

type StrategySectionPatch struct {
	ExpectedVersion int64           `json:"expected_version"`
	BaseRevision    int64           `json:"base_revision"`
	Section         string          `json:"section"`
	Value           json.RawMessage `json:"value"`
}

func (s Service) PatchStrategy(ctx context.Context, actor contract.ActorContext, key contract.IdempotencyKey, strategyID string, patch StrategySectionPatch) (Draft, bool, error) {
	if err := requireScope(actor, ScopeWrite); err != nil {
		return Draft{}, false, err
	}
	if err := key.Validate(); err != nil || patch.ExpectedVersion < 1 || patch.BaseRevision < 1 || !json.Valid(patch.Value) {
		return Draft{}, false, ErrInvalidRequest
	}
	draft, err := s.GetDraft(ctx, actor, strategyID)
	if err != nil {
		return Draft{}, false, err
	}
	hash, _ := contract.CanonicalJSONHash(patch)
	var prior Draft
	found, err := s.loadReceipt(ctx, actor, draft.ProjectID, "strategy.patch", key, hash, &prior)
	if found || err != nil {
		return prior, found, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Draft{}, false, err
	}
	defer tx.Rollback()
	locked, err := scanDraft(tx.QueryRowContext(ctx, draftSelect+` WHERE organization_id = ? AND project_id = ? AND id = ? FOR UPDATE`, actor.OrganizationID, draft.ProjectID, strategyID))
	if err != nil {
		return Draft{}, false, err
	}
	if locked.Version != patch.ExpectedVersion || locked.CurrentRevision != patch.BaseRevision {
		return Draft{}, false, ErrVersionConflict
	}
	if locked.Status != "draft" && locked.Status != "ready_for_review" && locked.Status != "returned" && locked.Status != "approved" {
		return Draft{}, false, ErrInvalidState
	}
	current, err := scanDraftRevision(tx.QueryRowContext(ctx, draftRevisionSelect+` WHERE organization_id = ?
		AND project_id = ? AND strategy_id = ? AND revision = ?`, actor.OrganizationID,
		draft.ProjectID, strategyID, locked.CurrentRevision))
	if err != nil {
		return Draft{}, false, err
	}
	document := current.Document
	if err := setStrategySection(&document, patch.Section, patch.Value); err != nil {
		return Draft{}, false, err
	}
	if err := document.Validate(); err != nil {
		return Draft{}, false, err
	}
	now := s.now()
	next := DraftRevision{StrategyID: strategyID, Revision: current.Revision + 1, BaseRevision: &current.Revision, Document: document, ChangedSections: []string{patch.Section}, CreatedBy: actor.Principal.ID, CreatedAt: now}
	next.ContentHash, err = contract.NewContentHash(next.Document)
	if err != nil {
		return Draft{}, false, err
	}
	if err := insertDraftRevision(ctx, tx, actor.OrganizationID, draft.ProjectID, next); err != nil {
		return Draft{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE strategy_reviews SET status = 'invalidated',
		updated_at = ? WHERE organization_id = ? AND project_id = ? AND strategy_id = ? AND status = 'open'`,
		now, actor.OrganizationID, draft.ProjectID, strategyID); err != nil {
		return Draft{}, false, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE strategy_drafts SET current_revision = ?, status = 'draft',
		current_review_id = NULL, version = version + 1, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ?`,
		next.Revision, now, actor.OrganizationID, draft.ProjectID, strategyID, locked.Version)
	if err != nil {
		return Draft{}, false, err
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return Draft{}, false, ErrVersionConflict
	}
	locked.CurrentRevision = next.Revision
	locked.Status = "draft"
	locked.CurrentReviewID = ""
	locked.Version++
	locked.UpdatedAt = now
	locked.Revision = &next
	if err := insertReceipt(ctx, tx, actor, draft.ProjectID, "strategy.patch", key, hash, 200, locked, now); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(ctx, actor, draft.ProjectID, "strategy.patch", key, hash, &prior)
			return prior, found, readErr
		}
		return Draft{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Draft{}, false, err
	}
	return locked, false, nil
}

type ReviseRequest struct {
	ExpectedVersion int64  `json:"expected_version"`
	BaseRevision    int64  `json:"base_revision"`
	Instruction     string `json:"instruction"`
}

func (s Service) ReviseStrategy(ctx context.Context, actor contract.ActorContext, key contract.IdempotencyKey, strategyID string, request ReviseRequest) (agent.Task, bool, error) {
	if err := requireScope(actor, ScopeWrite); err != nil {
		return agent.Task{}, false, err
	}
	if err := key.Validate(); err != nil || request.ExpectedVersion < 1 || request.BaseRevision < 1 ||
		strings.TrimSpace(request.Instruction) == "" || len(request.Instruction) > 4096 {
		return agent.Task{}, false, ErrInvalidRequest
	}
	draft, err := s.GetDraft(ctx, actor, strategyID)
	if err != nil {
		return agent.Task{}, false, err
	}
	if draft.Version != request.ExpectedVersion || draft.CurrentRevision != request.BaseRevision {
		return agent.Task{}, false, ErrVersionConflict
	}
	if err := s.ensureConcurrencyLimit(ctx, actor.OrganizationID, draft.ProjectID, 4); err != nil {
		return agent.Task{}, false, err
	}
	hash, _ := contract.CanonicalJSONHash(request)
	var prior agent.Task
	found, err := s.loadReceipt(ctx, actor, draft.ProjectID, "strategy.revise", key, hash, &prior)
	if found || err != nil {
		return prior, found, err
	}
	if err := s.ensureTextProviderReady(ctx, actor.OrganizationID); err != nil {
		return agent.Task{}, false, err
	}
	id, err := s.newID("agenttask")
	if err != nil {
		return agent.Task{}, false, err
	}
	now := s.now()
	task := agent.Task{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: draft.ProjectID,
		SourceSystem: "strategy", SourceType: "strategy_draft", SourceID: strategyID,
		Kind: AgentKindDraftRevise, Status: agent.TaskDispatchPending, Version: 1,
		InputSnapshot: mustJSON(request), CreatedBy: actor.Principal, CreatedAt: now, UpdatedAt: now,
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return agent.Task{}, false, err
	}
	defer tx.Rollback()
	writer, err := s.agentWriter()
	if err != nil {
		return agent.Task{}, false, err
	}
	if err := writer.CreateIn(ctx, tx, agent.CreateRequest{Task: task}); err != nil {
		return agent.Task{}, false, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE strategy_drafts SET status = 'generating',
		version = version + 1, updated_at = ? WHERE organization_id = ? AND project_id = ?
		AND id = ? AND version = ?`, now, actor.OrganizationID, draft.ProjectID,
		strategyID, draft.Version)
	if err != nil {
		return agent.Task{}, false, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return agent.Task{}, false, err
	}
	if changed != 1 {
		return agent.Task{}, false, ErrVersionConflict
	}
	if err := insertReceipt(ctx, tx, actor, draft.ProjectID, "strategy.revise", key, hash, 202, task, now); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(ctx, actor, draft.ProjectID, "strategy.revise", key, hash, &prior)
			return prior, found, readErr
		}
		return agent.Task{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return agent.Task{}, false, err
	}
	return task, false, nil
}

func (s Service) HandleAgentTask(ctx context.Context, task agent.Task) (*contract.ResourceRef, error) {
	if ref, found, err := s.completedAgentResult(ctx, task); found || err != nil {
		return ref, err
	}
	switch task.Kind {
	case AgentKindBriefExtract:
		return s.handleBriefExtract(ctx, task)
	case AgentKindDraftGenerate:
		return s.handleDraftGenerate(ctx, task)
	case AgentKindDraftRevise:
		return s.handleDraftRevise(ctx, task)
	default:
		return nil, jobruntime.ExecutionError{JobError: contract.JobError{Code: "STRATEGY_TASK_KIND_UNSUPPORTED", Message: "Strategy task kind is unsupported"}}
	}
}

func (s Service) completedAgentResult(ctx context.Context, task agent.Task) (*contract.ResourceRef, bool, error) {
	var exists int
	err := s.DB.QueryRowContext(ctx, `SELECT 1 FROM platform_skill_runs
		WHERE organization_id = ? AND project_id = ? AND agent_task_id = ? AND status = 'succeeded'
		LIMIT 1`, task.OrganizationID, task.ProjectID, task.ID).Scan(&exists)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	switch task.Kind {
	case AgentKindBriefExtract:
		strategyTask, err := scanTask(s.DB.QueryRowContext(ctx, taskSelect+` WHERE organization_id = ?
			AND project_id = ? AND id = ?`, task.OrganizationID, task.ProjectID, task.SourceID))
		if err != nil {
			return nil, true, err
		}
		draft, err := scanBriefDraft(s.DB.QueryRowContext(ctx, briefDraftSelect+` WHERE organization_id = ?
			AND project_id = ? AND brief_id = ? ORDER BY created_at DESC LIMIT 1`,
			task.OrganizationID, task.ProjectID, strategyTask.BriefID))
		if err != nil {
			return nil, true, err
		}
		version := draft.Version
		return &contract.ResourceRef{Type: "strategy.brief_draft", ID: draft.ID, Version: &version}, true, nil
	default:
		draft, err := scanDraft(s.DB.QueryRowContext(ctx, draftSelect+` WHERE organization_id = ?
			AND project_id = ? AND id = ?`, task.OrganizationID, task.ProjectID, task.SourceID))
		if err != nil {
			return nil, true, err
		}
		version := draft.CurrentRevision
		return &contract.ResourceRef{Type: "strategy.draft_revision", ID: draft.ID, Version: &version}, true, nil
	}
}

func (s Service) handleBriefExtract(ctx context.Context, agentTask agent.Task) (*contract.ResourceRef, error) {
	var input struct {
		StrategyTaskID string `json:"strategy_task_id"`
		MessageID      string `json:"message_id"`
	}
	if err := json.Unmarshal(agentTask.InputSnapshot, &input); err != nil {
		return nil, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	task, err := scanTask(tx.QueryRowContext(ctx, taskSelect+` WHERE organization_id = ? AND project_id = ? AND id = ? FOR UPDATE`,
		agentTask.OrganizationID, agentTask.ProjectID, input.StrategyTaskID))
	if err != nil {
		return nil, err
	}
	message, err := scanMessage(tx.QueryRowContext(ctx, messageSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`,
		agentTask.OrganizationID, agentTask.ProjectID, input.MessageID))
	if err != nil {
		return nil, err
	}
	draft, err := scanBriefDraft(tx.QueryRowContext(ctx, briefDraftSelect+` WHERE organization_id = ? AND project_id = ?
		AND brief_id = ? ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, agentTask.OrganizationID, agentTask.ProjectID, task.BriefID))
	if err != nil {
		return nil, err
	}
	patch, trace, err := s.generateBriefPatch(ctx, agentTask, draft, message)
	if err != nil {
		return nil, err
	}
	updated := draft
	if len(patch.Operations) > 0 {
		updated, err = ApplyBriefPatch(draft, patch, PatchFromModel, agentTask.ID, s.now())
		if err != nil {
			return nil, err
		}
		if err := updateBriefDraft(ctx, tx, draft.Version, updated); err != nil {
			return nil, err
		}
	}
	skillRunID, err := s.insertSkillRun(ctx, tx, agentTask, "strategy.brief.extract", "v2.0.0", trace, patch)
	if err != nil {
		return nil, err
	}
	assistantID, _ := s.newID("msg")
	now := s.now()
	assistant := Message{
		ID: assistantID, OrganizationID: agentTask.OrganizationID, ProjectID: agentTask.ProjectID,
		ConversationID: task.ConversationID, Role: "assistant", ContentType: "text",
		Content: briefAssistantText(updated), AIGenerated: true, AgentTaskID: agentTask.ID,
		SkillRunIDs: []string{skillRunID}, CreatedBy: "strategy-agent", CreatedAt: now,
	}
	if err := insertMessage(ctx, tx, assistant); err != nil {
		return nil, err
	}
	eventID, _ := s.newID("stratevent")
	payload := mustJSON(map[string]any{"message_id": assistant.ID, "brief_id": draft.BriefID, "brief_draft_version": updated.Version})
	if err := insertConversationEvent(ctx, tx, eventID, agentTask.OrganizationID, agentTask.ProjectID, task.ConversationID, "brief.updated", payload, now); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE strategy_tasks SET status = ?, version = version + 1,
		updated_at = ? WHERE organization_id = ? AND project_id = ? AND id = ?`,
		map[bool]string{true: "ready_to_confirm", false: "waiting_user"}[updated.Completeness.Ready],
		now, agentTask.OrganizationID, agentTask.ProjectID, task.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	version := updated.Version
	return &contract.ResourceRef{Type: "strategy.brief_draft", ID: updated.ID, Version: &version}, nil
}

func (s Service) handleDraftGenerate(ctx context.Context, agentTask agent.Task) (*contract.ResourceRef, error) {
	draft, err := scanDraft(s.DB.QueryRowContext(ctx, draftSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`,
		agentTask.OrganizationID, agentTask.ProjectID, agentTask.SourceID))
	if err != nil {
		return nil, err
	}
	if draft.CurrentRevision > 0 && draft.Status != "generating" {
		version := draft.CurrentRevision
		return &contract.ResourceRef{Type: "strategy.draft_revision", ID: draft.ID, Version: &version}, nil
	}
	brief, err := scanBriefVersion(s.DB.QueryRowContext(ctx, briefVersionSelect+` WHERE organization_id = ?
		AND project_id = ? AND brief_id = ? AND version = ?`, agentTask.OrganizationID,
		agentTask.ProjectID, draft.BriefID, draft.BriefVersion))
	if err != nil {
		return nil, err
	}
	document, trace, err := s.generateStrategy(ctx, agentTask, brief, draft)
	if err != nil {
		return nil, err
	}
	return s.persistGeneratedRevision(ctx, agentTask, draft, document, []string{"all"}, "strategy.strategy.generate", "v2.0.0", trace)
}

func (s Service) handleDraftRevise(ctx context.Context, agentTask agent.Task) (*contract.ResourceRef, error) {
	var request ReviseRequest
	if err := json.Unmarshal(agentTask.InputSnapshot, &request); err != nil {
		return nil, err
	}
	draft, err := scanDraft(s.DB.QueryRowContext(ctx, draftSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`,
		agentTask.OrganizationID, agentTask.ProjectID, agentTask.SourceID))
	if err != nil {
		return nil, err
	}
	if draft.CurrentRevision != request.BaseRevision {
		return nil, ErrVersionConflict
	}
	revision, err := scanDraftRevision(s.DB.QueryRowContext(ctx, draftRevisionSelect+` WHERE organization_id = ?
		AND project_id = ? AND strategy_id = ? AND revision = ?`, agentTask.OrganizationID,
		agentTask.ProjectID, draft.ID, draft.CurrentRevision))
	if err != nil {
		return nil, err
	}
	brief, err := scanBriefVersion(s.DB.QueryRowContext(ctx, briefVersionSelect+` WHERE organization_id = ?
		AND project_id = ? AND brief_id = ? AND version = ?`, agentTask.OrganizationID,
		agentTask.ProjectID, draft.BriefID, draft.BriefVersion))
	if err != nil {
		return nil, err
	}
	document, changed, trace, err := s.generateStrategyRevision(ctx, agentTask, brief, draft, revision.Document, request)
	if err != nil {
		return nil, err
	}
	return s.persistGeneratedRevision(ctx, agentTask, draft, document, changed, "strategy.strategy.revise", "v2.0.0", trace)
}

func (s Service) persistGeneratedRevision(ctx context.Context, agentTask agent.Task, draft Draft, document StrategyDocument, changed []string, skillName, skillVersion string, trace SkillExecutionTrace) (*contract.ResourceRef, error) {
	if err := document.Validate(); err != nil {
		return nil, jobruntime.ExecutionError{JobError: contract.JobError{Code: "MODEL_OUTPUT_INVALID", Message: err.Error()}}
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	locked, err := scanDraft(tx.QueryRowContext(ctx, draftSelect+` WHERE organization_id = ? AND project_id = ? AND id = ? FOR UPDATE`,
		agentTask.OrganizationID, agentTask.ProjectID, draft.ID))
	if err != nil {
		return nil, err
	}
	if locked.Status != "generating" {
		if locked.CurrentRevision > 0 {
			version := locked.CurrentRevision
			return &contract.ResourceRef{Type: "strategy.draft_revision", ID: locked.ID, Version: &version}, nil
		}
		return nil, ErrInvalidState
	}
	now := s.now()
	revisionNumber := locked.CurrentRevision + 1
	var base *int64
	if locked.CurrentRevision > 0 {
		value := locked.CurrentRevision
		base = &value
	}
	revision := DraftRevision{StrategyID: locked.ID, Revision: revisionNumber, BaseRevision: base, Document: document, ChangedSections: changed, CreatedBy: agentTask.ID, CreatedAt: now}
	revision.ContentHash, err = contract.NewContentHash(document)
	if err != nil {
		return nil, err
	}
	if err := insertDraftRevision(ctx, tx, agentTask.OrganizationID, agentTask.ProjectID, revision); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE strategy_reviews SET status = 'invalidated',
		updated_at = ? WHERE organization_id = ? AND project_id = ? AND strategy_id = ? AND status = 'open'`,
		now, agentTask.OrganizationID, agentTask.ProjectID, locked.ID); err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE strategy_drafts SET status = 'draft',
		current_revision = ?, current_review_id = NULL, version = version + 1, updated_at = ? WHERE organization_id = ?
		AND project_id = ? AND id = ? AND status = 'generating'`, revisionNumber, now,
		agentTask.OrganizationID, agentTask.ProjectID, locked.ID)
	if err != nil {
		return nil, err
	}
	changedRows, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if changedRows != 1 {
		return nil, ErrVersionConflict
	}
	if _, err := s.insertSkillRun(ctx, tx, agentTask, skillName, skillVersion, trace, document); err != nil {
		return nil, err
	}
	task, err := scanTask(tx.QueryRowContext(ctx, taskSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`,
		agentTask.OrganizationID, agentTask.ProjectID, locked.TaskID))
	if err == nil {
		eventID, _ := s.newID("stratevent")
		payload := mustJSON(map[string]any{"strategy_id": locked.ID, "revision": revisionNumber, "content_hash": revision.ContentHash})
		if err := insertConversationEvent(ctx, tx, eventID, agentTask.OrganizationID, agentTask.ProjectID,
			task.ConversationID, "strategy.revision.created", payload, now); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &contract.ResourceRef{Type: "strategy.draft_revision", ID: locked.ID, Version: &revisionNumber}, nil
}

func (s Service) insertSkillRun(ctx context.Context, tx *sql.Tx, task agent.Task, name, version string, trace SkillExecutionTrace, output any) (string, error) {
	id, err := s.newID("skillrun")
	if err != nil {
		return "", err
	}
	inputHash, err := contract.CanonicalJSONHash(task.InputSnapshot)
	if err != nil {
		return "", err
	}
	now := s.now()
	var inputTokens, outputTokens, totalTokens any
	if trace.Usage != nil {
		inputTokens, outputTokens, totalTokens = trace.Usage.InputTokens, trace.Usage.OutputTokens, trace.Usage.TotalTokens
	}
	var quality any
	if trace.QualityReport != nil {
		quality = mustJSON(trace.QualityReport)
	}
	outputHash, err := contract.CanonicalJSONHash(output)
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO platform_skill_runs
		(id, organization_id, project_id, agent_task_id, skill_name, skill_version, status,
		 input_hash, provider_code, model_version, generation_mode, model_alias,
		 route_revision_id, response_mode, prompt_version, skill_versions, skill_snapshot_hashes,
		 generation_context_hash,
		 input_tokens, output_tokens, total_tokens, latency_ms, validation_attempts, quality_report,
		 output_snapshot, output_hash, started_at, completed_at,
		 created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 'succeeded', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, task.OrganizationID, task.ProjectID, task.ID, name, version, inputHash,
		trace.ProviderCode, trace.ModelVersion, trace.GenerationMode, trace.ModelAlias,
		trace.RouteRevisionID, trace.ResponseMode, trace.PromptVersion, mustJSON(trace.SkillVersions),
		mustJSON(trace.SkillSnapshotHashes), trace.GenerationContextHash,
		inputTokens, outputTokens, totalTokens, trace.LatencyMS, trace.ValidationAttempts, quality,
		mustJSON(output), outputHash, now, now, now, now)
	return id, err
}

func (s Service) generateBriefPatch(ctx context.Context, task agent.Task, draft BriefDraft, message Message) (BriefPatch, SkillExecutionTrace, error) {
	started := time.Now()
	promptVersion := "strategy.brief.extract.v2"
	if s.Text == nil {
		return deterministicBriefPatch(draft, message), SkillExecutionTrace{
			GenerationMode: "deterministic", ProviderCode: "deterministic", ModelVersion: "v1",
			PromptVersion: promptVersion, SkillVersions: map[string]string{"strategy.brief.extract": "v2.0.0"},
			LatencyMS: time.Since(started).Milliseconds(), ValidationAttempts: 1,
		}, nil
	}
	actor := contract.ActorContext{OrganizationID: task.OrganizationID, Principal: task.CreatedBy, Scopes: []contract.Scope{provider.ScopeTextGenerate}}
	projectContext, err := s.Projects.GetContext(ctx, actor, task.ProjectID)
	if err != nil {
		return BriefPatch{}, SkillExecutionTrace{}, err
	}
	modelAlias := s.TextModelAlias
	if strings.TrimSpace(modelAlias) == "" {
		modelAlias = "cookies.text.standard"
	}
	prompt := fmt.Sprintf("Current brief draft (version %d): %s\nUser message: %s\nReturn only a constrained brief patch. Never mark fields confirmed.",
		draft.Version, mustJSON(draft.Document), message.Content)
	response, err := s.Text.GenerateText(ctx, provider.TextGenerateRequest{
		Actor: actor, Project: projectContext, ModelAlias: modelAlias,
		InvocationKey: contract.IdempotencyKey("agent-" + task.ID + "-brief"),
		Messages: []provider.TextMessage{
			{Role: provider.TextRoleSystem, Content: `Extract advertising requirements. Use only allowed fields and cite the supplied conversation message. For the channels field, return exactly ["xiaohongshu"]; never translate or decorate this enum.`},
			{Role: provider.TextRoleUser, Content: prompt},
		},
		OutputJSONSchema: briefPatchOutputSchema(),
	})
	if err != nil {
		return BriefPatch{}, SkillExecutionTrace{}, textGenerationError(err)
	}
	trace := SkillExecutionTrace{
		GenerationMode: "provider", ProviderCode: response.ProviderCode, ModelAlias: modelAlias,
		ModelVersion: response.ModelVersion, RouteRevisionID: response.RouteRevisionID,
		ResponseMode: response.ResponseMode, PromptVersion: promptVersion,
		SkillVersions: map[string]string{"strategy.brief.extract": "v2.0.0"},
		Usage:         response.Usage, LatencyMS: time.Since(started).Milliseconds(), ValidationAttempts: 1,
	}
	if len(response.StructuredOutput) == 0 && response.ProviderCode == "fake" {
		trace.GenerationMode = "fake_template"
		return deterministicBriefPatch(draft, message), trace, nil
	}
	var patch BriefPatch
	candidate := response.StructuredOutput
	if len(candidate) == 0 {
		candidate = normalizeJSONCandidate(response.Text)
	}
	if err := decodeStrictJSON(candidate, &patch); err != nil {
		return BriefPatch{}, SkillExecutionTrace{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "MODEL_OUTPUT_INVALID", Message: "Brief patch output is invalid"}}
	}
	if err := normalizeModelBriefPatch(&patch); err != nil {
		return BriefPatch{}, SkillExecutionTrace{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "MODEL_OUTPUT_INVALID", Message: "Brief patch contains unsupported values"}}
	}
	patch.ContractVersion = "strategy-brief-patch/v1"
	patch.BaseVersion = draft.Version
	for index := range patch.Operations {
		patch.Operations[index].Source = FieldSource{Type: "conversation_message", ID: message.ID}
		patch.Operations[index].Confirmation = "unconfirmed"
	}
	return patch, trace, nil
}

func (s Service) generateStrategy(ctx context.Context, task agent.Task, brief BriefVersion, draft Draft) (StrategyDocument, SkillExecutionTrace, error) {
	started := time.Now()
	generation, err := s.buildGenerationContext(ctx, task, brief, draft)
	if err != nil {
		return StrategyDocument{}, SkillExecutionTrace{}, err
	}
	versions := skillVersions(generation.Skills)
	snapshotHashes := skillSnapshotHashes(generation.Skills)
	contextHash, err := contract.CanonicalJSONHash(generation)
	if err != nil {
		return StrategyDocument{}, SkillExecutionTrace{}, err
	}
	if s.Text == nil {
		document := deterministicStrategy(brief, draft)
		report := evaluateStrategyQuality(document, generation)
		return document, SkillExecutionTrace{
			GenerationMode: "deterministic", ProviderCode: "deterministic", ModelVersion: "v1",
			PromptVersion: generation.PromptVersion, SkillVersions: versions,
			SkillSnapshotHashes: snapshotHashes, GenerationContextHash: contextHash,
			LatencyMS: time.Since(started).Milliseconds(), ValidationAttempts: 1, QualityReport: &report,
		}, nil
	}
	actor := contract.ActorContext{OrganizationID: task.OrganizationID, Principal: task.CreatedBy, Scopes: []contract.Scope{provider.ScopeTextGenerate}}
	modelAlias := s.TextModelAlias
	if strings.TrimSpace(modelAlias) == "" {
		modelAlias = "cookies.text.standard"
	}
	response, err := s.Text.GenerateText(ctx, provider.TextGenerateRequest{
		Actor: actor, Project: generation.Project, ModelAlias: modelAlias,
		InvocationKey: contract.IdempotencyKey("agent-" + task.ID + "-strategy"),
		Messages: []provider.TextMessage{
			{Role: provider.TextRoleSystem, Content: strategySystemPrompt(generation)},
			{Role: provider.TextRoleUser, Content: strategyUserPrompt(generation)},
		},
		OutputJSONSchema: strategyOutputSchema(),
	})
	if err != nil {
		return StrategyDocument{}, SkillExecutionTrace{}, textGenerationError(err)
	}
	trace := SkillExecutionTrace{
		GenerationMode: "provider", ProviderCode: response.ProviderCode, ModelAlias: modelAlias,
		ModelVersion: response.ModelVersion, RouteRevisionID: response.RouteRevisionID,
		ResponseMode: response.ResponseMode, PromptVersion: generation.PromptVersion,
		SkillVersions: versions, SkillSnapshotHashes: snapshotHashes,
		GenerationContextHash: contextHash, Usage: response.Usage, ValidationAttempts: 1,
	}
	if len(response.StructuredOutput) == 0 && response.ProviderCode == "fake" {
		document := deterministicStrategy(brief, draft)
		report := evaluateStrategyQuality(document, generation)
		trace.GenerationMode = "fake_template"
		trace.QualityReport = &report
		trace.LatencyMS = time.Since(started).Milliseconds()
		return document, trace, nil
	}
	document, report, decodeErr := decodeAndEvaluateStrategy(response, generation, brief, draft)
	if decodeErr == nil {
		normalizeGeneratedStrategy(&document, brief, draft)
		report = evaluateStrategyQuality(document, generation)
	}
	needsRepair := decodeErr != nil || (s.CriticEnabled && !report.Passed)
	if needsRepair {
		reasons := report.Errors
		if decodeErr != nil {
			reasons = append(reasons, decodeErr.Error())
		}
		repair, repairErr := s.Text.GenerateText(ctx, provider.TextGenerateRequest{
			Actor: actor, Project: generation.Project, ModelAlias: modelAlias,
			InvocationKey: contract.IdempotencyKey("agent-" + task.ID + "-strategy-repair"),
			Messages: []provider.TextMessage{
				{Role: provider.TextRoleSystem, Content: strategySystemPrompt(generation)},
				{Role: provider.TextRoleUser, Content: strategyUserPrompt(generation)},
				{Role: provider.TextRoleAssistant, Content: response.Text},
				{Role: provider.TextRoleUser, Content: "上一个输出未通过校验。只修复这些问题并返回完整 JSON：\n- " + strings.Join(reasons, "\n- ")},
			},
			OutputJSONSchema: strategyOutputSchema(),
		})
		if repairErr != nil {
			return StrategyDocument{}, SkillExecutionTrace{}, textGenerationError(repairErr)
		}
		trace.ValidationAttempts = 2
		trace.ProviderCode = repair.ProviderCode
		trace.ModelVersion = repair.ModelVersion
		trace.RouteRevisionID = repair.RouteRevisionID
		trace.ResponseMode = repair.ResponseMode
		trace.Usage = addUsage(trace.Usage, repair.Usage)
		document, report, decodeErr = decodeAndEvaluateStrategy(repair, generation, brief, draft)
		if decodeErr == nil {
			normalizeGeneratedStrategy(&document, brief, draft)
			report = evaluateStrategyQuality(document, generation)
		}
	}
	if decodeErr != nil || !report.Passed {
		return StrategyDocument{}, SkillExecutionTrace{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "MODEL_OUTPUT_INVALID", Message: "Strategy output failed structural or quality validation"}}
	}
	trace.QualityReport = &report
	trace.LatencyMS = time.Since(started).Milliseconds()
	return document, trace, nil
}

func (s Service) generateStrategyRevision(ctx context.Context, task agent.Task, brief BriefVersion, draft Draft, current StrategyDocument, request ReviseRequest) (StrategyDocument, []string, SkillExecutionTrace, error) {
	started := time.Now()
	generation, err := s.buildGenerationContext(ctx, task, brief, draft)
	if err != nil {
		return StrategyDocument{}, nil, SkillExecutionTrace{}, err
	}
	versions := skillVersions(generation.Skills)
	snapshotHashes := skillSnapshotHashes(generation.Skills)
	contextHash, err := contract.CanonicalJSONHash(generation)
	if err != nil {
		return StrategyDocument{}, nil, SkillExecutionTrace{}, err
	}
	if s.Text == nil {
		document := current
		document.AssumptionsAndGaps = append(document.AssumptionsAndGaps, "用户修订要求："+strings.TrimSpace(request.Instruction))
		report := evaluateStrategyQuality(document, generation)
		return document, []string{"assumptions_and_gaps"}, SkillExecutionTrace{
			GenerationMode: "deterministic", ProviderCode: "deterministic", ModelVersion: "v1",
			PromptVersion: "strategy.revise.v2", SkillVersions: versions,
			SkillSnapshotHashes: snapshotHashes, GenerationContextHash: contextHash,
			LatencyMS: time.Since(started).Milliseconds(), ValidationAttempts: 1, QualityReport: &report,
		}, nil
	}
	actor := contract.ActorContext{OrganizationID: task.OrganizationID, Principal: task.CreatedBy, Scopes: []contract.Scope{provider.ScopeTextGenerate}}
	modelAlias := strings.TrimSpace(s.TextModelAlias)
	if modelAlias == "" {
		modelAlias = "cookies.text.standard"
	}
	currentJSON := string(mustJSON(current))
	allowedSections := allowedRevisionSections(request.Instruction)
	response, err := s.Text.GenerateText(ctx, provider.TextGenerateRequest{
		Actor: actor, Project: generation.Project, ModelAlias: modelAlias,
		InvocationKey: contract.IdempotencyKey("agent-" + task.ID + "-strategy-revise"),
		Messages: []provider.TextMessage{
			{Role: provider.TextRoleSystem, Content: strategySystemPrompt(generation) + "\n只修改服务端允许的章节；其他章节保持语义和数据不变。"},
			{Role: provider.TextRoleUser, Content: strategyUserPrompt(generation) + "\n\n<current_strategy>\n" + currentJSON + "\n</current_strategy>\n<allowed_sections>\n" + strings.Join(allowedSections, ",") + "\n</allowed_sections>\n<revision_instruction>\n" + strings.TrimSpace(request.Instruction) + "\n</revision_instruction>"},
		},
		OutputJSONSchema: strategyOutputSchema(),
	})
	if err != nil {
		return StrategyDocument{}, nil, SkillExecutionTrace{}, textGenerationError(err)
	}
	trace := SkillExecutionTrace{
		GenerationMode: "provider", ProviderCode: response.ProviderCode, ModelAlias: modelAlias,
		ModelVersion: response.ModelVersion, RouteRevisionID: response.RouteRevisionID,
		ResponseMode: response.ResponseMode, PromptVersion: "strategy.revise.v2",
		SkillVersions: versions, SkillSnapshotHashes: snapshotHashes,
		GenerationContextHash: contextHash, Usage: response.Usage, LatencyMS: time.Since(started).Milliseconds(),
		ValidationAttempts: 1,
	}
	if len(response.StructuredOutput) == 0 && response.ProviderCode == "fake" {
		document := current
		document.AssumptionsAndGaps = append(document.AssumptionsAndGaps, "用户修订要求："+strings.TrimSpace(request.Instruction))
		report := evaluateStrategyQuality(document, generation)
		trace.GenerationMode = "fake_template"
		trace.QualityReport = &report
		return document, []string{"assumptions_and_gaps"}, trace, nil
	}
	document, report, decodeErr := decodeAndEvaluateStrategy(response, generation, brief, draft)
	if decodeErr == nil {
		retainAllowedRevisionSections(current, &document, allowedSections)
		report = evaluateStrategyQuality(document, generation)
	}
	if decodeErr != nil || !report.Passed {
		return StrategyDocument{}, nil, SkillExecutionTrace{}, jobruntime.ExecutionError{JobError: contract.JobError{
			Code: "MODEL_OUTPUT_INVALID", Message: "Strategy revision failed structural or quality validation",
		}}
	}
	changed := changedStrategySections(current, document)
	if len(changed) == 0 {
		return StrategyDocument{}, nil, SkillExecutionTrace{}, jobruntime.ExecutionError{JobError: contract.JobError{
			Code: "MODEL_OUTPUT_INVALID", Message: "Strategy revision did not change any section",
		}}
	}
	trace.QualityReport = &report
	return document, changed, trace, nil
}

func decodeAndEvaluateStrategy(response provider.SynchronousResponse, generation GenerationContext, brief BriefVersion, draft Draft) (StrategyDocument, QualityReport, error) {
	candidate := response.StructuredOutput
	if len(candidate) == 0 {
		candidate = normalizeJSONCandidate(response.Text)
	}
	var document StrategyDocument
	if err := decodeStrictJSON(candidate, &document); err != nil {
		return StrategyDocument{}, QualityReport{Errors: []string{"strategy output is not valid JSON"}}, err
	}
	document.ContractVersion = "strategy-draft/v1"
	document.Lineage = StrategyLineage{
		BriefID: brief.BriefID, BriefVersion: brief.Version,
		ProjectContextVersion: draft.ProjectContextVersion,
		SkillVersions:         skillVersions(generation.Skills),
	}
	report := evaluateStrategyQuality(document, generation)
	return document, report, nil
}

func normalizeGeneratedStrategy(document *StrategyDocument, brief BriefVersion, draft Draft) {
	fallback := deterministicStrategy(brief, draft)
	document.Objective = brief.Snapshot.Campaign.Objective
	document.Audience.Primary = brief.Snapshot.Audience.Primary
	document.Proposition = brief.Snapshot.Proposition
	if !hasNonEmptyStrings(document.Audience.Insights) {
		document.Audience.Insights = fallback.Audience.Insights
	}
	if len(document.ChannelStrategy) == 0 {
		document.ChannelStrategy = fallback.ChannelStrategy
	}
	for index := range document.ChannelStrategy {
		switch strings.ToLower(strings.TrimSpace(document.ChannelStrategy[index].Platform)) {
		case "xiaohongshu", "小红书", "小红书图文", "rednote", "red note", "redbook", "red book":
			document.ChannelStrategy[index].Platform = "xiaohongshu"
		}
	}
	for _, recommendation := range fallback.CreativeRecommendations {
		if len(document.CreativeRecommendations) >= 3 {
			break
		}
		document.CreativeRecommendations = append(document.CreativeRecommendations, recommendation)
	}
	validExperiments := make([]Experiment, 0, len(document.ExperimentMatrix))
	for _, experiment := range document.ExperimentMatrix {
		if strings.TrimSpace(experiment.Hypothesis) != "" &&
			strings.TrimSpace(experiment.Variable) != "" &&
			strings.TrimSpace(experiment.Metric) != "" {
			validExperiments = append(validExperiments, experiment)
		}
	}
	if len(validExperiments) == 0 {
		validExperiments = fallback.ExperimentMatrix
	}
	document.ExperimentMatrix = validExperiments
	primaryKPI := strings.TrimSpace(brief.Snapshot.Measurement.PrimaryKPI)
	if primaryKPI != "" && !sliceContainsInput(document.Measurement, primaryKPI) {
		document.Measurement = append([]string{primaryKPI}, document.Measurement...)
	}
	if !hasNonEmptyStrings(document.Measurement) {
		document.Measurement = fallback.Measurement
	}
	if strings.TrimSpace(document.BudgetAndCadence.Budget) == "" {
		document.BudgetAndCadence.Budget = brief.Snapshot.Budget.Total
	}
	if strings.TrimSpace(document.BudgetAndCadence.Cadence) == "" {
		document.BudgetAndCadence.Cadence = fallback.BudgetAndCadence.Cadence
	}
}

func normalizeJSONCandidate(value string) json.RawMessage {
	content := strings.TrimSpace(value)
	if strings.HasPrefix(content, "```") {
		if newline := strings.IndexByte(content, '\n'); newline >= 0 {
			content = content[newline+1:]
		}
		content = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(content), "```"))
	}
	return json.RawMessage(content)
}

func addUsage(first, second *provider.TokenUsage) *provider.TokenUsage {
	if first == nil {
		return second
	}
	if second == nil {
		return first
	}
	return &provider.TokenUsage{
		InputTokens:  first.InputTokens + second.InputTokens,
		OutputTokens: first.OutputTokens + second.OutputTokens,
		TotalTokens:  first.TotalTokens + second.TotalTokens,
	}
}

func textGenerationError(err error) error {
	var providerError provider.ExecutionError
	if errors.As(err, &providerError) {
		return jobruntime.ExecutionError{JobError: providerError.JobError}
	}
	return jobruntime.ExecutionError{JobError: contract.JobError{
		Code: "PROVIDER_UNAVAILABLE", Message: "Text provider is unavailable", Retryable: true,
	}}
}

func briefPatchOutputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object","additionalProperties":false,"required":["operations"],
		"properties":{"operations":{"type":"array","maxItems":32,"items":{"type":"object",
		"additionalProperties":false,"required":["op","field_path","value","confidence"],
		"properties":{"op":{"const":"set"},"field_path":{"enum":["campaign.objective","audience.primary","proposition","channels","budget.total","schedule.window","constraints","measurement.primary_kpi"]},
		"value":{"oneOf":[{"type":"string"},{"type":"array","items":{"type":"string"}}]},"confidence":{"enum":["low","medium","high"]}}}}}
	}`)
}

func strategyOutputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object","additionalProperties":false,
		"required":["contract_version","objective","audience","proposition","channel_strategy","creative_recommendations","constraints","budget_and_cadence","experiment_matrix","measurement","assumptions_and_gaps"],
		"properties":{
			"contract_version":{"type":"string"},"objective":{"type":"string","minLength":1},
			"audience":{"type":"object","additionalProperties":false,"required":["primary","insights"],"properties":{"primary":{"type":"string","minLength":1},"insights":{"type":"array","minItems":1,"maxItems":3,"items":{"type":"string","maxLength":80}}}},
			"proposition":{"type":"string","minLength":1},
			"channel_strategy":{"type":"array","minItems":1,"maxItems":1,"items":{"type":"object","additionalProperties":false,"required":["platform","role","formats"],"properties":{"platform":{"const":"xiaohongshu"},"role":{"type":"string","maxLength":80},"formats":{"type":"array","minItems":1,"maxItems":3,"items":{"type":"string","maxLength":50}}}}},
			"creative_recommendations":{"type":"array","minItems":3,"maxItems":3,"items":{"type":"string","maxLength":120}},"constraints":{"type":"array","maxItems":8,"items":{"type":"string","maxLength":100}},
			"budget_and_cadence":{"type":"object","additionalProperties":false,"required":["budget","cadence"],"properties":{"budget":{"type":"string","maxLength":80},"cadence":{"type":"string","maxLength":100}}},
			"experiment_matrix":{"type":"array","minItems":1,"maxItems":2,"items":{"type":"object","additionalProperties":false,"required":["hypothesis","variable","metric"],"properties":{"hypothesis":{"type":"string","maxLength":100},"variable":{"type":"string","maxLength":60},"metric":{"type":"string","maxLength":60}}}},
			"measurement":{"type":"array","minItems":1,"maxItems":4,"items":{"type":"string","maxLength":80}},"assumptions_and_gaps":{"type":"array","maxItems":5,"items":{"type":"string","maxLength":80}}
		}}
	`)
}

func decodeStrictJSON(value json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("structured output must contain exactly one JSON value")
	}
	return nil
}

func deterministicBriefPatch(draft BriefDraft, message Message) BriefPatch {
	content := strings.TrimSpace(message.Content)
	source := FieldSource{Type: "conversation_message", ID: message.ID}
	operations := []BriefPatchOperation{}
	add := func(path string, value any) {
		operations = append(operations, BriefPatchOperation{Op: "set", FieldPath: path, Value: mustJSON(value), Source: source, Confidence: "medium", Confirmation: "unconfirmed"})
	}
	fields := []struct {
		path  string
		label string
	}{
		{"campaign.objective", "目标"}, {"audience.primary", "受众"}, {"proposition", "卖点"}, {"budget.total", "预算"},
		{"schedule.window", "周期"}, {"measurement.primary_kpi", "指标"},
	}
	objectiveExtracted := false
	for _, field := range fields {
		pattern := regexp.MustCompile(field.label + `\s*[:：]\s*([^，。;；\n]+)`)
		if match := pattern.FindStringSubmatch(content); len(match) == 2 {
			add(field.path, strings.TrimSpace(match[1]))
			objectiveExtracted = objectiveExtracted || field.path == "campaign.objective"
		}
	}
	if draft.Document.Campaign.Objective == "" && !objectiveExtracted {
		add("campaign.objective", content)
	}
	if len(draft.Document.Channels) == 0 {
		add("channels", []string{"xiaohongshu"})
	}
	return BriefPatch{ContractVersion: "strategy-brief-patch/v1", BaseVersion: draft.Version, Operations: operations}
}

func briefAssistantText(draft BriefDraft) string {
	if draft.Completeness.Ready {
		return "Brief 已完整并通过确认规则，可以创建不可变版本。"
	}
	fields := make([]string, 0, len(draft.Completeness.Blockers))
	for _, blocker := range draft.Completeness.Blockers {
		fields = append(fields, blocker.Field)
	}
	return "我已整理本轮信息。请在右侧确认或补充这些字段：" + strings.Join(fields, "、")
}

func deterministicStrategy(brief BriefVersion, draft Draft) StrategyDocument {
	channels := make([]ChannelStrategy, 0, len(brief.Snapshot.Channels))
	for _, channel := range brief.Snapshot.Channels {
		channels = append(channels, ChannelStrategy{Platform: channel, Role: "建立种草认知并承接搜索决策", Formats: []string{"图文笔记", "清单型笔记"}})
	}
	gaps := []string{}
	if brief.Snapshot.Budget.Total == "" {
		gaps = append(gaps, "预算待确认")
	}
	if brief.Snapshot.Schedule.Window == "" {
		gaps = append(gaps, "排期待确认")
	}
	measurement := []string{brief.Snapshot.Measurement.PrimaryKPI}
	if strings.TrimSpace(brief.Snapshot.Measurement.PrimaryKPI) == "" {
		gaps = append(gaps, "核心指标待确认")
		measurement = []string{"核心指标待确认；生成前先定义与目标匹配的主指标"}
	}
	return StrategyDocument{
		ContractVersion: "strategy-draft/v1", Objective: brief.Snapshot.Campaign.Objective,
		Audience:    StrategyAudience{Primary: brief.Snapshot.Audience.Primary, Insights: []string{"围绕真实使用场景提供可验证信息"}},
		Proposition: brief.Snapshot.Proposition, ChannelStrategy: channels,
		CreativeRecommendations: []string{"用首屏问题直击目标人群", "正文用证据和场景支撑核心卖点", "结尾设置低门槛互动"},
		Constraints:             brief.Snapshot.Constraints,
		BudgetAndCadence:        BudgetAndCadence{Budget: brief.Snapshot.Budget.Total, Cadence: "首期 2 周验证，按周复盘"},
		ExperimentMatrix:        []Experiment{{Hypothesis: "场景痛点开篇可提高有效阅读", Variable: "首屏表达", Metric: "阅读完成率"}},
		Measurement:             measurement,
		AssumptionsAndGaps:      gaps,
		Lineage:                 StrategyLineage{BriefID: brief.BriefID, BriefVersion: brief.Version, ProjectContextVersion: draft.ProjectContextVersion, SkillVersions: draft.SkillVersions},
	}
}

func setStrategySection(document *StrategyDocument, section string, value json.RawMessage) error {
	switch section {
	case "objective":
		return decodeString(value, &document.Objective, section)
	case "proposition":
		return decodeString(value, &document.Proposition, section)
	case "creative_recommendations":
		return decodeStringSlice(value, &document.CreativeRecommendations, section)
	case "constraints":
		return decodeStringSlice(value, &document.Constraints, section)
	case "measurement":
		return decodeStringSlice(value, &document.Measurement, section)
	case "assumptions_and_gaps":
		return decodeStringSlice(value, &document.AssumptionsAndGaps, section)
	default:
		return fmt.Errorf("%w: strategy section %q is not writable", ErrInvalidRequest, section)
	}
}

func decodeStringSlice(raw json.RawMessage, target *[]string, section string) error {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return fmt.Errorf("%w: %s must be a string array", ErrInvalidRequest, section)
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%w: %s entries must not be empty", ErrInvalidRequest, section)
		}
	}
	*target = values
	return nil
}
