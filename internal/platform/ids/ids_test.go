package ids

import (
	"strings"
	"testing"
)

func TestNewCreatesOpaquePrefixedIDs(t *testing.T) {
	t.Parallel()
	first, err := New("asset")
	if err != nil {
		t.Fatal(err)
	}
	second, err := New("asset")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !strings.HasPrefix(first, "asset_") || len(first) != len("asset_")+32 {
		t.Fatalf("unexpected IDs %q and %q", first, second)
	}
}

func TestNewRejectsUnsafePrefix(t *testing.T) {
	t.Parallel()
	if _, err := New("../asset"); err == nil {
		t.Fatal("expected unsafe prefix to be rejected")
	}
}
