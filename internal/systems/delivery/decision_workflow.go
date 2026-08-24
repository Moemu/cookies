package delivery

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/delivery/calibrationmanifest"
)

const (
	DeliveryDecisionSchemaV1         = "delivery-decision/v1"
	DeliveryDecisionPolicyV1         = "delivery-decision-policy/v1"
	DeliveryDecisionPolicyV2         = "delivery-decision-policy/v2"
	CompiledDeliveryWorkflowSchemaV1 = "compiled-delivery-workflow/v1"
	DeliveryWorkflowCompilerV1       = "oceanengine-workflow-compiler/v1"
	OceanEngineCapabilityContractV01 = "oceanengine-capability/v0.1"
	OceanEngineSelectorContractV01   = "oceanengine-selector-contract/v0.1"
	OceanEngineActionContractV01     = "oceanengine-action-contract/v0.1"
	OceanEngineExecutionDriverV1     = "playwright-rpa/edge/v1"
)

type DecisionCandidateKind string

const (
	DecisionCandidateConservative DecisionCandidateKind = "conservative"
	DecisionCandidateBalanced     DecisionCandidateKind = "balanced"
	DecisionCandidateExploratory  DecisionCandidateKind = "exploratory"
)

type DecisionInputBindings struct {
	PlanID                     string `json:"plan_id"`
	PlanVersion                int    `json:"plan_version"`
	PlanCanonicalHash          string `json:"plan_canonical_hash"`
	IntentID                   string `json:"intent_id"`
	IntentVersion              int    `json:"intent_version"`
	IntentCanonicalHash        string `json:"intent_canonical_hash"`
	ConfigurationID            string `json:"configuration_id"`
	ConfigurationVersion       int    `json:"configuration_version"`
	ConfigurationCanonicalHash string `json:"configuration_canonical_hash"`
	FactSnapshotRef            string `json:"fact_snapshot_ref"`
	SimulationRunID            string `json:"simulation_run_id,omitempty"`
	SimulationInputHash        string `json:"simulation_input_hash,omitempty"`
	BaselineMetricID           string `json:"baseline_metric_id,omitempty"`
	BaselineMetricHash         string `json:"baseline_metric_hash,omitempty"`
	CurrentMetricID            string `json:"current_metric_id,omitempty"`
	CurrentMetricHash          string `json:"current_metric_hash,omitempty"`
}

type DecisionConstraint struct {
	Code        string `json:"code"`
	Passed      bool   `json:"passed"`
	Explanation string `json:"explanation"`
}

type DeliveryDecisionCandidate struct {
	ID                  string                     `json:"id"`
	Kind                DecisionCandidateKind      `json:"kind"`
	TargetConfiguration PlatformConfiguration      `json:"target_configuration"`
	BudgetChangePercent int64                      `json:"budget_change_percent"`
	Rationale           []string                   `json:"rationale"`
	Constraints         []DecisionConstraint       `json:"constraints"`
	Risks               []string                   `json:"risks"`
	Uncertainty         string                     `json:"uncertainty"`
	CalibrationManifest CalibrationManifestBinding `json:"calibration_manifest"`
	OptimizationFocus   string                     `json:"optimization_focus,omitempty"`
	ProposedAction      string                     `json:"proposed_action,omitempty"`
	ActionMagnitude     int64                      `json:"action_magnitude_percent,omitempty"`
	Scenario            string                     `json:"scenario,omitempty"`
	ScenarioProbability float64                    `json:"scenario_probability,omitempty"`
}

type DecisionDiagnostic struct {
	Code        string `json:"code"`
	Explanation string `json:"explanation"`
	NextAction  string `json:"next_action"`
}

type DeliveryDecision struct {
	SchemaVersion          string                      `json:"schema_version"`
	ID                     string                      `json:"id"`
	OrganizationID         contract.OrganizationID     `json:"organization_id"`
	ProjectID              contract.ProjectID          `json:"project_id"`
	PolicyVersion          string                      `json:"policy_version"`
	Diagnostic             DecisionDiagnostic          `json:"diagnostic"`
	Inputs                 DecisionInputBindings       `json:"inputs"`
	Candidates             []DeliveryDecisionCandidate `json:"candidates"`
	RecommendedCandidateID string                      `json:"recommended_candidate_id"`
	Evidence               []string                    `json:"evidence"`
	CanonicalHash          string                      `json:"canonical_hash"`
	CreatedBy              string                      `json:"created_by"`
	CreatedAt              time.Time                   `json:"created_at"`
}

func (d DeliveryDecision) canonicalPayload() any {
	type canonicalCandidateTarget struct {
		SchemaVersion   string           `json:"schema_version"`
		ConfigurationID string           `json:"configuration_id"`
		VersionNumber   int              `json:"version_number"`
		Platform        DeliveryPlatform `json:"platform"`
		ProfileVersion  string           `json:"profile_version"`
		Intent          IntentBinding    `json:"intent"`
		CanonicalHash   string           `json:"canonical_hash"`
	}
	type canonicalCandidate struct {
		ID                  string                     `json:"id"`
		Kind                DecisionCandidateKind      `json:"kind"`
		Target              canonicalCandidateTarget   `json:"target_configuration"`
		BudgetChangePercent int64                      `json:"budget_change_percent"`
		Rationale           []string                   `json:"rationale"`
		Constraints         []DecisionConstraint       `json:"constraints"`
		Risks               []string                   `json:"risks"`
		Uncertainty         string                     `json:"uncertainty"`
		CalibrationManifest CalibrationManifestBinding `json:"calibration_manifest"`
		OptimizationFocus   string                     `json:"optimization_focus,omitempty"`
		ProposedAction      string                     `json:"proposed_action,omitempty"`
		ActionMagnitude     int64                      `json:"action_magnitude_percent,omitempty"`
		Scenario            string                     `json:"scenario,omitempty"`
		ScenarioProbability float64                    `json:"scenario_probability,omitempty"`
	}
	candidates := make([]canonicalCandidate, len(d.Candidates))
	for index, candidate := range d.Candidates {
		target := candidate.TargetConfiguration
		candidates[index] = canonicalCandidate{
			ID: candidate.ID, Kind: candidate.Kind,
			Target:              canonicalCandidateTarget{target.SchemaVersion, target.ConfigurationID, target.VersionNumber, target.Platform, target.ProfileVersion, target.Intent, target.CanonicalHash},
			BudgetChangePercent: candidate.BudgetChangePercent, Rationale: candidate.Rationale, Constraints: candidate.Constraints, Risks: candidate.Risks, Uncertainty: candidate.Uncertainty, CalibrationManifest: candidate.CalibrationManifest,
			OptimizationFocus: candidate.OptimizationFocus, ProposedAction: candidate.ProposedAction, ActionMagnitude: candidate.ActionMagnitude, Scenario: candidate.Scenario, ScenarioProbability: candidate.ScenarioProbability,
		}
	}
	return struct {
		SchemaVersion          string                `json:"schema_version"`
		PolicyVersion          string                `json:"policy_version"`
		Diagnostic             DecisionDiagnostic    `json:"diagnostic"`
		Inputs                 DecisionInputBindings `json:"inputs"`
		Candidates             []canonicalCandidate  `json:"candidates"`
		RecommendedCandidateID string                `json:"recommended_candidate_id"`
		Evidence               []string              `json:"evidence"`
	}{d.SchemaVersion, d.PolicyVersion, d.Diagnostic, d.Inputs, candidates, d.RecommendedCandidateID, d.Evidence}
}

func (d DeliveryDecision) ComputeCanonicalHash() (string, error) {
	return contract.CanonicalJSONHash(d.canonicalPayload())
}

