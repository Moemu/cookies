package creative

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type CreativePlanningContext struct {
	ContractVersion   string                   `json:"contract_version"`
	InputIdentityHash string                   `json:"input_identity_hash"`
	GenerationID      string                   `json:"generation_id,omitempty"`
	SelectedRoute     CreativeRouteSnapshot    `json:"selected_route"`
	Objective         json.RawMessage          `json:"objective"`
	Audience          []json.RawMessage        `json:"audience"`
	Proposition       string                   `json:"proposition"`
	MessageHierarchy  []json.RawMessage        `json:"message_hierarchy"`
	Guardrails        []json.RawMessage        `json:"guardrails"`
	Claims            []json.RawMessage        `json:"claims"`
	Assets            []json.RawMessage        `json:"assets"`
	Hypotheses        []json.RawMessage        `json:"hypotheses"`
	TaskRefinements   *CreativeTaskRefinements `json:"task_refinements,omitempty"`
}

// CreativeTaskRefinements is deliberately strategy-only. Fields that would
// make Strategy author the concept, hook, script, storyboard, shot list, or
// model prompt do not exist in this type.
type CreativeTaskRefinements struct {
	Objective          string         `json:"objective,omitempty"`
	Audience           string         `json:"audience,omitempty"`
	MessagePriorities  []string       `json:"message_priorities"`
	StrategyDimensions map[string]any `json:"strategy_dimensions"`
	Hypotheses         []string       `json:"hypotheses"`
	Guardrails         []string       `json:"guardrails"`
	OpenQuestions      []string       `json:"open_questions"`
}

type DirectionCandidate struct {
	Concept           string   `json:"concept"`
	CreativeRationale string   `json:"creative_rationale"`
	MessagePlan       []string `json:"message_plan"`
	ExecutionOutline  []string `json:"execution_outline"`
	GuardrailTrace    []string `json:"guardrail_trace"`
	DirectionMode     string   `json:"direction_mode,omitempty"`
	EmotionalArc      string   `json:"emotional_arc,omitempty"`
	VisualGrammar     string   `json:"visual_grammar,omitempty"`
	BrandMemoryDevice string   `json:"brand_memory_device,omitempty"`
	HumanMoment       string   `json:"human_moment,omitempty"`
}

func (value DirectionCandidate) Validate() error {
	if strings.TrimSpace(value.Concept) == "" || strings.TrimSpace(value.CreativeRationale) == "" ||
		len(value.MessagePlan) == 0 || len(value.ExecutionOutline) == 0 ||
		len(value.GuardrailTrace) == 0 {
		return fmt.Errorf("creative direction candidate is incomplete")
	}
	for _, items := range [][]string{value.MessagePlan, value.ExecutionOutline, value.GuardrailTrace} {
		for _, item := range items {
			if strings.TrimSpace(item) == "" {
				return fmt.Errorf("creative direction candidate contains an empty item")
			}
		}
	}
	return nil
}

type CreativeDirectionVersion struct {
	ContractVersion   string                  `json:"contract_version"`
	ID                string                  `json:"direction_id"`
	OrganizationID    contract.OrganizationID `json:"organization_id"`
	ProjectID         contract.ProjectID      `json:"project_id"`
	BatchID           string                  `json:"batch_id"`
	IntakeID          string                  `json:"intake_id"`
	InputIdentityHash string                  `json:"input_identity_hash"`
	RouteID           string                  `json:"route_id"`
	Concept           string                  `json:"concept"`
	CreativeRationale string                  `json:"creative_rationale"`
	MessagePlan       []string                `json:"message_plan"`
	ExecutionOutline  []string                `json:"execution_outline"`
	GuardrailTrace    []string                `json:"guardrail_trace"`
	DirectionMode     string                  `json:"direction_mode,omitempty"`
	EmotionalArc      string                  `json:"emotional_arc,omitempty"`
	VisualGrammar     string                  `json:"visual_grammar,omitempty"`
	BrandMemoryDevice string                  `json:"brand_memory_device,omitempty"`
	HumanMoment       string                  `json:"human_moment,omitempty"`
	Status            CreativeDirectionStatus `json:"status"`
	Version           int64                   `json:"version"`
	ContentHash       string                  `json:"content_hash"`
	ConfirmedBy       string                  `json:"confirmed_by,omitempty"`
	ConfirmedAt       *time.Time              `json:"confirmed_at,omitempty"`
	CreatedAt         time.Time               `json:"created_at"`
}

