package delivery

import (
	"context"
	"fmt"
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
	Status   AlertStatus
	Type     AlertType
	Severity string
	Fixture  AlertEvaluationScenario
	Cursor   string
	Limit    int
}
type EvaluateAlertsRequest struct {
	Fixture AlertEvaluationScenario `json:"fixture"`
}
type UpdateAlertRequest struct {
	Action          AlertAction `json:"action"`
	ExpectedVersion int64       `json:"expected_version"`
}

func (r EvaluateAlertsRequest) Validate() error {
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
	if len(metrics) == 0 {
		executions, executionErr := s.Repository.ListExecutions(ctx, actor.OrganizationID, projectID, 1)
		if executionErr != nil {
			return EvaluateAlertsResponse{}, executionErr
		}
		if len(executions) == 0 {
			return empty, nil
		} // No durable scope exists; never manufacture an FK target.
		plan, planErr := s.Repository.GetPlan(ctx, actor.OrganizationID, projectID, executions[0].ChangeSet.PlanID)
		if planErr != nil {
			return EvaluateAlertsResponse{}, planErr
		}
		metrics = []DeliveryMetricSnapshot{{OrganizationID: actor.OrganizationID, ProjectID: projectID, ExecutionID: executions[0].Execution.ID, PlanID: plan.ID, CreativePackageID: plan.CreativePackageID, WindowStart: plan.StartAt, WindowEnd: plan.EndAt, DataThrough: plan.EndAt}}
	}
	seed := metrics[0]
	fixtureMetrics, err := s.persistMonitoringFixture(ctx, actor, projectID, seed, request.Fixture, now)
	if err != nil {
		return EvaluateAlertsResponse{}, err
	}
	metrics = fixtureMetrics
	// Stale and insufficient fixtures are retained as evidence but never manufacture alerts.
	if request.Fixture != AlertScenarioAnomalyDay {
		return empty, nil
	}
	result := make([]DeliveryAlert, 0, 4)
	// The second immutable window is the current observation. The first is its
	// fixed baseline, so the anomaly rules must never evaluate baseline values
	// as if they were the triggering evidence.
	m := metrics[len(metrics)-1]
	plan, planErr := s.Repository.GetPlan(ctx, actor.OrganizationID, projectID, m.PlanID)
	if planErr != nil {
		return EvaluateAlertsResponse{}, planErr
	}
	for _, kind := range []AlertType{AlertReviewRejected, AlertSpendSpike, AlertZeroConversion, AlertCostWorsening} {
		id, idErr := s.idGenerator()("deliveryalert")
		if idErr != nil {
			return EvaluateAlertsResponse{}, idErr
		}
		evidence := []string{"mock://metric/" + m.ID, "mock://fixture/anomaly_day/" + string(kind)}
		fingerprint, hashErr := alertFingerprint(actor.OrganizationID, projectID, string(kind), "v1", AlertMonitoredEntity{Type: "delivery_plan", ID: m.PlanID, AdvertiserID: plan.CurrentVersion.Advertiser.ID}, m, evidence)
		if hashErr != nil {
			return EvaluateAlertsResponse{}, hashErr
		}
		alert, upsertErr := s.Repository.UpsertAlert(ctx, DeliveryAlert{ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID, PlanID: m.PlanID, ExecutionID: m.ExecutionID, MonitoredEntity: AlertMonitoredEntity{Type: "delivery_plan", ID: m.PlanID, AdvertiserID: plan.CurrentVersion.Advertiser.ID}, Type: kind, RuleID: string(kind), RuleVersion: "v1", Status: AlertOpen, Fingerprint: fingerprint, Title: string(kind), Detail: "mock anomaly fixture", Severity: alertSeverity(kind), Window: AlertWindow{Start: m.WindowStart, End: m.WindowEnd, Timezone: plan.CurrentVersion.Schedule.Timezone, DataThrough: m.DataThrough}, MetricDefinition: ruleMetric(kind, m), Owner: AlertOwner{ID: actor.Principal.ID, DisplayName: actor.Principal.ID, Source: "actor_context"}, EvidenceRefs: evidence, Source: MetricSourceDemoFixture, IsSimulated: true, Scenario: request.Fixture, DatasetVersion: m.DatasetVersion, FixtureVersion: m.FixtureVersion, Freshness: AlertFreshness{Status: "fresh", AsOf: m.DataThrough, EvaluatedAt: now, MaxAgeSeconds: 86400}, Version: 1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now})
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
func (s Service) persistMonitoringFixture(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, seed DeliveryMetricSnapshot, fixture AlertEvaluationScenario, now time.Time) ([]DeliveryMetricSnapshot, error) {
	plan, err := s.Repository.GetPlan(ctx, actor.OrganizationID, projectID, seed.PlanID)
	if err != nil {
		return nil, err
	}
	baseStart := plan.StartAt
	if baseStart.IsZero() {
		baseStart = now.Add(-48 * time.Hour)
	}
	windows := []struct {
		sequence   int
		start, end time.Time
		metrics    RawMetrics
	}{{1, baseStart, baseStart.Add(24 * time.Hour), RawMetrics{Impressions: 10000, Clicks: 500, Conversions: 20, SpendCents: 20000}}, {2, baseStart.Add(24 * time.Hour), baseStart.Add(48 * time.Hour), RawMetrics{Impressions: 10000, Clicks: 500, Conversions: 20, SpendCents: 20000}}}
	if fixture == AlertScenarioAnomalyDay {
		windows[1].metrics = RawMetrics{Impressions: 10000, Clicks: 400, Conversions: 0, SpendCents: 60000}
	}
	if fixture == AlertScenarioInsufficientData {
		windows[1].metrics = RawMetrics{Impressions: 10, Clicks: 1, Conversions: 0, SpendCents: 20}
	}
	values := make([]DeliveryMetricSnapshot, 0, 2)
	for _, w := range windows {
		id, err := s.idGenerator()("deliverymetric")
		if err != nil {
			return nil, err
		}
		through := w.end
		if fixture == AlertScenarioStaleData {
			through = w.end.Add(-72 * time.Hour)
		}
		v := DeliveryMetricSnapshot{ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID, ExecutionID: seed.ExecutionID, PlanID: seed.PlanID, CreativePackageID: seed.CreativePackageID, Source: MetricSourceDemoFixture, IsSimulated: true, DatasetVersion: DemoMetricDatasetVersion, FixtureVersion: string(fixture) + "/v1", WindowSequence: w.sequence, Currency: "CNY", WindowStart: w.start, WindowEnd: w.end, DataThrough: through, RawMetrics: w.metrics, CreatedBy: actor.Principal.ID, CreatedAt: now}
		stored, _, err := s.Repository.CreateMetricSnapshot(ctx, v)
		if err != nil {
			return nil, err
		}
		values = append(values, stored)
	}
	return values, nil
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
func ruleMetric(kind AlertType, m DeliveryMetricSnapshot) AlertMetricDefinition {
	f := func(v int64) *float64 { x := float64(v); return &x }
	switch kind {
	case AlertReviewRejected:
		return AlertMetricDefinition{Name: "review_rejection", Unit: "boolean", ObservedValue: f(1), Threshold: f(1)}
	case AlertSpendSpike:
		return AlertMetricDefinition{Name: "spend_cents", Unit: "CNY_cents", ObservedValue: f(m.RawMetrics.SpendCents), BaselineValue: f(20000), Threshold: f(30000)}
	case AlertZeroConversion:
		return AlertMetricDefinition{Name: "conversions", Unit: "count", Numerator: f(m.RawMetrics.Conversions), Denominator: f(m.RawMetrics.Clicks), ObservedValue: f(m.RawMetrics.Conversions), Threshold: f(1)}
	default:
		return AlertMetricDefinition{Name: "cpa_cents", Unit: "CNY_cents", Numerator: f(m.RawMetrics.SpendCents), Denominator: f(maxInt64(1, m.RawMetrics.Conversions)), ObservedValue: f(m.RawMetrics.SpendCents / maxInt64(1, m.RawMetrics.Conversions)), BaselineValue: f(1000), Threshold: f(1500)}
	}
}
func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
func alertSeverity(kind AlertType) string {
	switch kind {
	case AlertReviewRejected:
		return "critical"
	case AlertSpendSpike:
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
