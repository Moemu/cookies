package delivery

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/connector"
	"github.com/shikanon/cookies/internal/platform/contract"
)

const connectorAlertRuleVersion = "connector-v1"

type ConnectorInspectionRequest struct {
	PlanID     string `json:"plan_id"`
	WindowDays int    `json:"window_days,omitempty"`
}

func (r *ConnectorInspectionRequest) normalize() error {
	r.PlanID = strings.TrimSpace(r.PlanID)
	if r.PlanID == "" {
		return ErrInvalidRequest
	}
	if r.WindowDays == 0 {
		r.WindowDays = 14
	}
	if r.WindowDays < 2 || r.WindowDays > 90 {
		return ErrInvalidRequest
	}
	return nil
}

type ConnectorInspectionResponse struct {
	Items          []DeliveryAlert `json:"items"`
	CreatedCount   int             `json:"created_count"`
	ReusedCount    int             `json:"reused_count"`
	Source         string          `json:"source"`
	IsSimulated    bool            `json:"is_simulated"`
	Status         string          `json:"status"`
	StatusReason   string          `json:"status_reason,omitempty"`
	DatasetVersion string          `json:"dataset_version"`
	EvaluatedAt    time.Time       `json:"evaluated_at"`
	DataThrough    *time.Time      `json:"data_through,omitempty"`
	EvidenceRefs   []string        `json:"evidence_refs"`
}

type connectorAlertCandidate struct {
	kind     AlertType
	entity   AlertMonitoredEntity
	baseline connector.MetricWindow
	current  connector.MetricWindow
	metric   AlertMetricDefinition
}

