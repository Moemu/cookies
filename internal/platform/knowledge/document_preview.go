package knowledge

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

const documentPreviewContractVersion = "platform-document-preview/v1"
const documentPreviewTextRunes = 40_000

type DocumentPreviewChunk struct {
	ID         string          `json:"id"`
	Index      int             `json:"index"`
	Section    string          `json:"section"`
	PageNumber *int            `json:"page_number,omitempty"`
	StartLine  int             `json:"start_line"`
	EndLine    int             `json:"end_line"`
	Snippet    string          `json:"snippet"`
	Locator    json.RawMessage `json:"locator"`
}

type DocumentPreview struct {
	ContractVersion string `json:"contract_version"`
	DocumentParseState
	DocumentID        string                 `json:"document_id"`
	Filename          string                 `json:"filename"`
	MIMEType          string                 `json:"mime_type"`
	Status            string                 `json:"status"`
	Text              string                 `json:"text"`
	TextTruncated     bool                   `json:"text_truncated"`
	TotalCharacters   int                    `json:"total_characters"`
	ChunkCount        int                    `json:"chunk_count"`
	OriginalAvailable bool                   `json:"original_available"`
	Chunks            []DocumentPreviewChunk `json:"chunks"`
}

func (s Service) GetDocumentPreview(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	documentID string,
) (DocumentPreview, error) {
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return DocumentPreview{}, err
	}
	var preview DocumentPreview
	var parseProgress, processedPages, totalPages sql.NullInt64
	var qualityScore sql.NullFloat64
	var heartbeatAt sql.NullTime
	var summary []byte
	err := s.DB.QueryRowContext(ctx, `SELECT id, filename, mime_type, status,
		LEFT(extracted_text, ?), CHAR_LENGTH(extracted_text), chunk_count,
		parse_strategy, parse_phase, parse_progress, progress_kind,
		processed_pages, total_pages, quality_score, quality_tier,
		fallback_reason, preview_status, page_quality_summary, heartbeat_at,
		CASE WHEN object_key <> '' THEN 1 ELSE 0 END
		FROM platform_knowledge_documents
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		documentPreviewTextRunes+1, actor.OrganizationID, projectID, documentID,
	).Scan(
		&preview.DocumentID, &preview.Filename, &preview.MIMEType, &preview.Status,
		&preview.Text, &preview.TotalCharacters, &preview.ChunkCount,
		&preview.ParseStrategy, &preview.ParsePhase, &parseProgress, &preview.ProgressKind,
		&processedPages, &totalPages, &qualityScore, &preview.QualityTier,
		&preview.FallbackReason, &preview.PreviewStatus, &summary, &heartbeatAt,
		&preview.OriginalAvailable,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return DocumentPreview{}, ErrNotFound
	}
	if err != nil {
		return DocumentPreview{}, err
	}
	preview.ContractVersion = documentPreviewContractVersion
	preview.DocumentParseState.ContractVersion = DocumentParseContractVersion
	if len(summary) > 0 {
		preview.PageQualitySummary = append(json.RawMessage(nil), summary...)
	}
	preview.ParseProgress = nullIntPointer(parseProgress)
	preview.ProcessedPages = nullIntPointer(processedPages)
	preview.TotalPages = nullIntPointer(totalPages)
	if qualityScore.Valid {
		preview.QualityScore = &qualityScore.Float64
	}
	if heartbeatAt.Valid {
		preview.HeartbeatAt = &heartbeatAt.Time
	}
	runes := []rune(preview.Text)
	if len(runes) > documentPreviewTextRunes {
		preview.Text = string(runes[:documentPreviewTextRunes])
		preview.TextTruncated = true
	}
	preview.Chunks, err = s.listDocumentPreviewChunks(ctx, actor.OrganizationID, projectID, documentID)
	return preview, err
}

func (s Service) listDocumentPreviewChunks(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	documentID string,
) ([]DocumentPreviewChunk, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, ordinal, section, page_number,
		start_line, end_line, LEFT(text, 500), locator_json
		FROM platform_knowledge_chunks
		WHERE organization_id = ? AND project_id = ? AND document_id = ?
		ORDER BY ordinal LIMIT 24`, organizationID, projectID, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]DocumentPreviewChunk, 0, 24)
	for rows.Next() {
		var value DocumentPreviewChunk
		var pageNumber sql.NullInt64
		if err := rows.Scan(&value.ID, &value.Index, &value.Section, &pageNumber,
			&value.StartLine, &value.EndLine, &value.Snippet, &value.Locator); err != nil {
			return nil, err
		}
		if pageNumber.Valid {
			value.PageNumber = intPointer(int(pageNumber.Int64))
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s Service) OpenDocumentContent(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	documentID string,
) (io.ReadCloser, assets.ObjectInfo, string, error) {
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return nil, assets.ObjectInfo{}, "", err
	}
	var location assets.ObjectLocation
	var filename string
	var versionID, etag sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT filename, object_provider, object_bucket,
		object_key, object_version_id, object_etag
		FROM platform_knowledge_documents
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		actor.OrganizationID, projectID, documentID,
	).Scan(&filename, &location.Provider, &location.Bucket, &location.Key, &versionID, &etag)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, assets.ObjectInfo{}, "", ErrNotFound
	}
	if err != nil {
		return nil, assets.ObjectInfo{}, "", err
	}
	location.VersionID, location.ETag = versionID.String, etag.String
	if s.Blobs == nil || !knowledgeDocumentLocationInScope(actor.OrganizationID, projectID, documentID, location, s.AssetsBucket) {
		return nil, assets.ObjectInfo{}, "", ErrNotFound
	}
	reader, info, err := s.Blobs.Open(ctx, location)
	return reader, info, filename, err
}

func nullIntPointer(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	return intPointer(int(value.Int64))
}
