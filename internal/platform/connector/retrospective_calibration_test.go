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
		metric := MetricWindow{FactHeader: metricHeader, ID: "raw_metric_" + windowStart.Format("20060102"), ObjectRef: "ref_promotion", WindowStart: windowStart, WindowEnd: windowEnd, Granularity: "day", TimeZone: "Asia/Shanghai", AttributionWindow: "unknown", MetricDefinitionVersion: "oceanengine-stat-v1", Currency: "CNY", AmountUnit: "fen", Metrics: map[string]int64{"spend": 100, "impressions": 1000, "clicks": 10, "conversions": 1}, QualityIssues: []QualityIssue{{Disposition: QualityQuarantine, Code: "attribution_immature"}}}
		snapshot.Metrics = append(snapshot.Metrics, metric)
		metric.ID = "raw_historical_only_metric_" + windowStart.Format("20060102")
		metric.ObjectRef = "ref_historical_only_promotion"
		snapshot.Metrics = append(snapshot.Metrics, metric)
	}
	builder := RetrospectiveCalibrationBuilder{Reader: retrospectiveSnapshotReader{snapshot: snapshot}, Key: []byte("0123456789abcdef0123456789abcdef"), Now: func() time.Time { return available.Add(time.Hour) }}
	result, err := builder.Build(context.Background(), RetrospectiveCalibrationRequest{OrganizationID: "org", AccountRef: "ref_account", KnowledgeCutoff: available, ReplayStart: base.AddDate(0, 0, 7), ReplayEnd: base.AddDate(0, 0, 21), LookbackDays: 7, HorizonDays: 2, StepDays: 2, MinimumHistoryWindows: 7, KeyVersion: "test-v1"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.RetrospectiveOnly || len(result.Cases) != 14 || result.Policy.ConfigurationUsage != "excluded_current_snapshot_not_historical" {
		t.Fatalf("result summary=%#v policy=%#v", result.Summary, result.Policy)
	}
	if result.Summary.MetricPromotionCount != 2 || result.Summary.InventoryMatchedMetricPromotionCount != 1 || result.Summary.LineageLimitedCaseCount != 7 {
		t.Fatalf("historical metric lineage summary=%#v", result.Summary)
	}
	if result.SchemaVersion != RetrospectiveCalibrationSchemaVersion || result.Diagnostics.UsedForPrediction {
		t.Fatalf("invalid diagnostics boundary: schema=%s diagnostics=%#v", result.SchemaVersion, result.Diagnostics)
	}
	var first RetrospectiveCalibrationCase
	lineageLimited := 0
	for _, value := range result.Cases {
		if value.LineageStatus == "project_relationship_available" {
			first = value
		} else if value.LineageStatus == "project_relationship_unavailable" && value.ProjectRef == "" && value.QualityStatus == "retrospective_lineage_limited" {
			lineageLimited++
		}
	}
	if lineageLimited != 7 {
		t.Fatalf("lineage-limited cases=%d", lineageLimited)
	}
	if first.CookiesPlanBinding.State != "unbound_historical" || first.CookiesPlanBinding.PlanID != nil || first.BaselinePrediction.Conversions != nil || first.History.Conversions != nil || first.Observed.Conversions != nil || first.HistoryWindowCount != 7 || first.LabelWindowCount != 2 {
		t.Fatalf("case=%#v", first)
	}
	if len(result.Calibration) != 3 || len(result.Evaluation) < 6 {
		t.Fatalf("conversion entered evaluation: %#v", result.Evaluation)
	}
	for _, evaluation := range result.Evaluation {
		if !strings.HasPrefix(evaluation.DatasetSplit, "time_holdout") || evaluation.Metric == "conversions" {
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
	for _, forbidden := range []string{"raw_promotion_object", "raw_metric_", "raw_historical_only_metric_", "current_configuration_must_not_be_used", "ref_promotion", "ref_historical_only_promotion", "ref_project"} {
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

func TestCohortContinuationCanPassStratifiedTimeHoldout(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := make([]RetrospectiveCalibrationCase, 0, 180)
	for day := 0; day < 180; day++ {
		cutoff := base.AddDate(0, 0, day)
		promotionRef := "known_promotion"
		if day >= 150 {
			promotionRef = "cold_" + cutoff.Format("20060102")
		}
		signal := RetrospectiveLifecycleSignal{AgeDays: 2, AgeBucket: "launch", RecencyBucket: "recent", StreakBucket: "short", TrendBucket: "stable"}
		spendSignal, impressionSignal, clickSignal := signal, signal, signal
		spendSignal.LatestValue, impressionSignal.LatestValue, clickSignal.LatestValue = 200, 2000, 20
		cases = append(cases, RetrospectiveCalibrationCase{CaseID: cutoff.Format("20060102"), PromotionRef: promotionRef, PredictionCutoff: cutoff, HorizonEnd: cutoff.Add(24 * time.Hour), History: RetrospectiveMetricTotals{SpendMinor: 1000, Impressions: 10000, Clicks: 100}, HistoryActivity: RetrospectiveMetricActivity{ObservedWindows: 7, SpendPositiveWindows: 7, ImpressionPositiveWindows: 7, ClickPositiveWindows: 7}, Lifecycle: RetrospectiveLifecycleFeatures{Spend: spendSignal, Impressions: impressionSignal, Clicks: clickSignal}, BaselinePrediction: RetrospectiveMetricEstimate{SpendMinor: 500, Impressions: 5000, Clicks: 50}, Observed: RetrospectiveMetricTotals{SpendMinor: 200, Impressions: 2000, Clicks: 20}})
	}
	holdoutStart := base.AddDate(0, 0, 120)
	calibration, evaluation, predictions := calibrateAndEvaluateRetrospectiveCohort(cases, holdoutStart)
	if calibration.Status != "evaluated" || calibration.TrainingCaseCount != 120 || calibration.HoldoutCaseCount != 60 || len(predictions) != 60 {
		t.Fatalf("calibration=%#v predictions=%d", calibration, len(predictions))
	}
	training, holdout := splitRetrospectiveCases(cases, holdoutStart)
	cold, warm := retrospectiveHoldoutCohorts(holdout, retrospectivePromotionSet(training))
	baseline := evaluateRetrospectiveCases(holdout, "trailing_mean_baseline", "time_holdout", nil)
	baseline = append(baseline, evaluateRetrospectiveCases(cold, "trailing_mean_baseline", "time_holdout_cold_start", nil)...)
	baseline = append(baseline, evaluateRetrospectiveCases(warm, "trailing_mean_baseline", "time_holdout_warm_start", nil)...)
	selection := selectRetrospectiveModel(append(baseline, evaluation...), diagnoseRetrospectiveCalibration(cases, holdoutStart))
	if !selection.HoldoutGatePassed || selection.SelectedModel != RetrospectiveCohortModelVersion {
		t.Fatalf("selection=%#v evaluation=%#v", selection, evaluation)
	}
}

func TestRetrospectiveLifecycleFeaturesExcludeMetricsAfterCutoff(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	metrics := make([]MetricWindow, 0, 4)
	for day := 0; day < 4; day++ {
		start := base.AddDate(0, 0, day)
		spend := int64(100)
		if day == 3 {
			spend = 0
		}
		metrics = append(metrics, MetricWindow{WindowStart: start, WindowEnd: start.Add(24 * time.Hour), Granularity: "day", Currency: "CNY", AmountUnit: "fen", MetricDefinitionVersion: "v1", Metrics: map[string]int64{"spend": spend, "impressions": spend, "clicks": spend, "conversions": 0}})
	}
	cutoff := base.AddDate(0, 0, 3)
	features := retrospectiveLifecycleFeatures(metrics, metrics[:3], cutoff)
	if features.Spend.AgeDays != 3 || features.Spend.RecencyBucket != "recent" || features.Spend.ConsecutivePositiveWindows != 3 || features.Spend.StreakBucket != "short" {
		t.Fatalf("future metric changed lifecycle features: %#v", features.Spend)
	}
}

func TestRetrospectiveDiagnosticsDetectTemporalAndCohortShift(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	holdoutStart := base.AddDate(0, 0, 10)
	cases := make([]RetrospectiveCalibrationCase, 0, 40)
	for day := 0; day < 10; day++ {
		cutoff := base.AddDate(0, 0, day)
		cases = append(cases, RetrospectiveCalibrationCase{PromotionRef: "known", PredictionCutoff: cutoff, Observed: RetrospectiveMetricTotals{}})
	}
	for index := 0; index < 30; index++ {
		cutoff := holdoutStart.AddDate(0, 0, index%3)
		cases = append(cases, RetrospectiveCalibrationCase{PromotionRef: "new_" + cutoff.Format("20060102"), PredictionCutoff: cutoff, Observed: RetrospectiveMetricTotals{SpendMinor: 100, Impressions: 100, Clicks: 100}})
	}
	diagnostics := diagnoseRetrospectiveCalibration(cases, holdoutStart)
	if diagnostics.Status != "insufficient_segment_coverage" || diagnostics.UsedForPrediction || diagnostics.Holdout.CutoffDateCount != 3 || diagnostics.Holdout.UnseenPromotionShare != 1 || diagnostics.HoldoutColdStart.CaseCount != 30 || diagnostics.HoldoutWarmStart.CaseCount != 0 {
		t.Fatalf("diagnostics=%#v", diagnostics)
	}
	wanted := map[string]bool{"holdout_cutoff_dates_below_minimum": false, "warm_start_holdout_cases_below_minimum": false}
	for _, signal := range diagnostics.Signals {
		if _, ok := wanted[signal]; ok {
			wanted[signal] = true
		}
	}
	for signal, found := range wanted {
		if !found {
			t.Fatalf("missing signal %s in %#v", signal, diagnostics.Signals)
		}
	}
	for _, signal := range diagnostics.Signals {
		if signal == "holdout_unseen_promotion_share_above_limit" {
			t.Fatalf("cold-start workload became a hard signal: %#v", diagnostics.Signals)
		}
	}
	baselineWAPE, candidateWAPE := 0.8, 0.4
	evaluations := make([]RetrospectiveMetricEvaluation, 0, 6)
	for _, metric := range []string{"spend_minor", "impressions", "clicks"} {
		for _, split := range []string{"time_holdout", "time_holdout_cold_start", "time_holdout_warm_start"} {
			evaluations = append(evaluations,
				RetrospectiveMetricEvaluation{Model: "trailing_mean_baseline", DatasetSplit: split, Metric: metric, MeanBias: 10, WAPE: &baselineWAPE},
				RetrospectiveMetricEvaluation{Model: RetrospectiveCohortModelVersion, DatasetSplit: split, Metric: metric, MeanBias: 5, WAPE: &candidateWAPE},
			)
		}
	}
	selection := selectRetrospectiveModel(evaluations, diagnostics)
	if selection.HoldoutGatePassed || selection.CandidateStatus != "rejected" {
		t.Fatalf("distribution shift did not block promotion: %#v", selection)
	}
}

func TestRetrospectiveDiagnosticsRejectMissingUnclassifiedLabels(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := make([]RetrospectiveCalibrationCase, 0, 80)
	for day := 0; day < 80; day++ {
		cases = append(cases, RetrospectiveCalibrationCase{PromotionRef: "promotion", PredictionCutoff: base.AddDate(0, 0, day), Observed: RetrospectiveMetricTotals{SpendMinor: 1, Impressions: 1, Clicks: 1}})
	}
	diagnostics := diagnoseRetrospectiveCalibration(cases, base.AddDate(0, 0, 40), RetrospectiveCalibrationSummary{ExportedCaseCount: 80, SkippedCaseCounts: map[string]int{"label_windows_missing": 20}})
	if diagnostics.Status != "insufficient_label_observability" || diagnostics.LabelObservability.HistoryEligibleFoldCount != 100 || diagnostics.LabelObservability.ObservedLabelShare != 0.8 {
		t.Fatalf("diagnostics=%#v", diagnostics)
	}
	found := false
	for _, signal := range diagnostics.Signals {
		if signal == "missing_daily_rows_lack_coverage_evidence" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing label observability signal: %#v", diagnostics.Signals)
	}
}

func TestLifecycleHurdleRejectsTrainingWithoutPositiveOutcomes(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := make([]RetrospectiveCalibrationCase, 0, 60)
	for day := 0; day < 60; day++ {
		cutoff := base.AddDate(0, 0, day)
		observed := RetrospectiveMetricTotals{}
		if day >= 30 {
			observed = RetrospectiveMetricTotals{SpendMinor: 100, Impressions: 100, Clicks: 10}
		}
		cases = append(cases, RetrospectiveCalibrationCase{CaseID: cutoff.Format("20060102"), PredictionCutoff: cutoff, HorizonEnd: cutoff.Add(24 * time.Hour), HistoryActivity: RetrospectiveMetricActivity{ObservedWindows: 1}, Observed: observed})
	}
	calibration, evaluation, predictions := calibrateAndEvaluateRetrospectiveLifecycle(cases, base.AddDate(0, 0, 30))
	if calibration.Status != "insufficient_positive_training_cases" || calibration.MinimumPositiveTrainingCases != 10 || len(evaluation) != 0 || len(predictions) != 0 {
		t.Fatalf("calibration=%#v evaluation=%#v predictions=%#v", calibration, evaluation, predictions)
	}
}
