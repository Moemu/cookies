package knowledge

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/ids"
)

const MaxDocumentBytes int64 = 10 * 1024 * 1024
const maxExtractedBytes int64 = 20 * 1024 * 1024

var ErrInvalidDocument = errors.New("invalid knowledge document")
var ErrExternalConfirmationRequired = errors.New("external research confirmation is required")
var ErrExternalRunnerUnavailable = errors.New("external research runner is not configured")
var ErrInvalidResearchRequest = errors.New("invalid research request")

type ProjectReader interface {
	GetContext(context.Context, contract.ActorContext, contract.ProjectID) (contract.ProjectContext, error)
}

type Service struct {
	DB           *sql.DB
	Projects     ProjectReader
	Blobs        assets.BlobStore
	Scanner      assets.ContentScanner
	AssetsBucket string
	Runner       ExternalResearchRunner
	Scheduler    ResearchScheduler
	Now          func() time.Time
	NewID        ids.Generator
}

type Document struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	Title          string                  `json:"title,omitempty"`
	SourceURI      string                  `json:"source_uri,omitempty"`
	SourceType     string                  `json:"source_type,omitempty"`
	ChunkCount     int                     `json:"chunk_count,omitempty"`
	ImportedBy     contract.Principal      `json:"imported_by,omitempty"`
	Filename       string                  `json:"filename"`
	MIMEType       string                  `json:"mime_type"`
	SizeBytes      int64                   `json:"size_bytes"`
	ContentSHA256  string                  `json:"content_sha256"`
	TextSHA256     string                  `json:"text_sha256"`
	ExtractedText  string                  `json:"extracted_text,omitempty"`
	Status         string                  `json:"status"`
	CreatedBy      string                  `json:"created_by"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
	Blob           assets.ObjectLocation   `json:"-"`
}

type ResearchRequest struct {
	Mode            string   `json:"mode"`
	Category        string   `json:"category,omitempty"`
	Query           string   `json:"query"`
	DocumentIDs     []string `json:"document_ids"`
	DisclosedFields []string `json:"disclosed_fields"`
	Confirmed       bool     `json:"confirmed"`
}

type ExternalResearchInput struct {
	Mode      string             `json:"mode"`
	Query     string             `json:"query"`
	Documents []ExternalDocument `json:"documents"`
}

type ExternalDocument struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

type ExternalResearchResult struct {
	Title     string   `json:"title"`
	SourceURL string   `json:"source_url,omitempty"`
	Content   string   `json:"content"`
	Citations []string `json:"citations"`
}

type ExternalResearchRunner interface {
	Run(context.Context, ExternalResearchInput) ([]ExternalResearchResult, error)
}

type ResearchScheduler interface {
	Schedule(context.Context, ResearchRun) error
}

type ResearchRun struct {
	ID              string                  `json:"id"`
	OrganizationID  contract.OrganizationID `json:"organization_id"`
	ProjectID       contract.ProjectID      `json:"project_id"`
	Mode            string                  `json:"mode"`
	Category        string                  `json:"category"`
	Query           string                  `json:"query"`
	DocumentIDs     []string                `json:"document_ids"`
	DisclosedFields []string                `json:"disclosed_fields"`
	Status          string                  `json:"status"`
	ConfirmedBy     string                  `json:"confirmed_by"`
	ConfirmedAt     time.Time               `json:"confirmed_at"`
	ErrorCode       string                  `json:"error_code,omitempty"`
	ErrorMessage    string                  `json:"error_message,omitempty"`
	Artifacts       []ResearchArtifact      `json:"artifacts"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

type ResearchArtifact struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	ResearchRunID  string                  `json:"research_run_id"`
	SourceType     string                  `json:"source_type"`
	Category       string                  `json:"category"`
	Title          string                  `json:"title"`
	SourceURL      string                  `json:"source_url,omitempty"`
	Content        string                  `json:"content"`
	Citations      []string                `json:"citations"`
	ContentHash    string                  `json:"content_hash"`
	CreatedAt      time.Time               `json:"created_at"`
}

type Reference struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	ContentHash string   `json:"content_hash"`
	Citations   []string `json:"citations,omitempty"`
}

