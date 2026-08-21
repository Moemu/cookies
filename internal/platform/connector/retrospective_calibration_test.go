package connector

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type retrospectiveSnapshotReader struct{ snapshot CanonicalSnapshot }

func (r retrospectiveSnapshotReader) Snapshot(context.Context, Query) (CanonicalSnapshot, error) {
	return r.snapshot, nil
}

func TestRetrospectiveCalibrationBuildsLeakageMarkedRollingCases(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	available := base.AddDate(0, 0, 30)
	header := FactHeader{OrganizationID: "org", SourceSystem: SourceSystem, SourceRef: "ref_account", IngestRunID: "run", SchemaVersion: DatasetVersion, PayloadHash: strings.Repeat("a", 64), CollectedAt: available, AvailableAt: available, DataThrough: available, ValidFrom: available, QualityStatus: QualityAccept, EvidenceRef: "raw"}
	snapshot := CanonicalSnapshot{DatasetVersion: DatasetVersion, Objects: []ObjectSnapshot{{FactHeader: header, ID: "raw_promotion_object", ObjectKind: "promotion", ObjectRef: "ref_promotion", ParentRef: "ref_project"}}, Configurations: []ConfigurationSnapshot{{FactHeader: header, ID: "current_configuration_must_not_be_used", ObjectRef: "ref_promotion", Values: map[string]any{"budget": 999999}}}}
	for day := 0; day < 21; day++ {
		windowStart := base.AddDate(0, 0, day)
		windowEnd := windowStart.AddDate(0, 0, 1)
		metricHeader := header
		metricHeader.PayloadHash = strings.Repeat(string(rune('b'+day%20)), 64)
		metricHeader.ValidFrom = windowStart
		metricHeader.DataThrough = windowEnd
		metricHeader.QualityStatus = QualityQuarantine
		snapshot.Metrics = append(snapshot.Metrics, MetricWindow{FactHeader: metricHeader, ID: "raw_metric_" + windowStart.Format("20060102"), ObjectRef: "ref_promotion", WindowStart: windowStart, WindowEnd: windowEnd, Granularity: "day", TimeZone: "Asia/Shanghai", AttributionWindow: "unknown", MetricDefinitionVersion: "oceanengine-stat-v1", Currency: "CNY", AmountUnit: "fen", Metrics: map[string]int64{"spend": 100, "impressions": 1000, "clicks": 10, "conversions": 1}, QualityIssues: []QualityIssue{{Disposition: QualityQuarantine, Code: "attribution_immature"}}})
	}
	builder := RetrospectiveCalibrationBuilder{Reader: retrospectiveSnapshotReader{snapshot: snapshot}, Key: []byte("0123456789abcdef0123456789abcdef"), Now: func() time.Time { return available.Add(time.Hour) }}
	result, err := builder.Build(context.Background(), RetrospectiveCalibrationRequest{OrganizationID: "org", AccountRef: "ref_account", KnowledgeCutoff: available, ReplayStart: base.AddDate(0, 0, 7), ReplayEnd: base.AddDate(0, 0, 21), LookbackDays: 7, HorizonDays: 2, StepDays: 2, MinimumHistoryWindows: 7, KeyVersion: "test-v1"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.RetrospectiveOnly || len(result.Cases) != 7 || result.Policy.ConfigurationUsage != "excluded_current_snapshot_not_historical" {
		t.Fatalf("result summary=%#v policy=%#v", result.Summary, result.Policy)
	}
	first := result.Cases[0]
	if first.CookiesPlanBinding.State != "unbound_historical" || first.CookiesPlanBinding.PlanID != nil || first.BaselinePrediction.Conversions != nil || first.History.Conversions != nil || first.Observed.Conversions != nil || first.HistoryWindowCount != 7 || first.LabelWindowCount != 2 {
		t.Fatalf("case=%#v", first)
	}
	if len(result.Calibration) != 3 || len(result.Evaluation) != 6 {
		t.Fatalf("conversion entered evaluation: %#v", result.Evaluation)
	}
	for _, evaluation := range result.Evaluation {
		if evaluation.DatasetSplit != "time_holdout" || evaluation.Metric == "conversions" {
			t.Fatalf("invalid evaluation boundary: %#v", evaluation)
		}
	}
	if result.ModelSelection.AppliedToSimulator || result.ModelSelection.CandidateStatus == "" {
		t.Fatalf("model selection did not preserve the manual promotion boundary: %#v", result.ModelSelection)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"raw_promotion_object", "raw_metric_", "current_configuration_must_not_be_used", "ref_promotion", "ref_project"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("raw Connector lineage leaked: %s", forbidden)
		}
	}
}

func TestRetrospectiveCalibrationRejectsReplayBeyondKnowledgeCutoff(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	builder := RetrospectiveCalibrationBuilder{Reader: retrospectiveSnapshotReader{}, Key: make([]byte, 32)}
	_, err := builder.Build(context.Background(), RetrospectiveCalibrationRequest{OrganizationID: "org", AccountRef: "ref_account", KnowledgeCutoff: now, ReplayStart: now.AddDate(0, 0, -7), ReplayEnd: now.AddDate(0, 0, 1), LookbackDays: 7, HorizonDays: 1, StepDays: 1, MinimumHistoryWindows: 1, KeyVersion: "v1"})
	if err != ErrInvalidFact {
		t.Fatalf("error=%v", err)
	}
}
