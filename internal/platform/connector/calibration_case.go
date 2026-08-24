package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	CalibrationCaseSchemaVersion   = "delivery-calibration-case/v1"
	CalibrationExportResultVersion = "delivery-calibration-export-result/v1"
)

type CalibrationCaseExporter struct {
	Reader SnapshotReader
	Key    []byte
	Now    func() time.Time
}

type CalibrationExportRequest struct {
	OrganizationID   string
	ProjectID        string
	AccountRef       string
	PredictionCutoff time.Time
	LabelCutoff      time.Time
	HorizonDays      int
	KeyVersion       string
}

type CalibrationExportResult struct {
	SchemaVersion string                 `json:"schema_version"`
	Audit         CalibrationAuditReport `json:"audit"`
	Cases         []CalibrationCase      `json:"cases"`
}

type CalibrationAuditReport struct {
	DatasetVersion                        string         `json:"dataset_version"`
	PredictionCutoff                      time.Time      `json:"prediction_cutoff"`
	ObjectCounts                          map[string]int `json:"object_counts"`
	PromotionCount                        int            `json:"promotion_count"`
	PromotionCreateTimeCoverageCount      int            `json:"promotion_create_time_coverage_count"`
	PromotionMetricCoverageCount          int            `json:"promotion_metric_coverage_count"`
	PromotionLaunchMetricCoverageCount    int            `json:"promotion_launch_metric_coverage_count"`
	PromotionMetricBeforeCreateCount      int            `json:"promotion_metric_before_create_count"`
	PromotionMetricAfterLaunchWindowCount int            `json:"promotion_metric_after_launch_window_count"`
	ProductLinkedPromotionCount           int            `json:"product_linked_promotion_count"`
	ConfigurationCoverageCount            int            `json:"configuration_coverage_count"`
	ConfigurationFeatureCoverage          map[string]int `json:"configuration_feature_coverage"`
	ConfigurationFeatureDistinctCount     map[string]int `json:"configuration_feature_distinct_count"`
	MaterialBindingCoverageCount          int            `json:"material_binding_coverage_count"`
	MetricWindowCount                     int            `json:"metric_window_count"`
	MetricQualityCounts                   map[string]int `json:"metric_quality_counts"`
	EarliestMetricStart                   *time.Time     `json:"earliest_metric_start"`
	LatestMetricEnd                       *time.Time     `json:"latest_metric_end"`
	ExportedCaseCount                     int            `json:"exported_case_count"`
	SkippedPromotionCounts                map[string]int `json:"skipped_promotion_counts"`
}

type CalibrationCase struct {
	SchemaVersion       string                   `json:"schema_version"`
	CaseID              string                   `json:"case_id"`
	DatasetVersion      string                   `json:"dataset_version"`
	ExportPolicyVersion string                   `json:"export_policy_version"`
	SourceSystem        string                   `json:"source_system"`
	SourceBinding       CalibrationSourceBinding `json:"source_binding"`
	Prediction          CalibrationPrediction    `json:"prediction"`
	Materials           []CalibrationMaterial    `json:"materials"`
	Labels              CalibrationLabels        `json:"labels"`
	Quality             CalibrationCaseQuality   `json:"quality"`
	Anonymization       CalibrationAnonymization `json:"anonymization"`
	Lineage             CalibrationLineage       `json:"lineage"`
}

type CalibrationSourceBinding struct {
	AccountRef         string                 `json:"account_ref"`
	ProductRef         *string                `json:"product_ref"`
	ProjectRef         string                 `json:"project_ref"`
	PromotionRef       string                 `json:"promotion_ref"`
	CookiesPlanBinding CalibrationPlanBinding `json:"cookies_plan_binding"`
}

type CalibrationPlanBinding struct {
	State       string `json:"state"`
	PlanID      any    `json:"plan_id"`
	PlanVersion any    `json:"plan_version"`
}

type CalibrationPrediction struct {
	PredictionCutoff time.Time                `json:"prediction_cutoff"`
	Horizon          string                   `json:"horizon"`
	Configuration    CalibrationConfiguration `json:"configuration"`
}

type CalibrationConfiguration struct {
	SnapshotRef     string              `json:"snapshot_ref"`
	PayloadHash     string              `json:"payload_hash"`
	AvailableAt     time.Time           `json:"available_at"`
	ValidFrom       time.Time           `json:"valid_from"`
	Features        CalibrationFeatures `json:"features"`
	MissingFeatures []string            `json:"missing_features"`
}

