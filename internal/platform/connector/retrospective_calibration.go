package connector

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"
)

const RetrospectiveCalibrationSchemaVersion = "delivery-retrospective-calibration/v1"

type RetrospectiveCalibrationBuilder struct {
	Reader SnapshotReader
	Key    []byte
	Now    func() time.Time
}

type RetrospectiveCalibrationRequest struct {
	OrganizationID        string
	ProjectID             string
	AccountRef            string
	KnowledgeCutoff       time.Time
	ReplayStart           time.Time
	ReplayEnd             time.Time
	LookbackDays          int
	HorizonDays           int
	StepDays              int
	MinimumHistoryWindows int
	KeyVersion            string
}

type RetrospectiveCalibrationResult struct {
	SchemaVersion     string                              `json:"schema_version"`
	RetrospectiveOnly bool                                `json:"retrospective_only"`
	DatasetVersion    string                              `json:"dataset_version"`
	KnowledgeCutoff   time.Time                           `json:"knowledge_cutoff"`
	ReplayStart       time.Time                           `json:"replay_start"`
	ReplayEnd         time.Time                           `json:"replay_end"`
	Policy            RetrospectiveCalibrationPolicy      `json:"policy"`
	Summary           RetrospectiveCalibrationSummary     `json:"summary"`
	Calibration       []RetrospectiveCalibrationParameter `json:"calibration"`
	Evaluation        []RetrospectiveMetricEvaluation     `json:"evaluation"`
	ModelSelection    RetrospectiveModelSelection         `json:"model_selection"`
	Cases             []RetrospectiveCalibrationCase      `json:"cases"`
	Limitations       []string                            `json:"limitations"`
	ExportedAt        time.Time                           `json:"exported_at"`
}

type RetrospectiveCalibrationPolicy struct {
	LookbackDays          int       `json:"lookback_days"`
	HorizonDays           int       `json:"horizon_days"`
	StepDays              int       `json:"step_days"`
	MinimumHistoryWindows int       `json:"minimum_history_windows"`
	ConfigurationUsage    string    `json:"configuration_usage"`
	EligibleMetrics       []string  `json:"eligible_metrics"`
	ConversionUsage       string    `json:"conversion_usage"`
	SplitPolicy           string    `json:"split_policy"`
	HoldoutStart          time.Time `json:"holdout_start"`
}

type RetrospectiveCalibrationSummary struct {
	PromotionCount      int            `json:"promotion_count"`
	MetricWindowCount   int            `json:"metric_window_count"`
	CandidateFoldCount  int            `json:"candidate_fold_count"`
	ExportedCaseCount   int            `json:"exported_case_count"`
	SkippedCaseCounts   map[string]int `json:"skipped_case_counts"`
	ConversionCaseCount int            `json:"conversion_case_count"`
}

type RetrospectiveCalibrationCase struct {
	CaseID             string                      `json:"case_id"`
	AccountRef         string                      `json:"account_ref"`
	ProjectRef         string                      `json:"project_ref"`
	PromotionRef       string                      `json:"promotion_ref"`
	CookiesPlanBinding CalibrationPlanBinding      `json:"cookies_plan_binding"`
	PredictionCutoff   time.Time                   `json:"prediction_cutoff"`
	HistoryStart       time.Time                   `json:"history_start"`
	HorizonEnd         time.Time                   `json:"horizon_end"`
	HistoryWindowCount int                         `json:"history_window_count"`
	LabelWindowCount   int                         `json:"label_window_count"`
	History            RetrospectiveMetricTotals   `json:"history"`
	BaselinePrediction RetrospectiveMetricEstimate `json:"baseline_prediction"`
	Observed           RetrospectiveMetricTotals   `json:"observed"`
	EligibleMetrics    []string                    `json:"eligible_metrics"`
	ExcludedMetrics    map[string]string           `json:"excluded_metrics"`
	FeatureWindowRefs  []string                    `json:"feature_window_refs"`
	LabelWindowRefs    []string                    `json:"label_window_refs"`
	QualityStatus      string                      `json:"quality_status"`
}

