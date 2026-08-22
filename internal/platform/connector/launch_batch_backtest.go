package connector

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	LaunchBatchBacktestSchemaVersion = "connector-oceanengine-launch-batch-backtest/v1"
	LaunchBatchModelVersion          = "account-product-launch-batch-v1"
)

type LaunchBatchBacktestRequest struct {
	ExternalAccount string
	Core            OfflineXLSXSource
	Supplement      OfflineXLSXSource
	Inventory       CanonicalSnapshot
	TimeZone        string
}

type LaunchBatchBacktestResult struct {
	SchemaVersion           string                         `json:"schema_version"`
	ModelVersion            string                         `json:"model_version"`
	Status                  string                         `json:"status"`
	BehaviorMode            string                         `json:"behavior_mode"`
	EvaluationMode          string                         `json:"evaluation_mode"`
	Audit                   LaunchBatchAudit               `json:"audit"`
	Split                   LaunchBatchSplit               `json:"split"`
	Parameters              []LaunchBatchModelParameter    `json:"parameters"`
	Evaluation              []LaunchBatchMetricEvaluation  `json:"evaluation"`
	FinalHoldoutEvaluation  []LaunchBatchMetricEvaluation  `json:"final_holdout_evaluation"`
	Scenario                LaunchBatchScenarioCalibration `json:"scenario"`
	ScenarioEvaluation      LaunchBatchScenarioEvaluation  `json:"scenario_evaluation"`
	FinalScenarioEvaluation LaunchBatchScenarioEvaluation  `json:"final_scenario_evaluation"`
	Holdout                 []LaunchBatchHoldoutPrediction `json:"holdout"`
	Diagnostics             []LaunchBatchCaseDiagnostic    `json:"diagnostics"`
	Limitations             []string                       `json:"limitations"`
}

type LaunchBatchAudit struct {
	CoreSourceRowCount              int            `json:"core_source_row_count"`
	CoreGroupedRowCount             int            `json:"core_grouped_row_count"`
	CoreDuplicateKeyCount           int            `json:"core_duplicate_key_count"`
	SupplementSourceRowCount        int            `json:"supplement_source_row_count"`
	SupplementGroupedRowCount       int            `json:"supplement_grouped_row_count"`
	SupplementDuplicateKeyCount     int            `json:"supplement_duplicate_key_count"`
	ProjectCount                    int            `json:"project_count"`
	PromotionCount                  int            `json:"promotion_count"`
	InventoryMatchedPromotionCount  int            `json:"inventory_matched_promotion_count"`
	CreateTimeMatchedPromotionCount int            `json:"create_time_matched_promotion_count"`
	ParentMismatchPromotionCount    int            `json:"parent_mismatch_promotion_count"`
	EligibleBatchCount              int            `json:"eligible_batch_count"`
	ExcludedBatchCounts             map[string]int `json:"excluded_batch_counts"`
	ConfigurationDistinctCounts     map[string]int `json:"configuration_distinct_counts"`
}

type LaunchBatchSplit struct {
	HoldoutStart      time.Time `json:"holdout_start"`
	TrainingBatches   int       `json:"training_batches"`
	HoldoutBatches    int       `json:"holdout_batches"`
	TrainingDates     int       `json:"training_dates"`
	HoldoutDates      int       `json:"holdout_dates"`
	TrainingUnits     int       `json:"training_units"`
	HoldoutUnits      int       `json:"holdout_units"`
	EvaluationBatches int       `json:"evaluation_batches"`
	EvaluationDates   int       `json:"evaluation_dates"`
}

type LaunchBatchModelParameter struct {
	Metric                    string  `json:"metric"`
	Estimator                 string  `json:"estimator"`
	InnerTrainingBatchCount   int     `json:"inner_training_batch_count"`
	InnerValidationBatchCount int     `json:"inner_validation_batch_count"`
	InnerValidationWAPE       float64 `json:"inner_validation_wape"`
	Central80Lower            float64 `json:"central_80_lower"`
	Central80Upper            float64 `json:"central_80_upper"`
	IntervalScale             string  `json:"interval_scale"`
}

type LaunchBatchMetricEvaluation struct {
	Metric                    string  `json:"metric"`
	CaseCount                 int     `json:"case_count"`
	ActualTotal               float64 `json:"actual_total"`
	PredictedTotal            float64 `json:"predicted_total"`
	WAPE                      float64 `json:"wape"`
	MAE                       float64 `json:"mae"`
	Central80Coverage         float64 `json:"central_80_coverage"`
	MeanIntervalWidth         float64 `json:"mean_interval_width"`
	IntervalWidthToActualMean float64 `json:"interval_width_to_actual_mean"`
}

type LaunchBatchHoldoutPrediction struct {
	BatchRef            string                               `json:"batch_ref"`
	LaunchDay           time.Time                            `json:"launch_day"`
	UnitCount           int                                  `json:"unit_count"`
	Metrics             map[string]LaunchBatchMetricForecast `json:"metrics"`
	BreakoutProbability float64                              `json:"breakout_probability"`
	BreakoutObserved    bool                                 `json:"breakout_observed"`
}

