package plancompile

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/browserautomation/rparunner"
	"github.com/shikanon/cookies/internal/systems/delivery"
)

const configurationPlanSetV1 = "oceanengine-configuration-plan-set/v1"

// V3ObjectBindings bind Cookies draft IDs to confirmed OceanEngine IDs.
// A missing binding selects a create form. A present binding selects an edit form.
type V3ObjectBindings struct {
	ProjectPlatformID    string            `json:"project_platform_id,omitempty"`
	PromotionPlatformIDs map[string]string `json:"promotion_platform_ids,omitempty"`
}

type V3FieldDifference struct {
	FieldKey  string `json:"field_key"`
	Operation string `json:"operation"`
	Target    any    `json:"target"`
}

type V3PlannedForm struct {
	InternalObjectKind string              `json:"internal_object_kind"`
	InternalObjectID   string              `json:"internal_object_id"`
	PlatformObjectID   string              `json:"platform_object_id,omitempty"`
	DependsOn          string              `json:"depends_on,omitempty"`
	Plan               json.RawMessage     `json:"plan"`
	Diff               []V3FieldDifference `json:"diff"`
}

type V3ConfigurationPlanSet struct {
	SchemaVersion     string          `json:"schema_version"`
	ConfigurationID   string          `json:"configuration_id"`
	ConfigurationHash string          `json:"configuration_hash"`
	AccountReference  string          `json:"account_reference"`
	Forms             []V3PlannedForm `json:"forms"`
}

// V3BindingsFromMappings selects confirmed mappings for the configured
// objects and account. It ignores mappings for other plans in the project.
// It never treats an internal draft ID as a platform object ID.
func V3BindingsFromMappings(configuration delivery.PlatformConfiguration, account string, mappings []delivery.PlatformEntityMapping) (V3ObjectBindings, error) {
	bindings := V3ObjectBindings{PromotionPlatformIDs: map[string]string{}}
	ocean := configuration.Payload.OceanEngine
	if ocean == nil || ocean.Project == nil {
		return V3ObjectBindings{}, fmt.Errorf("configuration has no OceanEngine project")
	}
	for _, mapping := range mappings {
		isProject := mapping.InternalObjectKind == "project" && mapping.InternalObjectID == ocean.Project.ProjectDraftID
		isPromotion := mapping.InternalObjectKind == "promotion" && slices.ContainsFunc(ocean.Promotions, func(value delivery.OceanEnginePromotionDraft) bool {
			return value.PromotionDraftID == mapping.InternalObjectID
		})
		if !isProject && !isPromotion {
			continue
		}
		if mapping.Status == delivery.PlatformEntityMappingPending {
			continue
		}
		if mapping.Status != delivery.PlatformEntityMappingConfirmed || mapping.AccountReferenceID != account || !numericReference(mapping.PlatformObjectID) {
			return V3ObjectBindings{}, fmt.Errorf("platform mapping %s is not a confirmed binding for this configuration", mapping.ID)
		}
		switch mapping.InternalObjectKind {
		case "project":
			if mapping.PlatformObjectKind != "project" || bindings.ProjectPlatformID != "" {
				return V3ObjectBindings{}, fmt.Errorf("platform mapping %s does not bind the configured project", mapping.ID)
			}
			bindings.ProjectPlatformID = mapping.PlatformObjectID
		case "promotion":
			if mapping.PlatformObjectKind != "promotion" {
				return V3ObjectBindings{}, fmt.Errorf("platform mapping %s does not bind a configured promotion", mapping.ID)
			}
			if bindings.PromotionPlatformIDs[mapping.InternalObjectID] != "" {
				return V3ObjectBindings{}, fmt.Errorf("promotion %s has duplicate platform bindings", mapping.InternalObjectID)
			}
			bindings.PromotionPlatformIDs[mapping.InternalObjectID] = mapping.PlatformObjectID
		}
	}
	return bindings, nil
}

type fieldSpec struct {
	Key, Operation, Scope, Target string
	Required                      bool
}

