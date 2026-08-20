package connector

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

var baseTime = time.Date(2026, 8, 20, 8, 0, 0, 0, time.UTC)

func header(id string, available time.Time) FactHeader {
	return FactHeader{
		OrganizationID: "org_1", ProjectID: "project_1", SourceSystem: SourceSystem,
		SourceRef: "account_opaque", IngestRunID: "run_1", SchemaVersion: DatasetVersion,
		PayloadHash: canonicalHash(id), CollectedAt: available, AvailableAt: available,
		DataThrough: available, ValidFrom: available, QualityStatus: QualityAccept, EvidenceRef: "raw_1",
	}
}

func TestLedgerIsIdempotentAndRejectsConflictingPayload(t *testing.T) {
	ledger := NewLedger()
	value := ObjectSnapshot{FactHeader: header("payload-a", baseTime), ID: "object-snapshot-1", ObjectKind: "promotion", ObjectRef: "promotion_opaque"}
	created, err := ledger.AppendObject(value)
	if err != nil || !created {
		t.Fatalf("first append: created=%v error=%v", created, err)
	}
	created, err = ledger.AppendObject(value)
	if err != nil || created {
		t.Fatalf("repeat append: created=%v error=%v", created, err)
	}
	value.PayloadHash = canonicalHash("payload-b")
	if _, err := ledger.AppendObject(value); !errors.Is(err, ErrImmutableConflict) {
		t.Fatalf("conflict error=%v", err)
	}
}

