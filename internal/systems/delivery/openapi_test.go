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
		"/api/delivery/v1/plans:",
		"/api/delivery/v1/plans/{plan_id}:",
		"/api/delivery/v1/plans/{plan_id}/versions:",
		"/api/delivery/v1/plans/{plan_id}/versions/{version}:",
		"/api/delivery/v1/plans/{plan_id}/preflight:",
		"DeliveryPlanVersion:",
		"PreflightResult:",
		"VERSION_CONFLICT",
		"PROJECT_ACCESS_DENIED",
		"source=mock",
		"x-required-scope: delivery.plan.read",
		"x-required-scope: delivery.plan.write",
	}
	for _, expected := range required {
		if !strings.Contains(contract, expected) {
			t.Errorf("Delivery OpenAPI is missing %q", expected)
		}
	}
}