func (d DeliveryDecision) Validate() error {
	if d.SchemaVersion != DeliveryDecisionSchemaV1 || (d.PolicyVersion != DeliveryDecisionPolicyV1 && d.PolicyVersion != DeliveryDecisionPolicyV2) || strings.TrimSpace(d.ID) == "" || d.OrganizationID == "" || d.ProjectID == "" ||
		strings.TrimSpace(d.Inputs.PlanID) == "" || d.Inputs.PlanVersion < 1 || !isLowercaseSHA256(d.Inputs.PlanCanonicalHash) ||
		strings.TrimSpace(d.Inputs.IntentID) == "" || d.Inputs.IntentVersion < 1 || !isLowercaseSHA256(d.Inputs.IntentCanonicalHash) ||
		strings.TrimSpace(d.Inputs.ConfigurationID) == "" || d.Inputs.ConfigurationVersion < 1 || !isLowercaseSHA256(d.Inputs.ConfigurationCanonicalHash) {
		return ErrInvalidRequest
	}
	hash, err := d.ComputeCanonicalHash()
	if err != nil || hash != d.CanonicalHash {
		return ErrApprovalContentMismatch
	}
	switch d.Diagnostic.Code {
	case "ready", "insufficient_data", "stale_data", "blocked_by_asset", "platform_pending":
	default:
		return ErrInvalidState
	}
	if d.Diagnostic.Code == "ready" {
		if len(d.Candidates) != 3 || strings.TrimSpace(d.RecommendedCandidateID) == "" || strings.TrimSpace(d.Inputs.SimulationRunID) == "" || !isLowercaseSHA256(d.Inputs.SimulationInputHash) ||
			strings.TrimSpace(d.Inputs.BaselineMetricID) == "" || !isLowercaseSHA256(d.Inputs.BaselineMetricHash) || strings.TrimSpace(d.Inputs.CurrentMetricID) == "" || !isLowercaseSHA256(d.Inputs.CurrentMetricHash) {
			return ErrInvalidState
		}
	} else if len(d.Candidates) != 0 || d.RecommendedCandidateID != "" {
		return ErrInvalidState
	}
	seen := map[string]bool{}
	recommendedFound := false
	for _, candidate := range d.Candidates {
		switch candidate.Kind {
		case DecisionCandidateConservative, DecisionCandidateBalanced, DecisionCandidateExploratory:
		default:
			return ErrInvalidState
		}
		if seen[candidate.ID] || candidate.ID != string(candidate.Kind) {
			return ErrInvalidState
		}
		seen[candidate.ID] = true
		if candidate.ID == d.RecommendedCandidateID {
			recommendedFound = true
		}
		targetHash, targetErr := candidate.TargetConfiguration.ComputeCanonicalHash()
		if targetErr != nil || targetHash != candidate.TargetConfiguration.CanonicalHash || candidate.TargetConfiguration.validateStructure() != nil {
			return ErrApprovalContentMismatch
		}
		if candidate.TargetConfiguration.Payload.OceanEngine == nil || candidate.CalibrationManifest != candidate.TargetConfiguration.Payload.OceanEngine.CalibrationManifest || candidate.CalibrationManifest.validate("candidate.calibration_manifest") != nil {
			return ErrApprovalContentMismatch
		}
		if candidate.TargetConfiguration.ConfigurationID != d.Inputs.ConfigurationID || candidate.TargetConfiguration.VersionNumber != d.Inputs.ConfigurationVersion+1 || candidate.TargetConfiguration.Intent.IntentID != d.Inputs.IntentID || candidate.TargetConfiguration.Intent.VersionNumber != d.Inputs.IntentVersion || candidate.TargetConfiguration.Intent.CanonicalHash != d.Inputs.IntentCanonicalHash {
			return ErrApprovalContentMismatch
		}
		if d.PolicyVersion == DeliveryDecisionPolicyV2 && (strings.TrimSpace(candidate.OptimizationFocus) == "" || strings.TrimSpace(candidate.ProposedAction) == "" || strings.TrimSpace(candidate.Scenario) == "" || candidate.ScenarioProbability < 0 || candidate.ScenarioProbability > 1 || candidate.ActionMagnitude < 0 || candidate.ActionMagnitude > 100) {
			return ErrInvalidState
		}
		for _, constraint := range candidate.Constraints {
			if !constraint.Passed {
				return ErrInvalidState
			}
		}
	}
	if len(d.Candidates) > 0 && !recommendedFound {
		return ErrInvalidState
	}
	return nil
}

type DecisionEngineInput struct {
	DecisionID      string
	OrganizationID  contract.OrganizationID
	ProjectID       contract.ProjectID
	Plan            DeliveryPlan
	Simulation      OutcomeSimulationRun
	Baseline        DeliveryMetricSnapshot
	Current         DeliveryMetricSnapshot
	Scenarios       []SimulationScenarioProbability
	Recommendations []SimulationRecommendationDraft
	Evidence        []string
	CreatedBy       string
	CreatedAt       time.Time
}

