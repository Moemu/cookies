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
		"DeliveryPlanVersion:",
		"PreflightResult:",
		"VERSION_CONFLICT",
		"STALE_PLAN_VERSION",
		"const: mock",
		"const: local_simulation",
		"const: demo_fixture",
	}
	for _, expected := range required {
		if !strings.Contains(contract, expected) {
			t.Errorf("Delivery OpenAPI is missing %q", expected)
		}
	}
}
