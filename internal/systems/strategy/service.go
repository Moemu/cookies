package strategy

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"

	"github.com/shikanon/cookies/internal/platform/agent"
	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/ids"
	"github.com/shikanon/cookies/internal/platform/knowledge"
	"github.com/shikanon/cookies/internal/platform/provider"
)

const (
	AgentKindBriefExtract  = "strategy.brief.extract"
	AgentKindDraftGenerate = "strategy.draft.generate"
	AgentKindDraftRevise   = "strategy.draft.revise"
	AgentKindReviewDeep    = "strategy.review.deep"
)

type ProjectReader interface {
	GetContext(context.Context, contract.ActorContext, contract.ProjectID) (contract.ProjectContext, error)
}

type KnowledgeReader interface {
	GetReference(context.Context, contract.ActorContext, contract.ProjectID, string) (knowledge.Reference, error)
}

type CreativeAssetReader interface {
	Get(context.Context, contract.ActorContext, contract.ProjectID, contract.AssetVersionRef) (assets.ProjectAsset, error)
	OpenPreview(context.Context, contract.ActorContext, contract.ProjectID, contract.AssetVersionRef) (io.ReadCloser, assets.ObjectInfo, error)
	ListFeatures(context.Context, contract.ActorContext, contract.ProjectID, int) ([]assets.AssetFeature, error)
}

type Service struct {
	DB                          *sql.DB
	Projects                    ProjectReader
	Knowledge                   KnowledgeReader
	CreativeAssets              CreativeAssetReader
	Agents                      agent.TransactionalTaskWriter
	Text                        *provider.Service
	TextModelAlias              string
	DeepReviewModelAlias        string
	PromptVersion               string
	ConversationPromptVersion   string
	RevisePromptVersion         string
	ReviewPromptVersion         string
	RepairPromptVersion         string
	CreativeTaskPromptVersion   string
	CriticEnabled               bool
	ContextSelectionEnabled     bool
	ContextSelector             ContextSelector
	V2Enabled                   bool
	CreativeTaskPlanningEnabled bool
	DisableApproval             bool
	AllowedOrganizations        map[contract.OrganizationID]struct{}
	NewID                       func(string) (string, error)
	Now                         func() time.Time
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s Service) newID(prefix string) (string, error) {
	if s.NewID != nil {
		return s.NewID(prefix)
	}
	return ids.New(prefix)
}

func requireScope(actor contract.ActorContext, scope contract.Scope) error {
	if !actor.HasScope(scope) {
		return ErrScopeRequired
	}
	return nil
}

func (s Service) project(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) (contract.ProjectContext, error) {
	if len(s.AllowedOrganizations) > 0 {
		if _, ok := s.AllowedOrganizations[actor.OrganizationID]; !ok {
			return contract.ProjectContext{}, ErrProjectAccessDenied
		}
	}
	if s.Projects == nil {
		return contract.ProjectContext{}, fmt.Errorf("project service is required")
	}
	projectContext, err := s.Projects.GetContext(ctx, actor, projectID)
	if err != nil || projectContext.OrganizationID != actor.OrganizationID || projectContext.ProjectID != projectID {
		return contract.ProjectContext{}, ErrProjectAccessDenied
	}
	if err := projectContext.ValidateBrandBound(); err != nil {
		return contract.ProjectContext{}, ErrInvalidState
	}
	return projectContext, nil
}

func (s Service) AuthorizeProject(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, scope contract.Scope) error {
	if err := requireScope(actor, scope); err != nil {
		return err
	}
	_, err := s.project(ctx, actor, projectID)
	return err
}

