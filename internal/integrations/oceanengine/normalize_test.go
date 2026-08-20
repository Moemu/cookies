package oceanengine

import "testing"

func TestHashCanonicalJSONIsStableAndRedactsValuesFromOutput(t *testing.T) {
	first, err := HashCanonicalJSON(map[string]any{"id": "900719925474099312345", "name": "redacted"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashCanonicalJSON(map[string]any{"id": "900719925474099312345", "name": "redacted"})
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(first) != 64 {
		t.Fatalf("unstable evidence hash: %q %q", first, second)
	}
	if first == "900719925474099312345" || first == "redacted" {
		t.Fatal("evidence hash leaked source value")
	}
}

func TestNormalizeQualityFailsClosedForUnknownMapping(t *testing.T) {
	if got := NormalizeQuality(map[string]any{}, false); got != QualityIncomplete {
		t.Fatalf("quality = %q, want %q", got, QualityIncomplete)
	}
	if got := NormalizeQuality(map[string]any{"quality": "blocked"}, true); got != QualityBlocked {
		t.Fatalf("quality = %q, want %q", got, QualityBlocked)
	}
	if got := NormalizeQuality(map[string]any{"quality": "unknown"}, true); got != QualityHealthy {
		t.Fatalf("quality = %q, want %q", got, QualityHealthy)
	}
}
