package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
)

const ResearchJobKind = "knowledge.research.execute"

type researchJobStore interface {
	Enqueue(context.Context, jobruntime.CreateRequest) (contract.Job, bool, error)
}

type JobRuntimeResearchScheduler struct {
	Store researchJobStore
	NewID func() (string, error)
	Now   func() time.Time
}

func (s JobRuntimeResearchScheduler) Schedule(ctx context.Context, run ResearchRun) error {
	return s.schedule(ctx, run, false)
}

func (s JobRuntimeResearchScheduler) ScheduleResearchRetry(ctx context.Context, run ResearchRun) error {
	return s.schedule(ctx, run, true)
}

func (s JobRuntimeResearchScheduler) schedule(ctx context.Context, run ResearchRun, retry bool) error {
	if s.Store == nil || s.NewID == nil {
		return fmt.Errorf("research job store and ID generator are required")
	}
	jobID, err := s.NewID()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		ResearchRunID string `json:"research_run_id"`
	}{ResearchRunID: run.ID})
	if err != nil {
		return err
	}
	sum := sha256.Sum256(payload)
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	idempotencyKey := "knowledge_research_" + run.ID
	if retry {
		idempotencyKey += "_retry_" + jobID
	}
	_, _, err = s.Store.Enqueue(ctx, jobruntime.CreateRequest{
		Job: contract.Job{
			ID: jobID, Kind: ResearchJobKind,
			OrganizationID: run.OrganizationID, ProjectID: run.ProjectID,
			Status: contract.JobQueued, Progress: 0, Cancellable: true,
			AttemptCount: 0, MaxAttempts: 2, Version: 1,
			CreatedAt: now().UTC(), UpdatedAt: now().UTC(),
		},
		Payload:        payload,
		IdempotencyKey: contract.IdempotencyKey(idempotencyKey),
		RequestHash:    hex.EncodeToString(sum[:]),
	})
	return err
}

func (s Service) HandleResearchJob(ctx context.Context, claim jobruntime.Claim) (jobruntime.Result, error) {
	if claim.Job.Kind != ResearchJobKind {
		return jobruntime.Result{}, fmt.Errorf("unsupported research job kind %q", claim.Job.Kind)
	}
	var payload struct {
		ResearchRunID string `json:"research_run_id"`
	}
	if err := json.Unmarshal(claim.Payload, &payload); err != nil || strings.TrimSpace(payload.ResearchRunID) == "" {
		return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{
			Code: "INVALID_RESEARCH_JOB", Message: "Research job payload is invalid", Retryable: false,
		}}
	}
	run, err := s.getResearchRun(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, payload.ResearchRunID)
	if err != nil {
		return jobruntime.Result{}, err
	}
	ref := contract.ResourceRef{Type: "knowledge_research_run", ID: run.ID}
	if !researchStatusActive(run.Status) {
		return jobruntime.Result{Ref: &ref}, nil
	}
	if err := s.reportJobProgress(ctx, claim, 10, "研究任务已启动，正在确认目标与可用资料"); err != nil {
		return jobruntime.Result{}, err
	}
	if s.Runner == nil {
		s.markResearchTerminal(ctx, run, "failed", "EXTERNAL_RUNNER_UNAVAILABLE", ErrExternalRunnerUnavailable.Error())
		return jobruntime.Result{Ref: &ref}, nil
	}
	documents, err := s.selectResearchChunks(
		ctx, run.OrganizationID, run.ProjectID, run.DocumentIDs, run.Query,
	)
	if err != nil {
		s.markResearchTerminal(
			ctx, run, "failed", "RESEARCH_DOCUMENT_UNAVAILABLE",
			"研究所选择的内部资料片段已不可用",
		)
		return jobruntime.Result{Ref: &ref}, nil
	}
	if err := s.reportJobProgress(ctx, claim, 30, "内部资料已准备，正在执行联网检索与来源整理"); err != nil {
		return jobruntime.Result{}, err
	}
	var executeErr error
	if run.RunMode == "deep" {
		allowProviderRetry := claim.Job.AttemptCount > 0 && claim.Job.AttemptCount < claim.Job.MaxAttempts
		_, executeErr = s.executeDeepResearch(ctx, run, documents, func(round int, status, message string) error {
			progress := 20
			if run.MaxRounds > 0 {
				progress = 15 + (round*65)/run.MaxRounds
			}
			if status == "drafting" || status == "auditing" {
				progress = 88
			}
			return s.reportJobProgress(ctx, claim, progress, message)
		}, allowProviderRetry)
	} else {
		_, executeErr = s.executeResearch(ctx, run, documents)
	}
	if errors.Is(executeErr, errResearchProviderRetryable) {
		backoff := time.Duration(maxInt(claim.Job.AttemptCount, 1)*2) * time.Second
		return jobruntime.Result{}, jobruntime.DeferredError{AvailableAt: s.now().Add(backoff)}
	}
	if executeErr != nil {
		s.markResearchTerminal(ctx, run, "failed", "RESEARCH_PERSIST_FAILED", "研究结果持久化失败")
		return jobruntime.Result{}, executeErr
	}
	if err := s.reportJobProgress(ctx, claim, 95, "研究结果已返回，正在确认持久化结果"); err != nil {
		return jobruntime.Result{}, err
	}
	return jobruntime.Result{Ref: &ref}, nil
}

func (s Service) markResearchTerminal(ctx context.Context, run ResearchRun, status, code, message string) {
	now := s.now()
	_, _ = s.DB.ExecContext(ctx, `UPDATE platform_research_runs
		SET status = ?, error_code = ?, error_message = ?, stop_reason = ?,
			heartbeat_at = ?, completed_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?
		  AND status IN ('queued', 'planning', 'searching', 'reading', 'cross_checking', 'drafting', 'auditing')`,
		status, code, message, strings.ToLower(code), now, now, now,
		run.OrganizationID, run.ProjectID, run.ID)
}
