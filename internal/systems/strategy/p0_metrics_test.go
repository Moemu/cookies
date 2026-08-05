package strategy

import "testing"

func TestMedianMetricDoesNotMutateInput(t *testing.T) {
	t.Parallel()
	values := []int64{9, 1, 5, 3}
	median := medianMetric(values)
	if median == nil || *median != 4 {
		t.Fatalf("median=%v", median)
	}
	if values[0] != 9 || values[1] != 1 {
		t.Fatalf("median mutated input: %#v", values)
	}
}

func TestMedianMetricHandlesEmptyAndOddSamples(t *testing.T) {
	t.Parallel()
	if medianMetric(nil) != nil {
		t.Fatal("empty samples must not invent a metric")
	}
	median := medianMetric([]int64{12, 2, 7})
	if median == nil || *median != 7 {
		t.Fatalf("median=%v", median)
	}
}
