package connector

import (
	"math"
	"sort"
	"time"
)

type RetrospectiveCalibrationDiagnostics struct {
	Status            string                            `json:"status"`
	UsedForPrediction bool                              `json:"used_for_prediction"`
	Thresholds        RetrospectiveDiagnosticThresholds `json:"thresholds"`
	Training          RetrospectiveSplitDiagnostics     `json:"training"`
	Holdout           RetrospectiveSplitDiagnostics     `json:"holdout"`
	MetricDrift       []RetrospectiveMetricDrift        `json:"metric_drift"`
	Signals           []string                          `json:"signals"`
}

type RetrospectiveDiagnosticThresholds struct {
	MinimumHoldoutCutoffDates   int     `json:"minimum_holdout_cutoff_dates"`
	MaximumDateCaseShare        float64 `json:"maximum_date_case_share"`
	MaximumUnseenPromotionShare float64 `json:"maximum_unseen_promotion_share"`
	MaximumPositiveRateDelta    float64 `json:"maximum_positive_rate_delta"`
	MinimumPositiveMeanRatio    float64 `json:"minimum_positive_mean_ratio"`
	MaximumPositiveMeanRatio    float64 `json:"maximum_positive_mean_ratio"`
}

type RetrospectiveSplitDiagnostics struct {
	CaseCount            int                               `json:"case_count"`
	CutoffDateCount      int                               `json:"cutoff_date_count"`
	ProjectCount         int                               `json:"project_count"`
	PromotionCount       int                               `json:"promotion_count"`
	UnseenPromotionCases int                               `json:"unseen_promotion_cases"`
	UnseenPromotionShare float64                           `json:"unseen_promotion_share"`
	MaximumDateCaseShare float64                           `json:"maximum_date_case_share"`
	Metrics              []RetrospectiveMetricDistribution `json:"metrics"`
}

type RetrospectiveMetricDistribution struct {
	Metric            string  `json:"metric"`
	CaseCount         int     `json:"case_count"`
	PositiveCaseCount int     `json:"positive_case_count"`
	PositiveRate      float64 `json:"positive_rate"`
	PositiveMean      float64 `json:"positive_mean"`
	P90               float64 `json:"p90"`
	P99               float64 `json:"p99"`
	Maximum           float64 `json:"maximum"`
}

type RetrospectiveMetricDrift struct {
	Metric            string   `json:"metric"`
	PositiveRateDelta float64  `json:"positive_rate_delta"`
	PositiveMeanRatio *float64 `json:"positive_mean_ratio,omitempty"`
}

func diagnoseRetrospectiveCalibration(cases []RetrospectiveCalibrationCase, holdoutStart time.Time) RetrospectiveCalibrationDiagnostics {
	thresholds := RetrospectiveDiagnosticThresholds{
		MinimumHoldoutCutoffDates: 7, MaximumDateCaseShare: 0.4, MaximumUnseenPromotionShare: 0.2,
		MaximumPositiveRateDelta: 0.2, MinimumPositiveMeanRatio: 0.67, MaximumPositiveMeanRatio: 1.5,
	}
	training, holdout := make([]RetrospectiveCalibrationCase, 0), make([]RetrospectiveCalibrationCase, 0)
	for _, value := range cases {
		if value.PredictionCutoff.Before(holdoutStart) {
			training = append(training, value)
		} else {
			holdout = append(holdout, value)
		}
	}
	trainingPromotions := map[string]struct{}{}
	for _, value := range training {
		trainingPromotions[value.PromotionRef] = struct{}{}
	}
	result := RetrospectiveCalibrationDiagnostics{
		Status: "stable", UsedForPrediction: false, Thresholds: thresholds,
		Training: retrospectiveSplitDiagnostics(training, nil), Holdout: retrospectiveSplitDiagnostics(holdout, trainingPromotions),
		MetricDrift: []RetrospectiveMetricDrift{}, Signals: []string{},
	}
	if result.Holdout.CutoffDateCount < thresholds.MinimumHoldoutCutoffDates {
		result.Signals = append(result.Signals, "holdout_cutoff_dates_below_minimum")
	}
	if result.Holdout.MaximumDateCaseShare > thresholds.MaximumDateCaseShare {
		result.Signals = append(result.Signals, "holdout_case_concentration_above_limit")
	}
	if result.Holdout.UnseenPromotionShare > thresholds.MaximumUnseenPromotionShare {
		result.Signals = append(result.Signals, "holdout_unseen_promotion_share_above_limit")
	}
	trainingMetrics := retrospectiveMetricDistributionByName(result.Training.Metrics)
	holdoutMetrics := retrospectiveMetricDistributionByName(result.Holdout.Metrics)
	for _, metric := range []string{"spend_minor", "impressions", "clicks"} {
		trainingMetric := trainingMetrics[metric]
		holdoutMetric := holdoutMetrics[metric]
		drift := RetrospectiveMetricDrift{Metric: metric, PositiveRateDelta: holdoutMetric.PositiveRate - trainingMetric.PositiveRate}
		if trainingMetric.PositiveMean > 0 {
			ratio := holdoutMetric.PositiveMean / trainingMetric.PositiveMean
			drift.PositiveMeanRatio = &ratio
			if ratio < thresholds.MinimumPositiveMeanRatio || ratio > thresholds.MaximumPositiveMeanRatio {
				result.Signals = append(result.Signals, metric+":positive_magnitude_shift")
			}
		}
		if math.Abs(drift.PositiveRateDelta) > thresholds.MaximumPositiveRateDelta {
			result.Signals = append(result.Signals, metric+":positive_rate_shift")
		}
		result.MetricDrift = append(result.MetricDrift, drift)
	}
	if len(result.Signals) > 0 {
		result.Status = "distribution_shift_detected"
	}
	return result
}

