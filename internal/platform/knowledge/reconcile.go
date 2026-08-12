package knowledge

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// JobStateReconciler continuously closes the small consistency window between
// a generic Job reaching a terminal state and its public Knowledge resource.
// It is deliberately a repair worker, not an executor or a second scheduler.
type JobStateReconciler struct {
	Service *Service
	Limit   int
}

func (r JobStateReconciler) RunOnce(ctx context.Context) (bool, error) {
	if r.Service == nil {
		return false, fmt.Errorf("knowledge service is required")
	}
	limit := r.Limit
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return r.Service.ReconcileJobStates(ctx, limit)
}

func (s Service) ReconcileJobStates(ctx context.Context, limit int) (bool, error) {
	if s.DB == nil {
		return false, fmt.Errorf("knowledge database is required")
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	processed := 0
	for processed < limit {
		changedAny := false
		documentChanged, err := s.reconcileOneDocumentJob(ctx)
		if err != nil {
			return processed > 0, err
		}
		if documentChanged {
			processed++
			changedAny = true
		}
		if processed >= limit {
			break
		}
		visionChanged, err := s.reconcileOneDocumentVisionJob(ctx)
		if err != nil {
			return processed > 0, err
		}
		if visionChanged {
			processed++
			changedAny = true
		}
		if processed >= limit {
			break
		}
		researchChanged, err := s.reconcileOneResearchJob(ctx)
		if err != nil {
			return processed > 0, err
		}
		if researchChanged {
			processed++
			changedAny = true
		}
		if processed >= limit {
			break
		}
		reconciliationRecovered, err := s.recoverOnePendingVisionReconciliation(ctx)
		if err != nil {
			return processed > 0, err
		}
		if reconciliationRecovered {
			processed++
			changedAny = true
		}
		if processed >= limit {
			break
		}
		orphanRecovered, err := s.recoverOneOrphanedVisionJob(ctx)
		if err != nil {
			return processed > 0, err
		}
		if orphanRecovered {
			processed++
			changedAny = true
		}
		if !changedAny {
			break
		}
	}
	return processed > 0, nil
}

func (s Service) reconcileOneDocumentVisionJob(ctx context.Context) (bool, error) {
	var value terminalKnowledgeJob
	err := s.DB.QueryRowContext(ctx, `SELECT document.organization_id, document.project_id,
		document.id, job.status, COALESCE(job.error_code, ''),
		COALESCE(job.error_message, ''), job.updated_at
		FROM platform_knowledge_documents AS document
		INNER JOIN platform_jobs AS job
		  ON job.organization_id = document.organization_id
		 AND job.project_id = document.project_id
		 AND job.kind = 'knowledge.document.vision_fallback'
		 AND JSON_UNQUOTE(JSON_EXTRACT(job.payload, '$.document_id')) = document.id
		WHERE document.vision_fallback_status IN ('queued', 'running')
		  AND job.status IN ('failed', 'cancelled')
		  AND job.updated_at >= document.updated_at
		  AND NOT EXISTS (
			SELECT 1 FROM platform_jobs AS newer
			WHERE newer.organization_id = job.organization_id
			  AND newer.project_id = job.project_id
			  AND newer.kind = job.kind
			  AND JSON_UNQUOTE(JSON_EXTRACT(newer.payload, '$.document_id')) = document.id
			  AND (newer.created_at > job.created_at
				OR (newer.created_at = job.created_at AND newer.id > job.id))
		  )
		ORDER BY job.updated_at, job.id LIMIT 1`).Scan(
		&value.OrganizationID, &value.ProjectID, &value.ResourceID, &value.Status,
		&value.ErrorCode, &value.ErrorMessage, &value.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	document, err := scanDocument(s.DB.QueryRowContext(ctx, documentSelect+`
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		value.OrganizationID, value.ProjectID, value.ResourceID,
	))
	if err != nil {
		return false, err
	}
	code, message := strings.TrimSpace(value.ErrorCode), strings.TrimSpace(value.ErrorMessage)
	if value.Status == contract.JobCancelled {
		code, message = "DOCUMENT_VISION_CANCELLED", "Document visual fallback was cancelled; prior text remains available"
	} else {
		if code == "" {
			code = "DOCUMENT_VISION_JOB_FAILED"
		}
		message = terminalJobMessage(message, "Document visual fallback could not be completed; prior text remains available")
	}
	return s.markDocumentVisionFailed(ctx, document, code, message), nil
}

func (s Service) recoverOneOrphanedVisionJob(ctx context.Context) (bool, error) {
	if s.VisionScheduler == nil {
		return false, nil
	}
	cutoff := s.now().Add(-5 * time.Second)
	document, err := scanDocument(s.DB.QueryRowContext(ctx, documentSelect+`
		WHERE vision_fallback_status = 'queued' AND updated_at <= ?
		  AND NOT EXISTS (
			SELECT 1 FROM platform_knowledge_document_vision_reconciliations AS reconciliation
			WHERE reconciliation.organization_id = platform_knowledge_documents.organization_id
			  AND reconciliation.project_id = platform_knowledge_documents.project_id
			  AND reconciliation.document_id = platform_knowledge_documents.id
			  AND reconciliation.attempt_id = platform_knowledge_documents.vision_attempt_id
			  AND reconciliation.status = 'applied'
			  AND reconciliation.decision = 'accepted'
			  AND reconciliation.scheduled_at IS NULL
		  )
		  AND NOT EXISTS (
			SELECT 1 FROM platform_jobs AS job
			WHERE job.organization_id = platform_knowledge_documents.organization_id
			  AND job.project_id = platform_knowledge_documents.project_id
			  AND job.kind = 'knowledge.document.vision_fallback'
			  AND JSON_UNQUOTE(JSON_EXTRACT(job.payload, '$.document_id')) = platform_knowledge_documents.id
			  AND job.status IN ('queued', 'running')
		  )
		ORDER BY updated_at, id LIMIT 1`, cutoff))
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := s.VisionScheduler.ScheduleDocumentVisionFallback(ctx, document, document.VisionSelectedPages, ""); err != nil {
		return false, err
	}
	return true, nil
}

type terminalKnowledgeJob struct {
	OrganizationID contract.OrganizationID
	ProjectID      contract.ProjectID
	ResourceID     string
	Status         contract.JobStatus
	ErrorCode      string
	ErrorMessage   string
	UpdatedAt      time.Time
}

func (s Service) reconcileOneDocumentJob(ctx context.Context) (bool, error) {
	var value terminalKnowledgeJob
	err := s.DB.QueryRowContext(ctx, `SELECT document.organization_id, document.project_id,
		document.id, job.status, COALESCE(job.error_code, ''),
		COALESCE(job.error_message, ''), job.updated_at
		FROM platform_knowledge_documents AS document
		INNER JOIN platform_jobs AS job
		  ON job.organization_id = document.organization_id
		 AND job.project_id = document.project_id
		 AND job.kind = 'knowledge.document.parse'
		 AND JSON_UNQUOTE(JSON_EXTRACT(job.payload, '$.document_id')) = document.id
		WHERE document.status IN ('parse_queued', 'parsing')
		  AND job.status IN ('failed', 'cancelled')
		  AND job.updated_at >= document.updated_at
		  AND NOT EXISTS (
			SELECT 1 FROM platform_jobs AS newer
			WHERE newer.organization_id = job.organization_id
			  AND newer.project_id = job.project_id
			  AND newer.kind = job.kind
			  AND JSON_UNQUOTE(JSON_EXTRACT(newer.payload, '$.document_id')) = document.id
			  AND (newer.created_at > job.created_at
				OR (newer.created_at = job.created_at AND newer.id > job.id))
		  )
		ORDER BY job.updated_at, job.id LIMIT 1`).Scan(
		&value.OrganizationID, &value.ProjectID, &value.ResourceID, &value.Status,
		&value.ErrorCode, &value.ErrorMessage, &value.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	status, code, message := documentTerminalState(value)
	result, err := s.DB.ExecContext(ctx, `UPDATE platform_knowledge_documents
		SET status = ?, parse_error_code = ?, parse_error_message = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?
		  AND status IN ('parse_queued', 'parsing') AND updated_at <= ?`,
		status, code, message, value.UpdatedAt,
		value.OrganizationID, value.ProjectID, value.ResourceID, value.UpdatedAt,
	)
	if err != nil {
		return false, err
	}
	changed, err := result.RowsAffected()
	return changed > 0, err
}

func (s Service) reconcileOneResearchJob(ctx context.Context) (bool, error) {
	var value terminalKnowledgeJob
	err := s.DB.QueryRowContext(ctx, `SELECT run.organization_id, run.project_id,
		run.id, job.status, COALESCE(job.error_code, ''),
		COALESCE(job.error_message, ''), job.updated_at
		FROM platform_research_runs AS run
		INNER JOIN platform_jobs AS job
		  ON job.organization_id = run.organization_id
		 AND job.project_id = run.project_id
		 AND job.kind = 'knowledge.research.execute'
		 AND JSON_UNQUOTE(JSON_EXTRACT(job.payload, '$.research_run_id')) = run.id
		WHERE run.status IN ('queued', 'planning', 'searching', 'reading', 'cross_checking', 'drafting', 'auditing')
		  AND job.status IN ('failed', 'cancelled')
		  AND job.updated_at >= run.updated_at
		  AND NOT EXISTS (
			SELECT 1 FROM platform_jobs AS newer
			WHERE newer.organization_id = job.organization_id
			  AND newer.project_id = job.project_id
			  AND newer.kind = job.kind
			  AND JSON_UNQUOTE(JSON_EXTRACT(newer.payload, '$.research_run_id')) = run.id
			  AND (newer.created_at > job.created_at
				OR (newer.created_at = job.created_at AND newer.id > job.id))
		  )
		ORDER BY job.updated_at, job.id LIMIT 1`).Scan(
		&value.OrganizationID, &value.ProjectID, &value.ResourceID, &value.Status,
		&value.ErrorCode, &value.ErrorMessage, &value.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	status, code, message := researchTerminalState(value)
	result, err := s.DB.ExecContext(ctx, `UPDATE platform_research_runs
		SET status = ?, error_code = ?, error_message = ?, stop_reason = ?,
			heartbeat_at = ?, completed_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?
		  AND status IN ('queued', 'planning', 'searching', 'reading', 'cross_checking', 'drafting', 'auditing')
		  AND updated_at <= ?`,
		status, code, message, strings.ToLower(code), value.UpdatedAt, value.UpdatedAt, value.UpdatedAt,
		value.OrganizationID, value.ProjectID, value.ResourceID, value.UpdatedAt,
	)
	if err != nil {
		return false, err
	}
	changed, err := result.RowsAffected()
	return changed > 0, err
}

func documentTerminalState(job terminalKnowledgeJob) (status, code, message string) {
	if job.Status == contract.JobCancelled {
		return "parse_failed", "DOCUMENT_PARSE_CANCELLED", "文档解析已取消"
	}
	code = strings.TrimSpace(job.ErrorCode)
	if code == "" {
		code = "DOCUMENT_PARSE_JOB_FAILED"
	}
	message = terminalJobMessage(job.ErrorMessage, "文档解析任务未能完成")
	return "parse_failed", code, message
}

func researchTerminalState(job terminalKnowledgeJob) (status, code, message string) {
	if job.Status == contract.JobCancelled {
		return "cancelled", "RESEARCH_CANCELLED", "研究任务已取消"
	}
	code = strings.TrimSpace(job.ErrorCode)
	if code == "" {
		code = "RESEARCH_JOB_FAILED"
	}
	message = terminalJobMessage(job.ErrorMessage, "研究任务未能完成")
	return "failed", code, message
}

func terminalJobMessage(message, fallback string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		message = fallback
	}
	if len(message) > 1024 {
		message = message[:1024]
	}
	return message
}
