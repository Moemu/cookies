package delivery

import (
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/connector"
)

func TestInspectConnectorSnapshotFailsClosedForQuarantinedMetrics(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	metric := connectorInspectionMetric(now, "promotion_1", now.Add(-48*time.Hour), now.Add(-24*time.Hour), connector.QualityQuarantine, 10000, 1000, 120, 4)

	status, _, _, _, candidates := inspectConnectorSnapshot(connector.CanonicalSnapshot{DatasetVersion: connector.DatasetVersion, Metrics: []connector.MetricWindow{metric}}, now)

	if status != "quarantined" || len(candidates) != 0 {
		t.Fatalf("status=%q candidates=%d", status, len(candidates))
	}
}

func TestInspectConnectorSnapshotUsesTwoUsableWindows(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	baseline := connectorInspectionMetric(now, "promotion_1", now.Add(-48*time.Hour), now.Add(-24*time.Hour), connector.QualityAccept, 10000, 1000, 120, 10)
	current := connectorInspectionMetric(now, "promotion_1", now.Add(-24*time.Hour), now, connector.QualityWarning, 25000, 1000, 150, 5)

	status, _, _, _, candidates := inspectConnectorSnapshot(connector.CanonicalSnapshot{DatasetVersion: connector.DatasetVersion, Metrics: []connector.MetricWindow{baseline, current}}, now)

	if status != "ready" {
		t.Fatalf("status=%q", status)
	}
	if len(candidates) != 2 || candidates[0].kind != AlertSpendSpike || candidates[1].kind != AlertCostWorsening {
		t.Fatalf("candidates=%+v", candidates)
	}
	for _, candidate := range candidates {
		if candidate.entity.Type != "platform_promotion" || candidate.entity.ID != "promotion_1" {
			t.Fatalf("entity=%+v", candidate.entity)
		}
	}
}

func TestInspectConnectorSnapshotRejectsStaleUsableMetrics(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	baseline := connectorInspectionMetric(now.Add(-72*time.Hour), "promotion_1", now.Add(-96*time.Hour), now.Add(-72*time.Hour), connector.QualityAccept, 10000, 1000, 120, 10)
	current := connectorInspectionMetric(now.Add(-49*time.Hour), "promotion_1", now.Add(-72*time.Hour), now.Add(-49*time.Hour), connector.QualityAccept, 25000, 1000, 150, 5)

	status, _, _, _, candidates := inspectConnectorSnapshot(connector.CanonicalSnapshot{DatasetVersion: connector.DatasetVersion, Metrics: []connector.MetricWindow{baseline, current}}, now)

	if status != "stale" || len(candidates) != 0 {
		t.Fatalf("status=%q candidates=%d", status, len(candidates))
	}
}

func TestInspectConnectorSnapshotDoesNotCompareDifferentMetricDefinitions(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	baseline := connectorInspectionMetric(now, "promotion_1", now.Add(-48*time.Hour), now.Add(-24*time.Hour), connector.QualityAccept, 10000, 1000, 120, 10)
	current := connectorInspectionMetric(now, "promotion_1", now.Add(-24*time.Hour), now, connector.QualityAccept, 25000, 1000, 150, 5)
	current.MetricDefinitionVersion = "oceanengine-atomic-v2"

	status, _, _, _, candidates := inspectConnectorSnapshot(connector.CanonicalSnapshot{DatasetVersion: connector.DatasetVersion, Metrics: []connector.MetricWindow{baseline, current}}, now)

	if status != "insufficient_data" || len(candidates) != 0 {
		t.Fatalf("status=%q candidates=%d", status, len(candidates))
	}
}

func connectorInspectionMetric(dataThrough time.Time, objectRef string, start, end time.Time, quality connector.QualityDisposition, spend, impressions, clicks, conversions int64) connector.MetricWindow {
	return connector.MetricWindow{
		FactHeader: connector.FactHeader{SourceRef: "ref_account", EvidenceRef: "raw://" + end.Format(time.RFC3339), AvailableAt: end, DataThrough: dataThrough, QualityStatus: quality},
		ID:         end.Format(time.RFC3339), ObjectRef: objectRef, WindowStart: start, WindowEnd: end, Granularity: "day", TimeZone: "Asia/Shanghai",
		MetricDefinitionVersion: "oceanengine-atomic-v1", Currency: "CNY", AmountUnit: "fen",
		Metrics: map[string]int64{"spend": spend, "impressions": impressions, "clicks": clicks, "conversions": conversions},
	}
}
