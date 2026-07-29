package knowledge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/shikanon/cookies/internal/platform/contract"
)

var ErrKnowledgeNotFound = errors.New("knowledge resource not found")
var ErrNotFound = ErrKnowledgeNotFound

type MemoryService struct {
	mu        sync.RWMutex
	documents map[string]Document
	chunks    map[string][]Chunk
	newID     func(string) (string, error)
	nowUTC    func() time.Time
}

func NewMemoryService(newID func(string) (string, error)) *MemoryService {
	if newID == nil {
		newID = func(prefix string) (string, error) {
			return fmt.Sprintf("%s_%d", prefix, time.Now().UTC().UnixNano()), nil
		}
	}
	return &MemoryService{
		documents: make(map[string]Document),
		chunks:    make(map[string][]Chunk),
		newID:     newID,
		nowUTC:    func() time.Time { return time.Now().UTC() },
	}
}

func (s *MemoryService) CreateDocument(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, filename, _ string, source io.Reader, size int64) (Document, error) {
	if source == nil || size < 1 || size > MaxDocumentBytes {
		return Document{}, ErrInvalidDocument
	}
	content, err := io.ReadAll(io.LimitReader(source, MaxDocumentBytes+1))
	if err != nil || int64(len(content)) != size {
		return Document{}, ErrInvalidDocument
	}
	return s.ImportDocument(ctx, actor, projectID, ImportDocumentRequest{
		Title: filename, SourceType: "docs", Text: string(content),
	})
}

