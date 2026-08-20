package oceanengine

import "time"

const (
	PlatformCode      = "ocean_engine"
	ConnectorSchemaV1 = "oceanengine-connector/v1"
	WebAPISessionMode = "web_api"
	SourceConnector   = "connector"
)

type ObjectKind string

const (
	ObjectAccount   ObjectKind = "account"
	ObjectProject   ObjectKind = "project"
	ObjectPromotion ObjectKind = "promotion"
	ObjectMaterial  ObjectKind = "material"
)

type QualityStatus string

const (
	QualityHealthy    QualityStatus = "healthy"
	QualityPartial    QualityStatus = "partial"
	QualityDelayed    QualityStatus = "delayed"
	QualityBlocked    QualityStatus = "blocked"
	QualityIncomplete QualityStatus = "mapping_incomplete"
)

type AccountRef struct {
	OrganizationID string `json:"organization_id"`
	ProjectID      string `json:"project_id"`
	ExternalID     string `json:"external_id"`
}

type EvidenceRef struct {
	BatchID       string    `json:"batch_id"`
	Endpoint      string    `json:"endpoint"`
	RequestHash   string    `json:"request_hash"`
	ResponseHash  string    `json:"response_hash"`
	ObservedAt    time.Time `json:"observed_at"`
	SchemaVersion string    `json:"schema_version"`
}

type SnapshotEnvelope struct {
	SchemaVersion string        `json:"schema_version"`
	Source        string        `json:"source"`
	Platform      string        `json:"platform"`
	Account       AccountRef    `json:"account"`
	CollectedAt   time.Time     `json:"collected_at"`
	DataThrough   *time.Time    `json:"data_through,omitempty"`
	TimeZone      string        `json:"time_zone"`
	Currency      string        `json:"currency"`
	Quality       QualityStatus `json:"quality"`
	QualityNote   string        `json:"quality_note,omitempty"`
	Evidence      []EvidenceRef `json:"evidence,omitempty"`
}

type ObjectSnapshot struct {
	SnapshotEnvelope
	Kind       ObjectKind     `json:"kind"`
	ExternalID string         `json:"external_id"`
	ParentID   string         `json:"parent_id,omitempty"`
	NameHash   string         `json:"name_hash,omitempty"`
	Raw        map[string]any `json:"raw,omitempty"`
}

type MetricWindow struct {
	SnapshotEnvelope
	Kind              ObjectKind     `json:"kind"`
	ExternalID        string         `json:"external_id"`
	ParentID          string         `json:"parent_id,omitempty"`
	Start             time.Time      `json:"start"`
	End               time.Time      `json:"end"`
	Granularity       string         `json:"granularity"`
	MetricSchema      string         `json:"metric_schema"`
	AttributionWindow string         `json:"attribution_window"`
	Counts            MetricCounts   `json:"counts"`
	Raw               map[string]any `json:"raw,omitempty"`
}

type MetricCounts struct {
	SpendCents      int64 `json:"spend_cents"`
	Impressions     int64 `json:"impressions"`
	Clicks          int64 `json:"clicks"`
	Conversions     int64 `json:"conversions"`
	DeepConversions int64 `json:"deep_conversions"`
}

type SyncBatch struct {
	SchemaVersion string        `json:"schema_version"`
	ID            string        `json:"id"`
	Account       AccountRef    `json:"account"`
	WindowStart   time.Time     `json:"window_start"`
	WindowEnd     time.Time     `json:"window_end"`
	Quality       QualityStatus `json:"quality"`
	QualityNote   string        `json:"quality_note,omitempty"`
	ObjectCount   int           `json:"object_count"`
	MetricCount   int           `json:"metric_count"`
	Evidence      []EvidenceRef `json:"evidence,omitempty"`
}

func (k ObjectKind) Valid() bool {
	return k == ObjectAccount || k == ObjectProject || k == ObjectPromotion || k == ObjectMaterial
}

func (q QualityStatus) Valid() bool {
	switch q {
	case QualityHealthy, QualityPartial, QualityDelayed, QualityBlocked, QualityIncomplete:
		return true
	default:
		return false
	}
}