var projectSpecs = map[string]fieldSpec{
	"project.marketing_purpose":             {"project.marketing_purpose", "choose_exact_visible_option", "营销目的", "电商", true},
	"project.marketing_scenario":            {"project.marketing_scenario", "choose_exact_visible_option", "营销场景", "短视频+图文", true},
	"project.marketing_product_reference":   {"project.marketing_product_reference", "open_reference_picker", "营销产品", "更换", true},
	"project.carrier":                       {"project.carrier", "choose_exact_visible_option", "投放载体", "橙子落地页", true},
	"project.optimization_target_reference": {"project.optimization_target_reference", "choose_exact_visible_option", "优化目标", "请选择", true},
	"project.deep_optimization_mode":        {"project.deep_optimization_mode", "choose_exact_visible_option", "深度优化方式", "不启用", true},
	"project.delivery_mode":                 {"project.delivery_mode", "choose_exact_visible_option", "投放模式", "自动投放(UBMax)", true},
	"project.aigc_dynamic_creative":         {"project.aigc_dynamic_creative", "toggle", "素材补充方式", "AIGC动态创意", false},
	"project.placement_strategy":            {"project.placement_strategy", "choose_exact_visible_option", "投放位置", "通投智选", true},
	"project.placement_media":               {"project.placement_media", "configure_object", "媒体选择", "全选", true},
	"project.schedule":                      {"project.schedule", "configure_object", "投放时间", "设置开始和结束日期", true},
	"project.daily_budget":                  {"project.daily_budget", "fill_money", "日预算", "spinbutton", true},
	"project.bid":                           {"project.bid", "fill_money", "出价", "spinbutton", true},
	"project.roi_coefficient":               {"project.roi_coefficient", "fill_decimal", "净成交ROI系数", "spinbutton", true},
	"project.search_bid_coefficient":        {"project.search_bid_coefficient", "fill_decimal", "出价系数", "请输入", true},
	"project.search_targeting_expansion":    {"project.search_targeting_expansion", "choose_exact_visible_option", "定向拓展", "启用", false},
	"project.project_name":                  {"project.project_name", "fill_text", "项目名称", "请输入项目名称", true},
}

var promotionSpecs = map[string]fieldSpec{
	"promotion.delivery_identity":        {"promotion.delivery_identity", "open_reference_picker", "投放身份", "请选择投放抖音号", true},
	"promotion.base_materials":           {"promotion.base_materials", "open_reference_picker", "基础素材", "添加素材", true},
	"promotion.copy_materials":           {"promotion.copy_materials", "configure_object", "文案素材", "请输入5-55个字的标题或输入关键词后选择推荐标题", true},
	"promotion.product_image_references": {"promotion.product_image_references", "open_reference_picker", "产品主图", "产品主图", true},
	"promotion.product_selling_points":   {"promotion.product_selling_points", "configure_object", "产品卖点", "最多10个产品卖点，每个6-9个字，可空格分隔，回车(Enter)提交", true},
	"promotion.landing_page_reference":   {"promotion.landing_page_reference", "open_reference_picker", "落地页", "请选择橙子落地页链接", true},
	"promotion.call_to_action":           {"promotion.call_to_action", "configure_object", "行动号召", "行动号召", true},
	"promotion.source_label":             {"promotion.source_label", "fill_text", "来源", "请输入来源", true},
	"promotion.comments_enabled":         {"promotion.comments_enabled", "choose_exact_visible_option", "单元评论", "不启用", true},
	"promotion.smart_generation_enabled": {"promotion.smart_generation_enabled", "toggle", "行动号召", "开启智能生成", false},
	"promotion.category":                 {"promotion.category", "choose_exact_visible_option", "所属类别", "请选择", true},
	"promotion.brand_reference":          {"promotion.brand_reference", "open_reference_picker", "品牌名称", "选择或手动输入品牌", true},
	"promotion.daily_budget":             {"promotion.daily_budget", "fill_money", "单元预算", "spinbutton", true},
	"promotion.bid":                      {"promotion.bid", "fill_money", "单元出价", "spinbutton", true},
	"promotion.roi_coefficient":          {"promotion.roi_coefficient", "fill_decimal", "ROI系数", "spinbutton", true},
	"promotion.pangle_bid_coefficient":   {"promotion.pangle_bid_coefficient", "fill_decimal", "穿山甲系数", "spinbutton", true},
	"promotion.promotion_name":           {"promotion.promotion_name", "fill_text", "单元名称", "请输入", true},
}

