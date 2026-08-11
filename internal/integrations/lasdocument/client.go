package lasdocument

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/knowledge"
	"github.com/shikanon/cookies/internal/platform/provider"
)

const (
	providerCode      = "volcengine_las"
	checkpointVersion = "las-document-checkpoint/v2"
	maxInputBytes     = int64(1 << 30)
	sourceURLTTL      = time.Hour
)

var (
	remoteMarkdownImagePattern = regexp.MustCompile(`!\[([^\]\r\n]*)\]\((https?://[^)\s]+)(?:\s+["'][^)\r\n]*["'])?\)`)
	httpURLPattern             = regexp.MustCompile(`https?://[^\s<>"')\]]+`)
)

type sourceURLSigner interface {
	SignGet(context.Context, assets.ObjectLocation, time.Duration) (assets.SignedRequest, error)
}

type Client struct {
	Routes            provider.DocumentVisionRouteResolver
	Credentials       provider.GatewayCredentialResolver
	SourceURLs        sourceURLSigner
	OutputBucket      string
	OutputPrefix      string
	AllowInsecureHTTP bool
	HTTPClient        *http.Client
	Now               func() time.Time
}

type checkpoint struct {
	Version       string                        `json:"version"`
	IntentID      string                        `json:"intent_id"`
	Route         provider.GatewayRouteSnapshot `json:"route"`
	PageNumbers   []int                         `json:"page_numbers"`
	SourceTOSPath string                        `json:"source_tos_path"`
	OutputTOSPath string                        `json:"output_tos_path"`
	PreparedAt    time.Time                     `json:"prepared_at"`
}

func (c Client) Inspect(ctx context.Context, organizationID contract.OrganizationID, modelAlias string) (knowledge.DocumentVisionCapability, error) {
	if strings.TrimSpace(c.OutputBucket) == "" || c.Credentials == nil || c.SourceURLs == nil {
		return knowledge.DocumentVisionCapability{ModelAlias: modelAlias, ReasonCode: "DOCUMENT_VISION_PROVIDER_DISABLED"}, fmt.Errorf("LAS document storage and credential resolvers are required")
	}
	route, err := c.resolveRoute(ctx, organizationID, modelAlias)
	if err != nil {
		return knowledge.DocumentVisionCapability{ModelAlias: modelAlias, ReasonCode: "DOCUMENT_VISION_ROUTE_UNAVAILABLE"}, err
	}
	if _, err := c.Credentials.ResolveGatewayCredential(ctx, route.CredentialID, route.CredentialVersion); err != nil {
		return knowledge.DocumentVisionCapability{ModelAlias: modelAlias, ReasonCode: "DOCUMENT_VISION_ROUTE_UNAVAILABLE"}, fmt.Errorf("resolve LAS credential: %w", err)
	}
	return knowledge.DocumentVisionCapability{
		Available: true, ModelAlias: modelAlias, UpstreamModel: route.UpstreamModel,
		RouteRevisionID: route.RouteRevisionID, SupportedMIMEs: []string{"application/pdf"},
	}, nil
}

func (c Client) PrepareSubmission(ctx context.Context, request knowledge.DocumentVisionParseRequest) (knowledge.DocumentVisionSubmissionIntent, error) {
	if err := c.validateRequest(request); err != nil {
		return knowledge.DocumentVisionSubmissionIntent{}, err
	}
	route, err := c.resolveRoute(ctx, request.OrganizationID, request.ModelAlias)
	if err != nil {
		return knowledge.DocumentVisionSubmissionIntent{}, err
	}
	outputPath := c.outputTOSPath(request)
	sourcePath := c.sourceTOSPath(request)
	intentID := c.submissionIntentID(request, route, outputPath)
	encodedCheckpoint, err := json.Marshal(checkpoint{
		Version: checkpointVersion, IntentID: intentID, Route: route,
		PageNumbers: append([]int(nil), request.PageNumbers...), SourceTOSPath: sourcePath,
		OutputTOSPath: outputPath, PreparedAt: c.now(),
	})
	if err != nil {
		return knowledge.DocumentVisionSubmissionIntent{}, err
	}
	return knowledge.DocumentVisionSubmissionIntent{
		IntentID: intentID, ProviderCode: providerCode, ModelVersion: modelVersion(route),
		RouteRevisionID: route.RouteRevisionID, Checkpoint: encodedCheckpoint,
	}, nil
}

