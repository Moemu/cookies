package knowledge

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
)

const maxDocumentVisionCheckpointBytes = 32 * 1024

func (task documentVisionTask) submission() DocumentVisionSubmission {
	return DocumentVisionSubmission{
		ExternalTaskID: task.ExternalTaskID, ProviderCode: task.ProviderCode,
		ModelVersion: task.ModelVersion, RouteRevisionID: task.RouteRevisionID,
		Checkpoint: append(json.RawMessage(nil), task.Checkpoint...),
	}
}

func validateDocumentVisionSubmissionIntent(value DocumentVisionSubmissionIntent, expectedRouteRevision string) error {
	if len(value.IntentID) != sha256.Size*2 {
		return fmt.Errorf("document vision submission intent id is invalid")
	}
	if _, err := hex.DecodeString(value.IntentID); err != nil || strings.ToLower(value.IntentID) != value.IntentID {
		return fmt.Errorf("document vision submission intent id is invalid")
	}
	if strings.TrimSpace(value.ProviderCode) == "" || len(value.ProviderCode) > 64 ||
		strings.TrimSpace(value.ModelVersion) == "" || len(value.ModelVersion) > 160 ||
		strings.TrimSpace(value.RouteRevisionID) == "" || value.RouteRevisionID != expectedRouteRevision {
		return fmt.Errorf("document vision submission intent identity is invalid")
	}
	if len(value.Checkpoint) == 0 || len(value.Checkpoint) > maxDocumentVisionCheckpointBytes || !json.Valid(value.Checkpoint) {
		return fmt.Errorf("document vision submission intent checkpoint is invalid")
	}
	return nil
}

func validateDocumentVisionSubmission(value DocumentVisionSubmission, intent DocumentVisionSubmissionIntent) error {
	if strings.TrimSpace(value.ExternalTaskID) == "" || len(value.ExternalTaskID) > 160 ||
		value.ProviderCode != intent.ProviderCode || value.ModelVersion != intent.ModelVersion ||
		value.RouteRevisionID != intent.RouteRevisionID {
		return fmt.Errorf("document vision submission identity is invalid")
	}
	if !jsonEqual(value.Checkpoint, intent.Checkpoint) {
		return fmt.Errorf("document vision submission checkpoint is invalid")
	}
	if value.PollAfter < 0 || value.PollAfter > 10*time.Second {
		return fmt.Errorf("document vision submission polling delay is invalid")
	}
	return nil
}

func jsonEqual(left, right json.RawMessage) bool {
	var leftValue, rightValue any
	return json.Unmarshal(left, &leftValue) == nil && json.Unmarshal(right, &rightValue) == nil &&
		reflect.DeepEqual(leftValue, rightValue)
}

func deferDocumentVision(now time.Time, after time.Duration) jobruntime.DeferredError {
	if after < 500*time.Millisecond {
		after = 500 * time.Millisecond
	}
	if after > 10*time.Second {
		after = 10 * time.Second
	}
	return jobruntime.DeferredError{AvailableAt: now.Add(after)}
}

func executionDocumentVisionError(code, message string) jobruntime.ExecutionError {
	code = strings.TrimSpace(code)
	if code == "" {
		code = "DOCUMENT_VISION_FAILED"
	}
	message = boundedVisionMessage(message)
	if message == "" {
		message = "Document visual fallback failed"
	}
	return jobruntime.ExecutionError{JobError: contract.JobError{Code: code, Message: message, Retryable: false}}
}