// CompileConfigurationV3 validates one immutable configuration and produces
// one Runner v3 form for each configured project and promotion.
func CompileConfigurationV3(configuration delivery.PlatformConfiguration, intent *delivery.DeliveryIntent, account string, bindings V3ObjectBindings, now time.Time) (V3ConfigurationPlanSet, error) {
	ocean := configuration.Payload.OceanEngine
	if configuration.Platform != delivery.DeliveryPlatformOceanEngine || ocean == nil || ocean.Project == nil {
		return V3ConfigurationPlanSet{}, fmt.Errorf("unsupported platform configuration")
	}
	project := *ocean.Project
	if err := validateAccountPath(project, account); err != nil {
		return V3ConfigurationPlanSet{}, err
	}
	if err := validateConfigurationLimits(project, ocean.Promotions, intent, now); err != nil {
		return V3ConfigurationPlanSet{}, err
	}
	parent, err := parentContext(project)
	if err != nil {
		return V3ConfigurationPlanSet{}, err
	}
	projectValues, err := projectPlanValues(project, intent)
	if err != nil {
		return V3ConfigurationPlanSet{}, err
	}
	projectKind := "project_create"
	if bindings.ProjectPlatformID != "" {
		if !numericReference(bindings.ProjectPlatformID) {
			return V3ConfigurationPlanSet{}, fmt.Errorf("project platform binding is not numeric")
		}
		projectKind = "project_edit"
	}
	projectPlan := formPlan(projectKind, account, bindings.ProjectPlatformID, "", parent, orderedProjectFields(project, parent), projectValues)
	projectRaw, _ := json.Marshal(projectPlan)
	forms := []V3PlannedForm{{InternalObjectKind: "project", InternalObjectID: project.ProjectDraftID, PlatformObjectID: bindings.ProjectPlatformID, Plan: projectRaw, Diff: planDiff(projectPlan)}}

	parentID := bindings.ProjectPlatformID
	if parentID == "" {
		parentID = "binding:" + project.ProjectDraftID
	}
	for _, promotion := range ocean.Promotions {
		values, valueErr := promotionPlanValues(promotion, project, intent)
		if valueErr != nil {
			return V3ConfigurationPlanSet{}, fmt.Errorf("promotion %s: %w", promotion.PromotionDraftID, valueErr)
		}
		objectID := bindings.PromotionPlatformIDs[promotion.PromotionDraftID]
		kind := "promotion_create"
		if objectID != "" {
			if !numericReference(objectID) {
				return V3ConfigurationPlanSet{}, fmt.Errorf("promotion %s platform binding is not numeric", promotion.PromotionDraftID)
			}
			kind = "promotion_edit"
		}
		plan := formPlan(kind, account, objectID, parentID, parent, orderedPromotionFields(project), values)
		raw, _ := json.Marshal(plan)
		depends := ""
		if bindings.ProjectPlatformID == "" {
			depends = project.ProjectDraftID
		}
		forms = append(forms, V3PlannedForm{InternalObjectKind: "promotion", InternalObjectID: promotion.PromotionDraftID, PlatformObjectID: objectID, DependsOn: depends, Plan: raw, Diff: planDiff(plan)})
	}
	return V3ConfigurationPlanSet{SchemaVersion: configurationPlanSetV1, ConfigurationID: configuration.ConfigurationID, ConfigurationHash: configuration.CanonicalHash, AccountReference: account, Forms: forms}, nil
}

