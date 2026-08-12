package insights

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// MiyunConnectionStatus describes only the safe, public state of a Miyun
// connection. Session material is encrypted before it reaches this domain.
type MiyunConnectionStatus string

const (
	MiyunConnectionUnverified   MiyunConnectionStatus = "unverified"
	MiyunConnectionReady        MiyunConnectionStatus = "ready"
	MiyunConnectionAuthRequired MiyunConnectionStatus = "auth_required"
	MiyunConnectionDisabled     MiyunConnectionStatus = "disabled"
)

func (s MiyunConnectionStatus) valid() bool {
	switch s {
	case MiyunConnectionUnverified, MiyunConnectionReady, MiyunConnectionAuthRequired, MiyunConnectionDisabled:
		return true
	}
	return false
}

type MiyunProfileStatus string

const (
	MiyunProfileDraft      MiyunProfileStatus = "draft"
	MiyunProfileConfirmed  MiyunProfileStatus = "confirmed"
	MiyunProfileSuperseded MiyunProfileStatus = "superseded"
)

func (s MiyunProfileStatus) valid() bool {
	switch s {
	case MiyunProfileDraft, MiyunProfileConfirmed, MiyunProfileSuperseded:
		return true
	}
	return false
}

type MiyunImportMethod string

const (
	MiyunImportCrawler MiyunImportMethod = "crawler"
	MiyunImportManual  MiyunImportMethod = "manual"
)

func (m MiyunImportMethod) valid() bool {
	return m == MiyunImportCrawler || m == MiyunImportManual
}

type MiyunProfileFieldSource struct {
	Field       string   `json:"field"`
	SourceKind  string   `json:"source_kind"`
	SourceRefs  []string `json:"source_refs"`
	Confidence  string   `json:"confidence"`
	ReviewState string   `json:"review_state"`
	Explanation string   `json:"explanation"`
}

func (s MiyunProfileFieldSource) Validate() error {
	if strings.TrimSpace(s.Field) == "" || strings.TrimSpace(s.SourceKind) == "" ||
		strings.TrimSpace(s.Confidence) == "" || strings.TrimSpace(s.ReviewState) == "" ||
		strings.TrimSpace(s.Explanation) == "" || len(s.SourceRefs) == 0 {
		return fmt.Errorf("%w: profile field source must be explainable", ErrInvalidRequest)
	}
	if s.Confidence != "high" && s.Confidence != "medium" && s.Confidence != "low" && s.Confidence != "unknown" {
		return fmt.Errorf("%w: profile field source confidence is invalid", ErrInvalidRequest)
	}
	if s.ReviewState != "suggested" && s.ReviewState != "unknown" && s.ReviewState != "human_confirmed" {
		return fmt.Errorf("%w: profile field source review state is invalid", ErrInvalidRequest)
	}
	return validateUniqueStrings("profile source reference", s.SourceRefs)
}

type MiyunCrawlJobStatus string

const (
	MiyunCrawlJobQueued       MiyunCrawlJobStatus = "queued"
	MiyunCrawlJobRunning      MiyunCrawlJobStatus = "running"
	MiyunCrawlJobCoolingDown  MiyunCrawlJobStatus = "cooling_down"
	MiyunCrawlJobAuthRequired MiyunCrawlJobStatus = "auth_required"
	MiyunCrawlJobPartial      MiyunCrawlJobStatus = "partial"
	MiyunCrawlJobSucceeded    MiyunCrawlJobStatus = "succeeded"
	MiyunCrawlJobFailed       MiyunCrawlJobStatus = "failed"
	MiyunCrawlJobCancelled    MiyunCrawlJobStatus = "cancelled"
)

func (s MiyunCrawlJobStatus) valid() bool {
	switch s {
	case MiyunCrawlJobQueued, MiyunCrawlJobRunning, MiyunCrawlJobCoolingDown,
		MiyunCrawlJobAuthRequired, MiyunCrawlJobPartial, MiyunCrawlJobSucceeded,
		MiyunCrawlJobFailed, MiyunCrawlJobCancelled:
		return true
	}
	return false
}

