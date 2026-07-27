package strategy

import "testing"

func TestCreativeRoutesForPackageRecommendsConfirmedPreRollForVideoChannels(t *testing.T) {
	t.Parallel()
	brief := BriefVersion{Snapshot: BriefDocument{Channels: []string{"douyin", "kuaishou", "douyin"}}}
	document := StrategyDocument{CreativeRecommendations: []string{"用五秒利益点前贴承接目标人群"}}
	routes := creativeRoutesForPackage(brief, document)
	if len(routes) != 1 {
		t.Fatalf("route count = %d, want 1", len(routes))
	}
	route := routes[0]
	if err := route.Validate(); err != nil {
		t.Fatalf("generated route is invalid: %v", err)
	}
	if route.RouteType != "pre_roll" || route.TargetDurationSeconds != 5 || len(route.Channels) != 2 || !route.RequiresHumanConfirmation {
		t.Fatalf("unexpected route: %+v", route)
	}
}

func TestCreativeRoutesForPackageDoesNotPretendXiaohongshuIsAStageOneVideoChannel(t *testing.T) {
	t.Parallel()
	brief := BriefVersion{Snapshot: BriefDocument{Channels: []string{"xiaohongshu"}}}
	if routes := creativeRoutesForPackage(brief, StrategyDocument{}); len(routes) != 0 {
		t.Fatalf("unexpected pre-roll route: %+v", routes)
	}
}

func TestCalculateReadinessAllowsApprovedVideoRouteWithoutXiaohongshuPlan(t *testing.T) {
	t.Parallel()
	brief := BriefVersion{Snapshot: BriefDocument{Channels: []string{"douyin"}}}
	document := StrategyDocument{CreativeRecommendations: []string{"用五秒利益点前贴承接目标人群"}}

	readiness := calculateReadiness(brief, document)

	if !readiness.CreativeReady {
		t.Fatal("a strategy with a supported pre-roll route must be ready for Creative handoff")
	}
}
