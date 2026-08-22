package connector

import (
	"math"
	"sort"
	"time"
)

const RetrospectiveLifecycleHurdleModelVersion = "lifecycle_hurdle_v3"

type RetrospectiveLifecycleFeatures struct {
	Spend       RetrospectiveLifecycleSignal `json:"spend"`
	Impressions RetrospectiveLifecycleSignal `json:"impressions"`
	Clicks      RetrospectiveLifecycleSignal `json:"clicks"`
}

type RetrospectiveLifecycleSignal struct {
	AgeDays                    int     `json:"age_days"`
	AgeBucket                  string  `json:"age_bucket"`
	DaysSinceLastPositive      *int    `json:"days_since_last_positive,omitempty"`
	RecencyBucket              string  `json:"recency_bucket"`
	ConsecutivePositiveWindows int     `json:"consecutive_positive_windows"`
	StreakBucket               string  `json:"streak_bucket"`
	TrendBucket                string  `json:"trend_bucket"`
	LatestValue                int64   `json:"latest_value"`
	RecentWindowCount          int     `json:"recent_window_count"`
	RecentMean                 float64 `json:"recent_mean"`
}

type RetrospectiveLifecycleCalibration struct {
	ModelVersion                 string                               `json:"model_version"`
	Status                       string                               `json:"status"`
	TrainingCaseCount            int                                  `json:"training_case_count"`
	HoldoutCaseCount             int                                  `json:"holdout_case_count"`
	MinimumPositiveTrainingCases int                                  `json:"minimum_positive_training_cases"`
	ActivityPriorStrength        float64                              `json:"activity_prior_strength"`
	MagnitudePriorStrength       float64                              `json:"magnitude_prior_strength"`
	SegmentPriorStrength         float64                              `json:"segment_prior_strength"`
	SegmentWeightCap             float64                              `json:"segment_weight_cap"`
	GlobalPriors                 []RetrospectiveHurdlePrior           `json:"global_priors"`
	ProjectPriors                []RetrospectiveProjectHurdlePrior    `json:"project_priors"`
	SegmentPriors                []RetrospectiveLifecycleSegmentPrior `json:"segment_priors"`
	OutputScales                 []RetrospectiveCalibrationParameter  `json:"output_scales"`
}

type RetrospectiveLifecycleSegmentPrior struct {
	Metric            string  `json:"metric"`
	SegmentKey        string  `json:"segment_key"`
	TrainingCaseCount int     `json:"training_case_count"`
	ActiveCaseCount   int     `json:"active_case_count"`
	ActiveProbability float64 `json:"active_probability"`
	PositiveMean      float64 `json:"positive_mean"`
}

func retrospectiveLifecycleFeatures(allMetrics, history []MetricWindow, cutoff time.Time) RetrospectiveLifecycleFeatures {
	return RetrospectiveLifecycleFeatures{
		Spend:       retrospectiveLifecycleSignal(allMetrics, history, cutoff, "spend"),
		Impressions: retrospectiveLifecycleSignal(allMetrics, history, cutoff, "impressions"),
		Clicks:      retrospectiveLifecycleSignal(allMetrics, history, cutoff, "clicks"),
	}
}

func retrospectiveLifecycleSignal(allMetrics, history []MetricWindow, cutoff time.Time, metric string) RetrospectiveLifecycleSignal {
	prior := make([]MetricWindow, 0, len(allMetrics))
	for _, value := range allMetrics {
		if isRetrospectiveAtomicWindow(value) && !value.WindowEnd.After(cutoff) {
			prior = append(prior, value)
		}
	}
	sort.Slice(prior, func(i, j int) bool { return prior[i].WindowStart.Before(prior[j].WindowStart) })
	result := RetrospectiveLifecycleSignal{AgeBucket: "unknown", RecencyBucket: "never_positive", StreakBucket: "none", TrendBucket: retrospectiveTrendBucket(history, metric)}
	if len(prior) == 0 {
		return result
	}
	result.AgeDays = int(math.Max(0, cutoff.Sub(prior[0].WindowStart).Hours()/24))
	switch {
	case result.AgeDays <= 7:
		result.AgeBucket = "launch"
	case result.AgeDays <= 30:
		result.AgeBucket = "learning"
	default:
		result.AgeBucket = "established"
	}
	for index := len(prior) - 1; index >= 0; index-- {
		if prior[index].Metrics[metric] > 0 {
			days := int(math.Max(0, cutoff.Sub(prior[index].WindowEnd).Hours()/24))
			result.DaysSinceLastPositive = &days
			switch {
			case days <= 1:
				result.RecencyBucket = "recent"
			case days <= 3:
				result.RecencyBucket = "cooling"
			case days <= 7:
				result.RecencyBucket = "dormant"
			default:
				result.RecencyBucket = "long_dormant"
			}
			break
		}
	}
	for index := len(prior) - 1; index >= 0; index-- {
		if prior[index].Metrics[metric] <= 0 {
			break
		}
		result.ConsecutivePositiveWindows++
	}
	result.StreakBucket = lifecycleStreakBucket(result.ConsecutivePositiveWindows)
	recent := append([]MetricWindow(nil), history...)
	sort.Slice(recent, func(i, j int) bool { return recent[i].WindowStart.Before(recent[j].WindowStart) })
	if len(recent) > 0 {
		result.LatestValue = recent[len(recent)-1].Metrics[metric]
		start := len(recent) - 3
		if start < 0 {
			start = 0
		}
		for _, value := range recent[start:] {
			result.RecentMean += float64(value.Metrics[metric])
			result.RecentWindowCount++
		}
		result.RecentMean /= float64(result.RecentWindowCount)
	}
	return result
}

