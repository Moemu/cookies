package strategy

import "testing"

func TestPercentileMetricUsesNearestRankWithoutInventingSamples(t *testing.T) {
	values := []int64{100, 200, 300, 400, 500}
	p50, p95 := percentileMetric(values, 0.50), percentileMetric(values, 0.95)
	if p50 == nil || *p50 != 300 || p95 == nil || *p95 != 500 {
		t.Fatalf("p50=%v p95=%v", p50, p95)
	}
	if percentileMetric(nil, 0.95) != nil {
		t.Fatal("empty evidence must remain null")
	}
}

func TestMissingMetricResourcesCountsOnlySubmittedCommands(t *testing.T) {
	expected := map[string]struct{}{"message_1": {}, "message_2": {}}
	observed := map[string]struct{}{"message_2": {}, "unrelated": {}}
	if got := missingMetricResources(expected, observed); got != 1 {
		t.Fatalf("missing resources = %d", got)
	}
}