func validateAccountPath(project delivery.OceanEngineProjectDraft, account string) error {
	if !numericReference(account) || project.AccountReference.State != delivery.ReferenceResolved || project.AccountReference.ID != account {
		return fmt.Errorf("unsupported account path: exact numeric OceanEngine account binding is required")
	}
	if project.MarketingPurpose != "ecommerce" || project.MarketingScenario != "short_video_image_text" {
		return fmt.Errorf("unsupported account path: only calibrated ecommerce short-video and image-text forms are allowed")
	}
	return nil
}

func validateConfigurationLimits(project delivery.OceanEngineProjectDraft, promotions []delivery.OceanEnginePromotionDraft, intent *delivery.DeliveryIntent, now time.Time) error {
	budget := project.BudgetAndBidding
	if budget.Currency != "CNY" || (budget.BudgetMode != delivery.OceanEngineBudgetModeUnlimited && budget.DailyBudgetMinor < 30000) {
		return fmt.Errorf("project daily budget must be unlimited or at least CNY 300")
	}
	if err := validateBid(budget); err != nil {
		return fmt.Errorf("project: %w", err)
	}
	if project.Schedule.Timezone != "Asia/Shanghai" || !project.Schedule.EndAt.After(project.Schedule.StartAt) {
		return fmt.Errorf("project schedule must use Asia/Shanghai and have an ordered range")
	}
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	tomorrow := time.Date(now.In(shanghai).Year(), now.In(shanghai).Month(), now.In(shanghai).Day()+1, 0, 0, 0, 0, shanghai)
	if project.Schedule.StartAt.In(shanghai).Before(tomorrow) {
		return fmt.Errorf("project start date must be no earlier than the next day")
	}
	if intent != nil {
		boundary := intent.Payload.BudgetBoundary
		if boundary.Currency != "CNY" {
			return fmt.Errorf("intent budget currency is not CNY")
		}
		if boundary.MinimumDailyMinor != nil && budget.DailyBudgetMinor < *boundary.MinimumDailyMinor {
			return fmt.Errorf("project daily budget is below intent limit")
		}
		if boundary.MaximumDailyMinor != nil && budget.DailyBudgetMinor > *boundary.MaximumDailyMinor {
			return fmt.Errorf("project daily budget exceeds intent limit")
		}
		schedule := intent.Payload.ScheduleBoundary
		if project.Schedule.StartAt.Before(schedule.EarliestStart) || project.Schedule.EndAt.After(schedule.LatestEnd) {
			return fmt.Errorf("project schedule is outside intent limits")
		}
	}
	for _, promotion := range promotions {
		if len(promotion.CopyItems) == 0 || strings.TrimSpace(promotion.PromotionName) == "" || strings.TrimSpace(promotion.Settings.SourceLabel) == "" {
			return fmt.Errorf("promotion %s requires copy, source, and name", promotion.PromotionDraftID)
		}
		if len(promotion.Settings.CallToAction) < 1 || len(promotion.Settings.CallToAction) > 10 || duplicateOrBlank(promotion.Settings.CallToAction) {
			return fmt.Errorf("promotion %s call to action needs 1 to 10 unique values", promotion.PromotionDraftID)
		}
		if promotion.BudgetAndBidding == nil {
			continue
		}
		value := *promotion.BudgetAndBidding
		if value.Currency != "CNY" || value.DailyBudgetMinor < 30000 {
			return fmt.Errorf("promotion %s daily budget must be at least CNY 300", promotion.PromotionDraftID)
		}
		if err := validateBid(value); err != nil {
			return fmt.Errorf("promotion %s: %w", promotion.PromotionDraftID, err)
		}
	}
	return nil
}

func duplicateOrBlank(values []string) bool {
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return true
		}
		if _, ok := seen[value]; ok {
			return true
		}
		seen[value] = struct{}{}
	}
	return false
}