func (s Service) CreateDocument(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, filename, declaredMIME string, source io.Reader, size int64) (Document, error) {
	if s.DB == nil || s.Projects == nil || s.Blobs == nil || s.Scanner == nil {
		return Document{}, fmt.Errorf("knowledge service dependencies are incomplete")
	}
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return Document{}, err
	}
	filename = strings.TrimSpace(filepath.Base(filename))
	extension := strings.ToLower(filepath.Ext(filename))
	if filename == "" || len(filename) > 512 || (extension != ".md" && extension != ".docx") ||
		size < 1 || size > MaxDocumentBytes {
		return Document{}, ErrInvalidDocument
	}
	content, err := io.ReadAll(io.LimitReader(source, MaxDocumentBytes+1))
	if err != nil || int64(len(content)) != size || int64(len(content)) > MaxDocumentBytes {
		return Document{}, ErrInvalidDocument
	}
	if err := s.Scanner.Scan(ctx, bytes.NewReader(content)); err != nil {
		return Document{}, err
	}
	extracted, mimeType, err := extractDocument(extension, content)
	if err != nil {
		return Document{}, err
	}
	if declaredMIME != "" && !allowedMIME(extension, declaredMIME) {
		return Document{}, ErrInvalidDocument
	}
	id, err := s.newID("knowledgedoc")
	if err != nil {
		return Document{}, err
	}
	contentSum := sha256.Sum256(content)
	textSum := sha256.Sum256([]byte(extracted))
	now := s.now()
	key := fmt.Sprintf("knowledge/%s/%s/%s/source%s", actor.OrganizationID, projectID, id, extension)
	object, err := s.Blobs.Put(ctx, s.AssetsBucket, key, bytes.NewReader(content), size, mimeType)
	if err != nil {
		return Document{}, err
	}
	document := Document{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		Title: filename, SourceType: "docs", ChunkCount: 1,
		Filename: filename, MIMEType: mimeType, SizeBytes: size,
		ContentSHA256: hex.EncodeToString(contentSum[:]), TextSHA256: hex.EncodeToString(textSum[:]),
		ExtractedText: extracted, Status: "ready", CreatedBy: actor.Principal.ID,
		CreatedAt: now, UpdatedAt: now, Blob: object.ObjectLocation,
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO platform_knowledge_documents
		(id, organization_id, project_id, title, source_uri, source_type, chunk_count,
		 filename, mime_type, size_bytes, content_sha256,
		 text_sha256, extracted_text, object_provider, object_bucket, object_key,
		 object_version_id, object_etag, status, created_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		document.ID, document.OrganizationID, document.ProjectID, document.Title, nil,
		document.SourceType, document.ChunkCount, document.Filename, document.MIMEType,
		document.SizeBytes, document.ContentSHA256, document.TextSHA256, document.ExtractedText,
		object.Provider, object.Bucket, object.Key, object.VersionID, object.ETag, document.Status,
		document.CreatedBy, document.CreatedAt, document.UpdatedAt)
	if err != nil {
		_ = s.Blobs.Delete(ctx, object.ObjectLocation)
		return Document{}, err
	}
	return document, nil
}