type RetrospectiveMetricTotals struct {
	SpendMinor  int64  `json:"spend_minor"`
	Impressions int64  `json:"impressions"`
	Clicks      int64  `json:"clicks"`
	Conversions *int64 `json:"conversions,omitempty"`
}

type RetrospectiveMetricEstimate struct {
	SpendMinor  float64  `json:"spend_minor"`
	Impressions float64  `json:"impressions"`
	Clicks      float64  `json:"clicks"`
	Conversions *float64 `json:"conversions,omitempty"`
}

type RetrospectiveMetricEvaluation struct {
	Model             string   `json:"model"`
	DatasetSplit      string   `json:"dataset_split"`
	Metric            string   `json:"metric"`
	CaseCount         int      `json:"case_count"`
	MeanAbsoluteError float64  `json:"mean_absolute_error"`
	MeanBias          float64  `json:"mean_bias"`
	WAPE              *float64 `json:"wape,omitempty"`
	ActualTotal       float64  `json:"actual_total"`
	PredictedTotal    float64  `json:"predicted_total"`
}

type RetrospectiveCalibrationParameter struct {
	Metric            string    `json:"metric"`
	Parameter         string    `json:"parameter"`
	Value             float64   `json:"value"`
	TrainingCaseCount int       `json:"training_case_count"`
	TrainingEnd       time.Time `json:"training_end"`
}

type RetrospectiveModelSelection struct {
	SelectedModel      string   `json:"selected_model"`
	CandidateModel     string   `json:"candidate_model"`
	CandidateStatus    string   `json:"candidate_status"`
	HoldoutGatePassed  bool     `json:"holdout_gate_passed"`
	AppliedToSimulator bool     `json:"applied_to_simulator"`
	Reasons            []string `json:"reasons"`
}