type MiyunMaterialSelectionStatus string

const (
	MiyunMaterialDiscovered MiyunMaterialSelectionStatus = "discovered"
	MiyunMaterialConfirmed  MiyunMaterialSelectionStatus = "confirmed"
	MiyunMaterialRejected   MiyunMaterialSelectionStatus = "rejected"
)

func (s MiyunMaterialSelectionStatus) valid() bool {
	switch s {
	case MiyunMaterialDiscovered, MiyunMaterialConfirmed, MiyunMaterialRejected:
		return true
	}
	return false
}

type MiyunMaterialImportStatus string

const (
	MiyunMaterialImportPending      MiyunMaterialImportStatus = "pending"
	MiyunMaterialImportDownloading  MiyunMaterialImportStatus = "downloading"
	MiyunMaterialImportImported     MiyunMaterialImportStatus = "imported"
	MiyunMaterialImportDeduplicated MiyunMaterialImportStatus = "deduplicated"
	MiyunMaterialImportFailed       MiyunMaterialImportStatus = "failed"
	MiyunMaterialImportSkipped      MiyunMaterialImportStatus = "skipped"
)

func (s MiyunMaterialImportStatus) valid() bool {
	switch s {
	case MiyunMaterialImportPending, MiyunMaterialImportDownloading, MiyunMaterialImportImported,
		MiyunMaterialImportDeduplicated, MiyunMaterialImportFailed, MiyunMaterialImportSkipped:
		return true
	}
	return false
}

type MiyunHandoffStatus string

const (
	MiyunHandoffExporting MiyunHandoffStatus = "exporting"
	MiyunHandoffExported  MiyunHandoffStatus = "exported"
	MiyunHandoffDelivered MiyunHandoffStatus = "delivered"
	MiyunHandoffReturned  MiyunHandoffStatus = "returned"
	MiyunHandoffFailed    MiyunHandoffStatus = "failed"
)

func (s MiyunHandoffStatus) valid() bool {
	switch s {
	case MiyunHandoffExporting, MiyunHandoffExported, MiyunHandoffDelivered,
		MiyunHandoffReturned, MiyunHandoffFailed:
		return true
	}
	return false
}

type MiyunConnection struct {
	ID                      string                  `json:"id"`
	OrganizationID          contract.OrganizationID `json:"organization_id"`
	ProjectID               contract.ProjectID      `json:"project_id"`
	Status                  MiyunConnectionStatus   `json:"status"`
	SessionCiphertext       []byte                  `json:"-"`
	SessionKeyVersion       string                  `json:"-"`
	SessionExpiresAt        *time.Time              `json:"session_expires_at,omitempty"`
	LastVerifiedAt          *time.Time              `json:"last_verified_at,omitempty"`
	LastSuccessfulRequestAt *time.Time              `json:"last_successful_request_at,omitempty"`
	CooldownUntil           *time.Time              `json:"cooldown_until,omitempty"`
	LastErrorKind           string                  `json:"last_error_kind,omitempty"`
	LastErrorCode           string                  `json:"last_error_code,omitempty"`
	LastErrorAt             *time.Time              `json:"last_error_at,omitempty"`
	Version                 int64                   `json:"version"`
	CreatedBy               string                  `json:"created_by"`
	CreatedAt               time.Time               `json:"created_at"`
	UpdatedAt               time.Time               `json:"updated_at"`
}

func (c MiyunConnection) Validate() error {
	if err := validateMiyunAggregate(c.ID, c.OrganizationID, c.ProjectID, c.Version, c.CreatedBy, c.CreatedAt, c.UpdatedAt); err != nil {
		return err
	}
	if !c.Status.valid() {
		return fmt.Errorf("%w: unsupported Miyun connection status %q", ErrInvalidRequest, c.Status)
	}
	if len(c.SessionCiphertext) == 0 || strings.TrimSpace(c.SessionKeyVersion) == "" {
		return fmt.Errorf("%w: encrypted session and key version are required", ErrInvalidRequest)
	}
	if (c.LastErrorKind == "") != (c.LastErrorCode == "") {
		return fmt.Errorf("%w: last error kind and code must be supplied together", ErrInvalidRequest)
	}
	return nil
}