func (c Client) SubmitPrepared(ctx context.Context, request knowledge.DocumentVisionParseRequest, intent knowledge.DocumentVisionSubmissionIntent) (knowledge.DocumentVisionSubmission, error) {
	if err := c.validateRequest(request); err != nil {
		return knowledge.DocumentVisionSubmission{}, err
	}
	saved, err := c.validatePreparedIntent(request, intent)
	if err != nil {
		return knowledge.DocumentVisionSubmission{}, err
	}
	token, err := c.Credentials.ResolveGatewayCredential(ctx, saved.Route.CredentialID, saved.Route.CredentialVersion)
	if err != nil {
		return knowledge.DocumentVisionSubmission{}, fmt.Errorf("resolve LAS credential: %w", err)
	}
	source, err := c.SourceURLs.SignGet(ctx, request.Object, sourceURLTTL)
	if err != nil {
		return knowledge.DocumentVisionSubmission{}, fmt.Errorf("sign LAS document source: %w", err)
	}
	if source.Method != http.MethodGet || strings.TrimSpace(source.URL) == "" ||
		(!c.AllowInsecureHTTP && !strings.HasPrefix(strings.ToLower(source.URL), "https://")) {
		return knowledge.DocumentVisionSubmission{}, fmt.Errorf("signed LAS document source is invalid")
	}
	payload := map[string]any{
		"operator_id": saved.Route.UpstreamModel, "operator_version": saved.Route.DocumentOperatorVersion,
		"data": map[string]any{
			"url": source.URL, "start_page": request.PageNumbers[0],
			"num_pages": len(request.PageNumbers), "parse_mode": saved.Route.DocumentParseMode,
			"full_result": saved.Route.DocumentFullResult, "aspect_ratio_threshold": saved.Route.DocumentAspectRatioThreshold,
		},
	}
	var response struct {
		Metadata struct {
			TaskID     string `json:"task_id"`
			Status     string `json:"status"`
			TaskStatus string `json:"task_status"`
		} `json:"metadata"`
	}
	if err := c.call(ctx, saved.Route, saved.Route.DocumentSubmitPath, token, payload, &response); err != nil {
		return knowledge.DocumentVisionSubmission{}, err
	}
	if strings.TrimSpace(response.Metadata.TaskID) == "" || len(response.Metadata.TaskID) > 160 {
		return knowledge.DocumentVisionSubmission{}, fmt.Errorf("LAS submit response did not contain a valid task id")
	}
	state := lasTaskStatus(response.Metadata.Status, response.Metadata.TaskStatus)
	if state != "" && state != "PENDING" && state != "RUNNING" {
		return knowledge.DocumentVisionSubmission{}, fmt.Errorf("LAS submit returned unexpected status %q", state)
	}
	return knowledge.DocumentVisionSubmission{
		ExternalTaskID: response.Metadata.TaskID, ProviderCode: providerCode,
		ModelVersion: modelVersion(saved.Route), RouteRevisionID: saved.Route.RouteRevisionID,
		Checkpoint: append(json.RawMessage(nil), intent.Checkpoint...), PollAfter: time.Duration(saved.Route.DocumentPollIntervalMS) * time.Millisecond,
	}, nil
}

