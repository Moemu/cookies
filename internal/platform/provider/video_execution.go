package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func (s Service) RecordVideoExecutionAttempt(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, jobID string, attemptCount, maxAttempts int) (contract.ProviderJob, error) {
	return s.RecordImageExecutionAttempt(ctx, organizationID, projectID, jobID, attemptCount, maxAttempts)
}

// ExecuteVideoJob advances one asynchronous video job by one external
// operation. Provider owns the task state; Assets only sees a verified output
// reference after the upstream task succeeds.
func (s Service) ExecuteVideoJob(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, jobID string) (contract.ProviderJob, *time.Time, error) {
	if s.Store == nil {
		return contract.ProviderJob{}, nil, fmt.Errorf("provider job store is required")
	}
	record, err := s.Store.Get(ctx, organizationID, projectID, jobID)
	if err != nil {
		return contract.ProviderJob{}, nil, err
	}
	if record.Operation != videoGenerateOperation {
		return contract.ProviderJob{}, nil, fmt.Errorf("provider job %s is not a video generation job", jobID)
	}
	if isProviderTerminal(record.Job.ProviderStatus) {
		return record.Job, nil, nil
	}
	switch record.Job.ProviderStatus {
	case contract.ProviderJobSubmitted:
		return s.submitVideoJob(ctx, record)
	case contract.ProviderJobRunning:
		return s.pollVideoJob(ctx, record)
	case contract.ProviderJobOutputsReady, contract.ProviderJobIngesting:
		return s.ProcessVideoJob(ctx, organizationID, projectID, jobID)
	default:
		return contract.ProviderJob{}, nil, fmt.Errorf("provider video job %s has unsupported status %q", jobID, record.Job.ProviderStatus)
	}
}

func (s Service) submitVideoJob(ctx context.Context, record JobRecord) (contract.ProviderJob, *time.Time, error) {
	if s.VideoAdapter == nil {
		return contract.ProviderJob{}, nil, fmt.Errorf("video provider adapter is required")
	}
	submission, err := s.VideoAdapter.Submit(ctx, VideoGenerationRequest{
		OrganizationID: record.Job.OrganizationID,
		ProjectID:      record.Job.ProjectID,
		ProviderJobID:  record.Job.ID,
		ModelAlias:     record.ModelAlias,
		IdempotencyKey: record.IdempotencyKey,
		Input:          record.VideoInput,
		Route:          record.Route,
	})
	if err != nil {
		return record.Job, nil, err
	}
	if err := submission.Validate(); err != nil {
		return contract.ProviderJob{}, nil, fmt.Errorf("video provider submission: %w", err)
	}
	now := s.nowUTC()
	record.ProviderCode = submission.ProviderCode
	record.ModelVersion = submission.ModelVersion
	record.ExternalTaskID = submission.ExternalTaskID
	record.Job.ExecutionStatus = contract.JobRunning
	record.Job.ProviderStatus = contract.ProviderJobRunning
	record.Job.Progress = 20
	record.Job.UpdatedAt = now
	updated, err := s.Store.Update(ctx, record)
	if err != nil {
		return contract.ProviderJob{}, nil, err
	}
	return updated.Job, deferAt(now), nil
}

func (s Service) pollVideoJob(ctx context.Context, record JobRecord) (contract.ProviderJob, *time.Time, error) {
	if s.VideoAdapter == nil {
		return contract.ProviderJob{}, nil, fmt.Errorf("video provider adapter is required")
	}
	result, err := s.VideoAdapter.Poll(ctx, VideoTaskReference{
		OrganizationID: record.Job.OrganizationID,
		ProjectID:      record.Job.ProjectID,
		ProviderJobID:  record.Job.ID,
		ProviderCode:   record.ProviderCode,
		ModelAlias:     record.ModelAlias,
		ModelVersion:   record.ModelVersion,
		ExternalTaskID: record.ExternalTaskID,
		Route:          record.Route,
	})
	if err != nil {
		return record.Job, nil, err
	}
	if err := result.Validate(); err != nil {
		return contract.ProviderJob{}, nil, fmt.Errorf("video provider poll result: %w", err)
	}
	now := s.nowUTC()
	switch result.Status {
	case VideoTaskRunning:
		record.Job.ExecutionStatus = contract.JobRunning
		record.Job.ProviderStatus = contract.ProviderJobRunning
		record.Job.Progress = result.Progress
		record.Job.UpdatedAt = now
		updated, updateErr := s.Store.Update(ctx, record)
		if updateErr != nil {
			return contract.ProviderJob{}, nil, updateErr
		}
		return updated.Job, deferAt(now), nil
	case VideoTaskSucceeded:
		outputs, outputErr := normalizeReadyOutputs(record, result.Outputs)
		if outputErr != nil {
			return contract.ProviderJob{}, nil, outputErr
		}
		record.Outputs = outputs
		record.Job.ExecutionStatus = contract.JobRunning
		record.Job.ProviderStatus = contract.ProviderJobOutputsReady
		record.Job.Progress = 70
		record.Job.UpdatedAt = now
		if _, updateErr := s.Store.Update(ctx, record); updateErr != nil {
			return contract.ProviderJob{}, nil, updateErr
		}
		return s.ProcessVideoJob(ctx, record.Job.OrganizationID, record.Job.ProjectID, record.Job.ID)
	case VideoTaskFailed:
		record.Job.ExecutionStatus = contract.JobFailed
		record.Job.ProviderStatus = contract.ProviderJobFailed
		record.Job.Progress = 100
		record.Job.Error = result.Error
		record.Job.UpdatedAt = now
		updated, updateErr := s.Store.Update(ctx, record)
		if updateErr != nil {
			return contract.ProviderJob{}, nil, updateErr
		}
		return updated.Job, nil, nil
	default:
		return contract.ProviderJob{}, nil, fmt.Errorf("video task status is invalid")
	}
}
