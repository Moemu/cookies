package strategy

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type rowScanner interface{ Scan(...any) error }

const workspaceSelect = `SELECT id, organization_id, project_id, name, is_primary, status,
	version, created_by, created_at, updated_at FROM strategy_workspaces`

func scanWorkspace(row rowScanner) (Workspace, error) {
	var value Workspace
	if err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.Name,
		&value.IsPrimary, &value.Status, &value.Version, &value.CreatedBy, &value.CreatedAt,
		&value.UpdatedAt); err != nil {
		return Workspace{}, mapNotFound(err)
	}
	return value, nil
}

const conversationSelect = `SELECT id, organization_id, project_id, workspace_id, status,
	version, created_by, created_at, updated_at FROM strategy_conversations`

func scanConversation(row rowScanner) (Conversation, error) {
	var value Conversation
	if err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.WorkspaceID,
		&value.Status, &value.Version, &value.CreatedBy, &value.CreatedAt, &value.UpdatedAt); err != nil {
		return Conversation{}, mapNotFound(err)
	}
	return value, nil
}

const taskSelect = `SELECT id, organization_id, project_id, workspace_id, conversation_id,
	brief_id, COALESCE(current_agent_task_id, ''), COALESCE(current_strategy_id, ''),
	status, discarded_at, COALESCE(discarded_by, ''), COALESCE(discard_reason, ''),
	version, created_at, updated_at FROM strategy_tasks`

func scanTask(row rowScanner) (Task, error) {
	var value Task
	var discardedAt sql.NullTime
	if err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.WorkspaceID,
		&value.ConversationID, &value.BriefID, &value.CurrentAgentTaskID, &value.CurrentStrategyID,
		&value.Status, &discardedAt, &value.DiscardedBy, &value.DiscardReason,
		&value.Version, &value.CreatedAt, &value.UpdatedAt); err != nil {
		return Task{}, mapNotFound(err)
	}
	if discardedAt.Valid {
		value.DiscardedAt = &discardedAt.Time
	}
	return value, nil
}

const messageSelect = `SELECT id, organization_id, project_id, conversation_id, role,
	content_type, content, ai_generated, COALESCE(agent_task_id, ''), skill_run_ids,
	created_by, created_at FROM strategy_messages`

func scanMessage(row rowScanner) (Message, error) {
	var value Message
	var skillRuns []byte
	if err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.ConversationID,
		&value.Role, &value.ContentType, &value.Content, &value.AIGenerated, &value.AgentTaskID,
		&skillRuns, &value.CreatedBy, &value.CreatedAt); err != nil {
		return Message{}, mapNotFound(err)
	}
	if len(skillRuns) > 0 {
		_ = json.Unmarshal(skillRuns, &value.SkillRunIDs)
	}
	return value, nil
}

const briefDraftSelect = `SELECT id, organization_id, project_id, brief_id, status, version,
	base_brief_version, document, field_states, completeness, updated_by, created_at, updated_at
	FROM strategy_brief_drafts`

func scanBriefDraft(row rowScanner) (BriefDraft, error) {
	var value BriefDraft
	var base sql.NullInt64
	var document, states, completeness json.RawMessage
	if err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.BriefID,
		&value.Status, &value.Version, &base, &document, &states, &completeness,
		&value.UpdatedBy, &value.CreatedAt, &value.UpdatedAt); err != nil {
		return BriefDraft{}, mapNotFound(err)
	}
	if base.Valid {
		value.BaseBriefVersion = &base.Int64
	}
	if err := json.Unmarshal(document, &value.Document); err != nil {
		return BriefDraft{}, err
	}
	if err := json.Unmarshal(states, &value.FieldStates); err != nil {
		return BriefDraft{}, err
	}
	if err := json.Unmarshal(completeness, &value.Completeness); err != nil {
		return BriefDraft{}, err
	}
	return value, nil
}

const briefVersionSelect = `SELECT brief_id, version, organization_id, project_id, snapshot,
	content_hash, source_draft_id, source_draft_version, confirmed_by, confirmed_at
	FROM strategy_brief_versions`

func scanBriefVersion(row rowScanner) (BriefVersion, error) {
	var value BriefVersion
	var snapshot json.RawMessage
	if err := row.Scan(&value.BriefID, &value.Version, &value.OrganizationID, &value.ProjectID,
		&snapshot, &value.ContentHash, &value.SourceDraftID, &value.SourceDraftVersion,
		&value.ConfirmedBy, &value.ConfirmedAt); err != nil {
		return BriefVersion{}, mapNotFound(err)
	}
	var stored struct {
		Document    BriefDocument         `json:"document"`
		FieldStates map[string]FieldState `json:"field_states"`
	}
	if err := json.Unmarshal(snapshot, &stored); err != nil {
		return BriefVersion{}, err
	}
	value.Snapshot = stored.Document
	value.FieldStates = stored.FieldStates
	return value, nil
}

func insertMessage(ctx context.Context, executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, message Message) error {
	skillRuns := any(nil)
	if len(message.SkillRunIDs) > 0 {
		skillRuns = mustJSON(message.SkillRunIDs)
	}
	_, err := executor.ExecContext(ctx, `INSERT INTO strategy_messages
		(id, organization_id, project_id, conversation_id, role, content_type, content,
		 ai_generated, agent_task_id, skill_run_ids, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?)`, message.ID,
		message.OrganizationID, message.ProjectID, message.ConversationID, message.Role,
		message.ContentType, message.Content, message.AIGenerated, message.AgentTaskID,
		skillRuns, message.CreatedBy, message.CreatedAt)
	return err
}

