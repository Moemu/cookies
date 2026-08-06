package delivery

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type AlertType string
type AlertStatus string
type AlertAction string
type AlertEvaluationScenario string

const (
	AlertReviewRejected           AlertType               = "review_rejected"
	AlertSpendSpike               AlertType               = "spend_spike"
	AlertZeroConversion           AlertType               = "zero_conversion"
	AlertCostWorsening            AlertType               = "cost_worsening"
	AlertUnderDelivery            AlertType               = "under_delivery"
	AlertCreativeFatigue          AlertType               = "creative_fatigue"
	AlertTrackingAnomaly          AlertType               = "tracking_anomaly"
	AlertOpen                     AlertStatus             = "open"
	AlertAcknowledged             AlertStatus             = "acknowledged"
	AlertDismissed                AlertStatus             = "dismissed"
	AlertAcknowledge              AlertAction             = "acknowledge"
	AlertDismiss                  AlertAction             = "dismiss"
	AlertScenarioNormalDay        AlertEvaluationScenario = "normal_day"
	AlertScenarioAnomalyDay       AlertEvaluationScenario = "anomaly_day"
	AlertScenarioStaleData        AlertEvaluationScenario = "stale_data"
	AlertScenarioInsufficientData AlertEvaluationScenario = "insufficient_data"
)

type DeliveryAlert struct {
	ID               string                  `json:"id"`
	OrganizationID   contract.OrganizationID `json:"organization_id"`
	ProjectID        contract.ProjectID      `json:"project_id"`
	PlanID           string                  `json:"plan_id"`
	ExecutionID      string                  `json:"execution_id"`
	SimulationRunID  string                  `json:"simulation_run_id,omitempty"`
	MonitoredEntity  AlertMonitoredEntity    `json:"monitored_entity"`
	Type             AlertType               `json:"type"`
	RuleID           string                  `json:"rule_id"`
	RuleVersion      string                  `json:"rule_version"`
	Status           AlertStatus             `json:"status"`
	Fingerprint      string                  `json:"fingerprint"`
	Title            string                  `json:"title"`
	Detail           string                  `json:"detail"`
	Severity         string                  `json:"severity"`
	Window           AlertWindow             `json:"window"`
	MetricDefinition AlertMetricDefinition   `json:"metric_definition"`
	Owner            AlertOwner              `json:"owner"`
	EvidenceRefs     []string                `json:"evidence_refs"`
	Source           string                  `json:"source"`
	IsSimulated      bool                    `json:"is_simulated"`
	Scenario         AlertEvaluationScenario `json:"scenario"`
	DatasetVersion   string                  `json:"dataset_version"`
	FixtureVersion   string                  `json:"fixture_version"`
	Freshness        AlertFreshness          `json:"freshness"`
	Version          int64                   `json:"version"`
	CreatedAt        time.Time               `json:"created_at"`
	UpdatedAt        time.Time               `json:"updated_at"`
	CreatedBy        string                  `json:"created_by"`
	AcknowledgedAt   *time.Time              `json:"acknowledged_at"`
	DismissedAt      *time.Time              `json:"dismissed_at"`
	ResolvedBy       string                  `json:"-"`
}
type AlertFreshness struct {
	Status         string    `json:"status"`
	AsOf           time.Time `json:"as_of"`
	EvaluatedAt    time.Time `json:"evaluated_at"`
	AgeSeconds     int64     `json:"age_seconds"`
	MaxAgeSeconds  int64     `json:"max_age_seconds"`
	MissingMetrics []string  `json:"missing_metrics,omitempty"`
}
type AlertMonitoredEntity struct {
	Type         string `json:"type"`
	ID           string `json:"id"`
	AdvertiserID string `json:"advertiser_id"`
}
type AlertWindow struct {
	Start         time.Time  `json:"start"`
	End           time.Time  `json:"end"`
	Timezone      string     `json:"timezone"`
	DataThrough   time.Time  `json:"data_through"`
	BaselineStart *time.Time `json:"baseline_start,omitempty"`
	BaselineEnd   *time.Time `json:"baseline_end,omitempty"`
}
type AlertMetricDefinition struct {
	Name          string   `json:"name"`
	Unit          string   `json:"unit"`
	Numerator     *float64 `json:"numerator,omitempty"`
	Denominator   *float64 `json:"denominator,omitempty"`
	ObservedValue *float64 `json:"observed_value,omitempty"`
	BaselineValue *float64 `json:"baseline_value,omitempty"`
	Threshold     *float64 `json:"threshold,omitempty"`
}
type AlertOwner struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Source      string `json:"source"`
}
type EvaluateAlertsResponse struct {
	Items        []DeliveryAlert         `json:"items"`
	CreatedCount int                     `json:"created_count"`
	ReusedCount  int                     `json:"reused_count"`
	Source       string                  `json:"source"`
	IsSimulated  bool                    `json:"is_simulated"`
	Scenario     AlertEvaluationScenario `json:"scenario"`
	EvaluatedAt  time.Time               `json:"evaluated_at"`
}
type AlertList struct {
	Items       []DeliveryAlert `json:"items"`
	NextCursor  string          `json:"next_cursor,omitempty"`
	Source      string          `json:"source"`
	IsSimulated bool            `json:"is_simulated"`
}
type AlertFilter struct {
	PlanID      string
	ExecutionID string
	Status      AlertStatus
	Type        AlertType
	Severity    string
	Fixture     AlertEvaluationScenario
	Cursor      string
	Limit       int
}
type EvaluateAlertsRequest struct {
	Fixture     AlertEvaluationScenario `json:"fixture"`
	ExecutionID string                  `json:"execution_id,omitempty"`
}
type UpdateAlertRequest struct {
	Action          AlertAction `json:"action"`
	ExpectedVersion int64       `json:"expected_version"`
}