func (b RetrospectiveCalibrationBuilder) Build(ctx context.Context, request RetrospectiveCalibrationRequest) (RetrospectiveCalibrationResult, error) {
	if b.Reader == nil || len(b.Key) != 32 || request.OrganizationID == "" || !strings.HasPrefix(request.AccountRef, "ref_") || request.KnowledgeCutoff.IsZero() || request.ReplayStart.IsZero() || !request.ReplayEnd.After(request.ReplayStart) || request.LookbackDays < 1 || request.HorizonDays < 1 || request.StepDays < 1 || request.MinimumHistoryWindows < 1 || strings.TrimSpace(request.KeyVersion) == "" {
		return RetrospectiveCalibrationResult{}, ErrInvalidFact
	}
	if request.ReplayEnd.After(request.KnowledgeCutoff) || request.LookbackDays > 90 || request.HorizonDays > 30 || request.StepDays > 30 {
		return RetrospectiveCalibrationResult{}, ErrInvalidFact
	}
	snapshot, err := b.Reader.Snapshot(ctx, Query{OrganizationID: request.OrganizationID, ProjectID: request.ProjectID, SourceRef: request.AccountRef, PredictionCutoff: request.KnowledgeCutoff, WindowStart: request.ReplayStart.AddDate(0, 0, -request.LookbackDays), WindowEnd: request.ReplayEnd})
	if err != nil {
		return RetrospectiveCalibrationResult{}, err
	}
	now := time.Now().UTC()
	if b.Now != nil {
		now = b.Now().UTC()
	}
	accountRef, err := CalibrationExportRef(b.Key, "account", request.AccountRef)
	if err != nil {
		return RetrospectiveCalibrationResult{}, err
	}
	result := RetrospectiveCalibrationResult{
		SchemaVersion: RetrospectiveCalibrationSchemaVersion, RetrospectiveOnly: true, DatasetVersion: snapshot.DatasetVersion,
		KnowledgeCutoff: request.KnowledgeCutoff.UTC(), ReplayStart: request.ReplayStart.UTC(), ReplayEnd: request.ReplayEnd.UTC(), ExportedAt: now,
		Policy:  RetrospectiveCalibrationPolicy{LookbackDays: request.LookbackDays, HorizonDays: request.HorizonDays, StepDays: request.StepDays, MinimumHistoryWindows: request.MinimumHistoryWindows, ConfigurationUsage: "excluded_current_snapshot_not_historical", EligibleMetrics: []string{"spend_minor", "impressions", "clicks"}, ConversionUsage: "excluded_until_attribution_maturity_is_proven", SplitPolicy: "rolling_origin_with_final_20_percent_observed_cutoff_holdout"},
		Summary: RetrospectiveCalibrationSummary{SkippedCaseCounts: map[string]int{}}, Cases: []RetrospectiveCalibrationCase{},
		Limitations: []string{"not_a_production_point_in_time_replay", "finalized_historical_metrics_were_not_available_to_cookies_at_the_replay_cutoff", "current_configuration_is_excluded", "no_causal_budget_or_bid_effect", "conversion_feedback_maturity_not_proven", "cookies_project_and_plan_are_not_created"},
	}
	promotions := latestObjects(snapshot.Objects, "promotion")
	result.Summary.PromotionCount = len(promotions)
	promotionByRef := make(map[string]ObjectSnapshot, len(promotions))
	for _, promotion := range promotions {
		promotionByRef[promotion.ObjectRef] = promotion
	}
	metricsByPromotion := latestMetrics(snapshot.Metrics, request.ReplayStart.AddDate(0, 0, -request.LookbackDays), request.ReplayEnd)
	for _, values := range metricsByPromotion {
		result.Summary.MetricWindowCount += len(values)
	}
	for cutoff := request.ReplayStart.UTC(); !cutoff.AddDate(0, 0, request.HorizonDays).After(request.ReplayEnd.UTC()); cutoff = cutoff.AddDate(0, 0, request.StepDays) {
		for promotionRef, metrics := range metricsByPromotion {
			result.Summary.CandidateFoldCount++
			promotion, ok := promotionByRef[promotionRef]
			if !ok {
				result.Summary.SkippedCaseCounts["promotion_inventory_snapshot_missing"]++
				continue
			}
			if promotion.ParentRef == "" {
				result.Summary.SkippedCaseCounts["project_relationship_missing"]++
				continue
			}
			value, reason, buildErr := b.buildRetrospectiveCase(request, accountRef, promotion, metrics, cutoff)
			if buildErr != nil {
				return RetrospectiveCalibrationResult{}, buildErr
			}
			if reason != "" {
				result.Summary.SkippedCaseCounts[reason]++
				continue
			}
			for _, metric := range value.EligibleMetrics {
				if metric == "conversions" {
					result.Summary.ConversionCaseCount++
					break
				}
			}
			result.Cases = append(result.Cases, value)
		}
	}
	sort.Slice(result.Cases, func(i, j int) bool { return result.Cases[i].CaseID < result.Cases[j].CaseID })
	result.Summary.ExportedCaseCount = len(result.Cases)
	holdoutStart := retrospectiveHoldoutStart(result.Cases, request.ReplayEnd.UTC())
	result.Policy.HoldoutStart = holdoutStart
	result.Calibration, result.Evaluation = calibrateAndEvaluateRetrospectiveCases(result.Cases, holdoutStart)
	result.ModelSelection = selectRetrospectiveModel(result.Evaluation)
	return result, nil
}