func validateBid(value delivery.OceanEngineBudgetAndBidding) error {
	if value.BidMinor != nil {
		minimum, maximum := int64(1), int64(0)
		if value.ChargingMode == "CPM" {
			minimum, maximum = 400, 10000
		}
		if *value.BidMinor < minimum || (maximum > 0 && *value.BidMinor > maximum) {
			return fmt.Errorf("bid is outside the calibrated limit")
		}
	}
	if value.ROICoefficient != nil && *value.ROICoefficient <= 0 {
		return fmt.Errorf("ROI coefficient must be positive")
	}
	return nil
}

func projectPlanValues(project delivery.OceanEngineProjectDraft, intent *delivery.DeliveryIntent) (map[string]any, error) {
	values := map[string]any{
		"project.marketing_purpose": "电商", "project.marketing_scenario": "短视频+图文",
		"project.carrier": carrierLabel(project.Carrier), "project.delivery_mode": deliveryModeLabel(project.DeliveryMode),
		"project.schedule":     map[string]string{"start": project.Schedule.StartAt.Format(time.DateOnly), "end": project.Schedule.EndAt.Format(time.DateOnly)},
		"project.daily_budget": money(project.BudgetAndBidding.DailyBudgetMinor), "project.project_name": project.ProjectName,
	}
	if project.MarketingProductReference != nil {
		spec, err := stableReferenceSpec(*project.MarketingProductReference, intentRefs(intent, "product"))
		if err != nil {
			return nil, fmt.Errorf("marketing product: %w", err)
		}
		values["project.marketing_product_reference"] = spec
	}
	optimization := referenceKey(project.OptimizationTargetReference)
	values["project.optimization_target_reference"] = optimizationLabel(optimization)
	if project.DeepOptimizationMode != "" {
		values["project.deep_optimization_mode"] = deepOptimizationLabel(project.DeepOptimizationMode)
	}
	if project.AIGCDynamicCreative != nil {
		values["project.aigc_dynamic_creative"] = *project.AIGCDynamicCreative
	}
	if project.PlacementStrategy != "" {
		values["project.placement_strategy"] = placementLabel(project.PlacementStrategy)
	}
	if len(project.PlacementMedia) > 0 {
		values["project.placement_media"] = project.PlacementMedia
	}
	if project.BudgetAndBidding.BidMinor != nil {
		values["project.bid"] = money(*project.BudgetAndBidding.BidMinor)
	}
	if project.BudgetAndBidding.ROICoefficient != nil {
		values["project.roi_coefficient"] = decimal(*project.BudgetAndBidding.ROICoefficient)
	}
	if project.SearchBoost != nil {
		if project.SearchBoost.BidCoefficient != nil {
			values["project.search_bid_coefficient"] = decimal(*project.SearchBoost.BidCoefficient)
		}
		if project.SearchBoost.TargetingExpansion != nil {
			if *project.SearchBoost.TargetingExpansion {
				values["project.search_targeting_expansion"] = "启用"
			} else {
				values["project.search_targeting_expansion"] = "不启用"
			}
		}
	}
	return values, nil
}