func (c Client) Poll(ctx context.Context, submission knowledge.DocumentVisionSubmission) (knowledge.DocumentVisionPollResult, error) {
	var saved checkpoint
	if len(submission.Checkpoint) == 0 || json.Unmarshal(submission.Checkpoint, &saved) != nil || saved.Version != checkpointVersion {
		return knowledge.DocumentVisionPollResult{}, fmt.Errorf("LAS document checkpoint is invalid")
	}
	if err := saved.Route.ValidateDocumentVisionWithPolicy(c.AllowInsecureHTTP); err != nil {
		return knowledge.DocumentVisionPollResult{}, fmt.Errorf("LAS document checkpoint route is invalid: %w", err)
	}
	if submission.RouteRevisionID != saved.Route.RouteRevisionID || strings.TrimSpace(submission.ExternalTaskID) == "" {
		return knowledge.DocumentVisionPollResult{}, fmt.Errorf("LAS document checkpoint identity does not match the submission")
	}
	token, err := c.Credentials.ResolveGatewayCredential(ctx, saved.Route.CredentialID, saved.Route.CredentialVersion)
	if err != nil {
		return knowledge.DocumentVisionPollResult{}, fmt.Errorf("resolve LAS credential: %w", err)
	}
	payload := map[string]any{
		"operator_id": saved.Route.UpstreamModel, "operator_version": saved.Route.DocumentOperatorVersion,
		"task_id": submission.ExternalTaskID,
	}
	var response pollResponse
	if err := c.call(ctx, saved.Route, saved.Route.DocumentPollPath, token, payload, &response); err != nil {
		return knowledge.DocumentVisionPollResult{}, err
	}
	status := lasTaskStatus(response.Metadata.Status, response.Metadata.TaskStatus)
	pollAfter := time.Duration(saved.Route.DocumentPollIntervalMS) * time.Millisecond
	billable := response.Data.BillablePages
	switch status {
	case "PENDING":
		return knowledge.DocumentVisionPollResult{Status: knowledge.DocumentVisionPollPending, BillablePages: billable, PollAfter: pollAfter}, nil
	case "RUNNING":
		return knowledge.DocumentVisionPollResult{Status: knowledge.DocumentVisionPollRunning, BillablePages: billable, PollAfter: pollAfter}, nil
	case "FAILED", "TIMEOUT":
		code := "DOCUMENT_VISION_UPSTREAM_FAILED"
		message := "Volcengine LAS document parsing did not complete"
		if status == "TIMEOUT" {
			code = "DOCUMENT_VISION_UPSTREAM_TIMEOUT"
		}
		if businessCode := safeLASBusinessCode(response.Metadata.BusinessCode); businessCode != "" {
			message += " (" + businessCode + ")"
		}
		return knowledge.DocumentVisionPollResult{
			Status: knowledge.DocumentVisionPollFailed, ErrorCode: code,
			ErrorMessage: message, BillablePages: billable,
		}, nil
	case "COMPLETED", "SUCCESS", "SUCCEEDED":
		result, err := normalizePollResult(saved, response)
		if err != nil {
			return knowledge.DocumentVisionPollResult{}, err
		}
		result.Latency = c.now().Sub(saved.PreparedAt)
		if result.Latency < 0 {
			result.Latency = 0
		}
		return knowledge.DocumentVisionPollResult{Status: knowledge.DocumentVisionPollCompleted, Result: &result, BillablePages: billable}, nil
	default:
		return knowledge.DocumentVisionPollResult{}, fmt.Errorf("LAS poll returned unexpected status %q", status)
	}
}

func (c Client) validatePreparedIntent(request knowledge.DocumentVisionParseRequest, intent knowledge.DocumentVisionSubmissionIntent) (checkpoint, error) {
	var saved checkpoint
	if len(intent.Checkpoint) == 0 || json.Unmarshal(intent.Checkpoint, &saved) != nil || saved.Version != checkpointVersion {
		return checkpoint{}, fmt.Errorf("LAS document submission intent checkpoint is invalid")
	}
	if err := saved.Route.ValidateDocumentVisionWithPolicy(c.AllowInsecureHTTP); err != nil {
		return checkpoint{}, fmt.Errorf("LAS document submission intent route is invalid: %w", err)
	}
	expectedIntentID := c.submissionIntentID(request, saved.Route, c.outputTOSPath(request))
	if saved.IntentID != expectedIntentID || intent.IntentID != expectedIntentID ||
		intent.ProviderCode != providerCode || intent.ModelVersion != modelVersion(saved.Route) ||
		intent.RouteRevisionID != saved.Route.RouteRevisionID || saved.PreparedAt.IsZero() ||
		saved.SourceTOSPath != c.sourceTOSPath(request) || saved.OutputTOSPath != c.outputTOSPath(request) ||
		!equalPages(saved.PageNumbers, request.PageNumbers) {
		return checkpoint{}, fmt.Errorf("LAS document submission intent does not match the request")
	}
	return saved, nil
}