func retrospectiveSplitDiagnostics(cases []RetrospectiveCalibrationCase, trainingPromotions map[string]struct{}) RetrospectiveSplitDiagnostics {
	result := RetrospectiveSplitDiagnostics{CaseCount: len(cases), Metrics: []RetrospectiveMetricDistribution{}}
	cutoffDates, projects, promotions := map[string]int{}, map[string]struct{}{}, map[string]struct{}{}
	for _, value := range cases {
		cutoffDates[value.PredictionCutoff.UTC().Format("2006-01-02")]++
		projects[value.ProjectRef] = struct{}{}
		promotions[value.PromotionRef] = struct{}{}
		if trainingPromotions != nil {
			if _, ok := trainingPromotions[value.PromotionRef]; !ok {
				result.UnseenPromotionCases++
			}
		}
	}
	result.CutoffDateCount = len(cutoffDates)
	result.ProjectCount = len(projects)
	result.PromotionCount = len(promotions)
	if len(cases) > 0 {
		result.UnseenPromotionShare = float64(result.UnseenPromotionCases) / float64(len(cases))
		for _, count := range cutoffDates {
			result.MaximumDateCaseShare = math.Max(result.MaximumDateCaseShare, float64(count)/float64(len(cases)))
		}
	}
	for _, metric := range []string{"spend_minor", "impressions", "clicks"} {
		result.Metrics = append(result.Metrics, retrospectiveMetricDistribution(cases, metric))
	}
	return result
}

func retrospectiveMetricDistribution(cases []RetrospectiveCalibrationCase, metric string) RetrospectiveMetricDistribution {
	result := RetrospectiveMetricDistribution{Metric: metric, CaseCount: len(cases)}
	values := make([]float64, 0, len(cases))
	positiveTotal := 0.0
	for _, value := range cases {
		actual, _, ok := retrospectiveMetricPair(value, metric)
		if !ok {
			continue
		}
		values = append(values, actual)
		if actual > 0 {
			result.PositiveCaseCount++
			positiveTotal += actual
		}
		result.Maximum = math.Max(result.Maximum, actual)
	}
	if len(values) == 0 {
		return result
	}
	result.CaseCount = len(values)
	result.PositiveRate = float64(result.PositiveCaseCount) / float64(len(values))
	if result.PositiveCaseCount > 0 {
		result.PositiveMean = positiveTotal / float64(result.PositiveCaseCount)
	}
	sort.Float64s(values)
	result.P90 = retrospectivePercentile(values, 0.9)
	result.P99 = retrospectivePercentile(values, 0.99)
	return result
}

func retrospectivePercentile(sortedValues []float64, percentile float64) float64 {
	if len(sortedValues) == 0 {
		return 0
	}
	index := int(math.Floor(float64(len(sortedValues)-1) * percentile))
	return sortedValues[index]
}

func retrospectiveMetricDistributionByName(values []RetrospectiveMetricDistribution) map[string]RetrospectiveMetricDistribution {
	result := make(map[string]RetrospectiveMetricDistribution, len(values))
	for _, value := range values {
		result[value.Metric] = value
	}
	return result
}