type MiyunProductProfile struct {
	ID                    string                     `json:"id"`
	OrganizationID        contract.OrganizationID    `json:"organization_id"`
	ProjectID             contract.ProjectID         `json:"project_id"`
	ConnectionID          string                     `json:"connection_id"`
	Status                MiyunProfileStatus         `json:"status"`
	ProductID             contract.ProductID         `json:"product_id"`
	ProductName           string                     `json:"product_name"`
	BrandName             string                     `json:"brand_name,omitempty"`
	CategoryID            string                     `json:"category_id,omitempty"`
	CategoryName          string                     `json:"category_name,omitempty"`
	Keywords              []string                   `json:"keywords"`
	MaterialTypes         []string                   `json:"material_types"`
	MaterialContentTypes  []string                   `json:"material_content_types"`
	WindowStart           time.Time                  `json:"window_start"`
	WindowEnd             time.Time                  `json:"window_end"`
	ProjectContextVersion int64                      `json:"project_context_version"`
	ProductAssetRefs      []contract.AssetVersionRef `json:"product_asset_refs"`
	KnowledgeDocumentIDs  []string                   `json:"knowledge_document_ids"`
	RuleVersion           string                     `json:"rule_version"`
	ModelVersion          string                     `json:"model_version,omitempty"`
	AnalysisMethod        string                     `json:"analysis_method"`
	InputHash             string                     `json:"input_hash"`
	InputSnapshot         json.RawMessage            `json:"input_snapshot"`
	FieldSources          []MiyunProfileFieldSource  `json:"field_sources"`
	AnalysisWarnings      []string                   `json:"analysis_warnings"`
	ConfirmedBy           string                     `json:"confirmed_by,omitempty"`
	ConfirmedAt           *time.Time                 `json:"confirmed_at,omitempty"`
	Version               int64                      `json:"version"`
	CreatedBy             string                     `json:"created_by"`
	CreatedAt             time.Time                  `json:"created_at"`
	UpdatedAt             time.Time                  `json:"updated_at"`
}

func (p MiyunProductProfile) Validate() error {
	if err := validateMiyunAggregate(p.ID, p.OrganizationID, p.ProjectID, p.Version, p.CreatedBy, p.CreatedAt, p.UpdatedAt); err != nil {
		return err
	}
	if !p.Status.valid() || strings.TrimSpace(p.ConnectionID) == "" || strings.TrimSpace(string(p.ProductID)) == "" ||
		strings.TrimSpace(p.ProductName) == "" || len(p.Keywords) == 0 {
		return fmt.Errorf("%w: profile status, connection, product name, and at least one keyword are required", ErrInvalidRequest)
	}
	if len(p.ConnectionID) > 96 || len(p.ProductID) > 96 || len([]rune(p.ProductName)) > 255 ||
		len(p.CategoryID) > 96 || len([]rune(p.CategoryName)) > 255 || len(p.RuleVersion) > 96 {
		return fmt.Errorf("%w: profile identity or query field is too long", ErrInvalidRequest)
	}
	if p.WindowStart.IsZero() || p.WindowEnd.IsZero() || p.WindowEnd.Before(p.WindowStart) {
		return fmt.Errorf("%w: profile date window is invalid", ErrInvalidRequest)
	}
	if p.ProjectContextVersion < 1 || strings.TrimSpace(p.RuleVersion) == "" || p.AnalysisMethod != "rules" ||
		len(p.InputHash) != 64 || !json.Valid(p.InputSnapshot) {
		return fmt.Errorf("%w: profile context version, rule version, and SHA-256 input hash are required", ErrInvalidRequest)
	}
	if p.ModelVersion != "" {
		return fmt.Errorf("%w: deterministic rule profiles must not claim model lineage", ErrInvalidRequest)
	}
	if err := validateUniqueStrings("keyword", p.Keywords); err != nil {
		return err
	}
	if err := validateUniqueStrings("material type", p.MaterialTypes); err != nil {
		return err
	}
	if err := validateUniqueStrings("material content type", p.MaterialContentTypes); err != nil {
		return err
	}
	if err := validateUniqueStrings("knowledge document ID", p.KnowledgeDocumentIDs); err != nil {
		return err
	}
	if err := validateUniqueStrings("analysis warning", p.AnalysisWarnings); err != nil {
		return err
	}
	if len(p.FieldSources) == 0 {
		return fmt.Errorf("%w: profile field sources are required", ErrInvalidRequest)
	}
	seenFields := map[string]struct{}{}
	for _, source := range p.FieldSources {
		if err := source.Validate(); err != nil {
			return err
		}
		if _, exists := seenFields[source.Field]; exists {
			return fmt.Errorf("%w: duplicate profile field source %q", ErrInvalidRequest, source.Field)
		}
		seenFields[source.Field] = struct{}{}
	}
	for _, ref := range p.ProductAssetRefs {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("%w: invalid product asset reference: %v", ErrInvalidRequest, err)
		}
	}
	if p.Status == MiyunProfileConfirmed && (strings.TrimSpace(p.ConfirmedBy) == "" || p.ConfirmedAt == nil) {
		return fmt.Errorf("%w: confirmed profile requires confirmer and timestamp", ErrInvalidRequest)
	}
	return nil
}