func equalPages(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

type pollResponse struct {
	Metadata struct {
		// LAS uses task_status in its live async operator contract. Status is
		// retained for compatibility with earlier gateways and recorded fixtures.
		Status       string `json:"status"`
		TaskStatus   string `json:"task_status"`
		BusinessCode string `json:"business_code"`
		ErrorMessage string `json:"error_msg"`
	} `json:"metadata"`
	Data struct {
		Markdown      string       `json:"markdown"`
		Detail        []pageDetail `json:"detail"`
		NumPages      *int         `json:"num_pages"`
		BillablePages *int         `json:"billable_pages"`
	} `json:"data"`
}

func lasTaskStatus(status, taskStatus string) string {
	if value := strings.TrimSpace(taskStatus); value != "" {
		return strings.ToUpper(value)
	}
	return strings.ToUpper(strings.TrimSpace(status))
}

func safeLASBusinessCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 96 {
		return ""
	}
	for _, character := range value {
		if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '.' || character == '_' || character == '-') {
			return ""
		}
	}
	return value
}

type pageDetail struct {
	PageID     int         `json:"page_id"`
	PageMD     string      `json:"page_md"`
	TextBlocks []textBlock `json:"text_blocks"`
}

type textBlock struct {
	Text  string          `json:"text"`
	Label string          `json:"label"`
	Box   json.RawMessage `json:"box"`
}

func normalizePollResult(saved checkpoint, response pollResponse) (knowledge.DocumentVisionParseResult, error) {
	pages := make([]knowledge.DocumentVisionPage, 0, len(response.Data.Detail))
	selected := make(map[int]struct{}, len(saved.PageNumbers))
	for _, page := range saved.PageNumbers {
		selected[page] = struct{}{}
	}
	for _, detail := range response.Data.Detail {
		pageNumber := detail.PageID
		if _, ok := selected[pageNumber]; !ok && detail.PageID >= 1 && detail.PageID <= len(saved.PageNumbers) {
			pageNumber = saved.PageNumbers[detail.PageID-1]
		}
		if _, ok := selected[pageNumber]; !ok {
			return knowledge.DocumentVisionParseResult{}, fmt.Errorf("LAS returned page %d outside the submitted page range", detail.PageID)
		}
		markdown := sanitizeLASMarkdown(detail.PageMD)
		if markdown == "" {
			continue
		}
		boxes := make([]any, 0, len(detail.TextBlocks))
		for _, block := range detail.TextBlocks {
			if box, ok := normalizeTextBlockBox(block.Box); ok {
				coordinates := make([]any, len(box))
				for index, coordinate := range box {
					coordinates[index] = coordinate
				}
				boxes = append(boxes, map[string]any{"label": bounded(block.Label, 64), "box": coordinates})
			}
		}
		locator := map[string]any{"page_number": pageNumber}
		if len(boxes) > 0 {
			locator["bounding_boxes"] = boxes
		}
		pages = append(pages, knowledge.DocumentVisionPage{PageNumber: pageNumber, Markdown: markdown, Locator: locator})
	}
	if len(pages) == 0 && len(saved.PageNumbers) == 1 && strings.TrimSpace(response.Data.Markdown) != "" {
		pageNumber := saved.PageNumbers[0]
		pages = append(pages, knowledge.DocumentVisionPage{
			PageNumber: pageNumber, Markdown: sanitizeLASMarkdown(response.Data.Markdown),
			Locator: map[string]any{"page_number": pageNumber},
		})
	}
	if len(pages) == 0 {
		return knowledge.DocumentVisionParseResult{}, fmt.Errorf("LAS completed without page-level markdown")
	}
	sort.Slice(pages, func(left, right int) bool { return pages[left].PageNumber < pages[right].PageNumber })
	usage, _ := json.Marshal(map[string]any{"num_pages": response.Data.NumPages, "billable_pages": response.Data.BillablePages})
	return knowledge.DocumentVisionParseResult{
		ProviderCode: providerCode, ModelVersion: modelVersion(saved.Route),
		RouteRevisionID: saved.Route.RouteRevisionID, Pages: pages, Usage: usage,
	}, nil
}