func (s Service) InspectConnectorAlerts(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request ConnectorInspectionRequest) (ConnectorInspectionResponse, error) {
	if err := s.ready(actor, projectID, ScopeWrite); err != nil {
		return ConnectorInspectionResponse{}, err
	}
	if err := request.normalize(); err != nil {
		return ConnectorInspectionResponse{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return ConnectorInspectionResponse{}, err
	}
	plan, err := s.Repository.GetPlan(ctx, actor.OrganizationID, projectID, request.PlanID)
	if err != nil {
		return ConnectorInspectionResponse{}, err
	}
	now := s.now()
	result := ConnectorInspectionResponse{Items: []DeliveryAlert{}, Source: "connector", Status: "unavailable", DatasetVersion: connector.DatasetVersion, EvaluatedAt: now, EvidenceRefs: []string{}}
	if s.ConnectorSnapshots == nil {
		result.StatusReason = "Connector snapshot reader is not configured."
		return result, nil
	}
	accountID := strings.TrimSpace(plan.CurrentVersion.Advertiser.ID)
	if accountID == "" {
		result.Status = "insufficient_data"
		result.StatusReason = "The plan does not contain an advertiser account reference."
		return result, nil
	}
	snapshot, err := s.ConnectorSnapshots.Snapshot(ctx, connector.Query{
		OrganizationID: string(actor.OrganizationID), ProjectID: string(projectID),
		SourceRef: connector.AnonymizeRef(accountID), WindowStart: now.AddDate(0, 0, -request.WindowDays),
		WindowEnd: now, PredictionCutoff: now, IncludeDiagnosis: true,
	})
	if err != nil {
		return ConnectorInspectionResponse{}, err
	}
	result.DatasetVersion = snapshot.DatasetVersion
	status, reason, dataThrough, evidence, candidates := inspectConnectorSnapshot(snapshot, now)
	result.Status, result.StatusReason, result.EvidenceRefs = status, reason, evidence
	if !dataThrough.IsZero() {
		result.DataThrough = &dataThrough
	}
	if status != "ready" {
		return result, nil
	}
	for _, candidate := range candidates {
		id, idErr := s.idGenerator()("deliveryalert")
		if idErr != nil {
			return ConnectorInspectionResponse{}, idErr
		}
		evidenceRefs := []string{candidate.current.EvidenceRef, candidate.baseline.EvidenceRef}
		fingerprint, hashErr := connectorAlertFingerprint(actor.OrganizationID, projectID, request.PlanID, candidate, evidenceRefs)
		if hashErr != nil {
			return ConnectorInspectionResponse{}, hashErr
		}
		freshness := AlertFreshness{Status: "fresh", AsOf: candidate.current.DataThrough, EvaluatedAt: now, AgeSeconds: maxInt64(0, int64(now.Sub(candidate.current.DataThrough).Seconds())), MaxAgeSeconds: 172800}
		alert, upsertErr := s.Repository.UpsertAlert(ctx, DeliveryAlert{
			ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID, PlanID: request.PlanID,
			MonitoredEntity: candidate.entity, Type: candidate.kind, RuleID: string(candidate.kind), RuleVersion: connectorAlertRuleVersion,
			Status: AlertOpen, Fingerprint: fingerprint, Title: alertTitle(candidate.kind), Detail: connectorAlertDetail(candidate.kind), Severity: alertSeverity(candidate.kind),
			Window:           AlertWindow{Start: candidate.current.WindowStart, End: candidate.current.WindowEnd, Timezone: candidate.current.TimeZone, DataThrough: candidate.current.DataThrough, BaselineStart: &candidate.baseline.WindowStart, BaselineEnd: &candidate.baseline.WindowEnd},
			MetricDefinition: candidate.metric, Owner: AlertOwner{Source: "workflow_context"}, EvidenceRefs: evidenceRefs,
			Source: "connector", IsSimulated: false, Scenario: AlertScenarioConnectorInspection, DatasetVersion: snapshot.DatasetVersion,
			FixtureVersion: candidate.current.MetricDefinitionVersion, Freshness: freshness, Version: 1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now,
		})
		if upsertErr != nil {
			return ConnectorInspectionResponse{}, upsertErr
		}
		if alert.ID == id {
			result.CreatedCount++
		} else {
			result.ReusedCount++
		}
		result.Items = append(result.Items, alert)
	}
	return result, nil
}

func inspectConnectorSnapshot(snapshot connector.CanonicalSnapshot, now time.Time) (string, string, time.Time, []string, []connectorAlertCandidate) {
	if len(snapshot.Metrics) == 0 {
		return "insufficient_data", "Connector has no metric windows for this account.", time.Time{}, []string{}, nil
	}
	latestByWindow := map[string]connector.MetricWindow{}
	quarantined := 0
	for _, metric := range snapshot.Metrics {
		if metric.QualityStatus != connector.QualityAccept && metric.QualityStatus != connector.QualityWarning {
			quarantined++
			continue
		}
		key := fmt.Sprintf("%s|%s|%s|%s", metric.ObjectRef, metric.WindowStart.UTC().Format(time.RFC3339Nano), metric.WindowEnd.UTC().Format(time.RFC3339Nano), metric.MetricDefinitionVersion)
		if previous, ok := latestByWindow[key]; !ok || metric.AvailableAt.After(previous.AvailableAt) {
			latestByWindow[key] = metric
		}
	}
	if len(latestByWindow) == 0 {
		return "quarantined", fmt.Sprintf("Connector quarantined %d metric windows. Confirm attribution and metric definitions before alert evaluation.", quarantined), latestDataThrough(snapshot.Metrics), metricEvidence(snapshot.Metrics), nil
	}
	bySeries := map[string][]connector.MetricWindow{}
	completeMetrics := []connector.MetricWindow{}
	for _, metric := range latestByWindow {
		if !hasConnectorMetrics(metric) {
			continue
		}
		seriesKey := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s", metric.ObjectRef, metric.Granularity, metric.TimeZone, metric.AttributionWindow, metric.MetricDefinitionVersion, metric.Currency, metric.AmountUnit)
		bySeries[seriesKey] = append(bySeries[seriesKey], metric)
		completeMetrics = append(completeMetrics, metric)
	}
	if len(completeMetrics) == 0 {
		return "insufficient_data", "Connector metric windows do not contain all required atomic metrics and units.", latestDataThrough(mapMetricValues(latestByWindow)), metricEvidence(mapMetricValues(latestByWindow)), nil
	}
	dataThrough := latestDataThrough(completeMetrics)
	if dataThrough.IsZero() || now.Sub(dataThrough) > 48*time.Hour {
		return "stale", "Connector data is older than 48 hours. Refresh the account before alert evaluation.", dataThrough, metricEvidence(mapMetricValues(latestByWindow)), nil
	}
	candidates := []connectorAlertCandidate{}
	completeObjects := 0
	for _, windows := range bySeries {
		sort.Slice(windows, func(i, j int) bool { return windows[i].WindowEnd.Before(windows[j].WindowEnd) })
		if len(windows) < 2 {
			continue
		}
		completeObjects++
		baseline, current := windows[len(windows)-2], windows[len(windows)-1]
		entity := AlertMonitoredEntity{Type: "platform_promotion", ID: current.ObjectRef, AdvertiserID: current.SourceRef}
		candidates = append(candidates, connectorRuleCandidates(entity, baseline, current)...)
	}
	if completeObjects == 0 {
		return "insufficient_data", "Connector needs two complete metric windows for one promotion.", dataThrough, metricEvidence(mapMetricValues(latestByWindow)), nil
	}
	reason := "Connector metrics passed quality, freshness, and completeness gates."
	if quarantined > 0 {
		reason = fmt.Sprintf("Usable metrics passed the gates. Connector also quarantined %d windows.", quarantined)
	}
	return "ready", reason, dataThrough, metricEvidence(mapMetricValues(latestByWindow)), candidates
}

func connectorRuleCandidates(entity AlertMonitoredEntity, baseline, current connector.MetricWindow) []connectorAlertCandidate {
	f := func(value int64) *float64 { converted := float64(value); return &converted }
	baseSpend, spend := baseline.Metrics["spend"], current.Metrics["spend"]
	baseConversions, conversions := baseline.Metrics["conversions"], current.Metrics["conversions"]
	clicks := current.Metrics["clicks"]
	result := []connectorAlertCandidate{}
	add := func(kind AlertType, metric AlertMetricDefinition) {
		result = append(result, connectorAlertCandidate{kind: kind, entity: entity, baseline: baseline, current: current, metric: metric})
	}
	if baseSpend > 0 && spend >= baseSpend*2 {
		add(AlertSpendSpike, AlertMetricDefinition{Name: "spend", Unit: current.Currency + "_" + current.AmountUnit, ObservedValue: f(spend), BaselineValue: f(baseSpend), Threshold: f(baseSpend * 2)})
	}
	if clicks >= 100 && conversions == 0 {
		add(AlertZeroConversion, AlertMetricDefinition{Name: "conversions", Unit: "count", Numerator: f(conversions), Denominator: f(clicks), ObservedValue: f(conversions), Threshold: f(1)})
	}
	if baseConversions > 0 && conversions > 0 {
		baseCPA, currentCPA := baseSpend/baseConversions, spend/conversions
		if currentCPA >= baseCPA*2 {
			add(AlertCostWorsening, AlertMetricDefinition{Name: "cpa", Unit: current.Currency + "_" + current.AmountUnit, Numerator: f(spend), Denominator: f(conversions), ObservedValue: f(currentCPA), BaselineValue: f(baseCPA), Threshold: f(baseCPA * 2)})
		}
	}
	if baseSpend > 0 && spend*2 <= baseSpend {
		add(AlertUnderDelivery, AlertMetricDefinition{Name: "spend", Unit: current.Currency + "_" + current.AmountUnit, ObservedValue: f(spend), BaselineValue: f(baseSpend), Threshold: f(baseSpend / 2)})
	}
	return result
}

func hasConnectorMetrics(value connector.MetricWindow) bool {
	for _, name := range []string{"spend", "impressions", "clicks", "conversions"} {
		if _, ok := value.Metrics[name]; !ok {
			return false
		}
	}
	return value.MetricDefinitionVersion != "" && value.Currency != "" && value.AmountUnit != ""
}

func mapMetricValues(values map[string]connector.MetricWindow) []connector.MetricWindow {
	result := make([]connector.MetricWindow, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	return result
}

func latestDataThrough(values []connector.MetricWindow) time.Time {
	var result time.Time
	for _, value := range values {
		if value.DataThrough.After(result) {
			result = value.DataThrough
		}
	}
	return result
}

func metricEvidence(values []connector.MetricWindow) []string {
	seen, result := map[string]bool{}, []string{}
	for _, value := range values {
		if value.EvidenceRef != "" && !seen[value.EvidenceRef] {
			seen[value.EvidenceRef] = true
			result = append(result, value.EvidenceRef)
		}
	}
	sort.Strings(result)
	return result
}

func connectorAlertFingerprint(org contract.OrganizationID, project contract.ProjectID, planID string, candidate connectorAlertCandidate, evidence []string) (string, error) {
	return contract.CanonicalJSONHash(struct {
		OrganizationID contract.OrganizationID `json:"organization_id"`
		ProjectID      contract.ProjectID      `json:"project_id"`
		PlanID         string                  `json:"plan_id"`
		RuleID         AlertType               `json:"rule_id"`
		RuleVersion    string                  `json:"rule_version"`
		Entity         AlertMonitoredEntity    `json:"entity"`
		WindowStart    time.Time               `json:"window_start"`
		WindowEnd      time.Time               `json:"window_end"`
		Evidence       []string                `json:"evidence"`
	}{org, project, planID, candidate.kind, connectorAlertRuleVersion, candidate.entity, candidate.current.WindowStart, candidate.current.WindowEnd, evidence})
}

func connectorAlertDetail(kind AlertType) string {
	return map[AlertType]string{
		AlertSpendSpike:     "Connector shows that current spend is at least two times the prior window.",
		AlertZeroConversion: "Connector shows at least 100 clicks and no conversion in the current window.",
		AlertCostWorsening:  "Connector shows that current cost per conversion is at least two times the prior window.",
		AlertUnderDelivery:  "Connector shows that current spend is no more than half of the prior window.",
	}[kind]
}
