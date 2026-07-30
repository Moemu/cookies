package strategy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/systems/strategy/creativecatalog"
)

func TestDeterministicCreativeBusinessStrategyUsesConfirmedAnswers(t *testing.T) {
	t.Parallel()
	registry, err := creativecatalog.DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	profile, found := registry.FindCurrent("commerce_preroll")
	if !found {
		t.Fatal("commerce profile missing")
	}
	generation := CreativeTaskGenerationContext{
		Brief: BriefVersion{Snapshot: BriefDocument{
			Campaign: BriefCampaign{Objective: "提升首购转化"},
			Audience: BriefAudience{Primary: "一线城市年轻白领"},
			Product: BriefProduct{
				Name:          "轻氧青柠气泡水",
				SellingPoints: []string{"0 糖", "真实青柠风味"},
				Evidence:      []string{"配料表确认每 100ml 糖含量为 0g"},
			},
			Proposition: "清爽无负担",
		}},
		Profile: profile,
		Plan: CreativeTaskPlanSnapshot{Answers: map[string]json.RawMessage{
			"offer_facts":       json.RawMessage(`"首单 9.9 元，限新用户，活动至 8 月 31 日"`),
			"conversion_action": json.RawMessage(`"purchase"`),
		}},
	}
	result := deterministicCreativeBusinessStrategy(generation)
	conversion, _ := result["conversion_message"].(string)
	sequence, _ := result["message_sequence"].([]string)
	encoded, _ := json.Marshal(sequence)
	if !strings.Contains(conversion, "首单 9.9 元") ||
		!strings.Contains(string(encoded), "购买") ||
		!strings.Contains(string(encoded), "0 糖") {
		t.Fatalf("strategy did not derive from confirmed answers and Brief: %#v", result)
	}
	for _, field := range profile.OutputFields {
		if result[field.Key] == field.Description {
			t.Fatalf("%s repeated the field description", field.Key)
		}
	}
}

func TestViralStrategySeparatesAbstractAnalysisFromProductionRights(t *testing.T) {
	t.Parallel()
	registry, err := creativecatalog.DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	profile, found := registry.FindCurrent("viral_remake")
	if !found {
		t.Fatal("viral profile missing")
	}
	generation := CreativeTaskGenerationContext{
		Brief: BriefVersion{Snapshot: BriefDocument{
			Campaign:    BriefCampaign{Objective: "提升购买转化"},
			Audience:    BriefAudience{Primary: "通勤人群"},
			Product:     BriefProduct{Name: "气泡水", SellingPoints: []string{"0 糖"}},
			Proposition: "通勤也能清爽无负担",
		}},
		Profile: profile,
		Plan: CreativeTaskPlanSnapshot{Answers: map[string]json.RawMessage{
			"mechanism_focus": json.RawMessage(`["rhythm","message_order"]`),
			"rights_status":   json.RawMessage(`"unknown"`),
		}},
	}
	result := deterministicCreativeBusinessStrategy(generation)
	encoded, _ := json.Marshal(result)
	if !strings.Contains(string(encoded), "节奏") ||
		!strings.Contains(string(encoded), "信息顺序") ||
		!strings.Contains(string(encoded), "不得把参考链接或公开可访问等同于可下载") {
		t.Fatalf("viral strategy boundary is shallow: %s", encoded)
	}
}