type MiyunCrawlJob struct {
	ID                 string                  `json:"id"`
	OrganizationID     contract.OrganizationID `json:"organization_id"`
	ProjectID          contract.ProjectID      `json:"project_id"`
	ConnectionID       string                  `json:"connection_id"`
	ProductProfileID   string                  `json:"product_profile_id"`
	Status             MiyunCrawlJobStatus     `json:"status"`
	Operation          string                  `json:"operation"`
	QuerySchemaVersion string                  `json:"query_schema_version"`
	QuerySnapshot      json.RawMessage         `json:"query_snapshot"`
	IdempotencyKey     string                  `json:"-"`
	RequestHash        string                  `json:"-"`
	RuntimeJobID       string                  `json:"runtime_job_id"`
	CompletedPages     int64                   `json:"completed_pages"`
	DiscoveredCount    int64                   `json:"discovered_count"`
	DeduplicatedCount  int64                   `json:"deduplicated_count"`
	DownloadedCount    int64                   `json:"downloaded_count"`
	FailedCount        int64                   `json:"failed_count"`
	CooldownUntil      *time.Time              `json:"cooldown_until,omitempty"`
	LastErrorKind      string                  `json:"last_error_kind,omitempty"`
	LastErrorCode      string                  `json:"last_error_code,omitempty"`
	Version            int64                   `json:"version"`
	CreatedBy          string                  `json:"created_by"`
	CreatedAt          time.Time               `json:"created_at"`
	UpdatedAt          time.Time               `json:"updated_at"`
}