func (s Service) CreateWorkspace(ctx context.Context, actor contract.ActorContext, key contract.IdempotencyKey, projectID contract.ProjectID, name string) (Workspace, bool, error) {
	if err := requireScope(actor, ScopeWrite); err != nil {
		return Workspace{}, false, err
	}
	if err := key.Validate(); err != nil || strings.TrimSpace(name) == "" || len(strings.TrimSpace(name)) > 255 {
		return Workspace{}, false, ErrInvalidRequest
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return Workspace{}, false, err
	}
	request := struct {
		ProjectID contract.ProjectID `json:"project_id"`
		Name      string             `json:"name"`
	}{ProjectID: projectID, Name: strings.TrimSpace(name)}
	hash, _ := contract.CanonicalJSONHash(request)
	var prior Workspace
	found, err := s.loadReceipt(ctx, actor, projectID, "workspace.create", key, hash, &prior)
	if found || err != nil {
		return prior, found, err
	}
	id, err := s.newID("strategyws")
	if err != nil {
		return Workspace{}, false, err
	}
	now := s.now()
	workspace := Workspace{ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID, Name: request.Name, Status: "active", Version: 1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Workspace{}, false, err
	}
	defer tx.Rollback()
	var primaryCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM strategy_workspaces
		WHERE organization_id = ? AND project_id = ? AND status = 'active' AND is_primary = TRUE`,
		actor.OrganizationID, projectID).Scan(&primaryCount); err != nil {
		return Workspace{}, false, err
	}
	workspace.IsPrimary = primaryCount == 0
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_workspaces
		(id, organization_id, project_id, name, is_primary, status, version, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		workspace.ID, workspace.OrganizationID, workspace.ProjectID, workspace.Name, workspace.IsPrimary,
		workspace.Status, workspace.Version, workspace.CreatedBy, now, now); err != nil {
		return Workspace{}, false, err
	}
	if err := insertReceipt(ctx, tx, actor, projectID, "workspace.create", key, hash, 201, workspace, now); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(ctx, actor, projectID, "workspace.create", key, hash, &prior)
			return prior, found, readErr
		}
		return Workspace{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Workspace{}, false, err
	}
	return workspace, false, nil
}

func (s Service) ListWorkspaces(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) ([]Workspace, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return nil, err
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, workspaceSelect+` WHERE organization_id = ? AND project_id = ? ORDER BY is_primary DESC, created_at`, actor.OrganizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Workspace
	for rows.Next() {
		workspace, err := scanWorkspace(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, workspace)
	}
	if result == nil {
		result = []Workspace{}
	}
	return result, rows.Err()
}

func (s Service) CreateTask(
	ctx context.Context,
	actor contract.ActorContext,
	key contract.IdempotencyKey,
	projectID contract.ProjectID,
	request CreateTaskRequest,
) (TaskBundle, bool, error) {
	if err := requireScope(actor, ScopeWrite); err != nil {
		return TaskBundle{}, false, err
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Objective = strings.TrimSpace(request.Objective)
	if err := key.Validate(); err != nil || request.Name == "" || len(request.Name) > 255 ||
		request.Objective == "" || len(request.Objective) > 4096 {
		return TaskBundle{}, false, ErrInvalidRequest
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return TaskBundle{}, false, err
	}
	requestHash, _ := contract.CanonicalJSONHash(struct {
		ProjectID contract.ProjectID `json:"project_id"`
		Request   CreateTaskRequest  `json:"request"`
	}{ProjectID: projectID, Request: request})
	var prior TaskBundle
	found, err := s.loadReceipt(ctx, actor, projectID, "strategy.task.create", key, requestHash, &prior)
	if found || err != nil {
		return prior, found, err
	}
	workspaceID, err := s.newID("strategyws")
	if err != nil {
		return TaskBundle{}, false, err
	}
	conversationID, err := s.newID("conversation")
	if err != nil {
		return TaskBundle{}, false, err
	}
	taskID, err := s.newID("strategytask")
	if err != nil {
		return TaskBundle{}, false, err
	}
	briefID, err := s.newID("brief")
	if err != nil {
		return TaskBundle{}, false, err
	}
	draftID, err := s.newID("briefdraft")
	if err != nil {
		return TaskBundle{}, false, err
	}
	now := s.now()
	workspace := Workspace{
		ID: workspaceID, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		Name: request.Name, Status: "active", Version: 1, CreatedBy: actor.Principal.ID,
		CreatedAt: now, UpdatedAt: now,
	}
	conversation := Conversation{
		ID: conversationID, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		WorkspaceID: workspaceID, Status: "open", Version: 1, CreatedBy: actor.Principal.ID,
		CreatedAt: now, UpdatedAt: now,
	}
	task := Task{
		ID: taskID, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		WorkspaceID: workspaceID, ConversationID: conversationID, BriefID: briefID,
		Status: "active", Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	document := EmptyBriefDocument()
	if s.V2Enabled {
		document = EmptyBriefDocumentV2()
	}
	document.Campaign.Objective = request.Objective
	states := map[string]FieldState{
		"campaign.objective": {
			FieldPath:  "campaign.objective",
			Source:     FieldSource{Type: "user_edit", ID: actor.Principal.ID},
			Confidence: "high", Confirmation: "confirmed", UpdatedBy: actor.Principal.ID,
			UpdatedAt: now, Conflicts: []FieldSource{},
		},
	}
	draft := BriefDraft{
		ID: draftID, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		BriefID: briefID, Status: "open", Version: 1, Document: document,
		FieldStates: states, Completeness: ComputeCompleteness(document, states),
		UpdatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now,
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return TaskBundle{}, false, err
	}
	defer tx.Rollback()
	var primaryCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM strategy_workspaces
		WHERE organization_id = ? AND project_id = ? AND status = 'active' AND is_primary = TRUE`,
		actor.OrganizationID, projectID).Scan(&primaryCount); err != nil {
		return TaskBundle{}, false, err
	}
	workspace.IsPrimary = primaryCount == 0
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_workspaces
		(id, organization_id, project_id, name, is_primary, status, version, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		workspace.ID, workspace.OrganizationID, workspace.ProjectID, workspace.Name, workspace.IsPrimary,
		workspace.Status, workspace.Version, workspace.CreatedBy, now, now); err != nil {
		return TaskBundle{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_conversations
		(id, organization_id, project_id, workspace_id, status, version, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, conversation.ID, conversation.OrganizationID, conversation.ProjectID,
		conversation.WorkspaceID, conversation.Status, conversation.Version, conversation.CreatedBy, now, now); err != nil {
		return TaskBundle{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_tasks
		(id, organization_id, project_id, workspace_id, conversation_id, brief_id, status, version, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, task.ID, task.OrganizationID, task.ProjectID,
		task.WorkspaceID, task.ConversationID, task.BriefID, task.Status, task.Version,
		actor.Principal.ID, now, now); err != nil {
		return TaskBundle{}, false, err
	}
	documentJSON, _ := snapshotJSON(document)
	statesJSON, _ := snapshotJSON(states)
	completenessJSON, _ := snapshotJSON(draft.Completeness)
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_briefs
		(id, organization_id, project_id, latest_draft_id, latest_version, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, ?, ?)`, briefID, actor.OrganizationID, projectID, draftID, now, now); err != nil {
		return TaskBundle{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_brief_drafts
		(id, organization_id, project_id, brief_id, status, version, document, field_states, completeness, updated_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, draft.ID, draft.OrganizationID, draft.ProjectID,
		draft.BriefID, draft.Status, draft.Version, documentJSON, statesJSON, completenessJSON,
		draft.UpdatedBy, now, now); err != nil {
		return TaskBundle{}, false, err
	}
	bundle := TaskBundle{Workspace: workspace, Conversation: conversation, Task: task, BriefDraft: draft}
	if err := insertReceipt(ctx, tx, actor, projectID, "strategy.task.create", key, requestHash, 201, bundle, now); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(ctx, actor, projectID, "strategy.task.create", key, requestHash, &prior)
			return prior, found, readErr
		}
		return TaskBundle{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return TaskBundle{}, false, err
	}
	return bundle, false, nil
}

func (s Service) ListTasks(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) ([]TaskListItem, error) {
	return s.ListTasksByLifecycle(ctx, actor, projectID, "active")
}

func (s Service) ListTasksByLifecycle(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, lifecycle string) ([]TaskListItem, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return nil, err
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return nil, err
	}
	lifecycleClause := ""
	switch lifecycle {
	case "", "active":
		lifecycleClause = " AND t.discarded_at IS NULL AND sd.archived_at IS NULL"
	case "archived":
		lifecycleClause = " AND (t.discarded_at IS NOT NULL OR sd.archived_at IS NOT NULL)"
	case "all":
	default:
		return nil, ErrInvalidRequest
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT
		t.id, t.organization_id, t.project_id, t.workspace_id, t.conversation_id, t.brief_id,
		COALESCE(t.current_agent_task_id, ''), COALESCE(t.current_strategy_id, ''),
		t.status, t.discarded_at, COALESCE(t.discarded_by, ''), COALESCE(t.discard_reason, ''),
		t.version, t.created_at, t.updated_at,
		w.name,
		COALESCE(JSON_UNQUOTE(JSON_EXTRACT(bd.document, '$.campaign.objective')), ''),
		bd.status,
		COALESCE(JSON_EXTRACT(bd.completeness, '$.ready'), FALSE),
		COALESCE(sd.status, ''),
		COALESCE(sr.status, ''), sd.archived_at, COALESCE(sd.archived_by, ''),
		COALESCE(sd.archive_reason, ''), COALESCE(sd.current_revision, 0),
		COALESCE(sd.version, 0)
		FROM strategy_tasks t
		JOIN strategy_workspaces w ON w.organization_id = t.organization_id AND w.project_id = t.project_id
			AND w.id = t.workspace_id
		JOIN strategy_briefs b ON b.organization_id = t.organization_id AND b.project_id = t.project_id
			AND b.id = t.brief_id
		JOIN strategy_brief_drafts bd ON bd.organization_id = b.organization_id AND bd.project_id = b.project_id
			AND bd.id = b.latest_draft_id
		LEFT JOIN strategy_drafts sd ON sd.organization_id = t.organization_id AND sd.project_id = t.project_id
			AND sd.id = t.current_strategy_id
		LEFT JOIN strategy_reviews sr ON sr.organization_id = sd.organization_id AND sr.project_id = sd.project_id
			AND sr.id = sd.current_review_id
		WHERE t.organization_id = ? AND t.project_id = ?`+lifecycleClause+`
		ORDER BY t.updated_at DESC, t.created_at DESC`, actor.OrganizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]TaskListItem, 0)
	for rows.Next() {
		var item TaskListItem
		var discardedAt, archivedAt sql.NullTime
		if err := rows.Scan(
			&item.Task.ID, &item.Task.OrganizationID, &item.Task.ProjectID, &item.Task.WorkspaceID,
			&item.Task.ConversationID, &item.Task.BriefID, &item.Task.CurrentAgentTaskID,
			&item.Task.CurrentStrategyID, &item.Task.Status, &discardedAt,
			&item.Task.DiscardedBy, &item.Task.DiscardReason, &item.Task.Version,
			&item.Task.CreatedAt, &item.Task.UpdatedAt, &item.Name, &item.Objective,
			&item.BriefStatus, &item.BriefReady, &item.StrategyStatus, &item.ReviewStatus,
			&archivedAt, &item.StrategyArchivedBy, &item.StrategyArchiveReason,
			&item.StrategyRevision, &item.StrategyVersion,
		); err != nil {
			return nil, err
		}
		if discardedAt.Valid {
			item.Task.DiscardedAt = &discardedAt.Time
		}
		if archivedAt.Valid {
			item.StrategyArchivedAt = &archivedAt.Time
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s Service) GetWorkspace(ctx context.Context, actor contract.ActorContext, id string) (Workspace, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return Workspace{}, err
	}
	workspace, err := scanWorkspace(s.DB.QueryRowContext(ctx, workspaceSelect+` WHERE organization_id = ? AND id = ?`, actor.OrganizationID, id))
	if err != nil {
		return Workspace{}, err
	}
	if _, err := s.project(ctx, actor, workspace.ProjectID); err != nil {
		return Workspace{}, err
	}
	return workspace, nil
}

type WorkspaceDetail struct {
	Workspace           Workspace     `json:"workspace"`
	CurrentConversation *Conversation `json:"current_conversation,omitempty"`
	CurrentTask         *Task         `json:"current_task,omitempty"`
}

func (s Service) GetWorkspaceDetail(ctx context.Context, actor contract.ActorContext, id string) (WorkspaceDetail, error) {
	workspace, err := s.GetWorkspace(ctx, actor, id)
	if err != nil {
		return WorkspaceDetail{}, err
	}
	result := WorkspaceDetail{Workspace: workspace}
	conversation, err := scanConversation(s.DB.QueryRowContext(ctx, conversationSelect+`
		WHERE organization_id = ? AND project_id = ? AND workspace_id = ?
		ORDER BY created_at DESC LIMIT 1`, actor.OrganizationID, workspace.ProjectID, id))
	if errors.Is(err, ErrNotFound) {
		return result, nil
	}
	if err != nil {
		return WorkspaceDetail{}, err
	}
	result.CurrentConversation = &conversation
	task, err := scanTask(s.DB.QueryRowContext(ctx, taskSelect+` WHERE organization_id = ?
		AND project_id = ? AND conversation_id = ? ORDER BY created_at DESC LIMIT 1`,
		actor.OrganizationID, workspace.ProjectID, conversation.ID))
	if errors.Is(err, ErrNotFound) {
		return result, nil
	}
	if err != nil {
		return WorkspaceDetail{}, err
	}
	result.CurrentTask = &task
	return result, nil
}

type ConversationBundle struct {
	Conversation Conversation `json:"conversation"`
	Task         Task         `json:"task"`
	BriefDraft   BriefDraft   `json:"brief_draft"`
}

func (s Service) CreateConversation(ctx context.Context, actor contract.ActorContext, key contract.IdempotencyKey, projectID contract.ProjectID, workspaceID string) (ConversationBundle, bool, error) {
	if err := requireScope(actor, ScopeWrite); err != nil {
		return ConversationBundle{}, false, err
	}
	if err := key.Validate(); err != nil || workspaceID == "" {
		return ConversationBundle{}, false, ErrInvalidRequest
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return ConversationBundle{}, false, err
	}
	workspace, err := scanWorkspace(s.DB.QueryRowContext(ctx, workspaceSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`, actor.OrganizationID, projectID, workspaceID))
	if err != nil {
		return ConversationBundle{}, false, err
	}
	if workspace.Status != "active" {
		return ConversationBundle{}, false, ErrInvalidState
	}
	request := struct {
		ProjectID   contract.ProjectID `json:"project_id"`
		WorkspaceID string             `json:"workspace_id"`
	}{projectID, workspaceID}
	hash, _ := contract.CanonicalJSONHash(request)
	var prior ConversationBundle
	found, err := s.loadReceipt(ctx, actor, projectID, "conversation.create", key, hash, &prior)
	if found || err != nil {
		return prior, found, err
	}
	conversationID, err := s.newID("conversation")
	if err != nil {
		return ConversationBundle{}, false, err
	}
	taskID, err := s.newID("strategytask")
	if err != nil {
		return ConversationBundle{}, false, err
	}
	briefID, err := s.newID("brief")
	if err != nil {
		return ConversationBundle{}, false, err
	}
	draftID, err := s.newID("briefdraft")
	if err != nil {
		return ConversationBundle{}, false, err
	}
	now := s.now()
	conversation := Conversation{ID: conversationID, OrganizationID: actor.OrganizationID, ProjectID: projectID, WorkspaceID: workspaceID, Status: "open", Version: 1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now}
	task := Task{ID: taskID, OrganizationID: actor.OrganizationID, ProjectID: projectID, WorkspaceID: workspaceID, ConversationID: conversationID, BriefID: briefID, Status: "active", Version: 1, CreatedAt: now, UpdatedAt: now}
	document := EmptyBriefDocument()
	if s.V2Enabled {
		document = EmptyBriefDocumentV2()
	}
	draft := BriefDraft{ID: draftID, OrganizationID: actor.OrganizationID, ProjectID: projectID, BriefID: briefID, Status: "open", Version: 1, Document: document, FieldStates: map[string]FieldState{}, Completeness: ComputeCompleteness(document, nil), UpdatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ConversationBundle{}, false, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_conversations
		(id, organization_id, project_id, workspace_id, status, version, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, conversation.ID, conversation.OrganizationID, conversation.ProjectID,
		conversation.WorkspaceID, conversation.Status, conversation.Version, conversation.CreatedBy, now, now); err != nil {
		return ConversationBundle{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_tasks
		(id, organization_id, project_id, workspace_id, conversation_id, brief_id, status, version, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, task.ID, task.OrganizationID, task.ProjectID,
		task.WorkspaceID, task.ConversationID, task.BriefID, task.Status, task.Version, actor.Principal.ID, now, now); err != nil {
		return ConversationBundle{}, false, err
	}
	documentJSON, _ := snapshotJSON(document)
	statesJSON, _ := snapshotJSON(draft.FieldStates)
	completenessJSON, _ := snapshotJSON(draft.Completeness)
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_briefs
		(id, organization_id, project_id, latest_draft_id, latest_version, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, ?, ?)`, briefID, actor.OrganizationID, projectID, draftID, now, now); err != nil {
		return ConversationBundle{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_brief_drafts
		(id, organization_id, project_id, brief_id, status, version, document, field_states, completeness, updated_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, draft.ID, draft.OrganizationID, draft.ProjectID,
		draft.BriefID, draft.Status, draft.Version, documentJSON, statesJSON, completenessJSON, draft.UpdatedBy, now, now); err != nil {
		return ConversationBundle{}, false, err
	}
	bundle := ConversationBundle{Conversation: conversation, Task: task, BriefDraft: draft}
	if err := insertReceipt(ctx, tx, actor, projectID, "conversation.create", key, hash, 201, bundle, now); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(ctx, actor, projectID, "conversation.create", key, hash, &prior)
			return prior, found, readErr
		}
		return ConversationBundle{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return ConversationBundle{}, false, err
	}
	return bundle, false, nil
}

func (s Service) agentWriter() (agent.TransactionalTaskWriter, error) {
	if s.Agents == nil {
		return nil, fmt.Errorf("agent transactional writer is required")
	}
	return s.Agents, nil
}

func (s Service) GetConversation(ctx context.Context, actor contract.ActorContext, id string) (Conversation, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return Conversation{}, err
	}
	conversation, err := scanConversation(s.DB.QueryRowContext(ctx, conversationSelect+` WHERE organization_id = ? AND id = ?`, actor.OrganizationID, id))
	if err != nil {
		return Conversation{}, err
	}
	if _, err := s.project(ctx, actor, conversation.ProjectID); err != nil {
		return Conversation{}, err
	}
	return conversation, nil
}

func (s Service) ListMessages(ctx context.Context, actor contract.ActorContext, conversationID string, after string, limit int) ([]Message, error) {
	conversation, err := s.GetConversation(ctx, actor, conversationID)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	query := messageSelect + ` WHERE organization_id = ? AND project_id = ?
		AND conversation_id = ? ORDER BY created_at, id LIMIT ?`
	args := []any{actor.OrganizationID, conversation.ProjectID, conversationID, limit}
	if after != "" {
		var cursorCreatedAt time.Time
		err := s.DB.QueryRowContext(ctx, `SELECT created_at FROM strategy_messages
			WHERE organization_id = ? AND project_id = ? AND conversation_id = ? AND id = ?`,
			actor.OrganizationID, conversation.ProjectID, conversationID, after).Scan(&cursorCreatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEventCursorExpired
		}
		if err != nil {
			return nil, err
		}
		query = messageSelect + ` WHERE organization_id = ? AND project_id = ?
			AND conversation_id = ? AND (created_at > ? OR (created_at = ? AND id > ?))
			ORDER BY created_at, id LIMIT ?`
		args = []any{actor.OrganizationID, conversation.ProjectID, conversationID, cursorCreatedAt, cursorCreatedAt, after, limit}
	}
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var messages []Message
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	if messages == nil {
		messages = []Message{}
	}
	return messages, rows.Err()
}

type SendMessageResult struct {
	Message   Message    `json:"message"`
	AgentTask agent.Task `json:"agent_task"`
}

func (s Service) SendMessage(ctx context.Context, actor contract.ActorContext, key contract.IdempotencyKey, conversationID, content string) (SendMessageResult, bool, error) {
	if err := requireScope(actor, ScopeWrite); err != nil {
		return SendMessageResult{}, false, err
	}
	if err := key.Validate(); err != nil || strings.TrimSpace(content) == "" || len(content) > 64<<10 {
		return SendMessageResult{}, false, ErrInvalidRequest
	}
	conversation, err := scanConversation(s.DB.QueryRowContext(ctx, conversationSelect+` WHERE organization_id = ? AND id = ?`, actor.OrganizationID, conversationID))
	if err != nil {
		return SendMessageResult{}, false, err
	}
	if _, err := s.project(ctx, actor, conversation.ProjectID); err != nil {
		return SendMessageResult{}, false, err
	}
	if conversation.Status == "closed" {
		return SendMessageResult{}, false, ErrInvalidState
	}
	if err := s.ensureConcurrencyLimit(ctx, actor.OrganizationID, conversation.ProjectID, 4); err != nil {
		return SendMessageResult{}, false, err
	}
	task, err := scanTask(s.DB.QueryRowContext(ctx, taskSelect+` WHERE organization_id = ? AND project_id = ? AND conversation_id = ? ORDER BY created_at DESC LIMIT 1`,
		actor.OrganizationID, conversation.ProjectID, conversationID))
	if err != nil {
		return SendMessageResult{}, false, err
	}
	if task.DiscardedAt != nil {
		return SendMessageResult{}, false, ErrInvalidState
	}
	request := struct {
		ConversationID string `json:"conversation_id"`
		Content        string `json:"content"`
	}{conversationID, strings.TrimSpace(content)}
	hash, _ := contract.CanonicalJSONHash(request)
	var prior SendMessageResult
	found, err := s.loadReceipt(ctx, actor, conversation.ProjectID, "message.create", key, hash, &prior)
	if found || err != nil {
		return prior, found, err
	}
	if err := s.ensureTextProviderReady(ctx, actor.OrganizationID); err != nil {
		return SendMessageResult{}, false, err
	}
	messageID, err := s.newID("msg")
	if err != nil {
		return SendMessageResult{}, false, err
	}
	agentTaskID, err := s.newID("agenttask")
	if err != nil {
		return SendMessageResult{}, false, err
	}
	eventID, err := s.newID("stratevent")
	if err != nil {
		return SendMessageResult{}, false, err
	}
	now := s.now()
	message := Message{ID: messageID, OrganizationID: actor.OrganizationID, ProjectID: conversation.ProjectID, ConversationID: conversationID, Role: "user", ContentType: "text", Content: request.Content, CreatedBy: actor.Principal.ID, CreatedAt: now}
	input, _ := snapshotJSON(map[string]string{
		"strategy_task_id": task.ID,
		"message_id":       message.ID,
		"prompt_version":   s.conversationPromptVersion(),
	})
	agentTask := agent.Task{ID: agentTaskID, OrganizationID: actor.OrganizationID, ProjectID: conversation.ProjectID, SourceSystem: "strategy", SourceType: "strategy_task", SourceID: task.ID, Kind: AgentKindBriefExtract, Status: agent.TaskDispatchPending, Version: 1, InputSnapshot: input, CreatedBy: actor.Principal, CreatedAt: now, UpdatedAt: now}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return SendMessageResult{}, false, err
	}
	defer tx.Rollback()
	if err := insertMessage(ctx, tx, message); err != nil {
		return SendMessageResult{}, false, err
	}
	eventPayload, _ := snapshotJSON(map[string]any{"message_id": message.ID, "role": message.Role})
	if err := insertConversationEvent(ctx, tx, eventID, actor.OrganizationID, conversation.ProjectID, conversationID, "message.created", eventPayload, now); err != nil {
		return SendMessageResult{}, false, err
	}
	writer, err := s.agentWriter()
	if err != nil {
		return SendMessageResult{}, false, err
	}
	if err := writer.CreateIn(ctx, tx, agent.CreateRequest{Task: agentTask}); err != nil {
		return SendMessageResult{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE strategy_tasks SET current_agent_task_id = ?, status = 'active',
		version = version + 1, updated_at = ? WHERE organization_id = ? AND project_id = ? AND id = ?`,
		agentTask.ID, now, actor.OrganizationID, conversation.ProjectID, task.ID); err != nil {
		return SendMessageResult{}, false, err
	}
	result := SendMessageResult{Message: message, AgentTask: agentTask}
	if err := insertReceipt(ctx, tx, actor, conversation.ProjectID, "message.create", key, hash, 202, result, now); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(ctx, actor, conversation.ProjectID, "message.create", key, hash, &prior)
			return prior, found, readErr
		}
		return SendMessageResult{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return SendMessageResult{}, false, err
	}
	return result, false, nil
}

func (s Service) ensureConcurrencyLimit(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, limit int) error {
	var running int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM platform_agent_tasks
		WHERE organization_id = ? AND project_id = ? AND status IN ('dispatch_pending', 'queued', 'running')`,
		organizationID, projectID).Scan(&running); err != nil {
		return err
	}
	if running >= limit {
		return ErrConcurrencyLimit
	}
	return nil
}

func (s Service) GetTaskBriefDraft(ctx context.Context, actor contract.ActorContext, taskID string) (BriefDraft, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return BriefDraft{}, err
	}
	task, err := scanTask(s.DB.QueryRowContext(ctx, taskSelect+` WHERE organization_id = ? AND id = ?`, actor.OrganizationID, taskID))
	if err != nil {
		return BriefDraft{}, err
	}
	if _, err := s.project(ctx, actor, task.ProjectID); err != nil {
		return BriefDraft{}, err
	}
	return scanBriefDraft(s.DB.QueryRowContext(ctx, briefDraftSelect+` WHERE organization_id = ? AND project_id = ? AND brief_id = ?
		ORDER BY created_at DESC LIMIT 1`, actor.OrganizationID, task.ProjectID, task.BriefID))
}

func (s Service) GetTask(ctx context.Context, actor contract.ActorContext, taskID string) (Task, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return Task{}, err
	}
	task, err := scanTask(s.DB.QueryRowContext(ctx, taskSelect+` WHERE organization_id = ? AND id = ?`, actor.OrganizationID, taskID))
	if err != nil {
		return Task{}, err
	}
	if _, err := s.project(ctx, actor, task.ProjectID); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s Service) ListConversationEvents(ctx context.Context, actor contract.ActorContext, conversationID string, after int64, limit int) ([]ConversationEvent, error) {
	conversation, err := s.GetConversation(ctx, actor, conversationID)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT sequence, event_id, conversation_id,
		event_type, payload, created_at FROM strategy_conversation_events
		WHERE organization_id = ? AND project_id = ? AND conversation_id = ? AND sequence > ?
		ORDER BY sequence LIMIT ?`, actor.OrganizationID, conversation.ProjectID, conversationID,
		after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ConversationEvent
	for rows.Next() {
		var value ConversationEvent
		if err := rows.Scan(&value.Sequence, &value.ID, &value.ConversationID, &value.Type,
			&value.Payload, &value.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	if result == nil {
		result = []ConversationEvent{}
	}
	return result, rows.Err()
}

func (s Service) ValidateConversationCursor(ctx context.Context, actor contract.ActorContext, conversationID string, cursor int64) error {
	conversation, err := s.GetConversation(ctx, actor, conversationID)
	if err != nil || cursor == 0 {
		return err
	}
	var exists int
	err = s.DB.QueryRowContext(ctx, `SELECT 1 FROM strategy_conversation_events
		WHERE organization_id = ? AND project_id = ? AND conversation_id = ? AND sequence = ?`,
		actor.OrganizationID, conversation.ProjectID, conversationID, cursor).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrEventCursorExpired
	}
	return err
}

func (s Service) PatchBriefDraft(ctx context.Context, actor contract.ActorContext, key contract.IdempotencyKey, taskID string, patch BriefPatch) (BriefDraft, bool, error) {
	if err := requireScope(actor, ScopeWrite); err != nil {
		return BriefDraft{}, false, err
	}
	if err := key.Validate(); err != nil {
		return BriefDraft{}, false, ErrInvalidRequest
	}
	task, err := scanTask(s.DB.QueryRowContext(ctx, taskSelect+` WHERE organization_id = ? AND id = ?`, actor.OrganizationID, taskID))
	if err != nil {
		return BriefDraft{}, false, err
	}
	if task.DiscardedAt != nil {
		return BriefDraft{}, false, ErrInvalidState
	}
	if _, err := s.project(ctx, actor, task.ProjectID); err != nil {
		return BriefDraft{}, false, err
	}
	hash, _ := contract.CanonicalJSONHash(patch)
	var prior BriefDraft
	found, err := s.loadReceipt(ctx, actor, task.ProjectID, "brief.patch", key, hash, &prior)
	if found || err != nil {
		return prior, found, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return BriefDraft{}, false, err
	}
	defer tx.Rollback()
	draft, err := scanBriefDraft(tx.QueryRowContext(ctx, briefDraftSelect+` WHERE organization_id = ? AND project_id = ? AND brief_id = ?
		ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, actor.OrganizationID, task.ProjectID, task.BriefID))
	if err != nil {
		return BriefDraft{}, false, err
	}
	updated, err := ApplyBriefPatch(draft, patch, PatchFromUser, actor.Principal.ID, s.now())
	if err != nil {
		return BriefDraft{}, false, err
	}
	if err := updateBriefDraft(ctx, tx, draft.Version, updated); err != nil {
		return BriefDraft{}, false, err
	}
	revisionID, _ := s.newID("briefrev")
	patchJSON, _ := snapshotJSON(patch)
	snapshotHash, _ := contract.NewContentHash(updated.Document)
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_brief_revisions
		(id, organization_id, project_id, draft_id, draft_version, patch, snapshot_hash, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, revisionID, actor.OrganizationID, task.ProjectID,
		updated.ID, updated.Version, patchJSON, snapshotHash, actor.Principal.ID, updated.UpdatedAt); err != nil {
		return BriefDraft{}, false, err
	}
	if err := syncEvidenceReferences(
		ctx, tx, actor.OrganizationID, task.ProjectID, "brief_draft", updated.ID,
		updated.Version, "reference_ids", updated.Document.ReferenceIDs,
		actor.Principal.ID, updated.UpdatedAt, true,
	); err != nil {
		return BriefDraft{}, false, err
	}
	if err := insertReceipt(ctx, tx, actor, task.ProjectID, "brief.patch", key, hash, 200, updated, updated.UpdatedAt); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(ctx, actor, task.ProjectID, "brief.patch", key, hash, &prior)
			return prior, found, readErr
		}
		return BriefDraft{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return BriefDraft{}, false, err
	}
	return updated, false, nil
}

