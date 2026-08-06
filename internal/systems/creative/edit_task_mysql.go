package creative

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func (r MySQLRepository) CreateEditTask(ctx context.Context, task EditTask, timeline TimelineVersion) (EditTask, error) {
	if r.DB == nil {
		return EditTask{}, fmt.Errorf("creative repository database is required")
	}
	if err := task.Validate(); err != nil {
		return EditTask{}, err
	}
	if task.CurrentTimeline.Version != timeline.Version || task.CurrentTimeline.ContentHash != timeline.ContentHash {
		return EditTask{}, fmt.Errorf("edit task current timeline must match the initial timeline version")
	}
	payload, err := json.Marshal(timeline.Timeline)
	if err != nil {
		return EditTask{}, err
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return EditTask{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO creative_edit_tasks
		(organization_id, project_id, edit_task_id, display_name, status, entry_source, source_creative_task_id,
		 current_timeline_version, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?)`,
		task.OrganizationID, task.ProjectID, task.ID, task.DisplayName, task.Status, task.EntrySource, task.SourceCreativeTaskID,
		timeline.Version, task.CreatedBy, task.CreatedAt, task.UpdatedAt)
	if err != nil {
		return EditTask{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO creative_edit_timeline_versions
		(organization_id, project_id, edit_task_id, version, content_payload, content_hash, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, task.OrganizationID, task.ProjectID, task.ID, timeline.Version, payload,
		timeline.ContentHash, timeline.CreatedBy, timeline.CreatedAt)
	if err != nil {
		return EditTask{}, err
	}
	if err := tx.Commit(); err != nil {
		return EditTask{}, err
	}
	return task, nil
}

func (r MySQLRepository) GetEditTask(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, taskID string) (EditTask, error) {
	if r.DB == nil {
		return EditTask{}, fmt.Errorf("creative repository database is required")
	}
	return scanEditTask(r.DB.QueryRowContext(ctx, `SELECT
		t.edit_task_id, t.organization_id, t.project_id, t.display_name, t.status, t.entry_source,
		COALESCE(t.source_creative_task_id, ''), t.created_by, t.created_at, t.updated_at,
		v.version, v.content_payload, v.content_hash, v.created_by, v.created_at
		FROM creative_edit_tasks t
		JOIN creative_edit_timeline_versions v
		  ON v.organization_id=t.organization_id AND v.project_id=t.project_id
		 AND v.edit_task_id=t.edit_task_id AND v.version=t.current_timeline_version
		WHERE t.organization_id=? AND t.project_id=? AND t.edit_task_id=?`, organizationID, projectID, taskID))
}

func (r MySQLRepository) FindEditTaskBySource(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, source EditTaskEntrySource, sourceCreativeTaskID string) (EditTask, error) {
	if r.DB == nil {
		return EditTask{}, fmt.Errorf("creative repository database is required")
	}
	var id string
	err := r.DB.QueryRowContext(ctx, `SELECT edit_task_id FROM creative_edit_tasks
		WHERE organization_id=? AND project_id=? AND entry_source=? AND source_creative_task_id=?
		ORDER BY updated_at DESC, edit_task_id DESC LIMIT 1`, organizationID, projectID, source, sourceCreativeTaskID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return EditTask{}, ErrNotFound
	}
	if err != nil {
		return EditTask{}, err
	}
	return r.GetEditTask(ctx, organizationID, projectID, id)
}

func (r MySQLRepository) AppendEditTimeline(ctx context.Context, task EditTask, expectedVersion int64, timeline TimelineVersion) (EditTask, error) {
	if r.DB == nil {
		return EditTask{}, fmt.Errorf("creative repository database is required")
	}
	if err := timeline.Validate(); err != nil {
		return EditTask{}, err
	}
	if timeline.Version != expectedVersion+1 {
		return EditTask{}, ErrVersionConflict
	}
	payload, err := json.Marshal(timeline.Timeline)
	if err != nil {
		return EditTask{}, err
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return EditTask{}, err
	}
	defer tx.Rollback()
	var current int64
	err = tx.QueryRowContext(ctx, `SELECT current_timeline_version FROM creative_edit_tasks
		WHERE organization_id=? AND project_id=? AND edit_task_id=? FOR UPDATE`, task.OrganizationID, task.ProjectID, task.ID).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return EditTask{}, ErrNotFound
	}
	if err != nil {
		return EditTask{}, err
	}
	if current != expectedVersion {
		return EditTask{}, ErrVersionConflict
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO creative_edit_timeline_versions
		(organization_id, project_id, edit_task_id, version, content_payload, content_hash, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, task.OrganizationID, task.ProjectID, task.ID, timeline.Version, payload,
		timeline.ContentHash, timeline.CreatedBy, timeline.CreatedAt)
	if err != nil {
		return EditTask{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE creative_edit_tasks SET current_timeline_version=?, updated_at=?
		WHERE organization_id=? AND project_id=? AND edit_task_id=? AND current_timeline_version=?`, timeline.Version, timeline.CreatedAt,
		task.OrganizationID, task.ProjectID, task.ID, expectedVersion)
	if err != nil {
		return EditTask{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return EditTask{}, err
	}
	if affected != 1 {
		return EditTask{}, ErrVersionConflict
	}
	if err := tx.Commit(); err != nil {
		return EditTask{}, err
	}
	return r.GetEditTask(ctx, task.OrganizationID, task.ProjectID, task.ID)
}

func scanEditTask(row *sql.Row) (EditTask, error) {
	var task EditTask
	var timeline TimelineVersion
	var payload []byte
	err := row.Scan(&task.ID, &task.OrganizationID, &task.ProjectID, &task.DisplayName, &task.Status, &task.EntrySource,
		&task.SourceCreativeTaskID, &task.CreatedBy, &task.CreatedAt, &task.UpdatedAt,
		&timeline.Version, &payload, &timeline.ContentHash, &timeline.CreatedBy, &timeline.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return EditTask{}, ErrNotFound
	}
	if err != nil {
		return EditTask{}, err
	}
	if err := json.Unmarshal(payload, &timeline.Timeline); err != nil {
		return EditTask{}, err
	}
	task.ContractVersion, task.CurrentTimeline = EditTaskContractVersion, timeline
	if err := task.Validate(); err != nil {
		return EditTask{}, err
	}
	return task, nil
}