func sanitizeLASMarkdown(markdown string) string {
	markdown = remoteMarkdownImagePattern.ReplaceAllStringFunc(markdown, func(image string) string {
		matches := remoteMarkdownImagePattern.FindStringSubmatch(image)
		if len(matches) < 2 {
			return "[文档图片已省略，请在原文预览中查看]"
		}
		alt := strings.TrimSpace(matches[1])
		if alt == "" {
			return "[文档图片已省略，请在原文预览中查看]"
		}
		return fmt.Sprintf("[文档图片“%s”已省略，请在原文预览中查看]", bounded(alt, 120))
	})
	markdown = httpURLPattern.ReplaceAllStringFunc(markdown, func(raw string) string {
		if isTemporaryLASResource(raw) {
			return "[LAS 临时资源已省略]"
		}
		return raw
	})
	return strings.TrimSpace(markdown)
}

func isTemporaryLASResource(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" {
		return false
	}
	for key := range parsed.Query() {
		if strings.HasPrefix(strings.ToLower(key), "x-tos-") {
			return true
		}
	}
	host := strings.ToLower(parsed.Hostname())
	return strings.HasSuffix(host, ".volces.com") && strings.Contains(strings.ToLower(parsed.Path), "/las-serving-tmp/")
}

func normalizeTextBlockBox(raw json.RawMessage) ([]int, bool) {
	if len(raw) == 0 || len(raw) > 256 || string(raw) == "null" {
		return nil, false
	}
	var coordinates []int
	if json.Unmarshal(raw, &coordinates) != nil || len(coordinates) != 4 {
		var value struct {
			X0 *int `json:"x0"`
			Y0 *int `json:"y0"`
			X1 *int `json:"x1"`
			Y1 *int `json:"y1"`
		}
		if json.Unmarshal(raw, &value) != nil || value.X0 == nil || value.Y0 == nil || value.X1 == nil || value.Y1 == nil {
			return nil, false
		}
		coordinates = []int{*value.X0, *value.Y0, *value.X1, *value.Y1}
	}
	for _, coordinate := range coordinates {
		if coordinate < 0 || coordinate > 1000 {
			return nil, false
		}
	}
	if coordinates[0] > coordinates[2] || coordinates[1] > coordinates[3] {
		return nil, false
	}
	return coordinates, true
}

func (c Client) validateRequest(request knowledge.DocumentVisionParseRequest) error {
	if c.Routes == nil || c.Credentials == nil || c.SourceURLs == nil {
		return fmt.Errorf("LAS route, credential, and source URL resolvers are required")
	}
	if strings.TrimSpace(c.OutputBucket) == "" || request.Object.Provider != "tos" ||
		request.Object.Bucket != c.OutputBucket || strings.TrimSpace(request.Object.Key) == "" {
		return fmt.Errorf("LAS document parsing requires one configured TOS bucket for input and output")
	}
	if request.MIMEType != "application/pdf" {
		return fmt.Errorf("LAS document parsing currently accepts PDF input only")
	}
	if !safeTOSBucket(request.Object.Bucket) || !safeTOSKey(request.Object.Key) ||
		!strings.HasPrefix(request.Object.Key, c.inputScopePrefix(request)) ||
		!safeTOSKey(c.outputScopePrefix(request)+"/output") {
		return fmt.Errorf("LAS document TOS scope is invalid")
	}
	if request.SizeBytes < 1 || request.SizeBytes > maxInputBytes || len(request.PageNumbers) < 1 {
		return fmt.Errorf("LAS document input size or page range is invalid")
	}
	for index, page := range request.PageNumbers {
		if page < 1 || index > 0 && page != request.PageNumbers[index-1]+1 {
			return fmt.Errorf("LAS document submission pages must form one contiguous range")
		}
	}
	return nil
}