// BuildDeliveryDecision is a pure, deterministic policy evaluation. Identity
// and audit timestamps are supplied by the caller and excluded from the
// canonical business hash.
func BuildDeliveryDecision(input DecisionEngineInput) (DeliveryDecision, error) {
	configuration := input.Plan.CurrentVersion.PlatformConfiguration
	intent := input.Plan.CurrentVersion.DeliveryIntent
	if configuration == nil || intent == nil || configuration.Platform != DeliveryPlatformOceanEngine || configuration.Payload.OceanEngine == nil || configuration.Payload.OceanEngine.Project == nil {
		return DeliveryDecision{}, ErrInvalidState
	}
	if input.Plan.CurrentVersionNumber != input.Plan.CurrentVersion.VersionNumber || input.Simulation.PlanID != input.Plan.ID || input.Simulation.PlanVersion != input.Plan.CurrentVersionNumber {
		return DeliveryDecision{}, ErrVersionConflict
	}
	baseDailyBudget := configuration.Payload.OceanEngine.Project.BudgetAndBidding.DailyBudgetMinor
	if !candidateAboveIntentBudgetFloor(baseDailyBudget, intent.Payload.BudgetBoundary) || !candidateWithinIntentBudget(baseDailyBudget, intent.Payload.BudgetBoundary) {
		return DeliveryDecision{}, ErrInvalidState
	}
	baselineCPA := input.Baseline.RawMetrics.SpendCents / maxInt64(1, input.Baseline.RawMetrics.Conversions)
	currentCPA := input.Current.RawMetrics.SpendCents / maxInt64(1, input.Current.RawMetrics.Conversions)
	baselineMetricHash, err := decisionMetricHash(input.Baseline)
	if err != nil {
		return DeliveryDecision{}, err
	}
	currentMetricHash, err := decisionMetricHash(input.Current)
	if err != nil {
		return DeliveryDecision{}, err
	}
	cpaRatioBP := currentCPA * 10000 / maxInt64(1, baselineCPA)
	options := scenarioDecisionOptions(input.Scenarios, input.Recommendations, cpaRatioBP)
	candidates := make([]DeliveryDecisionCandidate, 0, len(options))
	for _, option := range options {
		target := cloneJSONPointer(configuration)
		target.VersionNumber++
		target.ConfigurationProvenance.Kind = ConfigurationGeneratedByDecisionEngine
		target.ConfigurationProvenance.GeneratorRef = "delivery-decision-engine"
		target.ConfigurationProvenance.PolicyVersion = DeliveryDecisionPolicyV2
		baseBudget := target.Payload.OceanEngine.Project.BudgetAndBidding.DailyBudgetMinor
		targetBudget := baseBudget * (100 + option.budgetChangePercent) / 100
		if floor := intent.Payload.BudgetBoundary.MinimumDailyMinor; floor != nil && targetBudget < *floor {
			targetBudget = *floor
		}
		if ceiling := intent.Payload.BudgetBoundary.MaximumDailyMinor; ceiling != nil && targetBudget > *ceiling {
			targetBudget = *ceiling
		}
		if targetBudget > intent.Payload.BudgetBoundary.MaximumTotalMinor {
			targetBudget = intent.Payload.BudgetBoundary.MaximumTotalMinor
		}
		target.Payload.OceanEngine.Project.BudgetAndBidding.DailyBudgetMinor = targetBudget
		target.CanonicalHash = ""
		finalized, err := FinalizePlatformConfiguration(*target)
		if err != nil {
			return DeliveryDecision{}, err
		}
		candidateID := string(option.kind)
		candidates = append(candidates, DeliveryDecisionCandidate{
			ID: candidateID, Kind: option.kind, TargetConfiguration: finalized, BudgetChangePercent: budgetChangePercent(baseBudget, targetBudget),
			Rationale: []string{
				fmt.Sprintf("scenario %s has probability %.4f", option.scenario, option.probability),
				fmt.Sprintf("action %s uses magnitude %d%%", option.action, option.actionMagnitude),
				fmt.Sprintf("current CPA is %.2fx the baseline CPA", float64(cpaRatioBP)/10000),
			},
			Constraints: []DecisionConstraint{
				{Code: "NON_NEGATIVE_BUDGET", Passed: finalized.Payload.OceanEngine.Project.BudgetAndBidding.DailyBudgetMinor >= 0, Explanation: "candidate daily budget cannot be negative"},
				{Code: "INTENT_BUDGET_FLOOR", Passed: candidateAboveIntentBudgetFloor(finalized.Payload.OceanEngine.Project.BudgetAndBidding.DailyBudgetMinor, intent.Payload.BudgetBoundary), Explanation: "candidate daily budget must respect the immutable intent budget floor"},
				{Code: "INTENT_BUDGET_CEILING", Passed: candidateWithinIntentBudget(finalized.Payload.OceanEngine.Project.BudgetAndBidding.DailyBudgetMinor, intent.Payload.BudgetBoundary), Explanation: "candidate daily budget must not exceed the immutable intent budget ceiling"},
				{Code: "REMOTE_WRITE_PROHIBITED_IN_PHASE_C", Passed: true, Explanation: "selection only prepares a local immutable configuration and workflow"},
			},
			Risks:               append([]string(nil), option.risks...),
			Uncertainty:         option.uncertain,
			CalibrationManifest: finalized.Payload.OceanEngine.CalibrationManifest,
			OptimizationFocus:   option.focus, ProposedAction: option.action, ActionMagnitude: option.actionMagnitude, Scenario: option.scenario, ScenarioProbability: option.probability,
		})
	}
	for _, candidate := range candidates {
		for _, constraint := range candidate.Constraints {
			if !constraint.Passed {
				return DeliveryDecision{}, ErrInvalidState
			}
		}
	}
	evidence := append([]string(nil), input.Evidence...)
	sort.Strings(evidence)
	decision := DeliveryDecision{
		SchemaVersion: DeliveryDecisionSchemaV1, ID: input.DecisionID, OrganizationID: input.OrganizationID, ProjectID: input.ProjectID,
		PolicyVersion: DeliveryDecisionPolicyV2, Diagnostic: DecisionDiagnostic{Code: "ready", Explanation: "the latest mechanistic scenarios produced three reviewable action plans", NextAction: "select or modify one candidate"},
		Inputs: DecisionInputBindings{
			PlanID: input.Plan.ID, PlanVersion: input.Plan.CurrentVersionNumber, PlanCanonicalHash: input.Plan.CurrentVersion.CanonicalHash,
			IntentID: intent.IntentID, IntentVersion: intent.VersionNumber, IntentCanonicalHash: intent.CanonicalHash,
			ConfigurationID: configuration.ConfigurationID, ConfigurationVersion: configuration.VersionNumber, ConfigurationCanonicalHash: configuration.CanonicalHash,
			FactSnapshotRef: configuration.FactProvenance.SnapshotRef, SimulationRunID: input.Simulation.ID, SimulationInputHash: input.Simulation.InputHash,
			BaselineMetricID: input.Baseline.ID, BaselineMetricHash: baselineMetricHash, CurrentMetricID: input.Current.ID, CurrentMetricHash: currentMetricHash,
		},
		Candidates: candidates, RecommendedCandidateID: string(DecisionCandidateBalanced), Evidence: evidence,
		CreatedBy: input.CreatedBy, CreatedAt: input.CreatedAt,
	}
	hash, err := decision.ComputeCanonicalHash()
	if err != nil {
		return DeliveryDecision{}, err
	}
	decision.CanonicalHash = hash
	if err := decision.Validate(); err != nil {
		return DeliveryDecision{}, err
	}
	return decision, nil
}

type scenarioDecisionOption struct {
	kind                DecisionCandidateKind
	budgetChangePercent int64
	uncertain           string
	focus               string
	action              string
	actionMagnitude     int64
	scenario            string
	probability         float64
	risks               []string
}

func scenarioDecisionOptions(scenarios []SimulationScenarioProbability, recommendations []SimulationRecommendationDraft, cpaRatioBP int64) []scenarioDecisionOption {
	probabilities := make(map[string]float64, len(scenarios))
	for _, scenario := range scenarios {
		probabilities[scenario.Scenario] = scenario.Probability
	}
	recommendationScenario := map[string]string{
		"review_compliance": "review_rejected", "tracking_review": "tracking_anomaly", "delivery_review": "under_delivery",
		"cost_review": "cost_pressure", "creative_test": "creative_fatigue", "portfolio_test": "creative_fatigue", "portfolio_observation": "breakout_launch", "conversion_funnel_review": "zero_conversion", "budget_pacing_review": "spend_spike",
	}
	dominant, recommendationType, dominantProbability := "steady", "maintain_and_observe", probabilities["steady"]
	for _, recommendation := range recommendations {
		scenario := recommendationScenario[recommendation.RecommendationType]
		if scenario == "" {
			continue
		}
		if probability := probabilities[scenario]; probability > dominantProbability || recommendationType == "maintain_and_observe" {
			dominant, recommendationType, dominantProbability = scenario, recommendation.RecommendationType, probability
		}
	}
	baseRisks := []string{"Simulation assumptions can differ from platform outcomes.", "Platform readback remains unverified until a later authorized phase."}
	makeOptions := func(focus, conservativeAction, balancedAction, exploratoryAction string, magnitudes [3]int64, budgetChanges [3]int64) []scenarioDecisionOption {
		return []scenarioDecisionOption{
			{DecisionCandidateConservative, budgetChanges[0], "low", focus, conservativeAction, magnitudes[0], dominant, dominantProbability, baseRisks},
			{DecisionCandidateBalanced, budgetChanges[1], "medium", focus, balancedAction, magnitudes[1], dominant, dominantProbability, baseRisks},
			{DecisionCandidateExploratory, budgetChanges[2], "high", focus, exploratoryAction, magnitudes[2], dominant, dominantProbability, baseRisks},
		}
	}
	switch recommendationType {
	case "portfolio_observation":
		return makeOptions("parallel_launch_observation", "observe_first_seven_days", "prepare_parallel_project_test", "prepare_parallel_project_and_promotion_test", [3]int64{0, 0, 0}, [3]int64{})
	case "creative_test", "portfolio_test":
		return makeOptions("parallel_material_portfolio", "observe_existing_portfolio", "launch_controlled_parallel_test", "expand_parallel_test_and_prune_mature_losers", [3]int64{0, 20, 30}, [3]int64{})
	case "tracking_review":
		return makeOptions("tracking_integrity", "verify_primary_conversion_event", "audit_tracking_chain", "parallel_tracking_validation", [3]int64{25, 50, 100}, [3]int64{})
	case "delivery_review":
		return makeOptions("delivery_constraints", "review_delivery_restrictions", "controlled_constraint_test", "expanded_constraint_test", [3]int64{25, 50, 100}, [3]int64{})
	case "review_compliance":
		return makeOptions("review_compliance", "review_rejection_reason", "replace_flagged_material", "prepare_compliant_material_set", [3]int64{25, 50, 100}, [3]int64{})
	case "conversion_funnel_review":
		return makeOptions("conversion_funnel", "verify_conversion_event", "audit_conversion_funnel", "controlled_funnel_test", [3]int64{25, 50, 100}, [3]int64{-5, -10, -20})
	case "budget_pacing_review":
		return makeOptions("budget_pacing", "limit_budget_pacing", "tighten_budget_pacing", "strong_budget_pacing_control", [3]int64{5, 10, 20}, [3]int64{-5, -10, -20})
	case "cost_review":
		balancedReduction := clampInt64((cpaRatioBP-10000)/100, 10, 15)
		return makeOptions("cost_control", "limited_cost_control", "cpa_weighted_cost_control", "strong_cost_control", [3]int64{5, balancedReduction, 20}, [3]int64{-5, -balancedReduction, -20})
	default:
		return makeOptions("maintain_and_observe", "observe_short_window", "observe_full_horizon", "prepare_controlled_test", [3]int64{25, 50, 100}, [3]int64{})
	}
}