type LaunchBatchScenarioCalibration struct {
	ThresholdSpendMinor float64                                 `json:"threshold_spend_minor"`
	TrainingBatchCount  int                                     `json:"training_batch_count"`
	TypicalBatchCount   int                                     `json:"typical_batch_count"`
	BreakoutBatchCount  int                                     `json:"breakout_batch_count"`
	BreakoutProbability float64                                 `json:"breakout_probability"`
	Typical             []LaunchBatchScenarioMetricDistribution `json:"typical"`
	Breakout            []LaunchBatchScenarioMetricDistribution `json:"breakout"`
}

type LaunchBatchScenarioMetricDistribution struct {
	Metric string  `json:"metric"`
	P10    float64 `json:"p10"`
	P50    float64 `json:"p50"`
	P90    float64 `json:"p90"`
}

type LaunchBatchScenarioEvaluation struct {
	CaseCount                int     `json:"case_count"`
	ObservedBreakoutCount    int     `json:"observed_breakout_count"`
	ObservedBreakoutRate     float64 `json:"observed_breakout_rate"`
	PredictedBreakoutRate    float64 `json:"predicted_breakout_rate"`
	BrierScore               float64 `json:"brier_score"`
	AbsoluteCalibrationError float64 `json:"absolute_calibration_error"`
}

type LaunchBatchCaseDiagnostic struct {
	BatchRef      string    `json:"batch_ref"`
	LaunchDay     time.Time `json:"launch_day"`
	UnitCount     int       `json:"unit_count"`
	ActiveUnits   int       `json:"active_units"`
	SpendMinor    float64   `json:"spend_minor"`
	Impressions   float64   `json:"impressions"`
	Clicks        float64   `json:"clicks"`
	TopSpendShare float64   `json:"top_spend_share"`
	AverageBid    float64   `json:"average_bid"`
}

type LaunchBatchMetricForecast struct {
	Estimate float64 `json:"estimate"`
	Lower    float64 `json:"lower"`
	Upper    float64 `json:"upper"`
	Observed float64 `json:"observed"`
}

type launchBatchRowKey struct {
	Day       string
	Project   string
	Promotion string
}

type launchBatchParsed struct {
	Rows          map[launchBatchRowKey]offlineAtomicMetrics
	SourceRows    int
	DuplicateKeys int
	ConfigValues  map[string]map[string]struct{}
	PromotionBid  map[string]float64
}

type launchBatchCase struct {
	BatchRef      string
	LaunchDay     time.Time
	UnitCount     int
	ActiveUnits   int
	Spend         float64
	Impressions   float64
	Clicks        float64
	TopSpendShare float64
	AverageBid    float64
}

type launchBatchEstimator struct {
	Name       string
	Metric     string
	PerUnit    bool
	Value      float64
	Slope      float64
	Origin     time.Time
	Cap        float64
	RecentSize int
}

