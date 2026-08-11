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

const DocumentVisionFallbackJobKind = "knowledge.document.vision_fallback"

type JobRuntimeDocumentVisionFallbackScheduler struct {
	Store researchJobStore
	NewID func() (string, error)
	Now   func() time.Time
}

func (s JobRuntimeDocumentVisionFallbackScheduler) ScheduleDocumentVisionFallback(
	ctx context.Context,
	document Document,
	pages []int,
	scheduleKey string,
) error {
	if s.Store == nil || s.NewID == nil {
		return fmt.Errorf("document vision job store and ID generator are required")
	}
	jobID, err := s.NewID()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		DocumentID  string `json:"document_id"`
		AttemptID   string `json:"attempt_id"`
		PageNumbers []int  `json:"page_numbers"`
		ScheduleKey string `json:"schedule_key,omitempty"`
	}{DocumentID: document.ID, AttemptID: document.VisionAttemptID, PageNumbers: pages, ScheduleKey: scheduleKey})
	if err != nil {
		return err
	}
	digest := sha256.Sum256(payload)
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	keyDigest := sha256.Sum256([]byte(document.ID + "|" + document.VisionAttemptID + "|" + scheduleKey))
	_, _, err = s.Store.Enqueue(ctx, jobruntime.CreateRequest{
		Job: contract.Job{
			ID: jobID, Kind: DocumentVisionFallbackJobKind,
			OrganizationID: document.OrganizationID, ProjectID: document.ProjectID,
			Status: contract.JobQueued, Progress: 72, Cancellable: true,
			AttemptCount: 0, MaxAttempts: 1000, Version: 1,
			CreatedAt: now().UTC(), UpdatedAt: now().UTC(),
		},
		Payload: payload, IdempotencyKey: contract.IdempotencyKey("knowledge_vision_" + hex.EncodeToString(keyDigest[:])),
		RequestHash: hex.EncodeToString(digest[:]),
	})
	return err
}