func budgetChangePercent(base, target int64) int64 {
	if base <= 0 {
		return 0
	}
	return (target - base) * 100 / base
}

func candidateAboveIntentBudgetFloor(dailyBudget int64, boundary IntentBudgetBoundary) bool {
	return boundary.MinimumDailyMinor == nil || dailyBudget >= *boundary.MinimumDailyMinor
}

func BuildBlockedDeliveryDecision(decisionID string, organizationID contract.OrganizationID, projectID contract.ProjectID, plan DeliveryPlan, code, explanation, nextAction, actor string, now time.Time) (DeliveryDecision, error) {
	configuration := plan.CurrentVersion.PlatformConfiguration
	intent := plan.CurrentVersion.DeliveryIntent
	if configuration == nil || intent == nil {
		return DeliveryDecision{}, ErrInvalidState
	}
	decision := DeliveryDecision{
		SchemaVersion: DeliveryDecisionSchemaV1, ID: decisionID, OrganizationID: organizationID, ProjectID: projectID, PolicyVersion: DeliveryDecisionPolicyV2,
		Diagnostic: DecisionDiagnostic{Code: code, Explanation: explanation, NextAction: nextAction},
		Inputs: DecisionInputBindings{
			PlanID: plan.ID, PlanVersion: plan.CurrentVersionNumber, PlanCanonicalHash: plan.CurrentVersion.CanonicalHash,
			IntentID: intent.IntentID, IntentVersion: intent.VersionNumber, IntentCanonicalHash: intent.CanonicalHash,
			ConfigurationID: configuration.ConfigurationID, ConfigurationVersion: configuration.VersionNumber, ConfigurationCanonicalHash: configuration.CanonicalHash,
			FactSnapshotRef: configuration.FactProvenance.SnapshotRef,
		},
		Candidates: []DeliveryDecisionCandidate{}, Evidence: append([]string(nil), configuration.FactProvenance.EvidenceRefs...), CreatedBy: actor, CreatedAt: now,
	}
	sort.Strings(decision.Evidence)
	hash, err := decision.ComputeCanonicalHash()
	if err != nil {
		return DeliveryDecision{}, err
	}
	decision.CanonicalHash = hash
	if err := decision.Validate(); err != nil {
		return DeliveryDecision{}, err
	}
	return decision, nil
}

func decisionMetricHash(metric DeliveryMetricSnapshot) (string, error) {
	return contract.CanonicalJSONHash(struct {
		ID               string                 `json:"id"`
		SimulationRunID  string                 `json:"simulation_run_id"`
		WindowSequence   int                    `json:"window_sequence"`
		DataThrough      time.Time              `json:"data_through"`
		RawMetrics       RawMetrics             `json:"raw_metrics"`
		CalculationBasis MetricCalculationBasis `json:"calculation_basis"`
	}{metric.ID, metric.SimulationRunID, metric.WindowSequence, metric.DataThrough, metric.RawMetrics, metric.CalculationBasis})
}

func candidateWithinIntentBudget(dailyBudget int64, boundary IntentBudgetBoundary) bool {
	ceiling := boundary.MaximumTotalMinor
	if boundary.MaximumDailyMinor != nil {
		ceiling = *boundary.MaximumDailyMinor
	}
	return ceiling >= 0 && dailyBudget <= ceiling
}

type WorkflowRisk string

const (
	WorkflowRiskObserve          WorkflowRisk = "observe"
	WorkflowRiskPrepareLocalForm WorkflowRisk = "prepare_local_form"
	WorkflowRiskRemoteWrite      WorkflowRisk = "remote_write"
)

type WorkflowField struct {
	Key              string                `json:"key"`
	Value            any                   `json:"value"`
	ExpectedReadback any                   `json:"expected_readback"`
	EvidenceRef      string                `json:"evidence_ref"`
	Control          *WorkflowFieldControl `json:"control,omitempty"`
	Treatment        string                `json:"treatment,omitempty"`
}

// WorkflowFieldControl is copied from the typed Manifest projection. It is
// descriptive only: this workflow still cannot execute a remote write.
type WorkflowFieldControl struct {
	Operation           string                      `json:"operation"`
	Scope               calibrationmanifest.Locator `json:"scope"`
	Target              calibrationmanifest.Locator `json:"target"`
	Readback            calibrationmanifest.Locator `json:"readback"`
	ExpectedTargetCount int                         `json:"expected_target_count"`
	Unit                string                      `json:"unit,omitempty"`
	ObservedOptions     []string                    `json:"observed_options,omitempty"`
	InputConstraints    map[string]any              `json:"input_constraints,omitempty"`
}

type ManifestWorkflowDiagnostic struct {
	FieldKey  string `json:"field_key"`
	Treatment string `json:"treatment"`
	State     string `json:"state"`
	Reason    string `json:"reason,omitempty"`
}

type CompiledWorkflowStep struct {
	ID             string          `json:"id"`
	Sequence       int             `json:"sequence"`
	Page           string          `json:"page"`
	Action         string          `json:"action"`
	Risk           WorkflowRisk    `json:"risk"`
	Preconditions  []string        `json:"preconditions"`
	Fields         []WorkflowField `json:"fields"`
	TimeoutSeconds int             `json:"timeout_seconds"`
	Recovery       string          `json:"recovery"`
	Blocked        bool            `json:"blocked"`
	BlockReason    string          `json:"block_reason,omitempty"`
}