func BuildLaunchBatchBacktest(request LaunchBatchBacktestRequest) (LaunchBatchBacktestResult, error) {
	request.ExternalAccount = strings.TrimSpace(request.ExternalAccount)
	if request.TimeZone == "" {
		request.TimeZone = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(request.TimeZone)
	if err != nil {
		if request.TimeZone != "Asia/Shanghai" {
			return LaunchBatchBacktestResult{}, fmt.Errorf("%w: unsupported launch batch time zone", ErrInvalidFact)
		}
		location = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	if !validOfflinePlatformID(request.ExternalAccount) || offlineSourceAccount(request.Core.Name) != request.ExternalAccount || offlineSourceAccount(request.Supplement.Name) != request.ExternalAccount {
		return LaunchBatchBacktestResult{}, fmt.Errorf("%w: launch batch sources do not match the account", ErrInvalidFact)
	}
	coreRows, err := readOfflineXLSX(request.Core.Content)
	if err != nil {
		return LaunchBatchBacktestResult{}, err
	}
	supplementRows, err := readOfflineXLSX(request.Supplement.Content)
	if err != nil {
		return LaunchBatchBacktestResult{}, err
	}
	core, err := parseLaunchBatchRows(coreRows, location, []string{"转化目标", "计费类型", "深度转化目标", "投放模式", "单元出价", "深度转化出价", "ROI系数"})
	if err != nil {
		return LaunchBatchBacktestResult{}, fmt.Errorf("core report: %w", err)
	}
	supplement, err := parseLaunchBatchRows(supplementRows, location, []string{"营销目的", "项目类型", "是否搜索蓝海流量项目", "营销场景", "关键行为名称", "线索多载体优选", "是否原生营销"})
	if err != nil {
		return LaunchBatchBacktestResult{}, fmt.Errorf("supplement report: %w", err)
	}
	if err = reconcileLaunchBatchReports(core.Rows, supplement.Rows); err != nil {
		return LaunchBatchBacktestResult{}, err
	}
	result := LaunchBatchBacktestResult{
		SchemaVersion:  LaunchBatchBacktestSchemaVersion,
		ModelVersion:   LaunchBatchModelVersion,
		Status:         "insufficient_batches",
		BehaviorMode:   "fixed_account_product_prior",
		EvaluationMode: "rolling_origin_with_untouched_final_holdout",
		Audit: LaunchBatchAudit{
			CoreSourceRowCount: core.SourceRows, CoreGroupedRowCount: len(core.Rows), CoreDuplicateKeyCount: core.DuplicateKeys,
			SupplementSourceRowCount: supplement.SourceRows, SupplementGroupedRowCount: len(supplement.Rows), SupplementDuplicateKeyCount: supplement.DuplicateKeys,
			ExcludedBatchCounts: map[string]int{}, ConfigurationDistinctCounts: mergeLaunchBatchDistinctCounts(core.ConfigValues, supplement.ConfigValues),
		},
		Parameters: []LaunchBatchModelParameter{}, Evaluation: []LaunchBatchMetricEvaluation{}, FinalHoldoutEvaluation: []LaunchBatchMetricEvaluation{}, Holdout: []LaunchBatchHoldoutPrediction{}, Diagnostics: []LaunchBatchCaseDiagnostic{},
		Limitations: []string{"retrospective_offline_metrics", "fixed_account_product_behavior", "no_causal_budget_or_bid_effect", "unit_winner_identity_not_predictable_before_platform_learning", "cookies_project_and_plan_are_not_created"},
	}
	cases := buildLaunchBatchCases(core.Rows, core.PromotionBid, request.Inventory, location, &result.Audit)
	for _, value := range cases {
		result.Diagnostics = append(result.Diagnostics, LaunchBatchCaseDiagnostic{BatchRef: value.BatchRef, LaunchDay: value.LaunchDay, UnitCount: value.UnitCount, ActiveUnits: value.ActiveUnits, SpendMinor: value.Spend, Impressions: value.Impressions, Clicks: value.Clicks, TopSpendShare: value.TopSpendShare, AverageBid: value.AverageBid})
	}
	if len(cases) < 10 {
		return result, nil
	}
	holdoutStart := launchBatchHoldoutStart(cases)
	training, holdout := splitLaunchBatchCases(cases, holdoutStart)
	result.Split = launchBatchSplit(training, holdout, holdoutStart)
	if len(training) < 20 || len(holdout) < 5 {
		return result, nil
	}
	innerStart := launchBatchHoldoutStart(training)
	fit, validation := splitLaunchBatchCases(training, innerStart)
	if len(fit) < 12 || len(validation) < 4 {
		return result, nil
	}
	metrics := []string{"spend_minor", "impressions", "clicks", "active_units", "top_spend_share"}
	models := map[string]launchBatchEstimator{}
	intervals := map[string][2]float64{}
	intervalScales := map[string]string{}
	for _, metric := range metrics {
		model, parameter := selectLaunchBatchEstimator(fit, validation, metric)
		parameter.InnerTrainingBatchCount = len(fit)
		parameter.InnerValidationBatchCount = len(validation)
		models[metric] = fitLaunchBatchEstimator(training, metric, model.Name)
		lower, upper, scale := launchBatchEmpiricalInterval(training, metric)
		parameter.Central80Lower, parameter.Central80Upper, parameter.IntervalScale = lower, upper, scale
		intervals[metric], intervalScales[metric] = [2]float64{lower, upper}, scale
		result.Parameters = append(result.Parameters, parameter)
	}
	result.Scenario = calibrateLaunchBatchScenario(training)
	for _, value := range holdout {
		prediction := LaunchBatchHoldoutPrediction{BatchRef: value.BatchRef, LaunchDay: value.LaunchDay, UnitCount: value.UnitCount, Metrics: map[string]LaunchBatchMetricForecast{}, BreakoutProbability: result.Scenario.BreakoutProbability, BreakoutObserved: value.Spend > result.Scenario.ThresholdSpendMinor}
		for _, metric := range metrics {
			estimate := predictLaunchBatch(models[metric], value)
			observed := launchBatchMetric(value, metric)
			bounds := intervals[metric]
			lower, upper := bounds[0], bounds[1]
			if intervalScales[metric] == "per_unit" {
				lower *= float64(value.UnitCount)
				upper *= float64(value.UnitCount)
			}
			if metric == "top_spend_share" {
				lower, upper = clampProbability(lower), clampProbability(upper)
			}
			prediction.Metrics[metric] = LaunchBatchMetricForecast{Estimate: estimate, Lower: lower, Upper: upper, Observed: observed}
		}
		result.Holdout = append(result.Holdout, prediction)
	}
	for _, metric := range metrics {
		result.FinalHoldoutEvaluation = append(result.FinalHoldoutEvaluation, evaluateLaunchBatchMetric(result.Holdout, metric))
	}
	result.FinalScenarioEvaluation = evaluateLaunchBatchScenario(result.Holdout)
	rolling, evaluationDates := walkForwardLaunchBatchPredictions(cases, metrics)
	result.Split.EvaluationBatches, result.Split.EvaluationDates = len(rolling), evaluationDates
	for _, metric := range metrics {
		result.Evaluation = append(result.Evaluation, evaluateLaunchBatchMetric(rolling, metric))
	}
	result.ScenarioEvaluation = evaluateLaunchBatchScenario(rolling)
	result.Status = launchBatchReadiness(result.Evaluation, result.Split, result.Scenario, result.ScenarioEvaluation)
	return result, nil
}

func offlineSourceAccount(name string) string {
	base := strings.TrimSuffix(filepath.Base(strings.TrimSpace(name)), filepath.Ext(name))
	parts := strings.Split(base, "_")
	if len(parts) < 3 || !validOfflinePlatformID(parts[1]) {
		return ""
	}
	return parts[1]
}

func parseLaunchBatchRows(rows [][]string, location *time.Location, configColumns []string) (launchBatchParsed, error) {
	if len(rows) < 2 {
		return launchBatchParsed{}, ErrInvalidFact
	}
	header, err := offlineHeaderMap(rows[0])
	if err != nil {
		return launchBatchParsed{}, err
	}
	for _, required := range append([]string{"时间-天", "项目ID", "单元ID", "消耗", "展示数", "点击数", "转化数"}, configColumns...) {
		if _, ok := header[required]; !ok {
			return launchBatchParsed{}, fmt.Errorf("%w: required column %s is missing", ErrInvalidFact, required)
		}
	}
	result := launchBatchParsed{Rows: map[launchBatchRowKey]offlineAtomicMetrics{}, SourceRows: len(rows) - 1, ConfigValues: map[string]map[string]struct{}{}, PromotionBid: map[string]float64{}}
	seen := map[launchBatchRowKey]int{}
	for index, row := range rows[1:] {
		day, parseErr := time.ParseInLocation("2006-01-02", offlineCell(row, header["时间-天"]), location)
		project, promotion := offlineCell(row, header["项目ID"]), offlineCell(row, header["单元ID"])
		if parseErr != nil || !validOfflinePlatformID(project) || !validOfflinePlatformID(promotion) {
			return launchBatchParsed{}, fmt.Errorf("%w: invalid key at source row %d", ErrInvalidFact, index+2)
		}
		metrics, metricErr := parseLaunchBatchMetrics(row, header)
		if metricErr != nil {
			return launchBatchParsed{}, fmt.Errorf("%w: invalid atomic metric at source row %d", metricErr, index+2)
		}
		key := launchBatchRowKey{Day: day.Format("2006-01-02"), Project: project, Promotion: promotion}
		if seen[key] > 0 {
			result.DuplicateKeys++
		}
		seen[key]++
		current := result.Rows[key]
		current.Spend += metrics.Spend
		current.Impressions += metrics.Impressions
		current.Clicks += metrics.Clicks
		current.Conversions += metrics.Conversions
		result.Rows[key] = current
		if bidColumn, ok := header["单元出价"]; ok {
			bidText := strings.ReplaceAll(offlineCell(row, bidColumn), ",", "")
			bid, bidErr := strconv.ParseFloat(bidText, 64)
			if bidErr != nil || bid < 0 {
				return launchBatchParsed{}, fmt.Errorf("%w: invalid bid at source row %d", ErrInvalidFact, index+2)
			}
			if previous, exists := result.PromotionBid[promotion]; exists && previous != bid {
				return launchBatchParsed{}, fmt.Errorf("%w: promotion bid changes inside the export", ErrInvalidFact)
			}
			result.PromotionBid[promotion] = bid
		}
		for _, column := range configColumns {
			value := offlineCell(row, header[column])
			if value == "" {
				continue
			}
			if result.ConfigValues[column] == nil {
				result.ConfigValues[column] = map[string]struct{}{}
			}
			result.ConfigValues[column][value] = struct{}{}
		}
	}
	return result, nil
}

func parseLaunchBatchMetrics(row []string, header map[string]int) (offlineAtomicMetrics, error) {
	spend, err := parseOfflineMinor(offlineCell(row, header["消耗"]))
	if err != nil {
		return offlineAtomicMetrics{}, err
	}
	impressions, err := parseOfflineCount(offlineCell(row, header["展示数"]))
	if err != nil {
		return offlineAtomicMetrics{}, err
	}
	clicks, err := parseOfflineCount(offlineCell(row, header["点击数"]))
	if err != nil {
		return offlineAtomicMetrics{}, err
	}
	conversions, err := parseOfflineCount(offlineCell(row, header["转化数"]))
	if err != nil {
		return offlineAtomicMetrics{}, err
	}
	return offlineAtomicMetrics{Spend: spend, Impressions: impressions, Clicks: clicks, Conversions: conversions}, nil
}

func reconcileLaunchBatchReports(core, supplement map[launchBatchRowKey]offlineAtomicMetrics) error {
	if len(core) != len(supplement) {
		return fmt.Errorf("%w: custom report key coverage differs", ErrInvalidFact)
	}
	for key, expected := range core {
		actual, ok := supplement[key]
		if !ok || actual != expected {
			return fmt.Errorf("%w: custom report atomic totals differ", ErrInvalidFact)
		}
	}
	return nil
}

func mergeLaunchBatchDistinctCounts(values ...map[string]map[string]struct{}) map[string]int {
	result := map[string]int{}
	for _, group := range values {
		for name, distinct := range group {
			result[name] = len(distinct)
		}
	}
	return result
}

func buildLaunchBatchCases(rows map[launchBatchRowKey]offlineAtomicMetrics, promotionBids map[string]float64, inventory CanonicalSnapshot, location *time.Location, audit *LaunchBatchAudit) []launchBatchCase {
	projects, promotions := map[string]map[string]struct{}{}, map[string]string{}
	var firstReportDay, lastReportDay time.Time
	for key := range rows {
		day, _ := time.ParseInLocation("2006-01-02", key.Day, location)
		if firstReportDay.IsZero() || day.Before(firstReportDay) {
			firstReportDay = day
		}
		if lastReportDay.IsZero() || day.After(lastReportDay) {
			lastReportDay = day
		}
		if projects[key.Project] == nil {
			projects[key.Project] = map[string]struct{}{}
		}
		projects[key.Project][key.Promotion] = struct{}{}
		promotions[key.Promotion] = key.Project
	}
	audit.ProjectCount, audit.PromotionCount = len(projects), len(promotions)
	objects := map[string]ObjectSnapshot{}
	for _, object := range latestObjects(inventory.Objects, "promotion") {
		objects[object.ObjectRef] = object
	}
	createTimes := map[string]time.Time{}
	for promotion, project := range promotions {
		object, ok := objects[opaqueRef(promotion)]
		if !ok {
			continue
		}
		audit.InventoryMatchedPromotionCount++
		if object.ParentRef != "" && object.ParentRef != opaqueRef(project) {
			audit.ParentMismatchPromotionCount++
			continue
		}
		created, ok := calibrationObjectCreateTime(object.State)
		if !ok {
			continue
		}
		audit.CreateTimeMatchedPromotionCount++
		local := created.In(location)
		createTimes[promotion] = time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	}
	result := make([]launchBatchCase, 0, len(projects))
	for project, units := range projects {
		launchDay := time.Time{}
		complete := true
		for unit := range units {
			created, ok := createTimes[unit]
			if !ok {
				complete = false
				break
			}
			if launchDay.IsZero() || created.Before(launchDay) {
				launchDay = created
			}
		}
		if !complete {
			audit.ExcludedBatchCounts["create_time_incomplete"]++
			continue
		}
		if launchDay.Before(firstReportDay) {
			audit.ExcludedBatchCounts["left_censored"]++
			continue
		}
		horizonEnd := launchDay.AddDate(0, 0, 7)
		if lastReportDay.AddDate(0, 0, 1).Before(horizonEnd) {
			audit.ExcludedBatchCounts["right_censored"]++
			continue
		}
		eligibleUnits := map[string]struct{}{}
		unitSpend := map[string]int64{}
		value := launchBatchCase{BatchRef: "batch_" + canonicalHash([]string{opaqueRef(project), launchDay.Format("2006-01-02")}), LaunchDay: launchDay}
		for unit := range units {
			if createTimes[unit].Before(horizonEnd) {
				eligibleUnits[unit] = struct{}{}
			}
		}
		for key, metric := range rows {
			if key.Project != project {
				continue
			}
			if _, ok := eligibleUnits[key.Promotion]; !ok {
				continue
			}
			day, _ := time.ParseInLocation("2006-01-02", key.Day, location)
			if day.Before(launchDay) || !day.Before(horizonEnd) {
				continue
			}
			value.Spend += float64(metric.Spend)
			value.Impressions += float64(metric.Impressions)
			value.Clicks += float64(metric.Clicks)
			unitSpend[key.Promotion] += metric.Spend
		}
		value.UnitCount = len(eligibleUnits)
		maximumSpend := int64(0)
		for unit := range eligibleUnits {
			value.AverageBid += promotionBids[unit]
			if unitSpend[unit] > 0 {
				value.ActiveUnits++
			}
			if unitSpend[unit] > maximumSpend {
				maximumSpend = unitSpend[unit]
			}
		}
		value.AverageBid /= float64(value.UnitCount)
		if value.Spend > 0 {
			value.TopSpendShare = float64(maximumSpend) / value.Spend
		}
		if value.UnitCount == 0 {
			audit.ExcludedBatchCounts["empty_launch_batch"]++
			continue
		}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].LaunchDay.Equal(result[j].LaunchDay) {
			return result[i].BatchRef < result[j].BatchRef
		}
		return result[i].LaunchDay.Before(result[j].LaunchDay)
	})
	audit.EligibleBatchCount = len(result)
	return result
}

