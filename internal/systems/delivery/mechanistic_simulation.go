package delivery

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/delivery/calibrationmanifest"
)

const (
	MechanisticSimulationSchemaVersion = "delivery-mechanistic-simulation/v0"
	MechanisticSimulationModelVersion  = "delivery-mechanistic-monte-carlo/v0.1"
	MechanisticThresholdVersion        = "delivery-mechanistic-scenarios/v0"
	CalibrationStatusAssumptionDriven  = "assumption_driven"
)

type SimulationReviewState string

const (
	SimulationReviewUnknown  SimulationReviewState = "unknown"
	SimulationReviewApproved SimulationReviewState = "approved"
	SimulationReviewRejected SimulationReviewState = "rejected"
)

type SimulationProbabilityPrior struct {
	Value       float64  `json:"value"`
	Source      string   `json:"source"`
	Unit        string   `json:"unit"`
	Scope       []string `json:"scope"`
	Uncertainty string   `json:"uncertainty"`
}

type SimulationRangePrior struct {
	Minimum     float64  `json:"minimum"`
	Mode        float64  `json:"mode"`
	Maximum     float64  `json:"maximum"`
	Source      string   `json:"source"`
	Unit        string   `json:"unit"`
	Scope       []string `json:"scope"`
	Uncertainty string   `json:"uncertainty"`
}

type SimulationFatiguePrior struct {
	Enabled     bool     `json:"enabled"`
	DailyRate   float64  `json:"daily_rate"`
	Source      string   `json:"source"`
	Unit        string   `json:"unit"`
	Scope       []string `json:"scope"`
	Uncertainty string   `json:"uncertainty"`
}

type SimulationPriorSet struct {
	Version                string                     `json:"version"`
	ReviewPassProbability  SimulationProbabilityPrior `json:"review_pass_probability"`
	DeliveryProbability    SimulationProbabilityPrior `json:"delivery_probability"`
	BudgetUtilization      SimulationRangePrior       `json:"budget_utilization"`
	CPM                    SimulationRangePrior       `json:"cpm"`
	CTR                    SimulationRangePrior       `json:"ctr"`
	CVR                    SimulationRangePrior       `json:"cvr"`
	TrackingObservableRate SimulationProbabilityPrior `json:"tracking_observable_rate"`
	CreativeFatigue        *SimulationFatiguePrior    `json:"creative_fatigue,omitempty"`
}

