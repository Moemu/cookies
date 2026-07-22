package contract

import "testing"

func TestCanonicalJSONHashIgnoresObjectMemberOrder(t *testing.T) {
	t.Parallel()
	left, err := CanonicalJSONHash(map[string]any{"b": 2, "a": 1})
	if err != nil {
		t.Fatal(err)
	}
	right, err := CanonicalJSONHash(map[string]any{"a": 1, "b": 2})
	if err != nil {
		t.Fatal(err)
	}
	if left != right || len(left) != 64 {
		t.Fatalf("hashes differ: %q and %q", left, right)
	}
}
