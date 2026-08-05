package strategy

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func (s Service) DiscardTask(
	ctx context.Context,
	actor contract.ActorContext,
	key contract.IdempotencyKey,
	taskID string,
	request LifecycleRequest,
) (Task, bool, error) {
	if err := requireScope(actor, ScopeWrite); err != nil {
		return Task{}, false, err
	}
	request.Reason = strings.TrimSpace(request.Reason)
	if err := key.Validate(); err != nil || taskID == "" || request.ExpectedVersion < 1 ||
		request.Reason == "" || len(request.Reason) > 500 {
		return Task{}, false, ErrInvalidRequest
	}
	task, err := s.GetTask(ctx, actor, taskID)
	if err != nil {
		return Task{}, false, err
	}
	hash, _ := contract.CanonicalJSONHash(request)
	var prior Task
	found, err := s.loadReceipt(ctx, actor, task.ProjectID, "strategy.task.discard", key, hash, &prior)
	if found || err != nil {
		return prior, found, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, false, err
	}
	defer tx.Rollback()
	locked, err := scanTask(tx.QueryRowContext(ctx, taskSelect+`
		WHERE organization_id = ? AND project_id = ? AND id = ? FOR UPDATE`,
		actor.OrganizationID, task.ProjectID, taskID))
	if err != nil {
		return Task{}, false, err
	}
	if locked.Version != request.ExpectedVersion {
		return Task{}, false, ErrVersionConflict
	}
	if locked.DiscardedAt != nil || locked.Status == "completed" {
		return Task{}, false, ErrInvalidState
	}
	if locked.CurrentStrategyID != "" {
		draft, draftErr := scanDraft(tx.QueryRowContext(ctx, draftSelect+`
			WHERE organization_id = ? AND project_id = ? AND id = ?`,
			actor.OrganizationID, task.ProjectID, locked.CurrentStrategyID))
		if draftErr != nil && !errors.Is(draftErr, ErrNotFound) {
			return Task{}, false, draftErr
		}
		if draftErr == nil && draft.CurrentRevision > 0 {
			return Task{}, false, ErrInvalidState
		}
	}
	if locked.CurrentAgentTaskID != "" {
		var active int
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM platform_agent_tasks
			WHERE organization_id = ? AND project_id = ? AND id = ?
			AND status IN ('dispatch_pending', 'queued', 'running')`,
			actor.OrganizationID, task.ProjectID, locked.CurrentAgentTaskID).Scan(&active)
		if err != nil {
			return Task{}, false, err
		}
		if active > 0 {
			return Task{}, false, ErrInvalidState
		}
	}
	now := s.now()
	result, err := tx.ExecContext(ctx, `UPDATE strategy_tasks SET discarded_at = ?,
		discarded_by = ?, discard_reason = ?, version = version + 1, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ?
		AND discarded_at IS NULL`, now, actor.Principal.ID, request.Reason, now,
		actor.OrganizationID, task.ProjectID, taskID, locked.Version)
	if err != nil {
		return Task{}, false, err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return Task{}, false, ErrVersionConflict
	}
	locked.DiscardedAt, locked.DiscardedBy, locked.DiscardReason = &now, actor.Principal.ID, request.Reason
	locked.Version++
	locked.UpdatedAt = now
	if err := s.insertLifecycleEvent(ctx, tx, locked, "strategy.task.discarded", request.Reason, now); err != nil {
		return Task{}, false, err
	}
	if err := insertReceipt(ctx, tx, actor, task.ProjectID, "strategy.task.discard", key, hash, 200, locked, now); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(ctx, actor, task.ProjectID, "strategy.task.discard", key, hash, &prior)
			return prior, found, readErr
		}
		return Task{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Task{}, false, err
	}
	return locked, false, nil
}

func (s Service) RestoreTask(
	ctx context.Context,
	actor contract.ActorContext,
	key contract.IdempotencyKey,
	taskID string,
	request LifecycleRequest,
) (Task, bool, error) {
	if err := requireScope(actor, ScopeWrite); err != nil {
		return Task{}, false, err
	}
	if err := key.Validate(); err != nil || taskID == "" || request.ExpectedVersion < 1 {
		return Task{}, false, ErrInvalidRequest
	}
	task, err := s.GetTask(ctx, actor, taskID)
	if err != nil {
		return Task{}, false, err
	}
	hash, _ := contract.CanonicalJSONHash(request)
	var prior Task
	found, err := s.loadReceipt(ctx, actor, task.ProjectID, "strategy.task.restore", key, hash, &prior)
	if found || err != nil {
		return prior, found, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, false, err
	}
	defer tx.Rollback()
	locked, err := scanTask(tx.QueryRowContext(ctx, taskSelect+`
		WHERE organization_id = ? AND project_id = ? AND id = ? FOR UPDATE`,
		actor.OrganizationID, task.ProjectID, taskID))
	if err != nil {
		return Task{}, false, err
	}
	if locked.Version != request.ExpectedVersion {
		return Task{}, false, ErrVersionConflict
	}
	if locked.DiscardedAt == nil {
		return Task{}, false, ErrInvalidState
	}
	now := s.now()
	result, err := tx.ExecContext(ctx, `UPDATE strategy_tasks SET discarded_at = NULL,
		discarded_by = NULL, discard_reason = NULL, version = version + 1, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ?
		AND discarded_at IS NOT NULL`, now, actor.OrganizationID, task.ProjectID, taskID, locked.Version)
	if err != nil {
		return Task{}, false, err
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return Task{}, false, ErrVersionConflict
	}
	locked.DiscardedAt, locked.DiscardedBy, locked.DiscardReason = nil, "", ""
	locked.Version++
	locked.UpdatedAt = now
	if err := s.insertLifecycleEvent(ctx, tx, locked, "strategy.task.restored", "", now); err != nil {
		return Task{}, false, err
	}
	if err := insertReceipt(ctx, tx, actor, task.ProjectID, "strategy.task.restore", key, hash, 200, locked, now); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(ctx, actor, task.ProjectID, "strategy.task.restore", key, hash, &prior)
			return prior, found, readErr
		}
		return Task{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Task{}, false, err
	}
	return locked, false, nil
}

func (s Service) ArchiveStrategy(
	ctx context.Context,
	actor contract.ActorContext,
	key contract.IdempotencyKey,
	strategyID string,
	request LifecycleRequest,
) (Draft, bool, error) {
	return s.setStrategyArchived(ctx, actor, key, strategyID, request, true)
}

func (s Service) RestoreStrategy(
	ctx context.Context,
	actor contract.ActorContext,
	key contract.IdempotencyKey,
	strategyID string,
	request LifecycleRequest,
) (Draft, bool, error) {
	return s.setStrategyArchived(ctx, actor, key, strategyID, request, false)
}

func (s Service) setStrategyArchived(
	ctx context.Context,
	actor contract.ActorContext,
	key contract.IdempotencyKey,
	strategyID string,
	request LifecycleRequest,
	archive bool,
) (Draft, bool, error) {
	if err := requireScope(actor, ScopeWrite); err != nil {
		return Draft{}, false, err
	}
	request.Reason = strings.TrimSpace(request.Reason)
	if err := key.Validate(); err != nil || strategyID == "" || request.ExpectedVersion < 1 ||
		(archive && (request.Reason == "" || len(request.Reason) > 500)) {
		return Draft{}, false, ErrInvalidRequest
	}
	draft, err := s.GetDraft(ctx, actor, strategyID)
	if err != nil {
		return Draft{}, false, err
	}
	operation := "strategy.archive"
	if !archive {
		operation = "strategy.restore"
	}
	hash, _ := contract.CanonicalJSONHash(request)
	var prior Draft
	found, err := s.loadReceipt(ctx, actor, draft.ProjectID, operation, key, hash, &prior)
	if found || err != nil {
		return prior, found, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Draft{}, false, err
	}
	defer tx.Rollback()
	locked, err := scanDraft(tx.QueryRowContext(ctx, draftSelect+`
		WHERE organization_id = ? AND project_id = ? AND id = ? FOR UPDATE`,
		actor.OrganizationID, draft.ProjectID, strategyID))
	if err != nil {
		return Draft{}, false, err
	}
	if locked.Version != request.ExpectedVersion {
		return Draft{}, false, ErrVersionConflict
	}
	task, err := scanTask(tx.QueryRowContext(ctx, taskSelect+`
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		actor.OrganizationID, draft.ProjectID, locked.TaskID))
	if err != nil {
		return Draft{}, false, err
	}
	now := s.now()
	if archive {
		if locked.ArchivedAt != nil || locked.CurrentRevision < 1 || locked.Status == "generating" ||
			task.DiscardedAt != nil {
			return Draft{}, false, ErrInvalidState
		}
		if locked.CurrentReviewID != "" {
			var reviewStatus string
			err = tx.QueryRowContext(ctx, `SELECT status FROM strategy_reviews
				WHERE organization_id = ? AND project_id = ? AND id = ?`,
				actor.OrganizationID, draft.ProjectID, locked.CurrentReviewID).Scan(&reviewStatus)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return Draft{}, false, err
			}
			if reviewStatus == "open" {
				return Draft{}, false, ErrInvalidState
			}
		}
		result, updateErr := tx.ExecContext(ctx, `UPDATE strategy_drafts SET archived_at = ?,
			archived_by = ?, archive_reason = ?, version = version + 1, updated_at = ?
			WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ?
			AND archived_at IS NULL`, now, actor.Principal.ID, request.Reason, now,
			actor.OrganizationID, draft.ProjectID, strategyID, locked.Version)
		if updateErr != nil {
			return Draft{}, false, updateErr
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return Draft{}, false, ErrVersionConflict
		}
		locked.ArchivedAt, locked.ArchivedBy, locked.ArchiveReason = &now, actor.Principal.ID, request.Reason
	} else {
		if locked.ArchivedAt == nil || task.DiscardedAt != nil {
			return Draft{}, false, ErrInvalidState
		}
		result, updateErr := tx.ExecContext(ctx, `UPDATE strategy_drafts SET archived_at = NULL,
			archived_by = NULL, archive_reason = NULL, version = version + 1, updated_at = ?
			WHERE organization_id = ? AND project_id = ? AND id = ? AND version = ?
			AND archived_at IS NOT NULL`, now, actor.OrganizationID, draft.ProjectID, strategyID, locked.Version)
		if updateErr != nil {
			return Draft{}, false, updateErr
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return Draft{}, false, ErrVersionConflict
		}
		locked.ArchivedAt, locked.ArchivedBy, locked.ArchiveReason = nil, "", ""
	}
	locked.Version++
	locked.UpdatedAt = now
	eventType := "strategy.draft.restored"
	if archive {
		eventType = "strategy.draft.archived"
	}
	if err := s.insertLifecycleEvent(ctx, tx, task, eventType, request.Reason, now); err != nil {
		return Draft{}, false, err
	}
	if err := insertReceipt(ctx, tx, actor, draft.ProjectID, operation, key, hash, 200, locked, now); err != nil {
		if isDuplicate(err) {
			tx.Rollback()
			found, readErr := s.loadReceipt(ctx, actor, draft.ProjectID, operation, key, hash, &prior)
			return prior, found, readErr
		}
		return Draft{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Draft{}, false, err
	}
	return locked, false, nil
}

func (s Service) insertLifecycleEvent(
	ctx context.Context,
	tx *sql.Tx,
	task Task,
	eventType string,
	reason string,
	now time.Time,
) error {
	eventID, err := s.newID("stratevent")
	if err != nil {
		return err
	}
	return insertConversationEvent(ctx, tx, eventID, task.OrganizationID, task.ProjectID,
		task.ConversationID, eventType, mustJSON(map[string]any{
			"task_id": task.ID, "strategy_id": task.CurrentStrategyID, "reason": reason,
		}), now)
}