func (b RetrospectiveCalibrationBuilder) buildRetrospectiveCase(request RetrospectiveCalibrationRequest, accountRef string, promotion ObjectSnapshot, metrics []MetricWindow, cutoff time.Time) (RetrospectiveCalibrationCase, string, error) {
	historyStart := cutoff.AddDate(0, 0, -request.LookbackDays)
	horizonEnd := cutoff.AddDate(0, 0, request.HorizonDays)
	history, labels := make([]MetricWindow, 0), make([]MetricWindow, 0)
	for _, metric := range metrics {
		if !isRetrospectiveAtomicWindow(metric) {
			continue
		}
		if !metric.WindowStart.Before(historyStart) && !metric.WindowEnd.After(cutoff) {
			history = append(history, metric)
		}
		if !metric.WindowStart.Before(cutoff) && !metric.WindowEnd.After(horizonEnd) {
			labels = append(labels, metric)
		}
	}
	if len(history) < request.MinimumHistoryWindows {
		return RetrospectiveCalibrationCase{}, "history_windows_insufficient", nil
	}
	if len(labels) == 0 {
		return RetrospectiveCalibrationCase{}, "label_windows_missing", nil
	}
	projectRef, err := CalibrationExportRef(b.Key, "project", promotion.ParentRef)
	if err != nil {
		return RetrospectiveCalibrationCase{}, "", err
	}
	promotionRef, err := CalibrationExportRef(b.Key, "promotion", promotion.ObjectRef)
	if err != nil {
		return RetrospectiveCalibrationCase{}, "", err
	}
	historyTotals, featureRefs, historyConversionEligible, err := retrospectiveTotals(b.Key, history)
	if err != nil {
		return RetrospectiveCalibrationCase{}, "", err
	}
	observed, labelRefs, labelConversionEligible, err := retrospectiveTotals(b.Key, labels)
	if err != nil {
		return RetrospectiveCalibrationCase{}, "", err
	}
	scale := float64(request.HorizonDays) / float64(len(history))
	prediction := RetrospectiveMetricEstimate{SpendMinor: float64(historyTotals.SpendMinor) * scale, Impressions: float64(historyTotals.Impressions) * scale, Clicks: float64(historyTotals.Clicks) * scale}
	excluded := map[string]string{"conversions": "attribution_maturity_not_proven"}
	eligible := []string{"spend_minor", "impressions", "clicks"}
	if historyConversionEligible && labelConversionEligible {
		value := float64(*historyTotals.Conversions) * scale
		prediction.Conversions = &value
		eligible = append(eligible, "conversions")
		delete(excluded, "conversions")
	}
	caseID := "retrocase_" + canonicalHash([]any{accountRef, projectRef, promotionRef, cutoff, request.LookbackDays, request.HorizonDays, featureRefs, labelRefs})
	return RetrospectiveCalibrationCase{CaseID: caseID, AccountRef: accountRef, ProjectRef: projectRef, PromotionRef: promotionRef, CookiesPlanBinding: CalibrationPlanBinding{State: "unbound_historical", PlanID: nil, PlanVersion: nil}, PredictionCutoff: cutoff, HistoryStart: historyStart, HorizonEnd: horizonEnd, HistoryWindowCount: len(history), LabelWindowCount: len(labels), History: historyTotals, BaselinePrediction: prediction, Observed: observed, EligibleMetrics: eligible, ExcludedMetrics: excluded, FeatureWindowRefs: featureRefs, LabelWindowRefs: labelRefs, QualityStatus: "retrospective_baseline_only"}, "", nil
}

func isRetrospectiveAtomicWindow(metric MetricWindow) bool {
	if !metric.WindowEnd.After(metric.WindowStart) || metric.Granularity != "day" || metric.QualityStatus == QualityReject || metric.Currency == "" || metric.AmountUnit == "" || metric.MetricDefinitionVersion == "" {
		return false
	}
	for _, issue := range metric.QualityIssues {
		if issue.Disposition == QualityReject || issue.Disposition == QualityQuarantine && issue.Code != "attribution_immature" {
			return false
		}
	}
	_, complete := atomicMetrics(metric.Metrics)
	return complete
}