type CalibrationFeatures struct {
	BudgetMinor        *int64  `json:"budget_minor"`
	BidMinor           *int64  `json:"bid_minor"`
	Currency           *string `json:"currency"`
	ChargingMode       *string `json:"charging_mode"`
	OptimizationTarget *string `json:"optimization_target"`
	DeliveryMode       *string `json:"delivery_mode"`
	MaterialCount      int     `json:"material_count"`
}

type CalibrationMaterial struct {
	MaterialRef      string    `json:"material_ref"`
	BindingRef       string    `json:"binding_ref"`
	FirstAvailableAt time.Time `json:"first_available_at"`
}

type CalibrationLabels struct {
	LabelCutoff        time.Time                     `json:"label_cutoff"`
	AttributionStatus  string                        `json:"attribution_status"`
	MetricWindows      []CalibrationMetricWindow     `json:"metric_windows"`
	OperationalOutcome CalibrationOperationalOutcome `json:"operational_outcome"`
}

type CalibrationMetricWindow struct {
	WindowRef               string             `json:"window_ref"`
	WindowStart             time.Time          `json:"window_start"`
	WindowEnd               time.Time          `json:"window_end"`
	CollectedAt             time.Time          `json:"collected_at"`
	AvailableAt             time.Time          `json:"available_at"`
	DataThrough             time.Time          `json:"data_through"`
	Granularity             string             `json:"granularity"`
	TimeZone                string             `json:"time_zone"`
	AttributionWindow       string             `json:"attribution_window"`
	MetricDefinitionVersion string             `json:"metric_definition_version"`
	Currency                string             `json:"currency"`
	AmountUnit              string             `json:"amount_unit"`
	Metrics                 map[string]int64   `json:"metrics"`
	QualityStatus           QualityDisposition `json:"quality_status"`
}

type CalibrationOperationalOutcome struct {
	Disposition            string     `json:"disposition"`
	ObservedAt             *time.Time `json:"observed_at"`
	IsMaterialQualityLabel bool       `json:"is_material_quality_label"`
	Interpretation         string     `json:"interpretation"`
}

type CalibrationCaseQuality struct {
	Status   string   `json:"status"`
	Blockers []string `json:"blockers"`
	Warnings []string `json:"warnings"`
}

type CalibrationAnonymization struct {
	Algorithm             string `json:"algorithm"`
	KeyVersion            string `json:"key_version"`
	Scope                 string `json:"scope"`
	RawIdentifiersRemoved bool   `json:"raw_identifiers_removed"`
	FreeTextRemoved       bool   `json:"free_text_removed"`
	DiagnosisRemoved      bool   `json:"diagnosis_removed"`
}

type CalibrationLineage struct {
	FeatureSnapshotRefs []string  `json:"feature_snapshot_refs"`
	LabelSnapshotRefs   []string  `json:"label_snapshot_refs"`
	EvidenceHashes      []string  `json:"evidence_hashes"`
	ExportedAt          time.Time `json:"exported_at"`
}

