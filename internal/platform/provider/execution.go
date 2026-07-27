package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const providerPollDelay = 5 * time.Second

// RecordImageExecutionAttempt mirrors the shared execution attempt budget onto
// the public ProviderJob. The generic job remains the lease owner; this method
// makes the provider-facing state truthful for users and operators.
func (s Service) RecordImageExecutionAttempt(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, jobID string, attemptCount, maxAttempts int) (contract.ProviderJob, error) {
	if s.Store == nil {
		return contract.ProviderJob{}, fmt.Errorf("provider job store is required")
	}
	if attemptCount < 1 || maxAttempts < 1 || attemptCount > maxAttempts {
		return contract.ProviderJob{}, fmt.Errorf("provider execution attempts are invalid")
	}
	record, err := s.Store.Get(ctx, organizationID, projectID, jobID)
	if err != nil {
		return contract.ProviderJob{}, err
	}
	if isProviderTerminal(record.Job.ProviderStatus) {
		return record.Job, nil
	}
	if record.Job.AttemptCount == attemptCount && record.Job.MaxAttempts == maxAttempts {
		return record.Job, nil
	}
	record.Job.AttemptCount = attemptCount
	record.Job.MaxAttempts = maxAttempts
	record.Job.UpdatedAt = s.nowUTC()
	updated, err := s.Store.Update(ctx, record)
	if err != nil {
		return contract.ProviderJob{}, err
	}
	return updated.Job, nil
}

// ExecuteImageJob advances one durable image job by one external operation.
// It is safe for retrying workers: the same idempotency key is presented to
// Submit, and every observed transition is persisted before returning.
func (s Service) ExecuteImageJob(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, jobID string) (contract.ProviderJob, *time.Time, error) {
	if s.Store == nil {
		return contract.ProviderJob{}, nil, fmt.Errorf("provider job store is required")
	}
	record, err := s.Store.Get(ctx, organizationID, projectID, jobID)
	if err != nil {
		return contract.ProviderJob{}, nil, err
	}
	if isProviderTerminal(record.Job.ProviderStatus) {
		return record.Job, nil, nil
	}

	switch record.Job.ProviderStatus {
	case contract.ProviderJobSubmitted:
		return s.submitImageJob(ctx, record)
	case contract.ProviderJobRunning:
		return s.pollImageJob(ctx, record)
	case contract.ProviderJobOutputsReady, contract.ProviderJobIngesting:
		return s.ProcessImageJob(ctx, organizationID, projectID, jobID)
	default:
		return contract.ProviderJob{}, nil, fmt.Errorf("provider job %s has unsupported status %q", jobID, record.Job.ProviderStatus)
	}
}