func (r EvaluateAlertsRequest) Validate() error {
	if r.ExecutionID != strings.TrimSpace(r.ExecutionID) {
		return ErrInvalidRequest
	}
	switch r.Fixture {
	case AlertScenarioNormalDay, AlertScenarioAnomalyDay, AlertScenarioStaleData, AlertScenarioInsufficientData:
		return nil
	}
	return ErrInvalidRequest
}
func (r UpdateAlertRequest) Validate() error {
	if r.ExpectedVersion < 1 || (r.Action != AlertAcknowledge && r.Action != AlertDismiss) {
		return ErrInvalidRequest
	}
	return nil
}

func (s Service) EvaluateAlerts(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request EvaluateAlertsRequest) (EvaluateAlertsResponse, error) {
	if err := s.ready(actor, projectID, ScopeWrite); err != nil {
		return EvaluateAlertsResponse{}, err
	}
	if err := request.Validate(); err != nil {
		return EvaluateAlertsResponse{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return EvaluateAlertsResponse{}, err
	}
	now := s.now()
	empty := EvaluateAlertsResponse{Items: []DeliveryAlert{}, Source: MetricSourceDemoFixture, IsSimulated: true, Scenario: request.Fixture, EvaluatedAt: now}
	metrics, err := s.Repository.ListProjectMetricSnapshots(ctx, actor.OrganizationID, projectID, 100)
	if err != nil {
		return EvaluateAlertsResponse{}, err
	}
	if request.ExecutionID != "" {
		scoped := make([]DeliveryMetricSnapshot, 0, len(metrics))
		for _, metric := range metrics {
			if metric.ExecutionID == request.ExecutionID {
				scoped = append(scoped, metric)
			}
		}
		metrics = scoped
	}
	if len(metrics) == 0 {
		return empty, nil
	}
	// When no execution is specified, evaluate only the latest durable execution
	// represented by the newest metric. Never mix windows from different runs.
	if request.ExecutionID == "" {
		latestExecutionID := metrics[0].ExecutionID
		scoped := make([]DeliveryMetricSnapshot, 0, len(metrics))
		for _, metric := range metrics {
			if metric.ExecutionID == latestExecutionID {
				scoped = append(scoped, metric)
			}
		}
		metrics = scoped
	}
	simulationRepository, err := s.outcomeSimulations()
	if err != nil {
		return EvaluateAlertsResponse{}, err
	}
	simulationRun, _, err := simulationRepository.GetLatestOutcomeSimulation(ctx, actor.OrganizationID, projectID, metrics[0].ExecutionID)
	if errors.Is(err, ErrNotFound) {
		return empty, nil
	}
	if err != nil {
		return EvaluateAlertsResponse{}, err
	}
	scopedToRun := make([]DeliveryMetricSnapshot, 0, len(metrics))
	for _, metric := range metrics {
		if metric.SimulationRunID == simulationRun.ID {
			scopedToRun = append(scopedToRun, metric)
		}
	}
	metrics = scopedToRun
	sort.Slice(metrics, func(i, j int) bool { return metrics[i].WindowSequence < metrics[j].WindowSequence })
	if len(metrics) < 2 || request.Fixture == AlertScenarioStaleData || request.Fixture == AlertScenarioInsufficientData {
		return empty, nil
	}
	baseline, current := metrics[0], metrics[len(metrics)-1]
	empty.Source = current.Source
	plan, planErr := s.Repository.GetPlan(ctx, actor.OrganizationID, projectID, current.PlanID)
	if planErr != nil {
		return EvaluateAlertsResponse{}, planErr
	}
	kinds := make([]AlertType, 0, 4)
	if current.RawMetrics.SpendCents >= baseline.RawMetrics.SpendCents*2 {
		kinds = append(kinds, AlertSpendSpike)
	}
	if current.RawMetrics.Clicks >= 100 && current.RawMetrics.Conversions == 0 {
		kinds = append(kinds, AlertZeroConversion)
	}
	baselineCPA := baseline.RawMetrics.SpendCents / maxInt64(1, baseline.RawMetrics.Conversions)
	currentCPA := current.RawMetrics.SpendCents / maxInt64(1, current.RawMetrics.Conversions)
	if currentCPA >= baselineCPA*2 {
		kinds = append(kinds, AlertCostWorsening)
	}
	for _, event := range simulationRun.Events {
		switch event.Type {
		case "review_rejected":
			kinds = appendAlertKind(kinds, AlertReviewRejected)
		case "under_delivery":
			kinds = appendAlertKind(kinds, AlertUnderDelivery)
		case "creative_fatigue":
			kinds = appendAlertKind(kinds, AlertCreativeFatigue)
		case "tracking_anomaly":
			kinds = appendAlertKind(kinds, AlertTrackingAnomaly)
		}
	}
	result := make([]DeliveryAlert, 0, len(kinds))
	for _, kind := range kinds {
		id, idErr := s.idGenerator()("deliveryalert")
		if idErr != nil {
			return EvaluateAlertsResponse{}, idErr
		}
		evidence := []string{"simulation://execution/" + current.ExecutionID, "simulation://run/" + simulationRun.ID, "simulation://metric/" + baseline.ID, "simulation://metric/" + current.ID}
		if kind == AlertReviewRejected {
			evidence = append(evidence, "simulation://platform-event/review-rejected")
		}
		fingerprint, hashErr := alertFingerprint(actor.OrganizationID, projectID, string(kind), "v3", AlertMonitoredEntity{Type: "delivery_plan", ID: current.PlanID, AdvertiserID: plan.CurrentVersion.Advertiser.ID}, current, evidence)
		if hashErr != nil {
			return EvaluateAlertsResponse{}, hashErr
		}
		alert, upsertErr := s.Repository.UpsertAlert(ctx, DeliveryAlert{ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID, PlanID: current.PlanID, ExecutionID: current.ExecutionID, SimulationRunID: simulationRun.ID, MonitoredEntity: AlertMonitoredEntity{Type: "delivery_plan", ID: current.PlanID, AdvertiserID: plan.CurrentVersion.Advertiser.ID}, Type: kind, RuleID: string(kind), RuleVersion: "v3", Status: AlertOpen, Fingerprint: fingerprint, Title: alertTitle(kind), Detail: alertDetail(kind), Severity: alertSeverity(kind), Window: AlertWindow{Start: current.WindowStart, End: current.WindowEnd, Timezone: plan.CurrentVersion.Schedule.Timezone, DataThrough: current.DataThrough, BaselineStart: &baseline.WindowStart, BaselineEnd: &baseline.WindowEnd}, MetricDefinition: ruleMetric(kind, baseline, current), Owner: AlertOwner{Source: "workflow_context"}, EvidenceRefs: evidence, Source: current.Source, IsSimulated: true, Scenario: request.Fixture, DatasetVersion: current.DatasetVersion, FixtureVersion: current.FixtureVersion, Freshness: AlertFreshness{Status: "fresh", AsOf: current.DataThrough, EvaluatedAt: now, AgeSeconds: maxInt64(0, int64(now.Sub(current.DataThrough).Seconds())), MaxAgeSeconds: 86400}, Version: 1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now})
		if upsertErr != nil {
			return EvaluateAlertsResponse{}, upsertErr
		}
		if alert.ID == id {
			empty.CreatedCount++
		} else {
			empty.ReusedCount++
		}
		result = append(result, alert)
	}
	empty.Items = result
	return empty, nil
}
func alertFingerprint(org contract.OrganizationID, project contract.ProjectID, rule, version string, entity AlertMonitoredEntity, m DeliveryMetricSnapshot, evidence []string) (string, error) {
	return contract.CanonicalJSONHash(struct {
		OrganizationID contract.OrganizationID        `json:"organization_id"`
		ProjectID      contract.ProjectID             `json:"project_id"`
		RuleID         string                         `json:"rule_id"`
		RuleVersion    string                         `json:"rule_version"`
		Entity         AlertMonitoredEntity           `json:"entity"`
		Window         struct{ Start, End time.Time } `json:"window"`
		DatasetVersion string                         `json:"dataset_version"`
		FixtureVersion string                         `json:"fixture_version"`
		Evidence       []string                       `json:"evidence"`
	}{org, project, rule, version, entity, struct{ Start, End time.Time }{m.WindowStart, m.WindowEnd}, m.DatasetVersion, m.FixtureVersion, evidence})
}
func ruleMetric(kind AlertType, baseline, current DeliveryMetricSnapshot) AlertMetricDefinition {
	f := func(v int64) *float64 { x := float64(v); return &x }
	switch kind {
	case AlertReviewRejected:
		return AlertMetricDefinition{Name: "review_rejection", Unit: "boolean", ObservedValue: f(1), Threshold: f(1)}
	case AlertSpendSpike:
		return AlertMetricDefinition{Name: "spend_cents", Unit: "CNY_cents", ObservedValue: f(current.RawMetrics.SpendCents), BaselineValue: f(baseline.RawMetrics.SpendCents), Threshold: f(baseline.RawMetrics.SpendCents * 2)}
	case AlertZeroConversion:
		return AlertMetricDefinition{Name: "conversions", Unit: "count", Numerator: f(current.RawMetrics.Conversions), Denominator: f(current.RawMetrics.Clicks), ObservedValue: f(current.RawMetrics.Conversions), Threshold: f(1)}
	case AlertUnderDelivery:
		return AlertMetricDefinition{Name: "spend_cents", Unit: "CNY_cents", ObservedValue: f(current.RawMetrics.SpendCents), BaselineValue: f(baseline.RawMetrics.SpendCents), Threshold: f(baseline.RawMetrics.SpendCents / 2)}
	case AlertCreativeFatigue:
		return AlertMetricDefinition{Name: "click_through_rate", Unit: "ratio", Numerator: f(current.RawMetrics.Clicks), Denominator: f(maxInt64(1, current.RawMetrics.Impressions))}
	case AlertTrackingAnomaly:
		return AlertMetricDefinition{Name: "tracked_conversions", Unit: "count", Numerator: f(current.RawMetrics.Conversions), Denominator: f(current.RawMetrics.Clicks), ObservedValue: f(current.RawMetrics.Conversions), Threshold: f(1)}
	default:
		baselineCPA := baseline.RawMetrics.SpendCents / maxInt64(1, baseline.RawMetrics.Conversions)
		return AlertMetricDefinition{Name: "cpa_cents", Unit: "CNY_cents", Numerator: f(current.RawMetrics.SpendCents), Denominator: f(maxInt64(1, current.RawMetrics.Conversions)), ObservedValue: f(current.RawMetrics.SpendCents / maxInt64(1, current.RawMetrics.Conversions)), BaselineValue: f(baselineCPA), Threshold: f(baselineCPA * 2)}
	}
}

func alertTitle(kind AlertType) string {
	return map[AlertType]string{AlertReviewRejected: "平台审核被拒", AlertSpendSpike: "消耗较基准明显上升", AlertZeroConversion: "有点击但没有转化", AlertCostWorsening: "转化成本较基准恶化", AlertUnderDelivery: "跑量不足", AlertCreativeFatigue: "素材疲劳", AlertTrackingAnomaly: "追踪异常"}[kind]
}

func alertDetail(kind AlertType) string {
	return map[AlertType]string{AlertReviewRejected: "情景模拟记录到平台审核拒绝。", AlertSpendSpike: "当前窗口消耗达到基准窗口的两倍。", AlertZeroConversion: "当前窗口有足量点击但未产生转化。", AlertCostWorsening: "当前窗口转化成本达到基准窗口的两倍。", AlertUnderDelivery: "当前窗口消耗和曝光显著低于基准。", AlertCreativeFatigue: "素材点击率与转化率在连续窗口中衰减。", AlertTrackingAnomaly: "存在点击但追踪到的转化为零。"}[kind]
}

func appendAlertKind(kinds []AlertType, kind AlertType) []AlertType {
	for _, existing := range kinds {
		if existing == kind {
			return kinds
		}
	}
	return append(kinds, kind)
}
func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
func alertSeverity(kind AlertType) string {
	switch kind {
	case AlertReviewRejected, AlertTrackingAnomaly:
		return "critical"
	case AlertSpendSpike, AlertUnderDelivery:
		return "medium"
	default:
		return "high"
	}
}
func metricIDs(values []DeliveryMetricSnapshot) []string {
	ids := make([]string, 0, len(values))
	for _, v := range values {
		ids = append(ids, v.ID)
	}
	return ids
}
func (s Service) ListAlerts(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, filter AlertFilter) ([]DeliveryAlert, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return nil, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		filter.Limit = 50
	}
	return s.Repository.ListAlerts(ctx, actor.OrganizationID, projectID, filter)
}
func (s Service) UpdateAlert(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string, request UpdateAlertRequest) (DeliveryAlert, error) {
	if err := s.ready(actor, projectID, ScopeWrite); err != nil {
		return DeliveryAlert{}, err
	}
	if strings.TrimSpace(id) == "" {
		return DeliveryAlert{}, ErrInvalidRequest
	}
	if err := request.Validate(); err != nil {
		return DeliveryAlert{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return DeliveryAlert{}, err
	}
	return s.Repository.UpdateAlert(ctx, actor.OrganizationID, projectID, id, request.Action, request.ExpectedVersion, actor.Principal.ID, s.now())
}
func alertStatus(action AlertAction) (AlertStatus, error) {
	if action == AlertAcknowledge {
		return AlertAcknowledged, nil
	}
	if action == AlertDismiss {
		return AlertDismissed, nil
	}
	return "", fmt.Errorf("%w: alert action", ErrInvalidRequest)
}
