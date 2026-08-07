package knowledge

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const maxResearchDisclosedChunks = 8
const maxConversationContextChunks = 12

type chunkCandidate struct {
	document Document
	chunk    Chunk
}

// SelectConversationChunks returns a bounded, project-authorized set of ready
// chunks for an explicitly attached document list. It never searches across
// the project's unselected knowledge base.
func (s Service) SelectConversationChunks(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	documentIDs []string,
	query string,
) ([]SearchResult, error) {
	if len(documentIDs) == 0 || len(documentIDs) > 8 {
		return nil, ErrInvalidDocument
	}
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(documentIDs))
	for _, id := range documentIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, ErrInvalidDocument
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, ErrInvalidDocument
		}
		seen[id] = struct{}{}
	}
	terms := tokenize(query)
	candidates, err := s.loadChunkCandidates(ctx, actor.OrganizationID, projectID, documentIDs, terms)
	if err != nil {
		return nil, err
	}
	// A relevant query may match only one of several attached documents. That
	// does not mean the other documents are still parsing. Fall back to the
	// complete attached set so readiness is checked independently from ranking.
	if !candidateSetCoversDocuments(candidates, documentIDs) {
		candidates, err = s.loadChunkCandidates(ctx, actor.OrganizationID, projectID, documentIDs, nil)
		if err != nil {
			return nil, err
		}
	}
	if !candidateSetCoversDocuments(candidates, documentIDs) {
		return nil, fmt.Errorf("%w: every attached document must be parsed and ready", ErrInvalidDocument)
	}
	results := make([]SearchResult, 0, len(candidates))
	for _, candidate := range candidates {
		results = append(results, chunkSearchResult(candidate, scoreChunk(candidate.document, candidate.chunk, terms), terms))
	}
	sortSearchResults(results)
	if len(results) > maxConversationContextChunks {
		results = results[:maxConversationContextChunks]
	}
	return results, nil
}

func candidateSetCoversDocuments(candidates []chunkCandidate, documentIDs []string) bool {
	documentsWithChunks := make(map[string]struct{}, len(documentIDs))
	for _, candidate := range candidates {
		documentsWithChunks[candidate.document.ID] = struct{}{}
	}
	for _, documentID := range documentIDs {
		if _, ok := documentsWithChunks[documentID]; !ok {
			return false
		}
	}
	return len(documentIDs) > 0
}

func (s Service) searchChunks(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	documentIDs []string,
	query string,
	limit int,
) ([]SearchResult, error) {
	terms := tokenize(query)
	candidates, err := s.loadChunkCandidates(ctx, organizationID, projectID, documentIDs, terms)
	if err != nil {
		return nil, err
	}
	results := make([]SearchResult, 0, len(candidates))
	for _, candidate := range candidates {
		score := scoreChunk(candidate.document, candidate.chunk, terms)
		if score == 0 {
			continue
		}
		results = append(results, chunkSearchResult(candidate, score, terms))
	}
	sortSearchResults(results)
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (s Service) selectResearchChunks(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	documentIDs []string,
	query string,
) ([]ExternalDocument, error) {
	if len(documentIDs) == 0 {
		return nil, nil
	}
	terms := tokenize(query)
	candidates, err := s.loadChunkCandidates(ctx, organizationID, projectID, documentIDs, terms)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		candidates, err = s.loadChunkCandidates(ctx, organizationID, projectID, documentIDs, nil)
		if err != nil {
			return nil, err
		}
	}
	if len(candidates) == 0 {
		return nil, ErrInvalidResearchRequest
	}
	results := make([]SearchResult, 0, len(candidates))
	for _, candidate := range candidates {
		results = append(results, chunkSearchResult(
			candidate, scoreChunk(candidate.document, candidate.chunk, terms), terms,
		))
	}
	sortSearchResults(results)
	if len(results) > maxResearchDisclosedChunks {
		results = results[:maxResearchDisclosedChunks]
	}
	documents := make([]ExternalDocument, 0, len(results))
	for _, result := range results {
		citation := result.Citations[0]
		locator := citation.Section
		if citation.StartLine > 0 {
			locator = fmt.Sprintf("%s，第 %d-%d 行", locator, citation.StartLine, citation.EndLine)
		}
		documents = append(documents, ExternalDocument{
			ID:       result.Chunk.ID,
			Filename: fmt.Sprintf("%s（%s）", citation.Title, locator),
			Content:  result.Chunk.Text,
		})
	}
	return documents, nil
}