func (s Service) submitImageJob(ctx context.Context, record JobRecord) (contract.ProviderJob, *time.Time, error) {
	if s.ImageAdapter == nil {
		return contract.ProviderJob{}, nil, fmt.Errorf("image provider adapter is required")
	}
	sources, err := s.resolveImageSources(ctx, record)
	if err != nil {
		return contract.ProviderJob{}, nil, err
	}
	for _, source := range sources {
		defer source.Content.Close()
	}
	request := ImageGenerationRequest{
		OrganizationID: record.Job.OrganizationID, ProjectID: record.Job.ProjectID,
		ProviderJobID: record.Job.ID, ModelAlias: record.ModelAlias, IdempotencyKey: record.IdempotencyKey,
		Input: record.Input, Route: record.Route, Sources: sources,
	}
	if err := request.Validate(); err != nil {
		return contract.ProviderJob{}, nil, err
	}
	if record.Route != nil {
		switch record.SubmissionState {
		case SubmissionInFlight, SubmissionUnknown:
			return record.Job, nil, ExecutionError{JobError: contract.JobError{
				Code: "MODEL_SUBMISSION_UNKNOWN", Message: "Adapter gateway submission may already have been accepted and will not be retried automatically", Retryable: false,
			}}
		case SubmissionCompleted:
			return contract.ProviderJob{}, nil, fmt.Errorf("provider job %s has a completed submission but remains submitted", record.Job.ID)
		case "", SubmissionNotStarted:
			if preparer, ok := s.ImageAdapter.(ImageSubmissionPreparer); ok {
				if err := preparer.Prepare(ctx, request); err != nil {
					return record.Job, nil, err
				}
			}
			now := s.nowUTC()
			deadline := routeDeadline(*record.Route, now)
			record.SubmissionState = SubmissionInFlight
			record.SubmittedAt = &now
			record.ExecutionDeadlineAt = &deadline
			record.Job.ExecutionStatus = contract.JobRunning
			record.Job.UpdatedAt = now
			updated, updateErr := s.Store.Update(ctx, record)
			if updateErr != nil {
				return contract.ProviderJob{}, nil, updateErr
			}
			record = updated
		default:
			return contract.ProviderJob{}, nil, fmt.Errorf("provider job %s has invalid submission state %q", record.Job.ID, record.SubmissionState)
		}
	}
	submission, err := s.ImageAdapter.Submit(ctx, request)
	if err != nil {
		if record.Route != nil {
			now := s.nowUTC()
			record.SubmissionState = SubmissionUnknown
			record.ResponseReceivedAt = &now
			record.Job.UpdatedAt = now
			if _, updateErr := s.Store.Update(context.WithoutCancel(ctx), record); updateErr != nil {
				return record.Job, nil, fmt.Errorf("persist unknown adapter gateway submission: %w", updateErr)
			}
		}
		return record.Job, nil, err
	}
	if err := submission.Validate(); err != nil {
		return contract.ProviderJob{}, nil, fmt.Errorf("image provider submission: %w", err)
	}
	now := s.nowUTC()
	if record.Route != nil {
		record.SubmissionState = SubmissionCompleted
		record.ResponseReceivedAt = &now
		record.AdapterRequestID = submission.AdapterRequestID
		record.ActualProvider = submission.ActualProvider
		record.ActualModel = submission.ActualModel
	}
	record.ProviderCode = submission.ProviderCode
	record.ModelVersion = submission.ModelVersion
	record.ExternalTaskID = submission.ExternalTaskID
	switch submission.Status {
	case ImageSubmissionAccepted:
		record.Job.ExecutionStatus = contract.JobRunning
		record.Job.ProviderStatus = contract.ProviderJobRunning
		record.Job.Progress = 20
	case ImageSubmissionCompleted:
		outputs, outputErr := normalizeReadyOutputs(record, submission.Outputs)
		if outputErr != nil {
			return contract.ProviderJob{}, nil, outputErr
		}
		record.Outputs = outputs
		record.Job.ExecutionStatus = contract.JobRunning
		record.Job.ProviderStatus = contract.ProviderJobOutputsReady
		record.Job.Progress = 70
	default:
		return contract.ProviderJob{}, nil, fmt.Errorf("image submission status is invalid")
	}
	record.Job.UpdatedAt = now
	updated, err := s.Store.Update(ctx, record)
	if err != nil {
		return contract.ProviderJob{}, nil, err
	}
	if submission.Status == ImageSubmissionCompleted {
		return s.ProcessImageJob(ctx, updated.Job.OrganizationID, updated.Job.ProjectID, updated.Job.ID)
	}
	return updated.Job, deferAt(now), nil
}

func (s Service) resolveImageSources(ctx context.Context, record JobRecord) ([]VisionSource, error) {
	if len(record.Input.SourceAssets) == 0 {
		return nil, nil
	}
	if s.VisionSources == nil {
		return nil, fmt.Errorf("image source resolver is required")
	}
	project := contract.ProjectContext{
		OrganizationID:        record.Job.OrganizationID,
		ProjectID:             record.Job.ProjectID,
		ProjectContextVersion: record.ProjectContextVersion,
	}
	actor := contract.ActorContext{OrganizationID: record.Job.OrganizationID, Principal: record.Principal, Scopes: []contract.Scope{}}
	sources, err := s.VisionSources.ResolveVisionSources(ctx, actor, project, record.Input.SourceAssets)
	if err != nil {
		return nil, err
	}
	if err := validateVisionSources(record.Input.SourceAssets, sources); err != nil {
		for _, source := range sources {
			if source.Content != nil {
				source.Content.Close()
			}
		}
		return nil, err
	}
	return sources, nil
}

