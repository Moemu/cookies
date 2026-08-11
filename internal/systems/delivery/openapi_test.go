package delivery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenAPIContractCoversPlanLifecyclePreflightAndErrors(t *testing.T) {
	t.Parallel()
	path := filepath.Join("..", "..", "..", "api", "openapi", "delivery-v1.yaml")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Delivery OpenAPI: %v", err)
	}
	contract := string(contents)
	required := []string{
		"/api/delivery/v1/projects/{project_id}/plans:",
		"/api/delivery/v1/projects/{project_id}/plans/{plan_id}:",
		"/api/delivery/v1/projects/{project_id}/plans/{plan_id}/versions:",
		"/api/delivery/v1/projects/{project_id}/plans/{plan_id}/versions/{version}:",
		"/api/delivery/v1/projects/{project_id}/plans/{plan_id}/preflight:",
		"/api/delivery/v1/projects/{project_id}/plans/{plan_id}:create-change-set:",
		"/api/delivery/v1/projects/{project_id}/change-sets:",
		"/api/delivery/v1/projects/{project_id}/change-sets/{change_set_id}:",
		"/api/delivery/v1/projects/{project_id}/change-sets/{change_set_id}:approve:",
		"/api/delivery/v1/projects/{project_id}/change-sets/{change_set_id}:reject:",
		"DeliveryPlanVersion:",
		"StrategyReference:",
		"strategy_reference:",
		"content_hash:",
		"rejection_reason:",
		"canonical_hash:",
		"plan_name:",
		"ApprovalView:",
		"change_set_version:",
		"action_hash:",
		"const: execute_mock",
		"PreflightResult:",
		"VERSION_CONFLICT",
		"STALE_PLAN_VERSION",
		"APPROVAL_REQUIRED",
		"APPROVAL_EXPIRED",
		"APPROVAL_CONTENT_MISMATCH",
		"APPROVAL_SCOPE_EXCEEDED",
		"const: mock",
		"const: local_simulation",
		"const: post_launch_simulator",
		"const: post-launch-simulator/v1",
	}
	for _, expected := range required {
		if !strings.Contains(contract, expected) {
			t.Errorf("Delivery OpenAPI is missing %q", expected)
		}
	}
}

func TestOpenAPIContractCoversProjectScopedMonitoringAlerts(t *testing.T) {
	t.Parallel()
	path := filepath.Join("..", "..", "..", "api", "openapi", "delivery-v1.yaml")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Delivery OpenAPI: %v", err)
	}
	contract := string(contents)
	required := []string{
		"/api/delivery/v1/projects/{project_id}/alerts:evaluate:",
		"/api/delivery/v1/projects/{project_id}/alerts:",
		"/api/delivery/v1/projects/{project_id}/alerts/{alert_id}:",
		"AlertEvaluationRequest:",
		"AlertEvaluationResult:",
		"AlertList:",
		"UpdateAlert:",
		"enum: [review_rejected, spend_spike, zero_conversion, cost_worsening, under_delivery, creative_fatigue, tracking_anomaly]",
		"/api/delivery/v1/projects/{project_id}/executions/{execution_id}/simulation-runs:",
		"OutcomeSimulationRun:",
		"OutcomeSimulationResult:",
		"delivery-outcome-scenario/v1",
		"simulation_run_id:",
		"enum: [open, acknowledged, dismissed]",
		"enum: [acknowledge, dismiss]",
		"enum: [normal_day, anomaly_day, stale_data, insufficient_data]",
		"enum: [fresh, stale, unknown, insufficient_data]",
		"enum: [critical, high, medium, low]",
		"required: [action, expected_version]",
		"name: cursor",
		"AlertFreshness:",
		"InsightsQualityStatus:",
		"insights_source:",
		"insights_quality:",
		"insights_quality_reason:",
		"insights_fixture_version:",
		"insights_evidence_refs:",
		"monitored_entity:",
		"fixture_version:",
		"created_by:",
		"acknowledged_by:",
		"dismissed_by:",
		"evidence_refs:",
	}
	for _, expected := range required {
		if !strings.Contains(contract, expected) {
			t.Errorf("Delivery monitoring OpenAPI is missing %q", expected)
		}
	}
}

func TestOpenAPIContractCoversOwnerScopedDeliveryTour(t *testing.T) {
	t.Parallel()
	path := filepath.Join("..", "..", "..", "api", "openapi", "delivery-v1.yaml")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Delivery OpenAPI: %v", err)
	}
	contract := string(contents)
	required := []string{
		"/api/delivery/v1/projects/{project_id}/tour-runs/{run_id}:prepare:",
		"/api/delivery/v1/projects/{project_id}/tour-runs/{run_id}:",
		"/api/delivery/v1/projects/{project_id}/tour-runs/{run_id}:reset:",
		"DeliveryTourRun:",
		"DeliveryTourCase:",
		"DeliveryTourStep:",
		"DeliveryTourResetResult:",
		"enum: [golden_path, preflight_failure, approval_expired, plan_stale, partial_execution, result_unknown, review_rejected_alert]",
		"pattern: '^[a-z0-9][a-z0-9_-]{2,63}$'",
		"TOUR_OWNER_MISMATCH",
		"isolation_key:",
		"observed_at:",
		"suggested_next_url:",
		"enum: [plan_creation, configuration, first_approval, execution, monitoring, recommendation, new_change_set, second_approval]",
	}
	for _, expected := range required {
		if !strings.Contains(contract, expected) {
			t.Errorf("Delivery tour OpenAPI is missing %q", expected)
		}
	}
	if strings.Contains(contract, "A07") {
		t.Error("Delivery tour OpenAPI must use domain names rather than a phase identifier")
	}
}