func (j MiyunCrawlJob) Validate() error {
	if err := validateMiyunAggregate(j.ID, j.OrganizationID, j.ProjectID, j.Version, j.CreatedBy, j.CreatedAt, j.UpdatedAt); err != nil {
		return err
	}
	if !j.Status.valid() || strings.TrimSpace(j.ConnectionID) == "" || strings.TrimSpace(j.ProductProfileID) == "" {
		return fmt.Errorf("%w: job status, connection, and product profile are required", ErrInvalidRequest)
	}
	if j.Operation != "product" && j.Operation != "cid" {
		return fmt.Errorf("%w: job operation must be product or cid", ErrInvalidRequest)
	}
	if strings.TrimSpace(j.QuerySchemaVersion) == "" || !json.Valid(j.QuerySnapshot) {
		return fmt.Errorf("%w: versioned query snapshot must be valid JSON", ErrInvalidRequest)
	}
	if strings.TrimSpace(j.IdempotencyKey) == "" || len(j.IdempotencyKey) > 128 || len(j.RequestHash) != 64 || strings.TrimSpace(j.RuntimeJobID) == "" {
		return fmt.Errorf("%w: job idempotency identity and runtime job are required", ErrInvalidRequest)
	}
	if j.CompletedPages < 0 || j.DiscoveredCount < 0 || j.DeduplicatedCount < 0 || j.DownloadedCount < 0 || j.FailedCount < 0 {
		return fmt.Errorf("%w: job counters must be non-negative", ErrInvalidRequest)
	}
	if j.Status == MiyunCrawlJobCoolingDown && j.CooldownUntil == nil {
		return fmt.Errorf("%w: cooling_down job requires cooldown_until", ErrInvalidRequest)
	}
	if (j.LastErrorKind == "") != (j.LastErrorCode == "") {
		return fmt.Errorf("%w: last error kind and code must be supplied together", ErrInvalidRequest)
	}
	return nil
}

type MiyunMaterial struct {
	ID                    string                       `json:"id"`
	OrganizationID        contract.OrganizationID      `json:"organization_id"`
	ProjectID             contract.ProjectID           `json:"project_id"`
	MiyunMaterialID       string                       `json:"miyun_material_id"`
	FirstSeenCrawlJobID   string                       `json:"first_seen_crawl_job_id,omitempty"`
	ImportMethod          MiyunImportMethod            `json:"import_method"`
	ManualIdempotencyKey  string                       `json:"-"`
	ManualRequestHash     string                       `json:"-"`
	ResourceID            string                       `json:"resource_id,omitempty"`
	ResourceURLCiphertext []byte                       `json:"-"`
	ResourceURLKeyVersion string                       `json:"-"`
	ResourceExpectedSize  int64                        `json:"resource_expected_size,omitempty"`
	SourceRef             string                       `json:"source_ref,omitempty"`
	SourceRefStatus       string                       `json:"source_ref_status"`
	Title                 string                       `json:"title,omitempty"`
	SelectionStatus       MiyunMaterialSelectionStatus `json:"selection_status"`
	ImportStatus          MiyunMaterialImportStatus    `json:"import_status"`
	PlatformAssetID       contract.AssetID             `json:"platform_asset_id,omitempty"`
	PlatformAssetVersion  int64                        `json:"platform_asset_version,omitempty"`
	InsightAssetID        string                       `json:"insight_asset_id,omitempty"`
	ExternalImportID      string                       `json:"external_import_id,omitempty"`
	DecisionBy            string                       `json:"decision_by,omitempty"`
	DecisionAt            *time.Time                   `json:"decision_at,omitempty"`
	DecisionNote          string                       `json:"decision_note,omitempty"`
	LastImportErrorKind   string                       `json:"last_import_error_kind,omitempty"`
	LastImportErrorCode   string                       `json:"last_import_error_code,omitempty"`
	Version               int64                        `json:"version"`
	CreatedBy             string                       `json:"created_by"`
	CreatedAt             time.Time                    `json:"created_at"`
	UpdatedAt             time.Time                    `json:"updated_at"`
}

