package connector

import (
	"math"
	"sort"
	"time"
)

const RetrospectiveHurdleModelVersion = "hierarchical_hurdle_v2"

type RetrospectiveMetricActivity struct {
	ObservedWindows           int `json:"observed_windows"`
	SpendPositiveWindows      int `json:"spend_positive_windows"`
	ImpressionPositiveWindows int `json:"impression_positive_windows"`
	ClickPositiveWindows      int `json:"click_positive_windows"`
}

type RetrospectiveHurdleCalibration struct {
	ModelVersion                 string                              `json:"model_version"`
	Status                       string                              `json:"status"`
	TrainingCaseCount            int                                 `json:"training_case_count"`
	HoldoutCaseCount             int                                 `json:"holdout_case_count"`
	MinimumPositiveTrainingCases int                                 `json:"minimum_positive_training_cases"`
	ActivityPriorStrength        float64                             `json:"activity_prior_strength"`
	MagnitudePriorStrength       float64                             `json:"magnitude_prior_strength"`
	GlobalPriors                 []RetrospectiveHurdlePrior          `json:"global_priors"`
	ProjectPriors                []RetrospectiveProjectHurdlePrior   `json:"project_priors"`
	OutputScales                 []RetrospectiveCalibrationParameter `json:"output_scales"`
}

type RetrospectiveHurdlePrior struct {
	Metric            string  `json:"metric"`
	TrainingCaseCount int     `json:"training_case_count"`
	ActiveCaseCount   int     `json:"active_case_count"`
	ActiveProbability float64 `json:"active_probability"`
	PositiveMean      float64 `json:"positive_mean"`
}

type RetrospectiveProjectHurdlePrior struct {
	ProjectRef string                     `json:"project_ref"`
	Priors     []RetrospectiveHurdlePrior `json:"priors"`
}

type RetrospectiveHurdlePrediction struct {
	Estimate          RetrospectiveMetricEstimate      `json:"estimate"`
	ActiveProbability RetrospectiveActivityProbability `json:"active_probability"`
}

type RetrospectiveActivityProbability struct {
	Spend       float64 `json:"spend"`
	Impressions float64 `json:"impressions"`
	Clicks      float64 `json:"clicks"`
}

type hurdleStats struct {
	count       int
	active      int
	positiveSum float64
}

func retrospectiveMetricActivity(values []MetricWindow) RetrospectiveMetricActivity {
	result := RetrospectiveMetricActivity{ObservedWindows: len(values)}
	for _, value := range values {
		if value.Metrics["spend"] > 0 {
			result.SpendPositiveWindows++
		}
		if value.Metrics["impressions"] > 0 {
			result.ImpressionPositiveWindows++
		}
		if value.Metrics["clicks"] > 0 {
			result.ClickPositiveWindows++
		}
	}
	return result
}

func calibrateAndEvaluateRetrospectiveHurdle(cases []RetrospectiveCalibrationCase, holdoutStart time.Time) (RetrospectiveHurdleCalibration, []RetrospectiveMetricEvaluation, map[string]RetrospectiveHurdlePrediction) {
	const activityStrength = 3.0
	const magnitudeStrength = 3.0
	const minimumPositiveTrainingCases = 10
	training, holdout := make([]RetrospectiveCalibrationCase, 0), make([]RetrospectiveCalibrationCase, 0)
	for _, value := range cases {
		if value.HorizonEnd.Sub(value.PredictionCutoff) != 24*time.Hour {
			return RetrospectiveHurdleCalibration{ModelVersion: RetrospectiveHurdleModelVersion, Status: "unsupported_horizon", MinimumPositiveTrainingCases: minimumPositiveTrainingCases, ActivityPriorStrength: activityStrength, MagnitudePriorStrength: magnitudeStrength}, nil, map[string]RetrospectiveHurdlePrediction{}
		}
		if value.PredictionCutoff.Before(holdoutStart) {
			training = append(training, value)
		} else {
			holdout = append(holdout, value)
		}
	}
	calibration := RetrospectiveHurdleCalibration{ModelVersion: RetrospectiveHurdleModelVersion, Status: "insufficient_cases", TrainingCaseCount: len(training), HoldoutCaseCount: len(holdout), MinimumPositiveTrainingCases: minimumPositiveTrainingCases, ActivityPriorStrength: activityStrength, MagnitudePriorStrength: magnitudeStrength, GlobalPriors: []RetrospectiveHurdlePrior{}, ProjectPriors: []RetrospectiveProjectHurdlePrior{}}
	if len(training) < 30 || len(holdout) < 30 {
		return calibration, nil, map[string]RetrospectiveHurdlePrediction{}
	}
	metrics := []string{"spend_minor", "impressions", "clicks"}
	globalStats := map[string]*hurdleStats{}
	projectStats := map[string]map[string]*hurdleStats{}
	for _, metric := range metrics {
		globalStats[metric] = &hurdleStats{}
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
		value := RetrospectiveProjectHurdlePrior{ProjectRef: projectRef, Priors: []RetrospectiveHurdlePrior{}}
		for _, metric := range metrics {
			global := globalPriors[metric]
			prior := hurdlePrior(metric, *projectStats[projectRef][metric], global.ActiveProbability, global.PositiveMean)
			projectPriors[projectRef][metric] = prior
			value.Priors = append(value.Priors, prior)
		}
		calibration.ProjectPriors = append(calibration.ProjectPriors, value)
	}
	calibration.Status = "evaluated"
	outputScales := map[string]float64{}
	for _, metric := range metrics {
		actualTotal, predictedTotal := 0.0, 0.0
		for _, value := range training {
			actual, _, _ := retrospectiveMetricPair(value, metric)
			prediction, _ := predictHurdleMetric(value, metric, projectPriors, globalPriors, activityStrength, magnitudeStrength)
			actualTotal += actual
			predictedTotal += prediction
		}
		if predictedTotal <= 0 {
			continue
		}
		scale := math.Max(0.25, math.Min(4, actualTotal/predictedTotal))
		outputScales[metric] = scale
		calibration.OutputScales = append(calibration.OutputScales, RetrospectiveCalibrationParameter{Metric: metric, Parameter: "hurdle_output_scale", Value: scale, TrainingCaseCount: len(training), TrainingEnd: holdoutStart})
	}
	predictions := make(map[string]RetrospectiveHurdlePrediction, len(holdout))
	candidateCases := make([]RetrospectiveCalibrationCase, 0, len(holdout))
	for _, value := range holdout {
		prediction := RetrospectiveHurdlePrediction{}
		prediction.Estimate.SpendMinor, prediction.ActiveProbability.Spend = predictHurdleMetric(value, "spend_minor", projectPriors, globalPriors, activityStrength, magnitudeStrength)
		prediction.Estimate.Impressions, prediction.ActiveProbability.Impressions = predictHurdleMetric(value, "impressions", projectPriors, globalPriors, activityStrength, magnitudeStrength)
		prediction.Estimate.Clicks, prediction.ActiveProbability.Clicks = predictHurdleMetric(value, "clicks", projectPriors, globalPriors, activityStrength, magnitudeStrength)
		prediction.Estimate.SpendMinor *= outputScales["spend_minor"]
		prediction.Estimate.Impressions *= outputScales["impressions"]
		prediction.Estimate.Clicks *= outputScales["clicks"]
		predictions[value.CaseID] = prediction
		candidate := value
		candidate.BaselinePrediction = prediction.Estimate
		candidateCases = append(candidateCases, candidate)
	}
	evaluation := evaluateRetrospectiveCases(candidateCases, RetrospectiveHurdleModelVersion, "time_holdout", nil)
	return calibration, evaluation, predictions
}

