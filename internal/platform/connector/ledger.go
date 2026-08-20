// Package connector owns the immutable, point-in-time data ledger shared by
// Delivery and Insights. Platform clients must remain in integrations packages.
package connector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	DatasetVersion = "connector-oceanengine-point-in-time-v1"
	SourceSystem   = "ocean_engine"
	ScopeRead      = "connector.read"
	ScopeSync      = "connector.sync"
)

var (
	ErrInvalidFact       = errors.New("invalid connector fact")
	ErrImmutableConflict = errors.New("immutable connector fact conflict")
	ErrSensitiveValue    = errors.New("sensitive value is not permitted")
)

type QualityDisposition string

const (
	QualityAccept     QualityDisposition = "accept"
	QualityReject     QualityDisposition = "reject"
	QualityQuarantine QualityDisposition = "quarantine"
	QualityWarning    QualityDisposition = "warning"
)

type FactHeader struct {
	OrganizationID string             `json:"organization_id"`
	ProjectID      string             `json:"project_id"`
	SourceSystem   string             `json:"source_system"`
	SourceRef      string             `json:"source_ref"`
	IngestRunID    string             `json:"ingest_run_id"`
	SchemaVersion  string             `json:"schema_version"`
	PayloadHash    string             `json:"payload_hash"`
	CollectedAt    time.Time          `json:"collected_at"`
	AvailableAt    time.Time          `json:"available_at"`
	DataThrough    time.Time          `json:"data_through"`
	ValidFrom      time.Time          `json:"valid_from"`
	ValidTo        *time.Time         `json:"valid_to,omitempty"`
	QualityStatus  QualityDisposition `json:"quality_status"`
	EvidenceRef    string             `json:"evidence_ref"`
}

func (h FactHeader) validate() error {
	if h.OrganizationID == "" || h.ProjectID == "" || h.SourceSystem == "" || h.SourceRef == "" ||
		h.IngestRunID == "" || h.SchemaVersion == "" || h.PayloadHash == "" || h.CollectedAt.IsZero() ||
		h.AvailableAt.IsZero() || h.DataThrough.IsZero() || h.ValidFrom.IsZero() || h.QualityStatus == "" || h.EvidenceRef == "" {
		return fmt.Errorf("%w: required lineage or time field is missing", ErrInvalidFact)
	}
	if h.ValidTo != nil && !h.ValidTo.After(h.ValidFrom) {
		return fmt.Errorf("%w: valid_to must be after valid_from", ErrInvalidFact)
	}
	if h.AvailableAt.Before(h.CollectedAt) {
		return fmt.Errorf("%w: available_at cannot precede collection", ErrInvalidFact)
	}
	switch h.QualityStatus {
	case QualityAccept, QualityReject, QualityQuarantine, QualityWarning:
	default:
		return fmt.Errorf("%w: unknown quality disposition", ErrInvalidFact)
	}
	return nil
}

type SyncRun struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	ProjectID      string    `json:"project_id"`
	AccountRef     string    `json:"account_ref"`
	StartedAt      time.Time `json:"started_at"`
	CompletedAt    time.Time `json:"completed_at,omitempty"`
	Cursor         string    `json:"cursor,omitempty"`
	Attempt        int       `json:"attempt"`
	Status         string    `json:"status"`
}

// RawSnapshot contains encrypted or strictly redacted evidence. It has no JSON
// representation so canonical APIs cannot return its payload by accident.
type RawSnapshot struct {
	Header            FactHeader `json:"-"`
	ID                string     `json:"-"`
	Endpoint          string     `json:"-"`
	RequestHash       string     `json:"-"`
	EncryptedEvidence []byte     `json:"-"`
	KeyVersion        string     `json:"-"`
}

type ObjectSnapshot struct {
	FactHeader
	ID         string         `json:"id"`
	ObjectKind string         `json:"object_kind"`
	ObjectRef  string         `json:"object_ref"`
	ParentRef  string         `json:"parent_ref,omitempty"`
	State      map[string]any `json:"state"`
}

type ConfigurationSnapshot struct {
	FactHeader
	ID        string         `json:"id"`
	ObjectRef string         `json:"object_ref"`
	Values    map[string]any `json:"values"`
}

