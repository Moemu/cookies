package agent

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
)

type DomainHandler func(context.Context, Task) (*contract.ResourceRef, error)
type FinalFailureHandler func(Task, contract.JobError)

func RuntimeHandler(store MySQLStore, handler DomainHandler, controls ...jobruntime.Canceller) jobruntime.Handler {
	return RuntimeHandlerWithFinalFailure(store, handler, nil, controls...)
}

func RuntimeHandlerWithFinalFailure(
	store MySQLStore,
	handler DomainHandler,
	onFinalFailure FinalFailureHandler,
	controls ...jobruntime.Canceller,
) jobruntime.Handler {
	return func(ctx context.Context, claim jobruntime.Claim) (jobruntime.Result, error) {
		var payload struct {
			AgentTaskID string `json:"agent_task_id"`
		}
		if err := json.Unmarshal(claim.Payload, &payload); err != nil || payload.AgentTaskID == "" {
			return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "AGENT_PAYLOAD_INVALID", Message: "Agent task payload is invalid"}}
		}
		task, err := store.Get(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, payload.AgentTaskID)
		if err != nil {
			return jobruntime.Result{}, err
		}
		if task.Status == TaskSucceeded {
			return jobruntime.Result{Ref: task.ResultRef}, nil
		}
		if task.Status == TaskCancelled {
			return jobruntime.Result{}, context.Canceled
		}
		if task.Status == TaskQueued {
			if err := store.MarkRunning(ctx, task.OrganizationID, task.ProjectID, task.ID, time.Now().UTC()); err != nil {
				return jobruntime.Result{}, err
			}
		}
		ref, err := handler(ctx, task)
		if err != nil {
			var execution jobruntime.ExecutionError
			problem := contract.JobError{Code: "AGENT_EXECUTION_FAILED", Message: "Agent execution failed", Retryable: true}
			if errors.As(err, &execution) {
				problem = execution.JobError
			}
			if problem.Retryable && claim.Job.AttemptCount < claim.Job.MaxAttempts {
				if retryErr := store.MarkRetrying(ctx, task.OrganizationID, task.ProjectID, task.ID, time.Now().UTC()); retryErr != nil {
					return jobruntime.Result{}, retryErr
				}
				delay := time.Duration(1<<min(claim.Job.AttemptCount, 6)) * time.Second
				return jobruntime.Result{}, jobruntime.DeferredError{AvailableAt: time.Now().UTC().Add(delay)}
			}
			markErr := store.MarkFailed(ctx, task.OrganizationID, task.ProjectID, task.ID, problem, time.Now().UTC())
			if markErr == nil && onFinalFailure != nil {
				onFinalFailure(task, problem)
			}
			return jobruntime.Result{}, err
		}
		if len(controls) > 0 && controls[0] != nil {
			cancelled, cancelErr := controls[0].IsCancelRequested(ctx, task.OrganizationID, claim.Job.ID)
			if cancelErr != nil {
				return jobruntime.Result{}, cancelErr
			}
			if cancelled {
				if cancelErr := store.MarkRunningCancelled(ctx, task.OrganizationID, task.ProjectID, task.ID, time.Now().UTC()); cancelErr != nil {
					return jobruntime.Result{}, cancelErr
				}
				return jobruntime.Result{}, nil
			}
		}
		if err := store.MarkSucceeded(ctx, task.OrganizationID, task.ProjectID, task.ID, ref, time.Now().UTC()); err != nil {
			return jobruntime.Result{}, err
		}
		return jobruntime.Result{Ref: ref}, nil
	}
}
