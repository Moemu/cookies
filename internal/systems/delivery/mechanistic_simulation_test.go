package delivery

import (
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/connector"
)

func TestRunMechanisticSimulationConsumesAccountLaunchCalibration(t *testing.T) {
	input := mechanisticTestInput(100000)
	input.Schedule.EndAt = input.Schedule.StartAt.Add(8 * 24 * time.Hour)
	request := MechanisticSimulationRequest{StableSeed: "calibrated", SampleCount: 1000, PredictionHorizonDays: 7, ReviewState: SimulationReviewApproved, PriorSet: mechanisticTestPrior(), CalibrationAccountRef: "oeacct_abc"}
	request.LaunchBatchCalibration = &connector.LaunchBatchCalibrationSnapshot{
		ID: "oecal_test", SchemaVersion: connector.LaunchBatchCalibrationSchemaVersion, ModelVersion: connector.LaunchBatchModelVersion,
		Status: "ready_for_probabilistic_shadow", PayloadHash: "payload", BreakoutProbability: .2,
		Typical:  []connector.LaunchBatchScenarioMetricDistribution{{Metric: "spend_minor", P10: 100, P50: 200, P90: 300}, {Metric: "impressions", P10: 1000, P50: 2000, P90: 3000}, {Metric: "clicks", P10: 10, P50: 20, P90: 30}},
		Breakout: []connector.LaunchBatchScenarioMetricDistribution{{Metric: "spend_minor", P10: 10000, P50: 20000, P90: 30000}, {Metric: "impressions", P10: 100000, P50: 200000, P90: 300000}, {Metric: "clicks", P10: 1000, P50: 2000, P90: 3000}},
	}
	result, err := RunMechanisticSimulation(input, request)
	if err != nil {
		t.Fatal(err)
	}
	if result.CalibrationStatus != CalibrationStatusAccountProduct || result.CalibrationPriorRef != "oecal_test" || len(result.ScenarioProbabilities) != 2 || result.ScenarioProbabilities[1].Scenario != "breakout_launch" || result.ScenarioProbabilities[1].Probability != .2 {
		t.Fatalf("unexpected calibrated result: %#v", result)
	}
	if len(result.Alerts) != 0 || len(result.RecommendationDrafts) != 1 || result.RecommendationDrafts[0].RecommendationType != "portfolio_observation" {
		t.Fatalf("unsafe calibrated actions: alerts=%#v recommendations=%#v", result.Alerts, result.RecommendationDrafts)
	}
}

func mechanisticTestInput(budget int64) MechanisticSimulationInput {
	return MechanisticSimulationInput{
		PlanID: "plan_1", PlanVersion: 2, PlanCanonicalHash: string(make([]byte, 64)),
		IntentID: "intent_1", IntentVersion: 1, IntentCanonicalHash: string(make([]byte, 64)),
		ConfigurationID: "configuration_1", ConfigurationVersion: 1, ConfigurationCanonicalHash: string(make([]byte, 64)),
		ManifestBinding: CalibrationManifestBinding{SchemaVersion: OceanEngineCalibrationManifestV1, ManifestID: "oceanengine-calibration-current-test-account-2026-08-16"},
		BudgetMinor:     budget, Currency: "CNY", Schedule: OceanEngineSchedule{StartAt: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC), EndAt: time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC), Timezone: "Asia/Shanghai"},
		MarketingObjective: "conversion", OptimizationGoalState: "resolved", BiddingStrategy: "manual", ChargingMode: "ocpm", DeliveryMode: "standard", TargetingHash: string(make([]byte, 64)),
	}
}