func (m MiyunMaterial) Validate() error {
	if err := validateMiyunAggregate(m.ID, m.OrganizationID, m.ProjectID, m.Version, m.CreatedBy, m.CreatedAt, m.UpdatedAt); err != nil {
		return err
	}
	if strings.TrimSpace(m.MiyunMaterialID) == "" || !m.ImportMethod.valid() {
		return fmt.Errorf("%w: Miyun material ID and import method are required", ErrInvalidRequest)
	}
	if len(m.MiyunMaterialID) > 191 || len(m.FirstSeenCrawlJobID) > 96 || len(m.ManualIdempotencyKey) > 128 ||
		len(m.SourceRef) > 512 || len([]rune(m.Title)) > 255 || len([]rune(m.DecisionNote)) > 1000 {
		return fmt.Errorf("%w: Miyun material identity or source is too long", ErrInvalidRequest)
	}
	if m.ImportMethod == MiyunImportCrawler && (len(m.ResourceURLCiphertext) == 0 || strings.TrimSpace(m.ResourceURLKeyVersion) == "") {
		return fmt.Errorf("%w: crawler material requires an encrypted resource locator", ErrInvalidRequest)
	}
	if m.ResourceExpectedSize < 0 {
		return fmt.Errorf("%w: resource expected size must not be negative", ErrInvalidRequest)
	}
	if m.SourceRefStatus != "verified" && m.SourceRefStatus != "unknown" {
		return fmt.Errorf("%w: source reference status must be verified or unknown", ErrInvalidRequest)
	}
	if m.SourceRefStatus == "verified" && strings.TrimSpace(m.SourceRef) == "" {
		return fmt.Errorf("%w: verified source reference is missing", ErrInvalidRequest)
	}
	if (m.ImportMethod == MiyunImportCrawler) != (strings.TrimSpace(m.FirstSeenCrawlJobID) != "") {
		return fmt.Errorf("%w: crawler materials require a crawl job and manual materials must not have one", ErrInvalidRequest)
	}
	if m.ImportMethod == MiyunImportManual {
		if strings.TrimSpace(m.ManualIdempotencyKey) == "" || len(m.ManualRequestHash) != 64 {
			return fmt.Errorf("%w: manual material requires idempotency identity", ErrInvalidRequest)
		}
		if m.PlatformAssetID == "" || m.PlatformAssetVersion < 1 || strings.TrimSpace(m.InsightAssetID) == "" || strings.TrimSpace(m.SourceRef) == "" {
			return fmt.Errorf("%w: manual material requires its source AssetVersion and Insight Asset", ErrInvalidRequest)
		}
	} else if m.ManualIdempotencyKey != "" || m.ManualRequestHash != "" {
		return fmt.Errorf("%w: crawler material must not carry manual idempotency identity", ErrInvalidRequest)
	}
	if !m.SelectionStatus.valid() || !m.ImportStatus.valid() {
		return fmt.Errorf("%w: unsupported material selection or import status", ErrInvalidRequest)
	}
	if m.SelectionStatus != MiyunMaterialDiscovered && (strings.TrimSpace(m.DecisionBy) == "" || m.DecisionAt == nil) {
		return fmt.Errorf("%w: decided material requires operator and timestamp", ErrInvalidRequest)
	}
	if (m.LastImportErrorKind == "") != (m.LastImportErrorCode == "") {
		return fmt.Errorf("%w: import error kind and code must be supplied together", ErrInvalidRequest)
	}
	if (m.PlatformAssetID == "") != (m.PlatformAssetVersion == 0) {
		return fmt.Errorf("%w: platform asset ID and version must be supplied together", ErrInvalidRequest)
	}
	if m.PlatformAssetVersion < 0 {
		return fmt.Errorf("%w: platform asset version must not be negative", ErrInvalidRequest)
	}
	return nil
}

type MiyunMaterialSnapshot struct {
	ID                       string                  `json:"id"`
	OrganizationID           contract.OrganizationID `json:"organization_id"`
	ProjectID                contract.ProjectID      `json:"project_id"`
	MaterialID               string                  `json:"material_id"`
	CrawlJobID               string                  `json:"crawl_job_id,omitempty"`
	SourcePage               int64                   `json:"source_page"`
	ImportMethod             MiyunImportMethod       `json:"import_method"`
	SchemaVersion            string                  `json:"schema_version"`
	CapturedAt               time.Time               `json:"captured_at"`
	FirstPublishedAt         *time.Time              `json:"first_published_at,omitempty"`
	LastPublishedAt          *time.Time              `json:"last_published_at,omitempty"`
	DeliveryDays             int64                   `json:"delivery_days"`
	CumulativeImpressions    int64                   `json:"cumulative_impressions"`
	CumulativeImpressionsRaw string                  `json:"cumulative_impressions_raw"`
	RelatedAds               int64                   `json:"related_ads"`
	RelatedCreators          int64                   `json:"related_creators"`
	RelatedCreatorsRaw       string                  `json:"related_creators_raw"`
	RelatedCreatorsKnown     bool                    `json:"related_creators_known"`
	MaterialScore            float64                 `json:"material_score"`
	Views                    int64                   `json:"views"`
	Likes                    int64                   `json:"likes"`
	Comments                 int64                   `json:"comments"`
	Shares                   int64                   `json:"shares"`
	Saves                    int64                   `json:"saves"`
	SanitizedRaw             json.RawMessage         `json:"sanitized_raw,omitempty"`
	CreatedAt                time.Time               `json:"created_at"`
}