func promotionPlanValues(p delivery.OceanEnginePromotionDraft, project delivery.OceanEngineProjectDraft, intent *delivery.DeliveryIntent) (map[string]any, error) {
	values := map[string]any{"promotion.copy_materials": copyTexts(p.CopyItems), "promotion.product_selling_points": p.ProductSellingPoints, "promotion.call_to_action": p.Settings.CallToAction, "promotion.source_label": p.Settings.SourceLabel, "promotion.promotion_name": p.PromotionName}
	if p.DeliveryIdentity.Mode == "account_info" {
		values["promotion.delivery_identity"] = "账号信息"
	} else if p.DeliveryIdentity.AuthorizedIdentity != nil {
		spec, err := stableReferenceSpec(*p.DeliveryIdentity.AuthorizedIdentity, nil)
		if err != nil {
			return nil, fmt.Errorf("delivery identity: %w", err)
		}
		values["promotion.delivery_identity"] = spec
	} else {
		return nil, fmt.Errorf("delivery identity is unresolved")
	}
	materials := make([]any, 0, len(p.BaseMaterialReferences))
	for _, ref := range p.BaseMaterialReferences {
		spec, err := stableReferenceSpec(ref, intentRefs(intent, "material"))
		if err != nil {
			return nil, fmt.Errorf("base material: %w", err)
		}
		materials = append(materials, spec)
	}
	if len(materials) != 1 {
		return nil, fmt.Errorf("Runner v3 supports exactly one bound base material per form")
	}
	values["promotion.base_materials"] = materials[0]
	if len(p.ProductImageReferences) > 0 {
		if len(p.ProductImageReferences) != 1 {
			return nil, fmt.Errorf("Runner v3 supports exactly one product image")
		}
		spec, err := stableReferenceSpec(p.ProductImageReferences[0], intentRefs(intent, "material"))
		if err != nil {
			return nil, fmt.Errorf("product image: %w", err)
		}
		if p.ProductImageReferences[0].AuditAttributes["expected_total"] != "1" {
			return nil, fmt.Errorf("product image picker requires an observed expected_total of 1")
		}
		spec["selection_kind"] = "image_card"
		values["promotion.product_image_references"] = spec
	}
	if p.LandingPageReference != nil {
		spec, err := stableReferenceSpec(*p.LandingPageReference, intentRefs(intent, "landing_page"))
		if err != nil {
			return nil, fmt.Errorf("landing page: %w", err)
		}
		values["promotion.landing_page_reference"] = spec
	}
	if p.Settings.CategoryReference != nil {
		label, err := resolvedLabel(*p.Settings.CategoryReference)
		if err != nil {
			return nil, fmt.Errorf("category: %w", err)
		}
		values["promotion.category"] = label
	}
	if p.Settings.BrandReference != nil {
		spec, err := stableReferenceSpec(*p.Settings.BrandReference, nil)
		if err != nil {
			return nil, fmt.Errorf("brand: %w", err)
		}
		spec["selection_kind"] = "text_option"
		values["promotion.brand_reference"] = spec
	}
	if p.Settings.CommentsEnabled != nil {
		if *p.Settings.CommentsEnabled {
			values["promotion.comments_enabled"] = "启用"
		} else {
			values["promotion.comments_enabled"] = "不启用"
		}
	}
	if p.Settings.SmartGenerationEnabled != nil {
		values["promotion.smart_generation_enabled"] = *p.Settings.SmartGenerationEnabled
	}
	if p.BudgetAndBidding != nil {
		values["promotion.daily_budget"] = money(p.BudgetAndBidding.DailyBudgetMinor)
		if p.BudgetAndBidding.BidMinor != nil {
			values["promotion.bid"] = money(*p.BudgetAndBidding.BidMinor)
		}
		if p.BudgetAndBidding.ROICoefficient != nil {
			values["promotion.roi_coefficient"] = decimal(*p.BudgetAndBidding.ROICoefficient)
		}
	}
	_ = project
	return values, nil
}

func stableReferenceSpec(ref delivery.StableReference, allowed []delivery.StableReference) (map[string]any, error) {
	if ref.State != delivery.ReferenceResolved || strings.TrimSpace(ref.ID) == "" {
		return nil, fmt.Errorf("reference %s is not resolved", ref.ObjectKind)
	}
	if allowed != nil && !slices.ContainsFunc(allowed, func(candidate delivery.StableReference) bool {
		return candidate.State == delivery.ReferenceResolved && candidate.Namespace == ref.Namespace && candidate.ObjectKind == ref.ObjectKind && candidate.ID == ref.ID
	}) {
		return nil, fmt.Errorf("reference %s is outside the delivery intent", ref.ID)
	}
	platformID := platformReferenceID(ref)
	if platformID == "" {
		return nil, fmt.Errorf("reference %s has no OceanEngine platform ID", ref.ID)
	}
	value := map[string]any{"selection_kind": "async_row", "object_id": platformID, "confirm_button": "确定"}
	if ref.DisplayNameSnapshot != "" {
		value["label"] = ref.DisplayNameSnapshot
	}
	for _, key := range []string{"selection_kind", "confirm_button"} {
		if ref.AuditAttributes[key] != "" {
			value[key] = ref.AuditAttributes[key]
		}
	}
	for _, key := range []string{"expected_total", "index", "minimum_visible"} {
		if raw := ref.AuditAttributes[key]; raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				return nil, fmt.Errorf("reference %s has invalid %s", ref.ID, key)
			}
			value[key] = parsed
		}
	}
	return value, nil
}