type CompiledDeliveryWorkflow struct {
	SchemaVersion              string                       `json:"schema_version"`
	ID                         string                       `json:"id"`
	OrganizationID             contract.OrganizationID      `json:"organization_id"`
	ProjectID                  contract.ProjectID           `json:"project_id"`
	DecisionID                 string                       `json:"decision_id"`
	DecisionCanonicalHash      string                       `json:"decision_canonical_hash"`
	SelectedCandidateID        string                       `json:"selected_candidate_id"`
	ConfigurationCanonicalHash string                       `json:"configuration_canonical_hash"`
	ConfigurationID            string                       `json:"configuration_id"`
	ConfigurationVersion       int                          `json:"configuration_version"`
	Platform                   DeliveryPlatform             `json:"platform"`
	ProfileVersion             string                       `json:"profile_version"`
	AccountReference           StableReference              `json:"account_reference"`
	CapabilityContractVersion  string                       `json:"capability_contract_version"`
	SelectorContractVersion    string                       `json:"selector_contract_version"`
	ActionContractVersion      string                       `json:"action_contract_version"`
	ExecutionDriver            string                       `json:"execution_driver"`
	CalibrationManifest        CalibrationManifestBinding   `json:"calibration_manifest"`
	CompilerVersion            string                       `json:"compiler_version"`
	Status                     string                       `json:"status"`
	RemoteWriteEnabled         bool                         `json:"remote_write_enabled"`
	Steps                      []CompiledWorkflowStep       `json:"steps"`
	ManifestDiagnostics        []ManifestWorkflowDiagnostic `json:"manifest_diagnostics,omitempty"`
	CanonicalHash              string                       `json:"canonical_hash"`
	CreatedBy                  string                       `json:"created_by"`
	CreatedAt                  time.Time                    `json:"created_at"`
}

func (w CompiledDeliveryWorkflow) canonicalPayload() any {
	return struct {
		SchemaVersion              string                       `json:"schema_version"`
		DecisionID                 string                       `json:"decision_id"`
		DecisionCanonicalHash      string                       `json:"decision_canonical_hash"`
		SelectedCandidateID        string                       `json:"selected_candidate_id"`
		ConfigurationCanonicalHash string                       `json:"configuration_canonical_hash"`
		ConfigurationID            string                       `json:"configuration_id"`
		ConfigurationVersion       int                          `json:"configuration_version"`
		Platform                   DeliveryPlatform             `json:"platform"`
		ProfileVersion             string                       `json:"profile_version"`
		AccountReference           canonicalStableReference     `json:"account_reference"`
		CapabilityContractVersion  string                       `json:"capability_contract_version"`
		SelectorContractVersion    string                       `json:"selector_contract_version"`
		ActionContractVersion      string                       `json:"action_contract_version"`
		ExecutionDriver            string                       `json:"execution_driver"`
		CalibrationManifest        CalibrationManifestBinding   `json:"calibration_manifest"`
		CompilerVersion            string                       `json:"compiler_version"`
		Status                     string                       `json:"status"`
		RemoteWriteEnabled         bool                         `json:"remote_write_enabled"`
		Steps                      []CompiledWorkflowStep       `json:"steps"`
		ManifestDiagnostics        []ManifestWorkflowDiagnostic `json:"manifest_diagnostics,omitempty"`
	}{w.SchemaVersion, w.DecisionID, w.DecisionCanonicalHash, w.SelectedCandidateID, w.ConfigurationCanonicalHash, w.ConfigurationID, w.ConfigurationVersion, w.Platform, w.ProfileVersion, w.AccountReference.canonical(), w.CapabilityContractVersion, w.SelectorContractVersion, w.ActionContractVersion, w.ExecutionDriver, w.CalibrationManifest, w.CompilerVersion, w.Status, w.RemoteWriteEnabled, w.Steps, w.ManifestDiagnostics}
}

func (w CompiledDeliveryWorkflow) ComputeCanonicalHash() (string, error) {
	return contract.CanonicalJSONHash(w.canonicalPayload())
}

func (w CompiledDeliveryWorkflow) Validate() error {
	if w.SchemaVersion != CompiledDeliveryWorkflowSchemaV1 || strings.TrimSpace(w.ID) == "" || w.OrganizationID == "" || w.ProjectID == "" || strings.TrimSpace(w.DecisionID) == "" || strings.TrimSpace(w.SelectedCandidateID) == "" || w.CompilerVersion != DeliveryWorkflowCompilerV1 || w.Status != "ready_for_final_approval" || w.RemoteWriteEnabled ||
		w.Platform != DeliveryPlatformOceanEngine || w.ProfileVersion != OceanEngineConfigurationProfileV1 || w.CapabilityContractVersion != OceanEngineCapabilityContractV01 || w.SelectorContractVersion != OceanEngineSelectorContractV01 || w.ActionContractVersion != OceanEngineActionContractV01 || w.ExecutionDriver != OceanEngineExecutionDriverV1 {
		return ErrInvalidState
	}
	if w.CalibrationManifest.validate("calibration_manifest") != nil {
		return ErrInvalidState
	}
	if strings.TrimSpace(w.ConfigurationID) == "" || w.ConfigurationVersion < 1 || !isLowercaseSHA256(w.DecisionCanonicalHash) || !isLowercaseSHA256(w.ConfigurationCanonicalHash) || w.AccountReference.State != ReferenceResolved || w.AccountReference.validate("account_reference") != nil {
		return ErrInvalidState
	}
	hash, err := w.ComputeCanonicalHash()
	if err != nil || hash != w.CanonicalHash {
		return ErrApprovalContentMismatch
	}
	remoteWriteSteps := 0
	diagnostics := map[string]struct{}{}
	for _, diagnostic := range w.ManifestDiagnostics {
		if diagnostic.FieldKey == "" || diagnostic.Treatment == "" || diagnostic.State == "" || diagnostic.FieldKey+":"+diagnostic.Treatment+":"+diagnostic.State == "::" {
			return ErrInvalidState
		}
		key := diagnostic.FieldKey + ":" + diagnostic.Treatment
		if _, exists := diagnostics[key]; exists {
			return ErrInvalidState
		}
		diagnostics[key] = struct{}{}
	}
	for index, step := range w.Steps {
		if step.Sequence != index+1 {
			return ErrInvalidState
		}
		switch step.Risk {
		case WorkflowRiskObserve, WorkflowRiskPrepareLocalForm, WorkflowRiskRemoteWrite:
		default:
			return ErrInvalidState
		}
		if step.Risk == WorkflowRiskRemoteWrite {
			remoteWriteSteps++
			if !step.Blocked || step.BlockReason != "PHASE_C_REMOTE_WRITE_PROHIBITED" {
				return ErrInvalidState
			}
		}
		for _, field := range step.Fields {
			if field.Control == nil {
				continue
			}
			if field.Treatment != string(calibrationmanifest.Modelled) && field.Treatment != string(calibrationmanifest.DynamicReference) || field.Control.ExpectedTargetCount != 1 || !workflowLocatorValid(field.Control.Scope) || !workflowLocatorValid(field.Control.Target) || !workflowLocatorValid(field.Control.Readback) {
				return ErrInvalidState
			}
		}
	}
	if remoteWriteSteps != 1 {
		return ErrInvalidState
	}
	return nil
}