type ConfigurationChangeEvent struct {
	FactHeader
	ID               string    `json:"id"`
	ObjectRef        string    `json:"object_ref"`
	FieldPath        string    `json:"field_path"`
	OldValueHash     string    `json:"old_value_hash"`
	NewValueHash     string    `json:"new_value_hash"`
	OldValue         any       `json:"old_value"`
	NewValue         any       `json:"new_value"`
	BeforeSnapshotID string    `json:"before_snapshot_id"`
	AfterSnapshotID  string    `json:"after_snapshot_id"`
	ObservedAt       time.Time `json:"observed_at"`
}

type MetricWindow struct {
	FactHeader
	ID                      string           `json:"id"`
	ObjectRef               string           `json:"object_ref"`
	WindowStart             time.Time        `json:"window_start"`
	WindowEnd               time.Time        `json:"window_end"`
	Granularity             string           `json:"granularity"`
	TimeZone                string           `json:"time_zone"`
	AttributionWindow       string           `json:"attribution_window"`
	MetricDefinitionVersion string           `json:"metric_definition_version"`
	Currency                string           `json:"currency"`
	AmountUnit              string           `json:"amount_unit"`
	Metrics                 map[string]int64 `json:"metrics"`
	QualityIssues           []QualityIssue   `json:"quality_issues,omitempty"`
	RevisionOf              string           `json:"revision_of,omitempty"`
}

type MaterialBinding struct {
	FactHeader
	ID           string `json:"id"`
	MaterialRef  string `json:"material_ref"`
	PromotionRef string `json:"promotion_ref"`
}

type MaterialMetricWindow struct {
	MetricWindow
	MaterialRef  string `json:"material_ref"`
	PromotionRef string `json:"promotion_ref"`
}

type ConversionRevision struct {
	MetricWindow
	OriginalWindowID string `json:"original_window_id"`
	RevisionNumber   int    `json:"revision_number"`
}

type PlatformStatusEvent struct {
	FactHeader
	ID        string `json:"id"`
	ObjectRef string `json:"object_ref"`
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
}

type PlatformDiagnosisSnapshot struct {
	FactHeader
	ID                         string         `json:"id"`
	ObjectRef                  string         `json:"object_ref"`
	EligibleAsPrelaunchFeature bool           `json:"eligible_as_prelaunch_feature"`
	Diagnosis                  map[string]any `json:"diagnosis"`
}

type CanonicalSnapshot struct {
	DatasetVersion      string                      `json:"dataset_version"`
	PredictionCutoff    time.Time                   `json:"prediction_cutoff"`
	Objects             []ObjectSnapshot            `json:"objects"`
	Configurations      []ConfigurationSnapshot     `json:"configurations"`
	Changes             []ConfigurationChangeEvent  `json:"configuration_changes"`
	Metrics             []MetricWindow              `json:"metric_windows"`
	Bindings            []MaterialBinding           `json:"material_bindings"`
	MaterialMetrics     []MaterialMetricWindow      `json:"material_metric_windows"`
	ConversionRevisions []ConversionRevision        `json:"conversion_revisions"`
	Statuses            []PlatformStatusEvent       `json:"platform_status_events"`
	Diagnoses           []PlatformDiagnosisSnapshot `json:"platform_diagnoses,omitempty"`
}

