package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

const DocumentVisionFallbackContractVersion = "platform-document-vision-fallback/v1"

const (
	maxDocumentVisionPageBytes    = 2 * 1024 * 1024
	maxDocumentVisionResultBytes  = 8 * 1024 * 1024
	maxDocumentVisionUsageBytes   = 64 * 1024
	maxDocumentVisionLocatorBytes = 64 * 1024
)

var ErrDocumentVisionUnavailable = errors.New("document vision fallback is unavailable")
var ErrDocumentVisionIneligible = errors.New("document is not eligible for visual fallback")
var ErrDocumentVisionPageSelectionRequired = errors.New("document visual fallback requires an explicit page selection")
var ErrDocumentVisionReconciliationRequired = errors.New("document vision fallback requires external task reconciliation")

type DocumentVisionCapability struct {
	Available       bool
	ReasonCode      string
	ModelAlias      string
	UpstreamModel   string
	RouteRevisionID string
	SupportedMIMEs  []string
}

type DocumentVisionParseRequest struct {
	OrganizationID contract.OrganizationID
	ProjectID      contract.ProjectID
	DocumentID     string
	Filename       string
	MIMEType       string
	SizeBytes      int64
	Source         io.Reader
	Object         assets.ObjectLocation
	PageNumbers    []int
	ModelAlias     string
}

type DocumentVisionPage struct {
	PageNumber int
	Markdown   string
	Locator    map[string]any
}

type DocumentVisionParseResult struct {
	ProviderCode    string
	ModelVersion    string
	RouteRevisionID string
	Pages           []DocumentVisionPage
	Usage           json.RawMessage
	Latency         time.Duration
}

type DocumentVisionSubmission struct {
	ExternalTaskID  string
	ProviderCode    string
	ModelVersion    string
	RouteRevisionID string
	Checkpoint      json.RawMessage
	PollAfter       time.Duration
}

// DocumentVisionSubmissionIntent is frozen before the first paid network
// call. Its opaque checkpoint gives an operator enough immutable provider
// lineage to reconcile a timeout without reconstructing the request from
// mutable runtime configuration.
type DocumentVisionSubmissionIntent struct {
	IntentID        string
	ProviderCode    string
	ModelVersion    string
	RouteRevisionID string
	Checkpoint      json.RawMessage
}

type DocumentVisionPollStatus string

const (
	DocumentVisionPollPending   DocumentVisionPollStatus = "pending"
	DocumentVisionPollRunning   DocumentVisionPollStatus = "running"
	DocumentVisionPollCompleted DocumentVisionPollStatus = "completed"
	DocumentVisionPollFailed    DocumentVisionPollStatus = "failed"
)

type DocumentVisionPollResult struct {
	Status        DocumentVisionPollStatus
	Result        *DocumentVisionParseResult
	ErrorCode     string
	ErrorMessage  string
	BillablePages *int
	PollAfter     time.Duration
}

// DocumentVisionParser is the cookies-owned normalization seam for LAS/Seed
// document parsing. Vendor request bodies, signed URLs, and raw responses must
// not cross this interface.
type DocumentVisionParser interface {
	Inspect(context.Context, contract.OrganizationID, string) (DocumentVisionCapability, error)
	PrepareSubmission(context.Context, DocumentVisionParseRequest) (DocumentVisionSubmissionIntent, error)
	SubmitPrepared(context.Context, DocumentVisionParseRequest, DocumentVisionSubmissionIntent) (DocumentVisionSubmission, error)
	Poll(context.Context, DocumentVisionSubmission) (DocumentVisionPollResult, error)
}

