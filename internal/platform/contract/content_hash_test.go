package contract

import "testing"

func TestContentHashFormatAndComparison(t *testing.T) {
	t.Parallel()
	left, err := NewContentHash(map[string]any{"b": 2, "a": 1})
	if err != nil {
		t.Fatal(err)
	}
	right, err := NewContentHash(map[string]any{"a": 1, "b": 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := left.Validate(); err != nil {
		t.Fatal(err)
	}
	if !left.Equal(right) {
		t.Fatalf("hashes differ: %q and %q", left, right)
	}
	if _, err := ParseContentHash("sha256:ABC"); err == nil {
		t.Fatal("invalid content hash accepted")
	}
}