func (s Service) HandleDocumentVisionFallbackJob(ctx context.Context, claim jobruntime.Claim) (jobruntime.Result, error) {
	if claim.Job.Kind != DocumentVisionFallbackJobKind {
		return jobruntime.Result{}, fmt.Errorf("unsupported document vision job kind %q", claim.Job.Kind)
	}
	var payload struct {
		DocumentID  string `json:"document_id"`
		AttemptID   string `json:"attempt_id"`
		PageNumbers []int  `json:"page_numbers"`
	}
	if json.Unmarshal(claim.Payload, &payload) != nil || strings.TrimSpace(payload.DocumentID) == "" || strings.TrimSpace(payload.AttemptID) == "" {
		return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{
			Code: "INVALID_DOCUMENT_VISION_JOB", Message: "Document vision job payload is invalid", Retryable: false,
		}}
	}
	ref := contract.ResourceRef{Type: "knowledge_document", ID: payload.DocumentID}
	document, err := scanDocument(s.DB.QueryRowContext(ctx, documentSelect+`
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		claim.Job.OrganizationID, claim.Job.ProjectID, payload.DocumentID,
	))
	if err != nil {
		return jobruntime.Result{}, err
	}
	if document.VisionFallbackStatus == "succeeded" {
		return jobruntime.Result{Ref: &ref}, nil
	}
	if document.VisionAttemptID != payload.AttemptID {
		return jobruntime.Result{Ref: &ref}, nil
	}
	conversionActive := documentVisionNeedsConversion(document.MIMEType) && document.ParsePhase != "visual_fallback"
	pages, err := normalizeVisionPageSelection(payload.PageNumbers, document.TotalPages)
	if err != nil || s.DocumentVision == nil || s.Blobs == nil {
		s.markDocumentVisionFailed(ctx, document, "DOCUMENT_VISION_UNAVAILABLE", "Document vision parser is unavailable")
		return jobruntime.Result{Ref: &ref}, nil
	}
	now := s.now()
	initialPhase := "visual_fallback"
	initialProgress := 72
	if conversionActive {
		initialPhase = "visual_conversion"
		initialProgress = 73
	}
	if _, err := s.DB.ExecContext(ctx, `UPDATE platform_knowledge_documents
		SET status = 'parsing', parse_strategy = 'hybrid', parse_phase = ?,
			parse_progress = GREATEST(COALESCE(parse_progress, ?), ?), progress_kind = 'pages',
			processed_pages = COALESCE(processed_pages, 0),
			vision_fallback_status = 'running', vision_started_at = COALESCE(vision_started_at, ?),
			heartbeat_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		initialPhase, initialProgress, initialProgress, now, now, now,
		document.OrganizationID, document.ProjectID, document.ID,
	); err != nil {
		return jobruntime.Result{}, err
	}
	_, _ = s.DB.ExecContext(ctx, `UPDATE platform_knowledge_document_pages
		SET status = 'processing', updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND status = 'selected'
		  AND page_number IN (`+placeholders(len(pages))+`)`,
		append([]any{now, document.OrganizationID, document.ProjectID, document.ID}, intsToAny(pages)...)...,
	)
	if err := s.reportJobProgress(ctx, claim, max(72, claim.Job.Progress), "Selected pages entered visual parsing"); err != nil {
		return jobruntime.Result{}, err
	}
	tasks, err := s.loadDocumentVisionTasks(ctx, document, payload.AttemptID)
	if err != nil {
		return jobruntime.Result{}, fmt.Errorf("load document vision task checkpoints: %w", err)
	}
	if len(tasks) == 0 {
		s.markDocumentVisionFailed(ctx, document, "DOCUMENT_VISION_TASKS_MISSING", "Document visual fallback task checkpoints are missing")
		return jobruntime.Result{}, executionDocumentVisionError("DOCUMENT_VISION_TASKS_MISSING", "Document visual fallback task checkpoints are missing")
	}
	for _, task := range tasks {
		if task.Status == "submitting" || task.Status == "unknown" {
			_ = s.markDocumentVisionTaskUnknown(ctx, document, task, "DOCUMENT_VISION_SUBMISSION_UNKNOWN", "The external task may have been accepted, so cookies will not submit it again automatically")
			s.markDocumentVisionFailed(ctx, document, "DOCUMENT_VISION_SUBMISSION_UNKNOWN", "外部解析任务状态未知；为避免重复计费，系统没有自动重新提交。已有文本仍可使用。")
			return jobruntime.Result{}, executionDocumentVisionError("DOCUMENT_VISION_SUBMISSION_UNKNOWN", "Document vision submission state is unknown")
		}
		if task.Status == "failed" {
			s.markDocumentVisionFailed(ctx, document, task.ErrorCode, task.ErrorMessage)
			return jobruntime.Result{}, executionDocumentVisionError(task.ErrorCode, task.ErrorMessage)
		}
	}
	if conversionActive {
		if err := s.reportJobProgress(ctx, claim, max(73, claim.Job.Progress), "Converting presentation to a traceable PDF input"); err != nil {
			return jobruntime.Result{}, err
		}
	}
	visionInput, err := s.resolveDocumentVisionInput(ctx, document, payload.AttemptID, claim.Job.ID)
	if err != nil {
		if conversionError, ok := AsDocumentVisionInputConversionError(err); ok {
			if conversionError.Retryable {
				s.recordDocumentVisionRetry(ctx, document, conversionError)
				return jobruntime.Result{}, deferDocumentVision(s.now(), 2*time.Second)
			}
			s.markDocumentVisionInputConversionFailed(ctx, document, payload.AttemptID, conversionError.Code, conversionError.Message)
			s.markDocumentVisionFailed(ctx, document, conversionError.Code, conversionError.Message)
			return jobruntime.Result{}, executionDocumentVisionError(conversionError.Code, conversionError.Message)
		}
		return jobruntime.Result{}, err
	}
	if conversionActive {
		visualNow := s.now()
		if _, err := s.DB.ExecContext(ctx, `UPDATE platform_knowledge_documents
			SET parse_phase = 'visual_fallback', parse_progress = GREATEST(COALESCE(parse_progress, 73), 74),
				heartbeat_at = ?, updated_at = ?
			WHERE organization_id = ? AND project_id = ? AND id = ? AND vision_attempt_id = ?`,
			visualNow, visualNow, document.OrganizationID, document.ProjectID, document.ID, payload.AttemptID,
		); err != nil {
			return jobruntime.Result{}, err
		}
		if err := s.reportJobProgress(ctx, claim, max(74, claim.Job.Progress), "Presentation PDF is ready; starting selected visual pages"); err != nil {
			return jobruntime.Result{}, err
		}
	}
	for _, task := range tasks {
		if task.Status != "prepared" {
			continue
		}
		stream, info, openErr := s.Blobs.Open(ctx, visionInput.Location)
		if openErr != nil {
			_ = s.markDocumentVisionTaskFailed(ctx, document, task, "DOCUMENT_VISION_SOURCE_UNAVAILABLE", "Document source is unavailable", nil)
			s.markDocumentVisionFailed(ctx, document, "DOCUMENT_VISION_SOURCE_UNAVAILABLE", "Document source is unavailable")
			return jobruntime.Result{}, executionDocumentVisionError("DOCUMENT_VISION_SOURCE_UNAVAILABLE", "Document vision source is unavailable")
		}
		if !documentVisionObjectIdentityMatches(info.ObjectLocation, visionInput.Location) {
			_ = stream.Close()
			_ = s.markDocumentVisionTaskFailed(ctx, document, task, "DOCUMENT_VISION_SOURCE_SCOPE_INVALID", "Document vision source no longer matches its immutable storage lineage", nil)
			s.markDocumentVisionFailed(ctx, document, "DOCUMENT_VISION_SOURCE_SCOPE_INVALID", "Document vision source no longer matches its immutable storage lineage")
			return jobruntime.Result{}, executionDocumentVisionError("DOCUMENT_VISION_SOURCE_SCOPE_INVALID", "Document vision source storage lineage is invalid")
		}
		parseRequest := DocumentVisionParseRequest{
			OrganizationID: document.OrganizationID, ProjectID: document.ProjectID, DocumentID: document.ID,
			Filename: visionInput.Filename, MIMEType: visionInput.MIMEType, SizeBytes: info.SizeBytes,
			Source: stream, Object: info.ObjectLocation, PageNumbers: task.PageNumbers, ModelAlias: document.VisionModelAlias,
		}
		intent, prepareErr := s.DocumentVision.PrepareSubmission(ctx, parseRequest)
		if prepareErr != nil {
			_ = stream.Close()
			return jobruntime.Result{}, prepareErr
		}
		if err := validateDocumentVisionSubmissionIntent(intent, document.VisionRouteRevision); err != nil {
			_ = stream.Close()
			_ = s.markDocumentVisionTaskFailed(ctx, document, task, "DOCUMENT_VISION_INTENT_INVALID", err.Error(), nil)
			s.markDocumentVisionFailed(ctx, document, "DOCUMENT_VISION_INTENT_INVALID", "Document visual fallback could not freeze a valid submission intent")
			return jobruntime.Result{}, executionDocumentVisionError("DOCUMENT_VISION_INTENT_INVALID", "Document vision submission intent is invalid")
		}
		claimed, claimErr := s.persistDocumentVisionSubmissionIntent(ctx, document, task, intent)
		if claimErr != nil {
			_ = stream.Close()
			return jobruntime.Result{}, claimErr
		}
		if !claimed {
			_ = stream.Close()
			return jobruntime.Result{}, deferDocumentVision(s.now(), time.Second)
		}
		submission, submitErr := s.DocumentVision.SubmitPrepared(ctx, parseRequest, intent)
		_ = stream.Close()
		if submitErr != nil {
			_ = s.markDocumentVisionTaskUnknown(ctx, document, task, "DOCUMENT_VISION_SUBMISSION_UNKNOWN", boundedVisionError(submitErr))
			s.markDocumentVisionFailed(ctx, document, "DOCUMENT_VISION_SUBMISSION_UNKNOWN", "外部解析任务提交结果未知；为避免重复计费，系统没有自动重新提交。已有文本仍可使用。")
			return jobruntime.Result{}, executionDocumentVisionError("DOCUMENT_VISION_SUBMISSION_UNKNOWN", "Document vision submission result is unknown")
		}
		if err := validateDocumentVisionSubmission(submission, intent); err != nil {
			_ = s.markDocumentVisionTaskUnknown(ctx, document, task, "DOCUMENT_VISION_SUBMISSION_INVALID", err.Error())
			s.markDocumentVisionFailed(ctx, document, "DOCUMENT_VISION_SUBMISSION_INVALID", "External document vision submission returned an invalid checkpoint")
			return jobruntime.Result{}, executionDocumentVisionError("DOCUMENT_VISION_SUBMISSION_INVALID", "Document vision submission checkpoint is invalid")
		}
		if err := s.persistDocumentVisionSubmission(ctx, document, task, submission); err != nil {
			s.markDocumentVisionFailed(ctx, document, "DOCUMENT_VISION_CHECKPOINT_FAILED", "External task was accepted but its checkpoint could not be persisted; automatic resubmission is disabled")
			return jobruntime.Result{}, executionDocumentVisionError("DOCUMENT_VISION_CHECKPOINT_FAILED", "Document vision checkpoint could not be persisted")
		}
		return jobruntime.Result{}, deferDocumentVision(s.now(), submission.PollAfter)
	}
	for _, task := range tasks {
		if task.Status != "submitted" && task.Status != "running" {
			continue
		}
		poll, pollErr := s.DocumentVision.Poll(ctx, task.submission())
		if pollErr != nil {
			s.recordDocumentVisionRetry(ctx, document, pollErr)
			return jobruntime.Result{}, deferDocumentVision(s.now(), 2*time.Second)
		}
		switch poll.Status {
		case DocumentVisionPollPending, DocumentVisionPollRunning:
			if err := s.persistDocumentVisionPoll(ctx, document, task, poll); err != nil {
				return jobruntime.Result{}, err
			}
			return jobruntime.Result{}, deferDocumentVision(s.now(), poll.PollAfter)
		case DocumentVisionPollFailed:
			code := strings.TrimSpace(poll.ErrorCode)
			if code == "" {
				code = "DOCUMENT_VISION_UPSTREAM_FAILED"
			}
			message := boundedVisionMessage(poll.ErrorMessage)
			if message == "" {
				message = "External document vision task failed"
			}
			_ = s.markDocumentVisionTaskFailed(ctx, document, task, code, message, poll.BillablePages)
			s.markDocumentVisionFailed(ctx, document, code, message)
			return jobruntime.Result{}, executionDocumentVisionError(code, message)
		case DocumentVisionPollCompleted:
			if poll.Result == nil {
				return jobruntime.Result{}, executionDocumentVisionError("DOCUMENT_VISION_RESPONSE_INVALID", "Completed document vision task returned no result")
			}
			result := *poll.Result
			if result.RouteRevisionID == "" {
				result.RouteRevisionID = task.RouteRevisionID
			}
			if result.RouteRevisionID != document.VisionRouteRevision || result.RouteRevisionID != task.RouteRevisionID {
				s.markDocumentVisionFailed(ctx, document, "DOCUMENT_VISION_ROUTE_MISMATCH", "Document visual fallback returned an unexpected model route")
				return jobruntime.Result{}, executionDocumentVisionError("DOCUMENT_VISION_ROUTE_MISMATCH", "Document vision route changed during execution")
			}
			if err := validateDocumentVisionResult(result, task.PageNumbers); err != nil {
				s.markDocumentVisionFailed(ctx, document, "DOCUMENT_VISION_RESPONSE_INVALID", err.Error())
				return jobruntime.Result{}, executionDocumentVisionError("DOCUMENT_VISION_RESPONSE_INVALID", "Document visual fallback returned an invalid result")
			}
			if err := s.persistDocumentVisionTaskResult(ctx, document, task, result, poll.BillablePages); err != nil {
				s.markDocumentVisionFailed(ctx, document, "DOCUMENT_VISION_PERSIST_FAILED", "Visual page results could not be persisted")
				return jobruntime.Result{}, err
			}
			progress, progressErr := s.documentVisionAttemptProgress(ctx, document, pages)
			if progressErr != nil {
				return jobruntime.Result{}, progressErr
			}
			if err := s.reportJobProgress(ctx, claim, progress, "Visual pages are ready; continuing the selected page ranges"); err != nil {
				return jobruntime.Result{}, err
			}
			return jobruntime.Result{}, deferDocumentVision(s.now(), 500*time.Millisecond)
		default:
			return jobruntime.Result{}, executionDocumentVisionError("DOCUMENT_VISION_RESPONSE_INVALID", "Document vision poll status is invalid")
		}
	}
	if err := s.reportJobProgress(ctx, claim, 92, "Visual pages are ready; merging traceable locators"); err != nil {
		return jobruntime.Result{}, err
	}
	if err := s.finalizeDocumentVisionAttempt(ctx, document, pages, payload.AttemptID); err != nil {
		s.markDocumentVisionFailed(ctx, document, "DOCUMENT_VISION_PERSIST_FAILED", "Visual page results could not be finalized")
		return jobruntime.Result{}, err
	}
	return jobruntime.Result{Ref: &ref}, nil
}