func CompileDeliveryWorkflow(workflowID string, decision DeliveryDecision, candidate DeliveryDecisionCandidate, actor string, now time.Time) (CompiledDeliveryWorkflow, error) {
	if err := decision.Validate(); err != nil {
		return CompiledDeliveryWorkflow{}, err
	}
	configuration := candidate.TargetConfiguration
	if decision.CanonicalHash == "" || configuration.Platform != DeliveryPlatformOceanEngine || configuration.Payload.OceanEngine == nil || configuration.Payload.OceanEngine.Project == nil {
		return CompiledDeliveryWorkflow{}, ErrInvalidState
	}
	candidateFound := false
	for _, frozen := range decision.Candidates {
		if frozen.ID == candidate.ID && frozen.TargetConfiguration.CanonicalHash == candidate.TargetConfiguration.CanonicalHash {
			candidateFound = true
			break
		}
	}
	if !candidateFound {
		return CompiledDeliveryWorkflow{}, ErrApprovalContentMismatch
	}
	project := configuration.Payload.OceanEngine.Project
	manifest, err := currentOceanEngineCalibrationManifest()
	if err != nil || manifest.ValidateBinding(configuration.Payload.OceanEngine.CalibrationManifest.SchemaVersion, configuration.Payload.OceanEngine.CalibrationManifest.ManifestID) != nil {
		return CompiledDeliveryWorkflow{}, ErrInvalidState
	}
	projections := manifest.Project(calibrationmanifest.CompiledDeliveryWorkflow, oceanEngineManifestFacts(project))
	diagnostics := workflowManifestDiagnostics(projections)
	steps := []CompiledWorkflowStep{
		{ID: "observe-project-context", Sequence: 1, Page: "oceanengine/project", Action: "observe_project_context", Risk: WorkflowRiskObserve, Preconditions: []string{"authenticated context is supplied only in a later execution phase", "page fingerprint must match the frozen calibration Manifest"}, Fields: []WorkflowField{}, TimeoutSeconds: 30, Recovery: "capture read-only evidence and stop on mismatch"},
	}
	for _, projection := range projections {
		if !projection.Executable {
			continue
		}
		sequence := len(steps) + 1
		steps = append(steps, CompiledWorkflowStep{
			ID: fmt.Sprintf("prepare-%s-local-form", strings.ReplaceAll(projection.Field.Key, ".", "-")), Sequence: sequence, Page: "oceanengine/" + projection.Field.PageFamily, Action: "prepare_local_form", Risk: WorkflowRiskPrepareLocalForm,
			Preconditions: []string{"page fingerprint matches the frozen Manifest", "scope, target, and readback each resolve once", "reference picker is loaded before inspection"}, Fields: []WorkflowField{workflowFieldFromProjection(projection)}, TimeoutSeconds: 60, Recovery: "discard local form state; no platform mutation exists",
		})
	}
	steps = append(steps, CompiledWorkflowStep{
		ID: "submit-platform-configuration", Sequence: len(steps) + 1, Page: "oceanengine/review", Action: "submit_platform_configuration", Risk: WorkflowRiskRemoteWrite,
		Preconditions: []string{"formal approval binds decision, configuration, and workflow hashes", "Phase D write-capable runtime is enabled"}, Fields: []WorkflowField{}, TimeoutSeconds: 0,
		Recovery: "not executable in Phase C", Blocked: true, BlockReason: "PHASE_C_REMOTE_WRITE_PROHIBITED",
	})
	workflow := CompiledDeliveryWorkflow{
		SchemaVersion: CompiledDeliveryWorkflowSchemaV1, ID: workflowID, OrganizationID: decision.OrganizationID, ProjectID: decision.ProjectID,
		DecisionID: decision.ID, DecisionCanonicalHash: decision.CanonicalHash, SelectedCandidateID: candidate.ID, ConfigurationCanonicalHash: configuration.CanonicalHash,
		ConfigurationID: configuration.ConfigurationID, ConfigurationVersion: configuration.VersionNumber, Platform: configuration.Platform, ProfileVersion: configuration.ProfileVersion,
		AccountReference: project.AccountReference, CapabilityContractVersion: OceanEngineCapabilityContractV01, SelectorContractVersion: OceanEngineSelectorContractV01, ActionContractVersion: OceanEngineActionContractV01, ExecutionDriver: OceanEngineExecutionDriverV1,
		CalibrationManifest: configuration.Payload.OceanEngine.CalibrationManifest, CompilerVersion: DeliveryWorkflowCompilerV1, Status: "ready_for_final_approval", RemoteWriteEnabled: false, Steps: steps, ManifestDiagnostics: diagnostics, CreatedBy: actor, CreatedAt: now,
	}
	hash, err := workflow.ComputeCanonicalHash()
	if err != nil {
		return CompiledDeliveryWorkflow{}, err
	}
	workflow.CanonicalHash = hash
	if err := workflow.Validate(); err != nil {
		return CompiledDeliveryWorkflow{}, err
	}
	return workflow, nil
}

func workflowFieldFromProjection(projection calibrationmanifest.FieldProjection) WorkflowField {
	control := projection.Field.PlaywrightRPA
	return WorkflowField{
		Key: projection.Field.Key, Value: nil, ExpectedReadback: nil, EvidenceRef: "manifest://" + projection.Field.Key, Treatment: string(projection.Mapping.Treatment),
		Control: &WorkflowFieldControl{Operation: control.Operation, Scope: control.Scope, Target: control.Target, Readback: control.Readback, ExpectedTargetCount: control.ExpectedTargetCount, Unit: projection.Field.Unit, ObservedOptions: append([]string(nil), control.ObservedOptions...), InputConstraints: control.InputConstraints},
	}
}

func workflowManifestDiagnostics(projections []calibrationmanifest.FieldProjection) []ManifestWorkflowDiagnostic {
	diagnostics := make([]ManifestWorkflowDiagnostic, 0, len(projections))
	for _, projection := range projections {
		state := "ready"
		if projection.Blocked {
			state = "blocked"
		} else if !projection.Executable {
			state = "evidence_only"
		}
		diagnostics = append(diagnostics, ManifestWorkflowDiagnostic{FieldKey: projection.Field.Key, Treatment: string(projection.Mapping.Treatment), State: state, Reason: projection.Reason})
	}
	return diagnostics
}

func workflowLocatorValid(locator calibrationmanifest.Locator) bool {
	if locator.Value == "" {
		return false
	}
	switch locator.Kind {
	case "role_name", "label", "visible_text", "placeholder", "attribute", "domain_key":
		return true
	default:
		return false
	}
}

func oceanEngineManifestFacts(project *OceanEngineProjectDraft) map[string]string {
	if project == nil {
		return map[string]string{}
	}
	facts := map[string]string{
		"marketing_purpose":      project.MarketingPurpose,
		"marketing_scenario":     project.MarketingScenario,
		"application_scenario":   project.ApplicationScenario,
		"operating_system":       project.OperatingSystem,
		"lead_capture_mode":      project.LeadCaptureMode,
		"delivery_mode":          project.DeliveryMode,
		"carrier":                project.Carrier,
		"deep_optimization_mode": project.DeepOptimizationMode,
		"bidding_strategy":       project.BudgetAndBidding.BiddingStrategy,
	}
	if project.MarketingProductReference != nil {
		facts["marketing_product_reference"] = "present"
	}
	if project.ApplicationReference != nil {
		facts["application_reference"] = "present"
	}
	if project.OptimizationTargetReference != nil {
		facts["optimization_target_reference"] = "present"
	}
	return facts
}

type DecisionSelection struct {
	ID                    string                   `json:"id"`
	OrganizationID        contract.OrganizationID  `json:"organization_id"`
	ProjectID             contract.ProjectID       `json:"project_id"`
	DecisionID            string                   `json:"decision_id"`
	DecisionCanonicalHash string                   `json:"decision_canonical_hash"`
	CandidateID           string                   `json:"candidate_id"`
	Configuration         PlatformConfiguration    `json:"configuration"`
	Workflow              CompiledDeliveryWorkflow `json:"workflow"`
	FinalApprovalBinding  FinalApprovalBinding     `json:"final_approval_binding"`
	CreatedBy             string                   `json:"created_by"`
	CreatedAt             time.Time                `json:"created_at"`
}

