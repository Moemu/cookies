package plancompile

import (
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/systems/delivery"
)

const unavailablePlatformObjectsReason = "PLATFORM_OBJECTS_UNAVAILABLE"

// V3ObjectAvailability reports whether a Cookies reference can be used by an
// OceanEngine form. It does not query or change the remote platform.
type V3ObjectAvailability struct {
	FieldKey         string `json:"field_key"`
	ObjectKind       string `json:"object_kind"`
	InternalObjectID string `json:"internal_object_id"`
	DisplayName      string `json:"display_name,omitempty"`
	PlatformObjectID string `json:"platform_object_id,omitempty"`
	Available        bool   `json:"available"`
	Reason           string `json:"reason,omitempty"`
}

func configurationObjectAvailability(configuration delivery.OceanEngineConfiguration) []V3ObjectAvailability {
	values := make([]V3ObjectAvailability, 0)
	appendReference := func(field string, ref *delivery.StableReference) {
		if ref == nil {
			return
		}
		platformID := platformReferenceID(*ref)
		item := V3ObjectAvailability{
			FieldKey: field, ObjectKind: ref.ObjectKind, InternalObjectID: ref.ID,
			DisplayName: ref.DisplayNameSnapshot, PlatformObjectID: platformID,
			Available: platformID != "",
		}
		if !item.Available {
			item.Reason = "未绑定巨量平台 ID"
		}
		values = append(values, item)
	}
	appendReferences := func(field string, refs []delivery.StableReference) {
		for index := range refs {
			appendReference(fmt.Sprintf("%s.%d", field, index), &refs[index])
		}
	}

	project := configuration.Project
	if project == nil {
		return values
	}
	appendReference("project.marketing_product_reference", project.MarketingProductReference)
	appendReference("project.application_reference", project.ApplicationReference)
	appendReference("project.product_catalog_reference", project.ProductCatalogReference)
	for promotionIndex := range configuration.Promotions {
		promotion := &configuration.Promotions[promotionIndex]
		prefix := fmt.Sprintf("promotions.%d", promotionIndex)
		if promotion.DeliveryIdentity.Mode != "account_info" {
			appendReference(prefix+".delivery_identity.authorized_identity", promotion.DeliveryIdentity.AuthorizedIdentity)
		}
		appendReferences(prefix+".base_material_references", promotion.BaseMaterialReferences)
		appendReferences(prefix+".product_image_references", promotion.ProductImageReferences)
		appendReference(prefix+".native_anchor_reference", promotion.NativeAnchorReference)
		appendReference(prefix+".landing_page_reference", promotion.LandingPageReference)
		appendReference(prefix+".direct_link_reference", promotion.DirectLinkReference)
		appendReference(prefix+".product_reference", promotion.ProductReference)
		appendReferences(prefix+".creative_component_references", promotion.CreativeComponentReferences)
		appendReference(prefix+".settings.category_reference", promotion.Settings.CategoryReference)
		appendReference(prefix+".settings.brand_reference", promotion.Settings.BrandReference)
	}
	return values
}

func platformReferenceID(ref delivery.StableReference) string {
	if ref.State != delivery.ReferenceResolved {
		return ""
	}
	keys := []string{"platform_object_id"}
	switch ref.ObjectKind {
	case "product", "product_catalog":
		keys = append(keys, "ocean_engine_product_id")
	case "material", "product_image":
		keys = append(keys, "ocean_engine_material_id")
	case "landing_page":
		keys = append(keys, "ocean_engine_landing_page_id")
	case "delivery_identity":
		keys = append(keys, "ocean_engine_identity_id")
	case "application":
		keys = append(keys, "ocean_engine_application_id")
	case "native_anchor":
		keys = append(keys, "ocean_engine_anchor_id")
	case "direct_link":
		keys = append(keys, "ocean_engine_direct_link_id")
	case "creative_component":
		keys = append(keys, "ocean_engine_component_id")
	case "category":
		keys = append(keys, "ocean_engine_category_id")
	case "brand":
		keys = append(keys, "ocean_engine_brand_id")
	}
	for _, key := range keys {
		if value := strings.TrimSpace(ref.AuditAttributes[key]); numericReference(value) {
			return value
		}
	}
	if numericReference(strings.TrimSpace(ref.ID)) {
		return strings.TrimSpace(ref.ID)
	}
	return ""
}