func visionChunk(document Document, result DocumentVisionParseResult, page DocumentVisionPage, text string) Chunk {
	textDigest := sha256.Sum256([]byte(text))
	textHash := hex.EncodeToString(textDigest[:])
	idDigest := sha256.Sum256([]byte(fmt.Sprintf("%s|vision|%d|%s|%s", document.ID, page.PageNumber, result.ModelVersion, textHash)))
	pageNumber := page.PageNumber
	return Chunk{
		ID: "knowledgechunk_" + hex.EncodeToString(idDigest[:24]), DocumentID: document.ID,
		OrganizationID: document.OrganizationID, ProjectID: document.ProjectID,
		Index: 1_000_000 + page.PageNumber, Kind: "vision_markdown", Text: text,
		SourceURI: document.SourceURI, Section: fmt.Sprintf("视觉解析补充 · 第 %d 页", page.PageNumber),
		PageNumber: &pageNumber, StartLine: 1, EndLine: max(1, strings.Count(text, "\n")+1),
		TextSHA256: textHash, Locator: page.Locator, ParserCode: "document_vision",
		ParserVersion: result.ModelVersion, CreatedAt: document.UpdatedAt,
	}
}

func (s Service) recordDocumentVisionRetry(ctx context.Context, document Document, err error) {
	message := boundedVisionError(err)
	now := s.now()
	_, _ = s.DB.ExecContext(ctx, `UPDATE platform_knowledge_documents
		SET vision_fallback_status = 'running', vision_error_code = 'DOCUMENT_VISION_RETRYING',
			vision_error_message = ?, heartbeat_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		message, now, now, document.OrganizationID, document.ProjectID, document.ID)
}

func (s Service) markDocumentVisionFailed(ctx context.Context, document Document, code, message string) bool {
	message = boundedVisionMessage(message)
	now := s.now()
	result, updateErr := s.DB.ExecContext(ctx, `UPDATE platform_knowledge_documents
		SET status = CASE WHEN chunk_count > 0 THEN 'partial' ELSE status END,
			parse_phase = CASE WHEN chunk_count > 0 THEN 'partial' ELSE parse_phase END,
			parse_progress = CASE WHEN chunk_count > 0 THEN 100 ELSE parse_progress END,
			preview_status = CASE WHEN chunk_count > 0 THEN 'partial' ELSE preview_status END,
			vision_fallback_status = 'failed', vision_error_code = ?, vision_error_message = ?,
			vision_completed_at = ?, heartbeat_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?
		  AND vision_fallback_status IN ('queued', 'running')`,
		code, message, now, now, now, document.OrganizationID, document.ProjectID, document.ID)
	if updateErr != nil {
		return false
	}
	changed, changedErr := result.RowsAffected()
	if changedErr != nil || changed == 0 {
		return false
	}
	_, _ = s.DB.ExecContext(ctx, `UPDATE platform_knowledge_document_pages
		SET status = 'failed', error_code = ?, error_message = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND status IN ('selected', 'processing')`,
		code, message, now, document.OrganizationID, document.ProjectID, document.ID)
	duration := now.Sub(document.UpdatedAt)
	if document.VisionStartedAt != nil {
		duration = now.Sub(*document.VisionStartedAt)
	}
	s.recordDocumentEvent(ctx, document, DocumentEventVisionFallback, "failed", "partial", &duration)
	return true
}

func boundedVisionError(err error) string {
	if err == nil {
		return "Document visual fallback is retrying"
	}
	return boundedVisionMessage(err.Error())
}

func boundedVisionMessage(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 1024 {
		value = value[:1024]
	}
	return value
}

func placeholders(count int) string {
	if count < 1 {
		return "NULL"
	}
	return strings.TrimRight(strings.Repeat("?,", count), ",")
}

func intsToAny(values []int) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}