func (s Service) ImportDocument(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request ImportDocumentRequest) (Document, error) {
	if err := request.Validate(); err != nil {
		return Document{}, err
	}
	filename := strings.TrimSpace(request.Title)
	if !strings.HasSuffix(strings.ToLower(filename), ".md") {
		filename += ".md"
	}
	content := []byte(request.Text)
	document, err := s.CreateDocument(ctx, actor, projectID, filename, "text/markdown", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return Document{}, err
	}
	document.Title = strings.TrimSpace(request.Title)
	document.SourceURI = strings.TrimSpace(request.SourceURI)
	document.SourceType = normalizedSourceType(request.SourceType)
	document.ChunkCount = len(splitIntoChunks(request.Text))
	document.ImportedBy = actor.Principal
	_, err = s.DB.ExecContext(ctx, `UPDATE platform_knowledge_documents
		SET title = ?, source_uri = ?, source_type = ?, chunk_count = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		document.Title, nullable(document.SourceURI), document.SourceType, document.ChunkCount,
		document.UpdatedAt, actor.OrganizationID, projectID, document.ID)
	if err != nil {
		_, _ = s.DB.ExecContext(ctx, `DELETE FROM platform_knowledge_documents
			WHERE organization_id = ? AND project_id = ? AND id = ?`,
			actor.OrganizationID, projectID, document.ID)
		_ = s.Blobs.Delete(ctx, document.Blob)
		return Document{}, err
	}
	return document, nil
}

func (s Service) ListDocuments(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]Document, error) {
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		limit = 100
	}
	rows, err := s.DB.QueryContext(ctx, documentSelect+` WHERE organization_id = ? AND project_id = ?
		ORDER BY created_at DESC LIMIT ?`, actor.OrganizationID, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Document{}
	for rows.Next() {
		value, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		value.ExtractedText = ""
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s Service) Search(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request SearchRequest) ([]SearchResult, error) {
	if request.Limit == 0 {
		request.Limit = 10
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, documentSelect+` WHERE organization_id = ? AND project_id = ?
		ORDER BY created_at DESC LIMIT 100`, actor.OrganizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	terms := tokenize(request.Query)
	results := make([]SearchResult, 0)
	for rows.Next() {
		document, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		chunk := Chunk{
			ID: document.ID + "_chunk_0", DocumentID: document.ID, OrganizationID: document.OrganizationID,
			ProjectID: document.ProjectID, Text: document.ExtractedText, Section: "正文",
			StartLine: 1, EndLine: strings.Count(document.ExtractedText, "\n") + 1, CreatedAt: document.CreatedAt,
		}
		score := scoreChunk(document, chunk, terms)
		if score == 0 {
			continue
		}
		title := document.Title
		if title == "" {
			title = document.Filename
		}
		results = append(results, SearchResult{
			Chunk: chunk, Score: score,
			Citations: []Citation{{
				DocumentID: document.ID, ChunkID: chunk.ID, Title: title, Section: chunk.Section,
				StartLine: chunk.StartLine, EndLine: chunk.EndLine, Snippet: snippetFor(chunk.Text, terms),
			}},
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > request.Limit {
		results = results[:request.Limit]
	}
	return results, rows.Err()
}

func (s Service) GetDocument(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (Document, error) {
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return Document{}, err
	}
	return scanDocument(s.DB.QueryRowContext(ctx, documentSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`,
		actor.OrganizationID, projectID, id))
}