func insertConversationEvent(ctx context.Context, executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, eventID string, organizationID contract.OrganizationID, projectID contract.ProjectID, conversationID, eventType string, payload json.RawMessage, createdAt interface{}) error {
	_, err := executor.ExecContext(ctx, `INSERT INTO strategy_conversation_events
		(event_id, organization_id, project_id, conversation_id, event_type, payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, eventID, organizationID, projectID, conversationID,
		eventType, payload, createdAt)
	return err
}

func updateBriefDraft(ctx context.Context, executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, expectedVersion int64, draft BriefDraft) error {
	document, err := snapshotJSON(draft.Document)
	if err != nil {
		return err
	}
	states, err := snapshotJSON(draft.FieldStates)
	if err != nil {
		return err
	}
	completeness, err := snapshotJSON(draft.Completeness)
	if err != nil {
		return err
	}
	result, err := executor.ExecContext(ctx, `UPDATE strategy_brief_drafts SET document = ?,
		field_states = ?, completeness = ?, version = ?, updated_by = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ? AND status = 'open'`,
		document, states, completeness, draft.Version, draft.UpdatedBy, draft.UpdatedAt,
		draft.OrganizationID, draft.ProjectID, draft.ID, expectedVersion)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return ErrVersionConflict
	}
	return nil
}

func mapNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

const draftSelect = `SELECT id, organization_id, project_id, task_id, brief_id, brief_version,
	project_context_version, status, archived_at, COALESCE(archived_by, ''),
	COALESCE(archive_reason, ''), current_revision, COALESCE(current_review_id, ''),
	version, skill_versions, created_at, updated_at FROM strategy_drafts`

func scanDraft(row rowScanner) (Draft, error) {
	var value Draft
	var skills json.RawMessage
	var archivedAt sql.NullTime
	if err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.TaskID,
		&value.BriefID, &value.BriefVersion, &value.ProjectContextVersion, &value.Status,
		&archivedAt, &value.ArchivedBy, &value.ArchiveReason,
		&value.CurrentRevision, &value.CurrentReviewID, &value.Version, &skills,
		&value.CreatedAt, &value.UpdatedAt); err != nil {
		return Draft{}, mapNotFound(err)
	}
	if archivedAt.Valid {
		value.ArchivedAt = &archivedAt.Time
	}
	if err := json.Unmarshal(skills, &value.SkillVersions); err != nil {
		return Draft{}, err
	}
	return value, nil
}

const draftRevisionSelect = `SELECT strategy_id, revision, base_revision, document,
	changed_sections, content_hash, created_by, created_at FROM strategy_draft_revisions`

func scanDraftRevision(row rowScanner) (DraftRevision, error) {
	var value DraftRevision
	var base sql.NullInt64
	var document, changed json.RawMessage
	if err := row.Scan(&value.StrategyID, &value.Revision, &base, &document, &changed,
		&value.ContentHash, &value.CreatedBy, &value.CreatedAt); err != nil {
		return DraftRevision{}, mapNotFound(err)
	}
	if base.Valid {
		value.BaseRevision = &base.Int64
	}
	if err := json.Unmarshal(document, &value.Document); err != nil {
		return DraftRevision{}, err
	}
	if err := json.Unmarshal(changed, &value.ChangedSections); err != nil {
		return DraftRevision{}, err
	}
	return value, nil
}

func insertDraftRevision(ctx context.Context, executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, organizationID contract.OrganizationID, projectID contract.ProjectID, revision DraftRevision) error {
	document, err := snapshotJSON(revision.Document)
	if err != nil {
		return err
	}
	changed, err := snapshotJSON(revision.ChangedSections)
	if err != nil {
		return err
	}
	lineage, err := snapshotJSON(revision.Document.Lineage)
	if err != nil {
		return err
	}
	_, err = executor.ExecContext(ctx, `INSERT INTO strategy_draft_revisions
		(strategy_id, revision, organization_id, project_id, base_revision, document,
		 changed_sections, content_hash, lineage, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, revision.StrategyID, revision.Revision,
		organizationID, projectID, revision.BaseRevision, document, changed,
		revision.ContentHash, lineage, revision.CreatedBy, revision.CreatedAt)
	return err
}

const reviewSelect = `SELECT id, organization_id, project_id, strategy_id, candidate_revision,
	candidate_content_hash, brief_id, brief_version, project_context_version, status,
	COALESCE(decision_reason, ''), COALESCE(decided_by, ''), decided_at, created_by,
	created_at, updated_at FROM strategy_reviews`

func scanReview(row rowScanner) (Review, error) {
	var value Review
	var decidedAt sql.NullTime
	if err := row.Scan(&value.ID, &value.OrganizationID, &value.ProjectID, &value.StrategyID,
		&value.CandidateRevision, &value.CandidateContentHash, &value.BriefID,
		&value.BriefVersion, &value.ProjectContextVersion, &value.Status,
		&value.DecisionReason, &value.DecidedBy, &decidedAt, &value.CreatedBy,
		&value.CreatedAt, &value.UpdatedAt); err != nil {
		return Review{}, mapNotFound(err)
	}
	if decidedAt.Valid {
		value.DecidedAt = &decidedAt.Time
	}
	return value, nil
}

const packageVersionSelect = `SELECT package_id, version, organization_id, project_id,
	snapshot, content_hash, status, published_by, published_at FROM strategy_package_versions`

func scanPackageVersion(row rowScanner) (PackageVersion, error) {
	var value PackageVersion
	var snapshot json.RawMessage
	if err := row.Scan(&value.PackageID, &value.Version, &value.OrganizationID, &value.ProjectID,
		&snapshot, &value.ContentHash, &value.Status, &value.PublishedBy,
		&value.PublishedAt); err != nil {
		return PackageVersion{}, mapNotFound(err)
	}
	if err := json.Unmarshal(snapshot, &value.Snapshot); err != nil {
		return PackageVersion{}, err
	}
	return value, nil
}