func launchBatchHoldoutStart(values []launchBatchCase) time.Time {
	dates := map[time.Time]struct{}{}
	for _, value := range values {
		dates[value.LaunchDay] = struct{}{}
	}
	ordered := make([]time.Time, 0, len(dates))
	for value := range dates {
		ordered = append(ordered, value)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Before(ordered[j]) })
	if len(ordered) < 2 {
		return time.Time{}
	}
	index := int(math.Floor(float64(len(ordered)) * 0.8))
	if index >= len(ordered) {
		index = len(ordered) - 1
	}
	if index < 1 {
		index = 1
	}
	return ordered[index]
}

func splitLaunchBatchCases(values []launchBatchCase, cutoff time.Time) ([]launchBatchCase, []launchBatchCase) {
	training, holdout := []launchBatchCase{}, []launchBatchCase{}
	for _, value := range values {
		if value.LaunchDay.Before(cutoff) {
			training = append(training, value)
		} else {
			holdout = append(holdout, value)
		}
	}
	return training, holdout
}

func launchBatchSplit(training, holdout []launchBatchCase, cutoff time.Time) LaunchBatchSplit {
	result := LaunchBatchSplit{HoldoutStart: cutoff, TrainingBatches: len(training), HoldoutBatches: len(holdout)}
	trainingDates, holdoutDates := map[time.Time]struct{}{}, map[time.Time]struct{}{}
	for _, value := range training {
		trainingDates[value.LaunchDay] = struct{}{}
		result.TrainingUnits += value.UnitCount
	}
	for _, value := range holdout {
		holdoutDates[value.LaunchDay] = struct{}{}
		result.HoldoutUnits += value.UnitCount
	}
	result.TrainingDates, result.HoldoutDates = len(trainingDates), len(holdoutDates)
	return result
}