func (s Service) GetReference(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (Reference, error) {
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return Reference{}, err
	}
	document, err := scanDocument(s.DB.QueryRowContext(ctx, documentSelect+` WHERE organization_id = ? AND project_id = ? AND id = ?`,
		actor.OrganizationID, projectID, id))
	if err == nil {
		return Reference{
			ID: document.ID, Kind: "document", Title: document.Filename,
			Content: document.ExtractedText, ContentHash: document.TextSHA256,
		}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Reference{}, err
	}
	var artifact ResearchArtifact
	var sourceURL sql.NullString
	var citations []byte
	err = s.DB.QueryRowContext(ctx, `SELECT id, organization_id, project_id, research_run_id,
		source_type, title, source_url, content, citations, content_hash, created_at
		FROM platform_research_artifacts
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		actor.OrganizationID, projectID, id).Scan(
		&artifact.ID, &artifact.OrganizationID, &artifact.ProjectID, &artifact.ResearchRunID,
		&artifact.SourceType, &artifact.Title, &sourceURL, &artifact.Content, &citations,
		&artifact.ContentHash, &artifact.CreatedAt,
	)
	if err != nil {
		return Reference{}, err
	}
	artifact.SourceURL = sourceURL.String
	if err := json.Unmarshal(citations, &artifact.Citations); err != nil {
		return Reference{}, err
	}
	return Reference{
		ID: artifact.ID, Kind: "research_artifact", Title: artifact.Title,
		Content: artifact.Content, ContentHash: artifact.ContentHash,
		Citations: append([]string(nil), artifact.Citations...),
	}, nil
}

func (s Service) RunResearch(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request ResearchRequest) (ResearchRun, error) {
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return ResearchRun{}, err
	}
	request.Mode = strings.ToLower(strings.TrimSpace(request.Mode))
	if !validResearchCategory(request.Category, true) {
		return ResearchRun{}, ErrInvalidResearchRequest
	}
	request.Category = normalizedResearchCategory(request.Category)
	request.Query = strings.TrimSpace(request.Query)
	if !request.Confirmed {
		return ResearchRun{}, ErrExternalConfirmationRequired
	}
	var err error
	request.DocumentIDs, request.DisclosedFields, err = validateResearchRequest(request)
	if err != nil {
		return ResearchRun{}, err
	}
	documents := make([]ExternalDocument, 0, len(request.DocumentIDs))
	for _, id := range request.DocumentIDs {
		value, err := s.GetDocument(ctx, actor, projectID, id)
		if err != nil {
			return ResearchRun{}, err
		}
		documents = append(documents, ExternalDocument{ID: value.ID, Filename: value.Filename, Content: value.ExtractedText})
	}
	id, err := s.newID("researchrun")
	if err != nil {
		return ResearchRun{}, err
	}
	now := s.now()
	run := ResearchRun{
		ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		Mode: request.Mode, Category: request.Category, Query: request.Query,
		DocumentIDs:     append([]string(nil), request.DocumentIDs...),
		DisclosedFields: append([]string(nil), request.DisclosedFields...), Status: "running",
		ConfirmedBy: actor.Principal.ID, ConfirmedAt: now, Artifacts: []ResearchArtifact{},
		CreatedAt: now, UpdatedAt: now,
	}
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO platform_research_runs
		(id, organization_id, project_id, mode, category, query_text, document_ids, disclosed_fields,
		 status, confirmed_by, confirmed_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.OrganizationID, run.ProjectID, run.Mode, run.Category, run.Query, jsonBytes(run.DocumentIDs),
		jsonBytes(run.DisclosedFields), run.Status, run.ConfirmedBy, run.ConfirmedAt, now, now); err != nil {
		return ResearchRun{}, err
	}
	if s.Runner == nil {
		run.Status, run.ErrorCode, run.ErrorMessage = "unavailable", "EXTERNAL_RUNNER_UNAVAILABLE", ErrExternalRunnerUnavailable.Error()
		run.UpdatedAt = s.now()
		_, _ = s.DB.ExecContext(ctx, `UPDATE platform_research_runs SET status = ?, error_code = ?,
			error_message = ?, updated_at = ? WHERE organization_id = ? AND project_id = ? AND id = ?`,
			run.Status, run.ErrorCode, run.ErrorMessage, run.UpdatedAt, run.OrganizationID, run.ProjectID, run.ID)
		return run, nil
	}
	if s.Scheduler != nil {
		if err := s.Scheduler.Schedule(ctx, run); err != nil {
			run.Status, run.ErrorCode, run.ErrorMessage = "failed", "RESEARCH_SCHEDULE_FAILED", "研究任务暂时无法进入执行队列"
			run.UpdatedAt = s.now()
			_, _ = s.DB.ExecContext(ctx, `UPDATE platform_research_runs SET status = ?, error_code = ?,
				error_message = ?, updated_at = ? WHERE organization_id = ? AND project_id = ? AND id = ?`,
				run.Status, run.ErrorCode, run.ErrorMessage, run.UpdatedAt, actor.OrganizationID, projectID, run.ID)
		}
		return run, nil
	}
	return s.executeResearch(ctx, run, documents)
}

func (s Service) executeResearch(ctx context.Context, run ResearchRun, documents []ExternalDocument) (ResearchRun, error) {
	results, err := s.Runner.Run(ctx, ExternalResearchInput{Mode: run.Mode, Query: run.Query, Documents: documents})
	if err != nil {
		run.Status, run.ErrorCode, run.ErrorMessage = "failed", "EXTERNAL_RESEARCH_FAILED", "外部研究调用失败"
		run.UpdatedAt = s.now()
		_, _ = s.DB.ExecContext(ctx, `UPDATE platform_research_runs SET status = ?, error_code = ?,
			error_message = ?, updated_at = ? WHERE organization_id = ? AND project_id = ? AND id = ?`,
			run.Status, run.ErrorCode, run.ErrorMessage, run.UpdatedAt, run.OrganizationID, run.ProjectID, run.ID)
		return run, nil
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ResearchRun{}, err
	}
	defer tx.Rollback()
	for _, result := range results {
		artifact, err := s.insertArtifact(ctx, tx, run, result)
		if err != nil {
			return ResearchRun{}, err
		}
		run.Artifacts = append(run.Artifacts, artifact)
	}
	run.Status = "succeeded"
	run.UpdatedAt = s.now()
	if _, err := tx.ExecContext(ctx, `UPDATE platform_research_runs SET status = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		run.Status, run.UpdatedAt, run.OrganizationID, run.ProjectID, run.ID); err != nil {
		return ResearchRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return ResearchRun{}, err
	}
	return run, nil
}