type documentVisionTask struct {
	AttemptID       string
	TaskIndex       int
	PageNumbers     []int
	Status          string
	IntentID        string
	ExternalTaskID  string
	ProviderCode    string
	ModelAlias      string
	ModelVersion    string
	RouteRevisionID string
	Checkpoint      json.RawMessage
	BillablePages   *int
	ErrorCode       string
	ErrorMessage    string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type DocumentVisionFallbackScheduler interface {
	ScheduleDocumentVisionFallback(context.Context, Document, []int, string) error
}

type RunDocumentVisionFallbackRequest struct {
	PageNumbers []int `json:"page_numbers"`
}

type DocumentVisionFallbackCapabilityView struct {
	ContractVersion       string `json:"contract_version"`
	DocumentID            string `json:"document_id"`
	Eligible              bool   `json:"eligible"`
	Recommended           bool   `json:"recommended"`
	Available             bool   `json:"available"`
	ReasonCode            string `json:"reason_code,omitempty"`
	ModelAlias            string `json:"model_alias"`
	RouteRevisionID       string `json:"route_revision_id,omitempty"`
	ConversionRequired    bool   `json:"conversion_required"`
	ConverterCode         string `json:"converter_code,omitempty"`
	ConverterVersion      string `json:"converter_version,omitempty"`
	MaxPagesPerRequest    int    `json:"max_pages_per_request"`
	RequiresPageSelection bool   `json:"requires_page_selection"`
}

func (s Service) GetDocumentVisionFallbackCapability(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	documentID string,
) (DocumentVisionFallbackCapabilityView, error) {
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return DocumentVisionFallbackCapabilityView{}, err
	}
	document, err := s.GetDocument(ctx, actor, projectID, documentID)
	if err != nil {
		return DocumentVisionFallbackCapabilityView{}, err
	}
	view := DocumentVisionFallbackCapabilityView{
		ContractVersion: DocumentVisionFallbackContractVersion, DocumentID: document.ID,
		Eligible: documentVisionEligible(document), Recommended: document.QualityTier == "low",
		ModelAlias: strings.TrimSpace(s.VisionModelAlias), MaxPagesPerRequest: 24,
		RequiresPageSelection: document.TotalPages == nil || *document.TotalPages > 24,
	}
	if !view.Eligible {
		view.ReasonCode = "DOCUMENT_VISION_INELIGIBLE"
		return view, nil
	}
	if s.DocumentVision == nil || s.VisionScheduler == nil || view.ModelAlias == "" {
		view.ReasonCode = "DOCUMENT_VISION_PROVIDER_DISABLED"
		return view, nil
	}
	capability, inspectErr := s.DocumentVision.Inspect(ctx, actor.OrganizationID, view.ModelAlias)
	if inspectErr != nil {
		view.ReasonCode = "DOCUMENT_VISION_ROUTE_UNAVAILABLE"
		return view, nil
	}
	view.Available = capability.Available
	view.ReasonCode = capability.ReasonCode
	view.RouteRevisionID = capability.RouteRevisionID
	if view.Available && !knowledgeDocumentBlobInScope(document, s.AssetsBucket) {
		view.Available = false
		view.ReasonCode = "DOCUMENT_VISION_STORAGE_SCOPE_INVALID"
	}
	if view.Available && !documentVisionSupportsMIME(capability, document.MIMEType) {
		view.ConversionRequired = documentVisionNeedsConversion(document.MIMEType)
		converter, converterErr := s.inspectDocumentVisionConverter(ctx, capability, document.MIMEType)
		view.Available = converterErr == nil && converter.Available
		view.ConverterCode = converter.ConverterCode
		view.ConverterVersion = converter.Version
		view.ReasonCode = converter.ReasonCode
		if !view.Available && view.ReasonCode == "" {
			view.ReasonCode = "DOCUMENT_VISION_CONVERTER_DISABLED"
		}
	}
	if !view.Available && view.ReasonCode == "" {
		view.ReasonCode = "DOCUMENT_VISION_ROUTE_UNAVAILABLE"
	}
	return view, nil
}

