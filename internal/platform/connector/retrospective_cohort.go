package connector

import (
	"math"
	"sort"
	"time"
)

const RetrospectiveCohortModelVersion = "cohort_continuation_v4"

type RetrospectiveCohortCalibration struct {
	ModelVersion      string                               `json:"model_version"`
	Status            string                               `json:"status"`
	TrainingCaseCount int                                  `json:"training_case_count"`
	HoldoutCaseCount  int                                  `json:"holdout_case_count"`
	Parameters        []RetrospectiveCohortMetricParameter `json:"parameters"`
}

type RetrospectiveCohortMetricParameter struct {
	Metric                   string  `json:"metric"`
	RatioQuantile            float64 `json:"ratio_quantile"`
	PriorStrength            float64 `json:"prior_strength"`
	SegmentDepth             int     `json:"segment_depth"`
	RatioCap                 float64 `json:"ratio_cap"`
	InnerValidationCaseCount int     `json:"inner_validation_case_count"`
	InnerValidationWAPE      float64 `json:"inner_validation_wape"`
}

type retrospectiveCohortRatioModel struct {
	global   float64
	segments map[string]float64
}

func calibrateAndEvaluateRetrospectiveCohort(cases []RetrospectiveCalibrationCase, holdoutStart time.Time) (RetrospectiveCohortCalibration, []RetrospectiveMetricEvaluation, map[string]RetrospectiveHurdlePrediction) {
	training, holdout := splitRetrospectiveCases(cases, holdoutStart)
	calibration := RetrospectiveCohortCalibration{ModelVersion: RetrospectiveCohortModelVersion, Status: "insufficient_cases", TrainingCaseCount: len(training), HoldoutCaseCount: len(holdout), Parameters: []RetrospectiveCohortMetricParameter{}}
	if len(training) < 60 || len(holdout) < 30 {
		return calibration, nil, map[string]RetrospectiveHurdlePrediction{}
	}
	for _, value := range cases {
		if value.HorizonEnd.Sub(value.PredictionCutoff) != 24*time.Hour {
			calibration.Status = "unsupported_horizon"
			return calibration, nil, map[string]RetrospectiveHurdlePrediction{}
		}
	}
	innerStart := retrospectiveHoldoutStart(training, holdoutStart)
	fit, validation := splitRetrospectiveCases(training, innerStart)
	if len(fit) < 30 || len(validation) < 15 {
		return calibration, nil, map[string]RetrospectiveHurdlePrediction{}
	}
	metrics := []string{"spend_minor", "impressions", "clicks"}
	models := map[string]retrospectiveCohortRatioModel{}
	for _, metric := range metrics {
		parameter, ok := selectRetrospectiveCohortParameter(fit, validation, metric)
		if !ok {
			calibration.Status = "insufficient_continuation_cases"
			return calibration, nil, map[string]RetrospectiveHurdlePrediction{}
		}
		calibration.Parameters = append(calibration.Parameters, parameter)
		models[metric] = fitRetrospectiveCohortRatio(training, metric, parameter)
	}
	calibration.Status = "evaluated"
	predictions := make(map[string]RetrospectiveHurdlePrediction, len(holdout))
	candidateCases := make([]RetrospectiveCalibrationCase, 0, len(holdout))
	for _, value := range holdout {
		prediction := RetrospectiveHurdlePrediction{}
		prediction.Estimate.SpendMinor, prediction.ActiveProbability.Spend = predictRetrospectiveCohort(value, "spend_minor", models["spend_minor"], calibration.Parameters[0])
		prediction.Estimate.Impressions, prediction.ActiveProbability.Impressions = predictRetrospectiveCohort(value, "impressions", models["impressions"], calibration.Parameters[1])
		prediction.Estimate.Clicks, prediction.ActiveProbability.Clicks = predictRetrospectiveCohort(value, "clicks", models["clicks"], calibration.Parameters[2])
		predictions[value.CaseID] = prediction
		candidate := value
		candidate.BaselinePrediction = prediction.Estimate
		candidateCases = append(candidateCases, candidate)
	}
	evaluation := evaluateRetrospectiveCases(candidateCases, RetrospectiveCohortModelVersion, "time_holdout", nil)
	trainingPromotions := retrospectivePromotionSet(training)
	coldStart, warmStart := retrospectiveHoldoutCohorts(candidateCases, trainingPromotions)
	evaluation = append(evaluation, evaluateRetrospectiveCases(coldStart, RetrospectiveCohortModelVersion, "time_holdout_cold_start", nil)...)
	evaluation = append(evaluation, evaluateRetrospectiveCases(warmStart, RetrospectiveCohortModelVersion, "time_holdout_warm_start", nil)...)
	return calibration, evaluation, predictions
}