func (p SimulationPriorSet) Validate(currency string) error {
	if strings.TrimSpace(p.Version) == "" {
		return fmt.Errorf("%w: prior set version is required", ErrInvalidRequest)
	}
	probabilities := []SimulationProbabilityPrior{p.ReviewPassProbability, p.DeliveryProbability, p.TrackingObservableRate}
	for _, value := range probabilities {
		if !finite(value.Value) || value.Value < 0 || value.Value > 1 || strings.TrimSpace(value.Source) == "" || value.Unit != "probability" || len(value.Scope) == 0 || strings.TrimSpace(value.Uncertainty) == "" {
			return fmt.Errorf("%w: incomplete probability prior", ErrInvalidRequest)
		}
	}
	ranges := []SimulationRangePrior{p.BudgetUtilization, p.CPM, p.CTR, p.CVR}
	for _, value := range ranges {
		if !finite(value.Minimum) || !finite(value.Mode) || !finite(value.Maximum) || value.Minimum < 0 || value.Mode < value.Minimum || value.Maximum < value.Mode || strings.TrimSpace(value.Source) == "" || len(value.Scope) == 0 || strings.TrimSpace(value.Uncertainty) == "" {
			return fmt.Errorf("%w: incomplete range prior", ErrInvalidRequest)
		}
	}
	if p.BudgetUtilization.Maximum > 1 || p.CTR.Maximum > 1 || p.CVR.Maximum > 1 || p.BudgetUtilization.Unit != "ratio" || p.CTR.Unit != "ratio" || p.CVR.Unit != "ratio" {
		return fmt.Errorf("%w: ratio prior is outside [0,1]", ErrInvalidRequest)
	}
	if p.CPM.Minimum <= 0 || p.CPM.Unit != currency+"_minor_per_1000_impressions" {
		return fmt.Errorf("%w: CPM prior currency does not match input", ErrInvalidRequest)
	}
	if p.CreativeFatigue != nil && (strings.TrimSpace(p.CreativeFatigue.Source) == "" || p.CreativeFatigue.Unit != "ratio_per_day" || len(p.CreativeFatigue.Scope) == 0 || strings.TrimSpace(p.CreativeFatigue.Uncertainty) == "" || p.CreativeFatigue.DailyRate < 0 || p.CreativeFatigue.DailyRate > 1) {
		return fmt.Errorf("%w: incomplete creative fatigue prior", ErrInvalidRequest)
	}
	return nil
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

type MechanisticSimulationRequest struct {
	StableSeed            string                `json:"stable_seed"`
	SampleCount           int                   `json:"sample_count"`
	PredictionHorizonDays int                   `json:"prediction_horizon_days"`
	ReviewState           SimulationReviewState `json:"review_state"`
	PriorSet              SimulationPriorSet    `json:"prior_set"`
}

func (r MechanisticSimulationRequest) Validate(currency string) error {
	if strings.TrimSpace(r.StableSeed) == "" || r.SampleCount < 100 || r.SampleCount > 100000 || r.PredictionHorizonDays < 1 || r.PredictionHorizonDays > 31 || r.SampleCount*r.PredictionHorizonDays > 500000 {
		return ErrInvalidRequest
	}
	if r.ReviewState != SimulationReviewUnknown && r.ReviewState != SimulationReviewApproved && r.ReviewState != SimulationReviewRejected {
		return ErrInvalidRequest
	}
	return r.PriorSet.Validate(currency)
}

type MechanisticSimulationInput struct {
	PlanID                     string                     `json:"plan_id"`
	PlanVersion                int                        `json:"plan_version"`
	PlanCanonicalHash          string                     `json:"plan_canonical_hash"`
	IntentID                   string                     `json:"intent_id"`
	IntentVersion              int                        `json:"intent_version"`
	IntentCanonicalHash        string                     `json:"intent_canonical_hash"`
	ConfigurationID            string                     `json:"configuration_id"`
	ConfigurationVersion       int                        `json:"configuration_version"`
	ConfigurationCanonicalHash string                     `json:"configuration_canonical_hash"`
	ManifestBinding            CalibrationManifestBinding `json:"manifest_binding"`
	BudgetMinor                int64                      `json:"budget_minor"`
	BudgetMode                 string                     `json:"budget_mode"`
	DailyBudgetMinor           int64                      `json:"daily_budget_minor"`
	Currency                   string                     `json:"currency"`
	Schedule                   OceanEngineSchedule        `json:"schedule"`
	MarketingObjective         string                     `json:"marketing_objective"`
	OptimizationGoalState      string                     `json:"optimization_goal_state"`
	BiddingStrategy            string                     `json:"bidding_strategy"`
	ChargingMode               string                     `json:"charging_mode"`
	BidMinor                   *int64                     `json:"bid_minor,omitempty"`
	DeliveryMode               string                     `json:"delivery_mode"`
	TargetingHash              string                     `json:"targeting_hash"`
	MaterialReferences         []StableReference          `json:"material_references"`
}

type SimulationQuantiles struct {
	Available bool     `json:"available"`
	Unit      string   `json:"unit"`
	P10       *float64 `json:"p10,omitempty"`
	P50       *float64 `json:"p50,omitempty"`
	P90       *float64 `json:"p90,omitempty"`
	Mean      *float64 `json:"mean,omitempty"`
}

type MechanisticMetricWindow struct {
	Sequence int                            `json:"sequence"`
	Start    time.Time                      `json:"start"`
	End      time.Time                      `json:"end"`
	Timezone string                         `json:"timezone"`
	Metrics  map[string]SimulationQuantiles `json:"metrics"`
}

type SimulationScenarioProbability struct {
	Scenario         string   `json:"scenario"`
	ThresholdVersion string   `json:"threshold_version"`
	Probability      float64  `json:"probability"`
	Status           string   `json:"status"`
	EvidenceRefs     []string `json:"evidence_refs"`
	Limitations      []string `json:"limitations"`
}

type MechanisticSimulationAlert struct {
	Type         string   `json:"type"`
	Severity     string   `json:"severity"`
	Probability  float64  `json:"probability"`
	EvidenceRefs []string `json:"evidence_refs"`
	Limitations  []string `json:"limitations"`
}

type SimulationRecommendationDraft struct {
	RecommendationType  string     `json:"recommendation_type"`
	TargetField         string     `json:"target_field"`
	CurrentValue        any        `json:"current_value"`
	SuggestedRange      [2]float64 `json:"suggested_range"`
	ExpectedEffectRange [2]float64 `json:"expected_effect_range"`
	Confidence          string     `json:"confidence"`
	EffectBasis         string     `json:"effect_basis"`
	Rationale           string     `json:"rationale"`
	EvidenceRefs        []string   `json:"evidence_refs"`
	Risks               []string   `json:"risks"`
	Guardrails          []string   `json:"guardrails"`
	RequiresHumanReview bool       `json:"requires_human_review"`
}

type MechanisticSimulationResult struct {
	ID                    string                          `json:"id"`
	OrganizationID        contract.OrganizationID         `json:"organization_id"`
	ProjectID             contract.ProjectID              `json:"project_id"`
	PlanID                string                          `json:"plan_id"`
	PlanVersion           int                             `json:"plan_version"`
	SchemaVersion         string                          `json:"schema_version"`
	ModelVersion          string                          `json:"model_version"`
	PriorSetVersion       string                          `json:"prior_set_version"`
	PriorSet              SimulationPriorSet              `json:"prior_set"`
	ManifestBinding       CalibrationManifestBinding      `json:"manifest_binding"`
	InputSnapshotHash     string                          `json:"input_snapshot_hash"`
	StableSeed            string                          `json:"stable_seed"`
	PredictionHorizon     string                          `json:"prediction_horizon"`
	SampleCount           int                             `json:"sample_count"`
	MetricWindows         []MechanisticMetricWindow       `json:"metric_windows"`
	ScenarioProbabilities []SimulationScenarioProbability `json:"scenario_probabilities"`
	Alerts                []MechanisticSimulationAlert    `json:"alerts"`
	RecommendationDrafts  []SimulationRecommendationDraft `json:"recommendation_drafts"`
	Assumptions           []string                        `json:"assumptions"`
	Limitations           []string                        `json:"limitations"`
	EvidenceRefs          []string                        `json:"evidence_refs"`
	Status                string                          `json:"status"`
	IsSimulated           bool                            `json:"is_simulated"`
	CalibrationStatus     string                          `json:"calibration_status"`
}

type MechanisticSimulationEnvelope struct {
	Result MechanisticSimulationResult `json:"result"`
	Replay bool                        `json:"replay"`
}

type mechanisticSimulationRepository interface {
	CreateOrGetMechanisticSimulation(context.Context, MechanisticSimulationResult, string) (MechanisticSimulationResult, bool, error)
	GetMechanisticSimulation(context.Context, contract.OrganizationID, contract.ProjectID, string) (MechanisticSimulationResult, error)
	GetLatestMechanisticSimulation(context.Context, contract.OrganizationID, contract.ProjectID, string, int) (MechanisticSimulationResult, error)
}

func (s Service) mechanisticSimulations() (mechanisticSimulationRepository, error) {
	repository, ok := s.Repository.(mechanisticSimulationRepository)
	if !ok {
		return nil, ErrUnsupportedConfigurationWorkflow
	}
	return repository, nil
}

func (s Service) CreatePrelaunchMechanisticSimulation(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, planID string, versionNumber int, request MechanisticSimulationRequest) (MechanisticSimulationEnvelope, error) {
	if err := s.ready(actor, projectID, ScopeWrite); err != nil {
		return MechanisticSimulationEnvelope{}, err
	}
	if strings.TrimSpace(planID) == "" || versionNumber < 1 {
		return MechanisticSimulationEnvelope{}, ErrInvalidRequest
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return MechanisticSimulationEnvelope{}, err
	}
	version, err := s.Repository.GetPlanVersion(ctx, actor.OrganizationID, projectID, planID, versionNumber)
	if err != nil {
		return MechanisticSimulationEnvelope{}, err
	}
	input, status, err := BuildMechanisticSimulationInput(version)
	if err != nil {
		return MechanisticSimulationEnvelope{}, err
	}
	if err := request.Validate(input.Currency); err != nil {
		return MechanisticSimulationEnvelope{}, err
	}
	if status == "platform_pending" {
		inputHash, hashErr := contract.CanonicalJSONHash(input)
		if hashErr != nil {
			return MechanisticSimulationEnvelope{}, hashErr
		}
		return MechanisticSimulationEnvelope{Result: MechanisticSimulationResult{OrganizationID: actor.OrganizationID, ProjectID: projectID, PlanID: planID, PlanVersion: versionNumber, SchemaVersion: MechanisticSimulationSchemaVersion, ModelVersion: MechanisticSimulationModelVersion, PriorSetVersion: request.PriorSet.Version, PriorSet: request.PriorSet, ManifestBinding: input.ManifestBinding, InputSnapshotHash: inputHash, StableSeed: request.StableSeed, PredictionHorizon: fmt.Sprintf("P%dD", request.PredictionHorizonDays), SampleCount: request.SampleCount, MetricWindows: []MechanisticMetricWindow{}, ScenarioProbabilities: []SimulationScenarioProbability{}, Alerts: []MechanisticSimulationAlert{}, RecommendationDrafts: []SimulationRecommendationDraft{}, Assumptions: []string{"All distributions are supplied by an explicit prior set."}, Limitations: []string{"A required dynamic reference is unresolved."}, EvidenceRefs: []string{"plan-version://" + planID + fmt.Sprintf("/%d", versionNumber)}, Status: "platform_pending", IsSimulated: true, CalibrationStatus: CalibrationStatusAssumptionDriven}}, nil
	}
	result, err := RunMechanisticSimulation(input, request)
	if err != nil {
		return MechanisticSimulationEnvelope{}, err
	}
	fingerprint, err := contract.CanonicalJSONHash(struct {
		InputHash       string `json:"input_hash"`
		PriorSetVersion string `json:"prior_set_version"`
		StableSeed      string `json:"stable_seed"`
		Horizon         string `json:"horizon"`
		SampleCount     int    `json:"sample_count"`
		ModelVersion    string `json:"model_version"`
	}{result.InputSnapshotHash, result.PriorSetVersion, result.StableSeed, result.PredictionHorizon, result.SampleCount, result.ModelVersion})
	if err != nil {
		return MechanisticSimulationEnvelope{}, err
	}
	result.ID = "deliverymechanisticsimulation_" + fingerprint[:24]
	result.OrganizationID = actor.OrganizationID
	result.ProjectID = projectID
	result.PlanID = planID
	result.PlanVersion = versionNumber
	repository, err := s.mechanisticSimulations()
	if err != nil {
		return MechanisticSimulationEnvelope{}, err
	}
	stored, replay, err := repository.CreateOrGetMechanisticSimulation(ctx, result, fingerprint)
	return MechanisticSimulationEnvelope{Result: stored, Replay: replay}, err
}

func (s Service) GetPrelaunchMechanisticSimulation(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (MechanisticSimulationResult, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return MechanisticSimulationResult{}, err
	}
	if strings.TrimSpace(id) == "" {
		return MechanisticSimulationResult{}, ErrInvalidRequest
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return MechanisticSimulationResult{}, err
	}
	repository, err := s.mechanisticSimulations()
	if err != nil {
		return MechanisticSimulationResult{}, err
	}
	return repository.GetMechanisticSimulation(ctx, actor.OrganizationID, projectID, id)
}

func (s Service) GetLatestPrelaunchMechanisticSimulation(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, planID string, version int) (MechanisticSimulationResult, error) {
	if err := s.ready(actor, projectID, ScopeRead); err != nil {
		return MechanisticSimulationResult{}, err
	}
	if strings.TrimSpace(planID) == "" || version < 1 {
		return MechanisticSimulationResult{}, ErrInvalidRequest
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return MechanisticSimulationResult{}, err
	}
	repository, err := s.mechanisticSimulations()
	if err != nil {
		return MechanisticSimulationResult{}, err
	}
	return repository.GetLatestMechanisticSimulation(ctx, actor.OrganizationID, projectID, planID, version)
}

type mechanisticSample struct {
	spend, impressions, clicks, trueConversions, observedConversions int64
	cpm, ctr, cpc, cvr, cpa                                          *float64
	reviewRejected, deliveryClosed                                   bool
}

func BuildMechanisticSimulationInput(version DeliveryPlanVersion) (MechanisticSimulationInput, string, error) {
	if !version.IsPlatformConfigurationV2() || version.PlatformConfiguration.Platform != DeliveryPlatformOceanEngine || version.PlatformConfiguration.Payload.OceanEngine == nil || version.PlatformConfiguration.Payload.OceanEngine.Project == nil {
		return MechanisticSimulationInput{}, "blocked", ErrLegacyConfigurationUnsupported
	}
	intent := version.DeliveryIntent
	configuration := version.PlatformConfiguration
	if err := intent.Validate(); err != nil {
		return MechanisticSimulationInput{}, "blocked", err
	}
	if err := configuration.Validate(); err != nil {
		return MechanisticSimulationInput{}, "blocked", err
	}
	project := configuration.Payload.OceanEngine.Project
	refs := append([]StableReference(nil), intent.Payload.MaterialReferences...)
	refs = append(refs, intent.Payload.StrategyReference)
	if project.OptimizationTargetReference != nil {
		refs = append(refs, *project.OptimizationTargetReference)
	}
	pending := false
	for _, ref := range refs {
		if ref.State != ReferenceResolved {
			pending = true
		}
	}
	targetingHash, err := contract.CanonicalJSONHash(project.Targeting)
	if err != nil {
		return MechanisticSimulationInput{}, "blocked", err
	}
	goalState := "not_configured"
	if project.OptimizationTargetReference != nil {
		goalState = "resolved"
	}
	result := MechanisticSimulationInput{
		PlanID: version.PlanID, PlanVersion: version.VersionNumber, PlanCanonicalHash: version.CanonicalHash,
		IntentID: intent.IntentID, IntentVersion: intent.VersionNumber, IntentCanonicalHash: intent.CanonicalHash,
		ConfigurationID: configuration.ConfigurationID, ConfigurationVersion: configuration.VersionNumber, ConfigurationCanonicalHash: configuration.CanonicalHash,
		ManifestBinding: intent.Payload.CalibrationManifest, BudgetMinor: intent.Payload.BudgetBoundary.MaximumTotalMinor,
		BudgetMode: project.BudgetAndBidding.BudgetMode, DailyBudgetMinor: project.BudgetAndBidding.DailyBudgetMinor,
		Currency: intent.Payload.BudgetBoundary.Currency, Schedule: project.Schedule, MarketingObjective: intent.Payload.MarketingObjective,
		OptimizationGoalState: goalState, BiddingStrategy: project.BudgetAndBidding.BiddingStrategy, ChargingMode: project.BudgetAndBidding.ChargingMode,
		BidMinor: project.BudgetAndBidding.BidMinor, DeliveryMode: project.DeliveryMode, TargetingHash: targetingHash, MaterialReferences: refs,
	}
	if pending {
		return result, "platform_pending", nil
	}
	return result, "ready", nil
}

func RunMechanisticSimulation(input MechanisticSimulationInput, request MechanisticSimulationRequest) (MechanisticSimulationResult, error) {
	base := MechanisticSimulationResult{SchemaVersion: MechanisticSimulationSchemaVersion, ModelVersion: MechanisticSimulationModelVersion, PriorSetVersion: request.PriorSet.Version, PriorSet: request.PriorSet, ManifestBinding: input.ManifestBinding, StableSeed: request.StableSeed, SampleCount: request.SampleCount, IsSimulated: true, CalibrationStatus: CalibrationStatusAssumptionDriven, Status: "completed"}
	if err := request.Validate(input.Currency); err != nil {
		return base, err
	}
	if input.Schedule.StartAt.IsZero() || !input.Schedule.EndAt.After(input.Schedule.StartAt) || input.Schedule.StartAt.Add(time.Duration(request.PredictionHorizonDays)*24*time.Hour).After(input.Schedule.EndAt) {
		return base, fmt.Errorf("%w: prediction horizon exceeds the frozen schedule", ErrInvalidRequest)
	}
	manifest, err := calibrationmanifest.Current()
	if err != nil || manifest.ValidateBinding(input.ManifestBinding.SchemaVersion, input.ManifestBinding.ManifestID) != nil {
		return base, fmt.Errorf("%w: frozen calibration Manifest binding is unavailable", ErrInvalidRequest)
	}
	inputHash, err := contract.CanonicalJSONHash(input)
	if err != nil {
		return base, err
	}
	base.InputSnapshotHash = inputHash
	base.PredictionHorizon = fmt.Sprintf("P%dD", request.PredictionHorizonDays)
	base.EvidenceRefs = []string{"plan-version://" + input.PlanID + fmt.Sprintf("/%d", input.PlanVersion), "calibration-manifest://" + input.ManifestBinding.ManifestID, "simulation-prior://" + request.PriorSet.Version}
	base.Assumptions = []string{"All distributions are supplied by an explicit prior set.", "The model is predictive association only.", "All metric summaries use one Monte Carlo sample batch."}
	base.Limitations = []string{"No real Connector history is read.", "No model training or causal inference is done.", "Results require human review."}

	seedMaterial := sha256.Sum256([]byte(inputHash + "\x00" + request.PriorSet.Version + "\x00" + request.StableSeed + "\x00" + MechanisticSimulationModelVersion))
	rng := rand.New(rand.NewSource(int64(binary.BigEndian.Uint64(seedMaterial[:8])))) // #nosec G404 -- deterministic simulation, not security.
	windows := make([][]mechanisticSample, request.PredictionHorizonDays)
	for day := range windows {
		windows[day] = make([]mechanisticSample, request.SampleCount)
	}
	remainingBudget := make([]int64, request.SampleCount)
	for i := range remainingBudget {
		remainingBudget[i] = input.BudgetMinor
	}
	dailyCap := input.BudgetMinor
	if request.PredictionHorizonDays > 0 {
		dailyCap = (input.BudgetMinor + int64(request.PredictionHorizonDays) - 1) / int64(request.PredictionHorizonDays)
	}
	if input.BudgetMode == OceanEngineBudgetModeDaily && input.DailyBudgetMinor > 0 && input.DailyBudgetMinor < dailyCap {
		dailyCap = input.DailyBudgetMinor
	}
	for day := range windows {
		for i := 0; i < request.SampleCount; i++ {
			sample := generateMechanisticSample(rng, request, dailyCap, remainingBudget[i], day)
			remainingBudget[i] -= sample.spend
			windows[day][i] = sample
		}
		base.MetricWindows = append(base.MetricWindows, summarizeMechanisticWindow(input.Schedule.StartAt.Add(time.Duration(day)*24*time.Hour), input.Schedule.Timezone, day+1, windows[day], input.Currency))
	}
	all := flattenMechanisticSamples(windows)
	base.ScenarioProbabilities = detectMechanisticScenarios(all, request, dailyCap, base.EvidenceRefs)
	base.Alerts = scenarioAlerts(base.ScenarioProbabilities)
	base.RecommendationDrafts = draftMechanisticRecommendations(base.ScenarioProbabilities, input, base.EvidenceRefs)
	return base, nil
}

func generateMechanisticSample(rng *rand.Rand, request MechanisticSimulationRequest, dailyCap, remaining int64, day int) mechanisticSample {
	s := mechanisticSample{}
	if request.ReviewState == SimulationReviewRejected || (request.ReviewState == SimulationReviewUnknown && rng.Float64() > request.PriorSet.ReviewPassProbability.Value) {
		s.reviewRejected = true
		return s
	}
	if rng.Float64() > request.PriorSet.DeliveryProbability.Value {
		s.deliveryClosed = true
		return s
	}
	utilization := triangular(rng, request.PriorSet.BudgetUtilization)
	cap := dailyCap
	if remaining < cap {
		cap = remaining
	}
	s.spend = int64(math.Round(float64(cap) * utilization))
	if s.spend > cap {
		s.spend = cap
	}
	sampledCPM := triangular(rng, request.PriorSet.CPM)
	if s.spend > 0 {
		s.impressions = int64(math.Floor(float64(s.spend) * 1000 / sampledCPM))
	}
	ctr := triangular(rng, request.PriorSet.CTR)
	if request.PriorSet.CreativeFatigue != nil && request.PriorSet.CreativeFatigue.Enabled {
		ctr *= math.Pow(1-request.PriorSet.CreativeFatigue.DailyRate, float64(day))
	}
	s.clicks = binomial(rng, s.impressions, ctr)
	cvr := triangular(rng, request.PriorSet.CVR)
	s.trueConversions = binomial(rng, s.clicks, cvr)
	s.observedConversions = binomial(rng, s.trueConversions, request.PriorSet.TrackingObservableRate.Value)
	if s.impressions > 0 {
		s.ctr = floatPtr(float64(s.clicks) / float64(s.impressions))
		s.cpm = floatPtr(float64(s.spend) * 1000 / float64(s.impressions))
	}
	if s.clicks > 0 {
		s.cpc = floatPtr(float64(s.spend) / float64(s.clicks))
		s.cvr = floatPtr(float64(s.trueConversions) / float64(s.clicks))
	}
	if s.trueConversions > 0 {
		s.cpa = floatPtr(float64(s.spend) / float64(s.trueConversions))
	}
	return s
}

func triangular(rng *rand.Rand, p SimulationRangePrior) float64 {
	if p.Minimum == p.Maximum {
		return p.Minimum
	}
	u := rng.Float64()
	f := (p.Mode - p.Minimum) / (p.Maximum - p.Minimum)
	if u < f {
		return p.Minimum + math.Sqrt(u*(p.Maximum-p.Minimum)*(p.Mode-p.Minimum))
	}
	return p.Maximum - math.Sqrt((1-u)*(p.Maximum-p.Minimum)*(p.Maximum-p.Mode))
}

func binomial(rng *rand.Rand, n int64, probability float64) int64 {
	if n <= 0 || probability <= 0 {
		return 0
	}
	if probability >= 1 {
		return n
	}
	if n <= 100 {
		var result int64
		for i := int64(0); i < n; i++ {
			if rng.Float64() < probability {
				result++
			}
		}
		return result
	}
	mean := float64(n) * probability
	standardDeviation := math.Sqrt(float64(n) * probability * (1 - probability))
	result := int64(math.Round(mean + rng.NormFloat64()*standardDeviation))
	if result < 0 {
		return 0
	}
	if result > n {
		return n
	}
	return result
}

func summarizeMechanisticWindow(start time.Time, timezone string, sequence int, samples []mechanisticSample, currency string) MechanisticMetricWindow {
	metrics := map[string][]float64{"spend": {}, "impressions": {}, "clicks": {}, "true_conversions": {}, "observed_conversions": {}, "cpm": {}, "ctr": {}, "cpc": {}, "cvr": {}, "cpa": {}}
	for _, s := range samples {
		metrics["spend"] = append(metrics["spend"], float64(s.spend))
		metrics["impressions"] = append(metrics["impressions"], float64(s.impressions))
		metrics["clicks"] = append(metrics["clicks"], float64(s.clicks))
		metrics["true_conversions"] = append(metrics["true_conversions"], float64(s.trueConversions))
		metrics["observed_conversions"] = append(metrics["observed_conversions"], float64(s.observedConversions))
		appendAvailable(metrics, "cpm", s.cpm)
		appendAvailable(metrics, "ctr", s.ctr)
		appendAvailable(metrics, "cpc", s.cpc)
		appendAvailable(metrics, "cvr", s.cvr)
		appendAvailable(metrics, "cpa", s.cpa)
	}
	units := map[string]string{"spend": currency + "_minor", "impressions": "count", "clicks": "count", "true_conversions": "count", "observed_conversions": "count", "cpm": currency + "_minor_per_1000_impressions", "ctr": "ratio", "cpc": currency + "_minor_per_click", "cvr": "ratio", "cpa": currency + "_minor_per_conversion"}
	out := MechanisticMetricWindow{Sequence: sequence, Start: start, End: start.Add(24 * time.Hour), Timezone: timezone, Metrics: map[string]SimulationQuantiles{}}
	for key, values := range metrics {
		out.Metrics[key] = quantiles(values, units[key])
	}
	return out
}

func appendAvailable(values map[string][]float64, key string, value *float64) {
	if value != nil {
		values[key] = append(values[key], *value)
	}
}
func floatPtr(value float64) *float64 { return &value }

func quantiles(values []float64, unit string) SimulationQuantiles {
	if len(values) == 0 {
		return SimulationQuantiles{Available: false, Unit: unit}
	}
	sort.Float64s(values)
	var sum float64
	for _, value := range values {
		sum += value
	}
	return SimulationQuantiles{Available: true, Unit: unit, P10: floatPtr(percentile(values, .1)), P50: floatPtr(percentile(values, .5)), P90: floatPtr(percentile(values, .9)), Mean: floatPtr(sum / float64(len(values)))}
}

func percentile(values []float64, probability float64) float64 {
	index := probability * float64(len(values)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))
	if lower == upper {
		return values[lower]
	}
	return values[lower] + (values[upper]-values[lower])*(index-float64(lower))
}

