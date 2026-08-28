package plancompile

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/systems/delivery"
)

func TestCompileConfigurationV3CreatesAndEditsBoundObjects(t *testing.T) {
	now := time.Date(2026, 8, 25, 6, 0, 0, 0, time.UTC)
	configuration, intent := executableConfigurationFixture(now)

	created, err := CompileConfigurationV3(configuration, &intent, "1855554434276391", V3ObjectBindings{}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Forms) != 2 || created.Forms[0].DependsOn != "" || created.Forms[1].DependsOn != "project-draft-1" {
		t.Fatalf("create forms = %#v", created.Forms)
	}
	assertPlanKind(t, created.Forms[0].Plan, "project_create", "")
	assertPlanKind(t, created.Forms[1].Plan, "promotion_create", "")
	if len(created.Forms[0].Diff) == 0 || len(created.Forms[1].Diff) == 0 {
		t.Fatal("generated plans must show field differences")
	}

	edited, err := CompileConfigurationV3(configuration, &intent, "1855554434276391", V3ObjectBindings{
		ProjectPlatformID: "7677595885572784182", PromotionPlatformIDs: map[string]string{"promotion-draft-1": "7683558668450021382"},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	assertPlanKind(t, edited.Forms[0].Plan, "project_edit", "7677595885572784182")
	assertPlanKind(t, edited.Forms[1].Plan, "promotion_edit", "7683558668450021382")
	if edited.Forms[1].DependsOn != "" {
		t.Fatalf("bound promotion dependency = %q", edited.Forms[1].DependsOn)
	}
}

func TestCompileConfigurationV3FillsOwnedLandingPageLink(t *testing.T) {
	now := time.Date(2026, 8, 25, 6, 0, 0, 0, time.UTC)
	configuration, intent := executableConfigurationFixture(now)
	configuration.Payload.OceanEngine.Project.Carrier = "owned_landing_page"
	configuration.Payload.OceanEngine.Promotions[0].LandingPageReference = &delivery.StableReference{
		Namespace: "cookies", ObjectKind: "owned_landing_page", Scope: "current_project",
		ID: "https://example.test/landing", State: delivery.ReferenceResolved,
	}

	compiled, err := CompileConfigurationV3(configuration, &intent, "1855554434276391", V3ObjectBindings{}, now)
	if err != nil {
		t.Fatal(err)
	}
	var promotionPlan v3Plan
	if err := json.Unmarshal(compiled.Forms[1].Plan, &promotionPlan); err != nil {
		t.Fatal(err)
	}
	for _, step := range promotionPlan.Steps {
		if step.FieldKey == "promotion.landing_page_reference" {
			if step.Operation != "fill_text" || step.Target != "请选择或填写自研落地页链接" || step.Value != "https://example.test/landing" {
				t.Fatalf("owned landing-page step = %#v", step)
			}
			availability := configurationObjectAvailability(*configuration.Payload.OceanEngine)
			ownedAvailable := false
			for _, item := range availability {
				if item.FieldKey == "promotions.0.landing_page_reference" && item.Available {
					ownedAvailable = true
				}
			}
			if !ownedAvailable {
				t.Fatalf("owned landing-page availability = %#v", availability)
			}
			return
		}
	}
	t.Fatal("owned landing-page step is missing")
}

func TestCompileConfigurationV3AcceptsOtherBrandSentinel(t *testing.T) {
	now := time.Date(2026, 8, 25, 6, 0, 0, 0, time.UTC)
	configuration, intent := executableConfigurationFixture(now)
	configuration.Payload.OceanEngine.Promotions[0].Settings.BrandReference = &delivery.StableReference{
		Namespace: "oceanengine", ObjectKind: "brand", Scope: "account:oeacct_safe",
		ID: "-1", State: delivery.ReferenceResolved, DisplayNameSnapshot: "其他",
		AuditAttributes: map[string]string{"platform_object_id": "-1"},
	}

	availability := configurationObjectAvailability(*configuration.Payload.OceanEngine)
	for _, item := range availability {
		if item.FieldKey == "promotions.0.settings.brand_reference" {
			if !item.Available || item.PlatformObjectID != "-1" {
				t.Fatalf("other brand availability = %#v", item)
			}
			if _, err := CompileConfigurationV3(configuration, &intent, "1855554434276391", V3ObjectBindings{}, now); err != nil {
				t.Fatalf("compile other brand: %v", err)
			}
			return
		}
	}
	t.Fatalf("other brand availability is missing: %#v", availability)
}

func TestV3BindingsFromMappingsUsesConfirmedObjectsAndSkipsPendingStages(t *testing.T) {
	now := time.Date(2026, 8, 25, 6, 0, 0, 0, time.UTC)
	configuration, _ := executableConfigurationFixture(now)
	mappings := []delivery.PlatformEntityMapping{
		{ID: "mapping-project", ConfigurationID: configuration.ConfigurationID, AccountReferenceID: "1855554434276391", InternalObjectKind: "project", InternalObjectID: "project-draft-1", PlatformObjectKind: "project", PlatformObjectID: "7677595885572784182", Status: delivery.PlatformEntityMappingConfirmed},
		{ID: "mapping-promotion", ConfigurationID: configuration.ConfigurationID, AccountReferenceID: "1855554434276391", InternalObjectKind: "promotion", InternalObjectID: "promotion-draft-1", PlatformObjectKind: "promotion", PlatformObjectID: "7683558668450021382", Status: delivery.PlatformEntityMappingConfirmed},
	}
	bindings, err := V3BindingsFromMappings(configuration, "1855554434276391", mappings)
	if err != nil || bindings.ProjectPlatformID != "7677595885572784182" || bindings.PromotionPlatformIDs["promotion-draft-1"] != "7683558668450021382" {
		t.Fatalf("bindings=%#v err=%v", bindings, err)
	}
	mappings[1].Status = delivery.PlatformEntityMappingPending
	pending, err := V3BindingsFromMappings(configuration, "1855554434276391", mappings)
	if err != nil || pending.ProjectPlatformID == "" || pending.PromotionPlatformIDs["promotion-draft-1"] != "" {
		t.Fatalf("pending bindings=%#v err=%v", pending, err)
	}
	mappings = append(mappings, delivery.PlatformEntityMapping{ID: "other-plan", AccountReferenceID: "1855554434276391", InternalObjectKind: "promotion", InternalObjectID: "other-draft", PlatformObjectKind: "promotion", PlatformObjectID: "7683558668450021999", Status: delivery.PlatformEntityMappingConfirmed})
	if _, err := V3BindingsFromMappings(configuration, "1855554434276391", mappings); err != nil {
		t.Fatalf("unrelated mapping blocked current configuration: %v", err)
	}
}

func TestCompileConfigurationV3RejectsLimitsReferencesAndAccountPaths(t *testing.T) {
	now := time.Date(2026, 8, 25, 6, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		mutate  func(*delivery.PlatformConfiguration, *delivery.DeliveryIntent)
		account string
		want    string
	}{
		{"account", func(*delivery.PlatformConfiguration, *delivery.DeliveryIntent) {}, "1855554434276392", "unsupported account path"},
		{"budget", func(c *delivery.PlatformConfiguration, _ *delivery.DeliveryIntent) {
			c.Payload.OceanEngine.Project.BudgetAndBidding.DailyBudgetMinor = 29999
		}, "1855554434276391", "at least CNY 300"},
		{"bid", func(c *delivery.PlatformConfiguration, _ *delivery.DeliveryIntent) {
			value := int64(0)
			c.Payload.OceanEngine.Promotions[0].BudgetAndBidding.BidMinor = &value
		}, "1855554434276391", "bid is outside"},
		{"date", func(c *delivery.PlatformConfiguration, _ *delivery.DeliveryIntent) {
			c.Payload.OceanEngine.Project.Schedule.StartAt = now
		}, "1855554434276391", "next day"},
		{"material", func(c *delivery.PlatformConfiguration, _ *delivery.DeliveryIntent) {
			c.Payload.OceanEngine.Promotions[0].BaseMaterialReferences[0].State = delivery.ReferenceBlocked
		}, "1855554434276391", "not resolved"},
		{"landing intent", func(c *delivery.PlatformConfiguration, i *delivery.DeliveryIntent) {
			i.Payload.LandingPageReferences = nil
		}, "1855554434276391", "outside the delivery intent"},
		{"owned carrier with Orange landing page", func(c *delivery.PlatformConfiguration, _ *delivery.DeliveryIntent) {
			c.Payload.OceanEngine.Project.Carrier = "owned_landing_page"
			c.Payload.OceanEngine.Promotions[0].LandingPageReference.ObjectKind = "orange_landing_page"
		}, "1855554434276391", "cannot use an Orange landing page"},
		{"brand", func(c *delivery.PlatformConfiguration, _ *delivery.DeliveryIntent) {
			c.Payload.OceanEngine.Promotions[0].Settings.BrandReference.State = delivery.ReferenceUnresolved
		}, "1855554434276391", "not resolved"},
		{"category", func(c *delivery.PlatformConfiguration, _ *delivery.DeliveryIntent) {
			c.Payload.OceanEngine.Promotions[0].Settings.CategoryReference.State = delivery.ReferenceUnresolved
		}, "1855554434276391", "not resolved"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configuration, intent := executableConfigurationFixture(now)
			test.mutate(&configuration, &intent)
			_, err := CompileConfigurationV3(configuration, &intent, test.account, V3ObjectBindings{}, now)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func executableConfigurationFixture(now time.Time) (delivery.PlatformConfiguration, delivery.DeliveryIntent) {
	ref := func(kind, id, label string) delivery.StableReference {
		return delivery.StableReference{Namespace: "oceanengine", ObjectKind: kind, Scope: "account:1855554434276391", ID: id, State: delivery.ReferenceResolved, DisplayNameSnapshot: label}
	}
	product := ref("product", "1001", "测试商品")
	material := ref("material", "2001", "测试视频")
	image := ref("material", "2002", "测试图片")
	image.AuditAttributes = map[string]string{"selection_kind": "image_card", "expected_total": "1", "index": "0", "minimum_visible": "1"}
	landing := ref("landing_page", "3001", "我的落地页")
	brand := ref("brand", "4001", "测试品牌")
	category := ref("category", "5001", "零售/综合类2C电商")
	optimization := ref("optimization_target", "9001", "app内下单")
	optimization.SemanticKey = "in_app_order"
	start := now.Add(48 * time.Hour)
	end := start.Add(7 * 24 * time.Hour)
	minimum, maximum := int64(30000), int64(1000000)
	bid := int64(1)
	searchCoefficient := 1.0
	searchExpansion := true
	comments := false
	configuration := delivery.PlatformConfiguration{ConfigurationID: "configuration-1", CanonicalHash: strings.Repeat("c", 64), Platform: delivery.DeliveryPlatformOceanEngine, Payload: delivery.PlatformConfigurationPayload{OceanEngine: &delivery.OceanEngineConfiguration{Project: &delivery.OceanEngineProjectDraft{
		ProjectDraftID: "project-draft-1", AccountReference: ref("account", "1855554434276391", "测试账户"), MarketingPurpose: "ecommerce", MarketingScenario: "short_video_image_text", MarketingProductReference: &product, Carrier: "orange_landing_page", OptimizationTargetReference: &optimization, DeepOptimizationMode: "disabled", DeliveryMode: "manual", PlacementStrategy: "automatic", Schedule: delivery.OceanEngineSchedule{StartAt: start, EndAt: end, Timezone: "Asia/Shanghai"}, BudgetAndBidding: delivery.OceanEngineBudgetAndBidding{BudgetMode: delivery.OceanEngineBudgetModeDaily, Currency: "CNY", DailyBudgetMinor: 30000, ChargingMode: "CPC", BidMinor: &bid}, SearchBoost: &delivery.OceanEngineSearchBoost{BidCoefficient: &searchCoefficient, TargetingExpansion: &searchExpansion}, ProjectName: "一次性测试项目",
	}, Promotions: []delivery.OceanEnginePromotionDraft{{PromotionDraftID: "promotion-draft-1", DeliveryIdentity: delivery.OceanEngineDeliveryIdentity{Mode: "account_info"}, BaseMaterialReferences: []delivery.StableReference{material}, CopyItems: []delivery.OceanEngineCopyItem{{Text: "测试文案"}}, ProductImageReferences: []delivery.StableReference{image}, ProductSellingPoints: []string{"限时好物"}, LandingPageReference: &landing, Settings: delivery.OceanEnginePromotionSettings{CallToAction: []string{"立即预订"}, SourceLabel: "测试来源", CommentsEnabled: &comments, CategoryReference: &category, BrandReference: &brand}, BudgetAndBidding: &delivery.OceanEngineBudgetAndBidding{Currency: "CNY", DailyBudgetMinor: 30000, ChargingMode: "CPC", BidMinor: &bid}, PromotionName: "一次性测试单元"}}}}}
	intent := delivery.DeliveryIntent{Payload: delivery.DeliveryIntentPayload{BudgetBoundary: delivery.IntentBudgetBoundary{Currency: "CNY", MinimumDailyMinor: &minimum, MaximumDailyMinor: &maximum}, ScheduleBoundary: delivery.IntentScheduleBoundary{EarliestStart: start.Add(-time.Hour), LatestEnd: end.Add(time.Hour), Timezone: "Asia/Shanghai"}, ProductReferences: []delivery.StableReference{product}, LandingPageReferences: []delivery.StableReference{landing}, MaterialReferences: []delivery.StableReference{material, image}}}
	return configuration, intent
}

func assertPlanKind(t *testing.T, raw json.RawMessage, kind, objectID string) {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	if value["plan_kind"] != kind {
		t.Fatalf("plan kind = %v", value["plan_kind"])
	}
	if objectID != "" && value["object_reference"] != objectID {
		t.Fatalf("object reference = %v", value["object_reference"])
	}
}
