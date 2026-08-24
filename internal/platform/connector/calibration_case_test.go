package connector

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type calibrationSnapshotReader struct {
	feature CanonicalSnapshot
	label   CanonicalSnapshot
	cutoff  time.Time
	queries []Query
}

func (r *calibrationSnapshotReader) Snapshot(_ context.Context, query Query) (CanonicalSnapshot, error) {
	r.queries = append(r.queries, query)
	if query.PredictionCutoff.Equal(r.cutoff) {
		return r.feature, nil
	}
	return r.label, nil
}

func TestCalibrationExporterBuildsPlanIndependentAnonymizedCase(t *testing.T) {
	prediction := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	label := prediction.AddDate(0, 0, 9)
	promotionRef := AnonymizeRef("raw-promotion")
	projectRef := AnonymizeRef("raw-project")
	productRef := AnonymizeRef("raw-product")
	materialRef := AnonymizeRef("raw-material")
	accountRef := AnonymizeRef("local-account")
	header := func(payload string, available time.Time) FactHeader {
		return FactHeader{SourceRef: accountRef, PayloadHash: payload, AvailableAt: available, ValidFrom: prediction, QualityStatus: QualityAccept}
	}
	feature := CanonicalSnapshot{
		DatasetVersion:   DatasetVersion,
		PredictionCutoff: prediction,
		Objects:          []ObjectSnapshot{{FactHeader: header(strings.Repeat("1", 64), prediction), ID: "promotion-snapshot", ObjectKind: "promotion", ObjectRef: promotionRef, ParentRef: projectRef, State: map[string]any{"product_ref": productRef}}},
		Configurations:   []ConfigurationSnapshot{{FactHeader: header(strings.Repeat("2", 64), prediction), ID: "configuration-snapshot", ObjectRef: promotionRef, Values: map[string]any{"data": map[string]any{"campaign_budget": "1000.00", "project_bid": "90.50", "ad_pricing_name": "oCPM", "external_action_name": "in_app_order", "delivery_mode": 3}, "currency": "CNY"}}},
		Bindings:         []MaterialBinding{{FactHeader: header(strings.Repeat("3", 64), prediction), ID: "binding-snapshot", MaterialRef: materialRef, PromotionRef: promotionRef}},
	}
	metricHeader := header(strings.Repeat("4", 64), label)
	metricHeader.CollectedAt, metricHeader.DataThrough = label, label
	reader := &calibrationSnapshotReader{feature: feature, cutoff: prediction, label: CanonicalSnapshot{
		DatasetVersion:   DatasetVersion,
		PredictionCutoff: label,
		Objects:          feature.Objects, Configurations: feature.Configurations, Bindings: feature.Bindings,
		Metrics:  []MetricWindow{{FactHeader: metricHeader, ID: "metric-snapshot", ObjectRef: promotionRef, WindowStart: prediction, WindowEnd: prediction.AddDate(0, 0, 1), Granularity: "day", TimeZone: "Asia/Shanghai", AttributionWindow: "7d_click", MetricDefinitionVersion: "oceanengine-atomic-v1", Currency: "CNY", AmountUnit: "fen", Metrics: map[string]int64{"spend": 40050, "impressions": 16460, "clicks": 290, "conversions": 2}}},
		Statuses: []PlatformStatusEvent{{FactHeader: header(strings.Repeat("5", 64), label), ID: "status-snapshot", ObjectRef: promotionRef, Status: "active"}},
	}}
	exporter := CalibrationCaseExporter{Reader: reader, Key: []byte("0123456789abcdef0123456789abcdef"), Now: func() time.Time { return label.Add(time.Hour) }}
	result, err := exporter.Export(context.Background(), CalibrationExportRequest{OrganizationID: "org", AccountRef: accountRef, PredictionCutoff: prediction, LabelCutoff: label, HorizonDays: 7, KeyVersion: "local-v1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Cases) != 1 || result.Audit.ExportedCaseCount != 1 || len(reader.queries) != 2 {
		t.Fatalf("result=%#v queries=%d", result.Audit, len(reader.queries))
	}
	if reader.queries[0].ProjectID != "" || reader.queries[1].ProjectID != "" {
		t.Fatalf("Organization export unexpectedly required a Project: %#v", reader.queries)
	}
	caseValue := result.Cases[0]
	if caseValue.SourceBinding.CookiesPlanBinding.State != "unbound_historical" || caseValue.SourceBinding.CookiesPlanBinding.PlanID != nil || caseValue.SourceBinding.CookiesPlanBinding.PlanVersion != nil {
		t.Fatalf("unexpected Plan binding %#v", caseValue.SourceBinding.CookiesPlanBinding)
	}
	if caseValue.Quality.Status != "accepted" || caseValue.Labels.AttributionStatus != "mature" || caseValue.Labels.OperationalOutcome.Disposition != "retained" {
		t.Fatalf("unexpected case quality %#v labels=%#v", caseValue.Quality, caseValue.Labels)
	}
	if caseValue.Prediction.Configuration.Features.BudgetMinor == nil || *caseValue.Prediction.Configuration.Features.BudgetMinor != 100000 || caseValue.Prediction.Configuration.Features.MaterialCount != 1 {
		t.Fatalf("unexpected features %#v", caseValue.Prediction.Configuration.Features)
	}
	encoded, _ := json.Marshal(result)
	for _, raw := range []string{"raw-promotion", "raw-project", "raw-product", "raw-material", "local-account"} {
		if strings.Contains(string(encoded), raw) {
			t.Fatalf("raw identity %q leaked", raw)
		}
	}
	if strings.Contains(string(encoded), `"diagnoses"`) || strings.Contains(string(encoded), `"diagnosis":`) {
		t.Fatal("diagnosis leaked into calibration export")
	}
}

func TestCalibrationExporterQuarantinesUnconfirmedOrIncompleteCases(t *testing.T) {
	prediction := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	label := prediction.AddDate(0, 0, 8)
	promotionRef, projectRef, accountRef := AnonymizeRef("promotion"), AnonymizeRef("project"), AnonymizeRef("account")
	header := FactHeader{SourceRef: accountRef, PayloadHash: strings.Repeat("a", 64), AvailableAt: prediction, ValidFrom: prediction, QualityStatus: QualityAccept}
	metricHeader := header
	metricHeader.AvailableAt, metricHeader.CollectedAt, metricHeader.DataThrough, metricHeader.QualityStatus = label, label, label, QualityQuarantine
	reader := &calibrationSnapshotReader{cutoff: prediction,
		feature: CanonicalSnapshot{DatasetVersion: DatasetVersion, PredictionCutoff: prediction, Objects: []ObjectSnapshot{{FactHeader: header, ID: "promotion", ObjectKind: "promotion", ObjectRef: promotionRef, ParentRef: projectRef, State: map[string]any{}}}, Configurations: []ConfigurationSnapshot{{FactHeader: header, ID: "configuration", ObjectRef: promotionRef, Values: map[string]any{}}}},
		label:   CanonicalSnapshot{DatasetVersion: DatasetVersion, PredictionCutoff: label, Metrics: []MetricWindow{{FactHeader: metricHeader, ID: "metric", ObjectRef: promotionRef, WindowStart: prediction, WindowEnd: prediction.AddDate(0, 0, 1), Granularity: "day", TimeZone: "Asia/Shanghai", AttributionWindow: "platform_default_unconfirmed", MetricDefinitionVersion: "oceanengine-atomic-v1", Currency: "CNY", AmountUnit: "fen", Metrics: map[string]int64{"spend": 0, "impressions": 0, "clicks": 0, "conversions": 0}}}},
	}
	result, err := (CalibrationCaseExporter{Reader: reader, Key: make([]byte, 32)}).Export(context.Background(), CalibrationExportRequest{OrganizationID: "org", ProjectID: "project", AccountRef: accountRef, PredictionCutoff: prediction, LabelCutoff: label, HorizonDays: 7, KeyVersion: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Cases) != 1 || result.Cases[0].Quality.Status != "quarantined" || result.Cases[0].Labels.AttributionStatus != "immature" {
		t.Fatalf("unexpected result %#v", result)
	}
	blockers := strings.Join(result.Cases[0].Quality.Blockers, ",")
	for _, expected := range []string{"product_ref_missing", "material_binding_missing", "metric_window_quarantined", "budget_minor"} {
		if !strings.Contains(blockers, expected) {
			t.Fatalf("missing blocker %q in %q", expected, blockers)
		}
	}
}

func TestCalibrationExporterSkipsCasesWithoutRequiredSnapshots(t *testing.T) {
	prediction := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	label := prediction.AddDate(0, 0, 8)
	accountRef, promotionRef := AnonymizeRef("account"), AnonymizeRef("promotion")
	header := FactHeader{SourceRef: accountRef, PayloadHash: strings.Repeat("a", 64), AvailableAt: prediction, ValidFrom: prediction, QualityStatus: QualityAccept}
	reader := &calibrationSnapshotReader{cutoff: prediction, feature: CanonicalSnapshot{DatasetVersion: DatasetVersion, PredictionCutoff: prediction, Objects: []ObjectSnapshot{{FactHeader: header, ID: "promotion", ObjectKind: "promotion", ObjectRef: promotionRef}}}, label: CanonicalSnapshot{DatasetVersion: DatasetVersion, PredictionCutoff: label}}
	result, err := (CalibrationCaseExporter{Reader: reader, Key: make([]byte, 32)}).Export(context.Background(), CalibrationExportRequest{OrganizationID: "org", ProjectID: "project", AccountRef: accountRef, PredictionCutoff: prediction, LabelCutoff: label, HorizonDays: 7, KeyVersion: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Cases) != 0 || result.Audit.SkippedPromotionCounts["project_ref_missing"] != 1 {
		t.Fatalf("unexpected result %#v", result)
	}
}
