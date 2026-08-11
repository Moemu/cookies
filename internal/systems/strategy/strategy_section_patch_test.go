package strategy

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestSetStrategySectionAcceptsStructuredEditorSections(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		section string
		value   string
		assert  func(*testing.T, StrategyDocument)
	}{
		{
			name: "audience", section: "audience",
			value: `{"primary":"采购负责人","insights":["关注精度"]}`,
			assert: func(t *testing.T, document StrategyDocument) {
				if document.Audience.Primary != "采购负责人" || len(document.Audience.Insights) != 1 {
					t.Fatalf("audience = %#v", document.Audience)
				}
			},
		},
		{
			name: "channel strategy", section: "channel_strategy",
			value: `[{"platform":"xiaohongshu","role":"效果获客","formats":["精度检测实拍三联图","小红书图文笔记"]}]`,
			assert: func(t *testing.T, document StrategyDocument) {
				if len(document.ChannelStrategy) != 1 || document.ChannelStrategy[0].Formats[1] != "小红书图文笔记" {
					t.Fatalf("channel strategy = %#v", document.ChannelStrategy)
				}
			},
		},
		{
			name: "budget and cadence", section: "budget_and_cadence",
			value: `{"budget":"5万元","cadence":"两周"}`,
			assert: func(t *testing.T, document StrategyDocument) {
				if document.BudgetAndCadence.Budget != "5万元" || document.BudgetAndCadence.Cadence != "两周" {
					t.Fatalf("budget and cadence = %#v", document.BudgetAndCadence)
				}
			},
		},
		{
			name: "experiment matrix", section: "experiment_matrix",
			value: `[{"hypothesis":"实拍图提升留资","variable":"首图","metric":"留资率"}]`,
			assert: func(t *testing.T, document StrategyDocument) {
				if len(document.ExperimentMatrix) != 1 || document.ExperimentMatrix[0].Metric != "留资率" {
					t.Fatalf("experiment matrix = %#v", document.ExperimentMatrix)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			document := StrategyDocument{ContractVersion: "strategy-draft/v2"}
			if err := setStrategySection(&document, test.section, json.RawMessage(test.value)); err != nil {
				t.Fatal(err)
			}
			test.assert(t, document)
		})
	}
}

func TestSetStrategySectionKeepsLineageImmutable(t *testing.T) {
	t.Parallel()
	document := StrategyDocument{ContractVersion: "strategy-draft/v2"}
	err := setStrategySection(&document, "lineage", json.RawMessage(`{"brief_id":"changed"}`))
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("lineage patch error = %v", err)
	}
}