type CreativeDirectionBatch struct {
	ContractVersion   string                       `json:"contract_version"`
	ID                string                       `json:"batch_id"`
	OrganizationID    contract.OrganizationID      `json:"organization_id"`
	ProjectID         contract.ProjectID           `json:"project_id"`
	IntakeID          string                       `json:"intake_id"`
	InputIdentityHash string                       `json:"input_identity_hash"`
	Status            CreativeDirectionBatchStatus `json:"status"`
	Candidates        []CreativeDirectionVersion   `json:"candidates"`
	Model             string                       `json:"model"`
	PromptVersion     string                       `json:"prompt_version"`
	FailureCode       string                       `json:"failure_code,omitempty"`
	CreatedBy         string                       `json:"created_by"`
	CreatedAt         time.Time                    `json:"created_at"`
}

type GenerateDirectionRequest struct {
	CandidateCount int `json:"candidate_count"`
}

type DirectionPlannerResult struct {
	Candidates    []DirectionCandidate
	Model         string
	PromptVersion string
}

// CreativeDirectionPlanner is the LLM boundary. Implementations must request
// schema-constrained output and return an error for provider or validation
// failures. The service intentionally has no deterministic concept fallback.
type CreativeDirectionPlanner interface {
	Generate(context.Context, contract.ActorContext, contract.ProjectContext, CreativePlanningContext, int) (DirectionPlannerResult, error)
}

type DirectionRepository interface {
	CreateDirectionBatch(context.Context, CreativeDirectionBatch) (CreativeDirectionBatch, error)
	GetDirectionBatch(context.Context, contract.OrganizationID, contract.ProjectID, string) (CreativeDirectionBatch, error)
	CompleteDirectionBatch(context.Context, CreativeDirectionBatch) (CreativeDirectionBatch, error)
	FailDirectionBatch(context.Context, contract.OrganizationID, contract.ProjectID, string, string) error
	GetDirection(context.Context, contract.OrganizationID, contract.ProjectID, string) (CreativeDirectionVersion, error)
	ConfirmDirection(context.Context, contract.OrganizationID, contract.ProjectID, string, string, time.Time) (CreativeDirectionVersion, error)
}

type DirectionBatchReader interface {
	GetLatestDirectionBatch(context.Context, contract.OrganizationID, contract.ProjectID, string) (CreativeDirectionBatch, error)
}

func (s Service) GetLatestDirectionBatch(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	intakeID string,
) (CreativeDirectionBatch, error) {
	reader, ok := s.Directions.(DirectionBatchReader)
	if s.Projects == nil || !ok {
		return CreativeDirectionBatch{}, fmt.Errorf("creative direction history is unavailable")
	}
	if !actor.HasScope(ScopeRead) {
		return CreativeDirectionBatch{}, fmt.Errorf("%s scope is required", ScopeRead)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return CreativeDirectionBatch{}, err
	}
	return reader.GetLatestDirectionBatch(ctx, actor.OrganizationID, projectID, intakeID)
}

