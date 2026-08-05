package strategy

import (
	"testing"

	"github.com/shikanon/cookies/internal/systems/strategy/creativecatalog"
)

func TestCreativeRecommendationIsDeterministic(t *testing.T) {
	t.Parallel()
	registry, err := creativecatalog.DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	signals := RecommendationSignals{
		ObjectiveType: "conversion", Channels: []string{"douyin"},
		DeliverableType: "video", AssetRoles: []string{"product_image"},
	}
	first := make([]CreativeBusinessRecommendation, 0, len(registry.Current()))
	second := make([]CreativeBusinessRecommendation, 0, len(registry.Current()))
	for _, profile := range registry.Current() {
		first = append(first, evaluateCreativeBusiness(signals, profile))
		second = append(second, evaluateCreativeBusiness(signals, profile))
	}
	for index := range first {
		if first[index].BusinessCode != second[index].BusinessCode ||
			first[index].Score != second[index].Score {
			t.Fatalf("nondeterministic result: %#v %#v", first, second)
		}
	}
}

func TestRecommendationSignalsDoNotTreatKnowledgeDocumentAsReferenceVideo(t *testing.T) {
	t.Parallel()
	signals := recommendationSignals(BriefDocument{
		Campaign:     BriefCampaign{Objective: "转化"},
		Channels:     []string{"douyin"},
		ReferenceIDs: []string{"research_document"},
	})
	if signals.ReferencePresent {
		t.Fatal("an untyped knowledge document must not be treated as an inspected reference video")
	}
	registry, err := creativecatalog.DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	profile, found := registry.FindCurrent("viral_remake")
	if !found {
		t.Fatal("viral profile missing")
	}
	result := evaluateCreativeBusiness(signals, profile)
	if result.Eligible {
		t.Fatalf("viral recommendation must require an explicit reference signal: %#v", result)
	}
}

func TestCreativeRecommendationUsesBusinessCompatibilityGates(t *testing.T) {
	t.Parallel()
	registry, err := creativecatalog.DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	signals := RecommendationSignals{
		ObjectiveType: "conversion", Channels: []string{"douyin", "xiaohongshu"},
		DeliverableType: "mixed", DeliverableTypes: []string{"video", "image_text"},
		Industry: "beverage", AssetRoles: []string{"product_image"},
	}
	results := map[string]CreativeBusinessRecommendation{}
	for _, profile := range registry.Current() {
		results[profile.BusinessCode] = evaluateCreativeBusiness(signals, profile)
	}
	if results["game_preroll"].Eligible {
		t.Fatalf("generic conversion language must not recommend game_preroll: %#v", results["game_preroll"])
	}
	if !results["commerce_preroll"].Eligible || !results["xiaohongshu_image_text"].Eligible {
		t.Fatalf("expected commerce and xiaohongshu to remain eligible: %#v", results)
	}
}

func TestRecommendationSignalsNormalizeChannelsAndMixedDeliverables(t *testing.T) {
	t.Parallel()
	signals := recommendationSignals(BriefDocument{
		Campaign: BriefCampaign{Objective: "新品认知并提升首购转化"},
		Channels: []string{"小红书", "抖音"},
		PlatformBriefs: []BriefPlatform{
			{ContentFormats: []string{"图文"}},
			{ContentFormats: []string{"短视频"}},
		},
	})
	if !containsString(signals.Channels, "xiaohongshu") ||
		!containsString(signals.Channels, "douyin") ||
		signals.DeliverableType != "mixed" ||
		len(signals.DeliverableTypes) != 2 {
		t.Fatalf("signals were not normalized: %#v", signals)
	}
}

func TestRecommendationRuleReportsMissingSignal(t *testing.T) {
	t.Parallel()
	matched, missing := recommendationRuleMatches(
		RecommendationSignals{DeliverableType: "unknown"},
		creativecatalog.RecommendationRule{
			Field: "industry", Operator: "equals", Values: []string{"game"},
		},
	)
	if matched || !missing {
		t.Fatalf("matched=%v missing=%v", matched, missing)
	}
}
