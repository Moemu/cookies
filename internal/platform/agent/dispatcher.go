package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/ids"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
)

type Dispatcher struct {
	DB      *sql.DB
	Jobs    jobruntime.Store
	NewID   func(string) (string, error)
	Now     func() time.Time
	RetryIn time.Duration
}

func (d Dispatcher) RunOnce(ctx context.Context) (bool, error) {
	if d.DB == nil || d.Jobs == nil {
		return false, fmt.Errorf("agent dispatcher database and job store are required")
	}
	now := time.Now().UTC()
	if d.Now != nil {
		now = d.Now().UTC()
	}
	var taskID string
	var organizationID contract.OrganizationID
	var projectID contract.ProjectID
	err := d.DB.QueryRowContext(ctx, `SELECT d.agent_task_id, d.organization_id, d.project_id
		FROM platform_agent_dispatches d
		JOIN platform_agent_tasks t ON t.organization_id = d.organization_id
			AND t.project_id = d.project_id AND t.id = d.agent_task_id
			AND t.status = 'dispatch_pending'
		WHERE d.status = 'pending' AND d.available_at <= ?
		ORDER BY d.available_at, d.created_at LIMIT 1`, now).Scan(&taskID, &organizationID, &projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	store := MySQLStore{DB: d.DB}
	task, err := store.getByID(ctx, organizationID, projectID, taskID)
	if err != nil {
		return true, d.recordFailure(ctx, taskID, now, err)
	}
	payload, hash, err := dispatchPayload(task.ID)
	if err != nil {
		return true, d.recordFailure(ctx, taskID, now, err)
	}
	newID := d.NewID
	if newID == nil {
		newID = ids.New
	}
	jobID, err := newID("agentjob")
	if err != nil {
		return true, d.recordFailure(ctx, taskID, now, err)
	}
	job, _, err := d.Jobs.Enqueue(ctx, jobruntime.CreateRequest{
		Job: contract.Job{
			ID: jobID, Kind: task.Kind, OrganizationID: task.OrganizationID, ProjectID: task.ProjectID,
			Status: contract.JobQueued, Cancellable: true, MaxAttempts: 2, Version: 1,
			CreatedAt: now, UpdatedAt: now,
		},
		Payload: payload, IdempotencyKey: contract.IdempotencyKey("agent-" + task.ID), RequestHash: hash,
	})
	if err != nil {
		return true, d.recordFailure(ctx, taskID, now, err)
	}
	tx, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return true, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE platform_agent_tasks SET job_id = ?, status = ?,
		version = version + 1, updated_at = ? WHERE organization_id = ? AND project_id = ?
		AND id = ? AND status = ?`,
		job.ID, TaskQueued, now, task.OrganizationID, task.ProjectID, task.ID, TaskDispatchPending)
	if err != nil {
		return true, err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		if err == nil {
			err = ErrVersionConflict
		}
		return true, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE platform_agent_dispatches SET status = 'dispatched',
		attempt_count = attempt_count + 1, updated_at = ? WHERE agent_task_id = ? AND status = 'pending'`, now, task.ID); err != nil {
		return true, err
	}
	return true, tx.Commit()
}

func dispatchPayload(taskID string) (json.RawMessage, string, error) {
	payload, err := json.Marshal(struct {
		AgentTaskID string `json:"agent_task_id"`
	}{AgentTaskID: taskID})
	if err != nil {
		return nil, "", err
	}
	hash, err := contract.CanonicalJSONHash(json.RawMessage(payload))
	if err != nil {
		return nil, "", err
	}
	return payload, hash, nil
}

func (d Dispatcher) recordFailure(ctx context.Context, taskID string, now time.Time, cause error) error {
	delay := d.RetryIn
	if delay <= 0 {
		delay = 5 * time.Second
	}
	_, err := d.DB.ExecContext(ctx, `UPDATE platform_agent_dispatches SET attempt_count = attempt_count + 1,
		available_at = ?, last_error = ?, updated_at = ? WHERE agent_task_id = ? AND status = 'pending'`,
		now.Add(delay), truncate(cause.Error(), 1024), now, taskID)
	return err
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