func (e CalibrationCaseExporter) Export(ctx context.Context, request CalibrationExportRequest) (CalibrationExportResult, error) {
	if e.Reader == nil || len(e.Key) != 32 || request.OrganizationID == "" ||
		!strings.HasPrefix(request.AccountRef, "ref_") || request.PredictionCutoff.IsZero() ||
		request.LabelCutoff.IsZero() || !request.LabelCutoff.After(request.PredictionCutoff) ||
		request.HorizonDays < 1 || strings.TrimSpace(request.KeyVersion) == "" {
		return CalibrationExportResult{}, ErrInvalidFact
	}
	horizonEnd := request.PredictionCutoff.AddDate(0, 0, request.HorizonDays)
	if request.LabelCutoff.Before(horizonEnd) {
		return CalibrationExportResult{}, fmt.Errorf("%w: label cutoff precedes the prediction horizon", ErrInvalidFact)
	}
	featureSnapshot, err := e.Reader.Snapshot(ctx, Query{
		OrganizationID: request.OrganizationID, ProjectID: request.ProjectID,
		SourceRef: request.AccountRef, PredictionCutoff: request.PredictionCutoff,
	})
	if err != nil {
		return CalibrationExportResult{}, err
	}
	labelSnapshot, err := e.Reader.Snapshot(ctx, Query{
		OrganizationID: request.OrganizationID, ProjectID: request.ProjectID,
		SourceRef: request.AccountRef, PredictionCutoff: request.LabelCutoff,
		WindowStart: request.PredictionCutoff, WindowEnd: horizonEnd,
	})
	if err != nil {
		return CalibrationExportResult{}, err
	}
	now := time.Now().UTC()
	if e.Now != nil {
		now = e.Now().UTC()
	}
	result := CalibrationExportResult{
		SchemaVersion: CalibrationExportResultVersion,
		Audit:         AuditCalibrationSnapshot(labelSnapshot),
		Cases:         []CalibrationCase{},
	}
	promotions := latestObjects(featureSnapshot.Objects, "promotion")
	configs := latestConfigurations(featureSnapshot.Configurations)
	bindings := activeBindings(featureSnapshot.Bindings, request.PredictionCutoff)
	metrics := latestMetrics(labelSnapshot.Metrics, request.PredictionCutoff, horizonEnd)
	statuses := latestStatuses(labelSnapshot.Statuses)
	accountRef, err := CalibrationExportRef(e.Key, "account", request.AccountRef)
	if err != nil {
		return CalibrationExportResult{}, err
	}
	for _, promotion := range promotions {
		if promotion.ParentRef == "" {
			result.Audit.SkippedPromotionCounts["project_ref_missing"]++
			continue
		}
		configuration, ok := configs[promotion.ObjectRef]
		if !ok {
			result.Audit.SkippedPromotionCounts["configuration_missing"]++
			continue
		}
		promotionMetrics := metrics[promotion.ObjectRef]
		if len(promotionMetrics) == 0 {
			result.Audit.SkippedPromotionCounts["metric_window_missing"]++
			continue
		}
		caseValue, buildErr := e.buildCase(request, now, accountRef, promotion, configuration, bindings[promotion.ObjectRef], promotionMetrics, statuses[promotion.ObjectRef])
		if buildErr != nil {
			return CalibrationExportResult{}, buildErr
		}
		result.Cases = append(result.Cases, caseValue)
	}
	sort.Slice(result.Cases, func(i, j int) bool { return result.Cases[i].CaseID < result.Cases[j].CaseID })
	result.Audit.ExportedCaseCount = len(result.Cases)
	return result, nil
}

