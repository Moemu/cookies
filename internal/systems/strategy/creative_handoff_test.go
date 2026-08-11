package strategy

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestBuildCreativeHandoffFreezesPackageProjectionAndHash(t *testing.T) {
	t.Parallel()
	snapshot := packageHashFixture()
	snapshot.Brief.Snapshot.Region = "CN"
	snapshot.Brief.Snapshot.Language = "zh-CN"
	snapshot.Brief.Snapshot.Creative.Tone = []string{"可信"}
	snapshot.Strategy.AssumptionsAndGaps = []string{"活动权益待确认"}
	packageHash, err := PackageContentHash(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Approval.ContentHash = packageHash
	value := PackageVersion{
		PackageID: snapshot.PackageID, Version: snapshot.PackageVersion,
		OrganizationID: snapshot.OrganizationID, ProjectID: snapshot.ProjectID,
		Snapshot: snapshot, ContentHash: packageHash, Status: "published",
		PublishedBy: "user_1", PublishedAt: time.Date(2026, 7, 28, 6, 0, 0, 0, time.UTC),
	}

	handoff, err := BuildCreativeHandoff(value, []contract.ProductID{"product_1"})
	if err != nil {
		t.Fatal(err)
	}
	if handoff.ContractVersion != CreativeHandoffContractVersion {
		t.Fatalf("contract version = %q", handoff.ContractVersion)
	}
	if !handoff.PackageRef.PackageContentHash.Equal(packageHash) {
		t.Fatalf("package hash = %q", handoff.PackageRef.PackageContentHash)
	}
	if handoff.UpstreamReadiness.Status != "ready" || len(handoff.UpstreamReadiness.Blockers) != 0 {
		t.Fatalf("readiness = %#v", handoff.UpstreamReadiness)
	}
	if len(handoff.Routes) != 1 || handoff.Routes[0].RouteID != "route_xiaohongshu_image_text" ||
		handoff.Routes[0].Purpose != "brand" {
		t.Fatalf("routes = %#v", handoff.Routes)
	}
	if len(handoff.CreativeView.ProductAndOffer.ProductRefIDs) != 1 ||
		handoff.CreativeView.ProductAndOffer.ProductRefIDs[0] != "product_1" {
		t.Fatalf("product refs = %#v", handoff.CreativeView.ProductAndOffer.ProductRefIDs)
	}
	if len(handoff.CreativeView.Communication.MessageHierarchy) != 0 {
		t.Fatalf("creative recommendations leaked into message hierarchy: %#v", handoff.CreativeView.Communication.MessageHierarchy)
	}
	calculated, err := creativeHandoffContentHash(handoff)
	if err != nil {
		t.Fatal(err)
	}
	if !calculated.Equal(handoff.HandoffContentHash) {
		t.Fatalf("handoff hash = %q, calculated = %q", handoff.HandoffContentHash, calculated)
	}

	payload, err := json.Marshal(handoff)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		t.Fatal(err)
	}
	view := raw["creative_view"].(map[string]any)
	for _, field := range []string{"audience_segments", "guardrails", "claims", "assets", "creative_hypotheses", "open_questions", "source_refs"} {
		if _, ok := view[field].([]any); !ok {
			t.Fatalf("creative_view.%s is not an array: %#v", field, view[field])
		}
	}
	if _, ok := raw["routes"].([]any); !ok {
		t.Fatalf("routes is not an array: %#v", raw["routes"])
	}
}

