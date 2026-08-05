package knowledge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

type DocumentParseRequest struct {
	Filename string
	MIMEType string
	Size     int64
	Source   io.Reader
}

type ParsedDocument struct {
	Text          string
	MIMEType      string
	ParserCode    string
	ParserVersion string
	Metadata      json.RawMessage
}

type DocumentParser interface {
	Parse(context.Context, DocumentParseRequest) (ParsedDocument, error)
}

type TikaParser struct {
	BaseURL        string
	Version        string
	Timeout        time.Duration
	MaxOutputBytes int64
	HTTPClient     *http.Client
}

func (p TikaParser) Parse(ctx context.Context, input DocumentParseRequest) (ParsedDocument, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	if baseURL == "" || input.Source == nil || input.Size < 1 || strings.TrimSpace(input.MIMEType) == "" {
		return ParsedDocument{}, fmt.Errorf("Tika parse request is invalid")
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	limit := p.MaxOutputBytes
	if limit < 1 {
		limit = maxExtractedBytes
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPut, baseURL+"/rmeta/text", input.Source)
	if err != nil {
		return ParsedDocument{}, fmt.Errorf("build Tika request: %w", err)
	}
	request.Header.Set("Content-Type", input.MIMEType)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{
		"filename": input.Filename,
	}))
	request.Header.Set("maxEmbeddedResources", "0")
	request.Header.Set("writeLimit", fmt.Sprint(limit))
	client := p.HTTPClient
	if client == nil {
		client = &http.Client{}
	}
	response, err := client.Do(request)
	if err != nil {
		if requestCtx.Err() == context.DeadlineExceeded {
			return ParsedDocument{}, fmt.Errorf("Tika parsing timed out")
		}
		return ParsedDocument{}, fmt.Errorf("Tika parsing request failed")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil || int64(len(body)) > limit {
		return ParsedDocument{}, fmt.Errorf("Tika parsing response exceeded the safety limit")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return ParsedDocument{}, fmt.Errorf("Tika parsing returned HTTP %d", response.StatusCode)
	}
	var records []map[string]any
	if err := json.Unmarshal(body, &records); err != nil || len(records) == 0 {
		return ParsedDocument{}, fmt.Errorf("Tika parsing response is invalid")
	}
	contents := make([]string, 0, len(records))
	for _, record := range records {
		if value, ok := record["X-TIKA:content"].(string); ok && strings.TrimSpace(value) != "" {
			contents = append(contents, strings.TrimSpace(value))
		}
		delete(record, "X-TIKA:content")
	}
	text := strings.TrimSpace(strings.Join(contents, "\n\n"))
	if text == "" || !utf8.ValidString(text) {
		return ParsedDocument{}, fmt.Errorf("Tika parsing returned no valid text")
	}
	metadata, _ := json.Marshal(records)
	version := strings.TrimSpace(p.Version)
	if version == "" {
		version = "unknown"
	}
	return ParsedDocument{
		Text: text, MIMEType: input.MIMEType,
		ParserCode: "tika", ParserVersion: version, Metadata: metadata,
	}, nil
}

func chunksForParsedDocument(document Document, parsed ParsedDocument) []Chunk {
	const targetRunes = 800
	lines := strings.Split(strings.ReplaceAll(parsed.Text, "\r\n", "\n"), "\n")
	chunks := make([]Chunk, 0)
	var buffer bytes.Buffer
	startLine := 1
	flush := func(endLine int) {
		text := strings.TrimSpace(buffer.String())
		buffer.Reset()
		if text == "" {
			startLine = endLine + 1
			return
		}
		sum := sha256.Sum256([]byte(text))
		textHash := hex.EncodeToString(sum[:])
		ordinal := len(chunks)
		idInput := fmt.Sprintf("%s|%s|%s|%d|%s",
			document.ContentSHA256, parsed.ParserCode, parsed.ParserVersion, ordinal, textHash)
		idSum := sha256.Sum256([]byte(idInput))
		chunks = append(chunks, Chunk{
			ID:         "knowledgechunk_" + hex.EncodeToString(idSum[:24]),
			DocumentID: document.ID, OrganizationID: document.OrganizationID, ProjectID: document.ProjectID,
			Index: ordinal, Kind: "text", Text: text, SourceURI: document.SourceURI,
			Section: "正文", StartLine: startLine, EndLine: endLine,
			TextSHA256: textHash, Locator: map[string]any{
				"section": "正文", "start_line": startLine, "end_line": endLine,
			},
			ParserCode: parsed.ParserCode, ParserVersion: parsed.ParserVersion,
			CreatedAt: document.UpdatedAt,
		})
		startLine = endLine + 1
	}
	for index, line := range lines {
		lineNumber := index + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if buffer.Len() > 0 && utf8.RuneCount(buffer.Bytes()) >= targetRunes/2 {
				flush(lineNumber)
			}
			continue
		}
		lineRunes := []rune(trimmed)
		for len(lineRunes) > 0 {
			take := min(len(lineRunes), targetRunes)
			segment := string(lineRunes[:take])
			lineRunes = lineRunes[take:]
			if buffer.Len() > 0 &&
				utf8.RuneCount(buffer.Bytes())+1+utf8.RuneCountInString(segment) > targetRunes {
				flush(max(lineNumber-1, startLine))
				startLine = lineNumber
			}
			if buffer.Len() > 0 {
				buffer.WriteByte('\n')
			}
			buffer.WriteString(segment)
			if len(lineRunes) > 0 || utf8.RuneCount(buffer.Bytes()) >= targetRunes {
				flush(lineNumber)
				startLine = lineNumber
			}
		}
	}
	if buffer.Len() > 0 {
		flush(len(lines))
	}
	return chunks
}