func (c Client) resolveRoute(ctx context.Context, organizationID contract.OrganizationID, modelAlias string) (provider.GatewayRouteSnapshot, error) {
	if c.Routes == nil {
		return provider.GatewayRouteSnapshot{}, fmt.Errorf("LAS document route resolver is required")
	}
	route, err := c.Routes.ResolveDocumentVisionRoute(ctx, organizationID, modelAlias)
	if err != nil {
		return provider.GatewayRouteSnapshot{}, err
	}
	if err := route.ValidateDocumentVisionWithPolicy(c.AllowInsecureHTTP); err != nil {
		return provider.GatewayRouteSnapshot{}, err
	}
	return route, nil
}

func (c Client) call(ctx context.Context, route provider.GatewayRouteSnapshot, path, token string, payload any, target any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(route.TimeoutSeconds)*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, strings.TrimRight(route.BaseURL, "/")+path, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("LAS request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("LAS request returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, route.MaxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read LAS response: %w", err)
	}
	if int64(len(body)) > route.MaxResponseBytes {
		return fmt.Errorf("LAS response exceeded the configured response limit")
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode LAS response: %w", err)
	}
	return nil
}

func (c Client) sourceTOSPath(request knowledge.DocumentVisionParseRequest) string {
	return "tos://" + request.Object.Bucket + "/" + strings.TrimPrefix(request.Object.Key, "/")
}

func (c Client) outputTOSPath(request knowledge.DocumentVisionParseRequest) string {
	prefix := c.outputScopePrefix(request)
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%v|%s", request.OrganizationID, request.ProjectID, request.DocumentID, request.PageNumbers, request.Object.ETag)))
	return "tos://" + c.OutputBucket + "/" + prefix + "/" + hex.EncodeToString(digest[:16]) + "/"
}

func (c Client) submissionIntentID(request knowledge.DocumentVisionParseRequest, route provider.GatewayRouteSnapshot, outputPath string) string {
	encoded, _ := json.Marshal(struct {
		OrganizationID  string `json:"organization_id"`
		ProjectID       string `json:"project_id"`
		DocumentID      string `json:"document_id"`
		ModelAlias      string `json:"model_alias"`
		RouteRevisionID string `json:"route_revision_id"`
		SourceProvider  string `json:"source_provider"`
		SourceBucket    string `json:"source_bucket"`
		SourceKey       string `json:"source_key"`
		SourceVersionID string `json:"source_version_id"`
		SourceETag      string `json:"source_etag"`
		PageNumbers     []int  `json:"page_numbers"`
		OutputPath      string `json:"output_path"`
	}{
		OrganizationID: string(request.OrganizationID), ProjectID: string(request.ProjectID),
		DocumentID: request.DocumentID, ModelAlias: request.ModelAlias, RouteRevisionID: route.RouteRevisionID,
		SourceProvider: request.Object.Provider, SourceBucket: request.Object.Bucket, SourceKey: request.Object.Key,
		SourceVersionID: request.Object.VersionID, SourceETag: request.Object.ETag,
		PageNumbers: append([]int(nil), request.PageNumbers...), OutputPath: outputPath,
	})
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func (c Client) outputPrefix() string {
	prefix := strings.Trim(strings.TrimSpace(c.OutputPrefix), "/")
	if prefix == "" {
		return "provider-output/document-vision"
	}
	return prefix
}

func (c Client) inputScopePrefix(request knowledge.DocumentVisionParseRequest) string {
	return fmt.Sprintf("assets/%s/%s/knowledge/%s/", request.OrganizationID, request.ProjectID, request.DocumentID)
}

func (c Client) outputScopePrefix(request knowledge.DocumentVisionParseRequest) string {
	return fmt.Sprintf("%s/%s/%s/%s", c.outputPrefix(), request.OrganizationID, request.ProjectID, request.DocumentID)
}

func (c Client) now() time.Time {
	if c.Now != nil {
		return c.Now().UTC()
	}
	return time.Now().UTC()
}

func modelVersion(route provider.GatewayRouteSnapshot) string {
	return route.UpstreamModel + "@" + route.DocumentOperatorVersion
}

func bounded(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) > limit {
		return value[:limit]
	}
	return value
}

func safeTOSBucket(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= 255 && !strings.ContainsAny(value, "/\\") && value != "." && value != ".."
}

func safeTOSKey(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= 1024 && !strings.HasPrefix(value, "/") &&
		!strings.Contains(value, "..") && !strings.Contains(value, "\\")
}
