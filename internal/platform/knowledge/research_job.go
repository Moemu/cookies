package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	_, _, err = s.Store.Enqueue(ctx, jobruntime.CreateRequest{
		Job: contract.Job{
			ID: jobID, Kind: ResearchJobKind,
			OrganizationID: run.OrganizationID, ProjectID: run.ProjectID,
			Status: contract.JobQueued, Progress: 0, Cancellable: true,
			AttemptCount: 0, MaxAttempts: 2, Version: 1,
			CreatedAt: now().UTC(), UpdatedAt: now().UTC(),
		},
		Payload:        payload,
		IdempotencyKey: contract.IdempotencyKey("knowledge_research_" + run.ID),
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
	if run.Status != "running" {
		return jobruntime.Result{Ref: &ref}, nil
	}
	if s.Runner == nil {
		s.markResearchTerminal(ctx, run, "unavailable", "EXTERNAL_RUNNER_UNAVAILABLE", ErrExternalRunnerUnavailable.Error())
		return jobruntime.Result{Ref: &ref}, nil
	}
	documents := make([]ExternalDocument, 0, len(run.DocumentIDs))
	for _, id := range run.DocumentIDs {
		document, err := scanDocument(s.DB.QueryRowContext(ctx, documentSelect+`
			WHERE organization_id = ? AND project_id = ? AND id = ?`,
			run.OrganizationID, run.ProjectID, id))
		if err != nil {
			s.markResearchTerminal(ctx, run, "failed", "RESEARCH_DOCUMENT_UNAVAILABLE", "研究所引用的内部资料已不可用")
			return jobruntime.Result{Ref: &ref}, nil
		}
		documents = append(documents, ExternalDocument{
			ID: document.ID, Filename: document.Filename, Content: document.ExtractedText,
		})
	}
	if _, err := s.executeResearch(ctx, run, documents); err != nil {
		s.markResearchTerminal(ctx, run, "failed", "RESEARCH_PERSIST_FAILED", "研究结果持久化失败")
		return jobruntime.Result{}, err
	}
	return jobruntime.Result{Ref: &ref}, nil
}

func (s Service) markResearchTerminal(ctx context.Context, run ResearchRun, status, code, message string) {
	_, _ = s.DB.ExecContext(ctx, `UPDATE platform_research_runs
		SET status = ?, error_code = ?, error_message = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND status = 'running'`,
		status, code, message, s.now(), run.OrganizationID, run.ProjectID, run.ID)
}