func (s DecisionSelection) Validate() error {
	if s.DecisionID == "" || s.CandidateID == "" || s.DecisionCanonicalHash == "" {
		return ErrInvalidRequest
	}
	configurationHash, err := s.Configuration.ComputeCanonicalHash()
	if err != nil || configurationHash != s.Configuration.CanonicalHash || s.Configuration.validateStructure() != nil {
		return ErrApprovalContentMismatch
	}
	if err := s.Workflow.Validate(); err != nil {
		return err
	}
	binding := s.FinalApprovalBinding
	if binding.Status != "ready_for_final_approval" || binding.Action != "remote_write" || !isLowercaseSHA256(binding.PlanCanonicalHash) || !isLowercaseSHA256(binding.IntentCanonicalHash) ||
		binding.DecisionCanonicalHash != s.DecisionCanonicalHash || binding.ConfigurationCanonicalHash != s.Configuration.CanonicalHash || binding.WorkflowCanonicalHash != s.Workflow.CanonicalHash ||
		s.Workflow.DecisionID != s.DecisionID || s.Workflow.DecisionCanonicalHash != s.DecisionCanonicalHash || s.Workflow.SelectedCandidateID != s.CandidateID || s.Workflow.ConfigurationCanonicalHash != s.Configuration.CanonicalHash {
		return ErrApprovalContentMismatch
	}
	return nil
}

// FinalApprovalBinding freezes the exact Phase D authority tuple without
// granting authority or creating an approval during Phase C.
type FinalApprovalBinding struct {
	Status                     string `json:"status"`
	Action                     string `json:"action"`
	PlanCanonicalHash          string `json:"plan_canonical_hash"`
	IntentCanonicalHash        string `json:"intent_canonical_hash"`
	DecisionCanonicalHash      string `json:"decision_canonical_hash"`
	ConfigurationCanonicalHash string `json:"configuration_canonical_hash"`
	WorkflowCanonicalHash      string `json:"workflow_canonical_hash"`
}

type SelectDecisionRequest struct {
	CandidateID     string `json:"candidate_id"`
	ExpectedVersion int    `json:"expected_plan_version"`
}

func (r SelectDecisionRequest) Validate() error {
	if strings.TrimSpace(r.CandidateID) == "" || r.ExpectedVersion < 1 {
		return ErrInvalidRequest
	}
	return nil
}

type decisionWorkflowRepository interface {
	CreateDecision(context.Context, DeliveryDecision) (DeliveryDecision, error)
	ListDecisions(context.Context, contract.OrganizationID, contract.ProjectID, int) ([]DeliveryDecision, error)
	GetDecision(context.Context, contract.OrganizationID, contract.ProjectID, string) (DeliveryDecision, error)
	CreateDecisionSelection(context.Context, DecisionSelection, string, string) (DecisionSelection, bool, error)
	GetDecisionSelection(context.Context, contract.OrganizationID, contract.ProjectID, string) (DecisionSelection, error)
}

func (s Service) decisionWorkflows() (decisionWorkflowRepository, error) {
	repository, ok := s.Repository.(decisionWorkflowRepository)
	if !ok {
		return nil, ErrUnsupportedConfigurationWorkflow
	}
	return repository, nil
}

func (s Service) GenerateDecision(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, planID string, expectedVersion int) (DeliveryDecision, error) {
	if err := s.ready(actor, projectID, ScopeWrite); err != nil {
		return DeliveryDecision{}, err
	}
	repository, err := s.decisionWorkflows()
	if err != nil {
		return DeliveryDecision{}, err
	}
	plan, err := s.Repository.GetPlan(ctx, actor.OrganizationID, projectID, planID)
	if err != nil {
		return DeliveryDecision{}, err
	}
	if plan.CurrentVersionNumber != expectedVersion || plan.CurrentVersion.ReadOnly || !plan.CurrentVersion.IsPlatformConfigurationV2() {
		return DeliveryDecision{}, ErrVersionConflict
	}
	decisionID, err := s.idGenerator()("deliverydecision")
	if err != nil {
		return DeliveryDecision{}, err
	}
	persistBlocked := func(code, explanation, nextAction string) (DeliveryDecision, error) {
		decision, buildErr := BuildBlockedDeliveryDecision(decisionID, actor.OrganizationID, projectID, plan, code, explanation, nextAction, actor.Principal.ID, s.now())
		if buildErr != nil {
			return DeliveryDecision{}, buildErr
		}
		return repository.CreateDecision(ctx, decision)
	}
	for _, reference := range plan.CurrentVersion.DeliveryIntent.Payload.MaterialReferences {
		if reference.State != ReferenceResolved {
			return persistBlocked("blocked_by_asset", "an immutable material reference is not resolved", "resolve or replace the blocked material reference")
		}
	}
	if observedAt := plan.CurrentVersion.PlatformConfiguration.FactProvenance.ObservedAt; !observedAt.IsZero() && s.now().Sub(observedAt) > 72*time.Hour {
		return persistBlocked("stale_data", "the bound platform fact snapshot is older than the decision policy freshness window", "refresh the platform fact snapshot")
	}
	for _, evidence := range plan.CurrentVersion.PlatformConfiguration.CompilationMetadata.FieldEvidence {
		if evidence.State == PlatformEvidencePending || evidence.State == PlatformEvidenceWriteValidationPending {
			return persistBlocked("platform_pending", "platform field evidence is not ready for a decision candidate", "complete platform field calibration")
		}
		if evidence.State == PlatformEvidenceBlockedByEventAsset {
			return persistBlocked("blocked_by_asset", "a required platform asset is unavailable", "resolve or replace the blocked asset")
		}
	}
	configuration := plan.CurrentVersion.PlatformConfiguration
	if configuration.Platform != DeliveryPlatformOceanEngine || configuration.Payload.OceanEngine == nil || configuration.Payload.OceanEngine.Project == nil {
		return persistBlocked("platform_pending", "the selected platform has no compiled decision policy", "wait for a calibrated platform decision adapter")
	}
	currentBudget := configuration.Payload.OceanEngine.Project.BudgetAndBidding.DailyBudgetMinor
	if !candidateAboveIntentBudgetFloor(currentBudget, plan.CurrentVersion.DeliveryIntent.Payload.BudgetBoundary) || !candidateWithinIntentBudget(currentBudget, plan.CurrentVersion.DeliveryIntent.Payload.BudgetBoundary) {
		return persistBlocked("platform_pending", "the current platform budget is outside the immutable intent boundary", "create a corrected platform configuration version")
	}
	simulationRepository, err := s.mechanisticSimulations()
	if err != nil {
		return DeliveryDecision{}, err
	}
	simulation, err := simulationRepository.GetLatestMechanisticSimulation(ctx, actor.OrganizationID, projectID, plan.ID, plan.CurrentVersionNumber)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return persistBlocked("insufficient_data", "no mechanistic simulation is available for the current plan version", "run the prelaunch probability simulation")
		}
		return DeliveryDecision{}, err
	}
	if simulation.Status != "completed" {
		return persistBlocked("platform_pending", "the mechanistic simulation cannot resolve all required platform references", "resolve the pending platform references and run the simulation again")
	}
	if len(simulation.MetricWindows) < 2 {
		return persistBlocked("insufficient_data", "at least two mechanistic metric windows are required", "run a simulation with a prediction horizon of at least two days")
	}
	baseline, err := mechanisticDecisionMetric(simulation, simulation.MetricWindows[0], actor.Principal.ID)
	if err != nil {
		return persistBlocked("insufficient_data", "the first mechanistic window has no usable median metrics", "review the explicit priors and run the simulation again")
	}
	current, err := mechanisticDecisionMetric(simulation, simulation.MetricWindows[len(simulation.MetricWindows)-1], actor.Principal.ID)
	if err != nil {
		return persistBlocked("insufficient_data", "the latest mechanistic window has no usable median metrics", "review the explicit priors and run the simulation again")
	}
	evidence := []string{
		"mechanistic-simulation://run/" + simulation.ID,
		"mechanistic-simulation://window/" + baseline.ID,
		"mechanistic-simulation://window/" + current.ID,
	}
	if ref := strings.TrimSpace(plan.CurrentVersion.PlatformConfiguration.FactProvenance.SnapshotRef); ref != "" {
		evidence = append(evidence, ref)
	}
	decision, err := BuildDeliveryDecision(DecisionEngineInput{
		DecisionID: decisionID, OrganizationID: actor.OrganizationID, ProjectID: projectID, Plan: plan,
		Simulation: OutcomeSimulationRun{ID: simulation.ID, PlanID: simulation.PlanID, PlanVersion: simulation.PlanVersion, InputHash: simulation.InputSnapshotHash},
		Baseline:   baseline, Current: current, Scenarios: simulation.ScenarioProbabilities, Recommendations: simulation.RecommendationDrafts,
		Evidence: evidence, CreatedBy: actor.Principal.ID, CreatedAt: s.now(),
	})
	if err != nil {
		return DeliveryDecision{}, err
	}
	return repository.CreateDecision(ctx, decision)
}

