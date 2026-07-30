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
	_, _, err = s.Store.Enqueue(ctx, jobruntime.CreateRequest{
		Job: contract.Job{
			ID: jobID, Kind: DocumentParseJobKind,
			OrganizationID: document.OrganizationID, ProjectID: document.ProjectID,
			Status: contract.JobQueued, Progress: 0, Cancellable: true,
			AttemptCount: 0, MaxAttempts: 1, Version: 1,
			CreatedAt: now().UTC(), UpdatedAt: now().UTC(),
		},
		Payload:        payload,
		IdempotencyKey: contract.IdempotencyKey("knowledge_parse_" + document.ID),
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
	if document.Status == "ready" || document.Status == "parse_failed" {
		return jobruntime.Result{Ref: &ref}, nil
	}
	if s.DocumentParser == nil || s.Blobs == nil {
		s.markDocumentParseFailed(ctx, document, "DOCUMENT_PARSER_UNAVAILABLE", "Document parser is unavailable")
		return jobruntime.Result{Ref: &ref}, nil
	}
	_, _ = s.DB.ExecContext(ctx, `UPDATE platform_knowledge_documents
		SET status = 'parsing', parse_error_code = NULL, parse_error_message = NULL, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		s.now(), document.OrganizationID, document.ProjectID, document.ID)
	stream, info, err := s.Blobs.Open(ctx, document.Blob)
	if err != nil {
		s.markDocumentParseFailed(ctx, document, "DOCUMENT_SOURCE_UNAVAILABLE", "Document source is unavailable")
		return jobruntime.Result{Ref: &ref}, nil
	}
	parsed, parseErr := s.DocumentParser.Parse(ctx, DocumentParseRequest{
		Filename: document.Filename, MIMEType: document.MIMEType,
		Size: info.SizeBytes, Source: stream,
	})
	_ = stream.Close()
	if parseErr != nil {
		s.markDocumentParseFailed(ctx, document, "DOCUMENT_PARSE_FAILED", parseErr.Error())
		return jobruntime.Result{Ref: &ref}, nil
	}
	document.UpdatedAt = s.now()
	chunks := chunksForParsedDocument(document, parsed)
	if len(chunks) == 0 {
		s.markDocumentParseFailed(ctx, document, "DOCUMENT_PARSE_EMPTY", "Document parser returned no chunks")
		return jobruntime.Result{Ref: &ref}, nil
	}
	if err := s.persistParsedDocument(ctx, document, parsed, chunks); err != nil {
		return jobruntime.Result{}, err
	}
	return jobruntime.Result{Ref: &ref}, nil
}

func (s Service) persistParsedDocument(
	ctx context.Context,
	document Document,
	parsed ParsedDocument,
	chunks []Chunk,
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
	if _, err := tx.ExecContext(ctx, `UPDATE platform_knowledge_documents
		SET mime_type = ?, chunk_count = ?, text_sha256 = ?, extracted_text = ?,
			status = 'ready', parser_code = ?, parser_version = ?,
			parse_error_code = NULL, parse_error_message = NULL,
			parse_metadata = ?, parsed_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		parsed.MIMEType, len(chunks), hex.EncodeToString(textSum[:]), parsed.Text,
		parsed.ParserCode, parsed.ParserVersion, nullableJSON(parsed.Metadata), now, now,
		document.OrganizationID, document.ProjectID, document.ID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s Service) markDocumentParseFailed(ctx context.Context, document Document, code, message string) {
	if len(message) > 1024 {
		message = message[:1024]
	}
	_, _ = s.DB.ExecContext(ctx, `UPDATE platform_knowledge_documents
		SET status = 'parse_failed', parse_error_code = ?, parse_error_message = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		code, message, s.now(), document.OrganizationID, document.ProjectID, document.ID)
}