func (e CalibrationCaseExporter) buildCase(request CalibrationExportRequest, exportedAt time.Time, accountRef string, promotion ObjectSnapshot, configuration ConfigurationSnapshot, bindings []MaterialBinding, metrics []MetricWindow, status PlatformStatusEvent) (CalibrationCase, error) {
	promotionRef, err := CalibrationExportRef(e.Key, "promotion", promotion.ObjectRef)
	if err != nil {
		return CalibrationCase{}, err
	}
	projectRef, err := CalibrationExportRef(e.Key, "project", promotion.ParentRef)
	if err != nil {
		return CalibrationCase{}, err
	}
	var productRef *string
	productConnectorRef, _ := promotion.State["product_ref"].(string)
	if strings.HasPrefix(productConnectorRef, "ref_") {
		value, refErr := CalibrationExportRef(e.Key, "product", productConnectorRef)
		if refErr != nil {
			return CalibrationCase{}, refErr
		}
		productRef = &value
	}
	configurationRef, err := CalibrationExportRef(e.Key, "configuration", AnonymizeRef(configuration.ID))
	if err != nil {
		return CalibrationCase{}, err
	}
	features, missing := calibrationFeatures(configuration.Values, len(bindings))
	blockers := append([]string{}, missing...)
	if productRef == nil {
		blockers = append(blockers, "product_ref_missing")
	}
	if len(bindings) == 0 {
		blockers = append(blockers, "material_binding_missing")
	}
	materials := make([]CalibrationMaterial, 0, len(bindings))
	featureRefs := []string{configurationRef}
	evidenceHashes := []string{configuration.PayloadHash}
	for _, binding := range bindings {
		materialRef, refErr := CalibrationExportRef(e.Key, "material", binding.MaterialRef)
		if refErr != nil {
			return CalibrationCase{}, refErr
		}
		bindingRef, refErr := CalibrationExportRef(e.Key, "material_binding", AnonymizeRef(binding.ID))
		if refErr != nil {
			return CalibrationCase{}, refErr
		}
		materials = append(materials, CalibrationMaterial{MaterialRef: materialRef, BindingRef: bindingRef, FirstAvailableAt: binding.AvailableAt})
		featureRefs = append(featureRefs, bindingRef)
		evidenceHashes = append(evidenceHashes, binding.PayloadHash)
	}
	metricWindows := make([]CalibrationMetricWindow, 0, len(metrics))
	labelRefs := make([]string, 0, len(metrics))
	attributionStatus := "mature"
	for _, metric := range metrics {
		windowRef, refErr := CalibrationExportRef(e.Key, "metric_window", AnonymizeRef(metric.ID))
		if refErr != nil {
			return CalibrationCase{}, refErr
		}
		atomic, complete := atomicMetrics(metric.Metrics)
		if !complete {
			return CalibrationCase{}, fmt.Errorf("%w: metric window lacks one atomic metric", ErrInvalidFact)
		}
		if metric.QualityStatus == QualityQuarantine {
			attributionStatus = "immature"
			blockers = append(blockers, "metric_window_quarantined")
		}
		metricWindows = append(metricWindows, CalibrationMetricWindow{
			WindowRef: windowRef, WindowStart: metric.WindowStart, WindowEnd: metric.WindowEnd,
			CollectedAt: metric.CollectedAt, AvailableAt: metric.AvailableAt, DataThrough: metric.DataThrough,
			Granularity: metric.Granularity, TimeZone: metric.TimeZone, AttributionWindow: metric.AttributionWindow,
			MetricDefinitionVersion: metric.MetricDefinitionVersion, Currency: metric.Currency,
			AmountUnit: metric.AmountUnit, Metrics: atomic, QualityStatus: metric.QualityStatus,
		})
		labelRefs = append(labelRefs, windowRef)
		evidenceHashes = append(evidenceHashes, metric.PayloadHash)
	}
	blockers = uniqueSorted(blockers)
	featureRefs = uniqueSorted(featureRefs)
	labelRefs = uniqueSorted(labelRefs)
	evidenceHashes = uniqueSorted(evidenceHashes)
	disposition, observedAt := calibrationDisposition(status)
	caseID := "calcase_" + canonicalHash([]any{accountRef, productRef, projectRef, promotionRef, request.PredictionCutoff, request.LabelCutoff, request.HorizonDays, configuration.PayloadHash, labelRefs})
	qualityStatus := "accepted"
	if len(blockers) > 0 {
		qualityStatus = "quarantined"
	}
	return CalibrationCase{
		SchemaVersion: CalibrationCaseSchemaVersion, CaseID: caseID, DatasetVersion: DatasetVersion,
		ExportPolicyVersion: CalibrationExportPolicyVersion, SourceSystem: SourceSystem,
		SourceBinding: CalibrationSourceBinding{AccountRef: accountRef, ProductRef: productRef, ProjectRef: projectRef, PromotionRef: promotionRef, CookiesPlanBinding: CalibrationPlanBinding{State: "unbound_historical", PlanID: nil, PlanVersion: nil}},
		Prediction:    CalibrationPrediction{PredictionCutoff: request.PredictionCutoff, Horizon: fmt.Sprintf("P%dD", request.HorizonDays), Configuration: CalibrationConfiguration{SnapshotRef: configurationRef, PayloadHash: configuration.PayloadHash, AvailableAt: configuration.AvailableAt, ValidFrom: configuration.ValidFrom, Features: features, MissingFeatures: missing}},
		Materials:     materials,
		Labels:        CalibrationLabels{LabelCutoff: request.LabelCutoff, AttributionStatus: attributionStatus, MetricWindows: metricWindows, OperationalOutcome: CalibrationOperationalOutcome{Disposition: disposition, ObservedAt: observedAt, IsMaterialQualityLabel: false, Interpretation: "operator_and_platform_outcome_only"}},
		Quality:       CalibrationCaseQuality{Status: qualityStatus, Blockers: blockers, Warnings: []string{}},
		Anonymization: CalibrationAnonymization{Algorithm: "HMAC-SHA256", KeyVersion: request.KeyVersion, Scope: "stable_within_key_version", RawIdentifiersRemoved: true, FreeTextRemoved: true, DiagnosisRemoved: true},
		Lineage:       CalibrationLineage{FeatureSnapshotRefs: featureRefs, LabelSnapshotRefs: labelRefs, EvidenceHashes: evidenceHashes, ExportedAt: exportedAt},
	}, nil
}

