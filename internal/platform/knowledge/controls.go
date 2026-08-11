package knowledge

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
)

var ErrKnowledgeInvalidState = errors.New("knowledge resource is not in a controllable state")
var ErrKnowledgeControlUnavailable = errors.New("knowledge job control is unavailable")

// JobControl acknowledges a command against the existing Job Runtime. A
// running external call is not reported as cancelled until its worker reaches
// a safe terminal boundary.
type JobControl struct {
	ResourceType    string             `json:"resource_type"`
	ResourceID      string             `json:"resource_id"`
	ExecutionID     string             `json:"execution_id"`
	ExecutionStatus contract.JobStatus `json:"execution_status"`
	CancelRequested bool               `json:"cancel_requested"`
	AcceptedAt      time.Time          `json:"accepted_at"`
}

type activeKnowledgeJob struct {
	ID      string
	Version int64
}

func (s Service) CancelDocumentParse(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	documentID string,
) (JobControl, error) {
	if s.JobCanceller == nil {
		return JobControl{}, ErrKnowledgeControlUnavailable
	}
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return JobControl{}, err
	}
	document, err := scanDocument(s.DB.QueryRowContext(ctx, documentSelect+`
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		actor.OrganizationID, projectID, documentID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return JobControl{}, ErrNotFound
	}
	if err != nil {
		return JobControl{}, err
	}
	if document.Status != "parse_queued" && document.Status != "parsing" {
		return JobControl{}, ErrKnowledgeInvalidState
	}
	jobKind := DocumentParseJobKind
	if document.ParsePhase == "visual_fallback" &&
		(document.VisionFallbackStatus == "queued" || document.VisionFallbackStatus == "running") {
		jobKind = DocumentVisionFallbackJobKind
	}
	control, err := s.requestKnowledgeJobCancel(
		ctx, actor.OrganizationID, projectID, jobKind,
		"document_id", document.ID, "knowledge_document",
	)
	if err != nil {
		return JobControl{}, err
	}
	if control.ExecutionStatus == contract.JobCancelled {
		if jobKind == DocumentVisionFallbackJobKind {
			s.markDocumentVisionFailed(ctx, document, "DOCUMENT_VISION_CANCELLED", "Document visual fallback was cancelled")
		} else {
			_, _ = s.DB.ExecContext(ctx, `UPDATE platform_knowledge_documents
				SET status = 'parse_failed', parse_error_code = 'DOCUMENT_PARSE_CANCELLED',
					parse_error_message = '文档解析已取消', updated_at = ?
				WHERE organization_id = ? AND project_id = ? AND id = ?
				  AND status IN ('parse_queued', 'parsing')`,
				control.AcceptedAt, actor.OrganizationID, projectID, document.ID,
			)
			s.markDocumentParseFailed(ctx, document, "DOCUMENT_PARSE_CANCELLED", "Document parsing was cancelled")
		}
	}
	return control, nil
}

func (s Service) CancelResearch(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	researchRunID string,
) (JobControl, error) {
	if s.JobCanceller == nil {
		return JobControl{}, ErrKnowledgeControlUnavailable
	}
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return JobControl{}, err
	}
	run, err := s.getResearchRun(ctx, actor.OrganizationID, projectID, researchRunID)
	if err != nil {
		return JobControl{}, err
	}
	if !researchStatusActive(run.Status) || run.Purpose == "conversation_web_search" {
		return JobControl{}, ErrKnowledgeInvalidState
	}
	control, err := s.requestKnowledgeJobCancel(
		ctx, actor.OrganizationID, projectID, ResearchJobKind,
		"research_run_id", run.ID, "knowledge_research_run",
	)
	if err != nil {
		return JobControl{}, err
	}
	if control.ExecutionStatus == contract.JobCancelled {
		_, _ = s.DB.ExecContext(ctx, `UPDATE platform_research_runs
			SET status = 'cancelled', error_code = 'RESEARCH_CANCELLED',
				error_message = '研究任务已取消', stop_reason = 'user_cancelled',
				heartbeat_at = ?, completed_at = ?, updated_at = ?
			WHERE organization_id = ? AND project_id = ? AND id = ?
			  AND status IN ('queued', 'planning', 'searching', 'reading', 'cross_checking', 'drafting', 'auditing')`,
			control.AcceptedAt, control.AcceptedAt, control.AcceptedAt, actor.OrganizationID, projectID, run.ID,
		)
	}
	return control, nil
}

func (s Service) requestKnowledgeJobCancel(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	kind string,
	resourceField string,
	resourceID string,
	resourceType string,
) (JobControl, error) {
	for attempt := 0; attempt < 3; attempt++ {
		job, err := s.latestActiveKnowledgeJob(
			ctx, organizationID, projectID, kind, resourceField, resourceID,
		)
		if err != nil {
			return JobControl{}, err
		}
		now := s.now()
		cancelled, err := s.JobCanceller.RequestCancel(
			ctx, organizationID, projectID, job.ID, job.Version, now,
		)
		if errors.Is(err, jobruntime.ErrJobVersionConflict) {
			continue
		}
		if errors.Is(err, jobruntime.ErrJobNotFound) {
			return JobControl{}, ErrNotFound
		}
		if errors.Is(err, jobruntime.ErrJobNotCancellable) {
			return JobControl{}, ErrKnowledgeInvalidState
		}
		if err != nil {
			return JobControl{}, err
		}
		return JobControl{
			ResourceType: resourceType, ResourceID: resourceID,
			ExecutionID: cancelled.ID, ExecutionStatus: cancelled.Status,
			CancelRequested: true, AcceptedAt: now,
		}, nil
	}
	return JobControl{}, ErrKnowledgeInvalidState
}

func (s Service) latestActiveKnowledgeJob(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	kind string,
	resourceField string,
	resourceID string,
) (activeKnowledgeJob, error) {
	if resourceField != "document_id" && resourceField != "research_run_id" {
		return activeKnowledgeJob{}, fmt.Errorf("unsupported knowledge job resource field %q", resourceField)
	}
	var job activeKnowledgeJob
	err := s.DB.QueryRowContext(ctx, `SELECT id, version FROM platform_jobs
		WHERE organization_id = ? AND project_id = ? AND kind = ?
		  AND JSON_UNQUOTE(JSON_EXTRACT(payload, ?)) = ?
		  AND status IN ('queued', 'running')
		ORDER BY created_at DESC, id DESC LIMIT 1`,
		organizationID, projectID, kind, "$."+resourceField, resourceID,
	).Scan(&job.ID, &job.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return activeKnowledgeJob{}, ErrKnowledgeInvalidState
	}
	return job, err
}

func (s Service) RetryDocumentParse(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	documentID string,
) (Document, error) {
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return Document{}, err
	}
	retryScheduler, ok := s.DocumentScheduler.(DocumentParseRetryScheduler)
	if !ok {
		return Document{}, ErrKnowledgeControlUnavailable
	}
	document, err := scanDocument(s.DB.QueryRowContext(ctx, documentSelect+`
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		actor.OrganizationID, projectID, documentID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return Document{}, ErrNotFound
	}
	if err != nil {
		return Document{}, err
	}
	if document.Status == "parse_queued" || document.Status == "parsing" {
		return document, nil
	}
	if document.Status != "parse_failed" && document.Status != "partial" {
		return Document{}, ErrKnowledgeInvalidState
	}
	previousStatus := document.Status
	now := s.now()
	result, err := s.DB.ExecContext(ctx, `UPDATE platform_knowledge_documents
		SET status = 'parse_queued', parse_phase = 'queued', parse_progress = 0,
			progress_kind = 'milestone', processed_pages = NULL,
			preview_status = CASE WHEN chunk_count > 0 THEN 'partial' ELSE 'building' END,
			parse_error_code = NULL, parse_error_message = NULL, heartbeat_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND status = ?`,
		now, now, actor.OrganizationID, projectID, document.ID, previousStatus,
	)
	if err != nil {
		return Document{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return Document{}, err
	}
	if changed == 0 {
		return s.GetDocument(ctx, actor, projectID, document.ID)
	}
	document.Status = "parse_queued"
	document.ParsePhase = "queued"
	document.ParseProgress = intPointer(0)
	document.ProgressKind = "milestone"
	document.ProcessedPages = nil
	document.HeartbeatAt = &now
	if document.ChunkCount > 0 {
		document.PreviewStatus = "partial"
	} else {
		document.PreviewStatus = "building"
	}
	document.ParseErrorCode, document.ParseErrorMessage = "", ""
	document.UpdatedAt = now
	if err := retryScheduler.ScheduleDocumentParseRetry(ctx, document); err != nil {
		_, _ = s.DB.ExecContext(ctx, `UPDATE platform_knowledge_documents
			SET status = 'parse_failed', parse_error_code = 'DOCUMENT_PARSE_RETRY_SCHEDULE_FAILED',
				parse_error_message = '文档解析重试暂时无法进入队列', updated_at = ?
			WHERE organization_id = ? AND project_id = ? AND id = ? AND status = 'parse_queued'`,
			s.now(), actor.OrganizationID, projectID, document.ID,
		)
		s.markDocumentParseFailed(ctx, document, "DOCUMENT_PARSE_RETRY_SCHEDULE_FAILED", "Document parse retry could not be queued")
		return Document{}, err
	}
	return document, nil
}

func (s Service) RetryResearch(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	researchRunID string,
) (ResearchRun, error) {
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return ResearchRun{}, err
	}
	retryScheduler, ok := s.Scheduler.(ResearchRetryScheduler)
	if !ok {
		return ResearchRun{}, ErrKnowledgeControlUnavailable
	}
	run, err := s.getResearchRun(ctx, actor.OrganizationID, projectID, researchRunID)
	if err != nil {
		return ResearchRun{}, err
	}
	if run.Purpose == "conversation_web_search" {
		return ResearchRun{}, ErrKnowledgeInvalidState
	}
	if researchStatusActive(run.Status) {
		return run, nil
	}
	if run.Status != "failed" && run.Status != "partially_completed" && run.Status != "cancelled" {
		return ResearchRun{}, ErrKnowledgeInvalidState
	}
	previousStatus := run.Status
	now := s.now()
	result, err := s.DB.ExecContext(ctx, `UPDATE platform_research_runs
		SET status = 'planning',
			error_code = NULL, error_message = NULL,
			stop_reason = '', heartbeat_at = ?, completed_at = NULL, started_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND status = ?`,
		now, now, now, actor.OrganizationID, projectID, run.ID, previousStatus,
	)
	if err != nil {
		return ResearchRun{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return ResearchRun{}, err
	}
	if changed == 0 {
		return s.GetResearchRun(ctx, actor, projectID, run.ID)
	}
	run.Status = "planning"
	run.ErrorCode, run.ErrorMessage = "", ""
	run.StopReason = ""
	run.StartedAt = &now
	run.CompletedAt = nil
	run.UpdatedAt = now
	if err := retryScheduler.ScheduleResearchRetry(ctx, run); err != nil {
		_, _ = s.DB.ExecContext(ctx, `UPDATE platform_research_runs
			SET status = 'failed', error_code = 'RESEARCH_RETRY_SCHEDULE_FAILED',
				error_message = '研究重试暂时无法进入队列', stop_reason = 'retry_schedule_failed',
				completed_at = ?, updated_at = ?
			WHERE organization_id = ? AND project_id = ? AND id = ? AND status = 'planning'`,
			s.now(), s.now(), actor.OrganizationID, projectID, run.ID,
		)
		return ResearchRun{}, err
	}
	return run, nil
}