func retrospectiveTrendBucket(history []MetricWindow, metric string) string {
	if len(history) < 2 {
		return "insufficient"
	}
	values := append([]MetricWindow(nil), history...)
	sort.Slice(values, func(i, j int) bool { return values[i].WindowStart.Before(values[j].WindowStart) })
	middle := len(values) / 2
	if middle == 0 || middle == len(values) {
		return "insufficient"
	}
	previous, recent := 0.0, 0.0
	for _, value := range values[:middle] {
		previous += float64(value.Metrics[metric])
	}
	for _, value := range values[middle:] {
		recent += float64(value.Metrics[metric])
	}
	previous /= float64(middle)
	recent /= float64(len(values) - middle)
	switch {
	case previous == 0 && recent == 0:
		return "flat_zero"
	case previous == 0:
		return "rising"
	case recent > previous*1.25:
		return "rising"
	case recent < previous*0.75:
		return "falling"
	default:
		return "stable"
	}
}

func lifecycleSignal(value RetrospectiveCalibrationCase, metric string) RetrospectiveLifecycleSignal {
	switch metric {
	case "spend_minor":
		return value.Lifecycle.Spend
	case "impressions":
		return value.Lifecycle.Impressions
	case "clicks":
		return value.Lifecycle.Clicks
	default:
		return RetrospectiveLifecycleSignal{AgeBucket: "unknown", RecencyBucket: "unknown", TrendBucket: "unknown"}
	}
}

func lifecycleSegmentKey(value RetrospectiveCalibrationCase, metric string) string {
	signal := lifecycleSignal(value, metric)
	return signal.AgeBucket + "|" + signal.RecencyBucket + "|" + signal.StreakBucket + "|" + signal.TrendBucket
}

func lifecycleStreakBucket(value int) string {
	switch {
	case value == 0:
		return "none"
	case value == 1:
		return "single"
	case value <= 3:
		return "short"
	default:
		return "sustained"
	}
}