func retrospectiveTotals(key []byte, values []MetricWindow) (RetrospectiveMetricTotals, []string, bool, error) {
	result := RetrospectiveMetricTotals{}
	refs := make([]string, 0, len(values))
	conversionEligible := true
	conversionTotal := int64(0)
	for _, value := range values {
		atomic, complete := atomicMetrics(value.Metrics)
		if !complete {
			return RetrospectiveMetricTotals{}, nil, false, ErrInvalidFact
		}
		result.SpendMinor += atomic["spend"]
		result.Impressions += atomic["impressions"]
		result.Clicks += atomic["clicks"]
		conversionTotal += atomic["conversions"]
		if value.QualityStatus != QualityAccept || hasQualityIssue(value.QualityIssues, "attribution_immature") {
			conversionEligible = false
		}
		ref, err := CalibrationExportRef(key, "metric_window", AnonymizeRef(value.ID))
		if err != nil {
			return RetrospectiveMetricTotals{}, nil, false, err
		}
		refs = append(refs, ref)
	}
	if conversionEligible {
		result.Conversions = &conversionTotal
	}
	return result, uniqueSorted(refs), conversionEligible, nil
}

func retrospectiveHoldoutStart(cases []RetrospectiveCalibrationCase, fallback time.Time) time.Time {
	unique := map[time.Time]struct{}{}
	for _, value := range cases {
		unique[value.PredictionCutoff.UTC()] = struct{}{}
	}
	cutoffs := make([]time.Time, 0, len(unique))
	for value := range unique {
		cutoffs = append(cutoffs, value)
	}
	sort.Slice(cutoffs, func(i, j int) bool { return cutoffs[i].Before(cutoffs[j]) })
	if len(cutoffs) < 2 {
		return fallback
	}
	index := int(math.Floor(float64(len(cutoffs)) * 0.8))
	if index >= len(cutoffs) {
		index = len(cutoffs) - 1
	}
	if index < 1 {
		index = 1
	}
	return cutoffs[index]
}

func calibrateAndEvaluateRetrospectiveCases(cases []RetrospectiveCalibrationCase, holdoutStart time.Time) ([]RetrospectiveCalibrationParameter, []RetrospectiveMetricEvaluation) {
	training, holdout := make([]RetrospectiveCalibrationCase, 0), make([]RetrospectiveCalibrationCase, 0)
	for _, value := range cases {
		if value.PredictionCutoff.Before(holdoutStart) {
			training = append(training, value)
		} else {
			holdout = append(holdout, value)
		}
	}
	if len(holdout) == 0 {
		holdout = cases
	}
	scales := map[string]float64{}
	parameters := make([]RetrospectiveCalibrationParameter, 0, 3)
	for _, metric := range []string{"spend_minor", "impressions", "clicks"} {
		actual, predicted, count := retrospectiveTotalsForEvaluation(training, metric)
		if count == 0 || predicted <= 0 {
			continue
		}
		scale := math.Max(0.25, math.Min(4, actual/predicted))
		scales[metric] = scale
		parameters = append(parameters, RetrospectiveCalibrationParameter{Metric: metric, Parameter: "multiplicative_scale", Value: scale, TrainingCaseCount: count, TrainingEnd: holdoutStart})
	}
	evaluation := evaluateRetrospectiveCases(holdout, "trailing_mean_baseline", "time_holdout", nil)
	if len(scales) > 0 {
		evaluation = append(evaluation, evaluateRetrospectiveCases(holdout, "calibrated_scale_v1", "time_holdout", scales)...)
	}
	return parameters, evaluation
}

func retrospectiveTotalsForEvaluation(cases []RetrospectiveCalibrationCase, metric string) (float64, float64, int) {
	actualTotal, predictedTotal, count := 0.0, 0.0, 0
	for _, value := range cases {
		actual, predicted, ok := retrospectiveMetricPair(value, metric)
		if !ok {
			continue
		}
		actualTotal += actual
		predictedTotal += predicted
		count++
	}
	return actualTotal, predictedTotal, count
}