func mechanisticTestPrior() SimulationPriorSet {
	scope := []string{"test_fixture"}
	probability := func(value float64) SimulationProbabilityPrior {
		return SimulationProbabilityPrior{Value: value, Source: "fixture://mechanistic-prior-v0", Unit: "probability", Scope: scope, Uncertainty: "test-only"}
	}
	ratio := func(minimum, mode, maximum float64) SimulationRangePrior {
		return SimulationRangePrior{Minimum: minimum, Mode: mode, Maximum: maximum, Source: "fixture://mechanistic-prior-v0", Unit: "ratio", Scope: scope, Uncertainty: "test-only"}
	}
	return SimulationPriorSet{
		Version: "mechanistic-test-prior/v0", ReviewPassProbability: probability(.9), DeliveryProbability: probability(.8),
		BudgetUtilization: ratio(.4, .7, 1), CPM: SimulationRangePrior{Minimum: 1000, Mode: 2000, Maximum: 3000, Source: "fixture://mechanistic-prior-v0", Unit: "CNY_minor_per_1000_impressions", Scope: scope, Uncertainty: "test-only"},
		CTR: ratio(.01, .03, .05), CVR: ratio(.01, .04, .08), TrackingObservableRate: probability(.8),
	}
}

func TestMechanisticSimulationDeterministicReplayAndBoundaries(t *testing.T) {
	request := MechanisticSimulationRequest{StableSeed: "stable-seed", SampleCount: 500, PredictionHorizonDays: 3, ReviewState: SimulationReviewUnknown, PriorSet: mechanisticTestPrior()}
	first, err := RunMechanisticSimulation(mechanisticTestInput(90000), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RunMechanisticSimulation(mechanisticTestInput(90000), request)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(first)
	secondJSON, _ := json.Marshal(second)
	if string(firstJSON) != string(secondJSON) {
		t.Fatal("same frozen input and seed did not replay exactly")
	}
	if !first.IsSimulated || first.CalibrationStatus != CalibrationStatusAssumptionDriven {
		t.Fatalf("unsafe provenance: %+v", first)
	}
	for _, draft := range first.RecommendationDrafts {
		if !draft.RequiresHumanReview || draft.EffectBasis == "causal_experiment" {
			t.Fatalf("unsafe recommendation: %+v", draft)
		}
	}
}

func TestMechanisticSimulationRejectedReviewProducesStructuralZeros(t *testing.T) {
	request := MechanisticSimulationRequest{StableSeed: "rejected", SampleCount: 100, PredictionHorizonDays: 1, ReviewState: SimulationReviewRejected, PriorSet: mechanisticTestPrior()}
	result, err := RunMechanisticSimulation(mechanisticTestInput(10000), request)
	if err != nil {
		t.Fatal(err)
	}
	window := result.MetricWindows[0]
	for _, name := range []string{"spend", "impressions", "clicks", "true_conversions", "observed_conversions"} {
		if window.Metrics[name].P90 == nil || *window.Metrics[name].P90 != 0 {
			t.Fatalf("%s was not structurally zero", name)
		}
	}
	for _, name := range []string{"cpm", "ctr", "cpc", "cvr", "cpa"} {
		if window.Metrics[name].Available {
			t.Fatalf("%s must be unavailable for a zero denominator", name)
		}
	}
}

func TestMechanisticSampleAtomicInvariants(t *testing.T) {
	request := MechanisticSimulationRequest{StableSeed: "invariants", SampleCount: 100, PredictionHorizonDays: 1, ReviewState: SimulationReviewApproved, PriorSet: mechanisticTestPrior()}
	seed := int64(44)
	rng := rand.New(rand.NewSource(seed)) // #nosec G404 -- deterministic test input.
	for i := 0; i < 1000; i++ {
		sample := generateMechanisticSample(rng, request, 10000, 10000, 0)
		if sample.spend > 10000 || sample.clicks > sample.impressions || sample.trueConversions > sample.clicks || sample.observedConversions > sample.trueConversions {
			t.Fatalf("invalid atomic sample: %+v", sample)
		}
		if sample.impressions == 0 && (sample.ctr != nil || sample.cpm != nil) {
			t.Fatalf("zero denominator emitted a ratio: %+v", sample)
		}
	}
}

func TestMechanisticPriorFailsClosed(t *testing.T) {
	request := MechanisticSimulationRequest{StableSeed: "missing-prior", SampleCount: 100, PredictionHorizonDays: 1, ReviewState: SimulationReviewUnknown, PriorSet: SimulationPriorSet{Version: "incomplete"}}
	if _, err := RunMechanisticSimulation(mechanisticTestInput(10000), request); err == nil {
		t.Fatal("missing critical prior did not fail closed")
	}
}

func TestMechanisticInputUsesFrozenV2ContractsAndReturnsPending(t *testing.T) {
	intent := validDeliveryIntent(t)
	configuration := validOceanEnginePlatformConfiguration(t, intent, 1)
	version := DeliveryPlanVersion{SchemaVersion: DeliveryPlanVersionSchemaV2, PlanID: "plan_1", VersionNumber: 1, CanonicalHash: configuration.CanonicalHash, DeliveryIntent: &intent, PlatformConfiguration: &configuration}
	input, status, err := BuildMechanisticSimulationInput(version)
	if err != nil {
		t.Fatal(err)
	}
	if status != "platform_pending" {
		t.Fatalf("unresolved optimization target status=%q", status)
	}
	if input.ConfigurationCanonicalHash != configuration.CanonicalHash || input.ManifestBinding != intent.Payload.CalibrationManifest || input.TargetingHash == "" {
		t.Fatalf("input did not bind frozen contracts: %+v", input)
	}
}

func TestMechanisticBudgetCapDoesNotDecreaseWithHigherBudget(t *testing.T) {
	request := MechanisticSimulationRequest{StableSeed: "budget-cap", SampleCount: 500, PredictionHorizonDays: 3, ReviewState: SimulationReviewApproved, PriorSet: mechanisticTestPrior()}
	lower, err := RunMechanisticSimulation(mechanisticTestInput(30000), request)
	if err != nil {
		t.Fatal(err)
	}
	higher, err := RunMechanisticSimulation(mechanisticTestInput(60000), request)
	if err != nil {
		t.Fatal(err)
	}
	if *higher.MetricWindows[0].Metrics["spend"].P90 < *lower.MetricWindows[0].Metrics["spend"].P90 {
		t.Fatal("a higher budget reduced the simulated spend cap")
	}
}

func TestMechanisticZeroTrackingRateSuppressesBudgetDrafts(t *testing.T) {
	prior := mechanisticTestPrior()
	prior.TrackingObservableRate.Value = 0
	request := MechanisticSimulationRequest{StableSeed: "tracking-zero", SampleCount: 500, PredictionHorizonDays: 1, ReviewState: SimulationReviewApproved, PriorSet: prior}
	input := mechanisticTestInput(30000)
	input.Schedule.EndAt = input.Schedule.StartAt.Add(24 * time.Hour)
	result, err := RunMechanisticSimulation(input, request)
	if err != nil {
		t.Fatal(err)
	}
	for _, draft := range result.RecommendationDrafts {
		if strings.Contains(draft.TargetField, "budget") || strings.Contains(draft.TargetField, "bid") {
			t.Fatalf("tracking anomaly produced a budget or bid draft: %+v", draft)
		}
	}
}

func TestMechanisticCreativeRiskUsesParallelPortfolioNotRotation(t *testing.T) {
	drafts := draftMechanisticRecommendations([]SimulationScenarioProbability{{Scenario: "creative_fatigue", Probability: .8}}, mechanisticTestInput(30000), []string{"simulation://test"})
	if len(drafts) != 1 || drafts[0].RecommendationType != "portfolio_test" || drafts[0].TargetField != "parallel_project_promotion_portfolio" {
		t.Fatalf("unexpected creative recommendation: %#v", drafts)
	}
	if strings.Contains(strings.ToLower(drafts[0].Rationale), "rotation") {
		t.Fatalf("creative recommendation still instructs rotation: %#v", drafts[0])
	}
}
