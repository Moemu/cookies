package strategycreative

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestFrozenCreativeContractFixtures(t *testing.T) {
	t.Parallel()

	ready := readFixture(t, "strategy-creative-handoff-v1-ready.json")
	blocked := readFixture(t, "strategy-creative-handoff-v1-blocked.json")
	assertHandoffHash(t, ready)
	assertHandoffHash(t, blocked)

	create := readFixture(t, "creative-intake-create-v2.json")
	intake := readFixture(t, "creative-intake-v2-ready.json")
	readyRef := intakePackageRef(t, ready)
	assertSameJSONValue(t, "create strategy_package_ref", create["strategy_package_ref"], readyRef)
	assertSameJSONValue(t, "intake strategy_package_ref", intake["strategy_package_ref"], readyRef)
	assertSameJSONValue(t, "intake input_snapshot", intake["input_snapshot"], ready)

	selectedRouteID := requiredString(t, create, "selected_route_id")
	if selectedRouteID != requiredString(t, intake, "selected_route_id") {
		t.Fatalf("create and intake selected_route_id differ")
	}
	assertRouteExists(t, ready, selectedRouteID)
	assertReadyIntakeState(t, intake)

	baseV3 := readFixture(t, "creative-intake-create-v3-base.json")
	overlayCreate := readFixture(t, "creative-intake-create-v3-overlay.json")
	mismatchCreate := readFixture(t, "creative-intake-create-v3-overlay-mismatch.json")
	overlay := readFixture(t, "strategy-creative-task-overlay-v1-ready.json")
	assertContentHash(t, overlay)
	assertSameJSONValue(t, "v3 base strategy_package_ref", baseV3["strategy_package_ref"], readyRef)
	assertSameJSONValue(t, "v3 overlay strategy_package_ref", overlayCreate["strategy_package_ref"], readyRef)
	assertSameJSONValue(t, "overlay package_ref", overlay["package_ref"], readyRef)
	if overlayCreate["selected_route_id"] != overlay["selected_route_id"] {
		t.Fatal("ready v3 overlay fixture route does not match its task overlay")
	}
	if mismatchCreate["selected_route_id"] == overlay["selected_route_id"] {
		t.Fatal("mismatch fixture must exercise route-lineage rejection")
	}
}

func assertContentHash(t *testing.T, value map[string]any) {
	t.Helper()
	expected := requiredString(t, value, "content_hash")
	hashInput := cloneJSONMap(t, value)
	delete(hashInput, "content_hash")
	actual, err := contract.NewContentHash(hashInput)
	if err != nil {
		t.Fatalf("canonicalize content: %v", err)
	}
	if string(actual) != expected {
		t.Fatalf("content_hash mismatch: fixture=%s calculated=%s", expected, actual)
	}
}

func readFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	path := filepath.Join("..", "..", "..", "api", "fixtures", name)
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var value map[string]any
	if err := json.Unmarshal(payload, &value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return value
}

func assertHandoffHash(t *testing.T, handoff map[string]any) {
	t.Helper()
	expected := requiredString(t, handoff, "handoff_content_hash")
	hashInput := cloneJSONMap(t, handoff)
	delete(hashInput, "handoff_content_hash")
	actual, err := contract.NewContentHash(hashInput)
	if err != nil {
		t.Fatalf("canonicalize handoff: %v", err)
	}
	if string(actual) != expected {
		t.Fatalf("handoff_content_hash mismatch: fixture=%s calculated=%s", expected, actual)
	}
}

func intakePackageRef(t *testing.T, handoff map[string]any) map[string]any {
	t.Helper()
	packageRef, ok := handoff["package_ref"].(map[string]any)
	if !ok {
		t.Fatal("handoff package_ref is missing")
	}
	return map[string]any{
		"package_id":               packageRef["package_id"],
		"package_version":          packageRef["package_version"],
		"package_content_hash":     packageRef["package_content_hash"],
		"handoff_contract_version": handoff["contract_version"],
		"handoff_content_hash":     handoff["handoff_content_hash"],
	}
}

func assertSameJSONValue(t *testing.T, name string, actual, expected any) {
	t.Helper()
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("%s does not match frozen handoff: actual=%v expected=%v", name, actual, expected)
	}
}

func assertRouteExists(t *testing.T, handoff map[string]any, selectedRouteID string) {
	t.Helper()
	routes, ok := handoff["routes"].([]any)
	if !ok {
		t.Fatal("handoff routes are missing")
	}
	for _, value := range routes {
		route, ok := value.(map[string]any)
		if ok && route["route_id"] == selectedRouteID {
			return
		}
	}
	t.Fatalf("selected route %q is not present in handoff", selectedRouteID)
}

func assertReadyIntakeState(t *testing.T, intake map[string]any) {
	t.Helper()
	if intake["status"] != "ready" {
		t.Fatalf("ready fixture status=%v", intake["status"])
	}
	if requiredString(t, intake, "confirmed_by") == "" {
		t.Fatal("ready fixture confirmed_by is empty")
	}
	readiness, ok := intake["readiness"].(map[string]any)
	if !ok {
		t.Fatal("ready fixture readiness is missing")
	}
	planning, _ := readiness["planning_ready"].(bool)
	generation, _ := readiness["generation_ready"].(bool)
	production, _ := readiness["production_ready"].(bool)
	if !planning || production && !generation {
		t.Fatalf("invalid readiness implication: %v", readiness)
	}
}

func requiredString(t *testing.T, value map[string]any, field string) string {
	t.Helper()
	result, ok := value[field].(string)
	if !ok || result == "" {
		t.Fatalf("%s is missing", field)
	}
	return result
}

func cloneJSONMap(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode fixture clone: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("decode fixture clone: %v", err)
	}
	return result
}
