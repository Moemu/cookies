package strategy

import (
	"encoding/json"
	"testing"

	"github.com/shikanon/cookies/internal/systems/strategy/creativecatalog"
)

func TestCreativeTaskPlanCompletenessSeparatesStrategyAndProduction(t *testing.T) {
	t.Parallel()
	registry, err := creativecatalog.DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	profile, found := registry.FindCurrent("commerce_preroll")
	if !found {
		t.Fatal("commerce profile missing")
	}
	brief := BriefDocument{
		Product: BriefProduct{SellingPoints: []string{"核心卖点"}},
	}
	answers := map[string]json.RawMessage{
		"offer_facts":       json.RawMessage(`"限时优惠"`),
		"conversion_action": json.RawMessage(`"purchase"`),
	}
	completeness := evaluateCreativeTaskPlanCompleteness(brief, profile, answers)
	if !completeness.Ready {
		t.Fatalf("strategy inputs should be ready: %#v", completeness)
	}
	if len(completeness.Warnings) != 2 {
		t.Fatalf("production inputs should remain warnings: %#v", completeness)
	}
}

func TestCreativeTaskAnswersCannotOverrideBriefSource(t *testing.T) {
	t.Parallel()
	registry, err := creativecatalog.DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	profile, _ := registry.FindCurrent("commerce_preroll")
	err = applyCreativeTaskAnswerOperations(
		BriefDocument{Product: BriefProduct{SellingPoints: []string{"Brief 卖点"}}},
		profile,
		map[string]json.RawMessage{},
		[]CreativeTaskAnswerOperation{{
			Op: "set", QuestionID: "selling_point_priority",
			Value: json.RawMessage(`"override"`),
		}},
	)
	if err == nil {
		t.Fatal("brief-backed question must be read-only")
	}
}

func TestCreativeTaskAnswerValidatesOptions(t *testing.T) {
	t.Parallel()
	registry, err := creativecatalog.DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	profile, _ := registry.FindCurrent("commerce_preroll")
	answers := map[string]json.RawMessage{}
	err = applyCreativeTaskAnswerOperations(
		BriefDocument{},
		profile, answers,
		[]CreativeTaskAnswerOperation{{
			Op: "set", QuestionID: "conversion_action",
			Value: json.RawMessage(`"not-supported"`),
		}},
	)
	if err == nil {
		t.Fatal("unknown option must be rejected")
	}
}

func TestCreativeTaskAnswersCanFillMissingBriefSource(t *testing.T) {
	t.Parallel()
	registry, err := creativecatalog.DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	profile, _ := registry.FindCurrent("xiaohongshu_image_text")
	answers := map[string]json.RawMessage{}
	err = applyCreativeTaskAnswerOperations(
		BriefDocument{}, profile, answers,
		[]CreativeTaskAnswerOperation{{
			Op: "set", QuestionID: "audience_scene",
			Value: json.RawMessage(`"下班后在家快速完成护理"`),
		}},
	)
	if err != nil || !rawAnswerPresent(answers["audience_scene"]) {
		t.Fatalf("missing Brief source should be fillable in Plan: answers=%#v err=%v", answers, err)
	}
}

func TestViralStrategyDoesNotRequireKnownRightsStatus(t *testing.T) {
	t.Parallel()
	registry, err := creativecatalog.DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	profile, _ := registry.FindCurrent("viral_remake")
	completeness := evaluateCreativeTaskPlanCompleteness(
		BriefDocument{}, profile, map[string]json.RawMessage{
			"reference_locator": json.RawMessage(`"https://example.com/public-video"`),
			"mechanism_focus":   json.RawMessage(`["rhythm","message_order"]`),
		},
	)
	if !completeness.Ready {
		t.Fatalf("unknown rights must not block strategy analysis: %#v", completeness)
	}
	if len(completeness.Warnings) != 2 {
		t.Fatalf("rights and production-use confirmations should remain warnings: %#v", completeness)
	}
}