func (s Service) loadChunkCandidates(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	documentIDs []string,
	terms []string,
) ([]chunkCandidate, error) {
	query := `SELECT
		c.id, c.document_id, c.ordinal, c.kind, c.text, COALESCE(d.source_uri, ''),
		c.section, c.page_number, c.start_line, c.end_line, c.text_sha256,
		c.locator_json, c.parser_code, c.parser_version, c.created_at,
		d.title, d.filename
		FROM platform_knowledge_chunks c
		INNER JOIN platform_knowledge_documents d
		  ON d.organization_id = c.organization_id
		 AND d.project_id = c.project_id
		 AND d.id = c.document_id
		WHERE c.organization_id = ? AND c.project_id = ? AND d.status = 'ready'`
	args := []any{organizationID, projectID}
	if len(documentIDs) > 0 {
		query += ` AND c.document_id IN (` + strings.TrimRight(strings.Repeat("?,", len(documentIDs)), ",") + `)`
		for _, id := range documentIDs {
			args = append(args, id)
		}
	}
	if len(terms) > 0 {
		if len(terms) > 12 {
			terms = terms[:12]
		}
		query += ` AND (`
		for index, term := range terms {
			if index > 0 {
				query += ` OR `
			}
			query += `LOCATE(?, LOWER(CONCAT(COALESCE(d.title, ''), '\n', c.text))) > 0`
			args = append(args, strings.ToLower(term))
		}
		query += `)`
	}
	query += ` ORDER BY c.document_id, c.ordinal LIMIT 2000`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	candidates := make([]chunkCandidate, 0)
	for rows.Next() {
		var candidate chunkCandidate
		var locator []byte
		var page sql.NullInt64
		err := rows.Scan(
			&candidate.chunk.ID, &candidate.chunk.DocumentID, &candidate.chunk.Index,
			&candidate.chunk.Kind, &candidate.chunk.Text, &candidate.chunk.SourceURI,
			&candidate.chunk.Section, &page, &candidate.chunk.StartLine, &candidate.chunk.EndLine,
			&candidate.chunk.TextSHA256, &locator, &candidate.chunk.ParserCode,
			&candidate.chunk.ParserVersion, &candidate.chunk.CreatedAt,
			&candidate.document.Title, &candidate.document.Filename,
		)
		if err != nil {
			return nil, err
		}
		candidate.chunk.OrganizationID = organizationID
		candidate.chunk.ProjectID = projectID
		candidate.document.ID = candidate.chunk.DocumentID
		candidate.document.OrganizationID = organizationID
		candidate.document.ProjectID = projectID
		if page.Valid {
			value := int(page.Int64)
			candidate.chunk.PageNumber = &value
		}
		if err := json.Unmarshal(locator, &candidate.chunk.Locator); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

func chunkSearchResult(candidate chunkCandidate, score int, terms []string) SearchResult {
	title := strings.TrimSpace(candidate.document.Title)
	if title == "" {
		title = candidate.document.Filename
	}
	return SearchResult{
		Chunk: candidate.chunk,
		Score: score,
		Citations: []Citation{{
			DocumentID: candidate.chunk.DocumentID,
			ChunkID:    candidate.chunk.ID,
			Title:      title,
			SourceURI:  candidate.chunk.SourceURI,
			Section:    candidate.chunk.Section,
			StartLine:  candidate.chunk.StartLine,
			EndLine:    candidate.chunk.EndLine,
			Snippet:    snippetFor(candidate.chunk.Text, terms),
		}},
	}
}

func sortSearchResults(results []SearchResult) {
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Chunk.DocumentID != results[j].Chunk.DocumentID {
			return results[i].Chunk.DocumentID < results[j].Chunk.DocumentID
		}
		return results[i].Chunk.Index < results[j].Chunk.Index
	})
}