func (s *MemoryService) ImportDocument(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request ImportDocumentRequest) (Document, error) {
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	if err := request.Validate(); err != nil {
		return Document{}, err
	}
	documentID, err := s.newID("knowledgedoc")
	if err != nil {
		return Document{}, err
	}
	now := s.nowUTC()
	parts := splitIntoChunks(request.Text)
	title := strings.TrimSpace(request.Title)
	document := Document{
		ID: documentID, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		Title: title, Filename: title + ".md", SourceURI: strings.TrimSpace(request.SourceURI),
		SourceType: normalizedSourceType(request.SourceType), ChunkCount: len(parts),
		ImportedBy: actor.Principal, CreatedBy: actor.Principal.ID, Status: "ready",
		CreatedAt: now, UpdatedAt: now,
	}
	chunks := make([]Chunk, 0, len(parts))
	for index, part := range parts {
		chunkID, err := s.newID("knowledgechunk")
		if err != nil {
			return Document{}, err
		}
		chunks = append(chunks, Chunk{
			ID: chunkID, DocumentID: documentID, OrganizationID: actor.OrganizationID,
			ProjectID: projectID, Index: index, Text: part.text, SourceURI: document.SourceURI,
			Section: part.section, StartLine: part.startLine, EndLine: part.endLine, CreatedAt: now,
		})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.documents[documentID] = document
	s.chunks[documentID] = chunks
	return document, nil
}

func (s *MemoryService) ListDocuments(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("limit must be between 1 and 100")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	documents := make([]Document, 0, len(s.documents))
	for _, document := range s.documents {
		if document.OrganizationID == actor.OrganizationID && document.ProjectID == projectID {
			documents = append(documents, document)
		}
	}
	sort.Slice(documents, func(i, j int) bool {
		if documents[i].CreatedAt.Equal(documents[j].CreatedAt) {
			return documents[i].ID < documents[j].ID
		}
		return documents[i].CreatedAt.After(documents[j].CreatedAt)
	})
	if len(documents) > limit {
		documents = documents[:limit]
	}
	return documents, nil
}

func (s *MemoryService) Search(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request SearchRequest) ([]SearchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request.Limit == 0 {
		request.Limit = 10
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	terms := tokenize(request.Query)
	s.mu.RLock()
	defer s.mu.RUnlock()
	results := make([]SearchResult, 0)
	for documentID, chunks := range s.chunks {
		document, ok := s.documents[documentID]
		if !ok || document.OrganizationID != actor.OrganizationID || document.ProjectID != projectID {
			continue
		}
		for _, chunk := range chunks {
			score := scoreChunk(document, chunk, terms)
			if score == 0 {
				continue
			}
			results = append(results, SearchResult{
				Chunk: chunk, Score: score,
				Citations: []Citation{{
					DocumentID: document.ID, ChunkID: chunk.ID, Title: document.Title,
					SourceURI: document.SourceURI, Section: chunk.Section, StartLine: chunk.StartLine,
					EndLine: chunk.EndLine, Snippet: snippetFor(chunk.Text, terms),
				}},
			})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Chunk.DocumentID != results[j].Chunk.DocumentID {
			return results[i].Chunk.DocumentID < results[j].Chunk.DocumentID
		}
		return results[i].Chunk.Index < results[j].Chunk.Index
	})
	if len(results) > request.Limit {
		results = results[:request.Limit]
	}
	return results, nil
}

func (s *MemoryService) RunResearch(context.Context, contract.ActorContext, contract.ProjectID, ResearchRequest) (ResearchRun, error) {
	return ResearchRun{}, ErrExternalRunnerUnavailable
}

func (s *MemoryService) GetResearchRun(context.Context, contract.ActorContext, contract.ProjectID, string) (ResearchRun, error) {
	return ResearchRun{}, ErrNotFound
}

func (s *MemoryService) ListResearchRuns(context.Context, contract.ActorContext, contract.ProjectID, int) ([]ResearchRun, error) {
	return []ResearchRun{}, nil
}

type chunkPart struct {
	text      string
	section   string
	startLine int
	endLine   int
}

func splitIntoChunks(text string) []chunkPart {
	const maxChunkRunes = 900
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var parts []chunkPart
	var builder strings.Builder
	currentSection := "正文"
	startLine := 1
	flush := func(endLine int) {
		value := strings.TrimSpace(builder.String())
		if value == "" {
			return
		}
		parts = append(parts, chunkPart{text: value, section: currentSection, startLine: startLine, endLine: endLine})
		builder.Reset()
	}
	for index, line := range lines {
		lineNumber := index + 1
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			if builder.Len() > 0 {
				flush(lineNumber - 1)
			}
			currentSection = strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if currentSection == "" {
				currentSection = "正文"
			}
			startLine = lineNumber
		}
		if builder.Len() == 0 {
			startLine = lineNumber
		}
		if builder.Len() > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(line)
		if len([]rune(builder.String())) >= maxChunkRunes {
			flush(lineNumber)
		}
	}
	if builder.Len() > 0 {
		flush(len(lines))
	}
	if len(parts) == 0 {
		return []chunkPart{{text: strings.TrimSpace(text), section: "正文", startLine: 1, endLine: 1}}
	}
	return parts
}

func normalizedSourceType(value string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return "docs"
}

func tokenize(value string) []string {
	fields := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
	})
	seen := make(map[string]struct{}, len(fields))
	terms := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if _, exists := seen[field]; exists {
			continue
		}
		seen[field] = struct{}{}
		terms = append(terms, field)
	}
	return terms
}

func scoreChunk(document Document, chunk Chunk, terms []string) int {
	haystack := strings.ToLower(document.Title + "\n" + chunk.Text)
	score := 0
	for _, term := range terms {
		score += strings.Count(haystack, term)
	}
	return score
}

func snippetFor(text string, terms []string) string {
	normalized := strings.Join(strings.Fields(text), " ")
	lowered := strings.ToLower(normalized)
	start := 0
	for _, term := range terms {
		if index := strings.Index(lowered, term); index >= 0 {
			start = index
			break
		}
	}
	if start > 48 {
		start -= 48
	}
	runes := []rune(normalized[start:])
	if len(runes) > 180 {
		runes = runes[:180]
	}
	return strings.TrimSpace(string(runes))
}