func TestSyncRunResumeMetadataIsIdempotent(t *testing.T) {
	ledger := NewLedger()
	run := SyncRun{ID: "run-1", OrganizationID: "org_1", ProjectID: "project_1", AccountRef: "account_opaque", StartedAt: baseTime, Attempt: 1}
	if created, err := ledger.StartSync(run); err != nil || !created {
		t.Fatalf("start: %v %v", created, err)
	}
	if created, err := ledger.StartSync(run); err != nil || created {
		t.Fatalf("repeat: %v %v", created, err)
	}
	if err := ledger.CompleteSync(run.ID, "page:2", baseTime.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := ledger.CompleteSync(run.ID, "page:2", baseTime.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotEnforcesKnowledgeTime(t *testing.T) {
	ledger := NewLedger()
	for index, available := range []time.Time{baseTime, baseTime.Add(2 * time.Hour)} {
		value := ConfigurationSnapshot{FactHeader: header(string(rune('a'+index)), available), ID: "config-" + string(rune('a'+index)), ObjectRef: "promotion_opaque", Values: map[string]any{"budget": 100 + index}}
		if _, err := ledger.AppendConfiguration(value); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := ledger.Snapshot(context.Background(), Query{OrganizationID: "org_1", ProjectID: "project_1", PredictionCutoff: baseTime.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Configurations) != 1 || snapshot.Configurations[0].ID != "config-a" {
		t.Fatalf("future fact leaked: %#v", snapshot.Configurations)
	}
}

func TestConfigurationChangesAreDeterministic(t *testing.T) {
	ledger := NewLedger()
	before := ConfigurationSnapshot{FactHeader: header("before", baseTime), ID: "before", ObjectRef: "promotion_opaque", Values: map[string]any{"budget": 100, "goal": "lead"}}
	after := ConfigurationSnapshot{FactHeader: header("after", baseTime.Add(time.Hour)), ID: "after", ObjectRef: "promotion_opaque", Values: map[string]any{"budget": 200, "goal": "lead"}}
	if _, err := ledger.AppendConfiguration(before); err != nil {
		t.Fatal(err)
	}
	changes, err := ledger.AppendConfiguration(after)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].FieldPath != "budget" {
		t.Fatalf("changes=%#v", changes)
	}
	if changes[0].OldValue != 100 || changes[0].NewValue != 200 {
		t.Fatalf("change values=%#v", changes[0])
	}
	again := diffConfigurations(before, after)
	if again[0].ID != changes[0].ID || again[0].PayloadHash != changes[0].PayloadHash {
		t.Fatal("change hash is not deterministic")
	}
}

func TestConfigurationChangeUsesStableNestedFieldPath(t *testing.T) {
	before := ConfigurationSnapshot{FactHeader: header("before-nested", baseTime), ID: "before-nested", ObjectRef: "promotion_opaque", Values: map[string]any{"targeting": map[string]any{"region": "north", "age": 18}}}
	after := before
	after.ID = "after-nested"
	after.AvailableAt = baseTime.Add(time.Hour)
	after.Values = map[string]any{"targeting": map[string]any{"region": "south", "age": 18}}
	changes := diffConfigurations(before, after)
	if len(changes) != 1 || changes[0].FieldPath != "targeting.region" || changes[0].OldValue != "north" || changes[0].NewValue != "south" {
		t.Fatalf("changes=%#v", changes)
	}
}

func TestMetricRevisionPreservesOriginal(t *testing.T) {
	ledger := NewLedger()
	first := MetricWindow{FactHeader: header("metric-one", baseTime), ID: "metric-1", ObjectRef: "promotion_opaque", WindowStart: baseTime.Add(-time.Hour), WindowEnd: baseTime, Granularity: "hour", TimeZone: "Asia/Shanghai", AttributionWindow: "7d_click", MetricDefinitionVersion: "oceanengine-metrics/v1", Currency: "CNY", AmountUnit: "fen", Metrics: map[string]int64{"spend": 100, "conversions": 1}}
	if _, err := ledger.AppendMetric(first); err != nil {
		t.Fatal(err)
	}
	revision := first
	revision.ID = "metric-2"
	revision.PayloadHash = canonicalHash("metric-two")
	revision.AvailableAt = baseTime.Add(time.Hour)
	revision.CollectedAt = revision.AvailableAt
	revision.RevisionOf = first.ID
	revision.Metrics = map[string]int64{"spend": 100, "conversions": 2}
	if _, err := ledger.AppendMetric(revision); err != nil {
		t.Fatal(err)
	}
	snapshot, err := ledger.Snapshot(context.Background(), Query{OrganizationID: "org_1", ProjectID: "project_1", PredictionCutoff: baseTime.Add(2 * time.Hour)})
	if err != nil || len(snapshot.Metrics) != 2 {
		t.Fatalf("revision history missing: %#v error=%v", snapshot.Metrics, err)
	}
}

func TestConversionRevisionRequiresOriginalWindow(t *testing.T) {
	ledger := NewLedger()
	window := MetricWindow{FactHeader: header("metric-one", baseTime), ID: "metric-1", ObjectRef: "promotion_opaque", WindowStart: baseTime.Add(-time.Hour), WindowEnd: baseTime, Granularity: "hour", TimeZone: "Asia/Shanghai", AttributionWindow: "7d_click", MetricDefinitionVersion: "oceanengine-metrics/v1", Currency: "CNY", AmountUnit: "fen", Metrics: map[string]int64{"conversions": 1}}
	if _, err := ledger.AppendMetric(window); err != nil {
		t.Fatal(err)
	}
	revisionWindow := window
	revisionWindow.ID = "conversion-revision-1"
	revisionWindow.PayloadHash = canonicalHash("conversion-two")
	revisionWindow.RevisionOf = window.ID
	revision := ConversionRevision{MetricWindow: revisionWindow, OriginalWindowID: window.ID, RevisionNumber: 1}
	if created, err := ledger.AppendConversionRevision(revision); err != nil || !created {
		t.Fatalf("revision: %v %v", created, err)
	}
}

func TestMetricAndDiagnosisHardRules(t *testing.T) {
	ledger := NewLedger()
	invalid := MetricWindow{FactHeader: header("bad", baseTime), ID: "bad", ObjectRef: "p", WindowStart: baseTime, WindowEnd: baseTime, Metrics: map[string]int64{"spend": 1}}
	if _, err := ledger.AppendMetric(invalid); !errors.Is(err, ErrInvalidFact) {
		t.Fatalf("invalid metric error=%v", err)
	}
	diagnosis := PlatformDiagnosisSnapshot{FactHeader: header("diagnosis", baseTime), ID: "diagnosis-1", ObjectRef: "p", EligibleAsPrelaunchFeature: true}
	if _, err := ledger.AppendDiagnosis(diagnosis); !errors.Is(err, ErrInvalidFact) {
		t.Fatalf("diagnosis error=%v", err)
	}
}

func TestRawEvidenceRejectsSensitivePlaintext(t *testing.T) {
	ledger := NewLedger()
	value := RawSnapshot{Header: header("raw", baseTime), ID: "raw-1", Endpoint: "/read", RequestHash: canonicalHash("request"), EncryptedEvidence: []byte("Cookie=session-secret"), KeyVersion: "v1"}
	if _, err := ledger.AppendRaw(value); !errors.Is(err, ErrSensitiveValue) {
		t.Fatalf("sensitive raw error=%v", err)
	}
}

func TestRawSnapshotHasNoJSONRepresentation(t *testing.T) {
	encoded, err := json.Marshal(RawSnapshot{Header: header("raw", baseTime), ID: "raw-1", EncryptedEvidence: []byte("ciphertext")})
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != "{}" {
		t.Fatalf("raw evidence became serializable: %s", encoded)
	}
}

func TestCanonicalFactsRejectTrackingURLsAndCredentialFields(t *testing.T) {
	ledger := NewLedger()
	configuration := ConfigurationSnapshot{FactHeader: header("config", baseTime), ID: "config-1", ObjectRef: "promotion_opaque", Values: map[string]any{"landing_page": "https://example.test/?token=secret"}}
	if _, err := ledger.AppendConfiguration(configuration); !errors.Is(err, ErrSensitiveValue) {
		t.Fatalf("tracking URL error=%v", err)
	}
}

func TestPrelaunchProjectionRemovesDiagnosisAndRawPlatformRefs(t *testing.T) {
	snapshot := CanonicalSnapshot{
		Diagnoses: []PlatformDiagnosisSnapshot{{ID: "diagnosis-1"}},
		Objects:   []ObjectSnapshot{{FactHeader: header("object", baseTime), ObjectRef: "123456789", ParentRef: "987654321"}},
	}
	projection := snapshot.PrelaunchProjection()
	if len(projection.Diagnoses) != 0 {
		t.Fatal("diagnosis leaked into prelaunch projection")
	}
	if projection.Objects[0].ObjectRef == "123456789" || projection.Objects[0].ParentRef == "987654321" {
		t.Fatal("raw platform reference leaked")
	}
}

func TestPrelaunchProjectionRemovesQuarantinedFacts(t *testing.T) {
	accepted := MetricWindow{FactHeader: header("accepted", baseTime)}
	quarantined := MetricWindow{FactHeader: header("quarantined", baseTime)}
	quarantined.QualityStatus = QualityQuarantine
	projection := (CanonicalSnapshot{Metrics: []MetricWindow{accepted, quarantined}}).PrelaunchProjection()
	if len(projection.Metrics) != 1 || projection.Metrics[0].PayloadHash != accepted.PayloadHash {
		t.Fatalf("metrics=%#v", projection.Metrics)
	}
}

func TestMetricQualityDoesNotAssumeConversionsAreBelowClicks(t *testing.T) {
	value := MetricWindow{WindowStart: baseTime, WindowEnd: baseTime.Add(time.Hour), MetricDefinitionVersion: "v1", Currency: "CNY", AmountUnit: "fen", Metrics: map[string]int64{"spend": 10, "impressions": 100, "clicks": 1, "conversions": 2}}
	if issues := AssessMetric(value, true, true, true); len(issues) != 0 {
		t.Fatalf("unexpected conversion constraint: %#v", issues)
	}
	value.Metrics["impressions"] = 0
	if issues := AssessMetric(value, false, true, true); len(issues) != 2 || issues[0].Disposition != QualityQuarantine || issues[1].Disposition != QualityWarning {
		t.Fatalf("quality issues=%#v", issues)
	}
}

func TestMetricQualityWarnsOnRevisionRegressionAndDerivedRateMismatch(t *testing.T) {
	previous := MetricWindow{Metrics: map[string]int64{"spend": 100, "impressions": 100, "clicks": 10, "conversions": 3}}
	current := MetricWindow{Metrics: map[string]int64{"spend": 90, "impressions": 100, "clicks": 10, "conversions": 2}}
	issues := append(AssessMetricRevision(previous, current), AssessDerivedRates(current, map[string]float64{"ctr": 0.2, "cvr": 0.2})...)
	if len(issues) != 3 {
		t.Fatalf("issues=%#v", issues)
	}
	for _, issue := range issues {
		if issue.Disposition != QualityWarning {
			t.Fatalf("issue=%#v", issue)
		}
	}
}