func (s Service) ConfirmBrief(ctx context.Context, actor contract.ActorContext, key contract.IdempotencyKey, taskID string, expectedVersion int64) (BriefVersion, bool, error) {
	if err := requireScope(actor, ScopeConfirm); err != nil {
		return BriefVersion{}, false, err
	}
	if err := key.Validate(); err != nil || expectedVersion < 1 {
		return BriefVersion{}, false, ErrInvalidRequest
	}
	task, err := scanTask(s.DB.QueryRowContext(ctx, taskSelect+` WHERE organization_id = ? AND id = ?`, actor.OrganizationID, taskID))
	if err != nil {
		return BriefVersion{}, false, err
	}
	if task.DiscardedAt != nil {
		return BriefVersion{}, false, ErrInvalidState
	}
	if _, err := s.project(ctx, actor, task.ProjectID); err != nil {
		return BriefVersion{}, false, err
	}
	request := struct {
		TaskID          string `json:"task_id"`
		ExpectedVersion int64  `json:"expected_version"`
	}{taskID, expectedVersion}
	hash, _ := contract.CanonicalJSONHash(request)
	var prior BriefVersion
	found, err := s.loadReceipt(ctx, actor, task.ProjectID, "brief.confirm", key, hash, &prior)
	if found || err != nil {
		return prior, found, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return BriefVersion{}, false, err
	}
	defer tx.Rollback()
	draft, err := scanBriefDraft(tx.QueryRowContext(ctx, briefDraftSelect+` WHERE organization_id = ? AND project_id = ? AND brief_id = ?
		ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, actor.OrganizationID, task.ProjectID, task.BriefID))
	if err != nil {
		return BriefVersion{}, false, err
	}
	if draft.Version != expectedVersion {
		return BriefVersion{}, false, ErrVersionConflict
	}
	if draft.Status != "open" {
		return BriefVersion{}, false, ErrInvalidState
	}
	completeness := ComputeCompleteness(draft.Document, draft.FieldStates)
	if !completeness.Ready {
		return BriefVersion{}, false, BlockedError{Problems: completeness.Blockers}
	}
	var latest int64
	if err := tx.QueryRowContext(ctx, `SELECT latest_version FROM strategy_briefs
		WHERE organization_id = ? AND project_id = ? AND id = ? FOR UPDATE`,
		actor.OrganizationID, task.ProjectID, task.BriefID).Scan(&latest); err != nil {
		return BriefVersion{}, false, err
	}
	now := s.now()
	version := BriefVersion{
		BriefID: task.BriefID, Version: latest + 1, OrganizationID: actor.OrganizationID,
		ProjectID: task.ProjectID, Snapshot: draft.Document, FieldStates: draft.FieldStates,
		SourceDraftID: draft.ID, SourceDraftVersion: draft.Version,
		ConfirmedBy: actor.Principal.ID, ConfirmedAt: now,
	}
	version.ContentHash, err = contract.NewContentHash(struct {
		Snapshot    BriefDocument         `json:"snapshot"`
		FieldStates map[string]FieldState `json:"field_states"`
	}{version.Snapshot, version.FieldStates})
	if err != nil {
		return BriefVersion{}, false, err
	}
	snapshot, _ := snapshotJSON(struct {
		Document    BriefDocument         `json:"document"`
		FieldStates map[string]FieldState `json:"field_states"`
	}{version.Snapshot, version.FieldStates})
	if _, err := tx.ExecContext(ctx, `INSERT INTO strategy_brief_versions
		(brief_id, version, organization_id, project_id, snapshot, content_hash, source_draft_id,
		 source_draft_version, confirmed_by, confirmed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		version.BriefID, version.Version, version.OrganizationID, version.ProjectID, snapshot,
		version.ContentHash, version.SourceDraftID, version.SourceDraftVersion, version.ConfirmedBy, version.ConfirmedAt); err != nil {
		return BriefVersion{}, false, err
	}
	if err := syncEvidenceReferences(
		ctx, tx, actor.OrganizationID, task.ProjectID, "brief_version", version.BriefID,
		version.Version, "reference_ids", version.Snapshot.ReferenceIDs,
		actor.Principal.ID, now, false,
	); err != nil {
		return BriefVersion{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE strategy_brief_drafts SET status = 'confirmed',
		completeness = ?, updated_by = ?, updated_at = ? WHERE organization_id = ? AND project_id = ?
		AND id = ? AND version = ?`, mustJSON(completeness), actor.Principal.ID, now,
		actor.OrganizationID, task.ProjectID, draft.ID, draft.Version); err != nil {
		return BriefVersion{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE strategy_briefs SET latest_version = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?`, version.Version, now,
		actor.OrganizationID, task.ProjectID, version.BriefID); err != nil {
		return BriefVersion{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE strategy_tasks SET status = 'active',
		version = version + 1, updated_at = ? WHERE organization_id = ? AND project_id = ? AND id = ?`,
		now, actor.OrganizationID, task.ProjectID, task.ID); err != nil {
		return BriefVersion{}, false, err
	}
	eventID, _ := s.newID("stratevent")
	payload, _ := snapshotJSON(map[string]any{"brief_id": version.BriefID, "version": version.Version, "content_hash": version.ContentHash})
	if err := insertConversationEvent(ctx, tx, eventID, actor.OrganizationID, task.ProjectID, task.ConversationID, "brief.confirmed", payload, now); err != nil {
		return BriefVersion{}, false, err
	}
	if err := insertReceipt(ctx, tx, actor, task.ProjectID, "brief.confirm", key, hash, 201, version, now); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(ctx, actor, task.ProjectID, "brief.confirm", key, hash, &prior)
			return prior, found, readErr
		}
		return BriefVersion{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return BriefVersion{}, false, err
	}
	return version, false, nil
}

func (s Service) ListBriefVersions(ctx context.Context, actor contract.ActorContext, briefID string) ([]BriefVersion, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, briefVersionSelect+` WHERE organization_id = ? AND brief_id = ? ORDER BY version DESC`, actor.OrganizationID, briefID)
	if err != nil {
		return nil, err
	}
	var versions []BriefVersion
	for rows.Next() {
		version, err := scanBriefVersion(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	authorized := make(map[contract.ProjectID]struct{})
	for _, version := range versions {
		if _, ok := authorized[version.ProjectID]; ok {
			continue
		}
		if _, err := s.project(ctx, actor, version.ProjectID); err != nil {
			return nil, err
		}
		authorized[version.ProjectID] = struct{}{}
	}
	if versions == nil {
		versions = []BriefVersion{}
	}
	return versions, nil
}

// ListProjectBriefVersions returns the immutable, confirmed Brief versions that
// may be selected by downstream Creative work. It deliberately excludes
// mutable drafts and verifies project authorization before touching rows.
func (s Service) ListProjectBriefVersions(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) ([]BriefVersion, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return nil, err
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, briefVersionSelect+`
		WHERE organization_id = ? AND project_id = ?
		ORDER BY confirmed_at DESC, brief_id ASC, version DESC`,
		actor.OrganizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	versions := make([]BriefVersion, 0)
	for rows.Next() {
		version, scanErr := scanBriefVersion(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return versions, nil
}

func (s Service) GetBriefVersion(ctx context.Context, actor contract.ActorContext, briefID string, version int64) (BriefVersion, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return BriefVersion{}, err
	}
	value, err := scanBriefVersion(s.DB.QueryRowContext(ctx, briefVersionSelect+` WHERE organization_id = ? AND brief_id = ? AND version = ?`, actor.OrganizationID, briefID, version))
	if err != nil {
		return BriefVersion{}, err
	}
	if _, err := s.project(ctx, actor, value.ProjectID); err != nil {
		return BriefVersion{}, err
	}
	return value, nil
}

func (s Service) loadReceipt(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, operation string, key contract.IdempotencyKey, requestHash string, target any) (bool, error) {
	var storedHash string
	var snapshot json.RawMessage
	err := s.DB.QueryRowContext(ctx, `SELECT request_hash, response_snapshot FROM strategy_write_receipts
		WHERE organization_id = ? AND project_id = ? AND principal_kind = ? AND principal_id = ?
		AND operation_name = ? AND idempotency_key = ?`,
		actor.OrganizationID, projectID, actor.Principal.Kind, actor.Principal.ID, operation, key).Scan(&storedHash, &snapshot)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if storedHash != requestHash {
		return false, ErrIdempotencyConflict
	}
	if err := json.Unmarshal(snapshot, target); err != nil {
		return false, err
	}
	return true, nil
}

func insertReceipt(ctx context.Context, executor agent.DBTX, actor contract.ActorContext, projectID contract.ProjectID, operation string, key contract.IdempotencyKey, requestHash string, status int, response any, now time.Time) error {
	snapshot, err := snapshotJSON(response)
	if err != nil {
		return err
	}
	_, err = executor.ExecContext(ctx, `INSERT INTO strategy_write_receipts
		(organization_id, project_id, principal_kind, principal_id, operation_name, idempotency_key,
		 request_hash, response_status, response_snapshot, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, actor.OrganizationID, projectID, actor.Principal.Kind,
		actor.Principal.ID, operation, key, requestHash, status, snapshot, now.UTC())
	return err
}

func isDuplicate(err error) bool {
	var mysqlError *mysqlDriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}

func mustJSON(value any) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}
