package creative

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

const DirectionGenerationJobKind = "creative.direction.generate.v1"

type DirectionGenerationOperation struct {
	BatchID        string                `json:"batch_id"`
	IntakeID       string                `json:"intake_id"`
	CandidateCount int                   `json:"candidate_count"`
	Actor          contract.ActorContext `json:"actor"`
}

type DirectionGenerationScheduler interface {
	ScheduleDirectionGeneration(context.Context, contract.ProjectID, DirectionGenerationOperation) error
}

func (s Service) StartDirectionGeneration(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	intakeID string,
	request GenerateDirectionRequest,
) (CreativeDirectionBatch, error) {
	if s.Repository == nil || s.Projects == nil || s.DirectionPlanner == nil || s.Directions == nil || s.DirectionScheduler == nil {
		return CreativeDirectionBatch{}, fmt.Errorf("creative direction planning is unavailable")
	}
	if !actor.HasScope(ScopeWrite) {
		return CreativeDirectionBatch{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	prepared, err := s.prepareDirectionPlanning(ctx, actor, projectID, intakeID, request)
	if err != nil {
		return CreativeDirectionBatch{}, err
	}
	if reader, ok := s.Directions.(DirectionBatchReader); ok {
		latest, latestErr := reader.GetLatestDirectionBatch(ctx, actor.OrganizationID, projectID, intakeID)
		if latestErr == nil && latest.Status == DirectionBatchGenerating && latest.InputIdentityHash == prepared.Intake.InputIdentityHash {
			return latest, nil
		}
		if latestErr != nil && !errors.Is(latestErr, ErrNotFound) {
			return CreativeDirectionBatch{}, latestErr
		}
	}
	batchID, err := s.idGenerator()("directionbatch")
	if err != nil {
		return CreativeDirectionBatch{}, err
	}
	batch := CreativeDirectionBatch{
		ContractVersion: CreativeDirectionBatchV1,
		ID:              batchID, OrganizationID: actor.OrganizationID, ProjectID: projectID, IntakeID: intakeID,
		InputIdentityHash: prepared.Intake.InputIdentityHash, Status: DirectionBatchGenerating,
		Candidates: []CreativeDirectionVersion{}, Model: "pending", PromptVersion: "pending",
		CreatedBy: actor.Principal.ID, CreatedAt: s.now(),
	}
	created, err := s.Directions.CreateDirectionBatch(ctx, batch)
	if err != nil {
		return CreativeDirectionBatch{}, err
	}
	operation := DirectionGenerationOperation{BatchID: batchID, IntakeID: intakeID, CandidateCount: prepared.CandidateCount, Actor: actor}
	if err := s.DirectionScheduler.ScheduleDirectionGeneration(ctx, projectID, operation); err != nil {
		_ = s.Directions.FailDirectionBatch(ctx, actor.OrganizationID, projectID, batchID, "DIRECTION_JOB_SCHEDULE_FAILED")
		return CreativeDirectionBatch{}, fmt.Errorf("schedule creative direction generation: %w", err)
	}
	return created, nil
}

func (s Service) HandleDirectionGenerationJob(ctx context.Context, claim jobruntime.Claim) (jobruntime.Result, error) {
	if claim.Job.Kind != DirectionGenerationJobKind || s.Repository == nil || s.Projects == nil || s.DirectionPlanner == nil || s.Directions == nil {
		return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "DIRECTION_HANDLER_UNAVAILABLE", Message: "Creative direction handler is unavailable", Retryable: false}}
	}
	var operation DirectionGenerationOperation
	if err := json.Unmarshal(claim.Payload, &operation); err != nil || strings.TrimSpace(operation.BatchID) == "" || strings.TrimSpace(operation.IntakeID) == "" || operation.CandidateCount < 2 || operation.CandidateCount > 4 {
		return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: "DIRECTION_JOB_PAYLOAD_INVALID", Message: "Creative direction job payload is invalid", Retryable: false}}
	}
	batch, err := s.Directions.GetDirectionBatch(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, operation.BatchID)
	if err != nil {
		return jobruntime.Result{}, err
	}
	if batch.Status == DirectionBatchReady {
		return jobruntime.Result{Ref: &contract.ResourceRef{Type: "creative_direction_batch", ID: batch.ID}}, nil
	}
	if batch.Status == DirectionBatchFailed {
		return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: batch.FailureCode, Message: "Creative direction generation failed", Retryable: false}}
	}
	prepared, err := s.prepareDirectionPlanning(ctx, operation.Actor, claim.Job.ProjectID, operation.IntakeID, GenerateDirectionRequest{CandidateCount: operation.CandidateCount})
	if err != nil {
		return s.failDirectionGeneration(ctx, claim, operation.BatchID, "DIRECTION_INPUT_UNAVAILABLE", err)
	}
	if prepared.Intake.InputIdentityHash != batch.InputIdentityHash {
		return s.failDirectionGeneration(ctx, claim, operation.BatchID, "DIRECTION_INPUT_CHANGED", fmt.Errorf("creative intake identity changed"))
	}
	ready, err := s.planDirectionBatch(ctx, operation.Actor, prepared, operation.BatchID, batch.CreatedAt)
	if err != nil {
		return s.failDirectionGeneration(ctx, claim, operation.BatchID, directionFailureCode(err), err)
	}
	ready, err = s.Directions.CompleteDirectionBatch(ctx, ready)
	if err != nil {
		return jobruntime.Result{}, err
	}
	return jobruntime.Result{Ref: &contract.ResourceRef{Type: "creative_direction_batch", ID: ready.ID}}, nil
}

func (s Service) failDirectionGeneration(ctx context.Context, claim jobruntime.Claim, batchID, code string, cause error) (jobruntime.Result, error) {
	_ = s.Directions.FailDirectionBatch(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, batchID, code)
	return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{Code: code, Message: boundedError(cause), Retryable: false}}
}

func directionFailureCode(err error) string {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "provider") || strings.Contains(message, "deadline") || strings.Contains(message, "timeout") {
		return "DIRECTION_PROVIDER_FAILED"
	}
	if strings.Contains(message, "validation") || strings.Contains(message, "candidate") || strings.Contains(message, "similar") {
		return "DIRECTION_QUALITY_VALIDATION_FAILED"
	}
	return "DIRECTION_GENERATION_FAILED"
}

type JobRuntimeDirectionGenerationScheduler struct {
	Store jobruntime.Store
	Now   func() time.Time
}

func (s JobRuntimeDirectionGenerationScheduler) ScheduleDirectionGeneration(ctx context.Context, projectID contract.ProjectID, operation DirectionGenerationOperation) error {
	if s.Store == nil {
		return fmt.Errorf("creative direction job store is required")
	}
	payload, err := json.Marshal(operation)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	digest := sha256.Sum256(payload)
	_, _, err = s.Store.Enqueue(ctx, jobruntime.CreateRequest{
		Job: contract.Job{ID: operation.BatchID, Kind: DirectionGenerationJobKind, OrganizationID: operation.Actor.OrganizationID,
			ProjectID: projectID, Status: contract.JobQueued, MaxAttempts: 1, Version: 1, CreatedAt: now, UpdatedAt: now},
		Payload: payload, IdempotencyKey: contract.IdempotencyKey("creative-direction-" + operation.BatchID), RequestHash: hex.EncodeToString(digest[:]),
	})
	return err
}