func selectLaunchBatchEstimator(fit, validation []launchBatchCase, metric string) (launchBatchEstimator, LaunchBatchModelParameter) {
	best := launchBatchEstimator{Metric: metric}
	parameter := LaunchBatchModelParameter{Metric: metric, InnerValidationWAPE: math.Inf(1)}
	for _, name := range []string{
		"mean_per_unit", "median_per_unit", "mean_batch", "median_batch",
		"recent_3_mean_per_unit", "recent_5_mean_per_unit", "recent_10_mean_per_unit",
		"recent_3_median_per_unit", "recent_5_median_per_unit",
		"recent_3_mean_batch", "recent_5_mean_batch",
		"linear_time_per_unit", "linear_time_batch",
	} {
		candidate := fitLaunchBatchEstimator(fit, metric, name)
		wape := launchBatchWAPE(validation, candidate)
		if wape < parameter.InnerValidationWAPE {
			best, parameter.Estimator, parameter.InnerValidationWAPE = candidate, name, wape
		}
	}
	return best, parameter
}

func launchBatchEmpiricalInterval(values []launchBatchCase, metric string) (float64, float64, string) {
	scale := "batch"
	if metric == "active_units" {
		scale = "per_unit"
	}
	numbers := make([]float64, 0, len(values))
	for _, value := range values {
		number := launchBatchMetric(value, metric)
		if scale == "per_unit" && value.UnitCount > 0 {
			number /= float64(value.UnitCount)
		}
		numbers = append(numbers, number)
	}
	sort.Float64s(numbers)
	return retrospectivePercentile(numbers, 0.1), retrospectivePercentile(numbers, 0.9), scale
}