func (s MiyunMaterialSnapshot) Validate() error {
	if strings.TrimSpace(s.ID) == "" || strings.TrimSpace(string(s.OrganizationID)) == "" ||
		strings.TrimSpace(string(s.ProjectID)) == "" || strings.TrimSpace(s.MaterialID) == "" ||
		strings.TrimSpace(s.SchemaVersion) == "" || !s.ImportMethod.valid() ||
		s.CapturedAt.IsZero() || s.CreatedAt.IsZero() {
		return fmt.Errorf("%w: snapshot identity, scope, schema, and timestamps are required", ErrInvalidRequest)
	}
	if (s.ImportMethod == MiyunImportCrawler) != (strings.TrimSpace(s.CrawlJobID) != "") {
		return fmt.Errorf("%w: crawler snapshots require a crawl job and manual snapshots must not have one", ErrInvalidRequest)
	}
	if s.ImportMethod == MiyunImportCrawler && s.SourcePage < 1 {
		return fmt.Errorf("%w: crawler snapshot source page is required", ErrInvalidRequest)
	}
	if s.ImportMethod == MiyunImportManual && s.SourcePage != 0 {
		return fmt.Errorf("%w: manual snapshot must not carry a source page", ErrInvalidRequest)
	}
	if s.ImportMethod == MiyunImportManual && s.SchemaVersion != MiyunDataCardSchemaV1 {
		return fmt.Errorf("%w: manual snapshots require the supported Miyun data-card schema", ErrInvalidRequest)
	}
	if strings.TrimSpace(s.CumulativeImpressionsRaw) == "" || len(s.CumulativeImpressionsRaw) > 64 {
		return fmt.Errorf("%w: cumulative impressions raw value is required", ErrInvalidRequest)
	}
	if s.DeliveryDays < 0 || s.CumulativeImpressions < 0 || s.RelatedAds < 0 || s.RelatedCreators < 0 ||
		s.MaterialScore < 0 || s.Views < 0 || s.Likes < 0 || s.Comments < 0 || s.Shares < 0 || s.Saves < 0 {
		return fmt.Errorf("%w: snapshot metrics must be non-negative", ErrInvalidRequest)
	}
	if strings.TrimSpace(s.RelatedCreatorsRaw) == "" || (s.RelatedCreatorsKnown && s.RelatedCreatorsRaw == "unknown") {
		return fmt.Errorf("%w: related creator source state must be explicit", ErrInvalidRequest)
	}
	if len(s.SanitizedRaw) > 0 && !json.Valid(s.SanitizedRaw) {
		return fmt.Errorf("%w: sanitized raw snapshot must be valid JSON", ErrInvalidRequest)
	}
	if len(s.SanitizedRaw) > 0 {
		var raw any
		if err := json.Unmarshal(s.SanitizedRaw, &raw); err != nil || containsMiyunSecret(raw) {
			return fmt.Errorf("%w: sanitized raw snapshot contains forbidden session or account data", ErrInvalidRequest)
		}
	}
	if (s.FirstPublishedAt == nil) != (s.LastPublishedAt == nil) ||
		(s.FirstPublishedAt != nil && s.LastPublishedAt.Before(*s.FirstPublishedAt)) {
		return fmt.Errorf("%w: snapshot publication window is invalid", ErrInvalidRequest)
	}
	return nil
}

