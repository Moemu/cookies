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

const DocumentParseJobKind = "knowledge.document.parse"

type JobRuntimeDocumentParseScheduler struct {
	Store researchJobStore
	NewID func() (string, error)
	Now   func() time.Time
}

func (s JobRuntimeDocumentParseScheduler) ScheduleDocumentParse(ctx context.Context, document Document) error {
	return s.schedule(ctx, document, false)
}

func (s JobRuntimeDocumentParseScheduler) ScheduleDocumentParseRetry(ctx context.Context, document Document) error {
	return s.schedule(ctx, document, true)
}

func (s JobRuntimeDocumentParseScheduler) schedule(ctx context.Context, document Document, retry bool) error {
	if s.Store == nil || s.NewID == nil {
		return fmt.Errorf("document parse job store and ID generator are required")
	}
	jobID, err := s.NewID()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		DocumentID string `json:"document_id"`
	}{DocumentID: document.ID})
	if err != nil {
		return err
	}
	sum := sha256.Sum256(payload)
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	idempotencyKey := "knowledge_parse_" + document.ID
	if retry {
		idempotencyKey += "_retry_" + jobID
	}
	_, _, err = s.Store.Enqueue(ctx, jobruntime.CreateRequest{
		Job: contract.Job{
			ID: jobID, Kind: DocumentParseJobKind,
			OrganizationID: document.OrganizationID, ProjectID: document.ProjectID,
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

func (s Service) HandleDocumentParseJob(ctx context.Context, claim jobruntime.Claim) (jobruntime.Result, error) {
	if claim.Job.Kind != DocumentParseJobKind {
		return jobruntime.Result{}, fmt.Errorf("unsupported document parse job kind %q", claim.Job.Kind)
	}
	var payload struct {
		DocumentID string `json:"document_id"`
	}
	if err := json.Unmarshal(claim.Payload, &payload); err != nil || strings.TrimSpace(payload.DocumentID) == "" {
		return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{
			Code: "INVALID_DOCUMENT_PARSE_JOB", Message: "Document parse job payload is invalid", Retryable: false,
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
	if document.Status == "ready" {
		return jobruntime.Result{Ref: &ref}, nil
	}
	if err := s.updateDocumentParseCheckpoint(ctx, claim, document, "scanning", 5, nil, "Validating the source document"); err != nil {
		return jobruntime.Result{}, err
	}
	if s.DocumentParser == nil || s.Blobs == nil {
		s.markDocumentParseFailed(ctx, document, "DOCUMENT_PARSER_UNAVAILABLE", "Document parser is unavailable")
		return jobruntime.Result{Ref: &ref}, nil
	}
	if !knowledgeDocumentBlobInScope(document, s.AssetsBucket) {
		s.markDocumentParseFailed(ctx, document, "DOCUMENT_STORAGE_SCOPE_INVALID", "Document source is outside the project storage scope")
		return jobruntime.Result{Ref: &ref}, nil
	}
	if err := s.updateDocumentParseCheckpoint(ctx, claim, document, "extracting", 20, nil, "Extracting document text"); err != nil {
		return jobruntime.Result{}, err
	}
	stream, info, err := s.Blobs.Open(ctx, document.Blob)
	if err != nil {
		if claim.Job.AttemptCount < claim.Job.MaxAttempts {
			s.recordDocumentParseRetry(ctx, document, "DOCUMENT_SOURCE_RETRYING", "Document source is temporarily unavailable")
			return jobruntime.Result{}, jobruntime.DeferredError{AvailableAt: s.now().Add(time.Duration(claim.Job.AttemptCount) * 2 * time.Second)}
		}
		s.markDocumentParseFailed(ctx, document, "DOCUMENT_SOURCE_UNAVAILABLE", "Document source is unavailable")
		return jobruntime.Result{Ref: &ref}, nil
	}
	if info.SizeBytes != document.SizeBytes || !documentVisionObjectIdentityMatches(info.ObjectLocation, document.Blob) {
		_ = stream.Close()
		s.markDocumentParseFailed(ctx, document, "DOCUMENT_STORAGE_SCOPE_INVALID", "Document source no longer matches its immutable storage lineage")
		return jobruntime.Result{Ref: &ref}, nil
	}
	parsed, parseErr := s.DocumentParser.Parse(ctx, DocumentParseRequest{
		Filename: document.Filename, MIMEType: document.MIMEType,
		Size: info.SizeBytes, Source: stream,
	})
	_ = stream.Close()
	if parseErr != nil {
		if claim.Job.AttemptCount < claim.Job.MaxAttempts {
			s.recordDocumentParseRetry(ctx, document, "DOCUMENT_PARSE_RETRYING", parseErr.Error())
			return jobruntime.Result{}, jobruntime.DeferredError{AvailableAt: s.now().Add(time.Duration(claim.Job.AttemptCount) * 2 * time.Second)}
		}
		s.markDocumentParseFailed(ctx, document, "DOCUMENT_PARSE_FAILED", parseErr.Error())
		return jobruntime.Result{Ref: &ref}, nil
	}
	totalPages := qualityMetadataSignals(parsed.Metadata).TotalPages
	if err := s.updateDocumentParseCheckpoint(ctx, claim, document, "quality_checking", 70, totalPages, "Checking extraction quality"); err != nil {
		return jobruntime.Result{}, err
	}
	document.UpdatedAt = s.now()
	chunks := chunksForParsedDocument(document, parsed)
	if len(chunks) == 0 {
		s.markDocumentParseFailed(ctx, document, "DOCUMENT_PARSE_EMPTY", "Document parser returned no chunks")
		return jobruntime.Result{Ref: &ref}, nil
	}
	quality := evaluateParsedDocumentQuality(parsed, chunks)
	if err := s.updateDocumentParseCheckpoint(ctx, claim, document, "chunking", 85, quality.TotalPages, "Building traceable document chunks"); err != nil {
		return jobruntime.Result{}, err
	}
	if err := s.persistParsedDocument(ctx, document, parsed, chunks, quality); err != nil {
		s.markDocumentParseFailed(ctx, document, "DOCUMENT_PERSIST_FAILED", "Parsed document could not be persisted")
		return jobruntime.Result{}, err
	}
	return jobruntime.Result{Ref: &ref}, nil
}

func (s Service) updateDocumentParseCheckpoint(
	ctx context.Context,
	claim jobruntime.Claim,
	document Document,
	phase string,
	progress int,
	totalPages *int,
	message string,
) error {
	now := s.now()
	progressKind := "milestone"
	var processedPages *int
	if totalPages != nil {
		progressKind = "pages"
		processed := 0
		if phase == "quality_checking" || phase == "chunking" {
			processed = *totalPages
		}
		processedPages = &processed
	}
	if _, err := s.DB.ExecContext(ctx, `UPDATE platform_knowledge_documents
		SET status = 'parsing', parse_phase = ?, parse_progress = ?, progress_kind = ?,
			processed_pages = ?, total_pages = ?, preview_status = 'building',
			parse_error_code = NULL, parse_error_message = NULL, heartbeat_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		phase, progress, progressKind, processedPages, totalPages, now, now,
		document.OrganizationID, document.ProjectID, document.ID,
	); err != nil {
		return err
	}
	return s.reportJobProgress(ctx, claim, progress, message)
}

func (s Service) reportJobProgress(ctx context.Context, claim jobruntime.Claim, progress int, message string) error {
	if s.JobProgress == nil {
		return nil
	}
	return s.JobProgress.UpdateProgress(ctx, claim, progress, message, s.now())
}

func (s Service) persistParsedDocument(
	ctx context.Context,
	document Document,
	parsed ParsedDocument,
	chunks []Chunk,
	quality documentQualityResult,
) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM platform_knowledge_chunks
		WHERE organization_id = ? AND project_id = ? AND document_id = ?`,
		document.OrganizationID, document.ProjectID, document.ID,
	); err != nil {
		return err
	}
	for _, chunk := range chunks {
		if _, err := tx.ExecContext(ctx, `INSERT INTO platform_knowledge_chunks
			(id, organization_id, project_id, document_id, ordinal, kind, section,
			 page_number, start_line, end_line, text, text_sha256, locator_json,
			 parser_code, parser_version, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			chunk.ID, chunk.OrganizationID, chunk.ProjectID, chunk.DocumentID, chunk.Index,
			chunk.Kind, chunk.Section, chunk.PageNumber, chunk.StartLine, chunk.EndLine,
			chunk.Text, chunk.TextSHA256, jsonBytes(chunk.Locator), chunk.ParserCode,
			chunk.ParserVersion, chunk.CreatedAt,
		); err != nil {
			return err
		}
	}
	textSum := sha256.Sum256([]byte(parsed.Text))
	now := s.now()
	status := "ready"
	phase := "ready"
	if quality.Tier == "low" {
		status = "partial"
		phase = "partial"
	}
	qualitySummary, err := json.Marshal(quality.Summary)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE platform_knowledge_documents
		SET mime_type = ?, chunk_count = ?, text_sha256 = ?, extracted_text = ?,
			status = ?, parser_code = ?, parser_version = ?,
			parse_error_code = NULL, parse_error_message = NULL,
			parse_metadata = ?, parse_phase = ?, parse_progress = 100,
			progress_kind = ?, processed_pages = ?, total_pages = ?,
			quality_score = ?, quality_tier = ?, fallback_reason = ?,
			preview_status = ?, page_quality_summary = ?, heartbeat_at = ?,
			parsed_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		parsed.MIMEType, len(chunks), hex.EncodeToString(textSum[:]), parsed.Text,
		status, parsed.ParserCode, parsed.ParserVersion, nullableJSON(parsed.Metadata), phase,
		progressKindForPages(quality.TotalPages), quality.TotalPages, quality.TotalPages,
		quality.Score, quality.Tier, quality.FallbackReason, quality.PreviewStatus,
		qualitySummary, now, now, now,
		document.OrganizationID, document.ProjectID, document.ID,
	); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	document.QualityTier = quality.Tier
	document.Status = status
	duration := now.Sub(document.CreatedAt)
	kind, outcome := DocumentEventReady, "succeeded"
	if status == "partial" {
		kind, outcome = DocumentEventPartial, "partial"
	}
	s.recordDocumentEvent(ctx, document, kind, outcome, status, &duration)
	return nil
}

func (s Service) markDocumentParseFailed(ctx context.Context, document Document, code, message string) {
	if len(message) > 1024 {
		message = message[:1024]
	}
	now := s.now()
	_, _ = s.DB.ExecContext(ctx, `UPDATE platform_knowledge_documents
		SET status = CASE WHEN chunk_count > 0 THEN 'partial' ELSE 'parse_failed' END,
			parse_phase = CASE WHEN chunk_count > 0 THEN 'partial' ELSE 'failed' END,
			parse_progress = CASE WHEN chunk_count > 0 THEN 100 ELSE NULL END,
			preview_status = CASE WHEN chunk_count > 0 THEN 'partial' ELSE 'failed' END,
			parse_error_code = ?, parse_error_message = ?, heartbeat_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		code, message, now, now, document.OrganizationID, document.ProjectID, document.ID)
	duration := now.Sub(document.CreatedAt)
	s.recordDocumentEvent(ctx, document, DocumentEventFailed, "failed", "parse_failed", &duration)
}

func (s Service) recordDocumentParseRetry(ctx context.Context, document Document, code, message string) {
	if len(message) > 1024 {
		message = message[:1024]
	}
	now := s.now()
	_, _ = s.DB.ExecContext(ctx, `UPDATE platform_knowledge_documents
		SET status = 'parse_queued', parse_phase = 'queued', parse_progress = 5,
			progress_kind = 'milestone', parse_error_code = ?, parse_error_message = ?,
			heartbeat_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		code, message, now, now, document.OrganizationID, document.ProjectID, document.ID)
}

func progressKindForPages(totalPages *int) string {
	if totalPages != nil {
		return "pages"
	}
	return "milestone"
}