func fitLaunchBatchEstimator(values []launchBatchCase, metric, name string) launchBatchEstimator {
	selected := values
	recentSize := 0
	for size, marker := range map[int]string{3: "recent_3_", 5: "recent_5_", 10: "recent_10_"} {
		if strings.HasPrefix(name, marker) {
			recentSize = size
		}
	}
	if recentSize > 0 && len(selected) > recentSize {
		selected = selected[len(selected)-recentSize:]
	}
	perUnit := strings.HasSuffix(name, "_per_unit")
	numbers := make([]float64, 0, len(selected))
	for _, value := range selected {
		number := launchBatchMetric(value, metric)
		if perUnit && value.UnitCount > 0 && metric != "top_spend_share" {
			number /= float64(value.UnitCount)
		}
		numbers = append(numbers, number)
	}
	useMedian := strings.Contains(name, "median")
	estimate := 0.0
	model := launchBatchEstimator{Name: name, Metric: metric, PerUnit: perUnit && metric != "top_spend_share", RecentSize: len(selected)}
	if strings.HasPrefix(name, "linear_time_") && len(numbers) >= 2 {
		model.Origin = selected[0].LaunchDay
		xMean, yMean := 0.0, 0.0
		for index, number := range numbers {
			xMean += selected[index].LaunchDay.Sub(model.Origin).Hours() / 24
			yMean += number
		}
		xMean /= float64(len(numbers))
		yMean /= float64(len(numbers))
		numerator, denominator := 0.0, 0.0
		for index, number := range numbers {
			x := selected[index].LaunchDay.Sub(model.Origin).Hours() / 24
			numerator += (x - xMean) * (number - yMean)
			denominator += (x - xMean) * (x - xMean)
		}
		model.Value = yMean
		if denominator > 0 {
			model.Slope = numerator / denominator
			model.Value = yMean - model.Slope*xMean
		}
	} else if useMedian {
		sort.Float64s(numbers)
		estimate = retrospectivePercentile(numbers, 0.5)
	} else if len(numbers) > 0 {
		for _, number := range numbers {
			estimate += number
		}
		estimate /= float64(len(numbers))
	}
	if !strings.HasPrefix(name, "linear_time_") {
		model.Value = estimate
	}
	for _, number := range numbers {
		model.Cap = math.Max(model.Cap, number*2)
	}
	return model
}