func AuditCalibrationSnapshot(snapshot CanonicalSnapshot) CalibrationAuditReport {
	report := CalibrationAuditReport{DatasetVersion: snapshot.DatasetVersion, PredictionCutoff: snapshot.PredictionCutoff, ObjectCounts: map[string]int{}, ConfigurationFeatureCoverage: map[string]int{}, ConfigurationFeatureDistinctCount: map[string]int{}, MetricQualityCounts: map[string]int{}, SkippedPromotionCounts: map[string]int{}}
	promotions := latestObjects(snapshot.Objects, "promotion")
	configs := latestConfigurations(snapshot.Configurations)
	metricsByPromotion := latestMetrics(snapshot.Metrics, time.Time{}, snapshot.PredictionCutoff)
	featureValues := map[string]map[string]struct{}{}
	bindings := activeBindings(snapshot.Bindings, snapshot.PredictionCutoff)
	for _, object := range snapshot.Objects {
		report.ObjectCounts[object.ObjectKind]++
	}
	for _, promotion := range promotions {
		report.PromotionCount++
		createTime, hasCreateTime := calibrationObjectCreateTime(promotion.State)
		if hasCreateTime {
			report.PromotionCreateTimeCoverageCount++
		}
		promotionMetrics := metricsByPromotion[promotion.ObjectRef]
		if len(promotionMetrics) > 0 {
			report.PromotionMetricCoverageCount++
			if hasCreateTime {
				firstMetric := promotionMetrics[0].WindowStart
				metricDay := calibrationPlatformDay(firstMetric)
				createDay := calibrationPlatformDay(createTime)
				if metricDay.Before(createDay) {
					report.PromotionMetricBeforeCreateCount++
				} else if metricDay.Sub(createDay) <= 7*24*time.Hour {
					report.PromotionLaunchMetricCoverageCount++
				} else {
					report.PromotionMetricAfterLaunchWindowCount++
				}
			}
		}
		if value, ok := promotion.State["product_ref"].(string); ok && strings.HasPrefix(value, "ref_") {
			report.ProductLinkedPromotionCount++
		}
		if _, ok := configs[promotion.ObjectRef]; ok {
			report.ConfigurationCoverageCount++
			features, _ := calibrationFeatures(configs[promotion.ObjectRef].Values, 0)
			appendCalibrationAuditFeature(featureValues, report.ConfigurationFeatureCoverage, "budget_minor", features.BudgetMinor)
			appendCalibrationAuditFeature(featureValues, report.ConfigurationFeatureCoverage, "bid_minor", features.BidMinor)
			appendCalibrationAuditFeature(featureValues, report.ConfigurationFeatureCoverage, "currency", features.Currency)
			appendCalibrationAuditFeature(featureValues, report.ConfigurationFeatureCoverage, "charging_mode", features.ChargingMode)
			appendCalibrationAuditFeature(featureValues, report.ConfigurationFeatureCoverage, "optimization_target", features.OptimizationTarget)
			appendCalibrationAuditFeature(featureValues, report.ConfigurationFeatureCoverage, "delivery_mode", features.DeliveryMode)
		}
		if len(bindings[promotion.ObjectRef]) > 0 {
			report.MaterialBindingCoverageCount++
		}
	}
	for _, metric := range snapshot.Metrics {
		report.MetricWindowCount++
		report.MetricQualityCounts[string(metric.QualityStatus)]++
		if report.EarliestMetricStart == nil || metric.WindowStart.Before(*report.EarliestMetricStart) {
			value := metric.WindowStart
			report.EarliestMetricStart = &value
		}
		if report.LatestMetricEnd == nil || metric.WindowEnd.After(*report.LatestMetricEnd) {
			value := metric.WindowEnd
			report.LatestMetricEnd = &value
		}
	}
	for name, values := range featureValues {
		report.ConfigurationFeatureDistinctCount[name] = len(values)
	}
	return report
}