func resolvedLabel(ref delivery.StableReference) (string, error) {
	if ref.State != delivery.ReferenceResolved {
		return "", fmt.Errorf("reference is not resolved")
	}
	if ref.DisplayNameSnapshot != "" {
		return ref.DisplayNameSnapshot, nil
	}
	if ref.SemanticKey != "" {
		return ref.SemanticKey, nil
	}
	if ref.ID != "" {
		return ref.ID, nil
	}
	return "", fmt.Errorf("reference has no label")
}
func intentRefs(intent *delivery.DeliveryIntent, kind string) []delivery.StableReference {
	if intent == nil {
		return nil
	}
	var values []delivery.StableReference
	switch kind {
	case "product":
		values = intent.Payload.ProductReferences
	case "landing_page":
		values = intent.Payload.LandingPageReferences
	case "material":
		values = intent.Payload.MaterialReferences
	}
	if values == nil {
		return []delivery.StableReference{}
	}
	return values
}
func referenceKey(ref *delivery.StableReference) string {
	if ref == nil {
		return ""
	}
	if ref.SemanticKey != "" {
		return ref.SemanticKey
	}
	return ref.ID
}
func copyTexts(items []delivery.OceanEngineCopyItem) []string {
	out := make([]string, len(items))
	for i := range items {
		out[i] = items[i].Text
	}
	return out
}
func money(minor int64) string     { return strconv.FormatFloat(float64(minor)/100, 'f', 2, 64) }
func decimal(value float64) string { return strconv.FormatFloat(value, 'f', -1, 64) }
func carrierLabel(value string) string {
	return map[string]string{"orange_landing_page": "橙子落地页", "owned_landing_page": "自研落地页", "byte_miniapp": "字节小程序", "wechat_miniapp": "微信小程序"}[value]
}
func optimizationLabel(value string) string {
	return map[string]string{"button_jump": "按钮跳转", "in_app_order": "app内下单", "click": "点击", "impression": "展示", "store_call": "门店电话拨打", "store_stay": "门店停留"}[value]
}
func deepOptimizationLabel(value string) string {
	return map[string]string{"disabled": "不启用", "conversion_roi": "成交ROI", "net_order": "净成交下单", "net_roi": "净成交ROI"}[value]
}
func deliveryModeLabel(value string) string {
	if value == "automatic" || value == "ubmax" {
		return "自动投放(UBMax)"
	}
	return "手动投放"
}
func placementLabel(value string) string {
	if value == "preferred_media" {
		return "首选媒体"
	}
	return "通投智选"
}