func flattenMechanisticSamples(windows [][]mechanisticSample) []mechanisticSample {
	var values []mechanisticSample
	for _, window := range windows {
		values = append(values, window...)
	}
	return values
}

func detectMechanisticScenarios(samples []mechanisticSample, request MechanisticSimulationRequest, dailyCapMinor int64, evidence []string) []SimulationScenarioProbability {
	type predicate func(mechanisticSample) bool
	reviewStatus := "simulated"
	if request.ReviewState != SimulationReviewUnknown {
		reviewStatus = "known_state"
	}
	definitions := []struct {
		name, status string
		test         predicate
		limitation   string
	}{
		{"steady", "simulated", func(s mechanisticSample) bool {
			return !s.reviewRejected && !s.deliveryClosed && s.spend > 0 && s.trueConversions > 0
		}, "Steady is not proof of future stability."},
		{"under_delivery", "simulated", func(s mechanisticSample) bool { return s.deliveryClosed || s.spend == 0 }, "Threshold uses the explicit delivery gate."},
		{"cost_pressure", "simulated", func(s mechanisticSample) bool { return s.cpm != nil && *s.cpm > request.PriorSet.CPM.Mode }, "Association is not a causal budget effect."},
		{"creative_fatigue", "simulated", func(s mechanisticSample) bool {
			return request.PriorSet.CreativeFatigue != nil && request.PriorSet.CreativeFatigue.Enabled && request.PriorSet.CreativeFatigue.DailyRate > 0 && s.impressions > 0
		}, "Only an aggregate fatigue prior is used."},
		{"tracking_anomaly", "suspected", func(s mechanisticSample) bool { return s.trueConversions > 0 && s.observedConversions == 0 }, "No independent tracking evidence is available."},
		{"review_rejected", reviewStatus, func(s mechanisticSample) bool { return s.reviewRejected }, "Unknown review state uses the explicit prior."},
		{"zero_conversion", "simulated", func(s mechanisticSample) bool { return s.clicks > 0 && s.trueConversions == 0 }, "Zero conversions can occur from sampling variation."},
		{"spend_spike", "simulated", func(s mechanisticSample) bool {
			return s.spend > 0 && float64(s.spend) > float64(dailyCapMinor)*.9
		}, "No historical baseline is used."},
	}
	result := make([]SimulationScenarioProbability, 0, len(definitions))
	for _, definition := range definitions {
		count := 0
		for _, sample := range samples {
			if definition.test(sample) {
				count++
			}
		}
		result = append(result, SimulationScenarioProbability{Scenario: definition.name, ThresholdVersion: MechanisticThresholdVersion, Probability: float64(count) / float64(len(samples)), Status: definition.status, EvidenceRefs: append([]string(nil), evidence...), Limitations: []string{definition.limitation}})
	}
	return result
}