func calibrationPlatformDay(value time.Time) time.Time {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

func calibrationObjectCreateTime(values map[string]any) (time.Time, bool) {
	for _, key := range []string{"promotion_create_time", "create_time"} {
		raw, ok := values[key].(string)
		if !ok {
			continue
		}
		if value, ok := parseCalibrationPlatformTime(raw); ok {
			return value, true
		}
	}
	for _, value := range values {
		nested, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if result, found := calibrationObjectCreateTime(nested); found {
			return result, true
		}
	}
	return time.Time{}, false
}

func parseCalibrationPlatformTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if value, err := time.Parse(time.RFC3339, raw); err == nil {
		return value.UTC(), true
	}
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	if value, err := time.ParseInLocation("2006-01-02 15:04:05", raw, location); err == nil {
		return value.UTC(), true
	}
	return time.Time{}, false
}

func appendCalibrationAuditFeature[T comparable](values map[string]map[string]struct{}, coverage map[string]int, name string, value *T) {
	if value == nil {
		return
	}
	coverage[name]++
	if values[name] == nil {
		values[name] = map[string]struct{}{}
	}
	values[name][fmt.Sprint(*value)] = struct{}{}
}

func latestObjects(values []ObjectSnapshot, kind string) []ObjectSnapshot {
	latest := map[string]ObjectSnapshot{}
	for _, value := range values {
		if value.ObjectKind != kind {
			continue
		}
		current, ok := latest[value.ObjectRef]
		if !ok || value.AvailableAt.After(current.AvailableAt) || value.AvailableAt.Equal(current.AvailableAt) && value.ID > current.ID {
			latest[value.ObjectRef] = value
		}
	}
	result := make([]ObjectSnapshot, 0, len(latest))
	for _, value := range latest {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ObjectRef < result[j].ObjectRef })
	return result
}

func latestConfigurations(values []ConfigurationSnapshot) map[string]ConfigurationSnapshot {
	result := map[string]ConfigurationSnapshot{}
	for _, value := range values {
		current, ok := result[value.ObjectRef]
		if !ok || value.AvailableAt.After(current.AvailableAt) || value.AvailableAt.Equal(current.AvailableAt) && value.ID > current.ID {
			result[value.ObjectRef] = value
		}
	}
	return result
}

func activeBindings(values []MaterialBinding, cutoff time.Time) map[string][]MaterialBinding {
	latest := map[string]MaterialBinding{}
	for _, value := range values {
		if value.ValidFrom.After(cutoff) || value.ValidTo != nil && !value.ValidTo.After(cutoff) {
			continue
		}
		key := value.PromotionRef + "\x00" + value.MaterialRef
		current, ok := latest[key]
		if !ok || value.AvailableAt.Before(current.AvailableAt) {
			latest[key] = value
		}
	}
	result := map[string][]MaterialBinding{}
	for _, value := range latest {
		result[value.PromotionRef] = append(result[value.PromotionRef], value)
	}
	for key := range result {
		sort.Slice(result[key], func(i, j int) bool { return result[key][i].MaterialRef < result[key][j].MaterialRef })
	}
	return result
}

func latestMetrics(values []MetricWindow, start, end time.Time) map[string][]MetricWindow {
	latest := map[string]MetricWindow{}
	for _, value := range values {
		if value.WindowStart.Before(start) || value.WindowEnd.After(end) || !value.WindowEnd.After(value.WindowStart) {
			continue
		}
		key := value.ObjectRef + "\x00" + value.WindowStart.Format(time.RFC3339Nano) + "\x00" + value.WindowEnd.Format(time.RFC3339Nano) + "\x00" + value.AttributionWindow + "\x00" + value.MetricDefinitionVersion
		current, ok := latest[key]
		if !ok || value.AvailableAt.After(current.AvailableAt) || value.AvailableAt.Equal(current.AvailableAt) && value.ID > current.ID {
			latest[key] = value
		}
	}
	result := map[string][]MetricWindow{}
	for _, value := range latest {
		result[value.ObjectRef] = append(result[value.ObjectRef], value)
	}
	for key := range result {
		sort.Slice(result[key], func(i, j int) bool { return result[key][i].WindowStart.Before(result[key][j].WindowStart) })
	}
	return result
}

func latestStatuses(values []PlatformStatusEvent) map[string]PlatformStatusEvent {
	result := map[string]PlatformStatusEvent{}
	for _, value := range values {
		current, ok := result[value.ObjectRef]
		if !ok || value.AvailableAt.After(current.AvailableAt) || value.AvailableAt.Equal(current.AvailableAt) && value.ID > current.ID {
			result[value.ObjectRef] = value
		}
	}
	return result
}

