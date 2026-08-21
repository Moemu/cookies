package connector

import (
	"strings"
	"testing"
)

func TestCalibrationExportRefIsStableScopedAndKeyed(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	first, err := CalibrationExportRef(key, "promotion", AnonymizeRef("raw-promotion"))
	if err != nil {
		t.Fatal(err)
	}
	replay, err := CalibrationExportRef(key, "promotion", AnonymizeRef("raw-promotion"))
	if err != nil || replay != first {
		t.Fatalf("replay=%q error=%v", replay, err)
	}
	material, err := CalibrationExportRef(key, "material", AnonymizeRef("raw-promotion"))
	if err != nil || material == first {
		t.Fatalf("object kind did not scope the reference: %q", material)
	}
	if !strings.HasPrefix(first, "anon_v1_") || strings.Contains(first, "raw-promotion") {
		t.Fatalf("unsafe export reference %q", first)
	}
}

func TestCalibrationExportRefRejectsWeakKeyAndRawReference(t *testing.T) {
	if _, err := CalibrationExportRef([]byte("short"), "promotion", AnonymizeRef("p")); err == nil {
		t.Fatal("weak export key was accepted")
	}
	if _, err := CalibrationExportRef(make([]byte, 32), "promotion", "raw-promotion"); err == nil {
		t.Fatal("raw platform reference was accepted")
	}
}