func (s Service) GetResearchRun(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (ResearchRun, error) {
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return ResearchRun{}, err
	}
	return s.getResearchRun(ctx, actor.OrganizationID, projectID, id)
}

func (s Service) ListResearchRuns(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]ResearchRun, error) {
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	rows, err := s.DB.QueryContext(ctx, researchRunSelect+`
		WHERE organization_id = ? AND project_id = ?
		ORDER BY created_at DESC LIMIT ?`, actor.OrganizationID, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []ResearchRun{}
	for rows.Next() {
		value, err := scanResearchRun(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range values {
		values[index].Artifacts, err = s.listResearchArtifacts(ctx, values[index].OrganizationID, values[index].ProjectID, values[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return values, nil
}

const researchRunSelect = `SELECT id, organization_id, project_id, mode, query_text,
	category, document_ids, disclosed_fields, status, confirmed_by, confirmed_at,
	COALESCE(error_code, ''), COALESCE(error_message, ''), created_at, updated_at
	FROM platform_research_runs`

type researchRunScanner interface {
	Scan(...any) error
}

func scanResearchRun(scanner researchRunScanner) (ResearchRun, error) {
	var value ResearchRun
	var documentIDs, disclosedFields []byte
	err := scanner.Scan(
		&value.ID, &value.OrganizationID, &value.ProjectID, &value.Mode, &value.Query,
		&value.Category, &documentIDs, &disclosedFields, &value.Status, &value.ConfirmedBy, &value.ConfirmedAt,
		&value.ErrorCode, &value.ErrorMessage, &value.CreatedAt, &value.UpdatedAt,
	)
	if err != nil {
		return ResearchRun{}, err
	}
	if err := json.Unmarshal(documentIDs, &value.DocumentIDs); err != nil {
		return ResearchRun{}, err
	}
	if err := json.Unmarshal(disclosedFields, &value.DisclosedFields); err != nil {
		return ResearchRun{}, err
	}
	value.Artifacts = []ResearchArtifact{}
	return value, nil
}

func (s Service) getResearchRun(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (ResearchRun, error) {
	value, err := scanResearchRun(s.DB.QueryRowContext(ctx, researchRunSelect+`
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		organizationID, projectID, strings.TrimSpace(id)))
	if err != nil {
		return ResearchRun{}, err
	}
	value.Artifacts, err = s.listResearchArtifacts(ctx, organizationID, projectID, value.ID)
	if err != nil {
		return ResearchRun{}, err
	}
	return value, nil
}

func (s Service) listResearchArtifacts(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, runID string) ([]ResearchArtifact, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, organization_id, project_id, research_run_id,
		source_type, category, title, COALESCE(source_url, ''), content, citations, content_hash, created_at
		FROM platform_research_artifacts
		WHERE organization_id = ? AND project_id = ? AND research_run_id = ?
		ORDER BY created_at ASC`, organizationID, projectID, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []ResearchArtifact{}
	for rows.Next() {
		var value ResearchArtifact
		var citations []byte
		if err := rows.Scan(
			&value.ID, &value.OrganizationID, &value.ProjectID, &value.ResearchRunID,
			&value.SourceType, &value.Category, &value.Title, &value.SourceURL, &value.Content, &citations,
			&value.ContentHash, &value.CreatedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(citations, &value.Citations); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s Service) ListResearchArtifacts(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, category string, limit int) ([]ResearchArtifact, error) {
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	query := `SELECT id, organization_id, project_id, research_run_id, source_type, category,
		title, COALESCE(source_url, ''), content, citations, content_hash, created_at
		FROM platform_research_artifacts
		WHERE organization_id = ? AND project_id = ?`
	args := []any{actor.OrganizationID, projectID}
	if category = strings.ToLower(strings.TrimSpace(category)); category != "" && category != "all" {
		if !validResearchCategory(category, false) {
			return nil, ErrInvalidResearchRequest
		}
		query += ` AND category = ?`
		args = append(args, normalizedResearchCategory(category))
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]ResearchArtifact, 0)
	for rows.Next() {
		value, err := scanResearchArtifact(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s Service) GetResearchArtifact(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (ResearchArtifact, error) {
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return ResearchArtifact{}, err
	}
	return scanResearchArtifact(s.DB.QueryRowContext(ctx, `SELECT id, organization_id, project_id,
		research_run_id, source_type, category, title, COALESCE(source_url, ''), content,
		citations, content_hash, created_at FROM platform_research_artifacts
		WHERE organization_id = ? AND project_id = ? AND id = ?`,
		actor.OrganizationID, projectID, strings.TrimSpace(id)))
}

func scanResearchArtifact(scanner interface{ Scan(...any) error }) (ResearchArtifact, error) {
	var value ResearchArtifact
	var citations []byte
	if err := scanner.Scan(
		&value.ID, &value.OrganizationID, &value.ProjectID, &value.ResearchRunID,
		&value.SourceType, &value.Category, &value.Title, &value.SourceURL, &value.Content,
		&citations, &value.ContentHash, &value.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ResearchArtifact{}, ErrNotFound
		}
		return ResearchArtifact{}, err
	}
	if err := json.Unmarshal(citations, &value.Citations); err != nil {
		return ResearchArtifact{}, err
	}
	return value, nil
}

func validateResearchRequest(request ResearchRequest) ([]string, []string, error) {
	if (request.Mode != "web" && request.Mode != "mcp") || request.Query == "" ||
		len(request.Query) > 2000 || len(request.DocumentIDs) > 20 {
		return nil, nil, ErrInvalidResearchRequest
	}
	documentIDs := make([]string, 0, len(request.DocumentIDs))
	seenDocuments := map[string]struct{}{}
	for _, value := range request.DocumentIDs {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, nil, ErrInvalidResearchRequest
		}
		if _, exists := seenDocuments[value]; exists {
			continue
		}
		seenDocuments[value] = struct{}{}
		documentIDs = append(documentIDs, value)
	}
	disclosed := map[string]struct{}{}
	for _, value := range request.DisclosedFields {
		value = strings.TrimSpace(value)
		if value != "query" && value != "document_content" {
			return nil, nil, ErrInvalidResearchRequest
		}
		disclosed[value] = struct{}{}
	}
	if _, ok := disclosed["query"]; !ok {
		return nil, nil, ErrInvalidResearchRequest
	}
	_, disclosesDocuments := disclosed["document_content"]
	if disclosesDocuments != (len(documentIDs) > 0) {
		return nil, nil, ErrInvalidResearchRequest
	}
	fields := []string{"query"}
	if disclosesDocuments {
		fields = append(fields, "document_content")
	}
	return documentIDs, fields, nil
}

func (s Service) insertArtifact(ctx context.Context, tx *sql.Tx, run ResearchRun, result ExternalResearchResult) (ResearchArtifact, error) {
	if strings.TrimSpace(result.Title) == "" || strings.TrimSpace(result.Content) == "" {
		return ResearchArtifact{}, fmt.Errorf("external research returned an incomplete artifact")
	}
	id, err := s.newID("researchartifact")
	if err != nil {
		return ResearchArtifact{}, err
	}
	hash, err := contract.CanonicalJSONHash(result)
	if err != nil {
		return ResearchArtifact{}, err
	}
	value := ResearchArtifact{
		ID: id, OrganizationID: run.OrganizationID, ProjectID: run.ProjectID,
		ResearchRunID: run.ID, SourceType: run.Mode, Category: run.Category,
		Title:     strings.TrimSpace(result.Title),
		SourceURL: strings.TrimSpace(result.SourceURL), Content: strings.TrimSpace(result.Content),
		Citations: append([]string(nil), result.Citations...), ContentHash: hash, CreatedAt: s.now(),
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO platform_research_artifacts
		(id, organization_id, project_id, research_run_id, source_type, category, title, source_url,
		 content, citations, content_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		value.ID, value.OrganizationID, value.ProjectID, value.ResearchRunID, value.SourceType, value.Category,
		value.Title, nullable(value.SourceURL), value.Content, jsonBytes(value.Citations),
		value.ContentHash, value.CreatedAt)
	return value, err
}

func normalizedResearchCategory(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "audience", "competitor", "industry":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "general"
	}
}

func validResearchCategory(value string, allowEmpty bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return allowEmpty
	case "general", "audience", "competitor", "industry":
		return true
	default:
		return false
	}
}

func extractDocument(extension string, content []byte) (string, string, error) {
	switch extension {
	case ".md":
		content = bytes.TrimPrefix(content, []byte{0xef, 0xbb, 0xbf})
		if !utf8.Valid(content) {
			return "", "", ErrInvalidDocument
		}
		return strings.TrimSpace(string(content)), "text/markdown", nil
	case ".docx":
		reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
		if err != nil {
			return "", "", ErrInvalidDocument
		}
		for _, file := range reader.File {
			if file.Name != "word/document.xml" || file.UncompressedSize64 > uint64(maxExtractedBytes) {
				continue
			}
			stream, err := file.Open()
			if err != nil {
				return "", "", ErrInvalidDocument
			}
			text, err := extractWordXML(io.LimitReader(stream, maxExtractedBytes+1))
			_ = stream.Close()
			if err != nil || text == "" {
				return "", "", ErrInvalidDocument
			}
			return text, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", nil
		}
	}
	return "", "", ErrInvalidDocument
}

func extractWordXML(source io.Reader) (string, error) {
	decoder := xml.NewDecoder(source)
	var builder strings.Builder
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "t" {
				var text string
				if err := decoder.DecodeElement(&text, &value); err != nil {
					return "", err
				}
				builder.WriteString(text)
			} else if value.Name.Local == "tab" {
				builder.WriteByte('\t')
			} else if value.Name.Local == "br" {
				builder.WriteByte('\n')
			}
		case xml.EndElement:
			if value.Name.Local == "p" {
				builder.WriteByte('\n')
			}
		}
		if int64(builder.Len()) > maxExtractedBytes {
			return "", ErrInvalidDocument
		}
	}
	return strings.TrimSpace(builder.String()), nil
}

func allowedMIME(extension, value string) bool {
	value = strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
	switch extension {
	case ".md":
		return value == "text/markdown" || value == "text/plain" || value == "application/octet-stream"
	case ".docx":
		return value == "application/vnd.openxmlformats-officedocument.wordprocessingml.document" ||
			value == "application/octet-stream"
	default:
		return false
	}
}

const documentSelect = `SELECT id, organization_id, project_id, title, COALESCE(source_uri, ''),
	source_type, chunk_count, filename, mime_type, size_bytes,
	content_sha256, text_sha256, extracted_text, status, created_by, created_at, updated_at,
	object_provider, object_bucket, object_key, object_version_id, object_etag
	FROM platform_knowledge_documents`

type scanner interface {
	Scan(...any) error
}

func scanDocument(row scanner) (Document, error) {
	var value Document
	var versionID, etag sql.NullString
	err := row.Scan(
		&value.ID, &value.OrganizationID, &value.ProjectID, &value.Title, &value.SourceURI,
		&value.SourceType, &value.ChunkCount, &value.Filename, &value.MIMEType,
		&value.SizeBytes, &value.ContentSHA256, &value.TextSHA256, &value.ExtractedText,
		&value.Status, &value.CreatedBy, &value.CreatedAt, &value.UpdatedAt,
		&value.Blob.Provider, &value.Blob.Bucket, &value.Blob.Key, &versionID, &etag,
	)
	value.Blob.VersionID, value.Blob.ETag = versionID.String, etag.String
	return value, err
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s Service) newID(prefix string) (string, error) {
	if s.NewID != nil {
		return s.NewID(prefix)
	}
	return ids.New(prefix)
}

func jsonBytes(value any) []byte {
	encoded, _ := json.Marshal(value)
	return encoded
}

func nullable(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