func calibrationFeatures(values map[string]any, materialCount int) (CalibrationFeatures, []string) {
	budget := currencyMinor(values, []string{"budget_minor"}, []string{"campaign_budget", "budget_amount", "budget"})
	bid := currencyMinor(values, []string{"bid_minor"}, []string{"project_bid", "ad_bid", "deep_cpa_bid", "bid"})
	currency := controlledString(values, "currency")
	charging := controlledString(values, "charging_mode", "ad_pricing_name", "ad_pricing")
	optimization := controlledString(values, "optimization_target", "external_action_name", "external_action")
	delivery := controlledString(values, "delivery_mode", "delivery_scene_name")
	missing := []string{}
	for name, value := range map[string]any{"budget_minor": budget, "bid_minor": bid, "currency": currency, "charging_mode": charging, "optimization_target": optimization, "delivery_mode": delivery} {
		if value == nil || isNilPointer(value) {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return CalibrationFeatures{BudgetMinor: budget, BidMinor: bid, Currency: currency, ChargingMode: charging, OptimizationTarget: optimization, DeliveryMode: delivery, MaterialCount: materialCount}, missing
}

func currencyMinor(values map[string]any, minorKeys, majorKeys []string) *int64 {
	for _, key := range minorKeys {
		if value, ok := numericValue(lookupConfigurationValue(values, key)); ok && value >= 0 {
			result := int64(math.Round(value))
			return &result
		}
	}
	for _, key := range majorKeys {
		if value, ok := numericValue(lookupConfigurationValue(values, key)); ok && value >= 0 {
			result := int64(math.Round(value * 100))
			return &result
		}
	}
	return nil
}

func numericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		trimmed := strings.ReplaceAll(strings.TrimSpace(typed), ",", "")
		if trimmed == "" || trimmed == "-" || trimmed == "--" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(trimmed, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func controlledString(values map[string]any, keys ...string) *string {
	for _, key := range keys {
		value := lookupConfigurationValue(values, key)
		if value == nil {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" || text == "-" || text == "--" || containsSensitiveText(text) {
			continue
		}
		return &text
	}
	return nil
}

func lookupConfigurationValue(values map[string]any, key string) any {
	if value, ok := values[key]; ok {
		return value
	}
	keys := make([]string, 0, len(values))
	for nestedKey := range values {
		keys = append(keys, nestedKey)
	}
	sort.Strings(keys)
	for _, nestedKey := range keys {
		switch nested := values[nestedKey].(type) {
		case map[string]any:
			if value := lookupConfigurationValue(nested, key); value != nil {
				return value
			}
		case []any:
			for _, item := range nested {
				if mapped, ok := item.(map[string]any); ok {
					if value := lookupConfigurationValue(mapped, key); value != nil {
						return value
					}
				}
			}
		}
	}
	return nil
}

func isNilPointer(value any) bool {
	switch typed := value.(type) {
	case *int64:
		return typed == nil
	case *string:
		return typed == nil
	default:
		return value == nil
	}
}

func atomicMetrics(values map[string]int64) (map[string]int64, bool) {
	result := map[string]int64{}
	for _, key := range []string{"spend", "impressions", "clicks", "conversions"} {
		value, ok := values[key]
		if !ok || value < 0 {
			return nil, false
		}
		result[key] = value
	}
	return result, true
}

func calibrationDisposition(value PlatformStatusEvent) (string, *time.Time) {
	if value.ID == "" {
		return "unknown", nil
	}
	lower := strings.ToLower(value.Status)
	disposition := "unknown"
	switch {
	case strings.Contains(lower, "delete") || strings.Contains(lower, "remove"):
		disposition = "deleted"
	case strings.Contains(lower, "pause") || strings.Contains(lower, "stop") || strings.Contains(lower, "disable"):
		disposition = "paused"
	case strings.Contains(lower, "active") || strings.Contains(lower, "deliver") || strings.Contains(lower, "enable"):
		disposition = "retained"
	}
	observedAt := value.AvailableAt
	return disposition, &observedAt
}

func uniqueSorted(values []string) []string {
	set := map[string]struct{}{}
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			set[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