func (s Service) GenerateDirectionCandidates(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	intakeID string,
	request GenerateDirectionRequest,
) (CreativeDirectionBatch, error) {
	if s.Repository == nil || s.Projects == nil || s.DirectionPlanner == nil || s.Directions == nil {
		return CreativeDirectionBatch{}, fmt.Errorf("creative direction planning is unavailable")
	}
	if !actor.HasScope(ScopeWrite) {
		return CreativeDirectionBatch{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	prepared, err := s.prepareDirectionPlanning(ctx, actor, projectID, intakeID, request)
	if err != nil {
		return CreativeDirectionBatch{}, err
	}
	batchID, err := s.idGenerator()("directionbatch")
	if err != nil {
		return CreativeDirectionBatch{}, err
	}
	batch, err := s.planDirectionBatch(ctx, actor, prepared, batchID, s.now())
	if err != nil {
		return CreativeDirectionBatch{}, err
	}
	return s.Directions.CreateDirectionBatch(ctx, batch)
}

type preparedDirectionPlanning struct {
	Project         contract.ProjectContext
	Intake          CreativeIntake
	Route           CreativeRouteSnapshot
	PlanningContext CreativePlanningContext
	CandidateCount  int
}

func (s Service) prepareDirectionPlanning(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	intakeID string,
	request GenerateDirectionRequest,
) (preparedDirectionPlanning, error) {
	project, err := s.Projects.RequireActiveContext(ctx, actor, projectID)
	if err != nil {
		return preparedDirectionPlanning{}, err
	}
	intake, err := s.Repository.GetIntake(ctx, actor.OrganizationID, projectID, intakeID)
	if err != nil {
		return preparedDirectionPlanning{}, err
	}
	if intake.ContractVersion != CreativeIntakeV3ContractVersion || intake.Status != IntakeReady ||
		intake.InputIdentityHash == "" || intake.Request.SelectedRouteID == "" {
		return preparedDirectionPlanning{}, fmt.Errorf("a ready v3 creative intake is required")
	}
	route, err := selectedPlanningRoute(intake.Request.CreativeRoutes, intake.Request.SelectedRouteID)
	if err != nil {
		return preparedDirectionPlanning{}, err
	}
	if route.RouteType != CreativeRouteImageText && route.RouteType != CreativeRouteBrandVideo {
		return preparedDirectionPlanning{}, fmt.Errorf("creative direction planning is not enabled for route %q", route.RouteType)
	}
	if request.CandidateCount == 0 {
		request.CandidateCount = 3
	}
	if request.CandidateCount < 2 || request.CandidateCount > 4 {
		return preparedDirectionPlanning{}, fmt.Errorf("candidate_count must be between 2 and 4")
	}
	planningContext, err := planningContextFromIntake(intake, route)
	if err != nil {
		return preparedDirectionPlanning{}, err
	}
	if overlay := intake.Request.TaskOverlayInput; overlay != nil {
		planningContext.TaskRefinements = &CreativeTaskRefinements{
			Objective: overlay.ObjectiveRefinement, Audience: overlay.AudienceRefinement,
			MessagePriorities:  append([]string{}, overlay.MessagePriorities...),
			StrategyDimensions: cloneDirectionMap(overlay.StrategyDimensions),
			Hypotheses:         append([]string{}, overlay.Hypotheses...),
			Guardrails:         append([]string{}, overlay.Guardrails...),
			OpenQuestions:      append([]string{}, overlay.OpenQuestions...),
		}
	}
	return preparedDirectionPlanning{Project: project, Intake: intake, Route: route, PlanningContext: planningContext, CandidateCount: request.CandidateCount}, nil
}

func (s Service) planDirectionBatch(
	ctx context.Context,
	actor contract.ActorContext,
	prepared preparedDirectionPlanning,
	batchID string,
	createdAt time.Time,
) (CreativeDirectionBatch, error) {
	planningContext := prepared.PlanningContext
	planningContext.GenerationID = batchID
	result, err := s.DirectionPlanner.Generate(ctx, actor, prepared.Project, planningContext, prepared.CandidateCount)
	if err != nil {
		return CreativeDirectionBatch{}, fmt.Errorf("generate creative directions: %w", err)
	}
	if strings.TrimSpace(result.Model) == "" || strings.TrimSpace(result.PromptVersion) == "" ||
		len(result.Candidates) != prepared.CandidateCount {
		return CreativeDirectionBatch{}, fmt.Errorf("creative direction provider returned an invalid candidate batch")
	}
	directions := make([]CreativeDirectionVersion, 0, len(result.Candidates))
	seenConcepts := map[string]bool{}
	for _, candidate := range result.Candidates {
		if err := candidate.Validate(); err != nil {
			return CreativeDirectionBatch{}, err
		}
		if err := validateDirectionCandidateClaims(candidate); err != nil {
			return CreativeDirectionBatch{}, err
		}
		conceptKey := strings.ToLower(strings.TrimSpace(candidate.Concept))
		if seenConcepts[conceptKey] {
			return CreativeDirectionBatch{}, fmt.Errorf("creative direction candidates must have distinct concepts")
		}
		seenConcepts[conceptKey] = true
		directionID, idErr := s.idGenerator()("direction")
		if idErr != nil {
			return CreativeDirectionBatch{}, idErr
		}
		value := CreativeDirectionVersion{
			ContractVersion: CreativeDirectionVersionV1, ID: directionID,
			OrganizationID: actor.OrganizationID, ProjectID: prepared.Project.ProjectID, BatchID: batchID,
			IntakeID: prepared.Intake.ID, InputIdentityHash: prepared.Intake.InputIdentityHash, RouteID: prepared.Route.RouteID,
			Concept: candidate.Concept, CreativeRationale: candidate.CreativeRationale,
			MessagePlan:      append([]string{}, candidate.MessagePlan...),
			ExecutionOutline: append([]string{}, candidate.ExecutionOutline...),
			GuardrailTrace:   append([]string{}, candidate.GuardrailTrace...),
			DirectionMode:    candidate.DirectionMode, EmotionalArc: candidate.EmotionalArc,
			VisualGrammar: candidate.VisualGrammar, BrandMemoryDevice: candidate.BrandMemoryDevice,
			HumanMoment: candidate.HumanMoment,
			Status:      DirectionStatusCandidate, Version: 1, CreatedAt: createdAt,
		}
		contentHash, hashErr := contract.NewContentHash(value)
		if hashErr != nil {
			return CreativeDirectionBatch{}, hashErr
		}
		value.ContentHash = string(contentHash)
		directions = append(directions, value)
	}
	if err := validateDirectionBatchQuality(prepared.PlanningContext, result.Candidates); err != nil {
		return CreativeDirectionBatch{}, err
	}
	batch := CreativeDirectionBatch{
		ContractVersion: CreativeDirectionBatchV1, ID: batchID,
		OrganizationID: actor.OrganizationID, ProjectID: prepared.Project.ProjectID, IntakeID: prepared.Intake.ID,
		InputIdentityHash: prepared.Intake.InputIdentityHash, Status: DirectionBatchReady, Candidates: directions,
		Model: result.Model, PromptVersion: result.PromptVersion,
		CreatedBy: actor.Principal.ID, CreatedAt: createdAt,
	}
	return batch, nil
}

func validateDirectionBatchQuality(planningContext CreativePlanningContext, candidates []DirectionCandidate) error {
	for left := 0; left < len(candidates); left++ {
		for right := left + 1; right < len(candidates); right++ {
			if directionConceptSimilarity(candidates[left].Concept, candidates[right].Concept) >= 0.72 {
				return fmt.Errorf("creative direction candidates are too similar")
			}
		}
	}
	if planningContext.SelectedRoute.RouteType != CreativeRouteBrandVideo {
		return nil
	}
	utilityCount := 0
	brandLedCount := 0
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.EmotionalArc) == "" || strings.TrimSpace(candidate.VisualGrammar) == "" ||
			strings.TrimSpace(candidate.BrandMemoryDevice) == "" || strings.TrimSpace(candidate.HumanMoment) == "" {
			return fmt.Errorf("brand-video direction must define emotional_arc, visual_grammar, brand_memory_device, and human_moment")
		}
		mode := strings.ToLower(strings.TrimSpace(candidate.DirectionMode))
		if mode != "emotional" && mode != "cinematic" && mode != "utility" {
			return fmt.Errorf("brand-video direction_mode must be emotional, cinematic, or utility")
		}
		utilityLed := isUtilityLedDirection(candidate)
		if mode != "utility" && utilityLed {
			return fmt.Errorf("brand-video emotional or cinematic direction cannot be led by a guide, checklist, steps, or tool")
		}
		if phrase := firstBrandPerformanceCue(candidate); phrase != "" {
			return fmt.Errorf("brand-video direction contains performance CTA or unsupported production spec: %s", phrase)
		}
		if mode == "utility" || utilityLed {
			utilityCount++
		} else {
			brandLedCount++
		}
	}
	if utilityCount > 1 || brandLedCount < 2 {
		return fmt.Errorf("brand-video batch must contain at least two emotional or cinematic directions and at most one utility-led direction")
	}
	return nil
}

