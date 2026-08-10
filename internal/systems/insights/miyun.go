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
	ProductName           string                     `json:"product_name"`
	CategoryID            string                     `json:"category_id,omitempty"`
	CategoryName          string                     `json:"category_name,omitempty"`
	Keywords              []string                   `json:"keywords"`
	MaterialContentTypes  []string                   `json:"material_content_types"`
	WindowStart           time.Time                  `json:"window_start"`
	WindowEnd             time.Time                  `json:"window_end"`
	ProjectContextVersion int64                      `json:"project_context_version"`
	ProductAssetRefs      []contract.AssetVersionRef `json:"product_asset_refs"`
	KnowledgeDocumentIDs  []string                   `json:"knowledge_document_ids"`
	RuleVersion           string                     `json:"rule_version"`
	InputHash             string                     `json:"input_hash"`
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
	if !p.Status.valid() || strings.TrimSpace(p.ConnectionID) == "" || strings.TrimSpace(p.ProductName) == "" || len(p.Keywords) == 0 {
		return fmt.Errorf("%w: profile status, connection, product name, and at least one keyword are required", ErrInvalidRequest)
	}
	if p.WindowStart.IsZero() || p.WindowEnd.IsZero() || p.WindowEnd.Before(p.WindowStart) {
		return fmt.Errorf("%w: profile date window is invalid", ErrInvalidRequest)
	}
	if p.ProjectContextVersion < 1 || strings.TrimSpace(p.RuleVersion) == "" || len(p.InputHash) != 64 {
		return fmt.Errorf("%w: profile context version, rule version, and SHA-256 input hash are required", ErrInvalidRequest)
	}
	if err := validateUniqueStrings("keyword", p.Keywords); err != nil {
		return err
	}
	if err := validateUniqueStrings("material content type", p.MaterialContentTypes); err != nil {
		return err
	}
	if err := validateUniqueStrings("knowledge document ID", p.KnowledgeDocumentIDs); err != nil {
		return err
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
	ID                   string                       `json:"id"`
	OrganizationID       contract.OrganizationID      `json:"organization_id"`
	ProjectID            contract.ProjectID           `json:"project_id"`
	MiyunMaterialID      string                       `json:"miyun_material_id"`
	FirstSeenCrawlJobID  string                       `json:"first_seen_crawl_job_id"`
	ResourceID           string                       `json:"resource_id,omitempty"`
	SourceRef            string                       `json:"source_ref,omitempty"`
	Title                string                       `json:"title,omitempty"`
	SelectionStatus      MiyunMaterialSelectionStatus `json:"selection_status"`
	ImportStatus         MiyunMaterialImportStatus    `json:"import_status"`
	PlatformAssetID      contract.AssetID             `json:"platform_asset_id,omitempty"`
	PlatformAssetVersion int64                        `json:"platform_asset_version,omitempty"`
	InsightAssetID       string                       `json:"insight_asset_id,omitempty"`
	ExternalImportID     string                       `json:"external_import_id,omitempty"`
	Version              int64                        `json:"version"`
	CreatedBy            string                       `json:"created_by"`
	CreatedAt            time.Time                    `json:"created_at"`
	UpdatedAt            time.Time                    `json:"updated_at"`
}