func predictLaunchBatch(model launchBatchEstimator, value launchBatchCase) float64 {
	prediction := model.Value
	if !model.Origin.IsZero() {
		prediction += model.Slope * value.LaunchDay.Sub(model.Origin).Hours() / 24
		prediction = math.Min(model.Cap, math.Max(0, prediction))
	}
	if model.PerUnit {
		prediction *= float64(value.UnitCount)
	}
	if model.Metric == "top_spend_share" {
		prediction = clampProbability(prediction)
	}
	return math.Max(0, prediction)
}

func launchBatchMetric(value launchBatchCase, metric string) float64 {
	switch metric {
	case "spend_minor":
		return value.Spend
	case "impressions":
		return value.Impressions
	case "clicks":
		return value.Clicks
	case "active_units":
		return float64(value.ActiveUnits)
	case "top_spend_share":
		return value.TopSpendShare
	default:
		return 0
	}
}

func launchBatchWAPE(values []launchBatchCase, model launchBatchEstimator) float64 {
	absolute, actual := 0.0, 0.0
	for _, value := range values {
		observed := launchBatchMetric(value, model.Metric)
		absolute += math.Abs(observed - predictLaunchBatch(model, value))
		actual += math.Abs(observed)
	}
	if actual == 0 {
		return math.Inf(1)
	}
	return absolute / actual
}

func evaluateLaunchBatchMetric(values []LaunchBatchHoldoutPrediction, metric string) LaunchBatchMetricEvaluation {
	result := LaunchBatchMetricEvaluation{Metric: metric, CaseCount: len(values)}
	for _, value := range values {
		forecast := value.Metrics[metric]
		result.ActualTotal += forecast.Observed
		result.PredictedTotal += forecast.Estimate
		result.MAE += math.Abs(forecast.Observed - forecast.Estimate)
		result.MeanIntervalWidth += forecast.Upper - forecast.Lower
		if forecast.Observed >= forecast.Lower && forecast.Observed <= forecast.Upper {
			result.Central80Coverage++
		}
	}
	if result.CaseCount > 0 {
		result.MAE /= float64(result.CaseCount)
		result.Central80Coverage /= float64(result.CaseCount)
		result.MeanIntervalWidth /= float64(result.CaseCount)
	}
	if result.ActualTotal > 0 {
		absolute := 0.0
		for _, value := range values {
			forecast := value.Metrics[metric]
			absolute += math.Abs(forecast.Observed - forecast.Estimate)
		}
		result.WAPE = absolute / result.ActualTotal
		result.IntervalWidthToActualMean = result.MeanIntervalWidth / (result.ActualTotal / float64(result.CaseCount))
	}
	return result
}

func calibrateLaunchBatchScenario(values []launchBatchCase) LaunchBatchScenarioCalibration {
	result := LaunchBatchScenarioCalibration{TrainingBatchCount: len(values), Typical: []LaunchBatchScenarioMetricDistribution{}, Breakout: []LaunchBatchScenarioMetricDistribution{}}
	spend := make([]float64, 0, len(values))
	for _, value := range values {
		spend = append(spend, value.Spend)
	}
	sort.Float64s(spend)
	result.ThresholdSpendMinor = retrospectivePercentile(spend, 0.8)
	typical, breakout := []launchBatchCase{}, []launchBatchCase{}
	for _, value := range values {
		if value.Spend > result.ThresholdSpendMinor {
			breakout = append(breakout, value)
		} else {
			typical = append(typical, value)
		}
	}
	result.TypicalBatchCount, result.BreakoutBatchCount = len(typical), len(breakout)
	result.BreakoutProbability = float64(len(breakout)+1) / float64(len(values)+2)
	for _, metric := range []string{"spend_minor", "impressions", "clicks", "active_units", "top_spend_share"} {
		result.Typical = append(result.Typical, launchBatchScenarioDistribution(typical, metric))
		result.Breakout = append(result.Breakout, launchBatchScenarioDistribution(breakout, metric))
	}
	return result
}

func launchBatchScenarioDistribution(values []launchBatchCase, metric string) LaunchBatchScenarioMetricDistribution {
	numbers := make([]float64, 0, len(values))
	for _, value := range values {
		numbers = append(numbers, launchBatchMetric(value, metric))
	}
	sort.Float64s(numbers)
	return LaunchBatchScenarioMetricDistribution{Metric: metric, P10: retrospectivePercentile(numbers, 0.1), P50: retrospectivePercentile(numbers, 0.5), P90: retrospectivePercentile(numbers, 0.9)}
}