func firstBrandPerformanceCue(candidate DirectionCandidate) string {
	text := strings.ToLower(strings.Join(append(
		[]string{candidate.Concept, candidate.CreativeRationale, candidate.VisualGrammar, candidate.HumanMoment},
		candidate.ExecutionOutline...,
	), "\n"))
	for _, phrase := range []string{
		"点击了解", "点击查看", "点击购买", "点击咨询", "点击领取", "点击链接", "点击主页", "点击下单", "点击获取",
		"评论区", "私信", "扣1", "扣 1", "领取福利", "领取优惠", "领取资料", "领完整", "立即咨询", "获取专属", "4k", "8k",
	} {
		if strings.Contains(text, phrase) {
			return phrase
		}
	}
	return ""
}

func isUtilityLedDirection(candidate DirectionCandidate) bool {
	text := strings.ToLower(strings.Join(append(
		[]string{candidate.Concept, candidate.CreativeRationale},
		candidate.MessagePlan...,
	), "\n"))
	for _, phrase := range []string{
		"指南", "清单", "三步", "步骤", "避坑", "教程", "科普", "方法论", "checklist", "how-to", "how to",
		"判断工具", "核验工具", "评估工具", "选择工具", "决策工具", "工具箱",
	} {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

func directionConceptSimilarity(left string, right string) float64 {
	leftTokens := directionBigrams(left)
	rightTokens := directionBigrams(right)
	if len(leftTokens) == 0 || len(rightTokens) == 0 {
		return 0
	}
	intersection := 0
	union := make(map[string]struct{}, len(leftTokens)+len(rightTokens))
	for token := range leftTokens {
		union[token] = struct{}{}
		if _, ok := rightTokens[token]; ok {
			intersection++
		}
	}
	for token := range rightTokens {
		union[token] = struct{}{}
	}
	return float64(intersection) / float64(len(union))
}

func directionBigrams(value string) map[string]struct{} {
	runes := []rune(strings.ToLower(strings.TrimSpace(value)))
	filtered := make([]rune, 0, len(runes))
	for _, current := range runes {
		if current == ' ' || current == '-' || current == '—' || current == '：' || current == ':' || current == '·' {
			continue
		}
		filtered = append(filtered, current)
	}
	tokens := map[string]struct{}{}
	for index := 0; index+1 < len(filtered); index++ {
		tokens[string(filtered[index:index+2])] = struct{}{}
	}
	return tokens
}

func validateDirectionCandidateClaims(candidate DirectionCandidate) error {
	claimFields := append(
		[]string{candidate.Concept, candidate.CreativeRationale},
		append(append([]string{}, candidate.MessagePlan...), candidate.ExecutionOutline...)...,
	)
	if phrase := firstHighRiskOutboundClaim(claimFields...); phrase != "" {
		return fmt.Errorf("creative direction contains a high-risk claim: %s", phrase)
	}
	return nil
}

func firstHighRiskOutboundClaim(fields ...string) string {
	claimText := strings.ToLower(strings.Join(fields, "\n"))
	for _, phrase := range []string{
		"100%", "行业第一", "品类第一", "销量第一", "市场第一", "排名第一", "全网第一",
		"全国第一", "世界第一", "适配度第一", "最好", "最优", "首选", "必囤", "必买",
		"必看", "不踩雷", "神器", "神仙", "都爱", "大家都问", "保证", "绝对", "永久",
		"顶级", "零风险", "治愈", "药到病除", "完全没负担", "毫无负担", "放心入",
		"无额外负担", "放心喝", "全适配", "接受度超高", "能力拉满", "不会有负罪感",
		"零负罪感", "适合多数人", "多数人的喜好", "口味适配度高",
	} {
		if strings.Contains(claimText, strings.ToLower(phrase)) {
			return phrase
		}
	}
	for _, pattern := range []struct {
		label string
		expr  string
	}{
		{label: "群体普适性表述", expr: `(适合|适配).{0,6}(多数|所有|全部).{0,10}(人|用户|偏好|喜好|场景)`},
		{label: "无证据群体评价", expr: `大家.{0,6}(夸|喜欢|认可|爱)`},
	} {
		if regexp.MustCompile(pattern.expr).MatchString(claimText) {
			return pattern.label
		}
	}
	return ""
}

func (s Service) ConfirmDirection(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	directionID string,
) (CreativeDirectionVersion, error) {
	if s.Projects == nil || s.Directions == nil {
		return CreativeDirectionVersion{}, fmt.Errorf("creative direction planning is unavailable")
	}
	if !actor.HasScope(ScopeWrite) {
		return CreativeDirectionVersion{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return CreativeDirectionVersion{}, err
	}
	if strings.TrimSpace(directionID) == "" {
		return CreativeDirectionVersion{}, fmt.Errorf("direction_id is required")
	}
	return s.Directions.ConfirmDirection(
		ctx, actor.OrganizationID, projectID, directionID, actor.Principal.ID, s.now(),
	)
}

func selectedPlanningRoute(routes []CreativeRouteSnapshot, routeID string) (CreativeRouteSnapshot, error) {
	for _, route := range routes {
		if route.RouteID == routeID {
			if route.ReadinessStatus != "" && route.ReadinessStatus != "ready" {
				return CreativeRouteSnapshot{}, fmt.Errorf("selected Creative route is not ready")
			}
			return route, nil
		}
	}
	return CreativeRouteSnapshot{}, fmt.Errorf("selected_route_id is not present in the Strategy handoff")
}

func cloneDirectionMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func planningContextFromIntake(
	intake CreativeIntake,
	route CreativeRouteSnapshot,
) (CreativePlanningContext, error) {
	var handoff struct {
		CreativeView struct {
			Objective     json.RawMessage   `json:"objective"`
			Audience      []json.RawMessage `json:"audience_segments"`
			Communication struct {
				Proposition      string            `json:"single_minded_proposition"`
				MessageHierarchy []json.RawMessage `json:"message_hierarchy"`
			} `json:"communication"`
			Guardrails []json.RawMessage `json:"guardrails"`
			Claims     []json.RawMessage `json:"claims"`
			Assets     []json.RawMessage `json:"assets"`
			Hypotheses []json.RawMessage `json:"creative_hypotheses"`
		} `json:"creative_view"`
	}
	if len(intake.Request.StrategyHandoffInput) > 0 {
		if err := json.Unmarshal(intake.Request.StrategyHandoffInput, &handoff); err != nil {
			return CreativePlanningContext{}, fmt.Errorf("decode Creative planning handoff: %w", err)
		}
	}
	if len(handoff.CreativeView.Objective) == 0 {
		handoff.CreativeView.Objective, _ = json.Marshal(map[string]any{
			"statement": intake.Request.Objective,
		})
	}
	if len(handoff.CreativeView.Audience) == 0 {
		fallback, _ := json.Marshal(map[string]any{"label": intake.Request.Audience})
		handoff.CreativeView.Audience = []json.RawMessage{fallback}
	}
	if handoff.CreativeView.Communication.Proposition == "" {
		handoff.CreativeView.Communication.Proposition = intake.Request.CoreMessage
	}
	if handoff.CreativeView.Communication.MessageHierarchy == nil {
		handoff.CreativeView.Communication.MessageHierarchy = []json.RawMessage{}
	}
	if handoff.CreativeView.Guardrails == nil {
		handoff.CreativeView.Guardrails = []json.RawMessage{}
		for _, text := range append(
			append([]string{}, intake.Request.Mandatory...),
			intake.Request.Prohibited...,
		) {
			item, _ := json.Marshal(map[string]any{"text": text})
			handoff.CreativeView.Guardrails = append(handoff.CreativeView.Guardrails, item)
		}
	}
	if handoff.CreativeView.Claims == nil {
		handoff.CreativeView.Claims = []json.RawMessage{}
	}
	if handoff.CreativeView.Assets == nil {
		handoff.CreativeView.Assets = []json.RawMessage{}
	}
	if handoff.CreativeView.Hypotheses == nil {
		handoff.CreativeView.Hypotheses = []json.RawMessage{}
	}
	return CreativePlanningContext{
		ContractVersion: CreativePlanningContextV1, InputIdentityHash: intake.InputIdentityHash,
		SelectedRoute: route, Objective: handoff.CreativeView.Objective,
		Audience:         handoff.CreativeView.Audience,
		Proposition:      handoff.CreativeView.Communication.Proposition,
		MessageHierarchy: handoff.CreativeView.Communication.MessageHierarchy,
		Guardrails:       handoff.CreativeView.Guardrails, Claims: handoff.CreativeView.Claims,
		Assets: handoff.CreativeView.Assets, Hypotheses: handoff.CreativeView.Hypotheses,
	}, nil
}
