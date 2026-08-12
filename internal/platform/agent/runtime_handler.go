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
		runtimeNow := func() time.Time {
			current := time.Now().UTC()
			if claim.Job.UpdatedAt.After(current) {
				return claim.Job.UpdatedAt.UTC()
			}
			return current
		}
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
			if err := store.MarkRunning(ctx, task.OrganizationID, task.ProjectID, task.ID, runtimeNow()); err != nil {
				return jobruntime.Result{}, err
			}
		}
		if progress, ok := firstProgressReporter(controls); ok {
			if err := progress.UpdateProgress(ctx, claim, 10, agentProgressMessage(task.Kind, false), runtimeNow()); err != nil {
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
				if retryErr := store.MarkRetrying(ctx, task.OrganizationID, task.ProjectID, task.ID, runtimeNow()); retryErr != nil {
					return jobruntime.Result{}, retryErr
				}
				delay := time.Duration(1<<min(claim.Job.AttemptCount, 6)) * time.Second
				return jobruntime.Result{}, jobruntime.DeferredError{AvailableAt: runtimeNow().Add(delay)}
			}
			markErr := store.MarkFailed(ctx, task.OrganizationID, task.ProjectID, task.ID, problem, runtimeNow())
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
				if cancelErr := store.MarkRunningCancelled(ctx, task.OrganizationID, task.ProjectID, task.ID, runtimeNow()); cancelErr != nil {
					return jobruntime.Result{}, cancelErr
				}
				return jobruntime.Result{}, nil
			}
		}
		if progress, ok := firstProgressReporter(controls); ok {
			if err := progress.UpdateProgress(ctx, claim, 95, agentProgressMessage(task.Kind, true), runtimeNow()); err != nil {
				return jobruntime.Result{}, err
			}
		}
		if err := store.MarkSucceeded(ctx, task.OrganizationID, task.ProjectID, task.ID, ref, runtimeNow()); err != nil {
			return jobruntime.Result{}, err
		}
		return jobruntime.Result{Ref: ref}, nil
	}
}

func firstProgressReporter(controls []jobruntime.Canceller) (jobruntime.ProgressReporter, bool) {
	if len(controls) == 0 || controls[0] == nil {
		return nil, false
	}
	value, ok := controls[0].(jobruntime.ProgressReporter)
	return value, ok
}

func agentProgressMessage(kind string, saving bool) string {
	if saving {
		return "当前 AI 阶段已经完成，正在保存结构化结果"
	}
	switch kind {
	case "strategy.brief.extract":
		return "已读取当前对话，正在理解需求并更新 Brief"
	case "strategy.draft.generate":
		return "已固定 Brief 版本，正在生成策略草稿"
	case "strategy.draft.revise":
		return "已固定修改范围，正在生成策略修订"
	case "strategy.review.deep":
		return "已读取候选策略，正在生成客观的第二视角"
	default:
		return "已获得执行资源，正在处理当前 AI 阶段"
	}
}