// PrelaunchProjection removes platform diagnosis facts and hashes all platform
// object references. It is the only supported training export boundary.
func (s CanonicalSnapshot) PrelaunchProjection() CanonicalSnapshot {
	s.Diagnoses = nil
	s.Objects = filterTrainable(s.Objects, func(value ObjectSnapshot) QualityDisposition { return value.QualityStatus })
	s.Configurations = filterTrainable(s.Configurations, func(value ConfigurationSnapshot) QualityDisposition { return value.QualityStatus })
	s.Changes = filterTrainable(s.Changes, func(value ConfigurationChangeEvent) QualityDisposition { return value.QualityStatus })
	s.Metrics = filterTrainable(s.Metrics, func(value MetricWindow) QualityDisposition { return value.QualityStatus })
	s.Bindings = filterTrainable(s.Bindings, func(value MaterialBinding) QualityDisposition { return value.QualityStatus })
	s.MaterialMetrics = filterTrainable(s.MaterialMetrics, func(value MaterialMetricWindow) QualityDisposition { return value.QualityStatus })
	s.ConversionRevisions = filterTrainable(s.ConversionRevisions, func(value ConversionRevision) QualityDisposition { return value.QualityStatus })
	s.Statuses = filterTrainable(s.Statuses, func(value PlatformStatusEvent) QualityDisposition { return value.QualityStatus })
	for index := range s.Objects {
		s.Objects[index].SourceRef = opaqueRef(s.Objects[index].SourceRef)
		s.Objects[index].ObjectRef = opaqueRef(s.Objects[index].ObjectRef)
		s.Objects[index].ParentRef = opaqueRef(s.Objects[index].ParentRef)
	}
	for index := range s.Configurations {
		s.Configurations[index].SourceRef = opaqueRef(s.Configurations[index].SourceRef)
		s.Configurations[index].ObjectRef = opaqueRef(s.Configurations[index].ObjectRef)
	}
	for index := range s.Changes {
		s.Changes[index].SourceRef = opaqueRef(s.Changes[index].SourceRef)
		s.Changes[index].ObjectRef = opaqueRef(s.Changes[index].ObjectRef)
	}
	for index := range s.Metrics {
		s.Metrics[index].SourceRef = opaqueRef(s.Metrics[index].SourceRef)
		s.Metrics[index].ObjectRef = opaqueRef(s.Metrics[index].ObjectRef)
	}
	for index := range s.Bindings {
		s.Bindings[index].SourceRef = opaqueRef(s.Bindings[index].SourceRef)
		s.Bindings[index].MaterialRef = opaqueRef(s.Bindings[index].MaterialRef)
		s.Bindings[index].PromotionRef = opaqueRef(s.Bindings[index].PromotionRef)
	}
	for index := range s.MaterialMetrics {
		s.MaterialMetrics[index].SourceRef = opaqueRef(s.MaterialMetrics[index].SourceRef)
		s.MaterialMetrics[index].ObjectRef = opaqueRef(s.MaterialMetrics[index].ObjectRef)
		s.MaterialMetrics[index].MaterialRef = opaqueRef(s.MaterialMetrics[index].MaterialRef)
		s.MaterialMetrics[index].PromotionRef = opaqueRef(s.MaterialMetrics[index].PromotionRef)
	}
	for index := range s.ConversionRevisions {
		s.ConversionRevisions[index].SourceRef = opaqueRef(s.ConversionRevisions[index].SourceRef)
		s.ConversionRevisions[index].ObjectRef = opaqueRef(s.ConversionRevisions[index].ObjectRef)
	}
	for index := range s.Statuses {
		s.Statuses[index].SourceRef = opaqueRef(s.Statuses[index].SourceRef)
		s.Statuses[index].ObjectRef = opaqueRef(s.Statuses[index].ObjectRef)
	}
	return s
}

func filterTrainable[T any](values []T, status func(T) QualityDisposition) []T {
	result := make([]T, 0, len(values))
	for _, value := range values {
		if disposition := status(value); disposition == QualityAccept || disposition == QualityWarning {
			result = append(result, value)
		}
	}
	return result
}

func opaqueRef(value string) string {
	if value == "" {
		return ""
	}
	return "ref_" + canonicalHash(value)
}

type QualityIssue struct {
	Disposition QualityDisposition `json:"disposition"`
	Code        string             `json:"code"`
}

// AssessMetric applies deterministic quality rules. It does not assume that
// conversions are less than clicks because the platform definition is pending.
func AssessMetric(value MetricWindow, attributionMature, configurationResolved, sourceComplete bool) []QualityIssue {
	issues := make([]QualityIssue, 0)
	if !value.WindowEnd.After(value.WindowStart) {
		issues = append(issues, QualityIssue{QualityReject, "invalid_window"})
	}
	if value.MetricDefinitionVersion == "" {
		issues = append(issues, QualityIssue{QualityReject, "missing_metric_definition"})
	}
	if _, ok := value.Metrics["spend"]; ok && (value.Currency == "" || value.AmountUnit == "") {
		issues = append(issues, QualityIssue{QualityReject, "missing_money_unit"})
	}
	if !attributionMature {
		issues = append(issues, QualityIssue{QualityQuarantine, "attribution_immature"})
	}
	if !configurationResolved {
		issues = append(issues, QualityIssue{QualityQuarantine, "configuration_change_unresolved"})
	}
	if !sourceComplete {
		issues = append(issues, QualityIssue{QualityQuarantine, "source_incomplete"})
	}
	if value.Metrics["impressions"] == 0 && value.Metrics["clicks"] > 0 {
		issues = append(issues, QualityIssue{QualityWarning, "clicks_without_impressions"})
	}
	return issues
}