type MiyunHandoff struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	// SourceMaterialID is retained as the first, stable source for legacy
	// relational lineage. SourceMaterialIDs is the complete frozen selection.
	SourceMaterialID     string               `json:"source_material_id"`
	SourceMaterialIDs    []string             `json:"source_material_ids"`
	ProductProfileID     string               `json:"product_profile_id"`
	CrawlJobID           string               `json:"crawl_job_id,omitempty"`
	Status               MiyunHandoffStatus   `json:"status"`
	ManifestVersion      string               `json:"manifest_version"`
	ParameterVersion     string               `json:"parameter_version"`
	ProductFilesSnapshot json.RawMessage      `json:"product_files_snapshot"`
	SourceSnapshot       json.RawMessage      `json:"source_snapshot"`
	ProfileSnapshot      json.RawMessage      `json:"profile_snapshot"`
	InputHash            string               `json:"input_hash"`
	Version              int64                `json:"version"`
	CreatedBy            string               `json:"created_by"`
	CreatedAt            time.Time            `json:"created_at"`
	UpdatedAt            time.Time            `json:"updated_at"`
	Returns              []MiyunHandoffReturn `json:"returns,omitempty"`
}

func (h MiyunHandoff) Validate() error {
	if err := validateMiyunAggregate(h.ID, h.OrganizationID, h.ProjectID, h.Version, h.CreatedBy, h.CreatedAt, h.UpdatedAt); err != nil {
		return err
	}
	if !h.Status.valid() || strings.TrimSpace(h.SourceMaterialID) == "" || strings.TrimSpace(h.ProductProfileID) == "" ||
		strings.TrimSpace(h.ManifestVersion) == "" || strings.TrimSpace(h.ParameterVersion) == "" {
		return fmt.Errorf("%w: handoff status, source, profile, and schema versions are required", ErrInvalidRequest)
	}
	if len(h.InputHash) != 64 || !json.Valid(h.ProductFilesSnapshot) || !json.Valid(h.SourceSnapshot) || !json.Valid(h.ProfileSnapshot) {
		return fmt.Errorf("%w: handoff snapshots must be valid JSON", ErrInvalidRequest)
	}
	if err := validateUniqueStrings("source material ID", h.SourceMaterialIDs); err != nil || len(h.SourceMaterialIDs) == 0 || h.SourceMaterialIDs[0] != h.SourceMaterialID {
		return fmt.Errorf("%w: handoff source material selection is invalid", ErrInvalidRequest)
	}
	return nil
}

func validateMiyunAggregate(id string, organizationID contract.OrganizationID, projectID contract.ProjectID,
	version int64, createdBy string, createdAt, updatedAt time.Time) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(string(organizationID)) == "" ||
		strings.TrimSpace(string(projectID)) == "" || strings.TrimSpace(createdBy) == "" {
		return fmt.Errorf("%w: Miyun aggregate identity, scope, and creator are required", ErrInvalidRequest)
	}
	if version < 1 || createdAt.IsZero() || updatedAt.IsZero() || updatedAt.Before(createdAt) {
		return fmt.Errorf("%w: Miyun aggregate version and timestamps are invalid", ErrInvalidRequest)
	}
	return nil
}

func validateUniqueStrings(label string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return fmt.Errorf("%w: %s must not be empty", ErrInvalidRequest, label)
		}
		if _, exists := seen[trimmed]; exists {
			return fmt.Errorf("%w: duplicate %s %q", ErrInvalidRequest, label, trimmed)
		}
		seen[trimmed] = struct{}{}
	}
	return nil
}

func containsMiyunSecret(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalized := strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(key))
			for _, forbidden := range []string{"cookie", "authorization", "sessionid", "header", "account", "password", "token"} {
				if strings.Contains(normalized, forbidden) {
					return true
				}
			}
			if containsMiyunSecret(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsMiyunSecret(child) {
				return true
			}
		}
	case string:
		return strings.HasPrefix(typed, "eyJ") && strings.Count(typed, ".") == 2
	}
	return false
}
