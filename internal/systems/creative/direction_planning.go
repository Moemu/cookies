package creative

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type CreativePlanningContext struct {
	ContractVersion   string                   `json:"contract_version"`
	InputIdentityHash string                   `json:"input_identity_hash"`
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
	GetDirection(context.Context, contract.OrganizationID, contract.ProjectID, string) (CreativeDirectionVersion, error)
	ConfirmDirection(context.Context, contract.OrganizationID, contract.ProjectID, string, string, time.Time) (CreativeDirectionVersion, error)
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
	project, err := s.Projects.RequireActiveContext(ctx, actor, projectID)
	if err != nil {
		return CreativeDirectionBatch{}, err
	}
	intake, err := s.Repository.GetIntake(ctx, actor.OrganizationID, projectID, intakeID)
	if err != nil {
		return CreativeDirectionBatch{}, err
	}
	if intake.Source != IntakeSourceStrategyPackage || intake.Status != IntakeReady ||
		intake.InputIdentityHash == "" || intake.Request.SelectedRouteID == "" {
		return CreativeDirectionBatch{}, fmt.Errorf("a ready v3 strategy intake is required")
	}
	route, err := selectedPlanningRoute(intake.Request.CreativeRoutes, intake.Request.SelectedRouteID)
	if err != nil {
		return CreativeDirectionBatch{}, err
	}
	if route.RouteType != CreativeRouteImageText && route.RouteType != CreativeRouteBrandVideo {
		return CreativeDirectionBatch{}, fmt.Errorf("creative direction planning is not enabled for route %q", route.RouteType)
	}
	if request.CandidateCount == 0 {
		request.CandidateCount = 3
	}
	if request.CandidateCount < 2 || request.CandidateCount > 4 {
		return CreativeDirectionBatch{}, fmt.Errorf("candidate_count must be between 2 and 4")
	}
	planningContext, err := planningContextFromIntake(intake, route)
	if err != nil {
		return CreativeDirectionBatch{}, err
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
	result, err := s.DirectionPlanner.Generate(ctx, actor, project, planningContext, request.CandidateCount)
	if err != nil {
		return CreativeDirectionBatch{}, fmt.Errorf("generate creative directions: %w", err)
	}
	if strings.TrimSpace(result.Model) == "" || strings.TrimSpace(result.PromptVersion) == "" ||
		len(result.Candidates) != request.CandidateCount {
		return CreativeDirectionBatch{}, fmt.Errorf("creative direction provider returned an invalid candidate batch")
	}
	batchID, err := s.idGenerator()("directionbatch")
	if err != nil {
		return CreativeDirectionBatch{}, err
	}
	now := s.now()
	directions := make([]CreativeDirectionVersion, 0, len(result.Candidates))
	seenConcepts := map[string]bool{}
	for _, candidate := range result.Candidates {
		if err := candidate.Validate(); err != nil {
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
			OrganizationID: actor.OrganizationID, ProjectID: projectID, BatchID: batchID,
			IntakeID: intakeID, InputIdentityHash: intake.InputIdentityHash, RouteID: route.RouteID,
			Concept: candidate.Concept, CreativeRationale: candidate.CreativeRationale,
			MessagePlan:      append([]string{}, candidate.MessagePlan...),
			ExecutionOutline: append([]string{}, candidate.ExecutionOutline...),
			GuardrailTrace:   append([]string{}, candidate.GuardrailTrace...),
			Status:           DirectionStatusCandidate, Version: 1, CreatedAt: now,
		}
		contentHash, hashErr := contract.NewContentHash(value)
		if hashErr != nil {
			return CreativeDirectionBatch{}, hashErr
		}
		value.ContentHash = string(contentHash)
		directions = append(directions, value)
	}
	batch := CreativeDirectionBatch{
		ContractVersion: CreativeDirectionBatchV1, ID: batchID,
		OrganizationID: actor.OrganizationID, ProjectID: projectID, IntakeID: intakeID,
		InputIdentityHash: intake.InputIdentityHash, Status: DirectionBatchReady, Candidates: directions,
		Model: result.Model, PromptVersion: result.PromptVersion,
		CreatedBy: actor.Principal.ID, CreatedAt: now,
	}
	return s.Directions.CreateDirectionBatch(ctx, batch)
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