func splitRetrospectiveCases(cases []RetrospectiveCalibrationCase, cutoff time.Time) ([]RetrospectiveCalibrationCase, []RetrospectiveCalibrationCase) {
	before, after := make([]RetrospectiveCalibrationCase, 0), make([]RetrospectiveCalibrationCase, 0)
	for _, value := range cases {
		if value.PredictionCutoff.Before(cutoff) {
			before = append(before, value)
		} else {
			after = append(after, value)
		}
	}
	return before, after
}

func retrospectivePromotionSet(cases []RetrospectiveCalibrationCase) map[string]struct{} {
	result := make(map[string]struct{}, len(cases))
	for _, value := range cases {
		result[value.PromotionRef] = struct{}{}
	}
	return result
}

func selectRetrospectiveCohortParameter(fit, validation []RetrospectiveCalibrationCase, metric string) (RetrospectiveCohortMetricParameter, bool) {
	best := RetrospectiveCohortMetricParameter{}
	bestWAPE := math.Inf(1)
	for _, quantile := range []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6} {
		for _, strength := range []float64{2, 5, 10, 20} {
			for depth := 1; depth <= 4; depth++ {
				parameter := RetrospectiveCohortMetricParameter{Metric: metric, RatioQuantile: quantile, PriorStrength: strength, SegmentDepth: depth, RatioCap: 4, InnerValidationCaseCount: len(validation)}
				model := fitRetrospectiveCohortRatio(fit, metric, parameter)
				if len(model.segments) == 0 && model.global == 0 {
					continue
				}
				wape, ok := retrospectiveCohortWAPE(validation, metric, model, parameter)
				if ok && wape < bestWAPE {
					bestWAPE = wape
					parameter.InnerValidationWAPE = wape
					best = parameter
				}
			}
		}
	}
	return best, !math.IsInf(bestWAPE, 1)
}

func fitRetrospectiveCohortRatio(cases []RetrospectiveCalibrationCase, metric string, parameter RetrospectiveCohortMetricParameter) retrospectiveCohortRatioModel {
	globalValues := make([]float64, 0, len(cases))
	segmentValues := map[string][]float64{}
	for _, value := range cases {
		latest := retrospectiveCohortLatest(value, metric)
		actual, _, ok := retrospectiveMetricPair(value, metric)
		if !ok || latest <= 0 {
			continue
		}
		ratio := math.Max(0, math.Min(parameter.RatioCap, actual/latest))
		globalValues = append(globalValues, ratio)
		key := retrospectiveCohortSegment(value, metric, parameter.SegmentDepth)
		segmentValues[key] = append(segmentValues[key], ratio)
	}
	sort.Float64s(globalValues)
	model := retrospectiveCohortRatioModel{global: retrospectivePercentile(globalValues, parameter.RatioQuantile), segments: map[string]float64{}}
	for key, values := range segmentValues {
		sort.Float64s(values)
		local := retrospectivePercentile(values, parameter.RatioQuantile)
		weight := float64(len(values)) / (float64(len(values)) + parameter.PriorStrength)
		model.segments[key] = (1-weight)*model.global + weight*local
	}
	return model
}

func predictRetrospectiveCohort(value RetrospectiveCalibrationCase, metric string, model retrospectiveCohortRatioModel, parameter RetrospectiveCohortMetricParameter) (float64, float64) {
	latest := retrospectiveCohortLatest(value, metric)
	if latest <= 0 {
		return 0, 0
	}
	ratio := model.global
	if segment, ok := model.segments[retrospectiveCohortSegment(value, metric, parameter.SegmentDepth)]; ok {
		ratio = segment
	}
	return math.Max(0, latest*ratio), clampProbability(ratio)
}

func retrospectiveCohortWAPE(cases []RetrospectiveCalibrationCase, metric string, model retrospectiveCohortRatioModel, parameter RetrospectiveCohortMetricParameter) (float64, bool) {
	absolute, actualTotal := 0.0, 0.0
	for _, value := range cases {
		actual, _, ok := retrospectiveMetricPair(value, metric)
		if !ok {
			continue
		}
		prediction, _ := predictRetrospectiveCohort(value, metric, model, parameter)
		absolute += math.Abs(prediction - actual)
		actualTotal += actual
	}
	if actualTotal <= 0 {
		return 0, false
	}
	return absolute / actualTotal, true
}

func retrospectiveCohortLatest(value RetrospectiveCalibrationCase, metric string) float64 {
	return float64(lifecycleSignal(value, metric).LatestValue)
}

func retrospectiveCohortSegment(value RetrospectiveCalibrationCase, metric string, depth int) string {
	signal := lifecycleSignal(value, metric)
	parts := []string{signal.AgeBucket, signal.RecencyBucket, signal.TrendBucket, signal.StreakBucket}
	if depth < 1 {
		depth = 1
	}
	if depth > len(parts) {
		depth = len(parts)
	}
	result := parts[0]
	for _, part := range parts[1:depth] {
		result += "|" + part
	}
	return result
}