func calibrateAndEvaluateRetrospectiveLifecycle(cases []RetrospectiveCalibrationCase, holdoutStart time.Time) (RetrospectiveLifecycleCalibration, []RetrospectiveMetricEvaluation, map[string]RetrospectiveHurdlePrediction) {
	const activityStrength = 3.0
	const magnitudeStrength = 3.0
	const segmentStrength = 4.0
	const segmentWeightCap = 0.7
	const segmentFullWeightCases = 20.0
	const minimumPositiveTrainingCases = 10
	metrics := []string{"spend_minor", "impressions", "clicks"}
	training, holdout := make([]RetrospectiveCalibrationCase, 0), make([]RetrospectiveCalibrationCase, 0)
	for _, value := range cases {
		if value.HorizonEnd.Sub(value.PredictionCutoff) != 24*time.Hour {
			return RetrospectiveLifecycleCalibration{ModelVersion: RetrospectiveLifecycleHurdleModelVersion, Status: "unsupported_horizon", MinimumPositiveTrainingCases: minimumPositiveTrainingCases, ActivityPriorStrength: activityStrength, MagnitudePriorStrength: magnitudeStrength, SegmentPriorStrength: segmentStrength, SegmentWeightCap: segmentWeightCap}, nil, map[string]RetrospectiveHurdlePrediction{}
		}
		if value.PredictionCutoff.Before(holdoutStart) {
			training = append(training, value)
		} else {
			holdout = append(holdout, value)
		}
	}
	calibration := RetrospectiveLifecycleCalibration{
		ModelVersion: RetrospectiveLifecycleHurdleModelVersion, Status: "insufficient_cases",
		TrainingCaseCount: len(training), HoldoutCaseCount: len(holdout),
		MinimumPositiveTrainingCases: minimumPositiveTrainingCases,
		ActivityPriorStrength:        activityStrength, MagnitudePriorStrength: magnitudeStrength,
		SegmentPriorStrength: segmentStrength, SegmentWeightCap: segmentWeightCap,
		GlobalPriors: []RetrospectiveHurdlePrior{}, ProjectPriors: []RetrospectiveProjectHurdlePrior{},
		SegmentPriors: []RetrospectiveLifecycleSegmentPrior{}, OutputScales: []RetrospectiveCalibrationParameter{},
	}
	if len(training) < 30 || len(holdout) < 30 {
		return calibration, nil, map[string]RetrospectiveHurdlePrediction{}
	}

	globalStats := map[string]*hurdleStats{}
	projectStats := map[string]map[string]*hurdleStats{}
	segmentStats := map[string]map[string]*hurdleStats{}
	for _, metric := range metrics {
		globalStats[metric] = &hurdleStats{}
		segmentStats[metric] = map[string]*hurdleStats{}
	}
	for _, value := range training {
		if value.ProjectRef != "" && projectStats[value.ProjectRef] == nil {
			projectStats[value.ProjectRef] = map[string]*hurdleStats{}
			for _, metric := range metrics {
				projectStats[value.ProjectRef][metric] = &hurdleStats{}
			}
		}
		for _, metric := range metrics {
			actual, _, _ := retrospectiveMetricPair(value, metric)
			addHurdleObservation(globalStats[metric], actual)
			if value.ProjectRef != "" {
				addHurdleObservation(projectStats[value.ProjectRef][metric], actual)
			}
			segmentKey := lifecycleSegmentKey(value, metric)
			if segmentStats[metric][segmentKey] == nil {
				segmentStats[metric][segmentKey] = &hurdleStats{}
			}
			addHurdleObservation(segmentStats[metric][segmentKey], actual)
		}
	}

	globalPriors := map[string]RetrospectiveHurdlePrior{}
	for _, metric := range metrics {
		prior := globalHurdlePrior(metric, *globalStats[metric])
		globalPriors[metric] = prior
		calibration.GlobalPriors = append(calibration.GlobalPriors, prior)
	}
	for _, metric := range metrics {
		if globalStats[metric].active < minimumPositiveTrainingCases {
			calibration.Status = "insufficient_positive_training_cases"
			return calibration, nil, map[string]RetrospectiveHurdlePrediction{}
		}
	}
	projectPriors := map[string]map[string]RetrospectiveHurdlePrior{}
	projectRefs := make([]string, 0, len(projectStats))
	for projectRef := range projectStats {
		projectRefs = append(projectRefs, projectRef)
	}
	sort.Strings(projectRefs)
	for _, projectRef := range projectRefs {
		projectPriors[projectRef] = map[string]RetrospectiveHurdlePrior{}
		projectCalibration := RetrospectiveProjectHurdlePrior{ProjectRef: projectRef, Priors: []RetrospectiveHurdlePrior{}}
		for _, metric := range metrics {
			global := globalPriors[metric]
			prior := hurdlePrior(metric, *projectStats[projectRef][metric], global.ActiveProbability, global.PositiveMean)
			projectPriors[projectRef][metric] = prior
			projectCalibration.Priors = append(projectCalibration.Priors, prior)
		}
		calibration.ProjectPriors = append(calibration.ProjectPriors, projectCalibration)
	}
	segmentPriors := map[string]map[string]RetrospectiveLifecycleSegmentPrior{}
	for _, metric := range metrics {
		segmentPriors[metric] = map[string]RetrospectiveLifecycleSegmentPrior{}
		segmentKeys := make([]string, 0, len(segmentStats[metric]))
		for segmentKey := range segmentStats[metric] {
			segmentKeys = append(segmentKeys, segmentKey)
		}
		sort.Strings(segmentKeys)
		for _, segmentKey := range segmentKeys {
			stats := *segmentStats[metric][segmentKey]
			global := globalPriors[metric]
			probability := (float64(stats.active) + segmentStrength*global.ActiveProbability) / (float64(stats.count) + segmentStrength)
			positiveMean := (stats.positiveSum + segmentStrength*global.PositiveMean) / (float64(stats.active) + segmentStrength)
			prior := RetrospectiveLifecycleSegmentPrior{Metric: metric, SegmentKey: segmentKey, TrainingCaseCount: stats.count, ActiveCaseCount: stats.active, ActiveProbability: clampProbability(probability), PositiveMean: math.Max(0, positiveMean)}
			segmentPriors[metric][segmentKey] = prior
			calibration.SegmentPriors = append(calibration.SegmentPriors, prior)
		}
	}

	predict := func(value RetrospectiveCalibrationCase, metric string) (float64, float64) {
		prior := globalPriors[metric]
		if project, ok := projectPriors[value.ProjectRef]; ok {
			if projectPrior, found := project[metric]; found {
				prior = projectPrior
			}
		}
		if segment, ok := segmentPriors[metric][lifecycleSegmentKey(value, metric)]; ok {
			weight := math.Min(segmentWeightCap, segmentWeightCap*float64(segment.TrainingCaseCount)/segmentFullWeightCases)
			prior.ActiveProbability = (1-weight)*prior.ActiveProbability + weight*segment.ActiveProbability
			prior.PositiveMean = (1-weight)*prior.PositiveMean + weight*segment.PositiveMean
		}
		return predictHurdleMetricWithPrior(value, metric, prior, activityStrength, magnitudeStrength)
	}

	calibration.Status = "evaluated"
	outputScales := map[string]float64{}
	for _, metric := range metrics {
		actualTotal, predictedTotal := 0.0, 0.0
		for _, value := range training {
			actual, _, _ := retrospectiveMetricPair(value, metric)
			prediction, _ := predict(value, metric)
			actualTotal += actual
			predictedTotal += prediction
		}
		if predictedTotal <= 0 {
			continue
		}
		scale := math.Max(0.25, math.Min(4, actualTotal/predictedTotal))
		outputScales[metric] = scale
		calibration.OutputScales = append(calibration.OutputScales, RetrospectiveCalibrationParameter{Metric: metric, Parameter: "lifecycle_hurdle_output_scale", Value: scale, TrainingCaseCount: len(training), TrainingEnd: holdoutStart})
	}
	predictions := make(map[string]RetrospectiveHurdlePrediction, len(holdout))
	candidateCases := make([]RetrospectiveCalibrationCase, 0, len(holdout))
	for _, value := range holdout {
		prediction := RetrospectiveHurdlePrediction{}
		prediction.Estimate.SpendMinor, prediction.ActiveProbability.Spend = predict(value, "spend_minor")
		prediction.Estimate.Impressions, prediction.ActiveProbability.Impressions = predict(value, "impressions")
		prediction.Estimate.Clicks, prediction.ActiveProbability.Clicks = predict(value, "clicks")
		prediction.Estimate.SpendMinor *= retrospectiveOutputScale(outputScales, "spend_minor")
		prediction.Estimate.Impressions *= retrospectiveOutputScale(outputScales, "impressions")
		prediction.Estimate.Clicks *= retrospectiveOutputScale(outputScales, "clicks")
		predictions[value.CaseID] = prediction
		candidate := value
		candidate.BaselinePrediction = prediction.Estimate
		candidateCases = append(candidateCases, candidate)
	}
	evaluation := evaluateRetrospectiveCases(candidateCases, RetrospectiveLifecycleHurdleModelVersion, "time_holdout", nil)
	return calibration, evaluation, predictions
}

func predictHurdleMetricWithPrior(value RetrospectiveCalibrationCase, metric string, prior RetrospectiveHurdlePrior, activityStrength, magnitudeStrength float64) (float64, float64) {
	positiveWindows, historyTotal := hurdleHistory(value, metric)
	observedWindows := value.HistoryActivity.ObservedWindows
	probability := (float64(positiveWindows) + activityStrength*prior.ActiveProbability) / (float64(observedWindows) + activityStrength)
	positiveMean := (historyTotal + magnitudeStrength*prior.PositiveMean) / (float64(positiveWindows) + magnitudeStrength)
	return math.Max(0, clampProbability(probability)*positiveMean), clampProbability(probability)
}

func retrospectiveOutputScale(scales map[string]float64, metric string) float64 {
	if scale, ok := scales[metric]; ok {
		return scale
	}
	return 1
}