func (s Service) loadDocumentVisionTasks(ctx context.Context, document Document, attemptID string) ([]documentVisionTask, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT attempt_id, task_index, page_numbers, status, intent_id,
		external_task_id, provider_code, model_alias, model_version, route_revision_id,
		checkpoint_json, billable_pages, error_code, error_message, created_at, updated_at
		FROM platform_knowledge_document_vision_tasks
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ?
		ORDER BY task_index`, document.OrganizationID, document.ProjectID, document.ID, attemptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []documentVisionTask{}
	for rows.Next() {
		var task documentVisionTask
		var encodedPages, checkpoint []byte
		var billable sql.NullInt64
		if err := rows.Scan(
			&task.AttemptID, &task.TaskIndex, &encodedPages, &task.Status, &task.IntentID,
			&task.ExternalTaskID, &task.ProviderCode, &task.ModelAlias, &task.ModelVersion,
			&task.RouteRevisionID, &checkpoint, &billable, &task.ErrorCode, &task.ErrorMessage,
			&task.CreatedAt, &task.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(encodedPages, &task.PageNumbers); err != nil || len(task.PageNumbers) == 0 {
			return nil, fmt.Errorf("document vision task page checkpoint is invalid")
		}
		if len(checkpoint) > 0 {
			task.Checkpoint = append(json.RawMessage(nil), checkpoint...)
		}
		if billable.Valid {
			value := int(billable.Int64)
			task.BillablePages = &value
		}
		result = append(result, task)
	}
	return result, rows.Err()
}

func (s Service) persistDocumentVisionSubmissionIntent(ctx context.Context, document Document, task documentVisionTask, intent DocumentVisionSubmissionIntent) (bool, error) {
	now := s.now()
	result, err := s.DB.ExecContext(ctx, `UPDATE platform_knowledge_document_vision_tasks
		SET status = 'submitting', intent_id = ?, provider_code = ?, model_version = ?,
			route_revision_id = ?, checkpoint_json = ?, error_code = '', error_message = '', updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ?
		  AND task_index = ? AND status = 'prepared'`,
		intent.IntentID, intent.ProviderCode, intent.ModelVersion, intent.RouteRevisionID, intent.Checkpoint, now,
		document.OrganizationID, document.ProjectID, document.ID, task.AttemptID, task.TaskIndex)
	if err != nil {
		return false, err
	}
	changed, err := result.RowsAffected()
	return changed == 1, err
}

func (s Service) persistDocumentVisionSubmission(ctx context.Context, document Document, task documentVisionTask, submission DocumentVisionSubmission) error {
	now := s.now()
	result, err := s.DB.ExecContext(ctx, `UPDATE platform_knowledge_document_vision_tasks
		SET status = 'submitted', external_task_id = ?, provider_code = ?, model_version = ?,
			route_revision_id = ?, checkpoint_json = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ?
		  AND task_index = ? AND status = 'submitting'`,
		submission.ExternalTaskID, submission.ProviderCode, submission.ModelVersion,
		submission.RouteRevisionID, submission.Checkpoint, now,
		document.OrganizationID, document.ProjectID, document.ID, task.AttemptID, task.TaskIndex)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return fmt.Errorf("document vision submission checkpoint lost its prepared state")
	}
	return nil
}

func (s Service) persistDocumentVisionPoll(ctx context.Context, document Document, task documentVisionTask, poll DocumentVisionPollResult) error {
	status := "running"
	if poll.Status == DocumentVisionPollPending {
		status = "submitted"
	}
	now := s.now()
	_, err := s.DB.ExecContext(ctx, `UPDATE platform_knowledge_document_vision_tasks
		SET status = ?, billable_pages = COALESCE(?, billable_pages), updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ?
		  AND task_index = ? AND status IN ('submitted', 'running')`,
		status, nullableIntPointer(poll.BillablePages), now,
		document.OrganizationID, document.ProjectID, document.ID, task.AttemptID, task.TaskIndex)
	if err == nil {
		_, _ = s.DB.ExecContext(ctx, `UPDATE platform_knowledge_documents SET heartbeat_at = ?, updated_at = ?
			WHERE organization_id = ? AND project_id = ? AND id = ? AND vision_attempt_id = ?`,
			now, now, document.OrganizationID, document.ProjectID, document.ID, task.AttemptID)
	}
	return err
}

func nullableIntPointer(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func (s Service) markDocumentVisionTaskUnknown(ctx context.Context, document Document, task documentVisionTask, code, message string) error {
	return s.updateDocumentVisionTaskFailure(ctx, document, task, "unknown", code, message, nil)
}

func (s Service) markDocumentVisionTaskFailed(ctx context.Context, document Document, task documentVisionTask, code, message string, billablePages *int) error {
	return s.updateDocumentVisionTaskFailure(ctx, document, task, "failed", code, message, billablePages)
}

func (s Service) updateDocumentVisionTaskFailure(ctx context.Context, document Document, task documentVisionTask, status, code, message string, billablePages *int) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE platform_knowledge_document_vision_tasks
		SET status = ?, billable_pages = COALESCE(?, billable_pages), error_code = ?, error_message = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ? AND task_index = ?`,
		status, nullableIntPointer(billablePages), strings.TrimSpace(code), boundedVisionMessage(message), s.now(),
		document.OrganizationID, document.ProjectID, document.ID, task.AttemptID, task.TaskIndex)
	return err
}