func globalHurdlePrior(metric string, stats hurdleStats) RetrospectiveHurdlePrior {
	probability := (float64(stats.active) + 1) / (float64(stats.count) + 2)
	positiveMean := 0.0
	if stats.active > 0 {
		positiveMean = stats.positiveSum / float64(stats.active)
	}
	return RetrospectiveHurdlePrior{Metric: metric, TrainingCaseCount: stats.count, ActiveCaseCount: stats.active, ActiveProbability: clampProbability(probability), PositiveMean: math.Max(0, positiveMean)}
}

func addHurdleObservation(stats *hurdleStats, actual float64) {
	stats.count++
	if actual > 0 {
		stats.active++
		stats.positiveSum += actual
	}
}

func hurdlePrior(metric string, stats hurdleStats, fallbackProbability, fallbackPositiveMean float64) RetrospectiveHurdlePrior {
	probability := (float64(stats.active) + 2*fallbackProbability) / (float64(stats.count) + 2)
	positiveMean := fallbackPositiveMean
	if stats.active > 0 || fallbackPositiveMean > 0 {
		positiveMean = (stats.positiveSum + 2*fallbackPositiveMean) / (float64(stats.active) + 2)
	}
	return RetrospectiveHurdlePrior{Metric: metric, TrainingCaseCount: stats.count, ActiveCaseCount: stats.active, ActiveProbability: clampProbability(probability), PositiveMean: math.Max(0, positiveMean)}
}

func predictHurdleMetric(value RetrospectiveCalibrationCase, metric string, projectPriors map[string]map[string]RetrospectiveHurdlePrior, globalPriors map[string]RetrospectiveHurdlePrior, activityStrength, magnitudeStrength float64) (float64, float64) {
	prior := globalPriors[metric]
	if project, ok := projectPriors[value.ProjectRef]; ok {
		if projectPrior, found := project[metric]; found {
			prior = projectPrior
		}
	}
	positiveWindows, historyTotal := hurdleHistory(value, metric)
	observedWindows := value.HistoryActivity.ObservedWindows
	probability := (float64(positiveWindows) + activityStrength*prior.ActiveProbability) / (float64(observedWindows) + activityStrength)
	positiveMean := (historyTotal + magnitudeStrength*prior.PositiveMean) / (float64(positiveWindows) + magnitudeStrength)
	return math.Max(0, clampProbability(probability)*positiveMean), clampProbability(probability)
}

func hurdleHistory(value RetrospectiveCalibrationCase, metric string) (int, float64) {
	switch metric {
	case "spend_minor":
		return value.HistoryActivity.SpendPositiveWindows, float64(value.History.SpendMinor)
	case "impressions":
		return value.HistoryActivity.ImpressionPositiveWindows, float64(value.History.Impressions)
	case "clicks":
		return value.HistoryActivity.ClickPositiveWindows, float64(value.History.Clicks)
	default:
		return 0, 0
	}
}

func clampProbability(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}