func AssessMetricRevision(previous, current MetricWindow) []QualityIssue {
	issues := []QualityIssue{}
	if current.Metrics["spend"] < previous.Metrics["spend"] {
		issues = append(issues, QualityIssue{QualityWarning, "spend_regressed"})
	}
	if current.Metrics["conversions"] < previous.Metrics["conversions"] {
		issues = append(issues, QualityIssue{QualityWarning, "conversions_regressed"})
	}
	return issues
}

func AssessDerivedRates(value MetricWindow, derived map[string]float64) []QualityIssue {
	issues := []QualityIssue{}
	if value.Metrics["impressions"] > 0 {
		calculated := float64(value.Metrics["clicks"]) / float64(value.Metrics["impressions"])
		if rate, ok := derived["ctr"]; ok && absFloat(rate-calculated) > 0.000001 {
			issues = append(issues, QualityIssue{QualityWarning, "derived_ctr_mismatch"})
		}
	}
	if value.Metrics["clicks"] > 0 {
		calculated := float64(value.Metrics["conversions"]) / float64(value.Metrics["clicks"])
		if rate, ok := derived["cvr"]; ok && absFloat(rate-calculated) > 0.000001 {
			issues = append(issues, QualityIssue{QualityWarning, "derived_cvr_mismatch"})
		}
	}
	return issues
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func qualityDisposition(issues []QualityIssue) QualityDisposition {
	result := QualityAccept
	for _, issue := range issues {
		if issue.Disposition == QualityReject {
			return QualityReject
		}
		if issue.Disposition == QualityQuarantine {
			result = QualityQuarantine
		} else if issue.Disposition == QualityWarning && result == QualityAccept {
			result = QualityWarning
		}
	}
	return result
}

type Query struct {
	OrganizationID   string
	ProjectID        string
	ObjectRef        string
	SourceRef        string
	WindowStart      time.Time
	WindowEnd        time.Time
	PredictionCutoff time.Time
	IncludeDiagnosis bool
}

type Ledger struct {
	mu              sync.RWMutex
	runs            map[string]SyncRun
	raw             map[string]RawSnapshot
	objects         map[string]ObjectSnapshot
	configurations  map[string]ConfigurationSnapshot
	changes         map[string]ConfigurationChangeEvent
	metrics         map[string]MetricWindow
	bindings        map[string]MaterialBinding
	materialMetrics map[string]MaterialMetricWindow
	conversions     map[string]ConversionRevision
	statuses        map[string]PlatformStatusEvent
	diagnoses       map[string]PlatformDiagnosisSnapshot
}

func NewLedger() *Ledger {
	return &Ledger{runs: map[string]SyncRun{}, raw: map[string]RawSnapshot{}, objects: map[string]ObjectSnapshot{}, configurations: map[string]ConfigurationSnapshot{}, changes: map[string]ConfigurationChangeEvent{}, metrics: map[string]MetricWindow{}, bindings: map[string]MaterialBinding{}, materialMetrics: map[string]MaterialMetricWindow{}, conversions: map[string]ConversionRevision{}, statuses: map[string]PlatformStatusEvent{}, diagnoses: map[string]PlatformDiagnosisSnapshot{}}
}

func (l *Ledger) StartSync(value SyncRun) (bool, error) {
	if value.ID == "" || value.OrganizationID == "" || value.ProjectID == "" || value.AccountRef == "" || value.StartedAt.IsZero() || value.Attempt < 1 {
		return false, ErrInvalidFact
	}
	if value.Status == "" {
		value.Status = "running"
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if current, ok := l.runs[value.ID]; ok {
		if canonicalHash(current) == canonicalHash(value) {
			return false, nil
		}
		return false, ErrImmutableConflict
	}
	l.runs[value.ID] = value
	return true, nil
}

// CompleteSync changes only run-control metadata. Ledger facts stay immutable.
func (l *Ledger) CompleteSync(id, cursor string, completedAt time.Time) error {
	if id == "" || completedAt.IsZero() {
		return ErrInvalidFact
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	value, ok := l.runs[id]
	if !ok || completedAt.Before(value.StartedAt) {
		return ErrInvalidFact
	}
	if !value.CompletedAt.IsZero() {
		if value.CompletedAt.Equal(completedAt) && value.Cursor == cursor {
			return nil
		}
		return ErrImmutableConflict
	}
	value.CompletedAt, value.Cursor = completedAt, cursor
	value.Status = "completed"
	l.runs[id] = value
	return nil
}

type SnapshotReader interface {
	Snapshot(context.Context, Query) (CanonicalSnapshot, error)
}

var _ SnapshotReader = (*Ledger)(nil)

func canonicalHash(value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func appendImmutable[T any](records map[string]T, id, payloadHash string, value T, hashOf func(T) string) (bool, error) {
	if current, ok := records[id]; ok {
		if hashOf(current) == payloadHash {
			return false, nil
		}
		return false, ErrImmutableConflict
	}
	records[id] = value
	return true, nil
}

func (l *Ledger) AppendRaw(value RawSnapshot) (bool, error) {
	if err := value.Header.validate(); err != nil {
		return false, err
	}
	if value.ID == "" || value.Endpoint == "" || value.RequestHash == "" || len(value.EncryptedEvidence) == 0 || value.KeyVersion == "" {
		return false, ErrInvalidFact
	}
	if containsSensitiveText(string(value.EncryptedEvidence)) {
		return false, ErrSensitiveValue
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return appendImmutable(l.raw, value.ID, value.Header.PayloadHash, value, func(v RawSnapshot) string { return v.Header.PayloadHash })
}

func containsSensitiveText(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"cookie", "token", "csrf", "authorization", "http://", "https://", "device_id"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func containsSensitiveValue(value any) bool {
	encoded, err := json.Marshal(value)
	return err != nil || containsSensitiveText(string(encoded))
}

func (l *Ledger) AppendObject(value ObjectSnapshot) (bool, error) {
	if err := value.FactHeader.validate(); err != nil {
		return false, err
	}
	if value.ID == "" || value.ObjectKind == "" || value.ObjectRef == "" {
		return false, ErrInvalidFact
	}
	if containsSensitiveValue(value.State) {
		return false, ErrSensitiveValue
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return appendImmutable(l.objects, value.ID, value.PayloadHash, value, func(v ObjectSnapshot) string { return v.PayloadHash })
}

func (l *Ledger) AppendConfiguration(value ConfigurationSnapshot) ([]ConfigurationChangeEvent, error) {
	if err := value.FactHeader.validate(); err != nil {
		return nil, err
	}
	if value.ID == "" || value.ObjectRef == "" {
		return nil, ErrInvalidFact
	}
	if containsSensitiveValue(value.Values) {
		return nil, ErrSensitiveValue
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if current, ok := l.configurations[value.ID]; ok {
		if current.PayloadHash == value.PayloadHash {
			return nil, nil
		}
		return nil, ErrImmutableConflict
	}
	var previous *ConfigurationSnapshot
	for _, candidate := range l.configurations {
		if candidate.OrganizationID == value.OrganizationID && candidate.ProjectID == value.ProjectID && candidate.ObjectRef == value.ObjectRef && candidate.AvailableAt.Before(value.AvailableAt) && (previous == nil || candidate.AvailableAt.After(previous.AvailableAt)) {
			copy := candidate
			previous = &copy
		}
	}
	l.configurations[value.ID] = value
	if previous == nil {
		return nil, nil
	}
	changes := diffConfigurations(*previous, value)
	for _, change := range changes {
		l.changes[change.ID] = change
	}
	return changes, nil
}

func diffConfigurations(before, after ConfigurationSnapshot) []ConfigurationChangeEvent {
	beforeValues, afterValues := flattenConfiguration(before.Values), flattenConfiguration(after.Values)
	keys := map[string]struct{}{}
	for key := range beforeValues {
		keys[key] = struct{}{}
	}
	for key := range afterValues {
		keys[key] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	result := make([]ConfigurationChangeEvent, 0)
	for _, key := range ordered {
		oldHash, newHash := canonicalHash(beforeValues[key]), canonicalHash(afterValues[key])
		if oldHash == newHash {
			continue
		}
		id := canonicalHash([]string{before.ID, after.ID, key, oldHash, newHash})
		header := after.FactHeader
		header.PayloadHash = canonicalHash([]string{id, oldHash, newHash})
		result = append(result, ConfigurationChangeEvent{FactHeader: header, ID: id, ObjectRef: after.ObjectRef, FieldPath: key, OldValueHash: oldHash, NewValueHash: newHash, OldValue: beforeValues[key], NewValue: afterValues[key], BeforeSnapshotID: before.ID, AfterSnapshotID: after.ID, ObservedAt: after.AvailableAt})
	}
	return result
}

func flattenConfiguration(values map[string]any) map[string]any {
	result := map[string]any{}
	var walk func(string, any)
	walk = func(path string, value any) {
		nested, ok := value.(map[string]any)
		if !ok || len(nested) == 0 {
			result[path] = value
			return
		}
		keys := make([]string, 0, len(nested))
		for key := range nested {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			next := key
			if path != "" {
				next = path + "." + key
			}
			walk(next, nested[key])
		}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		walk(key, values[key])
	}
	return result
}

func validateMetric(value MetricWindow) error {
	if err := value.FactHeader.validate(); err != nil {
		return err
	}
	if value.ID == "" || value.ObjectRef == "" || !value.WindowEnd.After(value.WindowStart) || value.MetricDefinitionVersion == "" || value.Granularity == "" || value.TimeZone == "" || value.AttributionWindow == "" {
		return ErrInvalidFact
	}
	if _, hasSpend := value.Metrics["spend"]; hasSpend && (value.Currency == "" || value.AmountUnit == "") {
		return ErrInvalidFact
	}
	return nil
}

func (l *Ledger) AppendMetric(value MetricWindow) (bool, error) {
	if err := validateMetric(value); err != nil {
		return false, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if current, ok := l.metrics[value.ID]; ok {
		if current.PayloadHash == value.PayloadHash {
			return false, nil
		}
		return false, ErrImmutableConflict
	}
	if value.RevisionOf != "" {
		base, ok := l.metrics[value.RevisionOf]
		if !ok || base.ObjectRef != value.ObjectRef || !base.WindowStart.Equal(value.WindowStart) || !base.WindowEnd.Equal(value.WindowEnd) {
			return false, ErrInvalidFact
		}
	}
	l.metrics[value.ID] = value
	return true, nil
}

func (l *Ledger) AppendMaterialMetric(value MaterialMetricWindow) (bool, error) {
	if value.MaterialRef == "" || value.PromotionRef == "" {
		return false, ErrInvalidFact
	}
	if err := validateMetric(value.MetricWindow); err != nil {
		return false, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return appendImmutable(l.materialMetrics, value.ID, value.PayloadHash, value, func(v MaterialMetricWindow) string { return v.PayloadHash })
}

func (l *Ledger) AppendConversionRevision(value ConversionRevision) (bool, error) {
	if value.OriginalWindowID == "" || value.RevisionNumber < 1 || value.RevisionOf == "" {
		return false, ErrInvalidFact
	}
	if err := validateMetric(value.MetricWindow); err != nil {
		return false, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	base, ok := l.metrics[value.RevisionOf]
	if !ok || base.ID != value.OriginalWindowID || base.ObjectRef != value.ObjectRef || !base.WindowStart.Equal(value.WindowStart) || !base.WindowEnd.Equal(value.WindowEnd) {
		return false, ErrInvalidFact
	}
	return appendImmutable(l.conversions, value.ID, value.PayloadHash, value, func(v ConversionRevision) string { return v.PayloadHash })
}

func (l *Ledger) AppendBinding(value MaterialBinding) (bool, error) {
	return appendHeaderRecord(&l.mu, l.bindings, value.ID, value.FactHeader, value, func(v MaterialBinding) string { return v.PayloadHash })
}
func (l *Ledger) AppendStatus(value PlatformStatusEvent) (bool, error) {
	return appendHeaderRecord(&l.mu, l.statuses, value.ID, value.FactHeader, value, func(v PlatformStatusEvent) string { return v.PayloadHash })
}
func (l *Ledger) AppendDiagnosis(value PlatformDiagnosisSnapshot) (bool, error) {
	if value.EligibleAsPrelaunchFeature {
		return false, ErrInvalidFact
	}
	if containsSensitiveValue(value.Diagnosis) {
		return false, ErrSensitiveValue
	}
	return appendHeaderRecord(&l.mu, l.diagnoses, value.ID, value.FactHeader, value, func(v PlatformDiagnosisSnapshot) string { return v.PayloadHash })
}

func appendHeaderRecord[T any](mu *sync.RWMutex, records map[string]T, id string, header FactHeader, value T, hashOf func(T) string) (bool, error) {
	if err := header.validate(); err != nil {
		return false, err
	}
	if id == "" {
		return false, ErrInvalidFact
	}
	mu.Lock()
	defer mu.Unlock()
	return appendImmutable(records, id, header.PayloadHash, value, hashOf)
}

func visible(h FactHeader, q Query) bool {
	if h.OrganizationID != q.OrganizationID || h.ProjectID != q.ProjectID || h.AvailableAt.After(q.PredictionCutoff) || h.QualityStatus == QualityReject {
		return false
	}
	if q.SourceRef != "" && h.SourceRef != q.SourceRef {
		return false
	}
	return true
}

func AnonymizeRef(value string) string { return opaqueRef(value) }

func (l *Ledger) Snapshot(_ context.Context, q Query) (CanonicalSnapshot, error) {
	if q.OrganizationID == "" || q.ProjectID == "" || q.PredictionCutoff.IsZero() {
		return CanonicalSnapshot{}, ErrInvalidFact
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := CanonicalSnapshot{DatasetVersion: DatasetVersion, PredictionCutoff: q.PredictionCutoff}
	for _, v := range l.objects {
		if visible(v.FactHeader, q) && (q.ObjectRef == "" || v.ObjectRef == q.ObjectRef) {
			result.Objects = append(result.Objects, v)
		}
	}
	for _, v := range l.configurations {
		if visible(v.FactHeader, q) && (q.ObjectRef == "" || v.ObjectRef == q.ObjectRef) {
			result.Configurations = append(result.Configurations, v)
		}
	}
	for _, v := range l.changes {
		if visible(v.FactHeader, q) && (q.ObjectRef == "" || v.ObjectRef == q.ObjectRef) {
			result.Changes = append(result.Changes, v)
		}
	}
	for _, v := range l.metrics {
		if visible(v.FactHeader, q) && (q.ObjectRef == "" || v.ObjectRef == q.ObjectRef) && (q.WindowStart.IsZero() || !v.WindowEnd.Before(q.WindowStart)) && (q.WindowEnd.IsZero() || !v.WindowStart.After(q.WindowEnd)) {
			result.Metrics = append(result.Metrics, v)
		}
	}
	for _, v := range l.bindings {
		if visible(v.FactHeader, q) && (q.ObjectRef == "" || v.MaterialRef == q.ObjectRef || v.PromotionRef == q.ObjectRef) {
			result.Bindings = append(result.Bindings, v)
		}
	}
	for _, v := range l.materialMetrics {
		if visible(v.FactHeader, q) && (q.ObjectRef == "" || v.MaterialRef == q.ObjectRef || v.PromotionRef == q.ObjectRef) {
			result.MaterialMetrics = append(result.MaterialMetrics, v)
		}
	}
	for _, v := range l.conversions {
		if visible(v.FactHeader, q) && (q.ObjectRef == "" || v.ObjectRef == q.ObjectRef) {
			result.ConversionRevisions = append(result.ConversionRevisions, v)
		}
	}
	for _, v := range l.statuses {
		if visible(v.FactHeader, q) && (q.ObjectRef == "" || v.ObjectRef == q.ObjectRef) {
			result.Statuses = append(result.Statuses, v)
		}
	}
	if q.IncludeDiagnosis {
		for _, v := range l.diagnoses {
			if visible(v.FactHeader, q) && (q.ObjectRef == "" || v.ObjectRef == q.ObjectRef) {
				result.Diagnoses = append(result.Diagnoses, v)
			}
		}
	}
	sort.Slice(result.Objects, func(i, j int) bool { return result.Objects[i].AvailableAt.Before(result.Objects[j].AvailableAt) })
	sort.Slice(result.Configurations, func(i, j int) bool {
		return result.Configurations[i].AvailableAt.Before(result.Configurations[j].AvailableAt)
	})
	sort.Slice(result.Metrics, func(i, j int) bool { return result.Metrics[i].AvailableAt.Before(result.Metrics[j].AvailableAt) })
	return result, nil
}
