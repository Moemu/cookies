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

func TestConfigurationObjectAvailabilityRejectsImageMaterialAsProductImage(t *testing.T) {
	configuration := delivery.OceanEngineConfiguration{
		Project: &delivery.OceanEngineProjectDraft{AccountReference: delivery.StableReference{ID: "account"}},
		Promotions: []delivery.OceanEnginePromotionDraft{{
			ProductImageReferences: []delivery.StableReference{{
				Namespace: "oceanengine", ObjectKind: "image_material", ID: "7649703629105889290", State: delivery.ReferenceResolved,
			}},
		}},
	}
	items := configurationObjectAvailability(configuration)
	if len(items) != 1 {
		t.Fatalf("availability count = %d", len(items))
	}
	if items[0].Available || items[0].Reason != "产品主图必须来自巨量“我的图片”，不能使用图片素材" {
		t.Fatalf("availability = %#v", items[0])
	}
}