func (s Service) persistDocumentVisionTaskResult(
	ctx context.Context,
	document Document,
	task documentVisionTask,
	result DocumentVisionParseResult,
	billablePages *int,
) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := s.now()
	for _, page := range result.Pages {
		text := strings.TrimSpace(page.Markdown)
		locator, err := sanitizeDocumentVisionLocator(page.PageNumber, page.Locator)
		if err != nil {
			return err
		}
		page.Locator = locator
		chunk := visionChunk(document, result, page, text)
		pageUpdate, err := tx.ExecContext(ctx, `UPDATE platform_knowledge_document_pages
			SET status = 'ready', text_origin = 'vision', vision_text = ?, merged_text = ?,
				text_sha256 = ?, locator_json = ?, provider_code = ?, model_version = ?,
				route_revision_id = ?, latency_ms = ?, usage_json = ?, error_code = '',
				error_message = '', updated_at = ?
			WHERE organization_id = ? AND project_id = ? AND document_id = ? AND page_number = ?`,
			page.Markdown, page.Markdown, chunk.TextSHA256, jsonBytes(page.Locator), result.ProviderCode,
			result.ModelVersion, result.RouteRevisionID, result.Latency.Milliseconds(), nullableJSON(result.Usage), now,
			document.OrganizationID, document.ProjectID, document.ID, page.PageNumber,
		)
		if err != nil {
			return err
		}
		changed, err := pageUpdate.RowsAffected()
		if err != nil || changed != 1 {
			return fmt.Errorf("document vision page %d lost its selected checkpoint", page.PageNumber)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO platform_knowledge_chunks
			(id, organization_id, project_id, document_id, ordinal, kind, section,
			 page_number, start_line, end_line, text, text_sha256, locator_json,
			 parser_code, parser_version, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE text = VALUES(text), text_sha256 = VALUES(text_sha256),
				locator_json = VALUES(locator_json), parser_version = VALUES(parser_version)`,
			chunk.ID, chunk.OrganizationID, chunk.ProjectID, chunk.DocumentID, chunk.Index,
			chunk.Kind, chunk.Section, chunk.PageNumber, chunk.StartLine, chunk.EndLine,
			chunk.Text, chunk.TextSHA256, jsonBytes(chunk.Locator), chunk.ParserCode,
			chunk.ParserVersion, chunk.CreatedAt,
		); err != nil {
			return err
		}
	}
	taskUpdate, err := tx.ExecContext(ctx, `UPDATE platform_knowledge_document_vision_tasks
		SET status = 'completed', billable_pages = COALESCE(?, billable_pages),
			provider_code = ?, model_version = ?, route_revision_id = ?, error_code = '', error_message = '', updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ?
		  AND task_index = ? AND status IN ('submitted', 'running')`,
		nullableIntPointer(billablePages), result.ProviderCode, result.ModelVersion, result.RouteRevisionID, now,
		document.OrganizationID, document.ProjectID, document.ID, task.AttemptID, task.TaskIndex,
	)
	if err != nil {
		return err
	}
	changed, err := taskUpdate.RowsAffected()
	if err != nil || changed != 1 {
		return fmt.Errorf("document vision task lost its submitted checkpoint")
	}
	completed, err := readyDocumentVisionPages(ctx, tx, document)
	if err != nil {
		return err
	}
	completedJSON, _ := json.Marshal(completed)
	progress := 72
	if len(document.VisionSelectedPages) > 0 {
		progress += min(20, 20*len(completed)/len(document.VisionSelectedPages))
	}
	if _, err := tx.ExecContext(ctx, `UPDATE platform_knowledge_documents
		SET vision_completed_pages = ?, processed_pages = ?, parse_progress = ?,
			vision_model_version = ?, heartbeat_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND vision_attempt_id = ?`,
		completedJSON, len(completed), progress, result.ModelVersion, now, now,
		document.OrganizationID, document.ProjectID, document.ID, task.AttemptID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

type visionPageQuery interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func readyDocumentVisionPages(ctx context.Context, query visionPageQuery, document Document) ([]int, error) {
	rows, err := query.QueryContext(ctx, `SELECT page_number FROM platform_knowledge_document_pages
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND status = 'ready'
		ORDER BY page_number`, document.OrganizationID, document.ProjectID, document.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []int{}
	for rows.Next() {
		var page int
		if err := rows.Scan(&page); err != nil {
			return nil, err
		}
		result = append(result, page)
	}
	return result, rows.Err()
}

func (s Service) documentVisionAttemptProgress(ctx context.Context, document Document, selectedPages []int) (int, error) {
	var completed int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM platform_knowledge_document_pages
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND status = 'ready'
		  AND page_number IN (`+placeholders(len(selectedPages))+`)`,
		append([]any{document.OrganizationID, document.ProjectID, document.ID}, intsToAny(selectedPages)...)...).Scan(&completed)
	if err != nil {
		return 0, err
	}
	if len(selectedPages) == 0 {
		return 72, nil
	}
	return 72 + min(20, 20*completed/len(selectedPages)), nil
}

func (s Service) finalizeDocumentVisionAttempt(ctx context.Context, document Document, selectedPages []int, attemptID string) error {
	var remaining int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM platform_knowledge_document_vision_tasks
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ?
		  AND status <> 'completed'`, document.OrganizationID, document.ProjectID, document.ID, attemptID).Scan(&remaining); err != nil {
		return err
	}
	if remaining != 0 {
		return fmt.Errorf("document vision attempt still has unfinished external tasks")
	}
	type pageResult struct {
		PageNumber      int
		Markdown        string
		Locator         map[string]any
		ProviderCode    string
		ModelVersion    string
		RouteRevisionID string
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT page_number, COALESCE(vision_text, ''), locator_json,
		provider_code, model_version, route_revision_id
		FROM platform_knowledge_document_pages
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND status = 'ready'
		  AND page_number IN (`+placeholders(len(selectedPages))+`)
		ORDER BY page_number`, append([]any{document.OrganizationID, document.ProjectID, document.ID}, intsToAny(selectedPages)...)...)
	if err != nil {
		return err
	}
	pageResults := []pageResult{}
	for rows.Next() {
		var page pageResult
		var locator []byte
		if err := rows.Scan(&page.PageNumber, &page.Markdown, &locator, &page.ProviderCode, &page.ModelVersion, &page.RouteRevisionID); err != nil {
			rows.Close()
			return err
		}
		if err := json.Unmarshal(locator, &page.Locator); err != nil {
			rows.Close()
			return err
		}
		pageResults = append(pageResults, page)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	sort.Slice(pageResults, func(left, right int) bool { return pageResults[left].PageNumber < pageResults[right].PageNumber })
	completed := make([]int, 0, len(pageResults))
	visionChunks := make([]Chunk, 0, len(pageResults))
	var additions strings.Builder
	modelVersion := document.VisionModelVersion
	for _, page := range pageResults {
		completed = append(completed, page.PageNumber)
		additions.WriteString(fmt.Sprintf("\n\n## 视觉解析补充 · 第 %d 页\n%s", page.PageNumber, strings.TrimSpace(page.Markdown)))
		modelVersion = page.ModelVersion
		visionChunks = append(visionChunks, visionChunk(document, DocumentVisionParseResult{
			ProviderCode: page.ProviderCode, ModelVersion: page.ModelVersion, RouteRevisionID: page.RouteRevisionID,
		}, DocumentVisionPage{PageNumber: page.PageNumber, Markdown: page.Markdown, Locator: page.Locator}, strings.TrimSpace(page.Markdown)))
	}
	combinedText := strings.TrimSpace(document.ExtractedText + additions.String())
	allPagesCovered := document.TotalPages != nil && len(completed) == *document.TotalPages && len(selectedPages) == *document.TotalPages
	visionStatus := "succeeded"
	if len(completed) < len(selectedPages) {
		visionStatus = "partial"
	}
	qualityTier := document.QualityTier
	qualityScore := document.QualityScore
	qualitySummary := document.PageQualitySummary
	status, phase, previewStatus := "partial", "partial", "partial"
	if allPagesCovered {
		quality := evaluateParsedDocumentQuality(ParsedDocument{
			Text: combinedText, MIMEType: document.MIMEType, ParserCode: "hybrid", ParserVersion: modelVersion,
		}, visionChunks)
		qualityTier, qualityScore = quality.Tier, &quality.Score
		qualitySummary, _ = json.Marshal(quality.Summary)
		if quality.Tier != "low" {
			status, phase, previewStatus = "ready", "ready", "ready"
		}
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := s.now()
	if visionStatus == "partial" {
		if _, err := tx.ExecContext(ctx, `UPDATE platform_knowledge_document_pages
			SET status = 'failed', error_code = 'DOCUMENT_VISION_PAGE_MISSING',
				error_message = '视觉解析未返回这一页，已有文本结果仍被保留', updated_at = ?
			WHERE organization_id = ? AND project_id = ? AND document_id = ?
			  AND status IN ('selected', 'processing')`,
			now, document.OrganizationID, document.ProjectID, document.ID); err != nil {
			return err
		}
	}
	textDigest := sha256.Sum256([]byte(combinedText))
	completedJSON, _ := json.Marshal(completed)
	if _, err := tx.ExecContext(ctx, `UPDATE platform_knowledge_documents
		SET extracted_text = ?, text_sha256 = ?,
			chunk_count = (SELECT COUNT(*) FROM platform_knowledge_chunks
				WHERE organization_id = ? AND project_id = ? AND document_id = ?),
			status = ?, parse_strategy = 'hybrid', parse_phase = ?, parse_progress = 100,
			progress_kind = 'pages', processed_pages = ?, quality_score = ?, quality_tier = ?,
			preview_status = ?, page_quality_summary = ?, vision_fallback_status = ?,
			vision_completed_pages = ?, vision_model_version = ?,
			vision_error_code = '', vision_error_message = '', vision_completed_at = ?,
			heartbeat_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ? AND vision_attempt_id = ?`,
		combinedText, hex.EncodeToString(textDigest[:]), document.OrganizationID, document.ProjectID, document.ID,
		status, phase, len(completed), qualityScore, qualityTier, previewStatus, nullableJSON(qualitySummary), visionStatus,
		completedJSON, modelVersion, now, now, now,
		document.OrganizationID, document.ProjectID, document.ID, attemptID,
	); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	duration := now.Sub(document.UpdatedAt)
	if document.VisionStartedAt != nil {
		duration = now.Sub(*document.VisionStartedAt)
	}
	document.ParseStrategy, document.QualityTier, document.Status = "hybrid", qualityTier, status
	s.recordDocumentEvent(ctx, document, DocumentEventVisionFallback, visionStatus, status, &duration)
	terminalKind, terminalOutcome := DocumentEventPartial, "partial"
	if status == "ready" {
		terminalKind, terminalOutcome = DocumentEventReady, "succeeded"
	}
	s.recordDocumentEvent(ctx, document, terminalKind, terminalOutcome, status, &duration)
	return nil
}