func scenarioAlerts(scenarios []SimulationScenarioProbability) []MechanisticSimulationAlert {
	alerts := []MechanisticSimulationAlert{}
	for _, scenario := range scenarios {
		if scenario.Scenario == "steady" || scenario.Probability < .5 {
			continue
		}
		alerts = append(alerts, MechanisticSimulationAlert{Type: scenario.Scenario, Severity: "medium", Probability: scenario.Probability, EvidenceRefs: scenario.EvidenceRefs, Limitations: scenario.Limitations})
	}
	return alerts
}

func draftMechanisticRecommendations(scenarios []SimulationScenarioProbability, input MechanisticSimulationInput, evidence []string) []SimulationRecommendationDraft {
	probabilities := map[string]float64{}
	for _, scenario := range scenarios {
		probabilities[scenario.Scenario] = scenario.Probability
	}
	makeDraft := func(kind, target, rationale string) SimulationRecommendationDraft {
		return SimulationRecommendationDraft{RecommendationType: kind, TargetField: target, CurrentValue: "frozen_configuration", SuggestedRange: [2]float64{0, 0}, ExpectedEffectRange: [2]float64{0, 0}, Confidence: "low", EffectBasis: "rule_constraint", Rationale: rationale, EvidenceRefs: append([]string(nil), evidence...), Risks: []string{"Simulation assumptions can differ from platform outcomes."}, Guardrails: []string{"Do not execute automatically.", "Use a controlled test after human approval."}, RequiresHumanReview: true}
	}
	result := []SimulationRecommendationDraft{}
	if probabilities["review_rejected"] >= .5 {
		result = append(result, makeDraft("review_compliance", "material_references", "Review the rejection reason or replace non-compliant material."))
	}
	if probabilities["tracking_anomaly"] >= .5 {
		result = append(result, makeDraft("tracking_review", "monitoring_references", "Check tracking before budget or bid changes."))
	}
	if probabilities["under_delivery"] >= .5 {
		result = append(result, makeDraft("delivery_review", "platform_configuration.payload.ocean_engine.project", "Review delivery constraints and use a controlled test."))
	}
	if probabilities["cost_pressure"] >= .5 {
		result = append(result, makeDraft("cost_review", "platform_configuration.payload.ocean_engine.project.budget_and_bidding", "Review cost assumptions before increasing budget or bid."))
	}
	if probabilities["creative_fatigue"] >= .5 {
		result = append(result, makeDraft("creative_test", "material_references", "Consider a controlled creative rotation test."))
	}
	if probabilities["zero_conversion"] >= .5 {
		result = append(result, makeDraft("conversion_funnel_review", "platform_configuration.payload.ocean_engine.project.optimization_target_reference", "Check the conversion funnel before changing delivery settings."))
	}
	if probabilities["spend_spike"] >= .5 {
		result = append(result, makeDraft("budget_pacing_review", "platform_configuration.payload.ocean_engine.project.budget_and_bidding.daily_budget_minor", "Review budget pacing before changing the daily budget."))
	}
	_ = input
	return result
}
