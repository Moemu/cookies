package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const CreativeTaskOverlayContractVersion = "strategy-creative-task-overlay/v1"

type CreativeTaskOverlay struct {
	ContractVersion     string                         `json:"contract_version"`
	OverlayID           string                         `json:"overlay_id"`
	OrganizationID      contract.OrganizationID        `json:"organization_id"`
	ProjectID           contract.ProjectID             `json:"project_id"`
	PackageRef          CreativeTaskStrategyPackageRef `json:"package_ref"`
	HandoffRef          CreativeTaskStrategyHandoffRef `json:"handoff_ref"`
	SelectedRouteID     string                         `json:"selected_route_id"`
	TaskStrategyRef     CreativeTaskOverlayStrategyRef `json:"task_strategy_ref"`
	ObjectiveRefinement string                         `json:"objective_refinement"`
	AudienceRefinement  string                         `json:"audience_refinement"`
	MessagePriorities   []string                       `json:"message_priorities"`
	StrategyDimensions  map[string]any                 `json:"strategy_dimensions"`
	Hypotheses          []string                       `json:"hypotheses"`
	Guardrails          []string                       `json:"guardrails"`
	OpenQuestions       []string                       `json:"open_questions"`
	ContentHash         string                         `json:"content_hash"`
	CreatedAt           time.Time                      `json:"created_at"`
}

type CreativeTaskOverlayStrategyRef struct {
	PlanID      string `json:"plan_id"`
	Version     int64  `json:"version"`
	ContentHash string `json:"content_hash"`
}

func materializeCreativeTaskOverlay(
	overlayID string,
	version CreativeTaskStrategyVersion,
) (CreativeTaskOverlay, error) {
	document := version.Document
	hypotheses := make([]string, 0, len(document.Hypotheses))
	for _, hypothesis := range document.Hypotheses {
		hypotheses = append(hypotheses, hypothesis.Statement)
	}
	value := CreativeTaskOverlay{
		ContractVersion: CreativeTaskOverlayContractVersion, OverlayID: overlayID,
		OrganizationID: version.OrganizationID, ProjectID: version.ProjectID,
		PackageRef: *document.PackageRef, HandoffRef: *document.HandoffRef,
		SelectedRouteID: document.SelectedRouteID,
		TaskStrategyRef: CreativeTaskOverlayStrategyRef{
			PlanID: version.PlanID, Version: version.Version, ContentHash: version.ContentHash,
		},
		MessagePriorities:  append([]string{}, document.MessageHierarchy...),
		StrategyDimensions: cloneOverlayMap(document.BusinessStrategy),
		Hypotheses:         hypotheses, Guardrails: append([]string{}, document.Guardrails...),
		OpenQuestions: append([]string{}, document.OpenQuestions...),
		CreatedAt:     version.CreatedAt,
	}
	hashInput := value
	hashInput.ContentHash = ""
	hash, err := contract.NewContentHash(hashInput)
	if err != nil {
		return CreativeTaskOverlay{}, err
	}
	value.ContentHash = string(hash)
	return value, nil
}

func cloneOverlayMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}

func (s Service) GetCreativeTaskOverlay(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	overlayID string,
) (CreativeTaskOverlay, error) {
	if err := requireScope(actor, ScopePackageRead); err != nil {
		return CreativeTaskOverlay{}, err
	}
	if strings.TrimSpace(overlayID) == "" {
		return CreativeTaskOverlay{}, ErrInvalidRequest
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return CreativeTaskOverlay{}, err
	}
	var payload json.RawMessage
	var storedHash string
	if err := s.DB.QueryRowContext(ctx, `SELECT snapshot, content_hash
		FROM strategy_creative_task_overlays
		WHERE organization_id = ? AND project_id = ? AND overlay_id = ?`,
		actor.OrganizationID, projectID, overlayID,
	).Scan(&payload, &storedHash); err != nil {
		return CreativeTaskOverlay{}, mapNotFound(err)
	}
	var value CreativeTaskOverlay
	if err := json.Unmarshal(payload, &value); err != nil {
		return CreativeTaskOverlay{}, err
	}
	if value.ContractVersion != CreativeTaskOverlayContractVersion ||
		value.OverlayID != overlayID || value.ProjectID != projectID ||
		value.PackageRef.PackageID == "" || value.PackageRef.PackageVersion < 1 ||
		value.HandoffRef.ContractVersion != CreativeHandoffContractVersion ||
		value.SelectedRouteID == "" {
		return CreativeTaskOverlay{}, fmt.Errorf("%w: invalid task overlay snapshot", ErrInvalidState)
	}
	hashInput := value
	hashInput.ContentHash = ""
	calculated, err := contract.NewContentHash(hashInput)
	if err != nil {
		return CreativeTaskOverlay{}, err
	}
	if !strings.EqualFold(storedHash, value.ContentHash) ||
		!strings.EqualFold(string(calculated), value.ContentHash) {
		return CreativeTaskOverlay{}, fmt.Errorf("%w: task overlay hash mismatch", ErrInvalidState)
	}
	return value, nil
}

func (s Service) attachCreativeTaskOverlayRef(
	ctx context.Context,
	value CreativeTaskStrategyVersion,
) (CreativeTaskStrategyVersion, error) {
	if value.ContractVersion != "creative-task-strategy/v2" {
		return value, nil
	}
	var reference CreativeTaskOverlayReference
	if err := s.DB.QueryRowContext(ctx, `SELECT overlay_id, content_hash
		FROM strategy_creative_task_overlays
		WHERE organization_id = ? AND project_id = ? AND plan_id = ?
		AND task_strategy_version = ?`,
		value.OrganizationID, value.ProjectID, value.PlanID, value.Version,
	).Scan(&reference.OverlayID, &reference.ContentHash); err != nil {
		return CreativeTaskStrategyVersion{}, fmt.Errorf("%w: task overlay is missing: %v", ErrInvalidState, err)
	}
	value.TaskOverlayRef = &reference
	return value, nil
}
