package plancompile

import (
	"testing"

	"github.com/shikanon/cookies/internal/systems/delivery"
)

func TestPlatformReferenceIDUsesBoundOceanEngineID(t *testing.T) {
	ref := delivery.StableReference{
		Namespace: "cookies", ObjectKind: "product", ID: "product_internal_1", State: delivery.ReferenceResolved,
		AuditAttributes: map[string]string{"ocean_engine_product_id": "7665932008710946858"},
	}
	if got := platformReferenceID(ref); got != "7665932008710946858" {
		t.Fatalf("platform reference ID = %q", got)
	}
	spec, err := stableReferenceSpec(ref, nil)
	if err != nil {
		t.Fatal(err)
	}
	if spec["object_id"] != "7665932008710946858" {
		t.Fatalf("selection spec = %#v", spec)
	}
}

func TestPlatformReferenceIDRejectsUnboundCookiesObject(t *testing.T) {
	ref := delivery.StableReference{Namespace: "cookies", ObjectKind: "material", ID: "asset_internal_1", State: delivery.ReferenceResolved}
	if got := platformReferenceID(ref); got != "" {
		t.Fatalf("platform reference ID = %q", got)
	}
	if _, err := stableReferenceSpec(ref, nil); err == nil {
		t.Fatal("expected unbound reference to fail")
	}
}
