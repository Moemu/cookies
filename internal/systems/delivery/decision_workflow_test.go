package delivery

import (
	"reflect"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestBuildDeliveryDecisionIsDeterministicAndExplainable(t *testing.T) {
	input := validDecisionEngineInput(t)
	first, err := BuildDeliveryDecision(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildDeliveryDecision(input)
	if err != nil {
		t.Fatal(err)
	}
	if first.CanonicalHash != second.CanonicalHash || !reflect.DeepEqual(first.Candidates, second.Candidates) {
		t.Fatal("same immutable inputs must produce the same decision")
	}
	if len(first.Candidates) != 3 || first.RecommendedCandidateID != "balanced" {
		t.Fatalf("unexpected candidate set: %#v", first.Candidates)
	}
	for _, candidate := range first.Candidates {
		if candidate.TargetConfiguration.ConfigurationProvenance.Kind != ConfigurationGeneratedByDecisionEngine || candidate.TargetConfiguration.CanonicalHash == "" {
			t.Fatalf("candidate is not an immutable decision-engine configuration: %#v", candidate)
		}
		if candidate.CalibrationManifest != candidate.TargetConfiguration.Payload.OceanEngine.CalibrationManifest {
			t.Fatalf("candidate must retain the configuration calibration manifest: %#v", candidate)
		}
		for _, constraint := range candidate.Constraints {
			if !constraint.Passed {
				t.Fatalf("candidate %s violates %s", candidate.ID, constraint.Code)
			}
		}
	}
	mutated := input
	mutated.Current.RawMetrics.SpendCents++
	changed, err := BuildDeliveryDecision(mutated)
	if err != nil {
		t.Fatal(err)
	}
	if changed.CanonicalHash == first.CanonicalHash {
		t.Fatal("decision hash must change when a bound fact changes")
	}
	differentIdentity := input
	differentIdentity.DecisionID = "decision-2"
	replayed, err := BuildDeliveryDecision(differentIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.CanonicalHash != first.CanonicalHash {
		t.Fatal("decision identity must not leak into the deterministic business hash")
	}
}

func TestBuildDeliveryDecisionUsesCreativeFatigueForParallelPortfolio(t *testing.T) {
	input := validDecisionEngineInput(t)
	input.Scenarios = []SimulationScenarioProbability{{Scenario: "steady", Probability: .76}, {Scenario: "creative_fatigue", Probability: .76}, {Scenario: "cost_pressure", Probability: .38}}
	input.Recommendations = []SimulationRecommendationDraft{{RecommendationType: "portfolio_test", TargetField: "parallel_project_promotion_portfolio", RequiresHumanReview: true}}
	decision, err := BuildDeliveryDecision(input)
	if err != nil {
		t.Fatal(err)
	}
	if decision.PolicyVersion != DeliveryDecisionPolicyV2 || decision.Diagnostic.Code != "ready" {
		t.Fatalf("unexpected scenario policy decision: %#v", decision)
	}
	actions := map[string]bool{}
	for _, candidate := range decision.Candidates {
		if candidate.OptimizationFocus != "parallel_material_portfolio" || candidate.Scenario != "creative_fatigue" || candidate.ScenarioProbability != .76 || candidate.BudgetChangePercent != 0 {
			t.Fatalf("creative fatigue produced a budget candidate: %#v", candidate)
		}
		actions[candidate.ProposedAction] = true
	}
	if len(actions) != 3 || !actions["launch_controlled_parallel_test"] || !actions["expand_parallel_test_and_prune_mature_losers"] {
		t.Fatalf("parallel portfolio actions are not distinct: %#v", actions)
	}
}

func TestBuildDeliveryDecisionUsesCostPressureForDistinctBudgetPlans(t *testing.T) {
	input := validDecisionEngineInput(t)
	input.Scenarios = []SimulationScenarioProbability{{Scenario: "cost_pressure", Probability: .8}}
	input.Recommendations = []SimulationRecommendationDraft{{RecommendationType: "cost_review", RequiresHumanReview: true}}
	decision, err := BuildDeliveryDecision(input)
	if err != nil {
		t.Fatal(err)
	}
	actions := map[string]bool{}
	for _, candidate := range decision.Candidates {
		if candidate.OptimizationFocus != "cost_control" || candidate.BudgetChangePercent > 0 {
			t.Fatalf("cost pressure produced an invalid candidate: %#v", candidate)
		}
		actions[candidate.ProposedAction] = true
	}
	if len(actions) != 3 {
		t.Fatalf("cost candidates are duplicated: %#v", actions)
	}
}

func TestCompileDeliveryWorkflowHardStopsRemoteWrite(t *testing.T) {
	decision, err := BuildDeliveryDecision(validDecisionEngineInput(t))
	if err != nil {
		t.Fatal(err)
	}
	candidate := decision.Candidates[1]
	now := time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)
	workflow, err := CompileDeliveryWorkflow("workflow-1", decision, candidate, "operator-1", now)
	if err != nil {
		t.Fatal(err)
	}
	if workflow.Status != "ready_for_final_approval" || workflow.RemoteWriteEnabled {
		t.Fatalf("unexpected Phase C authority boundary: %#v", workflow)
	}
	last := workflow.Steps[len(workflow.Steps)-1]
	if last.Risk != WorkflowRiskRemoteWrite || !last.Blocked || last.BlockReason != "PHASE_C_REMOTE_WRITE_PROHIBITED" {
		t.Fatalf("remote write step is not hard blocked: %#v", last)
	}
	if workflow.CanonicalHash == "" || workflow.ConfigurationCanonicalHash != candidate.TargetConfiguration.CanonicalHash {
		t.Fatal("workflow must bind the selected immutable configuration")
	}
	if workflow.CalibrationManifest != candidate.CalibrationManifest {
		t.Fatalf("workflow must retain the selected candidate calibration manifest: %#v", workflow)
	}
	var typedControl, evidenceOnly, blocked bool
	for _, step := range workflow.Steps {
		for _, field := range step.Fields {
			if field.Control != nil {
				typedControl = field.Control.Operation != "" && field.Control.ExpectedTargetCount == 1 && field.Control.Scope.Value != "" && field.Control.Target.Value != "" && field.Control.Readback.Value != ""
			}
		}
	}
	for _, diagnostic := range workflow.ManifestDiagnostics {
		if diagnostic.FieldKey == "project.project_name" && diagnostic.State == "evidence_only" {
			evidenceOnly = true
		}
		if diagnostic.FieldKey == "promotion.bid" && diagnostic.State == "blocked" && diagnostic.Reason != "" {
			blocked = true
		}
	}
	if !typedControl || !evidenceOnly || !blocked {
		t.Fatalf("workflow must preserve typed controls and Manifest treatment diagnostics: control=%t evidence=%t blocked=%t", typedControl, evidenceOnly, blocked)
	}
	duplicate, err := CompileDeliveryWorkflow("workflow-1", decision, candidate, "operator-1", now)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.CanonicalHash != workflow.CanonicalHash {
		t.Fatal("compiler output must be deterministic")
	}
	differentIdentity, err := CompileDeliveryWorkflow("workflow-2", decision, candidate, "operator-1", now)
	if err != nil {
		t.Fatal(err)
	}
	if differentIdentity.CanonicalHash != workflow.CanonicalHash {
		t.Fatal("workflow identity must not leak into the deterministic compiler hash")
	}
}

func TestBlockedDecisionDoesNotForceCandidates(t *testing.T) {
	input := validDecisionEngineInput(t)
	decision, err := BuildBlockedDeliveryDecision(input.DecisionID, input.OrganizationID, input.ProjectID, input.Plan, "insufficient_data", "two metric windows are required", "collect another metric window", input.CreatedBy, input.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Diagnostic.Code != "insufficient_data" || len(decision.Candidates) != 0 || decision.RecommendedCandidateID != "" || decision.CanonicalHash == "" {
		t.Fatalf("blocked decision forced a recommendation: %#v", decision)
	}
}

func validDecisionEngineInput(t *testing.T) DecisionEngineInput {
	t.Helper()
	intent := validDeliveryIntent(t)
	configuration := validOceanEnginePlatformConfiguration(t, intent, 1)
	plan := DeliveryPlan{
		ID: "plan-1", OrganizationID: contract.OrganizationID("org-1"), ProjectID: contract.ProjectID("project-1"), CurrentVersionNumber: 1,
		CurrentVersion: DeliveryPlanVersion{PlanID: "plan-1", OrganizationID: contract.OrganizationID("org-1"), ProjectID: contract.ProjectID("project-1"), VersionNumber: 1, CanonicalHash: configuration.CanonicalHash, DeliveryIntent: &intent, PlatformConfiguration: &configuration},
	}
	now := time.Date(2026, 8, 11, 8, 0, 0, 0, time.UTC)
	return DecisionEngineInput{
		DecisionID: "decision-1", OrganizationID: plan.OrganizationID, ProjectID: plan.ProjectID, Plan: plan,
		Simulation: OutcomeSimulationRun{ID: "simulation-1", PlanID: plan.ID, PlanVersion: 1, InputHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		Baseline:   DeliveryMetricSnapshot{ID: "metric-1", RawMetrics: RawMetrics{SpendCents: 10000, Conversions: 10}},
		Current:    DeliveryMetricSnapshot{ID: "metric-2", RawMetrics: RawMetrics{SpendCents: 15000, Conversions: 10}},
		Evidence:   []string{"simulation://metric/metric-2", "simulation://metric/metric-1"}, CreatedBy: "operator-1", CreatedAt: now,
	}
}