func retrospectiveMetricPair(value RetrospectiveCalibrationCase, metric string) (float64, float64, bool) {
	switch metric {
	case "spend_minor":
		return float64(value.Observed.SpendMinor), value.BaselinePrediction.SpendMinor, true
	case "impressions":
		return float64(value.Observed.Impressions), value.BaselinePrediction.Impressions, true
	case "clicks":
		return float64(value.Observed.Clicks), value.BaselinePrediction.Clicks, true
	case "conversions":
		if value.Observed.Conversions != nil && value.BaselinePrediction.Conversions != nil {
			return float64(*value.Observed.Conversions), *value.BaselinePrediction.Conversions, true
		}
	}
	return 0, 0, false
}

func evaluateRetrospectiveCases(cases []RetrospectiveCalibrationCase, model, split string, scales map[string]float64) []RetrospectiveMetricEvaluation {
	type accumulator struct {
		count                             int
		absolute, bias, actual, predicted float64
	}
	values := map[string]*accumulator{"spend_minor": {}, "impressions": {}, "clicks": {}, "conversions": {}}
	for _, value := range cases {
		for _, metric := range []string{"spend_minor", "impressions", "clicks", "conversions"} {
			actual, prediction, ok := retrospectiveMetricPair(value, metric)
			if !ok {
				continue
			}
			if scale, exists := scales[metric]; exists {
				prediction *= scale
			}
			entry := values[metric]
			entry.count++
			entry.absolute += math.Abs(prediction - actual)
			entry.bias += prediction - actual
			entry.actual += actual
			entry.predicted += prediction
		}
	}
	result := make([]RetrospectiveMetricEvaluation, 0, len(values))
	for _, metric := range []string{"spend_minor", "impressions", "clicks", "conversions"} {
		value := values[metric]
		if value.count == 0 {
			continue
		}
		var wape *float64
		if value.actual > 0 {
			calculated := value.absolute / value.actual
			wape = &calculated
		}
		result = append(result, RetrospectiveMetricEvaluation{Model: model, DatasetSplit: split, Metric: metric, CaseCount: value.count, MeanAbsoluteError: value.absolute / float64(value.count), MeanBias: value.bias / float64(value.count), WAPE: wape, ActualTotal: value.actual, PredictedTotal: value.predicted})
	}
	return result
}

func selectRetrospectiveModel(evaluations []RetrospectiveMetricEvaluation) RetrospectiveModelSelection {
	result := RetrospectiveModelSelection{SelectedModel: "trailing_mean_baseline", CandidateModel: "calibrated_scale_v1", CandidateStatus: "rejected", AppliedToSimulator: false, Reasons: []string{}}
	baseline, candidate := map[string]RetrospectiveMetricEvaluation{}, map[string]RetrospectiveMetricEvaluation{}
	for _, value := range evaluations {
		switch value.Model {
		case "trailing_mean_baseline":
			baseline[value.Metric] = value
		case "calibrated_scale_v1":
			candidate[value.Metric] = value
		}
	}
	if len(candidate) == 0 {
		result.CandidateStatus = "not_evaluated"
		result.Reasons = []string{"training_or_holdout_cases_insufficient"}
		return result
	}
	for _, metric := range []string{"spend_minor", "impressions", "clicks"} {
		base, baseOK := baseline[metric]
		challenger, challengerOK := candidate[metric]
		if !baseOK || !challengerOK || base.WAPE == nil || challenger.WAPE == nil {
			result.Reasons = append(result.Reasons, metric+":wape_unavailable")
			continue
		}
		if *challenger.WAPE >= *base.WAPE {
			result.Reasons = append(result.Reasons, metric+":holdout_wape_not_improved")
		}
	}
	if len(result.Reasons) == 0 {
		result.SelectedModel = result.CandidateModel
		result.CandidateStatus = "passed_retrospective_holdout"
		result.HoldoutGatePassed = true
		result.Reasons = []string{"all_eligible_metric_wape_values_improved"}
	}
	return result
}