func orderedProjectFields(project delivery.OceanEngineProjectDraft, parent v3ParentContext) []fieldSpec {
	keys := []string{"project.marketing_purpose", "project.marketing_scenario", "project.marketing_product_reference", "project.carrier", "project.optimization_target_reference", "project.deep_optimization_mode", "project.delivery_mode", "project.schedule", "project.daily_budget", "project.project_name"}
	if parent.DeliveryMode == "ubmax" {
		keys = append(keys[:7], append([]string{"project.aigc_dynamic_creative"}, keys[7:]...)...)
		keys = append(keys, "project.bid")
	} else {
		keys = append(keys[:7], append([]string{"project.placement_strategy", "project.search_bid_coefficient", "project.search_targeting_expansion"}, keys[7:]...)...)
	}
	if parent.PlacementMode == "preferred_media" {
		keys = append(keys, "project.placement_media")
	}
	if parent.DeepOptimization == "conversion_roi" {
		keys = removeKey(keys, "project.bid")
	}
	if parent.DeepOptimization == "net_roi" {
		keys = removeKey(keys, "project.bid")
		keys = append(keys, "project.roi_coefficient")
	}
	if !slices.Contains([]string{"in_app_order"}, parent.OptimizationTarget) {
		keys = removeKey(keys, "project.deep_optimization_mode")
	}
	return specs(keys, projectSpecs)
}
func orderedPromotionFields(project delivery.OceanEngineProjectDraft) []fieldSpec {
	parent, _ := parentContext(project)
	keys := []string{"promotion.delivery_identity", "promotion.base_materials", "promotion.copy_materials", "promotion.product_image_references", "promotion.product_selling_points", "promotion.landing_page_reference", "promotion.call_to_action", "promotion.source_label", "promotion.comments_enabled", "promotion.smart_generation_enabled", "promotion.category", "promotion.brand_reference", "promotion.promotion_name"}
	if parent.DeliveryMode == "manual" {
		keys = append(keys, "promotion.daily_budget", "promotion.bid")
	}
	if parent.DeepOptimization == "conversion_roi" {
		keys = removeKey(keys, "promotion.bid")
		keys = append(keys, "promotion.roi_coefficient")
	}
	if parent.PlacementMode == "preferred_media" {
		keys = append(keys, "promotion.pangle_bid_coefficient")
	}
	return specs(keys, promotionSpecs)
}
func specs(keys []string, source map[string]fieldSpec) []fieldSpec {
	out := make([]fieldSpec, 0, len(keys))
	for _, key := range keys {
		if value, ok := source[key]; ok {
			out = append(out, value)
		}
	}
	return out
}
func removeKey(values []string, key string) []string {
	return slices.DeleteFunc(values, func(value string) bool { return value == key })
}

func formPlan(kind, account, object, parentProject string, parent v3ParentContext, fields []fieldSpec, values map[string]any) v3Plan {
	plan := v3Plan{SchemaVersion: rparunner.PlanSchemaV3, PlanKind: kind, Browser: "msedge", Mode: "prepare", Status: "ready", AccountReference: account, ObjectReference: object, ParentProjectReference: parentProject, ParentConditionManifestID: v3ParentManifestID, ParentContext: parent, BlockedReasons: []string{}, AllowRemoteWrite: false, MaximumFinalClicks: 0}
	plan.Steps = append(plan.Steps, v3Step{ID: "001-identify-page", Kind: "identify_page", PageKind: kind})
	for _, field := range fields {
		value, ok := values[field.Key]
		stepRequired := field.Required
		state := "missing"
		if ok {
			state = "provided"
		}
		plan.Steps = append(plan.Steps, v3Step{ID: fmt.Sprintf("%03d-%s", len(plan.Steps)+1, field.Key), Kind: "field_action", PageKind: kind, FieldKey: field.Key, Operation: field.Operation, Scope: field.Scope, Target: field.Target, Value: value, ValueState: state, Required: &stepRequired})
		if field.Required && !ok {
			plan.Status = "blocked"
			plan.BlockedReasons = append(plan.BlockedReasons, "missing_required_value:"+field.Key)
		}
	}
	plan.Steps = append(plan.Steps, v3Step{ID: fmt.Sprintf("%03d-readback", len(plan.Steps)+1), Kind: "readback", PageKind: kind})
	target := "保存并关闭"
	if strings.HasPrefix(kind, "project_") {
		target = "保存并新建单元"
	}
	plan.Steps = append(plan.Steps, v3Step{ID: fmt.Sprintf("%03d-final-click-boundary", len(plan.Steps)+1), Kind: "final_click_boundary", PageKind: kind, Target: target, RemoteWrite: true, Blocked: true, BlockReason: "PREPARE_PLAN_REMOTE_WRITE_PROHIBITED"})
	return plan
}
func planDiff(plan v3Plan) []V3FieldDifference {
	out := []V3FieldDifference{}
	for _, step := range plan.Steps {
		if step.Kind == "field_action" && step.ValueState == "provided" {
			out = append(out, V3FieldDifference{FieldKey: step.FieldKey, Operation: step.Operation, Target: step.Value})
		}
	}
	return out
}