func (s Service) pollImageJob(ctx context.Context, record JobRecord) (contract.ProviderJob, *time.Time, error) {
	if s.ImageAdapter == nil {
		return contract.ProviderJob{}, nil, fmt.Errorf("image provider adapter is required")
	}
	reference := ImageTaskReference{
		ProviderCode: record.ProviderCode, ModelAlias: record.ModelAlias, ModelVersion: record.ModelVersion, ExternalTaskID: record.ExternalTaskID,
	}
	if err := reference.Validate(); err != nil {
		return contract.ProviderJob{}, nil, fmt.Errorf("poll image provider job: %w", err)
	}
	result, err := s.ImageAdapter.Poll(ctx, reference)
	if err != nil {
		return record.Job, nil, err
	}
	if err := result.Validate(); err != nil {
		return contract.ProviderJob{}, nil, fmt.Errorf("image provider poll result: %w", err)
	}
	now := s.nowUTC()
	switch result.Status {
	case ImageTaskRunning:
		record.Job.ExecutionStatus = contract.JobRunning
		record.Job.ProviderStatus = contract.ProviderJobRunning
		record.Job.Progress = result.Progress
		record.Job.UpdatedAt = now
		updated, updateErr := s.Store.Update(ctx, record)
		if updateErr != nil {
			return contract.ProviderJob{}, nil, updateErr
		}
		return updated.Job, deferAt(now), nil
	case ImageTaskSucceeded:
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
		return s.ProcessImageJob(ctx, record.Job.OrganizationID, record.Job.ProjectID, record.Job.ID)
	case ImageTaskFailed:
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
		return contract.ProviderJob{}, nil, fmt.Errorf("image task status is invalid")
	}
}

func normalizeReadyOutputs(record JobRecord, refs []contract.ProviderOutputRef) ([]OutputRecord, error) {
	seen := make(map[string]struct{}, len(refs))
	outputs := make([]OutputRecord, 0, len(refs))
	for _, ref := range refs {
		if ref.ProviderJobID != record.Job.ID || ref.ProviderCode != record.ProviderCode {
			return nil, fmt.Errorf("image output does not belong to provider job %s", record.Job.ID)
		}
		if _, exists := seen[ref.OutputID]; exists {
			return nil, fmt.Errorf("image provider returned duplicate output ID %q", ref.OutputID)
		}
		seen[ref.OutputID] = struct{}{}
		outputs = append(outputs, OutputRecord{Ref: ref, Status: OutputReady})
	}
	return outputs, nil
}

// FailImageJobAfterExecutionExhausted moves the public ProviderJob to a
// terminal state when its shared runtime has exhausted recovery attempts.
// Any already-ingested output is retained, so the final state can truthfully
// be partially_succeeded rather than discarding durable project assets.
func (s Service) FailImageJobAfterExecutionExhausted(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, jobID string) (contract.ProviderJob, error) {
	if s.Store == nil {
		return contract.ProviderJob{}, fmt.Errorf("provider job store is required")
	}
	record, err := s.Store.Get(ctx, organizationID, projectID, jobID)
	if err != nil {
		return contract.ProviderJob{}, err
	}
	if isProviderTerminal(record.Job.ProviderStatus) {
		return record.Job, nil
	}
	now := s.nowUTC()
	problem := &contract.JobError{Code: "PROVIDER_EXECUTION_EXHAUSTED", Message: "Provider job exceeded its recovery attempt limit", Retryable: false}
	if len(record.Outputs) == 0 {
		record.Job.ExecutionStatus = contract.JobFailed
		record.Job.ProviderStatus = contract.ProviderJobFailed
		record.Job.Progress = 100
		record.Job.Error = problem
		record.Job.UpdatedAt = now
	} else {
		for index := range record.Outputs {
			if record.Outputs[index].Status == OutputReady || record.Outputs[index].Status == OutputIngesting {
				errCopy := *problem
				record.Outputs[index].Status = OutputFailed
				record.Outputs[index].Error = &errCopy
			}
		}
		finalizeImageJob(&record.Job, record.Outputs, now)
	}
	updated, err := s.Store.Update(ctx, record)
	if err != nil {
		return contract.ProviderJob{}, err
	}
	return updated.Job, nil
}

func deferAt(now time.Time) *time.Time {
	deferred := now.Add(providerPollDelay)
	return &deferred
}