func (m MiyunMaterial) Validate() error {
	if err := validateMiyunAggregate(m.ID, m.OrganizationID, m.ProjectID, m.Version, m.CreatedBy, m.CreatedAt, m.UpdatedAt); err != nil {
		return err
	}
	if strings.TrimSpace(m.MiyunMaterialID) == "" || strings.TrimSpace(m.FirstSeenCrawlJobID) == "" {
		return fmt.Errorf("%w: Miyun material ID and first crawl job are required", ErrInvalidRequest)
	}
	if !m.SelectionStatus.valid() || !m.ImportStatus.valid() {
		return fmt.Errorf("%w: unsupported material selection or import status", ErrInvalidRequest)
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
	ID                    string                  `json:"id"`
	OrganizationID        contract.OrganizationID `json:"organization_id"`
	ProjectID             contract.ProjectID      `json:"project_id"`
	MaterialID            string                  `json:"material_id"`
	CrawlJobID            string                  `json:"crawl_job_id"`
	SchemaVersion         string                  `json:"schema_version"`
	CapturedAt            time.Time               `json:"captured_at"`
	FirstPublishedAt      *time.Time              `json:"first_published_at,omitempty"`
	LastPublishedAt       *time.Time              `json:"last_published_at,omitempty"`
	DeliveryDays          int64                   `json:"delivery_days"`
	CumulativeImpressions int64                   `json:"cumulative_impressions"`
	RelatedAds            int64                   `json:"related_ads"`
	RelatedCreators       int64                   `json:"related_creators"`
	MaterialScore         float64                 `json:"material_score"`
	Views                 int64                   `json:"views"`
	Likes                 int64                   `json:"likes"`
	Comments              int64                   `json:"comments"`
	Shares                int64                   `json:"shares"`
	Saves                 int64                   `json:"saves"`
	SanitizedRaw          json.RawMessage         `json:"sanitized_raw,omitempty"`
	CreatedAt             time.Time               `json:"created_at"`
}

func (s MiyunMaterialSnapshot) Validate() error {
	if strings.TrimSpace(s.ID) == "" || strings.TrimSpace(string(s.OrganizationID)) == "" ||
		strings.TrimSpace(string(s.ProjectID)) == "" || strings.TrimSpace(s.MaterialID) == "" ||
		strings.TrimSpace(s.CrawlJobID) == "" || strings.TrimSpace(s.SchemaVersion) == "" ||
		s.CapturedAt.IsZero() || s.CreatedAt.IsZero() {
		return fmt.Errorf("%w: snapshot identity, scope, schema, and timestamps are required", ErrInvalidRequest)
	}
	if s.DeliveryDays < 0 || s.CumulativeImpressions < 0 || s.RelatedAds < 0 || s.RelatedCreators < 0 ||
		s.MaterialScore < 0 || s.Views < 0 || s.Likes < 0 || s.Comments < 0 || s.Shares < 0 || s.Saves < 0 {
		return fmt.Errorf("%w: snapshot metrics must be non-negative", ErrInvalidRequest)
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
	ID                   string                  `json:"id"`
	OrganizationID       contract.OrganizationID `json:"organization_id"`
	ProjectID            contract.ProjectID      `json:"project_id"`
	SourceMaterialID     string                  `json:"source_material_id"`
	ProductProfileID     string                  `json:"product_profile_id"`
	Status               MiyunHandoffStatus      `json:"status"`
	ManifestVersion      string                  `json:"manifest_version"`
	ParameterVersion     string                  `json:"parameter_version"`
	ProductFilesSnapshot json.RawMessage         `json:"product_files_snapshot"`
	SourceSnapshot       json.RawMessage         `json:"source_snapshot"`
	Version              int64                   `json:"version"`
	CreatedBy            string                  `json:"created_by"`
	CreatedAt            time.Time               `json:"created_at"`
	UpdatedAt            time.Time               `json:"updated_at"`
}

func (h MiyunHandoff) Validate() error {
	if err := validateMiyunAggregate(h.ID, h.OrganizationID, h.ProjectID, h.Version, h.CreatedBy, h.CreatedAt, h.UpdatedAt); err != nil {
		return err
	}
	if !h.Status.valid() || strings.TrimSpace(h.SourceMaterialID) == "" || strings.TrimSpace(h.ProductProfileID) == "" ||
		strings.TrimSpace(h.ManifestVersion) == "" || strings.TrimSpace(h.ParameterVersion) == "" {
		return fmt.Errorf("%w: handoff status, source, profile, and schema versions are required", ErrInvalidRequest)
	}
	if !json.Valid(h.ProductFilesSnapshot) || !json.Valid(h.SourceSnapshot) {
		return fmt.Errorf("%w: handoff snapshots must be valid JSON", ErrInvalidRequest)
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