func evaluateLaunchBatchScenario(values []LaunchBatchHoldoutPrediction) LaunchBatchScenarioEvaluation {
	result := LaunchBatchScenarioEvaluation{CaseCount: len(values)}
	for _, value := range values {
		observed := 0.0
		if value.BreakoutObserved {
			observed = 1
			result.ObservedBreakoutCount++
		}
		result.PredictedBreakoutRate += value.BreakoutProbability
		result.BrierScore += (value.BreakoutProbability - observed) * (value.BreakoutProbability - observed)
	}
	if result.CaseCount > 0 {
		result.ObservedBreakoutRate = float64(result.ObservedBreakoutCount) / float64(result.CaseCount)
		result.PredictedBreakoutRate /= float64(result.CaseCount)
		result.BrierScore /= float64(result.CaseCount)
		result.AbsoluteCalibrationError = math.Abs(result.ObservedBreakoutRate - result.PredictedBreakoutRate)
	}
	return result
}

func launchBatchReadiness(values []LaunchBatchMetricEvaluation, split LaunchBatchSplit, scenario LaunchBatchScenarioCalibration, scenarioEvaluation LaunchBatchScenarioEvaluation) string {
	if split.EvaluationDates < 5 || split.EvaluationBatches < 10 {
		return "blocked_temporal_coverage"
	}
	if scenario.BreakoutBatchCount > 0 && scenario.BreakoutBatchCount < 5 {
		return "blocked_breakout_segment_size"
	}
	if scenarioEvaluation.BrierScore > 0.3 || scenarioEvaluation.AbsoluteCalibrationError > 0.2 {
		return "blocked_scenario_calibration"
	}
	byMetric := map[string]LaunchBatchMetricEvaluation{}
	for _, value := range values {
		byMetric[value.Metric] = value
	}
	for _, metric := range []string{"spend_minor", "impressions", "clicks"} {
		value := byMetric[metric]
		if value.Central80Coverage < 0.6 {
			return "blocked_model_quality"
		}
	}
	if byMetric["active_units"].WAPE > 0.5 || byMetric["top_spend_share"].MAE > 0.25 {
		return "blocked_model_quality"
	}
	return "ready_for_probabilistic_shadow"
}

func walkForwardLaunchBatchPredictions(cases []launchBatchCase, metrics []string) ([]LaunchBatchHoldoutPrediction, int) {
	dates := []time.Time{}
	seenDates := map[time.Time]struct{}{}
	for _, value := range cases {
		if _, ok := seenDates[value.LaunchDay]; ok {
			continue
		}
		seenDates[value.LaunchDay] = struct{}{}
		dates = append(dates, value.LaunchDay)
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	result := []LaunchBatchHoldoutPrediction{}
	evaluatedDates := 0
	for _, cutoff := range dates {
		training, remaining := splitLaunchBatchCases(cases, cutoff)
		if len(training) < 15 {
			continue
		}
		current := make([]launchBatchCase, 0)
		for _, value := range remaining {
			if value.LaunchDay.Equal(cutoff) {
				current = append(current, value)
			}
		}
		if len(current) == 0 {
			continue
		}
		innerStart := launchBatchHoldoutStart(training)
		fit, validation := splitLaunchBatchCases(training, innerStart)
		if len(fit) < 10 || len(validation) < 3 {
			continue
		}
		models := map[string]launchBatchEstimator{}
		intervals := map[string][2]float64{}
		scales := map[string]string{}
		scenario := calibrateLaunchBatchScenario(training)
		for _, metric := range metrics {
			selected, _ := selectLaunchBatchEstimator(fit, validation, metric)
			models[metric] = fitLaunchBatchEstimator(training, metric, selected.Name)
			lower, upper, scale := launchBatchEmpiricalInterval(training, metric)
			intervals[metric], scales[metric] = [2]float64{lower, upper}, scale
		}
		for _, value := range current {
			prediction := LaunchBatchHoldoutPrediction{BatchRef: value.BatchRef, LaunchDay: value.LaunchDay, UnitCount: value.UnitCount, Metrics: map[string]LaunchBatchMetricForecast{}, BreakoutProbability: scenario.BreakoutProbability, BreakoutObserved: value.Spend > scenario.ThresholdSpendMinor}
			for _, metric := range metrics {
				bounds := intervals[metric]
				lower, upper := bounds[0], bounds[1]
				if scales[metric] == "per_unit" {
					lower *= float64(value.UnitCount)
					upper *= float64(value.UnitCount)
				}
				if metric == "top_spend_share" {
					lower, upper = clampProbability(lower), clampProbability(upper)
				}
				prediction.Metrics[metric] = LaunchBatchMetricForecast{Estimate: predictLaunchBatch(models[metric], value), Lower: lower, Upper: upper, Observed: launchBatchMetric(value, metric)}
			}
			result = append(result, prediction)
		}
		evaluatedDates++
	}
	return result, evaluatedDates
}
