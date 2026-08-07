package seedresearch

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/shikanon/cookies/internal/platform/knowledge"
)

type seedRequest struct {
	Model string      `json:"model"`
	Store bool        `json:"store"`
	Tools []seedTool  `json:"tools"`
	Input []seedInput `json:"input"`
}

type seedTool struct {
	Type string `json:"type"`
}

type seedInput struct {
	Role    string             `json:"role"`
	Content []seedInputContent `json:"content"`
}

type seedInputContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type seedResponse struct {
	ID     string `json:"id"`
	Model  string `json:"model"`
	Status string `json:"status"`
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type        string            `json:"type"`
			Text        string            `json:"text"`
			Annotations []json.RawMessage `json:"annotations"`
		} `json:"content"`
	} `json:"output"`
	Usage struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
		TotalTokens  int64 `json:"total_tokens"`
	} `json:"usage"`
}

type seedURLCitation struct {
	Type        string `json:"type"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	StartIndex  int    `json:"start_index"`
	EndIndex    int    `json:"end_index"`
	URLCitation *struct {
		URL        string `json:"url"`
		Title      string `json:"title"`
		StartIndex int    `json:"start_index"`
		EndIndex   int    `json:"end_index"`
	} `json:"url_citation,omitempty"`
}

func decodeSeedResponse(body []byte, input knowledge.ExternalResearchInput) (knowledge.ExternalResearchResult, error) {
	var response seedResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return knowledge.ExternalResearchResult{}, fmt.Errorf("Seed web research response is invalid")
	}
	if response.Status != "" && response.Status != "completed" {
		return knowledge.ExternalResearchResult{}, fmt.Errorf("Seed web research response did not complete")
	}
	var content strings.Builder
	sources := make([]knowledge.ExternalResearchSource, 0)
	seen := map[string]struct{}{}
	for _, output := range response.Output {
		if output.Type != "message" {
			continue
		}
		for _, item := range output.Content {
			if item.Type != "output_text" || strings.TrimSpace(item.Text) == "" {
				continue
			}
			if content.Len() > 0 {
				content.WriteString("\n\n")
			}
			content.WriteString(strings.TrimSpace(item.Text))
			for _, raw := range item.Annotations {
				source, ok := decodeURLCitation(raw)
				if !ok {
					continue
				}
				key := fmt.Sprintf("%s|%d|%d", source.URL, source.StartIndex, source.EndIndex)
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				sources = append(sources, source)
			}
		}
	}
	text := strings.TrimSpace(content.String())
	if text == "" {
		return knowledge.ExternalResearchResult{}, fmt.Errorf("Seed web research returned no report")
	}
	if len(sources) == 0 {
		return knowledge.ExternalResearchResult{}, fmt.Errorf("Seed web research returned no cited sources")
	}
	citations := make([]string, 0, len(sources))
	for _, source := range sources {
		citations = append(citations, source.URL)
	}
	title := "联网研究"
	switch input.Category {
	case "audience":
		title = "受众联网研究"
	case "competitor":
		title = "竞品联网研究"
	case "industry":
		title = "行业联网研究"
	}
	title = researchTitle(input.Category)
	usage := &knowledge.ResearchUsage{
		InputTokens: response.Usage.InputTokens, OutputTokens: response.Usage.OutputTokens,
		TotalTokens: response.Usage.TotalTokens,
	}
	return knowledge.ExternalResearchResult{
		Title: title, SourceURL: sources[0].URL, Content: text, Citations: citations,
		Sources: sources, ProviderCode: "ark", ModelVersion: strings.TrimSpace(response.Model),
		ProviderResponse: strings.TrimSpace(response.ID), Usage: usage,
	}, nil
}

func researchTitle(category string) string {
	switch category {
	case "audience":
		return "受众联网研究"
	case "competitor":
		return "竞品联网研究"
	case "industry":
		return "行业联网研究"
	default:
		return "联网研究"
	}
}

func decodeURLCitation(raw json.RawMessage) (knowledge.ExternalResearchSource, bool) {
	var citation seedURLCitation
	if err := json.Unmarshal(raw, &citation); err != nil {
		return knowledge.ExternalResearchSource{}, false
	}
	if citation.URLCitation != nil {
		citation.URL = citation.URLCitation.URL
		citation.Title = citation.URLCitation.Title
		citation.StartIndex = citation.URLCitation.StartIndex
		citation.EndIndex = citation.URLCitation.EndIndex
	}
	if citation.Type != "" && citation.Type != "url_citation" {
		return knowledge.ExternalResearchSource{}, false
	}
	citation.URL = strings.TrimSpace(citation.URL)
	parsedURL, err := url.Parse(citation.URL)
	if err != nil || len(citation.URL) > 2048 ||
		(parsedURL.Scheme != "http" && parsedURL.Scheme != "https") ||
		parsedURL.Host == "" || parsedURL.User != nil {
		return knowledge.ExternalResearchSource{}, false
	}
	sourceClass, mediaType := inferSourceClass(citation.URL)
	return knowledge.ExternalResearchSource{
		SourceClass: sourceClass, MediaType: mediaType, Title: truncateRunes(strings.TrimSpace(citation.Title), 512),
		URL: citation.URL, StartIndex: citation.StartIndex, EndIndex: citation.EndIndex,
		ProviderLocator: append(json.RawMessage(nil), raw...),
	}, true
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
