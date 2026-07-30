package strategy

import (
	"encoding/json"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/strategy/creativecatalog"
	strategyskills "github.com/shikanon/cookies/internal/systems/strategy/skills"
)

func TestDeterministicCreativeTaskStrategyValidates(t *testing.T) {
	t.Parallel()
	registry, err := creativecatalog.DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	profile, _ := registry.FindCurrent("viral_remake")
	skills, err := strategyskills.DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	skill, err := skills.SelectCreativeTask(profile.BusinessCode)
	if err != nil {
		t.Fatal(err)
	}
	generation := CreativeTaskGenerationContext{
		Project: contract.ProjectContext{ProjectContextVersion: 1},
		Brief: BriefVersion{
			BriefID: "brief_1", Version: 1,
			ContentHash: contract.ContentHash("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			Snapshot: BriefDocument{
				Campaign:    BriefCampaign{Objective: "提升商品转化"},
				Audience:    BriefAudience{Primary: "目标用户", PainPoints: []string{"选择困难"}},
				Proposition: "核心卖点", Measurement: BriefMeasurement{PrimaryKPI: "点击率"},
			},
		},
		PlanID: "plan_1", PlanRevision: 1,
		PlanContentHash: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Plan: CreativeTaskPlanSnapshot{
			BusinessCode: profile.BusinessCode,
			Answers:      map[string]json.RawMessage{},
			Completeness: CreativeTaskPlanCompleteness{Ready: true, Warnings: []ValidationError{}},
		},
		Profile: profile, Skill: skill, PromptVersion: "strategy.creative_task.generate.v2",
	}
	document := deterministicCreativeTaskStrategy(generation)
	normalizeCreativeTaskStrategy(&document, generation)
	report := validateCreativeTaskStrategy(document, profile)
	if !report.Passed {
		t.Fatalf("deterministic task strategy failed validation: %#v", report)
	}
	if document.PlanRef.PlanID != "plan_1" || document.BusinessRef.BusinessCode != "viral_remake" {
		t.Fatalf("lineage was not frozen: %#v", document)
	}
	if len(document.Hypotheses) < 2 {
		t.Fatalf("strategy depth must include at least two testable hypotheses: %#v", document.Hypotheses)
	}
	if document.ClaimsAndEvidence == nil || document.Media.Items == nil ||
		document.Media.Warnings == nil || document.OpenQuestions == nil {
		t.Fatalf("collection fields must serialize as arrays instead of null: %#v", document)
	}
}

func TestCreativeTaskStrategyRejectsReservedExecutionFields(t *testing.T) {
	t.Parallel()
	registry, err := creativecatalog.DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	profile, _ := registry.FindCurrent("commerce_preroll")
	document := CreativeTaskStrategyDocument{
		ContractVersion: "creative-task-strategy/v1",
		Objective:       "转化", Audience: StrategyAudience{Primary: "用户"},
		CoreMessage: "卖点", MessageHierarchy: []string{"卖点"},
		Hypotheses: []CreativeTaskHypothesis{{
			ID: "h1", Statement: "验证", Variable: "变量", Metric: "点击",
		}},
		BusinessStrategy: map[string]any{
			"conversion_message": "卖点",
			"message_sequence":   []string{"商品", "证据", "CTA"},
			"opening_mechanisms": []string{"结果先行"},
			"product_fidelity":   []string{"包装不得变形"},
			"script":             "不应出现",
		},
	}
	report := validateCreativeTaskStrategy(document, profile)
	if report.Passed {
		t.Fatal("reserved Creative execution field must be rejected")
	}
}

func TestNormalizeCreativeTaskStrategyFreezesFactsAndRights(t *testing.T) {
	t.Parallel()
	registry, err := creativecatalog.DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	profile, _ := registry.FindCurrent("viral_remake")
	generation := CreativeTaskGenerationContext{
		Brief: BriefVersion{
			BriefID: "brief_1", Version: 1,
			ContentHash: contract.ContentHash("sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			Snapshot: BriefDocument{
				Campaign:    BriefCampaign{Objective: "真实目标"},
				Audience:    BriefAudience{Primary: "真实受众"},
				Proposition: "真实主张",
				Product:     BriefProduct{Evidence: []string{"已确认检测报告"}},
				Constraints: []string{"不得夸大"},
			},
		},
		PlanID: "plan_1", PlanRevision: 1,
		PlanContentHash: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Plan: CreativeTaskPlanSnapshot{
			Answers: map[string]json.RawMessage{
				"reference_locator": json.RawMessage(`"https://example.com/reference"`),
				"rights_status":     json.RawMessage(`"public_analysis_only"`),
			},
			Completeness: CreativeTaskPlanCompleteness{Warnings: []ValidationError{{
				Field: "answers.production_use", Reason: "生产前确认使用范围",
			}}},
		},
		Profile: profile,
		Skill: strategyskills.Snapshot{
			Name: "creative_task.viral_remake", Version: "v1.0.0",
			ContentHash: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		},
		PromptVersion: "strategy.creative_task.generate.v2",
	}
	document := CreativeTaskStrategyDocument{
		Objective: "模型编造目标", Audience: StrategyAudience{Primary: "模型编造受众"},
		CoreMessage: "模型编造主张", ClaimsAndEvidence: []string{"模型编造证据"},
		Guardrails: []string{}, ReferenceUse: CreativeTaskReferenceUse{
			RightsStatus: "owned", IntendedUse: "production",
		},
	}
	normalizeCreativeTaskStrategy(&document, generation)
	if document.Objective != "真实目标" || document.Audience.Primary != "真实受众" ||
		document.CoreMessage != "真实主张" ||
		len(document.ClaimsAndEvidence) != 1 || document.ClaimsAndEvidence[0] != "已确认检测报告" {
		t.Fatalf("Brief facts were not frozen: %#v", document)
	}
	if document.ReferenceUse.RightsStatus != "public_analysis_only" ||
		document.ReferenceUse.IntendedUse != "strategy_analysis" {
		t.Fatalf("model-inferred rights leaked through normalization: %#v", document.ReferenceUse)
	}
	if !containsString(document.Guardrails, "不得夸大") ||
		!containsString(document.OpenQuestions, "生产前确认使用范围") {
		t.Fatalf("guardrails or production warning were lost: %#v", document)
	}
}
