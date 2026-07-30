package knowledge

import (
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	ScopeRead  contract.Scope = "assets.read"
	ScopeWrite contract.Scope = "assets.write"
)

type Chunk struct {
	ID             string                  `json:"id"`
	DocumentID     string                  `json:"document_id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	Index          int                     `json:"index"`
	Text           string                  `json:"text"`
	SourceURI      string                  `json:"source_uri"`
	Section        string                  `json:"section"`
	StartLine      int                     `json:"start_line"`
	EndLine        int                     `json:"end_line"`
	CreatedAt      time.Time               `json:"created_at"`
}

type Citation struct {
	DocumentID string `json:"document_id"`
	ChunkID    string `json:"chunk_id"`
	Title      string `json:"title"`
	SourceURI  string `json:"source_uri"`
	Section    string `json:"section"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	Snippet    string `json:"snippet"`
}

type ImportDocumentRequest struct {
	Title      string `json:"title"`
	SourceURI  string `json:"source_uri"`
	SourceType string `json:"source_type"`
	Text       string `json:"text"`
}

type SearchRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

type SearchResult struct {
	Chunk     Chunk      `json:"chunk"`
	Score     int        `json:"score"`
	Citations []Citation `json:"citations"`
}

func (r ImportDocumentRequest) Validate() error {
	if strings.TrimSpace(r.Title) == "" || len([]rune(r.Title)) > 160 {
		return fmt.Errorf("title must be between 1 and 160 characters")
	}
	if strings.TrimSpace(r.Text) == "" {
		return fmt.Errorf("text is required")
	}
	if len([]rune(r.Text)) > 200_000 {
		return fmt.Errorf("text is too large")
	}
	if len([]rune(r.SourceURI)) > 512 {
		return fmt.Errorf("source_uri is too long")
	}
	if r.SourceType != "" && r.SourceType != "docs" && r.SourceType != "strategy" &&
		r.SourceType != "retrospective" && r.SourceType != "feishu_summary" &&
		r.SourceType != "prelaunch_insight" {
		return fmt.Errorf("source_type is invalid")
	}
	return nil
}

func (r SearchRequest) Validate() error {
	if strings.TrimSpace(r.Query) == "" {
		return fmt.Errorf("query is required")
	}
	if len([]rune(r.Query)) > 160 {
		return fmt.Errorf("query is too long")
	}
	if r.Limit < 0 || r.Limit > 50 {
		return fmt.Errorf("limit must be between 0 and 50")
	}
	return nil
}