func mechanisticDecisionMetric(simulation MechanisticSimulationResult, window MechanisticMetricWindow, actor string) (DeliveryMetricSnapshot, error) {
	median := func(name string) (int64, error) {
		metric, ok := window.Metrics[name]
		if !ok || !metric.Available || metric.P50 == nil || !finite(*metric.P50) || *metric.P50 < 0 {
			return 0, ErrInvalidState
		}
		return int64(math.Round(*metric.P50)), nil
	}
	spend, err := median("spend")
	if err != nil {
		return DeliveryMetricSnapshot{}, err
	}
	impressions, err := median("impressions")
	if err != nil {
		return DeliveryMetricSnapshot{}, err
	}
	clicks, err := median("clicks")
	if err != nil {
		return DeliveryMetricSnapshot{}, err
	}
	conversions, err := median("true_conversions")
	if err != nil {
		return DeliveryMetricSnapshot{}, err
	}
	id := fmt.Sprintf("%s-window-%d", simulation.ID, window.Sequence)
	currency := strings.TrimSpace(simulation.PriorSet.CPM.Unit)
	if separator := strings.Index(currency, "_"); separator > 0 {
		currency = currency[:separator]
	}
	if currency == "" {
		return DeliveryMetricSnapshot{}, ErrInvalidState
	}
	return DeliveryMetricSnapshot{
		ID: id, OrganizationID: simulation.OrganizationID, ProjectID: simulation.ProjectID, SimulationRunID: simulation.ID, PlanID: simulation.PlanID,
		Source: "mechanistic_simulation", IsSimulated: true, DatasetVersion: simulation.SchemaVersion, FixtureVersion: simulation.PriorSetVersion,
		WindowSequence: window.Sequence, DataThrough: window.End, Currency: currency, WindowStart: window.Start, WindowEnd: window.End,
		RawMetrics:       RawMetrics{Impressions: impressions, Clicks: clicks, Conversions: conversions, SpendCents: spend},
		CalculationBasis: MetricCalculationBasis{Formula: "mechanistic simulation P50 projection", AppliedFactors: []OutcomeSimulationFactor{}, ScenarioEvidence: append([]string(nil), simulation.EvidenceRefs...)},
		CreatedBy:        actor, CreatedAt: window.End,
	}, nil
}

func (s Service) ListDecisions(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]DeliveryDecision, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return nil, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	repository, err := s.decisionWorkflows()
	if err != nil {
		return nil, err
	}
	return repository.ListDecisions(ctx, actor.OrganizationID, projectID, normalizeLimit(limit))
}

func (s Service) GetDecision(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (DeliveryDecision, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return DeliveryDecision{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return DeliveryDecision{}, err
	}
	repository, err := s.decisionWorkflows()
	if err != nil {
		return DeliveryDecision{}, err
	}
	return repository.GetDecision(ctx, actor.OrganizationID, projectID, id)
}

func (s Service) SelectDecision(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, decisionID, idempotencyKey string, request SelectDecisionRequest) (DecisionSelection, bool, error) {
	if err := s.ready(actor, projectID, ScopeWrite); err != nil {
		return DecisionSelection{}, false, err
	}
	if err := request.Validate(); err != nil || strings.TrimSpace(idempotencyKey) == "" {
		return DecisionSelection{}, false, ErrInvalidRequest
	}
	repository, err := s.decisionWorkflows()
	if err != nil {
		return DecisionSelection{}, false, err
	}
	decision, err := repository.GetDecision(ctx, actor.OrganizationID, projectID, decisionID)
	if err != nil {
		return DecisionSelection{}, false, err
	}
	if err := decision.Validate(); err != nil {
		return DecisionSelection{}, false, err
	}
	plan, err := s.Repository.GetPlan(ctx, actor.OrganizationID, projectID, decision.Inputs.PlanID)
	if err != nil {
		return DecisionSelection{}, false, err
	}
	if plan.CurrentVersionNumber != request.ExpectedVersion || request.ExpectedVersion != decision.Inputs.PlanVersion || plan.CurrentVersion.CanonicalHash != decision.Inputs.PlanCanonicalHash {
		return DecisionSelection{}, false, ErrVersionConflict
	}
	var selected *DeliveryDecisionCandidate
	for index := range decision.Candidates {
		if decision.Candidates[index].ID == request.CandidateID {
			selected = &decision.Candidates[index]
			break
		}
	}
	if selected == nil {
		return DecisionSelection{}, false, ErrInvalidRequest
	}
	selectionID, err := s.idGenerator()("deliverydecisionselect")
	if err != nil {
		return DecisionSelection{}, false, err
	}
	workflowID, err := s.idGenerator()("deliveryworkflow")
	if err != nil {
		return DecisionSelection{}, false, err
	}
	now := s.now()
	workflow, err := CompileDeliveryWorkflow(workflowID, decision, *selected, actor.Principal.ID, now)
	if err != nil {
		return DecisionSelection{}, false, err
	}
	selection := DecisionSelection{
		ID: selectionID, OrganizationID: actor.OrganizationID, ProjectID: projectID, DecisionID: decision.ID, DecisionCanonicalHash: decision.CanonicalHash,
		CandidateID: selected.ID, Configuration: selected.TargetConfiguration, Workflow: workflow,
		FinalApprovalBinding: FinalApprovalBinding{
			Status: "ready_for_final_approval", Action: "remote_write",
			PlanCanonicalHash: decision.Inputs.PlanCanonicalHash, IntentCanonicalHash: decision.Inputs.IntentCanonicalHash,
			DecisionCanonicalHash: decision.CanonicalHash, ConfigurationCanonicalHash: selected.TargetConfiguration.CanonicalHash, WorkflowCanonicalHash: workflow.CanonicalHash,
		},
		CreatedBy: actor.Principal.ID, CreatedAt: now,
	}
	if err := selection.Validate(); err != nil {
		return DecisionSelection{}, false, err
	}
	requestHash, err := contract.CanonicalJSONHash(struct {
		DecisionID string                `json:"decision_id"`
		Request    SelectDecisionRequest `json:"request"`
	}{decision.ID, request})
	if err != nil {
		return DecisionSelection{}, false, err
	}
	return repository.CreateDecisionSelection(ctx, selection, idempotencyKey, requestHash)
}

func (s Service) GetDecisionSelection(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (DecisionSelection, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return DecisionSelection{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return DecisionSelection{}, err
	}
	repository, err := s.decisionWorkflows()
	if err != nil {
		return DecisionSelection{}, err
	}
	return repository.GetDecisionSelection(ctx, actor.OrganizationID, projectID, id)
}
