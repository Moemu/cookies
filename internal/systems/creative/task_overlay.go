package creative

import (
	"encoding/json"
	"fmt"
	"strings"
)

const TaskOverlayContractVersion = "strategy-creative-task-overlay/v1"

type TaskOverlayReference struct {
	OverlayID           string `json:"overlay_id"`
	ExpectedContentHash string `json:"expected_content_hash"`
}

type TaskOverlayContractReference struct {
	OverlayID   string `json:"overlay_id"`
	ContentHash string `json:"content_hash"`
}

func (value TaskOverlayReference) Validate() error {
	if strings.TrimSpace(value.OverlayID) == "" ||
		strings.TrimSpace(value.ExpectedContentHash) == "" {
		return fmt.Errorf("task_overlay overlay_id and expected_content_hash are required")
	}
	return nil
}

type TaskOverlayInput struct {
	ContractVersion         string                   `json:"contract_version"`
	OverlayID               string                   `json:"overlay_id"`
	ContentHash             string                   `json:"content_hash"`
	PackageRef              StrategyPackageReference `json:"package_ref"`
	SelectedRouteID         string                   `json:"selected_route_id"`
	TaskStrategyPlanID      string                   `json:"task_strategy_plan_id"`
	TaskStrategyVersion     int64                    `json:"task_strategy_version"`
	TaskStrategyContentHash string                   `json:"task_strategy_content_hash"`
	ObjectiveRefinement     string                   `json:"objective_refinement,omitempty"`
	AudienceRefinement      string                   `json:"audience_refinement,omitempty"`
	MessagePriorities       []string                 `json:"message_priorities"`
	StrategyDimensions      map[string]any           `json:"strategy_dimensions"`
	Hypotheses              []string                 `json:"hypotheses"`
	Guardrails              []string                 `json:"guardrails"`
	OpenQuestions           []string                 `json:"open_questions"`
	RawSnapshot             json.RawMessage          `json:"-"`
}

type TaskOverlaySnapshot = TaskOverlayInput