func TestCreativeHandoffHashDetectsMutation(t *testing.T) {
	t.Parallel()
	snapshot := packageHashFixture()
	packageHash, err := PackageContentHash(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	value := PackageVersion{
		PackageID: snapshot.PackageID, Version: snapshot.PackageVersion,
		OrganizationID: snapshot.OrganizationID, ProjectID: snapshot.ProjectID,
		Snapshot: snapshot, ContentHash: packageHash,
		PublishedAt: time.Date(2026, 7, 28, 6, 0, 0, 0, time.UTC),
	}
	handoff, err := BuildCreativeHandoff(value, nil)
	if err != nil {
		t.Fatal(err)
	}
	original := handoff.HandoffContentHash
	handoff.CreativeView.Objective.Statement = "mutated"
	mutated, err := creativeHandoffContentHash(handoff)
	if err != nil {
		t.Fatal(err)
	}
	if original.Equal(mutated) {
		t.Fatalf("mutation retained hash %q", original)
	}
	if err := contract.ContentHash(mutated).Validate(); err != nil {
		t.Fatalf("mutated hash invalid: %v", err)
	}
}

func TestBuildCreativeHandoffDiagnosesMissingPlanningFields(t *testing.T) {
	t.Parallel()
	snapshot := packageHashFixture()
	snapshot.Strategy.ChannelStrategy[0].Role = "unclassified"
	value := packageVersionForHandoffTest(t, snapshot)

	handoff, err := BuildCreativeHandoff(value, nil)
	if err != nil {
		t.Fatal(err)
	}
	if handoff.UpstreamReadiness.Status != "blocked" {
		t.Fatalf("readiness = %#v", handoff.UpstreamReadiness)
	}
	for _, code := range []string{"objective_type_missing", "creative_route_missing", "market_missing", "language_missing"} {
		if !hasHandoffIssue(handoff.UpstreamReadiness.Blockers, code) {
			t.Fatalf("missing blocker %q in %#v", code, handoff.UpstreamReadiness.Blockers)
		}
	}
}

func TestBuildCreativeHandoffRejectsFrozenSchemaLengthViolation(t *testing.T) {
	t.Parallel()
	snapshot := packageHashFixture()
	snapshot.Brief.Snapshot.Region = "CN"
	snapshot.Brief.Snapshot.Language = "zh-CN"
	snapshot.Strategy.Objective = strings.Repeat("目", 1001)
	value := packageVersionForHandoffTest(t, snapshot)

	if _, err := BuildCreativeHandoff(value, nil); err == nil {
		t.Fatal("oversized objective unexpectedly produced a frozen handoff")
	}
}

func TestBuildCreativeHandoffDoesNotGuessLegacyPreRollMode(t *testing.T) {
	t.Parallel()
	snapshot := packageHashFixture()
	snapshot.Brief.Snapshot.Region = "CN"
	snapshot.Brief.Snapshot.Language = "zh-CN"
	snapshot.CreativeRoutes = []CreativeRoute{{
		RouteType: "pre_roll", VideoPurpose: "performance", Channels: []string{"douyin"},
		Reason: "承接正片", TargetDurationSeconds: 5, AspectRatio: "9:16",
		SourceAssetRefs: []contract.AssetVersionRef{}, EvidenceRefs: []string{},
		RequiresHumanConfirmation: true,
	}}
	snapshot.Strategy.ChannelStrategy = []ChannelStrategy{{
		Platform: "douyin", Role: "performance conversion", Formats: []string{"short_video"},
	}}
	value := packageVersionForHandoffTest(t, snapshot)

	handoff, err := BuildCreativeHandoff(value, []contract.ProductID{"product_1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(handoff.Routes) != 0 ||
		!hasHandoffIssue(handoff.UpstreamReadiness.Blockers, "creative_route_mode_missing") {
		t.Fatalf("legacy route was guessed or not diagnosed: %#v", handoff)
	}
}

func TestBuildCreativeHandoffKeepsStableImageTextRouteWhenLegacyVideoRouteExists(t *testing.T) {
	t.Parallel()
	snapshot := packageHashFixture()
	snapshot.Brief.Snapshot.Region = "CN"
	snapshot.Brief.Snapshot.Language = "zh-CN"
	snapshot.CreativeRoutes = []CreativeRoute{{
		RouteType: "pre_roll", VideoPurpose: "performance", Channels: []string{"douyin"},
		Reason: "承接正片", TargetDurationSeconds: 5, AspectRatio: "9:16",
		SourceAssetRefs: []contract.AssetVersionRef{}, EvidenceRefs: []string{},
		RequiresHumanConfirmation: true,
	}}
	snapshot.Strategy.ChannelStrategy = []ChannelStrategy{{
		Platform: "xiaohongshu", Role: "种草心智渗透", Formats: []string{"真实场景图文笔记"},
	}, {
		Platform: "douyin", Role: "泛兴趣触达", Formats: []string{"short_video"},
	}}
	value := packageVersionForHandoffTest(t, snapshot)

	handoff, err := BuildCreativeHandoff(value, []contract.ProductID{"product_1"})
	if err != nil {
		t.Fatal(err)
	}
	if handoff.UpstreamReadiness.Status != "ready" || len(handoff.Routes) != 2 ||
		handoff.Routes[0].RouteID != "route_xiaohongshu_image_text" ||
		handoff.Routes[1].RouteID != "route_brand_video" ||
		!hasHandoffIssue(handoff.UpstreamReadiness.Warnings, "creative_route_legacy_ignored") {
		t.Fatalf("stable image-text route was blocked by unrelated legacy video route: %#v", handoff)
	}
}

func TestBuildCreativeHandoffRecognizesConcreteImageFormats(t *testing.T) {
	t.Parallel()
	snapshot := packageHashFixture()
	snapshot.Brief.Snapshot.Region = "CN"
	snapshot.Brief.Snapshot.Language = "zh-CN"
	snapshot.Strategy.ChannelStrategy = []ChannelStrategy{{
		Platform: "xiaohongshu", Role: "种草心智渗透",
		Formats: []string{"车间实拍数据三联图", "工艺前后对比图", "采购流程步骤图"},
	}}
	value := packageVersionForHandoffTest(t, snapshot)

	handoff, err := BuildCreativeHandoff(value, []contract.ProductID{"product_1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(handoff.Routes) != 1 || handoff.Routes[0].RouteID != "route_xiaohongshu_image_text" {
		t.Fatalf("concrete image formats did not produce an image-text route: %#v", handoff.Routes)
	}
}

func TestBuildCreativeHandoffFreezesBrandVideoRouteFromApprovedVideoPlan(t *testing.T) {
	t.Parallel()
	snapshot := packageHashFixture()
	snapshot.Brief.Snapshot.Region = "CN"
	snapshot.Brief.Snapshot.Language = "zh-CN"
	snapshot.Strategy.ChannelStrategy = []ChannelStrategy{
		{Platform: "xiaohongshu", Role: "品牌种草", Formats: []string{"图文笔记"}},
		{Platform: "douyin", Role: "品牌认知", Formats: []string{"竖屏短视频"}},
	}
	snapshot.Strategy.PlatformPlans = []PlatformPlan{{Platform: "douyin", Role: "工程品牌认知"}}
	value := packageVersionForHandoffTest(t, snapshot)

	handoff, err := BuildCreativeHandoff(value, []contract.ProductID{"product_1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(handoff.Routes) != 2 {
		t.Fatalf("routes = %#v", handoff.Routes)
	}
	brandRoute := handoff.Routes[1]
	if brandRoute.RouteID != "route_brand_video" || brandRoute.DeliverableType != "video" ||
		brandRoute.Purpose != "brand" || brandRoute.PerformanceMode != "brand_video" ||
		brandRoute.Spec.TargetDurationSeconds != 30 || brandRoute.Spec.AspectRatio != "9:16" ||
		len(brandRoute.AssetRequirements) != 3 || brandRoute.RouteReadiness.Status != "ready" {
		t.Fatalf("brand route = %#v", brandRoute)
	}
}

func TestBuildCreativeHandoffKeepsPerformanceRouteReadyForTaskPlanningWithoutCTA(t *testing.T) {
	t.Parallel()
	snapshot := packageHashFixture()
	snapshot.Brief.Snapshot.Region = "CN"
	snapshot.Brief.Snapshot.Language = "zh-CN"
	snapshot.Strategy.ChannelStrategy = []ChannelStrategy{{
		Platform: "xiaohongshu", Role: "performance conversion", Formats: []string{"image_text"},
	}}
	value := packageVersionForHandoffTest(t, snapshot)

	handoff, err := BuildCreativeHandoff(value, []contract.ProductID{"product_1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(handoff.Routes) != 1 || handoff.Routes[0].RouteReadiness.Status != "ready" ||
		!handoff.Routes[0].CTAPolicy.RequiredForGeneration ||
		!hasHandoffIssue(handoff.Routes[0].RouteReadiness.Warnings, "cta_missing") ||
		!hasHandoffIssue(handoff.UpstreamReadiness.Warnings, "cta_missing") {
		t.Fatalf("performance route readiness = %#v", handoff)
	}
}

func TestBuildCreativeHandoffUsesExplicitLeadGoalForMixedXiaohongshuRole(t *testing.T) {
	t.Parallel()
	snapshot := packageHashFixture()
	snapshot.Brief.Snapshot.Region = "CN"
	snapshot.Brief.Snapshot.Language = "zh-CN"
	snapshot.Strategy.Objective = "通过小红书获客并增加有效销售线索"
	snapshot.Strategy.Measurement = []string{"有效线索留资率"}
	snapshot.Strategy.ChannelStrategy = []ChannelStrategy{{
		Platform: "xiaohongshu", Role: "搜索承接与种草获客，完成留资转化",
		Formats: []string{"精度检测实拍三联图", "参数对比图"},
	}}
	value := packageVersionForHandoffTest(t, snapshot)

	handoff, err := BuildCreativeHandoff(value, []contract.ProductID{"product_1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(handoff.Routes) != 1 || handoff.Routes[0].Purpose != "performance" ||
		handoff.Routes[0].RouteReadiness.Status != "ready" {
		t.Fatalf("lead objective did not produce a plannable performance route: %#v", handoff.Routes)
	}
}

func packageVersionForHandoffTest(t *testing.T, snapshot PackageSnapshot) PackageVersion {
	t.Helper()
	hash, err := PackageContentHash(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Approval.ContentHash = hash
	return PackageVersion{
		PackageID: snapshot.PackageID, Version: snapshot.PackageVersion,
		OrganizationID: snapshot.OrganizationID, ProjectID: snapshot.ProjectID,
		Snapshot: snapshot, ContentHash: hash, Status: "published",
		PublishedBy: "user_1", PublishedAt: time.Date(2026, 7, 28, 6, 0, 0, 0, time.UTC),
	}
}

func hasHandoffIssue(issues []HandoffIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