func (s Service) RunDocumentVisionFallback(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	documentID string,
	request RunDocumentVisionFallbackRequest,
) (Document, error) {
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return Document{}, err
	}
	document, err := s.GetDocument(ctx, actor, projectID, documentID)
	if err != nil {
		return Document{}, err
	}
	if document.VisionFallbackStatus == "queued" || document.VisionFallbackStatus == "running" {
		return document, nil
	}
	if document.VisionFallbackStatus == "succeeded" {
		return document, nil
	}
	if document.VisionFallbackStatus == "failed" && documentVisionRequiresReconciliation(document.VisionErrorCode) {
		return Document{}, ErrDocumentVisionReconciliationRequired
	}
	if !documentVisionEligible(document) {
		return Document{}, ErrDocumentVisionIneligible
	}
	if s.DocumentVision == nil || s.VisionScheduler == nil || strings.TrimSpace(s.VisionModelAlias) == "" {
		return Document{}, ErrDocumentVisionUnavailable
	}
	capability, err := s.DocumentVision.Inspect(ctx, actor.OrganizationID, s.VisionModelAlias)
	if err != nil || !capability.Available || !knowledgeDocumentBlobInScope(document, s.AssetsBucket) {
		return Document{}, ErrDocumentVisionUnavailable
	}
	var converterCapability DocumentVisionInputConversionCapability
	if !documentVisionSupportsMIME(capability, document.MIMEType) {
		converterCapability, err = s.inspectDocumentVisionConverter(ctx, capability, document.MIMEType)
		if err != nil || !converterCapability.Available {
			return Document{}, ErrDocumentVisionUnavailable
		}
	}
	pages, err := normalizeVisionPageSelection(request.PageNumbers, document.TotalPages)
	if err != nil {
		return Document{}, err
	}
	attemptID, err := s.newID("documentvisionattempt")
	if err != nil {
		return Document{}, err
	}
	tasks := splitContiguousVisionPages(pages)
	now := s.now()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Document{}, err
	}
	defer tx.Rollback()
	encodedPages, _ := json.Marshal(pages)
	result, err := tx.ExecContext(ctx, `UPDATE platform_knowledge_documents
		SET vision_fallback_status = 'queued', vision_attempt_id = ?,
			vision_selected_pages = ?, vision_completed_pages = JSON_ARRAY(),
			vision_model_alias = ?, vision_route_revision_id = ?, vision_model_version = '',
			vision_error_code = '', vision_error_message = '', vision_started_at = NULL,
			vision_completed_at = NULL, heartbeat_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND id = ?
		  AND vision_fallback_status NOT IN ('queued', 'running', 'succeeded')`,
		attemptID, encodedPages, s.VisionModelAlias, capability.RouteRevisionID, now, now,
		document.OrganizationID, document.ProjectID, document.ID,
	)
	if err != nil {
		return Document{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return Document{}, err
	}
	if changed == 0 {
		if err := tx.Rollback(); err != nil {
			return Document{}, err
		}
		return s.GetDocument(ctx, actor, projectID, document.ID)
	}
	for _, page := range pages {
		if _, err := tx.ExecContext(ctx, `INSERT INTO platform_knowledge_document_pages
			(organization_id, project_id, document_id, page_number, selection_reason,
			 status, text_origin, locator_json, model_alias, route_revision_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, 'manual_low_quality_review', 'selected', 'unknown', JSON_OBJECT('page_number', ?), ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE selection_reason = VALUES(selection_reason), status = 'selected',
				error_code = '', error_message = '', model_alias = VALUES(model_alias),
				route_revision_id = VALUES(route_revision_id), updated_at = VALUES(updated_at)`,
			document.OrganizationID, document.ProjectID, document.ID, page, page,
			s.VisionModelAlias, capability.RouteRevisionID, now, now,
		); err != nil {
			return Document{}, err
		}
	}
	for index, taskPages := range tasks {
		encodedTaskPages, _ := json.Marshal(taskPages)
		if _, err := tx.ExecContext(ctx, `INSERT INTO platform_knowledge_document_vision_tasks
			(organization_id, project_id, document_id, attempt_id, task_index,
			 page_numbers, status, model_alias, route_revision_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, 'prepared', ?, ?, ?, ?)`,
			document.OrganizationID, document.ProjectID, document.ID, attemptID, index,
			encodedTaskPages, s.VisionModelAlias, capability.RouteRevisionID, now, now,
		); err != nil {
			return Document{}, err
		}
	}
	if documentVisionNeedsConversion(document.MIMEType) {
		if err := insertDocumentVisionInputConversion(
			ctx, tx, document, attemptID, converterCapability, s.AssetsBucket, now,
		); err != nil {
			return Document{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Document{}, err
	}
	document.VisionFallbackStatus = "queued"
	document.VisionAttemptID = attemptID
	document.VisionSelectedPages = pages
	document.VisionCompletedPages = []int{}
	document.VisionModelAlias = s.VisionModelAlias
	document.VisionRouteRevision = capability.RouteRevisionID
	document.VisionModelVersion = ""
	document.VisionErrorCode, document.VisionErrorMessage = "", ""
	document.VisionStartedAt, document.VisionCompletedAt = nil, nil
	document.HeartbeatAt, document.UpdatedAt = &now, now
	if err := s.VisionScheduler.ScheduleDocumentVisionFallback(ctx, document, pages, ""); err != nil {
		s.markDocumentVisionFailed(ctx, document, "DOCUMENT_VISION_SCHEDULE_FAILED", "视觉解析暂时无法进入队列")
		return Document{}, err
	}
	s.recordDocumentEvent(ctx, document, DocumentEventVisionFallback, "accepted", "queued", nil)
	return document, nil
}

func (s Service) inspectDocumentVisionConverter(
	ctx context.Context,
	provider DocumentVisionCapability,
	mimeType string,
) (DocumentVisionInputConversionCapability, error) {
	if !documentVisionNeedsConversion(mimeType) || !documentVisionSupportsMIME(provider, "application/pdf") || s.DocumentConverter == nil {
		return DocumentVisionInputConversionCapability{ReasonCode: "DOCUMENT_VISION_CONVERTER_DISABLED"}, ErrDocumentVisionUnavailable
	}
	capability, err := s.DocumentConverter.Inspect(ctx)
	if err != nil || !capability.Available || strings.TrimSpace(capability.ConverterCode) == "" || len(capability.ConverterCode) > 64 ||
		strings.TrimSpace(capability.Version) == "" || len(capability.Version) > 160 {
		if capability.ReasonCode == "" {
			capability.ReasonCode = "DOCUMENT_VISION_CONVERTER_DISABLED"
		}
		return capability, ErrDocumentVisionUnavailable
	}
	return capability, nil
}

func documentVisionRequiresReconciliation(code string) bool {
	switch strings.TrimSpace(code) {
	case "DOCUMENT_VISION_SUBMISSION_UNKNOWN", "DOCUMENT_VISION_SUBMISSION_INVALID", "DOCUMENT_VISION_CHECKPOINT_FAILED":
		return true
	default:
		return false
	}
}

func documentVisionSupportsMIME(capability DocumentVisionCapability, mimeType string) bool {
	for _, supported := range capability.SupportedMIMEs {
		if strings.EqualFold(strings.TrimSpace(supported), strings.TrimSpace(mimeType)) {
			return true
		}
	}
	return false
}

func splitContiguousVisionPages(pages []int) [][]int {
	if len(pages) == 0 {
		return nil
	}
	result := make([][]int, 0, len(pages))
	current := []int{pages[0]}
	for _, page := range pages[1:] {
		if page == current[len(current)-1]+1 {
			current = append(current, page)
			continue
		}
		result = append(result, current)
		current = []int{page}
	}
	return append(result, current)
}

func documentVisionEligible(document Document) bool {
	if document.Status != "partial" || document.QualityTier != "low" {
		return false
	}
	switch document.MIMEType {
	case "application/pdf", PowerPointOpenXMLMIME, PowerPointLegacyMIME:
		return true
	default:
		return false
	}
}

func normalizeVisionPageSelection(values []int, totalPages *int) ([]int, error) {
	if len(values) == 0 {
		if totalPages == nil || *totalPages < 1 || *totalPages > 24 {
			return nil, ErrDocumentVisionPageSelectionRequired
		}
		values = make([]int, *totalPages)
		for index := range values {
			values[index] = index + 1
		}
	}
	if len(values) > 24 {
		return nil, ErrDocumentVisionPageSelectionRequired
	}
	seen := make(map[int]struct{}, len(values))
	pages := make([]int, 0, len(values))
	for _, page := range values {
		if page < 1 || totalPages != nil && page > *totalPages {
			return nil, ErrDocumentVisionPageSelectionRequired
		}
		if _, exists := seen[page]; exists {
			continue
		}
		seen[page] = struct{}{}
		pages = append(pages, page)
	}
	if len(pages) == 0 {
		return nil, ErrDocumentVisionPageSelectionRequired
	}
	sort.Ints(pages)
	return pages, nil
}

func validateDocumentVisionResult(result DocumentVisionParseResult, selectedPages []int) error {
	if strings.TrimSpace(result.ProviderCode) == "" || len(result.ProviderCode) > 128 ||
		strings.TrimSpace(result.ModelVersion) == "" || len(result.ModelVersion) > 128 ||
		len(result.RouteRevisionID) > 128 {
		return fmt.Errorf("document vision provider identity is missing")
	}
	if len(result.Usage) > maxDocumentVisionUsageBytes || len(result.Usage) > 0 && !json.Valid(result.Usage) {
		return fmt.Errorf("document vision usage is invalid or too large")
	}
	selected := make(map[int]struct{}, len(selectedPages))
	for _, page := range selectedPages {
		selected[page] = struct{}{}
	}
	seen := make(map[int]struct{}, len(result.Pages))
	totalBytes := 0
	for _, page := range result.Pages {
		pageBytes := len(page.Markdown)
		totalBytes += pageBytes
		if _, ok := selected[page.PageNumber]; !ok || strings.TrimSpace(page.Markdown) == "" ||
			pageBytes > maxDocumentVisionPageBytes || totalBytes > maxDocumentVisionResultBytes {
			return fmt.Errorf("document vision returned an invalid page")
		}
		if _, duplicate := seen[page.PageNumber]; duplicate {
			return fmt.Errorf("document vision returned a duplicate page")
		}
		if _, err := sanitizeDocumentVisionLocator(page.PageNumber, page.Locator); err != nil {
			return err
		}
		seen[page.PageNumber] = struct{}{}
	}
	if len(seen) == 0 {
		return fmt.Errorf("document vision returned no usable pages")
	}
	return nil
}

// sanitizeDocumentVisionLocator keeps the persistence contract intentionally
// smaller than any vendor response. It retains only traceability fields needed
// by the product and rejects credentials, object locations, and raw payloads.
func sanitizeDocumentVisionLocator(pageNumber int, locator map[string]any) (map[string]any, error) {
	normalized := map[string]any{"page_number": pageNumber}
	if locator == nil {
		return normalized, nil
	}
	for key, value := range locator {
		switch key {
		case "page_number":
			provided, ok := locatorInteger(value)
			if !ok || provided != pageNumber {
				return nil, fmt.Errorf("document vision locator page does not match result page")
			}
		case "section":
			section, ok := value.(string)
			if !ok || len(section) > 256 {
				return nil, fmt.Errorf("document vision locator section is invalid")
			}
			normalized[key] = section
		case "bounding_boxes":
			if err := validateDocumentVisionLocatorValue(value, 0); err != nil {
				return nil, err
			}
			normalized[key] = value
		default:
			return nil, fmt.Errorf("document vision locator field %q is not allowed", key)
		}
	}
	encoded, err := json.Marshal(normalized)
	if err != nil || len(encoded) > maxDocumentVisionLocatorBytes {
		return nil, fmt.Errorf("document vision locator is invalid or too large")
	}
	return normalized, nil
}

func validateDocumentVisionLocatorValue(value any, depth int) error {
	if depth > 5 {
		return fmt.Errorf("document vision locator is too deeply nested")
	}
	switch typed := value.(type) {
	case nil, bool:
		return nil
	case string:
		if len(typed) > 512 {
			return fmt.Errorf("document vision locator string is too large")
		}
		return nil
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) || math.Abs(typed) > 1_000_000 {
			return fmt.Errorf("document vision locator number is invalid")
		}
		return nil
	case float32:
		return validateDocumentVisionLocatorValue(float64(typed), depth)
	case int:
		return validateDocumentVisionLocatorValue(float64(typed), depth)
	case int32:
		return validateDocumentVisionLocatorValue(float64(typed), depth)
	case int64:
		return validateDocumentVisionLocatorValue(float64(typed), depth)
	case []any:
		if len(typed) > 256 {
			return fmt.Errorf("document vision locator array is too large")
		}
		for _, item := range typed {
			if err := validateDocumentVisionLocatorValue(item, depth+1); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		if len(typed) > 32 {
			return fmt.Errorf("document vision locator object is too large")
		}
		for key, item := range typed {
			lower := strings.ToLower(key)
			for _, forbidden := range []string{"url", "token", "secret", "credential", "authorization", "bucket", "object_key", "raw"} {
				if strings.Contains(lower, forbidden) {
					return fmt.Errorf("document vision locator contains a forbidden field")
				}
			}
			if err := validateDocumentVisionLocatorValue(item, depth+1); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("document vision locator contains an unsupported value")
	}
}

func locatorInteger(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		if math.Trunc(typed) != typed || typed > math.MaxInt || typed < math.MinInt {
			return 0, false
		}
		return int(typed), true
	default:
		return 0, false
	}
}
